package shortnumber

import (
	"strconv"
	"testing"

	"uradical.io/go/phonenumber"
)

// Tests ported from upstream's ShortNumberInfoTest.java. Like upstream's,
// they run against the real short-number metadata. Upstream parses with
// its test metadata; these parse with the real metadata, which gives the
// same numbers for these inputs.

func parse(t *testing.T, s, region string) phonenumber.Number {
	t.Helper()
	n, err := phonenumber.Parse(s, region)
	if err != nil {
		t.Fatalf("Parse(%q, %q): %v", s, region, err)
	}
	return n
}

// num builds a Number from a country calling code and national number, as
// upstream's tests build PhoneNumber protos.
func num(t *testing.T, cc int, national uint64) phonenumber.Number {
	t.Helper()
	nsn := strconv.FormatUint(national, 10)
	n := parse(t, "+"+strconv.Itoa(cc)+nsn, "")
	if n.CountryCode() != cc || n.NationalNumber() != nsn {
		t.Fatalf("num(%d, %d) parsed as +%d %s", cc, national, n.CountryCode(), n.NationalNumber())
	}
	return n
}

func check(t *testing.T, name string, got, want bool) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func checkCost(t *testing.T, name string, got, want Cost) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func TestIsPossibleShortNumber(t *testing.T) {
	check(t, "Possible(+33 123456)", Possible(num(t, 33, 123456)), true)
	check(t, "PossibleFor(123456, FR)", PossibleFor(parse(t, "123456", "FR"), "FR"), true)
	// Upstream also checks +33 9, but Parse rejects a one-digit national
	// number, so no Number can hold it. The zero Number stands in.
	check(t, "Possible(Number{})", Possible(phonenumber.Number{}), false)
	// GB and GG share country calling code 44; this number is possible
	// but not valid.
	check(t, "Possible(+44 11001)", Possible(num(t, 44, 11001)), true)
}

func TestIsValidShortNumber(t *testing.T) {
	check(t, "Valid(+33 1010)", Valid(num(t, 33, 1010)), true)
	check(t, "ValidFor(1010, FR)", ValidFor(parse(t, "1010", "FR"), "FR"), true)
	check(t, "Valid(+33 123456)", Valid(num(t, 33, 123456)), false)
	check(t, "ValidFor(123456, FR)", ValidFor(parse(t, "123456", "FR"), "FR"), false)
	// GB and GG share country calling code 44.
	check(t, "Valid(+44 18001)", Valid(num(t, 44, 18001)), true)
}

func TestIsCarrierSpecific(t *testing.T) {
	check(t, "CarrierSpecific(+1 33669)", CarrierSpecific(num(t, 1, 33669)), true)
	check(t, "CarrierSpecificFor(33669, US)", CarrierSpecificFor(parse(t, "33669", "US"), "US"), true)
	check(t, "CarrierSpecific(+1 911)", CarrierSpecific(num(t, 1, 911)), false)
	check(t, "CarrierSpecificFor(911, US)", CarrierSpecificFor(parse(t, "911", "US"), "US"), false)
	someRegion := num(t, 1, 211)
	check(t, "CarrierSpecific(+1 211)", CarrierSpecific(someRegion), true)
	check(t, "CarrierSpecificFor(+1 211, US)", CarrierSpecificFor(someRegion, "US"), true)
	check(t, "CarrierSpecificFor(+1 211, BB)", CarrierSpecificFor(someRegion, "BB"), false)
}

func TestIsSmsService(t *testing.T) {
	n := num(t, 1, 21234)
	check(t, "SMSServiceFor(+1 21234, US)", SMSServiceFor(n, "US"), true)
	check(t, "SMSServiceFor(+1 21234, BB)", SMSServiceFor(n, "BB"), false)
}

func TestGetExpectedCost(t *testing.T) {
	for _, c := range []Cost{PremiumRate, StandardRate, TollFree} {
		ex := exampleForCost("FR", c)
		checkCost(t, "ExpectedCostFor("+ex+", FR)", ExpectedCostFor(parse(t, ex, "FR"), "FR"), c)
		national, err := strconv.ParseUint(ex, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		checkCost(t, "ExpectedCost(+33 "+ex+")", ExpectedCost(num(t, 33, national)), c)
	}
	checkCost(t, "ExpectedCostFor(12345, FR)", ExpectedCostFor(parse(t, "12345", "FR"), "FR"), CostUnknown)
	checkCost(t, "ExpectedCost(+33 12345)", ExpectedCost(num(t, 33, 12345)), CostUnknown)

	// An invalid number may still have a cost other than CostUnknown.
	invalid := parse(t, "116123", "FR")
	check(t, "ValidFor(116123, FR)", ValidFor(invalid, "FR"), false)
	checkCost(t, "ExpectedCostFor(116123, FR)", ExpectedCostFor(invalid, "FR"), TollFree)
	check(t, "Valid(+33 116123)", Valid(num(t, 33, 116123)), false)
	checkCost(t, "ExpectedCost(+33 116123)", ExpectedCost(num(t, 33, 116123)), TollFree)

	// An unknown region, and an unknown country calling code. Parse
	// rejects +123, so upstream's PhoneNumber with country code 123 is
	// the zero Number here: neither has a region.
	checkCost(t, "ExpectedCostFor(911, ZZ)", ExpectedCostFor(parse(t, "911", "US"), "ZZ"), CostUnknown)
	checkCost(t, "ExpectedCost(Number{})", ExpectedCost(phonenumber.Number{}), CostUnknown)
}

func TestGetExpectedCostForSharedCountryCallingCode(t *testing.T) {
	// In Australia, 1234 is premium rate, 1194 standard rate and 733 toll
	// free. None is known to be valid on Christmas Island, which shares
	// the country calling code.
	for _, tc := range []struct {
		s    string
		cost Cost
		// shared is the cost across both regions.
		shared Cost
	}{
		{"1234", PremiumRate, PremiumRate},
		{"1194", StandardRate, CostUnknown},
		{"733", TollFree, CostUnknown},
	} {
		national, err := strconv.ParseUint(tc.s, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		n := num(t, 61, national)
		check(t, "Valid(+61 "+tc.s+")", Valid(n), true)
		au, cx := parse(t, tc.s, "AU"), parse(t, tc.s, "CX")
		check(t, "ValidFor("+tc.s+", AU)", ValidFor(au, "AU"), true)
		checkCost(t, "ExpectedCostFor("+tc.s+", AU)", ExpectedCostFor(au, "AU"), tc.cost)
		check(t, "ValidFor("+tc.s+", CX)", ValidFor(cx, "CX"), false)
		checkCost(t, "ExpectedCostFor("+tc.s+", CX)", ExpectedCostFor(cx, "CX"), CostUnknown)
		checkCost(t, "ExpectedCost(+61 "+tc.s+")", ExpectedCost(n), tc.shared)
	}
}

func TestExampleShortNumberPresence(t *testing.T) {
	check(t, `exampleNumber("AD") != ""`, exampleNumber("AD") != "", true)
	check(t, `exampleNumber("FR") != ""`, exampleNumber("FR") != "", true)
	check(t, `exampleNumber("001") == ""`, exampleNumber("001") == "", true)
	check(t, `exampleNumber("") == ""`, exampleNumber("") == "", true)
}

type emergencyCase struct {
	s, region string
	want      bool
}

func checkConnects(t *testing.T, tests []emergencyCase) {
	t.Helper()
	for _, tt := range tests {
		if got := ConnectsToEmergency(tt.s, tt.region); got != tt.want {
			t.Errorf("ConnectsToEmergency(%q, %q) = %v, want %v", tt.s, tt.region, got, tt.want)
		}
	}
}

func checkIsEmergency(t *testing.T, tests []emergencyCase) {
	t.Helper()
	for _, tt := range tests {
		if got := IsEmergency(tt.s, tt.region); got != tt.want {
			t.Errorf("IsEmergency(%q, %q) = %v, want %v", tt.s, tt.region, got, tt.want)
		}
	}
}

func TestConnectsToEmergencyNumber_US(t *testing.T) {
	checkConnects(t, []emergencyCase{{"911", "US", true}, {"112", "US", true}, {"999", "US", false}})
}

func TestConnectsToEmergencyNumberLongNumber_US(t *testing.T) {
	checkConnects(t, []emergencyCase{
		{"9116666666", "US", true}, {"1126666666", "US", true}, {"9996666666", "US", false},
	})
}

func TestConnectsToEmergencyNumberWithFormatting_US(t *testing.T) {
	checkConnects(t, []emergencyCase{{"9-1-1", "US", true}, {"1-1-2", "US", true}, {"9-9-9", "US", false}})
}

func TestConnectsToEmergencyNumberWithPlusSign_US(t *testing.T) {
	checkConnects(t, []emergencyCase{
		{"+911", "US", false}, {"＋911", "US", false}, {" +911", "US", false},
		{"+112", "US", false}, {"+999", "US", false},
	})
}

func TestConnectsToEmergencyNumber_BR(t *testing.T) {
	checkConnects(t, []emergencyCase{{"911", "BR", true}, {"190", "BR", true}, {"999", "BR", false}})
}

func TestConnectsToEmergencyNumberLongNumber_BR(t *testing.T) {
	// Brazilian emergency numbers do not work with digits appended.
	checkConnects(t, []emergencyCase{{"9111", "BR", false}, {"1900", "BR", false}, {"9996", "BR", false}})
}

func TestConnectsToEmergencyNumber_CL(t *testing.T) {
	checkConnects(t, []emergencyCase{{"131", "CL", true}, {"133", "CL", true}})
}

func TestConnectsToEmergencyNumberLongNumber_CL(t *testing.T) {
	// Chilean emergency numbers do not work with digits appended.
	checkConnects(t, []emergencyCase{{"1313", "CL", false}, {"1330", "CL", false}})
}

func TestConnectsToEmergencyNumber_AO(t *testing.T) {
	checkConnects(t, []emergencyCase{
		{"911", "AO", false}, {"222123456", "AO", false}, {"923123456", "AO", false},
	})
}

func TestConnectsToEmergencyNumber_ZW(t *testing.T) {
	checkConnects(t, []emergencyCase{
		{"911", "ZW", false}, {"01312345", "ZW", false}, {"0711234567", "ZW", false},
	})
}

func TestIsEmergencyNumber_US(t *testing.T) {
	checkIsEmergency(t, []emergencyCase{{"911", "US", true}, {"112", "US", true}, {"999", "US", false}})
}

func TestIsEmergencyNumberLongNumber_US(t *testing.T) {
	checkIsEmergency(t, []emergencyCase{
		{"9116666666", "US", false}, {"1126666666", "US", false}, {"9996666666", "US", false},
	})
}

func TestIsEmergencyNumberWithFormatting_US(t *testing.T) {
	checkIsEmergency(t, []emergencyCase{
		{"9-1-1", "US", true}, {"*911", "US", true}, {"1-1-2", "US", true},
		{"*112", "US", true}, {"9-9-9", "US", false}, {"*999", "US", false},
	})
}

func TestIsEmergencyNumberWithPlusSign_US(t *testing.T) {
	checkIsEmergency(t, []emergencyCase{
		{"+911", "US", false}, {"＋911", "US", false}, {" +911", "US", false},
		{"+112", "US", false}, {"+999", "US", false},
	})
}

func TestIsEmergencyNumber_BR(t *testing.T) {
	checkIsEmergency(t, []emergencyCase{{"911", "BR", true}, {"190", "BR", true}, {"999", "BR", false}})
}

func TestIsEmergencyNumberLongNumber_BR(t *testing.T) {
	checkIsEmergency(t, []emergencyCase{{"9111", "BR", false}, {"1900", "BR", false}, {"9996", "BR", false}})
}

func TestIsEmergencyNumber_AO(t *testing.T) {
	checkIsEmergency(t, []emergencyCase{
		{"911", "AO", false}, {"222123456", "AO", false}, {"923123456", "AO", false},
	})
}

func TestIsEmergencyNumber_ZW(t *testing.T) {
	checkIsEmergency(t, []emergencyCase{
		{"911", "ZW", false}, {"01312345", "ZW", false}, {"0711234567", "ZW", false},
	})
}

func TestEmergencyNumberForSharedCountryCallingCode(t *testing.T) {
	// 112 is an emergency number in both Australia and Christmas Island.
	for _, region := range []string{"AU", "CX"} {
		check(t, "IsEmergency(112, "+region+")", IsEmergency("112", region), true)
		n := parse(t, "112", region)
		check(t, "ValidFor(112, "+region+")", ValidFor(n, region), true)
		checkCost(t, "ExpectedCostFor(112, "+region+")", ExpectedCostFor(n, region), TollFree)
	}
	shared := num(t, 61, 112)
	check(t, "Valid(+61 112)", Valid(shared), true)
	checkCost(t, "ExpectedCost(+61 112)", ExpectedCost(shared), TollFree)
}

func TestOverlappingNANPANumber(t *testing.T) {
	// 211 is an emergency number in Barbados, and a toll-free information
	// line in Canada and the US.
	check(t, "IsEmergency(211, BB)", IsEmergency("211", "BB"), true)
	checkCost(t, "ExpectedCostFor(211, BB)", ExpectedCostFor(parse(t, "211", "BB"), "BB"), TollFree)
	check(t, "IsEmergency(211, US)", IsEmergency("211", "US"), false)
	checkCost(t, "ExpectedCostFor(211, US)", ExpectedCostFor(parse(t, "211", "US"), "US"), CostUnknown)
	check(t, "IsEmergency(211, CA)", IsEmergency("211", "CA"), false)
	checkCost(t, "ExpectedCostFor(211, CA)", ExpectedCostFor(parse(t, "211", "CA"), "CA"), TollFree)
}

func TestCountryCallingCodeIsNotIgnored(t *testing.T) {
	// +46 is Sweden's country calling code, and 40404 is a valid short
	// number in the US.
	n := parse(t, "+4640404", "SE")
	check(t, "PossibleFor(+46 40404, US)", PossibleFor(n, "US"), false)
	check(t, "ValidFor(+46 40404, US)", ValidFor(n, "US"), false)
	checkCost(t, "ExpectedCostFor(+46 40404, US)", ExpectedCostFor(n, "US"), CostUnknown)
}
