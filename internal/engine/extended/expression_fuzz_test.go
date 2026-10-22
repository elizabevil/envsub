package extended

import (
	"slices"
	"testing"

	"github.com/elizabevil/envsub/internal/pipeline"
	"github.com/elizabevil/envsub/internal/values"
)

// FuzzParseExpression pins that the expression grammar never panics and that a
// successful parse is internally consistent.
//
// The grammar (path, operator, inline | type and the formatter pipeline) is
// project-specific, so there is no standard-library parser to lean on. This
// target lives in the package rather than in test/fuzz because parseExpression
// is unexported; the targets in test/fuzz cover the parsers whose entry points
// are public.
//
// Run it with, for example:
//
//	go test -fuzz=FuzzParseExpression -fuzztime=30s ./internal/engine/extended/
func FuzzParseExpression(f *testing.F) {
	for _, seed := range []string{
		// Plain references and the path grammar.
		"A", "app.port", "hosts[0]", `app["my-key"]`, `["a.b"].k`, "@process.NAME",
		// Every operator, with and without a word.
		"A:-d", "A-d", "A:+alt", "A+alt", "A:?msg", "A?msg", "A:=x", "A=x", "A:-",
		// The formatter pipeline, alone and chained.
		"A | type int", `A | join ","`, `A | indent "2"`, "A | quote",
		"A | toJson", "A | toYaml", "A | toYaml 4", "A | toYaml align",
		"A | toYaml 4 align", "A | toJson align", `A | join "," | indent "2" | quote`,
		"A | type int | join \",\"", "A | type array<string>",
		// Nested defaults and the shapes the grammar must reject.
		"A:-${B}", "A:-${B:-c}", "", " ", "|", "A |", "A | ", "A | type",
		"A | type nonsense", "A | unknown", "A | quote x", "A | indent",
		"A | toYaml 0", "A | toYaml 1 2", "A | join ,", `A | join "unclosed`,
		"A | type array<array<int>>", "A | type int | type int",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		expr, err := parseExpression(raw)
		if err != nil {
			return
		}
		// A parsed path must be a path the grammar accepts on its own, so the
		// report and the lookup can both use it.
		if expr.Path == "" {
			t.Fatalf("parseExpression(%q) = %+v, want a non-empty path", raw, expr)
		}
		if _, err := values.ParsePath(expr.Path); err != nil {
			t.Fatalf("parseExpression(%q) path %q does not parse: %v", raw, expr.Path, err)
		}
		// The operator must be one the grammar knows.
		if expr.Op != "" && !isSupportedOperator(expr.Op) {
			t.Fatalf("parseExpression(%q) = %+v, want a supported operator", raw, expr)
		}
		// A plain reference carries no word.
		if expr.Op == "" && expr.Word != "" {
			t.Fatalf("parseExpression(%q) = %+v, want no word without an operator", raw, expr)
		}
		// An inline type assertion names a type and is recorded on Declared,
		// never as a pipeline step.
		if expr.HasType && expr.Declared.Name == "" {
			t.Fatalf("parseExpression(%q) = %+v, want a named type", raw, expr)
		}
		for _, step := range expr.Steps {
			if step.Name == pipeline.NameType {
				t.Fatalf("parseExpression(%q) = %+v, want the type assertion outside the steps", raw, expr)
			}
			if !slices.Contains(pipeline.Names(), step.Name) {
				t.Fatalf("parseExpression(%q) = %+v, want a known formatter, got %q", raw, expr, step.Name)
			}
		}
	})
}
