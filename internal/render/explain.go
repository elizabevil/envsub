package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/jsonx"
	"github.com/elizabevil/envsub/internal/load"
	"github.com/elizabevil/envsub/internal/values"
)

// --explain vocabulary (docs/07-serialization/13-materialization.md §15).
const (
	// explainMissing marks a referenced variable that resolved to nothing.
	explainMissing = "missing"
	// explainProcess marks a value read from the process environment through the
	// @process namespace.
	explainProcess = "process"
)

// explainRecord is one JSON line of the --explain report: where a referenced
// variable resolved, never its value, because values may be secrets.
type explainRecord struct {
	// Template is the template the variable is referenced from.
	Template string `json:"template"`
	// Variable is the referenced path in its canonical spelling, so it can be
	// pasted back into a template or used as an --explain=PATH filter.
	Variable string `json:"variable"`
	// Source is the human-readable origin: set, process, schema, missing, a
	// file, or file:line.
	Source string `json:"source"`
	// Kind classifies the origin: set, process, schema, missing or the loader
	// name of a value file (dotenv, yaml, json).
	Kind string `json:"kind,omitempty"`
	// File is the source file, when the origin is file-backed.
	File string `json:"file,omitempty"`
	// Line is the 1-based source line, when it is known. It is a pointer so the
	// field is omitted identically by both JSON backends: encoding/json/v2 keeps
	// a zero int under `omitempty` while sonic drops it (see jsonx tests).
	Line *int `json:"line,omitempty"`
	// Shadowed lists the origins the winner covered, closest first.
	Shadowed []string `json:"shadowed,omitempty"`
}

// explainRun collects origins for one render run. It is the load.Observer the
// loader reports through, so the loader does not depend on how origins are
// reported. A zero value records nothing.
type explainRun struct {
	sources explainer
	// tree supplies origins the layering did not record, for example a value
	// written by a template assignment.
	tree *values.Tree
}

// AddEnv implements load.Observer.
func (run *explainRun) AddEnv(name string) {
	run.sources.addEnv(name, explainOrigin{kind: explainProcess})
}

// AddRoot implements load.Observer.
func (run *explainRun) AddRoot(path string, origin load.Origin) {
	run.sources.addRoot(path, explainOrigin{
		kind: string(origin.Kind),
		file: origin.File,
		line: origin.Line,
	})
}

// explainOrigin is one recorded origin of a value.
type explainOrigin struct {
	// kind is set, process, schema, missing or a loader name (dotenv, yaml, json).
	kind string
	// file is the source file, empty when the origin has none.
	file string
	// line is the 1-based source line, 0 when unknown.
	line int
}

// valid reports whether the origin carries any information.
func (o explainOrigin) valid() bool {
	return o.kind != "" || o.file != ""
}

// source formats the origin the way the report renders it.
func (o explainOrigin) source() string {
	switch {
	case o.file != "" && o.line > 0:
		return fmt.Sprintf("%s:%d", o.file, o.line)
	case o.file != "":
		return o.file
	case o.kind != "":
		return o.kind
	default:
		return explainMissing
	}
}

// explainer records the origins of a name, lowest priority first: the last entry
// wins and the earlier ones were shadowed. A zero explainer records nothing, so
// "not reporting" needs no nil value.
type explainer struct {
	// root maps an encoded root path to its origins.
	root map[string][]explainOrigin
	// env maps an environment variable name to its origins.
	env map[string][]explainOrigin
}

// addRoot records one root-namespace origin for an encoded path.
func (e *explainer) addRoot(key string, origin explainOrigin) {
	if key == "" || !origin.valid() {
		return
	}
	if e.root == nil {
		e.root = map[string][]explainOrigin{}
	}
	e.root[key] = append(e.root[key], origin)
}

// addEnv records one environment origin for a variable name.
func (e *explainer) addEnv(name string, origin explainOrigin) {
	if name == "" || !origin.valid() {
		return
	}
	if e.env == nil {
		e.env = map[string][]explainOrigin{}
	}
	e.env[name] = append(e.env[name], origin)
}

// source returns the winning origin for a path and the origins it shadowed,
// closest loser first.
func (e explainer) source(path string, tree *values.Tree) (winner explainOrigin, shadowed []string) {
	// A namespace path reads only the process environment; any other path reads
	// the root layering the observer recorded.
	envName, rootKey := explainLookupKeys(path)
	var candidates []explainOrigin
	if envName != "" {
		candidates = append(candidates, e.env[envName]...)
	}
	if rootKey != "" {
		candidates = append(candidates, e.root[rootKey]...)
	}
	if len(candidates) > 0 {
		winner = candidates[len(candidates)-1]
		for i := len(candidates) - 2; i >= 0; i-- {
			shadowed = append(shadowed, candidates[i].source())
		}
		return winner, shadowed
	}
	// A value the layering did not record still carries meta on the tree.
	if tree != nil {
		if ref := tree.MetaFor(path); ref.Found {
			if origin := metaOrigin(ref.Meta); origin.valid() {
				return origin, nil
			}
		}
	}
	return explainOrigin{kind: explainMissing}, nil
}

// matches reports whether a reported name is selected by the --explain path
// filter. An empty filter selects every name; matches are exact.
func (e Explain) matches(name string) bool {
	if len(e.Paths) == 0 {
		return true
	}
	for _, path := range e.Paths {
		if explainName(path) == name {
			return true
		}
	}
	return false
}

// explainTemplate writes one JSON object per referenced variable, in
// first-appearance order. It only reads names and recorded origins, so it cannot
// change the rendered output.
func explainTemplate(w io.Writer, pathEngine engine.PathEngine, req Request, tmpl string, src []byte, run explainRun) error {
	paths, err := pathEngine.Paths(src)
	if err != nil {
		return diag.Wrap(diag.KindTemplate, fmt.Errorf("explain %s: %w", tmpl, err))
	}
	var b strings.Builder
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		name := explainName(path)
		if name == "" || seen[name] || !req.Explain.matches(name) {
			continue
		}
		seen[name] = true
		origin, shadowed := run.sources.source(path, run.tree)
		var line *int
		if origin.line > 0 {
			line = &origin.line
		}
		encoded, err := jsonx.Marshal(explainRecord{
			Template: tmpl,
			Variable: name,
			Source:   origin.source(),
			Kind:     origin.kind,
			File:     origin.file,
			Line:     line,
			Shadowed: shadowed,
		})
		if err != nil {
			return diag.Wrap(diag.KindInternal, fmt.Errorf("encode explain record: %w", err))
		}
		b.Write(encoded)
		b.WriteByte('\n')
	}
	if _, err := io.WriteString(w, b.String()); err != nil {
		return diag.Wrap(diag.KindIO, fmt.Errorf("write explain: %w", err))
	}
	return nil
}

// explainName is the canonical spelling of a path, so the report can be pasted
// back into a template and matches an --explain=PATH filter exactly. A path the
// parser cannot read is reported verbatim.
func explainName(path string) string {
	trimmed := strings.TrimSpace(path)
	segments, err := values.ParsePath(trimmed)
	if err != nil {
		return trimmed
	}
	return values.Canonical(segments)
}

// explainLookupKeys maps a path to the process environment name and the encoded
// root path the renderer consults; one of the two is always empty.
func explainLookupKeys(path string) (envName, rootKey string) {
	segments, err := values.ParsePath(strings.TrimSpace(path))
	if err != nil {
		return "", ""
	}
	if namespace := values.Namespace(segments); namespace != "" {
		name, ok := values.EnvKey(segments)
		if !ok {
			return "", ""
		}
		return name, ""
	}
	return "", values.EncodeSegments(segments)
}

// metaOrigin formats a recorded origin as an explain origin.
func metaOrigin(meta values.Meta) explainOrigin {
	switch values.SourceKind(meta.Kind) {
	case values.SourceEnv:
		// The environment namespace reports as "process" even though its
		// metadata kind is "env".
		return explainOrigin{kind: explainProcess}
	case "":
		return explainOrigin{file: meta.File, line: meta.Line}
	default:
		return explainOrigin{kind: meta.Kind, file: meta.File, line: meta.Line}
	}
}
