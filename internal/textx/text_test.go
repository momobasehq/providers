package textx

import (
	"testing"
	"unicode/utf8"
)

func TestTrim(t *testing.T) {
	if got := Trim("  hello  ", 10); got != "hello" {
		t.Fatalf("got %q", got)
	}
	if got := Trim("abcdef", 3); got != "abc" {
		t.Fatalf("got %q", got)
	}
	// "é" is two bytes: cutting at 2 must drop the partial rune, not emit it.
	got := Trim("aé", 2)
	if got != "a" || !utf8.ValidString(got) {
		t.Fatalf("got %q, want valid UTF-8 %q", got, "a")
	}
}
