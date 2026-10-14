package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const (
	caseInputFile    = "input.tmpl"
	caseEnvFile      = "env"
	caseEnvListFile  = "process.env"
	caseExpectFile   = "expect.out"
	caseErrorFile    = "expect.err"
	caseExitFile     = "expect.exit"
	caseSpecialFile  = "special"
	caseArgsFile     = "args"
	caseExpectDir    = "expect.dir"
	caseResultSubdir = "result"
)

var binaryPath string

func TestMain(m *testing.M) {
	root, err := repositoryRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	dir, err := os.MkdirTemp("", "envsub-e2e-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	binaryPath = filepath.Join(dir, "envsub")
	build := exec.Command("go", "build", "-trimpath", "-ldflags", "-s -w", "-o", binaryPath, "./cmd/envsub")
	build.Dir = root
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build e2e binary: %v\n%s", err, out)
		_ = os.RemoveAll(dir)
		os.Exit(1)
	}

	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func TestE2E(t *testing.T) {
	t.Parallel()
	dataDir, err := testdataRoot()
	if err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dataDir)
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		entry := entry
		t.Run(entry.Name(), func(t *testing.T) {
			t.Parallel()
			runCase(t, filepath.Join(dataDir, entry.Name()))
		})
	}
}

// TestExplainStdoutUnchanged pins the central --explain contract: the report is
// an extra stderr channel, so stdout stays byte-for-byte identical to a render
// without it (docs/07-serialization/13-materialization.md §15).
func TestExplainStdoutUnchanged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "input.tmpl")
	const content = "A=${A | toJson} B=${B} C=${C:-fallback}\n"
	if err := os.WriteFile(tmpl, []byte(content), 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}
	dotenv := filepath.Join(dir, "app.env")
	if err := os.WriteFile(dotenv, []byte("B=from_file\n"), 0o600); err != nil {
		t.Fatalf("write dotenv: %v", err)
	}

	base := []string{"render", "--dotenv-file", dotenv, "--set", "A=8080", tmpl}
	plain, _, status := execute(t, dir, nil, base)
	if status != 0 {
		t.Fatalf("render exit = %d, want 0", status)
	}
	explained, stderr, status := execute(t, dir, nil, append([]string{base[0], "--explain"}, base[1:]...))
	if status != 0 {
		t.Fatalf("render --explain exit = %d, want 0; stderr=%q", status, stderr)
	}
	if explained != plain {
		t.Fatalf("--explain changed stdout: got %q, want %q", explained, plain)
	}
	for _, want := range []string{
		`"variable":"A","origin":{"kind":"set","method":"flag","source":"A"`,
		`"variable":"B","origin":{"kind":"envfile","method":"file","source":"` + dotenv + `"`,
		`"variable":"C","origin":{"kind":"missing","method":"missing"`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("--explain report = %q, want it to contain %q", stderr, want)
		}
	}
}

// TestExplainValidateOnly pins that --explain and --validate-only combine: the
// report is written, stdout stays empty and the exit code is unaffected.
func TestExplainValidateOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "input.tmpl")
	if err := os.WriteFile(tmpl, []byte("A=${A}\n"), 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}
	stdout, stderr, status := execute(t, dir, nil, []string{
		"render", "--explain", "--validate-only", "--set", "A=1", tmpl,
	})
	if status != 0 {
		t.Fatalf("render exit = %d, want 0; stderr=%q", status, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty with --validate-only", stdout)
	}
	if !strings.Contains(stderr, `"variable":"A","origin":{"kind":"set","method":"flag","source":"A"`) {
		t.Fatalf("--explain report = %q, want the set frame for A", stderr)
	}
}

func runCase(t *testing.T, dir string) {
	t.Helper()
	// Cases with an args file run inside a writable copy of the case
	// directory so relative paths, in-place rendering and batch output can be
	// exercised end to end.
	if args, ok := readArgsFile(t, filepath.Join(dir, caseArgsFile)); ok {
		runCaseWithArgs(t, dir, args)
		return
	}

	inputPath := filepath.Join(dir, caseInputFile)
	if _, err := os.Stat(inputPath); err != nil {
		t.Fatalf("stat input: %v", err)
	}
	runLegacyCase(t, dir, inputPath)
}

// caseModifiedFile marks a case whose args rewrite an existing file in place
// rather than producing new output.
const caseModifiedFile = "modified"

// runLegacyCase keeps the original env + single-template behavior.
func runLegacyCase(t *testing.T, dir, inputPath string) {
	t.Helper()
	env, err := readEnvFile(filepath.Join(dir, caseEnvFile))
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	if env == nil {
		env = []string{}
	}

	special, _ := readOptionalString(filepath.Join(dir, caseSpecialFile))
	outputMode := special == "output_file"
	outFile := ""
	args := []string{"render", inputPath}
	if outputMode {
		outFile = filepath.Join(t.TempDir(), "out.txt")
		args = []string{"render", "-o", outFile, inputPath}
	}

	stdout, stderr, status := execute(t, dir, env, args)
	wantExit, hasWantExit := readOptionalInt(t, filepath.Join(dir, caseExitFile))
	if hasWantExit {
		if status != wantExit {
			t.Fatalf("exit = %d, want %d; stderr=%q", status, wantExit, stderr)
		}
		assertStderrContains(t, dir, stderr)
		return
	}
	if status != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", status, stderr)
	}

	got := stdout
	if outputMode {
		data, err := os.ReadFile(outFile)
		if err != nil {
			t.Fatalf("read output file: %v", err)
		}
		got = string(data)
	}
	want := readFile(t, filepath.Join(dir, caseExpectFile))
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

// runCaseWithArgs runs a case from a copied working directory using the flags
// listed in its args file.
func runCaseWithArgs(t *testing.T, dir string, args []string) {
	t.Helper()
	work := t.TempDir()
	copyDirectory(t, dir, work)
	// The input is optional: batch and in-place cases may only ship values,
	// schema and template directories. When present it is made read-only for
	// the owner-write path so in-place rendering must replace it.
	inputPath := filepath.Join(work, caseInputFile)
	if _, err := os.Stat(inputPath); err == nil {
		if err := os.Chmod(inputPath, 0o600); err != nil {
			t.Fatalf("chmod input: %v", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat input: %v", err)
	}

	env, err := readEnvFile(filepath.Join(work, caseEnvFile))
	if err != nil {
		t.Fatalf("read env: %v", err)
	}

	repoRoot, err := repositoryRoot()
	if err != nil {
		t.Fatalf("repository root: %v", err)
	}
	resolved := make([]string, 0, len(args))
	for _, arg := range args {
		resolved = append(resolved, resolveCaseArg(t, work, repoRoot, arg))
	}

	stdout, stderr, status := execute(t, work, env, resolved)
	wantExit, hasWantExit := readOptionalInt(t, filepath.Join(work, caseExitFile))
	if hasWantExit {
		if status != wantExit {
			t.Fatalf("exit = %d, want %d; stderr=%q", status, wantExit, stderr)
		}
		assertStderrContains(t, work, stderr)
		return
	}
	if status != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", status, stderr)
	}
	// A successful case may still pin a stderr substring, for example the
	// --explain report, which must not disturb the exact stdout comparison.
	assertStderrContains(t, work, stderr)

	expectDir := filepath.Join(work, caseExpectDir)
	if info, err := os.Stat(expectDir); err == nil && info.IsDir() {
		if _, err := os.Stat(filepath.Join(work, caseModifiedFile)); err == nil {
			assertDirectory(t, expectDir, work)
			return
		}
		assertDirectory(t, expectDir, filepath.Join(work, caseResultSubdir))
		return
	}

	want := readFile(t, filepath.Join(work, caseExpectFile))
	if stdout != want {
		t.Fatalf("output = %q, want %q", stdout, want)
	}
}

// resolveCaseArg maps a case argument to the file it names.
//
// A leading @ anchors the path at the working directory copy. Otherwise a
// relative path that does not exist in the work directory is resolved against
// the repository root, so a case can reuse checked-in assets such as
// examples/.
func resolveCaseArg(t *testing.T, work, repoRoot, arg string) string {
	t.Helper()
	if after, ok := strings.CutPrefix(arg, "@"); ok {
		return filepath.Join(work, after)
	}
	if strings.HasPrefix(arg, "-") || filepath.IsAbs(arg) {
		return arg
	}
	if _, err := os.Stat(filepath.Join(work, arg)); err == nil {
		return arg
	}
	candidate := filepath.Join(repoRoot, arg)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return arg
}

// execute runs the CLI binary with the given working directory and
// environment. A process.env file lists KEY=VALUE pairs to add.
func execute(t *testing.T, workDir string, env []string, args []string) (stdout, stderr string, status int) {
	t.Helper()
	extra, err := readEnvFile(filepath.Join(workDir, caseEnvListFile))
	if err != nil {
		t.Fatalf("read process env: %v", err)
	}
	cmd := exec.CommandContext(context.Background(), binaryPath, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), env...)
	cmd.Env = append(cmd.Env, extra...)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), exitCode(err)
}

// assertStderrContains checks the optional expect.err substring.
func assertStderrContains(t *testing.T, dir, stderr string) {
	t.Helper()
	wantErr, err := readOptionalString(filepath.Join(dir, caseErrorFile))
	if err != nil || wantErr == "" {
		return
	}
	if !strings.Contains(stderr, wantErr) {
		t.Fatalf("stderr = %q, want it to contain %q", stderr, wantErr)
	}
}

// assertDirectory compares every file under gotDir with wantDir.
func assertDirectory(t *testing.T, wantDir, gotDir string) {
	t.Helper()
	var wantFiles []string
	err := filepath.WalkDir(wantDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(wantDir, path)
		if err != nil {
			return err
		}
		wantFiles = append(wantFiles, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walk expect dir: %v", err)
	}
	if len(wantFiles) == 0 {
		t.Fatalf("expect.dir %s has no files", wantDir)
	}
	for _, rel := range wantFiles {
		want := readFile(t, filepath.Join(wantDir, rel))
		got := readFile(t, filepath.Join(gotDir, rel))
		if got != want {
			t.Fatalf("%s = %q, want %q", rel, got, want)
		}
	}
}

// copyDirectory copies a case directory into a writable location.
func copyDirectory(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
	if err != nil {
		t.Fatalf("copy case directory: %v", err)
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		_ = in.Close()
	}()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// readArgsFile reads one CLI argument per line, ignoring blanks and comments.
func readArgsFile(t *testing.T, path string) ([]string, bool) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false
		}
		t.Fatalf("read args: %v", err)
	}
	var args []string
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		args = append(args, line)
	}
	if len(args) == 0 {
		t.Fatalf("args file %s is empty", path)
	}
	return args, true
}

func readEnvFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var env []string
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("invalid env line %q", line)
		}
		env = append(env, key+"="+value)
	}
	return env, nil
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func readOptionalString(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func readOptionalInt(t *testing.T, path string) (int, bool) {
	t.Helper()
	data, err := readOptionalString(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, false
		}
		t.Fatalf("read %s: %v", path, err)
	}
	value, err := strconv.Atoi(data)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return value, true
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitErr.ExitCode()
	}
	return -1
}

func repositoryRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..")), nil
}

func testdataRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "testdata"), nil
}

// TestExplainFramesFollowADR005 pins the report's structure against the real
// binary: every frame carries kind/method/source/file/line, kinds and methods
// come from the closed sets, a file-backed frame names its file as the address,
// and the lineage lists the layers a value came through, lowest priority last —
// including a schema default a later layer covered.
func TestExplainFramesFollowADR005(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("testdata", "schema_default_is_fallback")
	args, ok := readArgsFile(t, filepath.Join(dir, caseArgsFile))
	if !ok {
		t.Fatalf("%s has no %s", dir, caseArgsFile)
	}
	env, err := readEnvFile(filepath.Join(dir, caseEnvFile))
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	_, stderr, status := execute(t, dir, env, args)
	if status != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", status, stderr)
	}

	type frame struct {
		Kind   string `json:"kind"`
		Method string `json:"method"`
		Source string `json:"source"`
		File   string `json:"file"`
		Line   int    `json:"line"`
	}
	type record struct {
		Variable string  `json:"variable"`
		Origin   frame   `json:"origin"`
		Shadowed []frame `json:"shadowed"`
	}

	kinds := map[string]bool{
		"schema": true, "values": true, "envfile": true,
		"set": true, "process": true, "assign": true, "missing": true,
	}
	methods := map[string]bool{
		"default": true, "file": true, "flag": true, "json": true,
		"env": true, "assign": true, "missing": true,
	}

	var reports []record
	for _, line := range strings.Split(strings.TrimSpace(stderr), "\n") {
		if line == "" {
			continue
		}
		var decoded record
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Fatalf("decode explain line %q: %v", line, err)
		}
		reports = append(reports, decoded)
	}
	if len(reports) != 2 {
		t.Fatalf("explain lines = %d, want 2: %q", len(reports), stderr)
	}

	check := func(where string, f frame) {
		t.Helper()
		if !kinds[f.Kind] {
			t.Errorf("%s kind = %q, want a member of the closed set", where, f.Kind)
		}
		if !methods[f.Method] {
			t.Errorf("%s method = %q, want a member of the closed set", where, f.Method)
		}
		// values and envfile frames are located by the file they came from; the
		// other kinds carry a pointer, a name or a key instead (ADR-005 I3).
		if f.Kind == "values" || f.Kind == "envfile" {
			if f.File == "" || f.Source != f.File {
				t.Errorf("%s source = %q file = %q, want the file as the address", where, f.Source, f.File)
			}
		}
	}

	for _, report := range reports {
		check(report.Variable+".origin", report.Origin)
		for i, f := range report.Shadowed {
			check(fmt.Sprintf("%s.shadowed[%d]", report.Variable, i), f)
		}
	}

	// K: the .env layer wins, and the schema default it covered is still in the
	// lineage, at the end.
	k := reports[0]
	if k.Origin.Kind != "envfile" || k.Origin.Line != 1 {
		t.Errorf("K origin = %+v, want the envfile line 1", k.Origin)
	}
	if len(k.Shadowed) != 1 || k.Shadowed[0].Kind != "schema" {
		t.Fatalf("K shadowed = %+v, want the schema default", k.Shadowed)
	}
	if want := "#/properties/K/default"; k.Shadowed[0].Source != want {
		t.Errorf("K shadowed source = %q, want %q", k.Shadowed[0].Source, want)
	}

	// D: only the schema declares it, so the default is the whole lineage.
	d := reports[1]
	if d.Origin.Kind != "schema" || d.Origin.Method != "default" || d.Origin.Source != "#/properties/D/default" {
		t.Errorf("D origin = %+v, want the schema default pointer", d.Origin)
	}
	if len(d.Shadowed) != 0 {
		t.Errorf("D shadowed = %+v, want none", d.Shadowed)
	}
}

// TestExplainReportsAssignmentOrigins pins the assign frame: a value a template
// wrote with := or = is reported with the template and line that wrote it, which
// is why the report runs after the render loop (ADR-005).
func TestExplainReportsAssignmentOrigins(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "input.tmpl")
	if err := os.WriteFile(tmpl, []byte("first=${K:=written}\nsecond=${K}\n"), 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}
	stdout, stderr, status := execute(t, dir, nil, []string{"render", "--explain", "--no-process-env", tmpl})
	if status != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", status, stderr)
	}
	if stdout != "first=written\nsecond=written\n" {
		t.Fatalf("stdout = %q, want both references resolved", stdout)
	}
	want := `"origin":{"kind":"set","method":"assign","source":"` + tmpl + `","file":"` + tmpl + `","line":1}`
	if !strings.Contains(stderr, want) {
		t.Fatalf("--explain report = %q, want it to contain %q", stderr, want)
	}
}

// TestExplainReportsBeforeAFailure pins that a failing render still explains what
// it resolved: the report is the diagnostic, so it is written before the error
// is returned.
func TestExplainReportsBeforeAFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "input.tmpl")
	if err := os.WriteFile(tmpl, []byte("ok=${K:=written}\nmissing=${NO_SUCH_VAR}\n"), 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}
	_, stderr, status := execute(t, dir, nil, []string{"render", "--explain", "--no-process-env", tmpl})
	if status != 6 {
		t.Fatalf("exit = %d, want 6 (missing); stderr=%q", status, stderr)
	}
	for _, want := range []string{
		`"variable":"K","origin":{"kind":"set","method":"assign"`,
		`"variable":"NO_SUCH_VAR","origin":{"kind":"missing","method":"missing"`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("--explain report = %q, want it to contain %q", stderr, want)
		}
	}
}

// TestLiteralDottedKeyIsDistinctFromNestedPath pins the identity contract that
// replaced the encoded key: a value file may define both the nested path a.b.c
// and the literal key "a.b.c", and each keeps its own origin, its own filter
// selection and its own unused verdict.
func TestLiteralDottedKeyIsDistinctFromNestedPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	nested := filepath.Join(dir, "nested.yaml")
	dotted := filepath.Join(dir, "dotted.json")
	tmpl := filepath.Join(dir, "input.tmpl")
	if err := os.WriteFile(nested, []byte("a:\n  b:\n    c: nested-value\n"), 0o600); err != nil {
		t.Fatalf("write nested values: %v", err)
	}
	if err := os.WriteFile(dotted, []byte(`{"a.b.c":"dotted-value","d.b.c":"dead-value"}`), 0o600); err != nil {
		t.Fatalf("write dotted values: %v", err)
	}
	if err := os.WriteFile(tmpl, []byte("${a.b.c} ${[\"a.b.c\"]}\n"), 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}

	// The report must name the file each value actually came from: the nested path
	// from the YAML file, the literal key from the JSON file, nothing shadowed.
	stdout, stderr, status := execute(t, dir, nil, []string{
		"render", "--no-process-env", "--explain", "-f", nested, "-f", dotted, tmpl,
	})
	if status != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", status, stderr)
	}
	if stdout != "nested-value dotted-value\n" {
		t.Fatalf("stdout = %q, want both values", stdout)
	}
	for _, want := range []string{
		`"variable":"a.b.c","origin":{"kind":"values","method":"file","source":"` + nested + `"`,
		`"variable":"[\"a.b.c\"]","origin":{"kind":"values","method":"file","source":"` + dotted + `"`,
		`"variable":"a.b.c","origin":{"kind":"values","method":"file","source":"` + nested + `","file":"` + nested + `","line":0},"shadowed":[]`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("--explain report = %q, want it to contain %q", stderr, want)
		}
	}

	// A dotted pattern selects the nested path only; the literal key needs its own
	// (quoted) spelling.
	stdout, _, status = execute(t, dir, nil, []string{
		"render", "--no-process-env", "--only-vars", "a.b.c", "-f", nested, "-f", dotted, tmpl,
	})
	if status != 0 {
		t.Fatalf("exit = %d, want 0", status)
	}
	if stdout != `nested-value ${["a.b.c"]}`+"\n" {
		t.Fatalf("stdout = %q, want the literal key left unselected", stdout)
	}

	// The literal key no template referenced is dead configuration, so the unused
	// report must list it (d.b.c too).
	used := filepath.Join(dir, "used.tmpl")
	if err := os.WriteFile(used, []byte("${a.b.c}\n"), 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}
	_, stderr, status = execute(t, dir, nil, []string{
		"render", "--no-process-env", "--report", "unused", "-f", nested, "-f", dotted, used,
	})
	if status != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", status, stderr)
	}
	for _, want := range []string{`["a.b.c"]`, `["d.b.c"]`} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("unused report = %q, want it to list %q", stderr, want)
		}
	}
}
