package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
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
	Rank(context.Context, string, string) (LeagueEntry, error)
	Recent(context.Context, string, string) ([]string, error)
	Match(context.Context, string, string) (Match, error)
}

func ImportSeeds(store *Store, seeds []Seed) error {
	if len(seeds) == 0 {
		return fmt.Errorf("player list is empty")
	}
	for i, seed := range seeds {
		if err := store.QueueSeed(seed); err != nil {
			return err
		}
		if (i+1)%50 == 0 || i+1 == len(seeds) {
			fmt.Printf("[IMPORT] Saved %d/%d IDs locally\n", i+1, len(seeds))
		}
	}
	fmt.Println("[IMPORT] Ready. Run update to resolve Riot IDs and fetch matches.")
	return nil
}

func UpdatePlayers(ctx context.Context, store *Store, api RiotAPI) error {
	return updatePlayers(ctx, store, api, nil)
}

func updatePlayers(ctx context.Context, store *Store, api RiotAPI, onProgress func(updateSnapshot)) error {
	pending, err := store.PendingSeeds()
	if err != nil {
		return err
	}
	existing, err := store.ListPlayers()
	if err != nil {
		return err
	}
	progress := newUpdateProgress(len(pending), len(existing)+len(pending))
	progress.onChange = onProgress
	progress.render()
	defer progress.close()
	failures, err := resolvePending(ctx, store, api, pending, progress)
	if err != nil {
		return err
	}
	players, err := store.ListPlayers()
	if err != nil {
		return err
	}
	if len(players) == 0 {
		return fmt.Errorf("player pool is empty; run import first")
	}
	progress.playerTotal = len(players)
	progress.render()
	for _, player := range players {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		rank, err := api.Rank(ctx, player.Region, player.PUUID)
		if err != nil {
			progress.logf("\r\x1b[2K[WARN] Rank %s#%s: %v\n", player.GameName, player.TagLine, err)
			failures++
		} else if err := store.UpdateRank(player.PUUID, rank); err != nil {
			return err
		}
		player, err = store.LoadPlayer(player.PUUID)
		if err != nil {
			return err
		}
		ids, err := api.Recent(ctx, player.Region, player.PUUID)
		if err != nil {
			progress.logf("\r\x1b[2K[ERROR] Match list %s#%s: %v\n", player.GameName, player.TagLine, err)
			failures++
			progress.players++
			progress.render()
			continue
		}
		progress.matchTotal += len(ids)
		progress.currentTotal = len(ids)
		progress.current = 0
		progress.render()
		jobs := make(chan string, len(ids))
		for _, id := range ids {
			jobs <- id
		}
		close(jobs)
		outcomes := make(chan matchOutcome, len(ids))
		workers := min(4, len(ids))
		var wg sync.WaitGroup
		for range workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for id := range jobs {
					apiFailure, err := collectMatch(ctx, store, api, player, id, progress)
					outcomes <- matchOutcome{apiFailure, err}
					if ctx.Err() != nil {
						return
					}
				}
			}()
		}
		go func() { wg.Wait(); close(outcomes) }()
		var fatal error
		for outcome := range outcomes {
			progress.current++
			progress.matches++
			progress.render()
			if outcome.err != nil && fatal == nil {
				fatal = outcome.err
			}
			if outcome.apiFailure {
				failures++
			}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if fatal != nil {
			return fatal
		}
		progress.players++
		progress.current = 0
		progress.currentTotal = 0
		progress.render()
	}
	if failures > 0 {
		return fmt.Errorf("collection finished with %d API/data errors; successful data was saved", failures)
	}
	return nil
}

func resolvePending(ctx context.Context, store *Store, api RiotAPI, pending []Seed, progress *updateProgress) (int, error) {
	jobs := make(chan Seed, len(pending))
	for _, seed := range pending {
		jobs <- seed
	}
	close(jobs)
	outcomes := make(chan matchOutcome, len(pending))
	var wg sync.WaitGroup
	for range min(4, len(pending)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for seed := range jobs {
				if ctx.Err() != nil {
					return
				}
				account, err := api.Resolve(ctx, seed)
				if err != nil {
					progress.logf("\r\x1b[2K[ERROR] Resolve %s#%s: %v\n", seed.GameName, seed.TagLine, err)
					outcomes <- matchOutcome{apiFailure: true}
					continue
				}
				if account.PUUID == "" {
					progress.logf("\r\x1b[2K[ERROR] %s#%s: empty PUUID\n", seed.GameName, seed.TagLine)
					outcomes <- matchOutcome{apiFailure: true}
					continue
				}
				outcomes <- matchOutcome{err: store.ResolveSeed(seed, account)}
			}
		}()
	}
	go func() { wg.Wait(); close(outcomes) }()
	failures := 0
	var fatal error
	for outcome := range outcomes {
		progress.resolved++
		progress.render()
		if outcome.err != nil && fatal == nil {
			fatal = outcome.err
		}
		if outcome.apiFailure {
			failures++
		}
	}
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	if fatal != nil {
		return failures, fatal
	}
	return failures, nil
}

type matchOutcome struct {
	apiFailure bool
	err        error
}

func collectMatch(ctx context.Context, store *Store, api RiotAPI, player Player, id string, progress *updateProgress) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	already, err := store.GameExists(id, player.PUUID)
	if err != nil {
		return false, err
	}
	if already {
		return false, nil
	}
	m, cached, err := store.CachedMatch(id)
	if err != nil {
		return false, err
	}
	if !cached {
		m, err = api.Match(ctx, player.Region, id)
		if err != nil {
			progress.logf("\n[ERROR] %s: %v\n", id, err)
			return true, nil
		}
		if err := store.SaveMatch(m); err != nil {
			return false, err
		}
	}
	if m.Info.QueueID != 420 {
		return false, store.MarkChecked(id, player.PUUID)
	}
	var f *Participant
	for k := range m.Info.Participants {
		if m.Info.Participants[k].PUUID == player.PUUID {
			f = &m.Info.Participants[k]
			break
		}
	}
	if f == nil {
		progress.logf("\n[WARN] %s: tracked player absent\n", id)
		return true, nil
	}
	o, status := opponent(m, *f)
	if err := store.SaveTrackedGame(m, player, *f, o, status); err != nil {
		return false, err
	}
	if err := store.MarkChecked(id, player.PUUID); err != nil {
		return false, err
	}
	if o == nil {
		return false, nil
	}
	return false, nil
}
