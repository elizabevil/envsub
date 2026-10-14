package cli_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/elizabevil/envsub/internal/cli"
)

// TestVersionCommandPrintsJSON pins the machine-readable build output.
func TestVersionCommandPrintsJSON(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	cmd := cli.NewRootCommand(stubEngine{}, noopRunner)
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"version"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var info map[string]any
	if err := json.Unmarshal(out.Bytes(), &info); err != nil {
		t.Fatalf("version output %q is not JSON: %v", out.String(), err)
	}
	for _, field := range []string{"gitVersion", "gitTreeState", "goVersion", "compiler", "platform"} {
		value, _ := info[field].(string)
		if value == "" {
			t.Fatalf("version output = %s, want a non-empty %s field", out.Bytes(), field)
		}
	}
}
