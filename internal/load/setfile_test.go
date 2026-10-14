package load_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/load"
)

// TestSetFileReadsTheFilesBytes pins --set-file: the value is the file's content
// with one trailing newline dropped, and no other transformation.
func TestSetFileReadsTheFilesBytes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, content, want string }{
		{name: "trailing newline", content: "secret\n", want: "secret"},
		{name: "crlf", content: "secret\r\n", want: "secret"},
		{name: "no trailing newline", content: "secret", want: "secret"},
		{name: "multi-line keeps inner newlines", content: "a\nb\n", want: "a\nb"},
		{name: "leading BOM", content: "\ufeffsecret\n", want: "secret"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := filepath.Join(dir, "value.txt")
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatalf("write value file: %v", err)
			}
			result, err := load.Load(context.Background(), load.Spec{
				NoProcessEnv: true,
				SetFiles:     []string{"TOKEN=" + path},
			}, nil)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			got, ok := result.Tree.ResolveName("TOKEN")
			if !ok {
				t.Fatal("TOKEN was not set")
			}
			if got != tc.want {
				t.Errorf("TOKEN = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestSetFileWinsOverSet pins the order of the assignment layer: --set-file is
// applied after --set and --set-json.
func TestSetFileWinsOverSet(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "value.txt")
	if err := os.WriteFile(path, []byte("from_file\n"), 0o600); err != nil {
		t.Fatalf("write value file: %v", err)
	}
	result, err := load.Load(context.Background(), load.Spec{
		NoProcessEnv: true,
		Sets:         []string{"TOKEN=from_set"},
		SetFiles:     []string{"TOKEN=" + path},
	}, nil)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	got, _ := result.Tree.ResolveName("TOKEN")
	if got != "from_file" {
		t.Errorf("TOKEN = %q, want the --set-file value", got)
	}
}

// TestSetFileReportsAnUnreadableFile pins the failure classification: a missing
// file is an IO error, not a usage error.
func TestSetFileReportsAnUnreadableFile(t *testing.T) {
	t.Parallel()
	_, err := load.Load(context.Background(), load.Spec{
		NoProcessEnv: true,
		SetFiles:     []string{"TOKEN=" + filepath.Join(t.TempDir(), "absent.txt")},
	}, nil)
	if err == nil {
		t.Fatal("Load() error = nil, want a read failure")
	}
	if got := diag.KindOf(err); got != diag.KindIO {
		t.Fatalf("Load() error kind = %v (%v), want KindIO", got, err)
	}
	if got := diag.ExitCode(err); got != 5 {
		t.Fatalf("Load() exit = %d (%v), want 5", got, err)
	}
	if !strings.Contains(err.Error(), "read --set-file") {
		t.Errorf("error = %v, want it to name the --set-file read failure", err)
	}
}
