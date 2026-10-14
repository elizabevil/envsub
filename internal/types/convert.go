// Package types converts, normalizes and stringifies rendered values.
package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/elizabevil/envsub/internal/codec"
	"github.com/elizabevil/envsub/internal/jsonx"
)

// Canonical type names accepted by the inline | type operator and returned by
// Declared.
const (
	NameString = "string"
	NameInt    = "int"
	NameFloat  = "float"
	NameBool   = "bool"
	NameArray  = "array"
	NameObject = "object"
	NameNull   = "null"
)

const (
	arrayGenericPrefix = "array<"
	// arrayGenericSuffix closes an array<T> declaration.
	arrayGenericSuffix = ">"
)

// Boolean spellings accepted from string values.
const (
	boolTrueText  = "true"
	boolFalseText = "false"
)

// Declared is a parsed inline type assertion such as int or array<string>.
type Declared struct {
	// Name is the canonical outer type name.
	Name string
	// Element is the canonical element type for array types.
	Element string
}

// String renders the declared type back to its template spelling.
func (d Declared) String() string {
	if d.Name == NameArray && d.Element != "" {
		return arrayGenericPrefix + d.Element + arrayGenericSuffix
	}
	return d.Name
}

// ParseType parses an inline type name: string, int, float, bool, object,
// null, array<T> or array.
//
// array<T> is this project's alias syntax, so no standard-library parser
// applies; the prefix and suffix are checked separately to keep the distinct
// "unclosed array element type" diagnostic.
func ParseType(raw string) (Declared, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return Declared{}, errTypeNameEmpty
	}
	if element, isAlias, aliasErr := ArrayElement(trimmed); isAlias {
		switch {
		case errors.Is(aliasErr, ErrUnclosedArray):
			return Declared{}, fmt.Errorf("unclosed array element type in %q", raw)
		case errors.Is(aliasErr, ErrEmptyArrayElement):
			return Declared{}, fmt.Errorf("invalid array element type in %q: %w", raw, errTypeNameEmpty)
		case errors.Is(aliasErr, ErrNestedArray):
			return Declared{}, fmt.Errorf("nested array types are not supported in %q", raw)
		}
		parsed, err := ParseType(element)
		if err != nil {
			return Declared{}, fmt.Errorf("invalid array element type in %q: %w", raw, err)
		}
		if parsed.Name == NameArray {
			return Declared{}, fmt.Errorf("nested array types are not supported in %q", raw)
		}
		return Declared{Name: NameArray, Element: parsed.Name}, nil
	}
	switch trimmed {
	case NameString, NameInt, NameFloat, NameBool, NameObject, NameNull:
		return Declared{Name: trimmed}, nil
	case NameArray:
		return Declared{Name: NameArray}, nil
	default:
		return Declared{}, fmt.Errorf("unsupported type %q", raw)
	}
}

// Assert converts or checks value against the declared type.
//
// Strings are parsed according to the confirmed rules: int accepts only an
// optional sign followed by digits, float follows strconv.ParseFloat, bool
// accepts only true/false, and array/object accept single-line JSON.
// Structured values are checked for compatibility; int and float stay
// distinct types except that a whole-number float satisfies int.
func Assert(value any, declared Declared) (any, error) {
	if declared.Name == "" {
		return value, nil
	}
	if text, ok := value.(string); ok {
		return parseString(text, declared)
	}
	if err := checkCompatible(value, declared); err != nil {
		return nil, err
	}
	return normalizeNumber(value, declared), nil
}

// parseString converts a string value to the declared type.
func parseString(text string, declared Declared) (any, error) {
	switch declared.Name {
	case NameString:
		return text, nil
	case NameInt:
		if parsed, err := strconv.Atoi(text); err == nil {
			return parsed, nil
		}
		if parsed, err := strconv.ParseUint(text, 10, 64); err == nil {
			return parsed, nil
		}
		if exact, ok := codec.New(text); ok {
			return exact, nil
		}
		return nil, fmt.Errorf("value %q is not a valid int", text)
	case NameFloat:
		// Integer-looking text must convert exactly; fractions may round, which
		// is inherent to float64 (rules F1/F7).
		if integer, err := parseString(text, Declared{Name: NameInt}); err == nil {
			if err := checkFloatExact(integer); err != nil {
				return nil, err
			}
			return normalizeScalar(integer), nil
		}
		parsed, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return nil, fmt.Errorf("value %q is not a valid float", text)
		}
		// NaN/±Inf are rejected on every path (rules F8); ParseFloat would accept
		// "nan", "inf" and "Infinity".
		if math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return nil, fmt.Errorf("value %q is not a finite float", text)
		}
		return parsed, nil
	case NameBool:
		switch text {
		case boolTrueText:
			return true, nil
		case boolFalseText:
			return false, nil
		default:
			return nil, fmt.Errorf("value %q is not a valid bool: want %s or %s", text, boolTrueText, boolFalseText)
		}
	case NameObject:
		parsed, err := jsonx.UnmarshalValue([]byte(text))
		if err != nil {
			return nil, fmt.Errorf("value is not a valid JSON object: %w", err)
		}
		object, ok := parsed.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("value is not a JSON object")
		}
		return object, nil
	case NameArray:
		parsed, err := jsonx.UnmarshalValue([]byte(text))
		if err != nil {
			return nil, fmt.Errorf("value is not valid JSON: %w", err)
		}
		items, ok := parsed.([]any)
		if !ok {
			return nil, fmt.Errorf("value is not a JSON array")
		}
		if declared.Element == "" {
			return items, nil
		}
		for i, item := range items {
			converted, err := Assert(item, Declared{Name: declared.Element})
			if err != nil {
				return nil, fmt.Errorf("array element %d: %w", i, err)
			}
			items[i] = converted
		}
		return items, nil
	case NameNull:
		// A string has no null form; injecting null goes through a JSON/YAML
		// null or a schema default (rules G18/TY6).
		return nil, fmt.Errorf("value %q cannot be converted to null", text)
	default:
		return nil, fmt.Errorf("unsupported type %q", declared.Name)
	}
}

// checkCompatible verifies that an already structured value satisfies the
// declared type.
func checkCompatible(value any, declared Declared) error {
	switch declared.Name {
	case NameString:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string, got %s", Describe(value))
		}
	case NameInt:
		if !isIntegral(value) {
			return fmt.Errorf("expected int, got %s", Describe(value))
		}
	case NameFloat:
		if !isNumeric(value) {
			return fmt.Errorf("expected float, got %s", Describe(value))
		}
		// F7/F8: the declared float type is float64 only. A value outside its
		// range, or NaN/±Inf, cannot satisfy it.
		switch typed := value.(type) {
		case float64:
			if math.IsNaN(typed) || math.IsInf(typed, 0) {
				return fmt.Errorf("expected a finite float, got %v", typed)
			}
		case float32:
			if number := float64(typed); math.IsNaN(number) || math.IsInf(number, 0) {
				return fmt.Errorf("expected a finite float, got %v", typed)
			}
		case json.Number:
			parsed, err := typed.Float64()
			if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
				return fmt.Errorf("value %s cannot be represented as a finite float64", typed.String())
			}
		case codec.LiteralNumber:
			// A fractional literal may round (rules F1/F7); only the range and
			// finiteness are checked here, unlike the integer-exactness check
			// below (which does not apply to LiteralNumber).
			parsed, err := strconv.ParseFloat(typed.String(), 64)
			if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
				return fmt.Errorf("value %s cannot be represented as a finite float64", typed.String())
			}
		}
		// An integer that float64 cannot hold exactly is not a valid float
		// (rules F7/G2): the three sources (text, uint64, exact integer) agree.
		if err := checkFloatExact(value); err != nil {
			return err
		}
	case NameBool:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected bool, got %s", Describe(value))
		}
	case NameObject:
		if _, ok := value.(map[string]any); !ok {
			return fmt.Errorf("expected object, got %s", Describe(value))
		}
	case NameArray:
		items, ok := value.([]any)
		if !ok {
			return fmt.Errorf("expected array, got %s", Describe(value))
		}
		if declared.Element == "" {
			return nil
		}
		for i, item := range items {
			if err := checkCompatible(item, Declared{Name: declared.Element}); err != nil {
				return fmt.Errorf("array element %d: %w", i, err)
			}
		}
	case NameNull:
		if value != nil {
			return fmt.Errorf("expected null, got %s", Describe(value))
		}
	}
	return nil
}

// normalizeNumber converts numeric representations into the canonical Go type
// for the declared type so later formatting is stable.
func normalizeNumber(value any, declared Declared) any {
	switch declared.Name {
	case NameInt:
		switch typed := value.(type) {
		case float64:
			return int(typed)
		case uint64:
			if typed <= math.MaxInt64 {
				return int(typed)
			}
		case codec.ExactInt:
			return typed
		case json.Number:
			if parsed, err := typed.Int64(); err == nil {
				return int(parsed)
			}
			if exact, ok := codec.New(typed.String()); ok {
				return exact
			}
		}
	case NameFloat:
		switch typed := value.(type) {
		case int:
			return float64(typed)
		case int64:
			return float64(typed)
		case uint64:
			return float64(typed)
		case json.Number:
			if parsed, err := typed.Float64(); err == nil {
				return parsed
			}
		case codec.LiteralNumber:
			if parsed, err := strconv.ParseFloat(typed.String(), 64); err == nil {
				return parsed
			}
		}
	}
	return normalizeScalar(value)
}

// normalizeScalar converts json.Number and YAML integer types into the plain
// Go types used by Stringify.
func normalizeScalar(value any) any {
	switch typed := value.(type) {
	case json.Number:
		if codec.IsIntegerText(typed.String()) {
			if parsed, err := typed.Int64(); err == nil {
				return int(parsed)
			}
			if parsed, err := strconv.ParseUint(typed.String(), 10, 64); err == nil {
				return parsed
			}
			if exact, ok := codec.New(typed.String()); ok {
				return exact
			}
			return typed
		}
		if literal, ok := codec.NewLiteral(typed.String()); ok {
			return literal
		}
		if parsed, err := typed.Float64(); err == nil {
			return parsed
		}
	case int64:
		return int(typed)
	case int32:
		return int(typed)
	case float32:
		return float64(typed)
	case codec.ExactInt:
		return typed
	case codec.LiteralNumber:
		return typed
	}
	return value
}

// checkFloatExact rejects an integer that float64 cannot hold exactly. Integer
// literals and the integer value family must round-trip exactly; fractional
// literals may round, which is inherent to float64 (rules F1/F7, gap G2).
func checkFloatExact(value any) error {
	if exact, ok := value.(codec.ExactInt); ok {
		return fmt.Errorf("value %s cannot be represented exactly as a float64", exact.String())
	}
	var text string
	switch typed := value.(type) {
	case json.Number:
		if !codec.IsIntegerText(typed.String()) {
			return nil
		}
		text = typed.String()
	case int:
		text = strconv.FormatInt(int64(typed), 10)
	case int8:
		text = strconv.FormatInt(int64(typed), 10)
	case int16:
		text = strconv.FormatInt(int64(typed), 10)
	case int32:
		text = strconv.FormatInt(int64(typed), 10)
	case int64:
		text = strconv.FormatInt(typed, 10)
	case uint:
		text = strconv.FormatUint(uint64(typed), 10)
	case uint8:
		text = strconv.FormatUint(uint64(typed), 10)
	case uint16:
		text = strconv.FormatUint(uint64(typed), 10)
	case uint32:
		text = strconv.FormatUint(uint64(typed), 10)
	case uint64:
		text = strconv.FormatUint(typed, 10)
	default:
		return nil
	}
	trimmed := strings.TrimPrefix(text, "+")
	parsed, err := strconv.ParseFloat(trimmed, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return fmt.Errorf("value %s is outside the float64 range", trimmed)
	}
	if strconv.FormatFloat(parsed, 'f', -1, 64) != trimmed {
		return fmt.Errorf("value %s cannot be represented exactly as a float64", trimmed)
	}
	return nil
}

// Normalize converts one decoded value into the Go number types the formatters
// expect.
//
// It runs at render time on a single value; values.NormalizeNumbers walks a whole
// decoded document at load time. A value that already went through the loader is
// unchanged here.
func Normalize(value any) any {
	return normalizeScalar(value)
}

func isNumeric(value any) bool {
	switch value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, json.Number, codec.ExactInt, codec.LiteralNumber:
		return true
	default:
		return false
	}
}

func isIntegral(value any) bool {
	switch typed := value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, codec.ExactInt:
		return true
	case codec.LiteralNumber:
		// A fractional literal never satisfies int (rules TY2/F6).
		return false
	case float32:
		return float64(typed) == float64(int64(typed))
	case float64:
		return typed == float64(int64(typed))
	case json.Number:
		if codec.IsIntegerText(typed.String()) {
			return true
		}
		if parsed, err := typed.Float64(); err == nil {
			return parsed == float64(int64(parsed))
		}
		return false
	default:
		return false
	}
}

// Type-name errors shared by the inline parser and the JSON Schema normalizer.
var (
	// errTypeNameEmpty reports an empty type declaration.
	errTypeNameEmpty = errors.New("type name is empty")
	// ErrUnclosedArray reports "array<" without the closing bracket.
	ErrUnclosedArray = errors.New("unclosed array element type")
	// ErrEmptyArrayElement reports "array<>".
	ErrEmptyArrayElement = errors.New("array element type is empty")
	// ErrNestedArray reports "array<array<...>>"; it is the single source of the
	// nesting diagnostic for both callers (rules SI1/G13).
	ErrNestedArray = errors.New("nested array types are not supported")
)

// ArrayElement parses the shared "array<T>" alias grammar. isAlias reports
// whether raw used the alias form; element is the trimmed inner text, which the
// caller canonicalizes with its own name table. Nested arrays are rejected here
// so schema and inline cannot diverge (rules SI1/G13).
func ArrayElement(raw string) (element string, isAlias bool, err error) {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, arrayGenericPrefix) {
		return "", false, nil
	}
	if !strings.HasSuffix(trimmed, arrayGenericSuffix) {
		return "", true, ErrUnclosedArray
	}
	element = strings.TrimSpace(trimmed[len(arrayGenericPrefix) : len(trimmed)-1])
	if element == "" {
		return "", true, ErrEmptyArrayElement
	}
	if strings.HasPrefix(element, arrayGenericPrefix) {
		return "", true, ErrNestedArray
	}
	return element, true, nil
}
