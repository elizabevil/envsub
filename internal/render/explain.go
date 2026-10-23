package render

import (
	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/event"
)

// explainFilter is a set of canonical path spellings. A nil filter selects every
// path, so "no --explain=PATH" needs no special case in the loop.
type explainFilter map[string]struct{}

// newExplainFilter parses the --explain paths once, not once per reference.
func newExplainFilter(paths []string) explainFilter {
	if len(paths) == 0 {
		return nil
	}
	filter := make(explainFilter, len(paths))
	for _, path := range paths {
		filter[explainName(path)] = struct{}{}
	}
	return filter
}

// matches reports whether a canonical name is selected. Exact match only; a
// parent path is not a prefix.
func (f explainFilter) matches(name string) bool {
	if f == nil {
		return true
	}
	_, ok := f[name]
	return ok
}

// explainTemplate emits one explain event per referenced variable, in
// first-appearance order. It only reads names and recorded origins, so it cannot
// change the rendered output.
func explainTemplate(events *event.Writer, pathEngine engine.PathEngine, req Request, tmpl string, src []byte, run explainRun) error {
	paths, err := pathEngine.PathsFor(src, req.Policies.replaces)
	if err != nil {
		return diag.Wrapf(diag.KindTemplate, "explain %s: %w", tmpl, err)
	}

	filter := newExplainFilter(req.Explain.Paths)
	seen := make(map[string]struct{}, len(paths))
	for _, raw := range paths {
		key := parseExplainKey(raw)
		if key.Name == "" || !filter.matches(key.Name) {
			continue
		}
		if _, ok := seen[key.Name]; ok {
			continue
		}
		seen[key.Name] = struct{}{}

		lineage := run.resolve(key)
		if err := events.Emit(event.TypeExplain, event.ExplainData{
			Template: tmpl,
			Variable: key.Name,
			Origin:   lineage.winner,
			Shadowed: lineage.shadowed,
		}); err != nil {
			return diag.Wrapf(diag.KindIO, "write explain: %w", err)
		}
	}
	return nil
}
