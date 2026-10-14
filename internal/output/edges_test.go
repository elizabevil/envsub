package output_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/output"
)

// TestFileWriterRejectsAnEmptyPath pins the misconfiguration error: with neither a
// fixed path nor a target, the writer must fail instead of writing nowhere.
func TestFileWriterRejectsAnEmptyPath(t *testing.T) {
	t.Parallel()
	err := output.NewFileWriter("").Write(context.Background(), "", []byte("x"))
	if got := diag.KindOf(err); got != diag.KindIO {
		t.Fatalf("Write() error = %v (kind %q), want kind %q", err, got, diag.KindIO)
	}
}

// TestCheckWriterSortsOutdatedTargets pins the report order: the --check message
// lists the outdated targets, so a stable order keeps the output diffable.
func TestCheckWriterSortsOutdatedTargets(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writer := output.NewCheckWriter()
	for _, name := range []string{"z.txt", "a.txt", "m.txt"} {
		if err := writer.Write(context.Background(), filepath.Join(dir, name), []byte("rendered")); err != nil {
			t.Fatalf("Write(%s) error = %v", name, err)
		}
	}
	got := writer.Outdated()
	want := []string{"a.txt", "m.txt", "z.txt"}
	for i := range want {
		if filepath.Base(got[i]) != want[i] {
			t.Fatalf("Outdated() = %v, want %v", got, want)
		}
	}
}

// TestCheckWriterReportsUnreadableTargetAsIOError pins the distinction --check
// makes: a target that differs is a finding (exit 7), a target that cannot be
// read at all is an IO failure.
func TestCheckWriterReportsUnreadableTargetAsIOError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// A directory at the target path cannot be read as a file.
	target := filepath.Join(dir, "adir")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	writer := output.NewCheckWriter()
	err := writer.Write(context.Background(), target, []byte("rendered"))
	if err == nil {
		t.Fatal("Write(directory) error = nil, want an IO failure")
	}
	if outdated := writer.Outdated(); len(outdated) != 0 {
		t.Errorf("Outdated() = %v, want none (an unreadable target is not a difference)", outdated)
	}
	if checked := writer.Checked(); checked != 1 {
		t.Errorf("Checked() = %d, want the target to count as inspected", checked)
	}
}
