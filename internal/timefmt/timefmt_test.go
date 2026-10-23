package timefmt_test

import (
	"strings"
	"testing"
	"time"

	"github.com/elizabevil/envsub/internal/timefmt"
)

// TestFormatUsesUTCAndFixedMilliseconds pins the single event timestamp shape:
// a non-UTC input becomes Zulu time with exactly three fractional digits.
func TestFormatUsesUTCAndFixedMilliseconds(t *testing.T) {
	t.Parallel()
	input := time.Date(2026, 10, 22, 19, 43, 37, 41_000_000, time.FixedZone("EDT", -4*60*60))
	if got, want := timefmt.Format(input), "2026-10-22T23:43:37.041Z"; got != want {
		t.Fatalf("Format(%v) = %q, want %q", input, got, want)
	}
}

// TestFormatKeepsFixedWidth pins that nanoseconds never leak into the string:
// the shared layout is what makes lexicographic order chronological.
func TestFormatKeepsFixedWidth(t *testing.T) {
	t.Parallel()
	input := time.Date(2026, 10, 22, 23, 43, 37, 123_456_789, time.UTC)
	got := timefmt.Format(input)
	if !strings.HasSuffix(got, ".123Z") {
		t.Fatalf("Format() = %q, want millisecond truncation and Z", got)
	}
	if _, err := time.Parse(time.RFC3339, got); err != nil {
		t.Fatalf("Format() = %q is not RFC3339: %v", got, err)
	}
}
