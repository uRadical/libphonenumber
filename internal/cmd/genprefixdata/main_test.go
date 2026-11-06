package main

import (
	"bytes"
	"compress/gzip"
	"errors"
	"flag"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

// The module root, relative to this package's directory.
const root = "../../.."

// readTable decompresses a generated table.
func readTable(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	b, err := io.ReadAll(z)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return string(b)
}

// TestGeneratedDataUpToDate regenerates every table and compares it, after
// decompressing, with the committed one, so that a change to resources/
// without a regenerate fails the tests. Comparing the decompressed text
// keeps the test independent of the compressor's output.
func TestGeneratedDataUpToDate(t *testing.T) {
	for _, tc := range []struct{ in, out string }{
		{"resources/geocoding", "geocoder/data"},
		{"resources/test/geocoding", "geocoder/testdata/geocoding"},
		{"resources/carrier", "carrier/data"},
		{"resources/test/carrier", "carrier/testdata/carrier"},
	} {
		t.Run(tc.out, func(t *testing.T) {
			tables, err := generate(os.DirFS(filepath.Join(root, tc.in)))
			if err != nil {
				t.Fatal(err)
			}
			committed, err := filepath.Glob(filepath.Join(root, tc.out, "*.gz"))
			if err != nil {
				t.Fatal(err)
			}
			var names []string
			for _, c := range committed {
				names = append(names, filepath.Base(c))
			}
			if want := slices.Sorted(maps.Keys(tables)); !slices.Equal(names, want) {
				t.Fatalf("%s holds %d tables, want %d: run go generate ./... in the module root", tc.out, len(names), len(want))
			}
			for name, m := range tables {
				if got, want := readTable(t, filepath.Join(root, tc.out, name)), text(m); got != want {
					t.Errorf("%s/%s is out of date: run go generate ./... in the module root", tc.out, name)
				}
			}
		})
	}
	for _, tc := range []struct {
		in, out string
		read    func([]byte) (mapping, error)
	}{
		{"resources/cldr/territories.tsv", "geocoder/names.gz", parseNames},
	} {
		t.Run(tc.out, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(root, tc.in))
			if err != nil {
				t.Fatal(err)
			}
			m, err := tc.read(raw)
			if err != nil {
				t.Fatal(err)
			}
			if readTable(t, filepath.Join(root, tc.out)) != text(m) {
				t.Errorf("%s is out of date: run go generate ./... in the module root", tc.out)
			}
		})
	}
}

// text is what encode compresses.
func text(m mapping) string {
	var b strings.Builder
	for _, p := range sortedPrefixes(m) {
		b.WriteString(p + "|" + m[p] + "\n")
	}
	return b.String()
}

func TestGenerate(t *testing.T) {
	fsys := fstest.MapFS{
		"en/44.txt": {Data: []byte("# comment\n\n4420|London\n44113|Leeds\n441|\n")},
		// German: London matches English and has no shorter prefix of its
		// own, so it goes; Leeds matches English but 441 is shorter, so it
		// is blanked; Bath differs and stays.
		"de/44.txt":  {Data: []byte("4420|London\n441|Vereinigtes\n44113|Leeds\n441225|Bath (de)\n")},
		"de/49.txt":  {Data: []byte("4930|Berlin\n")}, // No English data.
		"en/1.txt":   {Data: []byte("1201|NJ\n1201555|Jersey City\n1212|NY\n")},
		"en/.hidden": {Data: []byte("x")},
		".git/x":     {Data: []byte("x")},
	}
	tables, err := generate(fsys)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"44_en.gz":   "44113|Leeds\n4420|London\n",
		"44_de.gz":   "441|Vereinigtes\n44113|\n441225|Bath (de)\n",
		"49_de.gz":   "4930|Berlin\n",
		"1201_en.gz": "1201|NJ\n1201555|Jersey City\n",
		"1212_en.gz": "1212|NY\n",
	}
	if len(tables) != len(want) {
		t.Errorf("generate made %v", slices.Sorted(maps.Keys(tables)))
	}
	for name, w := range want {
		if got := text(tables[name]); got != w {
			t.Errorf("%s = %q, want %q", name, got, w)
		}
	}
}

func TestGenerateErrors(t *testing.T) {
	for name, fsys := range map[string]fstest.MapFS{
		"bad file name": {"en/x.txt": {Data: []byte("")}},
		"no pipe":       {"en/44.txt": {Data: []byte("4420 London\n")}},
		"bad prefix":    {"en/44.txt": {Data: []byte("44x|London\n")}},
		"zero prefix":   {"en/44.txt": {Data: []byte("044|London\n")}},
		"duplicate":     {"en/44.txt": {Data: []byte("4420|London\n4420|Londres\n")}},
		"bad english":   {"en/44.txt": {Data: []byte("bad\n")}, "de/44.txt": {Data: []byte("4420|London\n")}},
	} {
		if _, err := generate(fsys); err == nil {
			t.Errorf("%s: generate succeeded", name)
		}
	}
	if _, err := generate(fstest.MapFS{"en": {Mode: os.ModeDir}}); err != nil {
		t.Errorf("an empty language directory: %v", err)
	}
}

func TestParseNames(t *testing.T) {
	m, err := parseNames([]byte("# header\nen\tDE\tGermany\nde\tDE\tDeutschland\n"))
	if err != nil {
		t.Fatal(err)
	}
	if m["en/DE"] != "Germany" || m["de/DE"] != "Deutschland" {
		t.Errorf("parseNames = %v", m)
	}
	for _, bad := range []string{"en\tDE\n", "en\tDEU\tGermany\n", "\tDE\tGermany\n", "en\tDE\t\n", "en\tDE\tA|B\n"} {
		if _, err := parseNames([]byte(bad)); err == nil {
			t.Errorf("parseNames(%q) succeeded", bad)
		}
	}
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in")
	if err := os.MkdirAll(filepath.Join(in, "en"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, s string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(in, "en", "44.txt"), "4420|London\n")
	out := filepath.Join(dir, "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(out, "stale_en.gz"), "old")
	if err := run([]string{"-in", in, "-out", out}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if got := readTable(t, filepath.Join(out, "44_en.gz")); got != "4420|London\n" {
		t.Errorf("44_en.gz = %q", got)
	}
	if _, err := os.Stat(filepath.Join(out, "stale_en.gz")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("stale table not removed: %v", err)
	}

	single := filepath.Join(dir, "tz.txt")
	write(single, "1|America/New_York\n")
	if err := run([]string{"-single", "-in", single, "-out", filepath.Join(dir, "tz.gz")}, io.Discard); err != nil {
		t.Fatal(err)
	}
	names := filepath.Join(dir, "names.tsv")
	write(names, "en\tDE\tGermany\n")
	if err := run([]string{"-names", "-in", names, "-out", filepath.Join(dir, "names.gz")}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if got := readTable(t, filepath.Join(dir, "names.gz")); got != "en/DE|Germany\n" {
		t.Errorf("names.gz = %q", got)
	}
}

func TestRunErrors(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.txt")
	if err := os.WriteFile(bad, []byte("no pipe\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		args []string
		help bool
	}{
		{"no flags", nil, true},
		{"unknown flag", []string{"-nope"}, false},
		{"missing input", []string{"-single", "-in", filepath.Join(dir, "missing"), "-out", "x"}, false},
		{"bad input", []string{"-single", "-in", bad, "-out", filepath.Join(dir, "o.gz")}, false},
		{"missing directory", []string{"-in", filepath.Join(dir, "missing"), "-out", dir}, false},
		{"unwritable output", []string{"-single", "-in", bad, "-out", filepath.Join(dir, "no", "o.gz")}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := run(tc.args, io.Discard)
			if err == nil {
				t.Fatal("run succeeded")
			}
			if got := errors.Is(err, flag.ErrHelp); got != tc.help {
				t.Errorf("errors.Is(%v, flag.ErrHelp) = %v, want %v", err, got, tc.help)
			}
		})
	}
}
