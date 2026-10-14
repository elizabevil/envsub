package output_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/elizabevil/envsub/internal/output"
)

func TestStdoutWriter(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	w := output.NewStdoutWriter(&buf)
	if err := w.Write(context.Background(), "", []byte("hello")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if got, want := buf.String(), "hello"; got != want {
		t.Fatalf("Write() = %q, want %q", got, want)
	}
}

func TestFileWriter(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "out.txt")
	w := output.NewFileWriter(path)
	if err := w.Write(context.Background(), "", []byte("hello")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("file = %q, want %q", string(got), "hello")
	}
}
