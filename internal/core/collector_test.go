package core

import (
	"context"
	"fmt"
	"path/filepath"
	"sync/atomic"
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
func (f *fakeRiot) Rank(context.Context, string, string) (LeagueEntry, error) {
	return LeagueEntry{QueueType: "RANKED_SOLO_5x5", Tier: "DIAMOND", Rank: "II", LeaguePoints: 44}, nil
}
func (f *fakeRiot) Recent(context.Context, string, string) ([]string, error) {
	return []string{f.match.Metadata.MatchID}, nil
}
func (f *fakeRiot) Match(context.Context, string, string) (Match, error) {
	f.matchCalls++
	return f.match, nil
}

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
	seeds := []Seed{{GameName: "FioraPlayer", TagLine: "EUW", Region: "euw1", Champion: "Fiora"}, {GameName: "Other", TagLine: "EUW", Region: "euw1"}}
	if err := ImportSeeds(store, seeds); err != nil {
		t.Fatal(err)
	}
	pending, err := store.PendingSeeds()
	if err != nil || len(pending) != 2 {
		t.Fatalf("offline import: %d pending, %v", len(pending), err)
	}
	for i := 0; i < 2; i++ {
		if err := UpdatePlayers(context.Background(), store, api); err != nil {
			t.Fatal(err)
		}
	}
	if api.matchCalls != 1 {
		t.Fatalf("match fetched %d times; want one", api.matchCalls)
	}
	if err := ImportSeeds(store, []Seed{{GameName: "FioraPlayer", TagLine: "EUW", Region: "euw1", Champion: "Camille"}}); err != nil {
		t.Fatal(err)
	}
	champions, err := store.PlayerChampions("fiora")
	if err != nil || len(champions) != 2 || champions[0] != "Camille" || champions[1] != "Fiora" {
		t.Fatalf("player champions: %v %v", champions, err)
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
	if rows[0].Region != "EUW" {
		t.Fatalf("server label: %s", rows[0].Region)
	}
	if rows[0].Patch != "26.19" {
		t.Fatalf("displayed patch: %s", rows[0].Patch)
	}
	for _, patch := range []string{"16.19", "26.19"} {
		_, _, _, count, _, err := Search(store, Filters{Patch: patch, Champion: "Fiora", Limit: 10})
		if err != nil || count != 1 {
			t.Fatalf("patch %s: count=%d err=%v", patch, count, err)
		}
	}
	rows, _, _, matching, _, err = Search(store, Filters{Region: "na1", Limit: 10})
	if err != nil || matching != 0 || len(rows) != 0 {
		t.Fatalf("wrong server returned games: %d %v", matching, err)
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

func TestCollectMatchRefreshesLegacyLoadoutOnce(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "matches.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	player := Player{PUUID: "fiora", GameName: "FioraPlayer", TagLine: "EUW", Region: "euw1"}
	if err := store.UpsertPlayer(Seed{GameName: player.GameName, TagLine: player.TagLine, Region: player.Region}, Account{PUUID: player.PUUID, GameName: player.GameName, TagLine: player.TagLine}); err != nil {
		t.Fatal(err)
	}
	m := sampleMatch()
	if err := store.SaveMatch(m); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`UPDATE matches SET raw_json=json_remove(raw_json,
		'$.info.participants[0].summoner1Id', '$.info.participants[0].summoner2Id', '$.info.participants[0].perks')`); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveTrackedGame(m, player, m.Info.Participants[0], &m.Info.Participants[1], "confirmed"); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkChecked(m.Metadata.MatchID, player.PUUID); err != nil {
		t.Fatal(err)
	}
	m.Info.Participants[0].Summoner1ID = 4
	m.Info.Participants[0].Summoner2ID = 12
	m.Info.Participants[0].Perks = Perks{Styles: []PerkStyle{{Selections: []PerkSelection{{Perk: 8010}}}, {Selections: []PerkSelection{{Perk: 8473}, {Perk: 8451}}}}}
	api := &fakeRiot{match: m}
	progress := newUpdateProgress(0, 1)
	for range 2 {
		if failed, err := collectMatch(context.Background(), store, api, player, m.Metadata.MatchID, progress); failed || err != nil {
			t.Fatalf("refresh failed=%v err=%v", failed, err)
		}
	}
	if api.matchCalls != 1 {
		t.Fatalf("legacy match fetched %d times, want once", api.matchCalls)
	}
	rows, _, _, _, _, err := Search(store, Filters{Champion: "Fiora", Limit: 10})
	if err != nil || len(rows) != 1 || rows[0].Spells != "Flash + Teleport" || rows[0].SecondaryRunes != "Bone Plating + Overgrowth" {
		t.Fatalf("refreshed loadout: rows=%+v err=%v", rows, err)
	}
}

type noRecentRiot struct{ fakeRiot }

func (*noRecentRiot) Recent(context.Context, string, string) ([]string, error) {
	return nil, nil
}

func TestUpdateBackfillsLegacyMatchesOutsideRecentList(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "matches.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	player := Player{PUUID: "fiora", GameName: "FioraPlayer", TagLine: "EUW", Region: "euw1"}
	if err := store.UpsertPlayer(Seed{GameName: player.GameName, TagLine: player.TagLine, Region: player.Region}, Account{PUUID: player.PUUID, GameName: player.GameName, TagLine: player.TagLine}); err != nil {
		t.Fatal(err)
	}
	m := sampleMatch()
	if err := store.SaveMatch(m); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`UPDATE matches SET raw_json=json_remove(raw_json,
		'$.info.participants[0].summoner1Id', '$.info.participants[0].summoner2Id', '$.info.participants[0].perks')`); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveTrackedGame(m, player, m.Info.Participants[0], &m.Info.Participants[1], "confirmed"); err != nil {
		t.Fatal(err)
	}
	m.Info.Participants[0].Summoner1ID = 4
	m.Info.Participants[0].Summoner2ID = 12
	api := &noRecentRiot{fakeRiot: fakeRiot{match: m}}
	if err := UpdatePlayers(context.Background(), store, api); err != nil {
		t.Fatal(err)
	}
	if api.matchCalls != 1 {
		t.Fatalf("old match fetched %d times, want once", api.matchCalls)
	}
}

type parallelRiot struct {
	fakeRiot
	inFlight    atomic.Int32
	maxInFlight atomic.Int32
	calls       atomic.Int32
}

func (p *parallelRiot) Recent(context.Context, string, string) ([]string, error) {
	ids := make([]string, 8)
	for i := range ids {
		ids[i] = fmt.Sprintf("EUW1_%d", i+1)
	}
	return ids, nil
}

func (p *parallelRiot) Match(_ context.Context, _ string, id string) (Match, error) {
	n := p.inFlight.Add(1)
	for {
		max := p.maxInFlight.Load()
		if n <= max || p.maxInFlight.CompareAndSwap(max, n) {
			break
		}
	}
	time.Sleep(15 * time.Millisecond)
	p.inFlight.Add(-1)
	p.calls.Add(1)
	m := sampleMatch()
	m.Metadata.MatchID = id
	return m, nil
}

func TestParallelFetchAndResumeWithoutAPI(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "matches.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	api := &parallelRiot{fakeRiot: fakeRiot{accounts: map[string]Account{"FioraPlayer": {PUUID: "fiora", GameName: "FioraPlayer", TagLine: "EUW"}}}}
	if err := ImportSeeds(store, []Seed{{GameName: "FioraPlayer", TagLine: "EUW", Region: "euw1"}}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := UpdatePlayers(context.Background(), store, api); err != nil {
			t.Fatal(err)
		}
	}
	if got := api.calls.Load(); got != 8 {
		t.Fatalf("fetched %d matches, want 8", got)
	}
	if got := api.maxInFlight.Load(); got < 2 || got > 4 {
		t.Fatalf("concurrency %d, want 2-4", got)
	}
}

func TestLastSearchPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "matches.db")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	filters := Filters{Champion: "Fiora", Opponent: "Darius", Region: "euw1", Result: "win", Rank: "diamond", Days: 7, Limit: 100}
	if err := store.SaveFilters(filters); err != nil {
		t.Fatal(err)
	}
	store.Close()
	store, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	got, found, err := store.LastFilters()
	if err != nil || !found || got != filters {
		t.Fatalf("saved filters: %+v, found=%v, err=%v", got, found, err)
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
	if err := ImportSeeds(store, []Seed{{GameName: "FioraPlayer", TagLine: "EUW", Region: "euw1"}}); err != nil {
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
