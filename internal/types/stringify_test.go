package types_test

import (
	"math"
	"testing"

	"github.com/elizabevil/envsub/internal/types"
)

// TestStringifyRejectsNonFinite pins rule F8/OV8: NaN and ±Inf never reach the
// output, even when a caller put them in the tree directly.
func TestStringifyRejectsNonFinite(t *testing.T) {
	t.Parallel()
	for _, value := range []any{math.NaN(), math.Inf(1), math.Inf(-1), float32(math.NaN())} {
		if _, err := types.Stringify(value); err == nil {
			t.Errorf("Stringify(%v) error = nil, want a non-finite rejection", value)
		}
	}
	if got, err := types.Stringify(1.5); err != nil || got != "1.5" {
		t.Errorf("Stringify(1.5) = %q, %v; want 1.5, nil", got, err)
	}
}
