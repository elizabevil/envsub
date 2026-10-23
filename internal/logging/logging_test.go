package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/elizabevil/envsub/internal/event"
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
	// Text logs use the same UTC millisecond layout as the event stream.
	if !regexp.MustCompile(`time=\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{3}Z`).MatchString(buf.String()) {
		t.Fatalf("logger output = %q, want the shared UTC millisecond timestamp", buf.String())
	}
}

// TestEventLoggerWritesEnvelope pins that a log record becomes the same typed
// envelope as every other event, with its attributes under data.attrs.
func TestEventLoggerWritesEnvelope(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger, err := logging.NewEvent("debug", event.NewWriter(&buf))
	if err != nil {
		t.Fatalf("NewEvent() error = %v", err)
	}
	logger.Warn("hello", "answer", 42)

	var event struct {
		Schema int    `json:"schema"`
		Type   string `json:"type"`
		Time   string `json:"time"`
		Seq    uint64 `json:"seq"`
		Data   struct {
			Level   string         `json:"level"`
			Message string         `json:"message"`
			Attrs   map[string]any `json:"attrs"`
		} `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Fatalf("log line is not JSON: %v (%q)", err, buf.String())
	}
	if event.Schema != 1 || event.Type != "log" || event.Seq != 1 {
		t.Fatalf("envelope = %+v, want schema 1, type log, seq 1", event)
	}
	if _, err := time.Parse(time.RFC3339, event.Time); err != nil {
		t.Fatalf("time = %q, want RFC3339: %v", event.Time, err)
	}
	if event.Data.Level != "WARN" || event.Data.Message != "hello" {
		t.Fatalf("data = %+v, want WARN hello", event.Data)
	}
	if got := event.Data.Attrs["answer"]; got != float64(42) {
		t.Fatalf("attrs.answer = %#v, want 42", got)
	}
}
