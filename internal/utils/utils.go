// Package utils provides stdio path checks and small byte helpers.
package utils

// StdioPath is the conventional "-" path for stdin/stdout.
const StdioPath = "-"

// IsStdio reports whether path denotes standard input/output.
func IsStdio(path string) bool {
	return path == "" || path == StdioPath
}

// CloneBytes returns a copy of src.
//
// bytes.Clone is not a drop-in replacement: it returns a non-nil empty slice
// for a non-nil empty input, while this helper returns nil for any empty input.
// The slice idiom keeps that exact contract.
func CloneBytes(src []byte) []byte {
	return append([]byte(nil), src...)
}
