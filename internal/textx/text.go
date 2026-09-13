// Package textx holds text helpers shared by provider adapters.
package textx

import "strings"

// Trim trims surrounding space and truncates s to at most n bytes, dropping a
// trailing partial rune so the result is always valid UTF-8.
func Trim(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return strings.ToValidUTF8(s[:n], "")
}
