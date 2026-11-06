package phonenumber

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"uradical.io/go/phonenumber/internal/phonetext"
)

// Errors returned by [Parse]. Each is wrapped with more detail, so test for
// them with [errors.Is].
var (
	// ErrNotANumber means the input does not look like a phone number.
	ErrNotANumber = errors.New("phonenumber: not a number")
	// ErrInvalidCountryCode means the country calling code is unknown, or
	// the number has none and no valid default region was given.
	ErrInvalidCountryCode = errors.New("phonenumber: invalid country calling code")
	// ErrTooShortAfterIDD means too few digits follow an international
	// dialling prefix such as + or 00.
	ErrTooShortAfterIDD = errors.New("phonenumber: too short after international prefix")
	// ErrTooShort means the national number has fewer than two digits.
	ErrTooShort = errors.New("phonenumber: too short")
	// ErrTooLong means the input is over 250 characters or the national
	// number is over 17 digits.
	ErrTooLong = errors.New("phonenumber: too long")
)

const (
	minLengthForNSN       = 2
	maxLengthForNSN       = 17
	maxLengthCountryCode  = 3
	maxInputStringLength  = 250
	regionUnknown         = "ZZ"
	nanpaCountryCode      = 1
	rfc3966ExtnPrefix     = ";ext="
	rfc3966Prefix         = "tel:"
	rfc3966PhoneContext   = ";phone-context="
	rfc3966IsdnSubaddress = ";isub="
	defaultExtnPrefix     = " ext. "
)

func extnDigits(max int) string { return `(` + digits + `{1,` + strconv.Itoa(max) + `})` }

// extnPattern returns upstream's expression for the extension at the end of
// a number. Parsing accepts a few more forms than matching numbers in text.
func extnPattern(forParsing bool) string {
	const (
		limitAfterExplicitLabel = 20
		limitAfterLikelyLabel   = 15
		limitAfterAmbiguousChar = 9
		limitWhenNotSure        = 6

		separatorsBeforeLabel = `[ \x{00A0}\t,]*`
		charsAfterLabel       = `[:\.\x{FF0E}]?[ \x{00A0}\t,-]*`
		optionalSuffix        = `#?`
		explicitLabels        = `(?:e?xt(?:ensi(?:o\x{0301}?|\x{00F3}))?n?|\x{FF45}?\x{FF58}\x{FF54}\x{FF4E}?|\x{0434}\x{043E}\x{0431}|anexo)`
		ambiguousLabels       = `(?:[x\x{FF58}#\x{FF03}~\x{FF5E}]|int|\x{FF49}\x{FF4E}\x{FF54})`
		ambiguousSeparator    = `[- ]+`
	)
	rfc := rfc3966ExtnPrefix + extnDigits(limitAfterExplicitLabel)
	explicit := separatorsBeforeLabel + explicitLabels + charsAfterLabel +
		extnDigits(limitAfterExplicitLabel) + optionalSuffix
	ambiguous := separatorsBeforeLabel + ambiguousLabels + charsAfterLabel +
		extnDigits(limitAfterAmbiguousChar) + optionalSuffix
	american := ambiguousSeparator + extnDigits(limitWhenNotSure) + `#`
	p := rfc + `|` + explicit + `|` + ambiguous + `|` + american
	if forParsing {
		const separatorsNoComma = `[ \x{00A0}\t]*`
		autoDialling := separatorsNoComma + `(?:,{2}|;)` + charsAfterLabel +
			extnDigits(limitAfterLikelyLabel) + optionalSuffix
		onlyCommas := separatorsNoComma + `(?:,)+` + charsAfterLabel +
			extnDigits(limitAfterAmbiguousChar) + optionalSuffix
		p += `|` + autoDialling + `|` + onlyCommas
	}
	return p
}

const (
	rfc3966PhoneDigit  = `(` + digits + `|[\-\.\(\)]?)`
	rfc3966AlphaNum    = validAlpha + digits
	rfc3966DomainLabel = `[` + rfc3966AlphaNum + `]+((\-)*[` + rfc3966AlphaNum + `])*`
	rfc3966TopLabel    = `[` + validAlpha + `]+((\-)*[` + rfc3966AlphaNum + `])*`
)

var (
	extnRegexp = sync.OnceValue(func() *regexp.Regexp {
		return regexp.MustCompile(`(?i)(?:` + extnPattern(true) + `)$`)
	})
	validPhoneNumberRegexp = sync.OnceValue(func() *regexp.Regexp {
		validPhoneNumber := digits + `{` + strconv.Itoa(minLengthForNSN) + `}|` +
			`[` + plusChars + `]*(?:[` + validPunctuation + `*]*` + digits + `){3,}` +
			`[` + validPunctuation + `*` + validAlpha + digits + `]*`
		return regexp.MustCompile(`(?i)^(?:` + validPhoneNumber + `(?:` + extnPattern(true) + `)?)$`)
	})
	rfc3966GlobalNumber  = lazyRegexp(`^\+` + rfc3966PhoneDigit + `*` + digits + rfc3966PhoneDigit + `*$`)
	rfc3966DomainNameReg = lazyRegexp(`^(` + rfc3966DomainLabel + `\.)*` + rfc3966TopLabel + `\.?$`)
)

// Parse parses a phone number written in any common format, such as
// "020 7946 0958", "+44 (0)20-7946-0958" or "tel:+44-20-7946-0958".
//
// region is the CLDR region code, such as "GB", that a number written in
// national format belongs to. It is ignored when the number starts with a
// plus sign, so pass "" when every number is international. Region codes are
// not case sensitive.
//
// Parse accepts anything shaped like a phone number, including numbers that
// are not valid in their region's numbering plan; use [Number.Valid] to
// check. It fails with an error wrapping [ErrNotANumber],
// [ErrInvalidCountryCode], [ErrTooShortAfterIDD], [ErrTooShort] or
// [ErrTooLong].
func Parse(s, region string) (Number, error) {
	return defaultSet().parse(s, strings.ToUpper(region))
}

func (ms *metadataSet) parse(s, region string) (Number, error) {
	p, err := ms.parseRaw(s, region)
	return p.Number, err
}

// parseRaw parses s and records how it was written. Raw is left for the
// caller to set.
func (ms *metadataSet) parseRaw(s, region string) (Parsed, error) {
	number, err := extractNumber(s)
	if err != nil {
		return Parsed{}, err
	}
	var ext string
	number, ext = stripExtension(number)
	cc, nsn, src, meta, err := ms.splitNumber(number, region)
	if err != nil {
		return Parsed{}, err
	}
	if len(nsn) < minLengthForNSN {
		return Parsed{}, errTooShortNSN
	}
	nsn, carrier := meta.stripNationalPrefixIfPossible(nsn)
	n, err := newNumber(cc, nsn, ext)
	if err != nil {
		return Parsed{}, err
	}
	return Parsed{Number: n, Source: src, CarrierCode: carrier}, nil
}

var errTooShortNSN = fmt.Errorf("%w: national number has fewer than %d digits", ErrTooShort, minLengthForNSN)

// extractNumber finds the phone number in s, or reports that there is none.
func extractNumber(s string) (string, error) {
	if utf8.RuneCountInString(s) > maxInputStringLength {
		return "", fmt.Errorf("%w: input exceeds %d characters", ErrTooLong, maxInputStringLength)
	}
	number, err := buildNationalNumberForParsing(s)
	if err != nil {
		return "", err
	}
	if !isViablePhoneNumber(number) {
		return "", fmt.Errorf("%w: input does not look like a phone number", ErrNotANumber)
	}
	return number, nil
}

// splitNumber separates number into its country calling code and national
// number, and returns the metadata to finish parsing it with.
func (ms *metadataSet) splitNumber(number, region string) (cc int, nsn string, src CountryCodeSource, meta *territory, err error) {
	meta = ms.region(region)
	if meta == nil && !phonetext.PlusCharsPattern().MatchString(number) {
		return 0, "", 0, nil, fmt.Errorf("%w: no plus sign and no valid default region", ErrInvalidCountryCode)
	}
	cc, nsn, src, err = ms.extractCountryCode(number, meta)
	if errors.Is(err, ErrInvalidCountryCode) {
		cc, nsn, src, err = ms.retryWithoutPlus(number, meta, err)
	}
	if err != nil {
		return 0, "", 0, nil, err
	}
	if cc == 0 {
		// No country calling code, so the number is national to region,
		// which the check above guarantees is valid.
		return meta.countryCode, normalize(number), src, meta, nil
	}
	if main := ms.mainRegion(cc); main != region {
		meta = ms.forCode(cc, main)
	}
	return cc, nsn, src, meta, nil
}

// retryWithoutPlus handles a number that starts with a plus sign but no
// known country calling code, in case the plus was a typo: it parses the
// rest as a national number or one with an international prefix.
func (ms *metadataSet) retryWithoutPlus(number string, meta *territory, err error) (int, string, CountryCodeSource, error) {
	loc := phonetext.PlusCharsPattern().FindStringIndex(number)
	if loc == nil {
		return 0, "", 0, err
	}
	cc, nsn, src, err := ms.extractCountryCode(number[loc[1]:], meta)
	if err != nil {
		return 0, "", 0, err
	}
	if cc == 0 {
		return 0, "", 0, fmt.Errorf("%w: cannot read a country calling code after the plus sign", ErrInvalidCountryCode)
	}
	return cc, nsn, src, nil
}

// stripNationalPrefixIfPossible removes the national prefix, and any
// carrier code, from nsn, unless that leaves a number too short to be one.
// It returns the carrier code it removed. t may be nil, for a country
// calling code with no metadata.
func (t *territory) stripNationalPrefixIfPossible(nsn string) (string, string) {
	if t == nil {
		return nsn, ""
	}
	stripped, carrier, _ := t.stripNationalPrefix(nsn)
	switch t.testLength(stripped, TypeUnknown) {
	case LengthTooShort, LengthLocalOnly, LengthInvalid:
		return nsn, ""
	}
	return stripped, carrier
}

// newNumber builds a Number from a country calling code and a national
// significant number of ASCII digits.
func newNumber(cc int, nsn, ext string) (Number, error) {
	if len(nsn) < minLengthForNSN {
		return Number{}, errTooShortNSN
	}
	if len(nsn) > maxLengthForNSN {
		return Number{}, fmt.Errorf("%w: national number has more than %d digits", ErrTooLong, maxLengthForNSN)
	}
	if cc <= 0 || cc > math.MaxUint16 {
		// Country calling codes have at most three digits.
		return Number{}, fmt.Errorf("%w: %d", ErrInvalidCountryCode, cc)
	}
	n := Number{countryCode: uint16(cc), zeros: leadingZeros(nsn), ext: ext}
	// At most 17 digits, so this cannot overflow.
	for i := range len(nsn) {
		n.national = n.national*10 + uint64(nsn[i]-'0')
	}
	return n, nil
}

// leadingZeros counts the zeros that start a national number, as in Italian
// fixed lines, without counting a final digit.
func leadingZeros(nsn string) uint8 {
	if len(nsn) < 2 || nsn[0] != '0' {
		return 0
	}
	z := 1
	for z < len(nsn)-1 && nsn[z] == '0' {
		z++
	}
	return uint8(z)
}

// buildNationalNumberForParsing extracts the number from an RFC 3966 URI or
// from surrounding text.
func buildNationalNumberForParsing(s string) (string, error) {
	var b strings.Builder
	ctxIdx := strings.Index(s, rfc3966PhoneContext)
	if ctxIdx >= 0 {
		ctx := phoneContext(s, ctxIdx)
		if !validPhoneContext(ctx) {
			return "", fmt.Errorf("%w: invalid phone-context", ErrNotANumber)
		}
		// A global phone-context is a prefix for the number; a domain name
		// is ignored.
		if ctx[0] == '+' {
			b.WriteString(ctx)
		}
		start := 0
		if i := strings.Index(s, rfc3966Prefix); i >= 0 {
			start = i + len(rfc3966Prefix)
		}
		if start <= ctxIdx {
			b.WriteString(s[start:ctxIdx])
		}
	} else {
		b.WriteString(phonetext.ExtractPossibleNumber(s))
	}
	number := b.String()
	if i := strings.Index(number, rfc3966IsdnSubaddress); i > 0 {
		number = number[:i]
	}
	return number, nil
}

func phoneContext(s string, idx int) string {
	start := idx + len(rfc3966PhoneContext)
	if start >= len(s) {
		return ""
	}
	if end := strings.IndexByte(s[start:], ';'); end >= 0 {
		return s[start : start+end]
	}
	return s[start:]
}

func validPhoneContext(ctx string) bool {
	return ctx != "" && (rfc3966GlobalNumber().MatchString(ctx) || rfc3966DomainNameReg().MatchString(ctx))
}

var emptyPattern = &pattern{}

func isViablePhoneNumber(s string) bool {
	if utf8.RuneCountInString(s) < minLengthForNSN {
		return false
	}
	return validPhoneNumberRegexp().MatchString(s)
}

// stripExtension removes an extension from the end of s, returning the
// number and the extension's digits.
func stripExtension(s string) (number, ext string) {
	m := extnRegexp().FindStringSubmatchIndex(s)
	if len(m) == 0 || !isViablePhoneNumber(s[:m[0]]) {
		return s, ""
	}
	for g := 1; g < len(m)/2; g++ {
		if m[2*g] >= 0 {
			return s[:m[0]], s[m[2*g]:m[2*g+1]]
		}
	}
	return s, ""
}

// extractCountryCode reads the country calling code from the start of
// number, after any plus sign or international dialling prefix for meta's
// region, and reports how it was written. It returns 0 and no error when
// the number has no country calling code, in which case nsn is
// meaningless.
func (ms *metadataSet) extractCountryCode(number string, meta *territory) (cc int, nsn string, src CountryCodeSource, err error) {
	if number == "" {
		return 0, "", FromDefaultCountry, nil
	}
	var idd *pattern
	if meta != nil {
		// Upstream treats a region without an international prefix as
		// having an empty one, which matches before any digit but zero.
		idd = meta.internationalPrefix
		if idd == nil {
			idd = emptyPattern
		}
	}
	full, src := stripInternationalPrefix(number, idd)
	if src != FromDefaultCountry {
		if len(full) <= minLengthForNSN {
			return 0, "", 0, fmt.Errorf("%w: too few digits follow the prefix", ErrTooShortAfterIDD)
		}
		if cc, nsn := ms.splitCountryCode(full); cc != 0 {
			return cc, nsn, src, nil
		}
		return 0, "", 0, fmt.Errorf("%w: unknown country calling code", ErrInvalidCountryCode)
	}
	if meta == nil {
		return 0, "", src, nil
	}
	// The number may start with the default region's country calling code
	// but no plus sign, as in "44 20 7946 0958" for GB. Strip the code when
	// the rest is a better fit for the region than the whole.
	ccStr := strconv.Itoa(meta.countryCode)
	rest, ok := strings.CutPrefix(full, ccStr)
	if !ok {
		return 0, "", src, nil
	}
	rest, _, _ = meta.stripNationalPrefix(rest)
	general := &meta.generalDesc
	if (!general.matchesPattern(full) && general.matchesPattern(rest)) ||
		meta.testLength(full, TypeUnknown) == LengthTooLong {
		return meta.countryCode, rest, FromNumberWithoutPlusSign, nil
	}
	return 0, "", src, nil
}

// splitCountryCode reads a known country calling code of one to three
// digits from the start of full.
func (ms *metadataSet) splitCountryCode(full string) (cc int, nsn string) {
	if full == "" || full[0] == '0' {
		return 0, ""
	}
	for i := 1; i <= maxLengthCountryCode && i <= len(full); i++ {
		code, err := strconv.Atoi(full[:i])
		if err != nil {
			return 0, ""
		}
		if ms.knownCode(code) {
			return code, full[i:]
		}
	}
	return 0, ""
}

// stripInternationalPrefix normalizes number and removes a leading plus
// sign or international dialling prefix, reporting which it found:
// FromNumberWithPlusSign, FromNumberWithIDD, or FromDefaultCountry for
// neither.
func stripInternationalPrefix(number string, idd *pattern) (string, CountryCodeSource) {
	if number == "" {
		return "", FromDefaultCountry
	}
	if loc := phonetext.PlusCharsPattern().FindStringIndex(number); loc != nil {
		return normalize(number[loc[1]:]), FromNumberWithPlusSign
	}
	number = normalize(number)
	if idd == nil {
		return number, FromDefaultCountry
	}
	loc := idd.prefixRegexp().FindStringIndex(number)
	if loc == nil {
		return number, FromDefaultCountry
	}
	// A zero right after the prefix is a national prefix, not a country
	// calling code, so the match was not an international prefix after all.
	if m := capturingDigit().FindStringSubmatch(number[loc[1]:]); len(m) > 1 && phonetext.NormalizeDigits(m[1], false) == "0" {
		return number, FromDefaultCountry
	}
	return number[loc[1]:], FromNumberWithIDD
}

// stripNationalPrefix removes the national prefix, and any carrier code,
// from the start of a national number. It leaves the number alone when
// removing the prefix would turn a number that matches the region's general
// pattern into one that does not.
func (t *territory) stripNationalPrefix(number string) (nsn, carrier string, ok bool) {
	if number == "" || t.nationalPrefixForParsing == nil {
		return number, "", false
	}
	re := t.nationalPrefixForParsing.prefixRegexp()
	m := re.FindStringSubmatchIndex(number)
	if m == nil {
		return number, "", false
	}
	general := &t.generalDesc
	viable := general.matchesPattern(number)
	groups := len(m)/2 - 1
	// With no groups, upstream's "last group" is the whole match.
	lastGroupMatched := m[2*groups] >= 0
	if t.nationalPrefixTransformRule == "" || !lastGroupMatched {
		if viable && !general.matchesPattern(number[m[1]:]) {
			return number, "", false
		}
		if groups > 0 && lastGroupMatched {
			carrier = number[m[2]:m[3]]
		}
		return number[m[1]:], carrier, true
	}
	// The match is at the start of number, so expanding it in place is
	// the same as upstream's replaceFirst.
	var b strings.Builder
	expandJava(&b, t.nationalPrefixTransformRule, number, m)
	b.WriteString(number[m[1]:])
	transformed := b.String()
	if viable && !general.matchesPattern(transformed) {
		return number, "", false
	}
	if groups > 1 && m[2] >= 0 {
		carrier = number[m[2]:m[3]]
	}
	return transformed, carrier, true
}
