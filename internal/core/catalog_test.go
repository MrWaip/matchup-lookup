package core

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestChampionCatalogCachedAndFuzzy(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "matches.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	old := catalogHTTPClient
	calls := 0
	catalogHTTPClient = &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		body := `["16.19.1"]`
		if strings.HasSuffix(r.URL.Path, "champion.json") {
			body = `{"data":{"Fiora":{"id":"Fiora","name":"Fiora"},"MonkeyKing":{"id":"MonkeyKing","name":"Wukong"}}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	t.Cleanup(func() { catalogHTTPClient = old })
	for i := 0; i < 2; i++ {
		champions, err := EnsureChampions(context.Background(), s)
		if err != nil || len(champions) != 2 {
			t.Fatalf("catalog: %+v %v", champions, err)
		}
	}
	if calls != 2 {
		t.Fatalf("Data Dragon fetched %d times, want 2 HTTP requests for one refresh", calls)
	}
	if FuzzyScore("fioa", "Fiora") < 0 || FuzzyScore("wukg", "Wukong") < 0 || FuzzyScore("xyz", "Fiora") >= 0 {
		t.Fatal("fuzzy matching failed")
	}
}
