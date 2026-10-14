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

// Origin is where a contributed value came from: the layering stage, how the
// value entered it, and the location. See ADR-005 for the model.
type Origin struct {
	// Kind is the layering stage: schema, values, envfile, set or process.
	Kind values.SourceKind
	// Method is how the value entered that stage: default, file, flag, json,
	// env or assign.
	Method values.SourceMethod
	// Source is the address within the stage: a file path, a schema pointer, or
	// the key itself when the stage has no other address.
	Source string
	// File is the source file, empty when the source has none.
	File string
	// Line is the 1-based line, 0 when unknown.
	Line int
}

// sourceOrigin maps a value loader name to the stage and method it reports.
// A .env file is its own stage because it has its own flag and its own failure
// mode; YAML and JSON are both "values", distinguished by the file extension.
func sourceOrigin(loaderName string) (values.SourceKind, values.SourceMethod) {
	if loaderName == source.DotenvName {
		return values.SourceEnvFile, values.MethodFile
	}
	return values.SourceValues, values.MethodFile
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

	sch, err := loadSchema(ctx, spec.Schema, policy)
	if err != nil {
		return nil, err
	}
	// The schema defaults are the bottom layer, so they are reported before any
	// other layer: a default a later layer covered then appears in that value's
	// lineage instead of after its winner (ADR-005 I6).
	if observer != nil && sch != nil {
		for _, decl := range sch.DefaultOrigins() {
			observer.AddRoot(decl.Path, Origin{
				Kind:   values.SourceSchema,
				Method: values.MethodDefault,
				Source: decl.Pointer,
				File:   decl.Schema,
			})
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
		for _, applied := range setKeys {
			observer.AddRoot(applied.Key, Origin{
				Kind:   values.SourceSet,
				Method: applied.Method,
				Source: applied.Key,
				File:   applied.File,
			})
		}
	}

	if sch != nil {
		sch.ApplyDefaults(tree)
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
	kind, method := sourceOrigin(loader.Name)
	for _, key := range loaded.MetaKeys() {
		meta, _ := loaded.MetaAt(key)
		observer.AddRoot(key, Origin{
			Kind:   kind,
			Method: method,
			Source: path,
			File:   meta.File,
			Line:   meta.Line,
		})
	}
	return nil
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

// setApplied is one --set* assignment that reached the tree, with the method it
// used and, for --set-file, the file it read.
type setApplied struct {
	// Key is the encoded root path the assignment wrote.
	Key string
	// Method is flag, json or file.
	Method values.SourceMethod
	// File is the file --set-file read, empty for the other two.
	File string
}

// applySets applies --set then --set-json then --set-file, in that fixed order
// regardless of the command line, and returns what each assignment wrote.
func applySets(tree *values.Tree, spec Spec) ([]setApplied, error) {
	applied := make([]setApplied, 0, len(spec.Sets)+len(spec.SetJSON)+len(spec.SetFiles))
	for _, assignment := range spec.Sets {
		key, err := setOne(tree, assignment, values.SetString)
		if err != nil {
			return nil, err
		}
		applied = append(applied, setApplied{Key: key, Method: values.MethodFlag})
	}
	for _, assignment := range spec.SetJSON {
		key, err := setOne(tree, assignment, values.SetJSON)
		if err != nil {
			return nil, err
		}
		applied = append(applied, setApplied{Key: key, Method: values.MethodJSON})
	}
	for _, assignment := range spec.SetFiles {
		key, file, err := setOneFile(tree, assignment)
		if err != nil {
			return nil, err
		}
		applied = append(applied, setApplied{Key: key, Method: values.MethodFile, File: file})
	}
	return applied, nil
}

// setOneFile writes one --set-file assignment: the value is the file's bytes as
// a string. One trailing newline is dropped, because a file created by an editor
// or a secret store ends with one; any other byte is kept, so a multi-line value
// survives intact.
func setOneFile(tree *values.Tree, assignment string) (string, string, error) {
	key, path, err := values.ParseAssignment(assignment)
	if err != nil {
		return "", "", diag.Wrap(diag.KindUsage, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", diag.Wrap(diag.KindIO, fmt.Errorf("read --set-file %s: %w", path, err))
	}
	text := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
	origin := values.Origin{Kind: values.SourceSet, Method: values.MethodFile, Source: key, File: path}
	if err := tree.SetPath(key, text, values.SetString, origin); err != nil {
		return "", "", diag.Wrap(diag.KindUsage, err)
	}
	encoded, err := values.EncodePath(key)
	if err != nil {
		return "", "", diag.Wrap(diag.KindUsage, err)
	}
	return encoded, path, nil
}

// setOne writes one --set assignment into the root and returns its encoded path.
func setOne(tree *values.Tree, assignment string, mode values.SetMode) (string, error) {
	key, value, err := values.ParseAssignment(assignment)
	if err != nil {
		return "", diag.Wrap(diag.KindUsage, err)
	}
	method := values.MethodFlag
	if mode == values.SetJSON {
		method = values.MethodJSON
	}
	if err := tree.SetPath(key, value, mode, values.Origin{
		Kind:   values.SourceSet,
		Method: method,
		Source: key,
	}); err != nil {
		// An invalid JSON literal is a type error; the other SetPath failures
		// (assignment form, path) stay usage errors.
		if errors.Is(err, values.ErrInvalidJSON) {
			return "", diag.Wrap(diag.KindType, err)
		}
		return "", diag.Wrap(diag.KindUsage, err)
	}
	encoded, err := values.EncodePath(key)
	if err != nil {
		return "", diag.Wrap(diag.KindUsage, err)
	}
	return encoded, nil
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
