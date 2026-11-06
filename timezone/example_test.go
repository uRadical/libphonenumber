package timezone_test

import (
	"fmt"

	"uradical.io/go/phonenumber"
	"uradical.io/go/phonenumber/timezone"
)

func ExampleForNumber() {
	n, _ := phonenumber.Parse("+1 212 812 0000", "")
	fmt.Println(timezone.ForNumber(n))
	// Output:
	// [America/New_York]
}
