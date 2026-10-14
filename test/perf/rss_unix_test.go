//go:build unix

package perf_test

import (
	"os"
	"runtime"
	"syscall"
)

// maxRSS reports the peak resident set size of a finished process in bytes.
// Linux reports the value in kilobytes, the other unixes in bytes.
func maxRSS(state *os.ProcessState) (int64, bool) {
	if state == nil {
		return 0, false
	}
	usage, ok := state.SysUsage().(*syscall.Rusage)
	if !ok {
		return 0, false
	}
	if runtime.GOOS == "linux" {
		return usage.Maxrss * 1024, true
	}
	return usage.Maxrss, true
}
