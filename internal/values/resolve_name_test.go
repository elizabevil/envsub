package values_test

import (
	"testing"

	"github.com/elizabevil/envsub/internal/values"
)

// TestResolveNameStaysInRoot pins the resolution contract: a bare name reads the
// root namespace only. The process environment is never a fallback, so a host
// variable cannot change what ${NAME} renders; it is reachable only through the
// @process namespace.
func TestResolveNameStaysInRoot(t *testing.T) {
	t.Parallel()
	tree := values.NewTree()
	tree.Root["APP"] = 8080
	tree.Env["HOME"] = "/home/x"
	tree.Env["EMPTY"] = ""
	tree.Env["配置"] = "yes"

	for _, name := range []string{"APP"} {
		if value, ok := tree.ResolveName(name); !ok || value != 8080 {
			t.Fatalf("ResolveName(%q) = (%#v, %v), want (8080, true)", name, value, ok)
		}
	}
	for _, name := range []string{"HOME", "EMPTY", "配置", "missing", ""} {
		if value, ok := tree.ResolveName(name); ok {
			t.Fatalf("ResolveName(%q) = (%#v, true), want it to miss: a bare name never reads the environment", name, value)
		}
	}

	// The namespace path reads the same environment the bare name refused to
	// consult, including an empty value (which is set, not missing).
	path, err := tree.CompilePath("@process.HOME")
	if err != nil {
		t.Fatalf("CompilePath() error = %v", err)
	}
	if value, ok := tree.ResolvePath(path); !ok || value != "/home/x" {
		t.Fatalf("ResolvePath(@process.HOME) = (%#v, %v), want (/home/x, true)", value, ok)
	}
	emptyPath, err := tree.CompilePath("@process.EMPTY")
	if err != nil {
		t.Fatalf("CompilePath() error = %v", err)
	}
	if value, ok := tree.ResolvePath(emptyPath); !ok || value != "" {
		t.Fatalf("ResolvePath(@process.EMPTY) = (%#v, %v), want (\"\", true)", value, ok)
	}

	// A root value of the same name is a different slot, not a shadow.
	tree.Root["HOME"] = "/root"
	if value, ok := tree.ResolveName("HOME"); !ok || value != "/root" {
		t.Fatalf("ResolveName(HOME) = (%#v, %v), want (/root, true)", value, ok)
	}
	if value, ok := tree.ResolvePath(path); !ok || value != "/home/x" {
		t.Fatalf("ResolvePath(@process.HOME) = (%#v, %v), want (/home/x, true) after a root HOME exists", value, ok)
	}

	if value, ok := tree.ResolveRootName([]byte("APP")); !ok || value != 8080 {
		t.Fatalf("ResolveRootName(APP) = (%#v, %v), want (8080, true)", value, ok)
	}
	if value, ok := tree.ResolveEnvName([]byte("HOME")); !ok || value != "/home/x" {
		t.Fatalf("ResolveEnvName(HOME) = (%#v, %v), want (/home/x, true)", value, ok)
	}

}
