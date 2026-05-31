// Package text provides whitespace normalization for extracted HTML text.
package text

import "strings"

// Normalize collapses all runs of Unicode whitespace (including the
// non-breaking space U+00A0) to a single ASCII space and trims the ends.
func Normalize(s string) string {
	s = strings.ReplaceAll(s, " ", " ")
	return strings.Join(strings.Fields(s), " ")
}
