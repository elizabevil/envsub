package types_test

import (
	"encoding/json"
	"testing"

	"github.com/elizabevil/envsub/internal/types"
)

func TestParseType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		raw       string
		want      types.Declared
		wantError bool
	}{
		{raw: "string", want: types.Declared{Name: types.NameString}},
		{raw: "int", want: types.Declared{Name: types.NameInt}},
		{raw: "float", want: types.Declared{Name: types.NameFloat}},
		{raw: "bool", want: types.Declared{Name: types.NameBool}},
		{raw: "object", want: types.Declared{Name: types.NameObject}},
		{raw: "array", want: types.Declared{Name: types.NameArray}},
		{raw: "array<string>", want: types.Declared{Name: types.NameArray, Element: types.NameString}},
		{raw: "array<int>", want: types.Declared{Name: types.NameArray, Element: types.NameInt}},
		{raw: "map", wantError: true},
		{raw: "array<array<int>>", wantError: true},
		{raw: "array<>", wantError: true},
		{raw: "", wantError: true},
	}
	for _, tt := range tests {

		t.Run(tt.raw, func(t *testing.T) {
			t.Parallel()
			got, err := types.ParseType(tt.raw)
			if tt.wantError {
				if err == nil {
					t.Fatalf("ParseType(%q) error = nil, want error", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseType(%q) error = %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("ParseType(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestDeclaredString(t *testing.T) {
	t.Parallel()
	if got := (types.Declared{Name: types.NameArray, Element: types.NameString}).String(); got != "array<string>" {
		t.Fatalf("String() = %q, want array<string>", got)
	}
	if got := (types.Declared{Name: types.NameInt}).String(); got != "int" {
		t.Fatalf("String() = %q, want int", got)
	}
}

func TestAssertFromString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		text      string
		declared  types.Declared
		want      any
		wantError bool
	}{
		{name: "string", text: "x", declared: types.Declared{Name: types.NameString}, want: "x"},
		{name: "int", text: "8080", declared: types.Declared{Name: types.NameInt}, want: 8080},
		{name: "negative int", text: "-1", declared: types.Declared{Name: types.NameInt}, want: -1},
		{name: "int rejects float", text: "1.5", declared: types.Declared{Name: types.NameInt}, wantError: true},
		{name: "int rejects blank", text: " 1 ", declared: types.Declared{Name: types.NameInt}, wantError: true},
		{name: "float", text: "0.5", declared: types.Declared{Name: types.NameFloat}, want: 0.5},
		{name: "bool true", text: "true", declared: types.Declared{Name: types.NameBool}, want: true},
		{name: "bool false", text: "false", declared: types.Declared{Name: types.NameBool}, want: false},
		{name: "bool rejects 1", text: "1", declared: types.Declared{Name: types.NameBool}, wantError: true},
		{name: "bool rejects yes", text: "yes", declared: types.Declared{Name: types.NameBool}, wantError: true},
		{name: "array", text: `["a","b"]`, declared: types.Declared{Name: types.NameArray}, want: []any{"a", "b"}},
		{name: "array of int", text: `[1,2]`, declared: types.Declared{Name: types.NameArray, Element: types.NameInt}, want: []any{1, 2}},
		{name: "array element error", text: `[1,"x"]`, declared: types.Declared{Name: types.NameArray, Element: types.NameInt}, wantError: true},
		{name: "object", text: `{"a":1}`, declared: types.Declared{Name: types.NameObject}, want: map[string]any{"a": json.Number("1")}},
		{name: "array rejects object", text: `{"a":1}`, declared: types.Declared{Name: types.NameArray}, wantError: true},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := types.Assert(tt.text, tt.declared)
			if tt.wantError {
				if err == nil {
					t.Fatalf("Assert() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Assert() error = %v", err)
			}
			if !equalValue(got, tt.want) {
				t.Fatalf("Assert() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestAssertStructured(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		value     any
		declared  types.Declared
		wantError bool
	}{
		{name: "int from int", value: 1, declared: types.Declared{Name: types.NameInt}},
		{name: "int from whole float", value: float64(2), declared: types.Declared{Name: types.NameInt}},
		{name: "int rejects fraction", value: 1.5, declared: types.Declared{Name: types.NameInt}, wantError: true},
		{name: "float from int", value: 1, declared: types.Declared{Name: types.NameFloat}},
		{name: "float from string", value: "1", declared: types.Declared{Name: types.NameFloat}},
		{name: "bool", value: true, declared: types.Declared{Name: types.NameBool}},
		{name: "bool from string", value: "true", declared: types.Declared{Name: types.NameBool}},
		{name: "object", value: map[string]any{"a": 1}, declared: types.Declared{Name: types.NameObject}},
		{name: "object rejects array", value: []any{1}, declared: types.Declared{Name: types.NameObject}, wantError: true},
		{name: "array of string", value: []any{"a"}, declared: types.Declared{Name: types.NameArray, Element: types.NameString}},
		{name: "array element mismatch", value: []any{1}, declared: types.Declared{Name: types.NameArray, Element: types.NameString}, wantError: true},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := types.Assert(tt.value, tt.declared)
			if tt.wantError && err == nil {
				t.Fatalf("Assert() error = nil, want error")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("Assert() error = %v", err)
			}
		})
	}
}

func TestStringifyValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		value     any
		want      string
		wantError bool
	}{
		{name: "string", value: "x", want: "x"},
		{name: "int", value: 8080, want: "8080"},
		{name: "int64", value: int64(8080), want: "8080"},
		{name: "float whole", value: float64(8080), want: "8080"},
		{name: "float fraction", value: 0.5, want: "0.5"},
		{name: "bool", value: true, want: "true"},
		{name: "nil", value: nil, want: ""},
		{name: "json number integer", value: json.Number("8080"), want: "8080"},
		{name: "json number float", value: json.Number("0.5"), want: "0.5"},
		{name: "array needs formatter", value: []any{"a"}, wantError: true},
		{name: "object needs formatter", value: map[string]any{"a": 1}, wantError: true},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := types.Stringify(tt.value)
			if tt.wantError {
				if err == nil {
					t.Fatalf("Stringify() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Stringify() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("Stringify() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDescribe(t *testing.T) {
	t.Parallel()
	tests := []struct {
		value any
		want  string
	}{
		{value: nil, want: "null"},
		{value: "x", want: "string"},
		{value: true, want: "bool"},
		{value: 1, want: "number"},
		{value: 0.5, want: "float"},
		{value: []any{}, want: "array"},
		{value: map[string]any{}, want: "object"},
	}
	for _, tt := range tests {

		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			if got := types.Describe(tt.value); got != tt.want {
				t.Fatalf("Describe(%#v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func equalValue(got, want any) bool {
	switch wantTyped := want.(type) {
	case []any:
		gotTyped, ok := got.([]any)
		if !ok || len(gotTyped) != len(wantTyped) {
			return false
		}
		for i := range gotTyped {
			if !equalValue(gotTyped[i], wantTyped[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		gotTyped, ok := got.(map[string]any)
		if !ok || len(gotTyped) != len(wantTyped) {
			return false
		}
		for key, value := range wantTyped {
			if !equalValue(gotTyped[key], value) {
				return false
			}
		}
		return true
	default:
		return got == want
	}
}
