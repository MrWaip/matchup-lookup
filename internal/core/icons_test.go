package core

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestIconURLs(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "matches.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	responses := map[string]string{
		"/api/versions.json":                         `["16.19.1"]`,
		"/cdn/16.19.1/data/en_US/champion.json":      `{"data":{"Fiddlesticks":{"id":"Fiddlesticks","name":"Fiddlesticks"},"MonkeyKing":{"id":"MonkeyKing","name":"Wukong"}}}`,
		"/cdn/16.19.1/data/en_US/summoner.json":      `{"data":{"SummonerFlash":{"key":"4","image":{"full":"SummonerFlash.png"}}}}`,
		"/cdn/16.19.1/data/en_US/runesReforged.json": `[{"id":8000,"icon":"perk-images/Styles/7201_Precision.png","slots":[{"runes":[{"id":8010,"icon":"perk-images/Styles/Precision/Conqueror/Conqueror.png"}]}]}]`,
	}
	old := catalogHTTPClient
	catalogHTTPClient = &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		body, ok := responses[r.URL.Path]
		status := 200
		if !ok {
			status = 404
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	t.Cleanup(func() { catalogHTTPClient = old })

	icons, err := LoadIcons(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ got, want string }{
		{icons.Champion("FiddleSticks"), "https://ddragon.leagueoflegends.com/cdn/16.19.1/img/champion/Fiddlesticks.png"},
		{icons.Champion("Wukong"), "https://ddragon.leagueoflegends.com/cdn/16.19.1/img/champion/MonkeyKing.png"},
		{icons.Spell(4), "https://ddragon.leagueoflegends.com/cdn/16.19.1/img/spell/SummonerFlash.png"},
		{icons.Rune(8010), "https://ddragon.leagueoflegends.com/cdn/img/perk-images/Styles/Precision/Conqueror/Conqueror.png"},
		{icons.Rune(8000), "https://ddragon.leagueoflegends.com/cdn/img/perk-images/Styles/7201_Precision.png"},
		{icons.Champion("Unknown"), ""},
		{icons.Spell(0), ""},
	} {
		if tc.got != tc.want {
			t.Errorf("got %q, want %q", tc.got, tc.want)
		}
	}
	var none *Icons
	if none.Champion("Fiora") != "" || none.Spell(4) != "" || none.Rune(8010) != "" {
		t.Error("nil Icons must yield no URLs")
	}
}
