package values

import (
	"fmt"
	"maps"
	"slices"
)

// MergeInto merges src into dst, recording meta as the fallback origin of every
// value that src defines. A per-value origin recorded on src (a dotenv key keeps
// its own line number) wins over that fallback. Objects merge recursively,
// arrays and scalars replace.
func MergeInto(dst, src *Tree, meta Meta) {
	if dst == nil || src == nil {
		return
	}
	if dst.Root == nil {
		dst.Root = map[string]any{}
	}
	if dst.meta == nil {
		dst.meta = map[string]Meta{}
	}
	mergeMap(dst, src, dst.Root, src.Root, "", meta)
	// The process environment namespace is flat, so a plain overlay copy is the
	// merge.
	maps.Copy(dst.Env, src.Env)
}

// mergeMap merges incoming into target, recording an origin for every encoded
// path it defines. src is the tree the incoming values came from; it supplies the
// per-key origins (a dotenv key keeps its own line number).
func mergeMap(dst, src *Tree, target, incoming map[string]any, prefix string, meta Meta) {
	for key, value := range incoming {
		child := joinMetaKey(prefix, key)
		dst.meta[child] = srcMeta(src, child, meta)
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
func srcMeta(src *Tree, key string, fallback Meta) Meta {
	recorded, ok := src.MetaAt(key)
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

// joinMetaKey appends a map key to an encoded path prefix.
func joinMetaKey(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + string(pathSeparator) + key
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

// RecordRootMeta records an empty origin for every value reachable from the
// root. Loaders that replace Root wholesale call it so callers can attach a
// file origin afterwards.
func (t *Tree) RecordRootMeta() {
	if t == nil || t.Root == nil {
		return
	}
	if t.meta == nil {
		t.meta = map[string]Meta{}
	}
	collectMetaKeys(t.Root, "", func(key string) {
		t.meta[key] = Meta{}
	})
}

// collectMetaKeys walks a JSON-compatible value and reports every encoded path.
func collectMetaKeys(value any, prefix string, visit func(key string)) {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range sortedKeys(typed) {
			child := joinMetaKey(prefix, key)
			visit(child)
			collectMetaKeys(typed[key], child, visit)
		}
	case []any:
		for i, child := range typed {
			childKey := fmt.Sprintf(indexPathFormat, prefix, i)
			visit(childKey)
			collectMetaKeys(child, childKey, visit)
		}
	}
}

// sortedKeys returns the keys of a decoded object in a deterministic order.
//
// The slice is preallocated to len(object) and sorted with slices.Sort, which
// keeps it to one allocation; slices.Sorted(maps.Keys(object)) cannot size the
// slice up front and needs 7 allocations for 8 keys and 10 for 64 (measured).
func sortedKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
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
