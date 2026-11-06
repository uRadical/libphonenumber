package phonenumber

import (
	"regexp"
	"slices"
	"sync"
)

//go:generate go run ./internal/cmd/genmetadata -in resources/PhoneNumberMetadata.xml -out metadata_gen.go -var territoryData
//go:generate go run ./internal/cmd/genmetadata -in resources/PhoneNumberMetadataForTesting.xml -out testmetadata_gen_test.go -var testTerritoryData
//go:generate go run ./internal/cmd/genmetadata -alternate -in resources/PhoneNumberAlternateFormats.xml -out alternateformats_gen.go -var alternateFormatData
//go:generate go run ./internal/cmd/genprefixdata -in resources/geocoding -out geocoder/data
//go:generate go run ./internal/cmd/genprefixdata -in resources/test/geocoding -out geocoder/testdata/geocoding
//go:generate go run ./internal/cmd/genprefixdata -names -in resources/cldr/territories.tsv -out geocoder/names.gz
//go:generate go run ./internal/cmd/genprefixdata -in resources/carrier -out carrier/data
//go:generate go run ./internal/cmd/genprefixdata -in resources/test/carrier -out carrier/testdata/carrier
//go:generate go run ./internal/cmd/genprefixdata -single -in resources/timezones/map_data.txt -out timezone/data/map_data.gz
//go:generate go run ./internal/cmd/genprefixdata -single -in resources/test/timezones/map_data.txt -out timezone/testdata/map_data.gz
//go:generate go run ./internal/cmd/genmetadata -short -pkg shortnumber -in resources/ShortNumberMetadata.xml -out shortnumber/metadata_gen.go -var territoryData

// regionNonGeo is the region code upstream uses for non-geographical
// entities such as international freephone (+800).
const regionNonGeo = "001"

// territory is the numbering plan of one region, or of one non-geographical
// country calling code. Field meanings follow upstream's phonemetadata.proto.
type territory struct {
	id                            string
	countryCode                   int
	leadingDigits                 *pattern
	internationalPrefix           *pattern
	preferredInternationalPrefix  string
	nationalPrefix                string
	nationalPrefixForParsing      *pattern
	nationalPrefixTransformRule   string
	preferredExtnPrefix           string
	mainCountryForCode            bool
	mobileNumberPortableRegion    bool
	sameMobileAndFixedLinePattern bool

	numberFormats     []*numberFormat
	intlNumberFormats []*numberFormat

	generalDesc             numberDesc
	fixedLine               numberDesc
	mobile                  numberDesc
	tollFree                numberDesc
	premiumRate             numberDesc
	sharedCost              numberDesc
	personalNumber          numberDesc
	voip                    numberDesc
	pager                   numberDesc
	uan                     numberDesc
	voicemail               numberDesc
	noInternationalDialling numberDesc
}

// numberDesc describes the numbers of one type within a territory.
type numberDesc struct {
	pattern *pattern
	example string
	// lengths lists the possible lengths of a national significant number.
	// An empty list means "the same as the general description", and the
	// list [-1] means the territory has no numbers of this type.
	lengths   []int8
	localOnly []int8
}

func (d *numberDesc) hasPossibleLengthData() bool {
	return len(d.lengths) != 1 || d.lengths[0] != -1
}

func (d *numberDesc) hasData() bool {
	return d.example != "" || d.hasPossibleLengthData() || d.pattern != nil
}

// matches reports whether nsn has a possible length for d and matches its
// pattern.
func (d *numberDesc) matches(nsn string) bool {
	if len(d.lengths) > 0 && !hasLength(d.lengths, len(nsn)) {
		return false
	}
	return d.matchesPattern(nsn)
}

// hasLength reports whether lengths contains n.
func hasLength(lengths []int8, n int) bool {
	for _, l := range lengths {
		if int(l) == n {
			return true
		}
	}
	return false
}

func (d *numberDesc) matchesPattern(nsn string) bool {
	return d.pattern != nil && d.pattern.matchFull(nsn)
}

// numberFormat is one formatting rule. pattern selects the numbers it
// applies to and format rearranges the pattern's groups using $1-style
// references.
type numberFormat struct {
	pattern                              *pattern
	format                               string
	leadingDigits                        []*pattern
	nationalPrefixFormattingRule         string
	nationalPrefixOptionalWhenFormatting bool
	carrierCodeFormattingRule            string
}

// pattern is a metadata regular expression, compiled on first use. Upstream
// matches each expression three ways: against the whole input, against a
// prefix of it, or anywhere in it; each needs its own compiled form in Go.
type pattern struct {
	expr string

	fullOnce, prefixOnce, anyOnce sync.Once
	full, prefix, any             *regexp.Regexp
}

// matchFull reports whether the expression matches all of s.
func (p *pattern) matchFull(s string) bool {
	p.fullOnce.Do(func() { p.full = regexp.MustCompile(`^(?:` + p.expr + `)$`) })
	return p.full.MatchString(s)
}

// prefixRegexp returns the expression anchored at the start of the input.
func (p *pattern) prefixRegexp() *regexp.Regexp {
	p.prefixOnce.Do(func() { p.prefix = regexp.MustCompile(`^(?:` + p.expr + `)`) })
	return p.prefix
}

// matchPrefix reports whether the expression matches a prefix of s.
func (p *pattern) matchPrefix(s string) bool {
	return p.prefixRegexp().MatchString(s)
}

// anyRegexp returns the expression unanchored.
func (p *pattern) anyRegexp() *regexp.Regexp {
	p.anyOnce.Do(func() { p.any = regexp.MustCompile(p.expr) })
	return p.any
}

// metadataSet indexes a list of territories. The package uses one built
// from the generated territoryData; tests build one from upstream's test
// metadata.
type metadataSet struct {
	regions        map[string]*territory
	nonGeo         map[int]*territory
	regionsForCode map[int][]string
}

func newMetadataSet(ts []*territory) *metadataSet {
	s := &metadataSet{
		regions:        make(map[string]*territory, len(ts)),
		nonGeo:         make(map[int]*territory),
		regionsForCode: make(map[int][]string),
	}
	for _, t := range ts {
		if t.id == regionNonGeo {
			s.nonGeo[t.countryCode] = t
		} else {
			s.regions[t.id] = t
		}
		// The main region for a shared code comes first, then the others
		// in metadata order.
		if t.mainCountryForCode {
			s.regionsForCode[t.countryCode] = slices.Insert(s.regionsForCode[t.countryCode], 0, t.id)
		} else {
			s.regionsForCode[t.countryCode] = append(s.regionsForCode[t.countryCode], t.id)
		}
	}
	return s
}

var defaultSet = sync.OnceValue(func() *metadataSet { return newMetadataSet(territoryData) })

// region returns the metadata for a region code, or nil.
func (s *metadataSet) region(code string) *territory {
	return s.regions[code]
}

// forCode returns the metadata for a country calling code whose main region
// is region: the non-geographical entity for "001", else the region's.
func (s *metadataSet) forCode(cc int, region string) *territory {
	if region == regionNonGeo {
		return s.nonGeo[cc]
	}
	return s.regions[region]
}

func (s *metadataSet) knownCode(cc int) bool {
	_, ok := s.regionsForCode[cc]
	return ok
}

// mainRegion returns the main region for a country calling code, "001" for
// a non-geographical code, or "ZZ" for an unknown one.
func (s *metadataSet) mainRegion(cc int) string {
	if r := s.regionsForCode[cc]; len(r) > 0 {
		return r[0]
	}
	return regionUnknown
}

func (s *metadataSet) isNANPA(region string) bool {
	return slices.Contains(s.regionsForCode[nanpaCountryCode], region)
}

// descFor returns the description of numbers of type typ.
func (t *territory) descFor(typ Type) *numberDesc {
	switch typ {
	case PremiumRate:
		return &t.premiumRate
	case TollFree:
		return &t.tollFree
	case Mobile:
		return &t.mobile
	case FixedLine, FixedLineOrMobile:
		return &t.fixedLine
	case SharedCost:
		return &t.sharedCost
	case VoIP:
		return &t.voip
	case PersonalNumber:
		return &t.personalNumber
	case Pager:
		return &t.pager
	case UAN:
		return &t.uan
	case Voicemail:
		return &t.voicemail
	default:
		return &t.generalDesc
	}
}
