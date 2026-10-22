// Package cli defines the envsub command tree and its flags.
package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
		// The root command takes no arguments of its own: whatever follows the
		// program name has to name a subcommand. Setting Args also keeps Cobra
		// from producing its own unclassified error for an unknown name.
		Args: unknownCommand,
		// Cobra's own suggestion list uses a distance of two when the field is
		// left at zero, so the list here matches the one Cobra would print.
		SuggestionsMinimumDistance: 2,
		// The root command has no work of its own, but it has to be runnable:
		// Cobra checks that before it validates the arguments, and a
		// non-runnable root would print the help and exit 0 for any name.
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
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

// unknownCommand reports a name that does not match a subcommand as a usage
// error.
//
// Cobra's own error for this carries no diag kind, so it would fall back to
// the internal exit code. A name the user mistyped is a usage problem, and
// scripts branch on the usage code, so the error is classified as one here.
// The wording and the "Did you mean this?" list match Cobra's own.
func unknownCommand(c *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	var message strings.Builder
	fmt.Fprintf(&message, "unknown command %q for %q", args[0], c.CommandPath())
	if suggestions := c.SuggestionsFor(args[0]); len(suggestions) > 0 {
		message.WriteString("\n\nDid you mean this?\n")
		for _, suggestion := range suggestions {
			fmt.Fprintf(&message, "\t%v\n", suggestion)
		}
	}
	return diag.Wrap(diag.KindUsage, errors.New(message.String()))
}
