package extended

import "testing"

// expressionBenchCases are the expression shapes the renderer parses most
// often. Every non-bare ${...} reference goes through parseExpression once per
// render, so these numbers bound the parsing share of a render.
var expressionBenchCases = []struct {
	name string
	raw  string
}{
	{name: "bare", raw: "A"},
	{name: "default", raw: "A:-default"},
	{name: "toJson", raw: "A | toJson"},
	{name: "typeInt", raw: "A | type int"},
	{name: "joinQuoted", raw: `A | join ","`},
	{name: "indentQuoted", raw: `A | indent "2"`},
	{name: "quote", raw: "A | quote"},
	{name: "requiredAndType", raw: "A:?msg | type int"},
	{name: "multiStep", raw: `A | join "," | indent "2" | quote`},
}

// BenchmarkParseExpression measures parsing the content of ${...} for the
// shapes the extended syntax supports.
func BenchmarkParseExpression(b *testing.B) {
	for _, tc := range expressionBenchCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				if _, err := parseExpression(tc.raw); err != nil {
					b.Fatalf("parseExpression(%q) error = %v", tc.raw, err)
				}
			}
		})
	}
}

// BenchmarkSplitPipes measures the pipeline splitter alone. It is the only
// stage that rebuilds the input, so it backs the decision on whether a
// scan-and-slice splitter is worth adding.
func BenchmarkSplitPipes(b *testing.B) {
	for _, tc := range expressionBenchCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				if _, err := splitPipes(tc.raw); err != nil {
					b.Fatalf("splitPipes(%q) error = %v", tc.raw, err)
				}
			}
		})
	}
}
