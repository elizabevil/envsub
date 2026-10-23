package cli

import (
	"context"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/elizabevil/envsub/internal/diag"
)

const (
	commandRender  = "render"
	commandVersion = "version"

	flagLogLevel    = "log-level"
	flagErrorFormat = "error-format"
	flagJSONL       = "jsonl"
	// FlagErrorFormat is the name of the persistent error-format flag; the
	// assembler reads it back after Execute to format the failure.
	FlagErrorFormat      = flagErrorFormat
	flagValuesFile       = "values-file"
	flagDotenvFile       = "dotenv-file"
	flagNoProcessEnv     = "no-process-env"
	flagProcessEnvPrefix = "process-env-prefix"
	flagSchema           = "schema"
	flagSet              = "set"
	flagSetJSON          = "set-json"
	flagSetFile          = "set-file"
	flagOutput           = "output"
	flagInPlace          = "in-place"
	flagOutputDir        = "output-dir"
	flagRecursive        = "recursive"
	flagOnMissing        = "on-missing"
	flagOnEmpty          = "on-empty"
	flagUnknownKeywords  = "unknown-keywords"
	flagOnlyVars         = "only-vars"
	flagStripSuffix      = "strip-suffix"
	flagIgnoreVars       = "ignore-vars"
	flagCheck            = "check"
	flagReport           = "report"
	flagValidateOnly     = "validate-only"
	flagExplain          = "explain"
)

// Error output formats accepted by --error-format.
const (
	ErrorFormatText = "text"
	ErrorFormatJSON = "json"
)

// ErrorFormatFrom reads the parsed --error-format value from an executed
// command. It returns the text format when the flag is absent, so the assembler
// has one code path.
func ErrorFormatFrom(cmd *cobra.Command) string {
	flag := cmd.Flags().Lookup(FlagErrorFormat)
	if flag == nil {
		return ErrorFormatText
	}
	return flag.Value.String()
}

// JSONLFrom reports whether stderr must be a JSONL event stream. It is true
// when --jsonl was given, or when a machine report (--explain, --report) selected
// it implicitly. An explicit --jsonl=false wins over the implicit selection, so
// a caller can still get human-readable errors with --explain.
func JSONLFrom(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	if ctx := cmd.Context(); ctx != nil {
		if enabled, ok := ctx.Value(jsonlContextKey{}).(bool); ok {
			return enabled
		}
	}
	if root := cmd.Root(); root != nil {
		if ctx := root.Context(); ctx != nil {
			if enabled, ok := ctx.Value(jsonlContextKey{}).(bool); ok {
				return enabled
			}
		}
	}
	flag := cmd.Root().PersistentFlags().Lookup(flagJSONL)
	requested := flag != nil && flag.Value.String() == "true"
	if flag != nil && flag.Changed {
		return requested
	}
	// --error-format=json is the legacy spelling of machine mode. Treat it as
	// JSONL so there is one error shape, not a flat object alongside the event
	// envelopes.
	if legacy := cmd.Root().PersistentFlags().Lookup(flagErrorFormat); legacy != nil &&
		legacy.Changed && legacy.Value.String() == ErrorFormatJSON {
		return true
	}
	return requested || machineReport(cmd)
}

// jsonlContextKey carries the effective mode from the pre-run hook to the
// post-execution error report. The root command's context is set too, because
// app.Run holds the root command after Execute returns.
type jsonlContextKey struct{}

// WithJSONL records the effective mode on a context.
func WithJSONL(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, jsonlContextKey{}, enabled)
}

// JSONLFlagFromArgs pre-scans the command line for an explicit --jsonl before
// Cobra parses flags. It is only a fallback for the case where a flag error
// aborts parsing before the flag object exists: an explicit --jsonl must still
// produce a JSONL error instead of a text line. Auto-selection by
// --explain/--report is deliberately not inferred here; that needs the parsed
// command.
func JSONLFlagFromArgs(args []string) (enabled, present bool) {
	for _, arg := range args {
		if arg == "--" {
			break
		}
		switch {
		case arg == "--jsonl":
			enabled, present = true, true
		case strings.HasPrefix(arg, "--jsonl="):
			value, err := strconv.ParseBool(strings.TrimPrefix(arg, "--jsonl="))
			if err != nil {
				continue
			}
			enabled, present = value, true
		}
	}
	return enabled, present
}

// machineReport reports whether the executed command asked for a report that is
// consumed by a program, which makes JSONL the only sensible stderr format.
func machineReport(cmd *cobra.Command) bool {
	if flag := cmd.Flags().Lookup(flagExplain); flag != nil && flag.Changed {
		return true
	}
	if flag := cmd.Flags().Lookup(flagReport); flag != nil && flag.Value.String() != "" {
		return true
	}
	return false
}

// ValidateErrorFormat rejects an unknown --error-format value.
func ValidateErrorFormat(format string) error {
	switch format {
	case "", ErrorFormatText, ErrorFormatJSON:
		return nil
	}
	return diag.New(diag.KindUsage, "invalid --error-format value %q: want text or json", format)
}
