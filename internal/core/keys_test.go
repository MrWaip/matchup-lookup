package core

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestRiotKeysKeepOrderAndRejections(t *testing.T) {
	t.Setenv("RIOT_API_KEY", "")
	path := filepath.Join(t.TempDir(), "matches.db")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UsableRiotKeys(); err == nil || errors.Is(err, ErrKeyRejected) {
		t.Fatalf("no keys yet: %v", err)
	}
	for _, key := range [][2]string{{"personal", "RGAPI-personal-0001"}, {"dev", "RGAPI-dev-000000002"}} {
		if err := s.AddRiotKey(key[0], key[1]); err != nil {
			t.Fatal(err)
		}
	}
	keys, err := s.RiotKeys()
	if err != nil || len(keys) != 2 || keys[0].Label != "personal" || keys[1].Masked() != "RGAPI-…0002" {
		t.Fatalf("keys: %+v %v", keys, err)
	}
	if err := s.MarkKeyRejected(keys[0], "HTTP 403"); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	usable, err := s.UsableRiotKeys()
	if err != nil || len(usable) != 1 || usable[0].Label != "dev" {
		t.Fatalf("rejection must survive a restart: %+v %v", usable, err)
	}
	if err := s.MarkKeyRejected(usable[0], "HTTP 401"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UsableRiotKeys(); !errors.Is(err, ErrKeyRejected) {
		t.Fatalf("all keys rejected: %v", err)
	}
	// Re-adding a replaced key makes it usable again.
	if err := s.AddRiotKey("personal", "RGAPI-personal-0001"); err != nil {
		t.Fatal(err)
	}
	if usable, err := s.UsableRiotKeys(); err != nil || len(usable) != 1 || usable[0].Label != "personal" {
		t.Fatalf("re-added key: %+v %v", usable, err)
	}

	t.Setenv("RIOT_API_KEY", "RGAPI-from-env-9999")
	keys, err = s.RiotKeys()
	if err != nil || !keys[0].FromEnv || keys[0].ID != 0 {
		t.Fatalf("environment key goes first: %+v %v", keys, err)
	}
	if err := s.MarkKeyRejected(keys[0], "HTTP 403"); err != nil {
		t.Fatal(err)
	}
}

type rejectedRiot struct{ fakeRiot }

func (*rejectedRiot) Rank(context.Context, string, string) (LeagueEntry, error) {
	return LeagueEntry{}, ErrKeyRejected
}

func TestUpdateStopsOnRejectedKey(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "matches.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := ImportSeeds(store, []Seed{{GameName: "FioraPlayer", TagLine: "EUW", Region: "euw1"}, {GameName: "Other", TagLine: "EUW", Region: "euw1"}}, nil); err != nil {
		t.Fatal(err)
	}
	api := &rejectedRiot{fakeRiot{accounts: map[string]Account{
		"FioraPlayer": {PUUID: "fiora", GameName: "FioraPlayer", TagLine: "EUW"},
		"Other":       {PUUID: "other", GameName: "Other", TagLine: "EUW"},
	}}}
	var logs []string
	err = UpdatePlayers(context.Background(), store, api, logReporter(func(message string) { logs = append(logs, message) }))
	if !errors.Is(err, ErrKeyRejected) {
		t.Fatalf("update error: %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("a rejected key must not be logged per player: %v", logs)
	}
	players, err := store.ListPlayers()
	if err != nil || len(players) != 2 {
		t.Fatalf("resolved players are kept: %+v %v", players, err)
	}
}

type logReporter func(string)

func (logReporter) Progress(UpdateSnapshot) {}
func (r logReporter) Log(message string)    { r(message) }
