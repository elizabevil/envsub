package values_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/elizabevil/envsub/internal/source"
	"github.com/elizabevil/envsub/internal/values"
)

// TestMetaKeysIsSortedAndComplete pins the metadata index --explain and the
// unused report read: every recorded path is listed once, in sorted order.
func TestMetaKeysIsSortedAndComplete(t *testing.T) {
	t.Parallel()
	tree := values.NewTree()
	for path, meta := range map[values.Key]values.Meta{
		"b":    {File: "f.yaml", Line: 2, Kind: "yaml"},
		"a[0]": {File: "f.yaml", Line: 3, Kind: "yaml"},
		"a":    {File: "f.yaml", Line: 1, Kind: "yaml"},
	} {
		tree.RecordMeta(path, meta)
	}
	got := tree.MetaKeys()
	want := []values.Key{"a", "a[0]", "b"}
	if len(got) != len(want) {
		t.Fatalf("MetaKeys() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("MetaKeys() = %v, want %v", got, want)
		}
	}
	if empty := values.NewTree().MetaKeys(); empty != nil {
		t.Errorf("MetaKeys() on an empty tree = %v, want nil", empty)
	}
}

// TestRecordMetaAddressesAnElement pins the recording contract: an array element
// is addressed by its full encoded key (JoinIndex), so no separate index argument
// is needed and the origin is reachable from the element path.
func TestRecordMetaAddressesAnElement(t *testing.T) {
	t.Parallel()
	tree := values.NewTree()
	element := values.JoinIndex("hosts", 0)
	tree.RecordMeta(element, values.Meta{File: "f.yaml", Line: 4, Kind: "yaml"})

	meta, ok := tree.MetaAt(element)
	if !ok {
		t.Fatal("MetaAt(hosts[0]) not found")
	}
	if meta.Line != 4 || meta.Kind != "yaml" {
		t.Errorf("meta = %+v, want line 4 from f.yaml", meta)
	}
	// The same origin is what an element lookup resolves to.
	if ref := tree.MetaFor("hosts[0].name"); !ref.Found {
		t.Error("MetaFor(hosts[0].name) not found; an element origin must cover its children")
	}
}

// TestMergeIntoRecordsEveryRootKey pins that merging a value file records an
// origin for each top-level key, which is what makes the unused report able to
// tell a value-file entry from a default or an assignment.
func TestMergeIntoRecordsEveryRootKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "values.yaml")
	if err := os.WriteFile(path, []byte("app:\n  port: 8080\nhosts: [a]\n"), 0o600); err != nil {
		t.Fatalf("write values: %v", err)
	}
	loader, err := source.ForFile(path)
	if err != nil {
		t.Fatalf("ForFile: %v", err)
	}
	loaded, err := loader.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	tree := values.NewTree()
	values.MergeInto(tree, loaded, values.Meta{File: path, Kind: loader.Kind})

	for _, probe := range []string{"app.port", "hosts[0]"} {
		meta := tree.MetaFor(probe)
		if !meta.Found {
			t.Errorf("MetaFor(%q) not found", probe)
			continue
		}
		if meta.Meta.Kind != values.SourceValues {
			t.Errorf("MetaFor(%q).Kind = %q, want %q", probe, meta.Meta.Kind, values.SourceValues)
		}
		if meta.Meta.File != path {
			t.Errorf("MetaFor(%q).File = %q, want %q", probe, meta.Meta.File, path)
		}
	}
}
