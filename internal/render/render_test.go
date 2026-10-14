package render_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/render"
)

type stubEngine struct{}

func (stubEngine) Name() string { return "extended" }
func (stubEngine) Render(_ context.Context, in engine.Input) ([]byte, error) {
	return append([]byte("stub:"), in.Template...), nil
}

func TestReadTemplateFromStdin(t *testing.T) {
	t.Parallel()
	got, err := render.ReadTemplateForTest([]string{}, bytes.NewBufferString("hello"))
	if err != nil {
		t.Fatalf("readTemplate() error = %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("readTemplate() = %q, want %q", string(got), "hello")
	}
}

func TestReadTemplateFromFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "in.tmpl")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	got, err := render.ReadTemplateForTest([]string{path}, bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("readTemplate() error = %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("readTemplate() = %q, want %q", string(got), "hello")
	}
}

func TestRunWritesFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	in := filepath.Join(dir, "in.tmpl")
	out := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(in, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := render.Run(context.Background(), stubEngine{}, render.Request{Input: render.Input{Templates: []string{in}}, Output: render.Output{Path: out}}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "stub:hello" {
		t.Fatalf("output = %q, want %q", string(got), "stub:hello")
	}
}
