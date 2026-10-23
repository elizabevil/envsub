package extended

import "fmt"

// reference is one variable reference found while walking template text.
type reference struct {
	// Path is the referenced path, or the bare name.
	Path string
	// Op is the operator following the path, empty for a plain reference.
	Op string
	// Word is the operator's default/alternate/error word.
	Word string
	// Bare reports a ${NAME} or $NAME reference that needs no expression parse.
	Bare bool
}

// walkReferences scans src and calls visit for every variable reference. A
// non-bare reference's word is a template of its own, so when visit asks to
// recurse the word is walked as well. One traversal serves both --explain's
// path enumeration and the parallel renderer's dependency scan.
func walkReferences(src []byte, visit func(reference) (recurse bool, err error)) error {
	return ScanSpans(src, func(span Span) error {
		if span.Kind != TokenVariable {
			return nil
		}
		if name, ok := simpleSpanName(src, span); ok {
			_, err := visit(reference{Path: string(name), Bare: true})
			return err
		}
		token := spanToken(src, span)
		expr, err := parseExpression(token.Path)
		if err != nil {
			return fmt.Errorf("parse %q: %w", token.Path, err)
		}
		recurse, err := visit(reference{Path: expr.Path, Op: expr.Op, Word: expr.Word})
		if err != nil || !recurse || expr.Word == "" {
			return err
		}
		return walkReferences([]byte(expr.Word), visit)
	})
}
