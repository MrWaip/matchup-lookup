package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"matchup-lookup/internal/store"
)

type PlayerControl struct {
	Enabled bool
	Tags    []string
}

type PlayerIdentity struct {
	GameName string `json:"gameName"`
	TagLine  string `json:"tagLine"`
	Region   string `json:"region"`
}

func (s *Store) Control(seed Seed) (PlayerControl, error) {
	r, err := s.q.GetPlayerControl(context.Background(), store.GetPlayerControlParams{
		Region: seed.Region, GameName: seed.GameName, TagLine: seed.TagLine,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return PlayerControl{Enabled: true, Tags: []string{}}, nil
	}
	if err != nil {
		return PlayerControl{}, err
	}
	var tags []string
	if err := json.Unmarshal([]byte(r.Tags), &tags); err != nil {
		return PlayerControl{}, err
	}
	return PlayerControl{Enabled: r.Enabled != 0, Tags: tags}, nil
}

func (s *Store) AllPendingSeeds() ([]Seed, error) {
	rows, err := s.q.ListAllPendingSeeds(context.Background())
	if err != nil {
		return nil, err
	}
	seeds := make([]Seed, 0, len(rows))
	for _, r := range rows {
		seeds = append(seeds, Seed{GameName: r.GameName, TagLine: r.TagLine, Region: r.Region, Source: r.Source})
	}
	return seeds, nil
}

func (s *Store) SeedChampions(seed Seed) ([]string, error) {
	rows, err := s.q.ListSeedChampions(context.Background(), store.ListSeedChampionsParams{Region: seed.Region, GameName: seed.GameName, TagLine: seed.TagLine})
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(rows))
	for _, champion := range rows {
		if champion != "" {
			result = append(result, champion)
		}
	}
	return result, nil
}

func cleanLabels(values []string) ([]string, error) {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if len([]rune(value)) > 50 || strings.ContainsAny(value, "\r\n") {
			return nil, fmt.Errorf("labels must be at most 50 characters and one line")
		}
		key := strings.ToLower(value)
		if !seen[key] {
			result = append(result, value)
			seen[key] = true
		}
	}
	if len(result) > 20 {
		return nil, fmt.Errorf("at most 20 labels are allowed")
	}
	return result, nil
}

// SaveManagedPlayer adds a Riot ID offline or changes its collection state and labels.
func (s *Store) SaveManagedPlayer(seed Seed, enabled bool, tags, champions []string) error {
	seed.GameName = strings.TrimSpace(seed.GameName)
	seed.TagLine = strings.TrimSpace(seed.TagLine)
	seed.Region = strings.ToLower(strings.TrimSpace(seed.Region))
	if seed.GameName == "" || seed.TagLine == "" || strings.Contains(seed.GameName, "#") || strings.Contains(seed.TagLine, "#") {
		return fmt.Errorf("enter a Riot ID as game name and tag line")
	}
	if _, err := RegionalRoute(seed.Region); err != nil {
		return err
	}
	var err error
	tags, err = cleanLabels(tags)
	if err != nil {
		return err
	}
	champions, err = cleanLabels(champions)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(tags)
	if err != nil {
		return err
	}
	player, found, err := s.PlayerBySeed(seed)
	if err != nil {
		return err
	}
	ctx := context.Background()
	return s.withTx(func(q *store.Queries) error {
		identity := store.DeleteSeedIdentityParams{Region: seed.Region, GameName: seed.GameName, TagLine: seed.TagLine}
		if err := q.DeleteSeedIdentity(ctx, identity); err != nil {
			return err
		}
		if len(champions) == 0 {
			champions = []string{""}
		}
		for _, champion := range champions {
			if err := q.UpsertSeed(ctx, store.UpsertSeedParams{GameName: seed.GameName, TagLine: seed.TagLine, Region: seed.Region, Champion: champion, Source: "manual", ResolvedPuuid: player.PUUID}); err != nil {
				return err
			}
		}
		if found {
			if err := q.DeletePlayerChampions(ctx, player.PUUID); err != nil {
				return err
			}
			for _, champion := range champions {
				if champion != "" {
					if err := q.AddPlayerChampion(ctx, store.AddPlayerChampionParams{PlayerPuuid: player.PUUID, Champion: champion}); err != nil {
						return err
					}
				}
			}
		}
		flag := int64(0)
		if enabled {
			flag = 1
		}
		return q.SavePlayerControl(ctx, store.SavePlayerControlParams{Region: seed.Region, GameName: seed.GameName, TagLine: seed.TagLine, Enabled: flag, Tags: string(encoded)})
	})
}

// DeleteManagedPlayer removes the ID from tracking and its tracked-game records.
func (s *Store) DeleteManagedPlayer(seed Seed) error {
	return s.DeleteManagedPlayers([]PlayerIdentity{{GameName: seed.GameName, TagLine: seed.TagLine, Region: seed.Region}})
}

// SetPlayersEnabled changes collection state without changing tags or champions.
func (s *Store) SetPlayersEnabled(players []PlayerIdentity, enabled bool) error {
	ctx := context.Background()
	return s.withTx(func(q *store.Queries) error {
		for _, player := range players {
			control, err := q.GetPlayerControl(ctx, store.GetPlayerControlParams{Region: player.Region, GameName: player.GameName, TagLine: player.TagLine})
			if errors.Is(err, sql.ErrNoRows) {
				control.Tags = "[]"
			} else if err != nil {
				return err
			}
			flag := int64(0)
			if enabled {
				flag = 1
			}
			if err := q.SavePlayerControl(ctx, store.SavePlayerControlParams{Region: player.Region, GameName: player.GameName, TagLine: player.TagLine, Enabled: flag, Tags: control.Tags}); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteManagedPlayers removes selected IDs and their tracked-game records atomically.
func (s *Store) DeleteManagedPlayers(players []PlayerIdentity) error {
	ctx := context.Background()
	return s.withTx(func(q *store.Queries) error {
		for _, player := range players {
			identity := store.DeleteSeedIdentityParams{Region: player.Region, GameName: player.GameName, TagLine: player.TagLine}
			if err := q.DeleteSeedIdentity(ctx, identity); err != nil {
				return err
			}
			if err := q.DeletePlayerControl(ctx, store.DeletePlayerControlParams(identity)); err != nil {
				return err
			}
			r, err := q.FindPlayerByRiotID(ctx, store.FindPlayerByRiotIDParams{GameName: player.GameName, TagLine: player.TagLine, Region: player.Region})
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return err
			}
			if err := q.DeletePlayerGames(ctx, r.Puuid); err != nil {
				return err
			}
			if err := q.DeletePlayerChecks(ctx, r.Puuid); err != nil {
				return err
			}
			if err := q.DeletePlayerChampions(ctx, r.Puuid); err != nil {
				return err
			}
			if err := q.DeletePlayer(ctx, r.Puuid); err != nil {
				return err
			}
		}
		return nil
	})
}
