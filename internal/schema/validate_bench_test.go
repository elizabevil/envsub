package schema_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/elizabevil/envsub/internal/schema"
	"github.com/elizabevil/envsub/internal/values"
)

// BenchmarkValidateLargeTree measures validating a configuration-sized tree
// (200 typed properties, all present) against its schema.
func BenchmarkValidateLargeTree(b *testing.B) {
	const properties = 200
	props := make(map[string]any, properties)
	tree := values.NewTree()
	for i := range properties {
		name := fmt.Sprintf("key%03d", i)
		props[name] = map[string]any{"type": "integer"}
		tree.Root[name] = i
	}
	raw := map[string]any{"type": "object", "properties": props}
	sch, err := schema.Load(context.Background(), raw, "schema.yaml", schema.UnknownIgnore)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if err := sch.Validate(tree); err != nil {
			b.Fatal(err)
		}
	}
}
