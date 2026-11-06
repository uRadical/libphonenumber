// Package phonetext holds the text handling shared by the phonenumber
// package and its subpackages: finding the number in a string, recognizing
// a leading plus sign and reading Unicode digits.
package phonetext

import (
	"regexp"
	"strings"
	"sync"
	"unicode"
)

const (
	// PlusChars lists the plus signs a number may start with, for use
	// inside a character class.
	PlusChars = `+\x{FF0B}`
	// Digits matches one Unicode decimal digit.
	Digits = `\p{Nd}`
)

// The expressions compile on first use, so importing a package costs
// nothing until a number is read.
var (
	// PlusCharsPattern matches one or more plus signs at the start of the
	// input.
	PlusCharsPattern  = lazyRegexp(`^[` + PlusChars + `]+`)
	validStartChar    = lazyRegexp(`[` + PlusChars + Digits + `]`)
	unwantedEndChars  = lazyRegexp(`[^\p{N}\p{L}#]+$`)
	secondNumberStart = lazyRegexp(`[\\/] *x`)
)

func lazyRegexp(expr string) func() *regexp.Regexp {
	return sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(expr) })
}

// DigitValue returns the value of a Unicode decimal digit, or -1.
func DigitValue(r rune) int {
	if '0' <= r && r <= '9' {
		return int(r - '0')
	}
	if r < 0x80 || !unicode.Is(unicode.Nd, r) {
		return -1
	}
	// Unicode encodes every set of decimal digits as a run from zero to
	// nine, and sets that sit side by side each start on a zero, so a
	// digit's value is the count of digits before it in its run, modulo 10.
	before := 0
	for unicode.Is(unicode.Nd, r-rune(before)-1) {
		before++
	}
	return before % 10
}

// NormalizeDigits converts every decimal digit in s to ASCII, dropping
// everything else unless keepNonDigits is set.
func NormalizeDigits(s string, keepNonDigits bool) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if d := DigitValue(r); d >= 0 {
			b.WriteByte("0123456789"[d])
		} else if keepNonDigits {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ExtractPossibleNumber trims s to the part that could be a phone number:
// from the first digit or plus sign, without trailing punctuation, and
// before any second number such as the "/x" in "650 253 0000/x123".
func ExtractPossibleNumber(s string) string {
	loc := validStartChar().FindStringIndex(s)
	if loc == nil {
		return ""
	}
	return TrimSecondNumber(TrimUnwantedEnd(s[loc[0]:]))
}

// TrimUnwantedEnd removes trailing characters that cannot end a number:
// anything but a letter, a digit or '#'.
func TrimUnwantedEnd(s string) string {
	if loc := unwantedEndChars().FindStringIndex(s); loc != nil {
		return s[:loc[0]]
	}
	return s
}

// TrimSecondNumber removes a second number that follows the first, as the
// "/x123" in "650 253 0000/x123".
func TrimSecondNumber(s string) string {
	if loc := secondNumberStart().FindStringIndex(s); loc != nil {
		return s[:loc[0]]
	}
	return s
}
