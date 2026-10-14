package render_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/elizabevil/envsub/internal/render"
)

// TestRunLayerOrder pins the root layering through the render pipeline:
// --set beats a dotenv file, and a dotenv file beats a value file. The process
// environment is a separate namespace, so it never competes here.
func TestRunLayerOrder(t *testing.T) {
	dir := t.TempDir()
	values := filepath.Join(dir, "values.yaml")
	dotenv := filepath.Join(dir, "app.env")
	tmpl := filepath.Join(dir, "input.tmpl")
	out := filepath.Join(dir, "out.txt")

	if err := os.WriteFile(values, []byte("K: from_values\nONLY_FILE: file\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(values) error = %v", err)
	}
	if err := os.WriteFile(dotenv, []byte("K=from_dotenv\nONLY_DOTENV=dotenv\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(dotenv) error = %v", err)
	}
	if err := os.WriteFile(tmpl, []byte("K=${K} F=${ONLY_FILE} D=${ONLY_DOTENV}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(tmpl) error = %v", err)
	}

	t.Setenv("K", "from_process")
	req := render.Request{
		Input: render.Input{Templates: []string{tmpl}},
		Values: render.Values{
			Files:       []string{values},
			DotenvFiles: []string{dotenv},
		},
		Output: render.Output{Path: out},
	}
	if err := render.Run(context.Background(), extendedEngine(), req); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	// The dotenv file is the layer above the value file, and the host variable K
	// must not leak into a bare name.
	if want := "K=from_dotenv F=file D=dotenv\n"; string(got) != want {
		t.Fatalf("output = %q, want %q", got, want)
	}

	// --set is the top layer and wins over the dotenv file.
	req.Values.Sets = []string{"K=from_set"}
	if err := render.Run(context.Background(), extendedEngine(), req); err != nil {
		t.Fatalf("Run() with --set error = %v", err)
	}
	got, err = os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if want := "K=from_set F=file D=dotenv\n"; string(got) != want {
		t.Fatalf("output with --set = %q, want %q", got, want)
	}
}

// TestProcessNamespaceReadsHostEnvironment pins that only the @process namespace
// reads the process environment, that --no-process-env empties it, and that
// --process-env-prefix filters it without touching dotenv files.
func TestProcessNamespaceReadsHostEnvironment(t *testing.T) {
	dir := t.TempDir()
	dotenv := filepath.Join(dir, "app.env")
	tmpl := filepath.Join(dir, "input.tmpl")
	out := filepath.Join(dir, "out.txt")

	if err := os.WriteFile(dotenv, []byte("FROM_DOTENV=dotenv\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(dotenv) error = %v", err)
	}
	if err := os.WriteFile(tmpl, []byte("bare=${ENVSUB_LAYER_TEST:-missing} proc=${@process.ENVSUB_LAYER_TEST:-missing} file=${FROM_DOTENV}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(tmpl) error = %v", err)
	}
	t.Setenv("ENVSUB_LAYER_TEST", "from_process")

	base := render.Request{
		Input:  render.Input{Templates: []string{tmpl}},
		Values: render.Values{DotenvFiles: []string{dotenv}},
		Output: render.Output{Path: out},
	}
	if err := render.Run(context.Background(), extendedEngine(), base); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := readFile(t, out); got != "bare=missing proc=from_process file=dotenv\n" {
		t.Fatalf("output = %q, want the host value only behind @process", got)
	}

	hidden := base
	hidden.Values.NoProcessEnv = true
	if err := render.Run(context.Background(), extendedEngine(), hidden); err != nil {
		t.Fatalf("Run() --no-process-env error = %v", err)
	}
	if got := readFile(t, out); got != "bare=missing proc=missing file=dotenv\n" {
		t.Fatalf("output with --no-process-env = %q, want @process empty and dotenv unaffected", got)
	}

	filtered := base
	filtered.Values.ProcessEnvPrefix = "ENVSUB_OTHER_"
	if err := render.Run(context.Background(), extendedEngine(), filtered); err != nil {
		t.Fatalf("Run() --process-env-prefix error = %v", err)
	}
	if got := readFile(t, out); got != "bare=missing proc=missing file=dotenv\n" {
		t.Fatalf("output with --process-env-prefix = %q, want the whitelist to hide it", got)
	}
}
