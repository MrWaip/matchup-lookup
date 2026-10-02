package main

import (
	"strconv"
	"strings"
	"testing"
)

func TestSearchReturnsDirectlyToMenu(t *testing.T) {
	for _, action := range []string{"find", "repeat"} {
		if requiresReturnPrompt(action, true, nil) {
			t.Fatalf("%s should return directly to menu after replay choice", action)
		}
		if !requiresReturnPrompt(action, false, nil) {
			t.Fatalf("%s with no matches should leave the empty result readable", action)
		}
	}
	if !requiresReturnPrompt("players", false, nil) {
		t.Fatal("player list still needs a pause so it remains readable")
	}
}

func TestReplayPagesKeepEveryMatchReachableWithBothPlayersVisible(t *testing.T) {
	rows := make([]Result, 10)
	for i := range rows {
		rows[i] = Result{Champion: "Fiora", PlayerID: "Blue#EUW", Opponent: "Darius", OpponentID: "Red#EUW"}
	}
	seen := make(map[string]bool)
	for page := 0; page < 3; page++ {
		options := replayPageOptions(rows, page)
		for _, option := range options {
			if _, err := strconv.Atoi(option.Value); err == nil {
				seen[option.Value] = true
				if !strings.Contains(option.Key, "Fiora Blue#EUW vs Darius Red#EUW") {
					t.Fatalf("match label hides players: %q", option.Key)
				}
			}
		}
		if page < 2 && options[len(options)-2].Value != "next" {
			t.Fatalf("page %d has no next option: %+v", page, options)
		}
	}
	if len(seen) != len(rows) {
		t.Fatalf("reachable matches: %d, want %d", len(seen), len(rows))
	}
}
