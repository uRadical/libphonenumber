package phonenumber

import (
	"errors"
	"strconv"
	"strings"

	"uradical.io/go/phonenumber/internal/phonetext"
)

// MatchType is how closely two phone numbers match; see [Number.Match].
// Larger values are closer matches, so match >= MatchNSN reads as "the
// same number, though perhaps not known to be in the same country".
type MatchType int

const (
	// MatchNotANumber means at least one of the numbers could not be
	// parsed, or is the zero Number.
	MatchNotANumber MatchType = iota
	// MatchNone means the numbers are different.
	MatchNone
	// MatchShortNSN means one national number could be a shorter form of
	// the other, as 331 6005 is of +64 3 331 6005. It also covers numbers
	// that differ only in that one has an extension and the other has
	// none, or one has an Italian leading zero and the other has none.
	MatchShortNSN
	// MatchNSN means the national numbers and extensions are the same, but
	// at least one number has no country calling code, so they may be in
	// different countries.
	MatchNSN
	// MatchExact means the country calling codes, national numbers,
	// leading zeros and extensions are all the same.
	MatchExact
)

var matchTypeNames = [...]string{
	MatchNotANumber: "MatchNotANumber",
	MatchNone:       "MatchNone",
	MatchShortNSN:   "MatchShortNSN",
	MatchNSN:        "MatchNSN",
	MatchExact:      "MatchExact",
}

func (m MatchType) String() string {
	if m >= 0 && int(m) < len(matchTypeNames) {
		return matchTypeNames[m]
	}
	return "MatchType(" + strconv.Itoa(int(m)) + ")"
}

// Match reports how closely n matches other. Both have country calling
// codes, so the result is [MatchExact], [MatchShortNSN] or [MatchNone],
// or [MatchNotANumber] when either is the zero Number. For two Numbers,
// MatchExact is the same as n == other.
func (n Number) Match(other Number) MatchType {
	if n.IsZero() || other.IsZero() {
		return MatchNotANumber
	}
	return matchNumbers(n, other)
}

// MatchString reports how closely n matches the number written in s. When
// s has no country calling code, it is read as a number in n's country,
// and the best result is [MatchNSN] rather than [MatchExact].
func (n Number) MatchString(s string) MatchType { return defaultSet().matchString(n, s) }

// MatchStrings reports how closely the numbers written in a and b match.
// Neither has a default region: a number without a country calling code
// is compared by its national number alone, and nothing is taken to be a
// national prefix, so "03 331 6005" and "3 331 6005" are only a
// [MatchShortNSN]. When exactly one has a country calling code, the other
// is read as a number in that country, as [Number.MatchString] does.
func MatchStrings(a, b string) MatchType { return defaultSet().matchStrings(a, b) }

// matchNumbers is upstream's isNumberMatch for two numbers. A country
// calling code of 0 means the number has none, which only parseLoose
// produces.
func matchNumbers(a, b Number) MatchType {
	if a.ext != "" && b.ext != "" && a.ext != b.ext {
		return MatchNone
	}
	if a.countryCode != 0 && b.countryCode != 0 {
		if a == b {
			return MatchExact
		}
		if a.countryCode == b.countryCode && nationalSuffix(a, b) {
			return MatchShortNSN
		}
		return MatchNone
	}
	a.countryCode = b.countryCode
	if a == b {
		return MatchNSN
	}
	if nationalSuffix(a, b) {
		return MatchShortNSN
	}
	return MatchNone
}

// nationalSuffix reports whether one national number, without its leading
// zeros, ends with the other.
func nationalSuffix(a, b Number) bool {
	x := strconv.FormatUint(a.national, 10)
	y := strconv.FormatUint(b.national, 10)
	return strings.HasSuffix(x, y) || strings.HasSuffix(y, x)
}

// matchString is upstream's isNumberMatch for a number and a string.
func (ms *metadataSet) matchString(n Number, s string) MatchType {
	if n.IsZero() {
		return MatchNotANumber
	}
	other, err := ms.parse(s, regionUnknown)
	if err == nil {
		return matchNumbers(n, other)
	}
	if !errors.Is(err, ErrInvalidCountryCode) {
		return MatchNotANumber
	}
	// s has no country calling code, so an exact match is impossible.
	region := ms.mainRegion(int(n.countryCode))
	if region == regionUnknown {
		if other, err = ms.parseLoose(s); err != nil {
			return MatchNotANumber
		}
		return matchNumbers(n, other)
	}
	if other, err = ms.parse(s, region); err != nil {
		return MatchNotANumber
	}
	return min(matchNumbers(n, other), MatchNSN)
}

// matchStrings is upstream's isNumberMatch for two strings.
func (ms *metadataSet) matchStrings(a, b string) MatchType {
	first, err := ms.parse(a, regionUnknown)
	if err == nil {
		return ms.matchString(first, b)
	}
	if !errors.Is(err, ErrInvalidCountryCode) {
		return MatchNotANumber
	}
	second, err := ms.parse(b, regionUnknown)
	if err == nil {
		return ms.matchString(second, a)
	}
	if !errors.Is(err, ErrInvalidCountryCode) {
		return MatchNotANumber
	}
	if first, err = ms.parseLoose(a); err != nil {
		return MatchNotANumber
	}
	if second, err = ms.parseLoose(b); err != nil {
		return MatchNotANumber
	}
	return matchNumbers(first, second)
}

// parseLoose parses s with no default region, as upstream's parseHelper
// does for isNumberMatch. A number without a plus sign gets country calling
// code 0 and keeps any national prefix, since there is no region to say
// what one looks like.
func (ms *metadataSet) parseLoose(s string) (Number, error) {
	number, err := extractNumber(s)
	if err != nil {
		return Number{}, err
	}
	if phonetext.PlusCharsPattern().MatchString(number) {
		return ms.parse(s, regionUnknown)
	}
	number, ext := stripExtension(number)
	return newNationalNumber(normalize(number), ext)
}
