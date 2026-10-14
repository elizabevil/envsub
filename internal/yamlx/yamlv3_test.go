package yamlx_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/elizabevil/envsub/internal/codec"
	"github.com/elizabevil/envsub/internal/yamlx"
)

// readTestdata loads an on-disk YAML document.
func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read testdata %s: %v", name, err)
	}
	return data
}

// TestDecodePreservesTypes pins the contract of the yaml.v3 node walker:
// integers stay integers, quoted scalars stay string, and empty scalars become
// nil. Two number details are deliberate consequences of yaml.v3 and differ
// from the JSON loader, which keeps the exact spelling as json.Number:
//
//   - an integer that does not fit int64 is decoded as uint64;
//   - an integer that does not fit uint64 falls back to float64 and loses
//     precision.
func TestDecodePreservesTypes(t *testing.T) {
	t.Parallel()
	value, err := yamlx.Decode(readTestdata(t, "decode/types.yaml"))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("Decode() = %T, want map[string]any", value)
	}

	want := map[string]any{
		"int":    8080,
		"big":    uint64(12345678901234567890),
		"huge":   codec.ExactInt("123456789012345678901234567890"),
		"float":  codec.LiteralNumber("1.5"),
		"bool":   true,
		"string": "plain text",
		// A quoted scalar keeps its string type even when it looks numeric.
		"quoted_number": "8080",
		"empty":         nil,
		"nested": map[string]any{
			"list":   []any{1, "two", false},
			"object": map[string]any{"inner": "value"},
		},
	}
	if !reflect.DeepEqual(object, want) {
		t.Fatalf("Decode() = %#v, want %#v", object, want)
	}
}

// TestDecodeFollowsAliases covers anchors and aliases, which the values loader
// must resolve.
func TestDecodeFollowsAliases(t *testing.T) {
	t.Parallel()
	value, err := yamlx.Decode(readTestdata(t, "decode/aliases.yaml"))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	object := value.(map[string]any)
	service := object["service"].(map[string]any)
	if service["port"] != 9090 {
		t.Fatalf("service.port = %#v, want 9090", service["port"])
	}
	if service["host"] != "localhost" {
		t.Fatalf("service.host = %#v, want localhost (merged alias)", service["host"])
	}
	alias := object["alias"].(map[string]any)
	if alias["port"] != 8080 {
		t.Fatalf("alias.port = %#v, want 8080", alias["port"])
	}
}

// TestDecodeRejectsDuplicateKeys documents a deliberate difference from plain
// yaml.v3: decoding into a yaml.Node does not check for repeated mapping keys,
// while decoding into a map does. The values loader needs the check to match
// the JSON and .env loaders.
func TestDecodeRejectsDuplicateKeys(t *testing.T) {
	t.Parallel()
	document := readTestdata(t, "decode/duplicate_keys.yaml")

	if _, err := yamlx.Decode(document); err == nil {
		t.Fatal("Decode(duplicate keys) error = nil, want error")
	} else {
		t.Logf("duplicate keys rejected: %v", err)
	}

	// Unmarshal keeps yaml.v3's own semantics, which already reject them.
	var target map[string]any
	if err := yamlx.Unmarshal(document, &target); err == nil {
		t.Fatal("Unmarshal(duplicate keys) error = nil, want error")
	}
}

// TestDecodeSequenceTopLevel shows that Decode returns whatever the document
// holds; rejecting a non-mapping root is the values loader's policy, not the
// codec's.
func TestDecodeSequenceTopLevel(t *testing.T) {
	t.Parallel()
	value, err := yamlx.Decode(readTestdata(t, "decode/sequence.yaml"))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if want := []any{"one", "two"}; !reflect.DeepEqual(value, want) {
		t.Fatalf("Decode() = %#v, want %#v", value, want)
	}
}

// TestUnmarshalUsesYAMLSemantics checks the yaml.v3 passthrough used by the
// fixture runner.
func TestUnmarshalUsesYAMLSemantics(t *testing.T) {
	t.Parallel()
	var target struct {
		Unset string `yaml:"unset"`
		Empty string `yaml:"empty"`
	}
	if err := yamlx.Unmarshal([]byte("unset: error\nempty: keep\n"), &target); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if target.Unset != "error" || target.Empty != "keep" {
		t.Fatalf("Unmarshal() = %+v, want {error keep}", target)
	}
}
