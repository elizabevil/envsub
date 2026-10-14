package schema

import (
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
