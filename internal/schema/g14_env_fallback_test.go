package schema_test

import (
	"context"
	"testing"

	"github.com/elizabevil/envsub/internal/schema"
	"github.com/elizabevil/envsub/internal/values"
)

func g14Schema(t *testing.T) *schema.Schema {
	t.Helper()
	raw := map[string]any{
		"type":     "object",
		"required": []any{"app"},
		"properties": map[string]any{
			"app": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"hosts": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"port":  map[string]any{"type": "integer", "minimum": 0},
				},
			},
		},
	}
	sch, err := schema.Load(context.Background(), raw, "schema.yaml", schema.UnknownIgnore)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return sch
}

// TestMaterializeConvertsRootStrings pins that materialization only converts the
// values the root actually holds. A .env file writes root keys, so the strings it
// contributes are typed here exactly like a value file's strings.
func TestMaterializeConvertsRootStrings(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"PORT":  map[string]any{"type": "integer", "minimum": 0},
			"HOSTS": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
	}
	sch, err := schema.Load(context.Background(), raw, "schema.yaml", schema.UnknownIgnore)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	tree := values.NewTree()
	tree.Root["PORT"] = "8080"
	tree.Root["HOSTS"] = `["x","y"]`

	if err := sch.Materialize(tree); err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if tree.Root["PORT"] != 8080 {
		t.Fatalf("PORT = %#v (%T), want int 8080", tree.Root["PORT"], tree.Root["PORT"])
	}
	if list, ok := tree.Root["HOSTS"].([]any); !ok || len(list) != 2 {
		t.Fatalf("HOSTS = %#v, want a two-element array", tree.Root["HOSTS"])
	}
	if err := sch.Validate(tree); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

// TestProcessNamespaceIsNotAMaterializationSource pins that the @process namespace
// is read-only and explicit, so it never stands in for a schema path: a key the
// root does not define is missing, whatever the environment says.
func TestProcessNamespaceIsNotAMaterializationSource(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"PORT":  map[string]any{"type": "integer", "minimum": 0},
			"HOSTS": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
	}
	sch, err := schema.Load(context.Background(), raw, "schema.yaml", schema.UnknownIgnore)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	tree := values.NewTree()
	tree.Env["PORT"] = "8080"
	tree.Env["HOSTS"] = `["x","y"]`
	tree.Env["APP_PORT"] = "-1"

	if err := sch.Materialize(tree); err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if value, exists := tree.Root["PORT"]; exists {
		t.Fatalf("PORT was materialized from the process environment: %#v", value)
	}
	if value, exists := tree.Root["HOSTS"]; exists {
		t.Fatalf("HOSTS was materialized from the process environment: %#v", value)
	}
	if err := sch.Validate(tree); err != nil {
		t.Fatalf("Validate() error = %v, want nothing to validate", err)
	}
}

// TestNoPathDerivationForNestedPaths pins the removal: APP_PORT must NOT fill
// app.port, because there is no implicit path -> ENV_NAME derivation.
func TestNoPathDerivationForNestedPaths(t *testing.T) {
	t.Parallel()
	sch := g14Schema(t)
	tree := values.NewTree()
	tree.Env["APP_PORT"] = "8080"
	tree.Env["APP_HOSTS"] = `["x","y"]`

	if err := sch.Materialize(tree); err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if _, exists := tree.Root["app"]; exists {
		t.Fatalf("app was synthesized from APP_* variables: %#v", tree.Root["app"])
	}
	if err := sch.Validate(tree); err == nil {
		t.Fatal("Validate() error = nil, want the required app to be reported missing")
	}
}

// TestValidateCatchesRootConstraintViolation pins that a root value from any
// source (a dotenv file included) is validated against the schema, which is the
// O1=(a) decision: .env keys are ordinary root keys.
func TestValidateCatchesRootConstraintViolation(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"type":       "object",
		"properties": map[string]any{"PORT": map[string]any{"type": "integer", "minimum": 0}},
	}
	sch, err := schema.Load(context.Background(), raw, "schema.yaml", schema.UnknownIgnore)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	tree := values.NewTree()
	tree.Root["PORT"] = "-1"
	if err := sch.Materialize(tree); err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if err := sch.Validate(tree); err == nil {
		t.Fatal("Validate() error = nil, want a minimum violation from the value")
	}
}
