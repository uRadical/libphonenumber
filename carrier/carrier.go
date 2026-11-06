// Package carrier names the mobile carrier a phone number was first given
// by, in the language asked for. It is the Go port of upstream
// libphonenumber's PhoneNumberToCarrierMapper.
//
//	n, _ := phonenumber.Parse("+44 7400 123456", "")
//	carrier.Name(n, "en") // "Three"
//
// In regions with mobile number portability, a number can move to another
// carrier, so the name may be wrong: [SafeDisplayName] returns "" there.
// Languages are tags such as "en" or "zh-TW", and fall back to English
// except for Chinese, Japanese and Korean.
package carrier

import (
	"embed"
	"io/fs"
	"sync"

	"uradical.io/go/phonenumber"
	"uradical.io/go/phonenumber/internal/prefixmap"
)

var (
	//go:embed data/*.gz
	dataFS embed.FS

	defaultData = sync.OnceValue(func() *prefixmap.Files {
		data, err := fs.Sub(dataFS, "data")
		if err != nil {
			panic(err)
		}
		return prefixmap.NewFiles(data)
	})
)

// Name returns the carrier of n, a mobile or pager number, in language
// lang, or "" for other numbers and those with no data.
func Name(n phonenumber.Number, lang string) string { return name(defaultData(), n, lang) }

// NameForValid returns the carrier the data gives for n, which is assumed
// to be valid, of any type: fixed-line numbers can have one too. It does
// not check the number.
func NameForValid(n phonenumber.Number, lang string) string {
	return nameForValid(defaultData(), n, lang)
}

// SafeDisplayName is [Name], except that it returns "" for numbers from
// regions with mobile number portability, where the carrier a number was
// given by may no longer serve it.
func SafeDisplayName(n phonenumber.Number, lang string) string {
	return safeDisplayName(defaultData(), n, lang)
}

func name(f *prefixmap.Files, n phonenumber.Number, lang string) string {
	switch n.Type() {
	case phonenumber.Mobile, phonenumber.FixedLineOrMobile, phonenumber.Pager:
		return nameForValid(f, n, lang)
	}
	return ""
}

func nameForValid(f *prefixmap.Files, n phonenumber.Number, tag string) string {
	lang, _, region := prefixmap.ParseLanguage(tag)
	// Upstream passes no script.
	return f.Description(n.CountryCode(), n.NationalNumber(), lang, "", region)
}

func safeDisplayName(f *prefixmap.Files, n phonenumber.Number, lang string) string {
	if phonenumber.MobileNumberPortable(n.Region()) {
		return ""
	}
	return name(f, n, lang)
}
