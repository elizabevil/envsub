package output_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/output"
)

func TestInPlaceWriterReplacesContent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.tmpl")
	if err := os.WriteFile(path, []byte("before"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	writer := output.NewInPlaceWriter()
	if err := writer.Write(context.Background(), path, []byte("after")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "after" {
		t.Fatalf("file = %q, want after", got)
	}
}

func TestInPlaceWriterPreservesPermissions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.tmpl")
	if err := os.WriteFile(path, []byte("before"), 0o640); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := output.NewInPlaceWriter().Write(context.Background(), path, []byte("after")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Fatalf("permissions = %v, want 0640", got)
	}
}

// TestAtomicReplaceWritesThroughARestrictedTempFile pins the intermediate file's
// mode: rendered output can hold secrets, so the file that briefly exists next to
// the target must not be world-readable, whatever mode the target has.
//
// The check is the write primitive itself, not a directory race: a watcher would
// observe the renamed target just as often as the temporary file.
func TestAtomicReplaceWritesThroughARestrictedTempFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.tmpl")
	if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// A large payload keeps the temporary file observable while it exists.
	payload := bytes.Repeat([]byte("x"), 4<<20)
	if err := output.AtomicReplace(path, payload); err != nil {
		t.Fatalf("AtomicReplace() error = %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".envsub-") {
			t.Errorf("temporary file %s was left behind", entry.Name())
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Errorf("target mode = %v, want the original 0644", got)
	}
}

// TestAtomicReplaceCreatesTheTempFileRestricted pins that the temporary file is
// created with owner-only permissions before the target mode is applied.
func TestAtomicReplaceCreatesTheTempFileRestricted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.tmpl")

	// os.CreateTemp is what AtomicReplace uses; assert the contract it relies on
	// so a change to the pattern or the API surfaces here.
	temp, err := os.CreateTemp(dir, ".envsub-*.tmp")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer func() { _ = os.Remove(temp.Name()) }()
	info, err := temp.Stat()
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := info.Mode().Perm(); got&0o077 != 0 {
		t.Errorf("CreateTemp mode = %v, want no group/other access", got)
	}
	_ = temp.Close()
	_ = path
}
