package source_test

import (
	"github.com/elizabevil/envsub/internal/values"
	"testing"

	"github.com/elizabevil/envsub/internal/source"
)

func TestParseDotenv(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    map[string]string
		wantErr bool
	}{
		{
			name:  "plain and export",
			input: "A=1\nexport B=two\n",
			want:  map[string]string{"A": "1", "B": "two"},
		},
		{
			name:  "comments and blanks",
			input: "# comment\n\nA=1 # trailing\nB=2\n",
			want:  map[string]string{"A": "1", "B": "2"},
		},
		{
			name:  "single quotes keep JSON array",
			input: `HOSTS='["a","b"]'` + "\n",
			want:  map[string]string{"HOSTS": `["a","b"]`},
		},
		{
			name:  "double quotes with escapes",
			input: "A=\"line\\nbreak\"\n",
			want:  map[string]string{"A": "line\nbreak"},
		},
		{
			name:  "empty value",
			input: "A=\n",
			want:  map[string]string{"A": ""},
		},
		{
			name:    "duplicate key",
			input:   "A=1\nA=2\n",
			wantErr: true,
		},
		{
			name:    "invalid key",
			input:   "1BAD=2\n",
			wantErr: true,
		},
		{
			name:    "unclosed quote",
			input:   "A=\"x\n",
			wantErr: true,
		},
		{
			name:    "trailing text after quote",
			input:   "A=\"x\" y\n",
			wantErr: true,
		},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := source.ParseDotenv([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseDotenv() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDotenv() error = %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("ParseDotenv() = %#v, want %#v", got, tt.want)
			}
			for key, want := range tt.want {
				entry, ok := got[key]
				if !ok {
					t.Fatalf("ParseDotenv() missing key %q in %#v", key, got)
				}
				if entry.Value != want {
					t.Fatalf("ParseDotenv()[%q] = %q, want %q", key, entry.Value, want)
				}
				if entry.Line <= 0 {
					t.Fatalf("ParseDotenv()[%q] line = %d, want > 0", key, entry.Line)
				}
			}
		})
	}
}

func TestEnvironLoadsRootKeys(t *testing.T) {
	t.Parallel()
	tree, err := source.Environ{}.Load([]byte("A=1\nB=two\n"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	// A .env file is a configuration layer: its keys land in the root namespace like
	// a value file's keys, not in the process environment.
	if len(tree.Env) != 0 {
		t.Fatalf("Load() env = %#v, want empty", tree.Env)
	}
	if tree.Root["A"] != "1" || tree.Root["B"] != "two" {
		t.Fatalf("Load() root = %#v", tree.Root)
	}
	meta, ok := tree.MetaAt("A")
	if !ok {
		t.Fatal("Load() did not record the A origin")
	}
	if meta.Kind != values.SourceEnvFile || meta.Line != 1 {
		t.Fatalf("Load() meta for A = %+v, want kind %q line 1", meta, values.SourceEnvFile)
	}
}

func TestForFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path    string
		name    string
		kind    values.SourceKind
		wantErr bool
	}{
		// The loader declares the origin category, so this table is the only
		// place the mapping is pinned: a .env file is its own layer, a structured
		// value file is the values layer whatever its format.
		{path: ".env", name: source.DotenvName, kind: values.SourceEnvFile},
		{path: "values.yaml", name: source.YAMLName, kind: values.SourceValues},
		{path: "values.yml", name: source.YAMLName, kind: values.SourceValues},
		{path: "values.json", name: source.JSONName, kind: values.SourceValues},
		{path: "values.toml", wantErr: true},
	}
	for _, tt := range tests {

		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			loader, err := source.ForFile(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ForFile(%q) error = nil, want error", tt.path)
				}
				return
			}
			if err != nil {
				t.Fatalf("ForFile(%q) error = %v", tt.path, err)
			}
			if loader.Name() != tt.name {
				t.Fatalf("ForFile(%q).Name() = %q, want %q", tt.path, loader.Name(), tt.name)
			}
			if loader.Kind != tt.kind {
				t.Fatalf("ForFile(%q).Kind = %q, want %q", tt.path, loader.Kind, tt.kind)
			}
		})
	}
}

// TestDotenvLoaderDeclaresItsKind pins the --dotenv-file path: it selects the same
// loader as a .env extension, so it must report the same origin category.
func TestDotenvLoaderDeclaresItsKind(t *testing.T) {
	t.Parallel()
	loader := source.DotenvFormat().Loader()
	if loader.Kind != values.SourceEnvFile {
		t.Fatalf("DotenvLoader().Kind = %q, want %q", loader.Kind, values.SourceEnvFile)
	}
}

// TestParseDotenvStripsLeadingBOM pins the .env entry: a file saved with a UTF-8
// byte order mark defines A, not "\ufeffA".
func TestParseDotenvStripsLeadingBOM(t *testing.T) {
	t.Parallel()
	env, err := source.ParseDotenv([]byte("\ufeffA=1\nB=2\n"))
	if err != nil {
		t.Fatalf("ParseDotenv(BOM) error = %v", err)
	}
	if got, ok := env["A"]; !ok || got.Value != "1" {
		t.Fatalf("ParseDotenv(BOM) = %#v, want the key A with value 1", env)
	}
	if len(env) != 2 {
		t.Fatalf("ParseDotenv(BOM) defined %d keys, want 2 (%#v)", len(env), env)
	}
}
