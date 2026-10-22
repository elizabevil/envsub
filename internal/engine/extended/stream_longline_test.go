package extended_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/engine/extended"
)

// The streaming reader cuts its input at line boundaries, so a line longer
// than a window has to be carried across windows. These tests pin that the
// carry works: the reader must keep reading until it finds the line ending,
// and a line beyond the carry limit must be reported instead of spun on.
//
// Every test gives the render a deadline. A reader that spins on such a
// template never returns, so the deadline turns the spin into a failure
// instead of a hung test suite.

// longLineTemplate returns a template whose first line is lineBytes long,
// followed by ordinary short lines.
func longLineTemplate(lineBytes int) string {
	var sb strings.Builder
	sb.WriteString(strings.Repeat("x", lineBytes))
	sb.WriteString("\n")
	for i := range 100 {
		sb.WriteString("line")
		sb.WriteString(strings.Repeat(" ", i))
		sb.WriteString("=${NAME}\n")
	}
	return sb.String()
}

// streamWithDeadline streams a template and fails the test when the render did
// not finish on its own.
func streamWithDeadline(t *testing.T, source string, in engine.Input) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var out bytes.Buffer
	err := extended.New().StreamTo(ctx, &out, strings.NewReader(source), in)
	if ctx.Err() != nil && err != nil {
		t.Fatalf("StreamTo() did not finish: %v", err)
	}
	return out.String(), err
}

// TestStreamToLongLineTerminates pins that a line longer than a window does
// not make the reader spin.
//
// The window reader fills its buffer to the window size, then looks for the
// line ending that would let it hand the window over. A line that is longer
// than the window has no such ending in it, and the reader used to give up
// reading there: it never grew the buffer again, so it never found the
// ending and never returned.
func TestStreamToLongLineTerminates(t *testing.T) {
	t.Parallel()
	// Longer than the sequential window (1 MiB).
	source := longLineTemplate(1<<20 + (1 << 19))

	tree := streamTree()
	in := engine.Input{Name: "longline.tmpl", Values: tree}

	want, err := extended.New().Render(context.Background(), engine.Input{
		Template: []byte(source), Values: tree, Name: "longline.tmpl",
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	got, err := streamWithDeadline(t, source, in)
	if err != nil {
		t.Fatalf("StreamTo() error = %v", err)
	}
	if got != string(want) {
		t.Fatalf("StreamTo() = %d bytes, want %d bytes; the outputs differ", len(got), len(want))
	}
}

// TestStreamToLongLineInLargeTemplateTerminates pins the same for the
// parallel path, whose measurement pass reads the template in smaller windows
// (256 KiB) than the sequential one.
//
// A file long enough to be split is measured before it is rendered, and the
// measurement reads it in 256 KiB windows. A line longer than that window
// therefore has to survive the measurement pass too, not only the render.
func TestStreamToLongLineInLargeTemplateTerminates(t *testing.T) {
	t.Parallel()
	// Longer than the parallel measurement window (256 KiB), and the whole
	// template is longer than that window as well, so the parallel path is
	// the one under test.
	source := longLineTemplate(256<<10 + (1 << 18))

	tree := streamTree()
	in := engine.Input{
		Name:            "longline.tmpl",
		Values:          tree,
		SourceSize:      int64(len(source)),
		SourceSizeKnown: true,
	}

	want, err := extended.New().Render(context.Background(), engine.Input{
		Template: []byte(source), Values: tree, Name: "longline.tmpl",
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	got, err := streamWithDeadline(t, source, in)
	if err != nil {
		t.Fatalf("StreamTo() error = %v", err)
	}
	if got != string(want) {
		t.Fatalf("StreamTo() = %d bytes, want %d bytes; the outputs differ", len(got), len(want))
	}
}

// TestStreamToRejectsLineLongerThanCarryLimit pins that the carry limit is
// reachable: a line beyond it is reported, not spun on.
//
// The limit exists so a pathologically long line cannot be held in memory.
// It was unreachable, because the reader stopped growing its buffer before it
// ever reached the limit.
func TestStreamToRejectsLineLongerThanCarryLimit(t *testing.T) {
	t.Parallel()
	source := longLineTemplate(5 << 20)

	_, err := streamWithDeadline(t, source, engine.Input{Name: "carry.tmpl", Values: streamTree()})
	if err == nil {
		t.Fatal("StreamTo() error = nil, want the carry limit to be reported")
	}
	if !strings.Contains(err.Error(), "single line longer than") {
		t.Fatalf("StreamTo() error = %q, want it to report the carry limit", err.Error())
	}
}

// TestRenderAcceptsLongLine pins that the buffered renderer is not affected:
// the same template the streaming reader spins on renders fine in memory.
//
// The two paths read the template differently, so a template that only one of
// them can handle is a divergence, not a limitation of the template.
func TestRenderAcceptsLongLine(t *testing.T) {
	t.Parallel()
	source := longLineTemplate(5 << 20)

	out, err := extended.New().Render(context.Background(), engine.Input{
		Template: []byte(source), Values: streamTree(), Name: "longline.tmpl",
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	want := strings.ReplaceAll(source, "${NAME}", "world")
	if string(out) != want {
		t.Fatalf("Render() = %d bytes, want %d bytes; the outputs differ", len(out), len(want))
	}
}
