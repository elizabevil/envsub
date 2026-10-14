package cli

import (
	"github.com/spf13/cobra"

	"github.com/elizabevil/envsub/internal/diag"
)

const (
	commandRender  = "render"
	commandVersion = "version"

	flagLogLevel    = "log-level"
	flagErrorFormat = "error-format"
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

// ValidateErrorFormat rejects an unknown --error-format value.
func ValidateErrorFormat(format string) error {
	switch format {
	case "", ErrorFormatText, ErrorFormatJSON:
		return nil
	}
	return diag.New(diag.KindUsage, "invalid --error-format value %q: want text or json", format)
}
