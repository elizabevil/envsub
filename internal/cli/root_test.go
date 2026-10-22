package cli_test

import (
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/cli"
	"github.com/elizabevil/envsub/internal/diag"
)

func TestFlagErrorMapsToUsageExitCode(t *testing.T) {
	t.Parallel()
	cmd := cli.NewRootCommand(stubEngine{}, noopRunner)
	cmd.SetArgs([]string{"render", "--definitely-not-a-flag"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if got, want := diag.ExitCode(err), 2; got != want {
		t.Fatalf("ExitCode() = %d, want %d", got, want)
	}
}

// TestUnknownCommandMapsToUsageExitCode pins that a name that is not a
// subcommand is a usage error and not an unclassified one, and that the
// suggestion list comes along.
func TestUnknownCommandMapsToUsageExitCode(t *testing.T) {
	t.Parallel()
	cmd := cli.NewRootCommand(stubEngine{}, noopRunner)
	cmd.SetArgs([]string{"rendr"})
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if got, want := diag.ExitCode(err), 2; got != want {
		t.Fatalf("ExitCode() = %d, want %d; err=%v", got, want, err)
	}
	if diag.KindOf(err) != diag.KindUsage {
		t.Fatalf("KindOf() = %q, want %q", diag.KindOf(err), diag.KindUsage)
	}
	for _, want := range []string{`unknown command "rendr" for "envsub"`, "Did you mean this?", "render"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want it to contain %q", err.Error(), want)
		}
	}
}
