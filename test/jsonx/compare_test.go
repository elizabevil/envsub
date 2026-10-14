package jsonx_test

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/elizabevil/envsub/internal/jsonx"
)

// fixtureDir is the corpus shared with internal/jsonx: the comparison runs both
// implementations over exactly the documents the production tests use.
const fixtureDir = "../../internal/jsonx/testdata"

// TestCorpusEquivalence checks that the production decoder (encoding/json/v2
// with the number-preserving unmarshaler) and the token walker agree on every
// fixture: the same documents are accepted, the same ones are rejected, and
// accepted documents produce identical trees.
func TestCorpusEquivalence(t *testing.T) {
	t.Parallel()
	var checked, rejected int
	err := filepath.WalkDir(fixtureDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		name := strings.TrimPrefix(path, fixtureDir+"/")

		production, productionErr := jsonx.UnmarshalValue(data)
		alternative, alternativeErr := walkerUnmarshalValue(data)

		// Duplicate members are the one accepted backend difference (03-12 T9):
		// sonic keeps the last member while the token walker rejects the document.
		if name == "duplicate_members.json" && sonicBackend {
			if productionErr != nil {
				t.Errorf("%s: sonic production error = %v, want last-wins", name, productionErr)
			}
			if alternativeErr == nil {
				t.Errorf("%s: walker accepted duplicate members, want an error", name)
			}
			return nil
		}
		if (productionErr == nil) != (alternativeErr == nil) {
			t.Errorf("%s: production err = %v, walker err = %v", name, productionErr, alternativeErr)
			return nil
		}
		if productionErr != nil {
			rejected++
			return nil
		}
		if !reflect.DeepEqual(production, alternative) {
			t.Errorf("%s: production = %#v, walker = %#v", name, production, alternative)
			return nil
		}
		checked++
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", fixtureDir, err)
	}
	if checked == 0 {
		t.Fatalf("no fixtures compared under %s", fixtureDir)
	}
	t.Logf("compared %d documents, both rejected %d", checked, rejected)
}

// TestBothImplementationsPreserveBigIntegers states the functional requirement
// the comparison exists for: numbers above 2^53 must not be widened to float64.
func TestBothImplementationsPreserveBigIntegers(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join(fixtureDir, "big_integers.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	want := map[string]any{
		"pow53":  json.Number("9007199254740993"),
		"u64max": json.Number("18446744073709551615"),
		"i64min": json.Number("-9223372036854775808"),
	}

	production, err := jsonx.UnmarshalValue(data)
	if err != nil {
		t.Fatalf("production error = %v", err)
	}
	if !reflect.DeepEqual(production, want) {
		t.Fatalf("production = %#v, want %#v", production, want)
	}

	alternative, err := walkerUnmarshalValue(data)
	if err != nil {
		t.Fatalf("walker error = %v", err)
	}
	if !reflect.DeepEqual(alternative, want) {
		t.Fatalf("walker = %#v, want %#v", alternative, want)
	}
}

// TestTimingReport runs both implementations over a configuration-sized
// document and reports the cost of the production choice, so the trade-off can
// be reproduced with a plain `go test -v ./test/jsonx/` (no -bench needed).
func TestTimingReport(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("timing report skipped in short mode")
	}
	data := benchmarkPayload()
	const iterations = 30

	production := measure(t, data, jsonx.UnmarshalValue, iterations)
	alternative := measure(t, data, walkerUnmarshalValue, iterations)
	t.Logf("%d KB decode into any: production (encoding/json/v2 + unmarshaler) %v/op, token walker %v/op, walker %.2fx faster",
		len(data)/1024, production, alternative, float64(production)/float64(alternative))
}

// measure returns the average duration of decode over iterations runs.
func measure(t *testing.T, data []byte, decode func([]byte) (any, error), iterations int) time.Duration {
	t.Helper()
	start := time.Now()
	for range iterations {
		if _, err := decode(data); err != nil {
			t.Fatalf("decode: %v", err)
		}
	}
	return time.Since(start) / time.Duration(iterations)
}

func BenchmarkUnmarshalValueProduction(b *testing.B) {
	data := benchmarkPayload()
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := jsonx.UnmarshalValue(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshalValueWalker(b *testing.B) {
	data := benchmarkPayload()
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := walkerUnmarshalValue(data); err != nil {
			b.Fatal(err)
		}
	}
}

// benchmarkPayload builds the same configuration-sized document (5000 objects)
// internal/jsonx benchmarks against, so the numbers are comparable.
func benchmarkPayload() []byte {
	var b strings.Builder
	b.WriteByte('[')
	for i := range 5000 {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"host":"h%04d.internal","port":%d,"tags":["a","b","c"],"enabled":true}`, i, 8000+i)
	}
	b.WriteByte(']')
	return []byte(b.String())
}
