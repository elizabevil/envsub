// Package output writes rendered templates to stdout, to a file, or to an
// atomic in-place replacement.
package output

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/elizabevil/envsub/internal/diag"
)

// Writer abstracts stdout, file, in-place and batch outputs.
type Writer interface {
	Write(ctx context.Context, target string, data []byte) error
}

// StdoutWriter writes rendered bytes to an io.Writer.
type StdoutWriter struct {
	// out is the destination stream.
	out io.Writer
}

// NewStdoutWriter returns a writer that writes to w.
func NewStdoutWriter(w io.Writer) *StdoutWriter {
	return &StdoutWriter{out: w}
}

// Write writes rendered bytes to the underlying stream.
func (w *StdoutWriter) Write(ctx context.Context, _ string, data []byte) error {
	if w.out == nil {
		return diag.New(diag.KindIO, "stdout writer is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := w.out.Write(data); err != nil {
		return diag.Wrap(diag.KindIO, fmt.Errorf("write stdout: %w", err))
	}
	return nil
}

// FileWriter writes rendered bytes to a file path.
type FileWriter struct {
	// Path is the fixed destination used when the caller passes no target.
	Path string
	// Perm is the file mode new files are created with.
	Perm os.FileMode
}

// NewFileWriter returns a writer for the given output path. An empty path
// means the destination comes from the target argument of Write.
func NewFileWriter(path string) *FileWriter {
	return &FileWriter{Path: path, Perm: defaultFilePerm}
}

// Write writes data to the target path, falling back to the fixed Path.
func (w *FileWriter) Write(ctx context.Context, target string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if target == "" {
		target = w.Path
	}
	if target == "" {
		return diag.New(diag.KindIO, "output path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(target), defaultDirPerm); err != nil {
		return diag.Wrap(diag.KindIO, fmt.Errorf("create output directory: %w", err))
	}
	perm := w.Perm
	if perm == 0 {
		perm = defaultFilePerm
	}
	if err := os.WriteFile(target, data, perm); err != nil {
		return diag.Wrap(diag.KindIO, fmt.Errorf(writeTargetFormat, target, err))
	}
	return nil
}

// InPlaceWriter replaces a template file with its rendered output.
//
// The rendered bytes go to a temporary file in the same directory and are then
// renamed over the target, so a failure never truncates the original. The
// original permission bits are preserved.
type InPlaceWriter struct{}

// NewInPlaceWriter returns a writer that atomically replaces template files.
func NewInPlaceWriter() *InPlaceWriter {
	return &InPlaceWriter{}
}

// Write atomically replaces target with data.
func (w *InPlaceWriter) Write(ctx context.Context, target string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := AtomicReplace(target, data); err != nil {
		return diag.Wrap(diag.KindIO, err)
	}
	return nil
}

// AtomicReplace writes data to path through a temporary file and renames it
// into place, preserving the permission bits of an existing file.
func AtomicReplace(path string, data []byte) error {
	if path == "" {
		return errors.New("in-place target is empty")
	}
	perm := defaultFilePerm
	existing, err := os.Stat(path)
	switch {
	case err == nil:
		if existing.IsDir() {
			return fmt.Errorf("%s is a directory", path)
		}
		perm = existing.Mode().Perm()
	case errors.Is(err, fs.ErrNotExist):
	default:
		return fmt.Errorf("stat %s: %w", path, err)
	}

	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, tempFilePattern)
	if err != nil {
		return fmt.Errorf("create temporary file in %s: %w", dir, err)
	}
	tempName := temp.Name()
	discard := func() {
		if err := temp.Close(); err != nil {
			return
		}
		if err := os.Remove(tempName); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return
		}
	}

	if _, err := temp.Write(data); err != nil {
		discard()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		discard()
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := temp.Chmod(perm); err != nil {
		discard()
		return fmt.Errorf("set permissions on temporary file: %w", err)
	}
	if err := temp.Close(); err != nil {
		if removeErr := os.Remove(tempName); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
			return fmt.Errorf("close temporary file: %w (cleanup: %v)", err, removeErr)
		}
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		if removeErr := os.Remove(tempName); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
			return fmt.Errorf("replace %s: %w (cleanup: %v)", path, err, removeErr)
		}
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

const (
	defaultFilePerm fs.FileMode = 0o644
	defaultDirPerm  fs.FileMode = 0o755
	tempFilePattern             = ".envsub-*.tmp"
	// writeTargetFormat reports a failed write to a named destination.
	writeTargetFormat = "write %s: %w"
)

// StreamWriter is a Writer that can stream rendered output instead of
// accepting a fully buffered result. One instance handles one destination.
type StreamWriter interface {
	Writer
	// Stream returns the writer rendered bytes go to.
	Stream() io.Writer
	// Commit finalizes the destination. It must be called after a successful
	// stream and is a no-op for stdout.
	Commit() error
	// Discard abandons a partially written destination and cleans up.
	Discard() error
}

// WriterStream adapts a Writer that only accepts whole buffers into the
// streaming interface by buffering into memory.
type WriterStream struct {
	// writer is the buffered destination.
	writer Writer
	// path is the destination path handed to writer on Commit.
	path string
	// buf accumulates the streamed output until Commit.
	buf bytes.Buffer
}

// NewWriterStream wraps a buffered Writer as a StreamWriter.
func NewWriterStream(writer Writer, target string) *WriterStream {
	return &WriterStream{writer: writer, path: target}
}

// Stream returns the in-memory buffer.
func (w *WriterStream) Stream() io.Writer { return &w.buf }

// Write stores a complete buffer, replacing anything streamed before it.
func (w *WriterStream) Write(ctx context.Context, target string, data []byte) error {
	return w.writer.Write(ctx, target, data)
}

// Commit flushes the buffer to the wrapped writer.
func (w *WriterStream) Commit() error {
	if w.writer == nil {
		return diag.New(diag.KindIO, "stream writer is nil")
	}
	return w.writer.Write(context.Background(), w.path, w.buf.Bytes())
}

// Discard drops the buffer.
func (w *WriterStream) Discard() error {
	w.buf.Reset()
	return nil
}

// DirectStream writes rendered bytes straight to an io.Writer. Commit and
// Discard are no-ops because nothing is buffered.
type DirectStream struct {
	// out is the destination stream.
	out io.Writer
}

// NewDirectStream returns a StreamWriter that writes straight to w.
func NewDirectStream(w io.Writer) *DirectStream {
	return &DirectStream{out: w}
}

// Stream returns the underlying writer.
func (s *DirectStream) Stream() io.Writer { return s.out }

// Write writes a complete buffer to the underlying writer.
func (s *DirectStream) Write(ctx context.Context, _ string, data []byte) error {
	if s.out == nil {
		return diag.New(diag.KindIO, "stream writer is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := s.out.Write(data); err != nil {
		return diag.Wrap(diag.KindIO, fmt.Errorf("write stdout: %w", err))
	}
	return nil
}

// Commit is a no-op for an unbuffered stream.
func (s *DirectStream) Commit() error { return nil }

// Discard is a no-op for an unbuffered stream.
func (s *DirectStream) Discard() error { return nil }

// AtomicStream streams rendered bytes into a temporary file and renames it over
// the target on Commit, so a failure never truncates the original. The
// permission bits of an existing target are preserved.
type AtomicStream struct {
	// target is the final destination the temporary file is renamed onto.
	target string
	// temp is the open temporary file the render writes into.
	temp *os.File
	// tempPath is the path of temp, kept for cleanup after temp is closed.
	tempPath string
	// perm is the permission mode restored on the temporary file before the
	// rename, taken from an existing target when there is one.
	perm fs.FileMode
}

// NewAtomicStream prepares a temporary file next to target.
func NewAtomicStream(target string) (*AtomicStream, error) {
	if target == "" {
		return nil, diag.New(diag.KindIO, "in-place target is empty")
	}
	perm := defaultFilePerm
	existing, err := os.Stat(target)
	switch {
	case err == nil:
		if existing.IsDir() {
			return nil, diag.New(diag.KindIO, "%s is a directory", target)
		}
		perm = existing.Mode().Perm()
	case errors.Is(err, fs.ErrNotExist):
	default:
		return nil, diag.Wrap(diag.KindIO, fmt.Errorf("stat %s: %w", target, err))
	}

	temp, err := os.CreateTemp(filepath.Dir(target), tempFilePattern)
	if err != nil {
		return nil, diag.Wrap(diag.KindIO, fmt.Errorf("create temporary file: %w", err))
	}
	return &AtomicStream{target: target, temp: temp, tempPath: temp.Name(), perm: perm}, nil
}

// Stream returns the temporary file.
func (a *AtomicStream) Stream() io.Writer { return a.temp }

// Write writes a complete buffer to the temporary file.
func (a *AtomicStream) Write(_ context.Context, _ string, data []byte) error {
	if _, err := a.temp.Write(data); err != nil {
		return diag.Wrap(diag.KindIO, fmt.Errorf("write temporary file: %w", err))
	}
	return nil
}

// Commit syncs, sets permissions and renames the temporary file into place.
func (a *AtomicStream) Commit() error {
	if a.temp == nil {
		return diag.New(diag.KindIO, "atomic stream is closed")
	}
	if err := a.temp.Sync(); err != nil {
		_ = a.Discard()
		return diag.Wrap(diag.KindIO, fmt.Errorf("sync temporary file: %w", err))
	}
	if err := a.temp.Chmod(a.perm); err != nil {
		_ = a.Discard()
		return diag.Wrap(diag.KindIO, fmt.Errorf("set permissions on temporary file: %w", err))
	}
	if err := a.temp.Close(); err != nil {
		_ = os.Remove(a.tempPath)
		return diag.Wrap(diag.KindIO, fmt.Errorf("close temporary file: %w", err))
	}
	a.temp = nil
	if err := os.Rename(a.tempPath, a.target); err != nil {
		_ = os.Remove(a.tempPath)
		return diag.Wrap(diag.KindIO, fmt.Errorf("replace %s: %w", a.target, err))
	}
	return nil
}

// Discard removes the temporary file and leaves the target untouched.
func (a *AtomicStream) Discard() error {
	if a.temp == nil {
		return nil
	}
	err := a.temp.Close()
	a.temp = nil
	if removeErr := os.Remove(a.tempPath); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
		return diag.Wrap(diag.KindIO, removeErr)
	}
	if err != nil {
		return diag.Wrap(diag.KindIO, err)
	}
	return nil
}

// FileStream streams rendered bytes into a file, creating parent directories
// on Commit. The destination is replaced only once the stream succeeds.
type FileStream struct {
	// target is the final destination the temporary file is renamed onto.
	target string
	// temp is the open temporary file the render writes into.
	temp *os.File
	// tempPath is the path of temp, kept for cleanup after temp is closed.
	tempPath string
}

// NewStreamToFile prepares a temporary file next to target and returns a
// StreamWriter that renames it over target on Commit.
func NewStreamToFile(target string) (*FileStream, error) {
	if target == "" {
		return nil, diag.New(diag.KindIO, "output path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(target), defaultDirPerm); err != nil {
		return nil, diag.Wrap(diag.KindIO, fmt.Errorf("create output directory: %w", err))
	}
	temp, err := os.CreateTemp(filepath.Dir(target), tempFilePattern)
	if err != nil {
		return nil, diag.Wrap(diag.KindIO, fmt.Errorf("create temporary file: %w", err))
	}
	return &FileStream{target: target, temp: temp, tempPath: temp.Name()}, nil
}

// Stream returns the temporary file.
func (f *FileStream) Stream() io.Writer { return f.temp }

// Write writes a complete buffer into the temporary file.
func (f *FileStream) Write(_ context.Context, _ string, data []byte) error {
	if _, err := f.temp.Write(data); err != nil {
		return diag.Wrap(diag.KindIO, fmt.Errorf(writeTargetFormat, f.target, err))
	}
	return nil
}

// Commit renames the temporary file over the target.
func (f *FileStream) Commit() error {
	if f.temp == nil {
		return diag.New(diag.KindIO, "file stream is closed")
	}
	if err := f.temp.Close(); err != nil {
		_ = os.Remove(f.tempPath)
		f.temp = nil
		return diag.Wrap(diag.KindIO, fmt.Errorf("close %s: %w", f.target, err))
	}
	f.temp = nil
	if err := os.Rename(f.tempPath, f.target); err != nil {
		_ = os.Remove(f.tempPath)
		return diag.Wrap(diag.KindIO, fmt.Errorf(writeTargetFormat, f.target, err))
	}
	return nil
}

// Discard removes the temporary file and leaves any existing target untouched.
func (f *FileStream) Discard() error {
	if f.temp == nil {
		return nil
	}
	err := f.temp.Close()
	f.temp = nil
	if removeErr := os.Remove(f.tempPath); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
		return diag.Wrap(diag.KindIO, removeErr)
	}
	if err != nil {
		return diag.Wrap(diag.KindIO, err)
	}
	return nil
}
