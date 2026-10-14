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
	pathSeparator  = '.'
	indexOpen      = '['
	indexClose     = ']'
	quoteChar      = '"'
	escapeChar     = '\\'
	underscoreChar = '_'
	// namespaceSigil introduces a namespace and can only appear as the first byte
	// of a path, so it can never collide with an object key.
	namespaceSigil = '@'
	// envNamespace is the only namespace: the read-only process environment.
	envNamespace    = "process"
	indexPathFormat = "%s[%d]"
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

// ParsePathPrefix parses a leading path and returns the remaining expression.
func ParsePathPrefix(raw string) (path string, rest string, err error) {
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
// wanted (ParsePathPrefix), so no segments are built.
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
			strings.Count(p.input, string(pathSeparator))+strings.Count(p.input, string(indexOpen))+2)
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
	if p.pos < len(p.input) && p.input[p.pos] == pathSeparator {
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
		case pathSeparator:
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
		case pathSeparator:
			p.pos++
		case indexOpen:
			segments, err = p.parseIndexTail(segments, collect)
			if err != nil {
				return nil, err
			}
			if p.pos >= len(p.input) || p.input[p.pos] != pathSeparator {
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

// IsSimpleIdentifier reports whether path is a single bare identifier.
func IsSimpleIdentifier(path string) bool {
	if path == "" || !isIdentifierStart(path[0]) {
		return false
	}
	for i := 1; i < len(path); i++ {
		if !isIdentifierContinue(path[i]) {
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
	return strings.Join(parts, string(pathSeparator)), true
}

// RootSegment returns the first segment as the --only-vars/--ignore-vars filters
// see it: the leading key, or the namespace name. It is the single place the root
// is derived, so the filters cannot drift from the grammar.
func RootSegment(path string) string {
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
	root, _, _ := strings.Cut(path, string(pathSeparator))
	if open := strings.IndexByte(root, indexOpen); open >= 0 {
		root = root[:open]
	}
	return root
}

// Canonical returns the path in template syntax, so diagnostics and --explain
// report a spelling that can be pasted back into a template. It keeps the @
// namespace sigil and the brackets a non-identifier key needs; EncodePath is the
// metadata key, which drops both.
func Canonical(segments []Segment) string {
	var b strings.Builder
	// afterIndex allows the dot directly after a bracket, as in hosts[0].name.
	afterElement := false
	afterIndex := false
	for _, segment := range segments {
		switch {
		case segment.IsIndex:
			fmt.Fprintf(&b, "[%d]", segment.Index)
			afterIndex = true
		case segment.IsNamespace:
			b.WriteByte(namespaceSigil)
			b.WriteString(segment.Key)
			afterElement = true
		case (afterElement || afterIndex) && IsSimpleIdentifier(segment.Key):
			b.WriteByte(pathSeparator)
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
