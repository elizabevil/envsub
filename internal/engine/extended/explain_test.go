package extended

import (
	"reflect"
	"testing"
)

// TestPaths pins the enumeration used by --explain: variable paths in
// first-appearance order, deduplicated, with expressions reduced to their path.
func TestPaths(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{name: "bare and repeated", src: "${A} ${B} ${A}\n", want: []string{"A", "B"}},
		{name: "operator expression", src: "${A | toJson} ${B:-fallback}\n", want: []string{"A", "B"}},
		{name: "dotted and indexed", src: "${app.port} ${hosts[0]}\n", want: []string{"app.port", "hosts[0]"}},
		{name: "root env key stays a path", src: "${env.PORT} ${env.HOME}\n", want: []string{"env.PORT", "env.HOME"}},
		{name: "process namespace", src: "${@process.PORT} ${@process[\"MY-VAR\"]}\n", want: []string{"@process.PORT", `@process["MY-VAR"]`}},
		{name: "top-level quoted key", src: "${[\"a.b\"].k}\n", want: []string{`["a.b"].k`}},
		{name: "text only", src: "no variables here\n", want: nil},
		{name: "escaped dollar", src: "$${A} ${B}\n", want: []string{"B"}},
		{name: "unbraced", src: "$A ${B}\n", want: []string{"A", "B"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := New().Paths([]byte(tt.src))
			if err != nil {
				t.Fatalf("Paths() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Paths(%q) = %#v, want %#v", tt.src, got, tt.want)
			}
		})
	}
}
