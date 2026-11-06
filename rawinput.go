package phonenumber

import (
	"strconv"
	"strings"

	"uradical.io/go/phonenumber/internal/phonetext"
)

// CountryCodeSource is how the country calling code of a parsed number was
// written; see [Parsed].
type CountryCodeSource int

const (
	// SourceUnspecified is the zero value: the source is not known.
	SourceUnspecified CountryCodeSource = iota
	// FromNumberWithPlusSign means the number started with a plus sign and
	// the country calling code, as in "+44 20 7946 0958".
	FromNumberWithPlusSign
	// FromNumberWithIDD means the number started with an international
	// dialling prefix, as in "011 44 20 7946 0958" dialled from the US.
	FromNumberWithIDD
	// FromNumberWithoutPlusSign means the number started with the country
	// calling code but no plus sign, as in "44 20 7946 0958".
	FromNumberWithoutPlusSign
	// FromDefaultCountry means the number had no country calling code, so
	// it came from the region it was parsed with.
	FromDefaultCountry
)

var countryCodeSourceNames = [...]string{
	SourceUnspecified:         "SourceUnspecified",
	FromNumberWithPlusSign:    "FromNumberWithPlusSign",
	FromNumberWithIDD:         "FromNumberWithIDD",
	FromNumberWithoutPlusSign: "FromNumberWithoutPlusSign",
	FromDefaultCountry:        "FromDefaultCountry",
}

func (s CountryCodeSource) String() string {
	if s >= 0 && int(s) < len(countryCodeSourceNames) {
		return countryCodeSourceNames[s]
	}
	return "CountryCodeSource(" + strconv.Itoa(int(s)) + ")"
}

// Parsed is a [Number] together with how it was written, as returned by
// [ParseAndKeepRawInput]. Keep the Number for storing and comparing, and
// the Parsed for formatting a number back the way the user wrote it.
//
// Number is a named field rather than embedded, so that encoding a Parsed
// does not silently encode only its Number.
type Parsed struct {
	Number Number
	// Raw is the input exactly as given to ParseAndKeepRawInput.
	Raw string
	// Source is how the country calling code was written.
	Source CountryCodeSource
	// CarrierCode is the carrier selection code written before the
	// national number, as in Brazil or Colombia, or "" if there was none.
	CarrierCode string
}

// ParseAndKeepRawInput parses s like [Parse], and also records the input,
// how its country calling code was written and any carrier code.
func ParseAndKeepRawInput(s, region string) (Parsed, error) {
	return defaultSet().parseAndKeepRawInput(s, strings.ToUpper(region))
}

func (ms *metadataSet) parseAndKeepRawInput(s, region string) (Parsed, error) {
	p, err := ms.parseRaw(s, region)
	if err != nil {
		return Parsed{}, err
	}
	p.Raw = s
	return p, nil
}

// FormatNationalWithPreferredCarrierCode returns the number in [National]
// format for dialling through the carrier it was written with, or through
// fallback when it was written with none. See
// [Number.FormatNationalWithCarrierCode].
func (p Parsed) FormatNationalWithPreferredCarrierCode(fallback string) string {
	return defaultSet().formatPreferredCarrier(p, fallback)
}

func (ms *metadataSet) formatPreferredCarrier(p Parsed, fallback string) string {
	carrier := p.CarrierCode
	if carrier == "" {
		carrier = fallback
	}
	return ms.formatWithCarrier(p.Number, carrier)
}

// FormatForMobileDialing is [Number.FormatForMobileDialing], using the
// carrier code the number was written with, which Brazilian numbers need.
// A number with an unknown country calling code gives Raw.
func (p Parsed) FormatForMobileDialing(region string, withFormatting bool) string {
	return defaultSet().formatForMobileDialing(p, strings.ToUpper(region), withFormatting)
}

// FormatInOriginalFormat returns the number written the way it was given:
// international if it had a plus sign, with region's international prefix
// if it had one, and national otherwise, with or without the national
// prefix as written. It never adds, removes or changes a digit: when the
// formatted number would, or there is no rule for grouping the number, it
// returns Raw.
func (p Parsed) FormatInOriginalFormat(region string) string {
	return defaultSet().formatInOriginalFormat(p, strings.ToUpper(region))
}

func (ms *metadataSet) formatInOriginalFormat(p Parsed, from string) string {
	n := p.Number
	rule := ms.formatRuleFor(n)
	if p.Raw != "" && rule == nil {
		// Without a rule, the number might come out as one group with no
		// national prefix.
		return p.Raw
	}
	var s string
	switch p.Source {
	case SourceUnspecified:
		return ms.format(n, National)
	case FromNumberWithPlusSign:
		s = ms.format(n, International)
	case FromNumberWithIDD:
		s = ms.formatFrom(n, from)
	case FromNumberWithoutPlusSign:
		s = ms.format(n, International)[1:]
	default:
		s = ms.formatNationalAsWritten(p, rule)
	}
	// Return the raw input if formatting changed a digit.
	if p.Raw != "" && normalizeDiallable(s) != normalizeDiallable(p.Raw) {
		return p.Raw
	}
	return s
}

// formatNationalAsWritten formats a number written without a country
// calling code in national format, leaving out the national prefix if the
// input did.
func (ms *metadataSet) formatNationalAsWritten(p Parsed, rule *numberFormat) string {
	n := p.Number
	region := ms.mainRegion(int(n.countryCode))
	national := ms.format(n, National)
	np := ms.nationalPrefix(region)
	if np == "" || ms.rawHasNationalPrefix(p.Raw, np, region) || rule == nil {
		return national
	}
	// Does the rule for this number write a national prefix? The first
	// group never comes before it.
	npRule := rule.nationalPrefixFormattingRule
	i := strings.Index(npRule, "$1")
	if i <= 0 || phonetext.NormalizeDigits(npRule[:i], false) == "" {
		return national
	}
	// Format again with a copy of the rule that writes no national prefix.
	noPrefix := *rule
	noPrefix.nationalPrefixFormattingRule = ""
	meta := ms.forCode(int(n.countryCode), region)
	return formatNSNWithRule(n.NationalNumber(), &noPrefix, National, "") + extensionSuffix(n, meta, National)
}

// nationalPrefix returns region's national prefix, or "" when the region
// has none or is unknown. Upstream marks a pause for a second dial tone
// with "~"; it is not a digit to dial, so it is removed.
func (ms *metadataSet) nationalPrefix(region string) string {
	t := ms.region(region)
	if t == nil {
		return ""
	}
	return strings.ReplaceAll(t.nationalPrefix, "~", "")
}

// rawHasNationalPrefix reports whether raw, a national number, starts with
// the national prefix np. A prefix match alone is not enough: Japanese
// 0777123 is not 777123 with a prefix, which the rest being a valid number
// shows.
func (ms *metadataSet) rawHasNationalPrefix(raw, np, region string) bool {
	digits := phonetext.NormalizeDigits(raw, false)
	rest, ok := strings.CutPrefix(digits, np)
	if !ok {
		return false
	}
	n, err := ms.parse(rest, region)
	return err == nil && ms.valid(n)
}

// formatRuleFor returns the national formatting rule for n, or nil.
func (ms *metadataSet) formatRuleFor(n Number) *numberFormat {
	cc := int(n.countryCode)
	meta := ms.forCode(cc, ms.mainRegion(cc))
	if meta == nil {
		return nil
	}
	return chooseFormat(meta.numberFormats, n.NationalNumber())
}

// FormatOutOfCountryKeepingAlphaChars is [Number.FormatFrom] for a number
// written with letters, such as 1-800-SIX-FLAG: it keeps the letters and
// the grouping of the raw input, replacing only what came before the
// national number with what region dials.
//
// It works from the first three digits of the national number, so it can
// go wrong when the country calling code in the input is also the start of
// the national number, or the input groups those three digits apart. Numbers
// written with letters rarely do either.
func (p Parsed) FormatOutOfCountryKeepingAlphaChars(region string) string {
	return defaultSet().formatKeepingAlpha(p, strings.ToUpper(region))
}

func (ms *metadataSet) formatKeepingAlpha(p Parsed, from string) string {
	n := p.Number
	if p.Raw == "" {
		return ms.formatFrom(n, from)
	}
	cc := int(n.countryCode)
	if !ms.knownCode(cc) {
		return p.Raw
	}
	// Keep only letters, digits and grouping symbols, then drop anything
	// before the national number, such as a prefix or country calling
	// code. Every valid number written with letters starts with three
	// digits.
	raw := mapChars(p.Raw, groupingChars)
	nsn := n.NationalNumber()
	if len(nsn) > 3 {
		if i := strings.Index(raw, nsn[:3]); i >= 0 {
			raw = raw[i:]
		}
	}
	fromMeta := ms.region(from)
	switch {
	case cc == nanpaCountryCode:
		if ms.isNANPA(from) {
			return strconv.Itoa(cc) + " " + raw
		}
	case fromMeta != nil && cc == fromMeta.countryCode:
		return formatAlphaNational(fromMeta, raw, nsn)
	}
	// A region with several international prefixes uses its preferred one,
	// or else the number is written with a plus sign.
	var idd string
	if fromMeta != nil {
		idd = fromMeta.preferredInternationalPrefix
		if fromMeta.hasSingleIntlPrefix() {
			idd = fromMeta.internationalPrefix.expr
		}
	}
	meta := ms.forCode(cc, ms.mainRegion(cc))
	raw, _ = stripExtension(raw)
	s := raw + extensionSuffix(n, meta, International)
	if idd != "" {
		return idd + " " + strconv.Itoa(cc) + " " + s
	}
	return prefixCountryCode(cc, International, s)
}

// formatAlphaNational writes raw in national format for meta's region: the
// first group of digits as written, with the national prefix where the
// rule for nsn wants one, then the rest as written.
func formatAlphaNational(meta *territory, raw, nsn string) string {
	rule := chooseFormat(meta.numberFormats, nsn)
	if rule == nil {
		return raw
	}
	// The pattern and leading digits decide whether a national prefix is
	// used; the metadata does not yet depend on the pattern alone.
	asWritten := *rule
	asWritten.pattern = &pattern{expr: `(\d+)(.*)`}
	asWritten.format = "$1$2"
	return formatNSNWithRule(raw, &asWritten, National, "")
}

// groupingChars maps the characters kept by
// [Parsed.FormatOutOfCountryKeepingAlphaChars]: letters (as upper case),
// ASCII digits, and grouping symbols, normalized to ASCII.
var groupingChars = func() map[rune]rune {
	m := map[rune]rune{
		'-': '-', '\uFF0D': '-', '\u2010': '-', '\u2011': '-', '\u2012': '-', '\u2013': '-',
		'\u2014': '-', '\u2015': '-', '\u2212': '-',
		'/': '/', '\uFF0F': '/',
		' ': ' ', '\u3000': ' ', '\u2060': ' ',
		'.': '.', '\uFF0E': '.',
	}
	for r := 'A'; r <= 'Z'; r++ {
		m[r] = r
	}
	for r := '0'; r <= '9'; r++ {
		m[r] = r
	}
	return m
}()
