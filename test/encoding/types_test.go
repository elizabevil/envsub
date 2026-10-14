// Package encoding_test: step 8 of the serialization rules — a row-by-row
// matrix over the value classes of §3 (null, bool, string, int, uint64, exact
// integer, LiteralNumber, float64, array, object) covering decode, channels,
// materialization, operator emptiness and schema validation.
//
// Dimensions already pinned elsewhere are referenced instead of duplicated:
// backend equivalence in backend_*_test.go, encoder round trips in
// internal/jsonx and internal/yamlx, and the rule table in rules_test.go.
package encoding_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/codec"
	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/engine/extended"
	"github.com/elizabevil/envsub/internal/jsonx"
	"github.com/elizabevil/envsub/internal/schema"
	"github.com/elizabevil/envsub/internal/types"
	"github.com/elizabevil/envsub/internal/values"
	"github.com/elizabevil/envsub/internal/yamlx"
)

// renderTemplate renders one template against a value tree. It is shared with
// rules_test.go so the rule rows and the type matrix exercise the same path.
func renderTemplate(t *testing.T, tmpl string, tree *values.Tree) (string, error) {
	t.Helper()
	out, err := extended.New().Render(context.Background(), engine.Input{
		Template: []byte(tmpl),
		Values:   tree,
	})
	return string(out), err
}

func treeWith(x any) *values.Tree {
	tree := values.NewTree()
	tree.Root["x"] = x
	return tree
}

// TestTypeMatrixDecode covers the JSON and YAML decode dimensions of §3.
func TestTypeMatrixDecode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		jsonText string
		yamlText string
		want     any
	}{
		{name: "null", jsonText: `{"x":null}`, yamlText: "x: null\n", want: nil},
		{name: "bool", jsonText: `{"x":true}`, yamlText: "x: true\n", want: true},
		{name: "string", jsonText: `{"x":"hello"}`, yamlText: "x: hello\n", want: "hello"},
		{name: "int", jsonText: `{"x":42}`, yamlText: "x: 42\n", want: 42},
		{name: "int64 max", jsonText: `{"x":9223372036854775807}`, yamlText: "x: 9223372036854775807\n", want: 9223372036854775807},
		{name: "uint64 max", jsonText: `{"x":18446744073709551615}`, yamlText: "x: 18446744073709551615\n", want: uint64(18446744073709551615)},
		{name: "exact integer", jsonText: `{"x":123456789012345678901234567890}`, yamlText: "x: 123456789012345678901234567890\n", want: codec.ExactInt("123456789012345678901234567890")},
		{name: "LiteralNumber fraction", jsonText: `{"x":1.50}`, yamlText: "x: 1.50\n", want: codec.LiteralNumber("1.50")},
		{name: "LiteralNumber integral", jsonText: `{"x":1.0}`, yamlText: "x: 1.0\n", want: codec.LiteralNumber("1.0")},
		{name: "LiteralNumber exponent", jsonText: `{"x":1e3}`, yamlText: "x: 1e3\n", want: codec.LiteralNumber("1e3")},
		{name: "float64 from a YAML-only spelling", yamlText: "x: .5\n", want: 0.5},
		{name: "array", jsonText: `{"x":[1,2]}`, yamlText: "x: [1, 2]\n", want: []any{1, 2}},
		{name: "object", jsonText: `{"x":{"b":1,"a":2}}`, yamlText: "x: {b: 1, a: 2}\n", want: map[string]any{"a": 2, "b": 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.jsonText != "" {
				decoded, err := jsonx.UnmarshalValue([]byte(tt.jsonText))
				if err != nil {
					t.Fatalf("jsonx.UnmarshalValue() error = %v", err)
				}
				object, ok := values.NormalizeNumbers(decoded).(map[string]any)
				if !ok {
					t.Fatalf("JSON decode = %T, want map[string]any", decoded)
				}
				if !reflect.DeepEqual(object["x"], tt.want) {
					t.Fatalf("JSON x = %#v (%T), want %#v (%T)", object["x"], object["x"], tt.want, tt.want)
				}
			}
			if tt.yamlText != "" {
				decoded, err := yamlx.Decode([]byte(tt.yamlText))
				if err != nil {
					t.Fatalf("yamlx.Decode() error = %v", err)
				}
				object, ok := decoded.(map[string]any)
				if !ok {
					t.Fatalf("YAML decode = %T, want map[string]any", decoded)
				}
				if !reflect.DeepEqual(object["x"], tt.want) {
					t.Fatalf("YAML x = %#v (%T), want %#v (%T)", object["x"], object["x"], tt.want, tt.want)
				}
			}
		})
	}
}

// TestTypeMatrixChannels covers `${x}`, `| toJson`, `| toYaml` and the operator
// emptiness dimension of §3.
func TestTypeMatrixChannels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		value   any
		plain   string
		wantErr bool
		toJSON  string
		toYAML  string
		empty   bool
	}{
		{name: "null", value: nil, plain: "", toJSON: "null", toYAML: "null", empty: true},
		{name: "bool", value: true, plain: "true", toJSON: "true", toYAML: "true"},
		{name: "bool false is not empty", value: false, plain: "false", toJSON: "false", toYAML: "false"},
		{name: "string", value: "hello", plain: "hello", toJSON: `"hello"`, toYAML: "hello"},
		{name: "empty string is empty", value: "", plain: "", toJSON: `""`, toYAML: `""`, empty: true},
		{name: "int", value: 42, plain: "42", toJSON: "42", toYAML: "42"},
		{name: "zero is not empty", value: 0, plain: "0", toJSON: "0", toYAML: "0"},
		{name: "uint64", value: uint64(18446744073709551615), plain: "18446744073709551615", toJSON: "18446744073709551615", toYAML: "18446744073709551615"},
		{name: "exact integer is not empty", value: codec.ExactInt("123456789012345678901234567890"), plain: "123456789012345678901234567890", toJSON: "123456789012345678901234567890", toYAML: "123456789012345678901234567890"},
		{name: "LiteralNumber", value: codec.LiteralNumber("1.50"), plain: "1.50", toJSON: "1.50", toYAML: "1.50"},
		{name: "float64", value: 0.5, plain: "0.5", toJSON: "0.5", toYAML: "0.5"},
		{name: "array", value: []any{1, 2}, wantErr: true, toJSON: "[1,2]", toYAML: "- 1\n- 2"},
		{name: "empty array is empty", value: []any{}, wantErr: true, toJSON: "[]", toYAML: "[]", empty: true},
		{name: "object", value: map[string]any{"b": 1, "a": 2}, wantErr: true, toJSON: `{"a":2,"b":1}`, toYAML: "a: 2\nb: 1"},
		{name: "empty object is empty", value: map[string]any{}, wantErr: true, toJSON: "{}", toYAML: "{}", empty: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tree := treeWith(tt.value)

			got, err := renderTemplate(t, "${x}", tree)
			switch {
			case tt.wantErr && err == nil:
				t.Fatal("${x} error = nil, want an error")
			case !tt.wantErr && err != nil:
				t.Fatalf("${x} error = %v", err)
			case !tt.wantErr && got != tt.plain:
				t.Fatalf("${x} = %q, want %q", got, tt.plain)
			}

			got, err = renderTemplate(t, "${x | toJson}", tree)
			if err != nil {
				t.Fatalf("${x | toJson} error = %v", err)
			}
			if got != tt.toJSON {
				t.Fatalf("${x | toJson} = %q, want %q", got, tt.toJSON)
			}

			got, err = renderTemplate(t, "${x | toYaml}", tree)
			if err != nil {
				t.Fatalf("${x | toYaml} error = %v", err)
			}
			if got != tt.toYAML {
				t.Fatalf("${x | toYaml} = %q, want %q", got, tt.toYAML)
			}

			got, err = renderTemplate(t, "d=${x:-d}", tree)
			switch {
			case tt.wantErr && !tt.empty:
				// A structured value still needs a formatter when the operator
				// passes it through.
				if err == nil {
					t.Fatalf("${x:-d} error = nil, want an error")
				}
			case err != nil:
				t.Fatalf("${x:-d} error = %v", err)
			case tt.empty:
				if got != "d=d" {
					t.Fatalf("${x:-d} = %q, want %q", got, "d=d")
				}
			default:
				if want := "d=" + tt.plain; got != want {
					t.Fatalf("${x:-d} = %q, want %q", got, want)
				}
			}
		})
	}
}

// TestTypeMatrixMaterialize covers the `.env`/`--set` materialization dimension
// through both declared forms (§8 `| type`) and schema (`Materialize`).
func TestTypeMatrixMaterialize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		raw        string
		inline     string
		schemaType string
		wantJSON   string
	}{
		{name: "int via inline", raw: "8080", inline: "int", wantJSON: "8080"},
		{name: "int via schema", raw: "8080", schemaType: "integer", wantJSON: "8080"},
		{name: "float via inline", raw: "1.50", inline: "float", wantJSON: "1.5"},
		{name: "number via schema", raw: "1.50", schemaType: "number", wantJSON: "1.5"},
		{name: "bool via inline", raw: "true", inline: "bool", wantJSON: "true"},
		{name: "boolean via schema", raw: "true", schemaType: "boolean", wantJSON: "true"},
		{name: "array via inline", raw: `[1,2]`, inline: "array<int>", wantJSON: "[1,2]"},
		{name: "array via schema", raw: `[1,2]`, schemaType: "array", wantJSON: "[1,2]"},
		{name: "object via schema", raw: `{"b":1,"a":2}`, schemaType: "object", wantJSON: `{"a":2,"b":1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.inline != "" {
				tree := treeWith(tt.raw)
				got, err := renderTemplate(t, "${x | type "+tt.inline+" | toJson}", tree)
				if err != nil {
					t.Fatalf("inline materialization error = %v", err)
				}
				if got != tt.wantJSON {
					t.Fatalf("inline toJson = %q, want %q", got, tt.wantJSON)
				}
			}
			if tt.schemaType != "" {
				sch := loadSchema(t, tt.schemaType)
				tree := treeWith(tt.raw)
				if err := sch.Materialize(tree); err != nil {
					t.Fatalf("Materialize() error = %v", err)
				}
				got, err := renderTemplate(t, "${x | toJson}", tree)
				if err != nil {
					t.Fatalf("schema materialization render error = %v", err)
				}
				if got != tt.wantJSON {
					t.Fatalf("schema toJson = %q, want %q", got, tt.wantJSON)
				}
			}
		})
	}
}

// TestTypeMatrixSchema covers the schema validation dimension with one passing
// and one failing row per class family.
func TestTypeMatrixSchema(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		value      any
		schemaType string
		wantErr    bool
	}{
		{name: "null", value: nil, schemaType: "null"},
		{name: "bool", value: true, schemaType: "boolean"},
		{name: "string", value: "hello", schemaType: "string"},
		{name: "int", value: 42, schemaType: "integer"},
		{name: "uint64", value: uint64(18446744073709551615), schemaType: "integer"},
		{name: "exact integer", value: codec.ExactInt("123456789012345678901234567890"), schemaType: "integer"},
		{name: "LiteralNumber", value: codec.LiteralNumber("1.50"), schemaType: "number"},
		{name: "float64", value: 0.5, schemaType: "number"},
		{name: "array", value: []any{1, 2}, schemaType: "array"},
		{name: "object", value: map[string]any{"a": 1}, schemaType: "object"},
		{name: "int rejects a string", value: codec.LiteralNumber("1.50"), schemaType: "integer", wantErr: true},
		{name: "string rejects a number", value: 42, schemaType: "string", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sch := loadSchema(t, tt.schemaType)
			err := sch.Validate(treeWith(tt.value))
			if tt.wantErr && err == nil {
				t.Fatal("Validate() error = nil, want an error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

// TestTypeMatrixBoundaries pins the §3 boundary rows explicitly, including what
// `| type float` does with each one: an integer that float64 cannot hold exactly
// and a literal outside its range are rejected (rules F7/FB3), while -0.0 narrows
// and keeps its sign.
func TestTypeMatrixBoundaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		value     any
		want      string
		wantFloat string
		floatErr  bool
	}{
		{name: "int64 max", value: 9223372036854775807, want: "9223372036854775807", floatErr: true},
		{name: "uint64 max", value: uint64(18446744073709551615), want: "18446744073709551615", floatErr: true},
		{name: "beyond uint64", value: codec.ExactInt("18446744073709551616"), want: "18446744073709551616", floatErr: true},
		{name: "literal beyond float64 range", value: codec.LiteralNumber("1e309"), want: "1e309", floatErr: true},
		{name: "negative zero literal", value: codec.LiteralNumber("-0.0"), want: "-0.0", wantFloat: "-0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := renderTemplate(t, "${x}", treeWith(tt.value))
			if err != nil {
				t.Fatalf("${x} error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("${x} = %q, want %q", got, tt.want)
			}
			// A declared float either narrows or is rejected when the value
			// cannot be held exactly (rules F7/FB3).
			narrowed, err := renderTemplate(t, "${x | type float}", treeWith(tt.value))
			switch {
			case tt.floatErr:
				if err == nil {
					t.Fatalf("${x | type float} = %q, want an exactness or range error", narrowed)
				}
			case err != nil:
				t.Fatalf("${x | type float} error = %v", err)
			case narrowed != tt.wantFloat:
				t.Fatalf("${x | type float} = %q, want %q", narrowed, tt.wantFloat)
			}
		})
	}

	// YAML-only spellings are not JSON numbers and stay float64.
	for _, text := range []string{"x: .5\n", "x: 5.\n"} {
		decoded, err := yamlx.Decode([]byte(text))
		if err != nil {
			t.Fatalf("yamlx.Decode(%q) error = %v", text, err)
		}
		if _, ok := decoded.(map[string]any)["x"].(float64); !ok {
			t.Fatalf("YAML %q x = %#v (%T), want float64", text, decoded.(map[string]any)["x"], decoded.(map[string]any)["x"])
		}
	}
	// The non-finite YAML scalars (.nan/.inf, rule F8) and `| type string` on a
	// numeric literal are pinned by TestFloatRejectionRules and
	// TestLiteralNumberRules, so they are not repeated here.
}

// TestLiteralNumberRoundTrip pins F6 end to end: decode -> normalize -> encode
// keeps the exact text on both codecs.
func TestLiteralNumberRoundTrip(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"1.0", "1.50", "1e3", "-0.5", "1e309"} {
		value := values.NormalizeNumbers(json.Number(raw))
		encoded, err := jsonx.Marshal(value)
		if err != nil {
			t.Fatalf("jsonx.Marshal(%s) error = %v", raw, err)
		}
		if string(encoded) != raw {
			t.Fatalf("jsonx round trip %s = %q", raw, encoded)
		}
		encodedYAML, err := yamlx.Marshal(value)
		if err != nil {
			t.Fatalf("yamlx.Marshal(%s) error = %v", raw, err)
		}
		if strings.TrimRight(string(encodedYAML), "\n") != raw {
			t.Fatalf("yamlx round trip %s = %q", raw, encodedYAML)
		}
	}
}

// TestLiteralNumberValid pins the JSON number grammar accepted by the new type.
func TestLiteralNumberValid(t *testing.T) {
	t.Parallel()
	valid := []string{"0", "-0", "1", "1.0", "-1.50", "1e3", "1E+3", "1.5e-3", "0.5"}
	for _, text := range valid {
		if !codec.LiteralNumber(text).Valid() {
			t.Fatalf("LiteralNumber(%q).Valid() = false, want true", text)
		}
	}
	invalid := []string{"", ".5", "5.", "1_000", "+1", "01", "1e", "1e+", "-", "--1", "1.2.3", " 1", "1 "}
	for _, text := range invalid {
		if codec.LiteralNumber(text).Valid() {
			t.Fatalf("LiteralNumber(%q).Valid() = true, want false", text)
		}
	}
	// Integer spellings stay with their narrower types.
	if _, ok := codec.NewLiteral("1"); ok {
		t.Fatal("NewLiteral(1) = ok, want false (integer spelling)")
	}
	if _, ok := codec.NewLiteral("1.0"); !ok {
		t.Fatal("NewLiteral(1.0) = false, want true")
	}
}

// loadSchema compiles a one-property schema whose `x` has the given JSON Schema
// type.
func loadSchema(t *testing.T, schemaType string) *schema.Schema {
	t.Helper()
	raw := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"x": map[string]any{"type": schemaType},
		},
	}
	sch, err := schema.Load(context.Background(), raw, "types_test", schema.UnknownIgnore)
	if err != nil {
		t.Fatalf("schema.Load(%s) error = %v", schemaType, err)
	}
	return sch
}

// TestArrayAliasGrammarIsShared pins SI1/G13: the inline | type parser and the
// JSON Schema normalizer accept and reject exactly the same array<T> spellings,
// and nested arrays produce the same diagnostic phrase on both sides.
func TestArrayAliasGrammarIsShared(t *testing.T) {
	t.Parallel()
	cases := []struct {
		alias      string
		wantOK     bool
		wantPhrase string
	}{
		{alias: "array", wantOK: true},
		{alias: "array<int>", wantOK: true},
		{alias: "array<object>", wantOK: true},
		{alias: "array<string>", wantOK: true},
		{alias: "array<array<int>>", wantPhrase: "nested array types are not supported"},
		{alias: "array<array>", wantPhrase: "nested array types are not supported"},
		{alias: "array<>", wantPhrase: ""},
		{alias: "array<", wantPhrase: ""},
		{alias: "bogus", wantPhrase: ""},
	}
	for _, tt := range cases {
		t.Run(tt.alias, func(t *testing.T) {
			t.Parallel()
			_, inlineErr := types.ParseType(tt.alias)
			_, schemaErr := schema.Load(context.Background(),
				map[string]any{"type": tt.alias}, "schema.yaml", schema.UnknownIgnore)

			if (inlineErr == nil) != tt.wantOK {
				t.Fatalf("types.ParseType(%q) error = %v, wantOK = %v", tt.alias, inlineErr, tt.wantOK)
			}
			if (schemaErr == nil) != tt.wantOK {
				t.Fatalf("schema.Load(type=%q) error = %v, wantOK = %v", tt.alias, schemaErr, tt.wantOK)
			}
			if tt.wantPhrase != "" {
				for side, err := range map[string]error{"inline": inlineErr, "schema": schemaErr} {
					if err == nil || !strings.Contains(err.Error(), tt.wantPhrase) {
						t.Fatalf("%s side for %q = %v, want it to mention %q", side, tt.alias, err, tt.wantPhrase)
					}
				}
			}
		})
	}
}
