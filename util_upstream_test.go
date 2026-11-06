package phonenumber

import "testing"

// Tests ported from upstream's PhoneNumberUtilTest.java for the smaller
// helpers, run against the test metadata.

func TestIsNumberGeographical(t *testing.T) {
	ms := testSet()
	for _, tt := range []struct {
		n    Number
		want bool
	}{
		{bsMobile, false},
		{auNumber, true},
		{internationalTollFree, false},
		// Mobile numbers are geographical in some countries.
		{arMobile, true},
		{mxMobile1, true},
		{mxMobile2, true},
	} {
		if got := isGeographical(ms.numberType(tt.n), int(tt.n.countryCode)); got != tt.want {
			t.Errorf("geographical(%v) = %v, want %v", tt.n, got, tt.want)
		}
	}
}

func TestGetLengthOfGeographicalAreaCode(t *testing.T) {
	ms := testSet()
	for _, tt := range []struct {
		n    Number
		want int
	}{
		{usNumber, 3},   // 650
		{usTollFree, 0}, // Toll-free numbers have no area code.
		{gbNumber, 2},   // 20
		{gbMobile, 0},   // UK mobiles have no area code.
		{arNumber, 2},   // 11
		{arMobile, 3},   // Argentinian mobiles do.
		{auNumber, 1},   // 2
		{itNumber, 2},   // No national prefix, but an area code.
		{mxNumber1, 2},  // No national prefix, but an area code.
		{sgNumber, 0},   // Neither.
		{usShortByOneNumber, 0},
		{internationalTollFree, 0},
		// Chinese mobiles are geographical but have no area code.
		{num(86, 18912341234), 0},
	} {
		if got := ms.areaCodeLength(tt.n); got != tt.want {
			t.Errorf("areaCodeLength(%v) = %d, want %d", tt.n, got, tt.want)
		}
	}
}

func TestGetLengthOfNationalDestinationCode(t *testing.T) {
	ms := testSet()
	for _, tt := range []struct {
		n    Number
		want int
	}{
		{usNumber, 3},   // 650
		{usTollFree, 3}, // 800
		{gbNumber, 2},   // 20
		{gbMobile, 4},   // 7912
		{arNumber, 2},   // 11
		{arMobile, 3},   // 911
		{auNumber, 1},   // 2
		{sgNumber, 4},   // 6521
		{usShortByOneNumber, 0},
		// An unknown country calling code.
		{num(123, 6502530000), 0},
		{internationalTollFree, 4}, // 1234
		// Chinese mobiles have no area code but do have an NDC.
		{num(86, 18912341234), 3},
	} {
		if got := ms.ndcLength(tt.n); got != tt.want {
			t.Errorf("ndcLength(%v) = %d, want %d", tt.n, got, tt.want)
		}
	}
}

func TestGetCountryMobileToken(t *testing.T) {
	if got := CountryMobileToken(54); got != "9" {
		t.Errorf("CountryMobileToken(AR) = %q, want 9", got)
	}
	if got := CountryMobileToken(46); got != "" {
		t.Errorf("CountryMobileToken(SE) = %q, want \"\"", got)
	}
}

func TestGetInvalidExampleNumber(t *testing.T) {
	ms := testSet()
	for _, region := range []string{"001", "CS"} {
		if n, ok := ms.invalidExample(region); ok {
			t.Errorf("invalidExample(%s) = %v, want none", region, n)
		}
	}
	n, ok := ms.invalidExample("US")
	if !ok || n.CountryCode() != 1 || n.national == 0 {
		t.Errorf("invalidExample(US) = %v, %v", n, ok)
	}
	if ms.valid(n) {
		t.Errorf("invalidExample(US) = %v, which is valid", n)
	}
}

func TestConvertAlphaCharactersInNumber(t *testing.T) {
	if got := ConvertAlphaCharacters("1800-ABC-DEF"); got != "1800-222-333" {
		t.Errorf("ConvertAlphaCharacters = %q", got)
	}
}

func TestNormaliseStripNonDiallableCharacters(t *testing.T) {
	if got := NormalizeDiallable("03*4-56&+1a#234"); got != "03*456+1#234" {
		t.Errorf("NormalizeDiallable = %q", got)
	}
}

func TestTruncateTooLongNumber(t *testing.T) {
	ms := testSet()
	for _, tt := range []struct {
		n, want Number
		ok      bool
	}{
		// GB 080 1234 5678 with four extra digits.
		{num(44, 80123456780123), num(44, 8012345678), true},
		// IT 022 3456 7890 with three extra digits.
		{num(39, 2234567890123).withZeros(1), num(39, 2234567890).withZeros(1), true},
		// US 650 253 0000 with one extra digit.
		{usLongNumber, usNumber, true},
		{internationalTollFreeTooLong, internationalTollFree, true},
		// A valid number is returned as it is.
		{num(39, 2234567890).withZeros(1), num(39, 2234567890).withZeros(1), true},
		// The test metadata has no US numbers starting 240.
		{num(1, 2401234567), num(1, 2401234567), false},
		// Too short.
		{num(1, 1234), num(1, 1234), false},
	} {
		got, ok := ms.truncateTooLong(tt.n)
		if got != tt.want || ok != tt.ok {
			t.Errorf("truncateTooLong(%v) = %v, %v, want %v, %v", tt.n, got, ok, tt.want, tt.ok)
		}
	}
}

func TestIsAlphaNumber(t *testing.T) {
	for s, want := range map[string]bool{
		"1800 six-flags":                true,
		"1800 six-flags ext. 1234":      true,
		"+800 six-flags":                true,
		"180 six-flags":                 true,
		"1800 123-1234":                 false,
		"1 six-flags":                   false,
		"18 six-flags":                  false,
		"1800 123-1234 extension: 1234": false,
		"+800 1234-1234":                false,
	} {
		if got := IsAlphaNumber(s); got != want {
			t.Errorf("IsAlphaNumber(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestIsMobileNumberPortableRegion(t *testing.T) {
	ms := testSet()
	for region, want := range map[string]bool{"US": true, "GB": true, "AE": false, "BS": false} {
		if got := ms.mobilePortable(region); got != want {
			t.Errorf("mobilePortable(%s) = %v, want %v", region, got, want)
		}
	}
}
