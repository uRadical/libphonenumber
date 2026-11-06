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

### Keeping the original input

`ParseAndKeepRawInput` returns a `Parsed`: the `Number`, the input as given,
how its country calling code was written, and any carrier selection code.
Store the `Number`; use the `Parsed` to write a number back the way the user
did:

```go
p, _ := phonenumber.ParseAndKeepRawInput("020 7946 0958", "GB")
p.Number                            // +442079460958
p.FormatInOriginalFormat("GB")      // "020 7946 0958"
```

`Number` holds no raw input, so `==`, map keys and stored values depend only
on the number. Other formats:

- `Parsed.FormatOutOfCountryKeepingAlphaChars` keeps letters, as in
  "0011 1 800 SIX-FLAG".
- `Number.FormatNationalWithCarrierCode` and
  `Parsed.FormatNationalWithPreferredCarrierCode` write a carrier selection
  code, as Brazil and Colombia use.
- `FormatForMobileDialing` writes a number to dial from a mobile in a given
  region, or "" if it cannot be dialled from there.
- `Number.FormatByPattern` formats with your own grouping rules.

### Comparing numbers

`==` tells you two `Number`s are the same. To compare numbers that may be
written without a country calling code, use `Match`, `MatchString` or
`MatchStrings`. The result says how sure the match is:

```go
phonenumber.MatchStrings("+64 3 331 6005", "+64 03 331 6005") // MatchExact
phonenumber.MatchStrings("+64 3 331 6005", "03 331 6005")     // MatchNSN
phonenumber.MatchStrings("+64 3 331 6005", "331 6005")        // MatchShortNSN
phonenumber.MatchStrings("+64 3 331 6005", "+61 3 331 6005")  // MatchNone
```

`MatchNSN` means the national numbers are the same but one has no country
calling code, so they may be in different countries. `MatchShortNSN` means
one could be a shorter form of the other, such as a local number without its
area code. Larger values are closer matches, so `m >= phonenumber.MatchNSN`
reads as "the same number".

### Finding numbers in text

`FindNumbers` returns an iterator over the numbers in a piece of text:

```go
text := "Call us on 020 7946 0958, or +1 650-253-0000 from abroad."
for m := range phonenumber.FindNumbers(text, "GB") {
	fmt.Println(m.Raw, m.Number) // "020 7946 0958 +442079460958", ...
}
```

`m.Start` and `m.End` are byte offsets, so `text[m.Start:m.End]` is `m.Raw`.
`FindNumbers` accepts valid numbers only. `FindNumbersWithLeniency` takes a
`Leniency`, from `LeniencyPossible` to `LeniencyExactGrouping` (which also
checks the digits are grouped as the region writes them), and a limit on
failed candidates for text full of false positives.

### Formatting as you type

`AsYouTypeFormatter` formats a number one character at a time, as a dialler
does:

```go
f := phonenumber.NewAsYouTypeFormatter("US")
for _, r := range "6502530000" {
	fmt.Println(f.InputDigit(r)) // "6", "65", "650", "650-2", ... "(650) 253-0000"
}
f.Clear() // before the next number
```

`InputDigitAndRememberPosition` and `RememberedPosition` track where a
character ends up as formatting is added around it, for keeping a cursor in
place. A formatter is not safe for concurrent use.

### Short numbers

Package `shortnumber` covers emergency numbers and service codes such as
112, 911 or 116 123, from upstream's short-number metadata:

```go
n, _ := phonenumber.Parse("15", "FR")
shortnumber.ValidFor(n, "FR")        // true
shortnumber.ExpectedCostFor(n, "FR") // shortnumber.TollFree

shortnumber.IsEmergency("911", "US")                // true
shortnumber.IsEmergency("9116666666", "US")         // false
shortnumber.ConnectsToEmergency("9116666666", "US") // true
```

Emergency checks take the string as dialled, since "911" reaches the
emergency services where "+1 911" may not. Functions with a `For` suffix take
the region the number is dialled from; the others look at every region that
shares the number's country calling code.

### Where a number is: geocoding, carriers and time zones

Three subpackages carry upstream's prefix data. Each embeds its data, so a
program pays for it only if it imports the package, and decompresses a
country's data the first time it looks a number up:

```go
n, _ := phonenumber.Parse("+44 20 7946 0958", "")
geocoder.Description(n, "en") // "London"
timezone.ForNumber(n)         // ["Europe/London"]

m, _ := phonenumber.Parse("+44 7400 123456", "")
carrier.Name(m, "en")            // "Three"
carrier.SafeDisplayName(m, "en") // "": the UK has mobile number portability
```

| Package | Upstream class | Adds to a binary |
| --- | --- | --- |
| `geocoder` | `PhoneNumberOfflineGeocoder` | about 2 MB |
| `carrier` | `PhoneNumberToCarrierMapper` | about 110 KB |
| `timezone` | `PhoneNumberToTimeZonesMapper` | about 12 KB |

Languages are tags such as `"en"`, `"de-CH"` or `"zh-TW"`. When the geocoder
has no area for a number, such as a mobile, it gives the country's name,
which comes from Unicode CLDR for about 100 languages, as upstream's comes
from Java's locale data. Carrier names can be out of date wherever numbers
can move between carriers; `SafeDisplayName` returns "" in those regions.

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

Ported: parsing (with or without the original input), validation,
number types, possible-length checks, formatting (including carrier codes,
mobile dialling and custom rules), comparing numbers, finding numbers in
text (`PhoneNumberMatcher`), formatting as you type (`AsYouTypeFormatter`),
area code lengths and the other helpers of upstream's `PhoneNumberUtil`, and
in subpackages short numbers (`ShortNumberInfo`), geocoding
(`PhoneNumberOfflineGeocoder`), carriers (`PhoneNumberToCarrierMapper`) and
time zones (`PhoneNumberToTimeZonesMapper`). Every public class of
upstream's Java library has a Go counterpart, with an API that follows Go
conventions: `Number` is a comparable value, iterators replace Java's
`Iterator`, and offsets are byte offsets.

## Metadata

Numbering plans change every few weeks. The upstream XML and prefix data
live in `resources/`, and `resources/UPSTREAM` records the commit they came
from. A scheduled workflow, `upstream-sync`, checks every day for a new
upstream release. When it finds one, it copies in the data, regenerates, runs
the tests and opens a pull request. Merging that pull request tags the next
patch release, so a data update reaches users as, say, v1.4.2 to v1.4.3. Label
the pull request `release:minor` to bump the minor version instead, or
`release:skip` to tag nothing. To sync by hand:

```sh
make sync-upstream          # latest release; or scripts/sync-upstream.sh v9.0.43
go test ./...
```

The sync never moves the data to an older commit than `resources/UPSTREAM`
records. Set `FORCE=1` to sync anyway.

The generator in `internal/cmd/genmetadata` compiles every pattern with Go's
`regexp` package, so an expression that RE2 cannot run fails the build
rather than your program. `internal/cmd/genprefixdata` turns the geocoding,
carrier and time zone text files into the compressed tables the
subpackages embed, applying upstream's build steps.

The geocoder's country names come from Unicode CLDR. To update them, which
needs network access:

```sh
go run ./internal/cmd/fetchcldr -tag 48.2.3 -out resources/cldr/territories.tsv
go generate ./...
```

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
  Java-style group replacement, normalization, number extraction, matching,
  finding numbers in text, formatting as you type, emergency numbers and the
  data lookups.
  Inputs the fuzzer finds go in `testdata/fuzz` and run with every
  `go test`.
- **Generated code:** the generators' tests regenerate every metadata file
  and data table and fail if they differ from the committed ones.

CI runs the same Makefile targets, with every action pinned to a commit
SHA, and Dependabot keeps the actions current.

## License

Apache License 2.0, the same as upstream. See [LICENSE](LICENSE) and
[NOTICE](NOTICE). This is an independent port and is not affiliated with
Google.
