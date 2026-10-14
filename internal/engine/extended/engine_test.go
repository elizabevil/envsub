package extended_test

import (
	"context"
	"testing"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/engine/extended"
	"github.com/elizabevil/envsub/internal/values"
)

func renderTemplate(t *testing.T, template string, tree *values.Tree) (string, error) {
	t.Helper()
	out, err := extended.New().Render(context.Background(), engine.Input{
		Template: []byte(template),
		Values:   tree,
		Options:  engine.Options{Unset: "error"},
	})
	return string(out), err
}

// TestRenderBareNameIgnoresProcessEnv pins the resolution contract on the buffered
// path: a host variable must not fill a bare name, so ${NAME} fails unless the root
// defines it (the @process namespace is the only way to read it).
func TestRenderBareNameIgnoresProcessEnv(t *testing.T) {
	t.Parallel()
	tree := values.NewTree()
	tree.Env["NAME"] = "world"
	if _, err := renderTemplate(t, "hello $NAME", tree); err == nil {
		t.Fatal("Render() error = nil, want the missing-name policy to fire")
	} else if got := diag.ExitCode(err); got != 6 {
		t.Fatalf("ExitCode() = %d, want 6; err=%v", got, err)
	}
	if got, err := renderTemplate(t, "hello ${@process.NAME}", tree); err != nil || got != "hello world" {
		t.Fatalf("Render() with @process = (%q, %v), want (hello world, nil)", got, err)
	}
}

func TestRenderRootPath(t *testing.T) {
	t.Parallel()
	tree := values.NewTree()
	tree.Root["app"] = map[string]any{"port": 8080}
	got, err := renderTemplate(t, "port=${app.port}", tree)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if got != "port=8080" {
		t.Fatalf("Render() = %q, want %q", got, "port=8080")
	}
}

func TestRenderMissingVariable(t *testing.T) {
	t.Parallel()
	_, err := renderTemplate(t, "hello $NAME", values.NewTree())
	if err == nil {
		t.Fatal("Render() error = nil, want missing variable error")
	}
	if got, want := diag.ExitCode(err), 6; got != want {
		t.Fatalf("ExitCode() = %d, want %d", got, want)
	}
}

func TestRenderStructuredValueNeedsFormatter(t *testing.T) {
	t.Parallel()
	tree := values.NewTree()
	tree.Root["hosts"] = []any{"a", "b"}
	_, err := renderTemplate(t, "${hosts}", tree)
	if err == nil {
		t.Fatal("Render() error = nil, want type error")
	}
	if got, want := diag.ExitCode(err), 4; got != want {
		t.Fatalf("ExitCode() = %d, want %d", got, want)
	}
}
