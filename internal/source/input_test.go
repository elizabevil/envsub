package source_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/source"
)

func TestTemplateInputSelectsImplementation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		arg  string
		name string
		want any
	}{
		{arg: "", name: "-", want: source.StdinInput{}},
		{arg: "-", name: "-", want: source.StdinInput{}},
		{arg: "config.tmpl", name: "config.tmpl", want: source.FileInput{Path: "config.tmpl"}},
		{arg: "dir/config.tmpl", name: "dir/config.tmpl", want: source.FileInput{Path: "dir/config.tmpl"}},
		// Remote transports are not wired, so every non-stdio argument is a file path,
		// scheme-looking or not.
		{arg: "http://example.com/config.tmpl", name: "http://example.com/config.tmpl", want: source.FileInput{Path: "http://example.com/config.tmpl"}},
		{arg: "rpc://config-service/get", name: "rpc://config-service/get", want: source.FileInput{Path: "rpc://config-service/get"}},
	}
	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			t.Parallel()
			got := source.TemplateInput(tt.arg)
			if got != tt.want {
				t.Fatalf("TemplateInput(%q) = %#v, want %#v", tt.arg, got, tt.want)
			}
			if got.Name() != tt.name {
				t.Fatalf("Name() = %q, want %q", got.Name(), tt.name)
			}
		})
	}
}

func TestFileInputReadsFreshStreams(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "input.tmpl")
	content := []byte("host=${HOST}\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	input := source.NewFileInput(path)
	size, known := input.Size()
	if !known || size != int64(len(content)) {
		t.Fatalf("Size() = (%d, %v), want (%d, true)", size, known, len(content))
	}
	if input.Name() != path {
		t.Fatalf("Name() = %q, want %q", input.Name(), path)
	}

	// Every Open must return a stream positioned at the start, so the renderer
	// can measure a file and then read it again.
	for attempt := range 2 {
		stream, err := input.Open()
		if err != nil {
			t.Fatalf("Open() #%d error = %v", attempt, err)
		}
		data, err := io.ReadAll(stream)
		if err != nil {
			t.Fatalf("ReadAll() #%d error = %v", attempt, err)
		}
		if err := stream.Close(); err != nil {
			t.Fatalf("Close() #%d error = %v", attempt, err)
		}
		if string(data) != string(content) {
			t.Fatalf("Open() #%d read %q, want %q", attempt, data, content)
		}
	}
}

func TestFileInputErrors(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "missing.tmpl")
	input := source.NewFileInput(path)
	if _, known := input.Size(); known {
		t.Fatal("Size() reported a known size for a missing file")
	}
	if _, err := input.Open(); err == nil {
		t.Fatal("Open() error = nil, want an error for a missing file")
	}

	dir := t.TempDir()
	if _, known := source.NewFileInput(dir).Size(); known {
		t.Fatal("Size() reported a known size for a directory")
	}
}

func TestStdinInputIsUnknownAndClosingIsSafe(t *testing.T) {
	t.Parallel()
	input := source.StdinInput{Reader: strings.NewReader("body")}
	if input.Name() != "-" {
		t.Fatalf("Name() = %q, want %q", input.Name(), "-")
	}
	if _, known := input.Size(); known {
		t.Fatal("Size() reported a known size for stdin")
	}

	stream, err := input.Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	data, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(data) != "body" {
		t.Fatalf("ReadAll() = %q, want %q", data, "body")
	}
	// Closing must not close the process stdin; with a test reader it must not
	// close the reader either.
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := stream.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("Read() after Close() error = %v, want io.EOF", err)
	}
}
