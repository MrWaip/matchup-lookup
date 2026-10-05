package core

import (
	"path/filepath"
	"testing"
)

func TestManagedPlayerLifecycle(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "players.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	seed := Seed{GameName: "Example", TagLine: "EUW", Region: "euw1"}
	if err := s.SaveManagedPlayer(seed, false, []string{"coach", "top lane"}, []string{"Fiora", "Camille"}); err != nil {
		t.Fatal(err)
	}
	if pending, err := s.PendingSeeds(); err != nil || len(pending) != 0 {
		t.Fatalf("paused seed queued for update: %v, %v", pending, err)
	}
	if all, err := s.AllPendingSeeds(); err != nil || len(all) != 1 {
		t.Fatalf("paused seed absent from management: %v, %v", all, err)
	}
	if control, err := s.Control(seed); err != nil || control.Enabled || len(control.Tags) != 2 {
		t.Fatalf("control: %+v, %v", control, err)
	}
	if champions, err := s.SeedChampions(seed); err != nil || len(champions) != 2 {
		t.Fatalf("champions: %v, %v", champions, err)
	}

	if err := s.SaveManagedPlayer(seed, true, []string{"review"}, []string{"Garen"}); err != nil {
		t.Fatal(err)
	}
	if pending, err := s.PendingSeeds(); err != nil || len(pending) != 1 {
		t.Fatalf("enabled seed: %v, %v", pending, err)
	}
	if err := s.ResolveSeed(seed, Account{PUUID: "example-puuid", GameName: "Example", TagLine: "EUW"}); err != nil {
		t.Fatal(err)
	}
	match := sampleMatch()
	match.Info.Participants[0].PUUID = "example-puuid"
	match.Info.Participants[0].RiotIDGameName = "Example"
	if err := s.SaveMatch(match); err != nil {
		t.Fatal(err)
	}
	resolved, err := s.LoadPlayer("example-puuid")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveTrackedGame(match, resolved, match.Info.Participants[0], &match.Info.Participants[1], "confirmed"); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkChecked(match.Metadata.MatchID, resolved.PUUID); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveManagedPlayer(seed, false, []string{"review"}, []string{"Fiora"}); err != nil {
		t.Fatal(err)
	}
	if players, err := s.ListActivePlayers(); err != nil || len(players) != 0 {
		t.Fatalf("paused player in update: %v, %v", players, err)
	}
	if champions, err := s.PlayerChampions("example-puuid"); err != nil || len(champions) != 1 || champions[0] != "Fiora" {
		t.Fatalf("edited champions: %v, %v", champions, err)
	}
	second := Seed{GameName: "Another", TagLine: "EUW", Region: "euw1"}
	if err := s.SaveManagedPlayer(second, true, []string{"practice"}, nil); err != nil {
		t.Fatal(err)
	}
	identities := []PlayerIdentity{{GameName: seed.GameName, TagLine: seed.TagLine, Region: seed.Region}, {GameName: second.GameName, TagLine: second.TagLine, Region: second.Region}}
	if err := s.SetPlayersEnabled(identities, true); err != nil {
		t.Fatal(err)
	}
	if active, err := s.ListActivePlayers(); err != nil || len(active) != 1 {
		t.Fatalf("bulk enable: %v, %v", active, err)
	}
	if control, err := s.Control(seed); err != nil || !control.Enabled || len(control.Tags) != 1 || control.Tags[0] != "review" {
		t.Fatalf("bulk enable lost tags: %+v, %v", control, err)
	}
	if err := s.SetPlayersEnabled(identities, false); err != nil {
		t.Fatal(err)
	}
	if pending, err := s.PendingSeeds(); err != nil || len(pending) != 0 {
		t.Fatalf("bulk pause: %v, %v", pending, err)
	}
	if err := s.DeleteManagedPlayers(identities); err != nil {
		t.Fatal(err)
	}
	if players, err := s.ListPlayers(); err != nil || len(players) != 0 {
		t.Fatalf("deleted player: %v, %v", players, err)
	}
	if all, err := s.AllPendingSeeds(); err != nil || len(all) != 0 {
		t.Fatalf("deleted seeds: %v, %v", all, err)
	}
	if control, err := s.Control(seed); err != nil || !control.Enabled || len(control.Tags) != 0 {
		t.Fatalf("deleted control: %+v, %v", control, err)
	}
	for _, table := range []string{"tracked_games", "match_checks"} {
		var count int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s after delete: %d, %v", table, count, err)
		}
	}
}
