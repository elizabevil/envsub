package render

import (
	"context"
	"io"
	"testing"

	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/output"
)

// stubEngine implements every optional capability, so a test can prove that the
// plan, not the engine, decides which ones a request may use.
type stubEngine struct{}

func (stubEngine) Name() string { return "stub" }

func (stubEngine) Render(context.Context, engine.Input) ([]byte, error) { return nil, nil }

func (stubEngine) WriteTo(context.Context, io.Writer, engine.Input) error { return nil }

func (stubEngine) StreamTo(context.Context, io.Writer, io.Reader, engine.Input) error { return nil }

// TestNegotiateCapsTurnsStreamingOffForExplain pins the one place that decides
// whether the streaming paths may run: --explain needs the tree a render leaves
// behind, so it forces the buffered path instead of streaming.
func TestNegotiateCapsTurnsStreamingOffForExplain(t *testing.T) {
	t.Parallel()
	plain := negotiateCaps(stubEngine{}, Request{})
	if !plain.canStream || !plain.canStreamReader {
		t.Fatalf("negotiateCaps(no explain) = %+v, want streaming enabled", plain)
	}
	explained := negotiateCaps(stubEngine{}, Request{Explain: Explain{Enabled: true}})
	if explained.canStream || explained.canStreamReader {
		t.Fatalf("negotiateCaps(explain) = %+v, want streaming disabled", explained)
	}
	if explained.stream == nil || explained.reader == nil {
		t.Fatalf("negotiateCaps(explain) = %+v, want the capabilities kept but gated", explained)
	}
}

// TestBuildPlanLeavesComparisonModesUnstreamed pins that --check and
// --validate-only never stream: both compare or discard instead of writing, which
// the buffered writer owns.
func TestBuildPlanLeavesComparisonModesUnstreamed(t *testing.T) {
	t.Parallel()
	templates := []template{{Input: "a.tmpl", Output: "out/a"}}
	for _, tc := range []struct {
		name string
		req  Request
	}{
		{name: "check", req: Request{Output: Output{Check: true, Dir: "out"}}},
		{name: "validate only", req: Request{ValidateOnly: true, Output: Output{Dir: "out"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.req.buildPlan(stubEngine{}, templates)
			if err != nil {
				t.Fatalf("buildPlan() error = %v", err)
			}
			if len(got.streams) != 1 || got.streams[0] != nil {
				t.Fatalf("buildPlan() streams = %#v, want one nil target", got.streams)
			}
		})
	}
}

// TestBuildPlanSelectsOneWriterPerMode pins the writer the plan hands the loop: a
// check writer for --check, the stdout writer for the default destination, and one
// destination per template that shares that writer.
func TestBuildPlanSelectsOneWriterPerMode(t *testing.T) {
	t.Parallel()
	templates := []template{{Input: "a.tmpl"}, {Input: "b.tmpl"}}

	checked, err := Request{Output: Output{Check: true, Dir: "out"}}.buildPlan(stubEngine{}, templates)
	if err != nil {
		t.Fatalf("buildPlan(check) error = %v", err)
	}
	if _, ok := checked.writer.(*output.CheckWriter); !ok {
		t.Fatalf("buildPlan(check) writer = %T, want *output.CheckWriter", checked.writer)
	}

	stdout, err := Request{}.buildPlan(stubEngine{}, templates)
	if err != nil {
		t.Fatalf("buildPlan(default) error = %v", err)
	}
	if _, ok := stdout.writer.(*output.StdoutWriter); !ok {
		t.Fatalf("buildPlan(default) writer = %T, want *output.StdoutWriter", stdout.writer)
	}
	for i := range templates {
		if dst := stdout.destination(i); dst.writer != stdout.writer {
			t.Fatalf("destination(%d).writer = %T, want the plan writer", i, dst.writer)
		}
	}
}
