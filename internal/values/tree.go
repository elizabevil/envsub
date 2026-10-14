package values

import (
	"fmt"
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

	// meta mirrors the value tree: each node holds the origin recorded for the
	// value at its path, so two paths can never share one entry.
	meta *metaNode
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

// metaNode is one node of the origin tree: the origin recorded for the value at
// this path, plus the origin trees of its children. Parents are reached by
// following segments, which is why no encoded spelling is needed to key origins.
type metaNode struct {
	// origin is the recorded origin, meaningful only when recorded is set.
	origin Meta
	// recorded reports whether origin was written for this path.
	recorded bool
	// children maps each segment to its child node.
	children map[Segment]*metaNode
}

// child returns the node for seg, creating it when create is set.
func (n *metaNode) child(seg Segment, create bool) *metaNode {
	if child, ok := n.children[seg]; ok {
		return child
	}
	if !create {
		return nil
	}
	if n.children == nil {
		n.children = map[Segment]*metaNode{}
	}
	child := &metaNode{}
	n.children[seg] = child
	return child
}

// Meta describes the origin of a single value.
type Meta struct {
	// File is the source file, empty for values without a file origin.
	File string
	// Line is the 1-based source line, 0 when unknown.
	Line int
	// Kind is the layering stage the value came from.
	Kind SourceKind
	// Method is how the value entered that stage.
	Method SourceMethod
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
		meta: &metaNode{},
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

// RecordMeta remembers where the value at path came from.
func (t *Tree) RecordMeta(path []Segment, meta Meta) {
	if len(path) == 0 {
		return
	}
	if t.meta == nil {
		t.meta = &metaNode{}
	}
	node := t.meta
	for _, seg := range path {
		node = node.child(seg, true)
	}
	node.origin = meta
	node.recorded = true
}

// MetaAt returns the origin recorded exactly for rawPath, if any.
func (t *Tree) MetaAt(rawPath string) (Meta, bool) {
	segments, err := ParsePath(rawPath)
	if err != nil {
		return Meta{}, false
	}
	return t.metaAt(segments)
}

// metaAt returns the origin recorded exactly for path.
func (t *Tree) metaAt(path []Segment) (Meta, bool) {
	node := t.nodeAt(path)
	if node == nil || !node.recorded {
		return Meta{}, false
	}
	return node.origin, true
}

// WalkMeta calls visit for every recorded path, in the tree's order.
func (t *Tree) WalkMeta(visit func(path []Segment, meta Meta)) {
	walkRecordedPaths(t.meta, nil, func(path []Segment) {
		node := t.nodeAt(path)
		visit(path, node.origin)
	})
}

// MetaFor returns the closest recorded origin for rawPath: the origin of the path
// itself, else of its closest recorded ancestor.
func (t *Tree) MetaFor(rawPath string) MetaRef {
	segments, err := ParsePath(rawPath)
	if err != nil {
		return MetaRef{}
	}
	node := t.meta
	var found *metaNode
	for _, seg := range segments {
		node = node.child(seg, false)
		if node == nil {
			break
		}
		if node.recorded {
			found = node
		}
	}
	if found == nil {
		return MetaRef{}
	}
	return MetaRef{Meta: found.origin, Found: true}
}

// nodeAt returns the node for path, or nil when the tree does not reach it.
func (t *Tree) nodeAt(path []Segment) *metaNode {
	node := t.meta
	for _, seg := range path {
		node = node.child(seg, false)
		if node == nil {
			return nil
		}
	}
	return node
}

// walkRecordedPaths walks the origin tree, reporting every recorded path. The
// order is the map's, which is fine because MetaPaths sorts the spellings.
func walkRecordedPaths(node *metaNode, prefix []Segment, visit func(path []Segment)) {
	if node == nil {
		return
	}
	if node.recorded {
		visit(prefix)
	}
	for seg, child := range node.children {
		walkRecordedPaths(child, AppendSegment(prefix, seg), visit)
	}
}

// Loader loads one external value source into a tree.
type Loader interface {
	// Name identifies the source category for diagnostics.
	Name() string
	// Load parses data and returns the values it defines.
	Load(data []byte) (*Tree, error)
}
