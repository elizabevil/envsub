package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/cli"
)

// TestErrorFormatRoundTripsThroughTheCommand pins that the flag the assembler
// reads after Execute is the one the user wrote: --error-format=json must reach
// internal/app, which formats the failure.
func TestErrorFormatRoundTripsThroughTheCommand(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args []string
		want string
	}{
		{args: []string{"version"}, want: cli.ErrorFormatText},
		{args: []string{"--error-format=json", "version"}, want: cli.ErrorFormatJSON},
		{args: []string{"--error-format", "json", "version"}, want: cli.ErrorFormatJSON},
	} {
		var out bytes.Buffer
		cmd := cli.NewRootCommand(stubEngine{}, noopRunner)
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(tc.args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute(%v) error = %v", tc.args, err)
		}
		if got := cli.ErrorFormatFrom(cmd); got != tc.want {
			t.Errorf("ErrorFormatFrom(%v) = %q, want %q", tc.args, got, tc.want)
		}
	}
}

// TestInvalidErrorFormatIsRejectedBeforeExecution pins that an unknown format
// fails as a usage error and never reaches the runner.
func TestInvalidErrorFormatIsRejectedBeforeExecution(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	stub, run := newStubRunner()
	cmd := cli.NewRootCommand(stubEngine{}, run)
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--error-format=yaml", "render", "in.tmpl"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want a usage error")
	}
	if stub.called {
		t.Error("the runner ran despite an invalid --error-format")
	}
	if !strings.Contains(err.Error(), "invalid --error-format") {
		t.Errorf("error = %v, want it to name the invalid format", err)
	}
}
