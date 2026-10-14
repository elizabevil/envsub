// Package load builds the value tree a render runs against: it reads the value
// sources in precedence order, applies schema defaults, types the values and
// validates them. The tree is complete before any template is read.
package load

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/schema"
	"github.com/elizabevil/envsub/internal/source"
	"github.com/elizabevil/envsub/internal/values"
)

// Spec describes where the values of one run come from.
type Spec struct {
	// Files are YAML/JSON value files, merged in order (later wins).
	Files []string
	// DotenvFiles are .env files, the layer above Files (later wins).
	DotenvFiles []string
	// NoProcessEnv leaves the @process namespace empty.
	NoProcessEnv bool
	// ProcessEnvPrefix whitelists process variables by prefix.
	ProcessEnvPrefix string
	// Schema is the JSON Schema file; empty for none.
	Schema string
	// UnknownKeywords is the schema keyword policy: ignore, warn or error.
	UnknownKeywords string
	// Sets, SetJSON and SetFiles are --set, --set-json and --set-file
	// assignments; all write to the root, in that order.
	Sets     []string
	SetJSON  []string
	SetFiles []string
}

// Result is the value tree and the schema it was validated against.
type Result struct {
	// Tree is the merged root namespace plus the @process namespace.
	Tree *values.Tree
	// Schema is nil when the run has no schema.
	Schema *schema.Schema
}

// Observer is told which source contributed each name, in layering order, which is
// how --explain is built. It may be nil.
type Observer interface {
	// AddEnv reports a process environment variable.
	AddEnv(name string)
	// AddRoot reports a root path and the source it came from.
	AddRoot(path string, origin Origin)
}

// Origin is where a contributed value came from.
type Origin struct {
	// Kind is the source category: values, dotenv, set or schema.
	Kind values.SourceKind
	// File is the source file, empty when the source has none.
	File string
	// Line is the 1-based line, 0 when unknown.
	Line int
}

// Load reads every source in Spec and returns the tree the renderer resolves
// against.
//
// Layering is fixed: --set/--set-json beat a dotenv file, which beats a value
// file, which beats a schema default. The process environment is a separate
// read-only namespace and never fills a root key.
func Load(ctx context.Context, spec Spec, observer Observer) (*Result, error) {
	policy, err := schema.ParseUnknownPolicy(spec.UnknownKeywords)
	if err != nil {
		return nil, err
	}

	tree := values.NewTree()
	if spec.NoProcessEnv {
		tree.Env = map[string]string{}
	} else {
		tree.Env = values.EnvFromOSFiltered(spec.ProcessEnvPrefix)
	}
	if observer != nil {
		for name := range tree.Env {
			observer.AddEnv(name)
		}
	}

	// Layer order, lowest first: value files, dotenv files, --set. A .env file is
	// local to this machine or deployment, so it outranks the shared value files.
	for _, path := range spec.Files {
		loader, err := source.ForFile(path)
		if err != nil {
			return nil, err
		}
		if loader.Dotenv {
			return nil, diag.New(diag.KindUsage,
				"--values-file accepts YAML/JSON value files, not %s: use --dotenv-file for .env files", path)
		}
		if err := loadInto(tree, loader, path, observer); err != nil {
			return nil, err
		}
	}

	for _, path := range spec.DotenvFiles {
		// --dotenv-file names the format, so the file name does not matter.
		if err := loadInto(tree, source.DotenvLoader(), path, observer); err != nil {
			return nil, err
		}
	}

	setKeys, err := applySets(tree, spec)
	if err != nil {
		return nil, err
	}
	if observer != nil {
		for _, key := range setKeys {
			observer.AddRoot(key, Origin{Kind: values.SourceSet})
		}
	}

	sch, err := loadSchema(ctx, spec.Schema, policy)
	if err != nil {
		return nil, err
	}
	if sch != nil {
		sch.ApplyDefaults(tree)
		if observer != nil {
			reportSchemaDefaults(tree, observer)
		}
		if err := sch.Materialize(tree); err != nil {
			return nil, err
		}
		if err := sch.Validate(tree); err != nil {
			return nil, err
		}
	}
	return &Result{Tree: tree, Schema: sch}, nil
}

// loadInto reads one value file, merges it into the tree and reports its paths.
func loadInto(tree *values.Tree, loader source.Loader, path string, observer Observer) error {
	loaded, err := loader.ReadFile(path)
	if err != nil {
		return err
	}
	values.MergeInto(tree, loaded, values.Meta{File: path, Kind: loader.Name})
	if observer == nil {
		return nil
	}
	// The entries carry the origin the loader recorded: a dotenv key keeps its own
	// line number.
	for _, key := range loaded.MetaKeys() {
		meta, _ := loaded.MetaAt(key)
		observer.AddRoot(key, Origin{Kind: values.SourceKind(loader.Name), File: meta.File, Line: meta.Line})
	}
	return nil
}

// reportSchemaDefaults reports the root keys a schema default filled.
func reportSchemaDefaults(tree *values.Tree, observer Observer) {
	for _, key := range tree.MetaKeys() {
		if meta, ok := tree.MetaAt(key); ok && values.SourceKind(meta.Kind) == values.SourceSchema {
			observer.AddRoot(key, Origin{Kind: values.SourceSchema, File: meta.File, Line: meta.Line})
		}
	}
}

// loadSchema reads and compiles the configured schema file.
func loadSchema(ctx context.Context, path string, policy schema.UnknownPolicy) (*schema.Schema, error) {
	if path == "" {
		return nil, nil
	}
	loader, err := source.ForFile(path)
	if err != nil {
		return nil, err
	}
	loaded, err := loader.ReadFile(path)
	if err != nil {
		return nil, schemaFileError(path, err)
	}
	return schema.Load(ctx, mapToAny(loaded.Root), path, policy)
}

// mapToAny adapts the root map for the schema loader, which accepts any
// JSON-compatible document.
func mapToAny(root map[string]any) any {
	if root == nil {
		return nil
	}
	return root
}

// applySets applies --set then --set-json, in that fixed order regardless of the
// command line, and returns the encoded root paths they wrote.
func applySets(tree *values.Tree, spec Spec) ([]string, error) {
	keys := make([]string, 0, len(spec.Sets)+len(spec.SetJSON)+len(spec.SetFiles))
	for _, assignment := range spec.Sets {
		key, err := setOne(tree, assignment, values.SetString)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	for _, assignment := range spec.SetJSON {
		key, err := setOne(tree, assignment, values.SetJSON)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	for _, assignment := range spec.SetFiles {
		key, err := setOneFile(tree, assignment)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, nil
}

// setOneFile writes one --set-file assignment: the value is the file's bytes as
// a string. One trailing newline is dropped, because a file created by an editor
// or a secret store ends with one; any other byte is kept, so a multi-line value
// survives intact.
func setOneFile(tree *values.Tree, assignment string) (string, error) {
	key, path, err := values.ParseAssignment(assignment)
	if err != nil {
		return "", diag.Wrap(diag.KindUsage, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", diag.Wrap(diag.KindIO, fmt.Errorf("read --set-file %s: %w", path, err))
	}
	text := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
	if err := tree.SetPath(key, text, values.SetString, values.Origin{Kind: values.SourceSet}); err != nil {
		return "", diag.Wrap(diag.KindUsage, err)
	}
	return values.EncodePath(key)
}

// setOne writes one --set assignment into the root and returns its encoded path.
func setOne(tree *values.Tree, assignment string, mode values.SetMode) (string, error) {
	key, value, err := values.ParseAssignment(assignment)
	if err != nil {
		return "", diag.Wrap(diag.KindUsage, err)
	}
	if err := tree.SetPath(key, value, mode, values.Origin{Kind: values.SourceSet}); err != nil {
		// An invalid JSON literal is a type error; the other SetPath failures
		// (assignment form, path) stay usage errors.
		if errors.Is(err, values.ErrInvalidJSON) {
			return "", diag.Wrap(diag.KindType, err)
		}
		return "", diag.Wrap(diag.KindUsage, err)
	}
	return values.EncodePath(key)
}

// schemaFileError names the schema in a read failure. The shared loader reports a
// missing file as "value file", which would mislead a reader whose --schema path
// is wrong, so the message is rebuilt around the file the user passed.
func schemaFileError(path string, err error) error {
	message := err.Error()
	if after, ok := strings.CutPrefix(message, "value file "); ok {
		return diag.New(diag.KindIO, "schema file %s", after)
	}
	return diag.Wrap(diag.KindIO, fmt.Errorf("read schema %s: %w", path, err))
}
