//go:build !sonic

package jsonx_test

import (
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/jsonx"
)

// TestJSONv2DefaultsToFloat64ForAny pins the standard-library behaviour that
// forces this package to build its own tree: encoding/json/v2 decodes numbers
// into any as float64, so integers beyond 2^53 lose precision. There is no
// UseNumber switch — that is a v1 Decoder method, and the options that exist
// cover UTF-8, duplicate names and formatting, not the number type. The
// alternatives, measurements and the rejected WithUnmarshalers variant are
// recorded in docs/06-review/06-06-json-number-precision.md.
//
// If this test ever fails, the standard library changed its default for any
// interface values: check whether the tree walker in jsonv2.go can be deleted.
func TestJSONv2DefaultsToFloat64ForAny(t *testing.T) {
	t.Parallel()
	document := readTestdata(t, "big_integers.json")

	var plain map[string]any
	if err := jsonv2.Unmarshal(document, &plain); err != nil {
		t.Fatalf("jsonv2.Unmarshal() error = %v", err)
	}
	rounded, ok := plain["pow53"].(float64)
	if !ok {
		t.Fatalf("pow53 = %T, want float64 (the encoder reads the decoded number as float64)", plain["pow53"])
	}
	if got := strconv.FormatFloat(rounded, 'f', -1, 64); got != "9007199254740992" {
		t.Fatalf("pow53 = %s, want the rounded 9007199254740992", got)
	}

	// The jsonx tree keeps the exact spelling of every integer.
	exact, err := jsonx.UnmarshalValue(document)
	if err != nil {
		t.Fatalf("UnmarshalValue() error = %v", err)
	}
	want := map[string]any{
		"pow53":  json.Number("9007199254740993"),
		"u64max": json.Number("18446744073709551615"),
		"i64min": json.Number("-9223372036854775808"),
	}
	if !reflect.DeepEqual(exact, want) {
		t.Fatalf("UnmarshalValue() = %#v, want %#v", exact, want)
	}
}

// Unmarshal is the only function in jsonv2.go that dispatches on the target
// type: tree-shaped targets (*any, *map[string]any, *[]any) go through the
// number-preserving walker, everything else is handed to encoding/json/v2.
// The tests below cover that dispatch, including the error branches.
func TestJSONv2UnmarshalTreeTargets(t *testing.T) {
	t.Parallel()
	object := readTestdata(t, "numbers.json")
	array := readTestdata(t, "array.json")

	var anyTarget any
	if err := jsonx.Unmarshal(object, &anyTarget); err != nil {
		t.Fatalf("Unmarshal(*any) error = %v", err)
	}
	if _, ok := anyTarget.(map[string]any); !ok {
		t.Fatalf("Unmarshal(*any) = %T, want map[string]any", anyTarget)
	}

	var mapTarget map[string]any
	if err := jsonx.Unmarshal(object, &mapTarget); err != nil {
		t.Fatalf("Unmarshal(*map) error = %v", err)
	}
	if number, ok := mapTarget["int"].(json.Number); !ok || string(number) != "8080" {
		t.Fatalf("map int = %#v, want json.Number(8080)", mapTarget["int"])
	}

	var sliceTarget []any
	if err := jsonx.Unmarshal(array, &sliceTarget); err != nil {
		t.Fatalf("Unmarshal(*slice) error = %v", err)
	}
	want := []any{json.Number("8080"), json.Number("12345678901234567890"), json.Number("1.50"), true, nil, json.Number("-3")}
	if !reflect.DeepEqual(sliceTarget, want) {
		t.Fatalf("Unmarshal(*slice) = %#v, want %#v", sliceTarget, want)
	}

	var scalarTarget any
	if err := jsonx.Unmarshal(readTestdata(t, "scalar_null.json"), &scalarTarget); err != nil {
		t.Fatalf("Unmarshal(null) error = %v", err)
	}
	if scalarTarget != nil {
		t.Fatalf("Unmarshal(null) = %#v, want nil", scalarTarget)
	}
}

// TestJSONv2UnmarshalTargetMismatch covers the two branches that reject a
// document whose shape does not match the requested tree target. The message is
// encoding/json/v2's own, because the tree targets are decoded by v2 with the
// number-preserving unmarshaler installed rather than by jsonx itself.
func TestJSONv2UnmarshalTargetMismatch(t *testing.T) {
	t.Parallel()
	var sliceTarget []any
	if err := jsonx.Unmarshal(readTestdata(t, "numbers.json"), &sliceTarget); err == nil {
		t.Fatal("Unmarshal(object into *[]any) error = nil, want error")
	} else {
		// The wording is encoding/json/v2's and is not pinned here.
		t.Logf("object into *[]any: %v", err)
	}

	var mapTarget map[string]any
	if err := jsonx.Unmarshal(readTestdata(t, "array.json"), &mapTarget); err == nil {
		t.Fatal("Unmarshal(array into *map[string]any) error = nil, want error")
	} else {
		t.Logf("array into *map[string]any: %v", err)
	}
}

// TestJSONv2UnmarshalPropagatesDecodeErrors checks that a malformed document is
// reported for every tree target, not only through UnmarshalValue.
func TestJSONv2UnmarshalPropagatesDecodeErrors(t *testing.T) {
	t.Parallel()
	document := readTestdata(t, "invalid/unclosed_object.json")
	for _, name := range []string{"*any", "*map", "*slice"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var err error
			switch name {
			case "*any":
				var target any
				err = jsonx.Unmarshal(document, &target)
			case "*map":
				var target map[string]any
				err = jsonx.Unmarshal(document, &target)
			default:
				var target []any
				err = jsonx.Unmarshal(document, &target)
			}
			if err == nil {
				t.Fatalf("Unmarshal(%s) error = nil, want error", name)
			}
			t.Logf("%s: %v", name, err)
		})
	}
}

// TestJSONv2UnmarshalDelegatesOtherTargets covers the default branch: concrete
// Go types are decoded by encoding/json/v2 itself, and its pointer checks still
// apply.
func TestJSONv2UnmarshalDelegatesOtherTargets(t *testing.T) {
	t.Parallel()
	var typed struct {
		Port int `json:"port"`
	}
	if err := jsonx.Unmarshal(readTestdata(t, "numbers.json"), &typed); err != nil {
		t.Fatalf("Unmarshal(struct) error = %v", err)
	}
	// "int" is not a field of typed, so nothing is read; use a matching field.
	if typed.Port != 0 {
		t.Fatalf("Port = %d, want 0 (field absent from the document)", typed.Port)
	}

	var withPort struct {
		Int int `json:"int"`
	}
	if err := jsonx.Unmarshal(readTestdata(t, "numbers.json"), &withPort); err != nil {
		t.Fatalf("Unmarshal(struct) error = %v", err)
	}
	if withPort.Int != 8080 {
		t.Fatalf("Int = %d, want 8080", withPort.Int)
	}

	if err := jsonx.Unmarshal(readTestdata(t, "scalar_string.json"), map[string]any{}); err == nil {
		t.Fatal("Unmarshal(non-pointer) error = nil, want error")
	} else {
		t.Logf("non-pointer rejected: %v", err)
	}
	if err := jsonx.Unmarshal(readTestdata(t, "scalar_string.json"), nil); err == nil {
		t.Fatal("Unmarshal(nil) error = nil, want error")
	} else {
		t.Logf("nil target rejected: %v", err)
	}
}

// The remaining tests document where the default backend — the standard library
// encoding/json/v2, not v1 — is stricter than the sonic backend. Both
// differences are deliberate: they follow the JSON specification and match the
// YAML loader, which already rejects duplicate mapping keys.
//
// Consequences for users switching backends:
//   - a values file with duplicate object members now fails instead of silently
//     keeping the last one;
//   - a values file with invalid UTF-8 inside a string now fails instead of
//     passing the bytes through into the rendered output.
//
// The escape hatches are jsontext.AllowDuplicateNames and
// jsontext.AllowInvalidUTF8, both exercised below: they restore the old
// behaviour if a legacy file needs it.

func TestJSONv2RejectsDuplicateMembers(t *testing.T) {
	t.Parallel()
	document := readTestdata(t, "duplicate_members.json")
	_, err := jsonx.UnmarshalValue(document)
	if err == nil {
		t.Fatal("UnmarshalValue(duplicate members) error = nil, want error")
	}
	t.Logf("duplicate members rejected: %v", err)

	// The escape hatch restores the last-wins behaviour sonic has.
	var relaxed map[string]any
	options := jsontext.AllowDuplicateNames(true)
	if err := jsonv2.Unmarshal(document, &relaxed, options); err != nil {
		t.Fatalf("relaxed unmarshal error = %v", err)
	}
	if got := relaxed["a"]; got != float64(2) {
		t.Fatalf("relaxed a = %#v, want 2", got)
	}
}

func TestJSONv2RejectsInvalidUTF8(t *testing.T) {
	t.Parallel()
	document := readTestdata(t, "invalid_utf8.json")
	_, err := jsonx.UnmarshalValue(document)
	if err == nil {
		t.Fatal("UnmarshalValue(invalid UTF-8) error = nil, want error")
	}
	if !strings.Contains(err.Error(), "UTF-8") {
		t.Logf("invalid UTF-8 rejected with: %v", err)
	}

	// The escape hatch passes the bytes through, which is what sonic does.
	var relaxed any
	options := jsontext.AllowInvalidUTF8(true)
	if err := jsonv2.Unmarshal(document, &relaxed, options); err != nil {
		t.Fatalf("relaxed unmarshal error = %v", err)
	}
}

// Coverage note: Unmarshal is fully covered, and the walker is covered except
// for seven `return nil, err` lines that follow a PeekKind call. Those are
// defensive: PeekKind has already read the token, so the matching ReadToken or
// ReadValue cannot fail on a well-formed decoder state. They stay in the
// implementation rather than as a panic so a future jsontext change surfaces as
// an error instead of a crash.

// TestJSONv2MarshalKeepsOmitemptyZeroInt pins the standard-library backend side
// of the one struct-tag difference between the backends: encoding/json/v2 keeps
// a zero int tagged `omitempty`. Sonic drops it (see sonic_test.go). Features
// that need an omittable number therefore use a pointer (rule 03-13 G20).
func TestJSONv2MarshalKeepsOmitemptyZeroInt(t *testing.T) {
	t.Parallel()
	type record struct {
		Whole int `json:"whole,omitempty"`
	}
	got, err := jsonx.Marshal(record{})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if want := `{"whole":0}`; string(got) != want {
		t.Fatalf("Marshal() = %s, want %s", got, want)
	}
}
