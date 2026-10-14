package render_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/elizabevil/envsub/internal/engine/extended"
	"github.com/elizabevil/envsub/internal/render"
)

// TestReportUnusedListsValueFileLeaves pins the --report=unused contract: only
// value-file leaves no template referenced are listed, a reference to a subtree
// covers everything under it, and paths are sorted.
func TestReportUnusedListsValueFileLeaves(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		template string
		want     string
	}{
		{
			name:     "nothing referenced",
			template: "static\n",
			want:     "app.port\napp.unused\nextra[0]\nextra[1]\n",
		},
		{
			name:     "one leaf referenced",
			template: "port=${app.port}\n",
			want:     "app.unused\nextra[0]\nextra[1]\n",
		},
		{
			name:     "whole subtree referenced",
			template: "extra=${extra | toJson}\n",
			want:     "app.port\napp.unused\n",
		},
		{
			name:     "every leaf referenced",
			template: "port=${app.port}\nunused=${app.unused}\nextra=${extra | toJson}\n",
			want:     "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			values := "app:\n  port: 8080\n  unused: dead\nextra: [a, b]\n"
			if err := os.WriteFile(filepath.Join(dir, "values.yaml"), []byte(values), 0o600); err != nil {
				t.Fatalf("write values: %v", err)
			}
			tmpl := filepath.Join(dir, "input.tmpl")
			if err := os.WriteFile(tmpl, []byte(tc.template), 0o600); err != nil {
				t.Fatalf("write template: %v", err)
			}

			var out bytes.Buffer
			req := render.Request{
				Input:  render.Input{Templates: []string{tmpl}},
				Values: render.Values{Files: []string{filepath.Join(dir, "values.yaml")}, NoProcessEnv: true},
				Report: render.Report{Name: render.ReportUnused, Writer: &out},
			}
			if err := render.Run(context.Background(), extended.New(), req); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got := out.String(); got != tc.want {
				t.Errorf("unused report = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestReportUnusedHonoursVariableFilters pins that a path the --only-vars /
// --ignore-vars filters exclude is not substituted, so it does not make a value
// used.
func TestReportUnusedHonoursVariableFilters(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "values.yaml"),
		[]byte("app:\n  port: 8080\n  unused: dead\n"), 0o600); err != nil {
		t.Fatalf("write values: %v", err)
	}
	tmpl := filepath.Join(dir, "input.tmpl")
	if err := os.WriteFile(tmpl, []byte("port=${app.port}\n"), 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}

	var out bytes.Buffer
	req := render.Request{
		Input:    render.Input{Templates: []string{tmpl}},
		Values:   render.Values{Files: []string{filepath.Join(dir, "values.yaml")}, NoProcessEnv: true},
		Policies: render.Policies{Ignore: []string{"app"}},
		Report:   render.Report{Name: render.ReportUnused, Writer: &out},
	}
	if err := render.Run(context.Background(), extended.New(), req); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := "app.port\napp.unused\n"
	if got := out.String(); got != want {
		t.Errorf("unused report = %q, want %q (an ignored reference is not a use)", got, want)
	}
}

// TestReportUnusedIgnoresNonFileSources pins that the report describes dead
// configuration in a file: a schema default, a --set value and a .env key are
// configuration of another layer and are never listed.
func TestReportUnusedIgnoresNonFileSources(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	files := map[string]string{
		"values.yaml": "app:\n  port: 8080\n",
		"extra.yaml":  "schema_only: from_file\n",
		"app.env":     "FROM_DOTENV=1\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	tmpl := filepath.Join(dir, "input.tmpl")
	if err := os.WriteFile(tmpl, []byte("static\n"), 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}

	var out bytes.Buffer
	req := render.Request{
		Input: render.Input{Templates: []string{tmpl}},
		Values: render.Values{
			Files:       []string{filepath.Join(dir, "extra.yaml")},
			DotenvFiles: []string{filepath.Join(dir, "app.env")},
			Sets:        []string{"FROM_SET=1"},
			Schema:      schemaWithDefault(t, dir),
		},
		Report: render.Report{Name: render.ReportUnused, Writer: &out},
	}
	if err := render.Run(context.Background(), extended.New(), req); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	got := out.String()
	for _, unexpected := range []string{"FROM_DOTENV", "FROM_SET", "schema_default"} {
		if bytes.Contains([]byte(got), []byte(unexpected)) {
			t.Errorf("report %q lists %q, which is not a value-file entry", got, unexpected)
		}
	}
	if !bytes.Contains([]byte(got), []byte("schema_only")) {
		t.Errorf("report %q does not list the unused value-file entry", got)
	}
}

// schemaWithDefault writes a schema declaring a default for a key no source
// provides, and returns its path.
func schemaWithDefault(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "schema.json")
	const schema = `{"type":"object","properties":{"schema_default":{"type":"string","default":"filled"}}}`
	if err := os.WriteFile(path, []byte(schema), 0o600); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	return path
}
