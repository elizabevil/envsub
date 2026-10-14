package values

import (
	"errors"
	"fmt"
	"strings"

	"github.com/elizabevil/envsub/internal/jsonx"
)

// SourceKind names the category of a value origin.
type SourceKind string

const (
	// SourceDotenv marks values loaded from a .env file.
	SourceDotenv SourceKind = "dotenv"
	// SourceValues marks values loaded from a YAML or JSON values file.
	SourceValues SourceKind = "values"
	// SourceSet marks values produced by CLI --set* flags.
	SourceSet SourceKind = "set"
	// SourceSchema marks values produced by schema defaults.
	SourceSchema SourceKind = "schema"
	// SourceEnv marks values read from the OS environment.
	SourceEnv SourceKind = "env"
)

// ErrInvalidJSON marks a --set-json value that is not a valid JSON document.
// Callers map it to the type error kind so every invalid-JSON value reports the
// same way, whatever its source (rules G9).
var ErrInvalidJSON = errors.New("invalid JSON value")

// SetMode selects how a raw assignment value is parsed.
type SetMode int

const (
	// SetString stores the raw assignment text; the schema or an inline | type
	// performs the typed conversion later. It is what --set uses.
	SetString SetMode = iota
	// SetJSON requires a valid JSON document.
	SetJSON
)

// Origin describes where an assignment came from.
type Origin struct {
	// File is the source file, empty for values without a file origin.
	File string
	// Line is the 1-based source line, 0 when unknown.
	Line int
	// Kind is the source category, for example values or --set.
	Kind SourceKind
}

// assignmentSeparator splits KEY=VALUE on the command line and in .env files.
const assignmentSeparator = "="

// ParseAssignment splits a KEY=VALUE assignment.
func ParseAssignment(assignment string) (key string, value string, err error) {
	key, value, ok := strings.Cut(assignment, assignmentSeparator)
	key = strings.TrimSpace(key)
	if !ok || key == "" {
		return "", "", fmt.Errorf("assignment %q must be in KEY=VALUE form", assignment)
	}
	return key, value, nil
}

// SetPath assigns value at the raw path, creating intermediate objects.
func (t *Tree) SetPath(rawPath, value string, mode SetMode, origin Origin) error {
	segments, err := ParsePath(rawPath)
	if err != nil {
		return fmt.Errorf("invalid path %q: %w", rawPath, err)
	}
	parsed, err := parseSetValue(value, mode)
	if err != nil {
		return fmt.Errorf("set %s: %w", rawPath, err)
	}
	if t.Root == nil {
		t.Root = map[string]any{}
	}
	if err := assignPath(t.Root, segments, parsed); err != nil {
		return fmt.Errorf("set %s: %w", rawPath, err)
	}
	if origin.Kind != "" || origin.File != "" {
		t.RecordMeta(rawPath, Meta{File: origin.File, Line: origin.Line, Kind: string(origin.Kind)}, -1)
	}
	return nil
}

// Store writes a resolved value into the root namespace, creating intermediate
// objects. It is used by the template assignment operators := and =, which
// per the design only affect the current render and never touch the process
// environment.
func (t *Tree) Store(rawPath string, value any) error {
	segments, err := ParsePath(rawPath)
	if err != nil {
		return fmt.Errorf("invalid path %q: %w", rawPath, err)
	}
	if t.Root == nil {
		t.Root = map[string]any{}
	}
	if err := assignPath(t.Root, segments, CloneValue(value)); err != nil {
		return fmt.Errorf("assign %s: %w", rawPath, err)
	}
	t.MarkMutated()
	return nil
}

// parseSetValue converts a raw assignment value according to mode.
func parseSetValue(raw string, mode SetMode) (any, error) {
	switch mode {
	case SetString:
		return raw, nil
	case SetJSON:
		value, err := jsonx.UnmarshalValue([]byte(raw))
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidJSON, err)
		}
		return normalizeNumbers(value), nil
	default:
		// The raw text is kept; the schema or an inline | type assertion
		// performs the typed conversion later.
		return raw, nil
	}
}

// assignPath writes value into root following segments, creating objects for
// missing intermediate keys. Array indices must already exist, and a namespace
// (@process) is read-only, so neither can be an assignment target.
func assignPath(root map[string]any, segments []Segment, value any) error {
	if len(segments) == 0 {
		return fmt.Errorf("empty path")
	}
	if namespace := Namespace(segments); namespace != "" {
		return fmt.Errorf("namespace @%s is read-only: assignment paths target the root namespace", namespace)
	}
	node := root
	for i, segment := range segments {
		last := i == len(segments)-1
		if segment.IsIndex {
			return fmt.Errorf("array index %d is not supported in an assignment path", segment.Index)
		}
		if last {
			node[segment.Key] = value
			return nil
		}
		next := segments[i+1]
		existing, ok := node[segment.Key]
		if !ok || existing == nil {
			if next.IsIndex {
				return fmt.Errorf("cannot create array at %q", segment.Key)
			}
			child := map[string]any{}
			node[segment.Key] = child
			node = child
			continue
		}
		child, ok := existing.(map[string]any)
		if !ok {
			return fmt.Errorf("%q is already set to a %T value", segment.Key, existing)
		}
		node = child
	}
	return nil
}
