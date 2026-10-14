package schema_test

import (
	"context"
	"testing"

	"github.com/elizabevil/envsub/internal/schema"
	"github.com/elizabevil/envsub/internal/values"
)

func loadG12(t *testing.T, raw map[string]any) *schema.Schema {
	t.Helper()
	sch, err := schema.Load(context.Background(), raw, "schema.yaml", schema.UnknownIgnore)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return sch
}

// TestApplyDefaultsThroughRef pins G12: a default declared in a $defs target is
// applied at the referencing property.
func TestApplyDefaultsThroughRef(t *testing.T) {
	t.Parallel()
	sch := loadG12(t, map[string]any{
		"$defs":      map[string]any{"port": map[string]any{"type": "integer", "default": 9090}},
		"type":       "object",
		"properties": map[string]any{"port": map[string]any{"$ref": "#/$defs/port"}},
	})
	tree := values.NewTree()
	sch.ApplyDefaults(tree)
	if got := tree.Root["port"]; got != 9090 {
		t.Fatalf("port = %#v (%T), want 9090 from the $ref default", got, got)
	}
}

// TestApplyDefaultsThroughAllOf pins G12 for allOf branches.
func TestApplyDefaultsThroughAllOf(t *testing.T) {
	t.Parallel()
	sch := loadG12(t, map[string]any{
		"type": "object",
		"allOf": []any{
			map[string]any{"properties": map[string]any{"replicas": map[string]any{"type": "integer", "default": 3}}},
		},
	})
	tree := values.NewTree()
	sch.ApplyDefaults(tree)
	if got := tree.Root["replicas"]; got != 3 {
		t.Fatalf("replicas = %#v (%T), want 3 from the allOf default", got, got)
	}
}

// TestRefChainAndCycle covers a chain of references and a self reference, which
// must terminate.
func TestRefChainAndCycle(t *testing.T) {
	t.Parallel()
	sch := loadG12(t, map[string]any{
		"$defs": map[string]any{
			"a": map[string]any{"$ref": "#/$defs/b"},
			"b": map[string]any{"$ref": "#/$defs/c"},
			"c": map[string]any{"type": "string", "default": "deep"},
			"looper": map[string]any{
				"type":       "object",
				"default":    map[string]any{},
				"properties": map[string]any{"self": map[string]any{"$ref": "#/$defs/looper"}},
			},
		},
		"type": "object",
		"properties": map[string]any{
			"chain": map[string]any{"$ref": "#/$defs/a"},
			"node":  map[string]any{"$ref": "#/$defs/looper"},
		},
	})
	tree := values.NewTree()
	sch.ApplyDefaults(tree)
	if got := tree.Root["chain"]; got != "deep" {
		t.Fatalf("chain = %#v, want \"deep\" through the reference chain", got)
	}
	// The self-referencing definition must terminate: its own default creates the
	// object, while the cycle-skipped inner reference contributes no default.
	node, ok := tree.Root["node"].(map[string]any)
	if !ok {
		t.Fatalf("node = %#v (%T), want the defaulted object from the self-referencing definition",
			tree.Root["node"], tree.Root["node"])
	}
	if _, exists := node["self"]; exists {
		t.Fatal("self was created although the cycle-skipped reference declares no default")
	}
}

// TestMaterializeThroughRef pins that a $ref declared type is materialized.
func TestMaterializeThroughRef(t *testing.T) {
	t.Parallel()
	sch := loadG12(t, map[string]any{
		"$defs":      map[string]any{"port": map[string]any{"type": "integer"}},
		"type":       "object",
		"properties": map[string]any{"port": map[string]any{"$ref": "#/$defs/port"}},
	})
	tree := values.NewTree()
	tree.Root["port"] = "8080"
	if err := sch.Materialize(tree); err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if got := tree.Root["port"]; got != 8080 {
		t.Fatalf("port = %#v (%T), want int 8080 through the $ref type", got, got)
	}
}

// TestExternalRefIsRejected pins the actual behaviour for remote references:
// the schema compiler refuses to load them, so they can never silently
// contribute defaults.
func TestExternalRefIsRejected(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"type":       "object",
		"properties": map[string]any{"port": map[string]any{"$ref": "https://example.test/port.json"}},
	}
	if _, err := schema.Load(context.Background(), raw, "schema.yaml", schema.UnknownIgnore); err == nil {
		t.Fatal("Load() error = nil, want an error for an external $ref")
	}
}

// TestAllOfPrecedenceIsFirstWins pins the documented determinism: with the same
// property in two allOf branches, the earlier branch wins.
func TestAllOfPrecedenceIsFirstWins(t *testing.T) {
	t.Parallel()
	sch := loadG12(t, map[string]any{
		"type": "object",
		"allOf": []any{
			map[string]any{"properties": map[string]any{"x": map[string]any{"type": "integer", "default": 1}}},
			map[string]any{"properties": map[string]any{"x": map[string]any{"type": "integer", "default": 2}}},
		},
	})
	tree := values.NewTree()
	sch.ApplyDefaults(tree)
	if got := tree.Root["x"]; got != 1 {
		t.Fatalf("x = %#v (%T), want 1 from the first allOf branch", got, got)
	}
}

// TestRefPointerPathResolvesNestedDefinition pins that a reference may point at a
// nested location, not only at a whole definition.
func TestRefPointerPathResolvesNestedDefinition(t *testing.T) {
	t.Parallel()
	sch := loadG12(t, map[string]any{
		"$defs": map[string]any{
			"wrapper": map[string]any{
				"type":       "object",
				"properties": map[string]any{"inner": map[string]any{"type": "integer", "default": 7}},
			},
		},
		"type":       "object",
		"properties": map[string]any{"value": map[string]any{"$ref": "#/$defs/wrapper/properties/inner"}},
	})
	tree := values.NewTree()
	sch.ApplyDefaults(tree)
	if got := tree.Root["value"]; got != 7 {
		t.Fatalf("value = %#v (%T), want 7 through the nested pointer path", got, got)
	}
}
