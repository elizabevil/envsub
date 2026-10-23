package cli

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/event"
	"github.com/elizabevil/envsub/internal/render"
)

// explainAll is the sentinel a bare --explain appends, meaning "every
// referenced variable". No template path can be spelled "*".
const explainAll = "*"

// renderFlags mirrors the render command line. It is grouped exactly like
// render.Request so the mapping stays obvious; the CLI owns parsing only, and
// render owns validation and execution.
type renderFlags struct {
	input        inputFlags
	values       valueFlags
	output       outputFlags
	policies     policyFlags
	explain      explainFlags
	report       string
	check        bool
	validateOnly bool
}

// inputFlags selects the templates.
type inputFlags struct {
	recursive bool
}

// valueFlags selects the value sources.
type valueFlags struct {
	files            []string
	dotenvFiles      []string
	noProcessEnv     bool
	processEnvPrefix string
	schema           string
	sets             []string
	setJSON          []string
	setFiles         []string
}

// outputFlags selects the destination.
type outputFlags struct {
	path          string
	inPlace       bool
	dir           string
	stripSuffixes []string
}

// policyFlags tunes engine behavior.
type policyFlags struct {
	onMissing       string
	onEmpty         string
	unknownKeywords string
	onlyVars        []string
	ignoreVars      []string
}

// explainFlags holds the optional-value --explain flag. A bare --explain
// appends explainAll; each --explain=PATH appends one exact path.
type explainFlags struct {
	paths []string
}

// request assembles the parsed flags and the positional template arguments into
// a render request. It performs no validation: render.Request does that, so the
// CLI and any other caller share one rule set.
func (f renderFlags) request(args []string, stderr io.Writer, events *event.Writer, jsonl bool) render.Request {
	req := render.Request{
		Input: render.Input{Templates: args, Recursive: f.input.recursive},
		Values: render.Values{
			Files:            f.values.files,
			DotenvFiles:      f.values.dotenvFiles,
			NoProcessEnv:     f.values.noProcessEnv,
			ProcessEnvPrefix: f.values.processEnvPrefix,
			Schema:           f.values.schema,
			Sets:             f.values.sets,
			SetJSON:          f.values.setJSON,
			SetFiles:         f.values.setFiles,
		},
		Output: render.Output{
			Path:          f.output.path,
			InPlace:       f.output.inPlace,
			Dir:           f.output.dir,
			Check:         f.check,
			StripSuffixes: f.output.stripSuffixes,
		},
		Policies: render.Policies{
			OnMissing:       f.policies.onMissing,
			OnEmpty:         f.policies.onEmpty,
			UnknownKeywords: f.policies.unknownKeywords,
			Only:            f.policies.onlyVars,
			Ignore:          f.policies.ignoreVars,
		},
		Explain:      render.Explain{},
		Report:       render.Report{Name: f.report, Writer: stderr, JSONL: jsonl},
		Events:       events,
		ValidateOnly: f.validateOnly,
	}
	if paths := f.explain.paths; len(paths) > 0 {
		req.Explain.Enabled = true
		for _, path := range paths {
			if path == explainAll {
				// A bare --explain reports everything, whatever else was listed.
				req.Explain.Paths = nil
				break
			}
			req.Explain.Paths = append(req.Explain.Paths, path)
		}
	}
	return req
}

func newRenderCommand(eng RenderEngine, run Runner) *cobra.Command {
	flags := renderFlags{}
	cmd := &cobra.Command{
		Use:   commandRender + " [template...]",
		Short: "Render templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			events := event.FromContext(cmd.Context())
			return run(cmd.Context(), eng, flags.request(args, cmd.ErrOrStderr(), events, JSONLFrom(cmd)))
		},
	}

	f := cmd.Flags()
	f.StringArrayVarP(&flags.values.files, flagValuesFile, "f", nil, "YAML/JSON values files (`FILE`, repeatable)")
	f.StringArrayVar(&flags.values.dotenvFiles, flagDotenvFile, nil, "dotenv files, parsed as dotenv whatever the name (`FILE`, repeatable; outranks --values-file)")
	f.BoolVar(&flags.values.noProcessEnv, flagNoProcessEnv, false, "ignore the process environment")
	f.StringVar(&flags.values.processEnvPrefix, flagProcessEnvPrefix, "", "only expose process variables with this `PREFIX` (whitelist; .env files are unaffected)")
	f.StringVar(&flags.values.schema, flagSchema, "", "JSON Schema file (`FILE`)")
	f.StringArrayVar(&flags.values.sets, flagSet, nil, "set a raw string value (`KEY=VALUE`, typed by the schema or an inline | type)")
	f.StringArrayVar(&flags.values.setJSON, flagSetJSON, nil, "set a native JSON value (`KEY=JSON`)")
	f.StringArrayVar(&flags.values.setFiles, flagSetFile, nil, "set a value from a file, kept as a string (`KEY=FILE`)")
	f.StringVarP(&flags.output.path, flagOutput, "o", "", "output file (`FILE`)")
	f.BoolVarP(&flags.output.inPlace, flagInPlace, "i", false, "write back in place")
	f.StringVar(&flags.output.dir, flagOutputDir, "", "output directory (`DIR`)")
	f.StringArrayVar(&flags.output.stripSuffixes, flagStripSuffix, nil, "drop this file-name suffix from --output-dir targets (`EXT`, repeatable; longest match wins)")
	f.BoolVar(&flags.input.recursive, flagRecursive, false, "process templates recursively")
	// The policy flags name the condition they act on: --on-missing applies when
	// an expression has no value, --on-empty when the value is empty.
	f.StringVar(&flags.policies.onMissing, flagOnMissing, engine.UnsetError, "policy for a missing value (`POLICY`: error|empty|keep|zero)")
	f.StringVar(&flags.policies.onEmpty, flagOnEmpty, engine.EmptyOK, "policy for an empty value (`POLICY`: ok|error)")
	f.StringVar(&flags.policies.unknownKeywords, flagUnknownKeywords, engine.SchemaUnknownIgnore, "policy for unknown schema keywords (`POLICY`: ignore|warn|error)")
	f.StringArrayVar(&flags.policies.onlyVars, flagOnlyVars, nil, "only substitute these variables (`VAR`, repeatable)")
	f.StringArrayVar(&flags.policies.ignoreVars, flagIgnoreVars, nil, "never substitute these variables (`VAR`, repeatable)")
	f.StringVar(&flags.report, flagReport, "", "extra `REPORT` written to stderr: unused (value-file entries no template referenced)")
	f.BoolVar(&flags.check, flagCheck, false, "compare output with the existing target and report outdated targets (exit 7)")
	f.BoolVar(&flags.validateOnly, flagValidateOnly, false, "validate without writing output")
	f.StringArrayVar(&flags.explain.paths, flagExplain, nil,
		"print a JSON source report to stderr; `PATH` limits it to exact paths (repeatable, default all)")
	// A bare --explain appends the sentinel, so no value is required.
	f.Lookup(flagExplain).NoOptDefVal = explainAll

	return cmd
}
