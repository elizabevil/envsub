package utils_test

import (
	"bytes"
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

// TestTrimBOM pins the single BOM rule: only a leading UTF-8 mark is removed, and
// the same bytes anywhere else are content.
func TestTrimBOM(t *testing.T) {
	t.Parallel()
	bom := []byte{0xEF, 0xBB, 0xBF}
	for _, tc := range []struct {
		name string
		in   []byte
		want []byte
	}{
		{name: "no mark", in: []byte("a: 1\n"), want: []byte("a: 1\n")},
		{name: "leading mark", in: append(append([]byte{}, bom...), []byte("a: 1\n")...), want: []byte("a: 1\n")},
		{name: "mark only", in: append([]byte{}, bom...), want: []byte{}},
		{name: "empty", in: nil, want: []byte{}},
		{name: "mark inside stays", in: []byte("a\xef\xbb\xbfb"), want: []byte("a\xef\xbb\xbfb")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := utils.TrimBOM(tc.in); !bytes.Equal(got, tc.want) {
				t.Fatalf("TrimBOM(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
