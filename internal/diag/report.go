package diag

// Report is the machine-readable form of a failure, for --error-format=json.
// It carries no more than what a caller can act on: how the failure was
// classified, the process exit code it maps to, and the message.
type Report struct {
	// Kind is the error classification, empty for an unclassified error.
	Kind Kind `json:"kind"`
	// ExitCode is the process exit code the error maps to.
	ExitCode int `json:"exitCode"`
	// Message is the rendered error text, including any file:line prefix.
	Message string `json:"message"`
}

// NewReport builds the machine-readable form of err. A nil error yields the
// zero Report; the caller decides whether there is anything to report.
func NewReport(err error) Report {
	report := Report{Kind: KindOf(err), ExitCode: ExitCode(err)}
	if err != nil {
		report.Message = err.Error()
	}
	return report
}
