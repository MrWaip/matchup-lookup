package main

import (
	"context"
	"fmt"
	"strings"
)

func position(p Participant) string {
	a, b := strings.ToUpper(p.TeamPosition), strings.ToUpper(p.IndividualPosition)
	if a != "" && b != "" && a != b {
		return "CONFLICT"
	}
	if a != "" {
		return a
	}
	if b != "" {
		return b
	}
	lane := strings.ToUpper(p.Lane)
	if lane == "MID" {
		lane = "MIDDLE"
	}
	switch lane {
	case "TOP", "JUNGLE", "MIDDLE", "BOTTOM":
		return lane + "_LANE_FALLBACK"
	}
	return "UNKNOWN"
}

func opponent(m Match, f Participant) (*Participant, string) {
	playerPosition := position(f)
	role := strings.TrimSuffix(playerPosition, "_LANE_FALLBACK")
	switch role {
	case "TOP", "JUNGLE", "MIDDLE", "BOTTOM", "UTILITY":
	default:
		return nil, "player_position_unknown"
	}
	var primary, fallback []Participant
	for _, candidate := range m.Info.Participants {
		if candidate.TeamID == f.TeamID {
			continue
		}
		switch position(candidate) {
		case role:
			primary = append(primary, candidate)
		case role + "_LANE_FALLBACK":
			fallback = append(fallback, candidate)
		}
	}
	if len(primary) == 1 {
		if strings.HasSuffix(playerPosition, "_LANE_FALLBACK") {
			return &primary[0], "lane_fallback"
		}
		return &primary[0], "confirmed"
	}
	if len(primary) == 0 && len(fallback) == 1 {
		return &fallback[0], "lane_fallback"
	}
	return nil, "ambiguous"
}

type RiotAPI interface {
	Resolve(context.Context, Seed) (Account, error)
	Rank(context.Context, string) (LeagueEntry, error)
	Recent(context.Context, string) ([]string, error)
	Match(context.Context, string) (Match, error)
}

func ImportSeeds(ctx context.Context, store *Store, api RiotAPI, seeds []Seed) error {
	failures := 0
	for i, seed := range seeds {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		_, found, err := store.PlayerBySeed(seed)
		if err != nil {
			return err
		}
		if found {
			fmt.Printf("[SKIP %d/%d] %s#%s already imported\n", i+1, len(seeds), seed.GameName, seed.TagLine)
			continue
		}
		fmt.Printf("[IMPORT %d/%d] %s#%s\n", i+1, len(seeds), seed.GameName, seed.TagLine)
		a, err := api.Resolve(ctx, seed)
		if err != nil {
			fmt.Printf("[ERROR] Resolve: %v\n", err)
			failures++
			continue
		}
		if a.PUUID == "" {
			fmt.Println("[ERROR] Riot returned empty PUUID")
			failures++
			continue
		}
		if err := store.UpsertPlayer(seed, a); err != nil {
			return err
		}
	}
	if failures > 0 {
		return fmt.Errorf("import finished with %d errors; successful players were saved", failures)
	}
	return nil
}

func UpdatePlayers(ctx context.Context, store *Store, api RiotAPI) error {
	players, err := store.ListPlayers()
	if err != nil {
		return err
	}
	if len(players) == 0 {
		return fmt.Errorf("player pool is empty; run import first")
	}
	failures := 0
	for i, player := range players {
		fmt.Printf("[PLAYER %d/%d] %s#%s\n", i+1, len(players), player.GameName, player.TagLine)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		rank, err := api.Rank(ctx, player.PUUID)
		if err != nil {
			fmt.Printf("[WARN] Rank: %v\n", err)
			failures++
		} else if err := store.UpdateRank(player.PUUID, rank); err != nil {
			return err
		}
		player, err = store.LoadPlayer(player.PUUID)
		if err != nil {
			return err
		}
		ids, err := api.Recent(ctx, player.PUUID)
		if err != nil {
			fmt.Printf("[ERROR] Match list: %v\n", err)
			failures++
			continue
		}
		fmt.Printf("[MATCHES] %d recent Solo/Duo matches\n", len(ids))
		for j, id := range ids {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			already, err := store.GameExists(id, player.PUUID)
			if err != nil {
				return err
			}
			if already {
				fmt.Printf("[SKIP %d/%d] %s already stored\n", j+1, len(ids), id)
				continue
			}
			m, cached, err := store.CachedMatch(id)
			if err != nil {
				return err
			}
			if !cached {
				m, err = api.Match(ctx, id)
				if err != nil {
					fmt.Printf("[ERROR] %s: %v\n", id, err)
					failures++
					continue
				}
				if err := store.SaveMatch(m); err != nil {
					return err
				}
			} else {
				fmt.Printf("[CACHE] %s\n", id)
			}
			if m.Info.QueueID != 420 {
				if err := store.MarkChecked(id, player.PUUID); err != nil {
					return err
				}
				continue
			}
			var f *Participant
			for k := range m.Info.Participants {
				if m.Info.Participants[k].PUUID == player.PUUID {
					f = &m.Info.Participants[k]
					break
				}
			}
			if f == nil {
				fmt.Printf("[WARN] %s: tracked player absent\n", id)
				failures++
				continue
			}
			o, status := opponent(m, *f)
			if err := store.SaveTrackedGame(m, player, *f, o, status); err != nil {
				return err
			}
			if err := store.MarkChecked(id, player.PUUID); err != nil {
				return err
			}
			if o != nil {
				result := "LOSS"
				if f.Win {
					result = "WIN"
				}
				fmt.Printf("[STORE] %s vs %s - %s (%s)\n", f.ChampionName, o.ChampionName, result, status)
			} else {
				fmt.Printf("[STORE] %s - opponent uncertain (%s)\n", f.ChampionName, status)
			}
		}
	}
	if failures > 0 {
		return fmt.Errorf("collection finished with %d API/data errors; successful data was saved", failures)
	}
	return nil
}
