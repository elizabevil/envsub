// Package codec holds the value types the serialization codecs share.
//
// The rules live in docs/03-design/03-12-serialization-rules.md: an integer that
// does not fit uint64 is kept as exact decimal text so that both JSON and YAML
// can write it as a bare number instead of widening it to float64 or quoting it
// as a string.
package codec

import (
	"fmt"
	"math"
)

// errorPrefix names this package in error messages.
const errorPrefix = "codec: "

// ExactInt is an integer outside the uint64 range, stored as its exact decimal
// text. It marshals to a bare JSON number; the YAML encoder handles it through
// its own branch so this package does not depend on a YAML library.
type ExactInt string

// String returns the exact decimal text.
func (i ExactInt) String() string { return string(i) }

// Valid reports whether the text is a JSON integer literal: an optional minus
// sign followed by digits, without leading zeros.
func (i ExactInt) Valid() bool {
	return IsIntegerText(string(i)) && validJSONNumber(string(i))
}

// MarshalJSON writes the digits as a JSON number.
func (i ExactInt) MarshalJSON() ([]byte, error) {
	if !i.Valid() {
		return nil, fmt.Errorf(errorPrefix+"invalid exact integer %q", string(i))
	}
	return []byte(i), nil
}

// New returns text as an ExactInt when it is a valid integer literal.
func New(text string) (ExactInt, bool) {
	value := ExactInt(text)
	return value, value.Valid()
}

// RejectNonFinite returns an error when the tree holds NaN or ±Inf. JSON has no
// representation for them, and YAML would silently write ".nan"/".inf", so both
// codecs refuse to encode such a tree and YAML decoding refuses to create one
// (rules F8).
func RejectNonFinite(value any) error {
	switch typed := value.(type) {
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return nonFiniteError(typed)
		}
	case float32:
		if number := float64(typed); math.IsNaN(number) || math.IsInf(number, 0) {
			return nonFiniteError(typed)
		}
	case map[string]any:
		for _, child := range typed {
			if err := RejectNonFinite(child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range typed {
			if err := RejectNonFinite(child); err != nil {
				return err
			}
		}
	}
	return nil
}

// nonFiniteError reports a float the codecs refuse to encode.
func nonFiniteError(number any) error {
	return fmt.Errorf(errorPrefix+"non-finite number %v is not allowed", number)
}

// ToJSON returns the tree with every ExactInt and LiteralNumber replaced by
// json.Number, which is the numeric representation jsonschema/v6 understands.
// The original value is returned unchanged when there is nothing to convert, so
// the common case does not copy the tree.
func ToJSON(value any) any {
	if !containsJSONNumberType(value) {
		return value
	}
	return toJSONNumber(value)
}
