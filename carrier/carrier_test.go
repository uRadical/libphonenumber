package carrier

import (
	"testing"

	"uradical.io/go/phonenumber"
)

// The exported functions use the real data.
func TestExportedFunctions(t *testing.T) {
	n, err := phonenumber.Parse("+44 7400 123456", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := Name(n, "en"); got != "Three" {
		t.Errorf("Name = %q, want Three", got)
	}
	if got := NameForValid(n, "en"); got != "Three" {
		t.Errorf("NameForValid = %q, want Three", got)
	}
	if got := SafeDisplayName(n, "en"); got != "" {
		t.Errorf("SafeDisplayName = %q, want \"\" in a portable region", got)
	}
}
