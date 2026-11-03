package phonenumber

import (
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
	s := meta.formatNSN(nsn, f) + extensionSuffix(n, meta, f)
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
	case fromMeta.internationalPrefix != nil && singleIntlPrefix().MatchString(fromMeta.internationalPrefix.expr):
		idd = fromMeta.internationalPrefix.expr
	}
	meta := ms.forCode(cc, ms.mainRegion(cc))
	s := meta.formatNSN(nsn, International) + extensionSuffix(n, meta, International)
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

// formatNSN groups a national significant number with the first matching
// rule for the format, or returns it unchanged when no rule matches.
func (t *territory) formatNSN(nsn string, f Format) string {
	formats := t.numberFormats
	if len(t.intlNumberFormats) > 0 && f != National {
		formats = t.intlNumberFormats
	}
	nf := chooseFormat(formats, nsn)
	if nf == nil {
		return nsn
	}
	return formatNSNWithRule(nsn, nf, f)
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

func formatNSNWithRule(nsn string, nf *numberFormat, f Format) string {
	rule := nf.format
	if f == National && nf.nationalPrefixFormattingRule != "" {
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
