// Package schema loads, normalizes and validates JSON Schema contracts.
package schema

import (
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/elizabevil/envsub/internal/diag"
)

// UnknownPolicy selects how unknown schema keywords are handled.
type UnknownPolicy string

const (
	// UnknownIgnore silently accepts unknown non x-* keywords.
	UnknownIgnore UnknownPolicy = "ignore"
	// UnknownWarn logs unknown non x-* keywords.
	UnknownWarn UnknownPolicy = "warn"
	// UnknownError rejects unknown non x-* keywords.
	UnknownError UnknownPolicy = "error"
)

// Schema is a loaded and compiled JSON Schema document.
type Schema struct {
	// Raw is the normalized (alias-free) schema document.
	Raw any
	// Source is the file the schema was loaded from.
	Source string

	// compiled is the JSON Schema validator built from Raw.
	compiled *jsonschema.Schema
	// walk is the effective schema the walkers use: raw with local
	// $ref expanded and allOf merged (rule G12).
	walk map[string]any
}

// effective returns the schema map the walkers use: raw with local $ref
// expanded and allOf branches merged (rule G12). Raw is left untouched, so the
// compiled schema and validation semantics do not change.
func (s *Schema) effective() map[string]any {
	if s.walk != nil {
		return s.walk
	}
	root, _ := s.Raw.(map[string]any)
	return root
}

// ParseUnknownPolicy validates a --unknown-keywords value. An empty value means
// the default policy.
func ParseUnknownPolicy(raw string) (UnknownPolicy, error) {
	if raw == "" {
		return UnknownIgnore, nil
	}
	switch UnknownPolicy(raw) {
	case UnknownIgnore, UnknownWarn, UnknownError:
		return UnknownPolicy(raw), nil
	default:
		return "", diag.New(diag.KindUsage,
			"invalid --unknown-keywords value %q: want ignore, warn or error", raw)
	}
}
