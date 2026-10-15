package render

import (
	"io"
	"strings"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/jsonx"
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
	run.sources.addEnv(name, values.Meta{
		Kind:   values.SourceProcess,
		Method: values.MethodEnv,
		Source: name,
	})
}

// AddRoot implements load.Observer. The frame comes from the tree's own metadata,
// so the report and the tree cannot disagree about where a value came from.
func (run *explainRun) AddRoot(path string, meta values.Meta) {
	run.sources.addRoot(path, meta)
}

// frameFor renders one recorded origin as a report frame. Recording fills every
// field except method and source, which are inferred when a writer recorded only
// the stage (for example a set value written outside the loader).
func frameFor(meta values.Meta) explainFrame {
	kind := meta.Kind
	if kind == "" {
		kind = values.SourceMissing
	}
	method := meta.Method
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
	return explainFrame{
		Kind:   string(kind),
		Method: string(method),
		Source: source,
		File:   meta.File,
		Line:   meta.Line,
	}
}

// recorded reports whether an origin carries any information.
func recorded(meta values.Meta) bool {
	return meta.Kind != "" || meta.File != ""
}

// explainer records the origins of a name, lowest priority first: the last entry
// wins and the earlier ones were shadowed. A zero explainer records nothing, so
// "not reporting" needs no nil value.
type explainer struct {
	// root maps an encoded root path to its origins.
	root map[string][]values.Meta
	// env maps an environment variable name to its origins.
	env map[string][]values.Meta
}

// addRoot records one root-namespace origin for an encoded path.
func (e *explainer) addRoot(key string, meta values.Meta) {
	if key == "" || !recorded(meta) {
		return
	}
	if e.root == nil {
		e.root = map[string][]values.Meta{}
	}
	e.root[key] = append(e.root[key], meta)
}

// addEnv records one environment origin for a variable name.
func (e *explainer) addEnv(name string, meta values.Meta) {
	if name == "" || !recorded(meta) {
		return
	}
	if e.env == nil {
		e.env = map[string][]values.Meta{}
	}
	e.env[name] = append(e.env[name], meta)
}

// source returns the winning origin for a path and the origins it shadowed,
// closest loser first.
func (e explainer) source(path string, tree *values.Tree) (winner explainFrame, shadowed []explainFrame) {
	// A namespace path reads only the process environment; any other path reads
	// the root layering the observer recorded.
	envName, rootKey := explainLookupKeys(path)
	var candidates []values.Meta
	if envName != "" {
		candidates = append(candidates, e.env[envName]...)
	}
	if rootKey != "" {
		candidates = append(candidates, e.root[rootKey]...)
	}
	if len(candidates) > 0 {
		winner = frameFor(candidates[len(candidates)-1])
		for i := len(candidates) - 2; i >= 0; i-- {
			shadowed = append(shadowed, frameFor(candidates[i]))
		}
		return winner, shadowed
	}
	// A value the layering did not record still carries meta on the tree.
	if ref := tree.MetaFor(path); ref.Found {
		return frameFor(ref.Meta), nil
	}
	return frameFor(values.Meta{Kind: values.SourceMissing, Method: values.MethodMissing}), nil
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
		return diag.Wrapf(diag.KindTemplate, "explain %s: %w", tmpl, err)
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
			Origin:   origin,
			Shadowed: shadowed,
		})
		if err != nil {
			return diag.Wrapf(diag.KindInternal, "encode explain record: %w", err)
		}
		b.Write(encoded)
		b.WriteByte('\n')
	}
	if _, err := io.WriteString(w, b.String()); err != nil {
		return diag.Wrapf(diag.KindIO, "write explain: %w", err)
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
	return values.CanonicalPath(segments)
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
	return "", values.CanonicalPath(segments)
}
