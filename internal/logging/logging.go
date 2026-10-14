// Package logging builds the process logger and carries it through contexts.
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
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
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: lvl})), nil
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
