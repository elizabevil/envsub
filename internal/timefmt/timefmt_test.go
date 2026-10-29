package timefmt_test

import (
	"strings"
	"testing"
	"time"

	"github.com/elizabevil/envsub/internal/timefmt"
)

// TestFormatUsesUTCAndRFC3339Seconds pins the single timestamp shape: a
// non-UTC input becomes Zulu time with no fractional seconds.
func TestFormatUsesUTCAndRFC3339Seconds(t *testing.T) {
	t.Parallel()
	input := time.Date(2026, 10, 22, 19, 43, 37, 41_000_000, time.FixedZone("EDT", -4*60*60))
	if got, want := timefmt.Format(input), "2026-10-22T23:43:37Z"; got != want {
		t.Fatalf("Format(%v) = %q, want %q", input, got, want)
	}
}

// TestFormatDropsSubsecondPrecision pins that event ordering does not depend on
// fractional seconds: the timestamp is second-precision and seq is the order.
func TestFormatDropsSubsecondPrecision(t *testing.T) {
	t.Parallel()
	input := time.Date(2026, 10, 22, 23, 43, 37, 123_456_789, time.UTC)
	got := timefmt.Format(input)
	if got != "2026-10-22T23:43:37Z" {
		t.Fatalf("Format() = %q, want second precision", got)
	}
	if strings.Contains(got, ".") {
		t.Fatalf("Format() = %q, want no fractional part", got)
	}
	if _, err := time.Parse(time.RFC3339, got); err != nil {
		t.Fatalf("Format() = %q is not RFC3339: %v", got, err)
	}
}
