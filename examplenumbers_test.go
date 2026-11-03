package phonenumber

import (
	"maps"
	"slices"
	"strconv"
	"testing"
)

// These tests port upstream's ExampleNumbersTest. They run against the
// real metadata and check that it agrees with the code: every example number
// must parse, be valid and have the type it is an example of.

var exampleTypes = []struct {
	typ  Type
	want []Type
}{
	{FixedLine, []Type{FixedLine, FixedLineOrMobile}},
	{Mobile, []Type{Mobile, FixedLineOrMobile}},
	{TollFree, []Type{TollFree}},
	{PremiumRate, []Type{PremiumRate}},
	{SharedCost, []Type{SharedCost}},
	{VoIP, []Type{VoIP}},
	{PersonalNumber, []Type{PersonalNumber}},
	{Pager, []Type{Pager}},
	{UAN, []Type{UAN}},
	{Voicemail, []Type{Voicemail}},
}

func TestExampleNumbersValidAndCorrectType(t *testing.T) {
	ms := defaultSet()
	for _, et := range exampleTypes {
		t.Run(et.typ.String(), func(t *testing.T) {
			for _, region := range Regions() {
				ex := ms.region(region).descFor(et.typ).example
				if ex == "" {
					continue
				}
				n, err := ms.parse(ex, region)
				if err != nil {
					t.Errorf("%s: parse(%q): %v", region, ex, err)
					continue
				}
				if !ms.valid(n) {
					t.Errorf("%s: example %v is not valid", region, n)
					continue
				}
				if got := ms.numberType(n); !slices.Contains(et.want, got) {
					t.Errorf("%s: example %v has type %v, want one of %v", region, n, got, et.want)
				}
			}
		})
	}
}

func TestEveryRegionHasAnExampleNumber(t *testing.T) {
	for _, region := range Regions() {
		if _, ok := ExampleNumber(region, FixedLine); !ok {
			t.Errorf("no example number for %s", region)
		}
	}
}

func TestGlobalNetworkNumbers(t *testing.T) {
	ms := defaultSet()
	for _, cc := range slices.Sorted(maps.Keys(ms.nonGeo)) {
		tr := ms.nonGeo[cc]
		var found bool
		for _, d := range []*numberDesc{&tr.mobile, &tr.tollFree, &tr.sharedCost, &tr.voip, &tr.voicemail, &tr.uan, &tr.premiumRate} {
			if d.example == "" {
				continue
			}
			found = true
			n, err := Parse("+"+strconv.Itoa(cc)+d.example, "")
			if err != nil {
				t.Errorf("+%d: parse example %q: %v", cc, d.example, err)
				continue
			}
			if !n.Valid() {
				t.Errorf("+%d: example %v is not valid", cc, n)
			}
			if got := n.Region(); got != regionNonGeo {
				t.Errorf("+%d: example %v has region %q, want %q", cc, n, got, regionNonGeo)
			}
			break
		}
		if !found {
			t.Errorf("+%d: no example number", cc)
		}
	}
}

func TestCanDialInternationallyExamples(t *testing.T) {
	ms := defaultSet()
	for _, region := range Regions() {
		ex := ms.region(region).noInternationalDialling.example
		if ex == "" {
			continue
		}
		n, err := ms.parse(ex, region)
		if err != nil {
			continue // as upstream: unparseable examples are skipped
		}
		if ms.canDialInternationally(n) {
			t.Errorf("%s: %v should not be diallable internationally", region, n)
		}
	}
}

// TestExampleNumbersRoundTrip checks that every example number, written in
// each format, parses back to the same Number. Upstream has no such test; it
// guards the formatting and parsing code against each other.
func TestExampleNumbersRoundTrip(t *testing.T) {
	ms := defaultSet()
	for _, region := range Regions() {
		tr := ms.region(region)
		for _, et := range exampleTypes {
			ex := tr.descFor(et.typ).example
			if ex == "" {
				continue
			}
			n, err := ms.parse(ex, region)
			if err != nil {
				t.Errorf("%s: parse(%q): %v", region, ex, err)
				continue
			}
			for _, f := range []Format{E164, International, RFC3966, National} {
				s := ms.format(n, f)
				got, err := ms.parse(s, region)
				if err != nil {
					t.Errorf("%s %v: parse(%q) of %v format: %v", region, et.typ, s, f, err)
					continue
				}
				if got != n {
					t.Errorf("%s %v: %v format %q parses to %v, want %v", region, et.typ, f, s, got, n)
				}
			}
		}
	}
}
