package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/cli"
)

// TestRenderHelpNamesFlagArguments pins that every flag taking a value shows a
// semantic placeholder instead of pflag's Go type name (stringArray / string),
// and that --values-file advertises the YAML/JSON contract the render layer enforces.
func TestRenderHelpNamesFlagArguments(t *testing.T) {
	t.Parallel()
	cmd := cli.NewRootCommand(stubEngine{}, noopRunner)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"render", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	help := out.String()

	for _, want := range []string{
		"--values-file FILE",
		"--dotenv-file FILE",
		"--schema FILE",
		"--set KEY=VALUE",
		"--set-json KEY=JSON",
		"--only-vars VAR",
		"--ignore-vars VAR",
		"--on-missing POLICY",
		"--output FILE",
		"--output-dir DIR",
		"--process-env-prefix PREFIX",
		"--explain PATH",
		"--log-level LEVEL",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("help does not contain %q:\n%s", want, help)
		}
	}
	if strings.Contains(help, "stringArray") {
		t.Fatalf("help still leaks the Go type name:\n%s", help)
	}
}

// TestRenderHasNoLegacyFlagNames pins that the renamed flags left no aliases:
// --values and --env-prefix are gone, while -f/--values-file and
// --process-env-prefix are registered.
func TestRenderHasNoLegacyFlagNames(t *testing.T) {
	t.Parallel()
	renderCmd, _, err := cli.NewRootCommand(stubEngine{}, noopRunner).Find([]string{"render"})
	if err != nil {
		t.Fatalf("Find(render) error = %v", err)
	}
	flags := renderCmd.Flags()

	for _, legacy := range []string{"values", "env-prefix"} {
		if flags.Lookup(legacy) != nil {
			t.Fatalf("legacy flag --%s is still registered", legacy)
		}
	}
	valuesFile := flags.Lookup("values-file")
	if valuesFile == nil {
		t.Fatal("flag --values-file is not registered")
	}
	if valuesFile.Shorthand != "f" {
		t.Fatalf("--values-file shorthand = %q, want f", valuesFile.Shorthand)
	}
	if flags.Lookup("process-env-prefix") == nil {
		t.Fatal("flag --process-env-prefix is not registered")
	}
}

// TestRunnerReceivesAssembledRequest pins the CLI/assembler boundary: the command
// tree only parses flags, and the runner injected by internal/app receives the
// request they describe.
func TestRunnerReceivesAssembledRequest(t *testing.T) {
	t.Parallel()
	stub, run := newStubRunner()
	cmd := cli.NewRootCommand(stubEngine{}, run)
	cmd.SetArgs([]string{
		"render",
		"one.tmpl", "two.tmpl",
		"-f", "values.yaml",
		"--dotenv-file", "app.env",
		"--schema", "schema.json",
		"--set", "A=1",
		"--set-json", "B=2",
		"--no-process-env",
		"--on-missing", "keep",
		"--only-vars", "app",
		"--output-dir", "out",
		"--recursive",
		"--explain=app.port",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !stub.called {
		t.Fatal("the injected runner was not called")
	}
	req := stub.request
	if len(req.Input.Templates) != 2 || !req.Input.Recursive {
		t.Fatalf("Input = %+v, want the two templates and recursive", req.Input)
	}
	if len(req.Values.Files) != 1 || len(req.Values.DotenvFiles) != 1 || req.Values.Schema != "schema.json" {
		t.Fatalf("Values = %+v, want the file sources and schema", req.Values)
	}
	if len(req.Values.Sets) != 1 || len(req.Values.SetJSON) != 1 || !req.Values.NoProcessEnv {
		t.Fatalf("Values = %+v, want both set forms and no-process-env", req.Values)
	}
	if req.Output.Dir != "out" {
		t.Fatalf("Output = %+v, want the output directory", req.Output)
	}
	if req.Policies.OnMissing != "keep" || len(req.Policies.Only) != 1 {
		t.Fatalf("Policies = %+v, want keep and the only filter", req.Policies)
	}
	if !req.Explain.Enabled || len(req.Explain.Paths) != 1 || req.Explain.Paths[0] != "app.port" {
		t.Fatalf("Explain = %+v, want the exact path filter", req.Explain)
	}
	if req.Explain.Writer == nil {
		t.Fatal("Explain.Writer = nil, want the command's stderr")
	}
}

// TestRunnerReceivesEachOutputMode pins the output-mode wiring: the three flags
// are mutually exclusive choices of one destination, so each must reach the
// request as its own field and leave the others empty. The command matrix that
// exercises what the renderer does with them lives in test/e2e.
func TestRunnerReceivesEachOutputMode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		args     []string
		wantPath string
		wantDir  string
		wantIn   bool
	}{
		{name: "output file", args: []string{"--output", "out.txt"}, wantPath: "out.txt"},
		{name: "output directory", args: []string{"--output-dir", "out"}, wantDir: "out"},
		{name: "in place", args: []string{"--in-place"}, wantIn: true},
		{name: "default is stdout", args: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stub, run := newStubRunner()
			cmd := cli.NewRootCommand(stubEngine{}, run)
			cmd.SetArgs(append([]string{"render", "input.tmpl"}, tt.args...))
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			got := stub.request.Output
			if got.Path != tt.wantPath || got.Dir != tt.wantDir || got.InPlace != tt.wantIn {
				t.Fatalf("Output = %+v, want path=%q dir=%q inPlace=%v",
					got, tt.wantPath, tt.wantDir, tt.wantIn)
			}
		})
	}
}
