package yamlx

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/elizabevil/envsub/internal/codec"
	"go.yaml.in/yaml/v3"
)

// Decode parses a YAML document into JSON-compatible Go values: mappings become
// map[string]any, sequences []any, and scalars keep their native type.
//
// Decoding goes through yaml.Node rather than a JSON round trip so integers
// stay integers. Duplicate mapping keys are rejected, which yaml.v3 only does
// when decoding straight into a map.
func Decode(data []byte) (any, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	return nodeToValue(&document)
}

// Unmarshal decodes a YAML document into target with yaml.v3 semantics. It is
// the entry point for callers that want the library's own typing, such as the
// fixture runner reading meta.yaml.
func Unmarshal(data []byte, target any) error {
	return yaml.Unmarshal(data, target)
}

// nodeToValue converts a yaml.Node tree into JSON-compatible Go values.
func nodeToValue(node *yaml.Node) (any, error) {
	if node == nil {
		return nil, nil
	}
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) == 0 {
			return map[string]any{}, nil
		}
		return nodeToValue(node.Content[0])
	case yaml.MappingNode:
		return mappingValue(node)
	case yaml.SequenceNode:
		items := make([]any, 0, len(node.Content))
		for _, child := range node.Content {
			value, err := nodeToValue(child)
			if err != nil {
				return nil, err
			}
			items = append(items, value)
		}
		return items, nil
	case yaml.AliasNode:
		return nodeToValue(node.Alias)
	case yaml.ScalarNode:
		return scalarValue(node)
	default:
		return nil, fmt.Errorf("unsupported YAML node kind %d", node.Kind)
	}
}

// mergeKey is YAML's merge key: its value is a mapping, or a sequence of
// mappings, whose entries are added to the enclosing mapping.
const mergeKey = "<<"

// mappingValue converts one YAML mapping, rejecting duplicate keys and applying
// merge keys the way yaml.v3 does when decoding into a map: explicit keys win,
// and an earlier merge source wins over a later one.
func mappingValue(node *yaml.Node) (map[string]any, error) {
	object := make(map[string]any, len(node.Content)/2)
	var merged []map[string]any
	for i := 0; i+1 < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]
		var key string
		if keyNode.Tag == tagStr {
			// Plain and quoted keys are the overwhelmingly common case; taking the
			// text directly avoids a reflection call per key.
			key = keyNode.Value
		} else if err := keyNode.Decode(&key); err != nil {
			return nil, fmt.Errorf("decode mapping key: %w", err)
		}
		if key == mergeKey {
			sources, err := mergeSources(valueNode)
			if err != nil {
				return nil, err
			}
			merged = append(merged, sources...)
			continue
		}
		if _, exists := object[key]; exists {
			return nil, fmt.Errorf("line %d: duplicate mapping key %q", keyNode.Line, key)
		}
		value, err := nodeToValue(valueNode)
		if err != nil {
			return nil, err
		}
		object[key] = value
	}
	for _, source := range merged {
		for key, value := range source {
			if _, exists := object[key]; !exists {
				object[key] = value
			}
		}
	}
	return object, nil
}

// mergeSources decodes the value of a merge key into the mappings to merge.
func mergeSources(node *yaml.Node) ([]map[string]any, error) {
	value, err := nodeToValue(node)
	if err != nil {
		return nil, err
	}
	switch typed := value.(type) {
	case map[string]any:
		return []map[string]any{typed}, nil
	case []any:
		sources := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			source, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("merge key value must be a mapping or a sequence of mappings, got %T", item)
			}
			sources = append(sources, source)
		}
		return sources, nil
	default:
		return nil, fmt.Errorf("merge key value must be a mapping or a sequence of mappings, got %T", value)
	}
}

// YAML core-schema tags for the scalar types this package inspects. A tag is
// only read here, never written: yaml.v3 prints an explicit Tag as part of the
// value (see yamlScalarNode).
const (
	tagStr   = "!!str"
	tagInt   = "!!int"
	tagFloat = "!!float"
	tagBool  = "!!bool"
	tagNull  = "!!null"
)

// Boolean literals in the YAML 1.2 core schema. Other spellings (yes/on) stay
// strings in yaml.v3 and therefore never carry the !!bool tag.
const (
	trueText  = "true"
	falseText = "false"
)

// scalarValue decodes one YAML scalar by dispatching on the tag yaml.v3
// resolved, which avoids the reflection call node.Decode performs for every
// scalar. Spellings and tags this package does not parse directly fall back to
// the library, so the decoded values stay identical.
func scalarValue(node *yaml.Node) (any, error) {
	switch node.Tag {
	case tagStr:
		// Quoted scalars keep their string type even when they look like numbers.
		return node.Value, nil
	case tagNull:
		return nil, nil
	case tagBool:
		if strings.EqualFold(node.Value, trueText) {
			return true, nil
		}
		if strings.EqualFold(node.Value, falseText) {
			return false, nil
		}
	case tagInt:
		if value, ok := intValue(node.Value); ok {
			return value, nil
		}
	case tagFloat:
		// An integer outside uint64 is tagged !!float by yaml.v3; keep its exact
		// digits instead of rounding through float64 (rules N1/N3).
		if exact, ok := codec.New(node.Value); ok {
			return exact, nil
		}
		// NaN and ±Inf are not representable in JSON and must never enter the
		// tree (rules F8).
		if isNonFiniteText(node.Value) {
			return nil, fmt.Errorf("non-finite number %q is not allowed", node.Value)
		}
		// A fractional or exponent literal keeps its original text (rules
		// F3/F6/Q6). YAML-only spellings such as ".5"/"5."/"1_000" are not valid
		// JSON numbers, so NewLiteral rejects them and they are parsed below.
		if literal, ok := codec.NewLiteral(node.Value); ok {
			return literal, nil
		}
		if parsed, err := strconv.ParseFloat(node.Value, 64); err == nil {
			if math.IsNaN(parsed) || math.IsInf(parsed, 0) {
				return nil, fmt.Errorf("non-finite number %q is not allowed", node.Value)
			}
			return parsed, nil
		}
	}

	// Fall back to the library for any tag or spelling not handled above.
	var value any
	if err := node.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode scalar %q: %w", node.Value, err)
	}
	return value, nil
}

// intValue parses a YAML !!int scalar into the narrowest Go type that holds it
// exactly: int, then uint64, then an exact integer for digits that exceed
// uint64. Base 0 accepts the YAML spellings yaml.v3 resolves (0x, 0o, 0b and
// digit separators). It reports false when the text is not an integer, which
// sends the caller to the library fallback.
func intValue(text string) (any, bool) {
	if parsed, err := strconv.ParseInt(text, 0, 64); err == nil {
		return int(parsed), true
	}
	if parsed, err := strconv.ParseUint(text, 0, 64); err == nil {
		return parsed, true
	}
	if exact, ok := codec.New(text); ok {
		return exact, true
	}
	return nil, false
}

// isNonFiniteText reports whether text is one of YAML's non-finite float
// spellings (.nan, .inf, -.inf, …).
func isNonFiniteText(text string) bool {
	switch strings.ToLower(text) {
	case ".nan", ".inf", "+.inf", "-.inf":
		return true
	default:
		return false
	}
}
