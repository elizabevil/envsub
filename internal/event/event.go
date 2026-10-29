// Package event defines the JSONL event protocol the CLI writes to stderr and
// the single writer that produces it. Every line has the same envelope; the
// payload is specialized by type:
//
//	{"schema":1,"type":"explain","time":"2026-10-22T23:43:37Z","seq":1,"data":{...}}
//
// Keeping one writer means one encoder, one timestamp format and one sequence
// across logs, explain records, unused records and errors.
package event

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/elizabevil/envsub/internal/jsonx"
	"github.com/elizabevil/envsub/internal/timefmt"
)

// Schema is the event protocol version. Bump it when an existing field changes
// meaning or is removed; adding a field does not require a bump.
const Schema = 1

// Event types.
const (
	TypeExplain = "explain"
	TypeUnused  = "unused"
	TypeError   = "error"
	TypeLog     = "log"
)

// Event is one JSONL line.
type Event struct {
	// Schema is the protocol version.
	Schema int `json:"schema"`
	// Type discriminates the payload: explain, unused, error or log.
	Type string `json:"type"`
	// Time is the event time in UTC, RFC3339 at second precision. It is produced
	// by timefmt, never by a caller.
	Time string `json:"time"`
	// Seq is monotonic within one writer, starting at 1. It orders events that
	// share a second and makes a merged CI stream deterministic.
	Seq uint64 `json:"seq"`
	// Data is the type-specific payload. It is always an object.
	Data any `json:"data"`
}

// Writer serializes events to one stream. It is safe for concurrent use, which
// matters because render workers log in parallel while reports are written on
// the main goroutine.
type Writer struct {
	mu  sync.Mutex
	w   io.Writer
	seq uint64
	now func() time.Time
}

// NewWriter returns a writer that emits to w with the process clock.
func NewWriter(w io.Writer) *Writer {
	return NewWriterWithClock(w, time.Now)
}

// NewWriterWithClock is NewWriter with an injectable clock, so tests can pin
// timestamps and sequences.
func NewWriterWithClock(w io.Writer, now func() time.Time) *Writer {
	if now == nil {
		now = time.Now
	}
	return &Writer{w: w, now: now}
}

// Emit writes one event with the writer's clock.
func (w *Writer) Emit(typ string, data any) error {
	if w == nil {
		return errors.New("event: nil writer")
	}
	return w.EmitAt(w.now(), typ, data)
}

// EmitAt writes one event with an explicit time. Logs use the record time so a
// queued record keeps the instant it was logged, not the instant it was written.
func (w *Writer) EmitAt(t time.Time, typ string, data any) error {
	if w == nil || w.w == nil {
		return errors.New("event: no destination")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.seq++
	encoded, err := jsonx.Marshal(Event{
		Schema: Schema,
		Type:   typ,
		Time:   timefmt.Format(t),
		Seq:    w.seq,
		Data:   data,
	})
	if err != nil {
		return err
	}
	if _, err := w.w.Write(append(encoded, '\n')); err != nil {
		return err
	}
	return nil
}

// contextKey carries the shared writer from the assembler to the CLI and render
// layers.
type contextKey struct{}

// WithWriter stores the writer on ctx.
func WithWriter(ctx context.Context, w *Writer) context.Context {
	return context.WithValue(ctx, contextKey{}, w)
}

// FromContext returns the writer stored on ctx, or nil.
func FromContext(ctx context.Context) *Writer {
	if ctx == nil {
		return nil
	}
	w, _ := ctx.Value(contextKey{}).(*Writer)
	return w
}

// OriginFrame is one step of a value's lineage.
type OriginFrame struct {
	// Kind is the layering stage: schema, values, envfile, set, process,
	// assign or missing.
	Kind string `json:"kind"`
	// Method is how the value entered that stage: default, file, flag, json,
	// env, assign or missing.
	Method string `json:"method"`
	// Source is the address within the stage: a file path, a schema pointer, an
	// environment variable name, or the key itself.
	Source string `json:"source"`
	// File is the file the frame lives in, empty when it has none.
	File string `json:"file"`
	// Line is the 1-based line, 0 when unknown or not applicable.
	Line int `json:"line"`
}

// ExplainData is the payload of an explain event.
type ExplainData struct {
	// Template is the template the variable is referenced from.
	Template string `json:"template"`
	// Variable is the referenced path in canonical spelling.
	Variable string `json:"variable"`
	// Origin is the winning frame: the last step of the lineage.
	Origin OriginFrame `json:"origin"`
	// Shadowed lists the frames the winner covered, closest first.
	Shadowed []OriginFrame `json:"shadowed"`
}

// UnusedData is the payload of an unused event.
type UnusedData struct {
	// Path is the value-file leaf no template referenced.
	Path string `json:"path"`
}

// ErrorData is the payload of an error event.
type ErrorData struct {
	// Kind is the error classification.
	Kind string `json:"kind"`
	// ExitCode is the process exit code the error maps to.
	ExitCode int `json:"exitCode"`
	// Message is the rendered error text, including any file:line prefix.
	Message string `json:"message"`
}

// LogData is the payload of a log event.
type LogData struct {
	// Level is the slog level name, for example WARN.
	Level string `json:"level"`
	// Message is the log message.
	Message string `json:"message"`
	// Attrs holds the structured attributes, grouped by dotted key when slog
	// groups are used.
	Attrs map[string]any `json:"attrs"`
}
