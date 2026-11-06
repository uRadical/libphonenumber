package phonenumber

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"uradical.io/go/phonenumber/internal/phonetext"
)

// Country calling codes that need special handling when working out area
// codes, as upstream lists them.
var (
	// mobileTokens maps a country calling code to the digit dialled before
	// the area code of a mobile number from abroad.
	mobileTokens = map[int]string{54: "9"} // Argentina
	// geoMobileWithoutAreaCodes lists countries whose mobile numbers are
	// geographical but have no area code.
	geoMobileWithoutAreaCodes = []int{86} // China
	// noNationalPrefixWithAreaCodes lists countries with area codes but no
	// national prefix.
	noNationalPrefixWithAreaCodes = []int{52} // Mexico
	// geoMobileCountries lists countries whose mobile numbers are tied to
	// a place.
	geoMobileCountries = []int{
		52, // Mexico
		54, // Argentina
		55, // Brazil
		62, // Indonesia: some prefixes only (fixed CDMA wireless)
		86, // China
	}
	nonDigits = lazyRegexp(`\D+`)
)

// Geographical reports whether n is tied to a place: a fixed line, or a
// mobile in a country such as Mexico or Argentina where mobile numbers
// have area codes.
func (n Number) Geographical() bool {
	return isGeographical(defaultSet().numberType(n), int(n.countryCode))
}

func isGeographical(typ Type, cc int) bool {
	return typ == FixedLine || typ == FixedLineOrMobile ||
		(typ == Mobile && slices.Contains(geoMobileCountries, cc))
}

// AreaCodeLength returns the length of n's geographical area code, which
// is the start of [Number.NationalNumber], or 0 when it has none:
//
//	nsn := n.NationalNumber()
//	areaCode, subscriber := nsn[:n.AreaCodeLength()], nsn[n.AreaCodeLength():]
//
// Area codes are an ambiguous idea, and upstream recommends the national
// number instead for most purposes. They change over time; mobile phones
// may need the whole national number dialled; most non-geographical
// numbers have none, and some geographical numbers have none either.
func (n Number) AreaCodeLength() int { return defaultSet().areaCodeLength(n) }

func (ms *metadataSet) areaCodeLength(n Number) int {
	meta := ms.region(ms.regionFor(n))
	if meta == nil {
		return 0
	}
	cc := int(n.countryCode)
	// A country with no national prefix has a closed dialling plan with no
	// area codes, apart from the listed exceptions and numbers with an
	// Italian leading zero.
	if meta.nationalPrefix == "" && n.zeros == 0 && !slices.Contains(noNationalPrefixWithAreaCodes, cc) {
		return 0
	}
	typ := ms.numberType(n)
	// A rough rule: Indonesia, for one, has area codes for some mobiles
	// and not others.
	if typ == Mobile && slices.Contains(geoMobileWithoutAreaCodes, cc) {
		return 0
	}
	if !isGeographical(typ, cc) {
		return 0
	}
	return ms.ndcLength(n)
}

// NationalDestinationCodeLength returns the length of n's national
// destination code, which is the start of [Number.NationalNumber], or 0
// when it has none. The national destination code is usually the first
// group of digits after the country calling code in [International]
// format, when another group follows. Unlike an area code, a mobile or
// non-geographical number can have one.
func (n Number) NationalDestinationCodeLength() int { return defaultSet().ndcLength(n) }

func (ms *metadataSet) ndcLength(n Number) int {
	noExt := n
	noExt.ext = ""
	// "+CC NDC ..." splits as "", the country calling code, then the groups.
	groups := nonDigits().Split(ms.format(noExt, International), -1)
	if len(groups) <= 3 {
		return 0
	}
	// Argentinian mobiles are written +54 9 NDC ..., where the mobile
	// token 9 is part of the national number.
	if ms.numberType(n) == Mobile && CountryMobileToken(int(n.countryCode)) != "" {
		return len(groups[2]) + len(groups[3])
	}
	return len(groups[2])
}

// CountryMobileToken returns the digits dialled before the area code of a
// mobile number from abroad in country calling code cc, such as "9" for
// Argentina (54), or "" if there are none.
func CountryMobileToken(cc int) string { return mobileTokens[cc] }

// IsAlphaNumber reports whether s is a phone number written with letters,
// such as "1-800-SIX-FLAG": a possible phone number with at least three
// letters, not counting an extension.
func IsAlphaNumber(s string) bool {
	if utf8.RuneCountInString(s) > maxInputStringLength || !isViablePhoneNumber(s) {
		return false
	}
	number, _ := stripExtension(s)
	return validAlphaPhone().MatchString(number)
}

// TruncateTooLong returns n with digits removed from the end until it is
// valid, for a number typed with extra digits. It reports false, and
// returns n unchanged, when no shorter number is valid. A valid n is
// returned as it is.
func (n Number) TruncateTooLong() (Number, bool) { return defaultSet().truncateTooLong(n) }

func (ms *metadataSet) truncateTooLong(n Number) (Number, bool) {
	if ms.valid(n) {
		return n, true
	}
	t := n
	for {
		t.national /= 10
		if t.national == 0 || ms.checkLength(t, TypeUnknown) == LengthTooShort {
			return n, false
		}
		if ms.valid(t) {
			return t, true
		}
	}
}

// InvalidExampleNumber returns a number for region that parses but is not
// valid, for testing. It reports false for an unknown region or "001".
func InvalidExampleNumber(region string) (Number, bool) {
	return defaultSet().invalidExample(strings.ToUpper(region))
}

func (ms *metadataSet) invalidExample(region string) (Number, bool) {
	meta := ms.region(region)
	if meta == nil {
		return Number{}, false
	}
	// Shorten a valid fixed-line number until it is not valid. Plans often
	// overlap, so a shorter number may still be valid as another type;
	// trying every length gives numbers that look real.
	ex := meta.fixedLine.example
	for l := len(ex) - 1; l >= minLengthForNSN; l-- {
		n, err := ms.parse(ex[:l], region)
		if err == nil && !ms.valid(n) {
			return n, true
		}
	}
	return Number{}, false
}

// MobileNumberPortable reports whether region lets mobile numbers move
// between carriers, so the number does not say which carrier serves it.
// It reports false for an unknown region.
func MobileNumberPortable(region string) bool {
	return defaultSet().mobilePortable(strings.ToUpper(region))
}

func (ms *metadataSet) mobilePortable(region string) bool {
	meta := ms.region(region)
	return meta != nil && meta.mobileNumberPortableRegion
}

// ConvertAlphaCharacters replaces the letters in s with their keypad
// digits, leaving everything else as it is: "1800-ABC-DEF" becomes
// "1800-222-333".
func ConvertAlphaCharacters(s string) string {
	var b strings.Builder
	for _, r := range s {
		// Upper-case as upstream does, so that dotless i, for one, counts
		// as I.
		if v, ok := alphaPhoneMappings[unicode.ToUpper(r)]; ok {
			b.WriteRune(v)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// NormalizeDigits returns the decimal digits of s, in any script, as ASCII
// digits, dropping everything else.
func NormalizeDigits(s string) string { return phonetext.NormalizeDigits(s, false) }

// NormalizeDiallable returns only the characters of s that can be dialled
// on a keypad: ASCII digits, '+', '*' and '#'.
func NormalizeDiallable(s string) string { return normalizeDiallable(s) }
