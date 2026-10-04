package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"matchup-lookup/internal/core"
)

func TestRejectedKeyFallsBackToNextKey(t *testing.T) {
	var rejected []string
	client := NewRiotClient([]core.APIKey{{ID: 1, Label: "dev", Value: "expired"}, {ID: 2, Label: "personal", Value: "good"}},
		func(key core.APIKey, _ string) { rejected = append(rejected, key.Label) })
	requests := map[string]int{}
	client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		key := r.Header.Get("X-Riot-Token")
		requests[key]++
		status, body := 200, `["EUW1_1"]`
		if key != "good" {
			status, body = 403, `{"status":{"message":"Forbidden","status_code":403}}`
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})

	for range 2 {
		ids, err := client.Recent(context.Background(), "euw1", "puuid")
		if err != nil || len(ids) != 1 {
			t.Fatalf("recent: %v %v", ids, err)
		}
	}
	if len(rejected) != 1 || rejected[0] != "dev" || requests["expired"] != 1 || requests["good"] != 2 {
		t.Fatalf("rejected=%v requests=%v", rejected, requests)
	}

	client.reject(1, "HTTP 403")
	before := requests["good"]
	if _, err := client.Recent(context.Background(), "euw1", "puuid"); !errors.Is(err, core.ErrKeyRejected) {
		t.Fatalf("all keys rejected: %v", err)
	}
	if requests["good"] != before {
		t.Fatal("a client without usable keys must not call Riot")
	}
}
