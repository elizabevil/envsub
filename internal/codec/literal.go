package codec

import (
	"encoding/json"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
)

// LiteralNumber is an undeclared numeric literal kept as its original JSON text.
// It exists so a value that no schema type and no `| type` assertion declares
// keeps its spelling through every channel: `1.0` stays "1.0", `1.50` stays
// "1.50", `1e3` stays "1e3" (rules F3/F6/Q6/FB2).
//
// Only the JSON number grammar is accepted. YAML-only spellings such as `.5`,
// `5.` or `1_000` are not literal numbers: they fall back to float64.
type LiteralNumber string

// String returns the original numeric text.
func (n LiteralNumber) String() string { return string(n) }

// Valid reports whether the text is a JSON number literal: an optional minus
// sign, an integer part without leading zeros, an optional fraction with at
// least one digit, and an optional exponent.
func (n LiteralNumber) Valid() bool {
	return validJSONNumber(string(n))
}

// MarshalJSON writes the original text as a JSON number. The text is re-checked
// so an invalid literal can never produce invalid JSON.
func (n LiteralNumber) MarshalJSON() ([]byte, error) {
	if !n.Valid() {
		return nil, fmt.Errorf(errorPrefix+"invalid numeric literal %q", string(n))
	}
	return []byte(n), nil
}

// NewLiteral returns text as a LiteralNumber when it is a valid JSON number and
// not a plain integer. Integer spellings keep their narrower types (int, uint64
// or ExactInt), so callers must try New first.
func NewLiteral(text string) (LiteralNumber, bool) {
	value := LiteralNumber(text)
	if !value.Valid() {
		return "", false
	}
	if _, isInteger := New(text); isInteger {
		return "", false
	}
	return value, true
}

// validJSONNumber reports whether text is exactly one JSON number literal.
//
// The grammar is checked with jsontext, the standard library's JSON tokenizer
// and the layer encoding/json/v2 is built on, instead of a hand-rolled scanner
// or the v1 compatibility API (project rule: prefer stdlib APIs). The spelling
// must survive untouched, so surrounding whitespace is rejected up front.
func validJSONNumber(text string) bool {
	if text == "" || strings.TrimSpace(text) != text {
		return false
	}
	decoder := jsontext.NewDecoder(strings.NewReader(text))
	if decoder.PeekKind() != jsontext.KindNumber {
		return false
	}
	if _, err := decoder.ReadValue(); err != nil {
		return false
	}
	_, err := decoder.ReadToken()
	return errors.Is(err, io.EOF)
}

// containsJSONNumberType reports whether the tree holds a value that the
// jsonschema validator must see as a json.Number rather than as a string.
func containsJSONNumberType(value any) bool {
	switch typed := value.(type) {
	case ExactInt, LiteralNumber:
		return true
	case map[string]any:
		for _, child := range typed {
			if containsJSONNumberType(child) {
				return true
			}
		}
	case []any:
		if slices.ContainsFunc(typed, containsJSONNumberType) {
			return true
		}
	}
	return false
}

// toJSONNumber converts the text-backed numeric types into json.Number.
func toJSONNumber(value any) any {
	switch typed := value.(type) {
	case ExactInt:
		return json.Number(string(typed))
	case LiteralNumber:
		return json.Number(string(typed))
	case map[string]any:
		converted := make(map[string]any, len(typed))
		for key, child := range typed {
			converted[key] = toJSONNumber(child)
		}
		return converted
	case []any:
		converted := make([]any, len(typed))
		for i, child := range typed {
			converted[i] = toJSONNumber(child)
		}
		return converted
	default:
		return value
	}
}
