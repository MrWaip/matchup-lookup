package api

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

	"matchup-lookup/internal/core"
)

type RiotClient struct {
	key   string
	http  *http.Client
	limit rateLimiter
}

func NewRiotClient(key string) *RiotClient {
	return &RiotClient{key: key, http: &http.Client{Timeout: 25 * time.Second}}
}

// ReportWaits sends rate-limit and Retry-After waits of 2s or more to fn;
// a zero time means the wait is over.
func (c *RiotClient) ReportWaits(fn func(until time.Time, reason string)) {
	c.limit.onWait = fn
}

func (c *RiotClient) get(ctx context.Context, host, path string, dst any) error {
	u := "https://" + host + ".api.riotgames.com" + path
	for attempt := 0; attempt < 6; attempt++ {
		if err := c.limit.wait(ctx); err != nil {
			return err
		}
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
		if resp.StatusCode == 429 {
			c.limit.block(wait)
		}
		if err := c.limit.pauseWithNotice(ctx, wait, fmt.Sprintf("HTTP %d Retry-After", resp.StatusCode)); err != nil {
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
func (c *RiotClient) Resolve(ctx context.Context, s core.Seed) (core.Account, error) {
	var a core.Account
	route, err := core.AccountRoute(s.Region)
	if err != nil {
		return a, err
	}
	err = c.get(ctx, route, "/riot/account/v1/accounts/by-riot-id/"+url.PathEscape(s.GameName)+"/"+url.PathEscape(s.TagLine), &a)
	return a, err
}

func (c *RiotClient) Rank(ctx context.Context, platform, puuid string) (core.LeagueEntry, error) {
	var entries []core.LeagueEntry
	if _, err := core.RegionalRoute(platform); err != nil {
		return core.LeagueEntry{}, err
	}
	err := c.get(ctx, platform, "/lol/league/v4/entries/by-puuid/"+url.PathEscape(puuid), &entries)
	for _, entry := range entries {
		if entry.QueueType == "RANKED_SOLO_5x5" {
			return entry, err
		}
	}
	return core.LeagueEntry{}, err
}

func (c *RiotClient) Recent(ctx context.Context, platform, puuid string) ([]string, error) {
	var ids []string
	route, err := core.RegionalRoute(platform)
	if err != nil {
		return nil, err
	}
	err = c.get(ctx, route, "/lol/match/v5/matches/by-puuid/"+url.PathEscape(puuid)+"/ids?queue=420&start=0&count=20", &ids)
	return ids, err
}

func (c *RiotClient) Match(ctx context.Context, platform, id string) (core.Match, error) {
	var m core.Match
	route, err := core.RegionalRoute(platform)
	if err != nil {
		return m, err
	}
	var raw json.RawMessage
	if err := c.get(ctx, route, "/lol/match/v5/matches/"+url.PathEscape(id), &raw); err != nil {
		return m, err
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, err
	}
	m.Raw = raw
	return m, nil
}
