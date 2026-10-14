package schema

import "maps"

import "strings"

// Local JSON Schema reference keywords resolved by expandSchema. Only local
// pointers are expanded; remote or file references are left untouched.
const (
	refKeyword     = "$ref"
	allOfKeyword   = "allOf"
	refPrefix      = "#/"
	maxRefDepth    = 64
	definitionsKey = "definitions"
)

// expandSchema returns a copy of raw with local $ref resolved and allOf branches
// merged into their enclosing node, so every walker (defaults, materialization,
// type lookup, path lookup) sees the same effective schema (rule G12). raw itself
// is never modified, so the compiled schema and validation stay unchanged.
func expandSchema(raw any) map[string]any {
	root, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	expanded, ok := expandNode(root, root, 0, map[string]bool{}).(map[string]any)
	if !ok {
		return root
	}
	return expanded
}

// expandNode copies one schema node with its local references resolved.
func expandNode(node any, root map[string]any, depth int, seen map[string]bool) any {
	object, ok := node.(map[string]any)
	if !ok || depth > maxRefDepth {
		return node
	}
	merged := map[string]any{}

	if ref, ok := object[refKeyword].(string); ok && !seen[ref] {
		if target, ok := resolveLocalRef(root, ref); ok {
			seen[ref] = true
			if targetObject, ok := expandNode(target, root, depth+1, seen).(map[string]any); ok {
				maps.Copy(merged, targetObject)
			}
			delete(seen, ref)
		}
	}

	if branches, ok := object[allOfKeyword].([]any); ok {
		for _, branch := range branches {
			branchObject, ok := expandNode(branch, root, depth+1, seen).(map[string]any)
			if !ok {
				continue
			}
			mergeInherited(merged, branchObject)
		}
	}

	// The node's own keys win over anything it referenced or inherited.
	for key, value := range object {
		if key == refKeyword || key == allOfKeyword {
			continue
		}
		merged[key] = value
	}

	if properties, ok := merged[keywordProperties].(map[string]any); ok {
		expanded := make(map[string]any, len(properties))
		for key, value := range properties {
			expanded[key] = expandNode(value, root, depth+1, seen)
		}
		merged[keywordProperties] = expanded
	}
	if items, ok := merged[keywordItems]; ok {
		merged[keywordItems] = expandNode(items, root, depth+1, seen)
	}
	return merged
}

// mergeInherited copies keys that base does not declare yet, so earlier
// declarations win and the merged result is deterministic.
func mergeInherited(base, inherited map[string]any) {
	for key, value := range inherited {
		if key != keywordProperties {
			if _, exists := base[key]; !exists {
				base[key] = value
			}
			continue
		}
		incoming, ok := value.(map[string]any)
		if !ok {
			continue
		}
		existing, ok := base[key].(map[string]any)
		if !ok {
			base[key] = incoming
			continue
		}
		merged := make(map[string]any, len(existing)+len(incoming))
		maps.Copy(merged, incoming)
		maps.Copy(merged, existing)
		base[key] = merged
	}
}

// resolveLocalRef resolves a "#/a/b" JSON pointer inside root, with the ~0/~1
// escapes JSON Pointer defines.
func resolveLocalRef(root map[string]any, ref string) (any, bool) {
	if !strings.HasPrefix(ref, refPrefix) {
		return nil, false
	}
	pointer := strings.TrimPrefix(ref, refPrefix)
	if pointer == "" {
		return root, true
	}
	var current any = root
	for segment := range strings.SplitSeq(pointer, "/") {
		segment = strings.ReplaceAll(segment, "~1", "/")
		segment = strings.ReplaceAll(segment, "~0", "~")
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[segment]
		if !ok {
			return nil, false
		}
	}
	return current, true
}
