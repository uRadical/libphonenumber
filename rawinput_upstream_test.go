package phonenumber

import (
	"errors"
	"testing"
)

// Tests ported from upstream's PhoneNumberUtilTest.java for parsing with
// raw input and the formats that use it, run against the test metadata.

var (
	aeUAN       = num(971, 600123456)
	coFixedLine = num(57, 6012345678)
	uzFixedLine = num(998, 612201234)
	uzMobile    = num(998, 950123456)
	// usShortByOneNumber is a US number one digit too short.
	usShortByOneNumber = num(1, 650253000)
)

func mustParseRawTest(t *testing.T, s, region string) Parsed {
	t.Helper()
	p, err := testSet().parseAndKeepRawInput(s, region)
	if err != nil {
		t.Fatalf("parseAndKeepRawInput(%q, %q): %v", s, region, err)
	}
	return p
}

func TestParseAndKeepRaw(t *testing.T) {
	for _, tt := range []struct {
		in, region string
		want       Parsed
	}{
		{"800 six-flags", "US", Parsed{Number: alphaNumericNumber, Raw: "800 six-flags", Source: FromDefaultCountry}},
		{"1800 six-flag", "US", Parsed{Number: num(1, 8007493524), Raw: "1800 six-flag", Source: FromNumberWithoutPlusSign}},
		{"+1800 six-flag", "NZ", Parsed{Number: num(1, 8007493524), Raw: "+1800 six-flag", Source: FromNumberWithPlusSign}},
		{"001800 six-flag", "NZ", Parsed{Number: num(1, 8007493524), Raw: "001800 six-flag", Source: FromNumberWithIDD}},
		{"08122123456", "KR", Parsed{Number: num(82, 22123456), Raw: "08122123456", Source: FromDefaultCountry, CarrierCode: "81"}},
	} {
		if got := mustParseRawTest(t, tt.in, tt.region); got != tt.want {
			t.Errorf("parseAndKeepRawInput(%q, %q) = %+v, want %+v", tt.in, tt.region, got, tt.want)
		}
	}
	// An unknown region.
	if _, err := testSet().parseAndKeepRawInput("123 456 7890", "CS"); !errors.Is(err, ErrInvalidCountryCode) {
		t.Errorf("parseAndKeepRawInput with region CS: err = %v, want %v", err, ErrInvalidCountryCode)
	}
}

func TestFormatOutOfCountryKeepingAlphaChars(t *testing.T) {
	ms := testSet()
	check := func(p Parsed, from, want string) {
		t.Helper()
		if got := ms.formatKeepingAlpha(p, from); got != want {
			t.Errorf("formatKeepingAlpha(%+v, %q) = %q, want %q", p, from, got, want)
		}
	}
	alpha := num(1, 8007493524)
	check(Parsed{Number: alpha, Raw: "1800 six-flag"}, "AU", "0011 1 800 SIX-FLAG")
	check(Parsed{Number: alpha, Raw: "1-800-SIX-flag"}, "AU", "0011 1 800-SIX-FLAG")
	check(Parsed{Number: alpha, Raw: "Call us from UK: 00 1 800 SIX-flag"}, "AU", "0011 1 800 SIX-FLAG")
	check(Parsed{Number: alpha, Raw: "800 SIX-flag"}, "AU", "0011 1 800 SIX-FLAG")
	// From within the NANPA.
	check(Parsed{Number: alpha, Raw: "800 SIX-flag"}, "US", "1 800 SIX-FLAG")
	check(Parsed{Number: alpha, Raw: "800 SIX-flag"}, "BS", "1 800 SIX-FLAG")
	// With an extension.
	check(mustParseRawTest(t, "800 SIX-flag ext. 1234", "US"), "AU", "0011 1 800 SIX-FLAG extn. 1234")
	// Without raw input, the number is formatted as by FormatFrom.
	check(Parsed{Number: alpha}, "DE", "00 1 800 749 3524")
	// An Australian number from Australia has its national prefix fixed.
	au := num(61, 827493524)
	check(Parsed{Number: au, Raw: "+61 82749-FLAG"}, "AU", "082749-FLAG")
	check(Parsed{Number: au, Raw: "082749-FLAG"}, "AU", "082749-FLAG")
	// This rule overrides the national prefix.
	check(Parsed{Number: num(61, 18007493524), Raw: "1-800-SIX-flag"}, "AU", "1-800-SIX-FLAG")
	// The metadata is not changed by the call above.
	if got := ms.formatFrom(num(61, 1800749352), "AU"); got != "1800 749 352" {
		t.Errorf("formatFrom(+61 1800749352, AU) = %q, want %q", got, "1800 749 352")
	}
	auAlpha := Parsed{Number: num(61, 1800749352), Raw: "1-800-SIX-flag"}
	// A region with several international prefixes.
	check(auAlpha, "SG", "+61 1-800-SIX-FLAG")
	// An unsupported region.
	check(auAlpha, "AQ", "+61 1-800-SIX-FLAG")
	// An invalid country calling code: the raw input is used as is.
	check(Parsed{Number: Number{national: 18007493524}, Raw: "1-800-SIX-flag"}, "DE", "1-800-SIX-flag")
	// An invalid alpha number, so nothing can be stripped.
	invalid := Parsed{Number: num(1, 80749), Raw: "180-SIX"}
	check(invalid, "DE", "00 1 180-SIX")
	check(invalid, "AQ", "+1 180-SIX")
}

func TestFormatWithCarrierCode(t *testing.T) {
	ms := testSet()
	// Only AR has carrier code rules in the test metadata, and only for
	// some mobile numbers.
	ar := num(54, 92234654321)
	for _, tt := range []struct {
		n             Number
		carrier, want string
	}{
		{ar, "14", "02234 14 65-4321"},
		// No carrier code.
		{ar, "", "02234 65-4321"},
		// The US has no carrier code rules.
		{usNumber, "15", "650 253 0000"},
		// An unknown country calling code gives the national number.
		{unknownCountryCodeNoRawInput, "89", "12345"},
	} {
		if got := ms.formatWithCarrier(tt.n, tt.carrier); got != tt.want {
			t.Errorf("formatWithCarrier(%v, %q) = %q, want %q", tt.n, tt.carrier, got, tt.want)
		}
	}
	if got := ms.format(ar, National); got != "02234 65-4321" {
		t.Errorf("format(ar, National) = %q", got)
	}
	// International formats use no carrier code.
	if got := ms.format(ar, E164); got != "+5492234654321" {
		t.Errorf("format(ar, E164) = %q", got)
	}
}

func TestFormatWithPreferredCarrierCode(t *testing.T) {
	ms := testSet()
	ar := num(54, 91234125678)
	for _, tt := range []struct {
		carrier, fallback, want string
	}{
		// No carrier code in the number.
		{"", "15", "01234 15 12-5678"},
		{"", "", "01234 12-5678"},
		// The number's own carrier code wins.
		{"19", "15", "01234 19 12-5678"},
		{"19", "", "01234 19 12-5678"},
		// Even when it is a space.
		{" ", "15", "01234   12-5678"},
	} {
		p := Parsed{Number: ar, CarrierCode: tt.carrier}
		if got := ms.formatPreferredCarrier(p, tt.fallback); got != tt.want {
			t.Errorf("preferred carrier %q, fallback %q = %q, want %q", tt.carrier, tt.fallback, got, tt.want)
		}
	}
	// The carrier code does not change the plain national format.
	if got := ms.format(ar, National); got != "01234 12-5678" {
		t.Errorf("format(ar, National) = %q", got)
	}
	// The US has no carrier code rules.
	us := Parsed{Number: num(1, 4241231234), CarrierCode: "99"}
	if got := ms.formatPreferredCarrier(us, "15"); got != "424 123 1234" {
		t.Errorf("preferred carrier for US = %q", got)
	}
}

func TestFormatNumberForMobileDialing(t *testing.T) {
	ms := testSet()
	deWithExt := deNumber.withExt("1234")
	usWithExt := usNumber.withExt("1234")
	auEmergency := num(61, 0).withZeros(2)
	for _, tt := range []struct {
		n              Number
		from           string
		withFormatting bool
		want           string
	}{
		// National format in the country, international from outside.
		{coFixedLine, "CO", false, "6012345678"},
		{deNumber, "DE", false, "030123456"},
		{deNumber, "CH", false, "+4930123456"},
		{deWithExt, "DE", false, "030123456"},
		{deWithExt, "CH", false, "+4930123456"},
		// US toll-free numbers cannot be dialled from abroad in the test
		// metadata.
		{usTollFree, "US", true, "800 253 0000"},
		{usTollFree, "CN", true, ""},
		{usNumber, "US", true, "+1 650 253 0000"},
		{usWithExt, "US", true, "+1 650 253 0000"},
		{usTollFree, "US", false, "8002530000"},
		{usTollFree, "CN", false, ""},
		{usNumber, "US", false, "+16502530000"},
		{usWithExt, "US", false, "+16502530000"},
		// An invalid US number, one digit too long.
		{usLongNumber, "US", false, "+165025300001"},
		{usLongNumber, "US", true, "+1 65025300001"},
		// Star numbers, which the test metadata has in JP.
		{jpStarNumber, "JP", false, "*2345"},
		{jpStarNumber, "JP", true, "*2345"},
		{internationalTollFree, "JP", false, "+80012345678"},
		{internationalTollFree, "JP", true, "+800 1234 5678"},
		// UAE numbers starting 600 are dialled without +971 locally.
		{aeUAN, "JP", false, "+971600123456"},
		{aeUAN, "AE", false, "600123456"},
		{mxNumber1, "MX", false, "+523312345678"},
		{mxNumber1, "US", false, "+523312345678"},
		// Uzbek numbers are always international.
		{uzFixedLine, "UZ", false, "+998612201234"},
		{uzMobile, "UZ", false, "+998950123456"},
		{uzMobile, "US", false, "+998950123456"},
		// Non-geographical numbers are always international.
		{internationalTollFree, "US", false, "+80012345678"},
		{internationalTollFree, "001", false, "+80012345678"},
		// A short number dials within its region only.
		{num(49, 123), "DE", false, "123"},
		{num(49, 123), "IT", false, ""},
		// NANPA numbers of regular length are international, short ones
		// national.
		{usNumber, "US", false, "+16502530000"},
		{usNumber, "CA", false, "+16502530000"},
		{usNumber, "BR", false, "+16502530000"},
		{num(1, 911), "US", false, "911"},
		{num(1, 911), "CA", false, ""},
		{num(1, 911), "BR", false, ""},
		// The Australian emergency number 000.
		{auEmergency, "AU", false, "000"},
		{auEmergency, "NZ", false, ""},
	} {
		if got := ms.formatForMobileDialing(Parsed{Number: tt.n}, tt.from, tt.withFormatting); got != tt.want {
			t.Errorf("formatForMobileDialing(%v, %q, %t) = %q, want %q", tt.n, tt.from, tt.withFormatting, got, tt.want)
		}
	}
}

func TestFormatByPattern(t *testing.T) {
	ms := testSet()
	check := func(n Number, f Format, rule FormatRule, want string) {
		t.Helper()
		got, err := ms.formatByPattern(n, f, []FormatRule{rule})
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("formatByPattern(%v, %v, %+v) = %q, want %q", n, f, rule, got, want)
		}
	}
	rule := FormatRule{Pattern: `(\d{3})(\d{3})(\d{4})`, Format: "($1) $2-$3"}
	check(usNumber, National, rule, "(650) 253-0000")
	check(usNumber, International, rule, "+1 (650) 253-0000")
	check(num(1, 6507129823), RFC3966, rule, "tel:+1-650-712-9823")

	// $NP is 1 for the US, and other NANPA regions follow the US rules.
	rule.NationalPrefixFormattingRule = "$NP ($FG)"
	rule.Format = "$1 $2-$3"
	check(bsNumber, National, rule, "1 (242) 365-1234")
	check(bsNumber, International, rule, "+1 242 365-1234")

	rule.Pattern = `(\d{2})(\d{5})(\d{3})`
	rule.Format = "$1-$2 $3"
	check(itNumber, National, rule, "02-36618 300")
	check(itNumber, International, rule, "+39 02-36618 300")

	rule.NationalPrefixFormattingRule = "$NP$FG"
	rule.Pattern = `(\d{2})(\d{4})(\d{4})`
	rule.Format = "$1 $2 $3"
	check(gbNumber, National, rule, "020 7031 3000")
	rule.NationalPrefixFormattingRule = "($NP$FG)"
	check(gbNumber, National, rule, "(020) 7031 3000")
	rule.NationalPrefixFormattingRule = ""
	check(gbNumber, National, rule, "20 7031 3000")
	check(gbNumber, International, rule, "+44 20 7031 3000")
}

func TestFormatInOriginalFormat(t *testing.T) {
	ms := testSet()
	for _, tt := range []struct {
		in, region, want string
	}{
		{"+442087654321", "GB", "+44 20 8765 4321"},
		{"02087654321", "GB", "(020) 8765 4321"},
		{"011442087654321", "US", "011 44 20 8765 4321"},
		{"442087654321", "GB", "44 20 8765 4321"},
		// Invalid numbers with a formatting rule are formatted. The test
		// metadata leaves out area codes starting with 7.
		{"7345678901", "US", "734 567 8901"},
		// The US has no leading zeros, so this number keeps its raw input.
		{"0734567 8901", "US", "0734567 8901"},
		// Valid, but with no formatting rule, so the raw input is kept.
		{"02-4567-8900", "KR", "02-4567-8900"},
		{"01180012345678", "US", "011 800 1234 5678"},
		{"+80012345678", "KR", "+800 1234 5678"},
		// US local numbers have formatting rules.
		{"2530000", "US", "253 0000"},
		{"18003456789", "US", "1 800 345 6789"},
		{"2087654321", "GB", "20 8765 4321"},
		{"013312345678", "MX", "01 33 1234 5678"},
		{"3312345678", "MX", "33 1234 5678"},
		{"0212345678", "IT", "02 1234 5678"},
		{"00777012", "JP", "0077-7012"},
		{"0777012", "JP", "0777012"},
		{"012 3121286979", "BR", "012 3121286979"},
		// The national prefix in the rule is 045, so input with 044 is
		// returned as given.
		{"044(33)1234-5678", "MX", "044(33)1234-5678"},
		{"045(33)1234-5678", "MX", "045 33 1234 5678"},
		// The international prefix in the rule is 0011, so input with
		// 0012 is returned as given.
		{"0012 16502530000", "AU", "0012 16502530000"},
		{"0011 16502530000", "AU", "0011 1 650 253 0000"},
		// A star is neither removed nor added.
		{"*1234", "JP", "*1234"},
		{"1234", "JP", "1234"},
	} {
		p := mustParseRawTest(t, tt.in, tt.region)
		if got := ms.formatInOriginalFormat(p, tt.region); got != tt.want {
			t.Errorf("formatInOriginalFormat(%q from %s) = %q, want %q", tt.in, tt.region, got, tt.want)
		}
	}
	// Without raw input, the national format is used.
	gb := mustParseTest(t, "+442087654321", "GB")
	for i := range 2 {
		// The second call checks that the earlier calls did not change the
		// metadata.
		if got := ms.formatInOriginalFormat(Parsed{Number: gb}, "GB"); got != "(020) 8765 4321" {
			t.Errorf("call %d: formatInOriginalFormat(+44 20 8765 4321) = %q", i, got)
		}
	}
	// An invalid national number without raw input is formatted as the
	// national number.
	if got := ms.formatInOriginalFormat(Parsed{Number: usShortByOneNumber}, "US"); got != "650253000" {
		t.Errorf("formatInOriginalFormat(usShortByOneNumber) = %q", got)
	}
}
