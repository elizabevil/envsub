package extended

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/elizabevil/envsub/internal/codec"
	"github.com/elizabevil/envsub/internal/diag"
)

const (
	charDollar     = '$'
	charBraceOpen  = '{'
	charBraceClose = '}'
	charQuote      = '"'
	charEscape     = '\\'
)

// TokenKind identifies a scanner token.
type TokenKind int

const (
	// TokenText is literal template text.
	TokenText TokenKind = iota
	// TokenVariable is a $NAME or ${...} reference.
	TokenVariable
	// TokenEscapedDollar is a literal $ produced by $$. It is a distinct span
	// so the renderer can emit one byte without rebuilding a text run.
	TokenEscapedDollar
)

// Token is one lexical unit of an extended template.
type Token struct {
	// Kind is the token category.
	Kind TokenKind
	// Text is the literal content of a TokenText.
	Text string
	// Path is the raw path expression of a TokenVariable.
	Path string
	// Braced is true for ${path} and false for $NAME.
	Braced bool
	// Pos is the byte offset of the token in the source template.
	Pos int
	// Raw is the verbatim source spelling, used by --on-missing=keep.
	Raw string
}

func (t Token) String() string {
	if t.Kind == TokenText {
		return t.Text
	}
	if t.Braced {
		return "${" + t.Path + "}"
	}
	return "$" + t.Path
}

// Span is a token expressed as byte offsets into the template, avoiding the
// copies a materialized Token needs. The streaming renderer works on spans so
// large templates are never duplicated in memory.
type Span struct {
	// Kind is the token category, matching Token.Kind.
	Kind TokenKind
	// Start is the first byte of the span.
	Start int
	// End is one past the last byte of the span.
	End int
	// Braced is true for ${path} and false for $NAME.
	Braced bool
}

// Position returns the 1-based line and column of a byte offset in src.
// Line counting goes through bytes.Count and the column through
// bytes.LastIndexByte, both of which keep the two values consistent without a
// hand-written byte loop.
func Position(src []byte, offset int) (line, column int) {
	if offset > len(src) {
		offset = len(src)
	}
	if offset < 0 {
		offset = 0
	}
	prefix := src[:offset]
	line = bytes.Count(prefix, newlineBytes) + 1
	if cut := bytes.LastIndexByte(prefix, '\n'); cut >= 0 {
		column = offset - cut
	} else {
		column = offset + 1
	}
	return line, column
}

// Scan tokenizes a template using the TASK-005 syntax subset and materializes
// every token. Adjacent text, including the literal $ from $$, is merged into
// one text token. Prefer ScanSpans for large templates.
func Scan(input []byte) ([]Token, error) {
	var tokens []Token
	var pending strings.Builder
	flush := func() {
		if pending.Len() == 0 {
			return
		}
		tokens = append(tokens, Token{Kind: TokenText, Text: pending.String()})
		pending.Reset()
	}

	err := ScanSpans(input, func(span Span) error {
		switch span.Kind {
		case TokenText:
			pending.Write(input[span.Start:span.End])
		case TokenEscapedDollar:
			pending.WriteByte(charDollar)
		case TokenVariable:
			flush()
			tokens = append(tokens, spanToken(input, span))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	flush()
	return tokens, nil
}

// ScanSpans tokenizes a template and calls emit for each token without copying
// the token text. An error returned by emit stops the scan and is returned
// unchanged.
//
// The syntax is this project's own, so no standard-library scanner applies:
// text/template evaluates rather than tokenizes, and the scanner must report
// byte offsets, keep the verbatim ${...} spelling for --on-missing=keep, and
// treat $$, nested braces and quoted brace text specially.
func ScanSpans(input []byte, emit func(Span) error) error {
	textStart := 0

	for i := 0; i < len(input); {
		if input[i] != charDollar {
			i++
			continue
		}

		start := i
		if i+1 >= len(input) {
			// A trailing $ is literal text.
			i++
			continue
		}

		switch next := input[i+1]; {
		case next == charDollar:
			// $$ collapses to a single literal $, so the surrounding text is
			// not contiguous and must be flushed on both sides.
			if err := emitText(input, textStart, start, emit); err != nil {
				return err
			}
			if err := emit(Span{Kind: TokenEscapedDollar, Start: start, End: start + 2}); err != nil {
				return err
			}
			i += 2
			textStart = i
		case next == charBraceOpen:
			nextIndex, err := scanBraced(input, i+2)
			if err != nil {
				return diag.WithOffset(diag.Wrap(diag.KindTemplate, fmt.Errorf("offset %d: %w", start, err)), start)
			}
			if err := emitText(input, textStart, start, emit); err != nil {
				return err
			}
			if err := emit(Span{Kind: TokenVariable, Start: start, End: nextIndex + 1, Braced: true}); err != nil {
				return err
			}
			i = nextIndex + 1
			textStart = i
		case isIdentifierStart(next):
			j := i + 2
			for j < len(input) && isIdentifierByte(input[j]) {
				j++
			}
			if err := emitText(input, textStart, start, emit); err != nil {
				return err
			}
			if err := emit(Span{Kind: TokenVariable, Start: start, End: j}); err != nil {
				return err
			}
			i = j
			textStart = i
		default:
			// $1, $-, $ at an invalid position: keep the dollar literal.
			i++
		}
	}

	return emitText(input, textStart, len(input), emit)
}

// emitText emits input[from:to] as one text token when it is non-empty.
func emitText(_ []byte, from, to int, emit func(Span) error) error {
	if from >= to {
		return nil
	}
	return emit(Span{Kind: TokenText, Start: from, End: to})
}

// spanToken materializes one span into a Token.
func spanToken(input []byte, span Span) Token {
	if span.Kind == TokenText {
		return Token{Kind: TokenText, Text: string(input[span.Start:span.End])}
	}
	raw := input[span.Start:span.End]
	token := Token{
		Kind:   TokenVariable,
		Braced: span.Braced,
		Pos:    span.Start,
		Raw:    string(raw),
	}
	if span.Braced {
		token.Path = string(raw[2 : len(raw)-1])
	} else {
		token.Path = string(raw[1:])
	}
	return token
}

// scanBraced finds the closing brace of a ${...} expression that starts at
// start (just past the opening brace). It returns the index of that brace and
// does not copy the expression text; callers slice the template when needed.
func scanBraced(input []byte, start int) (int, error) {
	depth := 1
	inQuote := false
	escaped := false

	for i := start; i < len(input); i++ {
		ch := input[i]

		if inQuote {
			switch {
			case escaped:
				escaped = false
			case ch == charEscape:
				escaped = true
			case ch == charQuote:
				inQuote = false
			}
			continue
		}

		switch ch {
		case charQuote:
			inQuote = true
		case charBraceOpen:
			depth++
		case charBraceClose:
			depth--
			if depth == 0 {
				if i == start {
					return 0, fmt.Errorf("empty variable expression")
				}
				return i, nil
			}
		}
	}

	return 0, fmt.Errorf("unclosed variable expression")
}

// isIdentifierStart reports whether ch can start an unquoted identifier or a
// $NAME variable.
func isIdentifierStart(ch byte) bool {
	return ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

// isIdentifierByte reports whether ch can continue an unquoted identifier or a
// $NAME variable.
func isIdentifierByte(ch byte) bool {
	return isIdentifierStart(ch) || codec.IsDigit(rune(ch))
}
