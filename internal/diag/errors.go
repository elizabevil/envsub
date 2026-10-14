// Package diag defines the project error envelope, its kinds and the process
// exit codes derived from them.
package diag

import (
	"errors"
	"fmt"
)

// Kind classifies an error for exit codes and future JSON error output.
type Kind string

// Error kinds, each mapped to a stable process exit code by ExitCode.
const (
	KindInternal Kind = "internal"
	KindUsage    Kind = "usage"
	KindTemplate Kind = "template"
	KindType     Kind = "type"
	KindSchema   Kind = "schema"
	KindIO       Kind = "io"
	KindMissing  Kind = "missing"
	KindPolicy   Kind = "policy"
	// KindDiff reports that rendered output differs from an existing target
	// (--check). It is not an error of the render itself.
	KindDiff Kind = "diff"
)

// ExitCode returns a stable process exit code for an error.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if de, ok := errors.AsType[*Error](err); ok {
		return de.ExitCode()
	}
	return exitInternal
}

// Is reports whether err is or wraps a project Error.
func Is(err error) bool {
	var de *Error
	return errors.As(err, &de)
}

// KindOf returns the classification of err, or an empty Kind when err is nil or
// does not carry one.
func KindOf(err error) Kind {
	if err == nil {
		return ""
	}
	if de, ok := errors.AsType[*Error](err); ok {
		return de.Kind
	}
	return ""
}

// WrapIfNot wraps err only when it is not already a project Error.
func WrapIfNot(kind Kind, err error) error {
	if err == nil || Is(err) {
		return err
	}
	return Wrap(kind, err)
}

// NotImplemented returns a standard internal not-implemented error.
func NotImplemented(feature string) error {
	return New(KindInternal, "%s is not implemented yet", feature)
}

// offsetUnknown marks a Location whose byte offset was never recorded.
const offsetUnknown = -1

// Location is a position inside a source file. Line is 1-based and 0 when
// unknown, matching values.Meta; Offset is a byte offset and -1 when unknown.
type Location struct {
	File   string
	Line   int
	Offset int
}

// Error is the project error envelope.
type Error struct {
	// Kind classifies the error and selects its exit code.
	Kind Kind
	// Msg is the message without the wrapped cause.
	Msg string
	// Err is the wrapped cause, if any.
	Err error
	// Location points at the offending source position when known, for example
	// the template being scanned. Its fields are promoted, so e.File, e.Line and
	// e.Offset stay available and the whole position can be passed as a value.
	Location
}

func (e *Error) Error() string {
	message := e.message()
	if e.File == "" {
		return message
	}
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", e.File, e.Line, message)
	}
	return e.File + ": " + message
}

// message renders the error without its source location.
func (e *Error) message() string {
	switch {
	case e.Msg != "" && e.Err != nil:
		return e.Msg + ": " + e.Err.Error()
	case e.Msg != "":
		return e.Msg
	case e.Err != nil:
		return e.Err.Error()
	default:
		return string(e.Kind)
	}
}

func (e *Error) Unwrap() error { return e.Err }

// Process exit codes, one per error kind.
const (
	exitInternal = 1
	exitUsage    = 2
	exitTemplate = 3
	exitType     = 4
	exitIO       = 5
	exitValue    = 6
	exitDiff     = 7
)

// ExitCode maps the error kind to a stable process exit code.
func (e *Error) ExitCode() int {
	switch e.Kind {
	case KindUsage:
		return exitUsage
	case KindTemplate:
		return exitTemplate
	case KindType, KindSchema:
		return exitType
	case KindIO:
		return exitIO
	case KindMissing, KindPolicy:
		return exitValue
	case KindDiff:
		return exitDiff
	default:
		return exitInternal
	}
}

// New creates an error with a stable kind.
func New(kind Kind, format string, args ...any) error {
	return &Error{Kind: kind, Msg: fmt.Sprintf(format, args...), Offset: offsetUnknown}
}

// Wrap creates an error with a stable kind and a wrapped cause.
func Wrap(kind Kind, err error) error {
	if err == nil {
		return nil
	}
	return &Error{Kind: kind, Err: err, Offset: offsetUnknown}
}

// WithOffset returns err with the byte offset within its file attached. It is a
// no-op for errors that are not project Errors.
func WithOffset(err error, offset int) error {
	var de *Error
	if !errors.As(err, &de) {
		return err
	}
	clone := *de
	clone.Offset = offset
	return &clone
}

// At attaches a source file and line to an existing project error. Other
// errors are wrapped as internal errors so the location is never lost.
func At(err error, file string, line int) error {
	if err == nil {
		return nil
	}
	var de *Error
	if !errors.As(err, &de) {
		de = &Error{Kind: KindInternal, Err: err, Offset: offsetUnknown}
	}
	clone := *de
	clone.File = file
	clone.Line = line
	return &clone
}
