package schema

import (
	"sort"
	"strings"

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
	if s == nil || tree == nil || tree.Root == nil {
		return
	}
	applyDefaults(tree, s.effective(), tree.Root, "")
}

// defaultsVisitor fills a missing key with the default its schema declares.
type defaultsVisitor struct{}

func (defaultsVisitor) missing(tree *values.Tree, path string, childSchema map[string]any) (any, bool, error) {
	defaultValue, hasDefault := childSchema[keywordDefault]
	if !hasDefault {
		return nil, false, nil
	}
	tree.RecordEncodedMeta(path, values.Meta{Kind: string(values.SourceSchema)})
	return values.CloneValue(defaultValue), true, nil
}

func (defaultsVisitor) text(*values.Tree, string, map[string]any, string) (any, bool, error) {
	return nil, false, nil
}

// applyDefaults walks the schema and fills missing keys in the value tree.
func applyDefaults(tree *values.Tree, schemaNode any, node map[string]any, prefix string) {
	_ = walkProperties(tree, schemaNode, node, prefix, defaultsVisitor{})
}

// pointerPrefix is the JSON Pointer root every schema location hangs off.
const pointerPrefix = "#"

// DefaultOrigins returns the schema default declarations as origin tuples the
// loader reports: the encoded value path, the JSON pointer of the declaration and
// the schema file. Reporting these before any other layer is what puts a default
// at the bottom of a value's lineage, including when a later layer covers it.
func (s *Schema) DefaultOrigins() []DefaultOrigin {
	if s == nil {
		return nil
	}
	decls := s.DefaultDecls()
	origins := make([]DefaultOrigin, 0, len(decls))
	for key, pointer := range decls {
		origins = append(origins, DefaultOrigin{Path: key, Pointer: pointer, Schema: s.Source})
	}
	sort.Slice(origins, func(i, j int) bool { return origins[i].Path < origins[j].Path })
	return origins
}

// DefaultOrigin is one declared schema default.
type DefaultOrigin struct {
	// Path is the encoded value path the default applies to.
	Path string
	// Pointer is the JSON pointer of the declaration inside the schema document.
	Pointer string
	// Schema is the schema file, empty when the schema was not loaded from one.
	Schema string
}

// DefaultDecls returns every location the schema declares a `default` for, keyed
// by the encoded value path the walkers use. The pointer locates the declaration
// inside the schema document, so --explain can name where a default came from
// even after a later layer covered it.
func (s *Schema) DefaultDecls() map[string]string {
	if s == nil {
		return nil
	}
	decls := map[string]string{}
	collectDefaultDecls(s.effective(), "", pointerPrefix, decls)
	return decls
}

// collectDefaultDecls walks properties and items the way the walkers do, so the
// keys it reports match the keys those walkers record metadata for.
func collectDefaultDecls(schemaNode any, encodedPath, pointer string, decls map[string]string) {
	object, ok := schemaNode.(map[string]any)
	if !ok {
		return
	}
	if properties, ok := object[keywordProperties].(map[string]any); ok {
		for _, key := range sortedKeys(properties) {
			childSchema, ok := properties[key].(map[string]any)
			if !ok {
				continue
			}
			childPath := joinEncodedPath(encodedPath, key)
			childPointer := pointer + "/properties/" + escapePointerToken(key)
			if _, hasDefault := childSchema[keywordDefault]; hasDefault {
				decls[childPath] = childPointer + "/default"
			}
			collectDefaultDecls(childSchema, childPath, childPointer, decls)
		}
	}
}

// joinEncodedPath appends one key to an encoded path.
func joinEncodedPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

// escapePointerToken escapes one JSON Pointer reference token (RFC 6901).
func escapePointerToken(token string) string {
	token = strings.ReplaceAll(token, "~", "~0")
	return strings.ReplaceAll(token, "/", "~1")
}
