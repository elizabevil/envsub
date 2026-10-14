package schema

import (
	"github.com/elizabevil/envsub/internal/utils"
	"github.com/elizabevil/envsub/internal/values"
)

// propertyVisitor isolates the two decisions a schema walk makes, so defaults
// and materialization share one traversal skeleton (rule SI1): what a missing
// key means, and what a string value means.
type propertyVisitor interface {
	// missing handles a key the value tree does not have yet. stored reports
	// whether the returned value belongs in the tree.
	missing(tree *values.Tree, path []values.Segment, childSchema map[string]any) (value any, stored bool, err error)
	// text converts a present string value. changed reports whether the value
	// was replaced.
	text(tree *values.Tree, path []values.Segment, childSchema map[string]any, value string) (converted any, changed bool, err error)
}

// walkProperties applies visitor to every declared property in sorted order and
// recurses into objects and arrays through this single skeleton.
func walkProperties(tree *values.Tree, schemaNode any, node map[string]any, prefix []values.Segment, visitor propertyVisitor) error {
	object, ok := schemaNode.(map[string]any)
	if !ok {
		return nil
	}
	properties, ok := object[keywordProperties].(map[string]any)
	if !ok {
		return nil
	}
	for _, key := range utils.SortedKeys(properties) {
		childSchema, ok := properties[key].(map[string]any)
		if !ok {
			continue
		}
		childPath := values.AppendSegment(prefix, values.Segment{Key: key})
		value, exists := node[key]
		if !exists {
			replacement, stored, err := visitor.missing(tree, childPath, childSchema)
			if err != nil {
				return err
			}
			if stored {
				node[key] = replacement
				value, exists = replacement, true
			}
		}
		if !exists {
			continue
		}
		if text, isString := value.(string); isString {
			converted, changed, err := visitor.text(tree, childPath, childSchema, text)
			if err != nil {
				return err
			}
			if changed {
				node[key] = converted
				value = converted
			}
		}
		if err := walkValue(tree, childSchema, value, childPath, visitor); err != nil {
			return err
		}
	}
	return nil
}

// walkItems applies visitor to the elements of an array value.
func walkItems(tree *values.Tree, schemaNode any, items []any, prefix []values.Segment, visitor propertyVisitor) error {
	object, ok := schemaNode.(map[string]any)
	if !ok {
		return nil
	}
	itemSchema, ok := object[keywordItems].(map[string]any)
	if !ok {
		return nil
	}
	for i, item := range items {
		itemPath := values.AppendSegment(prefix, values.Segment{IsIndex: true, Index: i})
		value := item
		if text, isString := value.(string); isString {
			converted, changed, err := visitor.text(tree, itemPath, itemSchema, text)
			if err != nil {
				return err
			}
			if changed {
				items[i] = converted
				value = converted
			}
		}
		if err := walkValue(tree, itemSchema, value, itemPath, visitor); err != nil {
			return err
		}
	}
	return nil
}

// walkValue descends into the containers a value may hold.
func walkValue(tree *values.Tree, schemaNode map[string]any, value any, path []values.Segment, visitor propertyVisitor) error {
	switch typed := value.(type) {
	case map[string]any:
		return walkProperties(tree, schemaNode, typed, path, visitor)
	case []any:
		return walkItems(tree, schemaNode, typed, path, visitor)
	default:
		return nil
	}
}
