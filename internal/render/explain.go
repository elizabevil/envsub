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

// explainFrame is one step of a value's lineage: which layering stage it came
// through, how it entered that stage, the address within the stage, and where
// that is on disk. Every field is always emitted, so a consumer needs no
// presence checks (ADR-005 I4). The four fields answer four different questions;
// for a file-backed frame Source and File hold the same path on purpose.
type explainFrame struct {
	// Kind is the layering stage: schema, values, envfile, set, process,
	// assign or missing.
	Kind string `json:"kind"`
	// Method is how the value entered that stage: default, file, flag, json,
	// env, assign or missing.
	Method string `json:"method"`
	// Source is the address within the stage: a file path, a schema pointer, an
	// environment variable name, or the key itself.
	Source string `json:"source"`
	// File is the file the frame lives in, empty when it has none.
	File string `json:"file"`
	// Line is the 1-based line, 0 when unknown or not applicable.
	Line int `json:"line"`
}

// explainRecord is one JSON line of the --explain report: where a referenced
// variable resolved, never its value, because values may be secrets.
type explainRecord struct {
	// Template is the template the variable is referenced from.
	Template string `json:"template"`
	// Variable is the referenced path in its canonical spelling, so it can be
	// pasted back into a template or used as an --explain=PATH filter.
	Variable string `json:"variable"`
	// Origin is the winning frame: the last step of the lineage.
	Origin explainFrame `json:"origin"`
	// Shadowed lists the frames the winner covered, closest first, so that
	// Shadowed followed by Origin is the whole lineage, lowest priority last.
	// It is always present, empty when nothing was covered.
	Shadowed []explainFrame `json:"shadowed"`
}

// explainRun collects origins for one render run. It is the load.Observer the
// loader reports through, so the loader does not depend on how origins are
// reported. A zero value records nothing.
type explainRun struct {
	sources explainer
	// tree supplies origins the layering did not record, for example a value
	// written by a template assignment. It must be set before the report is
	// written; the renderer assigns the loaded tree right after loading.
	tree *values.Tree
}

// AddEnv implements load.Observer.
func (run *explainRun) AddEnv(name string) {
	run.sources.addEnv(name, explainOrigin{
		kind:   string(values.SourceProcess),
		method: string(values.MethodEnv),
		source: name,
	})
}

// AddRoot implements load.Observer.
func (run *explainRun) AddRoot(path string, origin load.Origin) {
	frame := explainOrigin{
		kind:   string(origin.Kind),
		method: string(origin.Method),
		source: origin.Source,
		file:   origin.File,
		line:   origin.Line,
	}
	if frame.source == "" {
		frame.source = origin.File
	}
	run.sources.addRoot(path, frame)
}

// explainOrigin is one recorded step of a value's lineage.
type explainOrigin struct {
	// kind is the layering stage: schema, values, envfile, set, process or
	// missing.
	kind string
	// method is how the value entered the stage: default, file, flag, json,
	// env, assign or missing.
	method string
	// source is the address within the stage: a file path, a schema pointer, an
	// environment variable name, or the key itself.
	source string
	// file is the source file, empty when the origin has none.
	file string
	// line is the 1-based source line, 0 when unknown.
	line int
}

// valid reports whether the origin carries any information.
func (o explainOrigin) valid() bool {
	return o.kind != "" || o.file != ""
}

// missingOrigin is the frame reported for a name that resolved to nothing.
func missingOrigin() explainOrigin {
	return explainOrigin{
		kind:   string(values.SourceMissing),
		method: string(values.MethodMissing),
	}
}

// frame renders the origin as one report frame. Recording fills every field, so
// the frame is a direct copy: a missing kind or method would be a bug in the
// recording path, not something this layer should paper over.
func (o explainOrigin) frame() explainFrame {
	return explainFrame{
		Kind:   o.kind,
		Method: o.method,
		Source: o.source,
		File:   o.file,
		Line:   o.line,
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
func (e explainer) source(path string, tree *values.Tree) (winner explainOrigin, shadowed []explainFrame) {
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
			shadowed = append(shadowed, candidates[i].frame())
		}
		return winner, shadowed
	}
	// A value the layering did not record still carries meta on the tree.
	if ref := tree.MetaFor(path); ref.Found {
		if origin := metaOrigin(ref.Meta); origin.valid() {
			return origin, nil
		}
	}
	return missingOrigin(), nil
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
		if shadowed == nil {
			shadowed = []explainFrame{}
		}
		encoded, err := jsonx.Marshal(explainRecord{
			Template: tmpl,
			Variable: name,
			Origin:   origin.frame(),
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

// metaOrigin turns a value's recorded metadata into a frame. It is the fallback
// for values the layering did not report, for example a template assignment,
// whose metadata carries the writing template and line.
func metaOrigin(meta values.Meta) explainOrigin {
	kind := values.SourceKind(meta.Kind)
	if kind == "" {
		kind = values.SourceMissing
	}
	// Prefer the method the writer recorded; only infer it when the metadata
	// predates it (a set value written outside the loader).
	method := values.SourceMethod(meta.Method)
	if method == "" {
		switch kind {
		case values.SourceValues, values.SourceEnvFile:
			method = values.MethodFile
		case values.SourceSet:
			method = values.MethodFlag
		case values.SourceSchema:
			method = values.MethodDefault
		case values.SourceProcess:
			method = values.MethodEnv
		default:
			method = values.MethodMissing
		}
	}
	source := meta.Source
	if source == "" {
		source = meta.File
	}
	return explainOrigin{
		kind:   string(kind),
		method: string(method),
		source: source,
		file:   meta.File,
		line:   meta.Line,
	}
}
