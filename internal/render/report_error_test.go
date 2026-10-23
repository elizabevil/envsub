package render_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/event"
	"github.com/elizabevil/envsub/internal/render"
)

// failingWriter makes the --explain report fail after the render already did.
type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

// TestExplainReportFailureKeepsRenderError pins E6: a failing report writer must
// not replace the render error. The joined error keeps the render failure first,
// so its kind and exit code still describe the actual failure, and both messages
// remain visible.
func TestExplainReportFailureKeepsRenderError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	single := filepath.Join(dir, "single.tmpl")
	writeFile(t, single, "${MISSING}\n")
	first := filepath.Join(dir, "first.tmpl")
	second := filepath.Join(dir, "second.tmpl")
	writeFile(t, first, "${MISSING_A}\n")
	writeFile(t, second, "${MISSING_B}\n")

	tests := []struct {
		name      string
		templates []string
		want      []string
	}{
		{name: "single template", templates: []string{single}, want: []string{`variable "MISSING" is not set`}},
		{name: "batch", templates: []string{first, second}, want: []string{"MISSING_A", "MISSING_B"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := render.Run(context.Background(), extendedEngine(), render.Request{
				Input:   render.Input{Templates: tt.templates},
				Values:  render.Values{NoProcessEnv: true},
				Explain: render.Explain{Enabled: true},
				Events:  event.NewWriter(failingWriter{err: errors.New("disk full")}),
			})
			if err == nil {
				t.Fatal("Run() error = nil, want a missing-value failure")
			}
			if got := diag.KindOf(err); got != diag.KindMissing {
				t.Fatalf("Run() error kind = %q (%v), want %q", got, err, diag.KindMissing)
			}
			if got := diag.ExitCode(err); got != 6 {
				t.Fatalf("Run() exit = %d (%v), want 6", got, err)
			}
			for _, want := range append(tt.want, "disk full") {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("Run() error = %q, want it to contain %q", err, want)
				}
			}
		})
	}
}
