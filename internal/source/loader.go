package source

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/values"
)

// Loader parses one external value file. It implements values.Loader and is built
// from a Format, so the format metadata lives in exactly one place (format.go).
type Loader struct {
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

// ForFile selects a loader from a file name extension.
func ForFile(path string) (Loader, error) {
	format, err := FormatFor(path)
	if err != nil {
		return Loader{}, err
	}
	return format.Loader(), nil
}

// Name identifies the file format in diagnostics. The parser owns the name, so a
// Loader never carries a second copy of it.
func (l Loader) Name() string { return l.Values.Name() }

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
	tree.WalkMeta(func(segments []values.Segment, meta values.Meta) {
		meta.File = path
		if meta.Kind == "" {
			meta.Kind = l.Kind
		}
		tree.RecordMeta(segments, meta)
	})
	return tree, nil
}
