package extended

import "fmt"

// Paths lists the variable paths a template references, deduplicated and in
// first-appearance order. The render layer uses it for --explain; it never
// resolves values, so it cannot change rendering.
func (e *Engine) Paths(src []byte) ([]string, error) {
	return e.PathsFor(src, nil)
}

// PathsFor is Paths restricted to the paths replaces keeps, implementing
// engine.PathFilterEngine. A nil predicate keeps everything.
func (e *Engine) PathsFor(src []byte, replaces func(string) bool) ([]string, error) {
	var paths []string
	seen := make(map[string]bool)
	if err := collectPaths(src, replaces, seen, &paths); err != nil {
		return nil, err
	}
	return paths, nil
}

// collectPaths appends the paths referenced by src that replaces keeps. An
// operator's word is a template of its own, so its paths are collected too:
// ${A:-${B}} references both A and B, even though only A is the expression's
// path. When the outer path is excluded, the renderer leaves the whole token
// literal and never evaluates the word, so the nested paths are not collected.
func collectPaths(src []byte, replaces func(string) bool, seen map[string]bool, paths *[]string) error {
	add := func(path string) {
		if path == "" || seen[path] {
			return
		}
		if replaces != nil && !replaces(path) {
			return
		}
		seen[path] = true
		*paths = append(*paths, path)
	}
	return ScanSpans(src, func(span Span) error {
		if span.Kind != TokenVariable {
			return nil
		}
		// A bare ${NAME} or $NAME needs no parsing; anything else carries a path
		// in its token.
		if name, ok := simpleSpanName(src, span); ok {
			add(string(name))
			return nil
		}
		token := spanToken(src, span)
		expr, err := parseExpression(token.Path)
		if err != nil {
			return fmt.Errorf("parse %q: %w", token.Path, err)
		}
		// An excluded expression is emitted verbatim before its word is
		// evaluated, so the word's references were never substituted.
		if replaces != nil && expr.Path != "" && !replaces(expr.Path) {
			return nil
		}
		add(expr.Path)
		if expr.Word == "" {
			return nil
		}
		// Nested words are rendered as templates of their own, so a reference
		// only mentioned in a fallback is still a reference --explain must
		// report and --report=unused must count as used.
		return collectPaths([]byte(expr.Word), replaces, seen, paths)
	})
}
