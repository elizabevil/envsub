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
	for spelling, meta := range map[string]values.Meta{
		"b":    {File: "f.yaml", Line: 2, Kind: values.SourceValues},
		"a[0]": {File: "f.yaml", Line: 3, Kind: values.SourceValues},
		"a":    {File: "f.yaml", Line: 1, Kind: values.SourceValues},
	} {
		tree.RecordMeta(metaSegments(t, spelling), meta)
	}
	got := map[string]values.Meta{}
	tree.WalkMeta(func(path []values.Segment, meta values.Meta) {
		got[values.CanonicalPath(path)] = meta
	})
	for spelling, want := range map[string]values.Meta{
		"a":    {File: "f.yaml", Line: 1, Kind: values.SourceValues},
		"a[0]": {File: "f.yaml", Line: 3, Kind: values.SourceValues},
		"b":    {File: "f.yaml", Line: 2, Kind: values.SourceValues},
	} {
		if got[spelling] != want {
			t.Errorf("WalkMeta() entry %q = %+v, want %+v", spelling, got[spelling], want)
		}
	}
	if len(got) != 3 {
		t.Errorf("WalkMeta() reported %d paths, want 3 (%v)", len(got), got)
	}
	if empty := values.NewTree(); empty != nil {
		empty.WalkMeta(func([]values.Segment, values.Meta) {
			t.Error("WalkMeta() on an empty tree reported an entry")
		})
	}
}

// TestRecordMetaAddressesAnElement pins the recording contract: an array element
// is addressed by its full encoded key (JoinIndex), so no separate index argument
// is needed and the origin is reachable from the element path.
func TestRecordMetaAddressesAnElement(t *testing.T) {
	t.Parallel()
	tree := values.NewTree()
	tree.RecordMeta(metaSegments(t, "hosts[0]"), values.Meta{File: "f.yaml", Line: 4, Kind: values.SourceValues})

	meta, ok := tree.MetaAt("hosts[0]")
	if !ok {
		t.Fatal("MetaAt(hosts[0]) not found")
	}
	if meta.Line != 4 || meta.Kind != values.SourceValues {
		t.Errorf("meta = %+v, want line 4 from a value file", meta)
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
