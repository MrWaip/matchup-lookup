package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"matchup-lookup/internal/store"
)

type Champion struct{ ID, Name string }

var catalogHTTPClient = &http.Client{Timeout: 15 * time.Second}

func (s *Store) Champions() ([]Champion, error) {
	rows, err := s.q.ListChampions(context.Background())
	if err != nil {
		return nil, err
	}
	result := make([]Champion, 0, len(rows))
	for _, r := range rows {
		result = append(result, Champion{ID: r.ID, Name: r.DisplayName})
	}
	return result, nil
}

func (s *Store) catalogAge() (time.Duration, error) {
	stamp, found, err := s.setting("champion_catalog_at")
	if err != nil {
		return 0, err
	}
	sec, parseErr := strconv.ParseInt(stamp, 10, 64)
	if !found || parseErr != nil {
		return 365 * 24 * time.Hour, nil
	}
	return time.Since(time.Unix(sec, 0)), nil
}

func (s *Store) SaveChampions(champions []Champion) error {
	ctx := context.Background()
	return s.withTx(func(q *store.Queries) error {
		for _, c := range champions {
			if c.ID == "" || c.Name == "" {
				continue
			}
			if err := q.UpsertChampion(ctx, store.UpsertChampionParams{ID: c.ID, DisplayName: c.Name}); err != nil {
				return err
			}
		}
		return q.SetSetting(ctx, store.SetSettingParams{Key: "champion_catalog_at", Value: strconv.FormatInt(time.Now().Unix(), 10)})
	})
}

func EnsureChampions(ctx context.Context, s *Store) ([]Champion, error) {
	cached, err := s.Champions()
	if err != nil {
		return nil, err
	}
	age, err := s.catalogAge()
	if err != nil {
		return nil, err
	}
	if len(cached) > 0 && age < 7*24*time.Hour {
		return cached, nil
	}
	fresh, err := fetchChampions(ctx)
	if err != nil {
		if len(cached) > 0 {
			return cached, nil
		}
		return nil, err
	}
	if err := s.SaveChampions(fresh); err != nil {
		return nil, err
	}
	return s.Champions()
}

func fetchChampions(ctx context.Context) ([]Champion, error) {
	var versions []string
	if err := catalogGET(ctx, "https://ddragon.leagueoflegends.com/api/versions.json", &versions); err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("Data Dragon returned no versions")
	}
	var payload struct {
		Data map[string]struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	address := "https://ddragon.leagueoflegends.com/cdn/" + url.PathEscape(versions[0]) + "/data/en_US/champion.json"
	if err := catalogGET(ctx, address, &payload); err != nil {
		return nil, err
	}
	if len(payload.Data) == 0 {
		return nil, fmt.Errorf("Data Dragon returned no champions")
	}
	result := make([]Champion, 0, len(payload.Data))
	for _, c := range payload.Data {
		result = append(result, Champion{ID: c.ID, Name: c.Name})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func catalogGET(ctx context.Context, address string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	resp, err := catalogHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("Data Dragon HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(dst)
}
