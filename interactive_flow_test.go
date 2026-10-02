package main

import "testing"

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
