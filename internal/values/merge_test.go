package values_test

import (
	"testing"

	"github.com/elizabevil/envsub/internal/values"
)

func TestMergeInto(t *testing.T) {
	t.Parallel()
	base := values.NewTree()
	base.Root["app"] = map[string]any{
		"port":  8080,
		"hosts": []any{"a", "b"},
		"nested": map[string]any{
			"keep":      "1",
			"overwrite": "old",
		},
	}

	overlay := values.NewTree()
	overlay.Root["app"] = map[string]any{
		"port":  9090,
		"hosts": []any{"z"},
		"nested": map[string]any{
			"overwrite": "new",
		},
	}
	values.MergeInto(base, overlay, values.Meta{File: "overlay.yaml"})

	assertResolved(t, base, "app.port", 9090)
	assertResolved(t, base, "app.nested.keep", "1")
	assertResolved(t, base, "app.nested.overwrite", "new")

	hosts, found, err := base.Resolve("app.hosts")
	if err != nil || !found {
		t.Fatalf("Resolve(app.hosts) = %#v/%v/%v", hosts, found, err)
	}
	items, ok := hosts.([]any)
	if !ok || len(items) != 1 || items[0] != "z" {
		t.Fatalf("array should be replaced, got %#v", hosts)
	}

	meta := base.MetaFor("app.nested.overwrite")
	if !meta.Found || meta.Meta.File != "overlay.yaml" {
		t.Fatalf("MetaFor() = %#v, want overlay.yaml", meta)
	}
}

func TestMergeIntoDoesNotAlias(t *testing.T) {
	t.Parallel()
	source := values.NewTree()
	source.Root["list"] = []any{"a"}

	target := values.NewTree()
	values.MergeInto(target, source, values.Meta{})
	source.Root["list"].([]any)[0] = "mutated"

	assertResolved(t, target, "list[0]", "a")
}

func TestMetaForWalksParents(t *testing.T) {
	t.Parallel()
	tree := values.NewTree()
	tree.RecordEncodedMeta("app", values.Meta{File: "v.yaml", Kind: "values"})

	meta := tree.MetaFor("app.port.deep")
	if !meta.Found || meta.Meta.File != "v.yaml" {
		t.Fatalf("MetaFor() = %#v, want v.yaml", meta)
	}
	if meta.Key != "app" {
		t.Fatalf("MetaFor().Key = %q, want app", meta.Key)
	}
}

func TestSetPath(t *testing.T) {
	t.Parallel()
	tree := values.NewTree()
	if err := tree.SetPath("app.port", "8080", values.SetString, values.Origin{Kind: values.SourceSet}); err != nil {
		t.Fatalf("SetPath() error = %v", err)
	}
	// --set without a schema keeps the raw string; type parsing happens later
	// through the schema or an inline | type assertion.
	assertResolved(t, tree, "app.port", "8080")

	if err := tree.SetPath("app.number", "8080", values.SetJSON, values.Origin{Kind: values.SourceSet}); err != nil {
		t.Fatalf("SetPath() error = %v", err)
	}
	assertResolved(t, tree, "app.number", 8080)

	if err := tree.SetPath("app.name", "x", values.SetString, values.Origin{Kind: values.SourceSet}); err != nil {
		t.Fatalf("SetPath() error = %v", err)
	}
	assertResolved(t, tree, "app.name", "x")

	if err := tree.SetPath("app.tags", `["a"]`, values.SetJSON, values.Origin{Kind: values.SourceSet}); err != nil {
		t.Fatalf("SetPath() error = %v", err)
	}
	got, found, err := tree.Resolve("app.tags[0]")
	if err != nil || !found || got != "a" {
		t.Fatalf("Resolve(app.tags[0]) = %#v/%v/%v", got, found, err)
	}
}

func TestSetPathErrors(t *testing.T) {
	t.Parallel()
	tree := values.NewTree()
	if err := tree.SetPath("app.port", "not-json", values.SetJSON, values.Origin{}); err == nil {
		t.Fatalf("SetPath() with invalid JSON error = nil, want error")
	}
	if err := tree.SetPath("app[0]", "x", values.SetString, values.Origin{}); err == nil {
		t.Fatalf("SetPath() with index error = nil, want error")
	}
	if err := tree.SetPath("app.port", "1", values.SetString, values.Origin{}); err != nil {
		t.Fatalf("SetPath() error = %v", err)
	}
	if err := tree.SetPath("app.port.deep", "1", values.SetString, values.Origin{}); err == nil {
		t.Fatalf("SetPath() over scalar error = nil, want error")
	}
}

func TestParseAssignment(t *testing.T) {
	t.Parallel()
	key, value, err := values.ParseAssignment("app.port=8080=extra")
	if err != nil {
		t.Fatalf("ParseAssignment() error = %v", err)
	}
	if key != "app.port" || value != "8080=extra" {
		t.Fatalf("ParseAssignment() = %q/%q", key, value)
	}
	if _, _, err := values.ParseAssignment("missing"); err == nil {
		t.Fatalf("ParseAssignment() error = nil, want error")
	}
}

func TestStore(t *testing.T) {
	t.Parallel()
	tree := values.NewTree()
	if err := tree.Store("app.port", 8080, values.Origin{Kind: values.SourceSet, Method: values.MethodFlag, Source: "app.port"}); err != nil {
		t.Fatalf("Store() error = %v", err)
	}
	assertResolved(t, tree, "app.port", 8080)

	list := []any{"a"}
	if err := tree.Store("app.list", list, values.Origin{Kind: values.SourceSet, Method: values.MethodFlag, Source: "app.list"}); err != nil {
		t.Fatalf("Store() error = %v", err)
	}
	list[0] = "mutated"
	assertResolved(t, tree, "app.list[0]", "a")
}

func TestDescribeMeta(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		ref  values.MetaRef
		want string
	}{
		{name: "empty", ref: values.MetaRef{}, want: ""},
		{name: "file only", ref: values.MetaRef{Found: true, Meta: values.Meta{File: "v.yaml"}}, want: "v.yaml"},
		{name: "file and line", ref: values.MetaRef{Found: true, Meta: values.Meta{File: "v.yaml", Line: 3}}, want: "v.yaml:3"},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := values.DescribeMeta(tt.ref); got != tt.want {
				t.Fatalf("DescribeMeta() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEncodePath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "app.port", want: "app.port"},
		{raw: `app["my-key"]`, want: "app.my-key"},
		{raw: "hosts[0].name", want: "hosts[0].name"},
	}
	for _, tt := range tests {

		t.Run(tt.raw, func(t *testing.T) {
			t.Parallel()
			got, err := values.EncodePath(tt.raw)
			if err != nil {
				t.Fatalf("EncodePath(%q) error = %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("EncodePath(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func assertResolved(t *testing.T, tree *values.Tree, path string, want any) {
	t.Helper()
	got, found, err := tree.Resolve(path)
	if err != nil {
		t.Fatalf("Resolve(%q) error = %v", path, err)
	}
	if !found {
		t.Fatalf("Resolve(%q) not found", path)
	}
	if got != want {
		t.Fatalf("Resolve(%q) = %#v (type %T), want %#v (type %T)", path, got, got, want, want)
	}
}
