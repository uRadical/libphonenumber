package phonenumber

// Numbers shared by the tests ported from upstream's PhoneNumberUtilTest.java.
// Each var is named after the upstream constant it mirrors, in camel case.
var (
	alphaNumericNumber           = num(1, 80074935247)
	arMobile                     = num(54, 91187654321)
	arNumber                     = num(54, 1187654321)
	auNumber                     = num(61, 236618300)
	bsMobile                     = num(1, 2423570000)
	bsNumber                     = num(1, 2423651234)
	deNumber                     = num(49, 30123456) // Same as DE's example number.
	deShortNumber                = num(49, 1234)
	gbMobile                     = num(44, 7912345678)
	gbNumber                     = num(44, 2070313000)
	itMobile                     = num(39, 345678901)
	itNumber                     = num(39, 236618300).withZeros(1)
	jpStarNumber                 = num(81, 2345)
	mxMobile1                    = num(52, 12345678900)
	mxMobile2                    = num(52, 15512345678)
	mxNumber1                    = num(52, 3312345678)
	mxNumber2                    = num(52, 8211234567)
	nzNumber                     = num(64, 33316005)
	sgNumber                     = num(65, 65218000)
	usLongNumber                 = num(1, 65025300001) // Too long, hence invalid.
	usNumber                     = num(1, 6502530000)
	usPremium                    = num(1, 9002530000)
	usLocalNumber                = num(1, 2530000) // Too short, but still possible.
	usTollFree                   = num(1, 8002530000)
	usSpoof                      = num(1, 0)
	internationalTollFree        = num(800, 12345678)
	internationalTollFreeTooLong = num(800, 123456789)
	universalPremiumRate         = num(979, 123456789)
	unknownCountryCodeNoRawInput = num(2, 12345)
)
