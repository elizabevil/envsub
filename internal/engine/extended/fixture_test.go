package extended_test

import (
	"bytes"
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
	// fixtureTemplateName is the name both render paths report, so their
	// diagnostics can be compared.
	fixtureTemplateName = "input.tmpl"
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
	// Repeat renders the template Repeat times, so the expected output is the
	// expected output of one copy repeated the same number of times. It is how
	// a fixture reaches the parallel render path, which needs a template of
	// several windows. Only a success fixture may repeat: a repeated failure
	// would fail at many lines, and one error cannot describe it.
	Repeat int `yaml:"repeat"`
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
			// A directory without a template is not a fixture: the group
			// directories only organise the cases, and fuzz/ holds the
			// corpus Go's fuzzing engine owns.
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

// runFixture renders a fixture twice: once buffered and once streamed from a
// file. The two paths must agree byte for byte, so a template that renders
// differently depending on how it was read cannot slip through.
func runFixture(t *testing.T, dir string) {
	t.Helper()
	input := mustReadFile(t, filepath.Join(dir, fixtureInput))

	var meta fixtureOptions
	if data, ok := readOptional(t, filepath.Join(dir, fixtureMeta)); ok {
		if err := yamlx.Unmarshal(data, &meta); err != nil {
			t.Fatalf("unmarshal meta.yaml: %v", err)
		}
	}
	if meta.Repeat < 0 {
		t.Fatalf("meta.yaml repeat = %d, want a count of at least 1", meta.Repeat)
	}
	if meta.Repeat > 1 && meta.ExitCode != 0 {
		t.Fatalf("meta.yaml repeat = %d on a failure fixture: the repeated template fails at many lines, so one error cannot describe it", meta.Repeat)
	}

	template := repeatBytes(input, meta.Repeat)
	options := engine.Options{
		Unset:         meta.Unset,
		Empty:         meta.Empty,
		SchemaUnknown: meta.SchemaUnknown,
		Only:          meta.Only,
		Ignore:        meta.Ignore,
	}

	// The buffered path renders the whole template in memory.
	bufferedTree, bufferedSchema, contextErr := fixtureContext(t, dir, meta)
	var buffered []byte
	bufferedErr := contextErr
	if contextErr == nil {
		buffered, bufferedErr = extended.New().Render(context.Background(), engine.Input{
			Name:     fixtureTemplateName,
			Template: template,
			Values:   bufferedTree,
			Schema:   bufferedSchema,
			Options:  options,
		})
	}

	// The streaming path reads the same template back from a file window by
	// window, which is the path a large template takes. It runs on a fresh
	// tree: a template assignment marks the tree as mutated, and a mutated
	// tree cannot take the parallel path.
	var streamed []byte
	var streamErr error
	if contextErr == nil {
		streamTree, streamSchema, streamContextErr := fixtureContext(t, dir, meta)
		if streamContextErr != nil {
			t.Fatalf("streaming context error = %v, want the same context as the buffered render", streamContextErr)
		}
		streamed, streamErr = renderStreamed(t, template, streamTree, streamSchema, options)
	}

	if meta.ExitCode != 0 {
		if bufferedErr == nil {
			t.Fatalf("Render() error = nil, want exit %d", meta.ExitCode)
		}
		assertFixtureFailure(t, dir, meta, bufferedErr, "Render")
		if contextErr != nil {
			// The fixture's own data failed to load, so there was nothing to
			// render and nothing for the streaming path to compare.
			return
		}
		if streamErr == nil {
			t.Fatalf("StreamTo() error = nil, want exit %d", meta.ExitCode)
		}
		assertFixtureFailure(t, dir, meta, streamErr, "StreamTo")
		return
	}

	if bufferedErr != nil {
		t.Fatalf("Render() error = %v", bufferedErr)
	}
	if streamErr != nil {
		t.Fatalf("StreamTo() error = %v", streamErr)
	}
	want := repeatBytes(mustReadFile(t, filepath.Join(dir, fixtureExpect)), meta.Repeat)
	if string(buffered) != string(want) {
		t.Fatalf("Render() = %q, want %q", string(buffered), string(want))
	}
	if string(streamed) != string(want) {
		t.Fatalf("StreamTo() = %q, want %q", string(streamed), string(want))
	}
}

// fixtureContext builds everything a fixture needs to render: the value tree,
// the compiled schema and the error the fixture's own data produced, if any.
//
// The tree is built from scratch on every call, so a caller that renders twice
// gets two independent trees.
func fixtureContext(t *testing.T, dir string, meta fixtureOptions) (*values.Tree, *schema.Schema, error) {
	t.Helper()

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
	return tree, sch, err
}

// renderStreamed renders a template through the streaming path: the template
// is written to a file and read back window by window. The source reports its
// size, so a template of several windows takes the parallel path.
func renderStreamed(t *testing.T, template []byte, tree *values.Tree, sch *schema.Schema, options engine.Options) ([]byte, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), fixtureTemplateName)
	if err := os.WriteFile(path, template, 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open template: %v", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("close template: %v", closeErr)
		}
	}()

	var out bytes.Buffer
	err = extended.New().StreamTo(context.Background(), &out, file, engine.Input{
		Name:            fixtureTemplateName,
		Values:          tree,
		Schema:          sch,
		Options:         options,
		SourceSize:      int64(len(template)),
		SourceSizeKnown: true,
	})
	if err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// assertFixtureFailure checks the exit code and the expected message of a
// failing fixture.
func assertFixtureFailure(t *testing.T, dir string, meta fixtureOptions, err error, path string) {
	t.Helper()
	if got := diag.ExitCode(err); got != meta.ExitCode {
		t.Fatalf("%s() exit = %d, want %d; err=%v", path, got, meta.ExitCode, err)
	}
	if want := readOptionalString(t, filepath.Join(dir, fixtureExpectErr)); want != "" {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("%s() error = %q, want it to contain %q", path, err.Error(), want)
		}
	}
}

// repeatBytes returns content repeated count times. A count of one is content
// unchanged, so the common fixture needs no special case.
func repeatBytes(content []byte, count int) []byte {
	if count <= 1 {
		return content
	}
	return bytes.Repeat(content, count)
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
	values.MergeInto(tree, loaded, values.Meta{File: name, Kind: values.SourceValues})
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
	loaded, err := source.DotenvFormat().Loader().Values.Load(data)
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

// metaSegments parses a spelling into the segments the metadata API takes.
func metaSegments(t *testing.T, spelling string) []values.Segment {
	t.Helper()
	segments, err := values.ParsePath(spelling)
	if err != nil {
		t.Fatalf("ParsePath(%q) error = %v", spelling, err)
	}
	return segments
}
