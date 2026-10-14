package engine_test

import (
	"testing"

	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/engine/extended"
)

// TestExtendedEngineImplementsPipelineInterfaces pins the interfaces the render
// pipeline discovers by type assertion. With no registry and no capability flags,
// this is what "the engine is wired" means.
func TestExtendedEngineImplementsPipelineInterfaces(t *testing.T) {
	t.Parallel()
	eng := extended.New()
	if eng.Name() != engine.NameExtended {
		t.Fatalf("Name() = %q, want %q", eng.Name(), engine.NameExtended)
	}
	var _ engine.Engine = eng
	var _ engine.StreamEngine = eng
	var _ engine.StreamReaderEngine = eng
	var _ engine.PathEngine = eng
}

// TestReplacesBytes covers the filter entry point used by the render fast path:
// with no filters it accepts everything without building a string, and with
// filters it matches Replaces exactly.
func TestReplacesBytes(t *testing.T) {
	t.Parallel()
	plain := engine.Options{}
	for _, name := range []string{"APP", "app.port", ""} {
		if !plain.ReplacesBytes([]byte(name)) {
			t.Fatalf("ReplacesBytes(%q) with no filters = false, want true", name)
		}
	}

	filtered := engine.Options{Only: []string{"app"}, Ignore: []string{"app.port"}}
	for _, name := range []string{"app", "app.port", "app.host", "other"} {
		if got, want := filtered.ReplacesBytes([]byte(name)), filtered.Replaces(name); got != want {
			t.Fatalf("ReplacesBytes(%q) = %v, Replaces = %v", name, got, want)
		}
	}
}

// TestReplacesMatchesGrammarRoots pins that the filters derive the root from the
// template grammar: a namespace path is selected by its namespace name, a
// top-level special key by its bracketed key, and a bare pattern never reaches
// inside a path.
func TestReplacesMatchesGrammarRoots(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		only     []string
		ignore   []string
		path     string
		replaces bool
	}{
		{name: "namespace selected by name", only: []string{"process"}, path: "@process.NAME", replaces: true},
		{name: "namespace not selected by variable", only: []string{"NAME"}, path: "@process.NAME", replaces: false},
		{name: "namespace ignored by name", ignore: []string{"process"}, path: "@process.NAME", replaces: false},
		{name: "namespace path prefix keeps the sigil", only: []string{"@process.NAME"}, path: "@process.NAME", replaces: true},
		{name: "namespace without sigil does not match", only: []string{"process.NAME"}, path: "@process.NAME", replaces: false},
		{name: "quoted top-level key by its key", only: []string{"a.b"}, path: `["a.b"].k`, replaces: true},
		{name: "quoted top-level key prefix", only: []string{"a.b.k"}, path: `["a.b"].k`, replaces: true},
		{name: "bare root does not reach inside", only: []string{"app"}, path: "app.port", replaces: true},
		{name: "unrelated root", only: []string{"other"}, path: "app.port", replaces: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opts := engine.Options{Only: tt.only, Ignore: tt.ignore}
			if got := opts.Replaces(tt.path); got != tt.replaces {
				t.Fatalf("Replaces(%q) with only=%v ignore=%v = %v, want %v",
					tt.path, tt.only, tt.ignore, got, tt.replaces)
			}
		})
	}
}
