package phonenumber

import (
	"slices"
	"testing"
)

func TestCountryCodeLists(t *testing.T) {
	all := CountryCodes()
	nonGeo := NonGeoCountryCodes()
	if !slices.IsSorted(all) || !slices.IsSorted(nonGeo) {
		t.Error("country calling codes are not sorted")
	}
	for _, cc := range []int{1, 44, 800} {
		if !slices.Contains(all, cc) {
			t.Errorf("CountryCodes() lacks %d", cc)
		}
	}
	if !slices.Contains(nonGeo, 800) || slices.Contains(nonGeo, 44) {
		t.Errorf("NonGeoCountryCodes() = %v", nonGeo)
	}
	if got := SupportedTypesForNonGeo(800); !slices.Equal(got, []Type{TollFree}) {
		t.Errorf("SupportedTypesForNonGeo(800) = %v, want [TollFree]", got)
	}
	if got := SupportedTypesForNonGeo(44); got != nil {
		t.Errorf("SupportedTypesForNonGeo(44) = %v, want nil", got)
	}
	if !IsNANPA("bs") || IsNANPA("GB") {
		t.Error("IsNANPA")
	}
}
