package values_test

import (
	"fmt"
	"testing"

	"github.com/elizabevil/envsub/internal/values"
)

// BenchmarkMergeInto measures overlaying 50 value files of 100 keys each, the
// shape a layered values setup produces.
func BenchmarkMergeInto(b *testing.B) {
	const files, keys = 50, 100
	sources := make([]*values.Tree, files)
	for f := range files {
		sources[f] = values.NewTree()
		for k := range keys {
			sources[f].Root[fmt.Sprintf("key%03d", k)] = f*keys + k
		}
	}
	meta := values.Meta{File: "overlay.yaml", Kind: "values"}
	b.ReportAllocs()
	for b.Loop() {
		dst := values.NewTree()
		for _, src := range sources {
			values.MergeInto(dst, src, meta)
		}
	}
}

// BenchmarkParsePath measures the general-path parser for dotted/indexed paths.
func BenchmarkParsePath(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := values.ParsePath(`app.servers[0].labels["my-key"]`); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParsePathPrefix measures extracting the leading path of an
// expression, which only needs the end offset.
func BenchmarkParsePathPrefix(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, _, err := values.CutPathPrefix(`app.servers[0].labels["my-key"] | toJson`); err != nil {
			b.Fatal(err)
		}
	}
}
