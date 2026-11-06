package phonenumber

import (
	"strings"
	"testing"
)

// Tests ported from upstream's PhoneNumberMatcherTest.java, run against the
// test metadata and the real alternate formats, as upstream's are.

// findAll returns every number found in text with the test metadata.
func findAll(text, region string, l Leniency) []FoundNumber {
	var found []FoundNumber
	for m := range testSet().findNumbers(text, region, l, -1) {
		found = append(found, m)
	}
	return found
}

// findFirst returns the first number found in text, if any.
func findFirst(text, region string, l Leniency) (FoundNumber, bool) {
	for m := range testSet().findNumbers(text, region, l, -1) {
		return m, true
	}
	return FoundNumber{}, false
}

func TestContainsMoreThanOneSlashInNationalNumber(t *testing.T) {
	for _, tt := range []struct {
		cc        int
		src       CountryCodeSource
		candidate string
		want      bool
	}{
		// A date.
		{1, FromDefaultCountry, "1/05/2013", true},
		// The source says the number starts with a country calling code,
		// but the part before the slash is not it.
		{274, FromNumberWithoutPlusSign, "27/4/2013", true},
		// The first slash follows the country calling code.
		{49, FromNumberWithPlusSign, "49/69/2013", false},
		{49, FromNumberWithoutPlusSign, "+49/69/2013", false},
		{49, FromNumberWithoutPlusSign, "+ 49/69/2013", false},
		{49, FromNumberWithoutPlusSign, "+ 49/69/20/13", true},
		// The first group matches the country calling code, but the
		// number was national.
		{49, FromDefaultCountry, "49/69/2013", true},
	} {
		p := Parsed{Number: Number{countryCode: uint16(tt.cc)}, Source: tt.src}
		if got := moreThanOneSlashInNationalNumber(p, tt.candidate); got != tt.want {
			t.Errorf("moreThanOneSlashInNationalNumber(+%d %v, %q) = %v, want %v", tt.cc, tt.src, tt.candidate, got, tt.want)
		}
	}
}

func TestFindNationalNumber(t *testing.T) {
	for _, tt := range [][2]string{
		{"033316005", "NZ"},
		// "33316005" in NZ is left out: the national prefix is required.
		{"03-331 6005", "NZ"},
		{"03 331 6005", "NZ"},
		// International prefixes.
		{"0064 3 331 6005", "NZ"},
		{"01164 3 331 6005", "US"},
		{"+64 3 331 6005", "US"},
		{"64(0)64123456", "NZ"},
		// A slash is fine. Real Polish numbers do not start with 0.
		{"0123/456789", "PL"},
		{"123-456-7890", "US"},
	} {
		doTestFindInContext(t, tt[0], tt[1])
	}
}

func TestFindWithInternationalPrefixes(t *testing.T) {
	for _, tt := range [][2]string{
		{"+1 (650) 333-6000", "NZ"},
		{"1-650-333-6000", "US"},
		// Calling the US from Singapore through different carriers.
		{"0011-650-333-6000", "SG"},
		{"0081-650-333-6000", "SG"},
		{"0191-650-333-6000", "SG"},
		// From Poland.
		{"0~01-650-333-6000", "PL"},
		{"++1 (650) 333-6000", "PL"},
		// A full-width plus sign.
		{"\uFF0B1 (650) 333-6000", "SG"},
		// The whole number in full-width characters.
		{"\uFF0B\uFF11\u3000\uFF08\uFF16\uFF15\uFF10\uFF09\u3000\uFF13\uFF13\uFF13\uFF0D\uFF16\uFF10\uFF10\uFF10", "SG"},
	} {
		doTestFindInContext(t, tt[0], tt[1])
	}
}

func TestFindWithLeadingZero(t *testing.T) {
	for _, tt := range [][2]string{
		{"+39 02-36618 300", "NZ"},
		{"02-36618 300", "IT"},
		{"312 345 678", "IT"},
	} {
		doTestFindInContext(t, tt[0], tt[1])
	}
}

func TestFindNationalNumberArgentina(t *testing.T) {
	for _, s := range []string{
		// Mobile numbers.
		"+54 9 343 555 1212", "0343 15 555 1212", "+54 9 3715 65 4320", "03715 15 65 4320",
		// Fixed lines.
		"+54 11 3797 0000", "011 3797 0000", "+54 3715 65 4321", "03715 65 4321",
		"+54 23 1234 0000", "023 1234 0000",
	} {
		doTestFindInContext(t, s, "AR")
	}
}

func TestFindWithXInNumber(t *testing.T) {
	doTestFindInContext(t, "(0xx) 123456789", "AR")
	// x as both a carrier code and an extension symbol.
	doTestFindInContext(t, "(0xx) 123456789 x 1234", "AR")
	// More than 7 digits after xx, so they cannot be an extension.
	doTestFindInContext(t, "011xx5481429712", "US")
}

func TestFindNumbersMexico(t *testing.T) {
	for _, s := range []string{
		// Fixed lines.
		"+52 (449)978-0001", "01 (449)978-0001", "(449)978-0001",
		// Mobiles.
		"+52 1 33 1234-5678", "044 (33) 1234-5678", "045 33 1234-5678",
	} {
		doTestFindInContext(t, s, "MX")
	}
}

func TestFindNumbersWithPlusWithNoRegion(t *testing.T) {
	// Without a region, only numbers with a plus sign are found.
	doTestFindInContext(t, "+64 3 331 6005", "ZZ")
	doTestFindInContext(t, "+64 3 331 6005", "")
}

func TestFindExtensions(t *testing.T) {
	for _, tt := range [][2]string{
		{"03 331 6005 ext 3456", "NZ"},
		{"03-3316005x3456", "NZ"},
		{"03-3316005 int.3456", "NZ"},
		{"03 3316005 #3456", "NZ"},
		{"0~0 1800 7493 524", "PL"},
		{"(1800) 7493.524", "US"},
		// The last extension token is used.
		{"0~0 1800 7493 524 ~1234", "PL"},
		// A last digit of 0 is not lost.
		{"+44 2034567890x456", "NZ"},
		{"+44 2034567890x456", "GB"},
		{"+44 2034567890 x456", "GB"},
		{"+44 2034567890 X456", "GB"},
		{"+44 2034567890 X 456", "GB"},
		{"+44 2034567890 X  456", "GB"},
		{"+44 2034567890  X 456", "GB"},
		{"(800) 901-3355 x 7246433", "US"},
		{"(800) 901-3355 , ext 7246433", "US"},
		{"(800) 901-3355 ,extension 7246433", "US"},
		// Unlike parsing, a lone comma does not start an extension here.
		{"(800) 901-3355 ,x 7246433", "US"},
		{"(800) 901-3355 ext: 7246433", "US"},
	} {
		doTestFindInContext(t, tt[0], tt[1])
	}
}

func TestFindInterspersedWithSpace(t *testing.T) {
	doTestFindInContext(t, "0 3   3 3 1   6 0 0 5", "NZ")
}

// Matching starting in the middle of a number.
func TestIntermediateParsePositions(t *testing.T) {
	text := "Call 033316005  or 032316005!"
	//      |    |    |    |    |    |
	//      0    5   10   15   20   25
	for i := 0; i <= 5; i++ {
		assertEqualRange(t, text, i, 5, 14)
	}
	// Seven and eight digits in a row still parse as a number.
	assertEqualRange(t, text, 6, 6, 14)
	assertEqualRange(t, text, 7, 7, 14)
	// Anything shorter is skipped for the second number.
	for i := 8; i <= 19; i++ {
		assertEqualRange(t, text, i, 19, 28)
	}
}

func TestFourMatchesInARow(t *testing.T) {
	numbers := []string{"415-666-7777", "800-443-1223", "212-443-1223", "650-443-1223"}
	text := strings.Join(numbers, " - ")
	assertMatches(t, text, "US", numbers)
}

func TestMatchesFoundWithMultipleSpaces(t *testing.T) {
	numbers := []string{"(415) 666-7777", "(800) 443-1223"}
	assertMatches(t, strings.Join(numbers, " "), "US", numbers)
}

func TestMatchWithSurroundingZipcodes(t *testing.T) {
	assertMatches(t, "My address is CA 34215 - 415-666-7777 is my number.", "US", []string{"415-666-7777"})
	// With spaces in the number.
	assertMatches(t, "My number is (415) 666 7777. 34215 is my zip-code.", "US", []string{"(415) 666 7777"})
}

// assertMatches checks that the first numbers found in text are want, at
// LeniencyValid.
func assertMatches(t *testing.T, text, region string, want []string) {
	t.Helper()
	found := findAll(text, region, LeniencyValid)
	for i, w := range want {
		if i >= len(found) {
			t.Errorf("in %q: found %d numbers, want %q next", text, len(found), w)
			return
		}
		assertMatchProperties(t, found[i], text, w, region)
	}
}

// assertMatchProperties checks that m is number, parsed in region.
func assertMatchProperties(t *testing.T, m FoundNumber, text, number, region string) {
	t.Helper()
	want := mustParseTest(t, number, region)
	if m.Number != want || m.Raw != number {
		t.Errorf("in %q: found %q (%v), want %q (%v)", text, m.Raw, m.Number, number, want)
	}
}

func TestIsLatinLetter(t *testing.T) {
	for r, want := range map[rune]bool{
		'c': true, 'C': true, '\u00C9': true,
		'\u0301': true, // Combining acute accent.
		// Punctuation, digits and white space are not letters.
		':': false, '5': false, '-': false, '.': false, ' ': false,
		'\u6211': false, // Chinese.
		'\u306E': false, // Hiragana.
	} {
		if got := isLatinLetter(r); got != want {
			t.Errorf("isLatinLetter(%U) = %v, want %v", r, got, want)
		}
	}
}

// numberContext is text to put before and after a number.
type numberContext struct{ leading, trailing string }

func TestMatchesWithSurroundingLatinChars(t *testing.T) {
	// Possible but not valid when surrounded by Latin letters.
	findMatchesInContexts(t, []numberContext{
		{"abc", "def"}, {"abc", ""}, {"", "def"},
		{"\u00C9", ""},
		// e with a combining acute accent.
		{"e\u0301", ""},
	}, false, true)
}

func TestMoneyNotSeenAsPhoneNumber(t *testing.T) {
	findMatchesInContexts(t, []numberContext{
		{"$", ""}, {"", "$"},
		{"\u00A3", ""}, // Pound sign.
		{"\u00A5", ""}, // Yen sign.
	}, false, true)
}

func TestPercentageNotSeenAsPhoneNumber(t *testing.T) {
	findMatchesInContexts(t, []numberContext{{"", "%"}}, false, true)
}

func TestPhoneNumberWithLeadingOrTrailingMoneyMatches(t *testing.T) {
	// The space before or after the amount keeps it apart from the number.
	findMatchesInContexts(t, []numberContext{{"$20 ", ""}, {"", " 100$"}}, true, true)
}

func TestMatchesWithSurroundingLatinCharsAndLeadingPunctuation(t *testing.T) {
	// The numbers start with punctuation, so leading letters are fine, but
	// trailing ones are not.
	possibleOnly := []numberContext{{"abc", "def"}, {"", "def"}, {"", "\u00C9"}}
	valid := []numberContext{
		{"abc", ""}, {"\u00C9", ""},
		{"\u00C9", "."},    // Trailing punctuation.
		{"\u00C9", " def"}, // Trailing white space.
	}
	for _, number := range []string{"+14156667777", "(415)6667777"} {
		findMatchesInContextsFor(t, possibleOnly, false, true, "US", number)
		findMatchesInContextsFor(t, valid, true, true, "US", number)
	}
}

func TestMatchesWithSurroundingChineseChars(t *testing.T) {
	findMatchesInContexts(t, []numberContext{
		{"\u6211\u7684\u7535\u8BDD\u53F7\u7801\u662F", ""},
		{"", "\u662F\u6211\u7684\u7535\u8BDD\u53F7\u7801"},
		{"\u8BF7\u62E8\u6253", "\u6211\u5728\u660E\u5929"},
	}, true, true)
}

func TestMatchesWithSurroundingPunctuation(t *testing.T) {
	findMatchesInContexts(t, []numberContext{
		{"My number-", ""},
		{"", ".Nice day."},
		{"Tel:", "."},
		{"Tel: ", " on Saturdays."},
	}, true, true)
}

func TestMatchesMultiplePhoneNumbersSeparatedByPhoneNumberPunctuation(t *testing.T) {
	got := findAll("Call 650-253-4561 -- 455-234-3451", "US", LeniencyValid)
	want := []FoundNumber{
		{Start: 5, End: 17, Raw: "650-253-4561", Number: num(1, 6502534561)},
		{Start: 21, End: 33, Raw: "455-234-3451", Number: num(1, 4552343451)},
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("found %+v, want %+v", got, want)
	}
}

func TestDoesNotMatchMultiplePhoneNumbersSeparatedWithNoWhiteSpace(t *testing.T) {
	if got := findAll("Call 650-253-4561--455-234-3451", "US", LeniencyValid); len(got) != 0 {
		t.Errorf("found %+v, want none", got)
	}
}

// numberTest is text with a number in a region.
type numberTest struct{ raw, region string }

// Number-like text that no leniency accepts.
var impossibleCases = []numberTest{
	{"12345", "US"},
	{"23456789", "US"},
	{"234567890112", "US"},
	{"650+253+1234", "US"},
	{"3/10/1984", "CA"},
	{"03/27/2011", "US"},
	{"31/8/2011", "US"},
	{"1/12/2011", "US"},
	{"10/12/82", "DE"},
	{"650x2531234", "US"},
	{"2012-01-02 08:00", "US"},
	{"2012/01/02 08:00", "US"},
	{"20120102 08:00", "US"},
	{"2014-04-12 04:04 PM", "US"},
	{"2014-04-12 &nbsp;04:04 PM", "US"},
	{"2014-04-12  04:04 PM", "US"},
}

// Number-like text that only LeniencyPossible accepts.
var possibleOnlyCases = []numberTest{
	// No valid US number starts with 7 in the test metadata.
	{"7121115678", "US"},
	// An x that is neither a carrier code nor an extension.
	{"1650 x 253 - 1234", "US"},
	{"650 x 253 - 1234", "US"},
	{"6502531x234", "US"},
	// A national prefix that may not be left out is.
	{"(20) 3346 1234", "GB"},
}

// Number-like text that leniencies up to LeniencyValid accept.
var validCases = []numberTest{
	{"65 02 53 00 00", "US"},
	{"6502 538365", "US"},
	// Two slashes are rejected by the stricter leniencies.
	{"650//253-1234", "US"},
	{"650/253/1234", "US"},
	{"9002309. 158", "US"},
	{"12 7/8 - 14 12/34 - 5", "US"},
	{"12.1 - 23.71 - 23.45", "US"},
	{"800 234 1 111x1111", "US"},
	{"1979-2011 100", "US"},
	// A national number in the wrong format.
	{"+494949-4-94", "DE"},
	{"\uFF14\uFF11\uFF15\uFF16\uFF16\uFF16\uFF16-\uFF17\uFF17\uFF17", "US"},
	// Very strange formatting.
	{"2012-0102 08", "US"},
	{"2012-01-02 08", "US"},
	// A breakdown assistance number formatted unexpectedly.
	{"1800-1-0-10 22", "AU"},
	{"030-3-2 23 12 34", "DE"},
	{"03 0 -3 2 23 12 34", "DE"},
	{"(0)3 0 -3 2 23 12 34", "DE"},
	{"0 3 0 -3 2 23 12 34", "DE"},
	// Fits an alternate format, but not its leading digits.
	{"+52 332 123 23 23", "MX"},
}

// Number-like text that leniencies up to LeniencyStrictGrouping accept.
var strictGroupingCases = []numberTest{
	{"(415) 6667777", "US"},
	{"415-6667777", "US"},
	// The last two groups are written as one block.
	{"0800-2491234", "DE"},
	// Nearly an alternate format, with the last two groups as one.
	{"0900-1 123123", "DE"},
	{"(0)900-1 123123", "DE"},
	{"0 900-1 123123", "DE"},
	// The national destination code is also in the country calling code.
	{"+33 3 34 2312", "FR"},
}

// Number-like text that every leniency accepts.
var exactGroupingCases = []numberTest{
	{"\uFF14\uFF11\uFF15\uFF16\uFF16\uFF16\uFF17\uFF17\uFF17\uFF17", "US"},
	{"\uFF14\uFF11\uFF15-\uFF16\uFF16\uFF16-\uFF17\uFF17\uFF17\uFF17", "US"},
	{"4156667777", "US"},
	{"4156667777 x 123", "US"},
	{"415-666-7777", "US"},
	{"415/666-7777", "US"},
	{"415-666-7777 ext. 503", "US"},
	{"1 415 666 7777 x 123", "US"},
	{"+1 415-666-7777", "US"},
	{"+494949 49", "DE"},
	{"+49-49-34", "DE"},
	{"+49-4931-49", "DE"},
	{"04931-49", "DE"},   // With the national prefix.
	{"+49-494949", "DE"}, // One group with the country calling code.
	{"+49-494949 ext. 49", "DE"},
	{"+49494949 ext. 49", "DE"},
	{"0494949", "DE"},
	{"0494949 ext. 49", "DE"},
	{"01 (33) 3461 2234", "MX"}, // Optional national prefix present.
	{"(33) 3461 2234", "MX"},    // Optional national prefix left out.
	{"1800-10-10 22", "AU"},     // Breakdown assistance number.
	// Matches an alternate format exactly.
	{"0900-1 123 123", "DE"},
	{"(0)900-1 123 123", "DE"},
	{"0 900-1 123 123", "DE"},
	{"+33 3 34 23 12", "FR"},
}

func concatCases(sets ...[]numberTest) []numberTest {
	var all []numberTest
	for _, s := range sets {
		all = append(all, s...)
	}
	return all
}

func TestMatchesWithPossibleLeniency(t *testing.T) {
	doTestNumberMatchesForLeniency(t, concatCases(strictGroupingCases, exactGroupingCases, validCases, possibleOnlyCases), LeniencyPossible)
}

func TestNonMatchesWithPossibleLeniency(t *testing.T) {
	doTestNumberNonMatchesForLeniency(t, impossibleCases, LeniencyPossible)
}

func TestMatchesWithValidLeniency(t *testing.T) {
	doTestNumberMatchesForLeniency(t, concatCases(strictGroupingCases, exactGroupingCases, validCases), LeniencyValid)
}

func TestNonMatchesWithValidLeniency(t *testing.T) {
	doTestNumberNonMatchesForLeniency(t, concatCases(impossibleCases, possibleOnlyCases), LeniencyValid)
}

func TestMatchesWithStrictGroupingLeniency(t *testing.T) {
	doTestNumberMatchesForLeniency(t, concatCases(strictGroupingCases, exactGroupingCases), LeniencyStrictGrouping)
}

func TestNonMatchesWithStrictGroupLeniency(t *testing.T) {
	doTestNumberNonMatchesForLeniency(t, concatCases(impossibleCases, possibleOnlyCases, validCases), LeniencyStrictGrouping)
}

func TestMatchesWithExactGroupingLeniency(t *testing.T) {
	doTestNumberMatchesForLeniency(t, exactGroupingCases, LeniencyExactGrouping)
}

func TestNonMatchesExactGroupLeniency(t *testing.T) {
	doTestNumberNonMatchesForLeniency(t, concatCases(impossibleCases, possibleOnlyCases, validCases, strictGroupingCases), LeniencyExactGrouping)
}

func doTestNumberMatchesForLeniency(t *testing.T, cases []numberTest, l Leniency) {
	t.Helper()
	for _, tt := range cases {
		m, ok := findFirst(tt.raw, tt.region, l)
		switch {
		case !ok:
			t.Errorf("%v: no match in %q (%s)", l, tt.raw, tt.region)
		case m.Raw != tt.raw:
			t.Errorf("%v: found %q in %q (%s)", l, m.Raw, tt.raw, tt.region)
		}
	}
}

func doTestNumberNonMatchesForLeniency(t *testing.T, cases []numberTest, l Leniency) {
	t.Helper()
	for _, tt := range cases {
		if m, ok := findFirst(tt.raw, tt.region, l); ok {
			t.Errorf("%v: found %q in %q (%s), want none", l, m.Raw, tt.raw, tt.region)
		}
	}
}

// findMatchesInContexts checks that 415-666-7777 is found in each context
// at LeniencyValid only if isValid, and at LeniencyPossible only if
// isPossible.
func findMatchesInContexts(t *testing.T, contexts []numberContext, isValid, isPossible bool) {
	t.Helper()
	findMatchesInContextsFor(t, contexts, isValid, isPossible, "US", "415-666-7777")
}

func findMatchesInContextsFor(t *testing.T, contexts []numberContext, isValid, isPossible bool, region, number string) {
	t.Helper()
	for _, c := range []struct {
		l    Leniency
		want bool
	}{{LeniencyValid, isValid}, {LeniencyPossible, isPossible}} {
		if c.want {
			doTestInContext(t, number, region, contexts, c.l)
			continue
		}
		for _, ctx := range contexts {
			text := ctx.leading + number + ctx.trailing
			if m, ok := findFirst(text, region, c.l); ok {
				t.Errorf("%v: found %q in %q, want none", c.l, m.Raw, text)
			}
		}
	}
}

func TestNonMatchingBracketsAreInvalid(t *testing.T) {
	for _, text := range []string{
		// The digits before ", " are a valid US number, but the bracket
		// is not closed.
		"80.585 [79.964, 81.191]",
		// The "]" is removed before parsing, which leaves an open bracket.
		"80.585 [79.964]",
		"80.585 ((79.964)",
		// Too many sets of brackets.
		"(80).(585) (79).(9)64",
	} {
		if got := findAll(text, "US", LeniencyValid); len(got) != 0 {
			t.Errorf("found %+v in %q, want none", got, text)
		}
	}
}

func TestNoMatches(t *testing.T) {
	for _, tt := range []struct{ text, region string }{
		// No region and no international prefix.
		{"Random text body - number is 0331 6005, see you there", ""},
		{"", "US"},
		{"  ", "US"},
		{"Random text body - number is foobar, see you there", "US"},
		// An invalid region and no plus sign.
		{"1 456 764 156", "ZZ"},
		{"", "ZZ"},
	} {
		if got := findAll(tt.text, tt.region, LeniencyValid); len(got) != 0 {
			t.Errorf("found %+v in %q (%s), want none", got, tt.text, tt.region)
		}
	}
}

func TestSequences(t *testing.T) {
	got := findAll("Call 033316005  or 032316005!", "NZ", LeniencyPossible)
	want := []FoundNumber{
		{Start: 5, End: 14, Raw: "033316005", Number: num(64, 33316005)},
		{Start: 19, End: 28, Raw: "032316005", Number: num(64, 32316005)},
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("found %+v, want %+v", got, want)
	}
}

func TestMaxMatches(t *testing.T) {
	// The limit applies to failed candidates only.
	text := strings.Repeat("My info: 415-666-7777,", 100)
	want := mustParseTest(t, "+14156667777", "")
	count := 0
	for m := range testSet().findNumbers(text, "US", LeniencyValid, 10) {
		if m.Number != want {
			t.Fatalf("found %v, want %v", m.Number, want)
		}
		count++
	}
	if count != 100 {
		t.Errorf("found %d numbers, want 100", count)
	}
}

func TestMaxMatchesInvalid(t *testing.T) {
	// Ten invalid numbers use up the limit before the valid ones.
	text := strings.Repeat("My address 949-8945-0", 10) + strings.Repeat("My info: 415-666-7777,", 100)
	for m := range testSet().findNumbers(text, "US", LeniencyValid, 10) {
		t.Fatalf("found %+v, want none", m)
	}
}

func TestMaxMatchesMixed(t *testing.T) {
	// 100 valid numbers within invalid ones: only the first 10 are found.
	text := strings.Repeat("My info: 415-666-7777 123 fake street", 100)
	want := mustParseTest(t, "+14156667777", "")
	count := 0
	for m := range testSet().findNumbers(text, "US", LeniencyValid, 10) {
		if m.Number != want {
			t.Fatalf("found %v, want %v", m.Number, want)
		}
		count++
	}
	if count != 10 {
		t.Errorf("found %d numbers, want 10", count)
	}
}

func TestSingleAndDoubleIteration(t *testing.T) {
	for _, tt := range []struct {
		text string
		want int
	}{
		{"+14156667777", 1},
		// The trailing space is part of upstream's test.
		{"+14156667777 foobar +14156667777 ", 2},
	} {
		if got := len(findAll(tt.text, "ZZ", LeniencyValid)); got != tt.want {
			t.Errorf("found %d numbers in %q, want %d", got, tt.text, tt.want)
		}
	}
}

// Iterator.remove and the iterator contract have no counterpart for an
// iter.Seq, so testRemovalNotSupported and the hasNext checks are not
// ported.

// assertEqualRange checks that the first number found in text[index:] is
// at [start, end) of text.
func assertEqualRange(t *testing.T, text string, index, start, end int) {
	t.Helper()
	sub := text[index:]
	m, ok := findFirst(sub, "NZ", LeniencyPossible)
	if !ok {
		t.Fatalf("no match in %q", sub)
	}
	if m.Start != start-index || m.End != end-index || sub[m.Start:m.End] != m.Raw {
		t.Errorf("in %q: found [%d, %d) %q, want [%d, %d)", sub, m.Start, m.End, m.Raw, start-index, end-index)
	}
}

// doTestFindInContext checks that number is found in text around it.
func doTestFindInContext(t *testing.T, number, region string) {
	t.Helper()
	findPossibleInContext(t, number, region)
	if n := mustParseTest(t, number, region); testSet().valid(n) {
		findValidInContext(t, number, region)
	}
}

// findPossibleInContext checks contexts in which LeniencyPossible finds a
// valid number.
func findPossibleInContext(t *testing.T, number, region string) {
	t.Helper()
	doTestInContext(t, number, region, []numberContext{
		{"", ""},
		{"   ", "\t"},
		{"Hello ", ""},
		{"", " to call me!"},
		{"Hi there, call ", " to reach me!"},
		{"Hi there, call ", ", or don't"},
		// No white space around the number.
		{"Hi call", ""},
		{"", "forme"},
		{"Hi call", "forme"},
		// Other small numbers.
		{"It's cheap! Call ", " before 6:30"},
		// A second number after.
		{"Call ", " or +1800-123-4567!"},
		// A month and day.
		{"Call me on June 2 at", ""},
		// Publication pages.
		{"As quoted by Alfonso 12-15 (2009), you may call me at ", ""},
		{"As quoted by Alfonso et al. 12-15 (2009), you may call me at ", ""},
		// An American date.
		{"As I said on 03/10/2011, you may call me at ", ""},
		// The 45 is not an extension.
		{"", ", 45 days a year"},
		// A semicolon with an extension symbol does not start an
		// extension here.
		{"", ";x 7246433"},
		// What looks like the start of another number is left out.
		{"Call ", "/x12 more"},
	}, LeniencyPossible)
}

// findValidInContext checks contexts in which LeniencyValid, but not
// LeniencyPossible, finds a valid number.
func findValidInContext(t *testing.T, number, region string) {
	t.Helper()
	doTestInContext(t, number, region, []numberContext{
		{"It's only 9.99! Call ", " to buy"},
		{"Call me on 21.6.1984 at ", ""},
		{"Call me on 06/21 at ", ""},
		{"Call me on 21.6. at ", ""},
		{"Call me on 06/21/84 at ", ""},
	}, LeniencyValid)
}

func doTestInContext(t *testing.T, number, region string, contexts []numberContext, l Leniency) {
	t.Helper()
	for _, ctx := range contexts {
		text := ctx.leading + number + ctx.trailing
		start, end := len(ctx.leading), len(ctx.leading)+len(number)
		m, ok := findFirst(text, region, l)
		if !ok {
			t.Errorf("%v: no number in %q, want %q", l, text, number)
			continue
		}
		if m.Start != start || m.End != end || m.Raw != number {
			t.Errorf("%v: found %q at [%d, %d) in %q, want [%d, %d)", l, m.Raw, m.Start, m.End, text, start, end)
		}
		ensureTermination(text, region, l)
	}
}

// ensureTermination searches text from every index, which must end.
func ensureTermination(text, region string, l Leniency) {
	for i := range len(text) + 1 {
		for range testSet().findNumbers(text[i:], region, l, -1) {
		}
	}
}
