package cli_test

import (
	"context"

	"github.com/elizabevil/envsub/internal/cli"
	"github.com/elizabevil/envsub/internal/render"
)

// stubEngine satisfies the CLI's engine view: the command tree only needs the
// engine's identity for help output and flag errors, and the concrete engine is
// supplied by internal/app.
type stubEngine struct{}

func (stubEngine) Name() string { return "stub" }

// stubRunner records the request the command tree produced, so a test can assert
// CLI wiring without rendering. Requests made through the stub never reach the
// pipeline; tests that need real rendering call render.Run directly.
type stubRunner struct {
	called  bool
	request render.Request
}

func (r *stubRunner) run(_ context.Context, _ cli.RenderEngine, req render.Request) error {
	r.called = true
	r.request = req
	return nil
}

// newStubRunner returns a runner whose captured request can be inspected.
func newStubRunner() (*stubRunner, cli.Runner) {
	stub := &stubRunner{}
	return stub, stub.run
}

// noopRunner accepts a request and renders nothing, for tests that only exercise
// the command tree (help, flags, version).
func noopRunner(context.Context, cli.RenderEngine, render.Request) error { return nil }
