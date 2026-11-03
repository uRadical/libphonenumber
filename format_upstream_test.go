package phonenumber

import "testing"

// Tests ported from upstream's PhoneNumberUtilTest.java, run against the
// test metadata. Each Go test is named after the upstream method it ports.

type formatCase struct {
	n    Number
	f    Format
	want string
}

func runFormatCases(t *testing.T, tests []formatCase) {
	t.Helper()
	for _, tt := range tests {
		if got := testSet().format(tt.n, tt.f); got != tt.want {
			t.Errorf("format(%v, %v) = %q, want %q", tt.n, tt.f, got, tt.want)
		}
	}
}

type formatFromCase struct {
	n    Number
	from string
	want string
}

func runFormatFromCases(t *testing.T, tests []formatFromCase) {
	t.Helper()
	for _, tt := range tests {
		if got := testSet().formatFrom(tt.n, tt.from); got != tt.want {
			t.Errorf("formatFrom(%v, %q) = %q, want %q", tt.n, tt.from, got, tt.want)
		}
	}
}

func TestFormatUSNumber(t *testing.T) {
	runFormatCases(t, []formatCase{
		{usNumber, National, "650 253 0000"},
		{usNumber, International, "+1 650 253 0000"},
		{usTollFree, National, "800 253 0000"},
		{usTollFree, International, "+1 800 253 0000"},
		{usPremium, National, "900 253 0000"},
		{usPremium, International, "+1 900 253 0000"},
		{usPremium, RFC3966, "tel:+1-900-253-0000"},
		// Upstream formats an all-zero number with its raw input when it
		// has one; Number keeps no raw input.
		{usSpoof, National, "0"},
	})
}

func TestFormatAUShortCodeNumber(t *testing.T) {
	n := mustParseTest(t, "000", "AU")
	if got, want := testSet().format(n, E164), "+61000"; got != want {
		t.Errorf("format(parse(%q, %q), E164) = %q, want %q", "000", "AU", got, want)
	}
	// Upstream's second case formats a PG number from its raw input,
	// which Number does not keep.
}

func TestFormatBSNumber(t *testing.T) {
	runFormatCases(t, []formatCase{
		{bsNumber, National, "242 365 1234"},
		{bsNumber, International, "+1 242 365 1234"},
	})
}

func TestFormatGBNumber(t *testing.T) {
	runFormatCases(t, []formatCase{
		{gbNumber, National, "(020) 7031 3000"},
		{gbNumber, International, "+44 20 7031 3000"},
		{gbMobile, National, "(07912) 345 678"},
		{gbMobile, International, "+44 7912 345 678"},
	})
}

func TestFormatDENumber(t *testing.T) {
	runFormatCases(t, []formatCase{
		{num(49, 301234), National, "030/1234"},
		{num(49, 301234), International, "+49 30/1234"},
		{num(49, 301234), RFC3966, "tel:+49-30-1234"},
		{num(49, 291123), National, "0291 123"},
		{num(49, 291123), International, "+49 291 123"},
		{num(49, 29112345678), National, "0291 12345678"},
		{num(49, 29112345678), International, "+49 291 12345678"},
		{num(49, 912312345), National, "09123 12345"},
		{num(49, 912312345), International, "+49 9123 12345"},
		{num(49, 80212345), National, "08021 2345"},
		{num(49, 80212345), International, "+49 8021 2345"},
		// Short numbers, which are mostly invalid, are formatted without
		// the national prefix.
		{deShortNumber, National, "1234"},
		{deShortNumber, International, "+49 1234"},
		{num(49, 41341234), National, "04134 1234"},
	})
}

func TestFormatITNumber(t *testing.T) {
	runFormatCases(t, []formatCase{
		{itNumber, National, "02 3661 8300"},
		{itNumber, International, "+39 02 3661 8300"},
		{itNumber, E164, "+390236618300"},
		{itMobile, National, "345 678 901"},
		{itMobile, International, "+39 345 678 901"},
		{itMobile, E164, "+39345678901"},
	})
}

func TestFormatAUNumber(t *testing.T) {
	runFormatCases(t, []formatCase{
		{auNumber, National, "02 3661 8300"},
		{auNumber, International, "+61 2 3661 8300"},
		{auNumber, E164, "+61236618300"},
		{num(61, 1800123456), National, "1800 123 456"},
		{num(61, 1800123456), International, "+61 1800 123 456"},
		{num(61, 1800123456), E164, "+611800123456"},
	})
}

func TestFormatARNumber(t *testing.T) {
	runFormatCases(t, []formatCase{
		{arNumber, National, "011 8765-4321"},
		{arNumber, International, "+54 11 8765-4321"},
		{arNumber, E164, "+541187654321"},
		{arMobile, National, "011 15 8765-4321"},
		{arMobile, International, "+54 9 11 8765 4321"},
		{arMobile, E164, "+5491187654321"},
	})
}

func TestFormatMXNumber(t *testing.T) {
	runFormatCases(t, []formatCase{
		{mxMobile1, National, "045 234 567 8900"},
		{mxMobile1, International, "+52 1 234 567 8900"},
		{mxMobile1, E164, "+5212345678900"},
		{mxMobile2, National, "045 55 1234 5678"},
		{mxMobile2, International, "+52 1 55 1234 5678"},
		{mxMobile2, E164, "+5215512345678"},
		{mxNumber1, National, "01 33 1234 5678"},
		{mxNumber1, International, "+52 33 1234 5678"},
		{mxNumber1, E164, "+523312345678"},
		{mxNumber2, National, "01 821 123 4567"},
		{mxNumber2, International, "+52 821 123 4567"},
		{mxNumber2, E164, "+528211234567"},
	})
}

func TestFormatOutOfCountryCallingNumber(t *testing.T) {
	arMobileWithExt := arMobile.withExt("1234")
	runFormatFromCases(t, []formatFromCase{
		{usPremium, "DE", "00 1 900 253 0000"},
		{usNumber, "BS", "1 650 253 0000"},
		{usNumber, "PL", "00 1 650 253 0000"},
		{gbMobile, "US", "011 44 7912 345 678"},
		{deShortNumber, "GB", "00 49 1234"},
		// Short numbers, which are mostly invalid, are formatted without
		// the national prefix.
		{deShortNumber, "DE", "1234"},
		{itNumber, "US", "011 39 02 3661 8300"},
		{itNumber, "IT", "02 3661 8300"},
		{itNumber, "SG", "+39 02 3661 8300"},
		{sgNumber, "SG", "6521 8000"},
		{arMobile, "US", "011 54 9 11 8765 4321"},
		{internationalTollFree, "US", "011 800 1234 5678"},
		{arMobileWithExt, "US", "011 54 9 11 8765 4321 ext. 1234"},
		{arMobileWithExt, "AU", "0011 54 9 11 8765 4321 ext. 1234"},
		{arMobileWithExt, "AR", "011 15 8765-4321 ext. 1234"},
	})
}

func TestFormatOutOfCountryWithInvalidRegion(t *testing.T) {
	runFormatFromCases(t, []formatFromCase{
		// AQ is not a region with a numbering plan, so this falls back to
		// the international format.
		{usNumber, "AQ", "+1 650 253 0000"},
		// So does 001, the code for non-geographical entities.
		{usNumber, "001", "+1 650 253 0000"},
	})
}

func TestFormatOutOfCountryWithPreferredIntlPrefix(t *testing.T) {
	runFormatFromCases(t, []formatFromCase{
		// AU accepts 0011 and 0012, and prefers 0011.
		{itNumber, "AU", "0011 39 02 3661 8300"},
		// A preferred prefix may contain "~", meaning wait for a tone.
		{itNumber, "UZ", "8~10 39 02 3661 8300"},
	})
}

func TestFormatE164Number(t *testing.T) {
	runFormatCases(t, []formatCase{
		{usNumber, E164, "+16502530000"},
		{deNumber, E164, "+4930123456"},
		{internationalTollFree, E164, "+80012345678"},
	})
}

func TestFormatNumberWithExtension(t *testing.T) {
	nz := nzNumber.withExt("1234")
	runFormatCases(t, []formatCase{
		// The default extension prefix.
		{nz, National, "03-331 6005 ext. 1234"},
		{nz, RFC3966, "tel:+64-3-331-6005;ext=1234"},
		// US metadata overrides the extension prefix.
		{usNumber.withExt("4567"), National, "650 253 0000 extn. 4567"},
	})
}

func TestCountryWithNoNumberDesc(t *testing.T) {
	// The test metadata has no number descriptions for Andorra.
	ad := num(376, 12345)
	runFormatCases(t, []formatCase{
		{ad, International, "+376 12345"},
		{ad, E164, "+37612345"},
		{ad, National, "12345"},
	})
	if got, want := testSet().numberType(ad), TypeUnknown; got != want {
		t.Errorf("numberType(%v) = %v, want %v", ad, got, want)
	}
	if testSet().valid(ad) {
		t.Errorf("valid(%v) = true, want false", ad)
	}
	// Dialling a US number from within Andorra.
	runFormatFromCases(t, []formatFromCase{
		{usNumber, "AD", "00 1 650 253 0000"},
	})
}

func TestUnknownCountryCallingCode(t *testing.T) {
	n := unknownCountryCodeNoRawInput
	if testSet().valid(n) {
		t.Errorf("valid(%v) = true, want false", n)
	}
	// The E.164 form of a number with an unknown country calling code is
	// not well defined; upstream joins the code and the national number.
	runFormatCases(t, []formatCase{
		{n, E164, "+212345"},
	})
}
