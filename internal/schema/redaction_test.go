package schema_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/schema"
	"github.com/elizabevil/envsub/internal/source"
	"github.com/elizabevil/envsub/internal/values"
)

// decodeSchema reads a JSON schema document the way the loader does.
func decodeSchema(t *testing.T, path string) any {
	t.Helper()
	loader, err := source.ForFile(path)
	if err != nil {
		t.Fatalf("loader for %s: %v", path, err)
	}
	loaded, err := loader.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return loaded.Root
}

// TestMaterializeErrorHidesTheValue pins the redaction rule: a value that fails
// to materialize may be a secret from a .env file or --set-file, so the error
// names where it came from but never repeats it.
func TestMaterializeErrorHidesTheValue(t *testing.T) {
	t.Parallel()
	const secret = "sup3r-s3cret-token"
	dir := t.TempDir()
	dotenv := filepath.Join(dir, "app.env")
	if err := os.WriteFile(dotenv, []byte("TOKEN="+secret+"\n"), 0o600); err != nil {
		t.Fatalf("write dotenv: %v", err)
	}
	schemaPath := filepath.Join(dir, "schema.json")
	const doc = `{"type":"object","properties":{"TOKEN":{"type":"array"}}}`
	if err := os.WriteFile(schemaPath, []byte(doc), 0o600); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	loader := source.DotenvFormat().Loader()
	loaded, err := loader.ReadFile(dotenv)
	if err != nil {
		t.Fatalf("read dotenv: %v", err)
	}
	tree := values.NewTree()
	values.MergeInto(tree, loaded, values.Meta{File: dotenv, Kind: source.DotenvName})

	compiled, err := schema.Load(context.Background(), decodeSchema(t, schemaPath), schemaPath, schema.UnknownIgnore)
	if err != nil {
		t.Fatalf("load schema: %v", err)
	}
	err = compiled.Materialize(tree)
	if err == nil {
		t.Fatal("Materialize() error = nil, want a type error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("error %q repeats the value; it must only name the origin", err)
	}
	if !strings.Contains(err.Error(), "TOKEN") || !strings.Contains(err.Error(), "app.env") {
		t.Errorf("error %q should still name the path and the origin", err)
	}
}
