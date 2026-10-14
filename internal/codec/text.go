package codec

import "strings"

// numberMarks are the characters that make a JSON number fractional or
// exponential instead of a plain integer. Every numeric normalizer uses them to
// decide whether a json.Number still has exact integer digits.
const numberMarks = ".eE"

// IsDigit reports whether r is an ASCII decimal digit. It takes a rune so that
// callers can pass either a byte or a rune without a lossy conversion.
func IsDigit(r rune) bool { return r >= '0' && r <= '9' }

// IsIntegerText reports whether text is spelled as a plain integer, that is
// without a fraction or exponent marker.
func IsIntegerText(text string) bool { return !strings.ContainsAny(text, numberMarks) }
