// Package phonenumber parses, validates and formats international phone
// numbers. It is a Go port of Google's libphonenumber and uses the same
// numbering-plan metadata.
//
// [Parse] reads a number written in any common format and returns a
// [Number], a small comparable value:
//
//	n, err := phonenumber.Parse("020 7946 0958", "GB")
//	if err != nil {
//		// errors.Is(err, phonenumber.ErrNotANumber), ...
//	}
//	n.Valid()                           // true
//	n.Type()                            // phonenumber.FixedLine
//	n.Format(phonenumber.International) // "+44 20 7946 0958"
//	n.FormatFrom("US")                  // "011 44 20 7946 0958"
//
// The region passed to Parse is used only for numbers written without a
// country calling code. Parse accepts numbers that are not valid in their
// region's numbering plan, since a number can be worth keeping even if it
// fails today's rules; check [Number.Valid] before relying on one.
//
// # Encoding
//
// Number implements [encoding.TextMarshaler] and
// [encoding.TextUnmarshaler] using E.164 with an RFC 3966 extension, such as
// "+442079460958;ext=123", so it works as a field with encoding/json and
// other encoders, and with libraries that bind request data through
// UnmarshalText. Text has no default region, so it must start with a plus
// sign. Number also implements [database/sql.Scanner] and [driver.Valuer].
//
// # Metadata
//
// Numbering plans change often. The metadata is generated from upstream's
// XML into metadata_gen.go; resources/UPSTREAM records the upstream commit it
// came from. Regular expressions are compiled when first used, so a program
// pays only for the regions it sees.
package phonenumber
