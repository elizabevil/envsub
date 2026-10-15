package extended

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/engine"
)

const (
	// streamWindowSize is how much template text is held at once. A larger
	// window trades memory for fewer copies.
	streamWindowSize = 1 << 20
	// streamReadSize is the read granularity.
	streamReadSize = 256 << 10
	// streamMaxCarry bounds how much unterminated text is buffered. Because
	// reads are cut at line boundaries, only one line can be pending, so this
	// bound is only reached by pathological single-line templates.
	streamMaxCarry = 4 << 20
)

// StreamTo implements engine.StreamReaderEngine. It renders src into w window
// by window, so a template of any size is never resident in memory in full.
func (e *Engine) StreamTo(ctx context.Context, w io.Writer, src io.Reader, in engine.Input) error {
	in = in.WithTree()
	parallel, err := e.shouldRenderParallel(ctx, src, in)
	if err != nil {
		return err
	}
	if parallel {
		return e.streamParallel(ctx, w, src, in)
	}
	return e.streamSequential(ctx, w, src, in)
}

// shouldRenderParallel reports whether the template is large enough to be worth
// splitting and safe to render out of order.
//
// Deciding requires reading the input, which consumes it, so only a source that
// reports a known length can take the parallel path: the caller can rewind such
// a source after the measurement. Stdin, HTTP bodies and RPC streams have no
// known length and stay sequential.
func (e *Engine) shouldRenderParallel(ctx context.Context, src io.Reader, in engine.Input) (bool, error) {
	tree := in.Values
	if tree.Mutated() {
		return false, nil
	}
	// --only-vars, --ignore-vars and --on-missing=keep send bare references down the general
	// path, which compiles paths. Those compilations would then happen inside
	// workers, so such renders stay sequential.
	if len(in.Options.Only) > 0 || len(in.Options.Ignore) > 0 || in.Options.Unset == engine.UnsetKeep {
		return false, nil
	}
	if !in.SourceSizeKnown {
		return false, nil
	}
	// A source that fits in one window yields exactly one chunk, which is below
	// the minimum; skipping the measurement also avoids reading small
	// templates twice.
	if in.SourceSize <= parallelChunkSize {
		return false, nil
	}
	seeker, ok := src.(io.Seeker)
	if !ok {
		return false, nil
	}
	start, err := seeker.Seek(0, io.SeekCurrent)
	if err != nil {
		return false, nil
	}
	chunks, measureErr := e.measureWindows(ctx, src, parallelChunkSize, tree)
	if _, err := seeker.Seek(start, io.SeekStart); err != nil {
		return false, err
	}
	if measureErr != nil {
		// An assignment, a malformed template or a read failure: the sequential
		// path re-reads from the start and reports the problem with the
		// template name and position.
		return false, nil
	}
	if chunks < parallelMinChunks {
		return false, nil
	}
	return true, nil
}

// streamSequential renders window by window on the calling goroutine.
func (e *Engine) streamSequential(ctx context.Context, w io.Writer, src io.Reader, in engine.Input) error {
	buffered := newStreamBuffer(w)
	line := 1
	err := streamWindow(ctx, src, streamWindowSize, func(window []byte, baseLine int) error {
		scanErr := ScanSpans(window, func(span Span) error {
			return e.emitSpan(ctx, buffered, window, span, in, true)
		})
		if scanErr != nil {
			return locateWindowError(scanErr, window, in.Name, baseLine)
		}
		return nil
	}, &line)
	if err != nil {
		if errors.Is(err, errKeepPlaceholder) {
			return nil
		}
		return err
	}
	if err := buffered.Flush(); err != nil {
		return diag.Wrap(diag.KindIO, err)
	}
	return nil
}

// streamWindow reads src in bounded windows and calls handle for each window.
// Windows are cut at line boundaries so a variable reference is never split,
// and startLine tracks the 1-based line number of each window's first byte.
//
// The buffer is shifted in place rather than reallocated, so a multi-gigabyte
// template never occupies more than one window plus one partial line.
func streamWindow(ctx context.Context, src io.Reader, windowSize int, handle func(window []byte, startLine int) error, line *int) error {
	buffer := make([]byte, 0, windowSize+streamReadSize)
	// One reusable read buffer: a chunk is copied into buffer immediately, so it
	// never needs to survive the next Read.
	chunk := make([]byte, streamReadSize)
	eof := false

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Fill the buffer until it holds a full window or the source ends.
		for !eof && len(buffer) < windowSize {
			n, readErr := src.Read(chunk)
			if n > 0 {
				buffer = append(buffer, chunk[:n]...)
			}
			switch {
			case readErr == nil:
			case errors.Is(readErr, io.EOF):
				eof = true
			default:
				return diag.Wrapf(diag.KindIO, "read template: %w", readErr)
			}
		}

		// Decide how much of the buffer is complete. While more input may
		// arrive, keep the trailing partial line so a variable is never split.
		// At EOF the whole buffer is processable.
		processable := len(buffer)
		if !eof {
			cut := bytes.LastIndexByte(buffer, '\n')
			if cut < 0 {
				if len(buffer) >= streamMaxCarry {
					return diag.New(diag.KindTemplate,
						"template contains a single line longer than %d bytes", streamMaxCarry)
				}
				continue
			}
			processable = cut + 1
		}

		if processable == 0 {
			return nil
		}
		chunk := buffer[:processable]
		if err := handle(chunk, *line); err != nil {
			return err
		}
		*line += bytes.Count(chunk, newlineBytes)
		buffer = append(buffer[:0], buffer[processable:]...)

		if eof && len(buffer) == 0 {
			return nil
		}
	}
}

// newlineBytes is used to count lines without allocating.
var newlineBytes = []byte{'\n'}

// locateWindowError converts a scanner offset inside one window into a template
// line number.
func locateWindowError(err error, window []byte, name string, baseLine int) error {
	if name == "" {
		return err
	}
	var de *diag.Error
	if !errors.As(err, &de) || de.Kind != diag.KindTemplate || de.File != "" {
		return err
	}
	if de.Offset < 0 {
		return diag.At(err, name, baseLine)
	}
	line, _ := Position(window, de.Offset)
	return diag.At(err, name, baseLine+line-1)
}
