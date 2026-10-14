package yamlx

import (
	"bytes"
	"encoding/json"
	"strconv"

	"github.com/elizabevil/envsub/internal/codec"
	"go.yaml.in/yaml/v3"
)

// marshalIndent is the mapping indentation used for rendered YAML. Two spaces
// matches the style of the values files and of the Kubernetes tooling the
// rendered output is usually consumed by.
const marshalIndent = 2

// Marshal encodes value as YAML.
//
// Numbers are written exactly (a json.Number keeps its digits instead of being
// widened to float64). A typed nil map or slice uses yaml.v3's default, "{}" and
// "[]", per the serialization rules (03-12 V1).
func Marshal(value any) ([]byte, error) {
	return MarshalIndent(value, marshalIndent)
}

// MarshalIndent encodes value with a nesting width of width spaces. A width of 0
// or less uses the default. The nesting width is the serializer's business: only
// it knows where a level begins, which is why it is not a text operation.
func MarshalIndent(value any, width int) ([]byte, error) {
	if err := codec.RejectNonFinite(value); err != nil {
		return nil, err
	}
	if width <= 0 {
		width = marshalIndent
	}
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(width)
	if err := encoder.Encode(normalizeForYAML(value)); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// normalizeForYAML converts the value tree into what yaml.v3 encodes correctly:
// a json.Number becomes a Go number, because yaml.v3 would quote it as a string.
// Typed nil collections are left alone so yaml.v3 writes its default "{}" and
// "[]" (rules 03-12 V1).
func normalizeForYAML(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			typed[key] = normalizeForYAML(child)
		}
		return typed
	case []any:
		for i, child := range typed {
			typed[i] = normalizeForYAML(child)
		}
		return typed
	case json.Number:
		return yamlNumber(typed)
	case codec.ExactInt:
		return yamlIntNode(typed)
	case codec.LiteralNumber:
		return yamlScalarNode(typed.String())
	default:
		return value
	}
}

// yamlScalarNode renders text-backed numbers as plain YAML scalars. The tag is
// left empty on purpose: yaml.v3 then resolves the text itself (digits as
// tagInt, a fraction or exponent as tagFloat) and writes it bare, while an
// explicit Tag would be printed as part of the value.
func yamlScalarNode(text string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: text}
}

// yamlIntNode renders an exact integer as a plain YAML scalar.
func yamlIntNode(value codec.ExactInt) *yaml.Node {
	return yamlScalarNode(value.String())
}

// yamlNumber converts a json.Number into the narrowest Go number that holds it
// exactly: int64, then uint64, then an exact integer node. A fractional or
// exponent literal becomes a text-backed scalar node so its spelling survives
// (rules F3/F6/Q6), matching jsonx; only a non-JSON spelling falls back to
// float64.
func yamlNumber(number json.Number) any {
	text := number.String()
	if codec.IsIntegerText(text) {
		if parsed, err := number.Int64(); err == nil {
			return parsed
		}
		if parsed, err := strconv.ParseUint(text, 10, 64); err == nil {
			return parsed
		}
		if exact, ok := codec.New(text); ok {
			return yamlIntNode(exact)
		}
		return text
	}
	if literal, ok := codec.NewLiteral(text); ok {
		return yamlScalarNode(literal.String())
	}
	if parsed, err := number.Float64(); err == nil {
		return parsed
	}
	return text
}
