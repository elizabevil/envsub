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

// withStdin swaps the process stdin for the duration of a test and restores it.
func withStdin(t *testing.T, content string) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}
	original := os.Stdin
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = original
		_ = reader.Close()
	})
	go func() {
		_, _ = writer.WriteString(content)
		_ = writer.Close()
	}()
}

// TestStdinIsReadOnce pins that --explain and --report=unused together still
// render stdin: stdin cannot be rewound, so a second read would render nothing.
func TestStdinIsReadOnce(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "values.yaml"),
		[]byte("app:\n  port: 8080\n  unused: dead\n"), 0o600); err != nil {
		t.Fatalf("write values: %v", err)
	}
	withStdin(t, "port=${app.port}\n")

	var explain, unused bytes.Buffer
	req := render.Request{
		Input:        render.Input{Templates: []string{"-"}},
		Values:       render.Values{Files: []string{filepath.Join(dir, "values.yaml")}, NoProcessEnv: true},
		Explain:      render.Explain{Enabled: true, Writer: &explain},
		Report:       render.Report{Name: render.ReportUnused, Writer: &unused},
		Output:       render.Output{},
		ValidateOnly: false,
	}
	// The rendered bytes go to stdout; capture them by pointing the writer at a
	// file instead, which also proves the render actually happened.
	out := filepath.Join(dir, "rendered.txt")
	req.Output = render.Output{Path: out}
	if err := render.Run(context.Background(), extended.New(), req); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	rendered, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("rendered output: %v", err)
	}
	if got, want := string(rendered), "port=8080\n"; got != want {
		t.Errorf("rendered = %q, want %q (stdin must be read exactly once)", got, want)
	}
	if !bytes.Contains(explain.Bytes(), []byte(`"variable":"app.port"`)) {
		t.Errorf("--explain report = %q, want the app.port record", explain.String())
	}
	if got, want := unused.String(), "app.unused\n"; got != want {
		t.Errorf("unused report = %q, want %q", got, want)
	}
}
