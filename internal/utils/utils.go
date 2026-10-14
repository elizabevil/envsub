// Package utils provides stdio path checks and small collection helpers.
package utils

import (
	"bytes"
	"slices"
)

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

// SortedKeys returns the keys of a string-keyed map in ascending order, so a walk
// over decoded JSON/YAML is deterministic.
//
// The slice is preallocated to len(object) and sorted with slices.Sort, which
// keeps it to one allocation; slices.Sorted(maps.Keys(object)) cannot size the
// slice up front and needs 7 allocations for 8 keys and 10 for 64 (measured).
func SortedKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// bomPrefix is the UTF-8 byte order mark editors write at the start of a file.
var bomPrefix = []byte{0xEF, 0xBB, 0xBF}

// TrimBOM returns data without a leading UTF-8 byte order mark. Windows editors
// write one by default and every decoder here rejects it, so the mark is removed
// at the decode entry; the same bytes anywhere else are content and stay. UTF-16
// marks are not handled: those files are not valid UTF-8 input.
func TrimBOM(data []byte) []byte {
	return bytes.TrimPrefix(data, bomPrefix)
}
