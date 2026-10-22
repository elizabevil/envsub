// Package fuzz_test holds the parser fuzz targets: every place the tool turns
// untrusted bytes into structure must fail cleanly, never panic, on arbitrary
// input. Run them with, for example:
//
//	go test -fuzz=FuzzTemplateScanner -fuzztime=10s ./test/fuzz/
package fuzz_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/elizabevil/envsub/internal/codec"
	"github.com/elizabevil/envsub/internal/engine/extended"
	"github.com/elizabevil/envsub/internal/source"
	"github.com/elizabevil/envsub/internal/types"
	"github.com/elizabevil/envsub/internal/values"
)

// FuzzTemplateScanner pins that scanning arbitrary template bytes either
// succeeds or returns an error.
func FuzzTemplateScanner(f *testing.F) {
	for _, seed := range []string{
		"", "plain", "$NAME", "${app.port}", "${A:-d}", "$${escaped}", "$1",
		"${@process.NAME}", "${app.labels[\"a.b\"]}", "${A:=x}", "${前}",
		"${", "}", "${A | type int}", "${\"unclosed}",
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(_ *testing.T, data []byte) {
		_, _ = extended.Scan(data)
	})
}

// FuzzTemplateSpans pins the streaming scanner: spans must never overlap or
// panic, whatever the input.
func FuzzTemplateSpans(f *testing.F) {
	for _, seed := range []string{"", "before ${A} after", "$$", "${A"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		last := 0
		_ = extended.ScanSpans(data, func(span extended.Span) error {
			if span.Start < last || span.End < span.Start {
				t.Fatalf("span %d..%d is out of order after %d", span.Start, span.End, last)
			}
			last = span.End
			return nil
		})
	})
}

// FuzzPathParser pins the template path grammar.
func FuzzPathParser(f *testing.F) {
	for _, seed := range []string{
		"", "NAME", "app.port", "hosts[0].name", `labels["a.b"]`, `["extra.key"]`,
		"@process.NAME", `@process["MY-VAR"]`, "[", "]", "a..b", `["unclosed`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, path string) {
		segments, err := values.ParsePath(path)
		if err != nil {
			return
		}
		// A parsed path must round-trip through CanonicalPath and re-parse.
		spelling := values.CanonicalPath(segments)
		if _, err := values.ParsePath(spelling); err != nil {
			t.Fatalf("Canonical(%q) = %q does not re-parse: %v", path, spelling, err)
		}
	})
}

// FuzzDotenvParser pins the .env parser.
func FuzzDotenvParser(f *testing.F) {
	for _, seed := range []string{
		"", "A=1\n", "A=\"quoted\"\n", "A='single'\n", "export A=1\n",
		"A=1 # comment\n", "A=\n", "=1\n", "A", "A=\"unterminated\n", "A=1\r\n",
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(_ *testing.T, data []byte) {
		_, _ = source.ParseDotenv(data)
	})
}

// FuzzExpressionPaths pins the expression parser through its exported door: every
// path the engine extracts from a template must be a path the grammar accepts.
//
// The scanner alone only tokenizes; the operator and pipeline grammar lives in
// parseExpression, which Paths reaches for every reference that is not a bare
// name. A path it reports must therefore parse, canonicalize and re-parse.
func FuzzExpressionPaths(f *testing.F) {
	for _, seed := range []string{
		"", "plain", "$A", "${A}", "${app.port}", "${A:-d}", "${A | join \",\"}",
		`${app["my-key"]}`, "${hosts[0]}", "${@process.NAME}", "${A:=x}",
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		paths, err := extended.New().Paths(data)
		if err != nil {
			return
		}
		for _, path := range paths {
			if path == "" {
				t.Fatalf("Paths(%q) reported an empty path", data)
			}
			segments, err := values.ParsePath(path)
			if err != nil {
				t.Fatalf("Paths(%q) reported %q, which does not parse: %v", data, path, err)
			}
			spelling := values.CanonicalPath(segments)
			if _, err := values.ParsePath(spelling); err != nil {
				t.Fatalf("Paths(%q) reported %q, whose canonical %q does not re-parse: %v",
					data, path, spelling, err)
			}
		}
	})
}

// FuzzNumberText pins the number ladder: reading a number out of text must never
// panic, must never round an integer into a float64 behind the caller's back,
// and must never produce a non-finite float.
//
// The rule is that nothing widens to float64 while an exact representation
// exists, so an int assertion yields an exact integer or an error.
func FuzzNumberText(f *testing.F) {
	for _, seed := range []string{
		"", "0", "1", "-1", "+1", "007", "8080", " 12 ", "12 ",
		"9223372036854775807", "9223372036854775808", "18446744073709551615",
		"18446744073709551616", "123456789012345678901234567890",
		"1.5", "-0.0", "1e3", "1E-3", "1e400", "1e-400", ".5", "5.",
		"NaN", "Inf", "-Inf", "Infinity", "0x10", "0b1", "1_000", "1e", "1.2.3",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		// An int assertion either fails or yields an exact integer.
		if value, err := types.Assert(text, types.Declared{Name: types.NameInt}); err == nil {
			switch value.(type) {
			case int, uint64, codec.ExactInt:
			default:
				t.Fatalf("Assert(%q, int) = %T (%v), want an exact integer", text, value, value)
			}
		}
		// A float assertion never yields a non-finite value.
		if value, err := types.Assert(text, types.Declared{Name: types.NameFloat}); err == nil {
			if number, ok := value.(float64); ok && (math.IsNaN(number) || math.IsInf(number, 0)) {
				t.Fatalf("Assert(%q, float) = %v, want a finite number", text, number)
			}
		}
		// The decoder-side ladders keep the same promise.
		normalized := []any{
			values.NormalizeNumbers(json.Number(text)),
			types.Normalize(json.Number(text)),
		}
		for _, value := range normalized {
			if number, ok := value.(float64); ok && (math.IsNaN(number) || math.IsInf(number, 0)) {
				t.Fatalf("Normalize(%q) = %v, want a finite number", text, number)
			}
		}
	})
}
