package jsonx

import (
	"strings"
	"testing"
)

// The decoders must not repeat the bytes they failed on: those bytes are a
// value the caller supplied, and a secret when they came from a .env file or
// --set-file (REQ-SEC-02). This runs under both backends so a backend switch
// cannot reintroduce the leak.
func TestDecodeErrorsDoNotRepeatTheInput(t *testing.T) {
	t.Parallel()
	const secret = "sup3r-s3cret-token"
	inputs := map[string]string{
		"truncated string": `{"TOKEN": "` + secret + `"`,
		"invalid value":    `{"TOKEN": ` + secret + `}`,
	}
	for name, input := range inputs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var target any
			err := Unmarshal([]byte(input), &target)
			if err == nil {
				t.Fatalf("Unmarshal(%q) error = nil, want a syntax error", input)
			}
			if strings.Contains(err.Error(), secret) {
				t.Errorf("error %q repeats the input; it must not", err)
			}
		})
	}
}
