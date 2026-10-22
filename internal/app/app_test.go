package app

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureRun executes Run with args as the process arguments and stdin as the
// standard input, capturing both output streams and the exit code.
//
// Run reads os.Args, os.Stdin, os.Stdout and os.Stderr directly (the render
// layer writes to the process streams), so the test swaps them around the call.
// The tests here must not run in parallel.
func captureRun(t *testing.T, args []string, stdin string) (stdout, stderr string, code int) {
	t.Helper()

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	in := stdinFile(t, stdin)

	oldArgs, oldStdin, oldStdout, oldStderr := os.Args, os.Stdin, os.Stdout, os.Stderr
	defer func() {
		os.Args, os.Stdin, os.Stdout, os.Stderr = oldArgs, oldStdin, oldStdout, oldStderr
	}()

	os.Args = append([]string{"envsub"}, args...)
	os.Stdin = in
	os.Stdout = outW
	os.Stderr = errW

	outCh := make(chan []byte, 1)
	errCh := make(chan []byte, 1)
	go func() { data, _ := io.ReadAll(outR); outCh <- data }()
	go func() { data, _ := io.ReadAll(errR); errCh <- data }()

	code = Run()

	_ = outW.Close()
	_ = errW.Close()
	stdout = string(<-outCh)
	stderr = string(<-errCh)
	_ = outR.Close()
	_ = errR.Close()
	_ = in.Close()
	return stdout, stderr, code
}

// stdinFile returns a rewound file holding content, usable as os.Stdin.
func stdinFile(t *testing.T, content string) *os.File {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "stdin-*")
	if err != nil {
		t.Fatalf("create stdin: %v", err)
	}
	if _, err := file.WriteString(content); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("rewind stdin: %v", err)
	}
	return file
}

// writeFile writes a fixture file into dir and returns its path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// TestRunVersion pins the version subcommand path through the real command tree.
func TestRunVersion(t *testing.T) {
	stdout, stderr, code := captureRun(t, []string{"version"}, "")
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, `"gitVersion"`) {
		t.Fatalf("stdout = %q, want the build info JSON", stdout)
	}
}

// TestRunRenderInputs covers the input shapes: file, stdin, directory and glob.
func TestRunRenderInputs(t *testing.T) {
	dir := t.TempDir()
	tmplDir := filepath.Join(dir, "templates")
	if err := os.MkdirAll(tmplDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	a := writeFile(t, tmplDir, "a.tmpl", "A=[${A}]\n")
	writeFile(t, tmplDir, "b.tmpl", "B=[${B}]\n")
	nested := filepath.Join(tmplDir, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	writeFile(t, nested, "c.tmpl", "C=[${C}]\n")

	base := []string{"render", "--no-process-env", "--set", "A=1", "--set", "B=2"}
	dirOut := "A=[1]\nB=[2]\n"

	tests := []struct {
		name  string
		args  []string
		stdin string
		want  string
	}{
		{name: "file", args: append(append([]string{}, base...), a), want: "A=[1]\n"},
		{name: "stdin", args: append(append([]string{}, base...), "-"), stdin: "S=${A}\n", want: "S=1\n"},
		{name: "directory", args: append(append([]string{}, base...), tmplDir), want: dirOut},
		{name: "glob", args: append(append([]string{}, base...), filepath.Join(tmplDir, "*.tmpl")), want: dirOut},
		{
			name: "directory recursive",
			args: append(append([]string{}, base...), "--set", "C=3", "--recursive", tmplDir),
			want: dirOut + "C=[3]\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, code := captureRun(t, tt.args, tt.stdin)
			if code != 0 {
				t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr)
			}
			if stdout != tt.want {
				t.Fatalf("stdout = %q, want %q", stdout, tt.want)
			}
		})
	}
}

// TestRunRenderOutputs covers stdout, --output, --output-dir and --in-place.
func TestRunRenderOutputs(t *testing.T) {
	dir := t.TempDir()
	tmpl := writeFile(t, dir, "input.tmpl", "K=[${K}]\n")
	writeFile(t, dir, "second.tmpl", "L=[${K}]\n")
	base := []string{"render", "--no-process-env", "--set", "K=v"}

	t.Run("output file", func(t *testing.T) {
		out := filepath.Join(dir, "out.txt")
		stdout, stderr, code := captureRun(t, append(append([]string{}, base...), "-o", out, tmpl), "")
		if code != 0 {
			t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr)
		}
		if stdout != "" {
			t.Fatalf("stdout = %q, want empty for -o", stdout)
		}
		if got := readFile(t, out); got != "K=[v]\n" {
			t.Fatalf("output = %q, want %q", got, "K=[v]\n")
		}
	})

	t.Run("output dir", func(t *testing.T) {
		outDir := filepath.Join(dir, "batch")
		_, stderr, code := captureRun(t, append(append([]string{}, base...), "--output-dir", outDir, tmpl, filepath.Join(dir, "second.tmpl")), "")
		if code != 0 {
			t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr)
		}
		if got := readFile(t, filepath.Join(outDir, "input.tmpl")); got != "K=[v]\n" {
			t.Fatalf("batched input.tmpl = %q", got)
		}
		if got := readFile(t, filepath.Join(outDir, "second.tmpl")); got != "L=[v]\n" {
			t.Fatalf("batched second.tmpl = %q", got)
		}
	})

	t.Run("in place", func(t *testing.T) {
		target := writeFile(t, t.TempDir(), "inplace.tmpl", "K=[${K}]\n")
		_, stderr, code := captureRun(t, append(append([]string{}, base...), "--in-place", target), "")
		if code != 0 {
			t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr)
		}
		if got := readFile(t, target); got != "K=[v]\n" {
			t.Fatalf("in-place target = %q, want %q", got, "K=[v]\n")
		}
	})
}

// TestRunValidateOnly pins that validation writes nothing and reports schema
// failures with the type exit code.
func TestRunValidateOnly(t *testing.T) {
	dir := t.TempDir()
	tmpl := writeFile(t, dir, "input.tmpl", "K=${K}\n")

	_, stderr, code := captureRun(t, []string{"render", "--no-process-env", "--set", "K=v", "--validate-only", tmpl}, "")
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr)
	}
	if got := readFile(t, tmpl); got != "K=${K}\n" {
		t.Fatalf("--validate-only rewrote the template: %q", got)
	}

	schema := writeFile(t, dir, "schema.json", `{"type":"object","properties":{"K":{"type":"integer"}}}`)
	_, _, code = captureRun(t, []string{"render", "--no-process-env", "--schema", schema, "--set", "K=v", "--validate-only", tmpl}, "")
	if code != 4 {
		t.Fatalf("invalid type exit = %d, want 4", code)
	}
}

// TestRunExplain pins that the report is stderr-only and does not change stdout.
func TestRunExplain(t *testing.T) {
	dir := t.TempDir()
	tmpl := writeFile(t, dir, "input.tmpl", "K=${K}\n")
	stdout, stderr, code := captureRun(t, []string{"render", "--explain", "--no-process-env", "--set", "K=v", tmpl}, "")
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr)
	}
	if stdout != "K=v\n" {
		t.Fatalf("stdout = %q, want the rendered template", stdout)
	}
	if !strings.Contains(stderr, `"variable":"K","origin":{"kind":"set","method":"flag","source":"K"`) {
		t.Fatalf("stderr = %q, want the JSON explain report", stderr)
	}
}

// TestRunExitCodes pins the user-visible exit-code contract through the real
// command tree.
//
// Exit 1 is the internal code, and it is also what an unclassified error gets:
// an error that carries no diag kind falls back to it.
func TestRunExitCodes(t *testing.T) {
	dir := t.TempDir()
	ok := writeFile(t, dir, "ok.tmpl", "K=${K}\n")
	missing := writeFile(t, dir, "missing.tmpl", "${NO_SUCH_VAR}\n")
	broken := writeFile(t, dir, "broken.tmpl", "${UNCLOSED\n")
	schema := writeFile(t, dir, "schema.json", `{"type":"object","properties":{"PORT":{"type":"integer"}}}`)

	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "unknown flag", args: []string{"render", "--nope", ok}, want: 2},
		{name: "removed --values is unknown", args: []string{"render", "--values", ok}, want: 2},
		{name: "removed --env-prefix is unknown", args: []string{"render", "--env-prefix", "APP_", ok}, want: 2},
		{name: "bad on-missing", args: []string{"render", "--on-missing", "bogus", ok}, want: 2},
		{name: "env options conflict", args: []string{"render", "--no-process-env", "--process-env-prefix", "APP_", ok}, want: 2},
		{name: "output with output-dir", args: []string{"render", "-o", filepath.Join(dir, "x"), "--output-dir", filepath.Join(dir, "d"), ok}, want: 2},
		{name: "validate-only with output", args: []string{"render", "--validate-only", "-o", filepath.Join(dir, "x"), ok}, want: 2},
		{name: "missing template", args: []string{"render", filepath.Join(dir, "absent.tmpl")}, want: 5},
		{name: "template syntax", args: []string{"render", "--no-process-env", broken}, want: 3},
		{name: "missing value", args: []string{"render", "--no-process-env", missing}, want: 6},
		{name: "type mismatch", args: []string{"render", "--no-process-env", "--schema", schema, "--set", "PORT=eighty", ok}, want: 4},
		// A namespace is read-only: --set and --set-json accept root paths only.
		{name: "set rejects the process namespace", args: []string{"render", "--set", "@process.K=v", ok}, want: 2},
		{name: "set-json rejects the process namespace", args: []string{"render", "--set-json", "@process.K=1", ok}, want: 2},
		// A name that is not a subcommand is a usage error: the CLI reports
		// it as one, so the exit code is 2 and not the internal 1.
		{name: "unknown command", args: []string{"bogus"}, want: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, stderr, code := captureRun(t, tt.args, "")
			if code != tt.want {
				t.Fatalf("exit = %d, want %d; stderr=%q", code, tt.want, stderr)
			}
		})
	}
}

// TestRunUnknownCommand pins how a name that is not a subcommand is reported:
// the usage exit code, the wording, and the "Did you mean this?" list.
func TestRunUnknownCommand(t *testing.T) {
	_, stderr, code := captureRun(t, []string{"rendr"}, "")
	if code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%q", code, stderr)
	}
	for _, want := range []string{`unknown command "rendr" for "envsub"`, "Did you mean this?", "render"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want it to contain %q", stderr, want)
		}
	}
}

// TestRunUnknownCommandAsJSON pins that the usage error also follows the
// requested error format: the report names the kind and the exit code.
func TestRunUnknownCommandAsJSON(t *testing.T) {
	_, stderr, code := captureRun(t, []string{"--error-format=json", "bogus"}, "")
	if code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%q", code, stderr)
	}
	for _, want := range []string{`"kind":"usage"`, `"exitCode":2`, `unknown command \"bogus\"`} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want it to contain %q", stderr, want)
		}
	}
}

// TestRunEnvFileLayering pins the documented environment priority through the
// command tree: later --dotenv-file wins, and --set beats every .env file.
func TestRunDotenvFileLayering(t *testing.T) {
	dir := t.TempDir()
	tmpl := writeFile(t, dir, "input.tmpl", "K=${K}\n")
	base := writeFile(t, dir, "base.env", "K=base\n")
	debug := writeFile(t, dir, "debug.env", "K=debug\n")

	stdout, stderr, code := captureRun(t, []string{
		"render", "--no-process-env", "--dotenv-file", base, "--dotenv-file", debug, tmpl,
	}, "")
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr)
	}
	if stdout != "K=debug\n" {
		t.Fatalf("later .env did not win: %q", stdout)
	}

	stdout, stderr, code = captureRun(t, []string{
		"render", "--no-process-env", "--dotenv-file", base, "--set", "K=set", tmpl,
	}, "")
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr)
	}
	if stdout != "K=set\n" {
		t.Fatalf("--set did not beat .env: %q", stdout)
	}
}

// TestRunDiagnostics pins the user-visible error wording and template location.
func TestRunDiagnostics(t *testing.T) {
	dir := t.TempDir()

	missing := writeFile(t, dir, "missing.tmpl", "line1\n${NO_SUCH_VAR}\n")
	_, stderr, code := captureRun(t, []string{"render", "--no-process-env", missing}, "")
	if code != 6 {
		t.Fatalf("missing value exit = %d, want 6; stderr=%q", code, stderr)
	}
	if !strings.Contains(stderr, `variable "NO_SUCH_VAR" is not set`) {
		t.Fatalf("stderr = %q, want the missing-variable wording", stderr)
	}

	broken := writeFile(t, dir, "broken.tmpl", "ok=text\nbad=${B\n")
	_, stderr, code = captureRun(t, []string{"render", "--no-process-env", broken}, "")
	if code != 3 {
		t.Fatalf("template error exit = %d, want 3; stderr=%q", code, stderr)
	}
	if !strings.Contains(stderr, "broken.tmpl:2") {
		t.Fatalf("stderr = %q, want the template error location", stderr)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
