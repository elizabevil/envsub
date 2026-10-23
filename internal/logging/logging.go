// Package logging builds the process logger and carries it through contexts.
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/elizabevil/envsub/internal/event"
	"github.com/elizabevil/envsub/internal/timefmt"
)

// Log level names accepted by --log-level.
const (
	LevelDebugName = "debug"
	LevelInfoName  = "info"
	LevelWarnName  = "warn"
	LevelErrorName = "error"
	// levelWarnAlias is the long spelling accepted for warn.
	levelWarnAlias = "warning"
)

type contextKey struct{}

// New creates a text slog logger for the given level.
func New(level string, w io.Writer) (*slog.Logger, error) {
	lvl, err := ParseLevel(level)
	if err != nil {
		return nil, err
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: lvl, ReplaceAttr: formatTextTime})), nil
}

// formatTextTime makes the human-readable text log use the same timestamp as the
// JSONL events: UTC with fixed milliseconds. Without it slog's TextHandler would
// format time itself (local zone, millisecond truncation) while JSON used
// RFC3339Nano, which is exactly the drift timefmt exists to prevent.
func formatTextTime(groups []string, attr slog.Attr) slog.Attr {
	if len(groups) == 0 && attr.Key == slog.TimeKey {
		return slog.String(slog.TimeKey, timefmt.Format(attr.Value.Time()))
	}
	return attr
}

// NewEvent creates a logger that writes log events through the shared event
// writer, so logs carry the same envelope, timestamp and sequence as explain,
// unused and error records.
func NewEvent(level string, events *event.Writer) (*slog.Logger, error) {
	lvl, err := ParseLevel(level)
	if err != nil {
		return nil, err
	}
	if events == nil {
		return nil, fmt.Errorf("log event writer is nil")
	}
	return slog.New(&eventHandler{events: events, level: lvl}), nil
}

// eventHandler converts slog records into event.LogData payloads.
type eventHandler struct {
	events *event.Writer
	level  slog.Level
	attrs  []slog.Attr
	groups []string
}

func (h *eventHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *eventHandler) Handle(_ context.Context, record slog.Record) error {
	attrs := make(map[string]any, len(h.attrs)+record.NumAttrs())
	for _, attr := range h.attrs {
		appendLogAttr(attrs, h.groups, attr)
	}
	record.Attrs(func(attr slog.Attr) bool {
		appendLogAttr(attrs, h.groups, attr)
		return true
	})
	t := record.Time
	if t.IsZero() {
		t = time.Now()
	}
	return h.events.EmitAt(t, event.TypeLog, event.LogData{
		Level:   record.Level.String(),
		Message: record.Message,
		Attrs:   attrs,
	})
}

func (h *eventHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &clone
}

func (h *eventHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	clone.groups = append(append([]string{}, h.groups...), name)
	return &clone
}

// appendLogAttr flattens slog groups into dotted attribute keys, so the event
// payload has one stable attrs object regardless of how the caller logged.
func appendLogAttr(attrs map[string]any, groups []string, attr slog.Attr) {
	if attr.Key == "" {
		return
	}
	if attr.Value.Kind() == slog.KindGroup {
		for _, child := range attr.Value.Group() {
			appendLogAttr(attrs, append(groups, attr.Key), child)
		}
		return
	}
	attrs[strings.Join(append(groups, attr.Key), ".")] = attr.Value.Any()
}

// ParseLevel parses a CLI log level.
//
// slog.Level.UnmarshalText is not used: it accepts neither the "warning" alias
// nor an empty string as the warn default, and its error message differs from
// the one this CLI reports for an unknown level.
func ParseLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case LevelDebugName:
		return slog.LevelDebug, nil
	case LevelInfoName:
		return slog.LevelInfo, nil
	case LevelWarnName, levelWarnAlias, "":
		return slog.LevelWarn, nil
	case LevelErrorName:
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unknown log level %q", raw)
	}
}

// WithLogger returns a context carrying the logger.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, logger)
}

// FromContext returns the logger stored in ctx, or a no-op logger.
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(contextKey{}).(*slog.Logger); ok {
		return logger
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
