package source

import (
	"encoding/json"
	"strconv"

	"github.com/elizabevil/envsub/internal/codec"
)

// normalizeValues converts the numeric types produced by YAML and JSON
// decoders into the canonical Go types the renderer expects: int for whole
// numbers and float64 for the rest. Arrays and objects are normalized
// recursively so the value tree stays deterministic regardless of source.
func normalizeValues(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			typed[key] = normalizeValues(child)
		}
		return typed
	case []any:
		for i, child := range typed {
			typed[i] = normalizeValues(child)
		}
		return typed
	case int64:
		return int(typed)
	case int32:
		return int(typed)
	case int:
		return typed
	case float32:
		return float64(typed)
	case json.Number:
		return normalizeNumber(typed)
	default:
		return value
	}
}

func normalizeNumber(number json.Number) any {
	text := number.String()
	if codec.IsIntegerText(text) {
		if parsed, err := strconv.Atoi(text); err == nil {
			return parsed
		}
	}
	if parsed, err := number.Float64(); err == nil {
		return parsed
	}
	return number
}
