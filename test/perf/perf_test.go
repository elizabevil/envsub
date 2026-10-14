// Package perf_test is the TASK-023 performance gate: it renders the REF-04
// baseline template with GNU envsubst and with envsub, and fails when envsub
// exceeds the configured multiple of the GNU baseline or the startup budget.
//
// The gate compares total wall time on the same machine, so it is a ratio, not
// an absolute number. Thresholds can be tuned per runner with PERF_RUNS,
// PERF_MAX_TIME_RATIO, PERF_MAX_RSS_RATIO and PERF_MAX_STARTUP_MS. GNU envsubst
// is required for the comparison; without it those assertions are skipped and
// only the absolute startup budget is enforced.
package perf_test

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	// baselineTemplateBytes and baselineTemplateLines are the REF-04 shape.
	baselineTemplateBytes = 10_466_670
	baselineTemplateLines = 200_000
	// baselineLinePrefix is len("host_000000=${HOST_000000} ").
	baselineLinePrefix = 28
	// baselineLinePadding completes each line to the REF-04 total.
	baselineLinePadding = 24

	// minComparableBaseline is the smallest GNU time worth taking a ratio of.
	minComparableBaseline = 5 * time.Millisecond

	defaultRuns          = 7
	defaultTimeRatio     = 5.0
	defaultRSSRatio      = 5.0
	defaultStartupBudget = 100 * time.Millisecond

	envRuns          = "PERF_RUNS"
	envTimeRatio     = "PERF_MAX_TIME_RATIO"
	envRSSRatio      = "PERF_MAX_RSS_RATIO"
	envStartupBudget = "PERF_MAX_STARTUP_MS"
)

var (
	// envsubPath is the binary built from the current sources.
	envsubPath string
	// envsubstPath is the GNU baseline, empty when it is not installed.
	envsubstPath string
	// templatePath is the generated REF-04 baseline template.
	templatePath string
)

func TestMain(m *testing.M) {
	code, err := runSuite(m)
	if err != nil {
		fmt.Fprintln(os.Stderr, "perf setup:", err)
		os.Exit(1)
	}
	os.Exit(code)
}

// runSuite builds the binary and the baseline template, then runs the tests.
func runSuite(m *testing.M) (int, error) {
	workDir, err := os.MkdirTemp("", "envsub-perf-*")
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = os.RemoveAll(workDir)
	}()

	root, err := repositoryRoot()
	if err != nil {
		return 0, err
	}
	envsubPath = filepath.Join(workDir, "envsub")
	build := exec.Command("go", "build", "-trimpath", "-ldflags", "-s -w", "-o", envsubPath, "./cmd/envsub")
	build.Dir = root
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		return 0, fmt.Errorf("build envsub: %w\n%s", err, out)
	}

	templatePath = filepath.Join(workDir, "baseline.tmpl")
	if err := writeBaselineTemplate(templatePath); err != nil {
		return 0, err
	}
	if path, err := exec.LookPath("envsubst"); err == nil {
		envsubstPath = path
	}
	return m.Run(), nil
}

// writeBaselineTemplate writes the REF-04 template: 200,000 lines of
// "host_NNNNNN=${HOST_NNNNNN} <padding>", 10,466,670 bytes in total.
func writeBaselineTemplate(path string) (err error) {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	writer := bufio.NewWriterSize(file, 1<<20)
	// Lines 0..extra-1 carry one padding byte more, which is how the exact
	// REF-04 size is reached with a whole number of lines.
	extra := baselineTemplateBytes - baselineTemplateLines*(baselineLinePrefix+baselineLinePadding)
	padding := strings.Repeat("x", baselineLinePadding+1)
	for i := range baselineTemplateLines {
		width := baselineLinePadding
		if i < extra {
			width++
		}
		if _, err := fmt.Fprintf(writer, "host_%06d=${HOST_%06d} %s\n", i, i, padding[:width]); err != nil {
			return err
		}
	}
	if err := writer.Flush(); err != nil {
		return err
	}

	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() != baselineTemplateBytes {
		return fmt.Errorf("baseline template is %d bytes, want %d", info.Size(), baselineTemplateBytes)
	}
	return nil
}

// TestPerformanceGate enforces the render-time and peak-RSS ratios.
func TestPerformanceGate(t *testing.T) {
	requireBaseline(t)
	runs := envInt(t, envRuns, defaultRuns)
	timeLimit := envFloat(t, envTimeRatio, defaultTimeRatio)
	rssLimit := envFloat(t, envRSSRatio, defaultRSSRatio)

	// Warm the page cache and each binary before measuring.
	runTemplate(t, envsubstPath)
	runTemplate(t, envsubPath, "render", "--on-missing=empty")

	gnuTime, gnuRSS, gnuRSSKnown := sampleTemplate(t, runs, envsubstPath)
	envsubTime, envsubRSS, envsubRSSKnown := sampleTemplate(t, runs, envsubPath, "render", "--on-missing=empty")

	timeRatio := float64(envsubTime) / float64(gnuTime)
	t.Logf("perf: render GNU=%v envsub=%v ratio=%.2fx limit=%.1fx runs=%d",
		gnuTime, envsubTime, timeRatio, timeLimit, runs)
	if gnuTime < minComparableBaseline {
		t.Logf("perf: GNU baseline %v is below the %v comparability floor, ratio not enforced",
			gnuTime, minComparableBaseline)
	} else if timeRatio > timeLimit {
		t.Fatalf("render is %.2fx the GNU baseline (limit %.1fx): GNU %v, envsub %v",
			timeRatio, timeLimit, gnuTime, envsubTime)
	}

	if !gnuRSSKnown || !envsubRSSKnown {
		t.Log("perf: peak RSS is not available on this platform, ratio not enforced")
		return
	}
	rssRatio := float64(envsubRSS) / float64(gnuRSS)
	t.Logf("perf: rss GNU=%s envsub=%s ratio=%.2fx limit=%.1fx",
		bytesLabel(gnuRSS), bytesLabel(envsubRSS), rssRatio, rssLimit)
	if rssRatio > rssLimit {
		t.Fatalf("peak RSS is %.2fx the GNU baseline (limit %.1fx): GNU %s, envsub %s",
			rssRatio, rssLimit, bytesLabel(gnuRSS), bytesLabel(envsubRSS))
	}
}

// TestStartupWithinBudget enforces the absolute startup target from REF-04.
func TestStartupWithinBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("performance gate skipped in short mode")
	}
	runs := envInt(t, envRuns, defaultRuns)
	budget := envMilliseconds(t, envStartupBudget, defaultStartupBudget)
	startup := sampleStartup(t, runs, envsubPath, "render", "--on-missing=empty")
	t.Logf("perf: startup envsub=%v budget=%v runs=%d", startup, budget, runs)
	if startup > budget {
		t.Fatalf("startup is %v, over the %v budget", startup, budget)
	}
}

// TestRenderMatchesGNUBaselineOutput keeps the comparison honest: a renderer
// that skipped the substitution would otherwise look fast.
func TestRenderMatchesGNUBaselineOutput(t *testing.T) {
	requireBaseline(t)
	want, err := renderToBytes(t, envsubstPath)
	if err != nil {
		t.Fatalf("GNU envsubst: %v", err)
	}
	got, err := renderToBytes(t, envsubPath, "render", "--on-missing=empty")
	if err != nil {
		t.Fatalf("envsub: %v", err)
	}
	if !bytes.Equal(want, got) {
		t.Fatalf("output differs from the GNU baseline: envsub %d bytes, GNU %d bytes", len(got), len(want))
	}
	t.Logf("perf: baseline output matches byte for byte (%d bytes)", len(got))
}

// requireBaseline skips the comparison when it cannot be meaningful.
func requireBaseline(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("performance gate skipped in short mode")
	}
	if envsubstPath == "" {
		t.Skip("GNU envsubst is not in PATH; install gettext-base to run the baseline comparison")
	}
}

// sampleTemplate runs a command over the baseline template and reports the
// median wall time plus the largest peak RSS observed.
func sampleTemplate(t *testing.T, runs int, name string, args ...string) (time.Duration, int64, bool) {
	t.Helper()
	times := make([]time.Duration, 0, runs)
	var peak int64
	rssKnown := false
	for range runs {
		elapsed, rss, ok := runTemplate(t, name, args...)
		times = append(times, elapsed)
		if ok {
			rssKnown = true
			peak = max(peak, rss)
		}
	}
	return median(times), peak, rssKnown
}

// runTemplate renders the baseline template once, measuring wall time and RSS.
func runTemplate(t *testing.T, name string, args ...string) (time.Duration, int64, bool) {
	t.Helper()
	file, err := os.Open(templatePath)
	if err != nil {
		t.Fatalf("open template: %v", err)
	}
	defer func() {
		_ = file.Close()
	}()

	cmd := exec.Command(name, args...)
	cmd.Stdin = file
	cmd.Stdout = io.Discard
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	start := time.Now()
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s %v: %v: %s", name, args, err, stderr.String())
	}
	rss, ok := maxRSS(cmd.ProcessState)
	return time.Since(start), rss, ok
}

// sampleStartup reports the median wall time of an empty-input invocation.
func sampleStartup(t *testing.T, runs int, name string, args ...string) time.Duration {
	t.Helper()
	times := make([]time.Duration, 0, runs)
	for range runs {
		cmd := exec.Command(name, args...)
		cmd.Stdout = io.Discard
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		start := time.Now()
		if err := cmd.Run(); err != nil {
			t.Fatalf("%s %v: %v: %s", name, args, err, stderr.String())
		}
		times = append(times, time.Since(start))
	}
	return median(times)
}

// renderToBytes captures the full rendered output of the baseline template.
func renderToBytes(t *testing.T, name string, args ...string) ([]byte, error) {
	t.Helper()
	file, err := os.Open(templatePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	cmd := exec.Command(name, args...)
	cmd.Stdin = file
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%v: %w: %s", args, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// median returns the middle value, so a single noisy run cannot decide the gate.
func median(values []time.Duration) time.Duration {
	sorted := append([]time.Duration(nil), values...)
	slices.Sort(sorted)
	return sorted[len(sorted)/2]
}

// bytesLabel renders a byte count for the log line.
func bytesLabel(size int64) string {
	return fmt.Sprintf("%.1fMiB", float64(size)/(1<<20))
}

// repositoryRoot walks up from the test directory to the module root.
func repositoryRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}

// envInt reads a positive integer override, failing on a malformed value.
func envInt(t *testing.T, name string, fallback int) int {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		t.Fatalf("%s=%q is not a positive integer", name, raw)
	}
	return value
}

// envFloat reads a positive ratio override, failing on a malformed value.
func envFloat(t *testing.T, name string, fallback float64) float64 {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value <= 0 {
		t.Fatalf("%s=%q is not a positive number", name, raw)
	}
	return value
}

// envMilliseconds reads a positive duration override in milliseconds.
func envMilliseconds(t *testing.T, name string, fallback time.Duration) time.Duration {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		t.Fatalf("%s=%q is not a positive number of milliseconds", name, raw)
	}
	return time.Duration(value) * time.Millisecond
}
