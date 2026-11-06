package geocoder

import (
	"testing"

	"uradical.io/go/phonenumber"
	"uradical.io/go/phonenumber/carrier"
	"uradical.io/go/phonenumber/timezone"
)

// FuzzLookups checks that describing any parsed number, in any language
// tag, never panics, and that every number gets at least one time zone.
func FuzzLookups(f *testing.F) {
	for _, s := range []string{"+1 650 253 0000", "+44 7400 123456", "+54 9 221 400 0000", "+800 1234 5678", "+86 10 1234 5678"} {
		f.Add(s, "en", "US")
		f.Add(s, "zh_Hant_TW", "")
		f.Add(s, "-_x-", "ZZ")
	}
	f.Fuzz(func(t *testing.T, s, lang, userRegion string) {
		n, err := phonenumber.Parse(s, "")
		if err != nil {
			return
		}
		_ = Description(n, lang)
		_ = DescriptionFrom(n, lang, userRegion)
		_ = carrier.SafeDisplayName(n, lang)
		if len(timezone.ForNumber(n)) == 0 {
			t.Fatalf("no time zones for %v", n)
		}
	})
}
