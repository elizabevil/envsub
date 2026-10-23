// Package engine defines the engine-neutral rendering interfaces shared by the
// render pipeline and the template engine implementation.
package engine

import (
	"context"
	"io"

	"github.com/elizabevil/envsub/internal/types"
	"github.com/elizabevil/envsub/internal/values"
)

// Built-in engine name and the render policies accepted on the command line.
const (
	NameExtended = "extended"

	// --on-missing policies. UnsetError is the default.
	UnsetError = "error"
	UnsetEmpty = "empty"
	UnsetKeep  = "keep"
	UnsetZero  = "zero"

	// --on-empty policies. EmptyOK is the default.
	EmptyOK    = "ok"
	EmptyError = "error"

	// --unknown-keywords policies for schema keywords the validator does not know.
	SchemaUnknownIgnore = "ignore"
	SchemaUnknownWarn   = "warn"
	SchemaUnknownError  = "error"
)

// Options are the render policies of one run.
type Options struct {
	Unset         string
	Empty         string
	SchemaUnknown string
	// Only and Ignore filter which paths are substituted.
	Only   []string
	Ignore []string
}

// Replaces reports whether a variable path is substituted under the Only and
// Ignore filters. A pattern with a dot matches the whole path or a prefix of it;
// a bare pattern matches the root segment.
func (o Options) Replaces(path string) bool {
	if len(o.Only) > 0 && !matchesAny(path, o.Only) {
		return false
	}
	return !matchesAny(path, o.Ignore)
}

// ReplacesBytes is Replaces for a name that is still a slice of the template, so
// the unfiltered case (the common one) allocates nothing.
func (o Options) ReplacesBytes(name []byte) bool {
	if len(o.Only) == 0 && len(o.Ignore) == 0 {
		return true
	}
	return o.Replaces(string(name))
}

// matchesAny reports whether path matches any of the patterns.
func matchesAny(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchesPattern(path, pattern) {
			return true
		}
	}
	return false
}

// matchesPattern reports whether one Only/Ignore pattern selects path.
//
// A single bare name selects a root segment by name, so `app` selects app.port and
// `process` selects the @process namespace. Any other pattern is compared segment
// by segment, so a.b selects a.b and a.b.c but never ["a.b"].c, and a namespace
// path keeps its @ sigil (process.NAME is a root path, @process.NAME is not).
func matchesPattern(path, pattern string) bool {
	patternSegments, err := values.ParsePath(pattern)
	if err != nil || len(patternSegments) == 0 {
		return false
	}
	if len(patternSegments) == 1 && !patternSegments[0].IsNamespace {
		return values.RootKey(path) == patternSegments[0].Key
	}
	pathSegments, err := values.ParsePath(path)
	if err != nil || len(patternSegments) > len(pathSegments) {
		return false
	}
	for i, segment := range patternSegments {
		if segment != pathSegments[i] {
			return false
		}
	}
	return true
}

// Input is one render: the template, the values it resolves against and the
// policies that apply.
type Input struct {
	// Template is the template source, empty when src is streamed instead.
	Template []byte
	// Name identifies the template in diagnostics; a file path or -.
	Name string
	// Values is the merged value tree.
	Values *values.Tree
	// Schema is the type contract the template is rendered against; nil means no
	// schema, so only inline | type assertions apply.
	Schema types.TypeResolver
	// Options are the render policies.
	Options Options
	// SourceSize is the template length in bytes when it is known.
	SourceSize int64
	// SourceSizeKnown reports whether SourceSize is meaningful. A source of unknown
	// length (stdin, HTTP body, RPC stream) cannot be measured or rewound, so it
	// keeps the renderer sequential.
	SourceSizeKnown bool
}

// WithTree returns a copy of in whose value tree is never nil, so renderers can
// resolve without a nil check per lookup.
func (in Input) WithTree() Input {
	if in.Values == nil {
		in.Values = values.NewTree()
	}
	return in
}

// Engine isolates template syntax and evaluation semantics. The pipeline drives
// exactly one engine, so there is no registry.
type Engine interface {
	Name() string
	Render(ctx context.Context, in Input) ([]byte, error)
}

// PathEngine is implemented by engines that can list the variable paths a template
// references; --explain uses it.
type PathEngine interface {
	Engine
	// Paths returns the referenced variable paths, deduplicated and ordered by
	// first appearance in the template.
	Paths(src []byte) ([]string, error)
}

// PathFilterEngine is an optional refinement of PathEngine: it lists only the
// paths the renderer's substitution policy keeps. It matters for references
// nested inside an operator word, because an excluded outer path leaves the
// whole token literal and its word is never evaluated.
type PathFilterEngine interface {
	PathEngine
	// PathsFor is Paths restricted to the paths replaces selects. A nil
	// predicate selects every path.
	PathsFor(src []byte, replaces func(string) bool) ([]string, error)
}

// StreamEngine is implemented by engines that can render straight into a writer,
// so a large template is never buffered.
type StreamEngine interface {
	Engine
	// WriteTo renders in.Template into w.
	WriteTo(ctx context.Context, w io.Writer, in Input) error
}

// StreamReaderEngine is implemented by engines that can render from a reader, so a
// file template is never buffered in full.
type StreamReaderEngine interface {
	Engine
	// StreamTo renders src into w. in.Template is ignored and in.Name
	// identifies the template in diagnostics.
	StreamTo(ctx context.Context, w io.Writer, src io.Reader, in Input) error
}
