package render_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/render"
)

// TestReportsRequireAPathEngine pins the failure mode when the engine cannot
// enumerate the paths a template references: --explain and --report=unused need
// PathEngine, and an engine without it must fail loudly instead of reporting
// nothing.
func TestReportsRequireAPathEngine(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		build func() render.Request
	}{
		{
			name: "--explain",
			build: func() render.Request {
				return render.Request{
					Input:   render.Input{Templates: []string{"-"}},
					Explain: render.Explain{Enabled: true},
				}
			},
		},
		{
			name: "--report=unused",
			build: func() render.Request {
				return render.Request{
					Input:  render.Input{Templates: []string{"-"}},
					Report: render.Report{Name: render.ReportUnused},
					Stdin:  strings.NewReader("plain\n"),
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := render.Run(context.Background(), stubEngine{}, tc.build())
			if got := diag.KindOf(err); got != diag.KindInternal {
				t.Fatalf("Run() error = %v (kind %q), want kind %q", err, got, diag.KindInternal)
			}
			if err == nil || !strings.Contains(err.Error(), "is not supported by engine") {
				t.Errorf("error = %v, want it to name the unsupported engine", err)
			}
		})
	}
}

// TestReportsDefaultToStderr pins that both reports fall back to stderr when the
// caller supplies no writer, which is what a library caller relies on.
func TestReportsDefaultToStderr(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "values.yaml"), []byte("app:\n  port: 1\n"), 0o600); err != nil {
		t.Fatalf("write values: %v", err)
	}
	tmpl := filepath.Join(dir, "input.tmpl")
	if err := os.WriteFile(tmpl, []byte("port=${app.port}\n"), 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}

	// No Explain.Writer and no Report.Writer: both reports must pick stderr
	// rather than panicking on a nil writer.
	req := render.Request{
		Input:   render.Input{Templates: []string{tmpl}},
		Values:  render.Values{Files: []string{filepath.Join(dir, "values.yaml")}, NoProcessEnv: true},
		Explain: render.Explain{Enabled: true},
		Report:  render.Report{Name: render.ReportUnused},
	}
	if err := render.Run(context.Background(), extendedEngine(), req); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}
