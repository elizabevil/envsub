package values_test

import (
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/values"
)

// TestEnvKey pins the environment spelling of a namespace path: only @process
// addresses the process environment, root paths have none, and the variable name
// is the path inside the namespace (dots included, because POSIX allows them).
func TestEnvKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		raw  string
		want string
		ok   bool
	}{
		{raw: "@process.PORT", want: "PORT", ok: true},
		{raw: "@process.Home", want: "Home", ok: true},
		{raw: `@process["MY-VAR"]`, want: "MY-VAR", ok: true},
		{raw: `@process["a.b"]`, want: "a.b", ok: true},
		{raw: "@process.a.b", want: "a.b", ok: true},
		{raw: "PORT", ok: false},
		{raw: "env.PORT", ok: false},
		{raw: "hosts[0]", ok: false},
		{raw: "@process", ok: false},
		{raw: "@process[0]", ok: false},
		{raw: "", ok: false},
	}
	for _, tt := range tests {
		segments, err := values.ParsePath(tt.raw)
		if err != nil {
			if tt.ok {
				t.Fatalf("ParsePath(%q) error = %v", tt.raw, err)
			}
			continue
		}
		got, ok := values.EnvKey(segments)
		if ok != tt.ok || got != tt.want {
			t.Fatalf("EnvKey(%q) = (%q, %v), want (%q, %v)", tt.raw, got, ok, tt.want, tt.ok)
		}
	}
}

// TestSetPathRejectsNamespace pins that the read-only namespace cannot be
// written: --set and the assignment operators target the root only, so a
// namespace path is a usage error instead of silently writing an unreachable
// root key.
func TestSetPathRejectsNamespace(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"@process.K", `@process["MY-VAR"]`, "@process.a.b"} {
		tree := values.NewTree()
		err := tree.SetPath(raw, "v", values.SetString, values.Meta{Kind: values.SourceSet})
		if err == nil {
			t.Fatalf("SetPath(%q) error = nil, want a read-only namespace error", raw)
		}
		if !strings.Contains(err.Error(), "read-only") {
			t.Fatalf("SetPath(%q) error = %q, want it to say the namespace is read-only", raw, err)
		}
		if len(tree.Root) != 0 {
			t.Fatalf("SetPath(%q) wrote %#v into the root", raw, tree.Root)
		}
	}
}
