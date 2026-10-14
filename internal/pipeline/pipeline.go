// Package pipeline applies the built-in formatter chain to rendered values.
package pipeline

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/jsonx"
	"github.com/elizabevil/envsub/internal/types"
	"github.com/elizabevil/envsub/internal/yamlx"
)

// Names of the built-in formatters. The whitelist is fixed: user-defined
// functions, code execution and network access are not supported.
const (
	NameJoin   = "join"
	NameToJSON = "toJson"
	NameToYAML = "toYaml"
	NameQuote  = "quote"
	NameIndent = "indent"
	NameType   = "type"
)

// DefaultJoinSeparator is used by join when no argument is given.
const DefaultJoinSeparator = ","

// newlineSeparator terminates the lines YAML emits and indent rewrites.
const newlineSeparator = "\n"

// Step is one formatter invocation of a pipeline.
type Step struct {
	// Name is the formatter name.
	Name string
	// Arg is the raw (already unquoted) argument, when the formatter takes one.
	Arg string
	// HasArg reports whether an argument was supplied.
	HasArg bool
	// Width is the nesting width a serializer should use, 0 when unset.
	Width int
	// Align asks the substitution layer to align every line of the result with
	// the column the expression starts on. A serializer sets it; without it a
	// multi-line value is inserted verbatim.
	Align bool
}

// Aligned is a serialized document that asked to be aligned by the substitution
// layer. It is a distinct type so a formatter that works on plain text (indent,
// quote, join) can reject it instead of silently stacking indentation.
type Aligned string

// AlignedText implements types.Aligned.
func (a Aligned) AlignedText() string { return string(a) }

// Text returns the document and whether it asked to be aligned.
func Text(value any) (string, bool) {
	if aligned, ok := value.(Aligned); ok {
		return string(aligned), true
	}
	return "", false
}

// Apply runs the steps in order, starting from value.
func Apply(value any, steps []Step) (any, error) {
	current := value
	for _, step := range steps {
		next, err := applyStep(current, step)
		if err != nil {
			return nil, err
		}
		current = next
	}
	return current, nil
}

func applyStep(value any, step Step) (any, error) {
	switch step.Name {
	case NameJoin:
		return formatJoin(value, step)
	case NameToJSON:
		return formatJSON(value, step)
	case NameToYAML:
		return formatYAML(value, step)
	case NameQuote:
		return formatQuote(value)
	case NameIndent:
		return formatIndent(value, step)
	default:
		return nil, diag.New(diag.KindTemplate, "unknown formatter %q", step.Name)
	}
}

// Names returns the supported formatter names, sorted.
func Names() []string {
	names := []string{NameJoin, NameToJSON, NameToYAML, NameQuote, NameIndent}
	slices.Sort(names)
	return names
}

func formatJoin(value any, step Step) (string, error) {
	separator := DefaultJoinSeparator
	if step.HasArg {
		separator = step.Arg
	}
	items, err := elementStrings(value)
	if err != nil {
		return "", err
	}
	return strings.Join(items, separator), nil
}

func formatJSON(value any, step Step) (any, error) {
	if step.Width != 0 {
		return nil, diag.New(diag.KindUsage, "toJson does not take a nesting width; use toYaml N")
	}
	encoded, err := jsonx.Marshal(value)
	if err != nil {
		return nil, diag.Wrap(diag.KindType, fmt.Errorf("toJson: %w", err))
	}
	return markAligned(string(encoded), step), nil
}

func formatYAML(value any, step Step) (any, error) {
	encoded, err := yamlx.MarshalIndent(value, step.Width)
	if err != nil {
		return nil, diag.Wrap(diag.KindType, fmt.Errorf("toYaml: %w", err))
	}
	text := strings.TrimRight(string(encoded), newlineSeparator)
	return markAligned(text, step), nil
}

// markAligned labels a serialized document that asked for alignment.
func markAligned(text string, step Step) any {
	if step.Align {
		return Aligned(text)
	}
	return text
}

func formatQuote(value any) (string, error) {
	text, err := types.Stringify(value)
	if err != nil {
		return "", err
	}
	return strconv.Quote(text), nil
}

func formatIndent(value any, step Step) (string, error) {
	if _, asked := Text(value); asked {
		return "", diag.New(diag.KindUsage,
			"indent cannot follow a serializer that asked to align; use align in the template or indent here, not both")
	}
	text, err := types.Stringify(value)
	if err != nil {
		return "", err
	}
	if !step.HasArg {
		return "", diag.New(diag.KindTemplate, "indent requires a non-negative integer argument")
	}
	width, err := strconv.Atoi(strings.TrimSpace(step.Arg))
	if err != nil {
		return "", diag.New(diag.KindTemplate, "indent argument %q is not an integer", step.Arg)
	}
	if width < 0 {
		return "", diag.New(diag.KindTemplate, "indent argument %d must not be negative", width)
	}
	if width == 0 || text == "" {
		return text, nil
	}
	prefix := strings.Repeat(" ", width)
	lines := strings.Split(text, newlineSeparator)
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, newlineSeparator), nil
}

// elementStrings converts each element of an array into text.
func elementStrings(value any) ([]string, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, diag.New(diag.KindType, "join requires an array value, got %s", types.Describe(value))
	}
	parts := make([]string, len(items))
	for i, item := range items {
		switch item.(type) {
		case []any:
			return nil, diag.New(diag.KindType, "join does not support nested arrays at index %d", i)
		case map[string]any:
			return nil, diag.New(diag.KindType, "join does not support objects at index %d", i)
		}
		text, err := types.Stringify(item)
		if err != nil {
			return nil, err
		}
		parts[i] = text
	}
	return parts, nil
}
