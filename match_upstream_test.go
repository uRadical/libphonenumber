package phonenumber

import "testing"

// Tests ported from upstream's PhoneNumberUtilTest.java testIsNumberMatch*
// methods, run against the test metadata.

type stringsMatch struct {
	a, b string
	want MatchType
}

func checkMatchStrings(t *testing.T, tests []stringsMatch) {
	t.Helper()
	for _, tt := range tests {
		if got := testSet().matchStrings(tt.a, tt.b); got != tt.want {
			t.Errorf("matchStrings(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

type numberStringMatch struct {
	n    Number
	s    string
	want MatchType
}

func checkMatchString(t *testing.T, tests []numberStringMatch) {
	t.Helper()
	for _, tt := range tests {
		if got := testSet().matchString(tt.n, tt.s); got != tt.want {
			t.Errorf("matchString(%#v, %q) = %v, want %v", tt.n, tt.s, got, tt.want)
		}
	}
}

func checkMatch(t *testing.T, a, b Number, want MatchType) {
	t.Helper()
	if got := a.Match(b); got != want {
		t.Errorf("%#v.Match(%#v) = %v, want %v", a, b, got, want)
	}
}

func TestIsNumberMatchMatches(t *testing.T) {
	checkMatchStrings(t, []stringsMatch{
		{"+64 3 331 6005", "+64 03 331 6005", MatchExact},
		{"+800 1234 5678", "+80012345678", MatchExact},
		{"+64 03 331-6005", "+64 03331 6005", MatchExact},
		{"+643 331-6005", "+64033316005", MatchExact},
		{"+643 331-6005", "+6433316005", MatchExact},
		{"+64 3 331-6005", "+6433316005", MatchExact},
		{"+64 3 331-6005", "tel:+64-3-331-6005;isub=123", MatchExact},
		// Alpha numbers.
		{"+1800 siX-Flags", "+1 800 7493 5247", MatchExact},
		// Extensions.
		{"+64 3 331-6005 extn 1234", "+6433316005#1234", MatchExact},
		{"+64 3 331-6005 ext. 1234", "+6433316005;1234", MatchExact},
		{"+7 423 202-25-11 ext 100", "+7 4232022511 доб. 100", MatchExact},
	})
	nzWithExt := nzNumber.withExt("3456")
	checkMatchString(t, []numberStringMatch{
		{nzNumber, "+6403 331 6005", MatchExact},
		{nzWithExt, "+643 331 6005 ext 3456", MatchExact},
		// An empty extension is no extension.
		{nzWithExt.withExt(""), "+6403 331 6005", MatchExact},
	})
	checkMatch(t, nzWithExt.withExt(""), nzNumber, MatchExact)
}

func TestIsNumberMatchShortMatchIfDiffNumLeadingZeros(t *testing.T) {
	checkMatch(t, nzNumber.withZeros(1), nzNumber.withZeros(2), MatchShortNSN)
	// Without an Italian leading zero, upstream ignores the count of
	// leading zeros; a Number has a count of 0.
	checkMatch(t, nzNumber, nzNumber.withZeros(1), MatchShortNSN)
}

func TestIsNumberMatchAcceptsProtoDefaultsAsMatch(t *testing.T) {
	// Upstream sets number_of_leading_zeros to its default of 1 on one of
	// two numbers with an Italian leading zero. A Number has no separate
	// default, so both hold one zero.
	checkMatch(t, nzNumber.withZeros(1), nzNumber.withZeros(1), MatchExact)
}

func TestIsNumberMatchMatchesDiffLeadingZerosIfItalianLeadingZeroFalse(t *testing.T) {
	// Upstream sets number_of_leading_zeros to 1 and then 10 on a number
	// without an Italian leading zero, which leaves it equal to nzNumber.
	// A Number cannot hold leading zeros without them counting, so this
	// is the same comparison.
	checkMatch(t, nzNumber, nzNumber, MatchExact)
}

// TestIsNumberMatchIgnoresSomeFields has no port: the fields it checks are
// ignored (raw input, country code source and carrier code) are not part
// of a Number.

func TestIsNumberMatchNonMatches(t *testing.T) {
	checkMatchStrings(t, []stringsMatch{
		{"03 331 6005", "03 331 6006", MatchNone},
		{"+800 1234 5678", "+1 800 1234 5678", MatchNone},
		// Different country calling code, partial number match.
		{"+64 3 331-6005", "+16433316005", MatchNone},
		// Different country calling code, same number.
		{"+64 3 331-6005", "+6133316005", MatchNone},
		// Extension different, all else the same.
		{"+64 3 331-6005 extn 1234", "0116433316005#1235", MatchNone},
		{"+64 3 331-6005 extn 1234", "tel:+64-3-331-6005;ext=1235", MatchNone},
		// NSN matches, but the extension is different.
		{"+64 3 331-6005 ext.1235", "3 331 6005#1234", MatchNone},
		// Numbers that cannot be parsed.
		{"4", "3 331 6043", MatchNotANumber},
		{"+43", "+64 3 331 6005", MatchNotANumber},
		{"+43", "64 3 331 6005", MatchNotANumber},
		{"Dog", "64 3 331 6005", MatchNotANumber},
	})
}

func TestIsNumberMatchNsnMatches(t *testing.T) {
	checkMatchStrings(t, []stringsMatch{
		{"+64 3 331-6005", "03 331 6005", MatchNSN},
		{"+64 3 331-6005", "tel:03-331-6005;isub=1234;phone-context=abc.nz", MatchNSN},
		{"+1 650-253 0000", "1 650 253 0000", MatchNSN},
		{"1 650-253 0000", "1 650 253 0000", MatchNSN},
		{"1 650-253 0000", "+1 650 253 0000", MatchNSN},
	})
	checkMatchString(t, []numberStringMatch{
		{nzNumber, "03 331 6005", MatchNSN},
		// The second number may start with New Zealand's country calling
		// code, though that is not certain.
		{nzNumber, "(64-3) 331 6005", MatchNSN},
		// Against a US number, the 1 may be a national prefix.
		{usNumber, "1-650-253-0000", MatchNSN},
		{usNumber, "6502530000", MatchNSN},
		// Against a Swiss number, the 1 is not taken to be a national
		// prefix, so it is not removed.
		{num(41, 6502530000), "1-650-253-0000", MatchShortNSN},
	})
}

func TestIsNumberMatchShortNsnMatches(t *testing.T) {
	checkMatchStrings(t, []stringsMatch{
		// The country is unknown for one or both numbers.
		{"+64 3 331-6005", "331 6005", MatchShortNSN},
		{"+64 3 331-6005", "tel:331-6005;phone-context=abc.nz", MatchShortNSN},
		{"+64 3 331-6005", "tel:331-6005;isub=1234;phone-context=abc.nz", MatchShortNSN},
		{"+64 3 331-6005", "tel:331-6005;isub=1234;phone-context=abc.nz;a=%A1", MatchShortNSN},
		// With no country calling code on either, the 0 is not known to
		// be a national prefix.
		{"3 331-6005", "03 331 6005", MatchShortNSN},
		{"3 331-6005", "331 6005", MatchShortNSN},
		{"3 331-6005", "tel:331-6005;phone-context=abc.nz", MatchShortNSN},
		{"3 331-6005", "+64 331 6005", MatchShortNSN},
		// The country is known.
		{"03 331-6005", "331 6005", MatchShortNSN},
		{"1 234 345 6789", "345 6789", MatchShortNSN},
		{"+1 (234) 345 6789", "345 6789", MatchShortNSN},
		// Country calling code omitted from one, extension from the other.
		{"+64 3 331-6005", "3 331 6005#1234", MatchShortNSN},
	})
	// One has an Italian leading zero and the other does not.
	itOne := num(39, 1234).withZeros(1)
	itTwo := num(39, 1234)
	checkMatch(t, itOne, itTwo, MatchShortNSN)
	// One has an extension and the other has none.
	checkMatch(t, itTwo.withExt("1234"), itTwo, MatchShortNSN)
}
