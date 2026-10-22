package extended

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/pipeline"
	"github.com/elizabevil/envsub/internal/types"
	"github.com/elizabevil/envsub/internal/values"
)

const (
	opDefaultOrEmpty    = ":-"
	opDefaultIfUnset    = "-"
	opAlternateNonEmpty = ":+"
	opAlternateIfSet    = "+"
	opRequiredNonEmpty  = ":?"
	opRequiredIfSet     = "?"
	opAssignDefault     = ":="
	opAssignIfUnset     = "="
)

const (
	// pipeSeparator starts one formatter step.
	pipeSeparator = "|"
	// doubleQuote and singleQuote delimit quoted formatter arguments.
	doubleQuote = '"'
	singleQuote = '\''
	// charColon introduces the ":" forms of the operators (:- :+ :? :=).
	charColon = ':'
	// argumentSeparator separates a formatter name from its argument.
	argumentSeparator = " "
	// trimWhitespace is trimmed around paths, operator words and defaults.
	trimWhitespace = " \t"
)

// errNotExpression reports an empty or whitespace-only expression, which the
// scanner can produce inside nested ${...} text.
var errNotExpression = errors.New("empty expression")

// expression is one parsed template expression: a path, an optional
// default/existence operator, an optional inline type assertion and the
// formatter pipeline steps.
type expression struct {
	// Offset is the byte offset of the ${...} span in the template, used to
	// report the line an assignment writes from.
	Offset int
	// Path is the value path, before any operator.
	Path string
	// Op is the operator name, empty for a plain reference.
	Op string
	// Word is the default/alternate/error word following the operator.
	Word string
	// Declared is the inline | type assertion, valid when HasType is set.
	Declared types.Declared
	// HasType reports whether an inline | type assertion was given.
	HasType bool
	// Steps are the formatter calls following the path.
	Steps []pipeline.Step
}

// parseExpression parses the content of ${...}.
//
// The expression grammar (path, default/existence operator, inline | type and
// the formatter pipeline) is project-specific, so it has no standard-library
// parser. Quoted arguments are deliberately literal: no escape processing is
// performed, which is why strconv.Unquote does not apply here.
func parseExpression(raw string) (expression, error) {
	// Fast path: a bare identifier with no operator or pipeline step. This is
	// the dominant shape in generated configuration templates.
	if isBareIdentifier(raw) {
		return expression{Path: raw}, nil
	}
	segments, err := splitPipes(raw)
	if err != nil {
		return expression{}, err
	}
	if len(segments) == 0 {
		return expression{}, fmt.Errorf("empty expression")
	}

	parsed, err := parsePathAndOperator(segments[0])
	if err != nil {
		return expression{}, err
	}
	// A pipeline with two or more formatter steps: size the step slice once.
	// The first append already allocates a one-slot backing array, so
	// preallocating only pays off from the second step on; a pipeline of one
	// step (including the common | type) keeps allocating nothing up front.
	if steps := len(segments) - 1; steps > 1 {
		parsed.Steps = make([]pipeline.Step, 0, steps)
	}
	for _, segment := range segments[1:] {
		step, declared, hasType, err := parseStep(segment)
		if err != nil {
			return expression{}, err
		}
		if hasType {
			if parsed.HasType {
				return expression{}, fmt.Errorf("expression %q declares more than one type", raw)
			}
			if len(parsed.Steps) > 0 {
				return expression{}, fmt.Errorf("inline type must be the first pipeline step in %q", raw)
			}
			parsed.Declared = declared
			parsed.HasType = true
			continue
		}
		parsed.Steps = append(parsed.Steps, step)
	}
	return parsed, nil
}

// parsePathAndOperator parses the leading path and its operator.
func parsePathAndOperator(segment string) (expression, error) {
	segment = strings.TrimSpace(segment)
	path, rest, err := values.CutPathPrefix(segment)
	if err != nil {
		return expression{}, diag.WrapIfNot(diag.KindTemplate, err)
	}
	if rest == "" {
		return expression{Path: path}, nil
	}
	if strings.Trim(rest, trimWhitespace) == "" {
		// Trailing whitespace is not an expression; callers must not treat it
		// as a variable reference that swallows following text.
		return expression{}, errNotExpression
	}

	var op, word string
	switch {
	case len(rest) >= 2 && rest[0] == charColon && isOperatorRune(rest[1]):
		op, word = rest[:2], rest[2:]
	case len(rest) >= 1 && isOperatorRune(rest[0]):
		op, word = rest[:1], rest[1:]
	default:
		return expression{}, fmt.Errorf("invalid expression %q", segment)
	}
	if !isSupportedOperator(op) {
		return expression{}, fmt.Errorf("unsupported operator %q", op)
	}
	return expression{Path: path, Op: op, Word: word}, nil
}

// parseStep parses one pipeline step: a formatter call or the inline type
// assertion.
func parseStep(segment string) (step pipeline.Step, declared types.Declared, hasType bool, err error) {
	trimmed := strings.TrimSpace(segment)
	if trimmed == "" {
		return pipeline.Step{}, types.Declared{}, false, fmt.Errorf("empty pipeline step")
	}
	name, rest, _ := strings.Cut(trimmed, argumentSeparator)
	rest = strings.TrimSpace(rest)

	if name == pipeline.NameType {
		if rest == "" {
			return pipeline.Step{}, types.Declared{}, false, fmt.Errorf("type requires a type name")
		}
		parsed, err := types.ParseType(unquote(rest))
		if err != nil {
			// An unknown type name is user input, not a template syntax bug.
			return pipeline.Step{}, types.Declared{}, false, diag.Wrap(diag.KindUsage, err)
		}
		return pipeline.Step{}, parsed, true, nil
	}

	switch name {
	case pipeline.NameJoin:
		if rest == "" {
			return pipeline.Step{Name: name}, types.Declared{}, false, nil
		}
		separator, err := parseQuotedArgument(rest)
		if err != nil {
			return pipeline.Step{}, types.Declared{}, false, err
		}
		return pipeline.Step{Name: name, Arg: separator, HasArg: true}, types.Declared{}, false, nil
	case pipeline.NameIndent:
		if rest == "" {
			return pipeline.Step{}, types.Declared{}, false, fmt.Errorf("indent requires a width")
		}
		return pipeline.Step{Name: name, Arg: unquote(rest), HasArg: true}, types.Declared{}, false, nil
	case pipeline.NameToYAML:
		// toYaml takes an optional nesting width and/or the align flag, in any
		// order: `toYaml`, `toYaml 3`, `toYaml align`, `toYaml 3 align`.
		step := pipeline.Step{Name: name}
		for _, field := range strings.Fields(rest) {
			switch field {
			case alignArgument:
				if step.Align {
					return pipeline.Step{}, types.Declared{}, false, fmt.Errorf("toYaml repeats %q", alignArgument)
				}
				step.Align = true
			default:
				width, err := strconv.Atoi(field)
				if err != nil {
					return pipeline.Step{}, types.Declared{}, false,
						fmt.Errorf("toYaml argument %q is not a nesting width or %q", field, alignArgument)
				}
				if width < 1 {
					return pipeline.Step{}, types.Declared{}, false,
						fmt.Errorf("toYaml nesting width %d must be at least 1", width)
				}
				if step.Width != 0 {
					return pipeline.Step{}, types.Declared{}, false,
						fmt.Errorf("toYaml takes at most one nesting width")
				}
				step.Width = width
			}
		}
		return step, types.Declared{}, false, nil
	case pipeline.NameToJSON:
		// toJson is a single line, so it only takes the align flag.
		step := pipeline.Step{Name: name}
		for _, field := range strings.Fields(rest) {
			if field != alignArgument {
				return pipeline.Step{}, types.Declared{}, false,
					fmt.Errorf("toJson argument %q is not %q", field, alignArgument)
			}
			step.Align = true
		}
		return step, types.Declared{}, false, nil
	case pipeline.NameQuote:
		if rest != "" {
			return pipeline.Step{}, types.Declared{}, false, fmt.Errorf("%s does not take an argument", name)
		}
		return pipeline.Step{Name: name}, types.Declared{}, false, nil
	default:
		return pipeline.Step{}, types.Declared{}, false, fmt.Errorf("unknown formatter %q", name)
	}
}

// splitPipes splits a pipeline on unquoted '|' characters. Segments are
// substrings of raw, so a segment is copied only when the caller unquotes it;
// the old builder-based version copied every byte of every segment.
func splitPipes(raw string) ([]string, error) {
	segments := make([]string, 0, strings.Count(raw, pipeSeparator)+1)
	start := 0
	var quote byte
	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		switch {
		case quote != 0:
			if ch == quote {
				quote = 0
			}
		case ch == doubleQuote || ch == singleQuote:
			quote = ch
		case ch == pipeSeparator[0]:
			segments = append(segments, raw[start:i])
			start = i + 1
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unclosed quote in %q", raw)
	}
	return append(segments, raw[start:]), nil
}

// parseQuotedArgument parses a quoted formatter argument such as ",".
func parseQuotedArgument(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) < 2 || (trimmed[0] != doubleQuote && trimmed[0] != singleQuote) {
		return "", fmt.Errorf("argument %q must be quoted", raw)
	}
	quote := trimmed[0]
	if trimmed[len(trimmed)-1] != quote {
		return "", fmt.Errorf("unclosed quoted argument %q", raw)
	}
	return trimmed[1 : len(trimmed)-1], nil
}

// alignArgument is the keyword a serializer accepts to ask the substitution layer
// to align every line of its result with the expression's column.
const alignArgument = "align"

// unquote removes a single pair of surrounding quotes when present.
func unquote(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) >= 2 {
		first, last := trimmed[0], trimmed[len(trimmed)-1]
		if first == last && (first == doubleQuote || first == singleQuote) {
			return trimmed[1 : len(trimmed)-1]
		}
	}
	return trimmed
}

// isBareIdentifier reports whether raw is exactly one identifier character
// run, with no operator, quoting or pipeline separator.
//
// The first byte must start an identifier, not merely continue one: a name
// that begins with a digit is not a path, so it has to reach the path grammar
// and be reported as "expected identifier" instead of being taken for a
// reference the resolver cannot address. This is the same first-byte test the
// scanner's $NAME rule and simpleSpanName use.
func isBareIdentifier(raw string) bool {
	if raw == "" || !isIdentifierStart(raw[0]) {
		return false
	}
	for i := 1; i < len(raw); i++ {
		if !isIdentifierByte(raw[i]) {
			return false
		}
	}
	return true
}

func isOperatorRune(ch byte) bool {
	return strings.ContainsRune("-+=?", rune(ch))
}

func isSupportedOperator(op string) bool {
	switch op {
	case opDefaultOrEmpty, opDefaultIfUnset, opAlternateNonEmpty, opAlternateIfSet,
		opRequiredNonEmpty, opRequiredIfSet, opAssignDefault, opAssignIfUnset:
		return true
	default:
		return false
	}
}
