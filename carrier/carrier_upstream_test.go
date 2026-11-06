package carrier

import (
	"embed"
	"io/fs"
	"strconv"
	"testing"

	"uradical.io/go/phonenumber"
	"uradical.io/go/phonenumber/internal/prefixmap"
)

// Tests ported from upstream's PhoneNumberToCarrierMapperTest.java, with
// the real metadata and upstream's test carrier data.

//go:embed testdata/carrier/*.gz
var testdataFS embed.FS

func testData(t *testing.T) *prefixmap.Files {
	t.Helper()
	data, err := fs.Sub(testdataFS, "testdata/carrier")
	if err != nil {
		t.Fatal(err)
	}
	return prefixmap.NewFiles(data)
}

// num parses a number from its country calling code and national number,
// or returns the zero Number when that does not parse.
func num(cc int, national uint64) phonenumber.Number {
	nsn := strconv.FormatUint(national, 10)
	n, err := phonenumber.Parse("+"+strconv.Itoa(cc)+nsn, "")
	if err == nil && (n.CountryCode() != cc || n.NationalNumber() != nsn) {
		panic("num(" + strconv.Itoa(cc) + ", " + nsn + ") parsed as " + n.String())
	}
	return n
}

var (
	aoMobile1       = num(244, 917654321)
	aoMobile2       = num(244, 927654321)
	aoFixed1        = num(244, 22254321)
	aoFixed2        = num(244, 26254321)
	aoInvalid       = num(244, 101234)
	ukMobile1       = num(44, 7387654321)
	ukMobile2       = num(44, 7487654321)
	ukFixed1        = num(44, 1123456789)
	ukFixed2        = num(44, 2987654321)
	ukInvalid       = num(44, 7301234)
	ukPager         = num(44, 7601234567)
	usFixedOrMobile = num(1, 6502123456)
	// Parse rejects country calling code 999, so this is the zero Number.
	invalidCountryCode    = num(999, 2423651234)
	internationalTollFree = num(800, 12345678)
)

func check(t *testing.T, what, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q, want %q", what, got, want)
	}
}

func TestGetNameForMobilePortableRegion(t *testing.T) {
	f := testData(t)
	check(t, "name(UK mobile, en)", name(f, ukMobile1, "en"), "British carrier")
	check(t, "name(UK mobile, sv-SE)", name(f, ukMobile1, "sv-SE"), "Brittisk operat\u00F6r")
	check(t, "name(UK mobile, fr)", name(f, ukMobile1, "fr"), "British carrier")
	// The UK has mobile number portability.
	check(t, "safeDisplayName(UK mobile)", safeDisplayName(f, ukMobile1, "en"), "")
}

func TestGetNameForNonMobilePortableRegion(t *testing.T) {
	f := testData(t)
	check(t, "name(AO mobile)", name(f, aoMobile1, "en"), "Angolan carrier")
	check(t, "safeDisplayName(AO mobile)", safeDisplayName(f, aoMobile1, "en"), "Angolan carrier")
}

func TestGetNameForFixedLineNumber(t *testing.T) {
	f := testData(t)
	check(t, "name(AO fixed)", name(f, aoFixed1, "en"), "")
	check(t, "name(UK fixed)", name(f, ukFixed1, "en"), "")
	// With data for it, a number assumed valid gets a carrier.
	check(t, "nameForValid(AO fixed)", nameForValid(f, aoFixed2, "en"), "Angolan fixed line carrier")
	check(t, "nameForValid(UK fixed)", nameForValid(f, ukFixed2, "en"), "")
}

func TestGetNameForFixedOrMobileNumber(t *testing.T) {
	check(t, "name(US)", name(testData(t), usFixedOrMobile, "en"), "US carrier")
}

func TestGetNameForPagerNumber(t *testing.T) {
	check(t, "name(UK pager)", name(testData(t), ukPager, "en"), "British pager")
}

func TestGetNameForNumberWithNoDataFile(t *testing.T) {
	f := testData(t)
	for _, n := range []phonenumber.Number{invalidCountryCode, internationalTollFree} {
		check(t, "name("+n.String()+")", name(f, n, "en"), "")
		check(t, "nameForValid("+n.String()+")", nameForValid(f, n, "en"), "")
	}
}

func TestGetNameForNumberWithMissingPrefix(t *testing.T) {
	f := testData(t)
	check(t, "name(UK mobile 2)", name(f, ukMobile2, "en"), "")
	check(t, "name(AO mobile 2)", name(f, aoMobile2, "en"), "")
}

func TestGetNameForInvalidNumber(t *testing.T) {
	f := testData(t)
	check(t, "name(UK invalid)", name(f, ukInvalid, "en"), "")
	check(t, "name(AO invalid)", name(f, aoInvalid, "en"), "")
}
