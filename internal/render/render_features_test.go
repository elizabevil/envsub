package render_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/engine/extended"
	"github.com/elizabevil/envsub/internal/render"
)

// extendedEngine returns the real extended engine.
func extendedEngine() render.RenderEngine {
	return extended.New()
}

// countingEngine wraps the real engine and counts every render it performs, so a
// test can prove the pipeline actually invoked the engine rather than skipping it.
type countingEngine struct {
	inner   *extended.Engine
	renders int
}

func (c *countingEngine) Name() string { return c.inner.Name() }

func (c *countingEngine) Render(ctx context.Context, in engine.Input) ([]byte, error) {
	c.renders++
	return c.inner.Render(ctx, in)
}

func (c *countingEngine) WriteTo(ctx context.Context, w io.Writer, in engine.Input) error {
	c.renders++
	return c.inner.WriteTo(ctx, w, in)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return string(data)
}

// request returns a request for the given templates, which most tests extend.
func request(templates ...string) render.Request {
	return render.Request{Input: render.Input{Templates: templates}}
}

func TestRunValuesEnvAndSchema(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "base.yaml"), "app:\n  port: 8080\n  hosts: [a, b]\n")
	writeFile(t, filepath.Join(dir, "prod.yaml"), "app:\n  port: 9090\n")
	writeFile(t, filepath.Join(dir, ".env"), "NAME=world\n")
	writeFile(t, filepath.Join(dir, "schema.yaml"), `
type: object
properties:
  app:
    type: object
    properties:
      port: {type: integer, default: 1}
      hosts: {type: array, items: {type: string}}
  NAME: {type: string}
`)
	writeFile(t, filepath.Join(dir, "in.tmpl"), "port=${app.port}\nhosts=${app.hosts | join \",\"}\nname=${NAME}\n")
	out := filepath.Join(dir, "out.txt")

	req := request(filepath.Join(dir, "in.tmpl"))
	req.Values = render.Values{
		Files:       []string{filepath.Join(dir, "base.yaml"), filepath.Join(dir, "prod.yaml")},
		DotenvFiles: []string{filepath.Join(dir, ".env")},
		Schema:      filepath.Join(dir, "schema.yaml"),
	}
	req.Output.Path = out
	req.Policies.UnknownKeywords = "ignore"

	if err := render.Run(context.Background(), extendedEngine(), req); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := "port=9090\nhosts=a,b\nname=world\n"
	if got := readFile(t, out); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestRunSetFlagsPrecedence(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "in.tmpl"), "a=${A}\nb=${B}\nc=${C}\n")
	out := filepath.Join(dir, "out.txt")

	req := request(filepath.Join(dir, "in.tmpl"))
	req.Values.Sets = []string{"A=from-set", "B=from-set", "B=from-string"}
	req.Values.SetJSON = []string{"C=42"}
	req.Output.Path = out

	if err := render.Run(context.Background(), extendedEngine(), req); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := "a=from-set\nb=from-string\nc=42\n"
	if got := readFile(t, out); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestRunSetWithSchema(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "schema.yaml"), "type: object\nproperties:\n  A: {type: integer}\n")
	writeFile(t, filepath.Join(dir, "in.tmpl"), "a=${A}\n")
	out := filepath.Join(dir, "out.txt")

	req := request(filepath.Join(dir, "in.tmpl"))
	req.Values.Schema = filepath.Join(dir, "schema.yaml")
	req.Values.Sets = []string{"A=8080"}
	req.Output.Path = out
	req.Policies.UnknownKeywords = "ignore"

	if err := render.Run(context.Background(), extendedEngine(), req); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := readFile(t, out); got != "a=8080\n" {
		t.Fatalf("output = %q, want a=8080", got)
	}
}

func TestRunSchemaValidationFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "schema.yaml"), `
type: object
properties:
  A: {type: integer, minimum: 1}
`)
	writeFile(t, filepath.Join(dir, "in.tmpl"), "a=${A}\n")

	req := request(filepath.Join(dir, "in.tmpl"))
	req.Values.Schema = filepath.Join(dir, "schema.yaml")
	req.Values.Sets = []string{"A=0"}
	req.Policies.UnknownKeywords = "ignore"

	err := render.Run(context.Background(), extendedEngine(), req)
	if err == nil {
		t.Fatalf("Run() error = nil, want schema error")
	}
	if !strings.Contains(err.Error(), "/A") {
		t.Fatalf("Run() error = %q, want it to mention /A", err.Error())
	}
}

func TestRunInPlace(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.tmpl")
	writeFile(t, path, "port=${app.port}\n")

	req := request(path)
	req.Output.InPlace = true
	req.Values.Sets = []string{"app.port=8080"}

	if err := render.Run(context.Background(), extendedEngine(), req); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := readFile(t, path); got != "port=8080\n" {
		t.Fatalf("file = %q, want port=8080", got)
	}
}

func TestRunOutputDirBatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "templates")
	writeFile(t, filepath.Join(src, "a.tmpl"), "a=${A}\n")
	writeFile(t, filepath.Join(src, "sub", "b.tmpl"), "b=${B}\n")
	outDir := filepath.Join(dir, "out")

	req := request(src)
	req.Input.Recursive = true
	req.Output.Dir = outDir
	req.Values.Sets = []string{"A=1", "B=2"}
	req.Policies = render.Policies{
		OnMissing:       engine.UnsetError,
		OnEmpty:         engine.EmptyOK,
		UnknownKeywords: "ignore",
	}

	if err := render.Run(context.Background(), extendedEngine(), req); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := readFile(t, filepath.Join(outDir, "a.tmpl")); got != "a=1\n" {
		t.Fatalf("a.tmpl = %q, want a=1", got)
	}
	if got := readFile(t, filepath.Join(outDir, "sub", "b.tmpl")); got != "b=2\n" {
		t.Fatalf("sub/b.tmpl = %q, want b=2", got)
	}
}

func TestRunValidateOnlyWritesNothing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "in.tmpl")
	writeFile(t, path, "a=${A}\n")

	eng := &countingEngine{inner: extended.New()}
	req := request(path)
	req.ValidateOnly = true
	req.Values.Sets = []string{"A=1"}

	if err := render.Run(context.Background(), eng, req); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	// --validate-only must still run the engine (validation is real) but discard
	// its output: the engine was invoked and the template file is untouched.
	if eng.renders == 0 {
		t.Fatal("the engine was never invoked, want --validate-only to still render")
	}
	if got := readFile(t, path); got != "a=${A}\n" {
		t.Fatalf("template changed: %q", got)
	}
}

func TestRunOptionValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		req  render.Request
	}{
		{name: "in place with output", req: render.Request{Output: render.Output{InPlace: true, Path: "x"}}},
		{name: "in place with output dir", req: render.Request{Output: render.Output{InPlace: true, Dir: "d"}}},
		{name: "output with output dir", req: render.Request{Output: render.Output{Path: "x", Dir: "d"}}},
		{name: "validate only with output", req: render.Request{ValidateOnly: true, Output: render.Output{Path: "x"}}},
		{name: "invalid unset", req: render.Request{Policies: render.Policies{OnMissing: "bogus"}}},
		{name: "invalid empty", req: render.Request{Policies: render.Policies{OnEmpty: "bogus"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := render.Run(context.Background(), extendedEngine(), tt.req)
			if err == nil {
				t.Fatalf("Run() error = nil, want usage error")
			}
			var de *diag.Error
			if !errors.As(err, &de) || de.Kind != diag.KindUsage {
				t.Fatalf("Run() error = %v, want a KindUsage error", err)
			}
			if got := diag.ExitCode(err); got != 2 {
				t.Fatalf("Run() exit = %d (%v), want 2", got, err)
			}
		})
	}
}

func TestRunUnsetPoliciesEndToEnd(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "in.tmpl")
	writeFile(t, path, "a=${MISSING}\n")

	tests := []struct {
		name string
		req  render.Request
		want string
	}{
		{name: "empty", req: render.Request{Policies: render.Policies{OnMissing: engine.UnsetEmpty}}, want: "a=\n"},
		{name: "keep", req: render.Request{Policies: render.Policies{OnMissing: engine.UnsetKeep}}, want: "a=${MISSING}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out := filepath.Join(dir, tt.name+".out")
			tt.req.Input.Templates = []string{path}
			tt.req.Output.Path = out
			if err := render.Run(context.Background(), extendedEngine(), tt.req); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got := readFile(t, out); got != tt.want {
				t.Fatalf("output = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunInvalidValueFileFormat(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "values.toml"), "a = 1\n")
	writeFile(t, filepath.Join(dir, "in.tmpl"), "a=${A}\n")

	req := request(filepath.Join(dir, "in.tmpl"))
	req.Values.Files = []string{filepath.Join(dir, "values.toml")}

	// An unrecognised value-file extension is rejected before any parse, so it is
	// a usage error rather than an IO failure.
	err := render.Run(context.Background(), extendedEngine(), req)
	if err == nil {
		t.Fatalf("Run() error = nil, want format error")
	}
	if got := diag.KindOf(err); got != diag.KindUsage {
		t.Fatalf("Run() error kind = %v (%v), want KindUsage", got, err)
	}
	if got := diag.ExitCode(err); got != 2 {
		t.Fatalf("Run() exit = %d (%v), want 2", got, err)
	}
}

// TestRunValuesFileRejectsDotenv pins the --values-file contract: it merges into the root
// namespace, so a .env file is a usage error naming --dotenv-file instead of being
// silently routed into the environment namespace.
func TestRunValuesFileRejectsDotenv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dotenv := filepath.Join(dir, "app.env")
	writeFile(t, dotenv, "K=1\n")
	writeFile(t, filepath.Join(dir, "in.tmpl"), "K=${K}\n")

	req := request(filepath.Join(dir, "in.tmpl"))
	req.Values.Files = []string{dotenv}

	err := render.Run(context.Background(), extendedEngine(), req)
	if err == nil {
		t.Fatal("Run() error = nil, want a usage error")
	}
	if got := diag.ExitCode(err); got != 2 {
		t.Fatalf("exit code = %d (%v), want 2", got, err)
	}
	if !strings.Contains(err.Error(), "--dotenv-file") {
		t.Fatalf("error = %q, want it to point at --dotenv-file", err)
	}
}

// TestRunEnvFileIgnoresExtension pins the --dotenv-file contract: the flag names
// the format, so a name ForFile cannot classify still parses as dotenv.
func TestRunEnvFileIgnoresExtension(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dotenv := filepath.Join(dir, ".env.production")
	writeFile(t, dotenv, "K=from_production\n")
	tmpl := filepath.Join(dir, "in.tmpl")
	writeFile(t, tmpl, "K=${K}\n")
	out := filepath.Join(dir, "out.txt")

	req := request(tmpl)
	req.Values.DotenvFiles = []string{dotenv}
	req.Values.NoProcessEnv = true
	req.Output.Path = out

	if err := render.Run(context.Background(), extendedEngine(), req); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := readFile(t, out); got != "K=from_production\n" {
		t.Fatalf("output = %q, want K=from_production", got)
	}
}

func TestRunInPlaceRequiresFile(t *testing.T) {
	t.Parallel()
	req := render.Request{Output: render.Output{InPlace: true}}
	err := render.Run(context.Background(), extendedEngine(), req)
	if err == nil {
		t.Fatalf("Run() error = nil, want usage error")
	}
	if got := diag.KindOf(err); got != diag.KindUsage {
		t.Fatalf("Run() error kind = %v (%v), want KindUsage", got, err)
	}
	if got := diag.ExitCode(err); got != 2 {
		t.Fatalf("Run() exit = %d (%v), want 2", got, err)
	}
}

// TestRunInvalidSetJSONIsTypeError covers rule G9: an invalid JSON literal is a
// value/type error (exit 4) whichever source it comes from, while a malformed
// assignment stays a usage error (exit 2).
func TestRunInvalidSetJSONIsTypeError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		req  render.Request
		want int
	}{
		{name: "invalid JSON literal", req: render.Request{Values: render.Values{SetJSON: []string{"A=[1,"}}}, want: 4},
		{name: "malformed assignment", req: render.Request{Values: render.Values{SetJSON: []string{"A"}}}, want: 2},
		{name: "invalid path", req: render.Request{Values: render.Values{Sets: []string{"app..port=1"}}}, want: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "in.tmpl"), "a=${A}\n")
			tt.req.Input.Templates = []string{filepath.Join(dir, "in.tmpl")}
			tt.req.Output.Path = filepath.Join(dir, "out.txt")

			err := render.Run(context.Background(), extendedEngine(), tt.req)
			if err == nil {
				t.Fatal("Run() error = nil, want an error")
			}
			if got := diag.ExitCode(err); got != tt.want {
				t.Fatalf("exit code = %d (%v), want %d", got, err, tt.want)
			}
		})
	}
}

// TestRunRendersYAMLDateScalar pins the user-visible regression: a value file with
// an unquoted date used to fail every path that is not a formatter
// ("value type time.Time requires a formatter", exit 4). Both file formats must
// render the text the source wrote.
func TestRunRendersYAMLDateScalar(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "values.yaml"), "d: 2024-01-01\nstamp: 2024-01-01T10:00:00Z\n")
	tmpl := filepath.Join(dir, "input.tmpl")
	writeFile(t, tmpl, "d=${d} stamp=${stamp}\n")

	out := filepath.Join(dir, "out.txt")
	req := request(tmpl)
	req.Values = render.Values{Files: []string{filepath.Join(dir, "values.yaml")}}
	req.Output.Path = out
	if err := render.Run(context.Background(), extendedEngine(), req); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := readFile(t, out), "d=2024-01-01 stamp=2024-01-01T10:00:00Z\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}
