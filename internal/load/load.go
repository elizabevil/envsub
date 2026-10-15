// Package load builds the value tree a render runs against: it reads the value
// sources in precedence order, applies schema defaults, types the values and
// validates them. The tree is complete before any template is read.
package load

import (
	"context"
	"errors"
	"fmt"
	"github.com/elizabevil/envsub/internal/utils"
	"os"
	"sort"
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
	// AddRoot reports the canonical spelling of a root path and its origin.
	AddRoot(path string, meta values.Meta)
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
	// lineage instead of after its winner (ADR-005 I6). The tuple comes from the
	// declaration, because an overridden default is never written to the tree.
	if observer != nil && sch != nil {
		reportDefaults(observer, sch)
	}

	// Layer order, lowest first: value files, dotenv files, --set. A .env file is
	// local to this machine or deployment, so it outranks the shared value files.
	for _, path := range spec.Files {
		format, err := source.FormatFor(path)
		if err != nil {
			return nil, err
		}
		if format.Dotenv {
			return nil, diag.New(diag.KindUsage,
				"--values-file accepts YAML/JSON value files, not %s: use --dotenv-file for .env files", path)
		}
		if err := loadInto(tree, format, path, observer); err != nil {
			return nil, err
		}
	}

	for _, path := range spec.DotenvFiles {
		// --dotenv-file names the format, so the file name does not matter.
		if err := loadInto(tree, source.DotenvFormat(), path, observer); err != nil {
			return nil, err
		}
	}

	setKeys, err := applySets(tree, spec)
	if err != nil {
		return nil, err
	}
	if observer != nil {
		for _, key := range setKeys {
			if meta, ok := tree.MetaAt(key); ok {
				observer.AddRoot(key, meta)
			}
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

// reportDefaults reports every declared schema default, lowest layer first, in a
// stable order. A default a later layer covered leaves no trace on the tree, so the
// declaration itself supplies the origin (its JSON pointer and the schema file).
func reportDefaults(observer Observer, sch *schema.Schema) {
	decls := sch.DefaultDecls()
	paths := make([]string, 0, len(decls))
	for path := range decls {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		observer.AddRoot(path, values.Meta{
			Kind:   values.SourceSchema,
			Method: values.MethodDefault,
			Source: decls[path],
			File:   sch.Source,
		})
	}
}

// loadInto reads one value file, merges it into the tree and reports its paths.
func loadInto(tree *values.Tree, format source.Format, path string, observer Observer) error {
	loaded, err := format.Loader().ReadFile(path)
	if err != nil {
		return err
	}
	values.MergeInto(tree, loaded, values.Meta{File: path, Kind: format.Kind, Method: values.MethodFile})
	if observer == nil {
		return nil
	}
	// The entries carry the origin the loader recorded: a dotenv key keeps its own
	// line number.
	loaded.WalkMeta(func(segments []values.Segment, meta values.Meta) {
		observer.AddRoot(values.CanonicalPath(segments), meta)
	})
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

// applySets applies --set then --set-json then --set-file, in that fixed order
// regardless of the command line, and returns the canonical spelling of every root
// path it wrote. Each value carries its own origin on the tree (kind set, method
// flag/json/file), so no separate record is needed here.
func applySets(tree *values.Tree, spec Spec) ([]string, error) {
	written := make([]string, 0, len(spec.Sets)+len(spec.SetJSON)+len(spec.SetFiles))
	for _, assignment := range spec.Sets {
		key, err := setOne(tree, assignment, values.SetString)
		if err != nil {
			return nil, err
		}
		written = append(written, key)
	}
	for _, assignment := range spec.SetJSON {
		key, err := setOne(tree, assignment, values.SetJSON)
		if err != nil {
			return nil, err
		}
		written = append(written, key)
	}
	for _, assignment := range spec.SetFiles {
		key, err := setOneFile(tree, assignment)
		if err != nil {
			return nil, err
		}
		written = append(written, key)
	}
	return written, nil
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
	text := strings.TrimSuffix(strings.TrimSuffix(string(utils.TrimBOM(data)), "\n"), "\r")
	meta := values.Meta{Kind: values.SourceSet, Method: values.MethodFile, Source: key, File: path}
	if err := tree.SetPath(key, text, values.SetString, meta); err != nil {
		return "", diag.Wrap(diag.KindUsage, err)
	}
	segments, err := values.ParsePath(key)
	if err != nil {
		return "", diag.Wrap(diag.KindUsage, err)
	}
	return values.CanonicalPath(segments), nil
}

// setOne writes one --set assignment into the root and returns its canonical path.
func setOne(tree *values.Tree, assignment string, mode values.SetMode) (string, error) {
	key, value, err := values.ParseAssignment(assignment)
	if err != nil {
		return "", diag.Wrap(diag.KindUsage, err)
	}
	method := values.MethodFlag
	if mode == values.SetJSON {
		method = values.MethodJSON
	}
	if err := tree.SetPath(key, value, mode, values.Meta{
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
	segments, err := values.ParsePath(key)
	if err != nil {
		return "", diag.Wrap(diag.KindUsage, err)
	}
	return values.CanonicalPath(segments), nil
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
