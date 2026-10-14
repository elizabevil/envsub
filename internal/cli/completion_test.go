package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/cli"
)

// TestCompletionCommandIsWired pins the completion entry point: it is the only
// way a user discovers flags without reading --help, and Cobra stops offering it
// as soon as a command tree replaces the default one.
//
// The generated script content is not asserted here: Cobra writes it straight to
// the process stdout, which a test cannot capture through the command's writers.
// The part that drifts with the command tree is the dynamic lookup, covered by
// TestCompletionSuggestsRenderFlags.
func TestCompletionCommandIsWired(t *testing.T) {
	t.Parallel()
	cmd := cli.NewRootCommand(stubEngine{}, noopRunner)
	// The default commands are installed lazily, before Execute resolves a
	// command name; install them so Find sees the same tree a user does.
	cmd.InitDefaultHelpCmd()
	cmd.InitDefaultCompletionCmd()
	found, _, err := cmd.Find([]string{"completion"})
	if err != nil || found.Name() != "completion" {
		t.Fatalf("Find(completion) = %v, %v; want the completion command", found, err)
	}
}

// TestCompletionSuggestsRenderFlags pins the dynamic half of completion: the
// generated script delegates flag lookup to the hidden __complete command, so a
// flag that disappears from the command tree also disappears from completion.
func TestCompletionSuggestsRenderFlags(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	cmd := cli.NewRootCommand(stubEngine{}, noopRunner)
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"__complete", "render", "--sch"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(__complete) error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "--schema") {
		t.Errorf("__complete render --sch = %q, want a --schema suggestion", got)
	}
}
