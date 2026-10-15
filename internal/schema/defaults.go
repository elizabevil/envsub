package schema

import (
	"strings"

	"github.com/elizabevil/envsub/internal/utils"

	"github.com/elizabevil/envsub/internal/values"
)

// ApplyDefaults fills missing keys with the defaults declared by the schema.
//
// A default is the last-resort fallback: it applies only when no source provides
// the value. Every explicit source (--set, dotenv files, value files) merges into
// the root before this runs, so a declared default never outranks a real source
// (rule G21). The process environment is a separate, read-only namespace
// (@process) and therefore never stands in for a root key.
func (s *Schema) ApplyDefaults(tree *values.Tree) {
	if s == nil || tree.Root == nil {
		return
	}
	// The visitor resolves each default's JSON pointer from the declaration map, so
	// the origin it records is the same tuple --explain reports.
	applyDefaults(tree, s.effective(), tree.Root, nil, defaultsVisitor{decls: s.DefaultDecls(), schema: s.Source})
}

// defaultsVisitor fills a missing key with the default its schema declares.
type defaultsVisitor struct {
	// decls maps a canonical value path to the JSON pointer of its declaration.
	decls map[string]string
	// schema is the schema file the defaults were declared in.
	schema string
}

func (v defaultsVisitor) missing(tree *values.Tree, path []values.Segment, childSchema map[string]any) (any, bool, error) {
	defaultValue, hasDefault := childSchema[keywordDefault]
	if !hasDefault {
		return nil, false, nil
	}
	tree.RecordMeta(path, values.Meta{
		Kind:   values.SourceSchema,
		Method: values.MethodDefault,
		Source: v.decls[values.CanonicalPath(path)],
		File:   v.schema,
	})
	return values.CloneValue(defaultValue), true, nil
}

func (defaultsVisitor) text(*values.Tree, []values.Segment, map[string]any, string) (any, bool, error) {
	return nil, false, nil
}

// applyDefaults walks the schema and fills missing keys in the value tree.
func applyDefaults(tree *values.Tree, schemaNode any, node map[string]any, prefix []values.Segment, visitor defaultsVisitor) {
	_ = walkProperties(tree, schemaNode, node, prefix, visitor)
}

// pointerPrefix is the JSON Pointer root every schema location hangs off.
const pointerPrefix = "#"

// DefaultDecls maps every declared default to the JSON pointer of its declaration,
// keyed by the canonical spelling of the value path it fills.
func (s *Schema) DefaultDecls() map[string]string {
	if s == nil {
		return nil
	}
	decls := map[string]string{}
	collectDefaultDecls(s.effective(), nil, pointerPrefix, decls)
	return decls
}

// collectDefaultDecls walks properties and items the way the walkers do, so the
// keys it reports match the keys those walkers record metadata for.
func collectDefaultDecls(schemaNode any, path []values.Segment, pointer string, decls map[string]string) {
	object, ok := schemaNode.(map[string]any)
	if !ok {
		return
	}
	if properties, ok := object[keywordProperties].(map[string]any); ok {
		for _, key := range utils.SortedKeys(properties) {
			childSchema, ok := properties[key].(map[string]any)
			if !ok {
				continue
			}
			childPath := values.AppendSegment(path, values.Segment{Key: key})
			childPointer := pointer + "/properties/" + escapePointerToken(key)
			if _, hasDefault := childSchema[keywordDefault]; hasDefault {
				decls[values.CanonicalPath(childPath)] = childPointer + "/default"
			}
			collectDefaultDecls(childSchema, childPath, childPointer, decls)
		}
	}
}

// escapePointerToken escapes one JSON Pointer reference token (RFC 6901).
func escapePointerToken(token string) string {
	token = strings.ReplaceAll(token, "~", "~0")
	return strings.ReplaceAll(token, "/", "~1")
}
