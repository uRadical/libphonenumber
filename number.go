package phonenumber

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
)

// Number is a parsed phone number: a country calling code, a national
// significant number and an optional extension.
//
// Number is a small comparable value. Two Numbers are == when they hold the
// same digits, so a Number can be used as a map key. The zero Number holds
// no number; see [Number.IsZero].
//
// A Number need not be valid: [Parse] accepts anything shaped like a phone
// number, and [Number.Valid] checks it against the numbering plan.
type Number struct {
	national    uint64 // national significant number, without leading zeros
	ext         string
	countryCode uint16
	zeros       uint8 // leading zeros of the national number, as in Italy
}

// IsZero reports whether n is the zero Number.
func (n Number) IsZero() bool { return n == Number{} }

// CountryCode returns the country calling code, such as 44 for the United
// Kingdom, or 0 for the zero Number.
func (n Number) CountryCode() int { return int(n.countryCode) }

// NationalNumber returns the national significant number: the digits after
// the country calling code, without any national prefix. For +44 20 7946
// 0958 it is "2079460958".
func (n Number) NationalNumber() string {
	if n.IsZero() {
		return ""
	}
	// Upstream caps the leading zeros it prints at ten.
	z := min(int(n.zeros), 10)
	return strings.Repeat("0", z) + strconv.FormatUint(n.national, 10)
}

// Extension returns the extension, or "" if there is none.
func (n Number) Extension() string { return n.ext }

// String returns n in E.164 format, followed by ";ext=" and the extension
// when there is one, such as "+442079460958;ext=123". [Parse] accepts this
// form. The zero Number formats as "".
func (n Number) String() string {
	if n.IsZero() {
		return ""
	}
	s := n.Format(E164)
	if n.ext != "" {
		s += rfc3966ExtnPrefix + n.ext
	}
	return s
}

// MarshalText implements [encoding.TextMarshaler] using the form returned by
// [Number.String].
func (n Number) MarshalText() ([]byte, error) {
	return []byte(n.String()), nil
}

// UnmarshalText implements [encoding.TextUnmarshaler]. Text has no default
// region, so it must be an international number: one that starts with a plus
// sign, such as "+44 20 7946 0958". Empty text sets the zero Number.
//
// Like [Parse], it does not check the number is valid; use [Number.Valid].
// A valid Number always reads back as itself. An invalid one may not, since
// parsing removes what looks like a national prefix even after the plus sign.
func (n *Number) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		*n = Number{}
		return nil
	}
	p, err := Parse(string(text), "")
	if err != nil {
		return err
	}
	*n = p
	return nil
}

// Value implements [driver.Valuer]. It stores n in the form returned by
// [Number.String], and the zero Number as NULL.
func (n Number) Value() (driver.Value, error) {
	if n.IsZero() {
		return nil, nil
	}
	return n.String(), nil
}

// Scan implements [database/sql.Scanner]. It accepts a string or []byte in
// any form [Number.UnmarshalText] does, and NULL as the zero Number.
func (n *Number) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*n = Number{}
		return nil
	case string:
		return n.UnmarshalText([]byte(v))
	case []byte:
		return n.UnmarshalText(v)
	default:
		return fmt.Errorf("phonenumber: cannot scan %T into Number", src)
	}
}

// Type is the kind of service a number reaches, such as a mobile phone or a
// toll-free line.
type Type int

const (
	// TypeUnknown means the number does not match any type in its region's
	// numbering plan, which makes it invalid.
	TypeUnknown Type = iota
	FixedLine
	Mobile
	// FixedLineOrMobile is used where the numbering plan cannot tell fixed
	// lines and mobiles apart, as in the United States.
	FixedLineOrMobile
	TollFree
	PremiumRate
	// SharedCost numbers split the cost of the call between caller and
	// recipient.
	SharedCost
	VoIP
	// PersonalNumber numbers route to another number of the subscriber's
	// choosing.
	PersonalNumber
	Pager
	// UAN is a universal access number: a single number for a company that
	// routes to its offices.
	UAN
	Voicemail
)

var typeNames = [...]string{
	TypeUnknown:       "TypeUnknown",
	FixedLine:         "FixedLine",
	Mobile:            "Mobile",
	FixedLineOrMobile: "FixedLineOrMobile",
	TollFree:          "TollFree",
	PremiumRate:       "PremiumRate",
	SharedCost:        "SharedCost",
	VoIP:              "VoIP",
	PersonalNumber:    "PersonalNumber",
	Pager:             "Pager",
	UAN:               "UAN",
	Voicemail:         "Voicemail",
}

func (t Type) String() string {
	if t >= 0 && int(t) < len(typeNames) {
		return typeNames[t]
	}
	return "Type(" + strconv.Itoa(int(t)) + ")"
}

// LengthCheck is the result of checking a number's length against its
// numbering plan; see [Number.CheckLength].
type LengthCheck int

const (
	// LengthOK means the number has a possible length.
	LengthOK LengthCheck = iota
	// LengthLocalOnly means the number is long enough to be dialled from
	// within its area, but not from elsewhere in the country.
	LengthLocalOnly
	// LengthInvalidCountryCode means the country calling code is not known.
	LengthInvalidCountryCode
	// LengthTooShort means the number is shorter than any possible length.
	LengthTooShort
	// LengthInvalid means the number falls between the possible lengths,
	// or its region has no numbers of the requested type.
	LengthInvalid
	// LengthTooLong means the number is longer than any possible length.
	LengthTooLong
)

var lengthCheckNames = [...]string{
	LengthOK:                 "LengthOK",
	LengthLocalOnly:          "LengthLocalOnly",
	LengthInvalidCountryCode: "LengthInvalidCountryCode",
	LengthTooShort:           "LengthTooShort",
	LengthInvalid:            "LengthInvalid",
	LengthTooLong:            "LengthTooLong",
}

func (c LengthCheck) String() string {
	if c >= 0 && int(c) < len(lengthCheckNames) {
		return lengthCheckNames[c]
	}
	return "LengthCheck(" + strconv.Itoa(int(c)) + ")"
}
