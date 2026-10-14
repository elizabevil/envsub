//go:build !sonic

package jsonx

import (
	"reflect"
	"testing"
)

// TestDecodeRejectsUnknownValue covers the defensive default branch of the
// tree decoder: encoding/json/v2 only hands out valid JSON values, so this guards
// against a future jsontext or v2 change rather than a reachable input.
func TestDecodeRejectsUnknownValue(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"", "   ", "not json"} {
		var target any
		if err := tree.decode([]byte(raw), &target); err == nil {
			t.Fatalf("tree.decode(%q) error = nil, want error", raw)
		}
	}
}

// TestUnmarshalTreeMatchesUnmarshalValue keeps the tree targets on the same
// code path: Unmarshal(*any) and UnmarshalValue must agree.
func TestUnmarshalTreeMatchesUnmarshalValue(t *testing.T) {
	t.Parallel()
	document := []byte(`{"port":8080,"list":[1,2],"nested":{"big":18446744073709551615}}`)

	value, err := UnmarshalValue(document)
	if err != nil {
		t.Fatalf("UnmarshalValue() error = %v", err)
	}
	var target any
	if err := Unmarshal(document, &target); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !reflect.DeepEqual(value, target) {
		t.Fatalf("UnmarshalValue() = %#v, Unmarshal(*any) = %#v", value, target)
	}

	var object map[string]any
	if err := Unmarshal(document, &object); err != nil {
		t.Fatalf("Unmarshal(*map) error = %v", err)
	}
	if !reflect.DeepEqual(object, value) {
		t.Fatalf("Unmarshal(*map) = %#v, want %#v", object, value)
	}
}
