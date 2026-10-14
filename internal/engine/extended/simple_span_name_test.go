package extended

import "testing"

// TestSimpleSpanName pins the fast-path name scanner, which now returns a
// sub-slice of the source instead of a string. Every accepted/rejected form is
// visible here so the zero-copy change stays reviewable.
func TestSimpleSpanName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		src    string
		braced bool
		end    int
		want   string
		ok     bool
	}{
		{name: "braced", src: "${VAR}", braced: true, end: 6, want: "VAR", ok: true},
		{name: "unbraced", src: "$VAR", braced: false, end: 4, want: "VAR", ok: true},
		{name: "underscore and digits", src: "${A1_b}", braced: true, end: 7, want: "A1_b", ok: true},
		{name: "dotted path is not simple", src: "${A.B}", braced: true, end: 6, ok: false},
		{name: "index is not simple", src: "${A[0]}", braced: true, end: 7, ok: false},
		{name: "empty braces", src: "${}", braced: true, end: 4, ok: false},
		{name: "leading digit", src: "${1A}", braced: true, end: 5, ok: false},
		{name: "dash", src: "${A-B}", braced: true, end: 6, ok: false},
		{name: "unclosed brace", src: "${VAR", braced: true, end: 5, ok: false},
		{name: "unbraced dotted", src: "$A.B", braced: false, end: 3, ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := simpleSpanName([]byte(tt.src), Span{Start: 0, End: tt.end, Braced: tt.braced})
			if ok != tt.ok || string(got) != tt.want {
				t.Fatalf("simpleSpanName(%q, braced=%v) = (%q, %v), want (%q, %v)",
					tt.src, tt.braced, got, ok, tt.want, tt.ok)
			}
		})
	}
}
