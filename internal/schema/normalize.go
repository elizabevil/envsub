package schema

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/elizabevil/envsub/internal/codec"
	"github.com/elizabevil/envsub/internal/jsonx"
	"github.com/elizabevil/envsub/internal/types"
	"github.com/elizabevil/envsub/internal/values"
)

// typeAliases maps every accepted type spelling to its JSON Schema standard
// type name. Go-style aliases and the standard names themselves are both
// accepted so schemas can be reused across systems unchanged.
var typeAliases = map[string]string{
	"int":       TypeInteger,
	TypeInteger: TypeInteger,
	"float":     TypeNumber,
	TypeNumber:  TypeNumber,
	"bool":      TypeBoolean,
	TypeBoolean: TypeBoolean,
	TypeString:  TypeString,
	TypeObject:  TypeObject,
	TypeArray:   TypeArray,
	TypeNull:    TypeNull,
}

// nameValuedKeywords hold maps whose child keys are user-chosen names rather
// than schema keywords, so their children must not be reported as unknown
// keywords and their type fields must still be normalized.
var nameValuedKeywords = map[string]bool{
	keywordProperties:   true,
	"patternProperties": true,
	"$defs":             true,
	"definitions":       true,
	"dependentSchemas":  true,
	"dependentRequired": true,
}

// knownKeywords is the set of JSON Schema keywords the adapter understands.
// Keywords outside this set that do not start with x- are reported through the
// unknown keyword policy.
var knownKeywords = map[string]bool{
	// core / structural
	"$schema": true, "$id": true, "$ref": true, "$defs": true, "$anchor": true,
	"$dynamicRef": true, "$dynamicAnchor": true, "$vocabulary": true, "$comment": true,
	"definitions": true, "title": true, "description": true, "examples": true,
	"default": true, "deprecated": true, "readOnly": true, "writeOnly": true,
	// validation
	"type": true, "enum": true, "const": true, "required": true,
	"properties": true, "patternProperties": true, "additionalProperties": true,
	"propertyNames": true, "items": true, "prefixItems": true, "contains": true,
	"minContains": true, "maxContains": true, "unevaluatedItems": true,
	"unevaluatedProperties": true, "dependentRequired": true, "dependentSchemas": true,
	"minItems": true, "maxItems": true, "uniqueItems": true,
	"minLength": true, "maxLength": true, "pattern": true,
	"minimum": true, "maximum": true, "exclusiveMinimum": true, "exclusiveMaximum": true,
	"multipleOf": true, "minProperties": true, "maxProperties": true,
	"allOf": true, "anyOf": true, "oneOf": true, "not": true,
	"if": true, "then": true, "else": true,
	// annotation / format
	"format": true, "contentEncoding": true, "contentMediaType": true, "contentSchema": true,
}

// normalizeSchema rewrites Go-style type aliases into JSON Schema standard
// types. It returns the sorted list of unknown non x-* keywords and fails when
// a type name cannot be normalized.
func normalizeSchema(node any, path values.Key) (unknown []string, err error) {
	switch typed := node.(type) {
	case map[string]any:
		if err := normalizeTypeField(typed, path); err != nil {
			return nil, err
		}
		for _, key := range sortedKeys(typed) {
			childPath := values.JoinPath(path, key)
			if nameValuedKeywords[key] {
				// Descend into the name map without judging its keys.
				childUnknown, err := normalizeNameMap(typed[key], childPath)
				if err != nil {
					return nil, err
				}
				unknown = append(unknown, childUnknown...)
				continue
			}
			if !knownKeywords[key] && !strings.HasPrefix(key, extensionPrefix) {
				// The list is diagnostic text, so the address is flattened here.
				unknown = append(unknown, string(childPath))
			}
			childUnknown, err := normalizeSchema(typed[key], childPath)
			if err != nil {
				return nil, err
			}
			unknown = append(unknown, childUnknown...)
		}
	case []any:
		for i, child := range typed {
			childUnknown, err := normalizeSchema(child, values.JoinIndex(path, i))
			if err != nil {
				return nil, err
			}
			unknown = append(unknown, childUnknown...)
		}
	}
	slices.Sort(unknown)
	return unknown, nil
}

// normalizeNameMap normalizes the schemas stored under a name-valued keyword
// such as properties, where each child key is a user-chosen name.
func normalizeNameMap(node any, path values.Key) ([]string, error) {
	object, ok := node.(map[string]any)
	if !ok {
		return nil, nil
	}
	var unknown []string
	for _, key := range sortedKeys(object) {
		childUnknown, err := normalizeSchema(object[key], values.JoinPath(path, key))
		if err != nil {
			return nil, err
		}
		unknown = append(unknown, childUnknown...)
	}
	return unknown, nil
}

// normalizeTypeField rewrites the type keyword of one schema object.
func normalizeTypeField(object map[string]any, path values.Key) error {
	raw, ok := object[keywordType]
	if !ok {
		return nil
	}
	switch typed := raw.(type) {
	case string:
		normalized, items, err := normalizeTypeName(typed)
		if err != nil {
			return fmt.Errorf("%s: %w", values.JoinPath(path, keywordType), err)
		}
		object[keywordType] = normalized
		if items != nil {
			if _, exists := object[keywordItems]; !exists {
				object[keywordItems] = items
			}
		}
	case []any:
		normalized := make([]any, 0, len(typed))
		for _, entry := range typed {
			name, ok := entry.(string)
			if !ok {
				normalized = append(normalized, entry)
				continue
			}
			canonical, _, err := normalizeTypeName(name)
			if err != nil {
				return fmt.Errorf("%s: %w", values.JoinPath(path, keywordType), err)
			}
			normalized = append(normalized, canonical)
		}
		object[keywordType] = normalized
	}
	return nil
}

// normalizeTypeName converts one type name. For array<T> it also returns the
// item schema implied by the generic argument.
//
// array<T> is a project alias layered on JSON Schema, so no standard-library
// parser applies. The prefix and suffix are checked separately so an unclosed
// array<...> falls through to the unsupported-type error instead of being
// silently rewritten.
func normalizeTypeName(raw string) (name string, items map[string]any, err error) {
	trimmed := strings.TrimSpace(raw)
	if element, isAlias, aliasErr := types.ArrayElement(trimmed); isAlias {
		switch {
		case errors.Is(aliasErr, types.ErrNestedArray):
			return "", nil, fmt.Errorf("nested array types are not supported in %q", raw)
		case errors.Is(aliasErr, types.ErrEmptyArrayElement):
			return "", nil, fmt.Errorf("array element type is missing in %q", raw)
		case aliasErr != nil:
			return "", nil, fmt.Errorf("unsupported type %q", raw)
		}
		elementName, _, err := normalizeTypeName(element)
		if err != nil {
			return "", nil, err
		}
		if elementName == TypeArray {
			return "", nil, fmt.Errorf("nested array types are not supported in %q", raw)
		}
		return TypeArray, map[string]any{keywordType: elementName}, nil
	}
	if canonical, ok := typeAliases[trimmed]; ok {
		return canonical, nil, nil
	}
	return "", nil, fmt.Errorf("unsupported type %q", raw)
}

// convertScalar converts a raw string into the scalar type the schema
// declares. int accepts an optional sign and digits only, float follows
// strconv.ParseFloat, and bool accepts only true/false.
func convertScalar(raw, declared string) (any, error) {
	switch declared {
	case TypeInteger:
		if parsed, err := strconv.Atoi(raw); err == nil {
			return parsed, nil
		}
		if parsed, err := strconv.ParseUint(raw, 10, 64); err == nil {
			return parsed, nil
		}
		if exact, ok := codec.New(raw); ok {
			return exact, nil
		}
		return nil, fmt.Errorf("expected an integer, got %q", raw)
	case TypeNumber:
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("expected a number, got %q", raw)
		}
		if math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return nil, fmt.Errorf("expected a finite number, got %q", raw)
		}
		return parsed, nil
	case TypeBoolean:
		switch raw {
		case boolTrueText:
			return true, nil
		case boolFalseText:
			return false, nil
		default:
			return nil, fmt.Errorf("expected a boolean, got %q", raw)
		}
	default:
		return raw, nil
	}
}

// parseStructured parses a JSON string into the requested structured type.
func parseStructured(raw, declared string) (any, error) {
	switch declared {
	case TypeArray:
		parsed, err := jsonx.UnmarshalValue([]byte(raw))
		if err != nil {
			return nil, fmt.Errorf("expected a JSON array: %w", err)
		}
		items, ok := parsed.([]any)
		if !ok {
			return nil, fmt.Errorf("expected a JSON array, got %T", parsed)
		}
		return values.NormalizeNumbers(items), nil
	case TypeObject:
		parsed, err := jsonx.UnmarshalValue([]byte(raw))
		if err != nil {
			return nil, fmt.Errorf("expected a JSON object: %w", err)
		}
		object, ok := parsed.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected a JSON object, got %T", parsed)
		}
		return values.NormalizeNumbers(object), nil
	default:
		return raw, nil
	}
}
