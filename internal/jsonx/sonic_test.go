//go:build sonic

package jsonx_test

import (
	"encoding/json"
	"testing"

	"github.com/elizabevil/envsub/internal/jsonx"
)

// This file documents where the sonic backend is more permissive than the
// default one (the standard library encoding/json/v2). See jsonv2_test.go for
// the stricter behaviour, its rationale and the options that relax it.

func TestSonicKeepsLastDuplicateMember(t *testing.T) {
	t.Parallel()
	value, err := jsonx.UnmarshalValue(readTestdata(t, "duplicate_members.json"))
	if err != nil {
		t.Fatalf("UnmarshalValue(duplicate members) error = %v, want last-wins", err)
	}
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("UnmarshalValue() = %T, want map[string]any", value)
	}
	number, ok := object["a"].(json.Number)
	if !ok || number.String() != "2" {
		t.Fatalf("a = %#v, want json.Number(2)", object["a"])
	}
}

func TestSonicRejectsInvalidUTF8(t *testing.T) {
	t.Parallel()
	// sonic's public decoder cannot validate UTF-8, so jsonx checks the decoded
	// tree; the rule is shared with the standard-library backend (03-12 Q3a).
	if _, err := jsonx.UnmarshalValue(readTestdata(t, "invalid_utf8.json")); err == nil {
		t.Fatal("UnmarshalValue(invalid UTF-8) error = nil, want error")
	}
}

// TestSonicMarshalOmitsOmitemptyZeroInt pins the sonic side of the one
// struct-tag difference between the backends: sonic drops a zero int tagged
// `omitempty`, while encoding/json/v2 keeps it (see jsonv2_test.go). Features
// that need an omittable number therefore use a pointer (rule 03-13 G20).
func TestSonicMarshalOmitsOmitemptyZeroInt(t *testing.T) {
	t.Parallel()
	type record struct {
		Whole int `json:"whole,omitempty"`
	}
	got, err := jsonx.Marshal(record{})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if want := `{}`; string(got) != want {
		t.Fatalf("Marshal() = %s, want %s", got, want)
	}
}
