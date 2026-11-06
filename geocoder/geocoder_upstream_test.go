package geocoder

import (
	"embed"
	"io/fs"
	"strconv"
	"testing"

	"uradical.io/go/phonenumber"
)

// Tests ported from upstream's PhoneNumberOfflineGeocoderTest.java. As
// upstream's, they use the real metadata and upstream's test geocoding
// data; country names come from the real CLDR data.

//go:embed testdata/geocoding/*.gz
var testdataFS embed.FS

func testGeocoder(t *testing.T) *geocoder {
	t.Helper()
	data, err := fs.Sub(testdataFS, "testdata/geocoding")
	if err != nil {
		t.Fatal(err)
	}
	return newGeocoder(data, namesFS, "names.gz")
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
	koNumber1      = num(82, 22123456)
	koNumber2      = num(82, 322123456)
	koNumber3      = num(82, 6421234567)
	koInvalid      = num(82, 1234)
	koMobile       = num(82, 101234567)
	usNumber1      = num(1, 6502530000)
	usNumber2      = num(1, 6509600000)
	usNumber3      = num(1, 2128120000)
	usNumber4      = num(1, 6174240000)
	usInvalid      = num(1, 123456789)
	nanpaTollFree  = num(1, 8002431234)
	bsNumber1      = num(1, 2423651234)
	auNumber       = num(61, 236618300)
	arMobileNumber = num(54, 92214000000)
	// Parse rejects country calling code 999, so this is the zero Number.
	invalidCountryCode    = num(999, 2423651234)
	internationalTollFree = num(800, 12345678)
)

type descCase struct {
	n          phonenumber.Number
	lang, want string
}

func checkDescriptions(t *testing.T, cases []descCase) {
	t.Helper()
	g := testGeocoder(t)
	for _, tt := range cases {
		if got := g.description(tt.n, tt.lang, "", false); got != tt.want {
			t.Errorf("description(%v, %q) = %q, want %q", tt.n, tt.lang, got, tt.want)
		}
	}
}

func TestGetDescriptionForNumberWithNoDataFile(t *testing.T) {
	checkDescriptions(t, []descCase{
		// No Chinese data for US numbers, so the country name in
		// simplified Chinese.
		{usNumber1, "zh-CN", "\u7F8E\u56FD"},
		{bsNumber1, "en-US", "Bahamas"},
		{auNumber, "en-US", "Australia"},
		{invalidCountryCode, "en-US", ""},
		{internationalTollFree, "en-US", ""},
	})
}

func TestGetDescriptionForNumberWithMissingPrefix(t *testing.T) {
	// A valid number the data does not cover gives its country.
	checkDescriptions(t, []descCase{{usNumber4, "en-US", "United States"}})
}

func TestGetDescriptionForNumberBelongingToMultipleCountriesIsEmpty(t *testing.T) {
	checkDescriptions(t, []descCase{{nanpaTollFree, "en-US", ""}})
}

func TestGetDescriptionForNumber_en_US(t *testing.T) {
	checkDescriptions(t, []descCase{
		{usNumber1, "en-US", "CA"},
		{usNumber2, "en-US", "Mountain View, CA"},
		{usNumber3, "en-US", "New York, NY"},
	})
}

func TestGetDescriptionForKoreanNumber(t *testing.T) {
	checkDescriptions(t, []descCase{
		{koNumber1, "en", "Seoul"},
		{koNumber2, "en", "Incheon"},
		{koNumber3, "en", "Jeju"},
		{koNumber1, "ko", "\uC11C\uC6B8"},
		{koNumber2, "ko", "\uC778\uCC9C"},
	})
}

func TestGetDescriptionForArgentinianMobileNumber(t *testing.T) {
	checkDescriptions(t, []descCase{{arMobileNumber, "en", "La Plata"}})
}

func TestGetDescriptionForFallBack(t *testing.T) {
	checkDescriptions(t, []descCase{
		// German data has the number.
		{usNumber1, "de", "Kalifornien"},
		// German falls back to English.
		{usNumber3, "de", "New York, NY"},
		// So does Italian.
		{usNumber1, "it", "CA"},
		// Korean does not.
		{koNumber3, "ko", "\uB300\uD55C\uBBFC\uAD6D"},
	})
}

func TestGetDescriptionForNumberWithUserRegion(t *testing.T) {
	g := testGeocoder(t)
	for _, tt := range []struct {
		n                      phonenumber.Number
		lang, userRegion, want string
	}{
		// A user in Italy with a US number sees only the country.
		{usNumber1, "es-ES", "IT", "Estados Unidos"},
		// So does a user in an unknown region.
		{usNumber1, "es-ES", "ZZ", "Estados Unidos"},
		// A user in the US sees the area.
		{usNumber1, "de", "US", "Kalifornien"},
		// French has no data, so English.
		{usNumber1, "fr", "US", "CA"},
		// An invalid number.
		{usInvalid, "en", "US", ""},
	} {
		if got := g.description(tt.n, tt.lang, tt.userRegion, true); got != tt.want {
			t.Errorf("description(%v, %q, %q) = %q, want %q", tt.n, tt.lang, tt.userRegion, got, tt.want)
		}
	}
}

func TestGetDescriptionForInvalidNumber(t *testing.T) {
	checkDescriptions(t, []descCase{{koInvalid, "en", ""}, {usInvalid, "en", ""}})
}

func TestGetDescriptionForNonGeographicalNumberWithGeocodingPrefix(t *testing.T) {
	// The data has the prefix, but the number is not geographical.
	checkDescriptions(t, []descCase{{koMobile, "en", "South Korea"}})
}
