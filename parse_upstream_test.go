package phonenumber

import (
	"errors"
	"strings"
	"testing"

	"uradical.io/go/phonenumber/internal/phonetext"
)

// Tests ported from upstream's PhoneNumberUtilTest.java, run against the
// test metadata. Each Go test is named after the upstream method it ports.

type parseCase struct {
	in, region string
	want       Number
}

func runParseCases(t *testing.T, tests []parseCase) {
	t.Helper()
	for _, tt := range tests {
		got, err := testSet().parse(tt.in, tt.region)
		if err != nil {
			t.Errorf("parse(%q, %q) failed: %v, want %#v", tt.in, tt.region, err, tt.want)
			continue
		}
		if got != tt.want {
			t.Errorf("parse(%q, %q) = %#v, want %#v", tt.in, tt.region, got, tt.want)
		}
	}
}

type parseErrorCase struct {
	in, region string
	want       error
}

func runParseErrorCases(t *testing.T, tests []parseErrorCase) {
	t.Helper()
	for _, tt := range tests {
		got, err := testSet().parse(tt.in, tt.region)
		if !errors.Is(err, tt.want) {
			in := tt.in
			if len(in) > 60 {
				in = in[:60] + "..."
			}
			t.Errorf("parse(%q, %q) = %#v, %v, want error %v", in, tt.region, got, err, tt.want)
		}
	}
}

func TestNormaliseRemovePunctuation(t *testing.T) {
	in, want := "034-56&+#2\u00AD34", "03456234"
	if got := normalize(in); got != want {
		t.Errorf("normalize(%q) = %q, want %q", in, got, want)
	}
}

func TestNormaliseReplaceAlphaCharacters(t *testing.T) {
	in, want := "034-I-am-HUNGRY", "034426486479"
	if got := normalize(in); got != want {
		t.Errorf("normalize(%q) = %q, want %q", in, got, want)
	}
}

func TestNormaliseOtherDigits(t *testing.T) {
	tests := []struct{ in, want string }{
		{"\uFF125\u0665", "255"},
		// Eastern-Arabic digits.
		{"\u06F52\u06F0", "520"},
	}
	for _, tt := range tests {
		if got := normalize(tt.in); got != tt.want {
			t.Errorf("normalize(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormaliseStripAlphaCharacters(t *testing.T) {
	in, want := "034-56&+a#234", "03456234"
	if got := phonetext.NormalizeDigits(in, false); got != want {
		t.Errorf("phonetext.NormalizeDigits(%q, false) = %q, want %q", in, got, want)
	}
}

func TestIsViablePhoneNumber(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"1", false},
		// Only one or two digits before strange non-possible punctuation.
		{"1+1+1", false},
		{"80+0", false},
		// Two digits is viable.
		{"00", true},
		{"111", true},
		// Alpha numbers.
		{"0800-4-pizza", true},
		{"0800-4-PIZZA", true},
		// At least three digits must come before any letters.
		{"08-PIZZA", false},
		{"8-PIZZA", false},
		{"12. March", false},
	}
	for _, tt := range tests {
		if got := isViablePhoneNumber(tt.in); got != tt.want {
			t.Errorf("isViablePhoneNumber(%q) = %t, want %t", tt.in, got, tt.want)
		}
	}
}

func TestIsViablePhoneNumberNonAscii(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		// Only one or two digits before possible punctuation followed by
		// more digits.
		{"1\u300034", true},
		{"1\u30003+4", false},
		// Unicode variants of possible starting character and other
		// allowed punctuation and digits.
		{"\uFF081\uFF09\u30003456789", true},
		// A leading plus sign.
		{"+1\uFF09\u30003456789", true},
	}
	for _, tt := range tests {
		if got := isViablePhoneNumber(tt.in); got != tt.want {
			t.Errorf("isViablePhoneNumber(%q) = %t, want %t", tt.in, got, tt.want)
		}
	}
}

func TestExtractPossibleNumber(t *testing.T) {
	tests := []struct{ in, want string }{
		// Leading punctuation and letters are removed; the rest is kept.
		{"Tel:0800-345-600", "0800-345-600"},
		{"Tel:0800 FOR PIZZA", "0800 FOR PIZZA"},
		// The plus sign is kept.
		{"Tel:+800-345-600", "+800-345-600"},
		// Wide digits can start a number.
		{"\uFF10\uFF12\uFF13", "\uFF10\uFF12\uFF13"},
		// Dashes cannot.
		{"Num-\uFF11\uFF12\uFF13", "\uFF11\uFF12\uFF13"},
		// No possible number.
		{"Num-....", ""},
		// Leading brackets are removed; parsing does not use them.
		{"(650) 253-0000", "650) 253-0000"},
		// Trailing characters that are not letters or digits are removed.
		{"(650) 253-0000..- ..", "650) 253-0000"},
		{"(650) 253-0000.", "650) 253-0000"},
		// A trailing right-to-left mark.
		{"(650) 253-0000\u200F", "650) 253-0000"},
	}
	for _, tt := range tests {
		if got := phonetext.ExtractPossibleNumber(tt.in); got != tt.want {
			t.Errorf("phonetext.ExtractPossibleNumber(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestMaybeStripNationalPrefix(t *testing.T) {
	meta := &territory{
		id:                       "ignored",
		nationalPrefixForParsing: &pattern{expr: "34"},
		generalDesc:              numberDesc{pattern: &pattern{expr: `\d{4,8}`}},
	}
	check := func(in, wantNSN, wantCarrier string, wantOK bool) {
		t.Helper()
		nsn, carrier, ok := meta.stripNationalPrefix(in)
		if nsn != wantNSN || carrier != wantCarrier || ok != wantOK {
			prefix := ""
			if meta.nationalPrefixForParsing != nil {
				prefix = meta.nationalPrefixForParsing.expr
			}
			t.Errorf("stripNationalPrefix(%q) with prefix %q, rule %q = %q, %q, %t, want %q, %q, %t",
				in, prefix, meta.nationalPrefixTransformRule,
				nsn, carrier, ok, wantNSN, wantCarrier, wantOK)
		}
	}
	check("34356778", "356778", "", true)
	// The result no longer starts with the national prefix.
	check("356778", "356778", "", false)

	// Some regions have no national prefix.
	meta.nationalPrefixForParsing = nil
	check("356778", "356778", "", false)

	// The result would not match the general pattern, so the prefix is
	// kept.
	meta.nationalPrefixForParsing = &pattern{expr: "3"}
	check("3123", "3123", "", false)

	// A carrier selection code.
	meta.nationalPrefixForParsing = &pattern{expr: "0(81)?"}
	check("08122123456", "22123456", "81", true)

	// A transform rule, using the capturing group.
	meta.nationalPrefixTransformRule = "5$15"
	meta.nationalPrefixForParsing = &pattern{expr: `0(\d{2})`}
	check("031123", "5315123", "", true)
}

func TestMaybeStripInternationalPrefix(t *testing.T) {
	idd := &pattern{expr: "00[39]"}
	tests := []struct {
		in      string
		want    string
		wantSrc CountryCodeSource
	}{
		// The dash is removed by normalization.
		{"0034567700-3898003", "45677003898003", FromNumberWithIDD},
		// Once stripped, there is no prefix left.
		{"45677003898003", "45677003898003", FromDefaultCountry},
		{"00945677003898003", "45677003898003", FromNumberWithIDD},
		// The prefix may be broken up by spaces.
		{"00 9 45677003898003", "45677003898003", FromNumberWithIDD},
		// The plus sign is stripped too.
		{"+45677003898003", "45677003898003", FromNumberWithPlusSign},
		// No country calling code starts with 0, so a prefix followed by
		// one is not stripped.
		{"0090112-3123", "00901123123", FromDefaultCountry},
		// Even when the 0 is separated from the prefix by a space.
		{"009 0-112-3123", "00901123123", FromDefaultCountry},
	}
	for _, tt := range tests {
		got, src := stripInternationalPrefix(tt.in, idd)
		if got != tt.want || src != tt.wantSrc {
			t.Errorf("stripInternationalPrefix(%q, %q) = %q, %v, want %q, %v", tt.in, idd.expr, got, src, tt.want, tt.wantSrc)
		}
	}
}

func TestMaybeExtractCountryCode(t *testing.T) {
	// The US international prefix is 011.
	meta := testSet().region("US")
	tests := []struct {
		in      string
		wantCC  int
		wantNSN string // Checked only when wantCC is not 0.
		wantSrc CountryCodeSource
	}{
		{"011112-3456789", 1, "123456789", FromNumberWithIDD},
		{"+6423456789", 64, "23456789", FromNumberWithPlusSign},
		{"+80012345678", 800, "12345678", FromNumberWithPlusSign},
		// No international prefix.
		{"2345-6789", 0, "", FromDefaultCountry},
		// The country calling code of the region, without a plus sign.
		{"(1 610) 619 4466", 1, "6106194466", FromNumberWithoutPlusSign},
		// The number is invalid once the uncertain country calling code is
		// removed.
		{"(1 610) 619 446", 0, "", FromDefaultCountry},
		// Too short both with and without the uncertain country calling
		// code.
		{"(1 610) 619", 0, "", FromDefaultCountry},
	}
	for _, tt := range tests {
		cc, nsn, src, err := testSet().extractCountryCode(tt.in, meta)
		if err != nil {
			t.Errorf("extractCountryCode(%q, US) failed: %v", tt.in, err)
			continue
		}
		if cc != tt.wantCC || (cc != 0 && nsn != tt.wantNSN) || src != tt.wantSrc {
			t.Errorf("extractCountryCode(%q, US) = %d, %q, %v, want %d, %q, %v", tt.in, cc, nsn, src, tt.wantCC, tt.wantNSN, tt.wantSrc)
		}
	}

	// No valid country calling code follows the prefix.
	in := "0119991123456789"
	if cc, nsn, _, err := testSet().extractCountryCode(in, meta); !errors.Is(err, ErrInvalidCountryCode) {
		t.Errorf("extractCountryCode(%q, US) = %d, %q, %v, want error %v", in, cc, nsn, err, ErrInvalidCountryCode)
	}
}

func TestParseNationalNumber(t *testing.T) {
	runParseCases(t, []parseCase{
		// With the national prefix.
		{"033316005", "NZ", nzNumber},
		{"33316005", "NZ", nzNumber},
		// With the national prefix and some formatting.
		{"03-331 6005", "NZ", nzNumber},
		{"03 331 6005", "NZ", nzNumber},
		// RFC 3966 with a phone context.
		{"tel:03-331-6005;phone-context=+64", "NZ", nzNumber},
		{"tel:331-6005;phone-context=+64-3", "NZ", nzNumber},
		{"tel:331-6005;phone-context=+64-3", "US", nzNumber},
		{"My number is tel:03-331-6005;phone-context=+64", "NZ", nzNumber},
		// Optional user-defined parameters follow the context.
		{"tel:03-331-6005;phone-context=+64;a=%A1", "NZ", nzNumber},
		// An ISDN subaddress.
		{"tel:03-331-6005;isub=12345;phone-context=+64", "NZ", nzNumber},
		{"tel:+64-3-331-6005;isub=12345", "NZ", nzNumber},
		// Without "tel:".
		{"03-331-6005;phone-context=+64", "NZ", nzNumber},
		// International prefixes; the country calling code is stripped.
		{"0064 3 331 6005", "NZ", nzNumber},
		// An international number with a different default region.
		{"01164 3 331 6005", "US", nzNumber},
		{"+64 3 331 6005", "US", nzNumber},
		// The plus sign is ignored when the US international prefix
		// follows it instead of a country calling code.
		{"+01164 3 331 6005", "US", nzNumber},
		{"+0064 3 331 6005", "NZ", nzNumber},
		{"+ 00 64 3 331 6005", "NZ", nzNumber},

		{"tel:253-0000;phone-context=www.google.com", "US", usLocalNumber},
		{"tel:253-0000;isub=12345;phone-context=www.google.com", "US", usLocalNumber},
		{"tel:2530000;isub=12345;phone-context=1234.com", "US", usLocalNumber},

		{"64(0)64123456", "NZ", num(64, 64123456)},
		// A "/" is fine in a number.
		{"301/23456", "DE", deNumber},
		// The 1 is not taken as a country calling code when the number is
		// already possible.
		{"123-456-7890", "US", num(1, 1234567890)},
		// Star numbers are not strictly valid, but formatting produces
		// them, so they should parse.
		{"+81 *2345", "JP", jpStarNumber},
		{"12", "NZ", num(64, 12)},
		// A leading zero is not taken as the national prefix when what
		// remains has only a local-only length, as 6 and 7 digits are in GB.
		{"0123456", "GB", num(44, 123456).withZeros(1)},
	})
}

func TestParseNumberWithAlphaCharacters(t *testing.T) {
	tollFree := num(64, 800332005)
	premium := num(64, 9003326005)
	runParseCases(t, []parseCase{
		{"0800 DDA 005", "NZ", tollFree},
		{"0900 DDA 6005", "NZ", premium},
		// Too few letters to be intentional, so they are stripped.
		{"0900 332 6005a", "NZ", premium},
		{"0900 332 600a5", "NZ", premium},
		{"0900 332 600A5", "NZ", premium},
		{"0900 a332 600A5", "NZ", premium},
	})
}

func TestParseMaliciousInput(t *testing.T) {
	runParseErrorCases(t, []parseErrorCase{
		// Lots of leading plus signs.
		{strings.Repeat("+", 6000) + "12222-33-244 extensioB 343+", "US", ErrTooLong},
		{strings.Repeat("200", 350) + " extensiOB 345", "US", ErrTooLong},
	})
}

func TestParseWithInternationalPrefixes(t *testing.T) {
	runParseCases(t, []parseCase{
		{"+1 (650) 253-0000", "NZ", usNumber},
		{"011 800 1234 5678", "US", internationalTollFree},
		{"1-650-253-0000", "US", usNumber},
		// Calling the US from Singapore through different providers:
		// SingTel (001), StarHub (008) and SingTel V019 (019).
		{"0011-650-253-0000", "SG", usNumber},
		{"0081-650-253-0000", "SG", usNumber},
		{"0191-650-253-0000", "SG", usNumber},
		// Calling the US from Poland.
		{"0~01-650-253-0000", "PL", usNumber},
		// Starting with "++".
		{"++1 (650) 253-0000", "PL", usNumber},
	})
}

func TestParseNonAscii(t *testing.T) {
	runParseCases(t, []parseCase{
		// A full-width plus sign.
		{"\uFF0B1 (650) 253-0000", "SG", usNumber},
		// A soft hyphen, U+00AD.
		{"1 (650) 253\u00AD-0000", "US", usNumber},
		// The whole number, punctuation included, in full-width form.
		{"\uFF0B\uFF11\u3000\uFF08\uFF16\uFF15\uFF10\uFF09\u3000\uFF12\uFF15\uFF13\uFF0D\uFF10\uFF10\uFF10\uFF10", "SG", usNumber},
		// With a U+30FC dash instead.
		{"\uFF0B\uFF11\u3000\uFF08\uFF16\uFF15\uFF10\uFF09\u3000\uFF12\uFF15\uFF13\u30FC\uFF10\uFF10\uFF10\uFF10", "SG", usNumber},
		// Mongolian digits.
		{"\u1811 \u1816\u1815\u1810 \u1812\u1815\u1813 \u1810\u1810\u1810\u1810", "US", usNumber},
	})
}

func TestParseWithLeadingZero(t *testing.T) {
	runParseCases(t, []parseCase{
		{"+39 02-36618 300", "NZ", itNumber},
		{"02-36618 300", "IT", itNumber},
		{"345 678 901", "IT", itMobile},
	})
}

func TestParseNationalNumberArgentina(t *testing.T) {
	runParseCases(t, []parseCase{
		// Mobiles.
		{"+54 9 343 555 1212", "AR", num(54, 93435551212)},
		{"0343 15 555 1212", "AR", num(54, 93435551212)},
		{"+54 9 3715 65 4320", "AR", num(54, 93715654320)},
		{"03715 15 65 4320", "AR", num(54, 93715654320)},
		{"911 876 54321", "AR", arMobile},
		// Fixed lines.
		{"+54 11 8765 4321", "AR", arNumber},
		{"011 8765 4321", "AR", arNumber},
		{"+54 3715 65 4321", "AR", num(54, 3715654321)},
		{"03715 65 4321", "AR", num(54, 3715654321)},
		{"+54 23 1234 0000", "AR", num(54, 2312340000)},
		{"023 1234 0000", "AR", num(54, 2312340000)},
	})
}

func TestParseWithXInNumber(t *testing.T) {
	runParseCases(t, []parseCase{
		// An "x" at the start of the number is removed.
		{"01187654321", "AR", arNumber},
		{"(0) 1187654321", "AR", arNumber},
		{"0 1187654321", "AR", arNumber},
		{"(0xx) 1187654321", "AR", arNumber},
		// More than seven digits follow the "xx", so they are not taken as
		// an extension.
		{"011xx5481429712", "US", num(54, 81429712)},
	})
}

func TestParseNumbersMexico(t *testing.T) {
	runParseCases(t, []parseCase{
		// Fixed lines.
		{"+52 (449)978-0001", "MX", num(52, 4499780001)},
		{"01 (449)978-0001", "MX", num(52, 4499780001)},
		{"(449)978-0001", "MX", num(52, 4499780001)},
		// Mobiles.
		{"+52 1 33 1234-5678", "MX", num(52, 13312345678)},
		{"044 (33) 1234-5678", "MX", num(52, 13312345678)},
		{"045 33 1234-5678", "MX", num(52, 13312345678)},
	})
}

func TestFailedParseOnInvalidNumbers(t *testing.T) {
	runParseErrorCases(t, []parseErrorCase{
		{"This is not a phone number", "NZ", ErrNotANumber},
		{"1 Still not a number", "NZ", ErrNotANumber},
		{"1 MICROSOFT", "NZ", ErrNotANumber},
		{"12 MICROSOFT", "NZ", ErrNotANumber},
		{"01495 72553301873 810104", "GB", ErrTooLong},
		{"+---", "DE", ErrNotANumber},
		{"+***", "DE", ErrNotANumber},
		{"+*******91", "DE", ErrNotANumber},
		{"+49 0", "DE", ErrTooShort},
		{"+210 3456 56789", "NZ", ErrInvalidCountryCode},
		// 00 is the right prefix, but 210 is not a country calling code.
		{"+ 00 210 3 331 6005", "NZ", ErrInvalidCountryCode},
		// Unknown, deprecated and missing default regions.
		{"123 456 7890", "ZZ", ErrInvalidCountryCode},
		{"123 456 7890", "CS", ErrInvalidCountryCode},
		{"123 456 7890", "", ErrInvalidCountryCode},
		// Only an international prefix and a country calling code.
		{"0044------", "GB", ErrTooShortAfterIDD},
		{"0044", "GB", ErrTooShortAfterIDD},
		{"011", "US", ErrTooShortAfterIDD},
		{"0119", "US", ErrTooShortAfterIDD},
		// Empty input; upstream also tries null, which Go cannot pass.
		{"", "ZZ", ErrNotANumber},
		{"", "US", ErrNotANumber},
		{"tel:555-1234;phone-context=www.google.com", "ZZ", ErrInvalidCountryCode},
		// The phone-context lacks a plus sign.
		{"tel:555-1234;phone-context=1-331", "ZZ", ErrNotANumber},
		// The phone-context is empty.
		{";phone-context=", "ZZ", ErrNotANumber},
	})
}

func TestParseNumbersWithPlusWithNoRegion(t *testing.T) {
	// With no default region, the number must start with a plus sign.
	runParseCases(t, []parseCase{
		{"+64 3 331 6005", "ZZ", nzNumber},
		// A full-width plus sign.
		{"\uFF0B64 3 331 6005", "ZZ", nzNumber},
		// Leading characters to strip.
		{"Tel: +64 3 331 6005", "ZZ", nzNumber},
		{"+64 3 331 6005", "", nzNumber},
		{"+800 1234 5678", "", internationalTollFree},
		{"+979 123 456 789", "", universalPremiumRate},
		// RFC 3966 with a phone context.
		{"tel:03-331-6005;phone-context=+64", "ZZ", nzNumber},
		{"  tel:03-331-6005;phone-context=+64", "ZZ", nzNumber},
		{"tel:03-331-6005;isub=12345;phone-context=+64", "ZZ", nzNumber},
	})
}

func TestParseNumberTooShortIfNationalPrefixStripped(t *testing.T) {
	runParseCases(t, []parseCase{
		// Leading digits that look like the national prefix are kept when
		// removing them would leave a number too short to be possible.
		{"8123", "BY", num(375, 8123)},
		{"81234", "BY", num(375, 81234)},
		// The input is a viable six-digit number; stripping would leave
		// five digits.
		{"812345", "BY", num(375, 812345)},
		// Only six-digit numbers are possible, so the prefix is stripped.
		{"8123456", "BY", num(375, 123456)},
	})
}

func TestParseExtensions(t *testing.T) {
	nz := nzNumber.withExt("3456")
	uk := num(44, 2034567890).withExt("456")
	us := num(1, 8009013355).withExt("7246433")
	ru := num(7, 4232022511).withExt("100")
	usTwoExt := num(1, 2121231234).withExt("508")
	usHash := num(1, 6451231234).withExt("910")
	runParseCases(t, []parseCase{
		{"03 331 6005 ext 3456", "NZ", nz},
		{"03-3316005x3456", "NZ", nz},
		{"03-3316005 int.3456", "NZ", nz},
		{"03 3316005 #3456", "NZ", nz},
		// These have no extension.
		{"1800 six-flags", "US", alphaNumericNumber},
		{"1800 SIX FLAGS", "US", alphaNumericNumber},
		{"0~0 1800 7493 5247", "PL", alphaNumericNumber},
		{"(1800) 7493.5247", "US", alphaNumericNumber},
		// The last extension token is the one used.
		{"0~0 1800 7493 5247 ~1234", "PL", alphaNumericNumber.withExt("1234")},
		// A final 0 of the number is kept when an extension follows.
		{"+44 2034567890x456", "NZ", uk},
		{"+44 2034567890x456", "GB", uk},
		{"+44 2034567890 x456", "GB", uk},
		{"+44 2034567890 X456", "GB", uk},
		{"+44 2034567890 X 456", "GB", uk},
		{"+44 2034567890 X  456", "GB", uk},
		{"+44 2034567890 x 456  ", "GB", uk},
		{"+44 2034567890  X 456", "GB", uk},
		{"+44-2034567890;ext=456", "GB", uk},
		{"tel:2034567890;ext=456;phone-context=+44", "ZZ", uk},
		// Full-width "extn", "xtn" and "xt".
		{"+442034567890\uFF45\uFF58\uFF54\uFF4E456", "GB", uk},
		{"+442034567890\uFF58\uFF54\uFF4E456", "GB", uk},
		{"+442034567890\uFF58\uFF54456", "GB", uk},

		{"(800) 901-3355 x 7246433", "US", us},
		{"(800) 901-3355 , ext 7246433", "US", us},
		{"(800) 901-3355 ; 7246433", "US", us},
		// An extension character without surrounding spaces.
		{"(800) 901-3355;7246433", "US", us},
		{"(800) 901-3355 ,extension 7246433", "US", us},
		{"(800) 901-3355 ,extensi\u00F3n 7246433", "US", us},
		// The accented o written with a combining character.
		{"(800) 901-3355 ,extensio\u0301n 7246433", "US", us},
		{"(800) 901-3355 , 7246433", "US", us},
		{"(800) 901-3355 ext: 7246433", "US", us},

		// The Russian extension label (dob), in variants found online.
		{"8 (423) 202-25-11, \u0434\u043E\u0431. 100", "RU", ru},
		{"8 (423) 202-25-11 \u0434\u043E\u0431. 100", "RU", ru},
		{"8 (423) 202-25-11, \u0434\u043E\u0431 100", "RU", ru},
		{"8 (423) 202-25-11 \u0434\u043E\u0431 100", "RU", ru},
		{"8 (423) 202-25-11\u0434\u043E\u0431100", "RU", ru},
		// In upper case.
		{"8 (423) 202-25-11, \u0414\u041E\u0411. 100", "RU", ru},

		// A second extension is ignored.
		{"(212)123-1234 x508/x1234", "US", usTwoExt},
		{"(212)123-1234 x508/ x1234", "US", usTwoExt},
		{`(212)123-1234 x508\x1234`, "US", usTwoExt},

		// The last three digits before the # are an extension.
		{"+1 (645) 123 1234-910#", "US", usHash},
		{"+1 (645) 123 1234 ext. 910#", "US", usHash},
	})
}

func TestParseHandlesLongExtensionsWithExplicitLabels(t *testing.T) {
	const ext20 = "12345678901234567890"
	runParseCases(t, []parseCase{
		// RFC 3966 format.
		{"tel:+6433316005;ext=0", "NZ", nzNumber.withExt("0")},
		{"tel:+6433316005;ext=01234567890123456789", "NZ", nzNumber.withExt("01234567890123456789")},
		// Explicit labels.
		{"03 3316005ext:1", "NZ", nzNumber.withExt("1")},
		{"03 3316005 xtn:" + ext20, "NZ", nzNumber.withExt(ext20)},
		{"03 3316005 extension\t" + ext20, "NZ", nzNumber.withExt(ext20)},
		{"03 3316005 xtensio:" + ext20, "NZ", nzNumber.withExt(ext20)},
		{"03 3316005 xtensi\u00F3n, " + ext20 + "#", "NZ", nzNumber.withExt(ext20)},
		{"03 3316005extension." + ext20, "NZ", nzNumber.withExt(ext20)},
		{"03 3316005 \u0434\u043E\u0431:" + ext20, "NZ", nzNumber.withExt(ext20)},
	})
	// Extensions that are too long.
	runParseErrorCases(t, []parseErrorCase{
		{"tel:+6433316005;ext=012345678901234567890", "NZ", ErrNotANumber},
		{"03 3316005 extension 123456789012345678901", "NZ", ErrTooLong},
	})
}

func TestParseHandlesLongExtensionsWithAutoDiallingLabels(t *testing.T) {
	us := num(1, 2679000000).withExt("123456789012345")
	runParseCases(t, []parseCase{
		{"+12679000000,,123456789012345#", "US", us},
		{"+12679000000;123456789012345#", "US", us},
		{"+442034000000,,123456789#", "GB", num(44, 2034000000).withExt("123456789")},
	})
	// An extension that is too long.
	runParseErrorCases(t, []parseErrorCase{
		{"+12679000000,,1234567890123456#", "US", ErrNotANumber},
	})
}

func TestParseHandlesShortExtensionsWithAmbiguousChar(t *testing.T) {
	nz := nzNumber.withExt("123456789")
	runParseCases(t, []parseCase{
		{"03 3316005 x 123456789", "NZ", nz},
		{"03 3316005 x. 123456789", "NZ", nz},
		{"03 3316005 #123456789#", "NZ", nz},
		{"03 3316005 ~ 123456789", "NZ", nz},
	})
	// An extension that is too long.
	runParseErrorCases(t, []parseErrorCase{
		{"03 3316005 ~ 1234567890", "NZ", ErrTooLong},
	})
}

func TestParseHandlesShortExtensionsWhenNotSureOfLabel(t *testing.T) {
	// With no label, a trailing # marks the extension.
	runParseCases(t, []parseCase{
		{"+1123-456-7890 666666#", "US", num(1, 1234567890).withExt("666666")},
		{"+11234567890-6#", "US", num(1, 1234567890).withExt("6")},
	})
	// An extension that is too long.
	runParseErrorCases(t, []parseErrorCase{
		{"+1123-456-7890 7777777#", "US", ErrNotANumber},
	})
}

func TestParseItalianLeadingZeros(t *testing.T) {
	runParseCases(t, []parseCase{
		{"011", "AU", num(61, 11).withZeros(1)},
		{"001", "AU", num(61, 1).withZeros(2)},
		// "000" has two leading zeros; the last zero is the number.
		{"000", "AU", num(61, 0).withZeros(2)},
		{"0000", "AU", num(61, 0).withZeros(3)},
	})
}

func TestParseWithPhoneContext(t *testing.T) {
	// context    = ";phone-context=" descriptor
	// descriptor = domainname / global-number-digits
	runParseCases(t, []parseCase{
		// Valid global-number-digits.
		{"tel:033316005;phone-context=+64", "ZZ", nzNumber},
		{"tel:033316005;phone-context=+64;{this isn't part of phone-context anymore!}", "ZZ", nzNumber},
		{"tel:033316005;phone-context=+64-3", "ZZ", num(64, 3033316005)},
		{"tel:033316005;phone-context=+(555)", "ZZ", num(55, 5033316005)},
		{"tel:033316005;phone-context=+-1-2.3()", "ZZ", num(1, 23033316005)},
		// Valid domain names.
		{"tel:033316005;phone-context=abc.nz", "NZ", nzNumber},
		{"tel:033316005;phone-context=www.PHONE-numb3r.com", "NZ", nzNumber},
		{"tel:033316005;phone-context=a", "NZ", nzNumber},
		{"tel:033316005;phone-context=3phone.J.", "NZ", nzNumber},
		{"tel:033316005;phone-context=a--z", "NZ", nzNumber},
	})
	// Invalid descriptors.
	var tests []parseErrorCase
	for _, ctx := range []string{"", "+", "64", "++64", "+abc", ".", "3phone", "a-.nz", "a{b}c"} {
		tests = append(tests, parseErrorCase{"tel:033316005;phone-context=" + ctx, "ZZ", ErrNotANumber})
	}
	runParseErrorCases(t, tests)
}
