package geocoder_test

import (
	"fmt"

	"uradical.io/go/phonenumber"
	"uradical.io/go/phonenumber/geocoder"
)

func ExampleDescription() {
	n, _ := phonenumber.Parse("+1 650 253 0000", "")
	fmt.Println(geocoder.Description(n, "en"))
	// A mobile number is described by its country.
	m, _ := phonenumber.Parse("+33 6 12 34 56 78", "")
	fmt.Println(geocoder.Description(m, "de"))
	// Output:
	// Mountain View, CA
	// Frankreich
}

func ExampleDescriptionFrom() {
	n, _ := phonenumber.Parse("+44 20 7946 0958", "")
	fmt.Println(geocoder.DescriptionFrom(n, "en", "GB"))
	fmt.Println(geocoder.DescriptionFrom(n, "en", "US"))
	// Output:
	// London
	// United Kingdom
}
