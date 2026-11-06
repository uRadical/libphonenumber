package timezone

import (
	"slices"
	"testing"

	"uradical.io/go/phonenumber"
)

// The exported functions use the real data.
func TestExportedFunctions(t *testing.T) {
	n, err := phonenumber.Parse("+44 20 7946 0958", "")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Europe/London"}
	if got := ForNumber(n); !slices.Equal(got, want) {
		t.Errorf("ForNumber = %v, want %v", got, want)
	}
	if got := ForGeographicalNumber(n); !slices.Equal(got, want) {
		t.Errorf("ForGeographicalNumber = %v, want %v", got, want)
	}
	// Callers may change the result without changing the next one.
	ForNumber(n)[0] = "changed"
	if got := ForNumber(n); !slices.Equal(got, want) {
		t.Errorf("ForNumber after changing a result = %v", got)
	}
}

func TestEmptyDescription(t *testing.T) {
	if got := lookup(map[string]string{"44": ""}, "4420"); !slices.Equal(got, []string{Unknown}) {
		t.Errorf("lookup of an empty entry = %v", got)
	}
}
