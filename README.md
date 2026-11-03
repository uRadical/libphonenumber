<p align="center">
  <img src=".github/logo.png" alt="phonenumber logo: a gopher on an old rotary phone" width="220">
</p>

<h1 align="center">phonenumber</h1>

<p align="center">
Parse, validate and format international phone numbers in Go.<br>
A port of Google's <a href="https://github.com/google/libphonenumber">libphonenumber</a>, with no dependencies.
</p>

---

```go
n, err := phonenumber.Parse("020 7946 0958", "GB")
if err != nil {
	return err
}
n.Valid()                           // true
n.Type()                            // phonenumber.FixedLine
n.Format(phonenumber.International) // "+44 20 7946 0958"
n.FormatFrom("US")                  // "011 44 20 7946 0958"
```

## Install

```sh
go get uradical.io/go/phonenumber
```

Requires Go 1.27 or later.

## Why another port?

- **A value type.** `Number` is a small comparable struct. Compare numbers
  with `==`, use them as map keys, and copy them freely. The zero `Number`
  means "no number".
- **No dependencies.** The numbering-plan metadata is generated into Go
  source, with no protobuf and no runtime XML parsing.
- **Fits the standard library.** `Number` implements
  `encoding.TextMarshaler`/`TextUnmarshaler` and `database/sql`'s `Scanner`
  and `Valuer`, so it works as a field with `encoding/json`, in database
  rows, and with request binders.
- **Pay for what you use.** Each region's regular expressions are compiled
  the first time a number from that region is seen.
- **Upstream's behaviour.** Upstream's own test suite is ported and runs
  against upstream's test metadata.

## Usage

### Parsing

`Parse` takes the number and the region it was written in. The region is
used only for numbers written without a country calling code. Pass `""` when
every number is international.

```go
phonenumber.Parse("020 7946 0958", "GB")          // national format
phonenumber.Parse("+44 (0)20-7946-0958", "")      // international
phonenumber.Parse("tel:+44-20-7946-0958", "")     // RFC 3966
phonenumber.Parse("1-800-FLOWERS", "US")          // letters
phonenumber.Parse("+1 650 253 0000 ext. 123", "") // extensions
```

`Parse` accepts anything shaped like a phone number, including numbers that
are not valid in their region's numbering plan. Call `Valid` before relying
on one. A failure wraps one of these errors, so test for them with
`errors.Is`:

| Error                   | Meaning                                                  |
| ----------------------- | -------------------------------------------------------- |
| `ErrNotANumber`         | The input does not look like a phone number.             |
| `ErrInvalidCountryCode` | Unknown calling code, or no `+` and no valid region.     |
| `ErrTooShortAfterIDD`   | Too few digits after an international prefix.           |
| `ErrTooShort`           | Fewer than two digits in the national number.           |
| `ErrTooLong`            | More than 250 characters, or more than 17 digits.       |

Errors never include the number itself, so they are safe to log.

### Validating

```go
n.Valid()                  // matches its region's numbering plan
n.ValidFor("CA")           // valid in this region in particular
n.Type()                   // FixedLine, Mobile, TollFree, VoIP, ...
n.Region()                 // "GB"; "001" for non-geographic numbers like +800
n.Possible()               // a quicker check of the length alone
n.CheckLength()            // LengthOK, LengthTooShort, LengthTooLong, ...
n.CanDialInternationally() // false for some freephone numbers
```

A valid number is not necessarily one that is in service.

### Formatting

| Call                       | Result                 |
| -------------------------- | ---------------------- |
| `n.Format(E164)`           | `+442079460958`        |
| `n.Format(International)`  | `+44 20 7946 0958`     |
| `n.Format(National)`       | `020 7946 0958`        |
| `n.Format(RFC3966)`        | `tel:+44-20-7946-0958` |
| `n.FormatFrom("US")`       | `011 44 20 7946 0958`  |

Extensions are included in every format except `E164`.

### Encoding and storage

`Number` encodes as E.164, with an RFC 3966 extension when it has one, such as
`+442079460958;ext=123`:

```go
type Contact struct {
	Name  string             `json:"name"`
	Phone phonenumber.Number `json:"phone"`
}
```

When decoding, text has no default region, so a number must start with `+`.
An empty string decodes to the zero `Number`, and the zero `Number` is stored
in a database as `NULL`. Decoding does not check validity.

A valid number always reads back as itself. An invalid one may not, because
parsing removes what looks like a national prefix even after a `+`, as
upstream does.

### With binder

[binder](https://gobinder.dev) fills any `encoding.TextUnmarshaler`, so
`Number` binds with no extra code. Put the validity check in `Validate`:

```go
type CreateContact struct {
	Name  string             `body:"name,required"`
	Phone phonenumber.Number `body:"phone,required"`
}

func (c CreateContact) Validate(ctx context.Context) error {
	if !c.Phone.Valid() {
		return errors.New("phone: not a valid phone number")
	}
	return nil
}
```

A number that cannot be parsed fails binding as a `BindError` (400), and a
parsed but invalid number fails `Validate` (422). In a query string, a
literal `+` decodes as a space, so clients must send it as `%2B`.

If clients send national numbers with a separate country field, bind both as
strings and call `phonenumber.Parse(phone, country)` yourself.

### Regions

```go
phonenumber.Regions()                  // ["AC", "AD", "AE", ...]
phonenumber.CountryCodeForRegion("GB") // 44
phonenumber.RegionsForCountryCode(1)   // ["US", "AG", "AI", ...]
phonenumber.NationalPrefix("GB")       // "0"
phonenumber.ExampleNumber("GB", phonenumber.Mobile)
phonenumber.ExampleNumberForNonGeo(800)  // +800 1234 5678
phonenumber.SupportedTypes("GB")
```

## Status

Ported so far: parsing, validation, number types, possible-length checks and
formatting, which is upstream's `PhoneNumberUtil` core.

Not yet ported:

- formatting with carrier codes, and keeping the original input
- comparing two numbers for a match
- short numbers, such as emergency and service codes
- the as-you-type formatter
- finding numbers in text
- geocoding, carrier and timezone data

## Metadata

Numbering plans change every few weeks. The upstream XML lives in
`resources/`, and `resources/UPSTREAM` records the commit it came from. To
update it, copy in the new XML and regenerate:

```sh
go generate ./...
go test ./...
```

The generator in `internal/cmd/genmetadata` compiles every pattern with Go's
`regexp` package, so an expression that RE2 cannot run fails the build
rather than your program.

## Development

```sh
make install-hooks   # once after cloning: gofmt and vet before each commit
make check           # what CI checks: lint, vet, race tests, govulncheck, CRAP
make fuzz            # every fuzz target, 30s each; FUZZTIME=10m for longer
make bench           # benchmarks
make mutants         # mutation testing with gremlins (not in CI)
```

- **Lint:** golangci-lint v2 with the configuration in `.golangci.yml`. A
  library must not print or log, which `forbidigo` enforces.
- **CRAP:** every function must score under 30, which weighs cyclomatic
  complexity against missing test coverage. Only generated files are exempt.
- **Fuzzing:** the targets cover parsing, formatting round trips, the
  Java-style group replacement, normalization and number extraction.
  Inputs the fuzzer finds go in `testdata/fuzz` and run with every
  `go test`.
- **Generated code:** the generator's tests regenerate both metadata files
  and fail if they differ from the committed ones.

CI runs the same Makefile targets, with every action pinned to a commit
SHA, and Dependabot keeps the actions current.

## License

Apache License 2.0, the same as upstream. See [LICENSE](LICENSE) and
[NOTICE](NOTICE). This is an independent port and is not affiliated with
Google.
