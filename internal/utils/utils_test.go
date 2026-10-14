package utils_test

import (
	"testing"

	"github.com/elizabevil/envsub/internal/utils"
)

func TestIsStdio(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		path string
		want bool
	}{
		{path: "", want: true},
		{path: "-", want: true},
		{path: "file.txt", want: false},
	} {

		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			if got := utils.IsStdio(tt.path); got != tt.want {
				t.Fatalf("IsStdio(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestCloneBytes(t *testing.T) {
	t.Parallel()
	src := []byte("hello")
	got := utils.CloneBytes(src)
	if string(got) != "hello" {
		t.Fatalf("CloneBytes() = %q, want %q", string(got), "hello")
	}
	src[0] = 'x'
	if string(got) != "hello" {
		t.Fatalf("CloneBytes() alias detected: %q", string(got))
	}
}
