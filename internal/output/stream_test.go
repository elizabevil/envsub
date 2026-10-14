package output_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/elizabevil/envsub/internal/output"
)

// streamAndWrite streams payload through w and commits it.
func streamAndWrite(t *testing.T, w output.StreamWriter, payload string) {
	t.Helper()
	if _, err := io.WriteString(w.Stream(), payload); err != nil {
		t.Fatalf("stream write: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
}

// TestAtomicStreamCommitReplacesFile checks the in-place path replaces content,
// preserves permission bits and leaves no temporary file behind.
func TestAtomicStreamCommitReplacesFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.tmpl")
	if err := os.WriteFile(path, []byte("before"), 0o640); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	stream, err := output.NewAtomicStream(path)
	if err != nil {
		t.Fatalf("NewAtomicStream() error = %v", err)
	}
	streamAndWrite(t, stream, "after")

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "after" {
		t.Fatalf("file = %q, want after", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o640 {
		t.Fatalf("permissions = %v, want 0640", perm)
	}
	assertOnlyFile(t, dir, "config.tmpl")
}

// TestAtomicStreamDiscardKeepsOriginal checks a failed render leaves the
// original file untouched and removes the temporary file.
func TestAtomicStreamDiscardKeepsOriginal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.tmpl")
	if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	stream, err := output.NewAtomicStream(path)
	if err != nil {
		t.Fatalf("NewAtomicStream() error = %v", err)
	}
	if _, err := io.WriteString(stream.Stream(), "partial"); err != nil {
		t.Fatalf("stream write: %v", err)
	}
	if err := stream.Discard(); err != nil {
		t.Fatalf("Discard() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "before" {
		t.Fatalf("file = %q, want it unchanged", got)
	}
	assertOnlyFile(t, dir, "config.tmpl")
}

// TestFileStreamCreatesTargetOnlyOnCommit checks batch output does not appear
// until the stream succeeds, and that missing directories are created.
func TestFileStreamCreatesTargetOnlyOnCommit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "nested", "out.tmpl")

	stream, err := output.NewStreamToFile(target)
	if err != nil {
		t.Fatalf("NewStreamToFile() error = %v", err)
	}
	if _, err := io.WriteString(stream.Stream(), "rendered"); err != nil {
		t.Fatalf("stream write: %v", err)
	}
	if _, err := os.Stat(target); err == nil {
		t.Fatal("target exists before Commit")
	}
	if err := stream.Commit(); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "rendered" {
		t.Fatalf("file = %q, want rendered", got)
	}
}

// TestDirectStreamWritesDirectly checks the unbuffered stdout stream.
func TestDirectStreamWritesDirectly(t *testing.T) {
	t.Parallel()
	var buf writerBuffer
	stream := output.NewDirectStream(&buf)
	if err := stream.Write(context.Background(), "", []byte("hello")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if buf.String() != "hello" {
		t.Fatalf("buffer = %q, want hello", buf.String())
	}
	if err := stream.Commit(); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
}

// writerBuffer is a minimal io.Writer for the stream test.
type writerBuffer struct {
	data []byte
}

func (b *writerBuffer) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *writerBuffer) String() string { return string(b.data) }

// assertOnlyFile fails when dir contains anything but the named file.
func assertOnlyFile(t *testing.T, dir, name string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != name {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("directory contains %v, want only %s", names, name)
	}
}
