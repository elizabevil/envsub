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

func TestParsePathPrefix(t *testing.T) {
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
			path, rest, err := values.ParsePathPrefix(tt.raw)
			if err != nil {
				t.Fatalf("ParsePathPrefix(%q) error = %v", tt.raw, err)
			}
			if path != tt.path || rest != tt.rest {
				t.Fatalf("ParsePathPrefix(%q) = (%q, %q), want (%q, %q)", tt.raw, path, rest, tt.path, tt.rest)
			}
		})
	}
}

// TestParsePathPrefixAgreesWithParsePath pins that the prefix-only scan (which
// collects no segments) consumes exactly the bytes ParsePath parses, and keeps
// the same errors.
func TestParsePathPrefixAgreesWithParsePath(t *testing.T) {
	t.Parallel()
	inputs := []string{
		"app", "app.port", "app.servers[0]", `app["my-key"]`, `app["a\"b"]`,
		`app.labels["x"] | toJson`, "a[10].b",
	}
	for _, input := range inputs {
		path, rest, err := values.ParsePathPrefix(input)
		if err != nil {
			t.Fatalf("ParsePathPrefix(%q) error = %v", input, err)
		}
		if path+rest != input {
			t.Fatalf("ParsePathPrefix(%q) = (%q, %q), want the pieces to rebuild the input", input, path, rest)
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
		if _, _, err := values.ParsePathPrefix(bad); err == nil {
			t.Fatalf("ParsePathPrefix(%q) error = nil, want error", bad)
		}
	}
}

// TestCanonical pins the spelling --explain reports: template syntax a reader can
// paste back, which is why EncodePath (a metadata key, unquoted) is not used.
func TestCanonical(t *testing.T) {
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
			got := values.Canonical(segments)
			if got != tt.raw {
				t.Fatalf("Canonical(%q) = %q, want the spelling it parsed from", tt.raw, got)
			}
			// The canonical form must parse back to the same segments.
			again, err := values.ParsePath(got)
			if err != nil {
				t.Fatalf("ParsePath(Canonical(%q)) = %q error = %v", tt.raw, got, err)
			}
			if !reflect.DeepEqual(again, segments) {
				t.Fatalf("round trip of %q: %#v, want %#v", tt.raw, again, segments)
			}
		})
	}
}

// TestCanonicalEscapesQuotes pins that a key which itself contains a quote stays
// expressible: the canonical form escapes it rather than emitting invalid syntax.
func TestCanonicalEscapesQuotes(t *testing.T) {
	t.Parallel()
	segments := []values.Segment{{Key: `a"b`}, {Key: "c"}}
	got := values.Canonical(segments)
	if want := `["a\"b"].c`; got != want {
		t.Fatalf("Canonical(%#v) = %q, want %q", segments, got, want)
	}
}

// TestRootSegment pins the root the Only/Ignore filters see: the leading key for a
// root path, the namespace name for a namespace path, and the extracted text for a
// path the parser rejects (which keeps the pre-grammar filtering behavior).
func TestRootSegment(t *testing.T) {
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
			if got := values.RootSegment(tt.path); got != tt.want {
				t.Fatalf("RootSegment(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
