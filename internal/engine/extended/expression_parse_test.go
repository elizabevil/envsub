package extended

import (
	"reflect"
	"testing"

	"github.com/elizabevil/envsub/internal/pipeline"
	"github.com/elizabevil/envsub/internal/types"
)

// TestSplitPipesCases pins the pipeline splitter that the parser optimization
// rewrote (substring slicing instead of a byte-by-byte builder). Every case is
// visible here, so the segment values and error messages can be reviewed without
// running throwaway commands.
func TestSplitPipesCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "single segment", raw: "A", want: []string{"A"}},
		{name: "bare pipeline", raw: "A | toJson", want: []string{"A ", " toJson"}},
		{name: "two formatters", raw: "A | toJson | indent 2", want: []string{"A ", " toJson ", " indent 2"}},
		{name: "leading pipe", raw: "| toJson", want: []string{"", " toJson"}},
		{name: "trailing pipe", raw: "A |", want: []string{"A ", ""}},
		{name: "consecutive pipes", raw: "A || toJson", want: []string{"A ", "", " toJson"}},
		{name: "double-quoted pipe stays inside", raw: `A | join "a|b"`, want: []string{"A ", ` join "a|b"`}},
		{name: "single-quoted pipe stays inside", raw: `A | join 'a|b'`, want: []string{"A ", ` join 'a|b'`}},
		{name: "quote then pipe", raw: `A | join "x" | quote`, want: []string{"A ", ` join "x" `, " quote"}},
		{name: "empty", raw: "", want: []string{""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := splitPipes(tt.raw)
			if err != nil {
				t.Fatalf("splitPipes(%q) error = %v", tt.raw, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("splitPipes(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

// TestSplitPipesRejectsUnclosedQuote keeps the error text of the rewritten
// splitter under test.
func TestSplitPipesRejectsUnclosedQuote(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{`A | join "x`, `A | join 'x`, `"x`} {
		if _, err := splitPipes(raw); err == nil {
			t.Fatalf("splitPipes(%q) error = nil, want an unclosed quote error", raw)
		} else if want := "unclosed quote in "; err.Error()[:len(want)] != want {
			t.Fatalf("splitPipes(%q) error = %q, want prefix %q", raw, err, want)
		}
	}
}

// TestParseExpressionCases pins the parsed shape of every expression form the
// parser handles: operators, inline types and pipeline steps. It is the single
// parse-shape test (the former TestParseExpression and
// TestParseExpressionPipeline were merged into it) and compares the whole
// expression with reflect.DeepEqual, so a dropped Path, Op, Word, Declared,
// HasType or Step field fails here.
func TestParseExpressionCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
		want expression
	}{
		{name: "bare", raw: "A", want: expression{Path: "A"}},
		{name: "default", raw: "A:-x", want: expression{Path: "A", Op: ":-", Word: "x"}},
		{name: "empty default", raw: "A:-", want: expression{Path: "A", Op: ":-", Word: ""}},
		{name: "default unset", raw: "A-x", want: expression{Path: "A", Op: "-", Word: "x"}},
		{name: "alternate", raw: "app.port:+x", want: expression{Path: "app.port", Op: ":+", Word: "x"}},
		{name: "required if set", raw: "A?boom", want: expression{Path: "A", Op: "?", Word: "boom"}},
		{name: "required with type", raw: "A:?msg | type int", want: expression{
			Path: "A", Op: ":?", Word: "msg", HasType: true, Declared: types.Declared{Name: types.NameInt},
		}},
		{name: "quoted formatter argument", raw: `A | join ","`, want: expression{
			Path: "A", Steps: []pipeline.Step{{Name: pipeline.NameJoin, Arg: ",", HasArg: true}},
		}},
		{name: "join default separator", raw: "hosts | join", want: expression{
			Path: "hosts", Steps: []pipeline.Step{{Name: pipeline.NameJoin}},
		}},
		{name: "chained", raw: `hosts | join "," | quote`, want: expression{
			Path: "hosts",
			Steps: []pipeline.Step{
				{Name: pipeline.NameJoin, Arg: ",", HasArg: true},
				{Name: pipeline.NameQuote},
			},
		}},
		{name: "multi step", raw: `A | toJson | indent 2`, want: expression{
			Path: "A",
			Steps: []pipeline.Step{
				{Name: pipeline.NameToJSON},
				{Name: pipeline.NameIndent, Arg: "2", HasArg: true},
			},
		}},
		{name: "indent", raw: "app | toYaml | indent 2", want: expression{
			Path: "app",
			Steps: []pipeline.Step{
				{Name: pipeline.NameToYAML},
				{Name: pipeline.NameIndent, Arg: "2", HasArg: true},
			},
		}},
		{name: "inline type first", raw: "PORT | type int", want: expression{
			Path: "PORT", HasType: true, Declared: types.Declared{Name: types.NameInt},
		}},
		{name: "inline type then formatter", raw: "HOSTS | type array<string> | toJson", want: expression{
			Path:     "HOSTS",
			HasType:  true,
			Declared: types.Declared{Name: types.NameArray, Element: types.NameString},
			Steps:    []pipeline.Step{{Name: pipeline.NameToJSON}},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseExpression(tt.raw)
			if err != nil {
				t.Fatalf("parseExpression(%q) error = %v", tt.raw, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseExpression(%q) = %+v, want %+v", tt.raw, got, tt.want)
			}
		})
	}
}

// TestParseSerializerArguments pins the parameter grammar of the serializers:
// toYaml takes an optional nesting width and/or align, toJson only align.
func TestParseSerializerArguments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		raw       string
		wantWidth int
		wantAlign bool
	}{
		{raw: "toYaml", wantWidth: 0},
		{raw: "toYaml 3", wantWidth: 3},
		{raw: "toYaml align", wantAlign: true},
		{raw: "toYaml 3 align", wantWidth: 3, wantAlign: true},
		{raw: "toYaml align 3", wantWidth: 3, wantAlign: true},
		{raw: "toJson", wantWidth: 0},
		{raw: "toJson align", wantAlign: true},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			t.Parallel()
			step, _, _, err := parseStep(tt.raw)
			if err != nil {
				t.Fatalf("parseStep(%q) error = %v", tt.raw, err)
			}
			if step.Width != tt.wantWidth || step.Align != tt.wantAlign {
				t.Errorf("parseStep(%q) = width %d align %v, want width %d align %v",
					tt.raw, step.Width, step.Align, tt.wantWidth, tt.wantAlign)
			}
		})
	}
}

// TestParseSerializerArgumentsRejected pins the parameter errors: a width that is
// not a positive integer, a width on a single-line serializer, and duplicates.
func TestParseSerializerArgumentsRejected(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		"toYaml 0",           // not positive
		"toYaml -1",          // negative
		"toYaml abc",         // not a number or align
		"toYaml 2 3",         // two widths
		"toYaml align align", // repeated flag word
		"toJson 3",           // single-line serializer takes no width
		"toJson 3 align",     // neither form is accepted
	} {
		if _, _, _, err := parseStep(raw); err == nil {
			t.Errorf("parseStep(%q) error = nil, want a parameter error", raw)
		}
	}
}
