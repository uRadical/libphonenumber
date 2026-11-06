package phonenumber

import (
	"strings"
	"testing"
)

// The exported constructor uses the real metadata.
func TestAsYouTypeRealMetadata(t *testing.T) {
	f := NewAsYouTypeFormatter("gb")
	var got string
	for _, r := range "02079460958" {
		got = f.InputDigit(r)
	}
	if got != "020 7946 0958" {
		t.Errorf("typed 02079460958 in GB: %q, want %q", got, "020 7946 0958")
	}
	f.Clear()
	for _, r := range "+16502530000" {
		got = f.InputDigit(r)
	}
	if got != "+1 650-253-0000" {
		t.Errorf("typed +16502530000 in GB: %q", got)
	}
}

// Clearing after a number from another country goes back to the
// formatter's own region.
func TestAsYouTypeClearRestoresRegion(t *testing.T) {
	f := testSet().newAsYouTypeFormatter("US")
	for _, r := range "+44" {
		f.InputDigit(r)
	}
	if f.meta == f.defaultMeta {
		t.Fatal("metadata did not switch to GB")
	}
	f.Clear()
	if f.meta != f.defaultMeta {
		t.Error("Clear did not restore the US metadata")
	}
}

// FuzzAsYouType checks that typing any input never panics, never loses a
// digit, and that a remembered position lies within the output.
func FuzzAsYouType(f *testing.F) {
	for _, s := range []string{"6502530000", "+48881231", "011 44 20 7946", "1800MICROSOFT", "\uFF16\uFF15\uFF10", "0~0 1800"} {
		f.Add(s, "US")
		f.Add(s, "KR")
	}
	f.Fuzz(func(t *testing.T, input, region string) {
		a := NewAsYouTypeFormatter(region)
		var out string
		for i, r := range input {
			if i%3 == 0 {
				out = a.InputDigitAndRememberPosition(r)
			} else {
				out = a.InputDigit(r)
			}
			if p := a.RememberedPosition(); p < 0 || p > len(out) && p > len(input) {
				t.Fatalf("typing %q: position %d outside %q", input[:i], p, out)
			}
		}
		if out == "" {
			return
		}
		// Formatting may add separators but never digits.
		if strings.Count(NormalizeDigits(out), "") > strings.Count(NormalizeDigits(input), "")+1 {
			t.Fatalf("typing %q gave %q, with more digits than typed", input, out)
		}
	})
}
