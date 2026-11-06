package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The module root, relative to this package's directory.
const root = "../../.."

// TestGeneratedFilesUpToDate regenerates both metadata files and compares
// them with the committed ones, so a change to resources/ or to the
// generator without a regenerate fails the tests.
func TestGeneratedFilesUpToDate(t *testing.T) {
	for _, tc := range []struct {
		in, out, pkg, varName string
		gen                   func([]byte, string, string, string) ([]byte, error)
	}{
		{"resources/PhoneNumberMetadata.xml", "metadata_gen.go", "phonenumber", "territoryData", generate},
		{"resources/PhoneNumberMetadataForTesting.xml", "testmetadata_gen_test.go", "phonenumber", "testTerritoryData", generate},
	} {
		t.Run(tc.out, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(root, tc.in))
			if err != nil {
				t.Fatal(err)
			}
			got, err := tc.gen(raw, tc.pkg, tc.varName, tc.in)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join(root, tc.out))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("%s is out of date: run go generate ./... in the module root", tc.out)
			}
		})
	}
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.xml")
	out := filepath.Join(dir, "out.go")
	if err := os.WriteFile(in, []byte(doc(`<territory id="XX" countryCode="999"/>`)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-in", in, "-out", out, "-var", "data"}, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}
	src, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"DO NOT EDIT", "var data = []*territory{", `"XX"`, "countryCode:"} {
		if !bytes.Contains(src, []byte(want)) {
			t.Errorf("generated source lacks %q:\n%s", want, src)
		}
	}
}

func TestRunErrors(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.xml")
	if err := os.WriteFile(bad, []byte(doc(`<territory id="XX" countryCode="x"/>`)), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		args []string
		help bool
	}{
		{"no flags", nil, true},
		{"no output", []string{"-in", bad}, true},
		{"unknown flag", []string{"-nope"}, false},
		{"missing input", []string{"-in", filepath.Join(dir, "missing.xml"), "-out", filepath.Join(dir, "o.go")}, false},
		{"bad input", []string{"-in", bad, "-out", filepath.Join(dir, "o.go")}, false},
		{"unwritable output", []string{"-in", bad, "-out", filepath.Join(dir, "no", "such", "dir.go")}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := run(tc.args, io.Discard)
			if err == nil {
				t.Fatal("run succeeded, want an error")
			}
			if got := errors.Is(err, flag.ErrHelp); got != tc.help {
				t.Errorf("errors.Is(%v, flag.ErrHelp) = %v, want %v", err, got, tc.help)
			}
		})
	}
}

// doc wraps territory elements in the document structure.
func doc(territories string) string {
	return `<phoneNumberMetadata><territories>` + territories + `</territories></phoneNumberMetadata>`
}

func TestGenerateRejectsBadMetadata(t *testing.T) {
	const general = `<generalDesc><nationalNumberPattern>\d{4,6}</nationalNumberPattern></generalDesc>`
	desc := func(lengths string) string {
		return general + `<fixedLine><possibleLengths ` + lengths + `/><nationalNumberPattern>\d+</nationalNumberPattern></fixedLine>`
	}
	format := func(attrs, body string) string {
		return `<availableFormats><numberFormat pattern="(\d{2})(\d{2})" ` + attrs + `>` + body + `</numberFormat></availableFormats>` + general
	}
	for _, tc := range []struct {
		name, xml, want string
	}{
		{"malformed XML", `<territory`, "XML syntax error"},
		{"country code", `<territory id="XX" countryCode="x"/>`, "countryCode"},
		{"leading digits", `<territory id="XX" countryCode="9" leadingDigits="("/>`, "pattern"},
		{"international prefix", `<territory id="XX" countryCode="9" internationalPrefix="("/>`, "pattern"},
		{"prefix for parsing", `<territory id="XX" countryCode="9" nationalPrefixForParsing="("/>`, "pattern"},
		{"empty alternative", `<territory id="XX" countryCode="9" internationalPrefix="(0|)"/>`, "'|' directly followed by ')'"},
		{"territory carrier rule", `<territory id="XX" countryCode="9" carrierCodeFormattingRule="("/>`, "pattern"},
		{"format carrier rule", `<territory id="XX" countryCode="9">` + format(`carrierCodeFormattingRule="("`, `<format>$1</format>`) + `</territory>`, "pattern"},
		{"format leading digits", `<territory id="XX" countryCode="9">` + format(``, `<leadingDigits>(</leadingDigits><format>$1</format>`) + `</territory>`, "pattern"},
		{"format pattern", `<territory id="XX" countryCode="9"><availableFormats><numberFormat pattern="("><format>$1</format></numberFormat></availableFormats></territory>`, "pattern"},
		{"no format", `<territory id="XX" countryCode="9">` + format(``, ``) + `</territory>`, "0 format elements"},
		{"two formats", `<territory id="XX" countryCode="9">` + format(``, `<format>$1</format><format>$2</format>`) + `</territory>`, "2 format elements"},
		{"two intl formats", `<territory id="XX" countryCode="9">` + format(``, `<format>$1</format><intlFormat>$1</intlFormat><intlFormat>$2</intlFormat>`) + `</territory>`, "2 intlFormat elements"},
		{"general lengths", `<territory id="XX" countryCode="9"><generalDesc><possibleLengths national="4"/><nationalNumberPattern>\d{4}</nationalNumberPattern></generalDesc></territory>`, "must not declare"},
		{"number pattern", `<territory id="XX" countryCode="9"><generalDesc><nationalNumberPattern>(</nationalNumberPattern></generalDesc></territory>`, "pattern"},
		{"type pattern", `<territory id="XX" countryCode="9">` + general + `<mobile><possibleLengths national="4"/><nationalNumberPattern>(</nationalNumberPattern></mobile></territory>`, "pattern"},
		{"empty lengths", `<territory id="XX" countryCode="9">` + desc(`national=""`) + `</territory>`, "empty possible length"},
		{"stray comma", `<territory id="XX" countryCode="9">` + desc(`national="4,"`) + `</territory>`, "stray comma"},
		{"bad length", `<territory id="XX" countryCode="9">` + desc(`national="four"`) + `</territory>`, "bad length"},
		{"unterminated range", `<territory id="XX" countryCode="9">` + desc(`national="[4-6"`) + `</territory>`, "unterminated range"},
		{"range without dash", `<territory id="XX" countryCode="9">` + desc(`national="[46]"`) + `</territory>`, "has no '-'"},
		{"bad range", `<territory id="XX" countryCode="9">` + desc(`national="[a-6]"`) + `</territory>`, "bad range"},
		{"short range", `<territory id="XX" countryCode="9">` + desc(`national="[4-5]"`) + `</territory>`, "written as a list"},
		{"duplicate length", `<territory id="XX" countryCode="9">` + desc(`national="4,[3-5]"`) + `</territory>`, "duplicate length"},
		{"bad local only", `<territory id="XX" countryCode="9">` + desc(`national="4" localOnly="x"`) + `</territory>`, "bad length"},
		{"local and national", `<territory id="XX" countryCode="9">` + desc(`national="4" localOnly="4"`) + `</territory>`, "both national and local-only"},
		{"uncovered length", `<territory id="XX" countryCode="9">` + general + `<noInternationalDialling><possibleLengths national="7"/><nationalNumberPattern>\d{7}</nationalNumberPattern></noInternationalDialling><fixedLine><possibleLengths national="4"/><nationalNumberPattern>\d{4}</nationalNumberPattern></fixedLine></territory>`, "not covered by parent"},
		{"uncovered local length", `<territory id="XX" countryCode="9">` + general + `<noInternationalDialling><possibleLengths national="5" localOnly="3"/><nationalNumberPattern>\d{5}</nationalNumberPattern></noInternationalDialling><fixedLine><possibleLengths national="4,5"/><nationalNumberPattern>\d{4}</nationalNumberPattern></fixedLine></territory>`, "local-only length 3 not covered"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := generate([]byte(doc(tc.xml)), "phonenumber", "data", "test.xml")
			if err == nil {
				t.Fatal("generate succeeded, want an error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

// build parses one territory element and processes it.
func build(t *testing.T, territoryXML string) territory {
	t.Helper()
	var d xmlDocument
	if err := xml.Unmarshal([]byte(doc(territoryXML)), &d); err != nil {
		t.Fatal(err)
	}
	if len(d.Territories) != 1 {
		t.Fatalf("got %d territories, want 1", len(d.Territories))
	}
	tr, err := buildTerritory(&d.Territories[0])
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

func TestBuildTerritoryAttributes(t *testing.T) {
	tr := build(t, `<territory id="XX" countryCode="999" mainCountryForCode="true"
		mobileNumberPortableRegion="true" internationalPrefix="00" preferredInternationalPrefix="00"
		nationalPrefix="0" preferredExtnPrefix=" x" leadingDigits="[1-3]"/>`)
	if tr.id != "XX" || tr.countryCode != 999 || !tr.mainCountryForCode || !tr.mobileNumberPortableRegion {
		t.Errorf("identity fields = %q %d %v %v", tr.id, tr.countryCode, tr.mainCountryForCode, tr.mobileNumberPortableRegion)
	}
	if tr.internationalPrefix != "00" || tr.preferredInternationalPrefix != "00" ||
		tr.preferredExtnPrefix != " x" || tr.leadingDigits != "[1-3]" {
		t.Errorf("prefix fields = %q %q %q %q", tr.internationalPrefix, tr.preferredInternationalPrefix, tr.preferredExtnPrefix, tr.leadingDigits)
	}
	// With no nationalPrefixForParsing, the national prefix is used.
	if tr.nationalPrefix != "0" || tr.nationalPrefixForParsing != "0" {
		t.Errorf("nationalPrefix, nationalPrefixForParsing = %q, %q; want 0, 0", tr.nationalPrefix, tr.nationalPrefixForParsing)
	}

	tr = build(t, `<territory id="XX" countryCode="9" nationalPrefix="0"
		nationalPrefixForParsing="0 (1\d)?" nationalPrefixTransformRule="5$1"/>`)
	if tr.nationalPrefixForParsing != `0(1\d)?` || tr.nationalPrefixTransformRule != "5$1" {
		t.Errorf("parsing rule = %q, transform = %q", tr.nationalPrefixForParsing, tr.nationalPrefixTransformRule)
	}

	// A transform rule without a parsing rule is ignored, as upstream does.
	tr = build(t, `<territory id="XX" countryCode="9" nationalPrefixTransformRule="5$1"/>`)
	if tr.nationalPrefixTransformRule != "" {
		t.Errorf("transform rule = %q, want \"\"", tr.nationalPrefixTransformRule)
	}
}

func TestBuildFormatsInheritance(t *testing.T) {
	tr := build(t, `<territory id="XX" countryCode="9" nationalPrefix="0"
		nationalPrefixFormattingRule="$NP $FG" nationalPrefixOptionalWhenFormatting="true"
		carrierCodeFormattingRule="$NP $CC $FG">
		<availableFormats>
			<numberFormat pattern="(\d{2})(\d{2})"><format>$1 $2</format></numberFormat>
			<numberFormat pattern="(\d{3})(\d{3})" nationalPrefixFormattingRule="($NP$FG)"
				nationalPrefixOptionalWhenFormatting="false" carrierCodeFormattingRule="$CC $FG">
				<leadingDigits> 1 [2-4] </leadingDigits>
				<format>$1-$2</format>
			</numberFormat>
		</availableFormats>
	</territory>`)
	if len(tr.numberFormats) != 2 {
		t.Fatalf("got %d formats, want 2", len(tr.numberFormats))
	}
	inherited, own := tr.numberFormats[0], tr.numberFormats[1]
	if inherited.nationalPrefixFormattingRule != "0 $1" || !inherited.nationalPrefixOptionalWhenFormatting ||
		inherited.carrierCodeFormattingRule != "0 $CC $1" {
		t.Errorf("inherited format = %+v", inherited)
	}
	if own.nationalPrefixFormattingRule != "(0$1)" || own.nationalPrefixOptionalWhenFormatting ||
		own.carrierCodeFormattingRule != "$CC $1" || !slices.Equal(own.leadingDigits, []string{"1[2-4]"}) {
		t.Errorf("own format = %+v", own)
	}
	// No intlFormat anywhere: the international list is dropped.
	if tr.intlNumberFormats != nil {
		t.Errorf("intlNumberFormats = %+v, want nil", tr.intlNumberFormats)
	}
}

func TestBuildFormatsInternational(t *testing.T) {
	tr := build(t, `<territory id="XX" countryCode="9" nationalPrefixFormattingRule="0$FG">
		<availableFormats>
			<numberFormat pattern="(\d{2})(\d{2})"><format>$1 $2</format></numberFormat>
			<numberFormat pattern="(\d{3})(\d{3})"><format>$1 $2</format><intlFormat>NA</intlFormat></numberFormat>
			<numberFormat pattern="(\d{4})(\d{4})"><leadingDigits>9</leadingDigits><format>$1 $2</format><intlFormat>$1-$2</intlFormat></numberFormat>
		</availableFormats>
	</territory>`)
	if len(tr.numberFormats) != 3 {
		t.Fatalf("got %d national formats, want 3", len(tr.numberFormats))
	}
	// The first is copied as is, the second is dropped as "NA", and the
	// third keeps its pattern and leading digits but none of the national
	// prefix rules.
	want := []numberFormat{
		tr.numberFormats[0],
		{pattern: `(\d{4})(\d{4})`, format: "$1-$2", leadingDigits: []string{"9"}},
	}
	if len(tr.intlNumberFormats) != len(want) {
		t.Fatalf("got %d international formats, want %d", len(tr.intlNumberFormats), len(want))
	}
	for i := range want {
		got := tr.intlNumberFormats[i]
		if got.pattern != want[i].pattern || got.format != want[i].format ||
			!slices.Equal(got.leadingDigits, want[i].leadingDigits) ||
			got.nationalPrefixFormattingRule != want[i].nationalPrefixFormattingRule {
			t.Errorf("intlNumberFormats[%d] = %+v, want %+v", i, got, want[i])
		}
	}
}

func TestBuildDescs(t *testing.T) {
	tr := build(t, `<territory id="XX" countryCode="9">
		<generalDesc><nationalNumberPattern>
			\d{4,7}
		</nationalNumberPattern></generalDesc>
		<noInternationalDialling><possibleLengths national="7"/><nationalNumberPattern>4\d{6}</nationalNumberPattern></noInternationalDialling>
		<fixedLine><possibleLengths national="[5-7]" localOnly="4"/><exampleNumber>51234</exampleNumber><nationalNumberPattern>5\d{4,6}</nationalNumberPattern></fixedLine>
		<mobile><possibleLengths national="5,6,7"/><exampleNumber>61234</exampleNumber><nationalNumberPattern>5\d{4,6}</nationalNumberPattern></mobile>
		<tollFree><possibleLengths national="7"/><exampleNumber>8001234</exampleNumber><nationalNumberPattern>800\d{4}</nationalNumberPattern></tollFree>
	</territory>`)
	if tr.generalDesc.pattern != `\d{4,7}` {
		t.Errorf("general pattern = %q, want whitespace stripped", tr.generalDesc.pattern)
	}
	if !slices.Equal(tr.generalDesc.lengths, []int{5, 6, 7}) || !slices.Equal(tr.generalDesc.localOnly, []int{4}) {
		t.Errorf("general lengths = %v local %v; want [5 6 7] local [4]", tr.generalDesc.lengths, tr.generalDesc.localOnly)
	}
	// Lengths equal to the general description's are not stored.
	if tr.fixedLine.lengths != nil || tr.mobile.lengths != nil {
		t.Errorf("fixedLine, mobile lengths = %v, %v; want nil, nil", tr.fixedLine.lengths, tr.mobile.lengths)
	}
	if !slices.Equal(tr.fixedLine.localOnly, []int{4}) || tr.fixedLine.example != "51234" {
		t.Errorf("fixedLine = %+v", tr.fixedLine)
	}
	if !slices.Equal(tr.tollFree.lengths, []int{7}) {
		t.Errorf("tollFree lengths = %v, want [7]", tr.tollFree.lengths)
	}
	if !tr.sameMobileAndFixedLinePattern {
		t.Error("sameMobileAndFixedLinePattern = false, want true")
	}
	// A missing type matches nothing.
	if !slices.Equal(tr.premiumRate.lengths, []int{-1}) || tr.premiumRate.pattern != "" {
		t.Errorf("premiumRate = %+v, want lengths [-1] and no pattern", tr.premiumRate)
	}
}

func TestSetPossibleLengthsDropsLocalOnlyNationals(t *testing.T) {
	// A length that is local-only for one type but national for another is
	// national in the general description.
	got, local, err := setPossibleLengths([]int{5, 7}, []int{4, 5}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []int{5, 7}) || !slices.Equal(local, []int{4}) {
		t.Errorf("setPossibleLengths = %v, %v; want [5 7], [4]", got, local)
	}
}

func TestRunShort(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.xml")
	out := filepath.Join(dir, "out.go")
	xml := doc(`<territory id="XX"><generalDesc><nationalNumberPattern>1\d{2}</nationalNumberPattern></generalDesc>
		<shortCode><possibleLengths national="3"/><exampleNumber>112</exampleNumber><nationalNumberPattern>1\d{2}</nationalNumberPattern></shortCode>
		<emergency><possibleLengths national="3"/><exampleNumber>112</exampleNumber><nationalNumberPattern>112</nationalNumberPattern></emergency></territory>`)
	if err := os.WriteFile(in, []byte(xml), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-short", "-pkg", "shortnumber", "-in", in, "-out", out}, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}
	src, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"package shortnumber", "var territoryData = []*territory{", `"XX"`,
		`numberDesc{pattern: &pattern{expr: "1\\d{2}"}, lengths: []int8{3}}`,
		`numberDesc{pattern: &pattern{expr: "112"}, example: "112"}`,
		`numberDesc{lengths: []int8{-1}}`} {
		if !bytes.Contains(src, []byte(want)) {
			t.Errorf("generated source lacks %q:\n%s", want, src)
		}
	}
}

func TestGenerateShortRejectsBadMetadata(t *testing.T) {
	const general = `<generalDesc><nationalNumberPattern>\d{3}</nationalNumberPattern></generalDesc>`
	for _, tc := range []struct {
		name, xml, want string
	}{
		{"malformed XML", `<territory`, "XML syntax error"},
		{"general pattern", `<territory id="XX"><generalDesc><nationalNumberPattern>(</nationalNumberPattern></generalDesc></territory>`, "generalDesc"},
		{"general lengths", `<territory id="XX"><generalDesc><possibleLengths national="3"/></generalDesc></territory>`, "must not declare"},
		{"short code lengths", `<territory id="XX">` + general + `<shortCode><possibleLengths national="x"/></shortCode></territory>`, "shortCode"},
		{"local only", `<territory id="XX">` + general + `<shortCode><possibleLengths national="3" localOnly="2"/></shortCode></territory>`, "local-only"},
		{"uncovered length", `<territory id="XX">` + general + `<shortCode><possibleLengths national="3"/></shortCode><emergency><possibleLengths national="4"/><nationalNumberPattern>1\d{3}</nationalNumberPattern></emergency></territory>`, "emergency"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := generateShort([]byte(doc(tc.xml)), "shortnumber", "data", "test.xml")
			if err == nil {
				t.Fatal("generateShort succeeded, want an error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestGenerateAlternate(t *testing.T) {
	src, err := generateAlternate([]byte(doc(`<territory countryCode="49"><availableFormats>
		<numberFormat pattern="(\d{3})(\d+)"><leadingDigits>9</leadingDigits><format>$1 $2</format></numberFormat>
	</availableFormats></territory>`)), "phonenumber", "alt", "test.xml")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"var alt = map[int][]*numberFormat{", "49: {", `format: "$1 $2"`, `leadingDigits: []*pattern{{expr: "9"}}`} {
		if !bytes.Contains(src, []byte(want)) {
			t.Errorf("generated source lacks %q:\n%s", want, src)
		}
	}
	for _, tc := range []struct{ name, xml, want string }{
		{"malformed XML", `<territory`, "XML syntax error"},
		{"bad country code", `<territory countryCode="x"/>`, "countryCode"},
		{"repeated country code", `<territory countryCode="49"/><territory countryCode="49"/>`, "repeated"},
		{"bad pattern", `<territory countryCode="49"><availableFormats><numberFormat pattern="("><format>$1</format></numberFormat></availableFormats></territory>`, "pattern"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := generateAlternate([]byte(doc(tc.xml)), "phonenumber", "alt", "test.xml")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %v does not mention %q", err, tc.want)
			}
		})
	}
}

func TestRunShortAndAlternate(t *testing.T) {
	if err := run([]string{"-short", "-alternate", "-in", "x", "-out", "y"}, io.Discard); err == nil {
		t.Error("run with -short and -alternate succeeded")
	}
}

func TestRunAlternate(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.xml")
	out := filepath.Join(dir, "out.go")
	if err := os.WriteFile(in, []byte(doc(`<territory countryCode="49"/>`)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-alternate", "-in", in, "-out", out, "-var", "alt"}, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}
}
