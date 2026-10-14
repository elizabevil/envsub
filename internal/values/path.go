package values

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/elizabevil/envsub/internal/codec"
)

// Segment is one path component: a named key, an array index, or the leading
// namespace selector of a path.
type Segment struct {
	Key     string
	Index   int
	IsIndex bool
	// IsNamespace marks a namespace selector such as @process. Only the first
	// segment can be one.
	IsNamespace bool
}

// Template path syntax.
const (
	// PathSeparator separates the named segments of a path (app.port). It is
	// exported because callers outside this package that build or match paths
	// (schema diagnostics, --only-vars filters) must use the same grammar rather
	// than a literal of their own.
	PathSeparator  = '.'
	indexOpen      = '['
	indexClose     = ']'
	quoteChar      = '"'
	escapeChar     = '\\'
	underscoreChar = '_'
	// namespaceSigil introduces a namespace and can only appear as the first byte
	// of a path, so it can never collide with an object key.
	namespaceSigil = '@'
	// envNamespace is the only namespace: the read-only process environment.
	envNamespace = "process"
	// indexFormat is the bracket notation of one array element([0]); IndexSuffix
	// exposes it so a caller that builds a diagnostic address uses the same
	// spelling as the value grammar.
	indexFormat = "[%d]"
)

// ParsePath parses a path like app.port, hosts[0], app["my-key"], ["a.b"].k or
// @process.NAME. The grammar is project-specific: no standard library parser
// handles quoted keys with escapes and reports byte offsets for diagnostics.
func ParsePath(raw string) ([]Segment, error) {
	if raw == "" {
		return nil, fmt.Errorf("empty path")
	}
	parser := pathParser{input: raw}
	segments, err := parser.parsePrefix(true)
	if err != nil {
		return nil, err
	}
	if parser.pos != len(parser.input) {
		return nil, fmt.Errorf("unexpected character %q at offset %d", parser.input[parser.pos], parser.pos)
	}
	return segments, nil
}

// CutPathPrefix splits raw after its leading path and returns that path text plus
// whatever follows.
func CutPathPrefix(raw string) (path string, rest string, err error) {
	if raw == "" {
		return "", "", fmt.Errorf("empty path")
	}
	parser := pathParser{input: raw}
	// The caller only wants the prefix end, so no segments are collected and no
	// quoted key is materialized.
	if _, err := parser.parsePrefix(false); err != nil {
		return "", "", err
	}
	return raw[:parser.pos], raw[parser.pos:], nil
}

type pathParser struct {
	input string
	// pos is the byte offset of the next character to read.
	pos int
}

// parsePrefix parses the leading path. With collect false only the end offset is
// wanted (CutPathPrefix), so no segments are built.
//
// A path is either a namespace selector (@process...) or a root expression; a
// leading bracket makes a top-level special key reachable, as in ["a.b"].k.
func (p *pathParser) parsePrefix(collect bool) ([]Segment, error) {
	var segments []Segment
	if collect {
		// One allocation sized from the separators actually present: the slice
		// used to grow once per segment (three reallocations for a five-segment
		// path).
		segments = make([]Segment, 0,
			strings.Count(p.input, string(PathSeparator))+strings.Count(p.input, string(indexOpen))+2)
	}

	var err error
	if p.pos < len(p.input) && p.input[p.pos] == namespaceSigil {
		segments, err = p.parseNamespace(segments, collect)
	} else {
		segments, err = p.parseRootSegments(segments, collect)
	}
	if err != nil {
		return nil, err
	}
	return segments, nil
}

// parseNamespace parses the leading @namespace segment and everything addressed
// inside it.
func (p *pathParser) parseNamespace(segments []Segment, collect bool) ([]Segment, error) {
	p.pos++ // consume namespaceSigil
	start := p.pos
	name, err := p.parseIdentifier()
	if err != nil {
		return nil, fmt.Errorf("expected a namespace name after %q at offset %d", string(namespaceSigil), start)
	}
	if name != envNamespace {
		return nil, fmt.Errorf("unknown namespace %q at offset %d: expected %q", name, start, envNamespace)
	}
	if collect {
		segments = append(segments, Segment{Key: name, IsNamespace: true})
	}
	if p.pos < len(p.input) && p.input[p.pos] == PathSeparator {
		p.pos++
		return p.parseElementTail(segments, collect)
	}
	return p.parseIndexTail(segments, collect)
}

// parseRootSegments parses a root expression: an identifier, or a bracketed
// segment for a top-level special key.
func (p *pathParser) parseRootSegments(segments []Segment, collect bool) ([]Segment, error) {
	if p.pos < len(p.input) && p.input[p.pos] == indexOpen {
		segment, err := p.parseBracket(collect)
		if err != nil {
			return nil, err
		}
		if segment.IsIndex {
			return nil, fmt.Errorf("expected a quoted key at offset %d, not an array index", p.pos)
		}
		if collect {
			segments = append(segments, segment)
		}
		if p.pos >= len(p.input) {
			return segments, nil
		}
		switch p.input[p.pos] {
		case indexOpen:
			return p.parseIndexTail(segments, collect)
		case PathSeparator:
			p.pos++
			return p.parseElementTail(segments, collect)
		default:
			return segments, nil
		}
	}
	return p.parseElementTail(segments, collect)
}

// parseElementTail parses a named element and the chain that follows it:
// "[" index "]", "[" quoted "]" or "." identifier, repeatedly.
func (p *pathParser) parseElementTail(segments []Segment, collect bool) ([]Segment, error) {
	for {
		ident, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}
		if collect {
			segments = append(segments, Segment{Key: ident})
		}
		if p.pos >= len(p.input) {
			return segments, nil
		}
		switch p.input[p.pos] {
		case PathSeparator:
			p.pos++
		case indexOpen:
			segments, err = p.parseIndexTail(segments, collect)
			if err != nil {
				return nil, err
			}
			if p.pos >= len(p.input) || p.input[p.pos] != PathSeparator {
				return segments, nil
			}
			p.pos++
		default:
			return segments, nil
		}
	}
}

// parseIndexTail parses the bracket segments that follow a path or an element.
func (p *pathParser) parseIndexTail(segments []Segment, collect bool) ([]Segment, error) {
	for p.pos < len(p.input) && p.input[p.pos] == indexOpen {
		segment, err := p.parseBracket(collect)
		if err != nil {
			return nil, err
		}
		if collect {
			segments = append(segments, segment)
		}
	}
	return segments, nil
}

func (p *pathParser) parseIdentifier() (string, error) {
	if p.pos >= len(p.input) || !isIdentifierStart(p.input[p.pos]) {
		return "", fmt.Errorf("expected identifier at offset %d", p.pos)
	}
	start := p.pos
	p.pos++
	for p.pos < len(p.input) && isIdentifierContinue(p.input[p.pos]) {
		p.pos++
	}
	return p.input[start:p.pos], nil
}

func (p *pathParser) parseBracket(collect bool) (Segment, error) {
	p.pos++ // consume indexOpen
	if p.pos >= len(p.input) {
		return Segment{}, fmt.Errorf("unclosed index at offset %d", p.pos)
	}
	if p.input[p.pos] == quoteChar {
		key, err := p.parseQuoted(collect)
		if err != nil {
			return Segment{}, err
		}
		if p.pos >= len(p.input) || p.input[p.pos] != indexClose {
			return Segment{}, fmt.Errorf("expected ']' at offset %d", p.pos)
		}
		p.pos++
		return Segment{Key: key}, nil
	}

	start := p.pos
	for p.pos < len(p.input) && codec.IsDigit(rune(p.input[p.pos])) {
		p.pos++
	}
	if start == p.pos {
		return Segment{}, fmt.Errorf("expected array index at offset %d", p.pos)
	}
	if p.pos >= len(p.input) || p.input[p.pos] != indexClose {
		return Segment{}, fmt.Errorf("expected ']' at offset %d", p.pos)
	}
	index, err := strconv.Atoi(p.input[start:p.pos])
	if err != nil {
		return Segment{}, fmt.Errorf("invalid index %q: %w", p.input[start:p.pos], err)
	}
	p.pos++
	return Segment{Index: index, IsIndex: true}, nil
}

// parseQuoted scans a quoted key, validating it when collect is false instead of
// building the string.
func (p *pathParser) parseQuoted(collect bool) (string, error) {
	p.pos++ // consume quoteChar
	var b strings.Builder
	escaped := false
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		p.pos++
		if escaped {
			if collect {
				b.WriteByte(ch)
			}
			escaped = false
			continue
		}
		switch ch {
		case escapeChar:
			escaped = true
		case quoteChar:
			if !collect {
				return "", nil
			}
			return b.String(), nil
		default:
			if collect {
				b.WriteByte(ch)
			}
		}
	}
	return "", fmt.Errorf("unclosed quoted key")
}

// IsSimpleIdentifier reports whether key can be written unquoted: it is a single
// bare identifier, so a path that needs brackets (app["my-key"]) is not simple.
func IsSimpleIdentifier(key string) bool {
	if key == "" || !isIdentifierStart(key[0]) {
		return false
	}
	for i := 1; i < len(key); i++ {
		if !isIdentifierContinue(key[i]) {
			return false
		}
	}
	return true
}

func isIdentifierStart(ch byte) bool {
	return ch == underscoreChar || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isIdentifierContinue(ch byte) bool {
	return isIdentifierStart(ch) || codec.IsDigit(rune(ch))
}

// Namespace returns the namespace a parsed path selects, empty for a root path.
// Only the first segment can be a namespace selector.
func Namespace(segments []Segment) string {
	if len(segments) > 0 && segments[0].IsNamespace {
		return segments[0].Key
	}
	return ""
}

// EnvKey returns the environment variable name a namespace path addresses.
// Environment names are not identifiers: @process["MY-VAR"] is MY-VAR, and
// @process.a.b is the dotted name "a.b".
func EnvKey(segments []Segment) (string, bool) {
	if Namespace(segments) == "" {
		return "", false
	}
	parts := make([]string, 0, len(segments)-1)
	for _, segment := range segments[1:] {
		if segment.IsIndex {
			return "", false
		}
		parts = append(parts, segment.Key)
	}
	if len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, string(PathSeparator)), true
}

// RootKey returns the leading key of a path, or the namespace name for a
// namespace path. It is the single place a filter's root is derived, so
// --only-vars/--ignore-vars cannot drift from the grammar.
func RootKey(path string) string {
	segments, err := ParsePath(path)
	if err != nil {
		return malformedRootSegment(path)
	}
	if len(segments) == 0 {
		return ""
	}
	return segments[0].Key
}

// malformedRootSegment extracts the root of a path the parser rejected.
func malformedRootSegment(path string) string {
	root, _, _ := strings.Cut(path, string(PathSeparator))
	if open := strings.IndexByte(root, indexOpen); open >= 0 {
		root = root[:open]
	}
	return root
}

// CanonicalPath formats segments in template syntax, so diagnostics and --explain
// report a spelling that can be pasted back into a template. It keeps the @
// namespace sigil and the brackets a non-identifier key needs; EncodeSegments is
// the encoded key, which drops both.
func CanonicalPath(segments []Segment) string {
	var b strings.Builder
	// afterIndex allows the dot directly after a bracket, as in hosts[0].name; it
	// is set by the bracket branch below and cleared at the end of each step.
	afterElement := false
	afterIndex := false
	for _, segment := range segments {
		switch {
		case segment.IsIndex:
			// The next segment always sees afterIndex, and the loop tail clears
			// it, so no assignment is needed here.
			fmt.Fprintf(&b, indexFormat, segment.Index)
		case segment.IsNamespace:
			b.WriteByte(namespaceSigil)
			b.WriteString(segment.Key)
			afterElement = true
		case (afterElement || afterIndex) && IsSimpleIdentifier(segment.Key):
			b.WriteByte(PathSeparator)
			b.WriteString(segment.Key)
			afterElement = true
		case IsSimpleIdentifier(segment.Key):
			b.WriteString(segment.Key)
			afterElement = true
		default:
			// A bracket is required for a leading key that is not a plain
			// identifier: ${["a.b"]} is a top-level special key, and
			// app["my-key"] keeps its brackets.
			writeQuotedKey(&b, segment.Key)
			afterElement = true
		}
		afterIndex = false
	}
	return b.String()
}

// Key is the encoded address of a value in the root namespace: dots between named
// segments, [i] for an index, no @ sigil and no quotes. Value metadata is keyed by
// it, so a raw template path and an encoded key cannot be passed for one another
// by mistake. Diagnostics that name schema locations use their own notation
// (JSON Pointer) and only borrow IndexSuffix for the element bracket.
type Key string

// JoinPath appends a named key to an encoded path.
func JoinPath(prefix Key, key string) Key {
	if prefix == "" {
		return Key(key)
	}
	return prefix + Key(PathSeparator) + Key(key)
}

// IndexSuffix returns the bracket notation of one array element, as in "[0]". It
// is the single definition of that spelling: JoinIndex builds value addresses
// with it, schema diagnostics append it to keyword paths.
func IndexSuffix(index int) string {
	return fmt.Sprintf(indexFormat, index)
}

// JoinIndex appends an array index to an encoded path, as in extra[0].
func JoinIndex(prefix Key, index int) Key {
	return prefix + Key(IndexSuffix(index))
}

// ParentPath drops the last segment of a path, returning "" for a root path or a
// spelling the grammar rejects. The result is the canonical spelling, so it is
// itself a valid path: a quoted key keeps its brackets and can be walked again.
func ParentPath(path string) string {
	segments, err := ParsePath(path)
	if err != nil || len(segments) < 2 {
		return ""
	}
	return CanonicalPath(segments[:len(segments)-1])
}

// IsBelow reports whether path addresses a value strictly under prefix. Segments
// are compared, not string prefixes, so ["a.b"].c and a.b.c agree. A spelling the
// grammar rejects is never below anything.
func IsBelow(path, prefix string) bool {
	pathSegments, err := ParsePath(path)
	if err != nil {
		return false
	}
	prefixSegments, err := ParsePath(prefix)
	if err != nil || len(pathSegments) <= len(prefixSegments) {
		return false
	}
	for i, segment := range prefixSegments {
		if pathSegments[i].Key != segment.Key ||
			pathSegments[i].Index != segment.Index ||
			pathSegments[i].IsIndex != segment.IsIndex {
			return false
		}
	}
	return true
}

// writeQuotedKey appends one bracketed key segment.
func writeQuotedKey(b *strings.Builder, key string) {
	b.WriteByte(indexOpen)
	b.WriteByte(quoteChar)
	for i := 0; i < len(key); i++ {
		ch := key[i]
		if ch == quoteChar || ch == escapeChar {
			b.WriteByte(escapeChar)
		}
		b.WriteByte(ch)
	}
	b.WriteByte(quoteChar)
	b.WriteByte(indexClose)
}

// resolvePath resolves a template path. @process.NAME reads the process
// environment; any other path reads the root only, with no environment fallback.
func resolvePath(root map[string]any, env map[string]string, rawPath string) (any, bool, error) {
	// Fast path: large templates repeat a bare name hundreds of thousands of
	// times, so it skips the parser.
	if IsSimpleIdentifier(rawPath) {
		if value, ok := root[rawPath]; ok {
			return value, true, nil
		}
		return nil, false, nil
	}

	segments, err := ParsePath(rawPath)
	if err != nil {
		return nil, false, err
	}
	if Namespace(segments) != "" {
		return lookupNamespace(env, segments)
	}

	value, ok := lookup(root, segments)
	if ok {
		return value, true, nil
	}
	return nil, false, nil
}

// lookupNamespace resolves a namespace path such as @process.NAME.
func lookupNamespace(env map[string]string, segments []Segment) (any, bool, error) {
	name, ok := EnvKey(segments)
	if !ok {
		return nil, false, fmt.Errorf("namespace %q needs a variable name", Namespace(segments))
	}
	if value, found := env[name]; found {
		return value, true, nil
	}
	return nil, false, nil
}

func lookup(root map[string]any, path []Segment) (any, bool) {
	var current any = root
	for _, segment := range path {
		switch node := current.(type) {
		case map[string]any:
			if segment.IsIndex {
				return nil, false
			}
			value, ok := node[segment.Key]
			if !ok {
				return nil, false
			}
			current = value
		case []any:
			if !segment.IsIndex || segment.Index < 0 || segment.Index >= len(node) {
				return nil, false
			}
			current = node[segment.Index]
		default:
			return nil, false
		}
	}
	return current, true
}
