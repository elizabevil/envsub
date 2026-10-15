package source

import (
	"errors"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/values"
)

// TestBuiltinFormatsAreOneSourceOfTruth guards the closed table: every accessor
// reads it, so the extension mapping, the diagnostic name, the source kind and the
// parser must agree, and no extension may be claimed twice.
func TestBuiltinFormatsAreOneSourceOfTruth(t *testing.T) {
	t.Parallel()
	seenExt := map[string]string{}
	seenName := map[string]bool{}
	for _, format := range builtinFormats {
		if format.Name == "" {
			t.Fatalf("format %+v has no name", format)
		}
		if seenName[format.Name] {
			t.Fatalf("format name %q is declared twice", format.Name)
		}
		seenName[format.Name] = true
		if format.Parser == nil {
			t.Fatalf("format %q has no parser", format.Name)
		}
		if got := format.Parser.Name(); got != format.Name {
			t.Fatalf("format %q is parsed by %q: the table and the parser disagree", format.Name, got)
		}
		if len(format.Extensions) == 0 {
			t.Fatalf("format %q claims no extension", format.Name)
		}
		for _, ext := range format.Extensions {
			if ext != strings.ToLower(ext) || !strings.HasPrefix(ext, ".") {
				t.Fatalf("format %q has extension %q: want lowercase and dot-prefixed", format.Name, ext)
			}
			if owner, ok := seenExt[ext]; ok {
				t.Fatalf("extension %q is claimed by both %q and %q", ext, owner, format.Name)
			}
			seenExt[ext] = format.Name
			// The lookup must return the same format the table declares.
			got, err := FormatFor("values" + ext)
			if err != nil {
				t.Fatalf("FormatFor(values%s) error = %v", ext, err)
			}
			if got.Name != format.Name || got.Kind != format.Kind || got.Dotenv != format.Dotenv {
				t.Fatalf("FormatFor(values%s) = %+v, want %+v", ext, got, format)
			}
		}
	}
	// The .env format feeds the envfile stage and is the one --dotenv-file uses.
	if DotenvFormat().Name != DotenvName || !DotenvFormat().Dotenv ||
		DotenvFormat().Kind != values.SourceEnvFile {
		t.Fatalf("DotenvFormat() = %+v, want the dotenv/envfile entry", DotenvFormat())
	}
	// Structured value formats stay on the values stage and are not dotenv.
	for _, format := range []Format{yamlFormat, jsonFormat} {
		if format.Dotenv || format.Kind != values.SourceValues {
			t.Fatalf("format %q = %+v, want values stage without the dotenv marker", format.Name, format)
		}
	}
}

// TestFormatForReportsAnUnknownExtension pins the usage error and the message every
// caller relies on (the extension list is written once, in the table's own spelling).
func TestFormatForReportsAnUnknownExtension(t *testing.T) {
	t.Parallel()
	_, err := FormatFor("values.toml")
	var de *diag.Error
	if err == nil || !errors.As(err, &de) || de.Kind != diag.KindUsage {
		t.Fatalf("FormatFor(values.toml) error = %v, want a KindUsage error", err)
	}
	for _, want := range []string{".env", ".yaml", ".yml", ".json"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("FormatFor error = %v, want it to mention %s", err, want)
		}
	}
}
