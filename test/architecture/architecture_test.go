// Package architecture_test pins the structural boundaries the render pipeline
// relies on. These are not behaviour tests: they fail when a package grows a
// dependency it is not supposed to have.
package architecture_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// modulePath is the import path prefix of every package under test.
const modulePath = "github.com/elizabevil/envsub"

// repoRoot walks up from the test file to the directory holding go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test directory")
		}
		dir = parent
	}
}

// packageImports parses the non-test import blocks of every Go file in a package
// directory and returns the module-internal imports it declares.
func packageImports(t *testing.T, pkgDir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(repoRoot(t), pkgDir))
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", pkgDir, err)
	}
	seen := map[string]bool{}
	var imports []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(repoRoot(t), pkgDir, name))
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", name, err)
		}
		for line := range strings.SplitSeq(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, `"`+modulePath) && !strings.HasPrefix(trimmed, `_ "`+modulePath) {
				continue
			}
			importPath := strings.Trim(strings.TrimPrefix(trimmed, "_ "), `"`)
			if !seen[importPath] {
				seen[importPath] = true
				imports = append(imports, importPath)
			}
		}
	}
	return imports
}

// assertDoesNotImport fails when pkgDir imports one of the forbidden packages.
func assertDoesNotImport(t *testing.T, pkgDir string, forbidden ...string) {
	t.Helper()
	blocked := map[string]bool{}
	for _, path := range forbidden {
		blocked[path] = true
	}
	for _, imported := range packageImports(t, pkgDir) {
		if blocked[imported] {
			t.Errorf("%s imports %s, which the layer boundary does not allow", pkgDir, imported)
		}
	}
}

// TestEngineDoesNotImportSchema pins D1: the template evaluator queries types
// through the types.TypeResolver interface, so it must not depend on the schema
// loader, normalizer or JSON Schema backend.
func TestEngineDoesNotImportSchema(t *testing.T) {
	t.Parallel()
	schemaPkg := modulePath + "/internal/schema"
	assertDoesNotImport(t, "internal/engine", schemaPkg)
	assertDoesNotImport(t, "internal/engine/extended", schemaPkg)
}

// TestRenderDoesNotImportSchema pins the same boundary one layer up: the render
// pipeline receives a built tree from internal/load and never compiles a schema
// itself.
func TestRenderDoesNotImportSchema(t *testing.T) {
	t.Parallel()
	assertDoesNotImport(t, "internal/render", modulePath+"/internal/schema")
}

// TestLoadDoesNotImportRenderOrEngine pins D5: value loading is the first half of
// the pipeline and must not know about templates, engines or output.
func TestLoadDoesNotImportRenderOrEngine(t *testing.T) {
	t.Parallel()
	assertDoesNotImport(t, "internal/load",
		modulePath+"/internal/render",
		modulePath+"/internal/engine",
		modulePath+"/internal/engine/extended",
		modulePath+"/internal/output",
	)
}

// TestCLIDoesNotExecuteThePipeline pins D6: the command tree parses flags and
// builds a request, while the runner injected by internal/app executes it. A
// direct render.Run call here would collapse the two responsibilities again.
func TestCLIDoesNotExecuteThePipeline(t *testing.T) {
	t.Parallel()
	assertDoesNotCall(t, "internal/cli", "render.Run(")
}

// TestValuesDoesNotDependOnHigherLayers pins that the value tree stays the
// foundation: it is used by every other package, so it may not import any of them.
func TestValuesDoesNotDependOnHigherLayers(t *testing.T) {
	t.Parallel()
	assertDoesNotImport(t, "internal/values",
		modulePath+"/internal/engine",
		modulePath+"/internal/engine/extended",
		modulePath+"/internal/render",
		modulePath+"/internal/load",
		modulePath+"/internal/schema",
		modulePath+"/internal/source",
		modulePath+"/internal/pipeline",
	)
}

// assertDoesNotCall fails when any non-test file in pkgDir contains needle.
func assertDoesNotCall(t *testing.T, pkgDir, needle string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(repoRoot(t), pkgDir))
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", pkgDir, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(repoRoot(t), pkgDir, name))
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", name, err)
		}
		if strings.Contains(string(data), needle) {
			t.Errorf("%s/%s contains %q; the CLI must not execute the render pipeline", pkgDir, name, needle)
		}
	}
}
