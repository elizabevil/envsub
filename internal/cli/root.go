// Package cli defines the envsub command tree and its flags.
package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/logging"
	"github.com/elizabevil/envsub/internal/render"
)

// RenderEngine is the template engine the render command drives. The CLI only
// needs the identity of the engine it is handed; the concrete implementation is
// wired by internal/app.
type RenderEngine interface {
	Name() string
}

// Runner executes one render request. The CLI parses flags and builds the
// request; who executes it (and with which engine) is the assembler's decision.
type Runner func(ctx context.Context, eng RenderEngine, req render.Request) error

// NewRootCommand builds the envsub command tree around one template engine and
// the runner that executes a parsed request.
func NewRootCommand(eng RenderEngine, run Runner) *cobra.Command {
	var logLevel string
	var errorFormat string
	cmd := &cobra.Command{
		Use:           "envsub",
		Short:         "Extended configuration renderer",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// The error format is checked before anything else runs, so an
			// invalid value is reported in the requested (default) format
			// rather than producing an unparseable error stream.
			if err := ValidateErrorFormat(errorFormat); err != nil {
				return err
			}
			logger, err := logging.New(logLevel, cmd.ErrOrStderr())
			if err != nil {
				return diag.Wrap(diag.KindUsage, err)
			}
			cmd.SetContext(logging.WithLogger(cmd.Context(), logger))
			return nil
		},
	}
	cmd.PersistentFlags().StringVar(&logLevel, flagLogLevel, logging.LevelWarnName, "log level `LEVEL`: debug|info|warn|error")
	cmd.PersistentFlags().StringVar(&errorFormat, flagErrorFormat, ErrorFormatText, "error output `FORMAT`: text|json")
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return diag.Wrap(diag.KindUsage, err)
	})
	cmd.AddCommand(newRenderCommand(eng, run))
	cmd.AddCommand(newVersionCommand())
	return cmd
}
