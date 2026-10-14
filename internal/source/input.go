package source

import (
	"io"
	"os"

	"github.com/elizabevil/envsub/internal/utils"
)

// Input is the source of one template render. The render layer consumes every
// kind through this interface, and implementations must not assume they can
// seek: an HTTP body or an RPC stream cannot be rewound.
//
// The interface also fits a streaming transport, whose Open would return a
// long-lived stream and Size would report ok == false, keeping the renderer
// sequential. No such transport is wired yet.
type Input interface {
	// Name identifies the input in diagnostics: a path, "-" or a URL.
	Name() string
	// Open returns a fresh stream positioned at the start; the caller owns it and
	// must close it.
	Open() (io.ReadCloser, error)
	// Size reports the input length in bytes; ok is false when it is unknown, as
	// for stdin or a chunked HTTP body.
	Size() (size int64, ok bool)
}

// FileInput reads a template from a file: the only input kind with a known size,
// and therefore the only one the parallel renderer may split.
type FileInput struct {
	Path string
}

// NewFileInput returns a file-backed template input.
func NewFileInput(path string) FileInput { return FileInput{Path: path} }

// Name implements Input.
func (f FileInput) Name() string { return f.Path }

// Open implements Input.
func (f FileInput) Open() (io.ReadCloser, error) {
	return os.Open(f.Path)
}

// Size implements Input. A missing or non-regular file reports an unknown
// size; Open surfaces the actual error.
func (f FileInput) Size() (int64, bool) {
	info, err := os.Stat(f.Path)
	if err != nil || info.IsDir() {
		return 0, false
	}
	return info.Size(), true
}

// StdinInput reads a template from standard input. Its length is never known,
// so it is always rendered as a sequential stream.
type StdinInput struct {
	// Reader overrides os.Stdin. It exists for tests and for embedding; nil
	// means os.Stdin.
	Reader io.Reader
}

// Name implements Input.
func (s StdinInput) Name() string { return utils.StdioPath }

// Open implements Input. Closing the returned reader is a no-op: the caller
// must be able to close every stream without closing the process stdin.
func (s StdinInput) Open() (io.ReadCloser, error) {
	reader := s.Reader
	if reader == nil {
		reader = os.Stdin
	}
	return io.NopCloser(reader), nil
}

// Size implements Input.
func (s StdinInput) Size() (int64, bool) { return 0, false }

// TemplateInput selects the input for a template argument: stdin or a file.
func TemplateInput(arg string) Input {
	if utils.IsStdio(arg) {
		return StdinInput{}
	}
	return FileInput{Path: arg}
}
