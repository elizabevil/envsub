package types_test

import (
	"testing"

	"github.com/elizabevil/envsub/internal/types"
)

// TestSchemaTypeToDeclared pins the JSON Schema name mapping the renderer uses for
// inline | type compatibility and --on-missing=zero, and that an unknown name is
// an error rather than a silent zero value.
func TestSchemaTypeToDeclared(t *testing.T) {
	t.Parallel()
	tests := []struct {
		schemaType string
		want       string
	}{
		{types.SchemaTypeInteger, types.NameInt},
		{types.SchemaTypeNumber, types.NameFloat},
		{types.SchemaTypeBoolean, types.NameBool},
		{types.SchemaTypeString, types.NameString},
		{types.SchemaTypeArray, types.NameArray},
		{types.SchemaTypeObject, types.NameObject},
		{types.SchemaTypeNull, types.NameNull},
	}
	for _, tt := range tests {
		t.Run(tt.schemaType, func(t *testing.T) {
			t.Parallel()
			got, err := types.SchemaTypeToDeclared(tt.schemaType)
			if err != nil {
				t.Fatalf("SchemaTypeToDeclared(%q) error = %v", tt.schemaType, err)
			}
			if got.Name != tt.want {
				t.Fatalf("SchemaTypeToDeclared(%q).Name = %q, want %q", tt.schemaType, got.Name, tt.want)
			}
		})
	}
	if _, err := types.SchemaTypeToDeclared("currency"); err == nil {
		t.Fatal("SchemaTypeToDeclared(currency) error = nil, want an error")
	}
}

// TestTypeResolverFuncs pins the function adapter: a nil field answers with the
// zero value instead of panicking, which keeps "no schema" a valid configuration.
func TestTypeResolverFuncs(t *testing.T) {
	t.Parallel()
	var empty types.TypeResolverFuncs
	if got := empty.DeclaredType("x"); got != "" {
		t.Fatalf("DeclaredType() = %q, want empty", got)
	}
	if got := empty.DeclaredTypes("x"); got != nil {
		t.Fatalf("DeclaredTypes() = %#v, want nil", got)
	}
	if got, changed, err := empty.NormalizeString("x", "1"); got != nil || changed || err != nil {
		t.Fatalf("NormalizeString() = (%#v, %v, %v), want (nil, false, nil)", got, changed, err)
	}
}
