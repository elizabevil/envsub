package encoding_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/jsonx"
	"github.com/elizabevil/envsub/internal/types"
	"github.com/elizabevil/envsub/internal/values"
)

// TestNumberNormalizationPathsAgree pins the two numeric normalization paths
// against each other:
//
//   - values.NormalizeNumbers runs at load time, converting a decoded json.Number
//     into the value tree's canonical Go number;
//   - types.Normalize runs at render time on a single value that may already be a
//     native Go number.
//
// They are not duplicates: the first walks a whole document, the second normalizes
// one value for formatting. What must not diverge is the result, so both inputs to
// the same JSON text have to end up as the same Go value.
//
// The table also pins what the load-time normalizer guarantees on its own, so the
// agreement check is not its only assertion: an integer beyond uint64 stays exact
// decimal text (codec.ExactInt) and a fractional or exponent literal keeps its own
// spelling (codec.LiteralNumber) instead of being widened to float64 (rules N1/F6).
func TestNumberNormalizationPathsAgree(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		text     string
		wantType string
		wantText string
	}{
		{name: "small integer", text: `42`, wantType: "int"},
		{name: "negative integer", text: `-42`, wantType: "int"},
		{name: "max int64", text: `9223372036854775807`, wantType: "int"},
		{name: "max int64 plus one", text: `9223372036854775808`, wantType: "uint64"},
		{name: "max uint64", text: `18446744073709551615`, wantType: "uint64"},
		{name: "beyond uint64", text: `18446744073709551616`, wantType: "codec.ExactInt", wantText: "18446744073709551616"},
		{name: "negative beyond int64", text: `-9223372036854775809`, wantType: "codec.ExactInt", wantText: "-9223372036854775809"},
		{name: "fraction", text: `0.5`, wantType: "codec.LiteralNumber", wantText: "0.5"},
		{name: "high precision fraction", text: `1.0000000000000000001`, wantType: "codec.LiteralNumber", wantText: "1.0000000000000000001"},
		{name: "exponent", text: `1e10`, wantType: "codec.LiteralNumber", wantText: "1e10"},
		{name: "negative zero", text: `-0.0`, wantType: "codec.LiteralNumber", wantText: "-0.0"},
		{name: "zero", text: `0`, wantType: "int"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Both decoders are asked to preserve the number as text, which is the
			// only form that keeps values beyond 2^53 exact.
			var fromStdlib any
			decoder := json.NewDecoder(strings.NewReader(tt.text))
			decoder.UseNumber()
			if err := decoder.Decode(&fromStdlib); err != nil {
				t.Fatalf("stdlib Decode(%q) error = %v", tt.text, err)
			}
			fromProject, err := jsonx.UnmarshalValue([]byte(tt.text))
			if err != nil {
				t.Fatalf("jsonx.UnmarshalValue(%q) error = %v", tt.text, err)
			}

			// The load-time normalizer runs over each decoded tree, and the
			// render-time normalizer must leave the canonical value alone.
			loadTime := values.NormalizeNumbers(fromStdlib)
			renderTime := types.Normalize(values.NormalizeNumbers(fromProject))

			if !reflect.DeepEqual(loadTime, renderTime) {
				t.Fatalf("normalization disagrees for %s: stdlib = %#v (%T), project = %#v (%T)",
					tt.text, loadTime, loadTime, renderTime, renderTime)
			}
			if !reflect.DeepEqual(loadTime, types.Normalize(loadTime)) {
				t.Fatalf("render-time normalization changed the canonical value for %s: %#v -> %#v",
					tt.text, loadTime, types.Normalize(loadTime))
			}

			// Exactly what the load-time normalizer produced: the type family
			// above and, for the text-backed types, the spelling it kept.
			if got := reflect.TypeOf(loadTime).String(); got != tt.wantType {
				t.Fatalf("NormalizeNumbers(%s) type = %s, want %s", tt.text, got, tt.wantType)
			}
			if tt.wantText != "" {
				if got := numberText(loadTime); got != tt.wantText {
					t.Fatalf("NormalizeNumbers(%s) text = %q, want %q", tt.text, got, tt.wantText)
				}
			}
		})
	}
}

// numberText returns the decimal spelling of a normalized number value.
func numberText(value any) string {
	type stringer interface{ String() string }
	if s, ok := value.(stringer); ok {
		return s.String()
	}
	return ""
}
