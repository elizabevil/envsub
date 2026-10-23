package render

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/event"
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

// fixedEventTime keeps envelope timestamps out of the assertions while still
// exercising the real encoder.
var fixedEventTime = time.Date(2026, 10, 22, 23, 43, 37, 41_000_000, time.UTC)

func newExplainEvents(w *bytes.Buffer) *event.Writer {
	return event.NewWriterWithClock(w, func() time.Time { return fixedEventTime })
}

// TestExplainTemplateWritesJSONL pins the report shape: one JSON object per
// referenced variable, in first-appearance order, with each path reported in the
// spelling the template used.
func TestExplainTemplateWritesJSONL(t *testing.T) {
	t.Parallel()
	run := explainRun{tree: values.NewTree(), sources: explainer{
		root: map[string][]values.Meta{
			"A": {{Kind: "set", Method: "flag", Source: "A"}},
			"K": {
				{Kind: "values", Method: "file", Source: "values.yaml", File: "values.yaml"},
				{Kind: "envfile", Method: "file", Source: ".env", File: ".env", Line: 1},
			},
		},
		env: map[string][]values.Meta{"K": {{Kind: "process", Method: "env", Source: "K"}}},
	}}
	req := Request{Explain: Explain{Enabled: true}}
	eng := stubPathEngine{paths: []string{"@process.K", "A", "K", "NOPE"}}

	var got bytes.Buffer
	if err := explainTemplate(newExplainEvents(&got), eng, req, "app.tmpl", []byte("irrelevant"), run); err != nil {
		t.Fatalf("explainTemplate() error = %v", err)
	}
	want := strings.Join([]string{
		`{"schema":1,"type":"explain","time":"2026-10-22T23:43:37.041Z","seq":1,"data":{"template":"app.tmpl","variable":"@process.K","origin":{"kind":"process","method":"env","source":"K","file":"","line":0},"shadowed":[]}}`,
		`{"schema":1,"type":"explain","time":"2026-10-22T23:43:37.041Z","seq":2,"data":{"template":"app.tmpl","variable":"A","origin":{"kind":"set","method":"flag","source":"A","file":"","line":0},"shadowed":[]}}`,
		`{"schema":1,"type":"explain","time":"2026-10-22T23:43:37.041Z","seq":3,"data":{"template":"app.tmpl","variable":"K","origin":{"kind":"envfile","method":"file","source":".env","file":".env","line":1},"shadowed":[{"kind":"values","method":"file","source":"values.yaml","file":"values.yaml","line":0}]}}`,
		`{"schema":1,"type":"explain","time":"2026-10-22T23:43:37.041Z","seq":4,"data":{"template":"app.tmpl","variable":"NOPE","origin":{"kind":"missing","method":"missing","source":"","file":"","line":0},"shadowed":[]}}`,
	}, "\n") + "\n"
	if got.String() != want {
		t.Fatalf("explainTemplate() =\n%swant\n%s", got.String(), want)
	}
}

// TestExplainTemplateShadowedOrder pins the order of a multi-frame shadowed
// list: the closest loser comes first and the lowest-priority layer comes
// last, so "shadowed followed by origin" is the whole lineage, lowest
// priority last.
//
// Every other assertion covers at most one shadowed frame, where the order
// carries no information at all.
func TestExplainTemplateShadowedOrder(t *testing.T) {
	t.Parallel()
	run := explainRun{tree: values.NewTree(), sources: explainer{
		root: map[string][]values.Meta{
			// Recorded lowest priority first: the schema default is the bottom
			// layer, then the value file, then the two .env files, then --set.
			"K": {
				{Kind: "schema", Method: "default", Source: "#/properties/K/default", File: "schema.json"},
				{Kind: "values", Method: "file", Source: "values.yaml", File: "values.yaml"},
				{Kind: "envfile", Method: "file", Source: "base.env", File: "base.env", Line: 1},
				{Kind: "envfile", Method: "file", Source: "debug.env", File: "debug.env", Line: 2},
				{Kind: "set", Method: "flag", Source: "K"},
			},
		},
	}}
	req := Request{Explain: Explain{Enabled: true}}
	eng := stubPathEngine{paths: []string{"K"}}

	var got bytes.Buffer
	if err := explainTemplate(newExplainEvents(&got), eng, req, "app.tmpl", nil, run); err != nil {
		t.Fatalf("explainTemplate() error = %v", err)
	}
	want := `{"schema":1,"type":"explain","time":"2026-10-22T23:43:37.041Z","seq":1,"data":{"template":"app.tmpl","variable":"K","origin":{"kind":"set","method":"flag","source":"K","file":"","line":0},"shadowed":[` +
		`{"kind":"envfile","method":"file","source":"debug.env","file":"debug.env","line":2},` +
		`{"kind":"envfile","method":"file","source":"base.env","file":"base.env","line":1},` +
		`{"kind":"values","method":"file","source":"values.yaml","file":"values.yaml","line":0},` +
		`{"kind":"schema","method":"default","source":"#/properties/K/default","file":"schema.json","line":0}` +
		`]}}` + "\n"
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
		root: map[string][]values.Meta{
			"app.port": {{Kind: "values", Method: "file", Source: "values.yaml", File: "values.yaml"}},
		},
	}}
	req := Request{Explain: Explain{Enabled: true, Paths: []string{"app.port", "app.port.x"}}}
	eng := stubPathEngine{paths: []string{"app.host", "app.port", "app.port.x"}}

	var got bytes.Buffer
	if err := explainTemplate(newExplainEvents(&got), eng, req, "app.tmpl", nil, run); err != nil {
		t.Fatalf("explainTemplate() error = %v", err)
	}
	want := strings.Join([]string{
		`{"schema":1,"type":"explain","time":"2026-10-22T23:43:37.041Z","seq":1,"data":{"template":"app.tmpl","variable":"app.port","origin":{"kind":"values","method":"file","source":"values.yaml","file":"values.yaml","line":0},"shadowed":[]}}`,
		`{"schema":1,"type":"explain","time":"2026-10-22T23:43:37.041Z","seq":2,"data":{"template":"app.tmpl","variable":"app.port.x","origin":{"kind":"missing","method":"missing","source":"","file":"","line":0},"shadowed":[]}}`,
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
		{path: `["a.b"].k`, wantName: `["a.b"].k`, wantRootKey: `["a.b"].k`},
		{path: `app["my key"]`, wantName: `app["my key"]`, wantRootKey: `app["my key"]`},
		// The report names the canonical spelling, and --explain compares against it,
		// so surrounding whitespace in the reference does not change the lookup key.
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
		root: map[string][]values.Meta{
			"FROM_FILE": {{Kind: "values", Method: "file", Source: "values.yaml", File: "values.yaml"}},
			"K": {
				{Kind: "values", Method: "file", Source: "values.yaml", File: "values.yaml"},
				{Kind: "envfile", Method: "file", Source: "base.env", File: "base.env", Line: 1},
				{Kind: "envfile", Method: "file", Source: "debug.env", File: "debug.env", Line: 2},
				{Kind: "set", Method: "flag", Source: "K"},
			},
			"DFLT": {{Kind: "schema", Method: "default", Source: "#/properties/DFLT/default", File: "schema.yaml"}},
		},
		env: map[string][]values.Meta{
			"K":    {{Kind: "process", Method: "env", Source: "K"}},
			"PROC": {{Kind: "process", Method: "env", Source: "PROC"}},
		},
	}

	tests := []struct {
		path         string
		wantSource   string
		wantKind     values.SourceKind
		wantShadowed []string
	}{
		{
			path:       "K",
			wantSource: "K",
			wantKind:   values.SourceSet,
			// closest loser first: debug.env, base.env, values.yaml
			wantShadowed: []string{"debug.env", "base.env", "values.yaml"},
		},
		{
			path:       "@process.K",
			wantSource: "K",
			wantKind:   values.SourceProcess,
		},
		{path: "FROM_FILE", wantSource: "values.yaml", wantKind: values.SourceValues},
		{path: "PROC", wantSource: "", wantKind: values.SourceMissing},
		{path: "DFLT", wantSource: "#/properties/DFLT/default", wantKind: values.SourceSchema},
		{path: "MISSING", wantSource: "", wantKind: values.SourceMissing},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			origin, shadowed := expl.source(tt.path, values.NewTree())
			if origin.Source != tt.wantSource || origin.Kind != string(tt.wantKind) {
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
	tree.RecordMeta(metaSegments(t, "K"), values.Meta{Kind: values.SourceEnvFile, File: ".env", Line: 4})

	origin, shadowed := (explainer{}).source("K", tree)
	if origin.Kind != "envfile" || origin.File != ".env" || origin.Line != 4 {
		t.Fatalf("source(K) = %+v, want envfile .env:4", origin)
	}
	if len(shadowed) != 0 {
		t.Fatalf("source(K) shadowed = %#v, want none", shadowed)
	}
}

// TestFrameFor pins how recorded metadata maps onto the report vocabulary.
func TestMetaOrigin(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		meta       values.Meta
		wantKind   values.SourceKind
		wantSource string
	}{
		{name: "dotenv keeps file and line", meta: values.Meta{Kind: values.SourceEnvFile, File: ".env", Line: 3}, wantKind: values.SourceEnvFile, wantSource: ".env"},
		{name: "values file", meta: values.Meta{Kind: values.SourceValues, File: "v.yaml"}, wantKind: values.SourceValues, wantSource: "v.yaml"},
		{name: "process", meta: values.Meta{Kind: values.SourceProcess, File: "ENVSUB_X"}, wantKind: values.SourceProcess, wantSource: "ENVSUB_X"},
		{name: "set", meta: values.Meta{Kind: values.SourceSet}, wantKind: values.SourceSet, wantSource: ""},
		{name: "schema", meta: values.Meta{Kind: values.SourceSchema, File: "schema.yaml"}, wantKind: values.SourceSchema, wantSource: "schema.yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			origin := frameFor(tt.meta)
			if origin.Kind != string(tt.wantKind) || origin.Source != tt.wantSource {
				t.Fatalf("frameFor(%+v) = %+v (%q), want kind %q source %q",
					tt.meta, origin, origin.Source, tt.wantKind, tt.wantSource)
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
	err := explainTemplate(newExplainEvents(&bytes.Buffer{}), eng, req, "app.tmpl", nil, explainRun{tree: values.NewTree()})
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
	observer.AddRoot("K", values.Meta{Kind: values.SourceEnvFile, Method: values.MethodFile, Source: ".env", File: ".env", Line: 1})
	observer.AddRoot("K", values.Meta{Kind: values.SourceSet, Method: values.MethodFlag, Source: "K"})

	winner, shadowed := run.sources.source("K", run.tree)
	if winner.Kind != string(values.SourceSet) {
		t.Fatalf("winner = %+v, want the set origin", winner)
	}
	if len(shadowed) != 1 || shadowed[0].Kind != string(values.SourceEnvFile) || shadowed[0].File != ".env" || shadowed[0].Line != 1 {
		t.Fatalf("shadowed = %#v, want the envfile frame", shadowed)
	}
	if got := run.sources.env["FROM_PROCESS"]; len(got) != 1 {
		t.Fatalf("@process origins = %#v, want one", got)
	}
}

// TestExplainerSourceAssignmentClosesTheChain pins that an assignment the
// renderer wrote, which never passes through the loader observer, still ends the
// lineage as the newest frame.
func TestExplainerSourceAssignmentClosesTheChain(t *testing.T) {
	t.Parallel()
	tree := values.NewTree()
	tree.RecordMeta(metaSegments(t, "K"), values.Meta{
		Kind: values.SourceAssign, Method: values.MethodAssign, Source: "app.tmpl", File: "app.tmpl", Line: 3,
	})
	sources := explainer{root: map[string][]values.Meta{
		"K": {{Kind: values.SourceValues, Method: values.MethodFile, Source: "values.yaml", File: "values.yaml"}},
	}}

	winner, shadowed := sources.source("K", tree)
	if winner.Kind != string(values.SourceAssign) || winner.Method != string(values.MethodAssign) || winner.Line != 3 {
		t.Fatalf("winner = %+v, want the assignment frame", winner)
	}
	if len(shadowed) != 1 || shadowed[0].Kind != string(values.SourceValues) {
		t.Fatalf("shadowed = %+v, want the value file it covered", shadowed)
	}
}

// TestExplainerSourceFallsBackToAncestorOnlyForExistingValues pins E3's repair:
// an exact origin always wins, an ancestor's origin is used only when the value
// itself exists (schema materialization creates such paths without their own
// meta), and a path that does not exist is missing rather than inheriting its
// parent's source.
func TestExplainerSourceFallsBackToAncestorOnlyForExistingValues(t *testing.T) {
	t.Parallel()
	tree := values.NewTree()
	tree.Root["app"] = map[string]any{"port": 8080}
	tree.RecordMeta(metaSegments(t, "app"), values.Meta{
		Kind: values.SourceValues, Method: values.MethodFile, Source: "values.yaml", File: "values.yaml",
	})

	origin, shadowed := (explainer{}).source("app.port", tree)
	if origin.Kind != string(values.SourceValues) || origin.Source != "values.yaml" {
		t.Fatalf("source(app.port) = %+v, want the file origin inherited from app", origin)
	}
	if len(shadowed) != 0 {
		t.Fatalf("source(app.port) shadowed = %+v, want none", shadowed)
	}

	// The exact child origin wins over the ancestor.
	tree.RecordMeta(metaSegments(t, "app.port"), values.Meta{
		Kind: values.SourceSet, Method: values.MethodFlag, Source: "app.port",
	})
	origin, _ = (explainer{}).source("app.port", tree)
	if origin.Kind != string(values.SourceSet) {
		t.Fatalf("source(app.port) = %+v, want the exact set origin", origin)
	}

	// app.port.x does not exist, so it must not inherit app's or app.port's
	// origin (the original E3 symptom).
	origin, _ = (explainer{}).source("app.port.x", tree)
	if origin.Kind != string(values.SourceMissing) {
		t.Fatalf("source(app.port.x) = %+v, want missing", origin)
	}
}

// TestExplainTemplateSkipsFilteredVariables pins E4: a path the --only-vars /
// --ignore-vars filters leave unsubstituted never resolves, so the report must
// not pretend it came from a source.
func TestExplainTemplateSkipsFilteredVariables(t *testing.T) {
	t.Parallel()
	run := explainRun{tree: values.NewTree(), sources: explainer{root: map[string][]values.Meta{
		"A": {{Kind: values.SourceSet, Method: values.MethodFlag, Source: "A"}},
		"B": {{Kind: values.SourceSet, Method: values.MethodFlag, Source: "B"}},
	}}}
	req := Request{Explain: Explain{Enabled: true}, Policies: Policies{Only: []string{"A"}}}
	eng := stubPathEngine{paths: []string{"A", "B"}}

	var got bytes.Buffer
	if err := explainTemplate(newExplainEvents(&got), eng, req, "app.tmpl", nil, run); err != nil {
		t.Fatalf("explainTemplate() error = %v", err)
	}
	if strings.Contains(got.String(), `"variable":"B"`) {
		t.Fatalf("report = %q, want B omitted because --only-vars excluded it", got.String())
	}
	if !strings.Contains(got.String(), `"variable":"A"`) {
		t.Fatalf("report = %q, want A kept", got.String())
	}
}

// metaSegments parses a spelling into the segments the metadata API takes.
func metaSegments(t *testing.T, spelling string) []values.Segment {
	t.Helper()
	segments, err := values.ParsePath(spelling)
	if err != nil {
		t.Fatalf("ParsePath(%q) error = %v", spelling, err)
	}
	return segments
}
