package source

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/values"
)

// Loader parses one external value file. It implements values.Loader.
type Loader struct {
	// Name identifies the file format in diagnostics.
	Name string
	// Kind is the layering stage the loader feeds: values for a structured value
	// file, envfile for a .env file. It is the origin category recorded for every
	// value the loader reads.
	Kind values.SourceKind
	// Values is the underlying parser.
	Values values.Loader
	// Dotenv reports whether the loader parses dotenv content. Callers use it to
	// reject a dotenv file passed to --values-file, and to accept any file name
	// for --dotenv-file.
	Dotenv bool
}

// Value file extensions recognised by ForFile.
const (
	extDotenv = ".env"
	extYAML   = ".yaml"
	extYML    = ".yml"
	extJSON   = ".json"
)

// ForFile selects a loader from a file name extension.
func ForFile(path string) (Loader, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case extDotenv:
		return Loader{Name: DotenvName, Kind: values.SourceEnvFile, Values: Environ{}, Dotenv: true}, nil
	case extYAML, extYML:
		return Loader{Name: YAMLName, Kind: values.SourceValues, Values: YAML{}}, nil
	case extJSON:
		return Loader{Name: JSONName, Kind: values.SourceValues, Values: JSON{}}, nil
	default:
		return Loader{}, diag.New(diag.KindUsage,
			"cannot determine value file format for %q: expected %s, %s, %s or %s",
			path, extDotenv, extYAML, extYML, extJSON)
	}
}

// DotenvLoader returns the .env loader. --dotenv-file names its format
// explicitly, so the loader must not depend on the file name: `.env.local`,
// `prod.env` and extensionless files all accept the same content.
func DotenvLoader() Loader {
	return Loader{Name: DotenvName, Kind: values.SourceEnvFile, Values: Environ{}, Dotenv: true}
}

// ReadFile loads one value file, recording path as the origin of its values.
func (l Loader) ReadFile(path string) (*values.Tree, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, diag.New(diag.KindIO, "value file %s does not exist", path)
		}
		return nil, diag.Wrap(diag.KindIO, fmt.Errorf("read %s: %w", path, err))
	}
	tree, err := l.Values.Load(data)
	if err != nil {
		return nil, diag.Wrap(diag.KindType, fmt.Errorf("parse %s: %w", path, err))
	}
	// Attach the file name to every value the loader produced. A dotenv file
	// records one entry per key (so each key keeps its line number); structured
	// value files record one entry per defined path.
	for _, key := range tree.MetaKeys() {
		meta, _ := tree.MetaAt(key)
		meta.File = path
		if meta.Kind == "" {
			meta.Kind = l.Kind
		}
		tree.RecordMeta(key, meta)
	}
	return tree, nil
}
