// Package source loads external values and streams template input.
package source

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/elizabevil/envsub/internal/codec"
	"github.com/elizabevil/envsub/internal/values"
)

// .env syntax.
const (
	// exportPrefix marks an exported assignment.
	exportPrefix = "export "
	// assignmentSeparator splits a key from its value.
	assignmentSeparator = "="
	// commentChar starts a comment.
	commentChar = '#'
	// doubleQuote and singleQuote delimit quoted values.
	doubleQuote = '"'
	singleQuote = '\''
	// escapeChar escapes the next byte inside a double-quoted value.
	escapeChar = '\\'
	// spaceChar and tabChar separate a value from an inline comment.
	spaceChar = ' '
	tabChar   = '\t'
	// underscoreChar is the non-letter byte allowed in keys.
	underscoreChar = '_'
	// newlineChar and returnChar are the escapes decoded in values.
	newlineChar = '\n'
	returnChar  = '\r'
	// scanBufferSize and maxLineSize bound the line scanner.
	scanBufferSize = 64 * 1024
	maxLineSize    = 4 * 1024 * 1024
)

// Environ parses dotenv content. It implements values.Loader.
type Environ struct{}

// DotenvName is the diagnostic name of the .env loader.
const DotenvName = "dotenv"

// Name implements values.Loader.
func (Environ) Name() string { return DotenvName }

// Load parses .env content into a tree whose root holds the key/value pairs.
// Values stay strings; JSON materialization happens later, driven by the schema.
//
// A .env file is a configuration layer, so its keys are root keys, exactly like
// a YAML/JSON value file. Each key records its own line number as metadata, so
// diagnostics and --explain can name the winning line.
func (Environ) Load(data []byte) (*values.Tree, error) {
	env, err := ParseDotenv(data)
	if err != nil {
		return nil, err
	}
	tree := values.NewTree()
	for key, entry := range env {
		tree.Root[key] = entry.Value
		tree.RecordEncodedMeta(key, values.Meta{
			Line: entry.Line,
			Kind: DotenvName,
		})
	}
	return tree, nil
}

// DotenvEntry is one parsed .env assignment.
type DotenvEntry struct {
	// Value is the decoded value, with quotes and escapes resolved.
	Value string
	// Line is the 1-based line the assignment was read from.
	Line int
}

// ParseDotenv parses .env content into string key/value pairs with line
// numbers. Keys must be unique; a duplicate key is reported with its line.
func ParseDotenv(data []byte) (map[string]DotenvEntry, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, scanBufferSize), maxLineSize)
	env := map[string]DotenvEntry{}
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		key, value, ok, err := ParseDotenvLine(scanner.Text())
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
		if !ok {
			continue
		}
		if _, exists := env[key]; exists {
			return nil, fmt.Errorf("line %d: duplicate key %q", lineNo, key)
		}
		env[key] = DotenvEntry{Value: value, Line: lineNo}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read dotenv: %w", err)
	}
	return env, nil
}

// ParseDotenvLine parses one .env line. It reports ok=false for blank lines
// and comments.
func ParseDotenvLine(line string) (key string, value string, ok bool, err error) {
	line = strings.TrimSpace(line)
	if line == "" || line[0] == commentChar {
		return "", "", false, nil
	}
	if rest, found := strings.CutPrefix(line, exportPrefix); found {
		line = strings.TrimSpace(rest)
	}

	rawKey, rawValue, hasValue := strings.Cut(line, assignmentSeparator)
	key = strings.TrimSpace(rawKey)
	if !isDotenvKey(key) {
		return "", "", false, fmt.Errorf("invalid key %q", key)
	}
	if !hasValue {
		return key, "", true, nil
	}
	value, err = parseDotenvValue(strings.TrimSpace(rawValue))
	if err != nil {
		return "", "", false, err
	}
	return key, value, true, nil
}

// parseDotenvValue decodes one .env value according to the .env dialect: double
// quotes process \n, \r and \t and keep the character of any other escape,
// single quotes are fully literal, and unquoted text is cut at an inline
// comment.
//
// strconv.Unquote is not an equivalent replacement: it rejects \q and \0 that
// .env keeps as q and 0, expands \xNN, \uNNNN and octal escapes that .env
// leaves verbatim, and treats single quotes as Go rune literals.
func parseDotenvValue(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	switch raw[0] {
	case doubleQuote:
		return parseDoubleQuoted(raw)
	case singleQuote:
		return parseSingleQuoted(raw)
	default:
		return stripInlineComment(raw), nil
	}
}

func parseDoubleQuoted(raw string) (string, error) {
	var b strings.Builder
	escaped := false
	for i := 1; i < len(raw); i++ {
		ch := raw[i]
		if escaped {
			switch ch {
			case 'n':
				b.WriteByte(newlineChar)
			case 'r':
				b.WriteByte(returnChar)
			case 't':
				b.WriteByte(tabChar)
			default:
				b.WriteByte(ch)
			}
			escaped = false
			continue
		}
		switch ch {
		case escapeChar:
			escaped = true
		case doubleQuote:
			if strings.TrimSpace(raw[i+1:]) != "" {
				return "", fmt.Errorf("unexpected characters after closing quote")
			}
			return b.String(), nil
		default:
			b.WriteByte(ch)
		}
	}
	return "", fmt.Errorf("unclosed double quote")
}

func parseSingleQuoted(raw string) (string, error) {
	inner, rest, found := strings.Cut(raw[1:], string(singleQuote))
	if !found {
		return "", fmt.Errorf("unclosed single quote")
	}
	if strings.TrimSpace(rest) != "" {
		return "", fmt.Errorf("unexpected characters after closing quote")
	}
	return inner, nil
}

func stripInlineComment(raw string) string {
	for i := 0; i < len(raw); i++ {
		if raw[i] != commentChar {
			continue
		}
		if i == 0 || raw[i-1] == spaceChar || raw[i-1] == tabChar {
			raw = raw[:i]
			break
		}
	}
	return strings.TrimSpace(raw)
}

func isDotenvKey(key string) bool {
	if key == "" {
		return false
	}
	for i := 0; i < len(key); i++ {
		ch := key[i]
		switch {
		case ch == underscoreChar:
		case ch >= 'a' && ch <= 'z':
		case ch >= 'A' && ch <= 'Z':
		case i > 0 && codec.IsDigit(rune(ch)):
		default:
			return false
		}
	}
	return true
}
