package phonenumber

import (
	"iter"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"uradical.io/go/phonenumber/internal/phonetext"
)

// Leniency is how strictly [FindNumbersWithLeniency] checks a candidate
// before reporting it. Each level accepts a subset of the one before.
type Leniency int

const (
	// LeniencyPossible accepts numbers with a possible length, valid or
	// not.
	LeniencyPossible Leniency = iota
	// LeniencyValid accepts valid numbers. A number in national format
	// must have its national prefix where the region usually writes one,
	// and the candidate must not run on from Latin letters, as in
	// "abc8005001234". [FindNumbers] uses this level.
	LeniencyValid
	// LeniencyStrictGrouping also requires the digits to be grouped in a
	// way the region could write them: "650 253 0000", "650 2530000" and
	// "6502530000" are accepted for a US number, but "65 02 53 00 00" and
	// "650253 0000" are not. More than one slash in the national number
	// is rejected. Numbers outside +1 may be missed.
	LeniencyStrictGrouping
	// LeniencyExactGrouping requires the digits to be grouped as they
	// would be formatted, or not at all: "650 253 0000" or "6502530000"
	// but not "650 2530000". Numbers outside +1 may be missed.
	LeniencyExactGrouping
)

var leniencyNames = [...]string{
	LeniencyPossible:       "LeniencyPossible",
	LeniencyValid:          "LeniencyValid",
	LeniencyStrictGrouping: "LeniencyStrictGrouping",
	LeniencyExactGrouping:  "LeniencyExactGrouping",
}

func (l Leniency) String() string {
	if l >= 0 && int(l) < len(leniencyNames) {
		return leniencyNames[l]
	}
	return "Leniency(" + strconv.Itoa(int(l)) + ")"
}

// FoundNumber is a phone number found in text by [FindNumbers].
type FoundNumber struct {
	// Start and End are the byte offsets of the number in the text, so
	// text[Start:End] is Raw.
	Start, End int
	// Raw is the number as written in the text.
	Raw string
	// Number is the parsed number.
	Number Number
}

// FindNumbers returns the phone numbers in text, in order, at
// [LeniencyValid]. region is the region to assume for numbers written
// without a country calling code; with "", only numbers with a plus sign
// or an international prefix are found.
//
// Numbers written with letters, such as 1-800-SIX-FLAG, are not found.
func FindNumbers(text, region string) iter.Seq[FoundNumber] {
	return FindNumbersWithLeniency(text, region, LeniencyValid, -1)
}

// FindNumbersWithLeniency is [FindNumbers] at the given leniency, giving
// up after maxTries candidates that are not numbers, for text with many
// false positives. A negative maxTries means no limit.
func FindNumbersWithLeniency(text, region string, leniency Leniency, maxTries int) iter.Seq[FoundNumber] {
	return defaultSet().findNumbers(text, strings.ToUpper(region), leniency, maxTries)
}

func (ms *metadataSet) findNumbers(text, region string, leniency Leniency, maxTries int) iter.Seq[FoundNumber] {
	return func(yield func(FoundNumber) bool) {
		f := &finder{ms: ms, text: text, region: region, leniency: leniency, triesLeft: maxTries}
		for index := 0; ; {
			m, ok := f.find(index)
			if !ok || !yield(m) {
				return
			}
			index = m.End
		}
	}
}

// The matcher's expressions, as upstream builds them.
const (
	openingParens = `(\[\x{FF08}\x{FF3B}`
	closingParens = `)\]\x{FF09}\x{FF3D}`
	nonParens     = `[^` + openingParens + closingParens + `]`
	leadClass     = `[` + openingParens + plusChars + `]`
	// Upstream allows as many digits in one block as in a national number
	// and a country calling code together.
	digitBlockLimit = "20"
	matchPunct      = `[` + validPunctuation + `]{0,4}`
	digitSequence   = digits + `{1,` + digitBlockLimit + `}`
)

var (
	// candidatePattern finds text that might be a phone number. Unlike
	// the parser's pattern, every repetition is bounded, no space may
	// start or end it, and letters are not allowed.
	candidatePattern = lazyRegexp(`(?i)(?:` + leadClass + matchPunct + `){0,2}` +
		digitSequence + `(?:` + matchPunct + digitSequence + `){0,` + digitBlockLimit + `}` +
		`(?:` + extnPattern(false) + `)?`)
	// matchingBrackets checks that brackets are closed within a number
	// and have something inside. An opening bracket at the start may be
	// left open, and a closing one may come first if its opening bracket
	// was dropped.
	matchingBrackets = lazyRegexp(`^(?:[` + openingParens + `])?(?:` + nonParens + `+[` + closingParens + `])?` +
		nonParens + `+(?:[` + openingParens + `]` + nonParens + `+[` + closingParens + `]){0,3}` + nonParens + `*$`)
	leadClassPattern = lazyRegexp(`^` + leadClass)
	// pubPages matches publication pages such as "211-227 (2003)".
	pubPages = lazyRegexp(`\d{1,5}-+\d{1,5}\s{0,4}\(\d{1,4}`)
	// slashSeparatedDates matches dates such as 3/10/2011 or 08/31/95.
	slashSeparatedDates = lazyRegexp(`(?:(?:[0-3]?\d/[01]?\d)|(?:[01]?\d/[0-3]?\d))/(?:[12]\d)?\d{2}`)
	// timeStamps matches the start of a timestamp such as
	// "2012-01-02 08:00"; timeStampsSuffix matches the ":00" after it.
	timeStamps       = lazyRegexp(`[12]\d{3}[-/]?[01]\d[-/]?[0-3]\d +[0-2]\d$`)
	timeStampsSuffix = lazyRegexp(`^:[0-5]\d`)
	// innerMatches split a candidate that is not one number into the
	// numbers inside it, most specific first: white space comes last as
	// it is common within numbers too. Each run breaks on one kind of
	// symbol only.
	innerMatches = []func() *regexp.Regexp{
		// "651-234-2345/332-445-1234"
		lazyRegexp(`/+(.*)`),
		// "(650) 223 3345 (754) 223 3321"; the bracket is part of the
		// number.
		lazyRegexp(`(\([^(]*)`),
		// "12345 - 332-445-1234 is my number", with a space beside the
		// hyphen.
		lazyRegexp(`(?:\pZ-|-\pZ)\pZ*(.+)`),
		// Wide hyphens, which are not used within numbers.
		lazyRegexp(`[\x{2012}-\x{2015}\x{FF0D}]\pZ*(.+)`),
		// "12345. 332-445-1234 is my number"
		lazyRegexp(`\.+\pZ*([^.]+)`),
		// "3324451234 8002341234"
		lazyRegexp(`\pZ+(\PZ+)`),
	}
	firstGroupOnlyPrefix = lazyRegexp(`^\(?\$1\)?$`)
)

// finder finds numbers in one text.
type finder struct {
	ms        *metadataSet
	text      string
	region    string
	leniency  Leniency
	triesLeft int // negative for no limit
}

func (f *finder) canTry() bool { return f.triesLeft != 0 }

func (f *finder) tried() {
	if f.triesLeft > 0 {
		f.triesLeft--
	}
}

// find returns the first number at or after index.
func (f *finder) find(index int) (FoundNumber, bool) {
	for f.canTry() && index <= len(f.text) {
		loc := candidatePattern().FindStringIndex(f.text[index:])
		if loc == nil {
			break
		}
		start := index + loc[0]
		candidate := f.text[start : index+loc[1]]
		// Leave out any second number, as in "650 253 0000/x123".
		candidate = phonetext.TrimSecondNumber(candidate)
		if m, ok := f.extractMatch(candidate, start); ok {
			return m, true
		}
		index = start + len(candidate)
		f.tried()
	}
	return FoundNumber{}, false
}

// extractMatch returns the number in candidate, which starts at offset in
// the text: all of it, or failing that a part.
func (f *finder) extractMatch(candidate string, offset int) (FoundNumber, bool) {
	// Skip what looks more like a date or a timestamp.
	if slashSeparatedDates().MatchString(candidate) {
		return FoundNumber{}, false
	}
	if timeStamps().MatchString(candidate) && timeStampsSuffix().MatchString(f.text[offset+len(candidate):]) {
		return FoundNumber{}, false
	}
	if m, ok := f.parseAndVerify(candidate, offset); ok {
		return m, true
	}
	return f.extractInnerMatch(candidate, offset)
}

// extractInnerMatch looks for a number within candidate when candidate as
// a whole is not one.
func (f *finder) extractInnerMatch(candidate string, offset int) (FoundNumber, bool) {
	for _, re := range innerMatches {
		first := true
		for _, m := range re().FindAllStringSubmatchIndex(candidate, -1) {
			if !f.canTry() {
				break
			}
			if first {
				// Try the text before the first match too.
				group := phonetext.TrimUnwantedEnd(candidate[:m[0]])
				if found, ok := f.parseAndVerify(group, offset); ok {
					return found, true
				}
				f.tried()
				first = false
			}
			group := phonetext.TrimUnwantedEnd(candidate[m[2]:m[3]])
			if found, ok := f.parseAndVerify(group, offset+m[2]); ok {
				return found, true
			}
			f.tried()
		}
	}
	return FoundNumber{}, false
}

// parseAndVerify parses candidate and checks it at the finder's leniency.
func (f *finder) parseAndVerify(candidate string, offset int) (FoundNumber, bool) {
	// Formatting that suggests it is not a phone number.
	if !matchingBrackets().MatchString(candidate) || pubPages().MatchString(candidate) {
		return FoundNumber{}, false
	}
	// From LeniencyValid on, skip numbers that run on from Latin letters,
	// as in abc8005001234 or 8005001234def.
	if f.leniency >= LeniencyValid {
		if offset > 0 && !leadClassPattern().MatchString(candidate) {
			prev, _ := utf8.DecodeLastRuneInString(f.text[:offset])
			if isInvalidPunctuation(prev) || isLatinLetter(prev) {
				return FoundNumber{}, false
			}
		}
		if end := offset + len(candidate); end < len(f.text) {
			next, _ := utf8.DecodeRuneInString(f.text[end:])
			if isInvalidPunctuation(next) || isLatinLetter(next) {
				return FoundNumber{}, false
			}
		}
	}
	p, err := f.ms.parseAndKeepRawInput(candidate, f.region)
	if err != nil || !f.ms.verify(f.leniency, p, candidate) {
		return FoundNumber{}, false
	}
	return FoundNumber{Start: offset, End: offset + len(candidate), Raw: candidate, Number: p.Number}, true
}

// isLatinLetter reports whether r is a Latin letter, or a combining mark,
// which is assumed to follow one.
func isLatinLetter(r rune) bool {
	if !unicode.IsLetter(r) && !unicode.Is(unicode.Mn, r) {
		return false
	}
	// Java's Basic Latin, Latin-1 Supplement, Latin Extended-A and -B,
	// Combining Diacritical Marks and Latin Extended Additional blocks.
	return r <= 0x024F || (0x0300 <= r && r <= 0x036F) || (0x1E00 <= r && r <= 0x1EFF)
}

func isInvalidPunctuation(r rune) bool {
	return r == '%' || unicode.Is(unicode.Sc, r)
}

// verify checks a parsed candidate at the given leniency.
func (ms *metadataSet) verify(l Leniency, p Parsed, candidate string) bool {
	n := p.Number
	if l == LeniencyPossible {
		c := ms.checkLength(n, TypeUnknown)
		return c == LengthOK || c == LengthLocalOnly
	}
	if !ms.valid(n) || !ms.onlyValidXChars(n, candidate) {
		return false
	}
	if l >= LeniencyStrictGrouping && moreThanOneSlashInNationalNumber(p, candidate) {
		return false
	}
	if !ms.nationalPrefixPresentIfRequired(p) {
		return false
	}
	switch l {
	case LeniencyStrictGrouping:
		return ms.groupingIsValid(p, candidate, allGroupsRemainGrouped)
	case LeniencyExactGrouping:
		return ms.groupingIsValid(p, candidate, allGroupsExactlyPresent)
	}
	return true
}

// onlyValidXChars checks the x and X characters in candidate. Two in a row
// come before a carrier code, which precedes the national number; one
// alone comes before an extension. An x at the very end is ignored.
func (ms *metadataSet) onlyValidXChars(n Number, candidate string) bool {
	for i := 0; i < len(candidate)-1; i++ {
		if c := candidate[i]; c != 'x' && c != 'X' {
			continue
		}
		if next := candidate[i+1]; next == 'x' || next == 'X' {
			i++
			if ms.matchString(n, candidate[i:]) != MatchNSN {
				return false
			}
		} else if phonetext.NormalizeDigits(candidate[i:], false) != n.ext {
			return false
		}
	}
	return true
}

// moreThanOneSlashInNationalNumber reports whether candidate has two
// slashes in its national number. One slash, or one after the country
// calling code, is allowed.
func moreThanOneSlashInNationalNumber(p Parsed, candidate string) bool {
	first := strings.IndexByte(candidate, '/')
	if first < 0 {
		return false
	}
	second := strings.IndexByte(candidate[first+1:], '/')
	if second < 0 {
		return false
	}
	second += first + 1
	hasCountryCode := p.Source == FromNumberWithPlusSign || p.Source == FromNumberWithoutPlusSign
	if hasCountryCode && phonetext.NormalizeDigits(candidate[:first], false) == strconv.Itoa(int(p.Number.countryCode)) {
		return strings.Contains(candidate[second+1:], "/")
	}
	return true
}

// nationalPrefixPresentIfRequired checks that a number written without a
// country calling code has a national prefix, when its region writes one
// for numbers like it.
func (ms *metadataSet) nationalPrefixPresentIfRequired(p Parsed) bool {
	if p.Source != FromDefaultCountry {
		return true
	}
	n := p.Number
	meta := ms.region(ms.mainRegion(int(n.countryCode)))
	if meta == nil {
		return true
	}
	rule := chooseFormat(meta.numberFormats, n.NationalNumber())
	// A rule with no national prefix, or one that is optional, or only
	// the first group in brackets, does not need one.
	if rule == nil || rule.nationalPrefixFormattingRule == "" ||
		rule.nationalPrefixOptionalWhenFormatting ||
		firstGroupOnlyPrefix().MatchString(rule.nationalPrefixFormattingRule) {
		return true
	}
	_, _, ok := meta.stripNationalPrefix(phonetext.NormalizeDigits(p.Raw, false))
	return ok
}

// groupChecker reports whether the digits of normalized, a candidate with
// its digits made ASCII, are grouped as groups.
type groupChecker func(ms *metadataSet, p Parsed, normalized string, groups []string) bool

// groupingIsValid checks the grouping of candidate against the number's
// format, then against its country's alternate formats.
func (ms *metadataSet) groupingIsValid(p Parsed, candidate string, check groupChecker) bool {
	normalized := phonetext.NormalizeDigits(candidate, true)
	n := p.Number
	if check(ms, p, normalized, ms.nationalNumberGroups(n)) {
		return true
	}
	nsn := n.NationalNumber()
	for _, alt := range alternateFormatData[int(n.countryCode)] {
		// Alternate formats have one leading digits pattern at most.
		if len(alt.leadingDigits) > 0 && !alt.leadingDigits[0].matchPrefix(nsn) {
			continue
		}
		groups := splitJava(formatNSNWithRule(nsn, alt, RFC3966, ""), "-")
		if check(ms, p, normalized, groups) {
			return true
		}
	}
	return false
}

// nationalNumberGroups returns the groups of digits of n's national number
// as the metadata formats it, without a national prefix.
func (ms *metadataSet) nationalNumberGroups(n Number) []string {
	// "tel:+CC-G1-G2-...;ext=X": the groups lie between the first dash and
	// any semicolon.
	s := ms.format(n, RFC3966)
	if i := strings.IndexByte(s, ';'); i >= 0 {
		s = s[:i]
	}
	s = s[strings.IndexByte(s, '-')+1:]
	return splitJava(s, "-")
}

// splitJava splits s around sep as Java's String.split does: trailing
// empty strings are dropped.
func splitJava(s, sep string) []string {
	parts := strings.Split(s, sep)
	for len(parts) > 1 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	if len(parts) == 1 && parts[0] == "" && strings.Contains(s, sep) {
		return nil
	}
	return parts
}

// allGroupsRemainGrouped reports whether each group of digits is together
// in normalized, in order, though normalized may group them further.
func allGroupsRemainGrouped(ms *metadataSet, p Parsed, normalized string, groups []string) bool {
	n := p.Number
	from := 0
	if p.Source != FromDefaultCountry {
		// Skip the country calling code.
		cc := strconv.Itoa(int(n.countryCode))
		// Index returns -1 when cc is missing, as upstream's indexOf does.
		from = strings.Index(normalized, cc) + len(cc)
	}
	for i, g := range groups {
		j := strings.Index(normalized[min(from, len(normalized)):], g)
		if j < 0 {
			return false
		}
		from += j + len(g)
		if i == 0 && from < len(normalized) {
			// Right after the national destination code. Without a
			// formatting symbol here, a region with a national prefix
			// accepts the number only if it has no formatting at all,
			// apart from an extension.
			if ms.nationalPrefix(ms.mainRegion(int(n.countryCode))) != "" && isASCIIDigit(normalized[from]) {
				return strings.HasPrefix(normalized[from-len(g):], n.NationalNumber())
			}
		}
	}
	// The extension must not have been used for the last group; it has
	// no formatting within it.
	return strings.Contains(normalized[min(from, len(normalized)):], n.ext)
}

func isASCIIDigit(c byte) bool { return '0' <= c && c <= '9' }

// allGroupsExactlyPresent reports whether normalized is grouped exactly
// as groups, or is one block.
func allGroupsExactlyPresent(ms *metadataSet, p Parsed, normalized string, groups []string) bool {
	n := p.Number
	cand := splitNonDigits(normalized)
	// The last group, or the one before an extension.
	ci := len(cand) - 1
	if n.ext != "" {
		ci--
	}
	// The national number may be one block, perhaps after a national
	// prefix or country calling code.
	if len(cand) == 1 || (ci >= 0 && strings.Contains(cand[ci], n.NationalNumber())) {
		return true
	}
	// Compare the groups from the end, all but the first.
	gi := len(groups) - 1
	for ; gi > 0 && ci >= 0; gi, ci = gi-1, ci-1 {
		if cand[ci] != groups[gi] {
			return false
		}
	}
	// The first group may follow a national prefix.
	return ci >= 0 && strings.HasSuffix(cand[ci], groups[0])
}

// splitNonDigits splits s around runs of non-digits as Java's String.split
// does with the pattern \D+: a leading empty string is kept, trailing ones
// are dropped.
func splitNonDigits(s string) []string {
	parts := nonDigits().Split(s, -1)
	for len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	if len(parts) == 0 && !nonDigits().MatchString(s) {
		return []string{s}
	}
	return parts
}
