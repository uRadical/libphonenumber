package phonenumber

import (
	"strings"
	"testing"
)

func TestLeniencyString(t *testing.T) {
	for l, want := range map[Leniency]string{
		LeniencyPossible:      "LeniencyPossible",
		LeniencyExactGrouping: "LeniencyExactGrouping",
		Leniency(-1):          "Leniency(-1)",
		Leniency(9):           "Leniency(9)",
	} {
		if got := l.String(); got != want {
			t.Errorf("Leniency(%d).String() = %q, want %q", int(l), got, want)
		}
	}
}

// The exported functions use the real metadata.
func TestFindNumbersRealMetadata(t *testing.T) {
	text := "Call 020 7946 0958 or +1 650-253-0000 ext. 12 today."
	var got []string
	for m := range FindNumbers(text, "gb") {
		if text[m.Start:m.End] != m.Raw {
			t.Errorf("text[%d:%d] = %q, but Raw = %q", m.Start, m.End, text[m.Start:m.End], m.Raw)
		}
		got = append(got, m.Number.String())
	}
	want := []string{"+442079460958", "+16502530000;ext=12"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("FindNumbers = %v, want %v", got, want)
	}
	// Stopping early stops the search.
	for range FindNumbers(text, "GB") {
		break
	}
	// No tries finds nothing.
	for m := range FindNumbersWithLeniency(text, "GB", LeniencyValid, 0) {
		t.Errorf("maxTries 0 found %+v", m)
	}
}

func TestSplitHelpers(t *testing.T) {
	for _, tt := range []struct {
		s    string
		want []string
	}{
		{"650-253-0000", []string{"650", "253", "0000"}},
		{"650-", []string{"650"}},
		{"--", nil},
		{"", []string{""}},
	} {
		if got := splitJava(tt.s, "-"); strings.Join(got, "|") != strings.Join(tt.want, "|") || len(got) != len(tt.want) {
			t.Errorf("splitJava(%q) = %q, want %q", tt.s, got, tt.want)
		}
	}
	for _, tt := range []struct {
		s    string
		want []string
	}{
		{"+1 650", []string{"", "1", "650"}},
		{"650 ", []string{"650"}},
		{"", []string{""}},
		{"--", nil},
	} {
		if got := splitNonDigits(tt.s); strings.Join(got, "|") != strings.Join(tt.want, "|") || len(got) != len(tt.want) {
			t.Errorf("splitNonDigits(%q) = %q, want %q", tt.s, got, tt.want)
		}
	}
}

// A candidate followed by a timestamp's minutes is skipped; one that is
// not stays.
func TestTimestampFollowedByMinutes(t *testing.T) {
	if got := findAll("2012-01-02 08:00 is when", "US", LeniencyPossible); len(got) != 0 {
		t.Errorf("found %+v, want none", got)
	}
}

// FuzzFindNumbers checks that finding numbers terminates without
// panicking, and reports ordered, non-overlapping spans of the text.
func FuzzFindNumbers(f *testing.F) {
	for _, s := range []string{
		"Call 033316005  or 032316005!", "+1 (650) 333-6000", "My info: 415-666-7777 123 fake street",
		"80.585 [79.964, 81.191]", "2012-01-02 08:00", "(0xx) 123456789 x 1234", "\uFF0B1 650",
	} {
		f.Add(s, "US", 1)
		f.Add(s, "NZ", 3)
	}
	f.Fuzz(func(t *testing.T, text, region string, l int) {
		leniency := Leniency(((l % 4) + 4) % 4)
		end := 0
		for m := range FindNumbersWithLeniency(text, region, leniency, 50) {
			if m.Start < end || m.End < m.Start || m.End > len(text) || text[m.Start:m.End] != m.Raw {
				t.Fatalf("FindNumbers(%q) gave %+v after offset %d", text, m, end)
			}
			end = m.End
		}
	})
}

func TestOnlyValidXChars(t *testing.T) {
	ms := testSet()
	ar := mustParseTest(t, "(0xx) 123456789", "AR")
	arExt := mustParseTest(t, "(0xx) 123456789 x 1234", "AR")
	for _, tt := range []struct {
		n         Number
		candidate string
		want      bool
	}{
		// xx before the carrier code and national number.
		{ar, "(0xx) 123456789", true},
		{arExt, "(0xx) 123456789 x 1234", true},
		// xx followed by something that is not the number.
		{ar, "(0xx) 999", false},
		// An x before something that is not the extension.
		{usNumber, "650 x 253 0000", false},
		// An x at the end is ignored.
		{usNumber, "6502530000x", true},
	} {
		if got := ms.onlyValidXChars(tt.n, tt.candidate); got != tt.want {
			t.Errorf("onlyValidXChars(%v, %q) = %v, want %v", tt.n, tt.candidate, got, tt.want)
		}
	}
}

// A non-geographical number has no region whose national prefix it needs.
func TestNationalPrefixNonGeo(t *testing.T) {
	p := Parsed{Number: internationalTollFree, Source: FromDefaultCountry, Raw: "12345678"}
	if !testSet().nationalPrefixPresentIfRequired(p) {
		t.Error("nationalPrefixPresentIfRequired(+800) = false, want true")
	}
}

// The limit on tries also stops the search within a candidate.
func TestMaxTriesWithinCandidate(t *testing.T) {
	for m := range testSet().findNumbers("1234 5678 9012 3456 7890", "US", LeniencyValid, 1) {
		t.Errorf("found %+v, want none", m)
	}
}
