package render_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/render"
)

// batchRequest returns a recursive --output-dir request over src with A and B set.
func batchRequest(src, outDir string, strip ...string) render.Request {
	req := request(src)
	req.Input.Recursive = true
	req.Output.Dir = outDir
	req.Output.StripSuffixes = strip
	req.Values.Sets = []string{"A=1", "B=2"}
	req.Policies = render.Policies{
		OnMissing:       engine.UnsetError,
		OnEmpty:         engine.EmptyOK,
		UnknownKeywords: "ignore",
	}
	return req
}

// TestStripSuffixRenamesBatchOutputs pins the flag contract: the suffix is removed
// from every --output-dir file name, the directory layout is kept, and a name that
// does not carry the suffix stays untouched.
func TestStripSuffixRenamesBatchOutputs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "templates")
	writeFile(t, filepath.Join(src, "nginx.conf.tmpl"), "a=${A}\n")
	writeFile(t, filepath.Join(src, "sub", "service.yaml.tmpl"), "b=${B}\n")
	writeFile(t, filepath.Join(src, "plain.conf"), "a=${A}\n")
	outDir := filepath.Join(dir, "out")

	req := batchRequest(src, outDir, ".tmpl")
	if err := render.Run(context.Background(), extendedEngine(), req); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for path, want := range map[string]string{
		"nginx.conf":                         "a=1\n",
		filepath.Join("sub", "service.yaml"): "b=2\n",
		"plain.conf":                         "a=1\n",
	} {
		if got := readFile(t, filepath.Join(outDir, path)); got != want {
			t.Fatalf("%s = %q, want %q", path, got, want)
		}
	}
	if _, err := os.Stat(filepath.Join(outDir, "nginx.conf.tmpl")); err == nil {
		t.Fatal("nginx.conf.tmpl still exists, want only nginx.conf")
	}
}

// TestStripSuffixLongestMatchWins pins that the order of the values does not matter:
// ".yaml.tmpl" removes more than ".tmpl" whichever way round they are given.
func TestStripSuffixLongestMatchWins(t *testing.T) {
	t.Parallel()
	for _, suffixes := range [][]string{{".tmpl", ".yaml.tmpl"}, {".yaml.tmpl", ".tmpl"}} {
		dir := t.TempDir()
		src := filepath.Join(dir, "templates")
		writeFile(t, filepath.Join(src, "service.yaml.tmpl"), "b=1\n")
		outDir := filepath.Join(dir, "out")

		req := batchRequest(src, outDir, suffixes...)
		if err := render.Run(context.Background(), extendedEngine(), req); err != nil {
			t.Fatalf("Run(%v) error = %v", suffixes, err)
		}
		if _, err := os.Stat(filepath.Join(outDir, "service")); err != nil {
			t.Fatalf("Run(%v): service not written: %v", suffixes, err)
		}
	}
}

// TestStripSuffixRejectsCollisions pins that two templates mapping to one output is
// a usage error instead of a silent overwrite.
func TestStripSuffixRejectsCollisions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "templates")
	writeFile(t, filepath.Join(src, "a.tmpl"), "one\n")
	writeFile(t, filepath.Join(src, "a"), "two\n")

	req := batchRequest(src, filepath.Join(dir, "out"), ".tmpl")
	err := render.Run(context.Background(), extendedEngine(), req)
	if err == nil {
		t.Fatal("Run() error = nil, want a collision error")
	}
	var de *diag.Error
	if !errors.As(err, &de) || de.Kind != diag.KindUsage {
		t.Fatalf("Run() error = %v, want KindUsage", err)
	}
	if !strings.Contains(err.Error(), "both map to") {
		t.Fatalf("Run() error = %v, want a collision message", err)
	}
}

// TestStripSuffixValidation pins the usage errors: an empty suffix, or the flag
// without --output-dir.
func TestStripSuffixValidation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		req  render.Request
		want string
	}{
		{
			name: "empty suffix",
			req:  batchRequest(t.TempDir(), "out", ""),
			want: "--strip-suffix needs a non-empty suffix",
		},
		{
			name: "without output dir",
			req: func() render.Request {
				r := request("a.tmpl")
				r.Output.StripSuffixes = []string{".tmpl"}
				return r
			}(),
			want: "--strip-suffix only applies to --output-dir",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.req.Validate()
			var de *diag.Error
			if !errors.As(err, &de) || de.Kind != diag.KindUsage {
				t.Fatalf("Validate() error = %v, want KindUsage", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tc.want)
			}
		})
	}
}

// TestStripSuffixIsIgnoredByOtherModes pins that -o keeps its explicit path and -i
// keeps the template name: only --output-dir names are derived.
func TestStripSuffixIsIgnoredByOtherModes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.tmpl"), "a=1\n")

	out := filepath.Join(dir, "explicit.tmpl")
	req := request(filepath.Join(dir, "a.tmpl"))
	req.Output.Path = out
	req.Values.Sets = []string{"A=1"}
	req.Policies = render.Policies{OnMissing: engine.UnsetError, OnEmpty: engine.EmptyOK, UnknownKeywords: "ignore"}
	if err := render.Run(context.Background(), extendedEngine(), req); err != nil {
		t.Fatalf("Run(-o) error = %v", err)
	}
	if got := readFile(t, out); got != "a=1\n" {
		t.Fatalf("explicit output = %q, want a=1", got)
	}
}

// TestCheckReportsTheStrippedTargetName pins that --check uses the same naming, so
// the diagnostic names the file the flag would write.
func TestCheckReportsTheStrippedTargetName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "templates")
	writeFile(t, filepath.Join(src, "a.tmpl"), "a=1\n")
	outDir := filepath.Join(dir, "out")

	req := batchRequest(src, outDir, ".tmpl")
	req.Output.Check = true
	req.Values.Sets = []string{"A=1"}
	err := render.Run(context.Background(), extendedEngine(), req)
	var de *diag.Error
	if !errors.As(err, &de) || de.Kind != diag.KindDiff {
		t.Fatalf("Run(--check) error = %v, want KindDiff", err)
	}
	if !strings.Contains(err.Error(), filepath.Join("out", "a")) {
		t.Fatalf("Run(--check) error = %v, want the stripped target out/a", err)
	}
}
