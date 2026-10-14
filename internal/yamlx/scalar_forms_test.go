package yamlx_test

import (
	"reflect"
	"testing"

	"github.com/elizabevil/envsub/internal/codec"
	"github.com/elizabevil/envsub/internal/jsonx"
	"github.com/elizabevil/envsub/internal/yamlx"
)

// TestScalarFormsDecode pins the scalar spellings the decoder now parses
// directly from the resolved tag instead of going through node.Decode.
func TestScalarFormsDecode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		doc  string
		want any
	}{
		{doc: "v: 8080", want: 8080},
		{doc: "v: -3", want: -3},
		{doc: "v: 0x1F", want: 31},
		{doc: "v: true", want: true},
		{doc: "v: false", want: false},
		{doc: "v: NULL", want: nil},
		{doc: "v: ~", want: nil},
		{doc: "v: hello", want: "hello"},
		{doc: `v: "8080"`, want: "8080"},
		{doc: "v: 1.5", want: codec.LiteralNumber("1.5")},
		{doc: "v: 1e3", want: codec.LiteralNumber("1e3")},
		{doc: "v: 18446744073709551615", want: uint64(18446744073709551615)},
		{doc: "v: 123456789012345678901234567890", want: codec.ExactInt("123456789012345678901234567890")},
	}
	for _, tt := range tests {
		t.Run(tt.doc, func(t *testing.T) {
			t.Parallel()
			value, err := yamlx.Decode([]byte(tt.doc))
			if err != nil {
				t.Fatalf("Decode(%q) error = %v", tt.doc, err)
			}
			if got := value.(map[string]any)["v"]; !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Decode(%q) = %#v (%T), want %#v (%T)", tt.doc, got, got, tt.want, tt.want)
			}
		})
	}
}

// TestScalarYAMLOnlyFormsStayUsable covers spellings the direct parsers may not
// accept: they must decode without error and stay JSON-marshalable, whether the
// library resolves them as numbers or strings.
func TestScalarYAMLOnlyFormsStayUsable(t *testing.T) {
	t.Parallel()
	for _, doc := range []string{"v: .5", "v: 5.", "v: 1_000", "v: 1_000.5", "v: 0o17", "v: 0b101", "v: +.5"} {
		value, err := yamlx.Decode([]byte(doc))
		if err != nil {
			t.Fatalf("Decode(%q) error = %v", doc, err)
		}
		if _, err := jsonx.Marshal(value); err != nil {
			t.Fatalf("Decode(%q) is not JSON-marshalable: %v", doc, err)
		}
	}
}
