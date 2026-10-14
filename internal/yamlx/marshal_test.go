package yamlx_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/yamlx"
)

// TestMarshalMatchesYAMLv3Output pins the encoder's exact output. Sequences are
// indented under their key, which is yaml.v3's style; test/yamlx compares it
// with the rejected sigs.k8s.io/yaml encoder, whose sequences stay at the
// parent indentation.
func TestMarshalMatchesYAMLv3Output(t *testing.T) {
	t.Parallel()
	value, err := yamlx.Decode(readTestdata(t, "encode/mapping.in.yaml"))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	got, err := yamlx.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	want := string(readTestdata(t, "encode/mapping.want.yaml"))
	if string(got) != want {
		t.Fatalf("Marshal() =\n%s\nwant:\n%s", got, want)
	}
}

// TestMarshalKeepsNumbersExact covers the value shapes that yaml.v3 would get
// wrong on its own: a json.Number would be quoted as a string.
func TestMarshalKeepsNumbersExact(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "int", value: json.Number("8080"), want: "8080\n"},
		{name: "float keeps its spelling", value: json.Number("1.50"), want: "1.50\n"},
		{name: "uint64", value: json.Number("18446744073709551615"), want: "18446744073709551615\n"},
		{name: "big integer", value: json.Number("12345678901234567890"), want: "12345678901234567890\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := yamlx.Marshal(tt.value)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("Marshal(%v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

// TestMarshalNilAndEmpty covers the shapes a rendered value can take.
func TestMarshalNilAndEmpty(t *testing.T) {
	t.Parallel()
	var nilMap map[string]any
	var nilSlice []any
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "nil", value: nil, want: "null\n"},
		{name: "typed nil map follows the library default", value: nilMap, want: "{}\n"},
		{name: "typed nil slice follows the library default", value: nilSlice, want: "[]\n"},
		{name: "empty map", value: map[string]any{}, want: "{}\n"},
		{name: "empty array", value: []any{}, want: "[]\n"},
		{name: "scalar string", value: "plain", want: "plain\n"},
		{name: "string needing quotes", value: "8080", want: "\"8080\"\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := yamlx.Marshal(tt.value)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("Marshal() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestMarshalKeepsStructuredValues checks the two-level shape used by the
// renderer's toYaml formatter.
func TestMarshalKeepsStructuredValues(t *testing.T) {
	t.Parallel()
	value := map[string]any{
		"hosts": []any{"a", "b"},
		"app":   map[string]any{"port": 8080},
	}
	got, err := yamlx.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	want := "app:\n  port: 8080\nhosts:\n  - a\n  - b\n"
	if string(got) != want {
		t.Fatalf("Marshal() = %q, want %q", got, want)
	}
	if strings.Contains(string(got), "hosts:\n- ") {
		t.Fatal("sequences must be indented under their key")
	}
}

// TestMarshalIndentSetsTheNestingWidth pins the serializer's width knob: the
// nesting width is the encoder's, not a text operation, so a caller that needs a
// different style asks for it here.
func TestMarshalIndentSetsTheNestingWidth(t *testing.T) {
	t.Parallel()
	value := map[string]any{"outer": map[string]any{"inner": map[string]any{"deep": 1}}}
	for _, tc := range []struct {
		width int
		want  string
	}{
		{width: 0, want: "outer:\n  inner:\n    deep: 1\n"}, // default
		{width: 2, want: "outer:\n  inner:\n    deep: 1\n"},
		{width: 3, want: "outer:\n   inner:\n      deep: 1\n"},
		{width: 4, want: "outer:\n    inner:\n        deep: 1\n"},
	} {
		got, err := yamlx.MarshalIndent(value, tc.width)
		if err != nil {
			t.Fatalf("MarshalIndent(%d) error = %v", tc.width, err)
		}
		if string(got) != tc.want {
			t.Errorf("MarshalIndent(%d) = %q, want %q", tc.width, got, tc.want)
		}
	}
}
