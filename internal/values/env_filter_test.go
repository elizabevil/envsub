package values_test

import (
	"testing"

	"github.com/elizabevil/envsub/internal/values"
)

// TestEnvFromOSFiltered pins the whitelist used by --process-env-prefix: only matching
// names are exposed and the prefix is kept.
func TestEnvFromOSFiltered(t *testing.T) {
	t.Setenv("ENVSUBKEEP_ONE", "1")
	t.Setenv("ENVSUBOTHER_TWO", "2")

	all := values.EnvFromOSFiltered("")
	if all["ENVSUBKEEP_ONE"] != "1" || all["ENVSUBOTHER_TWO"] != "2" {
		t.Fatalf("empty prefix must keep every variable, got %#v", all)
	}

	filtered := values.EnvFromOSFiltered("ENVSUBKEEP_")
	if filtered["ENVSUBKEEP_ONE"] != "1" {
		t.Fatalf("filtered env missing the whitelisted variable: %#v", filtered)
	}
	if _, exists := filtered["ENVSUBOTHER_TWO"]; exists {
		t.Fatalf("filtered env leaked a non-matching variable: %#v", filtered)
	}
}
