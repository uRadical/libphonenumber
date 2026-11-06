package phonenumber

import (
	"regexp"
	"strings"
	"sync"
	"unicode"

	"uradical.io/go/phonenumber/internal/phonetext"
)

const (
	plusChars = phonetext.PlusChars
	// validPunctuation lists the characters allowed between the digits of a
	// phone number, for use inside a character class.
	validPunctuation = `\-x\x{2010}-\x{2015}\x{2212}\x{30FC}\x{FF0D}-\x{FF0F} \x{00A0}\x{00AD}\x{200B}\x{2060}\x{3000}()\x{FF08}\x{FF09}\x{FF3B}\x{FF3D}.\[\]/~\x{2053}\x{223C}\x{FF5E}`
	digits           = phonetext.Digits
	validAlpha       = "A-Za-z"
)

// The package's own expressions compile on first use, so importing the
// package costs nothing until a number is parsed or formatted.
var (
	separatorPattern  = lazyRegexp(`[` + validPunctuation + `]+`)
	validAlphaPhone   = lazyRegexp(`^(?:.*?[A-Za-z]){3}.*$`)
	capturingDigit    = lazyRegexp(`(` + digits + `)`)
	singleIntlPrefix  = lazyRegexp(`^[\d]+(?:[~\x{2053}\x{223C}\x{FF5E}][\d]+)?$`)
	firstGroupPattern = lazyRegexp(`(\$\d)`)
)

// lazyRegexp returns a function that compiles expr on its first call and
// returns the same *regexp.Regexp on every call.
func lazyRegexp(expr string) func() *regexp.Regexp {
	return sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(expr) })
}

// alphaPhoneMappings maps keypad letters to digits, as on a phone keypad.
var alphaPhoneMappings = func() map[rune]rune {
	m := make(map[rune]rune, 36)
	for i, keys := range []string{"ABC", "DEF", "GHI", "JKL", "MNO", "PQRS", "TUV", "WXYZ"} {
		for _, r := range keys {
			m[r] = rune('2' + i)
		}
	}
	for r := '0'; r <= '9'; r++ {
		m[r] = r
	}
	return m
}()

// normalize reduces a number to its digits. A number with at least three
// letters, such as 1-800-FLOWERS, has its letters converted to keypad
// digits, and only ASCII characters are kept.
func normalize(s string) string {
	if validAlphaPhone().MatchString(s) {
		return mapChars(s, alphaPhoneMappings)
	}
	return phonetext.NormalizeDigits(s, false)
}

// mapChars replaces each character of s found in m, after upper-casing it,
// and drops the rest.
func mapChars(s string, m map[rune]rune) string {
	var b strings.Builder
	for _, r := range s {
		if v, ok := m[unicode.ToUpper(r)]; ok {
			b.WriteRune(v)
		}
	}
	return b.String()
}

// replaceJava replaces the first match of re in s, or every match if all is
// set, expanding repl the way java.util.regex.Matcher does. Metadata
// formats are written for Java: "$12" means group 12 only if the pattern has
// that many groups, and otherwise group 1 followed by "2", while Go's
// Expand would read it as a group named "12".
func replaceJava(re *regexp.Regexp, s, repl string, all bool) string {
	n := 1
	if all {
		n = -1
	}
	matches := re.FindAllStringSubmatchIndex(s, n)
	if matches == nil {
		return s
	}
	var b strings.Builder
	last := 0
	for _, m := range matches {
		b.WriteString(s[last:m[0]])
		expandJava(&b, repl, s, m)
		last = m[1]
	}
	b.WriteString(s[last:])
	return b.String()
}

func expandJava(b *strings.Builder, repl, s string, m []int) {
	groups := len(m)/2 - 1
	for i := 0; i < len(repl); i++ {
		c := repl[i]
		switch {
		case c == '\\' && i+1 < len(repl):
			i++
			b.WriteByte(repl[i])
		case c == '$' && i+1 < len(repl) && '0' <= repl[i+1] && repl[i+1] <= '9':
			i++
			ref := int(repl[i] - '0')
			for i+1 < len(repl) && '0' <= repl[i+1] && repl[i+1] <= '9' {
				next := ref*10 + int(repl[i+1]-'0')
				if next > groups {
					break
				}
				ref = next
				i++
			}
			if ref <= groups && m[2*ref] >= 0 {
				b.WriteString(s[m[2*ref]:m[2*ref+1]])
			}
		default:
			b.WriteByte(c)
		}
	}
}
