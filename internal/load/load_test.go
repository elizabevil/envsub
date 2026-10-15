package load_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/elizabevil/envsub/internal/load"
	"github.com/elizabevil/envsub/internal/values"
)

// recordingObserver captures the layering the loader reports.
type recordingObserver struct {
	env  []string
	root map[string][]values.Meta
}

func newRecordingObserver() *recordingObserver {
	return &recordingObserver{root: map[string][]values.Meta{}}
}

func (o *recordingObserver) AddEnv(name string) { o.env = append(o.env, name) }

func (o *recordingObserver) AddRoot(path string, meta values.Meta) {
	o.root[path] = append(o.root[path], meta)
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", name, err)
	}
	return path
}

// TestLoadLayerOrder pins the fixed layering of the value tree: --set-json beats
// --set, both beat a dotenv file, which beats a value file, which beats a schema
// default (docs/07-serialization/04-cli.md §6.2).
func TestLoadLayerOrder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	valuesFile := writeFile(t, dir, "values.yaml", "K: from_values\nONLY_FILE: file\n")
	dotenvFile := writeFile(t, dir, "app.env", "K=from_dotenv\nONLY_DOTENV=dotenv\n")
	schemaFile := writeFile(t, dir, "schema.json",
		`{"type":"object","properties":{"D":{"type":"string","default":"from_default"}}}`)

	tests := []struct {
		name    string
		spec    load.Spec
		wantK   any
		wantAll map[string]any
	}{
		{
			name:  "value file only",
			spec:  load.Spec{Files: []string{valuesFile}, Schema: schemaFile},
			wantK: "from_values",
			wantAll: map[string]any{
				"K": "from_values", "ONLY_FILE": "file", "D": "from_default",
			},
		},
		{
			name:  "dotenv outranks the value file",
			spec:  load.Spec{Files: []string{valuesFile}, DotenvFiles: []string{dotenvFile}},
			wantK: "from_dotenv",
			wantAll: map[string]any{
				"K": "from_dotenv", "ONLY_FILE": "file", "ONLY_DOTENV": "dotenv",
			},
		},
		{
			name:  "set outranks the dotenv file",
			spec:  load.Spec{DotenvFiles: []string{dotenvFile}, Sets: []string{"K=from_set"}},
			wantK: "from_set",
		},
		{
			name:  "set-json outranks set",
			spec:  load.Spec{Sets: []string{"K=from_set"}, SetJSON: []string{`K="from_set_json"`}},
			wantK: "from_set_json",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := load.Load(context.Background(), tt.spec, nil)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if got := result.Tree.Root["K"]; got != tt.wantK {
				t.Fatalf("K = %#v, want %#v", got, tt.wantK)
			}
			for key, want := range tt.wantAll {
				if got := result.Tree.Root[key]; got != want {
					t.Fatalf("%s = %#v, want %#v", key, got, want)
				}
			}
		})
	}
}

// TestLoadReportsOrigins pins the observer contract: every contributing source is
// reported with its kind and, for a file source, its file and line. The render
// layer turns these calls into the --explain report.
func TestLoadReportsOrigins(t *testing.T) {
	t.Setenv("ENVSUB_LOAD_TEST", "from_process")
	dir := t.TempDir()
	valuesFile := writeFile(t, dir, "values.yaml", "FROM_VALUES: v\n")
	dotenvFile := writeFile(t, dir, "app.env", "FROM_DOTENV=dotenv\n")
	schemaFile := writeFile(t, dir, "schema.json",
		`{"type":"object","properties":{"FROM_DEFAULT":{"type":"string","default":"d"}}}`)

	observer := newRecordingObserver()
	if _, err := load.Load(context.Background(), load.Spec{
		Files:       []string{valuesFile},
		DotenvFiles: []string{dotenvFile},
		Schema:      schemaFile,
		Sets:        []string{"FROM_SET=1"},
	}, observer); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !slices.Contains(observer.env, "ENVSUB_LOAD_TEST") {
		t.Fatalf("process env origins = %v, want ENVSUB_LOAD_TEST", observer.env)
	}
	tests := []struct {
		path string
		kind values.SourceKind
		file string
		line int
	}{
		// Both YAML and JSON value files report the same stage; the file
		// extension tells the two apart (ADR-005).
		{path: "FROM_VALUES", kind: values.SourceValues, file: valuesFile},
		{path: "FROM_DOTENV", kind: values.SourceEnvFile, file: dotenvFile, line: 1},
		{path: "FROM_SET", kind: values.SourceSet},
		{path: "FROM_DEFAULT", kind: values.SourceSchema, file: schemaFile},
	}
	for _, tt := range tests {
		origins := observer.root[tt.path]
		if len(origins) == 0 {
			t.Fatalf("%s: no origin reported", tt.path)
		}
		last := origins[len(origins)-1]
		if last.Kind != tt.kind || last.File != tt.file || last.Line != tt.line {
			t.Fatalf("%s origin = %+v, want kind=%s file=%s line=%d",
				tt.path, last, tt.kind, tt.file, tt.line)
		}
	}
}

// TestOriginKindAgreesBetweenTreeAndReport pins the single origin vocabulary: the
// category recorded on the tree and the one reported to --explain are the same
// value for the same key, whatever the source. A loader-name vocabulary on one
// side ("yaml", "dotenv") would make this test fail for file sources.
func TestOriginKindAgreesBetweenTreeAndReport(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	valuesFile := filepath.Join(dir, "values.yaml")
	dotenvFile := filepath.Join(dir, "app.env")
	schemaFile := filepath.Join(dir, "schema.json")
	for name, content := range map[string]string{
		valuesFile: "FROM_VALUES: 1\n",
		dotenvFile: "FROM_DOTENV=2\n",
		schemaFile: `{"type":"object","properties":{"FROM_DEFAULT":{"type":"string","default":"3"}}}`,
	} {
		if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	observer := newRecordingObserver()
	result, err := load.Load(context.Background(), load.Spec{
		Files:       []string{valuesFile},
		DotenvFiles: []string{dotenvFile},
		Schema:      schemaFile,
		Sets:        []string{"FROM_SET=4"},
	}, observer)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	for _, path := range []string{"FROM_VALUES", "FROM_DOTENV", "FROM_DEFAULT", "FROM_SET"} {
		origins := observer.root[path]
		if len(origins) == 0 {
			t.Fatalf("%s: no origin reported", path)
		}
		reported := origins[len(origins)-1].Kind
		recorded := result.Tree.MetaFor(path).Meta.Kind
		if recorded != reported {
			t.Errorf("%s: tree records kind %q but the report says %q", path, recorded, reported)
		}
	}
}
