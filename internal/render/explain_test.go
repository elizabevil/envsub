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
	run := explainRun{tree: values.NewTree(), sources: explainer{
		root: map[string][]explainOrigin{
			"A": {{kind: "set", method: "flag", source: "A"}},
			"K": {
				{kind: "values", method: "file", source: "values.yaml", file: "values.yaml"},
				{kind: "envfile", method: "file", source: ".env", file: ".env", line: 1},
			},
		},
		env: map[string][]explainOrigin{"K": {{kind: "process", method: "env", source: "K"}}},
	}}
	req := Request{Explain: Explain{Enabled: true}}
	eng := stubPathEngine{paths: []string{"@process.K", "A", "K", "NOPE"}}

	var got bytes.Buffer
	if err := explainTemplate(&got, eng, req, "app.tmpl", []byte("irrelevant"), run); err != nil {
		t.Fatalf("explainTemplate() error = %v", err)
	}
	want := strings.Join([]string{
		`{"template":"app.tmpl","variable":"@process.K","origin":{"kind":"process","method":"env","source":"K","file":"","line":0},"shadowed":[]}`,
		`{"template":"app.tmpl","variable":"A","origin":{"kind":"set","method":"flag","source":"A","file":"","line":0},"shadowed":[]}`,
		`{"template":"app.tmpl","variable":"K","origin":{"kind":"envfile","method":"file","source":".env","file":".env","line":1},"shadowed":[{"kind":"values","method":"file","source":"values.yaml","file":"values.yaml","line":0}]}`,
		`{"template":"app.tmpl","variable":"NOPE","origin":{"kind":"missing","method":"missing","source":"","file":"","line":0},"shadowed":[]}`,
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
	run := explainRun{tree: values.NewTree(), sources: explainer{
		root: map[string][]explainOrigin{
			"app.port": {{kind: "values", method: "file", source: "values.yaml", file: "values.yaml"}},
		},
	}}
	req := Request{Explain: Explain{Enabled: true, Paths: []string{"app.port", "app.port.x"}}}
	eng := stubPathEngine{paths: []string{"app.host", "app.port", "app.port.x"}}

	var got bytes.Buffer
	if err := explainTemplate(&got, eng, req, "app.tmpl", nil, run); err != nil {
		t.Fatalf("explainTemplate() error = %v", err)
	}
	want := strings.Join([]string{
		`{"template":"app.tmpl","variable":"app.port","origin":{"kind":"values","method":"file","source":"values.yaml","file":"values.yaml","line":0},"shadowed":[]}`,
		`{"template":"app.tmpl","variable":"app.port.x","origin":{"kind":"missing","method":"missing","source":"","file":"","line":0},"shadowed":[]}`,
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
			"FROM_FILE": {{kind: "values", method: "file", source: "values.yaml", file: "values.yaml"}},
			"K": {
				{kind: "values", method: "file", source: "values.yaml", file: "values.yaml"},
				{kind: "envfile", method: "file", source: "base.env", file: "base.env", line: 1},
				{kind: "envfile", method: "file", source: "debug.env", file: "debug.env", line: 2},
				{kind: "set", method: "flag", source: "K"},
			},
			"DFLT": {{kind: "schema", method: "default", source: "#/properties/DFLT/default", file: "schema.yaml"}},
		},
		env: map[string][]explainOrigin{
			"K":    {{kind: "process", method: "env", source: "K"}},
			"PROC": {{kind: "process", method: "env", source: "PROC"}},
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
			wantSource: "K",
			wantKind:   "set",
			// closest loser first: debug.env, base.env, values.yaml
			wantShadowed: []string{"debug.env", "base.env", "values.yaml"},
		},
		{
			path:       "@process.K",
			wantSource: "K",
			wantKind:   "process",
		},
		{path: "FROM_FILE", wantSource: "values.yaml", wantKind: "values"},
		{path: "PROC", wantSource: "", wantKind: "missing"},
		{path: "DFLT", wantSource: "#/properties/DFLT/default", wantKind: "schema"},
		{path: "MISSING", wantSource: "", wantKind: "missing"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			origin, shadowed := expl.source(tt.path, values.NewTree())
			if origin.source != tt.wantSource || origin.kind != tt.wantKind {
				t.Fatalf("source(%q) = %+v, want source %q kind %q", tt.path, origin, tt.wantSource, tt.wantKind)
			}
			got := make([]string, 0, len(shadowed))
			for _, frame := range shadowed {
				got = append(got, frame.Source)
			}
			if !equalStrings(got, tt.wantShadowed) {
				t.Fatalf("source(%q) shadowed = %#v, want %#v", tt.path, got, tt.wantShadowed)
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
	tree.RecordMeta("K", values.Meta{Kind: "envfile", File: ".env", Line: 4})

	origin, shadowed := (explainer{}).source("K", tree)
	if origin.kind != "envfile" || origin.file != ".env" || origin.line != 4 {
		t.Fatalf("source(K) = %+v, want envfile .env:4", origin)
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
		{name: "dotenv keeps file and line", meta: values.Meta{Kind: "envfile", File: ".env", Line: 3}, wantKind: "envfile", wantSource: ".env"},
		{name: "values file", meta: values.Meta{Kind: "values", File: "v.yaml"}, wantKind: "values", wantSource: "v.yaml"},
		{name: "process", meta: values.Meta{Kind: "process", File: "ENVSUB_X"}, wantKind: "process", wantSource: "ENVSUB_X"},
		{name: "set", meta: values.Meta{Kind: "set"}, wantKind: "set", wantSource: ""},
		{name: "schema", meta: values.Meta{Kind: "schema", File: "schema.yaml"}, wantKind: "schema", wantSource: "schema.yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			origin := metaOrigin(tt.meta)
			if origin.kind != tt.wantKind || origin.source != tt.wantSource {
				t.Fatalf("metaOrigin(%+v) = %+v (%q), want kind %q source %q",
					tt.meta, origin, origin.source, tt.wantKind, tt.wantSource)
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
	err := explainTemplate(&bytes.Buffer{}, eng, req, "app.tmpl", nil, explainRun{tree: values.NewTree()})
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
	run := explainRun{tree: values.NewTree()}
	var observer load.Observer = &run
	observer.AddEnv("FROM_PROCESS")
	observer.AddRoot("K", load.Origin{Kind: values.SourceEnvFile, Method: values.MethodFile, Source: ".env", File: ".env", Line: 1})
	observer.AddRoot("K", load.Origin{Kind: values.SourceSet, Method: values.MethodFlag, Source: "K"})

	winner, shadowed := run.sources.source("K", run.tree)
	if winner.kind != string(values.SourceSet) {
		t.Fatalf("winner = %+v, want the set origin", winner)
	}
	if len(shadowed) != 1 || shadowed[0].Kind != "envfile" || shadowed[0].File != ".env" || shadowed[0].Line != 1 {
		t.Fatalf("shadowed = %#v, want the envfile frame", shadowed)
	}
	if got := run.sources.env["FROM_PROCESS"]; len(got) != 1 {
		t.Fatalf("@process origins = %#v, want one", got)
	}
}
