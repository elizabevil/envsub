package cli_test

import (
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
