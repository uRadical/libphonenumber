package prefixmap

import (
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
	"testing/fstest"
)

func gz(t *testing.T, s string) []byte {
	t.Helper()
	var b bytes.Buffer
	z := gzip.NewWriter(&b)
	if _, err := z.Write([]byte(s)); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func testFiles(t *testing.T) *Files {
	t.Helper()
	return NewFiles(fstest.MapFS{
		"44_en.gz":   {Data: gz(t, "4420|London\n44113|Leeds\n")},
		"44_de.gz":   {Data: gz(t, "4420|London (de)\n44113|\n")},
		"82_en.gz":   {Data: gz(t, "822|Seoul\n")},
		"82_ko.gz":   {Data: gz(t, "8231|Gyeonggi\n")},
		"1650_en.gz": {Data: gz(t, "1650|California\n")},
		"39_en.gz":   {Data: []byte("not gzip")},
		"README":     {Data: []byte("ignored")},
		"x_en.gz":    {Data: gz(t, "")},
		"nolang.gz":  {Data: gz(t, "")},
	})
}

func TestDescription(t *testing.T) {
	f := testFiles(t)
	for _, tt := range []struct {
		cc                int
		nsn, lang, region string
		want              string
	}{
		{44, "2079460958", "en", "", "London"},
		{44, "2079460958", "de", "", "London (de)"},
		// An empty description falls back to English.
		{44, "1132123456", "de", "", "Leeds"},
		// A language with no data falls back to English.
		{44, "1132123456", "fr", "CA", "Leeds"},
		// Korean does not fall back.
		{82, "22123456", "ko", "", ""},
		{82, "312345678", "ko", "", "Gyeonggi"},
		// NANPA tables are by area code.
		{1, "6502530000", "en", "", "California"},
		{1, "2125550000", "en", "", ""},
		// A file that is not gzip reads as empty.
		{39, "0212345678", "en", "", ""},
		// No language falls back to English, as upstream's does.
		{44, "2079460958", "", "", "London"},
	} {
		if got := f.Description(tt.cc, tt.nsn, tt.lang, "", tt.region); got != tt.want {
			t.Errorf("Description(%d, %s, %q, %q) = %q, want %q", tt.cc, tt.nsn, tt.lang, tt.region, got, tt.want)
		}
	}
}

func TestReadIndexMissingDir(t *testing.T) {
	f := NewFiles(fstest.MapFS{"a/44_en.gz": {Data: nil}})
	if got := f.index(); len(got) != 0 {
		t.Errorf("index of a tree without top-level tables = %v", got)
	}
}

func TestReadErrors(t *testing.T) {
	if _, err := Read(strings.NewReader("no pipe\n")); err == nil {
		t.Error("Read of a line without '|' succeeded")
	}
	fsys := fstest.MapFS{
		"bad.gz":  {Data: gz(t, "no pipe\n")},
		"junk.gz": {Data: []byte("x")},
	}
	for _, name := range []string{"bad.gz", "junk.gz", "missing.gz"} {
		if _, err := ReadFile(fsys, name); err == nil {
			t.Errorf("ReadFile(%s) succeeded", name)
		}
	}
}

func TestParseLanguage(t *testing.T) {
	for _, tt := range []struct{ tag, lang, script, region string }{
		{"en", "en", "", ""},
		{"EN-us", "en", "", "US"},
		{"zh_Hant_TW", "zh", "Hant", "TW"},
		{"zh-hant", "zh", "Hant", ""},
		{"es-419", "es", "", "419"},
		{"he-IL", "iw", "", "IL"},
		{"in", "id", "", ""},
		{"", "", "", ""},
		{"en-US-x-private", "en", "", "US"},
	} {
		lang, script, region := ParseLanguage(tt.tag)
		if lang != tt.lang || script != tt.script || region != tt.region {
			t.Errorf("ParseLanguage(%q) = %q, %q, %q, want %q, %q, %q", tt.tag, lang, script, region, tt.lang, tt.script, tt.region)
		}
	}
}

func TestBestMatchingLanguageScriptAndRegion(t *testing.T) {
	for _, tt := range []struct {
		langs                []string
		lang, script, region string
		want                 string
	}{
		{[]string{"sr_Latn"}, "sr", "Latn", "RS", "sr_Latn"},
		{[]string{"pt_BR"}, "pt", "Latn", "BR", "pt_BR"},
		{[]string{"pt"}, "pt", "Latn", "BR", "pt"},
		{[]string{"en"}, "pt", "Latn", "BR", ""},
	} {
		if got := bestMatchingLanguage(tt.langs, tt.lang, tt.script, tt.region); got != tt.want {
			t.Errorf("bestMatchingLanguage(%v, %s_%s_%s) = %q, want %q", tt.langs, tt.lang, tt.script, tt.region, got, tt.want)
		}
	}
}
