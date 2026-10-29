package render

import (
	"github.com/elizabevil/envsub/internal/values"
)

// explainRun records the origins the loader reports and the tree the render left
// behind. It is the load.Observer for one render run, so the loader does not
// depend on how the report is built. A zero value records nothing.
type explainRun struct {
	// tree supplies origins the layering did not record, for example a value
	// written by a template assignment.
	tree *values.Tree
	// root maps a canonical root path to its origins, lowest priority first.
	root map[string][]values.Meta
	// env maps a process variable name to its origins; @process never mixes
	// with the root layering.
	env map[string][]values.Meta
}

// AddEnv implements load.Observer.
func (run *explainRun) AddEnv(name string) {
	addOrigin(&run.env, name, values.Meta{
		Kind:   values.SourceProcess,
		Method: values.MethodEnv,
		Source: name,
	})
}

// AddRoot implements load.Observer.
func (run *explainRun) AddRoot(path string, meta values.Meta) {
	addOrigin(&run.root, path, meta)
}

// addOrigin appends one origin, keeping the slice in layering order.
func addOrigin(index *map[string][]values.Meta, key string, meta values.Meta) {
	if key == "" || !recorded(meta) {
		return
	}
	if *index == nil {
		*index = map[string][]values.Meta{}
	}
	(*index)[key] = append((*index)[key], meta)
}

// lineage is a resolved origin chain: the winner plus the frames it covered,
// closest loser first. Shadowed is always an initialized slice so a report never
// serializes it as null.
type lineage struct {
	winner   explainFrame
	shadowed []explainFrame
}

// lineageFrom turns the observer's lowest-first candidate list into a lineage.
func lineageFrom(candidates []values.Meta) lineage {
	winner := frameFor(candidates[len(candidates)-1])
	shadowed := make([]explainFrame, 0, len(candidates)-1)
	for i := len(candidates) - 2; i >= 0; i-- {
		shadowed = append(shadowed, frameFor(candidates[i]))
	}
	return lineage{winner: winner, shadowed: shadowed}
}

// lineageOf is a lineage with a single frame.
func lineageOf(meta values.Meta) lineage {
	return lineage{winner: frameFor(meta), shadowed: []explainFrame{}}
}

// missingLineage is the empty lineage. Shadowed is an empty slice, not nil.
var missingLineage = lineage{winner: missingFrame, shadowed: []explainFrame{}}

// resolve returns the winning origin and its shadowed frames for one parsed
// reference. The order is deliberate:
//
//  1. @process reads only the process environment and never falls back to root;
//  2. the root layering is tried next, with a template assignment appended last;
//  3. an exact tree origin covers values written outside the loader;
//  4. an ancestor origin is used only when the value itself exists, which is what
//     schema materialization creates;
//  5. anything else is missing, never an inherited parent source.
func (run explainRun) resolve(key explainKey) lineage {
	if key.EnvName != "" {
		if candidates := run.env[key.EnvName]; len(candidates) > 0 {
			return lineageFrom(candidates)
		}
		return missingLineage
	}
	if key.RootKey == "" {
		return missingLineage
	}

	candidates := run.root[key.RootKey]
	if meta, ok := assignmentMeta(run.tree, key.RootKey); ok {
		candidates = appendMeta(candidates, meta)
	}
	if len(candidates) > 0 {
		return lineageFrom(candidates)
	}

	if meta, ok := exactMeta(run.tree, key.RootKey); ok {
		return lineageOf(meta)
	}
	if meta, ok := ancestorMetaIfValueExists(run.tree, key.RootKey); ok {
		return lineageOf(meta)
	}
	return missingLineage
}

// exactMeta returns the origin recorded exactly for path, tolerating a missing
// tree (a zero explainRun).
func exactMeta(tree *values.Tree, path string) (values.Meta, bool) {
	if tree == nil {
		return values.Meta{}, false
	}
	return tree.MetaAt(path)
}

// assignmentMeta reports the template write-back frame at path, the newest
// origin there is because it happens after the loader ran.
func assignmentMeta(tree *values.Tree, path string) (values.Meta, bool) {
	meta, ok := exactMeta(tree, path)
	if !ok || (meta.Kind != values.SourceAssign && meta.Method != values.MethodAssign) {
		return values.Meta{}, false
	}
	return meta, true
}

// ancestorMetaIfValueExists falls back to the nearest recorded ancestor only when
// the path itself resolves to a value. A missing child must not borrow the
// parent's origin.
func ancestorMetaIfValueExists(tree *values.Tree, path string) (values.Meta, bool) {
	if tree == nil {
		return values.Meta{}, false
	}
	if _, exists, err := tree.Resolve(path); err != nil || !exists {
		return values.Meta{}, false
	}
	ref := tree.MetaFor(path)
	return ref.Meta, ref.Found
}

// appendMeta returns candidates plus meta without mutating the map's backing
// slice.
func appendMeta(candidates []values.Meta, meta values.Meta) []values.Meta {
	out := make([]values.Meta, len(candidates)+1)
	copy(out, candidates)
	out[len(candidates)] = meta
	return out
}
