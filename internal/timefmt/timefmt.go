// Package timefmt is the single source of truth for timestamps the process
// emits: events, text logs and build metadata all use it.
//
// The layout is RFC 3339 in UTC with exactly three fractional digits. A fixed
// width keeps lexicographic ordering equal to chronological ordering, and UTC
// keeps a CI log independent of the machine's time zone. RFC3339Nano trims
// trailing zeros, so it does not have that property.
package timefmt

import "time"

// Layout is the one timestamp layout the process writes.
const Layout = time.RFC3339

// Format renders t in UTC using Layout.
func Format(t time.Time) string {
	return t.UTC().Format(Layout)
}
