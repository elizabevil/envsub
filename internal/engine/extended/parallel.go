package extended

import (
	"bytes"
	"context"
	"errors"
	"io"
	"runtime"
	"sync"

	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/values"
)

const (
	// parallelChunkSize is the template span each worker renders independently.
	// Windows are cut at line boundaries, so chunking never splits a variable.
	parallelChunkSize = 256 << 10
	// parallelMinChunks avoids goroutine overhead on small templates.
	parallelMinChunks = 4
	// parallelResultBuffer bounds unfinished per-chunk output. It is larger
	// than the chunk itself because rendered text can expand a variable.
	parallelResultBuffer = parallelChunkSize * 2
)

// errSequentialOnly stops the measurement pass as soon as the template proves
// it cannot be split.
var errSequentialOnly = errors.New("template requires sequential rendering")

// chunkJob is one unit of parallel work.
type chunkJob struct {
	// index is the position of the chunk in the template, used to write
	// results back in order.
	index int
	// source is the chunk text, owned by the job once it is queued.
	source []byte
}

// chunkResult is the rendered output of one chunk.
type chunkResult struct {
	// index is the index of the job this result belongs to.
	index int
	// output is the rendered chunk.
	output []byte
	// err is the first rendering failure of the chunk, if any.
	err error
}

// measureWindows consumes src in one sequential pass, compiling every path the
// workers will resolve, and reports how many renderable windows it holds. It
// fails with errSequentialOnly as soon as a window assigns to the value tree,
// because such a template depends on document order and cannot be rendered out
// of order.
//
// This pass is the only place that inspects the template before workers start:
// chunk boundaries, assignment safety and path compilation are settled
// together, so no worker has to take a lock or mutate the path cache while
// rendering.
func (e *Engine) measureWindows(ctx context.Context, src io.Reader, windowSize int, tree *values.Tree) (int, error) {
	chunks := 0
	line := 1
	err := streamWindow(ctx, src, windowSize, func(window []byte, _ int) error {
		chunks++
		return prepareWindow(window, tree)
	}, &line)
	if err != nil {
		return chunks, err
	}
	return chunks, nil
}

// prepareWindow reports whether one window is safe to render out of order and
// precompiles the paths it references.
func prepareWindow(window []byte, tree *values.Tree) error {
	assigns, err := inspectText(window, tree)
	if err != nil {
		return err
	}
	if assigns {
		return errSequentialOnly
	}
	return nil
}

// inspectText walks template text, compiling every referenced path and
// reporting whether it assigns to the value tree. Nested default words are
// inspected too, because they are rendered as templates of their own.
func inspectText(text []byte, tree *values.Tree) (bool, error) {
	assigns := false
	err := walkReferences(text, func(ref reference) (bool, error) {
		if assigns || ref.Bare {
			return false, nil
		}
		// A bare ${NAME} resolves through the renderer's fast path, which never
		// compiles a path. The options that send it down the general path
		// instead (--only-vars/--ignore-vars, --on-missing=keep) keep the whole render
		// sequential, so skipping it here cannot leave a worker compiling.
		if ref.Op == opAssignDefault || ref.Op == opAssignIfUnset {
			assigns = true
			return false, nil
		}
		if _, err := tree.CompilePath(ref.Path); err != nil {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return false, err
	}
	return assigns, nil
}

// streamParallel renders a large template with several workers and writes the
// results in template order.
//
// It is only used when the engine can prove the template is safe to split:
// every chunk boundary is a line boundary, and assignment operators (:= and =)
// are absent, as verified by measureWindows before any worker starts.
func (e *Engine) streamParallel(ctx context.Context, dst io.Writer, src io.Reader, in engine.Input) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan chunkJob, runtime.GOMAXPROCS(0))
	results := make(chan chunkResult, runtime.GOMAXPROCS(0))

	var workers sync.WaitGroup
	workerCount := runtime.GOMAXPROCS(0)
	for range workerCount {
		workers.Go(func() {
			for job := range jobs {
				results <- e.renderChunk(ctx, job, in)
			}
		})
	}

	// The producer runs in this goroutine so backpressure stays natural.
	produceErr := make(chan error, 1)
	go func() {
		defer close(jobs)
		line := 1
		index := 0
		produceErr <- streamWindow(ctx, src, parallelChunkSize, func(window []byte, _ int) error {
			// streamWindow shifts its buffer in place, so the window must be
			// copied before a worker can outlive this call.
			job := chunkJob{index: index, source: bytes.Clone(window)}
			index++
			select {
			case jobs <- job:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}, &line)
	}()

	// Closing results once the last worker has sent its chunk is what ends the
	// ordered writer loop above. Without it the render blocks forever.
	go func() {
		workers.Wait()
		close(results)
	}()

	// Write results strictly in order, buffering the few chunks that finish
	// early.
	pending := map[int]chunkResult{}
	next := 0
	var writeErr error
	for result := range results {
		if result.err != nil && writeErr == nil {
			writeErr = result.err
			cancel()
		}
		pending[result.index] = result
		for {
			buffered, ok := pending[next]
			if !ok {
				break
			}
			delete(pending, next)
			next++
			if writeErr != nil {
				continue
			}
			if len(buffered.output) > 0 {
				if _, err := dst.Write(buffered.output); err != nil {
					writeErr = err
					cancel()
				}
			}
		}
	}

	// The range above ends only after every worker stopped, so the workers are
	// already joined here.
	if err := <-produceErr; err != nil && writeErr == nil {
		writeErr = err
	}
	if writeErr != nil {
		return writeErr
	}
	return nil
}

// renderChunk renders one chunk into a private buffer. The worker only reads
// the value tree and the schema; measureWindows has already proven the chunk
// contains no assignment, so nothing here writes shared state.
func (e *Engine) renderChunk(ctx context.Context, job chunkJob, in engine.Input) chunkResult {
	var out bytes.Buffer
	out.Grow(min(len(job.source)*2, parallelResultBuffer))
	if err := ScanSpans(job.source, func(span Span) error {
		return e.emitSpan(ctx, &out, job.source, span, in, true)
	}); err != nil {
		return chunkResult{index: job.index, err: err}
	}
	return chunkResult{index: job.index, output: out.Bytes()}
}
