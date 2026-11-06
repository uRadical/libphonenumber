package phonenumber

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCountryCodeSourceString(t *testing.T) {
	for s, want := range map[CountryCodeSource]string{
		SourceUnspecified:         "SourceUnspecified",
		FromNumberWithIDD:         "FromNumberWithIDD",
		FromDefaultCountry:        "FromDefaultCountry",
		CountryCodeSource(-1):     "CountryCodeSource(-1)",
		CountryCodeSource(99):     "CountryCodeSource(99)",
		FromNumberWithoutPlusSign: "FromNumberWithoutPlusSign",
	} {
		if got := s.String(); got != want {
			t.Errorf("CountryCodeSource(%d).String() = %q, want %q", int(s), got, want)
		}
	}
}

// The exported functions use the real metadata.
func TestRawInputRealMetadata(t *testing.T) {
	p, err := ParseAndKeepRawInput("0 15 21 2345 6789", "br")
	if err != nil {
		t.Fatal(err)
	}
	if p.CarrierCode != "15" || p.Source != FromDefaultCountry || p.Raw != "0 15 21 2345 6789" {
		t.Fatalf("ParseAndKeepRawInput = %+v", p)
	}
	for _, tt := range []struct{ name, got, want string }{
		{"FormatInOriginalFormat", p.FormatInOriginalFormat("BR"), "0 15 21 2345 6789"},
		{"FormatNationalWithPreferredCarrierCode", p.FormatNationalWithPreferredCarrierCode("21"), "0 15 (21) 2345-6789"},
		{"FormatNationalWithCarrierCode", p.Number.FormatNationalWithCarrierCode("41"), "0 41 (21) 2345-6789"},
		// Brazilian numbers need the carrier code within Brazil.
		{"Parsed.FormatForMobileDialing", p.FormatForMobileDialing("br", false), "0152123456789"},
		{"Number.FormatForMobileDialing", p.Number.FormatForMobileDialing("BR", false), ""},
		// The first three digits of the national number, 212, are split
		// across groups in the input, so nothing before them is removed:
		// the caveat in the method's documentation.
		{"FormatOutOfCountryKeepingAlphaChars", p.FormatOutOfCountryKeepingAlphaChars("us"), "011 55 0 15 21 2345 6789"},
	} {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
	if _, err := ParseAndKeepRawInput("Dog", "BR"); err == nil {
		t.Error("ParseAndKeepRawInput(Dog) succeeded")
	}
	got, err := p.Number.FormatByPattern(National, []FormatRule{{Pattern: `(\d{2})(\d+)`, Format: "$1/$2", NationalPrefixFormattingRule: "$NP$FG"}})
	if err != nil || got != "021/23456789" {
		t.Errorf("FormatByPattern = %q, %v", got, err)
	}
}

func TestParsedEncodesAllFields(t *testing.T) {
	p, err := ParseAndKeepRawInput("+44 20 7946 0958", "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"Raw":"+44 20 7946 0958"`) || !strings.Contains(string(b), `"Number":"+442079460958"`) {
		t.Errorf("json.Marshal(Parsed) = %s, want every field", b)
	}
}

func TestFormatByPatternErrors(t *testing.T) {
	for _, rule := range []FormatRule{
		{Pattern: `(`},
		{Pattern: `(\d+)`, LeadingDigits: []string{`[`}},
	} {
		if _, err := usNumber.FormatByPattern(National, []FormatRule{rule}); err == nil {
			t.Errorf("FormatByPattern with %+v succeeded", rule)
		}
	}
}

func TestFormatByPatternEdgeCases(t *testing.T) {
	ms := testSet()
	rules := []FormatRule{{Pattern: `(\d{3})(\d+)`, Format: "$1 $2", LeadingDigits: []string{`9`, `99`}}}
	for _, tt := range []struct {
		n    Number
		want string
	}{
		{Number{}, ""},
		// An unknown country calling code gives the national number.
		{unknownCountryCodeNoRawInput, "12345"},
		// No rule matches, since the last leading digits do not.
		{usNumber, "6502530000"},
		{num(1, 9912345678), "991 2345678"},
		// The extension is written as the metadata says.
		{usNumber.withExt("12"), "6502530000 extn. 12"},
	} {
		got, err := ms.formatByPattern(tt.n, National, rules)
		if err != nil || got != tt.want {
			t.Errorf("formatByPattern(%v) = %q, %v, want %q", tt.n, got, err, tt.want)
		}
	}
}

func TestFormatZeroNumbers(t *testing.T) {
	ms := testSet()
	if got := ms.formatWithCarrier(Number{}, "15"); got != "" {
		t.Errorf("formatWithCarrier(Number{}) = %q", got)
	}
	if got := ms.formatForMobileDialing(Parsed{Raw: "x"}, "US", false); got != "x" {
		t.Errorf("formatForMobileDialing of an unknown country calling code = %q, want the raw input", got)
	}
}

func TestFormatInOriginalFormatUnknownCountryCode(t *testing.T) {
	p := Parsed{Number: unknownCountryCodeNoRawInput, Raw: "12345", Source: FromNumberWithPlusSign}
	if got := testSet().formatInOriginalFormat(p, "US"); got != "12345" {
		t.Errorf("formatInOriginalFormat = %q, want the raw input", got)
	}
}
