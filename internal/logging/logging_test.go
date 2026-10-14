package logging_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/logging"
)

func TestParseLevel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
		want slog.Level
	}{
		{name: "debug", raw: "debug", want: slog.LevelDebug},
		{name: "info", raw: "INFO", want: slog.LevelInfo},
		{name: "warn", raw: "warn", want: slog.LevelWarn},
		{name: "error", raw: "error", want: slog.LevelError},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := logging.ParseLevel(tt.raw)
			if err != nil {
				t.Fatalf("ParseLevel(%q) error = %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("ParseLevel(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestLoggerRoundTrip(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger, err := logging.New("debug", &buf)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx := logging.WithLogger(context.Background(), logger)
	logging.FromContext(ctx).Debug("hello", "answer", 42)
	if !strings.Contains(buf.String(), "hello") {
		t.Fatalf("logger output = %q, want it to contain hello", buf.String())
	}
}
