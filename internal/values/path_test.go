package values_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/values"
)

func TestParsePath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
		want []values.Segment
	}{
		{name: "simple", raw: "app", want: []values.Segment{{Key: "app"}}},
		{name: "dot", raw: "app.port", want: []values.Segment{{Key: "app"}, {Key: "port"}}},
		{name: "index", raw: "hosts[0]", want: []values.Segment{{Key: "hosts"}, {Index: 0, IsIndex: true}}},
		{name: "quoted", raw: `app["my-key"]`, want: []values.Segment{{Key: "app"}, {Key: "my-key"}}},
		{name: "quoted dot", raw: `app["my.key"]`, want: []values.Segment{{Key: "app"}, {Key: "my.key"}}},
		{name: "top-level quoted", raw: `["a.b"]`, want: []values.Segment{{Key: "a.b"}}},
		{
			name: "top-level quoted then dot",
			raw:  `["a.b"].k`,
			want: []values.Segment{{Key: "a.b"}, {Key: "k"}},
		},
		{
			name: "top-level quoted then index",
			raw:  `["a-b"][0]`,
			want: []values.Segment{{Key: "a-b"}, {Index: 0, IsIndex: true}},
		},
		{
			name: "namespace",
			raw:  "@process.NAME",
			want: []values.Segment{{Key: "process", IsNamespace: true}, {Key: "NAME"}},
		},
		{
			name: "namespace quoted",
			raw:  `@process["MY-VAR"]`,
			want: []values.Segment{{Key: "process", IsNamespace: true}, {Key: "MY-VAR"}},
		},
		{
			name: "namespace dotted name",
			raw:  "@process.a.b",
			want: []values.Segment{{Key: "process", IsNamespace: true}, {Key: "a"}, {Key: "b"}},
		},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := values.ParsePath(tt.raw)
			if err != nil {
				t.Fatalf("ParsePath(%q) error = %v", tt.raw, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("ParsePath(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("ParsePath(%q)[%d] = %#v, want %#v", tt.raw, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestParsePathRejects pins the paths the grammar refuses: a namespace sigil or a
// bracket may only open a path, and a namespace must be known. Each case also
// pins the diagnostic fragment, byte offset included, so a rejected path still
// tells the user where the grammar gave up.
func TestParsePathRejects(t *testing.T) {
	t.Parallel()
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "a@b", want: `unexpected character '@' at offset 1`},
		{raw: "env.X@", want: `unexpected character '@' at offset 5`},
		{raw: "@env.K", want: `unknown namespace "env" at offset 1`},
		{raw: "@", want: `expected a namespace name after "@" at offset 1`},
		{raw: "@[0]", want: `expected a namespace name after "@" at offset 1`},
		{raw: "[0]", want: "expected a quoted key at offset 3, not an array index"},
		{raw: `["a"`, want: "expected ']' at offset 4"},
		{raw: "a..b", want: "expected identifier at offset 2"},
		{raw: "", want: "empty path"},
		{raw: "a[", want: "unclosed index at offset 2"},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			t.Parallel()
			_, err := values.ParsePath(tt.raw)
			if err == nil {
				t.Fatalf("ParsePath(%q) error = nil, want an error", tt.raw)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ParsePath(%q) error = %q, want it to contain %q", tt.raw, err, tt.want)
			}
		})
	}
}

func TestTreeResolve(t *testing.T) {
	t.Parallel()
	tree := values.NewTree()
	tree.Root["app"] = map[string]any{
		"port":   8080,
		"hosts":  []any{"a", "b"},
		"my-key": "value",
	}
	tree.Root["env"] = map[string]any{"PORT": "from_root_env_key"}
	tree.Root["a.b"] = "dotted"
	tree.Env["PORT"] = "9090"
	tree.Env["APP_PORT"] = "7070"

	tests := []struct {
		name  string
		path  string
		want  any
		found bool
	}{
		{name: "root", path: "app.port", want: 8080, found: true},
		{name: "index", path: "app.hosts[1]", want: "b", found: true},
		{name: "quoted", path: `app["my-key"]`, want: "value", found: true},
		// "env" is an ordinary root key: ${env.PORT} is a root path, not a
		// namespace, so it reads the root object even though PORT exists in the
		// process environment.
		{name: "root env key", path: "env.PORT", want: "from_root_env_key", found: true},
		{name: "namespace", path: "@process.PORT", want: "9090", found: true},
		{name: "namespace other", path: "@process.APP_PORT", want: "7070", found: true},
		{name: "namespace missing", path: "@process.ABSENT", want: nil, found: false},
		{name: "top-level quoted", path: `["a.b"]`, want: "dotted", found: true},
		// No environment fallback: a bare name missing from the root is missing.
		{name: "no env fallback", path: "APP_PORT", want: nil, found: false},
		{name: "missing", path: "nosuchroot.missing", want: nil, found: false},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, found, err := tree.Resolve(tt.path)
			if err != nil {
				t.Fatalf("Resolve(%q) error = %v", tt.path, err)
			}
			if found != tt.found {
				t.Fatalf("Resolve(%q) found = %v, want %v", tt.path, found, tt.found)
			}
			if got != tt.want {
				t.Fatalf("Resolve(%q) = %#v, want %#v", tt.path, got, tt.want)
			}
		})
	}
}

func TestCutPathPrefix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
		path string
		rest string
	}{
		{name: "plain", raw: "A", path: "A", rest: ""},
		{name: "operator", raw: "A:-x", path: "A", rest: ":-x"},
		{name: "dash operator", raw: "A-x", path: "A", rest: "-x"},
		{name: "nested path", raw: "app.port:?boom", path: "app.port", rest: ":?boom"},
		{name: "quoted key", raw: `app["my-key"]:-x`, path: `app["my-key"]`, rest: ":-x"},
		{name: "top-level quoted", raw: `["a.b"].k | toJson`, path: `["a.b"].k`, rest: " | toJson"},
		{name: "namespace", raw: "@process.NAME", path: "@process.NAME", rest: ""},
		{name: "namespace operator", raw: "@process.NAME:-x", path: "@process.NAME", rest: ":-x"},
		{name: "namespace quoted", raw: `@process["MY-VAR"] | quote`, path: `@process["MY-VAR"]`, rest: " | quote"},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path, rest, err := values.CutPathPrefix(tt.raw)
			if err != nil {
				t.Fatalf("CutPathPrefix(%q) error = %v", tt.raw, err)
			}
			if path != tt.path || rest != tt.rest {
				t.Fatalf("CutPathPrefix(%q) = (%q, %q), want (%q, %q)", tt.raw, path, rest, tt.path, tt.rest)
			}
		})
	}
}

// TestParsePathPrefixAgreesWithParsePath pins that the prefix-only scan (which
// collects no segments) consumes exactly the bytes ParsePath parses, and keeps
// the same errors.
func TestCutPathPrefixAgreesWithParsePath(t *testing.T) {
	t.Parallel()
	inputs := []string{
		"app", "app.port", "app.servers[0]", `app["my-key"]`, `app["a\"b"]`,
		`app.labels["x"] | toJson`, "a[10].b",
	}
	for _, input := range inputs {
		path, rest, err := values.CutPathPrefix(input)
		if err != nil {
			t.Fatalf("CutPathPrefix(%q) error = %v", input, err)
		}
		if path+rest != input {
			t.Fatalf("CutPathPrefix(%q) = (%q, %q), want the pieces to rebuild the input", input, path, rest)
		}
		segments, err := values.ParsePath(path)
		if err != nil {
			t.Fatalf("ParsePath(%q) (prefix of %q) error = %v", path, input, err)
		}
		if len(segments) == 0 {
			t.Fatalf("ParsePath(%q) returned no segments", path)
		}
	}

	for _, bad := range []string{"", "app[", `app["x`, "app..b"} {
		if _, _, err := values.CutPathPrefix(bad); err == nil {
			t.Fatalf("CutPathPrefix(%q) error = nil, want error", bad)
		}
	}
}

// TestCanonical pins the spelling --explain reports: template syntax a reader can
// paste back, which is why EncodePath (a metadata key, unquoted) is not used.
func TestCanonicalPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		raw string
	}{
		{"NAME"},
		{"app.port"},
		{"hosts[0]"},
		{"app.hosts[1].name"},
		{`app["my-key"]`},
		{`app["my key"]`},
		{`["a.b"]`},
		{`["a.b"].k`},
		{`["a-b"][0]`},
		{"@process.NAME"},
		{`@process["MY-VAR"]`},
		{"@process.a.b"},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			t.Parallel()
			segments, err := values.ParsePath(tt.raw)
			if err != nil {
				t.Fatalf("ParsePath(%q) error = %v", tt.raw, err)
			}
			got := values.CanonicalPath(segments)
			if got != tt.raw {
				t.Fatalf("CanonicalPath(%q) = %q, want the spelling it parsed from", tt.raw, got)
			}
			// The canonical form must parse back to the same segments.
			again, err := values.ParsePath(got)
			if err != nil {
				t.Fatalf("ParsePath(CanonicalPath(%q)) = %q error = %v", tt.raw, got, err)
			}
			if !reflect.DeepEqual(again, segments) {
				t.Fatalf("round trip of %q: %#v, want %#v", tt.raw, again, segments)
			}
		})
	}
}

// TestCanonicalEscapesQuotes pins that a key which itself contains a quote stays
// expressible: the canonical form escapes it rather than emitting invalid syntax.
func TestCanonicalPathEscapesQuotes(t *testing.T) {
	t.Parallel()
	segments := []values.Segment{{Key: `a"b`}, {Key: "c"}}
	got := values.CanonicalPath(segments)
	if want := `["a\"b"].c`; got != want {
		t.Fatalf("CanonicalPath(%#v) = %q, want %q", segments, got, want)
	}
}

// TestRootSegment pins the root the Only/Ignore filters see: the leading key for a
// root path, the namespace name for a namespace path, and the extracted text for a
// path the parser rejects (which keeps the pre-grammar filtering behavior).
func TestRootKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		want string
	}{
		{"NAME", "NAME"},
		{"app.port", "app"},
		{"hosts[0]", "hosts"},
		{`app["my-key"]`, "app"},
		{`["a.b"].k`, "a.b"},
		{"@process.NAME", "process"},
		{`@process["MY-VAR"]`, "process"},
		{"", ""},
		// A path the parser rejects still filters like it used to.
		{"a[b", "a"},
		{"a..b", "a"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			if got := values.RootKey(tt.path); got != tt.want {
				t.Fatalf("RootKey(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// TestJoinPath pins the dotted join: an empty prefix yields the key alone, so a
// caller never has to special-case the root.
func TestJoinPath(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		prefix values.Key
		key    string
		want   values.Key
	}{
		{prefix: "", key: "app", want: "app"},
		{prefix: "app", key: "port", want: "app.port"},
		{prefix: "app.labels", key: "my-key", want: "app.labels.my-key"},
	} {
		if got := values.JoinPath(tc.prefix, tc.key); got != tc.want {
			t.Errorf("JoinPath(%q, %q) = %q, want %q", tc.prefix, tc.key, got, tc.want)
		}
	}
}

// TestJoinIndex pins the bracket form, which re-parses as an index segment.
func TestJoinIndex(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		prefix values.Key
		index  int
		want   values.Key
	}{
		{prefix: "extra", index: 0, want: "extra[0]"},
		{prefix: "app.hosts", index: 12, want: "app.hosts[12]"},
	} {
		got := values.JoinIndex(tc.prefix, tc.index)
		if got != tc.want {
			t.Errorf("JoinIndex(%q, %d) = %q, want %q", tc.prefix, tc.index, got, tc.want)
		}
		segments, err := values.ParsePath(string(got))
		if err != nil {
			t.Fatalf("ParsePath(%q) error = %v", got, err)
		}
		if last := segments[len(segments)-1]; !last.IsIndex || last.Index != tc.index {
			t.Errorf("ParsePath(%q) last segment = %+v, want index %d", got, last, tc.index)
		}
	}
}

// TestParentPath pins the walk used by closeOverPrefixes: the result is itself a
// valid path (a quoted key keeps its brackets), and a root or malformed spelling
// yields "".
func TestParentPath(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ in, want string }{
		{in: "", want: ""},
		{in: "app", want: ""},
		{in: "app.port", want: "app"},
		{in: "app.port.deep", want: "app.port"},
		{in: "extra[0]", want: "extra"},
		{in: `app["my-key"]`, want: "app"},
		{in: `app["my-key"].deep`, want: `app["my-key"]`},
		{in: `["a.b"].c`, want: `["a.b"]`},
		{in: `["unclosed`, want: ""},
	} {
		if got := values.ParentPath(tc.in); got != tc.want {
			t.Errorf("ParentPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestIsBelow pins the segment comparison: a quoted key and its dot spelling are
// the same address, an equal path is not below itself, and a malformed spelling
// is below nothing.
func TestIsBelow(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		path, prefix string
		want         bool
	}{
		{path: "app.port", prefix: "app", want: true},
		{path: "app.port.deep", prefix: "app.port", want: true},
		{path: "extra[0]", prefix: "extra", want: true},
		{path: `app["my-key"].deep`, prefix: `app["my-key"]`, want: true},
		// The comparison key of a quoted key is not itself a path, so it cannot
		// be used as a prefix: only grammar-valid spellings compare.
		{path: `app["my-key"].deep`, prefix: "app.my-key", want: false},
		{path: `app["a.b"]`, prefix: "app.a.b", want: false},
		{path: "app", prefix: "app", want: false},
		{path: "app.port", prefix: "app.port", want: false},
		{path: "apple.port", prefix: "app", want: false},
		{path: "app", prefix: "app.port", want: false},
		{path: `["unclosed`, prefix: "app", want: false},
		{path: "app.port", prefix: `["unclosed`, want: false},
	} {
		if got := values.IsBelow(tc.path, tc.prefix); got != tc.want {
			t.Errorf("IsBelow(%q, %q) = %v, want %v", tc.path, tc.prefix, got, tc.want)
		}
	}
}
