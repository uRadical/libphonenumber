package prefixmap

import (
	"testing"
)

// Tests ported from upstream's prefixmapper tests: MappingFileProviderTest,
// PhonePrefixMapTest and PrefixTimeZonesMapTest. Upstream's tests of its
// binary storage formats have no counterpart here.

func TestGetFileName(t *testing.T) {
	langs := map[int][]string{
		1:  {"en"},
		86: {"zh", "en", "zh_Hant"},
		41: {"de", "fr", "it", "rm"},
		65: {"en", "zh_Hans", "ms", "ta"},
	}
	f := &Files{index: func() map[int][]string { return langs }}
	for _, tt := range []struct {
		key                  int
		lang, script, region string
		want                 string
	}{
		{1, "en", "", "", "1_en.gz"},
		{1, "en", "", "US", "1_en.gz"},
		{1, "en", "", "GB", "1_en.gz"},
		{41, "de", "", "CH", "41_de.gz"},
		{44, "en", "", "GB", ""},
		{86, "zh", "", "", "86_zh.gz"},
		{86, "zh", "Hans", "", "86_zh.gz"},
		{86, "zh", "", "CN", "86_zh.gz"},
		{86, "", "", "CN", ""},
		{86, "zh", "Hans", "CN", "86_zh.gz"},
		{86, "zh", "Hans", "SG", "86_zh.gz"},
		{86, "zh", "", "SG", "86_zh.gz"},
		{86, "zh", "", "TW", "86_zh_Hant.gz"},
		{86, "zh", "", "HK", "86_zh_Hant.gz"},
		{86, "zh", "Hant", "TW", "86_zh_Hant.gz"},
	} {
		if got := f.fileName(tt.key, tt.lang, tt.script, tt.region); got != tt.want {
			t.Errorf("fileName(%d, %q, %q, %q) = %q, want %q", tt.key, tt.lang, tt.script, tt.region, got, tt.want)
		}
	}
}

type lookupCase struct {
	digits string
	want   string
	found  bool
}

func checkLookups(t *testing.T, table Table, cases []lookupCase) {
	t.Helper()
	for _, tt := range cases {
		got, found := table.Lookup(tt.digits)
		if got != tt.want || found != tt.found {
			t.Errorf("Lookup(%s) = %q, %v, want %q, %v", tt.digits, got, found, tt.want, tt.found)
		}
	}
}

func TestPhonePrefixMapLookup(t *testing.T) {
	us := Table{
		"1212": "New York", "1480": "Arizona", "1650": "California", "1907": "Alaska",
		"1201664": "Westwood, NJ", "1480893": "Phoenix, AZ", "1501372": "Little Rock, AR",
		"1626308": "Alhambra, CA", "1650345": "San Mateo, CA", "1867993": "Dawson, YT",
		"1972480": "Richardson, TX",
	}
	checkLookups(t, us, []lookupCase{
		// Central office codes cannot start with 1, but the prefix matches.
		{"12121234567", "New York", true},
		{"12016641234", "Westwood, NJ", true},
		{"12126641234", "New York", true},
		{"16503451234", "San Mateo, CA", true},
		{"16502531234", "California", true},
		{"19724801234", "Richardson, TX", true},
		{"19724811234", "", false},
		{"41446681300", "", false},
	})
	it := Table{
		"3902": "Milan", "3906": "Rome", "39010": "Genoa", "390131": "Alessandria",
		"390321": "Novara", "390975": "Potenza",
	}
	// Italian numbers keep their leading zero.
	checkLookups(t, it, []lookupCase{
		{"390212345678", "Milan", true},
		{"390612345678", "Rome", true},
		{"3903211234", "Novara", true},
		// A mobile number.
		{"39321123456", "", false},
		// Too short, but still a prefix.
		{"39321123", "", false},
		{"390321123", "Novara", true},
	})
}

func TestPrefixTimeZonesMapLookup(t *testing.T) {
	us := Table{
		"1":       "America/New_York&America/Chicago&America/Los_Angeles&America/Denver",
		"1201":    "America/New_York",
		"1205":    "America/Chicago",
		"1208292": "America/Los_Angeles",
		"1208234": "America/Denver",
		"1541367": "America/Los_Angeles",
		"1423843": "America/New_York",
		"1402721": "America/Chicago",
		"1208888": "America/Denver",
	}
	checkLookups(t, us, []lookupCase{
		// Only the country calling code matches.
		{"11000000000", "America/New_York&America/Chicago&America/Los_Angeles&America/Denver", true},
		{"12051235458", "America/Chicago", true},
		{"12082924565", "America/Los_Angeles", true},
		{"12016641234", "America/New_York", true},
		{"41446681300", "", false},
	})
	ru := Table{
		"7421": "Asia/Vladivostok", "7879": "Europe/Moscow",
		"7342": "Asia/Yekaterinburg", "7395": "Asia/Irkutsk",
	}
	checkLookups(t, ru, []lookupCase{
		{"787945154", "Europe/Moscow", true},
		{"7421548578", "Asia/Vladivostok", true},
		{"7342457897", "Asia/Yekaterinburg", true},
		// A mobile number.
		{"79342457897", "", false},
		// Too short, but still a prefix.
		{"73951", "Asia/Irkutsk", true},
	})
}
