// Package encoding_test holds the minimal conformance tests for the serialization
// rules in docs/03-design/03-12-serialization-rules.md.
//
// The table below is the specification: every row states a rule from that
// document. A row with a non-empty `pending` field is not implemented yet and is
// skipped with the rule ID, so the file doubles as the implementation checklist
// (03-12 §5). Remove the `pending` marker together with the implementation.
//
//	go test -v ./test/encoding/                 # rules that must hold today
//	go test -v -tags=sonic ./test/encoding/     # same rules for the sonic backend
package encoding_test

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/codec"

	"github.com/elizabevil/envsub/internal/jsonx"
	"github.com/elizabevil/envsub/internal/types"
	"github.com/elizabevil/envsub/internal/values"
	"github.com/elizabevil/envsub/internal/yamlx"
)

// bigInteger exceeds uint64 and must survive as exact decimal text (rules N1-N4).
const bigInteger = "123456789012345678901234567890"

func TestNumberRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		rule     string
		name     string
		value    any
		wantJSON string
		wantYAML string
		pending  string
	}{
		{rule: "N2", name: "int", value: 8080, wantJSON: "8080", wantYAML: "8080\n"},
		{rule: "N2", name: "negative int", value: -3, wantJSON: "-3", wantYAML: "-3\n"},
		{rule: "N2", name: "uint64 max", value: uint64(18446744073709551615), wantJSON: "18446744073709551615", wantYAML: "18446744073709551615\n"},
		{rule: "N2", name: "json uint64", value: json.Number("18446744073709551615"), wantJSON: "18446744073709551615", wantYAML: "18446744073709551615\n"},
		{rule: "N2", name: "float64", value: 1.5, wantJSON: "1.5", wantYAML: "1.5\n"},
		{rule: "N1/N2", name: "integer beyond uint64", value: json.Number(bigInteger), wantJSON: bigInteger, wantYAML: bigInteger + "\n"},
		{
			rule: "N1/N2", name: "exact integer value", value: codec.ExactInt(bigInteger),
			wantJSON: bigInteger, wantYAML: bigInteger + "\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.pending != "" {
				t.Skipf("%s 待实现：%s", tt.rule, tt.pending)
			}
			gotJSON, err := jsonx.Marshal(tt.value)
			if err != nil {
				t.Fatalf("jsonx.Marshal(%#v) error = %v", tt.value, err)
			}
			if string(gotJSON) != tt.wantJSON {
				t.Fatalf("%s toJson = %q, want %q", tt.rule, gotJSON, tt.wantJSON)
			}
			gotYAML, err := yamlx.Marshal(tt.value)
			if err != nil {
				t.Fatalf("yamlx.Marshal(%#v) error = %v", tt.value, err)
			}
			if string(gotYAML) != tt.wantYAML {
				t.Fatalf("%s toYaml = %q, want %q", tt.rule, gotYAML, tt.wantYAML)
			}
		})
	}
}

func TestNullOrEmptyRules(t *testing.T) {
	t.Parallel()
	var nilMap map[string]any
	var nilSlice []any
	tests := []struct {
		rule     string
		name     string
		value    any
		wantJSON string
		wantYAML string
		pending  string
	}{
		{rule: "V2", name: "explicit null", value: nil, wantJSON: "null", wantYAML: "null\n"},
		{rule: "V4", name: "empty string", value: "", wantJSON: `""`, wantYAML: "\"\"\n"},
		{rule: "Q1", name: "numeric string", value: "8080", wantJSON: `"8080"`, wantYAML: "\"8080\"\n"},
		{rule: "V1", name: "empty map", value: map[string]any{}, wantJSON: "{}", wantYAML: "{}\n"},
		{rule: "V1", name: "empty slice", value: []any{}, wantJSON: "[]", wantYAML: "[]\n"},
		{rule: "V1", name: "typed nil map", value: nilMap, wantJSON: "{}", wantYAML: "{}\n"},
		{rule: "V1", name: "typed nil slice", value: nilSlice, wantJSON: "[]", wantYAML: "[]\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.pending != "" {
				t.Skipf("%s 待实现：%s", tt.rule, tt.pending)
			}
			gotJSON, err := jsonx.Marshal(tt.value)
			if err != nil {
				t.Fatalf("jsonx.Marshal(%#v) error = %v", tt.value, err)
			}
			if string(gotJSON) != tt.wantJSON {
				t.Fatalf("%s toJson = %q, want %q", tt.rule, gotJSON, tt.wantJSON)
			}
			gotYAML, err := yamlx.Marshal(tt.value)
			if err != nil {
				t.Fatalf("yamlx.Marshal(%#v) error = %v", tt.value, err)
			}
			if string(gotYAML) != tt.wantYAML {
				t.Fatalf("%s toYaml = %q, want %q", tt.rule, gotYAML, tt.wantYAML)
			}
		})
	}
}

// TestDecodeRules covers rule N3: every input path must agree on integers, and
// none of them may silently fall back to float64.
func TestDecodeRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		rule    string
		name    string
		run     func(t *testing.T) any
		want    string
		pending string
	}{
		{
			rule: "N3", name: "JSON beyond uint64 stays exact",
			run: func(t *testing.T) any {
				value, err := jsonx.UnmarshalValue([]byte(`{"id":` + bigInteger + `}`))
				if err != nil {
					t.Fatalf("UnmarshalValue() error = %v", err)
				}
				return value.(map[string]any)["id"]
			},
			want: bigInteger,
		},
		{
			rule: "N3", name: "YAML beyond uint64 stays exact",
			run: func(t *testing.T) any {
				value, err := yamlx.Decode([]byte("id: " + bigInteger + "\n"))
				if err != nil {
					t.Fatalf("Decode() error = %v", err)
				}
				return value.(map[string]any)["id"]
			},
			want: bigInteger,
		},
		{
			rule: "N3", name: "env string materializes a uint64",
			run: func(t *testing.T) any {
				value, err := types.Assert("18446744073709551615", types.Declared{Name: types.NameInt})
				if err != nil {
					t.Fatalf("Assert() error = %v", err)
				}
				return value
			},
			want: "18446744073709551615",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.pending != "" {
				t.Skipf("%s 待实现：%s", tt.rule, tt.pending)
			}
			value := tt.run(t)
			if _, isFloat := value.(float64); isFloat {
				t.Fatalf("%s: decoded value is float64, which the rules forbid", tt.rule)
			}
			got, err := types.Stringify(value)
			if err != nil {
				t.Fatalf("Stringify(%#v) error = %v", value, err)
			}
			if got != tt.want {
				t.Fatalf("%s: rendered %q, want %q", tt.rule, got, tt.want)
			}
		})
	}
}

// TestDeterminismAndTreeRules covers D1 and the tree-level normalization that
// feeds both encoders.
func TestDeterminismAndTreeRules(t *testing.T) {
	t.Parallel()
	value := map[string]any{"z": 1, "a": 2, "m": 3}
	for range 20 {
		gotJSON, err := jsonx.Marshal(value)
		if err != nil {
			t.Fatalf("jsonx.Marshal() error = %v", err)
		}
		if want := `{"a":2,"m":3,"z":1}`; string(gotJSON) != want {
			t.Fatalf("D1 toJson = %q, want %q", gotJSON, want)
		}
		gotYAML, err := yamlx.Marshal(value)
		if err != nil {
			t.Fatalf("yamlx.Marshal() error = %v", err)
		}
		if want := "a: 2\nm: 3\nz: 1\n"; string(gotYAML) != want {
			t.Fatalf("D1 toYaml = %q, want %q", gotYAML, want)
		}
	}

	// normalizeNumbers is the single funnel for JSON-decoded numbers (N1/F6).
	normalized := values.NormalizeNumbers(map[string]any{
		"int":    json.Number("8080"),
		"uint64": json.Number("18446744073709551615"),
		"float":  json.Number("1.50"),
	})
	if got := normalized.(map[string]any)["uint64"]; got != uint64(18446744073709551615) {
		t.Fatalf("N1 uint64 normalized to %#v (%T)", got, got)
	}
	if got := normalized.(map[string]any)["float"]; got != codec.LiteralNumber("1.50") {
		t.Fatalf("F6 float normalized to %#v (%T), want codec.LiteralNumber(1.50)", got, got)
	}

	// The exact-spelling contract of an undeclared fractional or exponent
	// literal (F3/F6/Q6) is pinned by TestLiteralNumberRules (all three render
	// channels) and TestLiteralNumberRoundTrip (both encoders, including the
	// out-of-float64-range literal 1e309) rather than repeated here.
}

// TestFloatRules covers rules F1-F5: floats are IEEE-754 binary64, encoded in the
// shortest round-trip form, and non-finite values must never enter the tree.
func TestFloatRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		rule     string
		name     string
		value    float64
		wantJSON string
		wantYAML string
		pending  string
	}{
		{rule: "F3", name: "simple decimal", value: 0.5, wantJSON: "0.5", wantYAML: "0.5\n"},
		{rule: "F3", name: "exponent form", value: 1e21, wantJSON: "1e+21", wantYAML: "1e+21\n"},
		{rule: "F3", name: "negative zero keeps its sign", value: math.Copysign(0, -1), wantJSON: "-0", wantYAML: "-0\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.pending != "" {
				t.Skipf("%s 待实现：%s", tt.rule, tt.pending)
			}
			gotJSON, err := jsonx.Marshal(tt.value)
			if err != nil {
				t.Fatalf("jsonx.Marshal(%v) error = %v", tt.value, err)
			}
			if string(gotJSON) != tt.wantJSON {
				t.Fatalf("%s toJson = %q, want %q", tt.rule, gotJSON, tt.wantJSON)
			}
			gotYAML, err := yamlx.Marshal(tt.value)
			if err != nil {
				t.Fatalf("yamlx.Marshal(%v) error = %v", tt.value, err)
			}
			if string(gotYAML) != tt.wantYAML {
				t.Fatalf("%s toYaml = %q, want %q", tt.rule, gotYAML, tt.wantYAML)
			}
		})
	}
}

// TestLiteralNumberRules covers rules F3/F6/Q6/FB2: an undeclared numeric
// literal keeps its exact spelling through the three render channels and both
// value sources, while `| type float` still narrows it to float64.
func TestLiteralNumberRules(t *testing.T) {
	t.Parallel()
	declaredFloat := map[string]string{
		"1.0":  "1",
		"1.50": "1.5",
		"1e3":  "1000",
		"-0.5": "-0.5",
	}
	for _, raw := range []string{"1.0", "1.50", "1e3", "-0.5"} {
		t.Run("json/"+raw, func(t *testing.T) {
			t.Parallel()
			tree := values.NewTree()
			tree.Root["x"] = values.NormalizeNumbers(json.Number(raw))
			if tree.Root["x"] != codec.LiteralNumber(raw) {
				t.Fatalf("NormalizeNumbers(%s) = %#v (%T), want codec.LiteralNumber", raw, tree.Root["x"], tree.Root["x"])
			}
			for _, tmpl := range []string{"${x}", "${x | toJson}", "${x | toYaml}"} {
				got, err := renderTemplate(t, tmpl, tree)
				if err != nil {
					t.Fatalf("render %s error = %v", tmpl, err)
				}
				if got != raw {
					t.Fatalf("render %s = %q, want %q", tmpl, got, raw)
				}
			}
			got, err := renderTemplate(t, "${x | type float}", tree)
			if err != nil {
				t.Fatalf("render | type float error = %v", err)
			}
			if got != declaredFloat[raw] {
				t.Fatalf("render | type float (%s) = %q, want %q", raw, got, declaredFloat[raw])
			}
			if _, err := renderTemplate(t, "${x | type string}", tree); err == nil {
				t.Fatal("render | type string error = nil, want a type error")
			}
		})
	}

	// A YAML value file is the second source.
	decoded, err := yamlx.Decode([]byte("a: 1.0\nb: 1.50\nc: 1e3\nd: -0.5\n"))
	if err != nil {
		t.Fatalf("yamlx.Decode() error = %v", err)
	}
	object := decoded.(map[string]any)
	for key, want := range map[string]string{"a": "1.0", "b": "1.50", "c": "1e3", "d": "-0.5"} {
		if object[key] != codec.LiteralNumber(want) {
			t.Fatalf("YAML %s = %#v (%T), want codec.LiteralNumber(%s)", key, object[key], object[key], want)
		}
	}
}

// TestFloatRejectionRules covers F2/F4 at the decode and materialization side.
func TestFloatRejectionRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		rule    string
		name    string
		run     func() error
		pending string
	}{
		{
			rule: "F8", name: "YAML .nan is rejected",
			run: func() error {
				_, err := yamlx.Decode([]byte("x: .nan\n"))
				return err
			},
		},
		{
			rule: "F7", name: "| type float rejects nan",
			run: func() error {
				_, err := types.Assert("nan", types.Declared{Name: types.NameFloat})
				return err
			},
		},
		{
			rule: "F7", name: "| type float rejects inf",
			run: func() error {
				_, err := types.Assert("inf", types.Declared{Name: types.NameFloat})
				return err
			},
		},
		{
			rule: "F7", name: "| type float rejects a value beyond float64 range",
			run: func() error {
				_, err := types.Assert(types.Normalize(json.Number("1e309")), types.Declared{Name: types.NameFloat})
				return err
			},
		},
		{
			rule: "F8", name: "both encoders reject NaN",
			run: func() error {
				if _, err := jsonx.Marshal(math.NaN()); err == nil {
					return errors.New("jsonx.Marshal(NaN) returned nil error")
				}
				_, err := yamlx.Marshal(math.NaN())
				return err
			},
		},
		{
			rule: "F8", name: "both encoders reject +Inf",
			run: func() error {
				if _, err := jsonx.Marshal(math.Inf(1)); err == nil {
					return errors.New("jsonx.Marshal(+Inf) returned nil error")
				}
				_, err := yamlx.Marshal(math.Inf(1))
				return err
			},
		},
		{
			rule: "F8", name: "YAML .inf is rejected",
			run: func() error {
				_, err := yamlx.Decode([]byte("x: .inf\n"))
				return err
			},
		},
		{
			rule: "TY6/G18", name: "| type null rejects the empty string",
			run: func() error {
				_, err := types.Assert("", types.Declared{Name: types.NameNull})
				return err
			},
		},
		{
			rule: "TY6/G18", name: "| type null rejects the text null",
			run: func() error {
				_, err := types.Assert("null", types.Declared{Name: types.NameNull})
				return err
			},
		},
		{
			rule: "F4", name: "| type float rejects blank",
			run: func() error {
				_, err := types.Assert(" 1 ", types.Declared{Name: types.NameFloat})
				return err
			},
		},
		{
			rule: "F4", name: "| type float rejects text",
			run: func() error {
				_, err := types.Assert("abc", types.Declared{Name: types.NameFloat})
				return err
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.pending != "" {
				t.Skipf("%s 待实现：%s", tt.rule, tt.pending)
			}
			if err := tt.run(); err == nil {
				t.Fatalf("%s: expected an error", tt.rule)
			}
		})
	}
}

// TestTypeRangeRules covers the 03-12 §3 table rows that are observable today:
// booleans follow YAML 1.2 core, invalid UTF-8 and duplicate keys are rejected.
func TestTypeRangeRules(t *testing.T) {
	t.Parallel()

	// bool: only true/false are booleans; YAML 1.1 spellings stay strings.
	value, err := yamlx.Decode([]byte("a: yes\nb: on\nc: true\n"))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	object := value.(map[string]any)
	for _, key := range []string{"a", "b"} {
		if _, isBool := object[key].(bool); isBool {
			t.Fatalf("bool: %s decoded as bool, want string (YAML 1.2 core)", key)
		}
	}
	if object["c"] != true {
		t.Fatalf("bool: c = %#v, want true", object["c"])
	}

	// string: invalid UTF-8 is rejected on decode (Q3a) by both backends; the
	// sonic backend checks the decoded tree because its public decoder cannot.
	if _, err := jsonx.UnmarshalValue([]byte("{\"a\":\"\xff\xfe\"}")); err == nil {
		t.Fatal("string: invalid UTF-8 accepted, want error")
	}

	// object: duplicate keys are rejected by the default backend and by YAML.
	// sonic has no switch for this and T9 accepts it as the single backend
	// exception, so the JSON half is skipped there.
	if sonicBackend {
		t.Skip("Q3b：sonic 的重复成员名是已接受的唯一例外（03-12 T9）")
	}
	if _, err := jsonx.UnmarshalValue([]byte(`{"a":1,"a":2}`)); err == nil {
		t.Fatal("object: duplicate JSON key accepted, want error")
	}
	if _, err := yamlx.Decode([]byte("a: 1\na: 2\n")); err == nil {
		t.Fatal("object: duplicate YAML key accepted, want error")
	}

	// int: int64 与 uint64 边界都必须精确（N1/N3）；此处断言 int64 上界。
	if got, err := types.Assert("9223372036854775807", types.Declared{Name: types.NameInt}); err != nil {
		t.Fatalf("int: int64 max rejected: %v", err)
	} else if text, err := types.Stringify(got); err != nil || text != "9223372036854775807" {
		t.Fatalf("int: int64 max rendered %q (err=%v)", text, err)
	}
}

// TestDepthLimitIsACleanError pins the depth behaviour recorded in 03-12 §3
// (rule L1): both decoders reject a pathologically nested document with a clean
// error instead of panicking, so the renderer never crashes on hostile input.
func TestDepthLimitIsACleanError(t *testing.T) {
	t.Parallel()
	const depth = 100000
	document := []byte(strings.Repeat("[", depth) + strings.Repeat("]", depth))

	if _, err := jsonx.UnmarshalValue(document); err == nil {
		t.Fatal("json: deeply nested document accepted, want a depth error")
	} else if !strings.Contains(err.Error(), "depth") {
		t.Fatalf("json: error = %v, want a depth error", err)
	}
	if _, err := yamlx.Decode(document); err == nil {
		t.Fatal("yaml: deeply nested document accepted, want a depth error")
	} else if !strings.Contains(err.Error(), "depth") {
		t.Fatalf("yaml: error = %v, want a depth error", err)
	}
}

// TestFloatExactnessRules covers rules F7/G2: `| type float` accepts only values
// float64 can hold exactly. Integer text, uint64 values and exact integers must
// all behave the same way, and 2^53 (9007199254740992) is the boundary.
func TestFloatExactnessRules(t *testing.T) {
	t.Parallel()
	declared := types.Declared{Name: types.NameFloat}
	rejected := []struct {
		name  string
		value any
	}{
		{name: "string integer above 2^53", value: "9007199254740993"},
		{name: "uint64 above 2^53", value: uint64(9007199254740993)},
		{name: "exact integer", value: codec.ExactInt("123456789012345678901234567890")},
		{name: "json integer above 2^53", value: json.Number("18446744073709551615")},
	}
	for _, tt := range rejected {
		t.Run("reject "+tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := types.Assert(tt.value, declared); err == nil {
				t.Fatalf("Assert(%#v, float) error = nil, want an exactness error", tt.value)
			}
		})
	}
	accepted := []struct {
		name  string
		value any
	}{
		{name: "string integer at 2^53", value: "9007199254740992"},
		{name: "uint64 at 2^53", value: uint64(9007199254740992)},
		{name: "small integer", value: "8080"},
		{name: "fraction", value: "0.5"},
	}
	for _, tt := range accepted {
		t.Run("accept "+tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := types.Assert(tt.value, declared); err != nil {
				t.Fatalf("Assert(%#v, float) error = %v, want nil", tt.value, err)
			}
		})
	}
}
