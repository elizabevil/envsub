package extended

import "fmt"

// Paths lists the variable paths a template references, deduplicated and in
// first-appearance order. The render layer uses it for --explain; it never
// resolves values, so it cannot change rendering.
func (e *Engine) Paths(src []byte) ([]string, error) {
	var paths []string
	seen := make(map[string]bool)
	err := ScanSpans(src, func(span Span) error {
		if span.Kind != TokenVariable {
			return nil
		}
		// A bare ${NAME} or $NAME needs no parsing; anything else carries a path
		// in its token.
		if name, ok := simpleSpanName(src, span); ok {
			if !seen[string(name)] {
				seen[string(name)] = true
				paths = append(paths, string(name))
			}
			return nil
		}
		token := spanToken(src, span)
		expr, err := parseExpression(token.Path)
		if err != nil {
			return fmt.Errorf("parse %q: %w", token.Path, err)
		}
		if expr.Path != "" && !seen[expr.Path] {
			seen[expr.Path] = true
			paths = append(paths, expr.Path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return paths, nil
}
