package timezone

import (
	"slices"
	"strconv"
	"testing"

	"uradical.io/go/phonenumber"
	"uradical.io/go/phonenumber/internal/prefixmap"
)

// Tests ported from upstream's PhoneNumberToTimeZonesMapperTest.java, with
// the real metadata and upstream's test time zone data.

func testTable(t *testing.T) prefixmap.Table {
	t.Helper()
	table, err := prefixmap.ReadFile(testdataFS, "testdata/map_data.gz")
	if err != nil {
		t.Fatal(err)
	}
	return table
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
	auNumber  = num(61, 236618300)
	caNumber  = num(1, 6048406565)
	koNumber  = num(82, 22123456)
	koInvalid = num(82, 1234)
	usNumber1 = num(1, 6509600000)
	usNumber2 = num(1, 2128120000)
	usNumber3 = num(1, 6174240000)
	usInvalid = num(1, 123456789)
	// Parse rejects country calling code 999, so this is the zero Number.
	invalidCountryCode    = num(999, 2423651234)
	internationalTollFree = num(800, 12345678)
)

const (
	chicago    = "America/Chicago"
	losAngeles = "America/Los_Angeles"
	newYork    = "America/New_York"
	winnipeg   = "America/Winnipeg"
	seoul      = "Asia/Seoul"
	sydney     = "Australia/Sydney"
)

var nanpaTimeZones = []string{newYork, chicago, winnipeg, losAngeles}

type zonesCase struct {
	n    phonenumber.Number
	want []string
}

func checkZones(t *testing.T, name string, f func(prefixmap.Table, phonenumber.Number) []string, cases []zonesCase) {
	t.Helper()
	table := testTable(t)
	for _, tt := range cases {
		if got := f(table, tt.n); !slices.Equal(got, tt.want) {
			t.Errorf("%s(%v) = %q, want %q", name, tt.n, got, tt.want)
		}
	}
}

func TestGetTimeZonesForNumber(t *testing.T) {
	checkZones(t, "forNumber", forNumber, []zonesCase{
		// Invalid numbers, though their country calling codes have data.
		{usInvalid, []string{Unknown}},
		{koInvalid, []string{Unknown}},
		{auNumber, []string{sydney}},
		{koNumber, []string{seoul}},
		{caNumber, []string{winnipeg}},
		{usNumber1, []string{losAngeles}},
		{usNumber2, []string{newYork}},
		{invalidCountryCode, []string{Unknown}},
		// Not geographical.
		{internationalTollFree, []string{Unknown}},
	})
}

func TestGetTimeZonesForValidNumber(t *testing.T) {
	checkZones(t, "forGeographical", forGeographical, []zonesCase{
		// Invalid numbers get their country calling code's time zones.
		{usInvalid, nanpaTimeZones},
		{koInvalid, []string{seoul}},
		{auNumber, []string{sydney}},
		{koNumber, []string{seoul}},
		{caNumber, []string{winnipeg}},
		{usNumber1, []string{losAngeles}},
		{usNumber2, []string{newYork}},
		{invalidCountryCode, []string{Unknown}},
		{internationalTollFree, []string{Unknown}},
	})
}

func TestGetTimeZonesForValidNumberSearchingAtCountryCodeLevel(t *testing.T) {
	// A valid number no longer prefix covers gets its country's zones.
	checkZones(t, "forNumber", forNumber, []zonesCase{{usNumber3, nanpaTimeZones}})
}
