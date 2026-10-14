package values

import (
	"encoding/json"
	"strconv"

	"github.com/elizabevil/envsub/internal/codec"
)

// normalizeNumbers converts every json.Number in a decoded document into the value
// tree's canonical Go number. Nothing widens to float64, which would silently
// round: an integer becomes int, then uint64, then codec.ExactInt for the exact
// decimal text, and a fractional or exponent literal stays codec.LiteralNumber.
func normalizeNumbers(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			typed[key] = normalizeNumbers(child)
		}
		return typed
	case []any:
		for i, child := range typed {
			typed[i] = normalizeNumbers(child)
		}
		return typed
	case json.Number:
		return normalizeJSONNumber(typed)
	default:
		return value
	}
}

func normalizeJSONNumber(number json.Number) any {
	text := number.String()
	if codec.IsIntegerText(text) {
		if parsed, err := strconv.Atoi(text); err == nil {
			return parsed
		}
		if parsed, err := strconv.ParseUint(text, 10, 64); err == nil {
			return parsed
		}
		if exact, ok := codec.New(text); ok {
			return exact
		}
		return number
	}
	if literal, ok := codec.NewLiteral(text); ok {
		return literal
	}
	if parsed, err := number.Float64(); err == nil {
		return parsed
	}
	return number
}

// NormalizeNumbers normalizes a whole decoded document, for callers that decode
// JSON outside this package; types.Normalize is the single-value counterpart.
func NormalizeNumbers(value any) any {
	return normalizeNumbers(value)
}
