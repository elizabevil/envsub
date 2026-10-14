//go:build !sonic

package jsonx_test

// sonicBackend reports whether the sonic backend is linked. The corpus comparison
// records one accepted difference for it: duplicate object members (03-12 T9).
const sonicBackend = false
