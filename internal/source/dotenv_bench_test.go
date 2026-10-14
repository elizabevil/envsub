package source_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/source"
)

// BenchmarkParseDotenv measures .env parsing for a configuration-sized file:
// 1000 keys mixing plain, double-quoted and single-quoted values.
func BenchmarkParseDotenv(b *testing.B) {
	var builder strings.Builder
	for i := range 1000 {
		index := strconv.Itoa(i)
		switch i % 3 {
		case 0:
			builder.WriteString("PLAIN_" + index + "=value-" + index + "\n")
		case 1:
			builder.WriteString("QUOTED_" + index + `="value ` + index + "\"\n")
		default:
			builder.WriteString("SINGLE_" + index + "='value " + index + "'\n")
		}
	}
	data := []byte(builder.String())
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := source.ParseDotenv(data); err != nil {
			b.Fatal(err)
		}
	}
}
