package render

import (
	"strings"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/event"
	"github.com/elizabevil/envsub/internal/values"
)

// explainFrame is the shared origin-frame schema. It is an alias so the report
// and the event protocol cannot drift apart.
type explainFrame = event.OriginFrame

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
		case values.SourceAssign:
			method = values.MethodAssign
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
		// A value a template assignment wrote is the newest origin there is:
		// the layering ran before the render, so the assignment frame closes
		// the chain. The layering did not report it, so it comes from the tree.
		if meta, ok := treeMeta(tree, rootKey); ok &&
			(meta.Kind == values.SourceAssign || meta.Method == values.MethodAssign) {
			candidates = append(candidates, meta)
		}
	}
	if len(candidates) > 0 {
		winner = frameFor(candidates[len(candidates)-1])
		for i := len(candidates) - 2; i >= 0; i-- {
			shadowed = append(shadowed, frameFor(candidates[i]))
		}
		return winner, shadowed
	}
	if rootKey == "" {
		return frameFor(values.Meta{Kind: values.SourceMissing, Method: values.MethodMissing}), nil
	}
	// A value the layering did not record still carries meta on the tree. The
	// lookup is exact first: an ancestor's origin says where the parent came
	// from, not where this name came from. The ancestor is still the best answer
	// when the value really exists without meta of its own, which is what schema
	// materialization produces (a JSON object parsed out of a string).
	if meta, ok := treeMeta(tree, rootKey); ok {
		return frameFor(meta), nil
	}
	if tree != nil {
		if _, exists, err := tree.Resolve(rootKey); err == nil && exists {
			if ref := tree.MetaFor(rootKey); ref.Found {
				return frameFor(ref.Meta), nil
			}
		}
	}
	return frameFor(values.Meta{Kind: values.SourceMissing, Method: values.MethodMissing}), nil
}

// treeMeta returns the origin recorded exactly for path, tolerating a tree that
// was never attached (a zero explainRun): the report then simply has no tree
// fallback.
func treeMeta(tree *values.Tree, path string) (values.Meta, bool) {
	if tree == nil {
		return values.Meta{}, false
	}
	return tree.MetaAt(path)
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

// explainTemplate emits one explain event per referenced variable, in
// first-appearance order. It only reads names and recorded origins, so it cannot
// change the rendered output.
func explainTemplate(events *event.Writer, pathEngine engine.PathEngine, req Request, tmpl string, src []byte, run explainRun) error {
	paths, err := pathEngine.PathsFor(src, req.Policies.replaces)
	if err != nil {
		return diag.Wrapf(diag.KindTemplate, "explain %s: %w", tmpl, err)
	}
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
		if err := events.Emit(event.TypeExplain, event.ExplainData{
			Template: tmpl,
			Variable: name,
			Origin:   origin,
			Shadowed: shadowed,
		}); err != nil {
			return diag.Wrapf(diag.KindIO, "write explain: %w", err)
		}
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
