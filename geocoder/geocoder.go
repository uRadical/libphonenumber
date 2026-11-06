// Package geocoder describes where a phone number is: a city or area such
// as "Mountain View, CA" where the data has one, and otherwise the name of
// its country, in the language asked for. It is the Go port of upstream
// libphonenumber's PhoneNumberOfflineGeocoder.
//
//	n, _ := phonenumber.Parse("+1 650 253 0000", "")
//	geocoder.Description(n, "en") // "Mountain View, CA"
//
//	m, _ := phonenumber.Parse("+33 6 12 34 56 78", "") // a mobile
//	geocoder.Description(m, "de") // "Frankreich"
//
// Languages are tags such as "en", "de-CH" or "zh-TW" ("zh_TW" works too).
// Area data comes in about 35 languages and falls back to English, except
// for Chinese, Japanese and Korean. Country names come from Unicode CLDR
// for the about 100 languages it covers fully, and fall back to English.
// As upstream does, the script of a tag is ignored for area data: use
// "zh-TW", not "zh-Hant", for traditional Chinese.
//
// The data adds about 2 MB to a program that imports this package.
// Each country's data is decompressed the first time a number from it is
// described.
package geocoder

import (
	"embed"
	"io/fs"
	"strings"
	"sync"

	"uradical.io/go/phonenumber"
	"uradical.io/go/phonenumber/internal/prefixmap"
)

var (
	//go:embed data/*.gz
	dataFS embed.FS
	//go:embed names.gz
	namesFS embed.FS

	defaultGeocoder = sync.OnceValue(func() *geocoder {
		data, err := fs.Sub(dataFS, "data")
		if err != nil {
			panic(err)
		}
		return newGeocoder(data, namesFS, "names.gz")
	})
)

// geocoder holds the data a description comes from, so that tests can use
// upstream's test data.
type geocoder struct {
	areas *prefixmap.Files
	names func() prefixmap.Table
}

func newGeocoder(areas, names fs.FS, namesFile string) *geocoder {
	return &geocoder{
		areas: prefixmap.NewFiles(areas),
		names: sync.OnceValue(func() prefixmap.Table {
			t, _ := prefixmap.ReadFile(names, namesFile)
			return t
		}),
	}
}

// Description returns where n is, in language lang: an area for a
// geographical number, else its country, or "" for an invalid number or
// one shared by several countries.
func Description(n phonenumber.Number, lang string) string {
	return defaultGeocoder().description(n, lang, "", false)
}

// DescriptionFrom is [Description] for a user in region userRegion: a
// number from elsewhere is described only by its country.
func DescriptionFrom(n phonenumber.Number, lang, userRegion string) string {
	return defaultGeocoder().description(n, lang, strings.ToUpper(userRegion), true)
}

// DescriptionForValid is [Description] for a number known to be valid,
// skipping the check. An invalid number may give a wrong description.
func DescriptionForValid(n phonenumber.Number, lang string) string {
	return defaultGeocoder().forValid(n, lang)
}

// DescriptionForValidFrom is [DescriptionFrom] for a number known to be
// valid, skipping the check.
func DescriptionForValidFrom(n phonenumber.Number, lang, userRegion string) string {
	return defaultGeocoder().forValidFrom(n, lang, strings.ToUpper(userRegion))
}

func (g *geocoder) description(n phonenumber.Number, lang, userRegion string, withUser bool) string {
	switch {
	case n.Type() == phonenumber.TypeUnknown:
		return ""
	case !n.Geographical():
		return g.countryName(n, lang)
	case withUser:
		return g.forValidFrom(n, lang, userRegion)
	}
	return g.forValid(n, lang)
}

func (g *geocoder) forValid(n phonenumber.Number, tag string) string {
	lang, _, region := prefixmap.ParseLanguage(tag)
	cc := n.CountryCode()
	nsn := n.NationalNumber()
	// Some countries, such as Argentina, put a mobile token before the
	// area code of mobile numbers; it is not part of the area.
	if token := phonenumber.CountryMobileToken(cc); token != "" && strings.HasPrefix(nsn, token) {
		rs := phonenumber.RegionsForCountryCode(cc)
		if m, err := phonenumber.Parse(nsn[len(token):], rs[0]); err == nil {
			nsn = m.NationalNumber()
			cc = m.CountryCode()
		}
	}
	// Upstream passes no script.
	if d := g.areas.Description(cc, nsn, lang, "", region); d != "" {
		return d
	}
	return g.countryName(n, tag)
}

func (g *geocoder) forValidFrom(n phonenumber.Number, lang, userRegion string) string {
	// For a number from the user's own region, show the area; otherwise
	// only the country.
	region := n.Region()
	if region != "" && userRegion == region {
		return g.forValid(n, lang)
	}
	return g.regionName(region, lang)
}

// countryName returns the name of n's country, or "" when several
// countries share its country calling code and it is valid in more than
// one or none of them.
func (g *geocoder) countryName(n phonenumber.Number, lang string) string {
	rs := phonenumber.RegionsForCountryCode(n.CountryCode())
	if len(rs) == 1 {
		return g.regionName(rs[0], lang)
	}
	valid := ""
	for _, r := range rs {
		if n.ValidFor(r) {
			if valid != "" {
				return ""
			}
			valid = r
		}
	}
	return g.regionName(valid, lang)
}

// regionName returns the name of a region in a language, falling back to
// English, as Java's Locale.getDisplayCountry does.
func (g *geocoder) regionName(region, tag string) string {
	if region == "" || region == "ZZ" || region == "001" {
		return ""
	}
	names := g.names()
	for _, loc := range cldrLocales(tag) {
		if name, ok := names[loc+"/"+region]; ok {
			return name
		}
	}
	return region
}

// cldrLocales returns the CLDR locales to try for a language tag, most
// specific first, ending with English.
func cldrLocales(tag string) []string {
	lang, script, region := prefixmap.ParseLanguage(tag)
	switch lang {
	case "iw":
		lang = "he"
	case "nb":
		lang = "no"
	}
	if script == "" {
		// The regions that write Chinese in traditional characters.
		switch {
		case lang == "zh" && (region == "TW" || region == "HK" || region == "MO"):
			script = "Hant"
		case lang == "yue" && region == "CN":
			script = "Hans"
		}
	}
	var locs []string
	if script != "" {
		locs = append(locs, lang+"-"+script)
	}
	if lang != "" {
		locs = append(locs, lang)
	}
	return append(locs, "en")
}
