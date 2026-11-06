package phonenumber

import "testing"

func TestMatchZeroNumber(t *testing.T) {
	for _, tt := range []struct{ a, b Number }{
		{Number{}, nzNumber},
		{nzNumber, Number{}},
		{Number{}, Number{}},
	} {
		if got := tt.a.Match(tt.b); got != MatchNotANumber {
			t.Errorf("%#v.Match(%#v) = %v, want MatchNotANumber", tt.a, tt.b, got)
		}
	}
	if got := (Number{}).MatchString("+64 3 331 6005"); got != MatchNotANumber {
		t.Errorf("Number{}.MatchString = %v, want MatchNotANumber", got)
	}
}

// A Number from Parse always has a known country calling code, but
// upstream handles one without: the string is then parsed with no region.
func TestMatchStringUnknownCountryCode(t *testing.T) {
	checkMatchString(t, []numberStringMatch{
		{unknownCountryCodeNoRawInput, "12345", MatchNSN},
		{unknownCountryCodeNoRawInput, "012345", MatchShortNSN},
		{unknownCountryCodeNoRawInput, "Dog", MatchNotANumber},
		{unknownCountryCodeNoRawInput, "1", MatchNotANumber},
		// A non-geographical code has no region to read the string in.
		{internationalTollFree, "1234 5678", MatchNotANumber},
	})
}

func TestMatchStringsSecondFails(t *testing.T) {
	checkMatchStrings(t, []stringsMatch{
		{"3 331 6005", "+64", MatchNotANumber},
		{"3 331 6005", "Dog", MatchNotANumber},
		{"3 331 6005", "123456789012345678", MatchNotANumber},
	})
}

func TestParseLooseTooLong(t *testing.T) {
	if _, err := testSet().parseLoose("123456789012345678"); err == nil {
		t.Error("parseLoose of 18 digits succeeded")
	}
}

func TestMatchTypeString(t *testing.T) {
	for m, want := range map[MatchType]string{
		MatchNotANumber: "MatchNotANumber",
		MatchExact:      "MatchExact",
		MatchType(-1):   "MatchType(-1)",
		MatchType(9):    "MatchType(9)",
	} {
		if got := m.String(); got != want {
			t.Errorf("MatchType(%d).String() = %q, want %q", int(m), got, want)
		}
	}
}

// The exported functions use the real metadata.
func TestMatchStringsRealMetadata(t *testing.T) {
	if got := MatchStrings("+44 20 7946 0958", "020 7946 0958"); got != MatchNSN {
		t.Errorf("MatchStrings = %v, want MatchNSN", got)
	}
	n, err := Parse("+44 20 7946 0958", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := n.MatchString("+44 (0)20 7946 0958"); got != MatchExact {
		t.Errorf("MatchString = %v, want MatchExact", got)
	}
}

// FuzzMatchStrings checks that matching never panics, and that a number
// that parses with a plus sign matches its own text exactly.
func FuzzMatchStrings(f *testing.F) {
	f.Add("+64 3 331 6005", "03 331 6005")
	f.Add("3 331-6005", "+64 331 6005")
	f.Add("1 650-253 0000", "1 650 253 0000")
	f.Add("+64 3 331-6005 extn 1234", "3 331 6005#1234")
	f.Add("Dog", "+43")
	f.Fuzz(func(t *testing.T, a, b string) {
		m := MatchStrings(a, b)
		if m < MatchNotANumber || m > MatchExact {
			t.Fatalf("MatchStrings(%q, %q) = %v", a, b, m)
		}
		n, err := Parse(a, "")
		if err != nil {
			return
		}
		if got := n.Match(n); got != MatchExact {
			t.Fatalf("Parse(%q).Match(itself) = %v", a, got)
		}
		if got := n.MatchString(n.String()); got != MatchExact && n.Valid() {
			t.Fatalf("%v.MatchString(%q) = %v", n, n.String(), got)
		}
	})
}
