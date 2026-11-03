package phonenumber

import (
	"sync"
	"testing"
)

// testSet is built from upstream's PhoneNumberMetadataForTesting.xml, the
// metadata upstream's own tests run against. Its numbering plans are
// simplified and differ from the real ones.
var testSet = sync.OnceValue(func() *metadataSet { return newMetadataSet(testTerritoryData) })

// num builds a Number the way upstream's tests build PhoneNumber protos.
func num(cc int, national uint64) Number {
	return Number{countryCode: uint16(cc), national: national}
}

// withZeros sets the number of leading zeros, as upstream's
// setItalianLeadingZero(true) (one zero) and setNumberOfLeadingZeros do.
func (n Number) withZeros(z int) Number {
	n.zeros = uint8(z)
	return n
}

// withExt sets the extension.
func (n Number) withExt(ext string) Number {
	n.ext = ext
	return n
}

// mustParseTest parses s with the test metadata and fails the test on error.
func mustParseTest(t *testing.T, s, region string) Number {
	t.Helper()
	n, err := testSet().parse(s, region)
	if err != nil {
		t.Fatalf("parse(%q, %q): %v", s, region, err)
	}
	return n
}
