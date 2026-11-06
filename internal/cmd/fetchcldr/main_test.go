package main

import (
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeCLDR serves canned responses by URL.
type fakeCLDR map[string]string

func (f fakeCLDR) Get(url string) (*http.Response, error) {
	body, ok := f[url]
	if !ok {
		return &http.Response{StatusCode: http.StatusNotFound, Status: "404 Not Found", Body: io.NopCloser(strings.NewReader(""))}, nil
	}
	return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body))}, nil
}

const root = base + "48.2.3/cldr-json/"

func fake() fakeCLDR {
	return fakeCLDR{
		root + "cldr-core/coverageLevels.json": `{"coverageLevels": {"en": "modern", "de": "modern", "xx": "basic"}}`,
		root + "cldr-localenames-full/main/en/territories.json": `{"main": {"en": {"localeDisplayNames": {"territories":
			{"DE": "Germany", "001": "World", "HK-alt-short": "Hong Kong", "HK": "Hong Kong SAR China"}}}}}`,
		root + "cldr-localenames-full/main/de/territories.json": `{"main": {"de": {"localeDisplayNames": {"territories":
			{"DE": "Deutschland"}}}}}`,
	}
}

func TestRun(t *testing.T) {
	out := filepath.Join(t.TempDir(), "territories.tsv")
	if err := run([]string{"-tag", "48.2.3", "-out", out}, io.Discard, fake()); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{"# Unicode CLDR 48.2.3", "de\tDE\tDeutschland\nen\tDE\tGermany\nen\tHK\tHong Kong SAR China\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "World") || strings.Contains(got, "alt") || strings.Contains(got, "xx\t") {
		t.Errorf("output has entries it should leave out:\n%s", got)
	}
}

func TestRunErrors(t *testing.T) {
	missingLocale := fake()
	delete(missingLocale, root+"cldr-localenames-full/main/de/territories.json")
	wrongLocale := fake()
	wrongLocale[root+"cldr-localenames-full/main/de/territories.json"] = `{"main": {"en": {}}}`
	badJSON := fake()
	badJSON[root+"cldr-core/coverageLevels.json"] = `{`
	for _, tc := range []struct {
		name   string
		args   []string
		client getter
		help   bool
	}{
		{"no flags", nil, fake(), true},
		{"unknown flag", []string{"-nope"}, fake(), false},
		{"missing coverage", []string{"-tag", "48.2.3", "-out", "x"}, fakeCLDR{}, false},
		{"missing locale", []string{"-tag", "48.2.3", "-out", "x"}, missingLocale, false},
		{"wrong locale", []string{"-tag", "48.2.3", "-out", "x"}, wrongLocale, false},
		{"bad JSON", []string{"-tag", "48.2.3", "-out", "x"}, badJSON, false},
		{"unwritable output", []string{"-tag", "48.2.3", "-out", filepath.Join(t.TempDir(), "no", "x")}, fake(), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := run(tc.args, io.Discard, tc.client)
			if err == nil {
				t.Fatal("run succeeded")
			}
			if got := errors.Is(err, flag.ErrHelp); got != tc.help {
				t.Errorf("errors.Is(%v, flag.ErrHelp) = %v, want %v", err, got, tc.help)
			}
		})
	}
}

// errorGetter fails every request.
type errorGetter struct{}

func (errorGetter) Get(string) (*http.Response, error) { return nil, errors.New("network down") }

func TestNetworkError(t *testing.T) {
	if err := run([]string{"-tag", "48.2.3", "-out", "x"}, io.Discard, errorGetter{}); err == nil {
		t.Error("run succeeded with no network")
	}
}
