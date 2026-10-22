package extended

import (
	"testing"
)

func TestParseExpressionErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
	}{
		{name: "unknown formatter", raw: "hosts | nope"},
		{name: "type after formatter", raw: "PORT | quote | type int"},
		{name: "duplicate type", raw: "PORT | type int | type string"},
		{name: "empty step", raw: "hosts |"},
		{name: "unquoted join argument", raw: "hosts | join ,"},
		{name: "unsupported type", raw: "PORT | type map"},
		{name: "toJson with argument", raw: "PORT | toJson x"},
		// A name that begins with a digit is not a path, so the path grammar
		// must reject it rather than the expression parser taking it for a
		// reference the resolver cannot address.
		{name: "digit leading name", raw: "0"},
		{name: "digit leading name with digits", raw: "0abc"},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := parseExpression(tt.raw); err == nil {
				t.Fatalf("parseExpression(%q) error = nil, want error", tt.raw)
			}
		})
	}
}
