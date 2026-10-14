package diag_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/diag"
)

// TestReportMatchesTheTextPath pins that the JSON report and the text output
// agree on the two fields a caller branches on: the classification and the exit
// code.
func TestReportMatchesTheTextPath(t *testing.T) {
	t.Parallel()
	for _, err := range []error{
		diag.New(diag.KindUsage, "bad flags"),
		diag.New(diag.KindDiff, "1 of 1 outputs are outdated: out.txt"),
		diag.Wrap(diag.KindIO, errors.New("boom")),
		errors.New("plain failure"),
	} {
		report := diag.NewReport(err)
		if report.ExitCode != diag.ExitCode(err) {
			t.Errorf("Report.ExitCode = %d, want %d", report.ExitCode, diag.ExitCode(err))
		}
		if report.Kind != diag.KindOf(err) {
			t.Errorf("Report.Kind = %q, want %q", report.Kind, diag.KindOf(err))
		}
		if report.Message == "" {
			t.Error("Report.Message is empty")
		}
	}
}

// TestReportIsJSONStable pins the wire format: the field names are a contract for
// callers parsing the error stream.
func TestReportIsJSONStable(t *testing.T) {
	t.Parallel()
	data, err := json.Marshal(diag.NewReport(diag.New(diag.KindUsage, "bad flags")))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for _, want := range []string{`"kind":"usage"`, `"exitCode":2`, `"message":"bad flags"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("JSON %s does not contain %s", data, want)
		}
	}
}

// TestExitCodeCatalog pins the mapping from error classification to process exit
// code, including the codes the newer features added: 7 for an outdated --check,
// 1 for anything unclassified.
func TestExitCodeCatalog(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		err  error
		want int
	}{
		{err: nil, want: 0},
		{err: diag.New(diag.KindUsage, "x"), want: 2},
		{err: diag.New(diag.KindTemplate, "x"), want: 3},
		{err: diag.New(diag.KindType, "x"), want: 4},
		{err: diag.New(diag.KindSchema, "x"), want: 4},
		{err: diag.New(diag.KindIO, "x"), want: 5},
		{err: diag.New(diag.KindMissing, "x"), want: 6},
		{err: diag.New(diag.KindPolicy, "x"), want: 6},
		{err: diag.New(diag.KindDiff, "x"), want: 7},
		{err: diag.New(diag.KindInternal, "x"), want: 1},
		{err: diag.New(diag.Kind("bogus"), "x"), want: 1},
		{err: errors.New("plain"), want: 1},
	} {
		if got := diag.ExitCode(tc.err); got != tc.want {
			t.Errorf("ExitCode(%v) = %d, want %d", tc.err, got, tc.want)
		}
	}
}

// TestReportOfNilIsEmpty pins the zero value a caller gets when there is nothing
// to report, so a JSON writer never emits a half-filled object.
func TestReportOfNilIsEmpty(t *testing.T) {
	t.Parallel()
	if got := diag.NewReport(nil); got != (diag.Report{}) {
		t.Errorf("NewReport(nil) = %+v, want the zero Report", got)
	}
}
