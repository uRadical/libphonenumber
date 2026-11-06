package phonenumber

import (
	"strings"
	"testing"
)

// The exported helpers use the real metadata.
func TestHelpersRealMetadata(t *testing.T) {
	n, err := Parse("+44 20 7946 0958", "")
	if err != nil {
		t.Fatal(err)
	}
	if !n.Geographical() || n.AreaCodeLength() != 2 || n.NationalDestinationCodeLength() != 2 {
		t.Errorf("Geographical, AreaCodeLength, NDC = %v, %d, %d", n.Geographical(), n.AreaCodeLength(), n.NationalDestinationCodeLength())
	}
	if got, ok := n.TruncateTooLong(); !ok || got != n {
		t.Errorf("TruncateTooLong(valid) = %v, %v", got, ok)
	}
	inv, ok := InvalidExampleNumber("gb")
	if !ok || inv.Valid() || inv.CountryCode() != 44 {
		t.Errorf("InvalidExampleNumber(gb) = %v, %v", inv, ok)
	}
	if !MobileNumberPortable("gb") || MobileNumberPortable("ZZ") {
		t.Error("MobileNumberPortable")
	}
	if got := NormalizeDigits("２5٥"); got != "255" {
		t.Errorf("NormalizeDigits = %q", got)
	}
}

func TestIsAlphaNumberTooLong(t *testing.T) {
	if IsAlphaNumber("1800 six-flags" + strings.Repeat(" ", maxInputStringLength)) {
		t.Error("IsAlphaNumber of an over-long input = true")
	}
}

// A region with no fixed-line example has no invalid example either.
func TestInvalidExampleNoFixedLine(t *testing.T) {
	ms := newMetadataSet([]*territory{{id: "XX", countryCode: 999}})
	if n, ok := ms.invalidExample("XX"); ok {
		t.Errorf("invalidExample = %v", n)
	}
}

func TestConvertAlphaDotlessI(t *testing.T) {
	// Java upper-cases dotless i to I, which is 4 on the keypad.
	if got := ConvertAlphaCharacters("ı"); got != "4" {
		t.Errorf("ConvertAlphaCharacters(dotless i) = %q, want 4", got)
	}
}
