package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type RiotClient struct {
	key  string
	http *http.Client
	last time.Time
}

func NewRiotClient(key string) *RiotClient {
	return &RiotClient{key: key, http: &http.Client{Timeout: 25 * time.Second}}
}

func (c *RiotClient) get(ctx context.Context, host, path string, dst any) error {
	u := "https://" + host + ".api.riotgames.com" + path
	for attempt := 0; attempt < 6; attempt++ {
		// A conservative process-wide pace; the server's 429 response remains authoritative.
		if wait := time.Until(c.last.Add(1200 * time.Millisecond)); wait > 0 {
			if err := pause(ctx, wait); err != nil {
				return err
			}
		}
		c.last = time.Now()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return err
		}
		req.Header.Set("X-Riot-Token", c.key)
		resp, err := c.http.Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode == 200 {
			defer resp.Body.Close()
			return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(dst)
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		resp.Body.Close()
		if resp.StatusCode != 429 && (resp.StatusCode < 500 || resp.StatusCode > 599) {
			return fmt.Errorf("Riot HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		if attempt == 5 {
			return fmt.Errorf("Riot HTTP %d after retries: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		wait := time.Duration(1<<attempt) * time.Second
		if v := resp.Header.Get("Retry-After"); v != "" {
			if sec, err := strconv.ParseFloat(v, 64); err == nil && sec >= 0 {
				wait = time.Duration(sec * float64(time.Second))
			} else if at, err := http.ParseTime(v); err == nil {
				wait = time.Until(at)
				if wait < 0 {
					wait = 0
				}
			}
		}
		fmt.Printf("[RATE LIMIT/RETRY] HTTP %d; waiting %s\n", resp.StatusCode, wait.Round(time.Second))
		if err := pause(ctx, wait); err != nil {
			return err
		}
	}
	return nil
}

func pause(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Account-V1 and Match-V5 are regional; League-V4 is platform routed.
func (c *RiotClient) Resolve(ctx context.Context, s Seed) (Account, error) {
	var a Account
	err := c.get(ctx, "europe", "/riot/account/v1/accounts/by-riot-id/"+url.PathEscape(s.GameName)+"/"+url.PathEscape(s.TagLine), &a)
	return a, err
}

func (c *RiotClient) Rank(ctx context.Context, puuid string) (LeagueEntry, error) {
	var entries []LeagueEntry
	err := c.get(ctx, "euw1", "/lol/league/v4/entries/by-puuid/"+url.PathEscape(puuid), &entries)
	for _, entry := range entries {
		if entry.QueueType == "RANKED_SOLO_5x5" {
			return entry, err
		}
	}
	return LeagueEntry{}, err
}

func (c *RiotClient) Recent(ctx context.Context, puuid string) ([]string, error) {
	var ids []string
	err := c.get(ctx, "europe", "/lol/match/v5/matches/by-puuid/"+url.PathEscape(puuid)+"/ids?queue=420&start=0&count=20", &ids)
	return ids, err
}

func (c *RiotClient) Match(ctx context.Context, id string) (Match, error) {
	var m Match
	err := c.get(ctx, "europe", "/lol/match/v5/matches/"+url.PathEscape(id), &m)
	return m, err
}
