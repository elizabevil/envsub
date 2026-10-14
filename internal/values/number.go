package values

import (
	"encoding/json"
	"strconv"

	"github.com/elizabevil/envsub/internal/codec"
)

// normalizeNumbers converts the numbers a decoder produced into the value tree's
// canonical Go number: int for a whole number that fits, uint64 above that,
// codec.ExactInt for exact decimal text, codec.LiteralNumber for a spelling that
// must survive (1.50), and float64 for the rest. Nothing widens to float64, which
// would silently round.
//
// It is the only number normalization in the project: a loader that decodes YAML
// or JSON feeds its document here, so no source owns a variant of this rule.
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
	case int64:
		return normalizeInt64(typed)
	case int32:
		return int(typed)
	case float32:
		// Widening is lossless, and float64 is the canonical float.
		return float64(typed)
	default:
		return value
	}
}

// normalizeInt64 narrows a whole number to int, keeping exact text when the value
// does not fit (a 32-bit target, where int is 32 bits wide).
func normalizeInt64(value int64) any {
	if int64(int(value)) == value {
		return int(value)
	}
	if exact, ok := codec.New(strconv.FormatInt(value, 10)); ok {
		return exact
	}
	return value
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
