package carrier_test

import (
	"fmt"

	"uradical.io/go/phonenumber"
	"uradical.io/go/phonenumber/carrier"
)

func ExampleName() {
	n, _ := phonenumber.Parse("+44 7400 123456", "")
	fmt.Printf("%q\n", carrier.Name(n, "en"))
	// The UK has mobile number portability.
	fmt.Printf("%q\n", carrier.SafeDisplayName(n, "en"))
	// Output:
	// "Three"
	// ""
}
