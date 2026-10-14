package yamlx_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/elizabevil/envsub/internal/yamlx"
)

// corpus is the set of values both encoders must agree on. Every entry is
// something the renderer's value tree can hold.
func corpus() []struct {
	name  string
	value any
} {
	return []struct {
		name  string
		value any
	}{
		{name: "nil", value: nil},
		{name: "empty map", value: map[string]any{}},
		{name: "empty slice", value: []any{}},
		{name: "string", value: "checkout"},
		{name: "numeric string", value: "8080"},
		{name: "multi-line string", value: "line1\nline2\n"},
		{name: "unicode", value: "配置 ✓"},
		{name: "bool", value: true},
		{name: "int", value: 8080},
		{name: "negative int", value: -3},
		{name: "float", value: 0.75},
		{name: "json integer", value: json.Number("8080")},
		{name: "json float", value: json.Number("1.5")},
		{name: "uint64 integer", value: json.Number("18446744073709551615")},
		{name: "nested map", value: map[string]any{"app": map[string]any{"port": 8080, "name": "checkout"}}},
		{name: "sequence under key", value: map[string]any{"hosts": []any{"a", "b"}}},
		{name: "sequence of maps", value: []any{map[string]any{"name": "a"}, map[string]any{"name": "b"}}},
		{name: "unsorted keys", value: map[string]any{"z": 1, "a": 2, "m": 3}},
	}
}

// TestEncodersAgreeSemantically encodes every corpus entry with both encoders,
// decodes both results again and compares the trees. The two encoders differ in
// style (see TestKnownStyleDifference), not in meaning.
func TestEncodersAgreeSemantically(t *testing.T) {
	t.Parallel()
	for _, entry := range corpus() {
		t.Run(entry.name, func(t *testing.T) {
			t.Parallel()
			production, err := yamlx.Marshal(entry.value)
			if err != nil {
				t.Fatalf("yamlx.Marshal() error = %v", err)
			}
			alternative, err := sigsMarshal(entry.value)
			if err != nil {
				t.Fatalf("sigs.Marshal() error = %v", err)
			}
			productionTree, err := yamlx.Decode(production)
			if err != nil {
				t.Fatalf("Decode(production %q) error = %v", production, err)
			}
			alternativeTree, err := yamlx.Decode(alternative)
			if err != nil {
				t.Fatalf("Decode(alternative %q) error = %v", alternative, err)
			}
			if !reflect.DeepEqual(productionTree, alternativeTree) {
				t.Fatalf("decoded trees differ:\n  yaml.v3 %q -> %#v\n  sigs    %q -> %#v",
					production, productionTree, alternative, alternativeTree)
			}
		})
	}
}

// TestKnownNilDifference documents the semantic difference the V1 rule accepts:
// yaml.v3 writes {} and [] for a typed nil collection, while sigs.k8s.io/yaml
// (through its JSON round trip) writes null. Rule V1 chose the library default,
// so production intentionally differs from the rejected encoder here.
func TestKnownNilDifference(t *testing.T) {
	t.Parallel()
	var nilMap map[string]any
	var nilSlice []any
	tests := []struct {
		name       string
		value      any
		wantYAMLv3 string
		wantSigs   string
	}{
		{name: "typed nil map", value: nilMap, wantYAMLv3: "{}\n", wantSigs: "null\n"},
		{name: "typed nil slice", value: nilSlice, wantYAMLv3: "[]\n", wantSigs: "null\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			production, err := yamlx.Marshal(tt.value)
			if err != nil {
				t.Fatalf("yamlx.Marshal() error = %v", err)
			}
			if string(production) != tt.wantYAMLv3 {
				t.Fatalf("yaml.v3 = %q, want %q (rule V1)", production, tt.wantYAMLv3)
			}
			alternative, err := sigsMarshal(tt.value)
			if err != nil {
				t.Fatalf("sigs.Marshal() error = %v", err)
			}
			if string(alternative) != tt.wantSigs {
				t.Fatalf("sigs = %q, want %q", alternative, tt.wantSigs)
			}
		})
	}
}

// TestKnownStyleDifference documents the other visible difference: yaml.v3
// indents a sequence under its key, sigs.k8s.io/yaml keeps it at the parent
// indentation. Both parse to the same value, which the comparison above proves
// for the whole corpus.
func TestKnownStyleDifference(t *testing.T) {
	t.Parallel()
	value := map[string]any{"hosts": []any{"a", "b"}}

	production, err := yamlx.Marshal(value)
	if err != nil {
		t.Fatalf("yamlx.Marshal() error = %v", err)
	}
	alternative, err := sigsMarshal(value)
	if err != nil {
		t.Fatalf("sigs.Marshal() error = %v", err)
	}

	if want := "hosts:\n  - a\n  - b\n"; string(production) != want {
		t.Fatalf("yaml.v3 output = %q, want %q", production, want)
	}
	if want := "hosts:\n- a\n- b\n"; string(alternative) != want {
		t.Fatalf("sigs output = %q, want %q", alternative, want)
	}
	t.Logf("yaml.v3 indents sequences (%q); sigs keeps them at the parent (%q)", production, alternative)
}

// TestBeyondUint64Difference documents the second, rarer difference: an integer
// too large for uint64 stays exact text with yaml.v3 (quoted, so it round trips
// as a string), while the JSON round trip sigs uses widens it to float64.
func TestBeyondUint64Difference(t *testing.T) {
	t.Parallel()
	value := json.Number("123456789012345678901234567890")

	production, err := yamlx.Marshal(value)
	if err != nil {
		t.Fatalf("yamlx.Marshal() error = %v", err)
	}
	alternative, err := sigsMarshal(value)
	if err != nil {
		t.Fatalf("sigs.Marshal() error = %v", err)
	}
	if !strings.Contains(string(production), "123456789012345678901234567890") {
		t.Fatalf("yaml.v3 output = %q, want the exact digits", production)
	}
	if strings.Contains(string(alternative), "123456789012345678901234567890") {
		t.Fatalf("sigs output = %q, expected float rounding", alternative)
	}
	t.Logf("beyond uint64: yaml.v3 %q, sigs %q", production, alternative)
}

// TestKnownLiteralSpellingDifference documents the third deliberate difference
// (rule F6): an undeclared fractional literal keeps its spelling, while
// sigs.k8s.io/yaml rounds it through its JSON/float64 round trip.
func TestKnownLiteralSpellingDifference(t *testing.T) {
	t.Parallel()
	value := json.Number("1.50")

	production, err := yamlx.Marshal(value)
	if err != nil {
		t.Fatalf("yamlx.Marshal() error = %v", err)
	}
	alternative, err := sigsMarshal(value)
	if err != nil {
		t.Fatalf("sigs.Marshal() error = %v", err)
	}
	if want := "1.50\n"; string(production) != want {
		t.Fatalf("yaml.v3 output = %q, want %q (rule F6)", production, want)
	}
	if string(alternative) == "1.50\n" {
		t.Fatalf("sigs output = %q, expected float rounding", alternative)
	}
	t.Logf("literal spelling: yaml.v3 %q, sigs %q", production, alternative)
}

// TestTimingReport runs both encoders over a configuration-sized document and
// reports the cost of the production choice, reproducible with a plain
// `go test -v ./test/yamlx/`.
func TestTimingReport(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("timing report skipped in short mode")
	}
	value, err := yamlx.Decode(benchmarkDocument(t))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if _, err := yamlx.Marshal(value); err != nil {
		t.Fatalf("warm-up error = %v", err)
	}
	const iterations = 30

	production := measure(t, value, yamlx.Marshal, iterations)
	alternative := measure(t, value, sigsMarshal, iterations)
	t.Logf("YAML encode: yaml.v3 %v/op, sigs.k8s.io/yaml %v/op, ratio %.2fx",
		production, alternative, float64(alternative)/float64(production))
}

func measure(t *testing.T, value any, encode func(any) ([]byte, error), iterations int) time.Duration {
	t.Helper()
	start := time.Now()
	for range iterations {
		if _, err := encode(value); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	return time.Since(start) / time.Duration(iterations)
}

func BenchmarkMarshalYAMLv3(b *testing.B) {
	value, err := yamlx.Decode(benchmarkDocument(b))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := yamlx.Marshal(value); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarshalSigs(b *testing.B) {
	value, err := yamlx.Decode(benchmarkDocument(b))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := sigsMarshal(value); err != nil {
			b.Fatal(err)
		}
	}
}

// benchmarkDocument builds a configuration-sized YAML document: 1000 services,
// each with scalars, a nested map and a sequence.
func benchmarkDocument(tb testing.TB) []byte {
	tb.Helper()
	var builder strings.Builder
	builder.WriteString("services:\n")
	for i := range 1000 {
		fmt.Fprintf(&builder, "  - name: service-%04d\n    port: %d\n    enabled: true\n    ratio: 0.%d\n    labels:\n      tier: backend\n      zone: eu\n    hosts:\n      - a.internal\n      - b.internal\n", i, 8000+i, i%10)
	}
	return []byte(builder.String())
}
