//go:build !unix

package perf_test

import "os"

// maxRSS is unavailable outside unix; the gate skips the RSS ratio there.
func maxRSS(*os.ProcessState) (int64, bool) {
	return 0, false
}
