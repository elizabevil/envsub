package extended_test

import (
	"testing"

	"github.com/elizabevil/envsub/internal/engine/extended"
)

func TestPosition(t *testing.T) {
	t.Parallel()
	src := []byte("one\ntwo\nthree")
	tests := []struct {
		name   string
		offset int
		line   int
		column int
	}{
		{name: "start", offset: 0, line: 1, column: 1},
		{name: "end of first line", offset: 3, line: 1, column: 4},
		{name: "second line", offset: 4, line: 2, column: 1},
		{name: "third line", offset: 8, line: 3, column: 1},
		{name: "end of source", offset: len(src), line: 3, column: 6},
		{name: "past end clamps", offset: len(src) + 100, line: 3, column: 6},
		{name: "negative clamps", offset: -1, line: 1, column: 1},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			line, column := extended.Position(src, tt.offset)
			if line != tt.line || column != tt.column {
				t.Fatalf("Position(%d) = %d:%d, want %d:%d", tt.offset, line, column, tt.line, tt.column)
			}
		})
	}
}
