// Package app assembles the process: it picks the template engine, hands the CLI
// a runner that executes a parsed request, and maps failures to exit codes.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/elizabevil/envsub/internal/cli"
	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/engine/extended"
	"github.com/elizabevil/envsub/internal/event"
	"github.com/elizabevil/envsub/internal/render"
)

// Run executes the CLI and returns the process exit code.
func Run() int {
	cmd := cli.NewRootCommand(extended.New(), runRender)
	// One writer owns the event stream: logs during the render, explain/unused
	// records after it, and the final error all share one encoder, clock and
	// sequence.
	events := event.NewWriter(os.Stderr)
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = event.WithWriter(ctx, events)
	// A flag error can abort parsing before Cobra sees --jsonl. An explicit flag
	// still has to produce a JSONL error, so seed the mode from the raw command
	// line; the parsed command overwrites it when parsing succeeds.
	if enabled, present := cli.JSONLFlagFromArgs(os.Args[1:]); present {
		ctx = cli.WithJSONL(ctx, enabled)
	}
	cmd.SetContext(ctx)

	err := cmd.Execute()
	if err == nil {
		return 0
	}
	report(err, events, cli.JSONLFrom(cmd), cli.ErrorFormatFrom(cmd))
	return diag.ExitCode(err)
}

// report writes err in the requested format. JSON is one object per failure, so
// a caller can parse stderr without depending on message wording. JSONL is the
// machine event stream: every joined failure gets its own typed line.
func report(err error, events *event.Writer, jsonl bool, format string) {
	if jsonl {
		reportJSONL(events, err)
		return
	}
	if format != cli.ErrorFormatJSON {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	encoded, marshalErr := json.Marshal(diag.NewReport(err))
	if marshalErr != nil {
		// The report is plain strings and an int; a marshal failure means the
		// error cannot be described, so fall back to text instead of emitting
		// invalid JSON.
		fmt.Fprintln(os.Stderr, err)
		return
	}
	fmt.Fprintln(os.Stderr, string(encoded))
}

// reportJSONL writes one error event per failure. errors.Join is flattened so a
// batch failure is several records rather than one object with embedded
// newlines, which is what a log pipeline wants.
func reportJSONL(events *event.Writer, err error) {
	for _, failure := range splitFailures(err) {
		report := diag.NewReport(failure)
		// The process is already failing; a write failure here has no better
		// channel, so keep going and emit the remaining failures.
		_ = events.Emit(event.TypeError, event.ErrorData{
			Kind:     string(report.Kind),
			ExitCode: report.ExitCode,
			Message:  report.Message,
		})
	}
}

// splitFailures flattens the multi-error tree errors.Join builds. A wrapped
// single cause is left alone: the wrapper is the user-facing failure and its
// message already includes the cause.
func splitFailures(err error) []error {
	if err == nil {
		return nil
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		var failures []error
		for _, cause := range joined.Unwrap() {
			failures = append(failures, splitFailures(cause)...)
		}
		return failures
	}
	return []error{err}
}

// runRender executes one parsed render request with the engine the command was
// built with. The CLI never calls the render pipeline itself.
func runRender(ctx context.Context, eng cli.RenderEngine, req render.Request) error {
	return render.Run(ctx, eng, req)
}
