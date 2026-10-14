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

// collectUnused returns the value-file leaves that no template referenced, sorted,
// each in the spelling that addresses it (app.port, hosts[0], ["extra.key"]).
// A parent is not reported when one of its children was referenced, and only
// leaves are considered: a subtree is configuration, not a value the template can
// resolve.
func collectUnused(tree *values.Tree, references map[string]bool) []string {
	if tree == nil {
		return nil
	}
	used := references
	var leaves []string
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
			leaves = append(leaves, values.Canonical(segments))
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
			if isBelow(leaf, reference) {
				expanded[leaf] = true
			}
		}
	}
	referenced := closeOverPrefixes(expanded)

	out := make([]string, 0, len(leaves))
	for _, leaf := range leaves {
		if leaf == "" || referenced[normalizePrefix(leaf)] {
			continue
		}
		if meta := tree.MetaFor(leaf); !meta.Found || !source.IsValueFileKind(meta.Meta.Kind) {
			// Only a value file can hold dead configuration the user deletes;
			// a schema default, --set, --dotenv-file or the process namespace
			// cannot be "unused" in that sense.
			continue
		}
		out = append(out, leaf)
	}
	slices.Sort(out)
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

// normalizePrefix rewrites the bracket-quoted form to dots, so ["a.b"].c and
// a.b.c compare equal. Only a quoted key is rewritten: an index (extra[0]) and a
// plain key keep their spelling. Everything before the bracket is copied as it
// is, so a dot after it (["a.b"].c) needs no special case.
func normalizePrefix(path string) string {
	var b strings.Builder
	for {
		open := strings.IndexByte(path, '[')
		if open < 0 || open+1 >= len(path) || path[open+1] != '"' {
			b.WriteString(path)
			return b.String()
		}
		b.WriteString(path[:open])
		rest := path[open+2:]
		end := strings.Index(rest, `"]`)
		if end < 0 {
			// An unterminated bracket is not a quoted key; keep it verbatim.
			b.WriteString(path[open:])
			return b.String()
		}
		if key := rest[:end]; key != "" {
			if b.Len() > 0 {
				b.WriteByte('.')
			}
			b.WriteString(key)
		}
		path = rest[end+2:]
	}
}

// writeUnused writes one path per line, already sorted, so the report can be
// piped. The caller passes the sorted paths, so this is a single write.
func writeUnused(w io.Writer, unused []string) error {
	if len(unused) == 0 {
		return nil
	}
	if _, err := io.WriteString(w, strings.Join(unused, "\n")+"\n"); err != nil {
		return diag.Wrap(diag.KindIO, fmt.Errorf("write unused report: %w", err))
	}
	return nil
}

// parentPath drops the last segment of a path, or returns "" for a root path. The
// last segment decides the separator: a trailing bracket closes an index or a
// quoted key (extra[0] -> extra), anything else sits after a dot
// (app.port -> app).
func parentPath(path string) string {
	if strings.HasSuffix(path, "]") {
		return path[:strings.LastIndexByte(path, '[')]
	}
	if index := strings.LastIndexByte(path, '.'); index > 0 {
		return path[:index]
	}
	return ""
}
