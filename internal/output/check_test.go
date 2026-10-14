package output_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/elizabevil/envsub/internal/output"
)

// TestCheckWriterReportsDifferences pins the three outcomes of a comparison:
// an identical file is clean, a differing file is outdated, and a missing file
// is outdated because the render would create it.
func TestCheckWriterReportsDifferences(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	same := filepath.Join(dir, "same.txt")
	diff := filepath.Join(dir, "diff.txt")
	missing := filepath.Join(dir, "missing.txt")
	if err := os.WriteFile(same, []byte("value"), 0o644); err != nil {
		t.Fatalf("WriteFile(same): %v", err)
	}
	if err := os.WriteFile(diff, []byte("stale"), 0o644); err != nil {
		t.Fatalf("WriteFile(diff): %v", err)
	}

	writer := output.NewCheckWriter()
	for _, target := range []string{same, diff, missing} {
		if err := writer.Write(context.Background(), target, []byte("value")); err != nil {
			t.Fatalf("Write(%s) error = %v", target, err)
		}
	}

	if got, want := writer.Checked(), 3; got != want {
		t.Errorf("Checked() = %d, want %d", got, want)
	}
	outdated := writer.Outdated()
	if len(outdated) != 2 || outdated[0] != diff || outdated[1] != missing {
		t.Errorf("Outdated() = %v, want [%s %s]", outdated, diff, missing)
	}
}

// TestCheckWriterNeverWrites pins the contract --check depends on: a comparison
// leaves the filesystem untouched, including the compared file itself.
func TestCheckWriterNeverWrites(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "target.txt")
	const original = "stale"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	writer := output.NewCheckWriter()
	if err := writer.Write(context.Background(), path, []byte("rendered")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != original {
		t.Errorf("file = %q, want the untouched %q", got, original)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}
}
