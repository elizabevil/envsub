package render

import (
	"strings"

	"github.com/elizabevil/envsub/internal/event"
	"github.com/elizabevil/envsub/internal/values"
)

// explainFrame is the shared origin-frame schema. It is an alias so the report
// and the event protocol cannot drift apart.
type explainFrame = event.OriginFrame

// explainKey is one reference parsed once into the three spellings the report
// needs: the name shown to the reader, the @process variable, and the root path
// the layering recorded.
type explainKey struct {
	// Name is the canonical spelling used in the report and --explain=PATH.
	Name string
	// EnvName is the process variable for @process.NAME; empty for root paths.
	EnvName string
	// RootKey is the canonical root path; empty for @process paths.
	RootKey string
}

// parseExplainKey parses a template reference once. A path the grammar rejects
// keeps its trimmed spelling but has no lookup key, so it resolves to missing
// instead of accidentally borrowing another namespace.
func parseExplainKey(raw string) explainKey {
	trimmed := strings.TrimSpace(raw)
	segments, err := values.ParsePath(trimmed)
	if err != nil {
		return explainKey{Name: trimmed}
	}
	name := values.CanonicalPath(segments)
	if values.Namespace(segments) != "" {
		envName, ok := values.EnvKey(segments)
		if !ok {
			return explainKey{Name: name}
		}
		return explainKey{Name: name, EnvName: envName}
	}
	return explainKey{Name: name, RootKey: name}
}

// explainName is the canonical spelling of a path, so the report can be pasted
// back into a template and matches an --explain=PATH filter exactly.
func explainName(path string) string {
	return parseExplainKey(path).Name
}

// impliedMethod maps a stage to the entry form recorded before method existed on
// every origin. An explicit method always wins.
var impliedMethod = map[values.SourceKind]values.SourceMethod{
	values.SourceValues:  values.MethodFile,
	values.SourceEnvFile: values.MethodFile,
	values.SourceSet:     values.MethodFlag,
	values.SourceSchema:  values.MethodDefault,
	values.SourceProcess: values.MethodEnv,
	values.SourceAssign:  values.MethodAssign,
}

// missingFrame is the frame reported for a name that resolved to nothing.
var missingFrame = explainFrame{
	Kind:   string(values.SourceMissing),
	Method: string(values.MethodMissing),
}

// frameFor renders one recorded origin as a report frame. Recording fills every
// field except method and source, which are inferred when a writer recorded only
// the stage (for example a set value written outside the loader).
func frameFor(meta values.Meta) explainFrame {
	kind := meta.Kind
	if kind == "" {
		kind = values.SourceMissing
	}
	method := meta.Method
	if method == "" {
		method = impliedMethod[kind]
	}
	if method == "" {
		method = values.MethodMissing
	}
	source := meta.Source
	if source == "" {
		source = meta.File
	}
	return explainFrame{
		Kind:   string(kind),
		Method: string(method),
		Source: source,
		File:   meta.File,
		Line:   meta.Line,
	}
}

// recorded reports whether an origin carries any information.
func recorded(meta values.Meta) bool {
	return meta.Kind != "" || meta.File != ""
}
