package render

import (
	"io"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/utils"
)

// ReportUnused is the --report value that lists value-file entries no template
// referenced.
const ReportUnused = "unused"

// RenderEngine is what the pipeline needs from a template engine: its name. The
// buffered, streaming and path capabilities are discovered by type assertion on
// the concrete value, so a test stand-in only has to provide Name.
type RenderEngine interface {
	Name() string
}

// Request is one render request, grouped by concern.
type Request struct {
	Input    Input
	Values   Values
	Output   Output
	Policies Policies
	Explain  Explain
	// Report selects an extra report about the render.
	Report       Report
	ValidateOnly bool
}

// Input identifies the templates to render.
type Input struct {
	// Templates are paths, globs, directories or -.
	Templates []string
	// Recursive walks template directories recursively.
	Recursive bool
}

// Values identifies the value sources, in application order.
type Values struct {
	// Files are YAML/JSON value files, merged in argument order.
	Files []string
	// DotenvFiles are .env files, the layer above Files.
	DotenvFiles []string
	// NoProcessEnv leaves the @process namespace empty.
	NoProcessEnv bool
	// ProcessEnvPrefix whitelists process variables by prefix in the @process
	// namespace. It never filters .env files.
	ProcessEnvPrefix string
	// Schema is the JSON Schema file; empty for none.
	Schema string
	// Sets are --set KEY=VALUE assignments, stored as strings and typed later by
	// the schema or an inline | type.
	Sets []string
	// SetJSON are --set-json KEY=JSON assignments.
	SetJSON []string
	// SetFiles are --set-file KEY=FILE assignments: the value is the file's
	// bytes, kept as a string.
	SetFiles []string
}

// Output identifies the destination of the rendered bytes.
type Output struct {
	// Path is a single output path; empty or - means stdout.
	Path string
	// InPlace rewrites each template atomically.
	InPlace bool
	// Dir is the batch output directory, mirroring the input layout.
	Dir string
	// Check compares every rendered result with its existing target instead of
	// writing it, and reports the outdated ones. It needs a target: --output,
	// --in-place or --output-dir.
	Check bool
}

// Policies tunes engine behavior.
type Policies struct {
	OnMissing       string
	OnEmpty         string
	UnknownKeywords string
	// Only and Ignore limit which paths are substituted.
	Only   []string
	Ignore []string
}

// Explain configures the --explain report: a JSON object per referenced variable
// naming where its value came from. It never changes the output and never prints
// a value, because values may be secrets.
type Explain struct {
	Enabled bool
	// Paths limits the report to these exact paths; empty reports every
	// referenced variable.
	Paths []string
	// Writer receives the report; nil means os.Stderr.
	Writer io.Writer
}

// Active reports whether an explain report should be produced. Listing paths
// implies enabling the report.
func (e Explain) Active() bool {
	return e.Enabled || len(e.Paths) > 0
}

// Report selects an extra report about the render, written to Writer.
type Report struct {
	// Name is the report the user asked for: empty or unused.
	Name string
	// Writer receives the report; nil means os.Stderr.
	Writer io.Writer
}

// Unused reports whether the unused-value report was selected.
func (r Report) Unused() bool { return r.Name == ReportUnused }

// Validate rejects an unknown report name.
func (r Report) Validate() error { return ValidateReportName(r.Name) }

// replaces reports whether a variable path is substituted under the Only and
// Ignore filters.
func (p Policies) replaces(path string) bool {
	return engine.Options{Only: p.Only, Ignore: p.Ignore}.Replaces(path)
}

// Validate rejects unsupported option combinations before any IO.
func (req Request) Validate() error {
	switch {
	case req.Output.InPlace && req.Output.Path != "":
		return diag.New(diag.KindUsage, "--in-place cannot be combined with --output")
	case req.Output.InPlace && req.Output.Dir != "":
		return diag.New(diag.KindUsage, "--in-place cannot be combined with --output-dir")
	case req.Output.Dir != "" && req.Output.Path != "":
		return diag.New(diag.KindUsage, "--output-dir cannot be combined with --output")
	case req.ValidateOnly && (req.Output.InPlace || req.Output.Path != "" || req.Output.Dir != ""):
		return diag.New(diag.KindUsage, "--validate-only cannot be combined with an output option")
	case req.Output.Check && req.ValidateOnly:
		return diag.New(diag.KindUsage, "--check cannot be combined with --validate-only")
	case req.Output.Check && !req.Output.InPlace && req.Output.Path == "" && req.Output.Dir == "":
		return diag.New(diag.KindUsage, "--check needs an output target: --output, --in-place or --output-dir")
	case !utils.IsStdio(req.Output.Path) && len(req.Input.Templates) > 1:
		return diag.New(diag.KindUsage, "--output accepts a single template, use --output-dir for several")
	}
	switch req.Policies.OnMissing {
	case "", engine.UnsetError, engine.UnsetEmpty, engine.UnsetKeep, engine.UnsetZero:
	default:
		return diag.New(diag.KindUsage,
			"invalid --on-missing value %q: want error, empty, keep or zero", req.Policies.OnMissing)
	}
	switch req.Policies.OnEmpty {
	case "", engine.EmptyOK, engine.EmptyError:
	default:
		return diag.New(diag.KindUsage, "invalid --on-empty value %q: want ok or error", req.Policies.OnEmpty)
	}
	if req.Values.NoProcessEnv && req.Values.ProcessEnvPrefix != "" {
		return diag.New(diag.KindUsage, "--no-process-env and --process-env-prefix are mutually exclusive")
	}
	if err := req.Report.Validate(); err != nil {
		return err
	}
	return nil
}

// ValidateReportName rejects an unknown --report value.
func ValidateReportName(name string) error {
	switch name {
	case "", ReportUnused:
		return nil
	}
	return diag.New(diag.KindUsage, "invalid --report value %q: want unused", name)
}
