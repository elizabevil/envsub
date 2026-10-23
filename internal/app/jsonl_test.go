package app

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// jsonlEvent is the shared envelope plus the error payload the tests inspect.
type jsonlEvent struct {
	Schema int    `json:"schema"`
	Type   string `json:"type"`
	Time   string `json:"time"`
	Seq    uint64 `json:"seq"`
	Data   struct {
		Kind     string `json:"kind"`
		ExitCode int    `json:"exitCode"`
		Message  string `json:"message"`
	} `json:"data"`
}

func decodeJSONL(t *testing.T, stderr string) []jsonlEvent {
	t.Helper()
	var events []jsonlEvent
	for _, line := range strings.Split(strings.TrimSpace(stderr), "\n") {
		if line == "" {
			continue
		}
		var event jsonlEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("stderr line %q is not JSON: %v\nfull stderr:\n%s", line, err, stderr)
		}
		if event.Type == "" {
			t.Fatalf("stderr line %q has no type\nfull stderr:\n%s", line, stderr)
		}
		if event.Schema != 1 {
			t.Fatalf("stderr line %q schema = %d, want 1", line, event.Schema)
		}
		if _, err := time.Parse(time.RFC3339, event.Time); err != nil {
			t.Fatalf("stderr line %q time = %q, want RFC3339: %v", line, event.Time, err)
		}
		if event.Seq == 0 {
			t.Fatalf("stderr line %q seq = 0, want a positive sequence", line)
		}
		if len(events) > 0 && event.Seq <= events[len(events)-1].Seq {
			t.Fatalf("stderr seq %d is not after %d\nfull stderr:\n%s", event.Seq, events[len(events)-1].Seq, stderr)
		}
		events = append(events, event)
	}
	return events
}

// TestJSONLStreamOnSuccess pins that --explain implies JSONL: every stderr line
// is a typed explain record.
func TestJSONLStreamOnSuccess(t *testing.T) {
	dir := t.TempDir()
	tmpl := writeFile(t, dir, "input.tmpl", "${K}\n")
	_, stderr, code := captureRun(t, []string{
		"render", "--no-process-env", "--set", "K=v", "--explain", tmpl,
	}, "")
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr)
	}
	events := decodeJSONL(t, stderr)
	if len(events) != 1 || events[0].Type != "explain" {
		t.Fatalf("events = %+v, want one explain record", events)
	}
}

// TestJSONLStreamOnFailure pins that a render failure emits an explain record
// followed by a typed error, with the missing exit code preserved.
func TestJSONLStreamOnFailure(t *testing.T) {
	dir := t.TempDir()
	tmpl := writeFile(t, dir, "input.tmpl", "${MISSING}\n")
	_, stderr, code := captureRun(t, []string{"render", "--no-process-env", "--explain", tmpl}, "")
	if code != 6 {
		t.Fatalf("exit = %d, want 6; stderr=%q", code, stderr)
	}
	events := decodeJSONL(t, stderr)
	var seenExplain, seenError bool
	for _, event := range events {
		switch event.Type {
		case "explain":
			seenExplain = true
		case "error":
			seenError = true
			if event.Data.Kind != "missing" || event.Data.ExitCode != 6 {
				t.Fatalf("error event = %+v, want missing/6", event)
			}
			if !strings.Contains(event.Data.Message, "is not set") {
				t.Fatalf("error message = %q, want the missing message", event.Data.Message)
			}
		}
	}
	if !seenExplain || !seenError {
		t.Fatalf("events = %+v, want both explain and error", events)
	}
}

// TestJSONLBatchFailureEmitsOneErrorPerFailure pins that errors.Join is flattened
// into one line per template, not one line with embedded newlines.
func TestJSONLBatchFailureEmitsOneErrorPerFailure(t *testing.T) {
	dir := t.TempDir()
	first := writeFile(t, dir, "a.tmpl", "${MISSING_A}\n")
	second := writeFile(t, dir, "b.tmpl", "${MISSING_B}\n")
	_, stderr, code := captureRun(t, []string{
		"render", "--no-process-env", "--explain", first, second,
	}, "")
	if code != 6 {
		t.Fatalf("exit = %d, want 6; stderr=%q", code, stderr)
	}
	events := decodeJSONL(t, stderr)
	var errors []jsonlEvent
	for _, event := range events {
		if event.Type == "error" {
			errors = append(errors, event)
		}
	}
	if len(errors) != 2 {
		t.Fatalf("error events = %+v, want one per template", errors)
	}
}

// TestJSONLUsageError covers the error path that never reaches the render
// pipeline: an unknown subcommand must still be a typed JSONL error.
func TestJSONLUsageError(t *testing.T) {
	_, stderr, code := captureRun(t, []string{"--jsonl", "bogus"}, "")
	if code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%q", code, stderr)
	}
	events := decodeJSONL(t, stderr)
	if len(events) != 1 || events[0].Type != "error" || events[0].Data.Kind != "usage" {
		t.Fatalf("events = %+v, want one usage error", events)
	}
}

// TestJSONLFalseOptsOutOfTheImplicitMode pins the escape hatch: --jsonl=false
// keeps human-readable errors even with --explain.
func TestJSONLFalseOptsOutOfTheImplicitMode(t *testing.T) {
	dir := t.TempDir()
	tmpl := writeFile(t, dir, "input.tmpl", "${MISSING}\n")
	_, stderr, code := captureRun(t, []string{
		"render", "--no-process-env", "--explain", "--jsonl=false", tmpl,
	}, "")
	if code != 6 {
		t.Fatalf("exit = %d, want 6; stderr=%q", code, stderr)
	}
	if !strings.Contains(stderr, `variable "MISSING" is not set`) {
		t.Fatalf("stderr = %q, want the text error when --jsonl=false", stderr)
	}
	// The explain line is still JSON; the deliberate opt-out is the text error.
	if strings.Contains(stderr, `"type":"error"`) {
		t.Fatalf("stderr = %q, want no JSON error event when --jsonl=false", stderr)
	}
}

// TestJSONLLogEvent pins that warnings share the stream with a type discriminator
// instead of leaking slog text into it.
func TestJSONLLogEvent(t *testing.T) {
	dir := t.TempDir()
	schema := writeFile(t, dir, "schema.json",
		`{"type":"object","properties":{"K":{"type":"string","bogus":1}}}`)
	tmpl := writeFile(t, dir, "input.tmpl", "${K}\n")
	_, stderr, code := captureRun(t, []string{
		"render", "--no-process-env", "--set", "K=v", "--schema", schema,
		"--unknown-keywords=warn", "--jsonl", tmpl,
	}, "")
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr)
	}
	events := decodeJSONL(t, stderr)
	if len(events) == 0 || events[0].Type != "log" {
		t.Fatalf("events = %+v, want a log record", events)
	}
}

// TestJSONLUnusedEvent pins that --report=unused implies JSONL and emits typed
// path records.
func TestJSONLUnusedEvent(t *testing.T) {
	dir := t.TempDir()
	values := writeFile(t, dir, "values.yaml", "used: 1\ndead: 2\n")
	tmpl := writeFile(t, dir, "input.tmpl", "${used}\n")
	_, stderr, code := captureRun(t, []string{
		"render", "--no-process-env", "-f", values, "--report=unused", tmpl,
	}, "")
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr)
	}
	events := decodeJSONL(t, stderr)
	if len(events) != 1 || events[0].Type != "unused" {
		t.Fatalf("events = %+v, want one unused record", events)
	}
}

// TestJSONLExplicitWithoutExplain pins that --jsonl alone is enough to make an
// ordinary failure machine-readable.
func TestJSONLExplicitWithoutExplain(t *testing.T) {
	dir := t.TempDir()
	tmpl := writeFile(t, dir, "input.tmpl", "${MISSING}\n")
	_, stderr, code := captureRun(t, []string{
		"render", "--no-process-env", "--jsonl", tmpl,
	}, "")
	if code != 6 {
		t.Fatalf("exit = %d, want 6; stderr=%q", code, stderr)
	}
	events := decodeJSONL(t, stderr)
	if len(events) != 1 || events[0].Type != "error" || events[0].Data.Kind != "missing" {
		t.Fatalf("events = %+v, want one missing error", events)
	}
}

// TestJSONLFalseKeepsUnusedText pins that the opt-out also restores the text
// unused report, so callers that need the old human format are not forced into
// the machine stream.
func TestJSONLFalseKeepsUnusedText(t *testing.T) {
	dir := t.TempDir()
	values := writeFile(t, dir, "values.yaml", "used: 1\ndead: 2\n")
	tmpl := writeFile(t, dir, "input.tmpl", "${used}\n")
	_, stderr, code := captureRun(t, []string{
		"render", "--no-process-env", "-f", values, "--report=unused", "--jsonl=false", tmpl,
	}, "")
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr)
	}
	if stderr != "dead\n" {
		t.Fatalf("stderr = %q, want the text unused report", stderr)
	}
}

// TestJSONLSurvivesFlagParseError pins the pre-parse fallback: even when an
// unknown flag aborts parsing, an explicit --jsonl still yields a JSON error.
func TestJSONLSurvivesFlagParseError(t *testing.T) {
	_, stderr, code := captureRun(t, []string{"render", "--jsonl", "--no-such-flag", "-"}, "")
	if code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%q", code, stderr)
	}
	events := decodeJSONL(t, stderr)
	if len(events) != 1 || events[0].Type != "error" || events[0].Data.Kind != "usage" {
		t.Fatalf("events = %+v, want one usage error", events)
	}
}
