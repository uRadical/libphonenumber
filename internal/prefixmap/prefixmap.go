// Package prefixmap looks up descriptions of phone numbers by prefix: the
// place, carrier or time zones that upstream's prefix data files give for
// the longest matching start of a number. It reads the gzipped files that
// internal/cmd/genprefixdata writes, and is the Go port of upstream's
// prefixmapper package.
package prefixmap

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// Table maps number prefixes, country calling code first, to descriptions.
type Table map[string]string

// Lookup returns the description for the longest prefix of digits in t,
// and whether there was one. digits is a country calling code followed by
// a national significant number.
func (t Table) Lookup(digits string) (string, bool) {
	for l := len(digits); l > 0; l-- {
		if d, ok := t[digits[:l]]; ok {
			return d, true
		}
	}
	return "", false
}

// Read reads a table of "prefix|description" lines.
func Read(r io.Reader) (Table, error) {
	t := Table{}
	s := bufio.NewScanner(r)
	s.Buffer(nil, 1<<20)
	for s.Scan() {
		prefix, desc, ok := strings.Cut(s.Text(), "|")
		if !ok {
			return nil, fmt.Errorf("prefixmap: line %q has no '|'", s.Text())
		}
		t[prefix] = desc
	}
	return t, s.Err()
}

// ReadFile reads a gzipped table from fsys.
func ReadFile(fsys fs.FS, name string) (Table, error) {
	raw, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, err
	}
	z, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("prefixmap: %s: %w", name, err)
	}
	t, err := Read(z)
	if err != nil {
		return nil, fmt.Errorf("prefixmap: %s: %w", name, err)
	}
	return t, nil
}

// Files is a set of tables, one per prefix and language, named
// "<prefix>_<language>.gz". The prefix is a country calling code, or for
// the NANPA, 1 followed by an area code. Tables load on first use.
type Files struct {
	fsys fs.FS

	index func() map[int][]string // prefix to languages, from the file names

	mu     sync.Mutex
	tables map[string]Table
}

// NewFiles returns the tables in fsys.
func NewFiles(fsys fs.FS) *Files {
	f := &Files{fsys: fsys, tables: map[string]Table{}}
	f.index = sync.OnceValue(f.readIndex)
	return f
}

func (f *Files) readIndex() map[int][]string {
	idx := map[int][]string{}
	entries, err := fs.ReadDir(f.fsys, ".")
	if err != nil {
		return idx
	}
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".gz")
		if !ok {
			continue
		}
		prefix, lang, ok := strings.Cut(name, "_")
		p, err := strconv.Atoi(prefix)
		if !ok || err != nil {
			continue
		}
		idx[p] = append(idx[p], lang)
	}
	return idx
}

// Description returns the description of a number in a language, or "".
// cc is its country calling code, nsn its national significant number,
// and lang, script and region describe the language, as upstream's
// PrefixFileReader takes them. Languages other than Chinese, Japanese and
// Korean fall back to English when they have no description.
func (f *Files) Description(cc int, nsn, lang, script, region string) string {
	key := cc
	if cc == 1 {
		// NANPA tables are split by area code: 1000 plus the national
		// number divided by 10^7, as upstream computes it.
		n, _ := strconv.ParseUint(strings.TrimLeft(nsn, "0"), 10, 64)
		key = 1000 + int(n/10_000_000)
	}
	digits := strconv.Itoa(cc) + nsn
	desc, _ := f.lookup(key, lang, script, region, digits)
	if desc == "" && mayFallBackToEnglish(lang) {
		desc, _ = f.lookup(key, "en", "", "", digits)
	}
	return desc
}

func mayFallBackToEnglish(lang string) bool {
	return lang != "zh" && lang != "ja" && lang != "ko"
}

func (f *Files) lookup(key int, lang, script, region, digits string) (string, bool) {
	name := f.fileName(key, lang, script, region)
	if name == "" {
		return "", false
	}
	t := f.table(name)
	return t.Lookup(digits)
}

// table loads and caches a table. A file that cannot be read gives an
// empty table, as upstream logs and carries on.
func (f *Files) table(name string) Table {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tables[name]
	if !ok {
		t, _ = ReadFile(f.fsys, name)
		f.tables[name] = t
	}
	return t
}

// localeNormalization maps locales to the data that serves them.
var localeNormalization = map[string]string{
	"zh_TW": "zh_Hant",
	"zh_HK": "zh_Hant",
	"zh_MO": "zh_Hant",
}

// fileName returns the file for a prefix in the language that best
// matches lang, script and region, or "" if there is none.
func (f *Files) fileName(key int, lang, script, region string) string {
	if lang == "" {
		return ""
	}
	langs := f.index()[key]
	if code := bestMatchingLanguage(langs, lang, script, region); code != "" {
		return strconv.Itoa(key) + "_" + code + ".gz"
	}
	return ""
}

// bestMatchingLanguage is upstream's MappingFileProvider rule for choosing
// among the available languages.
func bestMatchingLanguage(langs []string, lang, script, region string) string {
	full := lang
	for _, part := range []string{script, region} {
		if part != "" {
			full += "_" + part
		}
	}
	if n, ok := localeNormalization[full]; ok && slices.Contains(langs, n) {
		return n
	}
	if slices.Contains(langs, full) {
		return full
	}
	switch {
	case (script == "") != (region == ""):
		// Only one of script and region.
		if slices.Contains(langs, lang) {
			return lang
		}
	case script != "" && region != "":
		for _, c := range []string{lang + "_" + script, lang + "_" + region, lang} {
			if slices.Contains(langs, c) {
				return c
			}
		}
	}
	return ""
}

// ParseLanguage splits a language tag such as "en", "zh-TW", "zh_Hant_TW"
// or "sr-Latn" into language, script and region, normalizing their case.
// Hebrew is given as "iw" and Indonesian as "id" whichever code the tag
// uses, as upstream's data names them.
func ParseLanguage(tag string) (lang, script, region string) {
	parts := strings.FieldsFunc(tag, func(r rune) bool { return r == '-' || r == '_' })
	if len(parts) == 0 {
		return "", "", ""
	}
	lang = strings.ToLower(parts[0])
	for _, p := range parts[1:] {
		switch {
		case len(p) == 4 && script == "" && region == "":
			script = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
		case (len(p) == 2 || len(p) == 3) && region == "":
			region = strings.ToUpper(p)
		}
	}
	switch lang {
	case "he":
		lang = "iw"
	case "in":
		lang = "id"
	}
	return lang, script, region
}
