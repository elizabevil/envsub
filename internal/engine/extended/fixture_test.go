package extended_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/engine/extended"
	"github.com/elizabevil/envsub/internal/schema"
	"github.com/elizabevil/envsub/internal/source"
	"github.com/elizabevil/envsub/internal/values"
	"github.com/elizabevil/envsub/internal/yamlx"
)

const (
	fixtureInput     = "input.tmpl"
	fixtureEnv       = "env.yaml"
	fixtureValues    = "values.yaml"
	fixtureValues2   = "values-2.yaml"
	fixtureEnvFile   = "env.txt"
	fixtureSchema    = "schema.yaml"
	fixtureMeta      = "meta.yaml"
	fixtureExpect    = "expect.out"
	fixtureExpectErr = "expect.err"
)

// fixtureOptions is the fixture's meta.yaml contract.
type fixtureOptions struct {
	// Unset selects the --on-missing policy.
	Unset string `yaml:"unset"`
	// Empty selects the --on-empty policy.
	Empty string `yaml:"empty"`
	// Only and Ignore mirror the CLI filters.
	Only   []string `yaml:"only"`
	Ignore []string `yaml:"ignore"`
	// ExitCode is the expected diag exit code for failure fixtures.
	ExitCode int `yaml:"exit_code"`
	// SchemaUnknown selects the unknown keyword policy; defaults to ignore.
	SchemaUnknown string `yaml:"schema_unknown"`
}

func TestFixtures(t *testing.T) {
	t.Parallel()
	root := fixtureRoot(t)
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if _, err := os.Stat(filepath.Join(path, fixtureInput)); err != nil {
			// A directory without a template is a typo, not a skip: silently
			// ignoring it would drop fixtures from the suite.
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		t.Run(filepath.ToSlash(rel), func(t *testing.T) {
			t.Parallel()
			runFixture(t, path)
		})
		return nil
	}); err != nil {
		t.Fatalf("walk fixtures: %v", err)
	}
}

func runFixture(t *testing.T, dir string) {
	t.Helper()
	input := mustReadFile(t, filepath.Join(dir, fixtureInput))

	tree := values.NewTree()
	if data, ok := readOptional(t, filepath.Join(dir, fixtureValues)); ok {
		if err := yamlx.Unmarshal(data, &tree.Root); err != nil {
			t.Fatalf("unmarshal values.yaml: %v", err)
		}
	}
	if data, ok := readOptional(t, filepath.Join(dir, fixtureEnv)); ok {
		if err := yamlx.Unmarshal(data, &tree.Env); err != nil {
			t.Fatalf("unmarshal env.yaml: %v", err)
		}
	}

	var meta fixtureOptions
	if data, ok := readOptional(t, filepath.Join(dir, fixtureMeta)); ok {
		if err := yamlx.Unmarshal(data, &meta); err != nil {
			t.Fatalf("unmarshal meta.yaml: %v", err)
		}
	}

	sch, schemaErr := loadFixtureSchema(t, dir, meta)
	if err := mergeFixtureValues(t, dir, fixtureValues2, tree); err != nil {
		t.Fatalf("merge values-2.yaml: %v", err)
	}
	// Dotenv data is the layer above the value files, so it merges last and its keys
	// land in the root namespace.
	envErr := mergeFixtureDotenv(t, dir, tree)
	// Schema defaults, materialization and validation run before rendering, as
	// the design's processing order requires. A failure here is the fixture's
	// error unless the fixture expected success.
	err := schemaErr
	if err == nil {
		err = envErr
	}
	if sch != nil && err == nil {
		sch.ApplyDefaults(tree)
		if err = sch.Materialize(tree); err != nil {
			err = diag.WrapIfNot(diag.KindType, err)
		} else if err = sch.Validate(tree); err != nil {
			err = diag.WrapIfNot(diag.KindSchema, err)
		}
	}

	var out []byte
	if err == nil {
		out, err = extended.New().Render(context.Background(), engine.Input{
			Template: input,
			Values:   tree,
			Schema:   sch,
			Options: engine.Options{
				Unset:         meta.Unset,
				Empty:         meta.Empty,
				SchemaUnknown: meta.SchemaUnknown,
				Only:          meta.Only,
				Ignore:        meta.Ignore,
			},
		})
	}

	if meta.ExitCode != 0 {
		if err == nil {
			t.Fatalf("Render() error = nil, want exit %d", meta.ExitCode)
		}
		if got := diag.ExitCode(err); got != meta.ExitCode {
			t.Fatalf("ExitCode() = %d, want %d; err=%v", got, meta.ExitCode, err)
		}
		if want := readOptionalString(t, filepath.Join(dir, fixtureExpectErr)); want != "" {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("error = %q, want it to contain %q", err.Error(), want)
			}
		}
		return
	}

	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	want := string(mustReadFile(t, filepath.Join(dir, fixtureExpect)))
	if string(out) != want {
		t.Fatalf("Render() = %q, want %q", string(out), want)
	}
}

// loadFixtureSchema loads and compiles schema.yaml when the fixture has one.
// Failures are returned so a fixture can assert them.
func loadFixtureSchema(t *testing.T, dir string, meta fixtureOptions) (*schema.Schema, error) {
	t.Helper()
	data, ok := readOptional(t, filepath.Join(dir, fixtureSchema))
	if !ok {
		return nil, nil
	}
	schemaTree, err := source.YAML{}.Load(data)
	if err != nil {
		return nil, diag.Wrap(diag.KindSchema, err)
	}
	policy := schema.UnknownPolicy(meta.SchemaUnknown)
	if policy == "" {
		policy = schema.UnknownIgnore
	}
	sch, err := schema.Load(context.Background(), schemaTree.Root, fixtureSchema, policy)
	if err != nil {
		return nil, err
	}
	return sch, nil
}

// mergeFixtureValues merges an optional additional values file into the tree.
func mergeFixtureValues(t *testing.T, dir, name string, tree *values.Tree) error {
	t.Helper()
	data, ok := readOptional(t, filepath.Join(dir, name))
	if !ok {
		return nil
	}
	loaded, err := source.YAML{}.Load(data)
	if err != nil {
		return err
	}
	values.MergeInto(tree, loaded, values.Meta{File: name, Kind: string(values.SourceValues)})
	return nil
}

// mergeFixtureDotenv loads env.txt as a dotenv layer. A .env file defines root
// keys, so it merges into the root with the line numbers the parser reported,
// exactly like the render layer does.
func mergeFixtureDotenv(t *testing.T, dir string, tree *values.Tree) error {
	t.Helper()
	data, ok := readOptional(t, filepath.Join(dir, fixtureEnvFile))
	if !ok {
		return nil
	}
	loaded, err := source.DotenvLoader().Values.Load(data)
	if err != nil {
		return diag.Wrap(diag.KindType, err)
	}
	values.MergeInto(tree, loaded, values.Meta{File: fixtureEnvFile, Kind: source.DotenvName})
	return nil
}

func fixtureRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "testdata")
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func readOptional(t *testing.T, path string) ([]byte, bool) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false
		}
		t.Fatalf("read %s: %v", path, err)
	}
	return data, true
}

func readOptionalString(t *testing.T, path string) string {
	t.Helper()
	data, ok := readOptional(t, path)
	if !ok {
		return ""
	}
	return strings.TrimSpace(string(data))
}
