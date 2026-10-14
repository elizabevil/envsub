package diag_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/elizabevil/envsub/internal/diag"
)

func TestWrapIfNotPreservesExistingKind(t *testing.T) {
	t.Parallel()
	base := diag.New(diag.KindTemplate, "bad template")
	if got, want := diag.ExitCode(diag.WrapIfNot(diag.KindInternal, base)), 3; got != want {
		t.Fatalf("WrapIfNot() exit code = %d, want %d", got, want)
	}
	if got, want := diag.ExitCode(diag.WrapIfNot(diag.KindIO, errors.New("boom"))), 5; got != want {
		t.Fatalf("WrapIfNot() exit code = %d, want %d", got, want)
	}
}

func TestExitCode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "nil", err: nil, want: 0},
		{name: "plain", err: errors.New("boom"), want: 1},
		{name: "internal", err: diag.New(diag.KindInternal, "boom"), want: 1},
		{name: "usage", err: diag.New(diag.KindUsage, "bad flag"), want: 2},
		{name: "template", err: diag.New(diag.KindTemplate, "bad template"), want: 3},
		{name: "type", err: diag.New(diag.KindType, "bad type"), want: 4},
		{name: "schema", err: diag.New(diag.KindSchema, "bad schema"), want: 4},
		{name: "io", err: diag.New(diag.KindIO, "bad io"), want: 5},
		{name: "missing", err: diag.New(diag.KindMissing, "missing"), want: 6},
		{name: "policy", err: diag.New(diag.KindPolicy, "policy"), want: 6},
		{name: "wrapped", err: fmt.Errorf("outer: %w", diag.New(diag.KindIO, "inner")), want: 5},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := diag.ExitCode(tt.err); got != tt.want {
				t.Fatalf("ExitCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

// TestWithOffset covers the structured byte offset: constructors start at -1,
// WithOffset records a value (including 0) and it survives At.
func TestWithOffset(t *testing.T) {
	t.Parallel()
	if got := diag.New(diag.KindTemplate, "x").(*diag.Error).Offset; got != -1 {
		t.Fatalf("New().Offset = %d, want -1", got)
	}
	if got := diag.Wrap(diag.KindIO, errors.New("x")).(*diag.Error).Offset; got != -1 {
		t.Fatalf("Wrap().Offset = %d, want -1", got)
	}

	located := diag.At(diag.WithOffset(diag.New(diag.KindTemplate, "offset 0: bad"), 0), "in.tmpl", 1)
	var de *diag.Error
	if !errors.As(located, &de) {
		t.Fatalf("errors.As(%v, *Error) = false", located)
	}
	if want := (diag.Location{File: "in.tmpl", Line: 1, Offset: 0}); de.Location != want {
		t.Fatalf("Location = %+v, want %+v", de.Location, want)
	}

	plain := errors.New("plain")
	if got := diag.WithOffset(plain, 3); got != plain {
		t.Fatalf("WithOffset(non-project error) = %v, want the original error", got)
	}
}
