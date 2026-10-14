// Package examples_test keeps the runnable examples honest. The examples are the
// compass for the test suite: they must cover every documented rule family, every
// flag, every type and source dimension, and their rendered output must parse as
// the format each template claims.
package examples_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// ruleFamily matches a rule id in the documentation tables, e.g. "| C12 |".
var ruleFamily = regexp.MustCompile(`(?m)^\| *([A-Z]{1,3})[0-9]+ *\|`)

// registeredFlag matches a flag name registered in internal/cli/constants.go.
var registeredFlag = regexp.MustCompile(`flag[A-Za-z]+\s*=\s*"([a-z][a-z0-9-]*)"`)

// keyName matches the dataset's variable naming rule: <type>_<source>_<effect>.
var keyName = regexp.MustCompile(`^([a-z]+)_([a-z]+)_([a-z0-9_]+)$`)

// valueTypes and valueSources are the dimensions every dataset must cover.
var (
	valueTypes   = []string{"string", "int", "float", "bool", "array", "object", "null"}
	valueSources = []string{"fmt", "operator", "prec", "process", "schema", "src", "type"}
)

// repoRoot walks up from the test file to the directory holding go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test directory")
		}
		dir = parent
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	return string(data)
}

// exampleFiles lists every example file whose content is part of the surface: the
// recipes, the README, the dataset, the templates and the cases.
func exampleFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	for _, dir := range []string{"examples", "examples/data", "examples/templates", "examples/cases"} {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			t.Fatalf("ReadDir(%s): %v", dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				files = append(files, filepath.Join(dir, entry.Name())+"/")
				continue
			}
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	return files
}

// datasetKeys returns the top-level variable names the shared value files define.
// Only column-zero keys count: an indented key is a value's own structure, not a
// dataset variable.
func datasetKeys(t *testing.T, root string) []string {
	t.Helper()
	var keys []string
	for _, name := range []string{"values-base.yaml", "values-prod.yaml", "values-extra.json"} {
		body := readFile(t, filepath.Join(root, "examples", "data", name))
		for _, line := range strings.Split(body, "\n") {
			if line == "" || strings.HasPrefix(line, "#") || line[0] == ' ' || line[0] == '\t' || line[0] == '{' || line[0] == '}' {
				continue
			}
			key, _, found := strings.Cut(line, ":")
			if !found {
				continue
			}
			key = strings.TrimSpace(key)
			if keyName.MatchString(key) {
				keys = append(keys, key)
			}
		}
	}
	slices.Sort(keys)
	return slices.Compact(keys)
}

// TestDatasetNamesFollowTheConvention pins the naming rule: every key is
// <type>_<source>_<effect> with a known type and source, so a reader can tell
// what each key proves.
func TestDatasetNamesFollowTheConvention(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	keys := datasetKeys(t, root)
	if len(keys) == 0 {
		t.Fatal("no dataset keys found")
	}
	for _, key := range keys {
		match := keyName.FindStringSubmatch(key)
		if match == nil {
			t.Errorf("key %q does not match <type>_<source>_<effect>", key)
			continue
		}
		if !slices.Contains(valueTypes, match[1]) {
			t.Errorf("key %q uses unknown type %q", key, match[1])
		}
		if !slices.Contains(valueSources, match[2]) {
			t.Errorf("key %q uses unknown source %q", key, match[2])
		}
	}
}

// TestDatasetCoversEveryDimension pins the coverage contract: each type appears
// under at least one source and each source under at least one type, so the
// scenarios templates cannot quietly drop a dimension.
func TestDatasetCoversEveryDimension(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	keys := datasetKeys(t, root)

	types := map[string]bool{}
	sources := map[string]bool{}
	for _, key := range keys {
		match := keyName.FindStringSubmatch(key)
		if match == nil {
			continue
		}
		types[match[1]] = true
		sources[match[2]] = true
	}
	for _, want := range valueTypes {
		if !types[want] {
			t.Errorf("no dataset key uses type %q", want)
		}
	}
	for _, want := range valueSources {
		if !sources[want] {
			t.Errorf("no dataset key uses source %q", want)
		}
	}
}

// TestRuleFamiliesAreMapped pins that every rule family the serialization docs
// define appears in the examples README.
func TestRuleFamiliesAreMapped(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	docsDir := filepath.Join(root, "docs", "07-serialization")
	entries, err := os.ReadDir(docsDir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", docsDir, err)
	}
	families := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		for _, match := range ruleFamily.FindAllStringSubmatch(readFile(t, filepath.Join(docsDir, entry.Name())), -1) {
			families[match[1]] = true
		}
	}
	if len(families) == 0 {
		t.Fatal("no rule families found in docs/07-serialization")
	}
	readme := readFile(t, filepath.Join(root, "examples", "README.md"))
	for family := range families {
		if !regexp.MustCompile(family + `[0-9]`).MatchString(readme) {
			t.Errorf("rule family %s is not named in examples/README.md", family)
		}
	}
}

// TestExamplesCoverEveryRegisteredFlag pins that no flag the CLI registers is
// missing from the examples.
func TestExamplesCoverEveryRegisteredFlag(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	known := map[string]bool{}
	for _, match := range registeredFlag.FindAllStringSubmatch(
		readFile(t, filepath.Join(root, "internal", "cli", "constants.go")), -1) {
		known[match[1]] = true
	}
	if len(known) == 0 {
		t.Fatal("no flags found in internal/cli/constants.go")
	}

	var corpus strings.Builder
	for _, name := range exampleFiles(t, root) {
		if strings.HasSuffix(name, "/") {
			continue
		}
		corpus.WriteString(readFile(t, filepath.Join(root, filepath.FromSlash(name))))
	}
	text := corpus.String()
	for flag := range known {
		if !strings.Contains(text, "--"+flag) {
			t.Errorf("--%s is registered but no example uses or documents it", flag)
		}
	}
}

// TestTemplateFormattersAreExercised pins the pipeline contract: every formatter
// the code registers must appear in a template, so adding one forces the examples
// to demonstrate it.
func TestTemplateFormattersAreExercised(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	registered := readFile(t, filepath.Join(root, "internal", "pipeline", "pipeline.go"))
	var templates strings.Builder
	for _, name := range []string{"scenarios-json.tmpl", "scenarios-yaml.tmpl", "realworld.tmpl", "unused.tmpl", "precedence.tmpl"} {
		templates.WriteString(readFile(t, filepath.Join(root, "examples", "templates", name)))
	}
	body := templates.String()
	// The formatter names are the Name* constants of the pipeline package.
	names := make([]string, 0, 8)
	for _, line := range strings.Split(registered, "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "Name"); ok {
			if _, after, found := strings.Cut(value, "= \""); found {
				name, _, _ := strings.Cut(after, "\"")
				if name != "" {
					names = append(names, name)
				}
			}
		}
	}
	if len(names) == 0 {
		t.Fatal("no formatter names found in pipeline.go")
	}
	for _, name := range names {
		if !strings.Contains(body, "| "+name) {
			t.Errorf("formatter %q is registered but no example template uses it", name)
		}
	}
}

// TestScenariosParseAsTheirDeclaredFormat pins the two-template split: the JSON
// template's output is valid JSON and the YAML template's is valid YAML, which is
// what the parse marker next to each template claims.
func TestScenariosParseAsTheirDeclaredFormat(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	for _, name := range []string{"scenarios-json", "scenarios-yaml"} {
		marker := strings.TrimSpace(readFile(t, filepath.Join(root, "examples", "templates", name+".parse")))
		if marker != "json" && marker != "yaml" {
			t.Errorf("%s.parse = %q, want json or yaml", name, marker)
		}
		golden := readFile(t, filepath.Join(root, "examples", "templates", name+".golden"))
		if marker == "json" && !strings.HasPrefix(strings.TrimSpace(golden), "{") {
			t.Errorf("%s.golden does not look like a JSON object", name)
		}
		if marker == "yaml" && strings.HasPrefix(strings.TrimSpace(golden), "{") {
			t.Errorf("%s.golden looks like JSON, not YAML", name)
		}
	}
}
