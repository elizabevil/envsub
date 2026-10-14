package extended

import (
	"context"
	"testing"

	"github.com/elizabevil/envsub/internal/schema"
	"github.com/elizabevil/envsub/internal/types"
)

func unionSchema(t *testing.T, schemaType any) *schema.Schema {
	t.Helper()
	raw := map[string]any{
		"type":       "object",
		"properties": map[string]any{"x": map[string]any{"type": schemaType}},
	}
	sch, err := schema.Load(context.Background(), raw, "schema.yaml", schema.UnknownIgnore)
	if err != nil {
		t.Fatalf("schema.New() error = %v", err)
	}
	return sch
}

// TestSchemaCompatibleMembersHonoursUnion pins the SI11 probe: an inline string
// assertion must be compatible with a schema that allows ["integer","string"].
func TestSchemaCompatibleMembersHonoursUnion(t *testing.T) {
	sch := unionSchema(t, []any{"integer", "string"})
	got, err := schemaCompatibleMembers(types.Declared{Name: types.NameString}, sch, "x", "integer")
	if err != nil {
		t.Fatalf("schemaCompatibleMembers() error = %v", err)
	}
	if !got {
		t.Fatal("inline string vs schema [integer,string] = incompatible, want compatible")
	}
	if got, _ := schemaCompatibleMembers(types.Declared{Name: types.NameBool}, sch, "x", "integer"); got {
		t.Fatal("inline bool vs schema [integer,string] = compatible, want incompatible")
	}
}

// TestNormalizeBySchemaMembersKeepsLegalString pins that the render path keeps a
// string the union allows instead of failing on the first member.
func TestNormalizeBySchemaMembersKeepsLegalString(t *testing.T) {
	sch := unionSchema(t, []any{"integer", "string"})
	got, err := normalizeBySchemaMembers("x", "hello", sch, "integer")
	if err != nil {
		t.Fatalf("normalizeBySchemaMembers() error = %v", err)
	}
	if got != "hello" {
		t.Fatalf("value = %#v, want the original string", got)
	}
	converted, err := normalizeBySchemaMembers("x", "8080", sch, "integer")
	if err != nil || converted != 8080 {
		t.Fatalf("value = %#v err = %v, want int 8080", converted, err)
	}

	strict := unionSchema(t, []any{"integer", "boolean"})
	if _, err := normalizeBySchemaMembers("x", "hello", strict, "integer"); err == nil {
		t.Fatal("normalizeBySchemaMembers([integer,boolean], hello) error = nil, want an error")
	}
}

// TestNormalizeBySchemaMembersMatchesSchema pins that the renderer's helper and
// the schema's shared implementation agree for every union shape (rules SI11):
// the algorithm exists once, in schema.
func TestNormalizeBySchemaMembersMatchesSchema(t *testing.T) {
	t.Parallel()
	cases := []struct {
		schemaType any
		value      string
	}{
		{schemaType: []any{"integer", "string"}, value: "hello"},
		{schemaType: []any{"integer", "string"}, value: "8080"},
		{schemaType: []any{"string", "integer"}, value: "8080"},
		{schemaType: []any{"integer", "null"}, value: "8080"},
		{schemaType: []any{"integer", "boolean"}, value: "hello"},
		{schemaType: "integer", value: "8080"},
		{schemaType: "string", value: "hello"},
	}
	for _, tc := range cases {
		sch := unionSchema(t, tc.schemaType)
		engineValue, engineErr := normalizeBySchemaMembers("x", tc.value, sch, "integer")

		wantValue := any(tc.value)
		if converted, changed, err := sch.NormalizeString("x", tc.value); err != nil {
			wantValue = nil
		} else if changed {
			wantValue = converted
		}

		if (engineErr == nil) != (wantValue != nil) {
			t.Fatalf("schema %v value %q: engine err = %v, schema value = %#v", tc.schemaType, tc.value, engineErr, wantValue)
		}
		if engineErr == nil && engineValue != wantValue {
			t.Fatalf("schema %v value %q: engine = %#v, schema = %#v", tc.schemaType, tc.value, engineValue, wantValue)
		}
	}
}
