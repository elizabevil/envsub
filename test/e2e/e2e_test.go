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
	"slices"
	"strconv"
	"strings"
	"testing"

	eventpkg "github.com/elizabevil/envsub/internal/event"
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

// assertStderrContains checks the optional expect.err. A file made only of JSON
// objects is compared as ordered JSONL events with time/seq normalized, so a
// protocol event can be asserted without freezing its clock. Anything else stays
// a plain substring, which is what the human-readable error fixtures use.
func assertStderrContains(t *testing.T, dir, stderr string) {
	t.Helper()
	wantErr, err := readOptionalString(filepath.Join(dir, caseErrorFile))
	if err != nil || wantErr == "" {
		return
	}
	if wantEvents, ok := parseEventLines(wantErr); ok {
		gotEvents, ok := parseEventLines(stderr)
		if !ok {
			t.Fatalf("stderr is not JSONL:\n%s", stderr)
		}
		if len(gotEvents) != len(wantEvents) {
			t.Fatalf("stderr has %d events, want %d:\n%s", len(gotEvents), len(wantEvents), stderr)
		}
		for i := range wantEvents {
			normalizeEvent(wantEvents[i])
			normalizeEvent(gotEvents[i])
			wantJSON, _ := json.Marshal(wantEvents[i])
			gotJSON, _ := json.Marshal(gotEvents[i])
			if string(gotJSON) != string(wantJSON) {
				t.Fatalf("event %d = %s, want %s", i, gotJSON, wantJSON)
			}
		}
		return
	}
	if !strings.Contains(stderr, wantErr) {
		t.Fatalf("stderr = %q, want it to contain %q", stderr, wantErr)
	}
}

// parseEventLines decodes a blob that is only JSON objects, one per line. It
// returns false when the blob is a text expectation, so the caller can fall back
// to a substring check.
func parseEventLines(blob string) ([]map[string]any, bool) {
	lines := strings.Split(strings.TrimSpace(blob), "\n")
	if len(lines) == 0 {
		return nil, false
	}
	events := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "{") {
			return nil, false
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, false
		}
		events = append(events, event)
	}
	return events, len(events) > 0
}

// normalizeEvent drops the fields that depend on the wall clock or run order;
// protocol tests assert their presence separately.
func normalizeEvent(event map[string]any) {
	delete(event, "time")
	delete(event, "seq")
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
		Schema int    `json:"schema"`
		Type   string `json:"type"`
		Time   string `json:"time"`
		Seq    uint64 `json:"seq"`
		Data   struct {
			Variable string  `json:"variable"`
			Origin   frame   `json:"origin"`
			Shadowed []frame `json:"shadowed"`
		} `json:"data"`
	}

	kinds := map[string]bool{
		"schema": true, "values": true, "envfile": true,
		"set": true, "process": true, "assign": true, "missing": true,
	}
	methods := map[string]bool{
		"default": true, "file": true, "flag": true, "json": true,
		"env": true, "assign": true, "missing": true,
	}
	// I2 is a pairing, not two independent closed sets: a method is valid only
	// for the kinds that accept it (ADR-005 §4.5).
	allowedMethods := map[string]map[string]bool{
		"schema":  {"default": true},
		"values":  {"file": true},
		"envfile": {"file": true},
		"set":     {"flag": true, "json": true, "file": true},
		"process": {"env": true},
		"assign":  {"assign": true},
		"missing": {"missing": true},
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
		if !allowedMethods[f.Kind][f.Method] {
			t.Errorf("%s frame = %s/%s, want a method allowed for that kind", where, f.Kind, f.Method)
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
		if report.Schema != 1 || report.Type != eventpkg.TypeExplain || report.Time == "" || report.Seq == 0 {
			t.Errorf("envelope = schema %d type %q time %q seq %d, want a complete explain event", report.Schema, report.Type, report.Time, report.Seq)
		}
		check(report.Data.Variable+".origin", report.Data.Origin)
		for i, f := range report.Data.Shadowed {
			check(fmt.Sprintf("%s.shadowed[%d]", report.Data.Variable, i), f)
		}
	}

	// K: the .env layer wins, and the schema default it covered is still in the
	// lineage, at the end.
	k := reports[0]
	if k.Data.Origin.Kind != "envfile" || k.Data.Origin.Line != 1 {
		t.Errorf("K origin = %+v, want the envfile line 1", k.Data.Origin)
	}
	if len(k.Data.Shadowed) != 1 || k.Data.Shadowed[0].Kind != "schema" {
		t.Fatalf("K shadowed = %+v, want the schema default", k.Data.Shadowed)
	}
	if want := "#/properties/K/default"; k.Data.Shadowed[0].Source != want {
		t.Errorf("K shadowed source = %q, want %q", k.Data.Shadowed[0].Source, want)
	}

	// D: only the schema declares it, so the default is the whole lineage.
	d := reports[1]
	if d.Data.Origin.Kind != "schema" || d.Data.Origin.Method != "default" || d.Data.Origin.Source != "#/properties/D/default" {
		t.Errorf("D origin = %+v, want the schema default pointer", d.Data.Origin)
	}
	if len(d.Data.Shadowed) != 0 {
		t.Errorf("D shadowed = %+v, want none", d.Data.Shadowed)
	}
}

// TestExplainShadowedOrder pins the order of a multi-frame shadowed list
// through the real binary: the closest loser comes first and the lowest
// priority layer comes last, so "shadowed followed by origin" is the whole
// lineage.
//
// The e2e harness compares expect.err as a substring, which cannot pin the
// order of a JSON array, so the order is asserted here on the decoded report.
func TestExplainShadowedOrder(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("testdata", "explain_shadowed_order")
	args, ok := readArgsFile(t, filepath.Join(dir, caseArgsFile))
	if !ok {
		t.Fatalf("%s has no %s", dir, caseArgsFile)
	}
	_, stderr, status := execute(t, dir, nil, args)
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
		Schema int    `json:"schema"`
		Type   string `json:"type"`
		Time   string `json:"time"`
		Seq    uint64 `json:"seq"`
		Data   struct {
			Variable string  `json:"variable"`
			Origin   frame   `json:"origin"`
			Shadowed []frame `json:"shadowed"`
		} `json:"data"`
	}

	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	if len(lines) != 1 {
		t.Fatalf("explain lines = %d, want 1: %q", len(lines), stderr)
	}
	var report record
	if err := json.Unmarshal([]byte(lines[0]), &report); err != nil {
		t.Fatalf("decode explain line %q: %v", lines[0], err)
	}
	if report.Schema != 1 || report.Type != eventpkg.TypeExplain || report.Time == "" || report.Seq == 0 {
		t.Fatalf("envelope = schema %d type %q time %q seq %d, want a complete explain event", report.Schema, report.Type, report.Time, report.Seq)
	}

	// --set is the highest layer, so it wins.
	if report.Data.Origin.Kind != "set" || report.Data.Origin.Method != "flag" {
		t.Fatalf("origin = %+v, want the set frame", report.Data.Origin)
	}
	// The losers follow in reverse priority order: the closest one first and
	// the lowest layer last.
	want := []frame{
		{Kind: "envfile", Method: "file", Source: "debug.env", File: "debug.env", Line: 1},
		{Kind: "envfile", Method: "file", Source: "base.env", File: "base.env", Line: 1},
		{Kind: "values", Method: "file", Source: "values.yaml", File: "values.yaml", Line: 0},
		{Kind: "schema", Method: "default", Source: "#/properties/K/default", File: "schema.json", Line: 0},
	}
	if len(report.Data.Shadowed) != len(want) {
		t.Fatalf("shadowed = %+v, want %d frames", report.Data.Shadowed, len(want))
	}
	for i, expected := range want {
		if report.Data.Shadowed[i] != expected {
			t.Errorf("shadowed[%d] = %+v, want %+v", i, report.Data.Shadowed[i], expected)
		}
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
	want := `"origin":{"kind":"assign","method":"assign","source":"` + tmpl + `","file":"` + tmpl + `","line":1}`
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
		`"variable":"K","origin":{"kind":"assign","method":"assign"`,
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
	got := unusedPaths(t, stderr)
	for _, want := range []string{`["a.b.c"]`, `["d.b.c"]`} {
		if !slices.Contains(got, want) {
			t.Fatalf("unused report = %q, want it to list %q", stderr, want)
		}
	}
}

// unusedPaths decodes the type=unused records of a JSONL stderr stream.
func unusedPaths(t *testing.T, stderr string) []string {
	t.Helper()
	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(stderr), "\n") {
		if line == "" {
			continue
		}
		var event struct {
			Type string `json:"type"`
			Data struct {
				Path string `json:"path"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode stderr line %q: %v", line, err)
		}
		if event.Type == eventpkg.TypeUnused {
			paths = append(paths, event.Data.Path)
		}
	}
	return paths
}

// TestExplainReportsAssignmentOverAnEmptySource pins E1 with a value source that
// is present but empty: := still writes, so the assignment frame must close the
// chain and the empty source must appear as shadowed.
func TestExplainReportsAssignmentOverAnEmptySource(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "input.tmpl")
	if err := os.WriteFile(tmpl, []byte("first=${K:=written}\n"), 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}
	stdout, stderr, status := execute(t, dir, nil, []string{
		"render", "--no-process-env", "--set", "K=", "--explain", tmpl,
	})
	if status != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", status, stderr)
	}
	if stdout != "first=written\n" {
		t.Fatalf("stdout = %q, want the assignment to replace the empty source", stdout)
	}
	for _, want := range []string{
		`"variable":"K","origin":{"kind":"assign","method":"assign"`,
		`"shadowed":[{"kind":"set","method":"flag","source":"K"`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("report = %q, want it to contain %q", stderr, want)
		}
	}
}

// TestExplainOmitsUnreachableSchemaDefault pins E2: the schema declares
// app.port, but app does not exist, so ApplyDefaults never fills it and the
// report must say missing instead of naming the declaration.
func TestExplainOmitsUnreachableSchemaDefault(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	schema := filepath.Join(dir, "schema.json")
	if err := os.WriteFile(schema, []byte(
		`{"type":"object","properties":{"app":{"type":"object","properties":{"port":{"type":"integer","default":8080}}}}}`,
	), 0o600); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	tmpl := filepath.Join(dir, "input.tmpl")
	if err := os.WriteFile(tmpl, []byte("${app.port}\n"), 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}
	stdout, stderr, status := execute(t, dir, nil, []string{
		"render", "--no-process-env", "--schema", schema, "--on-missing=keep", "--explain", tmpl,
	})
	if status != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", status, stderr)
	}
	if stdout != "${app.port}\n" {
		t.Fatalf("stdout = %q, want the unreachable default not to fill the value", stdout)
	}
	if strings.Contains(stderr, `"kind":"schema"`) {
		t.Fatalf("report = %q, want no schema default frame", stderr)
	}
	if !strings.Contains(stderr, `"variable":"app.port","origin":{"kind":"missing"`) {
		t.Fatalf("report = %q, want app.port reported missing", stderr)
	}
}

// TestExplainSkipsFilteredVariables pins E4: --only-vars leaves a reference
// untouched, so it never resolved and must not be reported as a set value.
func TestExplainSkipsFilteredVariables(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "input.tmpl")
	if err := os.WriteFile(tmpl, []byte("${B}\n"), 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}
	stdout, stderr, status := execute(t, dir, nil, []string{
		"render", "--no-process-env", "--set", "B=from-set", "--only-vars", "A", "--explain", tmpl,
	})
	if status != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", status, stderr)
	}
	if stdout != "${B}\n" {
		t.Fatalf("stdout = %q, want B left unsubstituted", stdout)
	}
	if strings.Contains(stderr, `"variable":"B"`) {
		t.Fatalf("report = %q, want the excluded variable omitted", stderr)
	}
}

// TestExplainReportsNestedDefaultWord pins E5: a reference that only appears in
// an operator's fallback word is rendered as its own template, so it must be
// enumerated and reported like any other reference.
func TestExplainReportsNestedDefaultWord(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "input.tmpl")
	if err := os.WriteFile(tmpl, []byte("${A:-${B}}\n"), 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}
	stdout, stderr, status := execute(t, dir, nil, []string{
		"render", "--no-process-env", "--set", "B=from-set", "--explain", tmpl,
	})
	if status != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", status, stderr)
	}
	if stdout != "from-set\n" {
		t.Fatalf("stdout = %q, want the fallback word rendered", stdout)
	}
	for _, want := range []string{
		`"variable":"A","origin":{"kind":"missing"`,
		`"variable":"B","origin":{"kind":"set","method":"flag"`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("report = %q, want it to contain %q", stderr, want)
		}
	}
}

// TestExplainKeepsMaterializedChildOrigin pins the E3 repair: schema
// materialization creates app.port from a JSON string with no meta of its own,
// so the report must fall back to app's file origin when the value exists.
func TestExplainKeepsMaterializedChildOrigin(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	schema := filepath.Join(dir, "schema.json")
	if err := os.WriteFile(schema, []byte(
		`{"type":"object","properties":{"app":{"type":"object","properties":{"port":{"type":"integer"}}}}}`,
	), 0o600); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	valuesFile := filepath.Join(dir, "values.yaml")
	if err := os.WriteFile(valuesFile, []byte("app: '{\"port\": 8080}'\n"), 0o600); err != nil {
		t.Fatalf("write values: %v", err)
	}
	tmpl := filepath.Join(dir, "input.tmpl")
	if err := os.WriteFile(tmpl, []byte("${app.port}\n"), 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}
	stdout, stderr, status := execute(t, dir, nil, []string{
		"render", "--no-process-env", "--schema", schema, "-f", valuesFile, "--explain", tmpl,
	})
	if status != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", status, stderr)
	}
	if stdout != "8080\n" {
		t.Fatalf("stdout = %q, want the materialized value", stdout)
	}
	if !strings.Contains(stderr, `"origin":{"kind":"values","method":"file","source":"`+valuesFile+`"`) {
		t.Fatalf("report = %q, want the file origin inherited from app", stderr)
	}
}

// TestExplainDoesNotInheritAncestorForMissingChild pins E3's original symptom:
// app.port exists, app.port.x does not, so the missing child must not borrow its
// parent's file origin.
func TestExplainDoesNotInheritAncestorForMissingChild(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	valuesFile := filepath.Join(dir, "values.yaml")
	if err := os.WriteFile(valuesFile, []byte("app:\n  port: 1\n"), 0o600); err != nil {
		t.Fatalf("write values: %v", err)
	}
	tmpl := filepath.Join(dir, "input.tmpl")
	if err := os.WriteFile(tmpl, []byte("${app.port.x}\n"), 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}
	_, stderr, status := execute(t, dir, nil, []string{
		"render", "--no-process-env", "-f", valuesFile, "--on-missing=keep", "--explain", tmpl,
	})
	if status != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", status, stderr)
	}
	if !strings.Contains(stderr, `"variable":"app.port.x","origin":{"kind":"missing"`) {
		t.Fatalf("report = %q, want the missing child reported missing", stderr)
	}
}

// TestExplainAttributesParentDefaultObject pins the other half of E2: when an
// ancestor default supplies an object literal, a child declaration that the
// traversal never applied must not claim the value.
func TestExplainAttributesParentDefaultObject(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	schema := filepath.Join(dir, "schema.json")
	if err := os.WriteFile(schema, []byte(
		`{"type":"object","properties":{"app":{"type":"object","default":{"port":1},"properties":{"port":{"type":"integer","default":2}}}}}`,
	), 0o600); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	tmpl := filepath.Join(dir, "input.tmpl")
	if err := os.WriteFile(tmpl, []byte("${app.port}\n"), 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}
	stdout, stderr, status := execute(t, dir, nil, []string{
		"render", "--no-process-env", "--schema", schema, "--explain", tmpl,
	})
	if status != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", status, stderr)
	}
	if stdout != "1\n" {
		t.Fatalf("stdout = %q, want the parent default's literal", stdout)
	}
	if !strings.Contains(stderr, `"origin":{"kind":"schema","method":"default","source":"#/properties/app/default"`) {
		t.Fatalf("report = %q, want the parent default declaration", stderr)
	}
	if strings.Contains(stderr, `#/properties/app/properties/port/default`) {
		t.Fatalf("report = %q, want the unapplied child declaration omitted", stderr)
	}
}

// TestExplainSkipsNestedReferenceUnderExcludedParent pins the E4/E5 interaction:
// --ignore-vars A leaves the whole ${A:-${B}} token literal, so B was never
// substituted either and must not be reported.
func TestExplainSkipsNestedReferenceUnderExcludedParent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "input.tmpl")
	if err := os.WriteFile(tmpl, []byte("${A:-${B}}\n"), 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}
	stdout, stderr, status := execute(t, dir, nil, []string{
		"render", "--no-process-env", "--set", "B=from-set", "--ignore-vars", "A", "--explain", tmpl,
	})
	if status != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", status, stderr)
	}
	if stdout != "${A:-${B}}\n" {
		t.Fatalf("stdout = %q, want the whole token left literal", stdout)
	}
	if strings.Contains(stderr, `"variable":"B"`) {
		t.Fatalf("report = %q, want the nested reference omitted", stderr)
	}
}
