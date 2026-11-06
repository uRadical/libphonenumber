package phonenumber

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Format is a way of writing a phone number.
type Format int

const (
	// E164 is the international standard form, with no spaces:
	// "+442079460958".
	E164 Format = iota
	// International is the form for dialling from abroad, grouped as in the
	// number's region: "+44 20 7946 0958".
	International
	// National is the form for dialling within the number's region,
	// including any national prefix: "020 7946 0958".
	National
	// RFC3966 is a tel: URI: "tel:+44-20-7946-0958".
	RFC3966
)

func (f Format) String() string {
	switch f {
	case E164:
		return "E164"
	case International:
		return "International"
	case National:
		return "National"
	case RFC3966:
		return "RFC3966"
	}
	return "Format(" + strconv.Itoa(int(f)) + ")"
}

// Format returns n written in format f. The extension, if any, is included
// in every format except [E164]. The zero Number formats as "".
func (n Number) Format(f Format) string { return defaultSet().format(n, f) }

// FormatFrom returns n written for dialling from region: in national format
// when region shares n's country calling code, and otherwise preceded by
// region's international dialling prefix, as in "00 44 20 7946 0958" from
// Germany or "011 44 20 7946 0958" from the United States. When region is not
// a known region code, FormatFrom returns the [International] format.
func (n Number) FormatFrom(region string) string {
	return defaultSet().formatFrom(n, strings.ToUpper(region))
}

func (ms *metadataSet) format(n Number, f Format) string {
	if n.IsZero() {
		return ""
	}
	cc := int(n.countryCode)
	nsn := n.NationalNumber()
	if f == E164 {
		return prefixCountryCode(cc, E164, nsn)
	}
	if !ms.knownCode(cc) {
		return nsn
	}
	meta := ms.forCode(cc, ms.mainRegion(cc))
	s := meta.formatNSN(nsn, f, "") + extensionSuffix(n, meta, f)
	return prefixCountryCode(cc, f, s)
}

func (ms *metadataSet) formatFrom(n Number, from string) string {
	if n.IsZero() {
		return ""
	}
	fromMeta := ms.region(from)
	if fromMeta == nil {
		return ms.format(n, International)
	}
	cc := int(n.countryCode)
	nsn := n.NationalNumber()
	if !ms.knownCode(cc) {
		return nsn
	}
	switch {
	case cc == nanpaCountryCode && ms.isNANPA(from):
		// Within the NANPA, other countries are dialled as 1 and the
		// national format.
		return strconv.Itoa(cc) + " " + ms.format(n, National)
	case cc != nanpaCountryCode && cc == fromMeta.countryCode:
		return ms.format(n, National)
	}
	// Use the region's preferred international prefix, or its only one;
	// some regions accept several and name none.
	var idd string
	switch {
	case fromMeta.preferredInternationalPrefix != "":
		idd = fromMeta.preferredInternationalPrefix
	case fromMeta.hasSingleIntlPrefix():
		idd = fromMeta.internationalPrefix.expr
	}
	meta := ms.forCode(cc, ms.mainRegion(cc))
	s := meta.formatNSN(nsn, International, "") + extensionSuffix(n, meta, International)
	if idd != "" {
		return idd + " " + strconv.Itoa(cc) + " " + s
	}
	return prefixCountryCode(cc, International, s)
}

func prefixCountryCode(cc int, f Format, s string) string {
	switch f {
	case E164:
		return "+" + strconv.Itoa(cc) + s
	case International:
		return "+" + strconv.Itoa(cc) + " " + s
	case RFC3966:
		return rfc3966Prefix + "+" + strconv.Itoa(cc) + "-" + s
	}
	return s
}

func extensionSuffix(n Number, meta *territory, f Format) string {
	switch {
	case n.ext == "":
		return ""
	case f == RFC3966:
		return rfc3966ExtnPrefix + n.ext
	case meta.preferredExtnPrefix != "":
		return meta.preferredExtnPrefix + n.ext
	}
	return defaultExtnPrefix + n.ext
}

// hasSingleIntlPrefix reports whether t has exactly one international
// dialling prefix, which can then be written in front of a number.
func (t *territory) hasSingleIntlPrefix() bool {
	return t.internationalPrefix != nil && singleIntlPrefix().MatchString(t.internationalPrefix.expr)
}

// formatNSN groups a national significant number with the first matching
// rule for the format, or returns it unchanged when no rule matches. A
// carrier code, if not "", is written in national format where the rule
// says how.
func (t *territory) formatNSN(nsn string, f Format, carrier string) string {
	formats := t.numberFormats
	if len(t.intlNumberFormats) > 0 && f != National {
		formats = t.intlNumberFormats
	}
	nf := chooseFormat(formats, nsn)
	if nf == nil {
		return nsn
	}
	return formatNSNWithRule(nsn, nf, f, carrier)
}

// chooseFormat returns the first rule whose last leading-digits pattern
// matches the start of nsn and whose pattern matches all of it.
func chooseFormat(formats []*numberFormat, nsn string) *numberFormat {
	for _, nf := range formats {
		if k := len(nf.leadingDigits); k > 0 && !nf.leadingDigits[k-1].matchPrefix(nsn) {
			continue
		}
		if nf.pattern.matchFull(nsn) {
			return nf
		}
	}
	return nil
}

func formatNSNWithRule(nsn string, nf *numberFormat, f Format, carrier string) string {
	rule := nf.format
	switch {
	case f == National && carrier != "" && nf.carrierCodeFormattingRule != "":
		// Put the carrier code, and the national prefix if the rule has
		// one, in front of the first group.
		ccRule := strings.ReplaceAll(nf.carrierCodeFormattingRule, "$CC", carrier)
		rule = replaceJava(firstGroupPattern(), rule, ccRule, false)
	case f == National && nf.nationalPrefixFormattingRule != "":
		// Put the national prefix in front of the first group, as in
		// "($1)" becoming "(0$1)".
		rule = replaceJava(firstGroupPattern(), rule, nf.nationalPrefixFormattingRule, false)
	}
	s := replaceJava(nf.pattern.anyRegexp(), nsn, rule, true)
	if f == RFC3966 {
		if loc := separatorPattern().FindStringIndex(s); len(loc) == 2 && loc[0] == 0 {
			s = s[loc[1]:]
		}
		s = separatorPattern().ReplaceAllLiteralString(s, "-")
	}
	return s
}

// FormatNationalWithCarrierCode returns n in [National] format for dialling
// through the carrier selected by carrier, a carrier selection code such as
// "15" in Brazil. Where the region writes no carrier code, or carrier is "",
// it is the same as n.Format(National).
func (n Number) FormatNationalWithCarrierCode(carrier string) string {
	return defaultSet().formatWithCarrier(n, carrier)
}

func (ms *metadataSet) formatWithCarrier(n Number, carrier string) string {
	if n.IsZero() {
		return ""
	}
	cc := int(n.countryCode)
	nsn := n.NationalNumber()
	if !ms.knownCode(cc) {
		return nsn
	}
	meta := ms.forCode(cc, ms.mainRegion(cc))
	return meta.formatNSN(nsn, National, carrier) + extensionSuffix(n, meta, National)
}

// FormatForMobileDialing returns n written so that it can be dialled from a
// mobile phone in region, or "" when it cannot be dialled from there, such
// as a toll-free number that is not reachable from abroad. Unless
// withFormatting is set, the result has only the characters to dial. The
// extension is left out, since it cannot normally be dialled with the
// number.
//
// Brazilian fixed-line and mobile numbers need a carrier code when dialled
// within Brazil, which a Number does not hold, so they give "": use
// [Parsed.FormatForMobileDialing] with a number parsed from input that
// includes one.
func (n Number) FormatForMobileDialing(region string, withFormatting bool) string {
	return defaultSet().formatForMobileDialing(Parsed{Number: n}, strings.ToUpper(region), withFormatting)
}

func (ms *metadataSet) formatForMobileDialing(p Parsed, from string, withFormatting bool) string {
	cc := int(p.Number.countryCode)
	if !ms.knownCode(cc) {
		return p.Raw
	}
	n := p.Number
	n.ext = ""
	region := ms.mainRegion(cc)
	typ := ms.numberType(n)
	var s string
	switch {
	case from == region:
		s = ms.formatForDomesticMobileDialing(n, region, from, typ, p.CarrierCode)
	case typ != TypeUnknown && ms.canDialInternationally(n):
		// A short number, which is not a valid regular number, is taken
		// to be unreachable from abroad.
		if withFormatting {
			return ms.format(n, International)
		}
		return ms.format(n, E164)
	}
	if withFormatting {
		return s
	}
	return normalizeDiallable(s)
}

// formatForDomesticMobileDialing formats n for dialling from a mobile within
// its own region.
func (ms *metadataSet) formatForDomesticMobileDialing(n Number, region, from string, typ Type, carrier string) string {
	fixedOrMobile := typ == FixedLine || typ == Mobile || typ == FixedLineOrMobile
	switch {
	case region == "BR" && fixedOrMobile:
		// Most Brazilian carriers do not connect a call without a carrier
		// code.
		if carrier == "" {
			return ""
		}
		return ms.formatWithCarrier(n, carrier)
	case int(n.countryCode) == nanpaCountryCode:
		// International format always works in the NANPA, except for what
		// may be short numbers, which are dialled nationally.
		if ms.canDialInternationally(n) && ms.region(from).testLength(n.NationalNumber(), TypeUnknown) != LengthTooShort {
			return ms.format(n, International)
		}
		return ms.format(n, National)
	case (region == regionNonGeo || ((region == "MX" || region == "CL" || region == "UZ") && fixedOrMobile)) &&
		ms.canDialInternationally(n):
		// Mexican fixed lines and mobiles need a carrier code that depends
		// on the caller in national format, Chilean ones need the national
		// prefix only for one of the two, and Uzbek ones need special
		// prefixes; international format works for all of them.
		return ms.format(n, International)
	}
	return ms.format(n, National)
}

// diallableChars maps the characters that can be dialled on a keypad to
// themselves.
var diallableChars = map[rune]rune{
	'0': '0', '1': '1', '2': '2', '3': '3', '4': '4', '5': '5', '6': '6', '7': '7', '8': '8', '9': '9',
	'+': '+', '*': '*', '#': '#',
}

// normalizeDiallable keeps only the characters of s that can be dialled
// on a keypad: ASCII digits, '+', '*' and '#'.
func normalizeDiallable(s string) string { return mapChars(s, diallableChars) }

// FormatRule is a caller-defined rule for [Number.FormatByPattern]. It has
// the shape of upstream's NumberFormat.
type FormatRule struct {
	// Pattern is a regular expression matching all of the national
	// significant number, with a group for each part of the result.
	Pattern string
	// Format rearranges Pattern's groups with $1-style references, as in
	// "($1) $2-$3".
	Format string
	// LeadingDigits optionally lists expressions for the start of the
	// national significant number. Only the last, which should be the most
	// detailed, is used.
	LeadingDigits []string
	// NationalPrefixFormattingRule says how to write the national prefix
	// in [National] format: $NP stands for the region's national prefix
	// and $FG for the first group, as in "($NP$FG)".
	NationalPrefixFormattingRule string
	// CarrierCodeFormattingRule says how to write a carrier code in
	// [National] format, with $CC for the code; see
	// [Number.FormatNationalWithCarrierCode]. FormatByPattern does not
	// use it.
	CarrierCodeFormattingRule string
}

// FormatByPattern returns n in format f, grouped by the first of rules
// that matches its national significant number instead of the metadata's
// rules. A number no rule matches is written ungrouped. It fails only when
// a rule has an invalid regular expression.
func (n Number) FormatByPattern(f Format, rules []FormatRule) (string, error) {
	return defaultSet().formatByPattern(n, f, rules)
}

func (ms *metadataSet) formatByPattern(n Number, f Format, rules []FormatRule) (string, error) {
	formats, err := compileRules(rules)
	if err != nil {
		return "", err
	}
	if n.IsZero() {
		return "", nil
	}
	cc := int(n.countryCode)
	nsn := n.NationalNumber()
	if !ms.knownCode(cc) {
		return nsn, nil
	}
	meta := ms.forCode(cc, ms.mainRegion(cc))
	s := nsn
	if nf := chooseFormat(formats, nsn); nf != nil {
		// Each call gets its own copy, so expanding $NP here does not
		// change the caller's rule.
		rule := *nf
		if rule.nationalPrefixFormattingRule != "" {
			if meta.nationalPrefix == "" {
				// No national prefix, so no rule for writing one.
				rule.nationalPrefixFormattingRule = ""
			} else {
				r := strings.ReplaceAll(rule.nationalPrefixFormattingRule, "$NP", meta.nationalPrefix)
				rule.nationalPrefixFormattingRule = strings.ReplaceAll(r, "$FG", "$1")
			}
		}
		s = formatNSNWithRule(nsn, &rule, f, "")
	}
	s += extensionSuffix(n, meta, f)
	return prefixCountryCode(cc, f, s), nil
}

// compileRules checks the expressions in rules and converts them to the
// metadata's form.
func compileRules(rules []FormatRule) ([]*numberFormat, error) {
	formats := make([]*numberFormat, len(rules))
	for i, r := range rules {
		nf := &numberFormat{
			pattern:                      &pattern{expr: r.Pattern},
			format:                       r.Format,
			nationalPrefixFormattingRule: r.NationalPrefixFormattingRule,
			carrierCodeFormattingRule:    r.CarrierCodeFormattingRule,
		}
		exprs := append([]string{r.Pattern}, r.LeadingDigits...)
		for j, expr := range exprs {
			if _, err := regexp.Compile(expr); err != nil {
				return nil, fmt.Errorf("phonenumber: format rule %d: %w", i, err)
			}
			if j > 0 {
				nf.leadingDigits = append(nf.leadingDigits, &pattern{expr: expr})
			}
		}
		formats[i] = nf
	}
	return formats, nil
}
