package phonenumber

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"uradical.io/go/phonenumber/internal/phonetext"
)

// FuzzParse checks that Parse never panics, and that a valid number
// survives a trip through its own text form. An invalid number need not:
// parsing strips what looks like a national prefix even after a plus sign,
// so an invalid number's text can parse to a different invalid number.
func FuzzParse(f *testing.F) {
	for _, s := range []string{
		"020 7946 0958", "+44 (0)20-7946-0958", "tel:+1-650-253-0000;ext=123;isub=1",
		"tel:253-0000;phone-context=+1-650", "1-800-FLOWERS", "011 54 9 11 8765 4321",
		"+800 1234 5678", "0236618300 x 123", "＋４４ 20 7946 0958", "++44", ";phone-context=",
	} {
		f.Add(s, "GB")
		f.Add(s, "")
	}
	f.Fuzz(func(t *testing.T, s, region string) {
		n, err := Parse(s, region)
		if err != nil {
			return
		}
		for _, format := range []Format{E164, International, National, RFC3966} {
			_ = n.Format(format)
		}
		_ = n.FormatFrom("US")
		if !n.Valid() {
			return
		}
		var back Number
		if err := back.UnmarshalText([]byte(n.String())); err != nil {
			t.Fatalf("Parse(%q, %q) = %v, which does not parse back: %v", s, region, n, err)
		}
		if back != n {
			t.Fatalf("Parse(%q, %q) = %#v, but its text %q parses to %#v", s, region, n, n.String(), back)
		}
	})
}

// FuzzFormat checks that any Number formats without panicking, and that a
// valid one parses back from every format.
func FuzzFormat(f *testing.F) {
	f.Add(uint16(44), uint64(2079460958), uint8(0), "")
	f.Add(uint16(39), uint64(236618300), uint8(1), "12")
	f.Add(uint16(1), uint64(6502530000), uint8(0), "999")
	f.Add(uint16(800), uint64(12345678), uint8(0), "")
	f.Add(uint16(0), uint64(0), uint8(17), "x")
	f.Fuzz(func(t *testing.T, cc uint16, national uint64, zeros uint8, ext string) {
		n := Number{countryCode: cc, national: national, zeros: zeros % 18, ext: ext}
		for _, format := range []Format{E164, International, National, RFC3966} {
			_ = n.Format(format)
		}
		_ = n.FormatFrom("GB")
		_, _, _ = n.Region(), n.Type(), n.CheckLength()
		// Some regions write extensions after an ambiguous label such as
		// " x" or " int ", and the parser, like upstream's, reads at most 9
		// digits after one. A longer extension formats but does not parse
		// back.
		if !n.Valid() || !allDigits(ext) || len(ext) > 9 {
			return
		}
		region := n.Region()
		for _, format := range []Format{E164, International, National, RFC3966} {
			if format == E164 && ext != "" {
				continue // E.164 has no extension
			}
			if format == National && region == regionNonGeo {
				continue // a non-geographical number has no region to dial it in
			}
			s := n.Format(format)
			got, err := Parse(s, region)
			if err != nil {
				t.Fatalf("%#v: %v format %q does not parse: %v", n, format, s, err)
			}
			if got != n {
				t.Fatalf("%#v: %v format %q parses to %#v", n, format, s, got)
			}
		}
	})
}

func allDigits(s string) bool {
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// FuzzReplaceJava checks the Java-style replacement against the standard
// library wherever the two agree: a replacement with no '$' or '\'.
func FuzzReplaceJava(f *testing.F) {
	for _, repl := range []string{"$1 $2", "$12", `\$1`, "0$1", "$", `\`, "$9$0", "plain"} {
		f.Add("12345 678", repl)
	}
	re := regexp.MustCompile(`(\d{2})(\d{3})`)
	f.Fuzz(func(t *testing.T, s, repl string) {
		got := replaceJava(re, s, repl, true)
		if strings.ContainsAny(repl, `$\`) {
			return
		}
		if want := re.ReplaceAllLiteralString(s, repl); got != want {
			t.Fatalf("replaceJava(%q, %q) = %q, want %q", s, repl, got, want)
		}
	})
}

// FuzzNormalize checks that normalizing keeps only ASCII digits, and never
// more of them than the input has characters.
func FuzzNormalize(f *testing.F) {
	for _, s := range []string{"1-800-FLOWERS", "(650) 253-0000", "０２٠", "abc", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		for _, got := range []string{normalize(s), phonetext.NormalizeDigits(s, false)} {
			if !allDigits(got) {
				t.Fatalf("normalize(%q) = %q, which is not all ASCII digits", s, got)
			}
			if len(got) > utf8.RuneCountInString(s) {
				t.Fatalf("normalize(%q) = %q, longer than the input", s, got)
			}
		}
	})
}

// FuzzExtractNumber checks that extracting a number and stripping its
// extension return parts of the input, and an extension of digits.
func FuzzExtractNumber(f *testing.F) {
	for _, s := range []string{"Tel: 650 253 0000 ext. 123.", "+44 20/x 12", "1 ,, 23", "x", "0236618300 #12#"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		number := phonetext.ExtractPossibleNumber(s)
		if !strings.Contains(s, number) {
			t.Fatalf("phonetext.ExtractPossibleNumber(%q) = %q, not part of the input", s, number)
		}
		rest, ext := stripExtension(number)
		if !strings.HasPrefix(number, rest) {
			t.Fatalf("stripExtension(%q) = %q, not a prefix of the input", number, rest)
		}
		for _, r := range ext {
			if phonetext.DigitValue(r) < 0 {
				t.Fatalf("stripExtension(%q) extension %q has a non-digit", number, ext)
			}
		}
	})
}
