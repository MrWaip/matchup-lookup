package core

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
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("gameName,tagLine,region,champion\nFiora Player,EUW,euw1,Fiora\n")), Header: make(http.Header)}, nil
	})}
	t.Cleanup(func() { seedHTTPClient = old })
	seeds, err := LoadSeeds("https://example.com/players.csv")
	if err != nil || len(seeds) != 1 || seeds[0].GameName != "Fiora Player" || seeds[0].Champion != "Fiora" {
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

func TestBundledPlayerPool(t *testing.T) {
	fromManifest, err := LoadSeeds("../../players/index.json")
	if err != nil {
		t.Fatal(err)
	}
	fromDirectory, err := LoadSeeds("../../players")
	if err != nil {
		t.Fatal(err)
	}
	if len(fromManifest) != 200 || len(fromDirectory) != 200 {
		t.Fatalf("pool sizes: manifest=%d directory=%d", len(fromManifest), len(fromDirectory))
	}
	seen := map[string]bool{}
	for _, seed := range fromDirectory {
		if seed.Region != "euw1" || seed.Champion != "Fiora" {
			t.Fatalf("unexpected seed: %+v", seed)
		}
		key := strings.ToLower(seed.GameName + "#" + seed.TagLine)
		if seen[key] {
			t.Fatalf("duplicate Riot ID: %s", key)
		}
		seen[key] = true
	}
}
