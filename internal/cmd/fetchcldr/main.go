// Command fetchcldr downloads the names of countries and regions in each
// language with modern coverage in Unicode CLDR, which the geocoder shows
// when it has no more detailed description, as upstream's does through
// Java's Locale.getDisplayCountry. It writes them as tab-separated lines of
// locale, region code and name. It needs network access, so it is run by
// hand when updating, not by go generate:
//
//	go run ./internal/cmd/fetchcldr -tag 48.2.3 -out resources/cldr/territories.tsv
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"
)

const base = "https://raw.githubusercontent.com/unicode-org/cldr-json/"

func main() {
	log.SetFlags(0)
	log.SetPrefix("fetchcldr: ")
	if err := run(os.Args[1:], os.Stderr, http.DefaultClient); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(2)
		}
		log.Fatal(err)
	}
}

// getter fetches a URL; *http.Client is one.
type getter interface {
	Get(url string) (*http.Response, error)
}

func run(args []string, stderr io.Writer, client getter) error {
	fs := flag.NewFlagSet("fetchcldr", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tag := fs.String("tag", "", "cldr-json release tag, such as 48.2.3")
	out := fs.String("out", "", "file to write")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *tag == "" || *out == "" {
		fs.Usage()
		return flag.ErrHelp
	}
	root := base + *tag + "/cldr-json/"
	var cov struct {
		CoverageLevels map[string]string `json:"coverageLevels"`
	}
	if err := getJSON(client, root+"cldr-core/coverageLevels.json", &cov); err != nil {
		return err
	}
	var locales []string
	for loc, level := range cov.CoverageLevels {
		if level == "modern" {
			locales = append(locales, loc)
		}
	}
	slices.Sort(locales)
	var b strings.Builder
	fmt.Fprintf(&b, "# Unicode CLDR %s territory names, for locales with modern coverage.\n", *tag)
	fmt.Fprintf(&b, "# Fetched %s by internal/cmd/fetchcldr. Unicode License v3: see NOTICE.\n", time.Now().UTC().Format("2006-01-02"))
	for _, loc := range locales {
		names, err := territories(client, root, loc)
		if err != nil {
			return err
		}
		writeNames(&b, loc, names)
	}
	return os.WriteFile(*out, []byte(b.String()), 0o644)
}

// territories returns a locale's names for regions, keyed by region code.
func territories(client getter, root, loc string) (map[string]string, error) {
	var doc struct {
		Main map[string]struct {
			LocaleDisplayNames struct {
				Territories map[string]string `json:"territories"`
			} `json:"localeDisplayNames"`
		} `json:"main"`
	}
	if err := getJSON(client, root+"cldr-localenames-full/main/"+loc+"/territories.json", &doc); err != nil {
		return nil, err
	}
	m, ok := doc.Main[loc]
	if !ok {
		return nil, fmt.Errorf("%s: no data for the locale", loc)
	}
	return m.LocaleDisplayNames.Territories, nil
}

// writeNames writes the names of two-letter regions, leaving out
// alternative forms such as "HK-alt-short" and macro-regions such as 001.
func writeNames(b *strings.Builder, loc string, names map[string]string) {
	codes := make([]string, 0, len(names))
	for code := range names {
		if len(code) == 2 && code[0] >= 'A' && code[0] <= 'Z' {
			codes = append(codes, code)
		}
	}
	slices.Sort(codes)
	for _, code := range codes {
		fmt.Fprintf(b, "%s\t%s\t%s\n", loc, code, names[code])
	}
}

func getJSON(client getter, url string, v any) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("%s: %w", url, err)
	}
	return nil
}
