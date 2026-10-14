package source_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/source"
)

func TestReadFileTracksOrigin(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "values.yaml")
	if err := os.WriteFile(path, []byte("app:\n  port: 8080\n  nested:\n    key: value\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	loader, err := source.ForFile(path)
	if err != nil {
		t.Fatalf("ForFile() error = %v", err)
	}
	tree, err := loader.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	value, found, err := tree.Resolve("app.nested.key")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !found || value != "value" {
		t.Fatalf("Resolve() = %#v/%v, want value/true", value, found)
	}

	meta := tree.MetaFor("app.nested.key")
	if !meta.Found || meta.Meta.File != path {
		t.Fatalf("MetaFor() = %#v, want file %q", meta, path)
	}
}

// TestDotenvLoaderIgnoresFileName pins the --env-file contract: the flag names
// the format, so names ForFile cannot classify (.env.local, extensionless,
// .txt) still parse as dotenv.
func TestDotenvLoaderIgnoresFileName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tests := []string{".env.local", "production", "debug.env.txt"}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(dir, name)
			if err := os.WriteFile(path, []byte("K=from_"+name+"\n"), 0o600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			// ForFile rejects these names; the forced loader must not.
			if _, err := source.ForFile(path); err == nil {
				t.Fatalf("ForFile(%q) error = nil, want it to reject the name", name)
			}
			loader := source.DotenvLoader()
			if !loader.Dotenv || loader.Name != source.DotenvName {
				t.Fatalf("DotenvLoader() = %+v, want the dotenv loader", loader)
			}
			tree, err := loader.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}
			// A .env file defines root keys like any other value file.
			if got := tree.Root["K"]; got != "from_"+name {
				t.Fatalf("K = %#v, want %q", got, "from_"+name)
			}
		})
	}
}

func TestReadFileMissing(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "missing.yaml")
	loader, err := source.ForFile(path)
	if err != nil {
		t.Fatalf("ForFile() error = %v", err)
	}
	if _, err := loader.ReadFile(path); err == nil {
		t.Fatalf("ReadFile() error = nil, want error")
	}
}

func TestLoadPreservesTypes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		file string
		data string
		path string
		want any
	}{
		{name: "yaml int", file: "v.yaml", data: "a: 8080\n", path: "a", want: 8080},
		{name: "yaml bool", file: "v.yaml", data: "a: true\n", path: "a", want: true},
		{name: "json string", file: "v.json", data: `{"a":"x"}`, path: "a", want: "x"},
		{name: "yaml array", file: "v.yaml", data: "a: [1, 2]\n", path: "a[1]", want: 2},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := filepath.Join(dir, tt.file)
			if err := os.WriteFile(path, []byte(tt.data), 0o600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			loader, err := source.ForFile(path)
			if err != nil {
				t.Fatalf("ForFile() error = %v", err)
			}
			tree, err := loader.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}
			got, found, err := tree.Resolve(tt.path)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if !found {
				t.Fatalf("Resolve(%q) not found", tt.path)
			}
			if got != tt.want {
				t.Fatalf("Resolve(%q) = %#v (type %T), want %#v (type %T)", tt.path, got, got, tt.want, tt.want)
			}
		})
	}
}

func TestReadFileParseError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("a: [unclosed\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	loader, err := source.ForFile(path)
	if err != nil {
		t.Fatalf("ForFile() error = %v", err)
	}
	if _, err := loader.ReadFile(path); err == nil {
		t.Fatalf("ReadFile() error = nil, want parse error")
	}
}

// TestReadFileMissingValueFile pins the dedicated diagnostic for a value file that
// does not exist: the missing-file error names the path, not the generic read error.
func TestReadFileMissingValueFile(t *testing.T) {
	t.Parallel()
	loader, err := source.ForFile("missing.yaml")
	if err != nil {
		t.Fatalf("ForFile() error = %v", err)
	}
	_, err = loader.ReadFile(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("ReadFile() error = nil, want error")
	}
	if want := "value file "; !strings.HasPrefix(err.Error(), want) || !strings.HasSuffix(err.Error(), "does not exist") {
		t.Fatalf("ReadFile() error = %q, want %q…%q", err, want, "does not exist")
	}
	if kind := diag.KindOf(err); kind != diag.KindIO {
		t.Fatalf("ReadFile() kind = %v, want io", kind)
	}
}
