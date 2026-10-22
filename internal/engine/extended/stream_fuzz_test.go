package extended

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

// FuzzStreamWindow pins that the window reader always terminates.
//
// It cuts its input at line boundaries, so a line longer than a window has to
// be carried across windows. The reader used to stop growing its buffer once
// it held a full window, which meant a line longer than that never found its
// ending: the reader spun on the same bytes forever.
//
// The fuzzed arguments stay small and the input is built from them, because a
// large seed makes the fuzzing engine spend its time minimizing the seed
// instead of looking for new cases. The deadline turns a spin into a failure
// instead of a hung fuzzer.
//
// Run it with, for example:
//
//	go test -fuzz=FuzzStreamWindow -fuzztime=30s ./internal/engine/extended/
func FuzzStreamWindow(f *testing.F) {
	for _, seed := range []struct {
		data   string
		repeat int
	}{
		{"", 1},
		{"a", 1},
		{"a\n", 1},
		{"\n", 1},
		{"\n\n", 1},
		{"a\nb\n", 1},
		{"a\r\nb\r\n", 1},
		// A line that is longer than the window below.
		{"x", 200000},
		{"x\n", 200000},
	} {
		f.Add([]byte(seed.data), seed.repeat)
	}
	f.Fuzz(func(t *testing.T, data []byte, repeat int) {
		if repeat < 1 {
			repeat = 1
		}
		if repeat > 1<<20 {
			repeat = 1 << 20
		}
		input := bytes.Repeat(data, repeat)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		line := 1
		err := streamWindow(ctx, bytes.NewReader(input), 64<<10, func(_ []byte, _ int) error {
			return nil
		}, &line)
		if err != nil && errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("streamWindow() did not finish on %d bytes: it spins instead of returning", len(input))
		}
	})
}
