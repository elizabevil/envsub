package yamlx_test

import (
	"fmt"
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

// TestScalarFormsStayInTheValueGrammar is the contract test for scalar types: any
// scalar a YAML value file can carry must decode to a type the value tree, the
// schema matcher and both encoders understand. A type outside the set reaches the
// renderer and fails on every path except toJson/toYaml — time.Time did exactly
// that before date-like scalars were kept as strings.
func TestScalarFormsStayInTheValueGrammar(t *testing.T) {
	t.Parallel()
	// Every common YAML scalar form, grouped by the family it belongs to.
	docs := []string{
		// strings
		"v: hello", "v: 'single'", "v: \"double\"", "v: yes", "v: no", "v: on", "v: off",
		"v: 12:30:00", "v: 2024-01-01", "v: 2024-01-01T10:00:00Z", "v: 2024-01-01 10:00:00",
		"v: !!timestamp 2024-01-01", "v: !!str 2024-01-01", "v: \"2024-01-01\"",
		// booleans and null
		"v: true", "v: false", "v: True", "v: FALSE", "v: null", "v: Null", "v: NULL", "v: ~", "v:",
		// integers
		"v: 0", "v: -3", "v: 8080", "v: 0x1F", "v: 0o17", "v: 0b101", "v: 1_000",
		"v: 9223372036854775807", "v: -9223372036854775808", "v: 18446744073709551615",
		"v: 123456789012345678901234567890",
		// floats
		"v: 1.5", "v: 1.50", "v: 3.0", "v: -0.0", "v: .5", "v: 5.", "v: 1e3", "v: 1E+3", "v: +.5",
		// explicitly tagged and structured scalars
		"v: !!binary aGk=", "v: !!set {x, y}", "v: !!omap [{x: 1}]",
	}
	for _, doc := range docs {
		t.Run(doc, func(t *testing.T) {
			t.Parallel()
			doc := doc
			root, err := yamlx.Decode([]byte(doc))
			if err != nil {
				t.Fatalf("Decode(%q) error = %v", doc, err)
			}
			value, ok := root.(map[string]any)
			if !ok {
				t.Fatalf("Decode(%q) = %T, want a mapping", doc, root)
			}
			assertValueGrammarType(t, doc+": v", value["v"])
		})
	}
}

// TestNonFiniteScalarsAreRejected pins the one family that must not decode at all.
func TestNonFiniteScalarsAreRejected(t *testing.T) {
	t.Parallel()
	for _, doc := range []string{"v: .nan", "v: .NaN", "v: .inf", "v: -.Inf"} {
		if _, err := yamlx.Decode([]byte(doc)); err == nil {
			t.Errorf("Decode(%q) error = nil, want a rejection", doc)
		}
	}
}

// assertValueGrammarType fails when value is not one of the types the value
// grammar carries, recursing into the containers.
func assertValueGrammarType(t *testing.T, where string, value any) {
	t.Helper()
	switch typed := value.(type) {
	case nil, bool, string, int, uint64, float64, codec.ExactInt, codec.LiteralNumber:
		return
	case map[string]any:
		for key, child := range typed {
			assertValueGrammarType(t, where+"."+key, child)
		}
	case []any:
		for i, child := range typed {
			assertValueGrammarType(t, fmt.Sprintf("%s[%d]", where, i), child)
		}
	default:
		t.Errorf("%s decoded to %T, which the value grammar does not carry", where, value)
	}
}
