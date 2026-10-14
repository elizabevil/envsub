package extended_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/engine/extended"
	"github.com/elizabevil/envsub/internal/values"
)

// benchmarkTemplate builds a large template with many variable references.
func benchmarkTemplate(variables int) []byte {
	var b strings.Builder
	for i := range variables {
		fmt.Fprintf(&b, "key%06d=${VAR%06d}\n", i, i)
	}
	return []byte(b.String())
}

func benchmarkTree(variables int) *values.Tree {
	tree := values.NewTree()
	for i := range variables {
		tree.Root[fmt.Sprintf("VAR%06d", i)] = fmt.Sprintf("value-%d", i)
	}
	return tree
}

// BenchmarkRenderLargeTemplate measures rendering a template with 200k
// variables, the documented REF-04 baseline shape.
func BenchmarkRenderLargeTemplate(b *testing.B) {
	const variables = 200000
	template := benchmarkTemplate(variables)
	tree := benchmarkTree(variables)
	eng := extended.New()
	ctx := context.Background()

	b.SetBytes(int64(len(template)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := eng.Render(ctx, engine.Input{Template: template, Values: tree}); err != nil {
			b.Fatalf("Render() error = %v", err)
		}
	}
}

// benchmarkOperatorTemplate is benchmarkTemplate with a default operator on
// every reference. The operator stops the bare-identifier fast path from
// matching, so every variable takes the general path through parseExpression.
func benchmarkOperatorTemplate(variables int) []byte {
	var b strings.Builder
	for i := range variables {
		fmt.Fprintf(&b, "key%06d=${VAR%06d:-x}\n", i, i)
	}
	return []byte(b.String())
}

// BenchmarkRenderLargeTemplateOperators measures rendering a 200k-variable
// template whose references all take the general path, so the parser cost is
// part of the measured render instead of being bypassed by the fast path.
func BenchmarkRenderLargeTemplateOperators(b *testing.B) {
	const variables = 200000
	template := benchmarkOperatorTemplate(variables)
	tree := benchmarkTree(variables)
	eng := extended.New()
	ctx := context.Background()

	b.SetBytes(int64(len(template)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := eng.Render(ctx, engine.Input{Template: template, Values: tree}); err != nil {
			b.Fatalf("Render() error = %v", err)
		}
	}
}

// BenchmarkRenderSmallTemplate measures startup-scale rendering, which the
// design targets at under 100ms per invocation.
func BenchmarkRenderSmallTemplate(b *testing.B) {
	template := []byte("port=${app.port}\nhosts=${hosts | join \",\"}\n")
	tree := values.NewTree()
	tree.Root["app"] = map[string]any{"port": 8080}
	tree.Root["hosts"] = []any{"a", "b"}
	eng := extended.New()
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := eng.Render(ctx, engine.Input{Template: template, Values: tree}); err != nil {
			b.Fatalf("Render() error = %v", err)
		}
	}
}
