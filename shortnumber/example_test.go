package shortnumber_test

import (
	"fmt"

	"uradical.io/go/phonenumber"
	"uradical.io/go/phonenumber/shortnumber"
)

func Example() {
	n, err := phonenumber.Parse("15", "FR")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(shortnumber.ValidFor(n, "FR"), shortnumber.ExpectedCostFor(n, "FR"))
	// Output:
	// true TollFree
}

func ExampleIsEmergency() {
	fmt.Println(shortnumber.IsEmergency("911", "US"))
	fmt.Println(shortnumber.IsEmergency("+1 911", "US"))
	fmt.Println(shortnumber.IsEmergency("9116666666", "US"))
	fmt.Println(shortnumber.ConnectsToEmergency("9116666666", "US"))
	// Output:
	// true
	// false
	// false
	// true
}
