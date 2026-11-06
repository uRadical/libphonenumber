// Package timezone gives the time zones a phone number may be in, as IANA
// names such as "America/New_York". It is the Go port of upstream
// libphonenumber's PhoneNumberToTimeZonesMapper.
//
//	n, _ := phonenumber.Parse("+1 212 812 0000", "")
//	timezone.ForNumber(n) // ["America/New_York"]
//
// A number that is not tied to a place, such as a mobile in most
// countries, gets every time zone of its country.
package timezone

import (
	"embed"
	"strconv"
	"strings"
	"sync"

	"uradical.io/go/phonenumber"
	"uradical.io/go/phonenumber/internal/prefixmap"
)

// Unknown is the time zone returned for a number with no known time zone,
// as ICU names it.
const Unknown = "Etc/Unknown"

var (
	//go:embed data/map_data.gz
	dataFS embed.FS

	defaultTable = sync.OnceValue(func() prefixmap.Table {
		t, _ := prefixmap.ReadFile(dataFS, "data/map_data.gz")
		return t
	})
)

// ForNumber returns the time zones of n: those of its area for a
// geographical number, else those of its country, or [Unknown] for an
// invalid number or one with no data.
func ForNumber(n phonenumber.Number) []string { return forNumber(defaultTable(), n) }

// ForGeographicalNumber returns the time zones of n's area, falling back
// to its country's, or [Unknown]. It assumes n is geographical, and does
// not check that it is valid.
func ForGeographicalNumber(n phonenumber.Number) []string {
	return forGeographical(defaultTable(), n)
}

func forNumber(t prefixmap.Table, n phonenumber.Number) []string {
	switch {
	case n.Type() == phonenumber.TypeUnknown:
		return []string{Unknown}
	case !n.Geographical():
		return lookup(t, strconv.Itoa(n.CountryCode()))
	}
	return forGeographical(t, n)
}

func forGeographical(t prefixmap.Table, n phonenumber.Number) []string {
	return lookup(t, strconv.Itoa(n.CountryCode())+n.NationalNumber())
}

// lookup returns the time zones for the longest prefix of digits, which
// the data separates with "&", or [Unknown].
func lookup(t prefixmap.Table, digits string) []string {
	zones, ok := t.Lookup(digits)
	if !ok || zones == "" {
		return []string{Unknown}
	}
	return strings.FieldsFunc(zones, func(r rune) bool { return r == '&' })
}
