package yamlx_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/yamlx"
)

// BenchmarkDecode measures YAML value-file decoding for a configuration-sized
// document: 1000 services, each with scalars, a nested map and a sequence.
func BenchmarkDecode(b *testing.B) {
	var builder strings.Builder
	builder.WriteString("services:\n")
	for i := range 1000 {
		index := strconv.Itoa(i)
		builder.WriteString("  - name: service-" + index + "\n")
		builder.WriteString("    port: " + strconv.Itoa(8000+i) + "\n")
		builder.WriteString("    enabled: true\n")
		builder.WriteString("    labels:\n      tier: backend\n      zone: eu\n")
		builder.WriteString("    hosts:\n      - a.internal\n      - b.internal\n")
	}
	data := []byte(builder.String())
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := yamlx.Decode(data); err != nil {
			b.Fatal(err)
		}
	}
}
