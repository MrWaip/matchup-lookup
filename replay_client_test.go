package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOpenReplayViaLocalClient(t *testing.T) {
	replayFolder := t.TempDir()
	var downloaded, watched bool
	lockfile := filepath.Join(t.TempDir(), "lockfile")
	if err := os.WriteFile(lockfile, []byte("LeagueClient:100:12345:temporary-password:https"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MATCHUP_LCU_LOCKFILE", lockfile)
	client, err := connectLeagueClient()
	if err != nil {
		t.Fatal(err)
	}
	client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "127.0.0.1:12345" {
			t.Errorf("request escaped local client: %s", r.URL)
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "riot" || pass != "temporary-password" {
			t.Errorf("wrong local client authentication")
		}
		status, body := http.StatusOK, ""
		switch r.URL.Path {
		case "/riotclient/region-locale":
			body = `{"region":"EUW"}`
		case "/lol-replays/v1/rofls/path":
			body = fmt.Sprintf("%q", replayFolder)
		case "/lol-replays/v1/rofls/123/download/graceful":
			downloaded = true
			if err := os.WriteFile(filepath.Join(replayFolder, "EUW1-123.rofl"), make([]byte, 2048), 0600); err != nil {
				t.Error(err)
			}
			status = http.StatusNoContent
		case "/lol-replays/v1/rofls/123/watch":
			watched = true
			status = http.StatusNoContent
		default:
			t.Errorf("unexpected local client endpoint %s", r.URL.Path)
			status = http.StatusNotFound
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	if err := openReplay(context.Background(), client, "EUW1_123"); err != nil {
		t.Fatal(err)
	}
	if !downloaded || !watched {
		t.Fatalf("download=%t watch=%t", downloaded, watched)
	}
	downloaded, watched = false, false
	if err := openReplay(context.Background(), client, "EUW1_123"); err != nil {
		t.Fatal(err)
	}
	if downloaded || !watched {
		t.Fatalf("cached replay: download=%t watch=%t", downloaded, watched)
	}
	downloaded, watched = false, false
	if err := openReplay(context.Background(), client, "NA1_123"); err == nil || !strings.Contains(err.Error(), "NA") {
		t.Fatalf("expected server mismatch, got %v", err)
	}
	if downloaded || watched {
		t.Fatal("cross-server replay reached download/watch")
	}
}

func TestInvalidReplayIDs(t *testing.T) {
	for _, id := range []string{"", "EUW1", "EUW1_abc", "EUW1_123/../../bad", "EUW1_12_3"} {
		if _, _, err := replayMatchID(id); err == nil {
			t.Errorf("accepted invalid match ID %q", id)
		}
	}
}
