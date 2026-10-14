//go:build sonic

package jsonx

import (
	"errors"
	"github.com/elizabevil/envsub/internal/utils"
	"unicode/utf8"

	"github.com/bytedance/sonic"
	"github.com/elizabevil/envsub/internal/codec"
)

// api decodes numbers as json.Number so integers keep their exact spelling and
// sorts object keys so the output matches the standard-library backend instead
// of following Go's random map iteration order.
//
// ValidateString is deliberately NOT set: measured on sonic v1.15.4 it replaces
// invalid UTF-8 with U+FFFD on decode and encodes it as \ufffd, i.e. it corrupts
// data silently instead of reporting it. jsonx checks invalid UTF-8 itself so
// both backends reject it (rules 03-12 Q3a).
var api = sonic.Config{
	UseNumber:   true,
	SortMapKeys: true,
}.Froze()

// Unmarshal decodes data into target.
func Unmarshal(data []byte, target any) error {
	data = utils.TrimBOM(data)
	if err := api.Unmarshal(data, target); err != nil {
		return err
	}
	// sonic cannot reject invalid UTF-8 on decode: Config.ValidateString does not
	// apply to Unmarshal and sonic.Decoder has no ValidateString method. Check the
	// decoded tree instead so both backends behave the same (rules 03-12 Q3a).
	switch typed := target.(type) {
	case *any:
		return rejectInvalidUTF8(*typed)
	case *map[string]any:
		return rejectInvalidUTF8(*typed)
	case *[]any:
		return rejectInvalidUTF8(*typed)
	default:
		return nil
	}
}

// rejectInvalidUTF8 reports invalid UTF-8 inside a decoded tree, mirroring the
// standard-library decoder's error (rules 03-12 Q3a).
func rejectInvalidUTF8(value any) error {
	switch typed := value.(type) {
	case string:
		if !utf8.ValidString(typed) {
			return errors.New(errorPrefix + "invalid UTF-8 in decoded string")
		}
	case map[string]any:
		for key, child := range typed {
			if !utf8.ValidString(key) {
				return errors.New(errorPrefix + "invalid UTF-8 in decoded object name")
			}
			if err := rejectInvalidUTF8(child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range typed {
			if err := rejectInvalidUTF8(child); err != nil {
				return err
			}
		}
	}
	return nil
}

// UnmarshalValue decodes any JSON document into a JSON-compatible tree with
// numbers preserved as json.Number.
func UnmarshalValue(data []byte) (any, error) {
	var value any
	if err := Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return value, nil
}

// Marshal encodes value as compact JSON.
//
// sonic writes null for a typed nil map or slice while encoding/json/v2 writes
// {} and [], so the tree is normalized to the library default first (rules
// 03-12 V1/V5).
func Marshal(value any) ([]byte, error) {
	if err := codec.RejectNonFinite(value); err != nil {
		return nil, err
	}
	if err := rejectInvalidUTF8(value); err != nil {
		return nil, err
	}
	return api.Marshal(emptyCollections(value))
}

// MarshalIndent encodes value as two-space indented JSON.
func MarshalIndent(value any) ([]byte, error) {
	if err := codec.RejectNonFinite(value); err != nil {
		return nil, err
	}
	if err := rejectInvalidUTF8(value); err != nil {
		return nil, err
	}
	return api.MarshalIndent(emptyCollections(value), "", "  ")
}

// emptyCollections replaces a typed nil map or slice with an empty one, because
// sonic would encode it as null (rules 03-12 V1).
func emptyCollections(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		if typed == nil {
			return map[string]any{}
		}
		for key, child := range typed {
			typed[key] = emptyCollections(child)
		}
		return typed
	case []any:
		if typed == nil {
			return []any{}
		}
		for i, child := range typed {
			typed[i] = emptyCollections(child)
		}
		return typed
	default:
		return value
	}
}
