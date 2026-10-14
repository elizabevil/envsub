package types_test

import (
	"encoding/json"
	"testing"

	"github.com/elizabevil/envsub/internal/codec"
	"github.com/elizabevil/envsub/internal/types"
)

// TestNormalizeKeepsBigIntegersExact pins the type-layer counterpart of
// values.NormalizeNumbers: integers above int64 stay exact (uint64, then the
// original json.Number) instead of being rounded through float64.
func TestNormalizeKeepsBigIntegersExact(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
		want any
	}{
		{name: "int", raw: "8080", want: 8080},
		{name: "beyond int64", raw: "12345678901234567890", want: uint64(12345678901234567890)},
		{name: "uint64 max", raw: "18446744073709551615", want: uint64(18446744073709551615)},
		{name: "beyond uint64", raw: "123456789012345678901234567890", want: codec.ExactInt("123456789012345678901234567890")},
		{name: "float", raw: "1.5", want: codec.LiteralNumber("1.5")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := types.Normalize(json.Number(tt.raw))
			if got != tt.want {
				t.Fatalf("Normalize(%s) = %#v (%T), want %#v (%T)", tt.raw, got, got, tt.want, tt.want)
			}
		})
	}
}

// TestStringifyBigIntegers checks the rendered text: exact digits, no rounding
// and no scientific notation.
func TestStringifyBigIntegers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "uint64", value: uint64(18446744073709551615), want: "18446744073709551615"},
		{name: "json.Number beyond uint64", value: json.Number("123456789012345678901234567890"), want: "123456789012345678901234567890"},
		{name: "codec.ExactInt", value: codec.ExactInt("123456789012345678901234567890"), want: "123456789012345678901234567890"},
		{name: "int", value: 8080, want: "8080"},
		{name: "float", value: 1.5, want: "1.5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := types.Stringify(tt.value)
			if err != nil {
				t.Fatalf("Stringify(%#v) error = %v", tt.value, err)
			}
			if got != tt.want {
				t.Fatalf("Stringify(%#v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

// TestAssertBigIntegerToInt checks the declared-type path: an integral value
// beyond int64 still satisfies `| type int` and renders exactly.
func TestAssertBigIntegerToInt(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"12345678901234567890", "18446744073709551615"} {
		value := types.Normalize(json.Number(raw))
		converted, err := types.Assert(value, types.Declared{Name: types.NameInt})
		if err != nil {
			t.Fatalf("Assert(%s, int) error = %v", raw, err)
		}
		text, err := types.Stringify(converted)
		if err != nil {
			t.Fatalf("Stringify(%s) error = %v", raw, err)
		}
		if text != raw {
			t.Fatalf("Assert(%s, int) rendered %q, want the exact value", raw, text)
		}
	}
}
