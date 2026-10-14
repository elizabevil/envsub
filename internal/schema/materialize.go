package schema

import (
	"fmt"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/values"
)

// Materialize converts string values the schema declares as object or array
// into parsed JSON. Values already holding structured types are left alone.
//
// It implements the six confirmed .env JSON array rules: only keys whose
// schema type is array or object are parsed, parse failures are type errors
// naming the value and its origin, and everything else stays a string.
func (s *Schema) Materialize(tree *values.Tree) error {
	if s == nil {
		return nil
	}
	return materializeNode(tree, s.effective(), tree.Root, "")
}

// materializeVisitor converts string values declared by the schema.
type materializeVisitor struct{}

func (materializeVisitor) missing(tree *values.Tree, path values.Key, childSchema map[string]any) (any, bool, error) {
	if !isObjectSchema(childSchema) {
		return nil, false, nil
	}
	// A declared object may sit below a missing key. Build the declared object
	// only when that actually finds something, so a schema never invents empty
	// objects.
	stub := map[string]any{}
	if err := walkProperties(tree, childSchema, stub, path, materializeVisitor{}); err != nil {
		return nil, false, err
	}
	if len(stub) == 0 {
		return nil, false, nil
	}
	return stub, true, nil
}

func (materializeVisitor) text(tree *values.Tree, path values.Key, childSchema map[string]any, value string) (any, bool, error) {
	converted, changed, err := convertDeclared(value, childSchema)
	if err != nil {
		return nil, false, diag.Wrap(diag.KindType, materializeError(tree, path, value, err))
	}
	return converted, changed, nil
}

// materializeNode converts string values according to the schema.
func materializeNode(tree *values.Tree, schemaNode any, node map[string]any, prefix values.Key) error {
	return walkProperties(tree, schemaNode, node, prefix, materializeVisitor{})
}

// isStructuredType reports whether a declared type holds parsed JSON.
func isStructuredType(declared string) bool {
	return declared == TypeArray || declared == TypeObject
}

// isScalarType reports whether a declared type is converted from its string
// spelling.
func isScalarType(declared string) bool {
	return declared == TypeInteger || declared == TypeNumber || declared == TypeBoolean
}

// materializeError attaches the origin of the offending value. The value itself
// is deliberately left out: it may come from a .env file or --set-file and be a
// secret, and diagnostics are the one place a value can leak.
func materializeError(tree *values.Tree, path values.Key, _ string, err error) error {
	location := values.DescribeMeta(tree.MetaForKey(path))
	if location == "" {
		location = "unknown source"
	}
	return fmt.Errorf("%s at %s: %w", path, location, err)
}

// NormalizeString converts a string with the types the schema declares at path,
// trying each member in written order and keeping the string when the union
// allows it (rules SI11). It is the single implementation of that algorithm;
// the renderer calls it instead of keeping its own copy.
func (s *Schema) NormalizeString(path, text string) (any, bool, error) {
	node, ok := s.schemaAt(path)
	if !ok {
		return nil, false, nil
	}
	return convertDeclared(text, node)
}

// convertDeclared converts a string value with every type the schema declares,
// in written order, and reports whether the value changed (rules SI11). A union
// must never reject a value the schema allows: when no member accepts the text
// but "string" is allowed, the string is kept and validation decides.
func convertDeclared(text string, schemaNode map[string]any) (any, bool, error) {
	members, allowsString := declaredTypes(schemaNode)
	var firstErr error
	for _, member := range members {
		switch {
		case isStructuredType(member):
			parsed, err := parseStructured(text, member)
			if err == nil {
				return parsed, true, nil
			}
			if firstErr == nil {
				firstErr = err
			}
		case isScalarType(member):
			converted, err := convertScalar(text, member)
			if err == nil {
				return converted, true, nil
			}
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	if firstErr == nil || allowsString {
		// Nothing to convert, or the string itself is a legal member.
		return nil, false, nil
	}
	return nil, false, firstErr
}

// isObjectSchema reports whether a schema node describes an object, either by
// declaring type: object or by carrying properties.
func isObjectSchema(node map[string]any) bool {
	if declaredTypeName(node) == TypeObject {
		return true
	}
	_, ok := node[keywordProperties]
	return ok
}
