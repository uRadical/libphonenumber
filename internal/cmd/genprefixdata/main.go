// Command genprefixdata turns upstream's prefix data into the gzipped tables
// that internal/prefixmap reads.
//
// For geocoding and carrier data, -in is a directory of language
// directories, each holding one "<country calling code>.txt" file per
// country, and -out receives one "<prefix>_<language>.gz" file per table.
// It follows upstream's GeneratePhonePrefixData: empty English descriptions
// are dropped, other languages drop what English already says (or blank it
// where a shorter prefix would otherwise show through), and NANPA data is
// split by area code.
//
// With -single, -in is one file, such as the time zone map, and -out the
// gzipped table to write.
//
// With -names, -in is the tab-separated CLDR region names that fetchcldr
// writes, and -out a gzipped table keyed "<locale>/<region>".
//
// Usage (normally through go generate in the module root):
//
//	genprefixdata -in resources/geocoding -out geocoder/data
//	genprefixdata -single -in resources/timezones/map_data.txt -out timezone/data/map_data.gz
//	genprefixdata -names -in resources/cldr/territories.tsv -out geocoder/names.gz
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("genprefixdata: ")
	if err := run(os.Args[1:], os.Stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(2)
		}
		log.Fatal(err)
	}
}

func run(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("genprefixdata", flag.ContinueOnError)
	fs.SetOutput(stderr)
	in := fs.String("in", "", "input directory, or file with -single")
	out := fs.String("out", "", "output directory, or file with -single")
	single := fs.Bool("single", false, "convert one file")
	names := fs.Bool("names", false, "convert CLDR region names")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *in == "" || *out == "" {
		fs.Usage()
		return flag.ErrHelp
	}
	if *single || *names {
		raw, err := os.ReadFile(*in)
		if err != nil {
			return err
		}
		read := parse
		if *names {
			read = parseNames
		}
		m, err := read(raw)
		if err != nil {
			return fmt.Errorf("%s: %w", *in, err)
		}
		return writeTable(*out, m)
	}
	tables, err := generate(os.DirFS(*in))
	if err != nil {
		return err
	}
	return writeDir(*out, tables)
}

// mapping is one data file: prefix to description.
type mapping map[string]string

// parse reads "prefix|description" lines, skipping blank lines and
// comments, as upstream does.
func parse(raw []byte) (mapping, error) {
	m := mapping{}
	s := bufio.NewScanner(bytes.NewReader(raw))
	s.Buffer(nil, 1<<20)
	for n := 1; s.Scan(); n++ {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		prefix, desc, ok := strings.Cut(line, "|")
		if !ok {
			return nil, fmt.Errorf("line %d: no '|'", n)
		}
		if _, err := strconv.ParseUint(prefix, 10, 31); err != nil || prefix[0] == '0' {
			return nil, fmt.Errorf("line %d: bad prefix %q", n, prefix)
		}
		if _, dup := m[prefix]; dup {
			return nil, fmt.Errorf("line %d: duplicate prefix %s", n, prefix)
		}
		m[prefix] = desc
	}
	return m, s.Err()
}

// parseNames reads tab-separated locale, region and name lines, skipping
// comments, into a table keyed "<locale>/<region>".
func parseNames(raw []byte) (mapping, error) {
	m := mapping{}
	for n, line := range strings.Split(string(raw), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 3 || f[0] == "" || len(f[1]) != 2 || f[2] == "" || strings.Contains(f[2], "|") {
			return nil, fmt.Errorf("line %d: want locale, region and name", n+1)
		}
		m[f[0]+"/"+f[1]] = f[2]
	}
	return m, nil
}

// generate reads every language directory of fsys and returns the tables
// to write, keyed by file name.
func generate(fsys fs.FS) (map[string]mapping, error) {
	langs, err := entries(fsys, ".", true)
	if err != nil {
		return nil, err
	}
	english := map[string]mapping{} // raw English data by file name
	tables := map[string]mapping{}
	for _, lang := range langs {
		files, err := entries(fsys, lang, false)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			cc, ok := strings.CutSuffix(file, ".txt")
			if _, err := strconv.Atoi(cc); !ok || err != nil {
				return nil, fmt.Errorf("%s/%s: want <country calling code>.txt", lang, file)
			}
			m, err := parseFile(fsys, lang+"/"+file)
			if err != nil {
				return nil, fmt.Errorf("%s/%s: %w", lang, file, err)
			}
			if lang == "en" {
				removeEmpty(m)
			} else if en, err := englishFor(fsys, english, file); err != nil {
				return nil, err
			} else if en != nil {
				compressAgainstEnglish(en, m)
			}
			for key, part := range split(cc, m) {
				tables[key+"_"+lang+".gz"] = part
			}
		}
	}
	return tables, nil
}

// englishFor returns the raw English data for a country, or nil if there
// is none, reading it once.
func englishFor(fsys fs.FS, cache map[string]mapping, file string) (mapping, error) {
	if m, ok := cache[file]; ok {
		return m, nil
	}
	m, err := parseFile(fsys, "en/"+file)
	if errors.Is(err, fs.ErrNotExist) {
		m, err = nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("en/%s: %w", file, err)
	}
	cache[file] = m
	return m, nil
}

// removeEmpty drops English mappings with no description.
func removeEmpty(m mapping) {
	for p, d := range m {
		if d == "" {
			delete(m, p)
		}
	}
}

// compressAgainstEnglish drops each mapping that English has with the same
// description, so lookups fall back to English. Where a shorter prefix of
// the language's own would then match instead, the description is blanked
// rather than dropped.
func compressAgainstEnglish(english, m mapping) {
	for _, p := range sortedPrefixes(m) {
		if d, ok := english[p]; !ok || d != m[p] {
			continue
		}
		if hasShorterPrefix(p, m) {
			m[p] = ""
		} else {
			delete(m, p)
		}
	}
}

func hasShorterPrefix(p string, m mapping) bool {
	for l := len(p) - 1; l > 0; l-- {
		if _, ok := m[p[:l]]; ok {
			return true
		}
	}
	return false
}

// split divides a country's data into tables: one per area code (the
// first four digits) for the NANPA, else one for the country.
func split(cc string, m mapping) map[string]mapping {
	out := map[string]mapping{}
	for p, d := range m {
		key := cc
		if cc == "1" {
			if len(p) < 4 {
				continue
			}
			key = p[:4]
		}
		if out[key] == nil {
			out[key] = mapping{}
		}
		out[key][p] = d
	}
	return out
}

func sortedPrefixes(m mapping) []string {
	ps := make([]string, 0, len(m))
	for p := range m {
		ps = append(ps, p)
	}
	slices.Sort(ps)
	return ps
}

// encode writes m as sorted "prefix|description" lines, gzipped with no
// name or time in the header so the output depends on the data alone.
func encode(m mapping) ([]byte, error) {
	var b bytes.Buffer
	z, err := gzip.NewWriterLevel(&b, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	for _, p := range sortedPrefixes(m) {
		if _, err := fmt.Fprintf(z, "%s|%s\n", p, m[p]); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func writeTable(path string, m mapping) error {
	data, err := encode(m)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// writeDir replaces the tables in dir with tables.
func writeDir(dir string, tables map[string]mapping) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	old, err := filepath.Glob(filepath.Join(dir, "*.gz"))
	if err != nil {
		return err
	}
	for _, f := range old {
		if err := os.Remove(f); err != nil {
			return err
		}
	}
	for name, m := range tables {
		if err := writeTable(filepath.Join(dir, name), m); err != nil {
			return err
		}
	}
	return nil
}

// entries returns the sorted names of the directories, or else the
// files, in dir, leaving out hidden ones.
func entries(fsys fs.FS, dir string, dirs bool) ([]string, error) {
	es, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range es {
		if e.IsDir() == dirs && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

func parseFile(fsys fs.FS, path string) (mapping, error) {
	raw, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, err
	}
	return parse(raw)
}
