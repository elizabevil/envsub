package extended

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/values"
)

// TestParallelOutputMatchesSequential pins the parallel renderer to the
// buffered renderer byte for byte.
func TestParallelOutputMatchesSequential(t *testing.T) {
	t.Parallel()
	const variables = 60000
	var source strings.Builder
	tree := values.NewTree()
	for i := range variables {
		fmt.Fprintf(&source, "key%05d=${VAR%05d} tail\n", i, i)
		tree.Root[fmt.Sprintf("VAR%05d", i)] = fmt.Sprintf("value-%d", i)
	}
	template := []byte(source.String())

	path := filepath.Join(t.TempDir(), "big.tmpl")
	if err := os.WriteFile(path, template, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	}()

	ctx := context.Background()
	want, err := New().Render(ctx, engine.Input{Template: template, Values: tree})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	var got bytes.Buffer
	in := engine.Input{Name: "big.tmpl", Values: tree, SourceSize: int64(len(template)), SourceSizeKnown: true}
	parallel, err := New().shouldRenderParallel(ctx, file, in)
	if err != nil {
		t.Fatalf("shouldRenderParallel() error = %v", err)
	}
	if !parallel {
		t.Fatal("shouldRenderParallel() = false, want true so the streaming path under test is the parallel one")
	}
	if err := New().StreamTo(ctx, &got, file, in); err != nil {
		t.Fatalf("StreamTo() error = %v", err)
	}
	if !bytes.Equal(got.Bytes(), want) {
		t.Fatalf("StreamTo() = %d bytes, want %d bytes; outputs differ", got.Len(), len(want))
	}
}

// TestInspectText pins the precise detection: only the := and =
// operators count, at any nesting depth, and a literal "=" never does.
func TestInspectText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text string
		want bool
	}{
		{text: "key=${VAR}\n", want: false},
		{text: "$VAR and VAR\n", want: false},
		{text: "${V:=d}", want: true},
		{text: "${V=d}", want: true},
		{text: "${V:-d}", want: false},
		{text: "${V-d}", want: false},
		{text: "${V:+d}", want: false},
		{text: "${V:?msg}", want: false},
		{text: "${V | quote}", want: false},
		{text: "${V | type string}", want: false},
		{text: "${A:-${B:=x}}", want: true},
		{text: "${A:-${B}}", want: false},
		{text: "${A:-a=b}", want: false},
		{text: "${A:-\"x=y\"}", want: false},
		{text: "a=b\n", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			t.Parallel()
			got, err := inspectText([]byte(tt.text), nil)
			if err != nil {
				t.Fatalf("inspectText(%q) error = %v", tt.text, err)
			}
			if got != tt.want {
				t.Fatalf("inspectText(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

// TestAssignmentTemplateFallsBackToSequential checks that a large template
// which assigns to the value tree is rendered sequentially, and that the
// fallback output matches the buffered renderer byte for byte.
func TestAssignmentTemplateFallsBackToSequential(t *testing.T) {
	t.Parallel()
	var source strings.Builder
	for i := range 100000 {
		fmt.Fprintf(&source, "key%05d=${VAR}\n", i)
	}
	source.WriteString("assigned=${NEW:=written}\n")
	source.WriteString("nested=${OTHER:-${INNER:=deep}}\n")
	template := []byte(source.String())

	path := filepath.Join(t.TempDir(), "assign.tmpl")
	if err := os.WriteFile(path, template, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	}()

	newTree := func() *values.Tree {
		tree := values.NewTree()
		tree.Root["VAR"] = "value"
		return tree
	}

	ctx := context.Background()
	want, err := New().Render(ctx, engine.Input{Template: template, Values: newTree()})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	in := engine.Input{Name: "assign.tmpl", Values: newTree(), SourceSize: int64(len(template)), SourceSizeKnown: true}
	parallel, err := New().shouldRenderParallel(ctx, file, in)
	if err != nil {
		t.Fatalf("shouldRenderParallel() error = %v", err)
	}
	if parallel {
		t.Fatal("shouldRenderParallel() = true, want false for a template that assigns")
	}

	var got bytes.Buffer
	if err := New().StreamTo(ctx, &got, file, in); err != nil {
		t.Fatalf("StreamTo() error = %v", err)
	}
	if !bytes.Equal(got.Bytes(), want) {
		t.Fatalf("StreamTo() = %d bytes, want %d bytes; outputs differ", got.Len(), len(want))
	}
}

// TestParallelDottedPathsMatchSequential pins the no-lock rule: the sequential
// pre-scan must compile every path the workers resolve, so no worker writes the
// tree's path cache. The rendering tree is deliberately never warmed by an
// earlier Render call, which is what makes the race detectable.
func TestParallelDottedPathsMatchSequential(t *testing.T) {
	t.Parallel()
	const variables = 40000
	var source strings.Builder
	buildTree := func() *values.Tree {
		tree := values.NewTree()
		services := make(map[string]any, variables)
		for i := range variables {
			services[fmt.Sprintf("s%05d", i)] = map[string]any{
				"port":     8080 + i,
				"hosts":    []any{"a", "b"},
				"fallback": fmt.Sprintf("fallback-%d", i),
			}
		}
		tree.Root["app"] = services
		return tree
	}
	for i := range variables {
		fmt.Fprintf(&source, "a=${app.s%05d.port} b=${app.s%05d.hosts | join \",\"} c=${app.s%05d.missing:-${app.s%05d.fallback}}\n",
			i, i, i, i)
	}
	template := []byte(source.String())

	path := filepath.Join(t.TempDir(), "dotted.tmpl")
	if err := os.WriteFile(path, template, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	}()

	ctx := context.Background()
	want, err := New().Render(ctx, engine.Input{Template: template, Values: buildTree()})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	var got bytes.Buffer
	in := engine.Input{Name: "dotted.tmpl", Values: buildTree(), SourceSize: int64(len(template)), SourceSizeKnown: true}
	// Prove the input gate selects the parallel path, but probe with a separate
	// tree: the tree StreamTo renders must stay as cold as a first run leaves it,
	// so that StreamTo's own pre-scan is the only thing that compiles the paths
	// before the workers start.
	probe := engine.Input{Name: "dotted.tmpl", Values: buildTree(), SourceSize: int64(len(template)), SourceSizeKnown: true}
	parallel, err := New().shouldRenderParallel(ctx, file, probe)
	if err != nil {
		t.Fatalf("shouldRenderParallel() error = %v", err)
	}
	if !parallel {
		t.Fatal("shouldRenderParallel() = false, want true so the streaming path under test is the parallel one")
	}
	if err := New().StreamTo(ctx, &got, file, in); err != nil {
		t.Fatalf("StreamTo() error = %v", err)
	}
	if !bytes.Equal(got.Bytes(), want) {
		t.Fatalf("StreamTo() = %d bytes, want %d bytes; outputs differ", got.Len(), len(want))
	}
}

// TestShouldRenderParallelDecision pins the input gate: only a rewindable
// source with a known size larger than one window may be split.
func TestShouldRenderParallelDecision(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	bigPath := filepath.Join(dir, "big.tmpl")
	big := strings.Repeat("key=${VAR}\n", 100000) // ~1.1 MB, well over four windows
	if err := os.WriteFile(bigPath, []byte(big), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	bigFile, err := os.Open(bigPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if closeErr := bigFile.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	}()

	smallTemplate := "key=${VAR}\n"
	smallPath := filepath.Join(dir, "small.tmpl")
	if err := os.WriteFile(smallPath, []byte(smallTemplate), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	smallFile, err := os.Open(smallPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if closeErr := smallFile.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	}()

	tree := values.NewTree()
	tree.Root["VAR"] = "value"
	mutated := values.NewTree()
	mutated.Root["VAR"] = "value"
	if err := mutated.Store("NEW", "value", values.Origin{Kind: values.SourceSet, Method: values.MethodAssign, Source: "test.tmpl", File: "test.tmpl", Line: 1}); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	tests := []struct {
		name string
		file *os.File
		size int64
		ok   bool
		tree *values.Tree
		want bool
	}{
		{name: "large file with known size", file: bigFile, size: int64(len(big)), ok: true, tree: tree, want: true},
		{name: "unknown size stays sequential", file: bigFile, ok: false, tree: tree, want: false},
		{name: "single window stays sequential", file: smallFile, size: int64(len(smallTemplate)), ok: true, tree: tree, want: false},
		{name: "mutated tree stays sequential", file: bigFile, size: int64(len(big)), ok: true, tree: mutated, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := engine.Input{Values: tt.tree, SourceSize: tt.size, SourceSizeKnown: tt.ok}
			got, err := New().shouldRenderParallel(context.Background(), tt.file, in)
			if err != nil {
				t.Fatalf("shouldRenderParallel() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("shouldRenderParallel() = %v, want %v", got, tt.want)
			}
		})
	}
}
