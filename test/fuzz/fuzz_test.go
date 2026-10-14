// Package fuzz_test holds the parser fuzz targets: every place the tool turns
// untrusted bytes into structure must fail cleanly, never panic, on arbitrary
// input. Run them with, for example:
//
//	go test -fuzz=FuzzTemplateScanner -fuzztime=10s ./test/fuzz/
package fuzz_test

import (
	"testing"

	"github.com/elizabevil/envsub/internal/engine/extended"
	"github.com/elizabevil/envsub/internal/source"
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
