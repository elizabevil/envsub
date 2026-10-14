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
	for path, meta := range map[string]values.Meta{
		"b":    {File: "f.yaml", Line: 2, Kind: "yaml"},
		"a[0]": {File: "f.yaml", Line: 3, Kind: "yaml"},
		"a":    {File: "f.yaml", Line: 1, Kind: "yaml"},
	} {
		tree.RecordEncodedMeta(path, meta)
	}
	got := tree.MetaKeys()
	want := []string{"a", "a[0]", "b"}
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

// TestRecordMetaIndexesIntoTheParentKey pins RecordMeta's index handling: the
// origin of hosts[0] is reachable through the element path.
func TestRecordMetaIndexesIntoTheParentKey(t *testing.T) {
	t.Parallel()
	tree := values.NewTree()
	tree.RecordMeta("hosts", values.Meta{File: "f.yaml", Line: 4, Kind: "yaml"}, 0)

	meta, ok := tree.MetaAt("hosts[0]")
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
	values.MergeInto(tree, loaded, values.Meta{File: path, Kind: loader.Name})

	for _, probe := range []string{"app.port", "hosts[0]"} {
		meta := tree.MetaFor(probe)
		if !meta.Found {
			t.Errorf("MetaFor(%q) not found", probe)
			continue
		}
		if !source.IsValueFileKind(meta.Meta.Kind) {
			t.Errorf("MetaFor(%q).Kind = %q, want a value-file kind", probe, meta.Meta.Kind)
		}
		if meta.Meta.File != path {
			t.Errorf("MetaFor(%q).File = %q, want %q", probe, meta.Meta.File, path)
		}
	}
}
