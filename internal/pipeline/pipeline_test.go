package pipeline_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/pipeline"
	"github.com/elizabevil/envsub/internal/types"
)

func TestApply(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		value any
		steps []pipeline.Step
		want  string
	}{
		{
			name:  "join with separator",
			value: []any{"a", "b", "c"},
			steps: []pipeline.Step{{Name: pipeline.NameJoin, Arg: ",", HasArg: true}},
			want:  "a,b,c",
		},
		{
			name:  "join default separator",
			value: []any{"a", "b"},
			steps: []pipeline.Step{{Name: pipeline.NameJoin}},
			want:  "a,b",
		},
		{
			name:  "join numbers",
			value: []any{1, 2},
			steps: []pipeline.Step{{Name: pipeline.NameJoin, Arg: "-", HasArg: true}},
			want:  "1-2",
		},
		{
			name:  "toJson",
			value: []any{"a", "b"},
			steps: []pipeline.Step{{Name: pipeline.NameToJSON}},
			want:  `["a","b"]`,
		},
		{
			name:  "toJson object keeps integer spelling",
			value: map[string]any{"port": 8080},
			steps: []pipeline.Step{{Name: pipeline.NameToJSON}},
			want:  `{"port":8080}`,
		},
		{
			name:  "toYaml",
			value: map[string]any{"port": 8080},
			steps: []pipeline.Step{{Name: pipeline.NameToYAML}},
			want:  "port: 8080",
		},
		{
			name:  "quote",
			value: "a b",
			steps: []pipeline.Step{{Name: pipeline.NameQuote}},
			want:  `"a b"`,
		},
		{
			name:  "indent",
			value: "a\nb",
			steps: []pipeline.Step{{Name: pipeline.NameIndent, Arg: "2", HasArg: true}},
			want:  "  a\n  b",
		},
		{
			name:  "indent zero",
			value: "a",
			steps: []pipeline.Step{{Name: pipeline.NameIndent, Arg: "0", HasArg: true}},
			want:  "a",
		},
		{
			name:  "chained",
			value: []any{"a", "b"},
			steps: []pipeline.Step{
				{Name: pipeline.NameJoin, Arg: ",", HasArg: true},
				{Name: pipeline.NameQuote},
			},
			want: `"a,b"`,
		},
		{
			name:  "toYaml then indent",
			value: map[string]any{"a": 1},
			steps: []pipeline.Step{
				{Name: pipeline.NameToYAML},
				{Name: pipeline.NameIndent, Arg: "2", HasArg: true},
			},
			want: "  a: 1",
		},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := pipeline.Apply(tt.value, tt.steps)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			text, ok := got.(string)
			if !ok {
				t.Fatalf("Apply() = %#v, want string", got)
			}
			if text != tt.want {
				t.Fatalf("Apply() = %q, want %q", text, tt.want)
			}
		})
	}
}

func TestApplyErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		value any
		steps []pipeline.Step
	}{
		{name: "join scalar", value: "a", steps: []pipeline.Step{{Name: pipeline.NameJoin}}},
		{name: "join nested array", value: []any{[]any{"a"}}, steps: []pipeline.Step{{Name: pipeline.NameJoin}}},
		{name: "join object", value: []any{map[string]any{"a": 1}}, steps: []pipeline.Step{{Name: pipeline.NameJoin}}},
		{name: "indent without argument", value: "a", steps: []pipeline.Step{{Name: pipeline.NameIndent}}},
		{name: "indent non integer", value: "a", steps: []pipeline.Step{{Name: pipeline.NameIndent, Arg: "x", HasArg: true}}},
		{name: "indent negative", value: "a", steps: []pipeline.Step{{Name: pipeline.NameIndent, Arg: "-1", HasArg: true}}},
		{name: "indent structured", value: []any{"a"}, steps: []pipeline.Step{{Name: pipeline.NameIndent, Arg: "2", HasArg: true}}},
		{name: "quote structured", value: []any{"a"}, steps: []pipeline.Step{{Name: pipeline.NameQuote}}},
		{name: "unknown formatter", value: "a", steps: []pipeline.Step{{Name: "nope"}}},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := pipeline.Apply(tt.value, tt.steps); err == nil {
				t.Fatalf("Apply() error = nil, want error")
			}
		})
	}
}

func TestNames(t *testing.T) {
	t.Parallel()
	names := pipeline.Names()
	if len(names) != 5 {
		t.Fatalf("Names() = %#v, want 5 entries", names)
	}
	if strings.Join(names, ",") != "indent,join,quote,toJson,toYaml" {
		t.Fatalf("Names() = %#v", names)
	}
}

// TestToYamlParameters pins the serializer's two parameters: the nesting width
// belongs to the serializer, and align labels the result so the substitution
// layer knows it may re-indent the block.
func TestToYamlParameters(t *testing.T) {
	t.Parallel()
	value := map[string]any{"inner": map[string]any{"deep": 1}}

	for _, tc := range []struct {
		name  string
		step  pipeline.Step
		lines []string
		align bool
	}{
		{
			name:  "default width is two",
			step:  pipeline.Step{Name: pipeline.NameToYAML},
			lines: []string{"inner:", "  deep: 1"},
		},
		{
			name:  "width three",
			step:  pipeline.Step{Name: pipeline.NameToYAML, Width: 3},
			lines: []string{"inner:", "   deep: 1"},
		},
		{
			name:  "align marks the result",
			step:  pipeline.Step{Name: pipeline.NameToYAML, Align: true},
			lines: []string{"inner:", "  deep: 1"},
			align: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := pipeline.Apply(value, []pipeline.Step{tc.step})
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			if _, aligned := pipeline.Text(got); aligned != tc.align {
				t.Errorf("aligned = %v, want %v", aligned, tc.align)
			}
			text, err := types.Stringify(got)
			if err != nil {
				t.Fatalf("Stringify() error = %v", err)
			}
			if got, want := strings.Split(text, "\n"), tc.lines; !slices.Equal(got, want) {
				t.Errorf("lines = %q, want %q", got, want)
			}
		})
	}
}

// TestAlignedSerializerRejectsIndent pins the mutual exclusion: align and indent
// both define a column, so combining them is a usage error rather than a silent
// double indent.
func TestAlignedSerializerRejectsIndent(t *testing.T) {
	t.Parallel()
	value := map[string]any{"a": 1}
	aligned, err := pipeline.Apply(value, []pipeline.Step{{Name: pipeline.NameToYAML, Align: true}})
	if err != nil {
		t.Fatalf("Apply(toYaml align) error = %v", err)
	}
	_, err = pipeline.Apply(aligned, []pipeline.Step{{Name: pipeline.NameIndent, Arg: "2", HasArg: true}})
	if err == nil {
		t.Fatal("Apply(indent after align) error = nil, want a usage error")
	}
	if kind := diag.KindOf(err); kind != diag.KindUsage {
		t.Errorf("error kind = %q, want %q", kind, diag.KindUsage)
	}
}

// TestToJsonTakesNoWidth pins that a single-line serializer refuses a nesting
// width instead of ignoring it.
func TestToJsonTakesNoWidth(t *testing.T) {
	t.Parallel()
	_, err := pipeline.Apply(map[string]any{"a": 1}, []pipeline.Step{{Name: pipeline.NameToJSON, Width: 2}})
	if err == nil {
		t.Fatal("Apply(toJson 2) error = nil, want an error")
	}
	if kind := diag.KindOf(err); kind != diag.KindUsage {
		t.Errorf("error kind = %q, want %q", kind, diag.KindUsage)
	}
}
