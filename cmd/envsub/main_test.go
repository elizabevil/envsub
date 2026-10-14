package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestRunExposesExitCodes pins the one piece of logic cmd/envsub owns: main
// delegates to run, and run returns the process exit code app.Run produced.
func TestRunExposesExitCodes(t *testing.T) {
	// run writes diagnostics to os.Stderr; keep the test output clean.
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer func() { _ = devNull.Close() }()

	oldArgs, oldStderr := os.Args, os.Stderr
	defer func() { os.Args, os.Stderr = oldArgs, oldStderr }()
	os.Stderr = devNull

	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "version succeeds", args: []string{"version"}, want: 0},
		{name: "unknown flag is a usage error", args: []string{"render", "--nope"}, want: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Args = append([]string{"envsub"}, tt.args...)
			if got := run(); got != tt.want {
				t.Fatalf("run(%v) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}

// TestRunRendersTemplate confirms run is wired to the render command instead of
// returning a constant.
func TestRunRendersTemplate(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "input.tmpl")
	if err := os.WriteFile(tmpl, []byte("ok\n"), 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	oldArgs, oldStdout := os.Args, os.Stdout
	defer func() { os.Args, os.Stdout = oldArgs, oldStdout }()
	os.Args = []string{"envsub", "render", "--no-process-env", tmpl}
	os.Stdout = outW

	code := run()
	_ = outW.Close()
	os.Stdout = oldStdout
	data, err := io.ReadAll(outR)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	_ = outR.Close()

	if code != 0 {
		t.Fatalf("run() = %d, want 0", code)
	}
	if got := string(data); got != "ok\n" {
		t.Fatalf("rendered = %q, want %q", got, "ok\n")
	}
}
