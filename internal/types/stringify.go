package types

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/elizabevil/envsub/internal/codec"
)

// Aligned is implemented by a serialized document that carries the intent to be
// aligned by the substitution layer. It is declared here as an interface so the
// text helpers can recognise it without depending on the pipeline package.
type Aligned interface {
	AlignedText() string
}

// Stringify converts a resolved scalar value to text.
// Structured values must go through a formatter in the pipeline package.
func Stringify(value any) (string, error) {
	if aligned, ok := value.(Aligned); ok {
		return aligned.AlignedText(), nil
	}
	switch v := value.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case bool:
		return strconv.FormatBool(v), nil
	case int:
		return strconv.Itoa(v), nil
	case int8:
		return strconv.FormatInt(int64(v), 10), nil
	case int16:
		return strconv.FormatInt(int64(v), 10), nil
	case int32:
		return strconv.FormatInt(int64(v), 10), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case uint:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint64:
		return strconv.FormatUint(v, 10), nil
	case float32:
		if err := codec.RejectNonFinite(float64(v)); err != nil {
			return "", err
		}
		return strconv.FormatFloat(float64(v), 'f', -1, 32), nil
	case float64:
		// Rule F8/OV8: a non-finite value must not reach the output, whatever
		// path put it in the tree.
		if err := codec.RejectNonFinite(v); err != nil {
			return "", err
		}
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case codec.ExactInt:
		return v.String(), nil
	case codec.LiteralNumber:
		return v.String(), nil
	case json.Number:
		return formatJSONNumber(v), nil
	case []any:
		return "", fmt.Errorf("array value requires a formatter such as toJson or join")
	case map[string]any:
		return "", fmt.Errorf("object value requires a formatter such as toYaml or toJson")
	default:
		return "", fmt.Errorf("value type %T requires a formatter", value)
	}
}

func formatJSONNumber(number json.Number) string {
	text := number.String()
	if codec.IsIntegerText(text) {
		return text
	}
	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return text
	}
	return strconv.FormatFloat(parsed, 'f', -1, 64)
}

// numberLabel names the integer family, which has no distinct Kind.
const numberLabel = "number"

// Describe names the type of a value for diagnostics.
func Describe(value any) string {
	switch value.(type) {
	case nil:
		return string(Null)
	case string:
		return string(String)
	case bool:
		return string(Bool)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, json.Number, codec.ExactInt, codec.LiteralNumber:
		return numberLabel
	case float32, float64:
		return string(Float)
	case []any:
		return string(Array)
	case map[string]any:
		return string(Object)
	default:
		return fmt.Sprintf("%T", value)
	}
}
