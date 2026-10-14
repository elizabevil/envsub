package render

import (
	"bytes"
	"strings"
	"testing"
)

// TestNormalizePrefix pins the bracket-to-dot rewrite that makes a quoted key and
// its dot spelling compare equal. Only a bracket holding a quoted key changes:
// an index and a plain key keep their spelling.
func TestNormalizePrefix(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ in, want string }{
		{in: "", want: ""},
		{in: "app.port", want: "app.port"},
		{in: "extra[0]", want: "extra[0]"},
		{in: `["a.b"]`, want: "a.b"},
		{in: `["a.b"].c`, want: "a.b.c"},
		{in: `app["my-key"]`, want: "app.my-key"},
		{in: `app["my-key"].deep`, want: "app.my-key.deep"},
		{in: `["x"]["y"]`, want: "x.y"},
		{in: `["unclosed`, want: `["unclosed`},
		{in: `a[""]`, want: "a"},
	} {
		if got := normalizePrefix(tc.in); got != tc.want {
			t.Errorf("normalizePrefix(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestParentPath pins the prefix step that closeOverPrefixes iterates: it drops
// the last segment, and both separators count, so a bracket element is dropped
// like a dotted one.
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
		{in: "[0]", want: ""},
	} {
		if got := parentPath(tc.in); got != tc.want {
			t.Errorf("parentPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestWriteUnused pins the report format: one path per line, newline terminated,
// and nothing at all when there is nothing unused.
func TestWriteUnused(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		in   []string
		want string
	}{
		{name: "empty", in: nil, want: ""},
		{name: "one", in: []string{"app.port"}, want: "app.port\n"},
		{name: "several", in: []string{"a", "b.c", "extra[0]"}, want: "a\nb.c\nextra[0]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			if err := writeUnused(&out, tc.in); err != nil {
				t.Fatalf("writeUnused() error = %v", err)
			}
			if out.String() != tc.want {
				t.Errorf("writeUnused() = %q, want %q", out.String(), tc.want)
			}
		})
	}
}

// failingWriter fails every write, so the error path of writeUnused stays
// covered whatever the report contains.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

var errWrite = errFailingWriter{}

type errFailingWriter struct{}

func (errFailingWriter) Error() string { return "write failed" }

// TestWriteUnusedReportsWriteFailure pins that a failed write is reported as an IO
// error rather than silently dropped.
func TestWriteUnusedReportsWriteFailure(t *testing.T) {
	t.Parallel()
	err := writeUnused(failingWriter{}, []string{"app.port"})
	if err == nil {
		t.Fatal("writeUnused() error = nil, want a write failure")
	}
	if !strings.Contains(err.Error(), "write unused report") {
		t.Errorf("error = %v, want it to name the report", err)
	}
}
