package phonetext

import (
	"testing"
	"unicode"
)

func TestDigitValue(t *testing.T) {
	// DigitValue relies on every range of unicode.Nd starting on a zero and
	// holding whole runs of ten digits.
	for _, r := range unicode.Nd.R16 {
		if r.Stride != 1 || (r.Hi-r.Lo+1)%10 != 0 {
			t.Errorf("unicode.Nd range %U-%U stride %d breaks DigitValue", r.Lo, r.Hi, r.Stride)
		}
	}
	for _, r := range unicode.Nd.R32 {
		if r.Stride != 1 || (r.Hi-r.Lo+1)%10 != 0 {
			t.Errorf("unicode.Nd range %U-%U stride %d breaks DigitValue", r.Lo, r.Hi, r.Stride)
		}
	}
	for _, tc := range []struct {
		r    rune
		want int
	}{
		{'7', 7}, {0xFF17, 7}, {0x0667, 7}, {0x06F7, 7}, {0x1D7D5, 7}, {'a', -1}, {0x2167, -1},
	} {
		if got := DigitValue(tc.r); got != tc.want {
			t.Errorf("DigitValue(%U) = %d, want %d", tc.r, got, tc.want)
		}
	}
}

func TestNormalizeDigitsKeep(t *testing.T) {
	if got, want := NormalizeDigits("１-a", true), "1-a"; got != want {
		t.Errorf("NormalizeDigits(keep) = %q, want %q", got, want)
	}
}

func TestExtractPossibleNumber(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"Tel:0800-345-600", "0800-345-600"},
		{"Num-....", ""},
		{"(650) 253-0000..- ..", "650) 253-0000"},
		{"650 253 0000/x123", "650 253 0000"},
		{"＋1 650", "＋1 650"},
	} {
		if got := ExtractPossibleNumber(tt.in); got != tt.want {
			t.Errorf("ExtractPossibleNumber(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestPlusCharsPattern(t *testing.T) {
	for _, tt := range []struct {
		s    string
		want bool
	}{
		{"+1", true}, {"\uFF0B1", true}, {"1", false},
		// Only a plus sign at the very start counts.
		{" +1", false},
	} {
		if got := PlusCharsPattern().MatchString(tt.s); got != tt.want {
			t.Errorf("PlusCharsPattern matches %q = %v, want %v", tt.s, got, tt.want)
		}
	}
}
