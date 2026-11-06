package phonenumber_test

import (
	"encoding/json"
	"errors"
	"fmt"

	"uradical.io/go/phonenumber"
)

func ExampleParse() {
	n, err := phonenumber.Parse("020 7946 0958", "GB")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(n)
	fmt.Println(n.Valid(), n.Type(), n.Region())
	// Output:
	// +442079460958
	// true FixedLine GB
}

func ExampleParse_errors() {
	_, err := phonenumber.Parse("020 7946 0958", "")
	fmt.Println(errors.Is(err, phonenumber.ErrInvalidCountryCode))
	// Output:
	// true
}

func ExampleNumber_Format() {
	n, _ := phonenumber.Parse("+1 650 253 0000 ext. 123", "")
	fmt.Println(n.Format(phonenumber.E164))
	fmt.Println(n.Format(phonenumber.International))
	fmt.Println(n.Format(phonenumber.National))
	fmt.Println(n.Format(phonenumber.RFC3966))
	// Output:
	// +16502530000
	// +1 650-253-0000 ext. 123
	// (650) 253-0000 ext. 123
	// tel:+1-650-253-0000;ext=123
}

func ExampleNumber_FormatFrom() {
	n, _ := phonenumber.Parse("+44 20 7946 0958", "")
	fmt.Println(n.FormatFrom("US"))
	fmt.Println(n.FormatFrom("DE"))
	fmt.Println(n.FormatFrom("GB"))
	// Output:
	// 011 44 20 7946 0958
	// 00 44 20 7946 0958
	// 020 7946 0958
}

func ExampleNumber_UnmarshalText() {
	var req struct {
		Phone phonenumber.Number `json:"phone"`
	}
	err := json.Unmarshal([]byte(`{"phone": "+44 7912 345678"}`), &req)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(req.Phone, req.Phone.Type())
	// Output:
	// +447912345678 Mobile
}

func ExampleMatchStrings() {
	fmt.Println(phonenumber.MatchStrings("+64 3 331 6005", "+64 03 331 6005"))
	fmt.Println(phonenumber.MatchStrings("+64 3 331 6005", "03 331 6005"))
	fmt.Println(phonenumber.MatchStrings("+64 3 331 6005", "331 6005"))
	fmt.Println(phonenumber.MatchStrings("+64 3 331 6005", "+61 3 331 6005"))
	fmt.Println(phonenumber.MatchStrings("+64 3 331 6005", "Dog"))
	// Output:
	// MatchExact
	// MatchNSN
	// MatchShortNSN
	// MatchNone
	// MatchNotANumber
}

func ExampleParseAndKeepRawInput() {
	p, err := phonenumber.ParseAndKeepRawInput("020 7946 0958", "GB")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(p.Number)
	fmt.Println(p.Source)
	fmt.Println(p.FormatInOriginalFormat("GB"))
	// Output:
	// +442079460958
	// FromDefaultCountry
	// 020 7946 0958
}

func ExampleNumber_FormatForMobileDialing() {
	n, _ := phonenumber.Parse("+44 20 7946 0958", "")
	fmt.Println(n.FormatForMobileDialing("GB", false))
	fmt.Println(n.FormatForMobileDialing("US", true))
	// Output:
	// 02079460958
	// +44 20 7946 0958
}
