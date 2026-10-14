package schema_test

import (
	"context"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/schema"
	"github.com/elizabevil/envsub/internal/values"
)

// TestMaterializeUnionNeverRejectsLegalString pins SI11: a union must not reject
// a value the schema allows, and the choice of member must not depend on where
// "string" appears.
func TestMaterializeUnionNeverRejectsLegalString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		schemaType any
		raw        string
		want       any
		wantErr    bool
	}{
		{name: "integer or string keeps hello", schemaType: []any{"integer", "string"}, raw: "hello", want: "hello"},
		{name: "string or integer keeps hello", schemaType: []any{"string", "integer"}, raw: "hello", want: "hello"},
		{name: "integer or string converts digits", schemaType: []any{"integer", "string"}, raw: "8080", want: 8080},
		{name: "string first still converts digits", schemaType: []any{"string", "integer"}, raw: "8080", want: 8080},
		{name: "nullable integer converts digits", schemaType: []any{"integer", "null"}, raw: "8080", want: 8080},
		{name: "integer only rejects hello", schemaType: "integer", raw: "hello", wantErr: true},
		{name: "integer or boolean rejects hello", schemaType: []any{"integer", "boolean"}, raw: "hello", wantErr: true},
		{name: "string only keeps hello", schemaType: "string", raw: "hello", want: "hello"},
		{name: "null only keeps string", schemaType: "null", raw: "hello", want: "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			raw := map[string]any{
				"type":       "object",
				"properties": map[string]any{"x": map[string]any{"type": tt.schemaType}},
			}
			sch, err := schema.Load(context.Background(), raw, "schema.yaml", schema.UnknownIgnore)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			tree := values.NewTree()
			tree.Root["x"] = tt.raw
			err = sch.Materialize(tree)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Materialize() error = nil, want an error")
				}
				if kind := diag.KindOf(err); kind != diag.KindType {
					t.Fatalf("Materialize() kind = %q, want %q (err = %v)", kind, diag.KindType, err)
				}
				if !strings.Contains(err.Error(), "x at") {
					t.Fatalf("Materialize() error = %q, want it to name the failing path x", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Materialize() error = %v", err)
			}
			if got, _, _ := tree.Resolve("x"); got != tt.want {
				t.Fatalf("x = %#v (%T), want %#v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

// TestMaterializeUnionArrayMembers covers structured members in a union.
func TestMaterializeUnionArrayMembers(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"x": map[string]any{"type": []any{"array", "string"}},
		},
	}
	sch, err := schema.Load(context.Background(), raw, "schema.yaml", schema.UnknownIgnore)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	tree := values.NewTree()
	tree.Root["x"] = `["a","b"]`
	if err := sch.Materialize(tree); err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	items, _, _ := tree.Resolve("x")
	list, ok := items.([]any)
	if !ok || len(list) != 2 {
		t.Fatalf("x = %#v (%T), want a two-element array", items, items)
	}
}

// TestMaterializeConvertsElementsOfParsedArray pins that a collection parsed
// from a string value is walked like one written inline: its string elements are
// converted too, so validation sees integers where the schema declares integers.
func TestMaterializeConvertsElementsOfParsedArray(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"PORTS": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
		},
	}
	sch, err := schema.Load(context.Background(), raw, "schema.yaml", schema.UnknownIgnore)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	tree := values.NewTree()
	tree.Root["PORTS"] = `["1","2"]`

	if err := sch.Materialize(tree); err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	ports, ok := tree.Root["PORTS"].([]any)
	if !ok || len(ports) != 2 {
		t.Fatalf("PORTS = %#v (%T), want a two-element array", tree.Root["PORTS"], tree.Root["PORTS"])
	}
	if ports[0] != 1 || ports[1] != 2 {
		t.Fatalf("PORTS = %#v, want int 1 and 2 (elements of a parsed array must be converted)", ports)
	}
	if err := sch.Validate(tree); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
