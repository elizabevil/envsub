package render

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/source"
	"github.com/elizabevil/envsub/internal/values"
)

// unusedPath is one value-file entry no template referenced, in the spelling
// that addresses it: app.port, hosts[0], ["extra.key"].
type unusedPath struct {
	path string
}

// collectUnused returns the value-file leaves that no template referenced,
// sorted. A parent is not reported when one of its children was referenced, and
// only leaves are considered: a subtree is configuration, not a value the
// template can resolve.
func collectUnused(tree *values.Tree, references map[string]bool) []unusedPath {
	if tree == nil {
		return nil
	}
	used := references
	var leaves []unusedPath
	var walk func(value any, segments []values.Segment)
	walk = func(value any, segments []values.Segment) {
		switch typed := value.(type) {
		case map[string]any:
			for _, key := range sortedKeys(typed) {
				walk(typed[key], append(segments, values.Segment{Key: key}))
			}
		case []any:
			for i, item := range typed {
				walk(item, append(segments, values.Segment{Index: i, IsIndex: true}))
			}
		default:
			leaves = append(leaves, unusedPath{path: values.Canonical(segments)})
		}
	}
	walk(tree.Root, nil)

	// A reference to a subtree covers everything under it, and a reference to a
	// leaf is itself covered by nothing above it; expand each reference over the
	// leaves first, then close over prefixes so a quoted key and its bracket
	// spelling compare equal.
	expanded := make(map[string]bool, len(used)+len(leaves))
	for reference := range used {
		expanded[reference] = true
		for _, leaf := range leaves {
			if isBelow(leaf.path, reference) {
				expanded[leaf.path] = true
			}
		}
	}
	referenced := closeOverPrefixes(expanded)

	out := make([]unusedPath, 0, len(leaves))
	for _, leaf := range leaves {
		if leaf.path == "" || referenced[normalizePrefix(leaf.path)] {
			continue
		}
		if meta := tree.MetaFor(leaf.path); !meta.Found || !source.IsValueFileKind(meta.Meta.Kind) {
			// Only a value file can hold dead configuration the user deletes;
			// a schema default, --set, --dotenv-file or the process namespace
			// cannot be "unused" in that sense.
			continue
		}
		out = append(out, leaf)
	}
	slices.SortFunc(out, func(a, b unusedPath) int { return strings.Compare(a.path, b.path) })
	return out
}

// sortedKeys returns the map keys in deterministic order.
func sortedKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// closeOverPrefixes returns the normalized spellings of every path and of every
// prefix of it.
func closeOverPrefixes(paths map[string]bool) map[string]bool {
	closed := make(map[string]bool, len(paths))
	for path := range paths {
		for prefix := path; prefix != ""; prefix = parentPath(prefix) {
			closed[normalizePrefix(prefix)] = true
		}
	}
	return closed
}

// isBelow reports whether path addresses something under prefix.
func isBelow(path, prefix string) bool {
	return strings.HasPrefix(path, prefix+".") || strings.HasPrefix(path, prefix+"[")
}

// normalizePrefix rewrites the bracket-quoted form to dots, so [\"a\"].b and
// a.b compare equal.
func normalizePrefix(path string) string {
	var b strings.Builder
	for i := 0; i < len(path); {
		if i+1 < len(path) && path[i] == '[' && path[i+1] >= '0' && path[i+1] <= '9' {
			end := strings.IndexByte(path[i:], ']')
			if end < 0 {
				b.WriteString(path[i:])
				break
			}
			if b.Len() > 0 {
				b.WriteByte('.')
			}
			b.WriteString(path[i+1 : i+end])
			i += end + 1
			if i < len(path) && path[i] == '.' {
				i++
			}
			continue
		}
		if i+1 < len(path) && path[i] == '[' && path[i+1] == '"' {
			end := strings.Index(path[i+2:], `"]`)
			if end < 0 {
				b.WriteString(path[i:])
				break
			}
			if b.Len() > 0 {
				b.WriteByte('.')
			}
			b.WriteString(path[i+2 : i+2+end])
			i += 2 + end + 2
			// Skip a single path separator right after the bracket.
			if i < len(path) && path[i] == '.' {
				i++
			}
			continue
		}
		b.WriteByte(path[i])
		i++
	}
	return b.String()
}

// writeUnused writes one path per line, sorted, so the report can be piped.
func writeUnused(w io.Writer, unused []unusedPath) error {
	var b strings.Builder
	for _, entry := range unused {
		b.WriteString(entry.path)
		b.WriteByte('\n')
	}
	if _, err := io.WriteString(w, b.String()); err != nil {
		return diag.Wrap(diag.KindIO, fmt.Errorf("write unused report: %w", err))
	}
	return nil
}

// parentPath drops the last segment of a path, or returns "" for a root path.
func parentPath(path string) string {
	if i := strings.LastIndexAny(path, ".["); i > 0 {
		return path[:i]
	}
	return ""
}
