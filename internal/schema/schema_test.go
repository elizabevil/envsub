package schema_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/codec"
	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/logging"
	"github.com/elizabevil/envsub/internal/schema"
	"github.com/elizabevil/envsub/internal/values"
)

const draft2020 = "https://json-schema.org/draft/2020-12/schema"

func TestLoadNormalizesTypeAliases(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"$schema": draft2020,
		"type":    "object",
		"properties": map[string]any{
			"port":  map[string]any{"type": "int"},
			"ratio": map[string]any{"type": "float"},
			"debug": map[string]any{"type": "bool"},
			"hosts": map[string]any{"type": "array<string>"},
		},
	}
	sch, err := schema.Load(context.Background(), raw, "schema.yaml", schema.UnknownIgnore)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	tests := map[string]string{
		"port":  "integer",
		"ratio": "number",
		"debug": "boolean",
		"hosts": "array",
	}
	for path, want := range tests {
		if got := sch.DeclaredType(path); got != want {
			t.Fatalf("DeclaredType(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestLoadUnknownKeywordPolicy(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"type":          "object",
		"customKeyword": true,
		"x-allowed":     true,
	}
	tests := []struct {
		name     string
		policy   schema.UnknownPolicy
		wantErr  bool
		wantWarn string
	}{
		{name: "ignore", policy: schema.UnknownIgnore},
		{name: "warn", policy: schema.UnknownWarn, wantWarn: "customKeyword"},
		{name: "error", policy: schema.UnknownError, wantErr: true},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var logged bytes.Buffer
			logger, err := logging.New(logging.LevelDebugName, &logged)
			if err != nil {
				t.Fatalf("logging.New() error = %v", err)
			}
			ctx := logging.WithLogger(context.Background(), logger)
			_, err = schema.Load(ctx, raw, "schema.yaml", tt.policy)
			if tt.wantErr && err == nil {
				t.Fatalf("Load() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if tt.wantWarn != "" {
				if !strings.Contains(logged.String(), "unknown schema keyword") {
					t.Fatalf("Load(warn) log = %q, want it to report the unknown keyword", logged.String())
				}
				if !strings.Contains(logged.String(), tt.wantWarn) {
					t.Fatalf("Load(warn) log = %q, want it to name %q", logged.String(), tt.wantWarn)
				}
			} else if logged.Len() != 0 {
				t.Fatalf("Load(%s) log = %q, want no warning", tt.policy, logged.String())
			}
			// x-* keywords are the extension escape hatch and are never reported.
			if strings.Contains(logged.String(), "x-allowed") {
				t.Fatalf("Load(%s) log = %q, want no warning for the x- keyword", tt.policy, logged.String())
			}
			if tt.wantErr {
				if kind := diag.KindOf(err); kind != diag.KindSchema {
					t.Fatalf("Load(error) kind = %q, want %q", kind, diag.KindSchema)
				}
			}
		})
	}
}

func TestLoadRejectsUnsupportedType(t *testing.T) {
	t.Parallel()
	raw := map[string]any{"type": "object", "properties": map[string]any{"a": map[string]any{"type": "map"}}}
	if _, err := schema.Load(context.Background(), raw, "schema.yaml", schema.UnknownIgnore); err == nil {
		t.Fatalf("Load() error = nil, want error")
	}
}

func TestApplyDefaults(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"app": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"port":  map[string]any{"type": "integer", "default": 8080},
					"ratio": map[string]any{"type": "number", "default": 0.5},
					"name":  map[string]any{"type": "string"},
				},
			},
		},
	}
	sch, err := schema.Load(context.Background(), raw, "schema.yaml", schema.UnknownIgnore)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	tree := values.NewTree()
	tree.Root["app"] = map[string]any{"name": "x"}
	sch.ApplyDefaults(tree)

	if got, _, _ := tree.Resolve("app.port"); got != 8080 {
		t.Fatalf("app.port = %#v, want 8080", got)
	}
	if got, _, _ := tree.Resolve("app.ratio"); got != 0.5 {
		t.Fatalf("app.ratio = %#v, want 0.5", got)
	}
	if got, _, _ := tree.Resolve("app.name"); got != "x" {
		t.Fatalf("app.name = %#v, want x", got)
	}
	if meta := tree.MetaFor("app.port"); !meta.Found || meta.Meta.Kind != string(values.SourceSchema) {
		t.Fatalf("app.port origin = %#v, want schema", meta)
	}
}

// TestApplyDefaultsDefersToRootValue pins rule G21: a schema default is the
// last-resort fallback, so a value any explicit source provides keeps priority
// over it while a key no source provides still receives the default. The
// @process namespace does not compete with defaults.
func TestApplyDefaultsDefersToRootValue(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"K": map[string]any{"type": "string", "default": "from_default"},
			"D": map[string]any{"type": "string", "default": "from_default"},
		},
	}
	sch, err := schema.Load(context.Background(), raw, "schema.yaml", schema.UnknownIgnore)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	tree := values.NewTree()
	tree.Root["K"] = "from_dotenv"
	tree.Env["D"] = "from_process"
	sch.ApplyDefaults(tree)

	if got := tree.Root["K"]; got != "from_dotenv" {
		t.Fatalf("K = %#v, want the source value to keep priority over the default", got)
	}
	// The read path sees the same value: a default must not overwrite a source
	// value anywhere the template can reach it.
	if got, found, err := tree.Resolve("K"); err != nil || !found || got != "from_dotenv" {
		t.Fatalf("Resolve(K) = (%#v, %v, %v), want the source value to keep priority over the default", got, found, err)
	}
	// D is only in the process environment, which is not a root source, so the
	// default still applies to the root key.
	if got := tree.Root["D"]; got != "from_default" {
		t.Fatalf("D = %#v, want the schema default (@process does not fill a root key)", got)
	}
}

func TestMaterialize(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"hosts": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"app":   map[string]any{"type": "object"},
			"name":  map[string]any{"type": "string"},
		},
	}
	sch, err := schema.Load(context.Background(), raw, "schema.yaml", schema.UnknownIgnore)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	tree := values.NewTree()
	tree.Root["hosts"] = `["a","b"]`
	tree.Root["app"] = `{"port":1}`
	tree.Root["name"] = `["not","parsed"]`
	tree.RecordEncodedMeta("hosts", values.Meta{File: ".env", Line: 3, Kind: string(values.SourceDotenv)})

	if err := sch.Materialize(tree); err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	hosts, _, _ := tree.Resolve("hosts[0]")
	if hosts != "a" {
		t.Fatalf("hosts[0] = %#v, want a", hosts)
	}
	port, _, _ := tree.Resolve("app.port")
	if port != 1 {
		t.Fatalf("app.port = %#v, want 1", port)
	}
	// A string-typed key keeps its text: materialization parses only the types
	// the schema declares as array or object.
	if got, _, _ := tree.Resolve("name"); got != `["not","parsed"]` {
		t.Fatalf("string-typed key was parsed: %#v", got)
	}
}

func TestMaterializeErrorMentionsOrigin(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"type":       "object",
		"properties": map[string]any{"hosts": map[string]any{"type": "array"}},
	}
	sch, err := schema.Load(context.Background(), raw, "schema.yaml", schema.UnknownIgnore)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	tree := values.NewTree()
	tree.Root["hosts"] = "not json"
	tree.RecordEncodedMeta("hosts", values.Meta{File: ".env", Line: 7, Kind: string(values.SourceDotenv)})

	err = sch.Materialize(tree)
	if err == nil {
		t.Fatalf("Materialize() error = nil, want error")
	}
	message := err.Error()
	for _, want := range []string{"hosts", ".env:7"} {
		if !strings.Contains(message, want) {
			t.Fatalf("Materialize() error = %q, want it to contain %q", message, want)
		}
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"port": map[string]any{"type": "integer", "minimum": 1, "maximum": 65535},
			"mode": map[string]any{"type": "string", "enum": []any{"dev", "prod"}},
		},
		"required": []any{"port"},
	}
	sch, err := schema.Load(context.Background(), raw, "schema.yaml", schema.UnknownIgnore)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	valid := values.NewTree()
	valid.Root["port"] = 8080
	valid.Root["mode"] = "dev"
	if err := sch.Validate(valid); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	invalid := values.NewTree()
	invalid.Root["port"] = 0
	if err := sch.Validate(invalid); err == nil {
		t.Fatalf("Validate() error = nil, want error")
	} else if !strings.Contains(err.Error(), "/port") {
		t.Fatalf("Validate() error = %q, want it to mention /port", err.Error())
	}

	missing := values.NewTree()
	if err := sch.Validate(missing); err == nil {
		t.Fatalf("Validate() error = nil, want required error")
	}
}

func TestParseUnknownPolicy(t *testing.T) {
	t.Parallel()
	if _, err := schema.ParseUnknownPolicy(""); err != nil {
		t.Fatalf("ParseUnknownPolicy(\"\") error = %v", err)
	}
	if policy, err := schema.ParseUnknownPolicy("warn"); err != nil || policy != schema.UnknownWarn {
		t.Fatalf("ParseUnknownPolicy(warn) = %v/%v", policy, err)
	}
	if _, err := schema.ParseUnknownPolicy("bogus"); err == nil {
		t.Fatalf("ParseUnknownPolicy(bogus) error = nil, want error")
	}
}

// TestValidateExactInteger covers step 4 of the serialization rules: an integer
// outside uint64 reaches the validator as json.Number, and validating must not
// disturb the tree that later gets rendered.
func TestValidateExactInteger(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{"type": "integer"},
		},
	}
	sch, err := schema.Load(context.Background(), raw, "schema.yaml", schema.UnknownIgnore)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	tree := values.NewTree()
	tree.Root["id"] = codec.ExactInt("123456789012345678901234567890")
	if err := sch.Validate(tree); err != nil {
		t.Fatalf("Validate(exact integer) error = %v", err)
	}
	if got, _, _ := tree.Resolve("id"); got != codec.ExactInt("123456789012345678901234567890") {
		t.Fatalf("Validate() changed the tree: id = %#v (%T)", got, got)
	}

	wrong := values.NewTree()
	wrong.Root["id"] = "abc"
	if err := sch.Validate(wrong); err == nil {
		t.Fatal("Validate(string) error = nil, want a type error")
	}
}

// TestMaterializeExactInteger covers the other half of step 3/4: a string from
// .env or --set materialized against an integer schema keeps its exact digits
// instead of failing at the int64 limit.
func TestMaterializeExactInteger(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"big":   map[string]any{"type": "integer"},
			"u64":   map[string]any{"type": "integer"},
			"small": map[string]any{"type": "integer"},
		},
	}
	sch, err := schema.Load(context.Background(), raw, "schema.yaml", schema.UnknownIgnore)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	tree := values.NewTree()
	tree.Root["big"] = "123456789012345678901234567890"
	tree.Root["u64"] = "18446744073709551615"
	tree.Root["small"] = "8080"
	if err := sch.Materialize(tree); err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if got, _, _ := tree.Resolve("big"); got != codec.ExactInt("123456789012345678901234567890") {
		t.Fatalf("big = %#v (%T), want codec.ExactInt", got, got)
	}
	if got, _, _ := tree.Resolve("u64"); got != uint64(18446744073709551615) {
		t.Fatalf("u64 = %#v (%T), want uint64", got, got)
	}
	if got, _, _ := tree.Resolve("small"); got != 8080 {
		t.Fatalf("small = %#v (%T), want int", got, got)
	}
	if err := sch.Validate(tree); err != nil {
		t.Fatalf("Validate() after materialize error = %v", err)
	}
}

// TestDeclaredTypeUnion covers G11: a union type materializes as its first
// non-null member, with null acting only as a nullability marker.
func TestDeclaredTypeUnion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		schemaType any
		want       string
	}{
		{name: "plain", schemaType: "integer", want: "integer"},
		{name: "integer or null", schemaType: []any{"integer", "null"}, want: "integer"},
		{name: "null first", schemaType: []any{"null", "integer"}, want: "integer"},
		{name: "null only", schemaType: []any{"null"}, want: "null"},
		{name: "string before integer", schemaType: []any{"string", "integer"}, want: "string"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			raw := map[string]any{
				"type": "object",
				"properties": map[string]any{
					"x": map[string]any{"type": tt.schemaType},
				},
			}
			sch, err := schema.Load(context.Background(), raw, "schema.yaml", schema.UnknownIgnore)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if got := sch.DeclaredType("x"); got != tt.want {
				t.Fatalf("DeclaredType(x) = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestMaterializeUnionType materializes a string against ["integer","null"].
func TestMaterializeUnionType(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"x": map[string]any{"type": []any{"integer", "null"}},
		},
	}
	sch, err := schema.Load(context.Background(), raw, "schema.yaml", schema.UnknownIgnore)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	tree := values.NewTree()
	tree.Root["x"] = "8080"
	if err := sch.Materialize(tree); err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if got, _, _ := tree.Resolve("x"); got != 8080 {
		t.Fatalf("x = %#v (%T), want int 8080", got, got)
	}
}

// TestLoadRejectsNonStringUnionMember keeps the metaschema contract: a union
// member must be a type name, so the library rejects anything else.
func TestLoadRejectsNonStringUnionMember(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"x": map[string]any{"type": []any{1, 2}},
		},
	}
	if _, err := schema.Load(context.Background(), raw, "schema.yaml", schema.UnknownIgnore); err == nil {
		t.Fatal("Load() error = nil, want a metaschema error for a non-string union member")
	}
}
