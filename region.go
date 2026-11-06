package phonenumber

import (
	"maps"
	"slices"
	"strconv"
	"strings"
)

// Regions returns the region codes this package has numbering plans for,
// sorted. It does not include "001", the code for non-geographical numbers.
func Regions() []string {
	return slices.Sorted(maps.Keys(defaultSet().regions))
}

// CountryCodeForRegion returns the country calling code for a region code,
// such as 44 for "GB", or 0 if the region is unknown.
func CountryCodeForRegion(region string) int {
	if t := defaultSet().region(strings.ToUpper(region)); t != nil {
		return t.countryCode
	}
	return 0
}

// RegionsForCountryCode returns the regions that use a country calling code,
// main region first: "US" leads the regions of +1. It returns ["001"] for a
// non-geographical code such as 800, and nil for an unknown one.
func RegionsForCountryCode(cc int) []string {
	return slices.Clone(defaultSet().regionsForCode[cc])
}

// NationalPrefix returns the prefix dialled before a national number within
// a region, such as "0" for "GB", or "" if the region has none or is
// unknown.
func NationalPrefix(region string) string {
	return defaultSet().nationalPrefix(strings.ToUpper(region))
}

// ExampleNumber returns an example number of type typ for a region. It
// reports false when the region is unknown or has no example of that type.
// As upstream, [FixedLineOrMobile] gives the fixed-line example, [TypeUnknown]
// gives none, and "001" is not a region here: use [ExampleNumberForNonGeo]
// for non-geographical numbers.
func ExampleNumber(region string, typ Type) (Number, bool) {
	return defaultSet().exampleNumber(strings.ToUpper(region), typ)
}

// ExampleNumberForType returns an example number of type typ from any
// region, trying the regions in sorted order and then the non-geographical
// country calling codes in ascending order. It reports false when no region
// or code has an example of that type.
func ExampleNumberForType(typ Type) (Number, bool) {
	return defaultSet().exampleForType(typ)
}

// ExampleNumberForNonGeo returns an example number for a non-geographical
// country calling code, such as 800 for international freephone. It reports
// false when cc is not a non-geographical code.
func ExampleNumberForNonGeo(cc int) (Number, bool) {
	return defaultSet().exampleNonGeo(cc)
}

func (ms *metadataSet) exampleNumber(region string, typ Type) (Number, bool) {
	t := ms.region(region)
	if t == nil {
		return Number{}, false
	}
	ex := t.descFor(typ).example
	if ex == "" {
		return Number{}, false
	}
	n, err := ms.parse(ex, region)
	return n, err == nil
}

func (ms *metadataSet) exampleForType(typ Type) (Number, bool) {
	for _, region := range slices.Sorted(maps.Keys(ms.regions)) {
		if n, ok := ms.exampleNumber(region, typ); ok {
			return n, true
		}
	}
	for _, cc := range slices.Sorted(maps.Keys(ms.nonGeo)) {
		if n, ok := ms.parseNonGeoExample(cc, ms.nonGeo[cc].descFor(typ)); ok {
			return n, true
		}
	}
	return Number{}, false
}

// exampleNonGeo returns the first example a non-geographical code has,
// checking the types in upstream's order. Fixed-line and personal numbers
// are not checked, as no non-geographical code uses them.
func (ms *metadataSet) exampleNonGeo(cc int) (Number, bool) {
	t := ms.nonGeo[cc]
	if t == nil {
		return Number{}, false
	}
	for _, d := range []*numberDesc{&t.mobile, &t.tollFree, &t.sharedCost, &t.voip, &t.voicemail, &t.uan, &t.premiumRate} {
		if n, ok := ms.parseNonGeoExample(cc, d); ok {
			return n, true
		}
	}
	return Number{}, false
}

func (ms *metadataSet) parseNonGeoExample(cc int, d *numberDesc) (Number, bool) {
	if d.example == "" {
		return Number{}, false
	}
	n, err := ms.parse("+"+strconv.Itoa(cc)+d.example, "")
	return n, err == nil
}

// SupportedTypes returns the types of number a region's numbering plan
// defines, or nil for an unknown region.
func SupportedTypes(region string) []Type {
	t := defaultSet().region(strings.ToUpper(region))
	if t == nil {
		return nil
	}
	return t.supportedTypes()
}

func (t *territory) supportedTypes() []Type {
	var types []Type
	for typ := FixedLine; typ <= Voicemail; typ++ {
		if typ == FixedLineOrMobile {
			continue
		}
		if t.descFor(typ).hasData() {
			types = append(types, typ)
		}
	}
	return types
}

// CountryCodes returns every country calling code this package has
// numbering plans for, geographical or not, sorted.
func CountryCodes() []int {
	return slices.Sorted(maps.Keys(defaultSet().regionsForCode))
}

// NonGeoCountryCodes returns the country calling codes of
// non-geographical entities, such as 800 for international freephone,
// sorted.
func NonGeoCountryCodes() []int {
	return slices.Sorted(maps.Keys(defaultSet().nonGeo))
}

// SupportedTypesForNonGeo returns the number types a non-geographical
// country calling code has, or nil if cc is not one.
func SupportedTypesForNonGeo(cc int) []Type {
	t := defaultSet().nonGeo[cc]
	if t == nil {
		return nil
	}
	return t.supportedTypes()
}

// IsNANPA reports whether region is in the North American Numbering Plan,
// the regions that share country calling code 1.
func IsNANPA(region string) bool {
	return defaultSet().isNANPA(strings.ToUpper(region))
}
