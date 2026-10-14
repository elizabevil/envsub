package values

import (
	"fmt"
	"maps"

	"github.com/elizabevil/envsub/internal/utils"
)

// MergeInto merges src into dst, recording meta as the fallback origin of every
// value that src defines. A per-value origin recorded on src (a dotenv key keeps
// its own line number) wins over that fallback. Objects merge recursively,
// arrays and scalars replace.
func MergeInto(dst, src *Tree, meta Meta) {
	if dst.Root == nil {
		dst.Root = map[string]any{}
	}
	if dst.meta == nil {
		dst.meta = &metaNode{}
	}
	mergeMap(dst, src, dst.Root, src.Root, nil, meta)
	// The process environment namespace is flat, so a plain overlay copy is the
	// merge.
	maps.Copy(dst.Env, src.Env)
}

// mergeMap merges incoming into target, recording an origin for every encoded
// path it defines. src is the tree the incoming values came from; it supplies the
// per-key origins (a dotenv key keeps its own line number).
func mergeMap(dst, src *Tree, target, incoming map[string]any, prefix []Segment, meta Meta) {
	for key, value := range incoming {
		child := AppendSegment(prefix, Segment{Key: key})
		dst.RecordMeta(child, srcMeta(src, child, meta))
		incomingMap, isObject := value.(map[string]any)
		if !isObject {
			target[key] = CloneValue(value)
			continue
		}
		// Objects merge recursively; a scalar already at this key is replaced.
		existing, ok := target[key].(map[string]any)
		if !ok {
			existing = map[string]any{}
			target[key] = existing
		}
		mergeMap(dst, src, existing, incomingMap, child, meta)
	}
}

// srcMeta returns the origin recorded for key on the source tree, completed with
// the uniform origin the caller passed for the fields the key did not record. The
// two are combined because a dotenv key records its own line number but not the
// file name, which only the caller knows.
func srcMeta(src *Tree, path []Segment, fallback Meta) Meta {
	recorded, ok := src.metaAt(path)
	if !ok {
		return fallback
	}
	if recorded.File == "" {
		recorded.File = fallback.File
	}
	if recorded.Kind == "" {
		recorded.Kind = fallback.Kind
	}
	return recorded
}

// CloneValue returns a deep copy of a JSON/YAML compatible value, so callers that
// adopt a decoded document never alias the structure a loader returned.
func CloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		clone := make(map[string]any, len(typed))
		for key, child := range typed {
			clone[key] = CloneValue(child)
		}
		return clone
	case []any:
		clone := make([]any, len(typed))
		for i, child := range typed {
			clone[i] = CloneValue(child)
		}
		return clone
	default:
		return value
	}
}

// SeedOrigins records an empty origin for every value reachable from the root.
// A loader that assigns Root wholesale calls it so later consumers can enumerate
// the keys: MergeInto fills the file origin in, and --explain reports each one.
func (t *Tree) SeedOrigins() {
	if t.Root == nil {
		return
	}
	collectMetaPaths(t.Root, nil, func(path []Segment) {
		t.RecordMeta(path, Meta{})
	})
}

// collectMetaPaths walks a JSON-compatible value and reports every path.
func collectMetaPaths(value any, prefix []Segment, visit func(path []Segment)) {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range utils.SortedKeys(typed) {
			child := AppendSegment(prefix, Segment{Key: key})
			visit(child)
			collectMetaPaths(typed[key], child, visit)
		}
	case []any:
		for i, child := range typed {
			path := AppendSegment(prefix, Segment{IsIndex: true, Index: i})
			visit(path)
			collectMetaPaths(child, path, visit)
		}
	}
}

// DescribeMeta formats an origin for error messages.
// It returns an empty string when no origin is known.
func DescribeMeta(ref MetaRef) string {
	if !ref.Found {
		return ""
	}
	switch {
	case ref.Meta.File != "" && ref.Meta.Line > 0:
		return fmt.Sprintf("%s:%d", ref.Meta.File, ref.Meta.Line)
	case ref.Meta.File != "":
		return ref.Meta.File
	default:
		return ""
	}
}
