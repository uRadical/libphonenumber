package phonenumber

import (
	"regexp"
	"strconv"
	"strings"
	"sync"

	"uradical.io/go/phonenumber/internal/phonetext"
)

// AsYouTypeFormatter formats a phone number as it is typed, one character
// at a time, as a phone's dialler does. It is the Go port of upstream's
// AsYouTypeFormatter. Create one with [NewAsYouTypeFormatter], feed it
// characters with [AsYouTypeFormatter.InputDigit], and call
// [AsYouTypeFormatter.Clear] before starting another number.
//
// An AsYouTypeFormatter is not safe for concurrent use.
type AsYouTypeFormatter struct {
	ms             *metadataSet
	defaultCountry string
	defaultMeta    *territory
	meta           *territory

	currentOutput string
	// template is the current formatting template, with placeholder for
	// each digit not yet entered. Runes, so that filling in a digit keeps
	// the positions of the rest.
	template      []rune
	currentFormat string // pattern of the rule the template came from

	accruedInput         strings.Builder
	accruedInputRunes    int
	accruedInputDigits   strings.Builder // digits and plus sign only, normalized
	ableToFormat         bool
	inputHasFormatting   bool
	isCompleteNumber     bool
	expectingCountryCode bool

	lastMatchPosition  int
	originalPosition   int
	positionToRemember int

	prefixBeforeNationalNumber  strings.Builder
	addSpaceAfterNationalPrefix bool
	extractedNationalPrefix     string
	nationalNumber              string
	possibleFormats             []*numberFormat
}

const (
	// digitPlaceholder stands for a digit not yet entered in a template:
	// the punctuation space.
	digitPlaceholder = '\u2008'
	// separatorBeforeNationalNumber separates a prefix, such as a long
	// national prefix or a country calling code, from the national number.
	separatorBeforeNationalNumber = ' '
	// minLeadingDigitsLength is how many digits of the national number
	// the formatter needs before choosing a rule. The first leading digits
	// pattern of each rule matches up to this many.
	minLeadingDigitsLength = 3
)

var (
	// eligibleFormat matches the format rules the formatter can use:
	// groups of $ and one digit, starting with $1, separated by
	// punctuation. It keeps characters such as the star of Israeli star
	// numbers out of the output, and makes sure no digit is lost.
	eligibleFormat = lazyRegexp(`^[` + validPunctuation + `]*\$1[` + validPunctuation + `]*(\$\d[` + validPunctuation + `]*)*$`)
	// emptyTerritory stands in for an unknown region, so that numbers
	// starting with a plus sign can still be formatted.
	emptyTerritory = &territory{internationalPrefix: &pattern{expr: "NA"}}
)

// NewAsYouTypeFormatter returns a formatter for numbers typed in region.
// With an unknown region, only numbers starting with a plus sign are
// formatted. Region codes are not case sensitive.
func NewAsYouTypeFormatter(region string) *AsYouTypeFormatter {
	return defaultSet().newAsYouTypeFormatter(strings.ToUpper(region))
}

func (ms *metadataSet) newAsYouTypeFormatter(region string) *AsYouTypeFormatter {
	f := &AsYouTypeFormatter{ms: ms, defaultCountry: region, ableToFormat: true}
	f.meta = f.metadataFor(region)
	f.defaultMeta = f.meta
	return f
}

// metadataFor returns the metadata of the main region for region's
// country calling code, which holds the formats for all of them.
func (f *AsYouTypeFormatter) metadataFor(region string) *territory {
	cc := 0
	if t := f.ms.region(region); t != nil {
		cc = t.countryCode
	}
	if t := f.ms.region(f.ms.mainRegion(cc)); t != nil {
		return t
	}
	return emptyTerritory
}

// Clear resets the formatter to format a new number.
func (f *AsYouTypeFormatter) Clear() {
	*f = AsYouTypeFormatter{
		ms: f.ms, defaultCountry: f.defaultCountry, defaultMeta: f.defaultMeta,
		meta: f.defaultMeta, ableToFormat: true,
	}
}

// InputDigit adds the next character typed and returns the number so far,
// formatted. Digits in any script are accepted, and a plus sign as the
// first character. Once anything else is typed, such as the user's own
// formatting, the formatter stops formatting and returns the input as
// typed.
func (f *AsYouTypeFormatter) InputDigit(r rune) string {
	f.currentOutput = f.inputDigit(r, false)
	return f.currentOutput
}

// InputDigitAndRememberPosition is [AsYouTypeFormatter.InputDigit], also
// remembering where r ends up, for [AsYouTypeFormatter.RememberedPosition].
// The position follows r as formatting is added or removed before it.
func (f *AsYouTypeFormatter) InputDigitAndRememberPosition(r rune) string {
	f.currentOutput = f.inputDigit(r, true)
	return f.currentOutput
}

// RememberedPosition returns the byte offset, in the string last returned,
// just after the character last passed to
// [AsYouTypeFormatter.InputDigitAndRememberPosition]: where a cursor
// placed after it should now be.
func (f *AsYouTypeFormatter) RememberedPosition() int {
	if !f.ableToFormat {
		return f.originalPosition
	}
	digits := f.accruedInputDigits.String()
	in, out := 0, 0
	for in < f.positionToRemember && out < len(f.currentOutput) {
		if digits[in] == f.currentOutput[out] {
			in++
		}
		out++
	}
	return out
}

func (f *AsYouTypeFormatter) inputDigit(r rune, remember bool) string {
	f.accruedInput.WriteRune(r)
	f.accruedInputRunes++
	if remember {
		f.originalPosition = f.accruedInput.Len()
	}
	// Format as typed only while every character is a digit, or a plus
	// sign at the start.
	if !f.isDigitOrLeadingPlus(r) {
		f.ableToFormat = false
		f.inputHasFormatting = true
	} else {
		r = f.accrueDigitOrPlus(r, remember)
	}
	if !f.ableToFormat {
		return f.inputWhenUnableToFormat()
	}
	// Formatting starts with three characters, a plus sign counting as
	// one.
	switch n := f.accruedInputDigits.Len(); {
	case n < 3:
		return f.accruedInput.String()
	case n == 3:
		if !f.extractIDD() {
			// No international prefix, so perhaps a national number.
			f.extractedNationalPrefix = f.removeNationalPrefix()
			return f.chooseFormattingPattern()
		}
		f.expectingCountryCode = true
	}
	if f.expectingCountryCode {
		if f.extractCountryCode() {
			f.expectingCountryCode = false
		}
		return f.prefixBeforeNationalNumber.String() + f.nationalNumber
	}
	if len(f.possibleFormats) == 0 {
		return f.chooseFormattingPattern()
	}
	// The rules are already chosen.
	tempNationalNumber := f.inputDigitHelper(r)
	// Format with a rule that matches all the digits so far, if there is
	// one; otherwise use the template.
	if s := f.formatAccruedDigits(); s != "" {
		return s
	}
	f.narrowDownPossibleFormats(f.nationalNumber)
	if f.maybeCreateNewTemplate() {
		return f.inputAccruedNationalNumber()
	}
	if f.ableToFormat {
		return f.appendNationalNumber(tempNationalNumber)
	}
	return f.accruedInput.String()
}

// inputWhenUnableToFormat handles input after formatting stopped. When
// that was not because of the user's own formatting, it may be a long
// international or national prefix, and extracting it may allow
// formatting again.
func (f *AsYouTypeFormatter) inputWhenUnableToFormat() string {
	switch {
	case f.inputHasFormatting:
	case f.extractIDD():
		if f.extractCountryCode() {
			return f.choosePatternWithPrefixExtracted()
		}
	case f.extractLongerNationalPrefix():
		// Separate a long national prefix for readability, but leave
		// addSpaceAfterNationalPrefix alone so a template does not
		// change it.
		f.prefixBeforeNationalNumber.WriteRune(separatorBeforeNationalNumber)
		return f.choosePatternWithPrefixExtracted()
	}
	return f.accruedInput.String()
}

func (f *AsYouTypeFormatter) isDigitOrLeadingPlus(r rune) bool {
	if phonetext.DigitValue(r) >= 0 {
		return true
	}
	return f.accruedInputRunes == 1 && (r == '+' || r == '\uFF0B')
}

// accrueDigitOrPlus records a digit, normalized to ASCII, or a plus sign,
// and returns the character recorded.
func (f *AsYouTypeFormatter) accrueDigitOrPlus(r rune, remember bool) rune {
	if r == '+' {
		f.accruedInputDigits.WriteRune(r)
	} else {
		// Upstream's Character.forDigit gives NUL for the full-width plus
		// sign, which is not a digit; so does this.
		var c rune
		if d := phonetext.DigitValue(r); d >= 0 {
			c = rune("0123456789"[d])
		}
		r = c
		f.accruedInputDigits.WriteRune(r)
		f.nationalNumber += string(r)
	}
	if remember {
		f.positionToRemember = f.accruedInputDigits.Len()
	}
	return r
}

func (f *AsYouTypeFormatter) choosePatternWithPrefixExtracted() string {
	f.ableToFormat = true
	f.expectingCountryCode = false
	f.possibleFormats = nil
	f.lastMatchPosition = 0
	f.template = nil
	f.currentFormat = ""
	return f.chooseFormattingPattern()
}

// chooseFormattingPattern chooses the rules for the number and returns the
// digits so far, formatted.
func (f *AsYouTypeFormatter) chooseFormattingPattern() string {
	// Wait for three digits of the national number, not counting the
	// national prefix.
	if len(f.nationalNumber) < minLeadingDigitsLength {
		return f.appendNationalNumber(f.nationalNumber)
	}
	f.availableFormats(f.nationalNumber)
	if s := f.formatAccruedDigits(); s != "" {
		return s
	}
	if f.maybeCreateNewTemplate() {
		return f.inputAccruedNationalNumber()
	}
	return f.accruedInput.String()
}

// availableFormats sets possibleFormats to the rules that fit the number
// so far.
func (f *AsYouTypeFormatter) availableFormats(leadingDigits string) {
	formats := f.meta.numberFormats
	if f.isCompleteNumber && f.extractedNationalPrefix == "" && len(f.meta.intlNumberFormats) > 0 {
		formats = f.meta.intlNumberFormats
	}
	for _, nf := range formats {
		firstGroupOnly := formattingRuleHasFirstGroupOnly(nf.nationalPrefixFormattingRule)
		switch {
		case f.extractedNationalPrefix != "" && firstGroupOnly &&
			!nf.nationalPrefixOptionalWhenFormatting && nf.carrierCodeFormattingRule == "":
			// The number has a national prefix, so leave out rules that
			// cannot have one. A rule with a carrier code rule stays: the
			// "national prefix" may really be a carrier code.
			continue
		case f.extractedNationalPrefix == "" && !f.isCompleteNumber && !firstGroupOnly &&
			!nf.nationalPrefixOptionalWhenFormatting:
			// No national prefix, but the rule needs one.
			continue
		}
		if eligibleFormat().MatchString(nf.format) {
			f.possibleFormats = append(f.possibleFormats, nf)
		}
	}
	f.narrowDownPossibleFormats(leadingDigits)
}

// formattingRuleHasFirstGroupOnly reports whether a national prefix
// formatting rule writes no national prefix, only the first group,
// perhaps in brackets.
func formattingRuleHasFirstGroupOnly(rule string) bool {
	return rule == "" || firstGroupOnlyPrefix().MatchString(rule)
}

// narrowDownPossibleFormats drops the rules whose leading digits do not
// fit. A rule's nth pattern is for n+3 digits; the last is used for more.
func (f *AsYouTypeFormatter) narrowDownPossibleFormats(leadingDigits string) {
	index := len(leadingDigits) - minLeadingDigitsLength
	kept := f.possibleFormats[:0]
	for _, nf := range f.possibleFormats {
		if n := len(nf.leadingDigits); n > 0 && !nf.leadingDigits[min(index, n-1)].matchPrefix(leadingDigits) {
			continue
		}
		kept = append(kept, nf)
	}
	f.possibleFormats = kept
}

// maybeCreateNewTemplate makes a template from the first possible rule
// that allows one, and reports whether it made a new one.
func (f *AsYouTypeFormatter) maybeCreateNewTemplate() bool {
	for i, nf := range f.possibleFormats {
		if f.currentFormat == nf.pattern.expr {
			// Upstream removes the rules tried before this one.
			f.possibleFormats = f.possibleFormats[i:]
			return false
		}
		if t := f.formattingTemplate(nf); t != nil {
			f.template = t
			f.currentFormat = nf.pattern.expr
			f.addSpaceAfterNationalPrefix = strings.ContainsAny(nf.nationalPrefixFormattingRule, "- ")
			// A new template starts matching from the start.
			f.lastMatchPosition = 0
			f.possibleFormats = f.possibleFormats[i:]
			return true
		}
	}
	f.possibleFormats = nil
	f.ableToFormat = false
	return false
}

// formattingTemplate returns the template for a rule: the rule applied to
// the longest number of nines its pattern matches, with each nine replaced
// by the placeholder. It returns nil when the number so far is already too
// long for the rule.
func (f *AsYouTypeFormatter) formattingTemplate(nf *numberFormat) []rune {
	re := nf.pattern.anyRegexp()
	longest := re.FindString("999999999999999")
	if len(longest) < len(f.nationalNumber) {
		return nil
	}
	s := replaceJava(re, longest, nf.format, true)
	t := []rune(s)
	for i, r := range t {
		if r == '9' {
			t[i] = digitPlaceholder
		}
	}
	if len(t) == 0 {
		return nil
	}
	return t
}

// formatAccruedDigits formats the national number with the first rule
// whose pattern matches all of it, as long as that adds or removes no
// digit, and returns "" otherwise. Mexican mobile rules, for one, drop the
// mobile token.
func (f *AsYouTypeFormatter) formatAccruedDigits() string {
	for _, nf := range f.possibleFormats {
		if !nf.pattern.matchFull(f.nationalNumber) {
			continue
		}
		f.addSpaceAfterNationalPrefix = strings.ContainsAny(nf.nationalPrefixFormattingRule, "- ")
		formatted := replaceJava(nf.pattern.anyRegexp(), f.nationalNumber, nf.format, true)
		out := f.appendNationalNumber(formatted)
		if normalizeDiallable(out) == f.accruedInputDigits.String() {
			return out
		}
	}
	return ""
}

// appendNationalNumber puts the prefix before the national number, with a
// space between when the rule writes one after the national prefix and
// there is not one already.
func (f *AsYouTypeFormatter) appendNationalNumber(nationalNumber string) string {
	prefix := f.prefixBeforeNationalNumber.String()
	if f.addSpaceAfterNationalPrefix && prefix != "" && prefix[len(prefix)-1] != separatorBeforeNationalNumber {
		return prefix + string(separatorBeforeNationalNumber) + nationalNumber
	}
	return prefix + nationalNumber
}

// inputAccruedNationalNumber puts every digit of the national number into
// the template and returns the result.
func (f *AsYouTypeFormatter) inputAccruedNationalNumber() string {
	if f.nationalNumber == "" {
		return f.prefixBeforeNationalNumber.String()
	}
	var temp string
	for _, r := range f.nationalNumber {
		temp = f.inputDigitHelper(r)
	}
	if f.ableToFormat {
		return f.appendNationalNumber(temp)
	}
	return f.accruedInput.String()
}

// inputDigitHelper puts r in the next placeholder of the template and
// returns the template up to it.
func (f *AsYouTypeFormatter) inputDigitHelper(r rune) string {
	for i := f.lastMatchPosition; i < len(f.template); i++ {
		if f.template[i] == digitPlaceholder {
			f.template[i] = r
			f.lastMatchPosition = i
			return string(f.template[:i+1])
		}
	}
	if len(f.possibleFormats) == 1 {
		// More digits than the only rule left can take.
		f.ableToFormat = false
	}
	f.currentFormat = ""
	return f.accruedInput.String()
}

// isNANPANumberWithNationalPrefix reports whether the number is in the
// NANPA and starts with the national prefix 1. National numbers there
// start with 2 to 9; those starting 10 or 11 are short numbers, which need
// no prefix.
func (f *AsYouTypeFormatter) isNANPANumberWithNationalPrefix() bool {
	n := f.nationalNumber
	return f.meta.countryCode == nanpaCountryCode && len(n) >= 2 && n[0] == '1' && n[1] != '0' && n[1] != '1'
}

// removeNationalPrefix moves a national prefix from the national number
// to the prefix, and returns it, or "" if there is none.
func (f *AsYouTypeFormatter) removeNationalPrefix() string {
	start := 0
	if f.isNANPANumberWithNationalPrefix() {
		start = 1
		f.prefixBeforeNationalNumber.WriteString("1 ")
		f.isCompleteNumber = true
	} else if f.meta.nationalPrefixForParsing != nil {
		// Some national prefix patterns are entirely optional, so check
		// that one was found.
		if loc := f.meta.nationalPrefixForParsing.prefixRegexp().FindStringIndex(f.nationalNumber); len(loc) == 2 && loc[1] > 0 {
			// With a national prefix, the number is complete: use the
			// international rules, as the national ones may include
			// local rules for numbers without an area code.
			f.isCompleteNumber = true
			start = loc[1]
			f.prefixBeforeNationalNumber.WriteString(f.nationalNumber[:start])
		}
	}
	np := f.nationalNumber[:start]
	f.nationalNumber = f.nationalNumber[start:]
	return np
}

// extractLongerNationalPrefix tries a longer national prefix, as some
// prefixes start with others, and reports whether it found a different one.
func (f *AsYouTypeFormatter) extractLongerNationalPrefix() bool {
	if np := f.extractedNationalPrefix; np != "" {
		// Put the prefix back in the national number. The prefix is not
		// simply cleared, since users sometimes type the national prefix
		// after the country calling code, as in +44 (0)20-1234-5678.
		f.nationalNumber = np + f.nationalNumber
		prefix := f.prefixBeforeNationalNumber.String()
		if i := strings.LastIndex(prefix, np); i >= 0 {
			f.prefixBeforeNationalNumber.Reset()
			f.prefixBeforeNationalNumber.WriteString(prefix[:i])
		}
	}
	return f.extractedNationalPrefix != f.removeNationalPrefix()
}

// extractIDD moves a plus sign or international dialling prefix to the
// prefix, and the rest to the national number, reporting whether there
// was one.
func (f *AsYouTypeFormatter) extractIDD() bool {
	idd := ""
	if f.meta.internationalPrefix != nil {
		idd = f.meta.internationalPrefix.expr
	}
	digits := f.accruedInputDigits.String()
	re := iddPatterns.get(idd)
	loc := re.FindStringIndex(digits)
	if loc == nil {
		return false
	}
	f.isCompleteNumber = true
	end := loc[1]
	f.nationalNumber = digits[end:]
	f.prefixBeforeNationalNumber.Reset()
	f.prefixBeforeNationalNumber.WriteString(digits[:end])
	if digits[0] != '+' {
		f.prefixBeforeNationalNumber.WriteRune(separatorBeforeNationalNumber)
	}
	return true
}

// extractCountryCode moves a country calling code from the start of the
// national number to the prefix, reporting whether there was one, and
// switches to that country's metadata.
func (f *AsYouTypeFormatter) extractCountryCode() bool {
	if f.nationalNumber == "" {
		return false
	}
	cc, rest := f.ms.splitCountryCode(f.nationalNumber)
	if cc == 0 {
		return false
	}
	f.nationalNumber = rest
	switch region := f.ms.mainRegion(cc); {
	case region == regionNonGeo:
		f.meta = f.ms.forCode(cc, region)
	case region != f.defaultCountry:
		f.meta = f.metadataFor(region)
	}
	f.prefixBeforeNationalNumber.WriteString(strconv.Itoa(cc))
	f.prefixBeforeNationalNumber.WriteRune(separatorBeforeNationalNumber)
	// A national prefix found before the international prefix no longer
	// counts.
	f.extractedNationalPrefix = ""
	return true
}

// iddPatterns caches, for each region's international prefix, the
// expression for a plus sign or that prefix at the start of the input. A
// region without one gets an empty alternative, as upstream does.
var iddPatterns = &regexpCache{m: map[string]*regexp.Regexp{}}

// regexpCache compiles each expression once.
type regexpCache struct {
	mu sync.Mutex
	m  map[string]*regexp.Regexp
}

func (c *regexpCache) get(idd string) *regexp.Regexp {
	c.mu.Lock()
	defer c.mu.Unlock()
	re, ok := c.m[idd]
	if !ok {
		re = regexp.MustCompile(`^(?:\+|` + idd + `)`)
		c.m[idd] = re
	}
	return re
}
