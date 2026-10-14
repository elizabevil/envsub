package extended_test

import (
	"context"
	"maps"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/engine/extended"
	"github.com/elizabevil/envsub/internal/schema"
	"github.com/elizabevil/envsub/internal/source"
	"github.com/elizabevil/envsub/internal/values"
)

// render runs a template with the given tree, schema and options.
func render(t *testing.T, tmpl string, tree *values.Tree, sch *schema.Schema, opts engine.Options) (string, error) {
	t.Helper()
	if tree == nil {
		tree = values.NewTree()
	}
	out, err := extended.New().Render(context.Background(), engine.Input{
		Template: []byte(tmpl),
		Values:   tree,
		Schema:   sch,
		Options:  opts,
	})
	return string(out), err
}

// newTree builds a value tree from root entries.
//
// A Tree memoizes compiled paths, so subtests that run in parallel must each
// get their own instance; sharing one would be a data race inside the memo.
func newTree(entries map[string]any) *values.Tree {
	tree := values.NewTree()
	maps.Copy(tree.Root, entries)
	return tree
}

// mustSchema narrows schema loading failures to test failures.
func mustSchema(t *testing.T, document string) *schema.Schema {
	t.Helper()
	tree, err := source.YAML{}.Load([]byte(document))
	if err != nil {
		t.Fatalf("load schema: %v", err)
	}
	sch, err := schema.Load(context.Background(), tree.Root, "schema.yaml", schema.UnknownIgnore)
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}
	return sch
}

// TestInlineTypeErrors keeps only the case the fixture suite cannot express:
// applying an inline `type string` to a structured (array) value. The other
// forms are covered by testdata fixtures:
//   - invalid int          -> inline_type/int_rejects_float, int_rejects_blank
//   - type after formatter -> inline_type/type_after_formatter (exit 3)
//   - unknown formatter    -> pipeline/unknown_formatter (exit 3)
func TestInlineTypeErrors(t *testing.T) {
	t.Parallel()
	buildTree := func() *values.Tree {
		return newTree(map[string]any{
			"HOSTS": []any{"a"},
		})
	}

	tests := []struct {
		name string
		tmpl string
		kind diag.Kind
	}{
		{name: "structured mismatch", tmpl: "${HOSTS | type string}", kind: diag.KindType},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := render(t, tt.tmpl, buildTree(), nil, engine.Options{})
			if err == nil {
				t.Fatalf("Render() error = nil, want error")
			}
			if got := diag.KindOf(err); got != tt.kind {
				t.Fatalf("error kind = %q, want %q (err=%v)", got, tt.kind, err)
			}
			if got := diag.ExitCode(err); got != 4 {
				t.Fatalf("ExitCode() = %d, want 4 (err=%v)", got, err)
			}
		})
	}
}

func TestSchemaAndInlineTypeUnion(t *testing.T) {
	t.Parallel()
	sch := mustSchema(t, `
type: object
properties:
  PORT:
    type: integer
  HOSTS:
    type: array
    items:
      type: string
`)
	buildTree := func() *values.Tree {
		return newTree(map[string]any{
			"PORT":  "8080",
			"HOSTS": `["a"]`,
		})
	}

	tests := []struct {
		name    string
		tmpl    string
		want    string
		wantErr bool
	}{
		{name: "compatible int", tmpl: "${PORT | type int}", want: "8080"},
		{name: "schema int accepts float assertion", tmpl: "${PORT | type float}", want: "8080"},
		{name: "schema array with inline array", tmpl: "${HOSTS | type array<string> | toJson}", want: `["a"]`},
		{name: "conflict", tmpl: "${PORT | type string}", wantErr: true},
		{name: "conflict bool", tmpl: "${PORT | type bool}", wantErr: true},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := render(t, tt.tmpl, buildTree(), sch, engine.Options{})
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Render() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("Render() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUnsetZero(t *testing.T) {
	t.Parallel()
	sch := mustSchema(t, `
type: object
properties:
  port: {type: integer}
  ratio: {type: number}
  debug: {type: boolean}
  hosts: {type: array}
  app: {type: object}
  name: {type: string}
`)
	tests := []struct {
		name    string
		tmpl    string
		want    string
		wantErr bool
	}{
		{name: "int", tmpl: "${port}", want: "0"},
		{name: "float", tmpl: "${ratio}", want: "0"},
		{name: "bool", tmpl: "${debug}", want: "false"},
		{name: "array", tmpl: "${hosts | toJson}", want: "[]"},
		{name: "object", tmpl: "${app | toJson}", want: "{}"},
		{name: "string", tmpl: "x${name}y", want: "xy"},
		{name: "inline type", tmpl: "${other | type int}", want: "0"},
		{name: "unknown type", tmpl: "${other}", wantErr: true},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := render(t, tt.tmpl, values.NewTree(), sch, engine.Options{Unset: engine.UnsetZero})
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Render() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("Render() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOnlyAndIgnore(t *testing.T) {
	t.Parallel()
	buildTree := func() *values.Tree {
		return newTree(map[string]any{
			"app":  map[string]any{"port": 8080},
			"NAME": "world",
		})
	}
	tmpl := "app=${app.port} name=$NAME"

	tests := []struct {
		name string
		opts engine.Options
		want string
	}{
		{name: "no filter", opts: engine.Options{}, want: "app=8080 name=world"},
		{name: "only root", opts: engine.Options{Only: []string{"app"}}, want: "app=8080 name=$NAME"},
		{name: "only full path", opts: engine.Options{Only: []string{"app.port"}}, want: "app=8080 name=$NAME"},
		{name: "only no match", opts: engine.Options{Only: []string{"other"}}, want: "app=${app.port} name=$NAME"},
		{name: "ignore root", opts: engine.Options{Ignore: []string{"app"}}, want: "app=${app.port} name=world"},
		{name: "ignore exact", opts: engine.Options{Ignore: []string{"NAME"}}, want: "app=8080 name=$NAME"},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := render(t, tmpl, buildTree(), nil, tt.opts)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("Render() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMissingErrorMentionsOrigin(t *testing.T) {
	t.Parallel()
	tree := values.NewTree()
	tree.RecordMeta(metaSegments(t, "app"), values.Meta{File: "values.yaml", Kind: values.SourceValues})

	_, err := render(t, "${app.port}", tree, nil, engine.Options{})
	if err == nil {
		t.Fatalf("Render() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "values.yaml") {
		t.Fatalf("Render() error = %q, want it to mention values.yaml", err.Error())
	}
}
