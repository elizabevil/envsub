package source

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/values"
)

// Value file extensions recognised by FormatFor.
const (
	extDotenv = ".env"
	extYAML   = ".yaml"
	extYML    = ".yml"
	extJSON   = ".json"
)

// Format describes one value file format: the name it reports in diagnostics, the
// extensions that select it, the layering stage it feeds, whether its content is
// dotenv, and the parser that decodes it. The built-in set is closed: an external
// format arrives through the plugin overlay described in
// docs/03-design/03-10-external-plugin-protocol.md, never by extending this table
// at run time.
type Format struct {
	Name       string
	Extensions []string
	Kind       values.SourceKind
	Dotenv     bool
	Parser     values.Loader
}

// The built-in formats, declared once. Every accessor reads this table, so the
// extension mapping, the source kind and the dotenv marker cannot drift apart.
var (
	yamlFormat   = Format{Name: YAMLName, Extensions: []string{extYAML, extYML}, Kind: values.SourceValues, Parser: YAML{}}
	jsonFormat   = Format{Name: JSONName, Extensions: []string{extJSON}, Kind: values.SourceValues, Parser: JSON{}}
	dotenvFormat = Format{Name: DotenvName, Extensions: []string{extDotenv}, Kind: values.SourceEnvFile, Dotenv: true, Parser: Environ{}}

	builtinFormats = []Format{yamlFormat, jsonFormat, dotenvFormat}
)

// FormatFor resolves the format of a path from its file name extension.
func FormatFor(path string) (Format, error) {
	ext := strings.ToLower(filepath.Ext(path))
	for _, format := range builtinFormats {
		if slices.Contains(format.Extensions, ext) {
			return format, nil
		}
	}
	return Format{}, diag.New(diag.KindUsage,
		"cannot determine value file format for %q: expected %s, %s, %s or %s",
		path, extDotenv, extYAML, extYML, extJSON)
}

// DotenvFormat returns the environment-style format. --dotenv-file names its format
// explicitly, so the loader must not depend on the file name: `.env.local`,
// `prod.env` and extensionless files all accept the same content.
func DotenvFormat() Format { return dotenvFormat }

// Loader builds the acquire/decode pair for this format.
func (f Format) Loader() Loader {
	return Loader{Kind: f.Kind, Values: f.Parser, Dotenv: f.Dotenv}
}
