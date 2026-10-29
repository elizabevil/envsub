// Package timefmt is the single source of truth for timestamps the process
// emits: events, text logs and build metadata all use it.
//
// The layout is RFC 3339 in UTC at second precision. Seconds are sufficient
// because event order is defined by the event sequence number, not by the
// timestamp; UTC keeps a CI log independent of the machine's time zone.
// RFC3339Nano is not used because it trims trailing zeros and therefore has no
// fixed width.
package timefmt

import "time"

// Layout is the one timestamp layout the process writes.
const Layout = time.RFC3339

// Format renders t in UTC using Layout.
func Format(t time.Time) string {
	return t.UTC().Format(Layout)
}
