package core

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSearchCanViewStoredMatchFromOpponentSide(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "matches.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	seed := Seed{GameName: "FioraMain", TagLine: "EUW", Region: "euw1"}
	if err := s.UpsertPlayer(seed, Account{PUUID: "fiora-puuid", GameName: seed.GameName, TagLine: seed.TagLine}); err != nil {
		t.Fatal(err)
	}
	m := Match{}
	m.Metadata.MatchID = "EUW1_12345"
	m.Info.PlatformID = "EUW1"
	m.Info.GameCreation = time.Now().UnixMilli()
	m.Info.GameDuration = 1800
	m.Info.GameVersion = "16.19.1"
	m.Info.QueueID = 420
	fiora := Participant{PUUID: "fiora-puuid", RiotIDGameName: "FioraMain", RiotIDTagline: "EUW", ChampionName: "Fiora", TeamID: 100, TeamPosition: "TOP", Win: false, Kills: 0, Deaths: 5, Assists: 1, TotalMinionsKilled: 95,
		Summoner1ID: 4, Summoner2ID: 12, Perks: Perks{Styles: []PerkStyle{{Selections: []PerkSelection{{Perk: 8010}}}, {Selections: []PerkSelection{{Perk: 8473}, {Perk: 8451}}}}}}
	darius := Participant{PUUID: "darius-puuid", RiotIDGameName: "DariusMain", RiotIDTagline: "EUW", ChampionName: "Darius", TeamID: 200, TeamPosition: "TOP", Win: true, Kills: 6, Deaths: 0, Assists: 3, TotalMinionsKilled: 174,
		Summoner1ID: 6, Summoner2ID: 4, Perks: Perks{Styles: []PerkStyle{{Selections: []PerkSelection{{Perk: 8010}}}, {Selections: []PerkSelection{{Perk: 8473}, {Perk: 8444}}}}}}
	m.Info.Participants = []Participant{fiora, darius}
	if err := s.SaveMatch(m); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveTrackedGame(m, Player{PUUID: fiora.PUUID, GameName: "FioraMain", TagLine: "EUW", Region: "euw1", Tier: "MASTER", Division: "I"}, fiora, &darius, "confirmed"); err != nil {
		t.Fatal(err)
	}
	forward, _, _, matching, wins, err := Search(s, Filters{Champion: "Fiora", Opponent: "Darius", Result: "loss", Limit: 10})
	if err != nil || matching != 1 || wins != 0 || len(forward) != 1 {
		t.Fatalf("forward: rows=%+v matching=%d wins=%d err=%v", forward, matching, wins, err)
	}
	if r := forward[0]; r.Spells != "Flash + Teleport" || r.Keystone != "Conqueror" || r.SecondaryRunes != "Bone Plating + Overgrowth" {
		t.Fatalf("forward loadout: %+v", r)
	}
	reverse, _, _, matching, wins, err := Search(s, Filters{Champion: "Darius", Opponent: "Fiora", Result: "win", Limit: 10})
	if err != nil || matching != 1 || wins != 1 || len(reverse) != 1 {
		t.Fatalf("reverse: rows=%+v matching=%d wins=%d err=%v", reverse, matching, wins, err)
	}
	if r := reverse[0]; r.PlayerID != "DariusMain#EUW" || r.OpponentID != "FioraMain#EUW" || r.KDA != "6/0/3" || r.OpponentKDA != "0/5/1" || r.CS != 174 || r.MatchID != m.Metadata.MatchID || r.Spells != "Ghost + Flash" || r.Keystone != "Conqueror" || r.SecondaryRunes != "Bone Plating + Second Wind" {
		t.Fatalf("reverse row: %+v", r)
	}
	_, _, _, matching, wins, err = Search(s, Filters{Champion: "Darius", Opponent: "Fiora", Result: "win", KDACompare: "gt", Limit: 10})
	if err != nil || matching != 1 || wins != 1 {
		t.Fatalf("reverse KDA comparison: matching=%d wins=%d err=%v", matching, wins, err)
	}
	// Match-v5 does not contain a rank for the opposing player.
	_, _, _, matching, _, err = Search(s, Filters{Champion: "Darius", Opponent: "Fiora", Rank: "Diamond+", Limit: 10})
	if err != nil || matching != 0 {
		t.Fatalf("unknown opponent rank must not pass Diamond+: matching=%d err=%v", matching, err)
	}
	if err := s.UpsertPlayer(Seed{GameName: "DariusMain", TagLine: "EUW", Region: "euw1"}, Account{PUUID: darius.PUUID, GameName: "DariusMain", TagLine: "EUW"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveTrackedGame(m, Player{PUUID: darius.PUUID, Tier: "DIAMOND", Division: "I"}, darius, &fiora, "confirmed"); err != nil {
		t.Fatal(err)
	}
	rows, _, _, matching, wins, err := Search(s, Filters{Champion: "Darius", Opponent: "Fiora", Result: "win", Rank: "Diamond+", Limit: 10})
	if err != nil || matching != 1 || wins != 1 || len(rows) != 1 || rows[0].Rank != "DIAMOND I" {
		t.Fatalf("tracked opponent must have one ranked row: rows=%+v matching=%d wins=%d err=%v", rows, matching, wins, err)
	}
}
