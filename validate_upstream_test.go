package phonenumber

import (
	"slices"
	"strings"
	"testing"
)

// Tests ported from upstream's PhoneNumberUtilTest.java, run against the
// test metadata. Each Go test is named after the upstream method it ports.

func TestGetNationalSignificantNumber(t *testing.T) {
	tests := []struct {
		n    Number
		want string
	}{
		{usNumber, "6502530000"},
		{itMobile, "345678901"},
		{itNumber, "0236618300"},
		{internationalTollFree, "12345678"},
	}
	for _, tt := range tests {
		if got := tt.n.NationalNumber(); got != tt.want {
			t.Errorf("%#v.NationalNumber() = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestGetNationalSignificantNumber_ManyLeadingZeros(t *testing.T) {
	n := num(1, 650).withZeros(2)
	if got, want := n.NationalNumber(), "00650"; got != want {
		t.Errorf("%#v.NationalNumber() = %q, want %q", n, got, want)
	}
	// Upstream's second case sets a negative number of leading zeros,
	// which Number cannot hold.
}

func TestGetExampleNumber(t *testing.T) {
	tests := []struct {
		region string
		typ    Type
		want   Number
		ok     bool
	}{
		{"DE", FixedLine, deNumber, true},
		// FixedLineOrMobile gives the same as FixedLine.
		{"DE", FixedLineOrMobile, deNumber, true},
		// The US has no voicemail numbers.
		{"US", Voicemail, Number{}, false},
		// CS is not a valid region.
		{"CS", Mobile, Number{}, false},
		// 001 is reserved for non-geographical codes, which have their own
		// function.
		{regionNonGeo, FixedLine, Number{}, false},
		{regionNonGeo, TollFree, Number{}, false},
		// Upstream's UNKNOWN reads the general description, which never has
		// an example.
		{"DE", TypeUnknown, Number{}, false},
	}
	for _, tt := range tests {
		got, ok := testSet().exampleNumber(tt.region, tt.typ)
		if got != tt.want || ok != tt.ok {
			t.Errorf("exampleNumber(%q, %v) = %#v, %t, want %#v, %t", tt.region, tt.typ, got, ok, tt.want, tt.ok)
		}
	}
	for _, typ := range []Type{FixedLine, Mobile} {
		if _, ok := testSet().exampleNumber("US", typ); !ok {
			t.Errorf("exampleNumber(%q, %v) reported false, want true", "US", typ)
		}
	}
}

func TestGetExampleNumberForNonGeoEntity(t *testing.T) {
	tests := []struct {
		cc   int
		want Number
	}{
		{800, internationalTollFree},
		{979, universalPremiumRate},
	}
	for _, tt := range tests {
		got, ok := testSet().exampleNonGeo(tt.cc)
		if got != tt.want || !ok {
			t.Errorf("exampleNonGeo(%d) = %#v, %t, want %#v, true", tt.cc, got, ok, tt.want)
		}
	}
	// 44 is geographical; 999 is unassigned.
	for _, cc := range []int{44, 999} {
		if got, ok := testSet().exampleNonGeo(cc); ok {
			t.Errorf("exampleNonGeo(%d) = %#v, true, want false", cc, got)
		}
	}
}

func TestGetExampleNumberWithoutRegion(t *testing.T) {
	// The test metadata does not cover every type; the real metadata does.
	for _, typ := range []Type{FixedLine, Mobile, PremiumRate} {
		if _, ok := testSet().exampleForType(typ); !ok {
			t.Errorf("exampleForType(%v) reported false, want true", typ)
		}
	}
}

func TestIsPremiumRate(t *testing.T) {
	testNumberTypes(t, PremiumRate, []Number{
		usPremium,
		num(39, 892123),
		num(44, 9187654321),
		num(49, 9001654321),
		num(49, 90091234567),
		universalPremiumRate,
	})
}

func TestIsTollFree(t *testing.T) {
	testNumberTypes(t, TollFree, []Number{
		num(1, 8881234567),
		num(39, 803123),
		num(44, 8012345678),
		num(49, 8001234567),
		internationalTollFree,
	})
}

func TestIsMobile(t *testing.T) {
	testNumberTypes(t, Mobile, []Number{
		bsMobile,
		gbMobile,
		itMobile,
		arMobile,
		num(49, 15123456789),
	})
}

func TestIsFixedLine(t *testing.T) {
	testNumberTypes(t, FixedLine, []Number{bsNumber, itNumber, gbNumber, deNumber})
}

func TestIsFixedLineAndMobile(t *testing.T) {
	testNumberTypes(t, FixedLineOrMobile, []Number{usNumber, num(54, 1987654321)})
}

func TestIsSharedCost(t *testing.T) {
	testNumberTypes(t, SharedCost, []Number{num(44, 8431231234)})
}

func TestIsVoip(t *testing.T) {
	testNumberTypes(t, VoIP, []Number{num(44, 5631231234)})
}

func TestIsPersonalNumber(t *testing.T) {
	testNumberTypes(t, PersonalNumber, []Number{num(44, 7031231234)})
}

func TestIsUnknown(t *testing.T) {
	// Invalid numbers have type TypeUnknown.
	testNumberTypes(t, TypeUnknown, []Number{usLocalNumber})
}

func testNumberTypes(t *testing.T, want Type, numbers []Number) {
	t.Helper()
	for _, n := range numbers {
		if got := testSet().numberType(n); got != want {
			t.Errorf("numberType(%#v) = %v, want %v", n, got, want)
		}
	}
}

func TestIsValidNumber(t *testing.T) {
	for _, n := range []Number{
		usNumber,
		itNumber,
		gbMobile,
		internationalTollFree,
		universalPremiumRate,
		num(64, 21387835),
	} {
		if !testSet().valid(n) {
			t.Errorf("valid(%#v) = false, want true", n)
		}
	}
}

func TestIsValidForRegion(t *testing.T) {
	// Valid for the Bahamas, but not for the US.
	if !testSet().valid(bsNumber) {
		t.Errorf("valid(%#v) = false, want true", bsNumber)
	}
	// No longer valid.
	if n := num(1, 2421232345); testSet().valid(n) {
		t.Errorf("valid(%#v) = true, want false", n)
	}

	tests := []struct {
		n      Number
		region string
		want   bool
	}{
		{bsNumber, "BS", true},
		{bsNumber, "US", false},
		// Mayotte and Réunion are told apart by their leading digits.
		{num(262, 262123456), "RE", true},
		{num(262, 262123456), "YT", false},
		{num(262, 269601234), "YT", true},
		{num(262, 269601234), "RE", false},
		// No longer valid for Réunion, nor for Mayotte.
		{num(262, 269123456), "YT", false},
		{num(262, 269123456), "RE", false},
		// Valid in both.
		{num(262, 800123456), "YT", true},
		{num(262, 800123456), "RE", true},
		{internationalTollFree, "001", true},
		{internationalTollFree, "US", false},
		{internationalTollFree, "ZZ", false},
		// Invalid country calling codes.
		{num(3923, 2366), "ZZ", false},
		{num(3923, 2366), "001", false},
		{num(0, 2366), "001", false},
		{num(0, 2366), "ZZ", false},
	}
	for _, tt := range tests {
		if got := testSet().validFor(tt.n, tt.region); got != tt.want {
			t.Errorf("validFor(%#v, %q) = %t, want %t", tt.n, tt.region, got, tt.want)
		}
	}

	re := num(262, 269123456)
	if testSet().valid(re) {
		t.Errorf("valid(%#v) = true, want false", re)
	}
	if !testSet().valid(num(262, 262123456)) {
		t.Errorf("valid(%#v) = false, want true", num(262, 262123456))
	}
	// Though invalid, it is recognized as from Mayotte by its leading
	// digits.
	if got, want := testSet().regionFor(re), "YT"; got != want {
		t.Errorf("regionFor(%#v) = %q, want %q", re, got, want)
	}
}

func TestIsNotValidNumber(t *testing.T) {
	for _, n := range []Number{
		usLocalNumber,
		num(39, 23661830000).withZeros(1),
		num(44, 791234567),
		num(49, 1234),
		num(64, 3316005),
		// Invalid country calling codes.
		num(3923, 2366),
		num(0, 2366),
		internationalTollFreeTooLong,
	} {
		if testSet().valid(n) {
			t.Errorf("valid(%#v) = true, want false", n)
		}
	}
}

func TestGetRegionCodeForCountryCode(t *testing.T) {
	tests := []struct {
		cc   int
		want string
	}{
		{1, "US"},
		{44, "GB"},
		{49, "DE"},
		{800, "001"},
		{979, "001"},
	}
	for _, tt := range tests {
		if got := testSet().mainRegion(tt.cc); got != tt.want {
			t.Errorf("mainRegion(%d) = %q, want %q", tt.cc, got, tt.want)
		}
	}
}

func TestGetRegionCodeForNumber(t *testing.T) {
	tests := []struct {
		n    Number
		want string
	}{
		{bsNumber, "BS"},
		{usNumber, "US"},
		{gbMobile, "GB"},
		{internationalTollFree, "001"},
		{universalPremiumRate, "001"},
	}
	for _, tt := range tests {
		if got := testSet().regionFor(tt.n); got != tt.want {
			t.Errorf("regionFor(%#v) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestGetRegionCodesForCountryCode(t *testing.T) {
	tests := []struct {
		cc     int
		region string
	}{
		{1, "US"},
		{1, "BS"},
		{44, "GB"},
		{49, "DE"},
		{800, "001"},
	}
	for _, tt := range tests {
		if got := testSet().regionsForCode[tt.cc]; !slices.Contains(got, tt.region) {
			t.Errorf("regionsForCode[%d] = %q, want it to contain %q", tt.cc, got, tt.region)
		}
	}
	// An invalid country calling code.
	if got := testSet().regionsForCode[-1]; len(got) != 0 {
		t.Errorf("regionsForCode[-1] = %q, want empty", got)
	}
}

func TestGetCountryCodeForRegion(t *testing.T) {
	tests := []struct {
		region string
		want   int
	}{
		{"US", 1},
		{"NZ", 64},
		{"", 0}, // Upstream's null.
		{"ZZ", 0},
		{"001", 0},
		// CS is deprecated, so not supported.
		{"CS", 0},
	}
	for _, tt := range tests {
		got := 0
		if meta := testSet().region(tt.region); meta != nil {
			got = meta.countryCode
		}
		if got != tt.want {
			t.Errorf("country code for region %q = %d, want %d", tt.region, got, tt.want)
		}
	}
}

func TestGetNationalDiallingPrefixForRegion(t *testing.T) {
	// nddPrefix mirrors upstream's getNddPrefixForRegion, reporting false
	// where upstream returns null.
	nddPrefix := func(region string, stripNonDigits bool) (string, bool) {
		meta := testSet().region(region)
		if meta == nil {
			return "", false
		}
		if stripNonDigits {
			return strings.ReplaceAll(meta.nationalPrefix, "~", ""), true
		}
		return meta.nationalPrefix, true
	}
	tests := []struct {
		region         string
		stripNonDigits bool
		want           string
		ok             bool
	}{
		{"US", false, "1", true},
		// A region that is not the main one for its code.
		{"BS", false, "1", true},
		{"NZ", false, "0", true},
		// A national prefix with a non-digit.
		{"AO", false, "0~0", true},
		{"AO", true, "00", true},
		// Invalid regions.
		{"", false, "", false}, // Upstream's null.
		{"ZZ", false, "", false},
		{"001", false, "", false},
		// CS is deprecated, so not supported.
		{"CS", false, "", false},
	}
	for _, tt := range tests {
		got, ok := nddPrefix(tt.region, tt.stripNonDigits)
		if got != tt.want || ok != tt.ok {
			t.Errorf("nddPrefix(%q, %t) = %q, %t, want %q, %t", tt.region, tt.stripNonDigits, got, ok, tt.want, tt.ok)
		}
	}
}

func TestIsNANPACountry(t *testing.T) {
	tests := []struct {
		region string
		want   bool
	}{
		{"US", true},
		{"BS", true},
		{"DE", false},
		{"ZZ", false},
		{"001", false},
		{"", false}, // Upstream's null.
	}
	for _, tt := range tests {
		if got := testSet().isNANPA(tt.region); got != tt.want {
			t.Errorf("isNANPA(%q) = %t, want %t", tt.region, got, tt.want)
		}
	}
}

// possible mirrors upstream's isPossibleNumberForType.
func possible(n Number, typ Type) bool {
	c := testSet().checkLength(n, typ)
	return c == LengthOK || c == LengthLocalOnly
}

// possibleString mirrors upstream's isPossibleNumber(String, String): the
// input must parse and the result be possible.
func possibleString(s, region string) bool {
	n, err := testSet().parse(s, region)
	return err == nil && possible(n, TypeUnknown)
}

type possibleCase struct {
	n    Number
	typ  Type
	want bool
}

func runPossibleCases(t *testing.T, tests []possibleCase) {
	t.Helper()
	for _, tt := range tests {
		if got := possible(tt.n, tt.typ); got != tt.want {
			t.Errorf("possible(%#v, %v) = %t, want %t (checkLength = %v)",
				tt.n, tt.typ, got, tt.want, testSet().checkLength(tt.n, tt.typ))
		}
	}
}

type lengthCase struct {
	n    Number
	typ  Type
	want LengthCheck
}

func runLengthCases(t *testing.T, tests []lengthCase) {
	t.Helper()
	for _, tt := range tests {
		if got := testSet().checkLength(tt.n, tt.typ); got != tt.want {
			t.Errorf("checkLength(%#v, %v) = %v, want %v", tt.n, tt.typ, got, tt.want)
		}
	}
}

func TestIsPossibleNumber(t *testing.T) {
	runPossibleCases(t, []possibleCase{
		{usNumber, TypeUnknown, true},
		{usLocalNumber, TypeUnknown, true},
		{gbNumber, TypeUnknown, true},
		{internationalTollFree, TypeUnknown, true},
	})
	tests := []struct{ in, region string }{
		{"+1 650 253 0000", "US"},
		{"+1 650 GOO OGLE", "US"},
		{"(650) 253-0000", "US"},
		{"253-0000", "US"},
		{"+1 650 253 0000", "GB"},
		{"+44 20 7031 3000", "GB"},
		{"(020) 7031 300", "GB"},
		{"7031 3000", "GB"},
		{"3331 6005", "NZ"},
		{"+800 1234 5678", "001"},
	}
	for _, tt := range tests {
		if !possibleString(tt.in, tt.region) {
			t.Errorf("possibleString(%q, %q) = false, want true", tt.in, tt.region)
		}
	}
}

func TestIsPossibleNumberForType_DifferentTypeLengths(t *testing.T) {
	// Argentina has different possible lengths for different types.
	runPossibleCases(t, []possibleCase{
		// Too short for any Argentinian number.
		{num(54, 12345), FixedLine, false},
		{num(54, 12345), TypeUnknown, false},
		// Six digits are possible for fixed lines, but not for mobiles or
		// toll-free numbers.
		{num(54, 123456), TypeUnknown, true},
		{num(54, 123456), FixedLine, true},
		{num(54, 123456), Mobile, false},
		{num(54, 123456), TollFree, false},
		// So are nine.
		{num(54, 123456789), TypeUnknown, true},
		{num(54, 123456789), FixedLine, true},
		{num(54, 123456789), Mobile, false},
		{num(54, 123456789), TollFree, false},
		// Ten digits are possible for every type.
		{num(54, 1234567890), TypeUnknown, true},
		{num(54, 1234567890), FixedLine, true},
		{num(54, 1234567890), Mobile, true},
		{num(54, 1234567890), TollFree, true},
		// Eleven digits are possible only for mobiles.
		{num(54, 12345678901), TypeUnknown, true},
		{num(54, 12345678901), FixedLine, false},
		{num(54, 12345678901), Mobile, true},
		{num(54, 12345678901), TollFree, false},
	})
}

func TestIsPossibleNumberForType_LocalOnly(t *testing.T) {
	runPossibleCases(t, []possibleCase{
		// A local-only length.
		{num(49, 12), TypeUnknown, true},
		{num(49, 12), FixedLine, true},
		// Mobiles have ten or eleven digits and no local-only lengths.
		{num(49, 12), Mobile, false},
	})
}

func TestIsPossibleNumberForType_DataMissingForSizeReasons(t *testing.T) {
	// The fixed-line lengths are the same as the general ones, so the
	// metadata omits them.
	runPossibleCases(t, []possibleCase{
		// A local-only number.
		{num(55, 12345678), TypeUnknown, true},
		{num(55, 12345678), FixedLine, true},
		{num(55, 1234567890), TypeUnknown, true},
		{num(55, 1234567890), FixedLine, true},
	})
}

func TestIsPossibleNumberForType_NumberTypeNotSupportedForRegion(t *testing.T) {
	runPossibleCases(t, []possibleCase{
		// BR has no mobile numbers at all.
		{num(55, 12345678), Mobile, false},
		// But this is a fixed-line length.
		{num(55, 12345678), FixedLine, true},
		{num(55, 12345678), FixedLineOrMobile, true},
		// 979 has no fixed-line or mobile numbers at all.
		{num(979, 123456789), Mobile, false},
		{num(979, 123456789), FixedLine, false},
		{num(979, 123456789), FixedLineOrMobile, false},
		{num(979, 123456789), PremiumRate, true},
	})
}

func TestIsPossibleNumberWithReason(t *testing.T) {
	runLengthCases(t, []lengthCase{
		// National numbers for +1 of 7 to 10 digits are possible.
		{usNumber, TypeUnknown, LengthOK},
		{usLocalNumber, TypeUnknown, LengthLocalOnly},
		{usLongNumber, TypeUnknown, LengthTooLong},
		{num(0, 2530000), TypeUnknown, LengthInvalidCountryCode},
		{num(1, 253000), TypeUnknown, LengthTooShort},
		{num(65, 1234567890), TypeUnknown, LengthOK},
		{internationalTollFreeTooLong, TypeUnknown, LengthTooLong},
	})
}

func TestIsPossibleNumberForTypeWithReason_DifferentTypeLengths(t *testing.T) {
	// Argentina has different possible lengths for different types.
	runLengthCases(t, []lengthCase{
		// Too short for any Argentinian number.
		{num(54, 12345), TypeUnknown, LengthTooShort},
		{num(54, 12345), FixedLine, LengthTooShort},
		// Six digits are possible for fixed lines, but not for mobiles or
		// toll-free numbers.
		{num(54, 123456), TypeUnknown, LengthOK},
		{num(54, 123456), FixedLine, LengthOK},
		{num(54, 123456), Mobile, LengthTooShort},
		{num(54, 123456), TollFree, LengthTooShort},
		// So are nine.
		{num(54, 123456789), TypeUnknown, LengthOK},
		{num(54, 123456789), FixedLine, LengthOK},
		{num(54, 123456789), Mobile, LengthTooShort},
		{num(54, 123456789), TollFree, LengthTooShort},
		// Ten digits are possible for every type.
		{num(54, 1234567890), TypeUnknown, LengthOK},
		{num(54, 1234567890), FixedLine, LengthOK},
		{num(54, 1234567890), Mobile, LengthOK},
		{num(54, 1234567890), TollFree, LengthOK},
		// Eleven digits are possible only for mobiles.
		{num(54, 12345678901), TypeUnknown, LengthOK},
		{num(54, 12345678901), FixedLine, LengthTooLong},
		{num(54, 12345678901), Mobile, LengthOK},
		{num(54, 12345678901), TollFree, LengthTooLong},
	})
}

func TestIsPossibleNumberForTypeWithReason_LocalOnly(t *testing.T) {
	runLengthCases(t, []lengthCase{
		// A local-only length.
		{num(49, 12), TypeUnknown, LengthLocalOnly},
		{num(49, 12), FixedLine, LengthLocalOnly},
		// Mobiles have ten or eleven digits and no local-only lengths.
		{num(49, 12), Mobile, LengthTooShort},
	})
}

func TestIsPossibleNumberForTypeWithReason_DataMissingForSizeReasons(t *testing.T) {
	// The fixed-line lengths are the same as the general ones, so the
	// metadata omits them.
	runLengthCases(t, []lengthCase{
		// A local-only number.
		{num(55, 12345678), TypeUnknown, LengthLocalOnly},
		{num(55, 12345678), FixedLine, LengthLocalOnly},
		// A normal-length number.
		{num(55, 1234567890), TypeUnknown, LengthOK},
		{num(55, 1234567890), FixedLine, LengthOK},
	})
}

func TestIsPossibleNumberForTypeWithReason_NumberTypeNotSupportedForRegion(t *testing.T) {
	runLengthCases(t, []lengthCase{
		// BR has no mobile numbers at all.
		{num(55, 12345678), Mobile, LengthInvalid},
		// But this is a fixed-line length.
		{num(55, 12345678), FixedLineOrMobile, LengthLocalOnly},
		// Too short for a fixed line, and there are no mobiles.
		{num(55, 1234567), Mobile, LengthInvalid},
		{num(55, 1234567), FixedLineOrMobile, LengthTooShort},
		{num(55, 1234567), FixedLine, LengthTooShort},
		// Too short for a mobile, and there are no fixed lines.
		{num(882, 1234567), Mobile, LengthTooShort},
		{num(882, 1234567), FixedLineOrMobile, LengthTooShort},
		{num(882, 1234567), FixedLine, LengthInvalid},
		// 979 has no fixed-line or mobile numbers at all.
		{num(979, 123456789), Mobile, LengthInvalid},
		{num(979, 123456789), FixedLine, LengthInvalid},
		{num(979, 123456789), FixedLineOrMobile, LengthInvalid},
		{num(979, 123456789), PremiumRate, LengthOK},
	})
}

func TestIsPossibleNumberForTypeWithReason_FixedLineOrMobile(t *testing.T) {
	// FixedLineOrMobile accepts the lengths of fixed lines and of mobiles.
	runLengthCases(t, []lengthCase{
		{num(290, 1234), FixedLine, LengthTooShort},
		{num(290, 1234), Mobile, LengthOK},
		{num(290, 1234), FixedLineOrMobile, LengthOK},
		{num(290, 12345), FixedLine, LengthTooShort},
		{num(290, 12345), Mobile, LengthTooLong},
		{num(290, 12345), FixedLineOrMobile, LengthInvalid},
		{num(290, 123456), FixedLine, LengthOK},
		{num(290, 123456), Mobile, LengthTooLong},
		{num(290, 123456), FixedLineOrMobile, LengthOK},
		{num(290, 1234567), FixedLine, LengthTooLong},
		{num(290, 1234567), Mobile, LengthTooLong},
		{num(290, 1234567), FixedLineOrMobile, LengthTooLong},
		// Eight digits are possible only for toll-free and premium-rate
		// numbers.
		{num(290, 12345678), TollFree, LengthOK},
		{num(290, 12345678), FixedLineOrMobile, LengthTooLong},
	})
}

func TestIsNotPossibleNumber(t *testing.T) {
	runPossibleCases(t, []possibleCase{
		{usLongNumber, TypeUnknown, false},
		{internationalTollFreeTooLong, TypeUnknown, false},
		{num(1, 253000), TypeUnknown, false},
		{num(44, 300), TypeUnknown, false},
	})
	tests := []struct{ in, region string }{
		{"+1 650 253 00000", "US"},
		{"(650) 253-00000", "US"},
		{"I want a Pizza", "US"},
		{"253-000", "US"},
		{"1 3000", "GB"},
		{"+44 300", "GB"},
		{"+800 1234 5678 9", "001"},
	}
	for _, tt := range tests {
		if possibleString(tt.in, tt.region) {
			t.Errorf("possibleString(%q, %q) = true, want false", tt.in, tt.region)
		}
	}
}

func TestCanBeInternationallyDialled(t *testing.T) {
	tests := []struct {
		n    Number
		want bool
	}{
		// The test metadata says US toll-free numbers cannot be dialled
		// internationally.
		{usTollFree, false},
		{usNumber, true},
		// An invalid number.
		{usLocalNumber, true},
		// There is no data for NZ.
		{nzNumber, true},
		{internationalTollFree, true},
	}
	for _, tt := range tests {
		if got := testSet().canDialInternationally(tt.n); got != tt.want {
			t.Errorf("canDialInternationally(%#v) = %t, want %t", tt.n, got, tt.want)
		}
	}
}

func TestGetSupportedTypesForRegion(t *testing.T) {
	tests := []struct {
		region string
		typ    Type
		want   bool
	}{
		{"BR", FixedLine, true},
		// The test metadata has no mobiles for Brazil.
		{"BR", Mobile, false},
		// TypeUnknown is never supported.
		{"BR", TypeUnknown, false},
		// US numbers are mostly FixedLineOrMobile, but that is reported as
		// FixedLine and Mobile.
		{"US", FixedLine, true},
		{"US", Mobile, true},
		{"US", FixedLineOrMobile, false},
	}
	for _, tt := range tests {
		types := testSet().region(tt.region).supportedTypes()
		if got := slices.Contains(types, tt.typ); got != tt.want {
			t.Errorf("region(%q).supportedTypes() = %v; contains %v = %t, want %t", tt.region, types, tt.typ, got, tt.want)
		}
	}
	// An invalid region has no metadata, so no types.
	if meta := testSet().region("ZZ"); meta != nil {
		t.Errorf("region(%q) = %v, want nil", "ZZ", meta.id)
	}
}
