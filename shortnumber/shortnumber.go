// Package shortnumber reports on short numbers, such as emergency numbers
// and service codes like 112, 911 or 116 123. It is the Go port of
// upstream libphonenumber's ShortNumberInfo.
//
// Most functions take a [phonenumber.Number] parsed with the region it is
// dialled in:
//
//	n, _ := phonenumber.Parse("15", "FR")
//	shortnumber.ValidFor(n, "FR")       // true
//	shortnumber.ExpectedCostFor(n, "FR") // shortnumber.TollFree
//
// Emergency checks take the string as dialled instead, since "911" may
// reach the emergency services where "+1 911" does not:
//
//	shortnumber.IsEmergency("911", "US") // true
//
// Functions with a For suffix take the region the number is dialled from,
// and report false or [CostUnknown] when the number's country calling code
// is not that region's. The others look at every region that shares the
// number's country calling code. Region codes are not case sensitive.
//
// Commercial short codes are mostly not covered: the metadata lists
// emergency, public service and some carrier and SMS numbers.
package shortnumber

import (
	"slices"
	"strconv"
	"strings"

	"uradical.io/go/phonenumber"
	"uradical.io/go/phonenumber/internal/phonetext"
)

// Cost is the expected cost of calling a short number.
type Cost int

const (
	// CostUnknown means the number matches no cost category.
	CostUnknown Cost = iota
	TollFree
	StandardRate
	PremiumRate
)

var costNames = [...]string{
	CostUnknown:  "CostUnknown",
	TollFree:     "TollFree",
	StandardRate: "StandardRate",
	PremiumRate:  "PremiumRate",
}

func (c Cost) String() string {
	if c >= 0 && int(c) < len(costNames) {
		return costNames[c]
	}
	return "Cost(" + strconv.Itoa(int(c)) + ")"
}

// regionsWhereEmergencyNumbersMustBeExact lists the regions where an
// emergency number with extra digits after it no longer connects.
var regionsWhereEmergencyNumbersMustBeExact = []string{"BR", "CL", "NI"}

// metadataFor returns the short-number metadata for a region, or nil.
func metadataFor(region string) *territory {
	return regions()[strings.ToUpper(region)]
}

// regionMatches reports whether n's country calling code belongs to region.
func regionMatches(n phonenumber.Number, region string) bool {
	return slices.Contains(phonenumber.RegionsForCountryCode(n.CountryCode()), strings.ToUpper(region))
}

// PossibleFor reports whether n has the length of a short number in
// region. It is a looser check than [ValidFor].
func PossibleFor(n phonenumber.Number, region string) bool {
	if !regionMatches(n, region) {
		return false
	}
	t := metadataFor(region)
	return t != nil && hasLength(t.generalDesc.lengths, len(n.NationalNumber()))
}

// Possible reports whether n has the length of a short number in any
// region that shares its country calling code. It is a looser check than
// [Valid].
func Possible(n phonenumber.Number) bool {
	l := len(n.NationalNumber())
	for _, r := range phonenumber.RegionsForCountryCode(n.CountryCode()) {
		if t := metadataFor(r); t != nil && hasLength(t.generalDesc.lengths, l) {
			return true
		}
	}
	return false
}

// ValidFor reports whether n matches a short number pattern in region.
// That does not mean the number is in use.
func ValidFor(n phonenumber.Number, region string) bool {
	if !regionMatches(n, region) {
		return false
	}
	return validIn(metadataFor(region), n.NationalNumber())
}

func validIn(t *territory, nsn string) bool {
	return t != nil && t.generalDesc.matches(nsn) && t.shortCode.matches(nsn)
}

// Valid reports whether n matches a short number pattern in any region
// that shares its country calling code. That does not mean the number is
// in use.
func Valid(n phonenumber.Number) bool {
	rs := phonenumber.RegionsForCountryCode(n.CountryCode())
	region, ok := regionFor(n, rs)
	if len(rs) > 1 && ok {
		// regionFor found a region where the number is valid.
		return true
	}
	return ok && validIn(metadataFor(region), n.NationalNumber())
}

// regionFor returns the region in rs that n is a short number of: the
// only one, or else the first where it matches the short code pattern.
func regionFor(n phonenumber.Number, rs []string) (string, bool) {
	switch len(rs) {
	case 0:
		return "", false
	case 1:
		return rs[0], true
	}
	nsn := n.NationalNumber()
	for _, r := range rs {
		if t := metadataFor(r); t != nil && t.shortCode.matches(nsn) {
			return r, true
		}
	}
	return "", false
}

// ExpectedCostFor returns the expected cost of calling n from region. It
// does not check that n is valid, and an invalid number may still match a
// cost category. Emergency numbers are toll free.
func ExpectedCostFor(n phonenumber.Number, region string) Cost {
	if !regionMatches(n, region) {
		return CostUnknown
	}
	t := metadataFor(region)
	if t == nil {
		return CostUnknown
	}
	nsn := n.NationalNumber()
	// A category stores no lengths when they equal the general
	// description's, so check those first.
	if !hasLength(t.generalDesc.lengths, len(nsn)) {
		return CostUnknown
	}
	// Most expensive first, in case the categories overlap.
	switch {
	case t.premiumRate.matches(nsn):
		return PremiumRate
	case t.standardRate.matches(nsn):
		return StandardRate
	case t.tollFree.matches(nsn), matchesEmergency(t, nsn, false):
		return TollFree
	}
	return CostUnknown
}

// ExpectedCost returns the expected cost of calling n. When regions share
// n's country calling code, it returns the highest cost among them in the
// order PremiumRate, CostUnknown, StandardRate, TollFree: a number of
// unknown cost in one region might be premium rate, so it outranks a
// known lower cost. Prefer [ExpectedCostFor] when the region is known.
func ExpectedCost(n phonenumber.Number) Cost {
	rs := phonenumber.RegionsForCountryCode(n.CountryCode())
	switch len(rs) {
	case 0:
		return CostUnknown
	case 1:
		return ExpectedCostFor(n, rs[0])
	}
	cost := TollFree
	for _, r := range rs {
		switch ExpectedCostFor(n, r) {
		case PremiumRate:
			return PremiumRate
		case CostUnknown:
			cost = CostUnknown
		case StandardRate:
			if cost != CostUnknown {
				cost = StandardRate
			}
		}
	}
	return cost
}

// CarrierSpecific reports whether n, assumed to be a valid short number,
// reaches a different service, or none, depending on the caller's carrier.
// Check validity first with [Valid].
func CarrierSpecific(n phonenumber.Number) bool {
	region, _ := regionFor(n, phonenumber.RegionsForCountryCode(n.CountryCode()))
	t := metadataFor(region)
	return t != nil && t.carrierSpecific.matches(n.NationalNumber())
}

// CarrierSpecificFor is [CarrierSpecific] for a number dialled from
// region. Check validity first with [ValidFor].
func CarrierSpecificFor(n phonenumber.Number, region string) bool {
	if !regionMatches(n, region) {
		return false
	}
	t := metadataFor(region)
	return t != nil && t.carrierSpecific.matches(n.NationalNumber())
}

// SMSServiceFor reports whether n, assumed to be a valid short number in
// region, is mainly for sending or receiving text messages. Check
// validity first with [ValidFor].
func SMSServiceFor(n phonenumber.Number, region string) bool {
	if !regionMatches(n, region) {
		return false
	}
	t := metadataFor(region)
	return t != nil && t.smsServices.matches(n.NationalNumber())
}

// IsEmergency reports whether s, exactly as dialled, is an emergency
// number in region. Formatting is ignored, but extra digits are not.
// IsEmergency implies [ConnectsToEmergency].
func IsEmergency(s, region string) bool { return emergency(s, region, false) }

// ConnectsToEmergency reports whether dialling s in region might reach
// the emergency services. Unlike [IsEmergency], it allows digits after an
// emergency number in regions where those still connect, as "9116666666"
// does in the US.
func ConnectsToEmergency(s, region string) bool { return emergency(s, region, true) }

func emergency(s, region string, allowPrefix bool) bool {
	s = phonetext.ExtractPossibleNumber(s)
	// Dialling a country calling code before an emergency number, as in
	// +1 911, is not known to work.
	if phonetext.PlusCharsPattern().MatchString(s) {
		return false
	}
	t := metadataFor(region)
	if t == nil {
		return false
	}
	allowPrefix = allowPrefix && !slices.Contains(regionsWhereEmergencyNumbersMustBeExact, strings.ToUpper(region))
	return matchesEmergency(t, phonetext.NormalizeDigits(s, false), allowPrefix)
}

// matchesEmergency matches number against t's emergency pattern, ignoring
// possible lengths, as upstream does.
func matchesEmergency(t *territory, number string, allowPrefix bool) bool {
	p := t.emergency.pattern
	if p == nil {
		return false
	}
	if allowPrefix {
		return p.matchPrefix(number)
	}
	return p.matchFull(number)
}

// exampleNumber returns a valid short number for region, or "".
func exampleNumber(region string) string {
	if t := metadataFor(region); t != nil {
		return t.shortCode.example
	}
	return ""
}

// exampleForCost returns a short number of the given cost in region, or
// "" when there is none or the cost is CostUnknown.
func exampleForCost(region string, c Cost) string {
	t := metadataFor(region)
	if t == nil {
		return ""
	}
	switch c {
	case TollFree:
		return t.tollFree.example
	case StandardRate:
		return t.standardRate.example
	case PremiumRate:
		return t.premiumRate.example
	}
	return ""
}
