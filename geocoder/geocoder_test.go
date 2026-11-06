package geocoder

import (
	"testing"

	"uradical.io/go/phonenumber"
)

func mustParse(t *testing.T, s string) phonenumber.Number {
	t.Helper()
	n, err := phonenumber.Parse(s, "")
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// The exported functions use the real data.
func TestExportedFunctions(t *testing.T) {
	london := mustParse(t, "+44 20 7946 0958")
	for _, tt := range []struct{ name, got, want string }{
		{"Description", Description(london, "en"), "London"},
		{"DescriptionFrom GB", DescriptionFrom(london, "en", "gb"), "London"},
		{"DescriptionFrom FR", DescriptionFrom(london, "fr", "FR"), "Royaume-Uni"},
		{"DescriptionForValid", DescriptionForValid(london, "en"), "London"},
		{"DescriptionForValidFrom", DescriptionForValidFrom(london, "de", "US"), "Vereinigtes K\u00F6nigreich"},
		{"DescriptionFrom mobile", DescriptionFrom(mustParse(t, "+33 6 12 34 56 78"), "en", "FR"), "France"},
	} {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

func TestRegionName(t *testing.T) {
	g := defaultGeocoder()
	for _, tt := range []struct{ region, tag, want string }{
		{"DE", "de", "Deutschland"},
		{"DE", "de-AT", "Deutschland"},
		// Traditional Chinese where it is written.
		{"US", "zh-TW", "\u7F8E\u570B"},
		{"US", "zh_Hant", "\u7F8E\u570B"},
		{"US", "zh-CN", "\u7F8E\u56FD"},
		// Old and new codes for Hebrew, and Norwegian Bokmal.
		{"DE", "iw", "\u05D2\u05E8\u05DE\u05E0\u05D9\u05D4"},
		{"DE", "he", "\u05D2\u05E8\u05DE\u05E0\u05D9\u05D4"},
		{"DE", "nb", "Tyskland"},
		// Cantonese in mainland China is written in simplified characters.
		{"US", "yue-CN", "\u7F8E\u56FD"},
		// A language with no names falls back to English.
		{"DE", "xx", "Germany"},
		{"DE", "", "Germany"},
		// A region with no name gives its code.
		{"QQ", "en", "QQ"},
		{"ZZ", "en", ""},
		{"001", "en", ""},
		{"", "en", ""},
	} {
		if got := g.regionName(tt.region, tt.tag); got != tt.want {
			t.Errorf("regionName(%s, %q) = %q, want %q", tt.region, tt.tag, got, tt.want)
		}
	}
}

// A number with no region is not the user's own.
func TestForValidFromNoRegion(t *testing.T) {
	if got := testGeocoder(t).forValidFrom(phonenumber.Number{}, "en", ""); got != "" {
		t.Errorf("forValidFrom(Number{}) = %q, want \"\"", got)
	}
}
