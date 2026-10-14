package values_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/elizabevil/envsub/internal/codec"
	"github.com/elizabevil/envsub/internal/values"
)

// TestNormalizeNumbersKeepsIntegersExact pins the numeric normalization: an
// integer decoded from JSON keeps its exact value instead of being widened to
// float64, which would round anything above 2^53.
func TestNormalizeNumbersKeepsIntegersExact(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
		want any
	}{
		{name: "int", raw: "8080", want: 8080},
		{name: "negative", raw: "-3", want: -3},
		{name: "int64 max", raw: "9223372036854775807", want: 9223372036854775807},
		{name: "beyond int64", raw: "9223372036854775808", want: uint64(9223372036854775808)},
		{name: "uint64 max", raw: "18446744073709551615", want: uint64(18446744073709551615)},
		{name: "beyond uint64", raw: "123456789012345678901234567890", want: codec.ExactInt("123456789012345678901234567890")},
		{name: "float", raw: "1.5", want: codec.LiteralNumber("1.5")},
		{name: "exponent", raw: "1e3", want: codec.LiteralNumber("1e3")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := values.NormalizeNumbers(json.Number(tt.raw))
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("NormalizeNumbers(%s) = %#v (%T), want %#v (%T)", tt.raw, got, got, tt.want, tt.want)
			}
		})
	}
}

// TestNormalizeNumbersWalksContainers checks that nested values are normalized
// too, which is what a JSON document or --set-json produces.
func TestNormalizeNumbersWalksContainers(t *testing.T) {
	t.Parallel()
	got := values.NormalizeNumbers(map[string]any{
		"list": []any{json.Number("18446744073709551615")},
		"nested": map[string]any{
			"id": json.Number("12345678901234567890"),
		},
	})
	want := map[string]any{
		"list": []any{uint64(18446744073709551615)},
		"nested": map[string]any{
			"id": uint64(12345678901234567890),
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeNumbers() = %#v, want %#v", got, want)
	}
}
