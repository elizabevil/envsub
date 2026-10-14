package render

import (
	"errors"
	"testing"

	"github.com/elizabevil/envsub/internal/diag"
)

// TestValidateEnvSources pins that contradictory environment options are a
// usage error instead of one silently winning.
func TestValidateEnvSources(t *testing.T) {
	t.Parallel()
	if err := (Request{}).Validate(); err != nil {
		t.Fatalf("no options: error = %v, want nil", err)
	}
	if err := (Request{Values: Values{NoProcessEnv: true}}).Validate(); err != nil {
		t.Fatalf("--no-process-env alone: error = %v, want nil", err)
	}
	if err := (Request{Values: Values{ProcessEnvPrefix: "APP_"}}).Validate(); err != nil {
		t.Fatalf("--process-env-prefix alone: error = %v, want nil", err)
	}

	err := (Request{Values: Values{NoProcessEnv: true, ProcessEnvPrefix: "APP_"}}).Validate()
	if err == nil {
		t.Fatal("both options: error = nil, want a usage error")
	}
	var typed *diag.Error
	if !errors.As(err, &typed) || typed.Kind != diag.KindUsage {
		t.Fatalf("both options: error = %v, want KindUsage", err)
	}
}
