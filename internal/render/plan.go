package render

import (
	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/output"
)

// plan is everything the render loop consumes: the buffered writer, one stream
// target per template, and the engine capabilities negotiated for this request.
// Building it in one place keeps every mode decision out of the loop, so the loop
// only renders.
type plan struct {
	// writer accepts a fully rendered template.
	writer output.Writer
	// streams holds one target per template; a nil entry means "buffer it and let
	// writer write the result".
	streams []output.StreamWriter
	// caps records which optional engine capabilities this request may use.
	caps engineCaps
}

// engineCaps records the optional engine capabilities a request may use.
type engineCaps struct {
	// stream is the engine's writer-facing API, if the engine supports it.
	stream engine.StreamEngine
	// canStream reports whether stream may be used.
	canStream bool
	// reader is the engine's reader-facing API, if the engine supports it.
	reader engine.StreamReaderEngine
	// canStreamReader reports whether reader may be used.
	canStreamReader bool
}

// buildPlan decides the whole render shape once: the writer, the per-template
// stream targets and the engine capabilities.
func (req Request) buildPlan(eng RenderEngine, templates []template) (plan, error) {
	writer, err := req.newWriter()
	if err != nil {
		return plan{}, err
	}
	streams, err := req.newStreamTargets(templates)
	if err != nil {
		return plan{}, err
	}
	return plan{writer: writer, streams: streams, caps: negotiateCaps(eng, req)}, nil
}

// negotiateCaps resolves the engine's optional capabilities once. --explain turns
// streaming off because its report needs the tree a render leaves behind, which the
// streaming path cannot expose.
func negotiateCaps(eng RenderEngine, req Request) engineCaps {
	caps := engineCaps{}
	if streamer, ok := eng.(engine.StreamEngine); ok {
		caps.stream, caps.canStream = streamer, true
	}
	if reader, ok := eng.(engine.StreamReaderEngine); ok {
		caps.reader, caps.canStreamReader = reader, true
	}
	if req.Explain.Active() {
		caps.canStream, caps.canStreamReader = false, false
	}
	return caps
}

// destination bundles the two output sinks of the template at index i.
func (p plan) destination(i int) destination {
	return destination{writer: p.writer, stream: p.streams[i]}
}
