package values

import (
	"fmt"
	"slices"
	"strings"
)

// Tree is the value tree a template resolves against: the merged root namespace
// plus the read-only process environment.
//
// Every method requires a non-nil receiver: NewTree returns a ready tree, and the
// engine hands one to any input that arrived without values (Input.WithTree). The
// zero Tree is usable as an empty tree, so a caller that wants "no values" passes
// NewTree() rather than nil.
type Tree struct {
	// Root holds root-namespace values by top-level name. Every source (--set,
	// dotenv files, value files, schema defaults) merges here in precedence order.
	Root map[string]any
	// Env holds the process environment, read through @process.NAME. It never
	// supplies a root value.
	Env map[string]string

	// meta records where each value came from, keyed by encoded path.
	meta map[Key]Meta
	// paths caches compiled paths: a template repeats the same handful of paths
	// hundreds of thousands of times.
	paths map[string]CompiledPath
	// mutated records a template assignment, which makes the tree unsafe to read
	// from parallel workers.
	mutated bool
}

// CompiledPath is a parsed template path that resolves without further
// allocation.
type CompiledPath struct {
	segments []Segment
	// keys holds each segment's map key and is indexed parallel to segments;
	// index segments need none.
	keys []string
	// envKey is the environment variable a namespace path addresses, empty for a
	// root path.
	envKey string
}

// Meta describes the origin of a single value.
type Meta struct {
	// File is the source file, empty for values without a file origin.
	File string
	// Line is the 1-based source line, 0 when unknown.
	Line int
	// Kind is the layering stage, e.g. "values", "envfile", "set".
	Kind string
	// Method is how the value entered that stage, e.g. "file", "flag".
	Method string
	// Source is the address within the stage: a file path, a schema pointer, an
	// environment variable name, or the key itself.
	Source string
}

// MetaRef is the closest known origin of a value.
type MetaRef struct {
	// Meta is the closest recorded origin.
	Meta Meta
	// Found reports whether any origin was recorded.
	Found bool
}

// NewTree returns an empty value tree.
func NewTree() *Tree {
	return &Tree{
		Root: map[string]any{},
		Env:  map[string]string{},
		meta: map[Key]Meta{},
	}
}

// Resolve looks up a raw template path in the root and env namespaces.
func (t *Tree) Resolve(rawPath string) (any, bool, error) {
	return resolvePath(t.Root, t.Env, rawPath)
}

// ResolvePath resolves an already compiled path, avoiding a re-parse per
// reference.
func (t *Tree) ResolvePath(path CompiledPath) (any, bool) {
	return path.lookup(t.Root, t.Env)
}

// CompilePath parses rawPath once and caches the result on the tree.
func (t *Tree) CompilePath(rawPath string) (CompiledPath, error) {
	if cached, ok := t.paths[rawPath]; ok {
		return cached, nil
	}
	path, err := compilePath(rawPath)
	if err != nil {
		return CompiledPath{}, err
	}
	if t.paths == nil {
		t.paths = make(map[string]CompiledPath)
	}
	t.paths[rawPath] = path
	return path, nil
}

// lookup resolves the compiled path without parsing or allocating. It mirrors
// resolvePath: a namespace path reads the process environment, anything else
// reads the root only.
func (p CompiledPath) lookup(root map[string]any, env map[string]string) (any, bool) {
	if p.envKey != "" {
		value, ok := env[p.envKey]
		if !ok {
			return nil, false
		}
		return value, true
	}
	return lookupRoot(root, p)
}

// lookupRoot walks the root namespace for the compiled path.
func lookupRoot(root map[string]any, p CompiledPath) (any, bool) {
	var current any = root
	for i, segment := range p.segments {
		if segment.IsIndex {
			items, ok := current.([]any)
			if !ok || segment.Index < 0 || segment.Index >= len(items) {
				return nil, false
			}
			current = items[segment.Index]
			continue
		}
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		value, ok := object[p.keys[i]]
		if !ok {
			return nil, false
		}
		current = value
	}
	return current, true
}

// compilePath parses a raw path and precomputes its lookup keys.
func compilePath(rawPath string) (CompiledPath, error) {
	segments, err := ParsePath(rawPath)
	if err != nil {
		return CompiledPath{}, err
	}
	path := CompiledPath{segments: segments, keys: make([]string, len(segments))}
	for i, segment := range segments {
		if segment.IsIndex {
			continue
		}
		path.keys[i] = segment.Key
	}
	if Namespace(segments) != "" {
		key, ok := EnvKey(segments)
		if !ok {
			return CompiledPath{}, fmt.Errorf("namespace %q needs a variable name", Namespace(segments))
		}
		path.envKey = key
		return path, nil
	}
	return path, nil
}

// ResolveRootName resolves a bare identifier in the root namespace, keeping the
// value's own type. name may still be a slice of the template.
func (t *Tree) ResolveRootName(name []byte) (any, bool) {
	value, ok := t.Root[string(name)]
	return value, ok
}

// ResolveEnvName resolves a name in the process environment as a string, so a
// caller that only needs the text never boxes it into an any. Only @process.NAME
// reaches it: a bare name never reads the environment.
func (t *Tree) ResolveEnvName(name []byte) (string, bool) {
	value, ok := t.Env[string(name)]
	return value, ok
}

// ResolveName resolves a bare identifier in the root namespace only; the process
// environment is deliberately not consulted, so the host cannot change what a
// template renders. It is the hot path for most template variables.
func (t *Tree) ResolveName(name string) (any, bool) {
	value, ok := t.Root[name]
	return value, ok
}

// Mutated reports whether a template assignment wrote to the tree. Renderers that
// split work across goroutines must fall back to sequential when it is set.
func (t *Tree) Mutated() bool {
	return t.mutated
}

// MarkMutated records that an assignment wrote to the tree.
func (t *Tree) MarkMutated() {
	t.mutated = true
}

// RecordMeta remembers where the value at key came from. The caller supplies the
// encoded key, so a path with an array element is addressed by JoinIndex and no
// separate index argument is needed.
func (t *Tree) RecordMeta(key Key, meta Meta) {
	if key == "" {
		return
	}
	if t.meta == nil {
		t.meta = map[Key]Meta{}
	}
	t.meta[key] = meta
}

// MetaKeys returns the encoded keys that have a recorded origin, sorted.
//
// Preallocating and sorting beats slices.Sorted(maps.Keys(...)): the iterator form
// cannot size the slice and allocates 7 times more for 8 keys (measured).
func (t *Tree) MetaKeys() []Key {
	if len(t.meta) == 0 {
		return nil
	}
	keys := make([]Key, 0, len(t.meta))
	for key := range t.meta {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// MetaAt returns the recorded origin for an encoded key.
func (t *Tree) MetaAt(key Key) (Meta, bool) {
	meta, ok := t.meta[key]
	return meta, ok
}

// MetaFor returns the closest recorded origin for a raw template path.
func (t *Tree) MetaFor(rawPath string) MetaRef {
	key, err := EncodePath(rawPath)
	if err != nil {
		return MetaRef{}
	}
	return t.MetaForKey(key)
}

// MetaForKey returns the closest recorded origin for an encoded key, walking from
// the most specific path prefix to the least specific one.
func (t *Tree) MetaForKey(key Key) MetaRef {
	if len(t.meta) == 0 {
		return MetaRef{}
	}
	for {
		if meta, ok := t.meta[key]; ok {
			return MetaRef{Meta: meta, Found: true}
		}
		trimmed, ok := trimMetaKey(key)
		if !ok {
			return MetaRef{}
		}
		key = trimmed
	}
}

// Loader loads one external value source into a tree.
type Loader interface {
	// Name identifies the source category for diagnostics.
	Name() string
	// Load parses data and returns the values it defines.
	Load(data []byte) (*Tree, error)
}

// EncodePath converts a raw template path into its encoded key: dots between
// named segments, [i] for an index, no @ sigil and no quotes. It is the spelling
// metadata is keyed by and the spelling --only-vars filters compare, so a quoted
// key and its dot spelling agree.
func EncodePath(rawPath string) (Key, error) {
	segments, err := ParsePath(rawPath)
	if err != nil {
		return "", err
	}
	return EncodeSegments(segments), nil
}

// EncodeSegments converts parsed segments into an encoded key: dots between named
// segments, [i] for an index, and no quotes. Callers that already hold segments
// (the --explain lookup) avoid re-parsing. Use CanonicalPath for a spelling a reader
// can paste back into a template.
func EncodeSegments(segments []Segment) Key {
	var b strings.Builder
	for _, segment := range segments {
		if segment.IsIndex {
			fmt.Fprintf(&b, indexFormat, segment.Index)
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(PathSeparator)
		}
		b.WriteString(segment.Key)
	}
	return Key(b.String())
}

// trimMetaKey removes the last path segment of an encoded key.
func trimMetaKey(key Key) (Key, bool) {
	if key == "" {
		return "", false
	}
	last := strings.LastIndexByte(string(key), PathSeparator)
	open := strings.LastIndexByte(string(key), indexOpen)
	switch {
	case last < 0 && open < 0:
		return "", false
	case open > last:
		return key[:open], true
	default:
		return key[:last], true
	}
}
