package types

import "fmt"

// TypeResolver is the type query surface a renderer needs from a schema, so the
// template evaluator does not depend on the schema loader or its JSON Schema
// backend. A caller without a schema passes nil and the inline assertions stand
// alone.
type TypeResolver interface {
	// DeclaredType returns the single type declared at path, empty when the path
	// has none or declares a union.
	DeclaredType(path string) string
	// DeclaredTypes returns every type declared at path, in written order.
	DeclaredTypes(path string) []string
	// NormalizeString converts text with the types declared at path, reporting
	// whether the value changed. changed is false when the path declares nothing
	// or the text is already a legal member.
	NormalizeString(path, text string) (value any, changed bool, err error)
}

// TypeResolverFuncs adapts functions to TypeResolver. A nil func answers with the
// zero value.
type TypeResolverFuncs struct {
	DeclaredTypeFunc    func(path string) string
	DeclaredTypesFunc   func(path string) []string
	NormalizeStringFunc func(path, text string) (any, bool, error)
}

// DeclaredType implements TypeResolver.
func (f TypeResolverFuncs) DeclaredType(path string) string {
	if f.DeclaredTypeFunc == nil {
		return ""
	}
	return f.DeclaredTypeFunc(path)
}

// DeclaredTypes implements TypeResolver.
func (f TypeResolverFuncs) DeclaredTypes(path string) []string {
	if f.DeclaredTypesFunc == nil {
		return nil
	}
	return f.DeclaredTypesFunc(path)
}

// NormalizeString implements TypeResolver.
func (f TypeResolverFuncs) NormalizeString(path, text string) (any, bool, error) {
	if f.NormalizeStringFunc == nil {
		return nil, false, nil
	}
	return f.NormalizeStringFunc(path, text)
}

// JSON Schema type names, the vocabulary a TypeResolver reports. The engine
// compares against this list instead of importing the schema package.
const (
	SchemaTypeNull    = "null"
	SchemaTypeBoolean = "boolean"
	SchemaTypeInteger = "integer"
	SchemaTypeNumber  = "number"
	SchemaTypeString  = "string"
	SchemaTypeArray   = "array"
	SchemaTypeObject  = "object"
)

// SchemaTypeToDeclared maps a JSON Schema type name to the type assertion it
// implies.
func SchemaTypeToDeclared(schemaType string) (Declared, error) {
	switch schemaType {
	case SchemaTypeInteger:
		return Declared{Name: NameInt}, nil
	case SchemaTypeNumber:
		return Declared{Name: NameFloat}, nil
	case SchemaTypeBoolean:
		return Declared{Name: NameBool}, nil
	case SchemaTypeString:
		return Declared{Name: NameString}, nil
	case SchemaTypeArray:
		return Declared{Name: NameArray}, nil
	case SchemaTypeObject:
		return Declared{Name: NameObject}, nil
	case SchemaTypeNull:
		return Declared{Name: NameNull}, nil
	default:
		return Declared{}, fmt.Errorf("unsupported schema type %q", schemaType)
	}
}
