package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

type fakeRiot struct {
	accounts   map[string]Account
	match      Match
	matchCalls int
}

func (f *fakeRiot) Resolve(_ context.Context, s Seed) (Account, error) {
	return f.accounts[s.GameName], nil
}
func (f *fakeRiot) Rank(context.Context, string) (LeagueEntry, error) {
	return LeagueEntry{QueueType: "RANKED_SOLO_5x5", Tier: "DIAMOND", Rank: "II", LeaguePoints: 44}, nil
}
func (f *fakeRiot) Recent(context.Context, string) ([]string, error) {
	return []string{f.match.Metadata.MatchID}, nil
}
func (f *fakeRiot) Match(context.Context, string) (Match, error) { f.matchCalls++; return f.match, nil }

func sampleMatch() Match {
	var m Match
	m.Metadata.MatchID = "EUW1_123"
	m.Info.PlatformID = "EUW1"
	m.Info.GameVersion = "16.19.123"
	m.Info.GameCreation = time.Now().Add(-time.Hour).UnixMilli()
	m.Info.GameDuration = 1800
	m.Info.QueueID = 420
	m.Info.Participants = []Participant{
		{PUUID: "fiora", RiotIDGameName: "FioraPlayer", RiotIDTagline: "EUW", ChampionName: "Fiora", TeamID: 100, TeamPosition: "TOP", Win: true, Kills: 8, Deaths: 2, Assists: 5, TotalMinionsKilled: 210},
		{PUUID: "darius", RiotIDGameName: "DariusPlayer", RiotIDTagline: "EUW", ChampionName: "Darius", TeamID: 200, TeamPosition: "TOP", Kills: 5, Deaths: 4, Assists: 2},
		{PUUID: "other", RiotIDGameName: "Other", RiotIDTagline: "EUW", ChampionName: "LeeSin", TeamID: 200, TeamPosition: "JUNGLE"},
	}
	return m
}

func TestCollectAndSearchOffline(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "matches.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	m := sampleMatch()
	api := &fakeRiot{accounts: map[string]Account{
		"FioraPlayer": {PUUID: "fiora", GameName: "FioraPlayer", TagLine: "EUW"},
		"Other":       {PUUID: "other", GameName: "Other", TagLine: "EUW"},
	}, match: m}
	seeds := []Seed{{GameName: "FioraPlayer", TagLine: "EUW", Region: "euw1"}, {GameName: "Other", TagLine: "EUW", Region: "euw1"}}
	if err := ImportSeeds(context.Background(), store, api, seeds); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := UpdatePlayers(context.Background(), store, api); err != nil {
			t.Fatal(err)
		}
	}
	if api.matchCalls != 1 {
		t.Fatalf("match fetched %d times; want one", api.matchCalls)
	}
	rows, players, total, matching, wins, err := Search(store, Filters{Champion: "Fiora", Opponent: "darius", Result: "win", KDACompare: "ge", Rank: "diamond", Days: 7, Region: "euw1", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if players != 2 || total != 2 || matching != 1 || wins != 1 || len(rows) != 1 {
		t.Fatalf("unexpected counts: %d %d %d %d rows=%d", players, total, matching, wins, len(rows))
	}
	if rows[0].PlayerID != "FioraPlayer#EUW" || rows[0].Champion != "Fiora" || rows[0].Opponent != "Darius" || rows[0].Status != "confirmed" || rows[0].OpponentKDA != "5/4/2" {
		t.Fatalf("bad result: %+v", rows[0])
	}
	rows, _, _, matching, _, err = Search(store, Filters{Opponent: "Darius", Result: "loss", Rank: "diamond", Days: 7, Region: "euw1", Limit: 10})
	if err != nil || matching != 0 || len(rows) != 0 {
		t.Fatalf("loss filter: %d rows, err %v", len(rows), err)
	}
	rows, _, _, matching, _, err = Search(store, Filters{Champion: "LeeSin", Days: 7, Region: "euw1", Limit: 10})
	if err != nil || matching != 1 || len(rows) != 1 {
		t.Fatalf("other champion: %d rows, err %v", len(rows), err)
	}
}

func TestKDAEqualityFilter(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "matches.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	m := sampleMatch()
	m.Info.Participants[1].Kills = 13
	m.Info.Participants[1].Deaths = 2
	m.Info.Participants[1].Assists = 0
	api := &fakeRiot{accounts: map[string]Account{"FioraPlayer": {PUUID: "fiora", GameName: "FioraPlayer", TagLine: "EUW"}}, match: m}
	if err := ImportSeeds(context.Background(), store, api, []Seed{{GameName: "FioraPlayer", TagLine: "EUW", Region: "euw1"}}); err != nil {
		t.Fatal(err)
	}
	if err := UpdatePlayers(context.Background(), store, api); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		compare string
		want    int
	}{{"ge", 1}, {"gt", 0}} {
		_, _, _, matching, _, err := Search(store, Filters{Champion: "Fiora", Opponent: "Darius", Result: "win", KDACompare: tc.compare, Limit: 10})
		if err != nil || matching != tc.want {
			t.Fatalf("compare %s: got %d, want %d; err %v", tc.compare, matching, tc.want, err)
		}
	}
}

func TestOpponentPositionFallbackAndAmbiguity(t *testing.T) {
	m := sampleMatch()
	f := m.Info.Participants[0]
	m.Info.Participants[1].TeamPosition = ""
	m.Info.Participants[1].Lane = "TOP"
	o, status := opponent(m, f)
	if o == nil || o.ChampionName != "Darius" || status != "lane_fallback" {
		t.Fatalf("fallback: %+v %s", o, status)
	}
	m.Info.Participants[2].TeamPosition = ""
	m.Info.Participants[2].Lane = "TOP"
	o, status = opponent(m, f)
	if o != nil || status != "ambiguous" {
		t.Fatalf("ambiguous: %+v %s", o, status)
	}
	f.TeamPosition = "MIDDLE"
	o, status = opponent(m, f)
	if o != nil || status != "ambiguous" {
		t.Fatalf("not top: %+v %s", o, status)
	}
}
