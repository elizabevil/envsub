package examples_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/yamlx"
)

var exampleBinary string

// TestMain builds the CLI once so the guards can render the examples exactly the
// way the recipes do.
func TestMain(m *testing.M) {
	if err := buildExampleBinary(); err != nil {
		panic(err)
	}
	code := m.Run()
	if exampleBinary != "" {
		_ = os.RemoveAll(filepath.Dir(exampleBinary))
	}
	os.Exit(code)
}

func buildExampleBinary() error {
	root, err := findRepoRoot()
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "envsub-examples-*")
	if err != nil {
		return err
	}
	exampleBinary = filepath.Join(dir, "envsub")
	cmd := exec.Command("go", "build", "-o", exampleBinary, "./cmd/envsub")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return &buildError{err: err, output: string(out)}
	}
	return nil
}

type buildError struct {
	err    error
	output string
}

func (e *buildError) Error() string { return e.err.Error() + ": " + e.output }

// findRepoRoot walks up from the working directory to the module root.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// exampleSources is the flag set this package renders with. It deliberately
// repeats what examples/justfile uses: the examples are the source of truth and
// the two layers stay independent, so neither reads the other's files.
func exampleSources(root string) []string {
	data := filepath.Join(root, "examples", "data")
	return []string{
		"-f", filepath.Join(data, "values-base.yaml"),
		"-f", filepath.Join(data, "values-prod.yaml"),
		"-f", filepath.Join(data, "values-extra.json"),
		"--dotenv-file", filepath.Join(data, "app.env"),
		"--schema", filepath.Join(data, "schema.yaml"),
		"--on-missing=empty",
		"--set", "string_src_from_set=from-set",
		"--set", "string_prec_set_over_dotenv=from-set",
		"--set", "string_prec_default_never_wins=from-set",
		"--set-json", `string_prec_setjson_over_set="from-set-json"`,
		"--set-file", "string_setfile_from_file=" + filepath.Join(root, "examples", "secrets", "tls.key"),
	}
}

// exampleEnv pins the process environment, for the same reason the recipes do:
// @process results must not depend on the machine.
func exampleEnv() []string {
	return []string{
		"ENVSUB_STRING=from-process",
		"ENVSUB_INT=42",
		"ENVSUB_BOOL=true",
		"ENVSUB-DASHED=dashed",
	}
}

// renderExample runs the binary with this package's own flags and environment.
func renderExample(t *testing.T, root string, args ...string) (string, string, int) {
	t.Helper()
	full := append([]string{"render"}, exampleSources(root)...)
	full = append(full, args...)
	cmd := exec.Command(exampleBinary, full...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), exampleEnv()...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatalf("run %v: %v", full, err)
	}
	return stdout.String(), stderr.String(), code
}

// parseMarker returns "json", "yaml" or "" for a template.
func parseMarker(t *testing.T, root, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "examples", "templates", name+".parse"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// TestExampleTemplatesMatchTheirGolden pins the examples' own expectations: every
// template with a golden renders to it, using the same flags and environment as
// the recipes. `just check` runs this, so a change to the dataset or a template
// cannot silently invalidate the examples.
func TestExampleTemplatesMatchTheirGolden(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, "examples", "templates"))
	if err != nil {
		t.Fatalf("ReadDir(templates): %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".golden") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".golden")
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			template := filepath.Join(root, "examples", "templates", name+".tmpl")
			stdout, stderr, code := renderExample(t, root, template)
			if code != 0 {
				t.Fatalf("render exit = %d, want 0; stderr=%s", code, stderr)
			}
			golden := readFile(t, filepath.Join(root, "examples", "templates", name+".golden"))
			if stdout != golden {
				t.Errorf("output differs from %s.golden\n--- got ---\n%s\n--- want ---\n%s", name, stdout, golden)
			}
			switch marker := parseMarker(t, root, name); marker {
			case "json":
				assertParsesAsJSON(t, name, stdout)
			case "yaml":
				assertParsesAsYAML(t, name, stdout)
			case "":
				// No parse contract for this template.
			default:
				t.Errorf("%s.parse = %q, want json or yaml", name, marker)
			}
		})
	}
}

// assertParsesAsJSON checks the rendered document is valid JSON.
func assertParsesAsJSON(t *testing.T, name, document string) {
	t.Helper()
	var decoded any
	if err := json.Unmarshal([]byte(document), &decoded); err != nil {
		t.Errorf("%s is not valid JSON: %v", name, err)
	}
}

// assertParsesAsYAML checks the rendered document is valid YAML.
func assertParsesAsYAML(t *testing.T, name, document string) {
	t.Helper()
	var decoded any
	if err := yamlx.Unmarshal([]byte(document), &decoded); err != nil {
		t.Errorf("%s is not valid YAML: %v", name, err)
	}
}

// TestExampleUnusedReportMatchesItsExpectation pins the --report=unused output the
// unused template is expected to produce.
func TestExampleUnusedReportMatchesItsExpectation(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	stdout, stderr, code := renderExample(t, root, "--report=unused",
		filepath.Join(root, "examples", "templates", "unused.tmpl"))
	if code != 0 {
		t.Fatalf("render exit = %d, want 0; stderr=%s", code, stderr)
	}
	wantGolden := readFile(t, filepath.Join(root, "examples", "templates", "unused.golden"))
	if stdout != wantGolden {
		t.Errorf("unused stdout differs from unused.golden")
	}
	wantReport := readFile(t, filepath.Join(root, "examples", "templates", "unused.unused"))
	if stderr != wantReport {
		t.Errorf("unused report differs\n--- got ---\n%s\n--- want ---\n%s", stderr, wantReport)
	}
}

// TestExampleFailureCasesMatchTheirExitCodes pins the cases/errors contract: each
// directory's expected exit code and stderr fragment hold.
func TestExampleFailureCasesMatchTheirExitCodes(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	cases, err := os.ReadDir(filepath.Join(root, "examples", "cases", "errors"))
	if err != nil {
		t.Fatalf("ReadDir(cases/errors): %v", err)
	}
	exitPattern := regexp.MustCompile(`^\d+$`)
	for _, entry := range cases {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(root, "examples", "cases", "errors", name)
			wantExit := strings.TrimSpace(readFile(t, filepath.Join(dir, "expect.exit")))
			if !exitPattern.MatchString(wantExit) {
				t.Fatalf("%s/expect.exit = %q, want a number", name, wantExit)
			}
			fragment := strings.TrimSpace(readFile(t, filepath.Join(dir, "expect.err")))
			extra := failureCaseExtra(root, name)

			stdout, stderr, code := renderExample(t, root, append(extra, filepath.Join(dir, "input.tmpl"))...)
			if code != atoi(t, wantExit) {
				t.Errorf("exit = %d, want %s; stderr=%s", code, wantExit, stderr)
			}
			if !strings.Contains(stderr, fragment) {
				t.Errorf("stderr = %q, want it to contain %q", stderr, fragment)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want nothing on a failure", stdout)
			}
		})
	}
}

// failureCaseExtra returns the flags a failure case needs beyond the shared set,
// mirroring the `failures` recipe.
func failureCaseExtra(root, name string) []string {
	switch name {
	case "missing":
		// The shared flag set tolerates a missing value; this case needs the
		// default policy back.
		return []string{"--set", "string_type_root=ok", "--on-missing=error"}
	case "schema":
		return []string{"--set", "string_type_root=!"}
	case "io":
		return []string{"--set-file", "string_setfile_from_file=" + filepath.Join(root, "examples", "secrets", "absent.key")}
	case "usage":
		return []string{"--on-missing=bogus"}
	case "type":
		return []string{"--set", "bool_schema_from_string=not-a-bool"}
	default:
		return []string{"--set", "string_type_root=ok"}
	}
}

// atoi parses the expected exit code.
func atoi(t *testing.T, text string) int {
	t.Helper()
	value := 0
	for _, r := range text {
		if r < '0' || r > '9' {
			t.Fatalf("exit %q is not a number", text)
		}
		value = value*10 + int(r-'0')
	}
	return value
}
