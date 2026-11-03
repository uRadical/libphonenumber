package phonenumber

import (
	"slices"
	"strings"
)

// Valid reports whether n is a valid number in its region's numbering plan:
// its country calling code is known, it has a possible length, and it
// matches the pattern of at least one type of number. Validity says nothing
// about whether the number is in service.
func (n Number) Valid() bool { return defaultSet().valid(n) }

// ValidFor reports whether n is a valid number for region in particular.
// Numbers valid in one region of a shared country calling code, such as +1,
// may not be valid in the others.
func (n Number) ValidFor(region string) bool {
	return defaultSet().validFor(n, strings.ToUpper(region))
}

// Type returns the type of n, or [TypeUnknown] if n is not valid.
func (n Number) Type() Type { return defaultSet().numberType(n) }

// Region returns the region code n belongs to, such as "GB", "001" for a
// non-geographical number such as +800 freephone, or "" if n is not valid
// for any region.
func (n Number) Region() string { return defaultSet().regionFor(n) }

// Possible reports whether n has a possible length for its region. It is a
// quicker, looser check than [Number.Valid], and still holds for numbers
// that are only diallable locally.
func (n Number) Possible() bool {
	c := n.CheckLength()
	return c == LengthOK || c == LengthLocalOnly
}

// CheckLength checks the length of n against its region's numbering plan.
func (n Number) CheckLength() LengthCheck { return defaultSet().checkLength(n, TypeUnknown) }

// CheckLengthForType checks the length of n against the numbers of type typ
// in its region's numbering plan.
func (n Number) CheckLengthForType(typ Type) LengthCheck { return defaultSet().checkLength(n, typ) }

// CanDialInternationally reports whether n can be dialled from outside its
// region. Some numbers, such as many toll-free ones, can only be dialled
// from within it.
func (n Number) CanDialInternationally() bool {
	return defaultSet().canDialInternationally(n)
}

func (ms *metadataSet) valid(n Number) bool {
	return ms.validFor(n, ms.regionFor(n))
}

func (ms *metadataSet) validFor(n Number, region string) bool {
	cc := int(n.countryCode)
	meta := ms.forCode(cc, region)
	if meta == nil || (region != regionNonGeo && meta.countryCode != cc) {
		return false
	}
	return meta.numberType(n.NationalNumber()) != TypeUnknown
}

func (ms *metadataSet) numberType(n Number) Type {
	meta := ms.forCode(int(n.countryCode), ms.regionFor(n))
	if meta == nil {
		return TypeUnknown
	}
	return meta.numberType(n.NationalNumber())
}

// regionFor finds the region of n among the regions that share its country
// calling code.
func (ms *metadataSet) regionFor(n Number) string {
	regions := ms.regionsForCode[int(n.countryCode)]
	switch len(regions) {
	case 0:
		return ""
	case 1:
		return regions[0]
	}
	nsn := n.NationalNumber()
	for _, r := range regions {
		meta := ms.region(r)
		if meta.leadingDigits != nil {
			if meta.leadingDigits.matchPrefix(nsn) {
				return r
			}
		} else if meta.numberType(nsn) != TypeUnknown {
			return r
		}
	}
	return ""
}

func (ms *metadataSet) checkLength(n Number, typ Type) LengthCheck {
	cc := int(n.countryCode)
	if !ms.knownCode(cc) {
		return LengthInvalidCountryCode
	}
	meta := ms.forCode(cc, ms.mainRegion(cc))
	return meta.testLength(n.NationalNumber(), typ)
}

func (ms *metadataSet) canDialInternationally(n Number) bool {
	meta := ms.region(ms.regionFor(n))
	if meta == nil {
		return true
	}
	return !meta.noInternationalDialling.matches(n.NationalNumber())
}

// numberType classifies a national significant number. Types are tried
// from the most specific to the least, as upstream does.
func (t *territory) numberType(nsn string) Type {
	if !t.generalDesc.matches(nsn) {
		return TypeUnknown
	}
	for _, typ := range []Type{PremiumRate, TollFree, SharedCost, VoIP, PersonalNumber, Pager, UAN, Voicemail} {
		if t.descFor(typ).matches(nsn) {
			return typ
		}
	}
	if t.fixedLine.matches(nsn) {
		if t.sameMobileAndFixedLinePattern || t.mobile.matches(nsn) {
			return FixedLineOrMobile
		}
		return FixedLine
	}
	if !t.sameMobileAndFixedLinePattern && t.mobile.matches(nsn) {
		return Mobile
	}
	return TypeUnknown
}

// testLength checks the length of a national significant number against
// the possible lengths of typ, or of every type for TypeUnknown.
func (t *territory) testLength(nsn string, typ Type) LengthCheck {
	desc := t.descFor(typ)
	lengths := desc.lengths
	if len(lengths) == 0 {
		lengths = t.generalDesc.lengths
	}
	local := desc.localOnly
	if typ == FixedLineOrMobile {
		if !t.fixedLine.hasPossibleLengthData() {
			return t.testLength(nsn, Mobile)
		}
		if mobile := &t.mobile; mobile.hasPossibleLengthData() {
			ml := mobile.lengths
			if len(ml) == 0 {
				ml = t.generalDesc.lengths
			}
			lengths = slices.Concat(lengths, ml)
			slices.Sort(lengths)
			local = slices.Concat(local, mobile.localOnly)
			slices.Sort(local)
		}
	}
	if len(lengths) == 0 || lengths[0] == -1 {
		return LengthInvalid
	}
	l := len(nsn)
	if hasLength(local, l) {
		return LengthLocalOnly
	}
	switch {
	case int(lengths[0]) == l:
		return LengthOK
	case int(lengths[0]) > l:
		return LengthTooShort
	case int(lengths[len(lengths)-1]) < l:
		return LengthTooLong
	case hasLength(lengths[1:], l):
		return LengthOK
	}
	return LengthInvalid
}
