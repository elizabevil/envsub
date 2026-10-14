//go:build sonic

package encoding_test

// sonicBackend reports whether the sonic JSON backend is linked. The rules in
// 03-12 apply to both backends; a few rows are still pending for sonic and are
// skipped with a pointer to the rule.
const sonicBackend = true
