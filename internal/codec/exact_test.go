package codec_test

import (
	"encoding/json"
	"testing"

	"github.com/elizabevil/envsub/internal/codec"
)

func TestExactIntValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text string
		want bool
	}{
		{text: "123456789012345678901234567890", want: true},
		{text: "-123456789012345678901234567890", want: true},
		{text: "0", want: true},
		{text: "-0", want: true},
		{text: "", want: false},
		{text: "-", want: false},
		{text: "007", want: false},
		{text: "1.5", want: false},
		{text: "1e21", want: false},
		{text: "12a", want: false},
		{text: " 1", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			t.Parallel()
			value, ok := codec.New(tt.text)
			if ok != tt.want {
				t.Fatalf("New(%q) ok = %v, want %v", tt.text, ok, tt.want)
			}
			if ok && value.String() != tt.text {
				t.Fatalf("String() = %q, want %q", value.String(), tt.text)
			}
		})
	}
}

// TestExactIntMarshalsAsNumber pins rule N2: the digits go out as a bare JSON
// number, never as a quoted string.
func TestExactIntMarshalsAsNumber(t *testing.T) {
	t.Parallel()
	got, err := json.Marshal(map[string]any{"id": codec.ExactInt("123456789012345678901234567890")})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if want := `{"id":123456789012345678901234567890}`; string(got) != want {
		t.Fatalf("Marshal() = %s, want %s", got, want)
	}
	if _, err := json.Marshal(codec.ExactInt("12a")); err == nil {
		t.Fatal("Marshal(invalid) error = nil, want error")
	}
}

// TestExactIntConversion covers the schema boundary helper (rule step 4).
func TestExactIntConversion(t *testing.T) {
	t.Parallel()
	original := map[string]any{"small": 1, "big": codec.ExactInt("123456789012345678901234567890")}
	converted, ok := codec.ToJSON(original).(map[string]any)
	if !ok {
		t.Fatal("ToJSON() did not return a map")
	}
	if _, isNumber := converted["big"].(json.Number); !isNumber {
		t.Fatalf("ToJSON() big = %#v (%T), want json.Number", converted["big"], converted["big"])
	}
	if _, unchanged := converted["small"].(int); !unchanged {
		t.Fatalf("ToJSON() small = %#v, want the original int", converted["small"])
	}

	// A tree without exact integers is returned as-is (no copy in the common
	// case), which is checked by mutating the result and looking at the input.
	plain := map[string]any{"a": 1}
	same, ok := codec.ToJSON(plain).(map[string]any)
	if !ok {
		t.Fatalf("ToJSON() = %T, want map[string]any", codec.ToJSON(plain))
	}
	same["b"] = 2
	if _, shared := plain["b"]; !shared {
		t.Fatal("ToJSON() copied a tree that had nothing to convert")
	}
}
