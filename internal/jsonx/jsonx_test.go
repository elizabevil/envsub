package jsonx_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/jsonx"
)

// readTestdata loads an on-disk test input. All documents live under testdata/
// so the same fixture feeds both backends (see jsonv2_test.go and sonic_test.go
// for the two places where they intentionally differ).
func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read testdata %s: %v", name, err)
	}
	return data
}

// TestUnmarshalValuePreservesNumbers pins the contract both backends share:
// integers keep their exact spelling instead of widening to float64.
func TestUnmarshalValuePreservesNumbers(t *testing.T) {
	t.Parallel()
	value, err := jsonx.UnmarshalValue(readTestdata(t, "numbers.json"))
	if err != nil {
		t.Fatalf("UnmarshalValue() error = %v", err)
	}
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("UnmarshalValue() = %T, want map[string]any", value)
	}

	want := map[string]string{
		"int":   "8080",
		"big":   "12345678901234567890",
		"float": "1.5",
		"exp":   "1e3",
		"neg":   "-3",
		"zero":  "0",
	}
	for key, wantNumber := range want {
		number, ok := object[key].(json.Number)
		if !ok {
			t.Fatalf("%s = %T, want json.Number", key, object[key])
		}
		if string(number) != wantNumber {
			t.Fatalf("%s = %q, want %q", key, number, wantNumber)
		}
	}
}

// TestUnmarshalValueNestedAndEscaped covers the tree shapes the renderer and
// schema loader rely on.
func TestUnmarshalValueNestedAndEscaped(t *testing.T) {
	t.Parallel()
	value, err := jsonx.UnmarshalValue(readTestdata(t, "nested.json"))
	if err != nil {
		t.Fatalf("UnmarshalValue() error = %v", err)
	}
	want := map[string]any{
		"a": []any{
			json.Number("1"),
			map[string]any{"b": "x\ny", "c": nil},
			true,
			json.Number("-2.5"),
		},
		"d": map[string]any{},
		"e": []any{},
	}
	if !reflect.DeepEqual(value, want) {
		t.Fatalf("UnmarshalValue() = %#v, want %#v", value, want)
	}
}

// TestUnmarshalValueRejectsBadDocuments keeps the strictness of the standard
// library decoders; the documents are the files under testdata/invalid.
func TestUnmarshalValueRejectsBadDocuments(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"invalid/empty.json",
		"invalid/unclosed_object.json",
		"invalid/trailing_comma.json",
		"invalid/leading_zero.json",
		"invalid/mismatched_delimiters.json",
		"trailing.json",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := jsonx.UnmarshalValue(readTestdata(t, name)); err == nil {
				t.Fatalf("UnmarshalValue(%s) error = nil, want error", name)
			}
		})
	}
}

// TestUnmarshalIntoTreeTargets checks the convenience targets used by the
// values loader.
func TestUnmarshalIntoTreeTargets(t *testing.T) {
	t.Parallel()
	document := readTestdata(t, "numbers.json")

	var object map[string]any
	if err := jsonx.Unmarshal(document, &object); err != nil {
		t.Fatalf("Unmarshal(map) error = %v", err)
	}
	if number, ok := object["int"].(json.Number); !ok || string(number) != "8080" {
		t.Fatalf("int = %#v, want json.Number(8080)", object["int"])
	}

	var array []any
	if err := jsonx.Unmarshal([]byte(`[1,2]`), &array); err != nil {
		t.Fatalf("Unmarshal(slice) error = %v", err)
	}
	if len(array) != 2 {
		t.Fatalf("array = %#v, want two elements", array)
	}

	var anyValue any
	if err := jsonx.Unmarshal([]byte(`"text"`), &anyValue); err != nil {
		t.Fatalf("Unmarshal(any) error = %v", err)
	}
	if anyValue != "text" {
		t.Fatalf("any = %#v, want text", anyValue)
	}

	var typed struct {
		Port int `json:"port"`
	}
	if err := jsonx.Unmarshal([]byte(`{"port":8080}`), &typed); err != nil {
		t.Fatalf("Unmarshal(struct) error = %v", err)
	}
	if typed.Port != 8080 {
		t.Fatalf("Port = %d, want 8080", typed.Port)
	}

	if err := jsonx.Unmarshal([]byte(`[1]`), &object); err == nil {
		t.Fatal("Unmarshal(array into map) error = nil, want error")
	}
}

// TestMarshalIsDeterministic pins the contract both backends must satisfy:
// object keys are sorted, so toJson output never depends on Go's random map
// iteration order. This was already broken before the backend split (sonic's
// default config does not sort), hence the explicit check.
func TestMarshalIsDeterministic(t *testing.T) {
	t.Parallel()
	value, err := jsonx.UnmarshalValue(readTestdata(t, "marshal/deterministic.in.json"))
	if err != nil {
		t.Fatalf("UnmarshalValue() error = %v", err)
	}
	want := string(trimNewline(readTestdata(t, "marshal/deterministic.want.json")))

	for range 20 {
		got, err := jsonx.Marshal(value)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		if string(got) != want {
			t.Fatalf("Marshal() = %q, want %q", got, want)
		}
	}

	wantIndent := string(trimNewline(readTestdata(t, "marshal/deterministic.want_indent.json")))
	indented, err := jsonx.MarshalIndent(value)
	if err != nil {
		t.Fatalf("MarshalIndent() error = %v", err)
	}
	if string(indented) != wantIndent {
		t.Fatalf("MarshalIndent() = %q, want %q", indented, wantIndent)
	}
}

// TestMarshalOptionalFieldsAgree pins the struct-tag contract both backends
// share, because features encode optional fields with it (the --explain report
// omits `line` when it is unknown): a nil pointer, an empty string and a nil
// slice tagged `omitempty` are omitted, and a set value is emitted.
//
// Optional numbers must therefore be pointers. A bare int cannot be used: the
// backends disagree on `omitempty` for numeric zero, which jsonv2_test.go and
// sonic_test.go pin from each side (rules 03-13 G20).
func TestMarshalOptionalFieldsAgree(t *testing.T) {
	t.Parallel()
	type record struct {
		Line   *int     `json:"line,omitempty"`
		File   string   `json:"file,omitempty"`
		Shadow []string `json:"shadow,omitempty"`
	}
	line := 7

	tests := []struct {
		name  string
		value record
		want  string
	}{
		{name: "unset fields are omitted", value: record{}, want: `{}`},
		{
			name:  "set fields are emitted",
			value: record{Line: &line, File: ".env", Shadow: []string{"process"}},
			want:  `{"line":7,"file":".env","shadow":["process"]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := jsonx.Marshal(tt.value)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("Marshal() = %s, want %s", got, tt.want)
			}
		})
	}
}

// TestMarshalRoundTripKeepsNumberSpelling checks that decoding and re-encoding
// a document preserves integer precision, float spelling and exponents.
func TestMarshalRoundTripKeepsNumberSpelling(t *testing.T) {
	t.Parallel()
	value, err := jsonx.UnmarshalValue(readTestdata(t, "marshal/roundtrip.in.json"))
	if err != nil {
		t.Fatalf("UnmarshalValue() error = %v", err)
	}
	got, err := jsonx.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	want := string(trimNewline(readTestdata(t, "marshal/roundtrip.want.json")))
	if string(got) != want {
		t.Fatalf("round trip = %q, want %q", got, want)
	}
}

// TestMarshalNilCollectionsAsEmpty pins rule V1: a typed nil map or slice uses
// the library default, {} and [], on both JSON backends; only an untyped nil is
// null. A typed nil cannot be expressed in a JSON document, so these values are
// built in code rather than loaded from testdata.
func TestMarshalNilCollectionsAsEmpty(t *testing.T) {
	t.Parallel()
	var nilSlice []any
	var nilMap map[string]any
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "nil slice", value: nilSlice, want: "[]"},
		{name: "nil map", value: nilMap, want: "{}"},
		{name: "empty slice", value: []any{}, want: "[]"},
		{name: "empty map", value: map[string]any{}, want: "{}"},
		{name: "nil value", value: nil, want: "null"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := jsonx.Marshal(tt.value)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("Marshal() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestMarshalRejectsInvalidUTF8 pins rule Q3a on the encode side: a string that
// is not valid UTF-8 must be reported, never silently replaced with U+FFFD.
// Both backends are held to this.
func TestMarshalRejectsInvalidUTF8(t *testing.T) {
	t.Parallel()
	if _, err := jsonx.Marshal("bad \xff\xfe end"); err == nil {
		t.Fatal("Marshal(invalid UTF-8) error = nil, want error")
	}
	if _, err := jsonx.MarshalIndent(map[string]any{"k": "bad \xff"}); err == nil {
		t.Fatal("MarshalIndent(invalid UTF-8) error = nil, want error")
	}
}

// TestMarshalKeepsNumbersAsNumbers checks the encoder writes json.Number as a
// JSON number, not as a quoted string.
func TestMarshalKeepsNumbersAsNumbers(t *testing.T) {
	t.Parallel()
	value := map[string]any{"n": json.Number("8080"), "f": json.Number("1.50"), "s": "x"}
	compact, err := jsonx.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(compact, &decoded); err != nil {
		t.Fatalf("std json rejected Marshal() output %q: %v", compact, err)
	}
	want := map[string]any{"n": float64(8080), "f": 1.5, "s": "x"}
	if !reflect.DeepEqual(decoded, want) {
		t.Fatalf("Marshal() = %q, decoded %#v, want %#v", compact, decoded, want)
	}
	if !strings.Contains(string(compact), "1.50") {
		t.Fatalf("Marshal() = %q, want the 1.50 spelling preserved", compact)
	}
}

// trimNewline drops the trailing newline of a golden file.
func trimNewline(data []byte) []byte {
	return []byte(strings.TrimRight(string(data), "\n"))
}

// benchmarkPayload builds a configuration-sized document (5000 objects) used to
// compare the two backends: `go test -bench=. ./internal/jsonx` for the default
// stdlib backend and `-tags=sonic` for the fast one.
func benchmarkPayload() []byte {
	var b strings.Builder
	b.WriteByte('[')
	for i := range 5000 {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"host":"h%04d.internal","port":%d,"tags":["a","b","c"],"enabled":true}`, i, 8000+i)
	}
	b.WriteByte(']')
	return []byte(b.String())
}

func BenchmarkUnmarshalValue(b *testing.B) {
	data := benchmarkPayload()
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := jsonx.UnmarshalValue(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarshal(b *testing.B) {
	value, err := jsonx.UnmarshalValue(benchmarkPayload())
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := jsonx.Marshal(value); err != nil {
			b.Fatal(err)
		}
	}
}
