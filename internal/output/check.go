package output

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"slices"
)

// CheckWriter compares rendered output with the existing target instead of
// writing it. It is the destination behind --check: one instance collects every
// target it looked at, so a batch run reports all outdated outputs at once.
type CheckWriter struct {
	// outdated holds the targets whose content differs, or which do not exist.
	outdated []string
	// checked counts the targets that were compared.
	checked int
}

// NewCheckWriter returns a writer that only compares.
func NewCheckWriter() *CheckWriter { return &CheckWriter{} }

// Write compares data with the file at target. A missing file counts as
// outdated: the render would create it. Nothing is written, and a difference is
// never an error here: the caller decides what to report.
func (w *CheckWriter) Write(_ context.Context, target string, data []byte) error {
	w.checked++
	existing, err := os.ReadFile(target)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		w.outdated = append(w.outdated, target)
		return nil
	case err != nil:
		// An unreadable target cannot be compared; that is an IO failure, not a
		// difference.
		return err
	}
	if !bytes.Equal(existing, data) {
		w.outdated = append(w.outdated, target)
	}
	return nil
}

// Outdated returns the targets that need to be regenerated, sorted.
func (w *CheckWriter) Outdated() []string {
	slices.Sort(w.outdated)
	return w.outdated
}

// Checked returns how many targets were compared.
func (w *CheckWriter) Checked() int { return w.checked }
