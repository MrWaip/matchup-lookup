package main

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestLoadSeedsFromURLAndFile(t *testing.T) {
	old := seedHTTPClient
	seedHTTPClient = &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("gameName,tagLine,region\nFiora Player,EUW,euw1\n")), Header: make(http.Header)}, nil
	})}
	t.Cleanup(func() { seedHTTPClient = old })
	seeds, err := LoadSeeds("https://example.com/players.csv")
	if err != nil || len(seeds) != 1 || seeds[0].GameName != "Fiora Player" {
		t.Fatalf("CSV URL: %+v %v", seeds, err)
	}
	path := filepath.Join(t.TempDir(), "players.json")
	if err := os.WriteFile(path, []byte(`[{"gameName":"Other","tagLine":"EUW","region":"euw1"}]`), 0600); err != nil {
		t.Fatal(err)
	}
	seeds, err = LoadSeeds(path)
	if err != nil || len(seeds) != 1 || seeds[0].GameName != "Other" {
		t.Fatalf("JSON file: %+v %v", seeds, err)
	}
}
