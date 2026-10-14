// Package extended implements the extended template syntax.
package extended

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/pipeline"
	"github.com/elizabevil/envsub/internal/types"
	"github.com/elizabevil/envsub/internal/values"
)

// Engine implements the extended template syntax.
type Engine struct{}

// New returns an extended engine.
func New() *Engine { return &Engine{} }

// Name implements engine.Engine.
func (e *Engine) Name() string { return engine.NameExtended }

// Render implements engine.Engine.
func (e *Engine) Render(ctx context.Context, in engine.Input) ([]byte, error) {
	in = in.WithTree()
	var out strings.Builder
	if err := e.RenderTo(ctx, &out, in); err != nil {
		return nil, err
	}
	return []byte(out.String()), nil
}

// renderBytes renders a nested word into memory. It backs the
// buffer-returning paths; the streaming paths use RenderTo and StreamTo.
func (e *Engine) renderBytes(ctx context.Context, src []byte, in engine.Input, keepRaw bool) ([]byte, error) {
	var out strings.Builder
	err := ScanSpans(src, func(span Span) error {
		return e.emitSpan(ctx, &out, src, span, in, keepRaw)
	})
	if err != nil {
		return nil, err
	}
	return []byte(out.String()), nil
}

// RenderTo renders in.Template directly to w without materializing the output
// or the token list, which keeps peak memory proportional to the value tree
// instead of the template size.
func (e *Engine) RenderTo(ctx context.Context, w io.Writer, in engine.Input) error {
	in = in.WithTree()
	buffered := bufio.NewWriterSize(w, streamBufferSize)
	scanErr := ScanSpans(in.Template, func(span Span) error {
		return e.emitSpan(ctx, buffered, in.Template, span, in, true)
	})
	if scanErr != nil {
		if errors.Is(scanErr, errKeepPlaceholder) {
			return nil
		}
		return locateTemplateError(scanErr, in.Template, in.Name)
	}
	if err := buffered.Flush(); err != nil {
		return diag.Wrap(diag.KindIO, err)
	}
	return nil
}

// WriteTo implements engine.StreamEngine: it renders in.Template directly into
// w, so peak memory stays proportional to the value tree rather than the
// template or the rendered output.
func (e *Engine) WriteTo(ctx context.Context, w io.Writer, in engine.Input) error {
	return e.RenderTo(ctx, w, in)
}

// emitSpan renders one scanner span into w.
func (e *Engine) emitSpan(ctx context.Context, w io.Writer, src []byte, span Span, in engine.Input, keepRaw bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	tree := in.Values
	opts := in.Options
	switch span.Kind {
	case TokenText:
		_, err := w.Write(src[span.Start:span.End])
		return err
	case TokenEscapedDollar:
		_, err := w.Write([]byte{charDollar})
		return err
	case TokenVariable:
		// Fast path: a bare ${NAME} or $NAME reference resolves directly in the
		// root namespace without materializing a Token or parsing a path. The process
		// environment is deliberately not consulted: a bare name never falls back to
		// it, so only ${@process.NAME} can read it.
		// The name stays a slice of src: no string is allocated on the hot path,
		// and one is built only by the slow branches below.
		if name, ok := simpleSpanName(src, span); ok && opts.ReplacesBytes(name) {
			if value, exists := tree.ResolveRootName(name); exists {
				if isEmptyValue(value) && opts.Empty == engine.EmptyError {
					return emptyValueError(in, expression{Path: string(name)})
				}
				return writeValueBytes(w, name, value, spanColumn(src, span.Start))
			}
			policy, _ := fastUnset(opts.Unset)
			switch policy {
			case policyHandled:
				// The policy emitted nothing at all.
				return nil
			case policyError:
				return diag.New(diag.KindMissing, "variable %q is not set", name)
			case policyKeep:
				// Only the top level may emit the placeholder verbatim;
				// inside a default word the keep signal must propagate.
				if keepRaw {
					_, err := w.Write(src[span.Start:span.End])
					return err
				}
				// Fall through to the general path, which propagates the
				// keep signal out of nested words.
			case policySchema:
				// --on-missing=zero needs the schema or an inline | type, so use the
				// general policy helper with the already-resolved value instead
				// of re-entering the fast path.
				result, err := e.applyPolicy(ctx, expression{Path: string(name)}, in, nil, false)
				if err != nil {
					return err
				}
				if !result.Found {
					return nil
				}
				return writeValueBytes(w, name, result.Value, spanColumn(src, span.Start))
			}
		}
		token := spanToken(src, span)
		result, err := e.renderToken(ctx, token, in, keepRaw)
		if err != nil {
			return err
		}
		if !result.Found {
			return nil
		}
		return writeValue(w, token.Path, result.Value, spanColumn(src, span.Start))
	default:
		return nil
	}
}

// spanColumn returns how many bytes of output precede the span on its line: the
// leading indentation plus anything written before the expression (for example
// "  labels: " before a | block). A substituted value that spans several lines
// continues at that column, which is what lets a multi-line value such as toYaml
// output be nested under a parent key: an indentation-aware format needs every
// line of the block at one column.
//
// A line indented with tabs cannot be reproduced safely, so the value is then
// emitted unchanged.
func spanColumn(src []byte, start int) int {
	if start > len(src) {
		start = len(src)
	}
	lineStart := start
	for lineStart > 0 && src[lineStart-1] != '\n' {
		lineStart--
	}
	width := 0
	for _, b := range src[lineStart:start] {
		switch b {
		case ' ':
			width++
		case '\t':
			// Tab indentation cannot be reproduced as a column, so the value is
			// emitted unchanged rather than guessed at.
			return 0
		default:
			// The expression is not alone after its indentation (for example
			// `key: ${x}`); a multi-line value there has no column to align to.
			return 0
		}
	}
	return width
}

// alignLines indents every line after the first to column spaces. A single-line
// value, or a substitution that starts at column 0, is returned unchanged so the
// common case allocates nothing.
func alignLines(text string, column int) string {
	if column <= 0 || !strings.Contains(text, "\n") {
		return text
	}
	return strings.ReplaceAll(text, "\n", "\n"+strings.Repeat(" ", column))
}

// errKeepPlaceholder signals that --on-missing=keep applies to the expression being
// rendered, so the caller must emit the original placeholder text. It is
// propagated out of nested default words as well.
var errKeepPlaceholder = errors.New("keep placeholder")

// renderResult is the outcome of evaluating one expression.
type renderResult struct {
	// Value is the resolved value.
	Value any
	// Found is false when the expression renders nothing.
	Found bool
}

func found(value any) renderResult {
	return renderResult{Value: value, Found: true}
}

func (e *Engine) renderToken(ctx context.Context, token Token, in engine.Input, keepRaw bool) (renderResult, error) {
	expr, err := parseExpression(token.Path)
	if err != nil {
		return renderResult{}, diag.WrapIfNot(diag.KindTemplate, fmt.Errorf("parse %q: %w", token.Path, err))
	}
	expr.Offset = token.Pos
	if !in.Options.Replaces(expr.Path) {
		return found(token.String()), nil
	}
	result, err := e.renderVariable(ctx, expr, in)
	if err != nil {
		if errors.Is(err, errKeepPlaceholder) {
			if keepRaw {
				return found(token.Raw), nil
			}
			return renderResult{}, errKeepPlaceholder
		}
		return renderResult{}, err
	}
	return result, nil
}

// renderVariable evaluates one expression, applying operators, policies and
// the formatter pipeline.
func (e *Engine) renderVariable(ctx context.Context, expr expression, in engine.Input) (renderResult, error) {
	tree := in.Values
	// Paths repeat heavily across a template, so parse once and reuse.
	path, err := tree.CompilePath(expr.Path)
	if err != nil {
		return renderResult{}, diag.Wrap(diag.KindTemplate, fmt.Errorf("resolve %q: %w", expr.Path, err))
	}
	value, exists := tree.ResolvePath(path)

	switch expr.Op {
	case "":
		return e.applyPolicy(ctx, expr, in, value, exists)
	case opDefaultOrEmpty:
		if !exists || isEmptyValue(value) {
			return e.pipelineWord(ctx, expr, in)
		}
		return pipelineValue(ctx, expr, value, in)
	case opAssignDefault:
		// := writes the default into the value tree so later references see it;
		// the process environment is never modified. The stored value is the
		// rendered word itself: the pipeline formats this output, it does not
		// change what the expression assigns.
		if !exists || isEmptyValue(value) {
			word, err := e.renderedWord(ctx, expr.Word, in)
			if err != nil {
				return renderResult{}, err
			}
			return e.assignWord(ctx, expr, word, in)
		}
		return pipelineValue(ctx, expr, value, in)
	case opDefaultIfUnset:
		if !exists {
			return e.pipelineWord(ctx, expr, in)
		}
		return pipelineValue(ctx, expr, value, in)
	case opAssignIfUnset:
		if !exists {
			word, err := e.renderedWord(ctx, expr.Word, in)
			if err != nil {
				return renderResult{}, err
			}
			return e.assignWord(ctx, expr, word, in)
		}
		return pipelineValue(ctx, expr, value, in)
	case opAlternateNonEmpty:
		if exists && !isEmptyValue(value) {
			return e.pipelineWord(ctx, expr, in)
		}
		return renderResult{}, nil
	case opAlternateIfSet:
		if exists {
			return e.pipelineWord(ctx, expr, in)
		}
		return renderResult{}, nil
	case opRequiredNonEmpty:
		if !exists || isEmptyValue(value) {
			return renderResult{}, e.requiredError(ctx, expr, in)
		}
		return pipelineValue(ctx, expr, value, in)
	case opRequiredIfSet:
		if !exists {
			return renderResult{}, e.requiredError(ctx, expr, in)
		}
		return pipelineValue(ctx, expr, value, in)
	default:
		return renderResult{}, diag.Wrap(diag.KindTemplate, fmt.Errorf("unsupported operator %q", expr.Op))
	}
}

// pipelineWord renders an operator's word and applies the expression's inline type
// and formatter pipeline to it. A word is a value like any other: the pipeline says
// what to do with whatever the expression produced, so ${A:-fallback | quote}
// quotes the fallback and ${A:+set | toJson} JSON-encodes the alternate word.
func (e *Engine) pipelineWord(ctx context.Context, expr expression, in engine.Input) (renderResult, error) {
	word, err := e.renderedWord(ctx, expr.Word, in)
	if err != nil {
		return renderResult{}, err
	}
	return pipelineValue(ctx, expr, word, in)
}

// renderedWord renders one operator word, expanding nested references.
func (e *Engine) renderedWord(ctx context.Context, word string, in engine.Input) (string, error) {
	rendered, err := e.renderBytes(ctx, []byte(word), in, false)
	if err != nil {
		return "", err
	}
	return string(rendered), nil
}

// assignWord stores word at the expression path and returns the value this
// expression emits, which is the same word after the pipeline. The stored value
// stays unformatted: the pipeline formats this output, it does not change what the
// expression assigns.
func (e *Engine) assignWord(ctx context.Context, expr expression, word string, in engine.Input) (renderResult, error) {
	origin := values.Origin{
		Kind:   values.SourceSet,
		Method: values.MethodAssign,
		Source: in.Name,
		File:   in.Name,
		Line:   lineAt(in.Template, expr.Offset),
	}
	if err := in.Values.Store(expr.Path, word, origin); err != nil {
		return renderResult{}, diag.Wrap(diag.KindTemplate, err)
	}
	return pipelineValue(ctx, expr, word, in)
}

// lineAt returns the 1-based line containing offset. It is how an assignment
// reports the template line it wrote from; 0 when the source is not available.
func lineAt(src []byte, offset int) int {
	if offset < 0 || offset > len(src) {
		return 0
	}
	line := 1
	for _, b := range src[:offset] {
		if b == '\n' {
			line++
		}
	}
	return line
}

// applyPolicy handles the --on-missing and --on-empty policies for a plain reference.
func (e *Engine) applyPolicy(ctx context.Context, expr expression, in engine.Input, value any, exists bool) (renderResult, error) {
	opts := in.Options
	if !exists {
		switch opts.Unset {
		case "", engine.UnsetError:
			return renderResult{}, missingValueError(in, expr.Path)
		case engine.UnsetEmpty:
			return found(""), nil
		case engine.UnsetKeep:
			return renderResult{}, errKeepPlaceholder
		case engine.UnsetZero:
			zero, err := zeroValue(expr, in.Schema)
			if err != nil {
				return renderResult{}, err
			}
			return pipelineValue(ctx, expr, zero, in)
		default:
			return renderResult{}, diag.New(diag.KindUsage,
				"invalid --on-missing value %q: want error, empty, keep or zero", opts.Unset)
		}
	}
	if isEmptyValue(value) && opts.Empty == engine.EmptyError {
		return renderResult{}, emptyValueError(in, expr)
	}
	return pipelineValue(ctx, expr, value, in)
}

// pipelineValue applies the inline | type assertion followed by the formatter
// chain.
func pipelineValue(ctx context.Context, expr expression, value any, in engine.Input) (renderResult, error) {
	if err := ctx.Err(); err != nil {
		return renderResult{}, err
	}
	typed, err := applyType(expr, value, in.Schema)
	if err != nil {
		return renderResult{}, err
	}
	result, err := pipeline.Apply(typed, expr.Steps)
	if err != nil {
		return renderResult{}, diag.WrapIfNot(diag.KindType, err)
	}
	return found(result), nil
}

// applyType enforces the schema type and the inline | type assertion. Schema is
// the global contract, the inline assertion is a local check; both must hold.
func applyType(expr expression, value any, sch types.TypeResolver) (any, error) {
	declared := expr.Declared
	schemaType := ""
	if sch != nil {
		schemaType = sch.DeclaredType(expr.Path)
	}
	switch {
	case schemaType == "" && declared.Name == "":
		return normalizeValue(value), nil
	case schemaType == "":
		converted, err := types.Assert(value, declared)
		if err != nil {
			return nil, diag.Wrap(diag.KindType, fmt.Errorf("%s: %w", expr.Path, err))
		}
		return converted, nil
	default:
		annotated, err := normalizeBySchemaMembers(expr.Path, value, sch, schemaType)
		if err != nil {
			// A value the declared type cannot hold is a type failure, whatever
			// produced it: a value from the tree or an operator's word.
			return nil, diag.WrapIfNot(diag.KindType, err)
		}
		if declared.Name == "" {
			return annotated, nil
		}
		compatible, err := schemaCompatibleMembers(declared, sch, expr.Path, schemaType)
		if err != nil {
			return nil, diag.Wrap(diag.KindType, fmt.Errorf("%s: %w", expr.Path, err))
		}
		if !compatible {
			return nil, diag.New(diag.KindType,
				"%s: inline type %s conflicts with schema type %s", expr.Path, declared, schemaType)
		}
		converted, err := types.Assert(annotated, declared)
		if err != nil {
			return nil, diag.Wrap(diag.KindType, fmt.Errorf("%s: %w", expr.Path, err))
		}
		return converted, nil
	}
}

// normalizeBySchemaType converts a string value to the type the schema
// declares, including Go-style aliases.
func normalizeBySchemaType(path string, value any, schemaType string) (any, error) {
	name, err := schemaTypeToDeclared(schemaType)
	if err != nil {
		return nil, err
	}
	converted, err := types.Assert(value, name)
	if err != nil {
		return nil, diag.Wrap(diag.KindType, fmt.Errorf("%s: %w", path, err))
	}
	return converted, nil
}

// schemaTypeToDeclared maps a JSON Schema type name to the type assertion it
// implies. The mapping lives in types so no evaluator has to know the schema
// vocabulary, and an unsupported name stays a schema-kind error.
func schemaTypeToDeclared(schemaType string) (types.Declared, error) {
	declared, err := types.SchemaTypeToDeclared(schemaType)
	if err != nil {
		return types.Declared{}, diag.Wrap(diag.KindSchema, err)
	}
	return declared, nil
}

// schemaCompatible reports whether an inline assertion may be combined with a
// schema type. int and float are distinct types, except that a whole-number
// float satisfies int for zero-value compatibility.
// schemaCompatibleMembers reports whether an inline type is compatible with any
// type the schema declares at path (rules SI11): a union such as
// ["integer","string"] must accept both | type int and | type string.
func schemaCompatibleMembers(declared types.Declared, sch types.TypeResolver, path, collapsed string) (bool, error) {
	if sch == nil {
		return false, nil
	}
	members := sch.DeclaredTypes(path)
	if len(members) == 0 {
		members = []string{collapsed}
	}
	for _, member := range members {
		ok, err := schemaCompatible(declared, member)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// normalizeBySchemaMembers converts a value with every type the schema declares
// at path, in written order, and keeps the value when no member accepts it but
// "string" is allowed (rules SI11). It mirrors the materializer, so the render
// path cannot reject a value the schema allows.
func normalizeBySchemaMembers(path string, value any, sch types.TypeResolver, collapsed string) (any, error) {
	// String values go through the shared union algorithm in schema, so the
	// renderer and the materializer can never disagree (rules SI11).
	if text, isString := value.(string); isString {
		converted, changed, err := sch.NormalizeString(path, text)
		if err != nil || changed {
			return converted, err
		}
	}
	// An already-typed value only needs a membership check against the declared
	// members; there is no conversion logic to duplicate here.
	members := sch.DeclaredTypes(path)
	if len(members) == 0 {
		members = []string{collapsed}
	}
	allowsString := false
	var firstErr error
	for _, member := range members {
		if member == types.SchemaTypeString {
			allowsString = true
			continue
		}
		converted, err := normalizeBySchemaType(path, value, member)
		if err == nil {
			return converted, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	if allowsString || firstErr == nil {
		return value, nil
	}
	return nil, firstErr
}

func schemaCompatible(declared types.Declared, schemaType string) (bool, error) {
	schemaDeclared, err := schemaTypeToDeclared(schemaType)
	if err != nil {
		return false, err
	}
	if declared.Name == schemaDeclared.Name {
		if declared.Name != types.NameArray {
			return true, nil
		}
		if declared.Element == "" || schemaDeclared.Element == "" {
			return true, nil
		}
		return declared.Element == schemaDeclared.Element, nil
	}
	if isNumericKind(declared.Name) && isNumericKind(schemaDeclared.Name) {
		return true, nil
	}
	return false, nil
}

func isNumericKind(name string) bool {
	return name == types.NameInt || name == types.NameFloat
}

// zeroValue returns the zero value for the type declared by the schema or the
// inline assertion, and fails when no type can be determined.
func zeroValue(expr expression, sch types.TypeResolver) (any, error) {
	declared := expr.Declared
	if sch != nil {
		schemaType := sch.DeclaredType(expr.Path)
		if schemaType != "" {
			converted, err := schemaTypeToDeclared(schemaType)
			if err != nil {
				return nil, err
			}
			if declared.Name != "" {
				compatible, err := schemaCompatibleMembers(declared, sch, expr.Path, schemaType)
				if err != nil {
					return nil, err
				}
				if !compatible {
					return nil, diag.New(diag.KindType,
						"%s: inline type %s conflicts with schema type %s", expr.Path, declared, schemaType)
				}
			}
			return zeroFor(converted), nil
		}
	}
	if declared.Name == "" {
		return nil, diag.New(diag.KindPolicy,
			"%s: --on-missing=zero requires a schema type or an inline | type", expr.Path)
	}
	return zeroFor(declared), nil
}

func zeroFor(declared types.Declared) any {
	switch declared.Name {
	case types.NameString:
		return ""
	case types.NameBool:
		return false
	case types.NameInt:
		return 0
	case types.NameFloat:
		return float64(0)
	case types.NameArray:
		return []any{}
	case types.NameObject:
		return map[string]any{}
	default:
		return nil
	}
}

// normalizeValue converts decoded numbers into consistent Go types.
func normalizeValue(value any) any {
	return types.Normalize(value)
}

func (e *Engine) requiredError(ctx context.Context, expr expression, in engine.Input) error {
	rendered, err := e.renderBytes(ctx, []byte(expr.Word), in, false)
	if err != nil {
		return err
	}
	msg := string(rendered)
	if msg == "" {
		msg = fmt.Sprintf("variable %q is required", expr.Path)
	}
	return diag.New(diag.KindMissing, "%s", msg)
}

// writeValue streams a resolved value, avoiding the intermediate string that
// stringifyValue would allocate for the common scalar cases.
func writeValue(w io.Writer, path string, value any, column int) error {
	if handled, err := writeScalar(w, types.Normalize(value), column); handled {
		return err
	}
	text, err := stringifyValue(path, value)
	if err != nil {
		return err
	}
	if _, aligned := pipeline.Text(value); aligned {
		text = alignLines(text, column)
	}
	_, err = io.WriteString(w, text)
	return err
}

// writeValueBytes is writeValue for a name that is still a slice of the
// template: the string is built only when the value needs the stringify
// fallback, whose error message carries the path.
func writeValueBytes(w io.Writer, name []byte, value any, column int) error {
	if handled, err := writeScalar(w, types.Normalize(value), column); handled {
		return err
	}
	text, err := stringifyValue(string(name), value)
	if err != nil {
		return err
	}
	if _, aligned := pipeline.Text(value); aligned {
		text = alignLines(text, column)
	}
	_, err = io.WriteString(w, text)
	return err
}

// writeScalar writes the scalar forms shared by writeValue and writeValueBytes.
// It reports false when the value needs the stringify fallback. A string can span
// several lines, so it is aligned like any other multi-line result.
func writeScalar(w io.Writer, value any, column int) (bool, error) {
	switch typed := value.(type) {
	case pipeline.Aligned:
		_, err := io.WriteString(w, alignLines(string(typed), column))
		return true, err
	case string:
		_, err := io.WriteString(w, typed)
		return true, err
	case int:
		var scratch [20]byte
		_, err := w.Write(strconv.AppendInt(scratch[:0], int64(typed), decimalBase))
		return true, err
	case bool:
		_, err := io.WriteString(w, strconv.FormatBool(typed))
		return true, err
	default:
		return false, nil
	}
}

func stringifyValue(path string, value any) (string, error) {
	text, err := types.Stringify(normalizeValue(value))
	if err != nil {
		return "", diag.Wrap(diag.KindType, fmt.Errorf("format %q: %w", path, err))
	}
	return text, nil
}

// missingValueError reports an unset variable together with its source.
func missingValueError(in engine.Input, path string) error {
	if location := values.DescribeMeta(in.Values.MetaFor(path)); location != "" {
		return diag.New(diag.KindMissing, "variable %q is not set (defined at %s)", path, location)
	}
	return diag.New(diag.KindMissing, "variable %q is not set", path)
}

// emptyValueError reports an empty value rejected by --on-empty=error.
func emptyValueError(in engine.Input, expr expression) error {
	location := values.DescribeMeta(in.Values.MetaFor(expr.Path))
	if location != "" {
		return diag.New(diag.KindPolicy, "variable %q is empty (defined at %s)", expr.Path, location)
	}
	return diag.New(diag.KindPolicy, "variable %q is empty", expr.Path)
}

// isEmptyValue reports whether a resolved value counts as empty.
func isEmptyValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return typed == ""
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

// fastPolicy names the unset policy the fast path can settle without the
// schema.
type fastPolicy int

const (
	// policyHandled means the policy emits nothing.
	policyHandled fastPolicy = iota
	// policyError means the variable must be reported as missing.
	policyError
	// policyKeep means the original placeholder must be preserved.
	policyKeep
	// policySchema means the policy needs the schema and must fall through.
	policySchema
)

// fastUnset classifies the unset policy for a missing bare identifier.
func fastUnset(unset string) (fastPolicy, bool) {
	switch unset {
	case "", engine.UnsetError:
		return policyError, false
	case engine.UnsetEmpty:
		return policyHandled, false
	case engine.UnsetKeep:
		return policyKeep, true
	case engine.UnsetZero:
		return policySchema, false
	default:
		return policySchema, true
	}
}

// simpleSpanName reports whether a variable span is a bare identifier, and
// returns its name as a sub-slice of src, so callers never allocate a string on
// the hot path. The slice must not outlive src.
func simpleSpanName(src []byte, span Span) ([]byte, bool) {
	raw := src[span.Start:span.End]
	if span.Braced {
		if len(raw) < 3 || raw[len(raw)-1] != '}' {
			return nil, false
		}
		raw = raw[2 : len(raw)-1]
	} else {
		raw = raw[1:]
	}
	if len(raw) == 0 || !isIdentifierStart(raw[0]) {
		return nil, false
	}
	for i := 1; i < len(raw); i++ {
		if !isIdentifierByte(raw[i]) {
			return nil, false
		}
	}
	return raw, true
}

// streamBufferSize is the per-template write buffer for streaming renders.
const streamBufferSize = 64 * 1024

// decimalBase is the number base used when formatting integers.
const decimalBase = 10

// newStreamBuffer wraps w in a per-template write buffer, so a template with
// many variables does not turn into one small write per token.
func newStreamBuffer(w io.Writer) *bufio.Writer {
	return bufio.NewWriterSize(w, streamBufferSize)
}

// locateTemplateError attaches the template name and position to scanner
// failures, which report a byte offset in their message.
func locateTemplateError(err error, src []byte, name string) error {
	if name == "" {
		return err
	}
	var de *diag.Error
	if !errors.As(err, &de) || de.Kind != diag.KindTemplate || de.File != "" {
		return err
	}
	if de.Offset < 0 {
		return diag.At(err, name, 0)
	}
	line, _ := Position(src, de.Offset)
	return diag.At(err, name, line)
}
