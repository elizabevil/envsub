package e2e_test

import (
	"bytes"
	"context"
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
		`"variable":"A","source":"set"`,
		`"variable":"B","source":"` + dotenv + `:1"`,
		`"variable":"C","source":"missing"`,
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
	if !strings.Contains(stderr, `"variable":"A","source":"set"`) {
		t.Fatalf("--explain report = %q, want it to contain %q", stderr, "A set")
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
