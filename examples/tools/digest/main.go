// Command digest compares a rendered tree with its checked-in expectation and
// verifies that a document parses as the format it claims.
//
// It exists so the examples' checks work the same way on every platform: the
// justfile never shells out to diff, cmp, wc or sha256sum.
//
// Usage:
//
//	digest --expect DIR --got DIR    compare two trees file by file
//	digest --parse json --file F     F must be valid JSON
//	digest --parse yaml --file F     F must be valid YAML
//	digest --digest DIR              print one line per file with its hash
//	digest --sources FILE            summarise an --explain log: one line per value
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/elizabevil/envsub/internal/yamlx"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run parses the flags and performs exactly one operation.
func run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("digest", flag.ContinueOnError)
	flags.SetOutput(stderr)
	expect := flags.String("expect", "", "expected tree")
	got := flags.String("got", "", "rendered tree")
	parse := flags.String("parse", "", "format to parse: json or yaml")
	file := flags.String("file", "", "file to parse, or - for stdin")
	digestDir := flags.String("digest", "", "tree to describe")
	sources := flags.String("sources", "", "explain log to summarise, or - for stdin")
	if err := flags.Parse(args); err != nil {
		return err
	}

	switch {
	case *sources != "":
		return summariseSources(*sources, stdout)
	case *digestDir != "":
		entries, err := treeDigest(*digestDir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			_, _ = fmt.Fprintf(stdout, "%s  %s\n", entry.hash, entry.path)
		}
		return nil
	case *parse != "":
		return parseFile(*parse, *file, stdout)
	case *expect != "" || *got != "":
		if *expect == "" || *got == "" {
			return errors.New("--expect and --got must be given together")
		}
		return compareTrees(*expect, *got, stdout)
	default:
		return errors.New("nothing to do: use --expect/--got, --parse or --digest")
	}
}

// digestEntry is one file of a tree with its content hash.
type digestEntry struct {
	path string
	hash string
}

// treeDigest hashes every regular file under root, keyed by its relative path.
func treeDigest(root string) ([]digestEntry, error) {
	var entries []digestEntry
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		sum, err := fileDigest(path)
		if err != nil {
			return err
		}
		entries = append(entries, digestEntry{path: filepath.ToSlash(relative), hash: sum})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })
	return entries, nil
}

// fileDigest returns the hex SHA-256 of one file.
func fileDigest(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// compareTrees reports every file that is missing, extra or different, and fails
// when there is any difference.
func compareTrees(expectDir, gotDir string, stdout io.Writer) error {
	expect, err := treeDigest(expectDir)
	if err != nil {
		return fmt.Errorf("read expected tree: %w", err)
	}
	got, err := treeDigest(gotDir)
	if err != nil {
		return fmt.Errorf("read rendered tree: %w", err)
	}
	expectIndex := index(expect)
	gotIndex := index(got)

	var problems []string
	for path, want := range expectIndex {
		have, ok := gotIndex[path]
		switch {
		case !ok:
			problems = append(problems, "missing: "+path)
		case have != want:
			problems = append(problems, "differs: "+path)
		}
	}
	for path := range gotIndex {
		if _, ok := expectIndex[path]; !ok {
			problems = append(problems, "unexpected: "+path)
		}
	}
	sort.Strings(problems)
	for _, problem := range problems {
		_, _ = fmt.Fprintln(stdout, problem)
	}
	if len(problems) > 0 {
		return fmt.Errorf("%d file(s) do not match", len(problems))
	}
	_, _ = fmt.Fprintf(stdout, "%d file(s) match\n", len(expect))
	return nil
}

// index turns a digest list into a path -> hash map.
func index(entries []digestEntry) map[string]string {
	out := make(map[string]string, len(entries))
	for _, entry := range entries {
		out[entry.path] = entry.hash
	}
	return out
}

// parseFile verifies that the named file is a document of the requested format.
// The parsed document is summarized on stdout so a failing run says what it read.
func parseFile(format, path string, stdout io.Writer) error {
	data, err := readInput(path)
	if err != nil {
		return err
	}
	switch format {
	case "json":
		var decoded any
		if err := json.Unmarshal(data, &decoded); err != nil {
			return fmt.Errorf("%s is not valid JSON: %w", path, err)
		}
		_, _ = fmt.Fprintf(stdout, "%s: valid JSON (%s)\n", path, summarize(decoded))
	case "yaml":
		var decoded any
		if err := yamlx.Unmarshal(data, &decoded); err != nil {
			return fmt.Errorf("%s is not valid YAML: %w", path, err)
		}
		_, _ = fmt.Fprintf(stdout, "%s: valid YAML (%s)\n", path, summarize(decoded))
	default:
		return fmt.Errorf("unsupported --parse format %q: want json or yaml", format)
	}
	return nil
}

// readInput reads a file, or stdin when the path is "-".
func readInput(path string) ([]byte, error) {
	if path == "" {
		return nil, errors.New("--file is required with --parse")
	}
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

// summarize describes a decoded document in one phrase.
func summarize(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return fmt.Sprintf("%d keys: %s", len(keys), strings.Join(keys, ", "))
	case []any:
		return fmt.Sprintf("%d items", len(typed))
	default:
		return fmt.Sprintf("%T", value)
	}
}

// explainEntry is one line of the --explain log.
type explainEntry struct {
	Template string         `json:"template"`
	Variable string         `json:"variable"`
	Origin   explainFrame   `json:"origin"`
	Shadowed []explainFrame `json:"shadowed"`
}

// explainEvent is the shared JSONL envelope; the provenance lives in data.
type explainEvent struct {
	Schema int          `json:"schema"`
	Type   string       `json:"type"`
	Data   explainEntry `json:"data"`
}

// explainFrame is one step of a value's lineage (ADR-005).
type explainFrame struct {
	Kind   string `json:"kind"`
	Method string `json:"method"`
	Source string `json:"source"`
	File   string `json:"file"`
	Line   int    `json:"line"`
}

// where renders a frame as one compact line: "kind/method source:line". Paths are
// shown relative to the repository root so the report reads the same on every
// machine and can be pasted into an issue.
func (f explainFrame) where() string {
	location := normalizePrefix(f.Source)
	if location == "" {
		location = normalizePrefix(f.File)
	}
	if location == "" {
		return f.Kind + "/" + f.Method
	}
	if f.Line > 0 {
		return f.Kind + "/" + f.Method + " " + location + ":" + strconv.Itoa(f.Line)
	}
	return f.Kind + "/" + f.Method + " " + location
}

// repoRoot is the working directory when it holds go.mod, computed once. The
// examples run the tool from the repository root, so this is the prefix to strip.
var repoRoot = sync.OnceValue(func() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
})

// normalizePrefix makes a path relative to the repository root when it is inside
// it, so a report never leaks the local checkout location. A path outside the
// root (or when the root is unknown) is returned unchanged.
func normalizePrefix(path string) string {
	root := repoRoot()
	if path == "" || root == "" {
		return path
	}
	// IsLocal rejects both an escaping path ("..") and an absolute one, so one
	// call covers the "outside the repository" case.
	relative, err := filepath.Rel(root, path)
	if err != nil || !filepath.IsLocal(relative) {
		return path
	}
	return filepath.ToSlash(relative)
}

// summariseSources reads an --explain log and prints the provenance of every
// value, so a reader can see where a golden's lines came from without running
// the render again by hand. Lines are grouped by kind, then by winning source.
func summariseSources(path string, stdout io.Writer) error {
	var reader io.Reader
	if path == "-" {
		reader = os.Stdin
	} else {
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()
		reader = file
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	entries := make([]explainEntry, 0, 64)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event explainEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return fmt.Errorf("explain line %d: %w", len(entries)+1, err)
		}
		if event.Type != "explain" {
			continue
		}
		entries = append(entries, event.Data)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if len(entries) == 0 {
		return fmt.Errorf("no explain entries in %s", path)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Origin.Kind != entries[j].Origin.Kind {
			return entries[i].Origin.Kind < entries[j].Origin.Kind
		}
		if entries[i].Origin.Source != entries[j].Origin.Source {
			return entries[i].Origin.Source < entries[j].Origin.Source
		}
		return entries[i].Variable < entries[j].Variable
	})

	_, _ = fmt.Fprintf(stdout, "%d values referenced\n", len(entries))
	kind, source := "", ""
	for _, entry := range entries {
		if entry.Origin.Kind != kind {
			kind, source = entry.Origin.Kind, ""
			_, _ = fmt.Fprintf(stdout, "\n[%s]\n", kind)
		}
		if entry.Origin.Source != source {
			source = entry.Origin.Source
			if where := normalizePrefix(source); where != "" {
				_, _ = fmt.Fprintf(stdout, "  from %s\n", where)
			}
		}
		_, _ = fmt.Fprintf(stdout, "    %s   <- %s", entry.Variable, entry.Origin.where())
		if len(entry.Shadowed) > 0 {
			parts := make([]string, 0, len(entry.Shadowed))
			for _, frame := range entry.Shadowed {
				parts = append(parts, frame.where())
			}
			_, _ = fmt.Fprintf(stdout, "   (covered: %s)", strings.Join(parts, " | "))
		}
		_, _ = fmt.Fprintln(stdout)
	}
	return nil
}
