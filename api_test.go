package phonenumber

import (
	"slices"
	"testing"
)

// These tests exercise the exported API against the real metadata. The
// upstream ports test the same logic in depth against the test metadata.

func TestNumberAccessors(t *testing.T) {
	n, err := Parse("02 3661 8300 ext. 7", "it")
	if err != nil {
		t.Fatal(err)
	}
	if got := n.CountryCode(); got != 39 {
		t.Errorf("CountryCode() = %d, want 39", got)
	}
	if got := n.NationalNumber(); got != "0236618300" {
		t.Errorf("NationalNumber() = %q, want 0236618300", got)
	}
	if got := n.Extension(); got != "7" {
		t.Errorf("Extension() = %q, want 7", got)
	}
	if got := (Number{}).NationalNumber(); got != "" {
		t.Errorf("zero NationalNumber() = %q, want \"\"", got)
	}
}

func TestNumberChecks(t *testing.T) {
	gb, _ := Parse("+44 20 7946 0958", "")
	us, _ := Parse("+1 650 253 0000", "")
	short, _ := Parse("+44 20 7946", "")
	for _, tc := range []struct {
		name string
		got  any
		want any
	}{
		{"gb Valid", gb.Valid(), true},
		{"gb ValidFor GB", gb.ValidFor("gb"), true},
		{"gb ValidFor US", gb.ValidFor("US"), false},
		{"us ValidFor CA", us.ValidFor("CA"), false},
		{"gb Possible", gb.Possible(), true},
		{"gb CheckLength", gb.CheckLength(), LengthOK},
		{"gb CheckLengthForType Mobile", gb.CheckLengthForType(Mobile), LengthOK},
		{"short Possible", short.Possible(), true},
		{"short Valid", short.Valid(), false},
		{"short CheckLength", short.CheckLength(), LengthLocalOnly},
		{"gb CanDialInternationally", gb.CanDialInternationally(), true},
		{"unknown code CheckLength", num(999, 1234).CheckLength(), LengthInvalidCountryCode},
		{"unknown code Region", num(999, 1234).Region(), ""},
		{"unknown code Type", num(999, 1234).Type(), TypeUnknown},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

func TestRegionFunctions(t *testing.T) {
	regions := Regions()
	if len(regions) < 200 || !slices.IsSorted(regions) || slices.Contains(regions, regionNonGeo) {
		t.Errorf("Regions() = %d regions, sorted %v, contains 001 %v", len(regions), slices.IsSorted(regions), slices.Contains(regions, regionNonGeo))
	}
	if got := CountryCodeForRegion("gb"); got != 44 {
		t.Errorf("CountryCodeForRegion(gb) = %d, want 44", got)
	}
	if got := CountryCodeForRegion("XX"); got != 0 {
		t.Errorf("CountryCodeForRegion(XX) = %d, want 0", got)
	}
	us := RegionsForCountryCode(1)
	if len(us) < 2 || us[0] != "US" || !slices.Contains(us, "CA") {
		t.Errorf("RegionsForCountryCode(1) = %v, want US first and CA included", us)
	}
	us[0] = "changed"
	if RegionsForCountryCode(1)[0] != "US" {
		t.Error("RegionsForCountryCode returned shared storage")
	}
	if got := RegionsForCountryCode(800); !slices.Equal(got, []string{regionNonGeo}) {
		t.Errorf("RegionsForCountryCode(800) = %v, want [001]", got)
	}
	if got := RegionsForCountryCode(999); got != nil {
		t.Errorf("RegionsForCountryCode(999) = %v, want nil", got)
	}
	for region, want := range map[string]string{"GB": "0", "us": "1", "BY": "8", "XX": ""} {
		if got := NationalPrefix(region); got != want {
			t.Errorf("NationalPrefix(%q) = %q, want %q", region, got, want)
		}
	}
}

func TestExampleNumberAPI(t *testing.T) {
	n, ok := ExampleNumber("gb", Mobile)
	if !ok || n.Type() != Mobile || n.Region() != "GB" {
		t.Errorf("ExampleNumber(gb, Mobile) = %v, %v; type %v region %q", n, ok, n.Type(), n.Region())
	}
	if _, ok := ExampleNumber("GB", TypeUnknown); ok {
		t.Error("ExampleNumber(GB, TypeUnknown) reported an example; upstream gives none")
	}
	if _, ok := ExampleNumber("GB", Voicemail); ok {
		t.Error("ExampleNumber(GB, Voicemail) reported an example; GB has none")
	}
	if _, ok := ExampleNumber("XX", FixedLine); ok {
		t.Error("ExampleNumber(XX) reported an example for an unknown region")
	}
	if _, ok := ExampleNumber("001", TollFree); ok {
		t.Error("ExampleNumber(001, TollFree) reported an example; upstream gives none for 001")
	}
	if n, ok := ExampleNumberForNonGeo(800); !ok || n.Region() != regionNonGeo || n.Type() != TollFree {
		t.Errorf("ExampleNumberForNonGeo(800) = %v, %v; type %v region %q", n, ok, n.Type(), n.Region())
	}
	if _, ok := ExampleNumberForNonGeo(44); ok {
		t.Error("ExampleNumberForNonGeo(44) reported an example; 44 is geographical")
	}
	// Every type but TypeUnknown has an example somewhere in the real metadata.
	for typ := FixedLine; typ <= Voicemail; typ++ {
		if n, ok := ExampleNumberForType(typ); !ok || !n.Valid() {
			t.Errorf("ExampleNumberForType(%v) = %v, %v", typ, n, ok)
		}
	}
	if _, ok := ExampleNumberForType(TypeUnknown); ok {
		t.Error("ExampleNumberForType(TypeUnknown) reported an example")
	}
}

func TestSupportedTypes(t *testing.T) {
	got := SupportedTypes("gb")
	for _, want := range []Type{FixedLine, Mobile, TollFree, PremiumRate} {
		if !slices.Contains(got, want) {
			t.Errorf("SupportedTypes(gb) = %v, missing %v", got, want)
		}
	}
	if slices.Contains(got, FixedLineOrMobile) || slices.Contains(got, TypeUnknown) {
		t.Errorf("SupportedTypes(gb) = %v, includes a type that is not a kind of number", got)
	}
	if got := SupportedTypes("XX"); got != nil {
		t.Errorf("SupportedTypes(XX) = %v, want nil", got)
	}
}

func TestEnumStrings(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{TypeUnknown.String(), "TypeUnknown"},
		{Voicemail.String(), "Voicemail"},
		{Type(-1).String(), "Type(-1)"},
		{Type(99).String(), "Type(99)"},
		{LengthOK.String(), "LengthOK"},
		{LengthTooLong.String(), "LengthTooLong"},
		{LengthCheck(99).String(), "LengthCheck(99)"},
		{E164.String(), "E164"},
		{International.String(), "International"},
		{National.String(), "National"},
		{RFC3966.String(), "RFC3966"},
		{Format(99).String(), "Format(99)"},
	} {
		if tc.got != tc.want {
			t.Errorf("String() = %q, want %q", tc.got, tc.want)
		}
	}
}

func TestFormatEdgeCases(t *testing.T) {
	gb, _ := Parse("+44 20 7946 0958", "")
	if got := (Number{}).FormatFrom("US"); got != "" {
		t.Errorf("zero FormatFrom = %q, want \"\"", got)
	}
	if got, want := gb.FormatFrom("XX"), gb.Format(International); got != want {
		t.Errorf("FormatFrom(XX) = %q, want the international format %q", got, want)
	}
	// An unknown country calling code has no rules: the digits are returned
	// as they are.
	unknown := num(999, 1234567)
	if got := unknown.Format(National); got != "1234567" {
		t.Errorf("unknown code Format(National) = %q, want 1234567", got)
	}
	if got := unknown.FormatFrom("GB"); got != "1234567" {
		t.Errorf("unknown code FormatFrom(GB) = %q, want 1234567", got)
	}
	// South Korea accepts several international prefixes and names none as
	// preferred, so the + form is used.
	if got, want := gb.FormatFrom("KR"), "+44 20 7946 0958"; got != want {
		t.Errorf("FormatFrom(KR) = %q, want %q", got, want)
	}
}
