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
	if err := ImportSeeds(store, seeds, nil); err != nil {
		t.Fatal(err)
	}
	pending, err := store.PendingSeeds()
	if err != nil || len(pending) != 2 {
		t.Fatalf("offline import: %d pending, %v", len(pending), err)
	}
	for i := 0; i < 2; i++ {
		if err := UpdatePlayers(context.Background(), store, api, nil); err != nil {
			t.Fatal(err)
		}
	}
	if api.matchCalls != 1 {
		t.Fatalf("match fetched %d times; want one", api.matchCalls)
	}
	if err := ImportSeeds(store, []Seed{{GameName: "FioraPlayer", TagLine: "EUW", Region: "euw1", Champion: "Camille"}}, nil); err != nil {
		t.Fatal(err)
	}
	champions, err := store.PlayerChampions("fiora")
	if err != nil || len(champions) != 2 || champions[0] != "Camille" || champions[1] != "Fiora" {
		t.Fatalf("player champions: %v %v", champions, err)
	}
	found, err := Search(store, Filters{Champion: "Fiora", Opponent: "darius", Result: "win", KDACompare: "ge", Rank: "diamond", Region: "euw1", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	rows := found.Results
	if found.Players != 2 || found.Stored != 2 || found.Matching != 1 || found.Wins != 1 || len(rows) != 1 || found.Patch.String() != "26.19" {
		t.Fatalf("unexpected counts: %+v", found)
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
	found, err = Search(store, Filters{Region: "na1", Limit: 10})
	if err != nil || found.Matching != 0 || len(found.Results) != 0 {
		t.Fatalf("wrong server returned games: %+v %v", found, err)
	}
	found, err = Search(store, Filters{Opponent: "Darius", Result: "loss", Rank: "diamond", Region: "euw1", Limit: 10})
	if err != nil || found.Matching != 0 {
		t.Fatalf("loss filter: %+v, err %v", found, err)
	}
	found, err = Search(store, Filters{Champion: "LeeSin", Region: "euw1", Limit: 10})
	if err != nil || found.Matching != 1 {
		t.Fatalf("other champion: %+v, err %v", found, err)
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
	if err := store.SaveTrackedGame(m, player, m.Info.Participants[0], &m.Info.Participants[1], "confirmed"); err != nil {
		t.Fatal(err)
	}
	forgetLoadout(t, store, player.PUUID)
	if err := store.MarkChecked(m.Metadata.MatchID, player.PUUID); err != nil {
		t.Fatal(err)
	}
	m.Info.Participants[0].Summoner1ID = 4
	m.Info.Participants[0].Summoner2ID = 12
	m.Info.Participants[0].Perks = Perks{Styles: []PerkStyle{{Selections: []PerkSelection{{Perk: 8010}}}, {Selections: []PerkSelection{{Perk: 8473}, {Perk: 8451}}}}}
	api := &fakeRiot{match: m}
	progress := newUpdateProgress(0, 1, nil)
	for range 2 {
		if failed, err := collectMatch(context.Background(), store, api, player, m.Metadata.MatchID, progress); failed || err != nil {
			t.Fatalf("refresh failed=%v err=%v", failed, err)
		}
	}
	if api.matchCalls != 1 {
		t.Fatalf("legacy match fetched %d times, want once", api.matchCalls)
	}
	found, err := Search(store, Filters{Champion: "Fiora", Limit: 10})
	if err != nil || len(found.Results) != 1 || found.Results[0].Spells != "Flash + Teleport" || found.Results[0].SecondaryRunes != "Bone Plating + Overgrowth" {
		t.Fatalf("refreshed loadout: %+v err=%v", found, err)
	}
}

// forgetLoadout makes a stored participant look like one saved before
// loadouts were collected (migrated from JSON without them).
func forgetLoadout(t *testing.T, store *Store, puuid string) {
	t.Helper()
	if _, err := store.db.Exec(`UPDATE match_participants
		SET summoner1_id = NULL, summoner2_id = NULL, keystone = NULL WHERE puuid = ?`, puuid); err != nil {
		t.Fatal(err)
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
	if err := store.SaveTrackedGame(m, player, m.Info.Participants[0], &m.Info.Participants[1], "confirmed"); err != nil {
		t.Fatal(err)
	}
	forgetLoadout(t, store, player.PUUID)
	m.Info.Participants[0].Summoner1ID = 4
	m.Info.Participants[0].Summoner2ID = 12
	api := &noRecentRiot{fakeRiot: fakeRiot{match: m}}
	if err := UpdatePlayers(context.Background(), store, api, nil); err != nil {
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
	if err := ImportSeeds(store, []Seed{{GameName: "FioraPlayer", TagLine: "EUW", Region: "euw1"}}, nil); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := UpdatePlayers(context.Background(), store, api, nil); err != nil {
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
	filters := Filters{Champion: "Fiora", Opponent: "Darius", Region: "euw1", Result: "win", Rank: "diamond", Limit: 100}
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
	if err := ImportSeeds(store, []Seed{{GameName: "FioraPlayer", TagLine: "EUW", Region: "euw1"}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := UpdatePlayers(context.Background(), store, api, nil); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		compare string
		want    int
	}{{"ge", 1}, {"gt", 0}} {
		found, err := Search(store, Filters{Champion: "Fiora", Opponent: "Darius", Result: "win", KDACompare: tc.compare, Limit: 10})
		if err != nil || found.Matching != tc.want {
			t.Fatalf("compare %s: got %d, want %d; err %v", tc.compare, found.Matching, tc.want, err)
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

func TestPruneOldPatchesKeepsChecks(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "matches.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	player := Player{PUUID: "fiora", GameName: "FioraPlayer", TagLine: "EUW", Region: "euw1"}
	if err := store.UpsertPlayer(Seed{Region: player.Region}, Account{PUUID: player.PUUID, GameName: player.GameName, TagLine: player.TagLine}); err != nil {
		t.Fatal(err)
	}
	for id, version := range map[string]string{"EUW1_OLD": "16.9.1", "EUW1_NEW": "16.19.1"} {
		m := sampleMatch()
		m.Metadata.MatchID, m.Info.GameVersion = id, version
		if err := store.SaveMatch(m); err != nil {
			t.Fatal(err)
		}
		if err := store.SaveTrackedGame(m, player, m.Info.Participants[0], &m.Info.Participants[1], "confirmed"); err != nil {
			t.Fatal(err)
		}
		if err := store.MarkChecked(id, player.PUUID); err != nil {
			t.Fatal(err)
		}
	}
	removed, err := store.PruneOldPatches()
	if err != nil || removed != 1 {
		t.Fatalf("removed %d, err %v; want the 16.9 match only (16.19 is newer)", removed, err)
	}
	found, err := Search(store, Filters{Champion: "Fiora", Limit: 10})
	if err != nil || found.Stored != 1 || len(found.Results) != 1 || found.Results[0].MatchID != "EUW1_NEW" {
		t.Fatalf("after prune: %+v err=%v", found, err)
	}
	if _, cached, _ := store.CachedMatch("EUW1_OLD"); cached {
		t.Fatal("old match still cached")
	}
	if checked, err := store.GameExists("EUW1_OLD", player.PUUID); err != nil || !checked {
		t.Fatal("pruned match must stay checked so it is not downloaded again")
	}
}

func TestSearchShowsOnlyCurrentPatch(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "matches.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	player := Player{PUUID: "fiora", GameName: "FioraPlayer", TagLine: "EUW", Region: "euw1"}
	if err := store.UpsertPlayer(Seed{Region: player.Region}, Account{PUUID: player.PUUID, GameName: player.GameName, TagLine: player.TagLine}); err != nil {
		t.Fatal(err)
	}
	for id, version := range map[string]string{"EUW1_OLD": "16.18.1", "EUW1_NEW": "16.19.1"} {
		m := sampleMatch()
		m.Metadata.MatchID, m.Info.GameVersion = id, version
		if err := store.SaveMatch(m); err != nil {
			t.Fatal(err)
		}
		if err := store.SaveTrackedGame(m, player, m.Info.Participants[0], &m.Info.Participants[1], "confirmed"); err != nil {
			t.Fatal(err)
		}
	}
	found, err := Search(store, Filters{Champion: "Fiora", Limit: 10})
	if err != nil || found.Patch.String() != "26.19" || found.Stored != 1 || len(found.Results) != 1 || found.Results[0].MatchID != "EUW1_NEW" {
		t.Fatalf("newest stored patch: %+v err=%v", found, err)
	}
	// Riot released 26.20 but no match of it is stored yet: nothing is watchable.
	if err := store.setSetting("live_patch", "16.20.1"); err != nil {
		t.Fatal(err)
	}
	found, err = Search(store, Filters{Champion: "Fiora", Limit: 10})
	if err != nil || found.Patch.String() != "26.20" || found.Stored != 0 || found.Matching != 0 {
		t.Fatalf("live patch ahead of stored matches: %+v err=%v", found, err)
	}
	// An outdated live patch never hides newer stored matches.
	if err := store.setSetting("live_patch", "16.17.1"); err != nil {
		t.Fatal(err)
	}
	if found, err = Search(store, Filters{Champion: "Fiora", Limit: 10}); err != nil || found.Patch.String() != "26.19" {
		t.Fatalf("stale live patch: %+v err=%v", found, err)
	}
}
