package phonenumber

import (
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"errors"
	"regexp"
	"testing"
	"unicode"
)

var (
	_ encoding.TextMarshaler   = Number{}
	_ encoding.TextUnmarshaler = (*Number)(nil)
	_ driver.Valuer            = Number{}
	_ sql.Scanner              = (*Number)(nil)
)

func TestZeroNumber(t *testing.T) {
	var n Number
	if !n.IsZero() {
		t.Fatal("zero Number: IsZero() = false")
	}
	for _, f := range []Format{E164, International, National, RFC3966} {
		if got := n.Format(f); got != "" {
			t.Errorf("zero Number: Format(%v) = %q, want \"\"", f, got)
		}
	}
	if got := n.String(); got != "" {
		t.Errorf("zero Number: String() = %q, want \"\"", got)
	}
	if n.Valid() {
		t.Error("zero Number: Valid() = true")
	}
	if got := n.Region(); got != "" {
		t.Errorf("zero Number: Region() = %q, want \"\"", got)
	}
	if v, err := n.Value(); v != nil || err != nil {
		t.Errorf("zero Number: Value() = %v, %v; want nil, nil", v, err)
	}
}

func TestStringRoundTrip(t *testing.T) {
	for _, tc := range []struct{ in, region, want string }{
		{"020 7946 0958", "GB", "+442079460958"},
		{"020 7946 0958 ext. 123", "GB", "+442079460958;ext=123"},
		{"02 3661 8300", "IT", "+390236618300"},
		{"+800 1234 5678", "", "+80012345678"},
	} {
		n, err := Parse(tc.in, tc.region)
		if err != nil {
			t.Fatalf("Parse(%q, %q): %v", tc.in, tc.region, err)
		}
		if got := n.String(); got != tc.want {
			t.Errorf("Parse(%q, %q).String() = %q, want %q", tc.in, tc.region, got, tc.want)
		}
		var back Number
		if err := back.UnmarshalText([]byte(n.String())); err != nil {
			t.Fatalf("UnmarshalText(%q): %v", n.String(), err)
		}
		if back != n {
			t.Errorf("UnmarshalText(%q) = %#v, want %#v", n.String(), back, n)
		}
	}
}

func TestUnmarshalTextNeedsCountryCode(t *testing.T) {
	var n Number
	err := n.UnmarshalText([]byte("020 7946 0958"))
	if !errors.Is(err, ErrInvalidCountryCode) {
		t.Errorf("UnmarshalText of a national number: err = %v, want %v", err, ErrInvalidCountryCode)
	}
}

func TestJSON(t *testing.T) {
	type contact struct {
		Phone  Number  `json:"phone"`
		Backup *Number `json:"backup,omitempty"`
	}
	var c contact
	if err := json.Unmarshal([]byte(`{"phone":"+44 20 7946 0958"}`), &c); err != nil {
		t.Fatal(err)
	}
	if want := num(44, 2079460958); c.Phone != want {
		t.Errorf("Phone = %v, want %v", c.Phone, want)
	}
	out, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"phone":"+442079460958"}`; string(out) != want {
		t.Errorf("Marshal = %s, want %s", out, want)
	}
	if err := json.Unmarshal([]byte(`{"phone":""}`), &c); err != nil || !c.Phone.IsZero() {
		t.Errorf(`Unmarshal of "": Phone = %v, err = %v; want zero, nil`, c.Phone, err)
	}
	if err := json.Unmarshal([]byte(`{"phone":"not a number"}`), &c); !errors.Is(err, ErrNotANumber) {
		t.Errorf("Unmarshal of junk: err = %v, want %v", err, ErrNotANumber)
	}
}

func TestSQL(t *testing.T) {
	n := num(44, 2079460958).withExt("12")
	v, err := n.Value()
	if err != nil {
		t.Fatal(err)
	}
	var back Number
	if err := back.Scan(v); err != nil {
		t.Fatalf("Scan(%v): %v", v, err)
	}
	if back != n {
		t.Errorf("Scan(Value()) = %v, want %v", back, n)
	}
	if err := back.Scan([]byte("+16502530000")); err != nil || back != num(1, 6502530000) {
		t.Errorf("Scan([]byte) = %v, %v", back, err)
	}
	if err := back.Scan(nil); err != nil || !back.IsZero() {
		t.Errorf("Scan(nil) = %v, %v; want zero, nil", back, err)
	}
	if err := back.Scan(42); err == nil {
		t.Error("Scan(42): want error")
	}
}

func TestNumberIsComparable(t *testing.T) {
	a, _ := Parse("+44 20 7946 0958", "")
	b, _ := Parse("020 7946 0958", "GB")
	seen := map[Number]bool{a: true}
	if !seen[b] {
		t.Errorf("%v and %v should be the same map key", a, b)
	}
}

func TestDigitValue(t *testing.T) {
	// digitValue relies on every range of unicode.Nd starting on a zero and
	// holding whole runs of ten digits.
	for _, r := range unicode.Nd.R16 {
		if r.Stride != 1 || (r.Hi-r.Lo+1)%10 != 0 {
			t.Errorf("unicode.Nd range %U-%U stride %d breaks digitValue", r.Lo, r.Hi, r.Stride)
		}
	}
	for _, r := range unicode.Nd.R32 {
		if r.Stride != 1 || (r.Hi-r.Lo+1)%10 != 0 {
			t.Errorf("unicode.Nd range %U-%U stride %d breaks digitValue", r.Lo, r.Hi, r.Stride)
		}
	}
	for _, tc := range []struct {
		r    rune
		want int
	}{
		{'7', 7}, {0xFF17, 7}, {0x0667, 7}, {0x06F7, 7}, {0x1D7D5, 7}, {'a', -1}, {0x2167, -1},
	} {
		if got := digitValue(tc.r); got != tc.want {
			t.Errorf("digitValue(%U) = %d, want %d", tc.r, got, tc.want)
		}
	}
}

func TestReplaceJava(t *testing.T) {
	re := regexp.MustCompile(`(\d{2})(\d{3})`)
	for _, tc := range []struct{ repl, want string }{
		{"$1 $2", "12 345"},
		{"$12", "122"}, // no group 12: group 1 then "2"
		{`\$1`, "$1"},  // escaped dollar
		{"0$1", "012"},
		{"$1-$2$3", "12-345"}, // unknown group 3 expands to nothing
	} {
		if got := replaceJava(re, "12345", tc.repl, false); got != tc.want {
			t.Errorf("replaceJava(%q) = %q, want %q", tc.repl, got, tc.want)
		}
	}
}
