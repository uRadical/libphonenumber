// Command genmetadata turns libphonenumber's XML metadata into Go source.
//
// It follows the rules of upstream's BuildMetadataFromXml: whitespace is
// stripped from patterns, $NP and $FG are expanded in formatting rules, the
// general description's possible lengths are derived from the number types,
// and a type's lengths are omitted when they equal the general description's.
// Every pattern is compiled with Go's regexp package, so an expression that
// RE2 cannot run fails the build instead of the program that uses it.
//
// Usage (normally through go generate in the module root):
//
//	genmetadata -in resources/PhoneNumberMetadata.xml -out metadata_gen.go -var territoryData
package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io"
	"log"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("genmetadata: ")
	if err := run(os.Args[1:], os.Stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(2)
		}
		log.Fatal(err)
	}
}

// run parses the command line, then converts one XML file to Go source.
func run(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("genmetadata", flag.ContinueOnError)
	fs.SetOutput(stderr)
	in := fs.String("in", "", "XML metadata file to read")
	out := fs.String("out", "", "Go file to write")
	varName := fs.String("var", "territoryData", "name of the generated variable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *in == "" || *out == "" {
		fs.Usage()
		return flag.ErrHelp
	}
	raw, err := os.ReadFile(*in)
	if err != nil {
		return err
	}
	src, err := generate(raw, *varName, *in)
	if err != nil {
		return err
	}
	return os.WriteFile(*out, src, 0o644)
}

// generate converts XML metadata to formatted Go source declaring varName.
// source names the input in the generated header and in errors.
func generate(raw []byte, varName, source string) ([]byte, error) {
	var doc xmlDocument
	if err := xml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", source, err)
	}
	ts := make([]territory, 0, len(doc.Territories))
	for i := range doc.Territories {
		xt := &doc.Territories[i]
		t, err := buildTerritory(xt)
		if err != nil {
			return nil, fmt.Errorf("%s: territory %s (+%s): %w", source, xt.ID, xt.CountryCode, err)
		}
		ts = append(ts, t)
	}
	return render(ts, varName, source)
}

// XML shape. Pointers distinguish an absent attribute from an empty one,
// which upstream's rules depend on.

type xmlDocument struct {
	Territories []xmlTerritory `xml:"territories>territory"`
}

type xmlTerritory struct {
	ID                                   string  `xml:"id,attr"`
	CountryCode                          string  `xml:"countryCode,attr"`
	MainCountryForCode                   *string `xml:"mainCountryForCode,attr"`
	LeadingDigits                        *string `xml:"leadingDigits,attr"`
	PreferredInternationalPrefix         *string `xml:"preferredInternationalPrefix,attr"`
	InternationalPrefix                  *string `xml:"internationalPrefix,attr"`
	NationalPrefix                       *string `xml:"nationalPrefix,attr"`
	NationalPrefixForParsing             *string `xml:"nationalPrefixForParsing,attr"`
	NationalPrefixTransformRule          *string `xml:"nationalPrefixTransformRule,attr"`
	PreferredExtnPrefix                  *string `xml:"preferredExtnPrefix,attr"`
	NationalPrefixFormattingRule         *string `xml:"nationalPrefixFormattingRule,attr"`
	NationalPrefixOptionalWhenFormatting *string `xml:"nationalPrefixOptionalWhenFormatting,attr"`
	CarrierCodeFormattingRule            *string `xml:"carrierCodeFormattingRule,attr"`
	MobileNumberPortableRegion           *string `xml:"mobileNumberPortableRegion,attr"`

	Formats []xmlNumberFormat `xml:"availableFormats>numberFormat"`

	GeneralDesc             *xmlDesc `xml:"generalDesc"`
	NoInternationalDialling *xmlDesc `xml:"noInternationalDialling"`
	FixedLine               *xmlDesc `xml:"fixedLine"`
	Mobile                  *xmlDesc `xml:"mobile"`
	Pager                   *xmlDesc `xml:"pager"`
	TollFree                *xmlDesc `xml:"tollFree"`
	PremiumRate             *xmlDesc `xml:"premiumRate"`
	SharedCost              *xmlDesc `xml:"sharedCost"`
	PersonalNumber          *xmlDesc `xml:"personalNumber"`
	VoIP                    *xmlDesc `xml:"voip"`
	UAN                     *xmlDesc `xml:"uan"`
	Voicemail               *xmlDesc `xml:"voicemail"`
}

type xmlNumberFormat struct {
	Pattern                              string   `xml:"pattern,attr"`
	NationalPrefixFormattingRule         *string  `xml:"nationalPrefixFormattingRule,attr"`
	NationalPrefixOptionalWhenFormatting *string  `xml:"nationalPrefixOptionalWhenFormatting,attr"`
	CarrierCodeFormattingRule            *string  `xml:"carrierCodeFormattingRule,attr"`
	LeadingDigits                        []string `xml:"leadingDigits"`
	Format                               []string `xml:"format"`
	IntlFormat                           []string `xml:"intlFormat"`
}

type xmlDesc struct {
	PossibleLengths       []xmlPossibleLengths `xml:"possibleLengths"`
	ExampleNumber         *string              `xml:"exampleNumber"`
	NationalNumberPattern *string              `xml:"nationalNumberPattern"`
}

type xmlPossibleLengths struct {
	National  string  `xml:"national,attr"`
	LocalOnly *string `xml:"localOnly,attr"`
}

// Processed metadata, mirroring the runtime types in the phonenumber package.

type territory struct {
	id                            string
	countryCode                   int
	leadingDigits                 string
	internationalPrefix           string
	preferredInternationalPrefix  string
	nationalPrefix                string
	nationalPrefixForParsing      string
	nationalPrefixTransformRule   string
	preferredExtnPrefix           string
	mainCountryForCode            bool
	mobileNumberPortableRegion    bool
	sameMobileAndFixedLinePattern bool

	numberFormats     []numberFormat
	intlNumberFormats []numberFormat

	generalDesc, fixedLine, mobile, tollFree, premiumRate, sharedCost,
	personalNumber, voip, pager, uan, voicemail, noInternationalDialling desc
}

type numberFormat struct {
	pattern                              string
	format                               string
	leadingDigits                        []string
	nationalPrefixFormattingRule         string
	nationalPrefixOptionalWhenFormatting bool
	carrierCodeFormattingRule            string
}

type desc struct {
	pattern   string
	example   string
	lengths   []int // nil: same as the general description; [-1]: no numbers of this type
	localOnly []int
}

func buildTerritory(x *xmlTerritory) (territory, error) {
	var t territory
	t.id = x.ID
	cc, err := strconv.Atoi(x.CountryCode)
	if err != nil {
		return t, fmt.Errorf("countryCode: %w", err)
	}
	t.countryCode = cc

	np := deref(x.NationalPrefix)
	if x.LeadingDigits != nil {
		if t.leadingDigits, err = checkRegexp(*x.LeadingDigits, false); err != nil {
			return t, err
		}
	}
	if x.InternationalPrefix != nil {
		if t.internationalPrefix, err = checkRegexp(*x.InternationalPrefix, false); err != nil {
			return t, err
		}
	}
	t.preferredInternationalPrefix = deref(x.PreferredInternationalPrefix)
	if x.NationalPrefixForParsing != nil {
		if t.nationalPrefixForParsing, err = checkRegexp(*x.NationalPrefixForParsing, true); err != nil {
			return t, err
		}
		if x.NationalPrefixTransformRule != nil {
			t.nationalPrefixTransformRule = *x.NationalPrefixTransformRule
		}
	}
	if np != "" {
		t.nationalPrefix = np
		if x.NationalPrefixForParsing == nil {
			t.nationalPrefixForParsing = np
		}
	}
	t.preferredExtnPrefix = deref(x.PreferredExtnPrefix)
	t.mainCountryForCode = x.MainCountryForCode != nil
	t.mobileNumberPortableRegion = x.MobileNumberPortableRegion != nil

	if err := buildFormats(&t, x, np); err != nil {
		return t, err
	}
	if err := buildDescs(&t, x); err != nil {
		return t, err
	}
	return t, nil
}

// formatDefaults holds the territory-level rules a numberFormat inherits
// when it does not set its own.
type formatDefaults struct {
	nationalPrefix string
	npRule         string
	optional       bool
	ccRule         string
}

func buildFormats(t *territory, x *xmlTerritory, np string) error {
	d := formatDefaults{
		nationalPrefix: np,
		npRule:         expandRule(deref(x.NationalPrefixFormattingRule), np),
		optional:       x.NationalPrefixOptionalWhenFormatting != nil,
	}
	if x.CarrierCodeFormattingRule != nil {
		r, err := checkRegexp(expandRule(*x.CarrierCodeFormattingRule, np), false)
		if err != nil {
			return err
		}
		d.ccRule = r
	}
	explicitIntl := false
	for i := range x.Formats {
		xf := &x.Formats[i]
		f, err := buildFormat(xf, &d)
		if err != nil {
			return err
		}
		t.numberFormats = append(t.numberFormats, f)
		intl, explicit, err := intlFormat(xf, &f)
		if err != nil {
			return err
		}
		explicitIntl = explicitIntl || explicit
		if intl != nil {
			t.intlNumberFormats = append(t.intlNumberFormats, *intl)
		}
	}
	// Most regions format numbers the same way nationally and
	// internationally; only keep the international list when it differs.
	if !explicitIntl {
		t.intlNumberFormats = nil
	}
	return nil
}

// buildFormat builds the national format of one numberFormat element.
func buildFormat(xf *xmlNumberFormat, d *formatDefaults) (numberFormat, error) {
	f := numberFormat{
		nationalPrefixFormattingRule:         d.npRule,
		nationalPrefixOptionalWhenFormatting: d.optional,
		carrierCodeFormattingRule:            d.ccRule,
	}
	if xf.NationalPrefixFormattingRule != nil {
		f.nationalPrefixFormattingRule = expandRule(*xf.NationalPrefixFormattingRule, d.nationalPrefix)
	}
	if xf.NationalPrefixOptionalWhenFormatting != nil {
		f.nationalPrefixOptionalWhenFormatting = *xf.NationalPrefixOptionalWhenFormatting == "true"
	}
	if xf.CarrierCodeFormattingRule != nil {
		r, err := checkRegexp(expandRule(*xf.CarrierCodeFormattingRule, d.nationalPrefix), false)
		if err != nil {
			return f, err
		}
		f.carrierCodeFormattingRule = r
	}
	for _, ld := range xf.LeadingDigits {
		r, err := checkRegexp(ld, true)
		if err != nil {
			return f, err
		}
		f.leadingDigits = append(f.leadingDigits, r)
	}
	p, err := checkRegexp(xf.Pattern, false)
	if err != nil {
		return f, err
	}
	f.pattern = p
	if len(xf.Format) != 1 {
		return f, fmt.Errorf("numberFormat %q has %d format elements, want 1", xf.Pattern, len(xf.Format))
	}
	f.format = xf.Format[0]
	return f, nil
}

// intlFormat returns the international counterpart of the national format
// f, or nil when the element says "NA". explicit reports whether the element
// has an intlFormat of its own rather than reusing the national one.
func intlFormat(xf *xmlNumberFormat, f *numberFormat) (intl *numberFormat, explicit bool, err error) {
	switch len(xf.IntlFormat) {
	case 0:
		return f, false, nil
	case 1:
		if xf.IntlFormat[0] == "NA" {
			return nil, true, nil
		}
		// Upstream builds the international format from the pattern and
		// leading digits only: no national prefix or carrier rules.
		return &numberFormat{
			pattern:       f.pattern,
			format:        xf.IntlFormat[0],
			leadingDigits: f.leadingDigits,
		}, true, nil
	}
	return nil, false, fmt.Errorf("numberFormat %q has %d intlFormat elements, want at most 1", xf.Pattern, len(xf.IntlFormat))
}

func buildDescs(t *territory, x *xmlTerritory) error {
	general, err := processDesc(nil, x.GeneralDesc)
	if err != nil {
		return fmt.Errorf("generalDesc: %w", err)
	}
	if x.GeneralDesc != nil && len(x.GeneralDesc.PossibleLengths) > 0 {
		return fmt.Errorf("generalDesc must not declare possible lengths")
	}
	// The general description's lengths are the union of every type's,
	// except noInternationalDialling, which is not a type of its own.
	var lengths, local []int
	for _, d := range []*xmlDesc{x.FixedLine, x.Mobile, x.Pager, x.TollFree, x.PremiumRate,
		x.SharedCost, x.PersonalNumber, x.VoIP, x.UAN, x.Voicemail} {
		if d == nil {
			continue
		}
		l, lo, err := possibleLengthSets(d)
		if err != nil {
			return err
		}
		lengths = append(lengths, l...)
		local = append(local, lo...)
	}
	general.lengths, general.localOnly, err = setPossibleLengths(sortedSet(lengths), sortedSet(local), nil)
	if err != nil {
		return fmt.Errorf("generalDesc: %w", err)
	}
	t.generalDesc = general

	for _, s := range []struct {
		name string
		dst  *desc
		src  *xmlDesc
	}{
		{"fixedLine", &t.fixedLine, x.FixedLine},
		{"mobile", &t.mobile, x.Mobile},
		{"sharedCost", &t.sharedCost, x.SharedCost},
		{"voip", &t.voip, x.VoIP},
		{"personalNumber", &t.personalNumber, x.PersonalNumber},
		{"pager", &t.pager, x.Pager},
		{"uan", &t.uan, x.UAN},
		{"voicemail", &t.voicemail, x.Voicemail},
		{"noInternationalDialling", &t.noInternationalDialling, x.NoInternationalDialling},
		{"tollFree", &t.tollFree, x.TollFree},
		{"premiumRate", &t.premiumRate, x.PremiumRate},
	} {
		d, err := processDesc(&general, s.src)
		if err != nil {
			return fmt.Errorf("%s: %w", s.name, err)
		}
		*s.dst = d
	}
	t.sameMobileAndFixedLinePattern = t.mobile.pattern == t.fixedLine.pattern
	return nil
}

func processDesc(parent *desc, x *xmlDesc) (desc, error) {
	if x == nil {
		// -1 never matches a length, so a missing type matches nothing.
		// An empty list would instead mean "same as the general description".
		return desc{lengths: []int{-1}}, nil
	}
	var d desc
	if parent != nil {
		l, lo, err := possibleLengthSets(x)
		if err != nil {
			return d, err
		}
		if d.lengths, d.localOnly, err = setPossibleLengths(l, lo, parent); err != nil {
			return d, err
		}
	}
	if x.NationalNumberPattern != nil {
		p, err := checkRegexp(*x.NationalNumberPattern, true)
		if err != nil {
			return d, err
		}
		d.pattern = p
	}
	if x.ExampleNumber != nil {
		d.example = *x.ExampleNumber
	}
	return d, nil
}

func possibleLengthSets(x *xmlDesc) (lengths, localOnly []int, err error) {
	for _, pl := range x.PossibleLengths {
		n, err := parseLengths(pl.National)
		if err != nil {
			return nil, nil, err
		}
		if pl.LocalOnly != nil {
			lo, err := parseLengths(*pl.LocalOnly)
			if err != nil {
				return nil, nil, err
			}
			for _, l := range lo {
				if slices.Contains(n, l) {
					return nil, nil, fmt.Errorf("length %d is both national and local-only", l)
				}
			}
			localOnly = append(localOnly, lo...)
		}
		lengths = append(lengths, n...)
	}
	return sortedSet(lengths), sortedSet(localOnly), nil
}

// parseLengths reads upstream's length syntax, such as "[3-5],7,9".
func parseLengths(s string) ([]int, error) {
	if s == "" {
		return nil, fmt.Errorf("empty possible length string")
	}
	var out []int
	for part := range strings.SplitSeq(s, ",") {
		ns, err := parseLengthPart(part)
		if err != nil {
			return nil, fmt.Errorf("possible lengths %q: %w", s, err)
		}
		for _, n := range ns {
			if slices.Contains(out, n) {
				return nil, fmt.Errorf("possible lengths %q: duplicate length %d", s, n)
			}
			out = append(out, n)
		}
	}
	return out, nil
}

// parseLengthPart reads one comma-separated part of a length string: a
// number, or a range such as "[3-5]".
func parseLengthPart(part string) ([]int, error) {
	if part == "" {
		return nil, fmt.Errorf("stray comma")
	}
	if part[0] != '[' {
		n, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("bad length %q", part)
		}
		return []int{n}, nil
	}
	body, ok := strings.CutSuffix(part[1:], "]")
	if !ok {
		return nil, fmt.Errorf("unterminated range %q", part)
	}
	lo, hi, ok := strings.Cut(body, "-")
	if !ok {
		return nil, fmt.Errorf("range %q has no '-'", part)
	}
	minLen, err1 := strconv.Atoi(lo)
	maxLen, err2 := strconv.Atoi(hi)
	if err1 != nil || err2 != nil {
		return nil, fmt.Errorf("bad range %q", part)
	}
	// Upstream prefers "6,7" to "[6-7]": a range must save a digit.
	if maxLen-minLen < 2 {
		return nil, fmt.Errorf("range %q should be written as a list", part)
	}
	var ns []int
	for n := minLen; n <= maxLen; n++ {
		ns = append(ns, n)
	}
	return ns, nil
}

// setPossibleLengths applies upstream's inheritance rules: a type whose
// lengths equal the general description's stores none, and every length
// must be covered by the parent.
func setPossibleLengths(lengths, localOnly []int, parent *desc) (outLengths, outLocal []int, err error) {
	if parent == nil || !slices.Equal(lengths, parent.lengths) {
		for _, l := range lengths {
			if parent != nil && !slices.Contains(parent.lengths, l) {
				return nil, nil, fmt.Errorf("length %d not covered by parent lengths %v", l, parent.lengths)
			}
			outLengths = append(outLengths, l)
		}
	}
	for _, l := range localOnly {
		if slices.Contains(lengths, l) {
			continue
		}
		if parent != nil && !slices.Contains(parent.localOnly, l) && !slices.Contains(parent.lengths, l) {
			return nil, nil, fmt.Errorf("local-only length %d not covered by parent", l)
		}
		outLocal = append(outLocal, l)
	}
	return outLengths, outLocal, nil
}

func sortedSet(s []int) []int {
	s = slices.Clone(s)
	slices.Sort(s)
	return slices.Compact(s)
}

// checkRegexp optionally strips whitespace from a metadata expression, then
// makes sure Go's regexp package accepts it.
func checkRegexp(expr string, stripSpace bool) (string, error) {
	if stripSpace {
		expr = strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return -1
			}
			return r
		}, expr)
	}
	if _, err := regexp.Compile(expr); err != nil {
		return "", fmt.Errorf("pattern %q: %w", expr, err)
	}
	if strings.Contains(expr, "|)") {
		return "", fmt.Errorf("pattern %q: '|' directly followed by ')'", expr)
	}
	return expr, nil
}

// expandRule replaces the first $NP with the national prefix and the first
// $FG with the first group, as upstream does when loading metadata.
func expandRule(rule, nationalPrefix string) string {
	rule = strings.Replace(rule, "$NP", nationalPrefix, 1)
	return strings.Replace(rule, "$FG", "$1", 1)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Rendering.

func render(ts []territory, varName, source string) ([]byte, error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated by genmetadata from %s. DO NOT EDIT.\n\n", source)
	b.WriteString("package phonenumber\n\n")
	fmt.Fprintf(&b, "var %s = []*territory{\n", varName)
	for i := range ts {
		renderTerritory(&b, &ts[i])
	}
	b.WriteString("}\n")
	src, err := format.Source(b.Bytes())
	if err != nil {
		return nil, fmt.Errorf("formatting generated source: %w", err)
	}
	return src, nil
}

func renderTerritory(b *bytes.Buffer, t *territory) {
	b.WriteString("{\n")
	field(b, "id", strconv.Quote(t.id))
	field(b, "countryCode", strconv.Itoa(t.countryCode))
	patternField(b, "leadingDigits", t.leadingDigits)
	patternField(b, "internationalPrefix", t.internationalPrefix)
	stringField(b, "preferredInternationalPrefix", t.preferredInternationalPrefix)
	stringField(b, "nationalPrefix", t.nationalPrefix)
	patternField(b, "nationalPrefixForParsing", t.nationalPrefixForParsing)
	stringField(b, "nationalPrefixTransformRule", t.nationalPrefixTransformRule)
	stringField(b, "preferredExtnPrefix", t.preferredExtnPrefix)
	boolField(b, "mainCountryForCode", t.mainCountryForCode)
	boolField(b, "mobileNumberPortableRegion", t.mobileNumberPortableRegion)
	boolField(b, "sameMobileAndFixedLinePattern", t.sameMobileAndFixedLinePattern)
	formatsField(b, "numberFormats", t.numberFormats)
	formatsField(b, "intlNumberFormats", t.intlNumberFormats)
	descField(b, "generalDesc", &t.generalDesc)
	descField(b, "fixedLine", &t.fixedLine)
	descField(b, "mobile", &t.mobile)
	descField(b, "tollFree", &t.tollFree)
	descField(b, "premiumRate", &t.premiumRate)
	descField(b, "sharedCost", &t.sharedCost)
	descField(b, "personalNumber", &t.personalNumber)
	descField(b, "voip", &t.voip)
	descField(b, "pager", &t.pager)
	descField(b, "uan", &t.uan)
	descField(b, "voicemail", &t.voicemail)
	descField(b, "noInternationalDialling", &t.noInternationalDialling)
	b.WriteString("},\n")
}

func field(b *bytes.Buffer, name, value string) { fmt.Fprintf(b, "%s: %s,\n", name, value) }

func stringField(b *bytes.Buffer, name, s string) {
	if s != "" {
		field(b, name, strconv.Quote(s))
	}
}

func boolField(b *bytes.Buffer, name string, v bool) {
	if v {
		field(b, name, "true")
	}
}

func patternField(b *bytes.Buffer, name, expr string) {
	if expr != "" {
		field(b, name, patternLit(expr))
	}
}

func patternLit(expr string) string { return "&pattern{expr: " + strconv.Quote(expr) + "}" }

func intsLit(s []int) string {
	parts := make([]string, len(s))
	for i, n := range s {
		parts[i] = strconv.Itoa(n)
	}
	return "[]int8{" + strings.Join(parts, ", ") + "}"
}

func descField(b *bytes.Buffer, name string, d *desc) {
	fmt.Fprintf(b, "%s: numberDesc{", name)
	if d.pattern != "" {
		fmt.Fprintf(b, "pattern: %s, ", patternLit(d.pattern))
	}
	if d.example != "" {
		fmt.Fprintf(b, "example: %s, ", strconv.Quote(d.example))
	}
	if d.lengths != nil {
		fmt.Fprintf(b, "lengths: %s, ", intsLit(d.lengths))
	}
	if d.localOnly != nil {
		fmt.Fprintf(b, "localOnly: %s, ", intsLit(d.localOnly))
	}
	b.WriteString("},\n")
}

func formatsField(b *bytes.Buffer, name string, fs []numberFormat) {
	if len(fs) == 0 {
		return
	}
	fmt.Fprintf(b, "%s: []*numberFormat{\n", name)
	for _, f := range fs {
		b.WriteString("{")
		fmt.Fprintf(b, "pattern: %s, format: %s, ", patternLit(f.pattern), strconv.Quote(f.format))
		if len(f.leadingDigits) > 0 {
			b.WriteString("leadingDigits: []*pattern{")
			for _, ld := range f.leadingDigits {
				b.WriteString("{expr: " + strconv.Quote(ld) + "}, ")
			}
			b.WriteString("}, ")
		}
		if f.nationalPrefixFormattingRule != "" {
			fmt.Fprintf(b, "nationalPrefixFormattingRule: %s, ", strconv.Quote(f.nationalPrefixFormattingRule))
		}
		if f.nationalPrefixOptionalWhenFormatting {
			b.WriteString("nationalPrefixOptionalWhenFormatting: true, ")
		}
		if f.carrierCodeFormattingRule != "" {
			fmt.Fprintf(b, "carrierCodeFormattingRule: %s, ", strconv.Quote(f.carrierCodeFormattingRule))
		}
		b.WriteString("},\n")
	}
	b.WriteString("},\n")
}
