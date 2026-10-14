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
	"github.com/elizabevil/envsub/internal/render"
)

// Run executes the CLI and returns the process exit code.
func Run() int {
	cmd := cli.NewRootCommand(extended.New(), runRender)
	err := cmd.Execute()
	if err == nil {
		return 0
	}
	report(err, cli.ErrorFormatFrom(cmd))
	return diag.ExitCode(err)
}

// report writes err in the requested format. JSON is one object per failure, so
// a caller can parse stderr without depending on message wording.
func report(err error, format string) {
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

// runRender executes one parsed render request with the engine the command was
// built with. The CLI never calls the render pipeline itself.
func runRender(ctx context.Context, eng cli.RenderEngine, req render.Request) error {
	return render.Run(ctx, eng, req)
}
