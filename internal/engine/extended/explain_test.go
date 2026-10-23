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
		{name: "nested default word", src: "${A:-${B}}\n", want: []string{"A", "B"}},
		{name: "nested unbraced reference", src: "${A:-$B}\n", want: []string{"A", "B"}},
		{name: "deeply nested default", src: "${A:-${B:-${C}}}\n", want: []string{"A", "B", "C"}},
		{name: "nested reference repeats the path", src: "${A:-${A}}\n", want: []string{"A"}},
		{name: "reference inside fallback text", src: "x${A:-pre-${B}-post}y\n", want: []string{"A", "B"}},
		{name: "nested assignment word", src: "${A:=${B}}\n", want: []string{"A", "B"}},
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

// TestPathsForHonorsPolicy pins the optional filtered enumeration: an excluded
// expression is emitted verbatim, so paths inside its word were never
// substituted and must not be listed. A kept expression still contributes its
// own path and the nested paths the policy keeps.
func TestPathsForHonorsPolicy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		src  string
		keep map[string]bool
		want []string
	}{
		{
			name: "excluded outer hides the word",
			src:  "${A:-${B}}\n",
			keep: map[string]bool{"B": true},
			want: nil,
		},
		{
			name: "kept outer excludes the word reference",
			src:  "${A:-${B}}\n",
			keep: map[string]bool{"A": true},
			want: []string{"A"},
		},
		{
			name: "both kept",
			src:  "${A:-${B}}\n",
			keep: map[string]bool{"A": true, "B": true},
			want: []string{"A", "B"},
		},
		{
			name: "simple paths filtered",
			src:  "${A} ${B} $C\n",
			keep: map[string]bool{"A": true, "C": true},
			want: []string{"A", "C"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := New().PathsFor([]byte(tt.src), func(path string) bool { return tt.keep[path] })
			if err != nil {
				t.Fatalf("PathsFor() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("PathsFor(%q) = %#v, want %#v", tt.src, got, tt.want)
			}
		})
	}
}
