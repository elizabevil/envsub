package schema

import "github.com/elizabevil/envsub/internal/values"

// DeclaredType returns the JSON Schema type declared at a raw path, if any.
func (s *Schema) DeclaredType(path string) string {
	node, ok := s.schemaAt(path)
	if !ok {
		return ""
	}
	return declaredTypeName(node)
}

// declaredTypeName returns the type a schema node wants values materialized to.
// A union such as ["integer","null"] materializes as its first non-null member,
// because null is a nullability marker rather than a conversion target
// (rule G11); a union made only of null still declares null.
func declaredTypeName(node map[string]any) string {
	switch typed := node[keywordType].(type) {
	case string:
		return typed
	case []any:
		var null string
		for _, entry := range typed {
			name, ok := entry.(string)
			if !ok {
				continue
			}
			if name == TypeNull {
				null = name
				continue
			}
			return name
		}
		return null
	default:
		return ""
	}
}

// schemaAt returns the sub-schema object declared at a raw path.
func (s *Schema) schemaAt(path string) (map[string]any, bool) {
	if s == nil {
		return nil, false
	}
	node, ok := lookupSchema(s.effective(), path)
	if !ok {
		return nil, false
	}
	object, ok := node.(map[string]any)
	return object, ok
}

// lookupSchema walks the schema document following a raw template path.
func lookupSchema(raw any, path string) (any, bool) {
	segments, err := values.ParsePath(path)
	if err != nil {
		return nil, false
	}
	node := raw
	for _, segment := range segments {
		object, ok := node.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := schemaChild(object, segment)
		if !ok {
			return nil, false
		}
		node = next
	}
	return node, true
}

// schemaChild resolves one path segment against a schema object.
func schemaChild(object map[string]any, segment values.Segment) (any, bool) {
	if segment.IsIndex {
		items, ok := object[keywordItems].(map[string]any)
		if !ok {
			return nil, false
		}
		return items, true
	}
	properties, ok := object[keywordProperties].(map[string]any)
	if !ok {
		return nil, false
	}
	child, ok := properties[segment.Key]
	return child, ok
}

// DeclaredTypes returns every type name the schema declares at path, in written
// order and with "string" included, or nil when the node declares no usable
// type. Callers that must honour a union (rules SI11) use this instead of the
// single collapsed name.
func (s *Schema) DeclaredTypes(path string) []string {
	node, ok := s.schemaAt(path)
	if !ok {
		return nil
	}
	members, allowsString := declaredTypes(node)
	if allowsString {
		members = append(members, TypeString)
	}
	return members
}

// declaredTypes splits a schema node's type declaration into the convertible
// members, in written order, and whether "string" is allowed. "null" is a
// nullability marker and never a materialization target.
func declaredTypes(node map[string]any) (members []string, allowsString bool) {
	switch typed := node[keywordType].(type) {
	case string:
		if typed == TypeString {
			return nil, true
		}
		if typed == TypeNull {
			return nil, false
		}
		return []string{typed}, false
	case []any:
		members = make([]string, 0, len(typed))
		for _, entry := range typed {
			name, ok := entry.(string)
			if !ok || name == TypeNull {
				continue
			}
			if name == TypeString {
				allowsString = true
				continue
			}
			members = append(members, name)
		}
	}
	return members, allowsString
}
