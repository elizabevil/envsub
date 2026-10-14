package render

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/load"
	"github.com/elizabevil/envsub/internal/values"
)

// stubPathEngine is a PathEngine that returns a fixed path list.
type stubPathEngine struct {
	paths []string
	err   error
}

func (stubPathEngine) Name() string { return "stub" }
func (stubPathEngine) Render(context.Context, engine.Input) ([]byte, error) {
	return nil, nil
}
func (s stubPathEngine) Paths([]byte) ([]string, error) { return s.paths, s.err }

// TestExplainTemplateWritesJSONL pins the report shape: one JSON object per
// referenced variable, in first-appearance order, with each path reported in the
// spelling the template used.
func TestExplainTemplateWritesJSONL(t *testing.T) {
	t.Parallel()
	run := explainRun{sources: explainer{
		root: map[string][]explainOrigin{
			"A": {{kind: "set"}},
			"K": {{kind: "values", file: "values.yaml"}, {kind: "dotenv", file: ".env", line: 1}},
		},
		env: map[string][]explainOrigin{"K": {{kind: "process"}}},
	}}
	req := Request{Explain: Explain{Enabled: true}}
	eng := stubPathEngine{paths: []string{"@process.K", "A", "K", "NOPE"}}

	var got bytes.Buffer
	if err := explainTemplate(&got, eng, req, "app.tmpl", []byte("irrelevant"), run); err != nil {
		t.Fatalf("explainTemplate() error = %v", err)
	}
	want := strings.Join([]string{
		`{"template":"app.tmpl","variable":"@process.K","source":"process","kind":"process"}`,
		`{"template":"app.tmpl","variable":"A","source":"set","kind":"set"}`,
		`{"template":"app.tmpl","variable":"K","source":".env:1","kind":"dotenv","file":".env","line":1,"shadowed":["values.yaml"]}`,
		`{"template":"app.tmpl","variable":"NOPE","source":"missing","kind":"missing"}`,
	}, "\n") + "\n"
	if got.String() != want {
		t.Fatalf("explainTemplate() =\n%swant\n%s", got.String(), want)
	}
}

// TestExplainTemplateFiltersByExactPath pins --explain=PATH: only the listed
// paths are reported, the filter matches the reported spelling, and a dotted path
// is exact rather than a prefix.
func TestExplainTemplateFiltersByExactPath(t *testing.T) {
	t.Parallel()
	run := explainRun{sources: explainer{
		root: map[string][]explainOrigin{
			"app.port": {{kind: "values", file: "values.yaml"}},
		},
	}}
	req := Request{Explain: Explain{Enabled: true, Paths: []string{"app.port", "app.port.x"}}}
	eng := stubPathEngine{paths: []string{"app.host", "app.port", "app.port.x"}}

	var got bytes.Buffer
	if err := explainTemplate(&got, eng, req, "app.tmpl", nil, run); err != nil {
		t.Fatalf("explainTemplate() error = %v", err)
	}
	want := strings.Join([]string{
		`{"template":"app.tmpl","variable":"app.port","source":"values.yaml","kind":"values","file":"values.yaml"}`,
		`{"template":"app.tmpl","variable":"app.port.x","source":"missing","kind":"missing"}`,
	}, "\n") + "\n"
	if got.String() != want {
		t.Fatalf("explainTemplate() =\n%swant\n%s", got.String(), want)
	}
}

// TestExplainMatches pins the exact-match filter rules.
func TestExplainMatches(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		paths  []string
		target string
		want   bool
	}{
		{name: "no filter matches everything", target: "PORT", want: true},
		{name: "exact match", paths: []string{"PORT"}, target: "PORT", want: true},
		{name: "different name", paths: []string{"PORT"}, target: "HOST", want: false},
		{name: "namespace needs its spelling", paths: []string{"@process.PORT"}, target: "PORT", want: false},
		{name: "namespace exact", paths: []string{"@process.PORT"}, target: "@process.PORT", want: true},
		{name: "root env key is not the namespace", paths: []string{"env.PORT"}, target: "PORT", want: false},
		{name: "root env key exact", paths: []string{"env.PORT"}, target: "env.PORT", want: true},
		{name: "dotted path is exact", paths: []string{"app.port"}, target: "app.port", want: true},
		{name: "root is not a prefix", paths: []string{"app"}, target: "app.port", want: false},
		{name: "dotted path is not a prefix", paths: []string{"app.port"}, target: "app.port.x", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			explain := Explain{Paths: tt.paths}
			if got := explain.matches(tt.target); got != tt.want {
				t.Fatalf("matches(%q) with %#v = %v, want %v", tt.target, tt.paths, got, tt.want)
			}
		})
	}
}

// TestExplainNameAndLookupKeys pins the canonical spelling and the namespace a
// path reads: only @process reads the process environment, everything else reads
// the root.
func TestExplainNameAndLookupKeys(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path        string
		wantName    string
		wantEnv     string
		wantRootKey string
	}{
		{path: "K", wantName: "K", wantRootKey: "K"},
		{path: "@process.K", wantName: "@process.K", wantEnv: "K"},
		{path: `@process["MY-VAR"]`, wantName: `@process["MY-VAR"]`, wantEnv: "MY-VAR"},
		{path: "env.K", wantName: "env.K", wantRootKey: "env.K"},
		{path: "app.port", wantName: "app.port", wantRootKey: "app.port"},
		{path: "hosts[0]", wantName: "hosts[0]", wantRootKey: "hosts[0]"},
		{path: `["a.b"].k`, wantName: `["a.b"].k`, wantRootKey: "a.b.k"},
		{path: `app["my key"]`, wantName: `app["my key"]`, wantRootKey: "app.my key"},
		// The report uses the canonical spelling, so a filter written the same way
		// matches even when the template used a different but equivalent form.
		{path: `  app.port  `, wantName: "app.port", wantRootKey: "app.port"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			if got := explainName(tt.path); got != tt.wantName {
				t.Fatalf("explainName(%q) = %q, want %q", tt.path, got, tt.wantName)
			}
			envName, rootKey := explainLookupKeys(tt.path)
			if envName != tt.wantEnv || rootKey != tt.wantRootKey {
				t.Fatalf("explainLookupKeys(%q) = (%q, %q), want (%q, %q)",
					tt.path, envName, rootKey, tt.wantEnv, tt.wantRootKey)
			}
		})
	}
}

// TestExplainerSource pins the priority stack: the root layers decide a bare name
// (--set beats dotenv beats value files), the @process namespace reads the process
// environment, and the shadowed list runs from the closest loser to the most
// distant.
func TestExplainerSource(t *testing.T) {
	t.Parallel()
	expl := explainer{
		root: map[string][]explainOrigin{
			"FROM_FILE": {{kind: "values", file: "values.yaml"}},
			"K":         {{kind: "values", file: "values.yaml"}, {kind: "dotenv", file: "base.env", line: 1}, {kind: "dotenv", file: "debug.env", line: 2}, {kind: "set"}},
			"DFLT":      {{kind: "schema"}},
		},
		env: map[string][]explainOrigin{
			"K":    {{kind: "process"}},
			"PROC": {{kind: "process"}},
		},
	}

	tests := []struct {
		path         string
		wantSource   string
		wantKind     string
		wantShadowed []string
	}{
		{
			path:       "K",
			wantSource: "set",
			wantKind:   "set",
			// closest loser first: debug.env, base.env, values.yaml
			wantShadowed: []string{"debug.env:2", "base.env:1", "values.yaml"},
		},
		{
			path:       "@process.K",
			wantSource: "process",
			wantKind:   "process",
		},
		{path: "FROM_FILE", wantSource: "values.yaml", wantKind: "values"},
		{path: "PROC", wantSource: "missing", wantKind: "missing"},
		{path: "DFLT", wantSource: "schema", wantKind: "schema"},
		{path: "MISSING", wantSource: "missing", wantKind: "missing"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			origin, shadowed := expl.source(tt.path, nil)
			if origin.source() != tt.wantSource || origin.kind != tt.wantKind {
				t.Fatalf("source(%q) = %+v, want source %q kind %q", tt.path, origin, tt.wantSource, tt.wantKind)
			}
			if !equalStrings(shadowed, tt.wantShadowed) {
				t.Fatalf("source(%q) shadowed = %#v, want %#v", tt.path, shadowed, tt.wantShadowed)
			}
		})
	}
}

// TestExplainerSourceFallsBackToTreeMeta pins that an origin recorded on the tree
// but not layered (for example a value written by an assignment) is still reported
// with its file and line.
func TestExplainerSourceFallsBackToTreeMeta(t *testing.T) {
	t.Parallel()
	tree := values.NewTree()
	tree.RecordEncodedMeta("K", values.Meta{Kind: "dotenv", File: ".env", Line: 4})

	origin, shadowed := (explainer{}).source("K", tree)
	if origin.kind != "dotenv" || origin.file != ".env" || origin.line != 4 {
		t.Fatalf("source(K) = %+v, want dotenv .env:4", origin)
	}
	if len(shadowed) != 0 {
		t.Fatalf("source(K) shadowed = %#v, want none", shadowed)
	}
}

// TestMetaOrigin pins how recorded metadata maps onto the report vocabulary.
func TestMetaOrigin(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		meta       values.Meta
		wantKind   string
		wantSource string
	}{
		{name: "dotenv keeps file and line", meta: values.Meta{Kind: "dotenv", File: ".env", Line: 3}, wantKind: "dotenv", wantSource: ".env:3"},
		{name: "values file", meta: values.Meta{Kind: "yaml", File: "v.yaml"}, wantKind: "yaml", wantSource: "v.yaml"},
		{name: "process", meta: values.Meta{Kind: "env"}, wantKind: "process", wantSource: "process"},
		{name: "set", meta: values.Meta{Kind: "set"}, wantKind: "set", wantSource: "set"},
		{name: "schema", meta: values.Meta{Kind: "schema"}, wantKind: "schema", wantSource: "schema"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			origin := metaOrigin(tt.meta)
			if origin.kind != tt.wantKind || origin.source() != tt.wantSource {
				t.Fatalf("metaOrigin(%+v) = %+v (%q), want kind %q source %q",
					tt.meta, origin, origin.source(), tt.wantKind, tt.wantSource)
			}
		})
	}
}

// TestExplainTemplateWrapsPathErrors pins that a scan failure is a template
// error, so --explain cannot mask a broken template.
func TestExplainTemplateWrapsPathErrors(t *testing.T) {
	t.Parallel()
	eng := stubPathEngine{err: errors.New("boom")}
	req := Request{Explain: Explain{Enabled: true}}
	err := explainTemplate(&bytes.Buffer{}, eng, req, "app.tmpl", nil, explainRun{})
	if err == nil {
		t.Fatal("explainTemplate() error = nil, want a wrapped error")
	}
	if got := diag.KindOf(err); got != diag.KindTemplate {
		t.Fatalf("explainTemplate() error kind = %v (%v), want KindTemplate", got, err)
	}
	if got := diag.ExitCode(err); got != 3 {
		t.Fatalf("explainTemplate() exit = %d (%v), want 3", got, err)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestObserverRecordsThroughTheInterface pins how the loader reports: it holds the
// observer as a load.Observer interface and calls it once per source. A value
// receiver here would write to a copy and the report would silently lose every
// origin, so this test goes through the interface the loader uses.
func TestObserverRecordsThroughTheInterface(t *testing.T) {
	t.Parallel()
	run := explainRun{}
	var observer load.Observer = &run
	observer.AddEnv("FROM_PROCESS")
	observer.AddRoot("K", load.Origin{Kind: values.SourceDotenv, File: ".env", Line: 1})
	observer.AddRoot("K", load.Origin{Kind: values.SourceSet})

	winner, shadowed := run.sources.source("K", run.tree)
	if winner.kind != string(values.SourceSet) {
		t.Fatalf("winner = %+v, want the set origin", winner)
	}
	if len(shadowed) != 1 || shadowed[0] != ".env:1" {
		t.Fatalf("shadowed = %#v, want the dotenv origin", shadowed)
	}
	if got := run.sources.env["FROM_PROCESS"]; len(got) != 1 {
		t.Fatalf("@process origins = %#v, want one", got)
	}
}
