package render_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/engine/extended"
	"github.com/elizabevil/envsub/internal/render"
)

// writeCheckCase writes a template and its values file into a fresh directory.
func writeCheckCase(t *testing.T, values, template string) (dir, tmpl string) {
	t.Helper()
	dir = t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "values.yaml"), []byte(values), 0o600); err != nil {
		t.Fatalf("write values: %v", err)
	}
	tmpl = filepath.Join(dir, "input.tmpl")
	if err := os.WriteFile(tmpl, []byte(template), 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}
	return dir, tmpl
}

func checkRequest(dir, tmpl string, out render.Output) render.Request {
	return render.Request{
		Input:  render.Input{Templates: []string{tmpl}},
		Values: render.Values{Files: []string{filepath.Join(dir, "values.yaml")}, NoProcessEnv: true},
		Output: out,
	}
}

// TestCheckReportsOutdatedAndWritesNothing pins the --check contract: a differing
// or missing target is reported with the diff exit code, and neither the target
// nor anything else is written.
func TestCheckReportsOutdatedAndWritesNothing(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		existing string
		create   bool
		wantExit int
	}{
		{name: "missing target", create: false, wantExit: 7},
		{name: "stale target", existing: "port=9999\n", create: true, wantExit: 7},
		{name: "matching target", existing: "port=8080\n", create: true, wantExit: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir, tmpl := writeCheckCase(t, "app:\n  port: 8080\n", "port=${app.port}\n")
			out := filepath.Join(dir, "out.txt")
			if tc.create {
				if err := os.WriteFile(out, []byte(tc.existing), 0o600); err != nil {
					t.Fatalf("write target: %v", err)
				}
			}
			before := readDirState(t, dir)

			req := checkRequest(dir, tmpl, render.Output{Path: out, Check: true})
			err := render.Run(context.Background(), extended.New(), req)
			if got := diag.ExitCode(err); got != tc.wantExit {
				t.Fatalf("Run() exit = %d (%v), want %d", got, err, tc.wantExit)
			}
			if tc.wantExit == 7 {
				var de *diag.Error
				if !errors.As(err, &de) || de.Kind != diag.KindDiff {
					t.Fatalf("Run() error = %v, want a KindDiff error", err)
				}
				if !strings.Contains(err.Error(), "out.txt") {
					t.Errorf("error %q does not name the outdated target", err)
				}
			}
			if after := readDirState(t, dir); after != before {
				t.Errorf("--check changed the directory: before %q, after %q", before, after)
			}
		})
	}
}

// TestCheckInPlaceLeavesTheTemplateAlone pins the in-place variant: the template
// file is the target, so a stale template is reported and left untouched.
func TestCheckInPlaceLeavesTheTemplateAlone(t *testing.T) {
	t.Parallel()
	dir, tmpl := writeCheckCase(t, "app:\n  port: 8080\n", "port=${app.port}\n")
	before := readDirState(t, dir)

	req := checkRequest(dir, tmpl, render.Output{InPlace: true, Check: true})
	err := render.Run(context.Background(), extended.New(), req)
	if got := diag.ExitCode(err); got != 7 {
		t.Fatalf("Run() exit = %d (%v), want 7", got, err)
	}
	if after := readDirState(t, dir); after != before {
		t.Errorf("--check -i changed the directory: before %q, after %q", before, after)
	}
}

// readDirState renders every entry of a directory as "name:content" so a test can
// compare the whole directory before and after a run.
func readDirState(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var b strings.Builder
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", entry.Name(), err)
		}
		b.WriteString(entry.Name())
		b.WriteString(":")
		b.Write(data)
		b.WriteString("\n")
	}
	return b.String()
}
