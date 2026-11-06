package shortnumber

import (
	"testing"

	"uradical.io/go/phonenumber"
)

func TestCostString(t *testing.T) {
	for c, want := range map[Cost]string{
		CostUnknown: "CostUnknown",
		PremiumRate: "PremiumRate",
		Cost(-1):    "Cost(-1)",
		Cost(9):     "Cost(9)",
	} {
		if got := c.String(); got != want {
			t.Errorf("Cost(%d).String() = %q, want %q", int(c), got, want)
		}
	}
}

func TestRegionsAreCaseInsensitive(t *testing.T) {
	n := parse(t, "1010", "FR")
	check(t, "ValidFor(1010, fr)", ValidFor(n, "fr"), true)
	check(t, "IsEmergency(911, us)", IsEmergency("911", "us"), true)
	check(t, "ConnectsToEmergency(1313, cl)", ConnectsToEmergency("1313", "cl"), false)
}

// A country calling code can belong to a region with no short-number
// metadata, or to a non-geographical entity, which has none.
func TestNoMetadata(t *testing.T) {
	n := parse(t, "+800 1234 5678", "")
	check(t, "Possible(+800)", Possible(n), false)
	check(t, "Valid(+800)", Valid(n), false)
	check(t, "PossibleFor(+800, 001)", PossibleFor(n, "001"), false)
	check(t, "ValidFor(+800, 001)", ValidFor(n, "001"), false)
	checkCost(t, "ExpectedCost(+800)", ExpectedCost(n), CostUnknown)
	check(t, "CarrierSpecific(+800)", CarrierSpecific(n), false)
	check(t, "CarrierSpecificFor(+800, 001)", CarrierSpecificFor(n, "001"), false)
	check(t, "SMSServiceFor(+800, 001)", SMSServiceFor(n, "001"), false)
	check(t, "IsEmergency(112, 001)", IsEmergency("112", "001"), false)

	fr := parse(t, "1010", "FR")
	check(t, "PossibleFor(1010, DE)", PossibleFor(fr, "DE"), false)
	check(t, "CarrierSpecificFor(1010, DE)", CarrierSpecificFor(fr, "DE"), false)
	check(t, "SMSServiceFor(1010, DE)", SMSServiceFor(fr, "DE"), false)
}

func TestZeroNumber(t *testing.T) {
	var n phonenumber.Number
	check(t, "Valid(Number{})", Valid(n), false)
	check(t, "CarrierSpecific(Number{})", CarrierSpecific(n), false)
}

// A shared country calling code where the number is valid in no region.
func TestValidSharedCodeNoRegion(t *testing.T) {
	check(t, "Valid(+44 99999)", Valid(parse(t, "+44 99999", "")), false)
}

// GN is the one region with short-number metadata but no emergency
// numbers.
func TestRegionWithoutEmergencyNumbers(t *testing.T) {
	check(t, "IsEmergency(112, GN)", IsEmergency("112", "GN"), false)
}

func TestShortNumberLengthOutsideRegion(t *testing.T) {
	// FR short numbers have two to six digits.
	n := parse(t, "1234567", "FR")
	checkCost(t, "ExpectedCostFor(1234567, FR)", ExpectedCostFor(n, "FR"), CostUnknown)
	check(t, "Possible(+33 1234567)", Possible(n), false)
}

func TestExampleForCost(t *testing.T) {
	if got := exampleForCost("FR", CostUnknown); got != "" {
		t.Errorf("exampleForCost(FR, CostUnknown) = %q, want \"\"", got)
	}
	if got := exampleForCost("001", TollFree); got != "" {
		t.Errorf("exampleForCost(001, TollFree) = %q, want \"\"", got)
	}
}

// Every example number in the metadata must be valid in its region, and
// match its cost category.
func TestMetadataExamples(t *testing.T) {
	for region, tr := range regions() {
		if ex := tr.shortCode.example; ex != "" {
			if n, err := phonenumber.Parse(ex, region); err == nil && !ValidFor(n, region) {
				t.Errorf("%s: example short code %s is not valid", region, ex)
			}
		}
		for _, c := range []Cost{TollFree, StandardRate, PremiumRate} {
			ex := exampleForCost(region, c)
			if ex == "" {
				continue
			}
			n, err := phonenumber.Parse(ex, region)
			if err != nil {
				continue // one-digit numbers do not parse
			}
			if got := ExpectedCostFor(n, region); got != c {
				t.Errorf("%s: example %v number %s has cost %v", region, c, ex, got)
			}
		}
	}
}

func FuzzEmergency(f *testing.F) {
	for _, s := range []string{"911", "+911", "9-1-1", "*112", "9116666666", "", "x"} {
		f.Add(s, "US")
		f.Add(s, "BR")
	}
	f.Fuzz(func(t *testing.T, s, region string) {
		if IsEmergency(s, region) && !ConnectsToEmergency(s, region) {
			t.Fatalf("IsEmergency(%q, %q) but not ConnectsToEmergency", s, region)
		}
	})
}
