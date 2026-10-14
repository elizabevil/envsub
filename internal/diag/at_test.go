package diag_test

import (
	"errors"
	"testing"

	"github.com/elizabevil/envsub/internal/diag"
)

func TestAtAttachesLocation(t *testing.T) {
	t.Parallel()
	base := diag.New(diag.KindTemplate, "unclosed variable expression")

	located := diag.At(base, "config.tmpl", 2)
	if got, want := located.Error(), "config.tmpl:2: unclosed variable expression"; got != want {
		t.Fatalf("At() = %q, want %q", got, want)
	}
	if got := diag.KindOf(located); got != diag.KindTemplate {
		t.Fatalf("KindOf() = %q, want template", got)
	}
	if got := diag.ExitCode(located); got != 3 {
		t.Fatalf("ExitCode() = %d, want 3", got)
	}

	// The original error must stay untouched.
	if got, want := base.Error(), "unclosed variable expression"; got != want {
		t.Fatalf("base error changed to %q, want %q", got, want)
	}

	fileOnly := diag.At(base, "config.tmpl", 0)
	if got, want := fileOnly.Error(), "config.tmpl: unclosed variable expression"; got != want {
		t.Fatalf("At() without line = %q, want %q", got, want)
	}
}

func TestAtKeepsWrappedCause(t *testing.T) {
	t.Parallel()
	cause := errors.New("boom")
	base := diag.Wrap(diag.KindIO, cause)

	located := diag.At(base, "values.yaml", 7)
	if !errors.Is(located, cause) {
		t.Fatalf("At() lost the wrapped cause")
	}
	if got, want := located.Error(), "values.yaml:7: boom"; got != want {
		t.Fatalf("At() = %q, want %q", got, want)
	}
}

func TestAtWrapsUnknownErrors(t *testing.T) {
	t.Parallel()
	located := diag.At(errors.New("boom"), "config.tmpl", 1)
	if got := diag.KindOf(located); got != diag.KindInternal {
		t.Fatalf("KindOf() = %q, want internal", got)
	}
	if got, want := located.Error(), "config.tmpl:1: boom"; got != want {
		t.Fatalf("At() = %q, want %q", got, want)
	}
	if diag.At(nil, "x", 1) != nil {
		t.Fatalf("At(nil) = non-nil")
	}
}
