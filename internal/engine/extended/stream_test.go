package extended_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/engine/extended"
	"github.com/elizabevil/envsub/internal/values"
)

// streamTree returns a small value tree used by the streaming tests.
func streamTree() *values.Tree {
	tree := values.NewTree()
	tree.Root["app"] = map[string]any{"port": 8080}
	tree.Root["hosts"] = []any{"a", "b"}
	tree.Root["NAME"] = "world"
	return tree
}

// TestStreamToMatchesRender pins the windowed path to the buffered renderer.
func TestStreamToMatchesRender(t *testing.T) {
	t.Parallel()
	source := "name=$NAME port=${app.port} hosts=${hosts | join \",\"}\n" +
		"esc=$$ dollar=$1 done=${app.port | type int | quote}\n"

	tree := streamTree()
	in := engine.Input{Template: []byte(source), Values: tree, Name: "small.tmpl"}

	want, err := extended.New().Render(context.Background(), in)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	var out bytes.Buffer
	if err := extended.New().StreamTo(context.Background(), &out, strings.NewReader(source), in); err != nil {
		t.Fatalf("StreamTo() error = %v", err)
	}
	if out.String() != string(want) {
		t.Fatalf("StreamTo() = %q, want %q", out.String(), want)
	}
}

// TestStreamToSpansWindows renders a template several windows long, with no
// trailing newline, and checks the full output.
func TestStreamToSpansWindows(t *testing.T) {
	t.Parallel()
	const lines = 150000
	var sb strings.Builder
	for range lines {
		sb.WriteString("host=${NAME} pad\n")
	}
	sb.WriteString("tail=${app.port}")
	source := sb.String()

	tree := streamTree()
	var out bytes.Buffer
	ctx := t.Context()
	if err := extended.New().StreamTo(ctx, &out, strings.NewReader(source), engine.Input{Name: "big.tmpl", Values: tree}); err != nil {
		t.Fatalf("StreamTo() error = %v", err)
	}

	want := strings.ReplaceAll(source, "${NAME}", "world")
	want = strings.ReplaceAll(want, "${app.port}", "8080")
	if out.String() != want {
		t.Fatalf("StreamTo() = %d bytes, want %d bytes", out.Len(), len(want))
	}
}

// TestStreamToReportsTemplateLine checks that a scanner error found in a later
// window carries the template name and the absolute line number.
func TestStreamToReportsTemplateLine(t *testing.T) {
	t.Parallel()
	var sb strings.Builder
	for range 100000 {
		sb.WriteString("ok=${NAME}\n")
	}
	sb.WriteString("bad=${NAME\n")
	source := sb.String()

	var out bytes.Buffer
	err := extended.New().StreamTo(context.Background(), &out, strings.NewReader(source), engine.Input{Name: "lines.tmpl", Values: streamTree()})
	if err == nil {
		t.Fatal("StreamTo() error = nil, want error")
	}
	message := err.Error()
	for _, want := range []string{"lines.tmpl:100001", "unclosed variable expression"} {
		if !strings.Contains(message, want) {
			t.Fatalf("StreamTo() error = %q, want it to contain %q", message, want)
		}
	}
}
