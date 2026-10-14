// Package render orchestrates value loading, schema checks and template
// rendering for the CLI.
package render

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/load"
	"github.com/elizabevil/envsub/internal/logging"
	"github.com/elizabevil/envsub/internal/output"
	"github.com/elizabevil/envsub/internal/source"
	"github.com/elizabevil/envsub/internal/utils"
	"github.com/elizabevil/envsub/internal/values"
)

// Options and its grouping live in options.go.

const (
	logRenderStart    = "render start"
	logTemplateLoaded = "template loaded"
	logRenderComplete = "render complete"
	logRenderBatch    = "render batch"

	templateGlobChars = "*?["

	// stdinTemplate is the template path that reads the standard input.
	stdinTemplate = "-"
)

// Run loads the value sources, expands the template arguments and renders every
// template into its destination. The engine's optional streaming and path
// capabilities are discovered by type assertion.
func Run(ctx context.Context, eng RenderEngine, req Request) error {
	logger := logging.FromContext(ctx)
	logger.Debug(logRenderStart, "templates", len(req.Input.Templates), "output", req.Output.Path)

	if err := req.Validate(); err != nil {
		return err
	}

	// The loader reports each contributing source to the observer, which is how
	// --explain is populated. With the report off the observer records nothing.
	run := explainRun{}
	result, err := load.Load(ctx, req.loadSpec(), &run)
	if err != nil {
		return err
	}
	tree, sch := result.Tree, result.Schema
	run.tree = tree

	templates, err := req.resolveTemplates()
	if err != nil {
		return err
	}
	writer, err := req.newWriter()
	if err != nil {
		return err
	}
	// --explain and --report=unused both need the template text; read every
	// template once here and let both reuse it. A second reader would consume
	// stdin twice and render nothing.
	explainActive := req.Explain.Active()
	if explainActive || req.Report.Unused() {
		if err := req.readSources(ctx, templates); err != nil {
			return err
		}
	}
	var referenced rootNames
	if req.Report.Unused() {
		referenced = rootNames{}
		if err := req.collectReferences(eng, templates, referenced); err != nil {
			return err
		}
	}
	targets, err := req.newStreamTargets(templates)
	if err != nil {
		return err
	}

	in := engine.Input{
		Values: tree,
		Schema: sch,
		Options: engine.Options{
			Unset:         req.Policies.OnMissing,
			Empty:         req.Policies.OnEmpty,
			SchemaUnknown: req.Policies.UnknownKeywords,
			Only:          req.Policies.Only,
			Ignore:        req.Policies.Ignore,
		},
	}
	logger.Debug(logRenderBatch, "templates", len(templates))

	// The explain report runs after the loop, which needs the tree a render left
	// behind. Buffering the template and the output is the price of reporting
	// assignment origins; the streaming path cannot see them.
	streamer, canStream := eng.(engine.StreamEngine)
	reader, canStreamReader := eng.(engine.StreamReaderEngine)
	if explainActive {
		canStream, canStreamReader = false, false
	}
	rs := renderers{stream: streamer, canStream: canStream, reader: reader, canStreamReader: canStreamReader}
	var failed []error
	for i, tmpl := range templates {
		dst := destination{writer: writer, stream: targets[i]}
		if err := req.renderOne(ctx, eng, rs, tmpl, in, dst); err != nil {
			if len(templates) == 1 {
				if explainActive {
					if reportErr := req.reportSources(eng, templates, run); reportErr != nil {
						return reportErr
					}
				}
				return err
			}
			logger.Error("render failed", "template", tmpl.Input, "error", err)
			failed = append(failed, diag.At(err, tmpl.Input, 0))
			continue
		}
	}
	// The report is written even when a batch had failures: it explains what the
	// render did manage to resolve, which is what a reader needs to diagnose.
	if explainActive {
		if err := req.reportSources(eng, templates, run); err != nil {
			return err
		}
	}
	// A batch fails as a whole, but each failure keeps its own classification, so
	// a caller still sees a missing value as a missing value rather than a
	// generic internal error.
	if len(failed) > 0 {
		return errors.Join(failed...)
	}
	if checker, ok := writer.(*output.CheckWriter); ok {
		if outdated := checker.Outdated(); len(outdated) > 0 {
			return diag.New(diag.KindDiff, "%d of %d outputs are outdated: %s",
				len(outdated), checker.Checked(), strings.Join(outdated, ", "))
		}
	}
	if req.Report.Unused() {
		if err := req.reportUnused(tree, referenced); err != nil {
			return err
		}
	}
	return nil
}

// collectReferences records every variable path the templates reference, which
// is what --report=unused subtracts from the value tree. It fills templates[i].Text
// so a stdin template is read exactly once.
func (req Request) collectReferences(eng RenderEngine, templates []template, referenced rootNames) error {
	pathEngine, ok := eng.(engine.PathEngine)
	if !ok {
		return diag.New(diag.KindInternal, "--report is not supported by engine %q", eng.Name())
	}
	for i := range templates {
		data := templates[i].Text
		if data == nil {
			// A file template is read by the render itself; only stdin has to be
			// consumed here, because it cannot be read twice.
			if templates[i].Input != stdinTemplate {
				continue
			}
			read, err := readTemplate(templates[i].Input, req.stdin())
			if err != nil {
				return err
			}
			data = read
			templates[i].Text = data
		}
		paths, err := pathEngine.Paths(data)
		if err != nil {
			return diag.Wrap(diag.KindTemplate, fmt.Errorf("report %s: %w", templates[i].Input, err))
		}
		for _, path := range paths {
			// A path the Only/Ignore filters exclude is not substituted, so it
			// cannot make a value used.
			if !req.Policies.replaces(path) {
				continue
			}
			referenced[canonicalPath(path)] = true
		}
	}
	return nil
}

// reportUnused writes the value-file entries no template referenced.
func (req Request) reportUnused(tree *values.Tree, referenced rootNames) error {
	w := req.Report.Writer
	if w == nil {
		w = os.Stderr
	}
	return writeUnused(w, collectUnused(tree, referenced))
}

// rootNames is the set of canonical variable paths one run referenced.
type rootNames map[string]bool

// canonicalPath normalizes a template path to the spelling the value tree walk
// produces (values.Canonical), so the comparison is exact for keys that need
// quoting. A path the parser cannot read is kept verbatim rather than dropped.
func canonicalPath(path string) string {
	segments, err := values.ParsePath(strings.TrimSpace(path))
	if err != nil {
		return strings.TrimSpace(path)
	}
	return values.Canonical(segments)
}

// template pairs a template's input path with its resolved output path.
type template struct {
	// Input is the template path, or - for stdin.
	Input string
	// Output is the resolved destination path, empty for stdout.
	Output string
	// Text is the template source, filled only when --explain had to read it.
	// It keeps the render from reading the same template twice.
	Text []byte
}

// destination bundles the two output sinks of one template.
type destination struct {
	// writer accepts a fully rendered template.
	writer output.Writer
	// stream receives the output while it is rendered; nil when the engine
	// cannot stream to this destination.
	stream output.StreamWriter
}

// renderers groups the optional streaming capabilities of the selected engine.
type renderers struct {
	// stream is the engine's writer-facing API, if the engine supports it.
	stream engine.StreamEngine
	// canStream reports whether stream may be used.
	canStream bool
	// reader is the engine's reader-facing API, if the engine supports it.
	reader engine.StreamReaderEngine
	// canStreamReader reports whether reader may be used.
	canStreamReader bool
}

// renderOne renders one template into its destination, preferring the engine's
// streaming path so neither template nor output is buffered. It is unaware of
// --explain, whose report reportSources produced before the loop began.
func (req Request) renderOne(ctx context.Context, eng RenderEngine, rs renderers, tmpl template, in engine.Input, dst destination) error {
	logger := logging.FromContext(ctx)

	templateInput := source.TemplateInput(tmpl.Input)
	in.Name = tmpl.Input
	in.SourceSize, in.SourceSizeKnown = templateInput.Size()
	if len(tmpl.Text) > 0 {
		// --explain already read this template, and stdin cannot be rewound.
		in.Template = tmpl.Text
		logger.Debug(logTemplateLoaded, "template", tmpl.Input, "bytes", len(tmpl.Text))
		return req.renderBuffered(ctx, eng, rs, tmpl, in, dst)
	}

	// Stream whenever the engine supports it.
	if rs.canStreamReader && dst.stream != nil && (!req.ValidateOnly || rs.canStream) {
		if err := req.streamReader(ctx, rs, tmpl, templateInput, in, dst.stream); err != nil {
			return err
		}
		logger.Debug(logRenderComplete, "template", tmpl.Input)
		return nil
	}

	data, err := readTemplate(tmpl.Input, req.stdin())
	if err != nil {
		return err
	}
	logger.Debug(logTemplateLoaded, "template", tmpl.Input, "bytes", len(data))
	in.Template = data
	return req.renderBuffered(ctx, eng, rs, tmpl, in, dst)
}

// renderBuffered renders an in-memory template. Validation renders to a discard
// writer so the semantics stay identical without producing output.
func (req Request) renderBuffered(ctx context.Context, eng RenderEngine, rs renderers, tmpl template, in engine.Input, dst destination) error {
	logger := logging.FromContext(ctx)

	buffered, ok := eng.(engine.Engine)
	if req.ValidateOnly {
		// Validation only needs the template parsed and resolved; rendering to
		// a discard writer keeps the semantics without producing output.
		if rs.canStream {
			return rs.stream.WriteTo(ctx, io.Discard, in)
		}
		if !ok {
			return diag.New(diag.KindInternal, "engine %q cannot render", eng.Name())
		}
		_, err := buffered.Render(ctx, in)
		return err
	}

	if rs.canStream && dst.stream != nil {
		if err := streamToTarget(ctx, rs.stream, dst.stream, in); err != nil {
			return err
		}
		logger.Debug(logRenderComplete, "template", tmpl.Input, "bytes", len(in.Template))
		return nil
	}

	if !ok {
		return diag.New(diag.KindInternal, "engine %q cannot render", eng.Name())
	}
	out, err := buffered.Render(ctx, in)
	if err != nil {
		return err
	}
	if err := dst.writer.Write(ctx, tmpl.Output, out); err != nil {
		return diag.Wrap(diag.KindIO, err)
	}
	logger.Debug(logRenderComplete, "template", tmpl.Input, "bytes", len(out))
	return nil
}

// readSources reads every template once and keeps its text on the template. The
// read has to happen before rendering: stdin cannot be read twice, so a template
// that is also rendered must be read once and reused by --explain and by the
// reference scan behind --report=unused.
func (req Request) readSources(ctx context.Context, templates []template) error {
	for i := range templates {
		if templates[i].Text != nil {
			continue
		}
		data, err := readTemplate(templates[i].Input, req.stdin())
		if err != nil {
			return err
		}
		templates[i].Text = data
		logging.FromContext(ctx).Debug(logTemplateLoaded, "template", templates[i].Input, "bytes", len(data))
	}
	return nil
}

// reportSources writes the --explain report for every template. It runs after the
// render loop, because an assignment (${K:=v}) only publishes its value while the
// template renders and the report must show that value's lineage (ADR-005).
func (req Request) reportSources(eng RenderEngine, templates []template, explain explainRun) error {
	for i := range templates {
		if err := req.emitExplain(eng, templates[i].Input, templates[i].Text, explain); err != nil {
			return err
		}
	}
	return nil
}

// emitExplain writes the --explain report for one template. Explain.Writer
// defaults to stderr so stdout stays the pure render result.
func (req Request) emitExplain(eng RenderEngine, tmpl string, src []byte, explain explainRun) error {
	pathEngine, ok := eng.(engine.PathEngine)
	if !ok {
		return diag.New(diag.KindInternal, "--explain is not supported by engine %q", eng.Name())
	}
	w := req.Explain.Writer
	if w == nil {
		w = os.Stderr
	}
	return explainTemplate(w, pathEngine, req, tmpl, src, explain)
}

// streamReader streams a template from its input source into dst. A file is
// opened only for the duration of the render; stdin is consumed directly. With
// --validate-only the destination is discarded.
func (req Request) streamReader(ctx context.Context, rs renderers, tmpl template, templateInput source.Input, in engine.Input, dst output.StreamWriter) error {
	stream, err := templateInput.Open()
	if err != nil {
		return diag.Wrap(diag.KindIO, fmt.Errorf("read template %s: %w", tmpl.Input, err))
	}
	defer func() {
		if closeErr := stream.Close(); closeErr != nil {
			logging.FromContext(ctx).Warn("close template failed", "template", tmpl.Input, "error", closeErr)
		}
	}()

	destination := dst.Stream()
	if req.ValidateOnly {
		destination = io.Discard
	}
	if err := rs.reader.StreamTo(ctx, destination, stream, in); err != nil {
		if discardErr := dst.Discard(); discardErr != nil {
			logging.FromContext(ctx).Error("discard failed output", "template", tmpl.Input, "error", discardErr)
		}
		return err
	}
	if err := dst.Commit(); err != nil {
		return diag.Wrap(diag.KindIO, err)
	}
	return nil
}

// streamToTarget streams an in-memory template into the destination.
func streamToTarget(ctx context.Context, streamer engine.StreamEngine, target output.StreamWriter, input engine.Input) error {
	if err := streamer.WriteTo(ctx, target.Stream(), input); err != nil {
		if discardErr := target.Discard(); discardErr != nil {
			logging.FromContext(ctx).Error("discard failed output", "template", input.Name, "error", discardErr)
		}
		return err
	}
	if err := target.Commit(); err != nil {
		return diag.Wrap(diag.KindIO, err)
	}
	return nil
}

// newStreamTargets builds one streaming destination per template. It returns
// nil entries when streaming is not possible, in which case the buffered
// Writer path is used instead.
func (req Request) newStreamTargets(templates []template) ([]output.StreamWriter, error) {
	// --check and --validate-only compare or discard instead of writing, so the
	// streaming path stays off and the buffered writer handles the destination.
	if req.ValidateOnly || req.Output.Check {
		return make([]output.StreamWriter, len(templates)), nil
	}
	targets := make([]output.StreamWriter, len(templates))
	for i, tmpl := range templates {
		target, err := req.newStreamTarget(tmpl)
		if err != nil {
			return nil, err
		}
		targets[i] = target
	}
	return targets, nil
}

// newStreamTarget selects the streaming destination for one template:
// stdout, a file, or an atomic in-place replacement.
func (req Request) newStreamTarget(tmpl template) (output.StreamWriter, error) {
	switch {
	case req.Output.InPlace:
		target, err := output.NewAtomicStream(tmpl.Input)
		if err != nil {
			return nil, diag.Wrap(diag.KindIO, err)
		}
		return target, nil
	case req.Output.Dir != "":
		return output.NewStreamToFile(tmpl.Output)
	case !utils.IsStdio(req.Output.Path):
		return output.NewStreamToFile(tmpl.Output)
	default:
		return output.NewDirectStream(os.Stdout), nil
	}
}

// loadSpec describes the value sources of this request for the load package.
func (req Request) loadSpec() load.Spec {
	return load.Spec{
		Files:            req.Values.Files,
		DotenvFiles:      req.Values.DotenvFiles,
		NoProcessEnv:     req.Values.NoProcessEnv,
		ProcessEnvPrefix: req.Values.ProcessEnvPrefix,
		Schema:           req.Values.Schema,
		UnknownKeywords:  req.Policies.UnknownKeywords,
		Sets:             req.Values.Sets,
		SetJSON:          req.Values.SetJSON,
		SetFiles:         req.Values.SetFiles,
	}
}

// resolveTemplates expands the template arguments into input/output pairs.
func (req Request) resolveTemplates() ([]template, error) {
	args := req.Input.Templates
	if len(args) == 0 {
		args = []string{utils.StdioPath}
	}

	var inputs []string
	for _, arg := range args {
		if utils.IsStdio(arg) {
			inputs = append(inputs, arg)
			continue
		}
		matches, err := expandTemplateArg(arg, req.Input.Recursive)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, matches...)
	}
	if len(inputs) == 0 {
		return nil, diag.New(diag.KindUsage, "no templates matched the given arguments")
	}
	if req.Output.InPlace {
		if slices.ContainsFunc(inputs, utils.IsStdio) {
			return nil, diag.New(diag.KindUsage, "--in-place requires a template file")
		}
	}

	// Batch output mirrors the directory layout relative to the common
	// ancestor of every input, so a single directory maps to the output
	// directory root.
	base := commonDir(inputs)
	templates := make([]template, 0, len(inputs))
	for _, input := range inputs {
		out, err := req.templateOutput(input, base)
		if err != nil {
			return nil, err
		}
		templates = append(templates, template{Input: input, Output: out})
	}
	return templates, nil
}

// commonDir returns the deepest directory that contains every input path.
// Stdin entries are ignored; the result is empty when none remain.
func commonDir(inputs []string) string {
	var base string
	for _, input := range inputs {
		if utils.IsStdio(input) {
			continue
		}
		dir := filepath.Dir(input)
		if base == "" {
			base = dir
			continue
		}
		for base != dir && !strings.HasPrefix(dir+string(filepath.Separator), base+string(filepath.Separator)) {
			parent := filepath.Dir(base)
			if parent == base {
				return string(filepath.Separator)
			}
			base = parent
		}
	}
	return base
}

// expandTemplateArg resolves a path, glob or directory into template files.
func expandTemplateArg(arg string, recursive bool) ([]string, error) {
	if strings.ContainsAny(arg, templateGlobChars) {
		matches, err := filepath.Glob(arg)
		if err != nil {
			return nil, diag.Wrap(diag.KindUsage, fmt.Errorf("invalid pattern %q: %w", arg, err))
		}
		if len(matches) == 0 {
			return nil, diag.New(diag.KindUsage, "no files match pattern %q", arg)
		}
		return matches, nil
	}

	info, err := os.Stat(arg)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, diag.Wrap(diag.KindIO, fmt.Errorf("template %s: %w", arg, err))
		}
		return nil, diag.Wrap(diag.KindIO, fmt.Errorf("stat %s: %w", arg, err))
	}
	if !info.IsDir() {
		return []string{arg}, nil
	}
	return listDirectory(arg, recursive)
}

// listDirectory lists template files in a directory.
func listDirectory(dir string, recursive bool) ([]string, error) {
	var files []string
	if recursive {
		err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			return nil, diag.Wrap(diag.KindIO, fmt.Errorf("walk %s: %w", dir, err))
		}
	} else {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, diag.Wrap(diag.KindIO, fmt.Errorf("read directory %s: %w", dir, err))
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	if len(files) == 0 {
		return nil, diag.New(diag.KindUsage, "no templates found in %s", dir)
	}
	return files, nil
}

// templateOutput computes the destination path for one template.
func (req Request) templateOutput(input, base string) (string, error) {
	switch {
	case req.Output.InPlace:
		return input, nil
	case req.Output.Dir != "":
		relative := filepath.Base(input)
		if !utils.IsStdio(input) && base != "" {
			rel, err := filepath.Rel(base, input)
			if err != nil {
				return "", diag.Wrap(diag.KindIO, err)
			}
			relative = rel
		}
		return filepath.Join(req.Output.Dir, relative), nil
	case !utils.IsStdio(req.Output.Path):
		return req.Output.Path, nil
	default:
		return "", nil
	}
}

// readTemplate reads a whole template into memory. It backs the buffered
// fallback path for engines without reader streaming.
func (req Request) stdin() io.Reader {
	if req.Stdin != nil {
		return req.Stdin
	}
	return os.Stdin
}

func readTemplate(path string, stdin io.Reader) ([]byte, error) {
	templateInput := source.TemplateInput(path)
	if stream, ok := templateInput.(source.StdinInput); ok {
		stream.Reader = stdin
		templateInput = stream
	}

	stream, err := templateInput.Open()
	if err != nil {
		return nil, templateReadError(path, err)
	}
	defer func() {
		_ = stream.Close()
	}()

	data, err := io.ReadAll(stream)
	if err != nil {
		return nil, templateReadError(path, err)
	}
	return data, nil
}

// templateReadError keeps the diagnostic wording distinct for stdin and files.
func templateReadError(path string, err error) error {
	if utils.IsStdio(path) {
		return diag.Wrap(diag.KindIO, fmt.Errorf("read stdin: %w", err))
	}
	return diag.Wrap(diag.KindIO, fmt.Errorf("read template %s: %w", path, err))
}

// newWriter selects the buffered writer used by engines that cannot stream.
func (req Request) newWriter() (output.Writer, error) {
	switch {
	case req.Output.Check:
		return output.NewCheckWriter(), nil
	case req.Output.InPlace:
		return output.NewInPlaceWriter(), nil
	case utils.IsStdio(req.Output.Path) && req.Output.Dir == "":
		return output.NewStdoutWriter(os.Stdout), nil
	default:
		return output.NewFileWriter(req.Output.Path), nil
	}
}
