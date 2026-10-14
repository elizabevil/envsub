package render_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/engine/extended"
	"github.com/elizabevil/envsub/internal/render"
)

// stdinReader returns a reader over content. The render request takes its stdin
// as a field, so a test never touches the process-wide os.Stdin: replacing it
// would race with every other parallel test in this package.
func stdinReader(content string) io.Reader {
	return strings.NewReader(content)
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
	stdin := stdinReader("port=${app.port}\n")

	var explain, unused bytes.Buffer
	req := render.Request{
		Input:        render.Input{Templates: []string{"-"}},
		Values:       render.Values{Files: []string{filepath.Join(dir, "values.yaml")}, NoProcessEnv: true},
		Explain:      render.Explain{Enabled: true, Writer: &explain},
		Report:       render.Report{Name: render.ReportUnused, Writer: &unused},
		Output:       render.Output{},
		ValidateOnly: false,
		Stdin:        stdin,
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
