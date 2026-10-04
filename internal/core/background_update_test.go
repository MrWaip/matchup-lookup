package core

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

type waitingRiot struct {
	entered chan struct{}
}

func (w *waitingRiot) Resolve(context.Context, Seed) (Account, error) {
	return Account{PUUID: "test-puuid", GameName: "Player", TagLine: "EUW"}, nil
}
func (w *waitingRiot) Rank(context.Context, string, string) (LeagueEntry, error) {
	return LeagueEntry{}, nil
}
func (w *waitingRiot) Recent(ctx context.Context, _, _ string) ([]string, error) {
	close(w.entered)
	<-ctx.Done()
	return nil, ctx.Err()
}
func (w *waitingRiot) Match(context.Context, string, string) (Match, error) {
	panic("unexpected match request")
}

func TestBackgroundUpdateDoesNotBlockSearchAndCanCancel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "matches.db")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := ImportSeeds(store, []Seed{{GameName: "Player", TagLine: "EUW", Region: "euw1"}}); err != nil {
		t.Fatal(err)
	}
	bg := NewBackgroundUpdate(path)
	api := &waitingRiot{entered: make(chan struct{})}
	if err := bg.Start(api); err != nil {
		t.Fatal(err)
	}
	defer bg.Stop()
	select {
	case <-api.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("background update did not reach Riot API")
	}
	if !bg.Snapshot().Running {
		t.Fatal("update unexpectedly stopped")
	}
	if err := bg.Start(api); err == nil {
		t.Fatal("second update was allowed")
	}
	if _, _, _, _, _, err := Search(store, Filters{Days: 7, Limit: 10}); err != nil {
		t.Fatalf("search while update runs: %v", err)
	}
	bg.Stop()
	if bg.Snapshot().Running {
		t.Fatal("cancel did not stop update")
	}
}
