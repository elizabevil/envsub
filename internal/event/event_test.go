package event_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/elizabevil/envsub/internal/event"
)

// TestWriterEmitsEnvelope pins the shared line shape and monotonic sequence.
func TestWriterEmitsEnvelope(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	clock := func() time.Time {
		return time.Date(2026, 10, 22, 23, 43, 37, 41_000_000, time.UTC)
	}
	writer := event.NewWriterWithClock(&buf, clock)
	if err := writer.Emit(event.TypeUnused, event.UnusedData{Path: "app.unused"}); err != nil {
		t.Fatalf("Emit() error = %v", err)
	}
	if err := writer.Emit(event.TypeError, event.ErrorData{Kind: "missing", ExitCode: 6, Message: "nope"}); err != nil {
		t.Fatalf("Emit() error = %v", err)
	}

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(lines))
	}
	for i, line := range lines {
		var decoded struct {
			Schema int             `json:"schema"`
			Type   string          `json:"type"`
			Time   string          `json:"time"`
			Seq    uint64          `json:"seq"`
			Data   json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(line, &decoded); err != nil {
			t.Fatalf("line %q is not JSON: %v", line, err)
		}
		if decoded.Schema != event.Schema {
			t.Fatalf("schema = %d, want %d", decoded.Schema, event.Schema)
		}
		if decoded.Time != "2026-10-22T23:43:37.041Z" {
			t.Fatalf("time = %q, want the shared format", decoded.Time)
		}
		if decoded.Seq != uint64(i+1) {
			t.Fatalf("seq = %d, want %d", decoded.Seq, i+1)
		}
		if len(decoded.Data) == 0 || decoded.Data[0] != '{' {
			t.Fatalf("data = %s, want an object", decoded.Data)
		}
	}
}

// TestWriterUsesExplicitLogTime pins that a record keeps its own timestamp even
// if the writer is called later.
func TestWriterUsesExplicitLogTime(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	writer := event.NewWriter(&buf)
	at := time.Date(2026, 1, 2, 3, 4, 5, 6_000_000, time.UTC)
	if err := writer.EmitAt(at, event.TypeLog, event.LogData{Level: "WARN", Message: "hello", Attrs: map[string]any{}}); err != nil {
		t.Fatalf("EmitAt() error = %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"time":"2026-01-02T03:04:05.006Z"`)) {
		t.Fatalf("output = %q, want the explicit record time", buf.String())
	}
}

// TestContextRoundTrip pins how app and CLI hand the one writer to the renderer.
func TestContextRoundTrip(t *testing.T) {
	t.Parallel()
	writer := event.NewWriter(&bytes.Buffer{})
	ctx := event.WithWriter(context.Background(), writer)
	if got := event.FromContext(ctx); got != writer {
		t.Fatalf("FromContext() = %p, want %p", got, writer)
	}
	var nilContext context.Context
	if got := event.FromContext(nilContext); got != nil {
		t.Fatalf("FromContext(nil context) = %v, want nil", got)
	}
}

// TestWriterWithoutDestination pins the error path instead of a panic.
func TestWriterWithoutDestination(t *testing.T) {
	t.Parallel()
	writer := event.NewWriter(nil)
	if err := writer.Emit(event.TypeError, event.ErrorData{}); err == nil {
		t.Fatal("Emit() error = nil, want a destination error")
	}
}
