package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"matchup-lookup/internal/store"
)

// Store is the application's repository: it maps domain types to the
// generated queries in internal/store.
type Store struct {
	db *sql.DB
	q  *store.Queries
}

// OpenStore opens the database at path and migrates it to the current schema.
func OpenStore(path string) (*Store, error) {
	db, err := store.Open(path)
	if err != nil {
		return nil, err
	}
	return &Store{db: db, q: store.New(db)}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// withTx runs fn in one transaction and commits only if fn succeeds.
func (s *Store) withTx(fn func(q *store.Queries) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(s.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) setting(key string) (string, bool, error) {
	value, err := s.q.GetSetting(context.Background(), key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return value, err == nil, err
}

func (s *Store) setSetting(key, value string) error {
	return s.q.SetSetting(context.Background(), store.SetSettingParams{Key: key, Value: value})
}

func (s *Store) SaveFilters(filters Filters) error {
	data, err := json.Marshal(filters)
	if err != nil {
		return err
	}
	return s.setSetting("last_filters", string(data))
}

func (s *Store) LastFilters() (Filters, bool, error) {
	raw, found, err := s.setting("last_filters")
	if err != nil {
		return Filters{}, false, err
	}
	if !found {
		return Filters{Limit: 100, Result: "any", KDACompare: "any", Rank: "any"}, false, nil
	}
	var filters Filters
	if err := json.Unmarshal([]byte(raw), &filters); err != nil {
		return Filters{}, false, err
	}
	if filters.Limit < 1 {
		filters.Limit = 100
	}
	return filters, true, nil
}

func playerFromRow(puuid, gameName, tagLine, region, tier, division string, lp int64) Player {
	return Player{PUUID: puuid, GameName: gameName, TagLine: tagLine, Region: region,
		Tier: tier, Division: division, LP: int(lp)}
}

func (s *Store) PlayerBySeed(seed Seed) (Player, bool, error) {
	r, err := s.q.FindPlayerByRiotID(context.Background(), store.FindPlayerByRiotIDParams{
		GameName: seed.GameName, TagLine: seed.TagLine, Region: seed.Region})
	if errors.Is(err, sql.ErrNoRows) {
		return Player{}, false, nil
	}
	if err != nil {
		return Player{}, false, err
	}
	return playerFromRow(r.Puuid, r.GameName, r.TagLine, r.Region, r.RankTier, r.RankDivision, r.LeaguePoints), true, nil
}

func (s *Store) QueueSeed(seed Seed) error {
	player, found, err := s.PlayerBySeed(seed)
	if err != nil {
		return err
	}
	err = s.q.UpsertSeed(context.Background(), store.UpsertSeedParams{
		GameName: seed.GameName, TagLine: seed.TagLine, Region: seed.Region,
		Champion: seed.Champion, Source: seed.Source, ResolvedPuuid: player.PUUID})
	if err != nil || !found {
		return err
	}
	return s.AddPlayerChampion(player.PUUID, seed.Champion)
}

func (s *Store) PendingSeeds() ([]Seed, error) {
	rows, err := s.q.ListPendingSeeds(context.Background())
	if err != nil {
		return nil, err
	}
	seeds := make([]Seed, 0, len(rows))
	for _, r := range rows {
		seeds = append(seeds, Seed{GameName: r.GameName, TagLine: r.TagLine, Region: r.Region, Source: r.Source})
	}
	return seeds, nil
}

func (s *Store) ResolveSeed(seed Seed, account Account) error {
	if err := s.UpsertPlayer(seed, account); err != nil {
		return err
	}
	ctx := context.Background()
	champions, err := s.q.ListSeedChampions(ctx, store.ListSeedChampionsParams{
		Region: seed.Region, GameName: seed.GameName, TagLine: seed.TagLine})
	if err != nil {
		return err
	}
	for _, champion := range champions {
		if err := s.AddPlayerChampion(account.PUUID, champion); err != nil {
			return err
		}
	}
	return s.q.ResolveSeed(ctx, store.ResolveSeedParams{
		Puuid: account.PUUID, Region: seed.Region, GameName: seed.GameName, TagLine: seed.TagLine})
}

func (s *Store) UpsertPlayer(seed Seed, a Account) error {
	source := seed.Source
	if source == "" {
		source = "seed"
	}
	return s.q.UpsertPlayer(context.Background(), store.UpsertPlayerParams{
		Puuid: a.PUUID, GameName: a.GameName, TagLine: a.TagLine, Region: seed.Region, Source: source})
}

func (s *Store) UpdateRank(puuid string, rank LeagueEntry) error {
	return s.q.UpdatePlayerRank(context.Background(), store.UpdatePlayerRankParams{
		RankTier: rank.Tier, RankDivision: rank.Rank, LeaguePoints: int64(rank.LeaguePoints),
		LastCheckedAt: time.Now().Unix(), Puuid: puuid})
}

func (s *Store) LoadPlayer(puuid string) (Player, error) {
	r, err := s.q.GetPlayer(context.Background(), puuid)
	if err != nil {
		return Player{}, err
	}
	return playerFromRow(r.Puuid, r.GameName, r.TagLine, r.Region, r.RankTier, r.RankDivision, r.LeaguePoints), nil
}

func (s *Store) ListPlayers() ([]Player, error) {
	rows, err := s.q.ListPlayers(context.Background())
	if err != nil {
		return nil, err
	}
	players := make([]Player, 0, len(rows))
	for _, r := range rows {
		players = append(players, playerFromRow(r.Puuid, r.GameName, r.TagLine, r.Region, r.RankTier, r.RankDivision, r.LeaguePoints))
	}
	return players, nil
}

func (s *Store) ListActivePlayers() ([]Player, error) {
	rows, err := s.q.ListActivePlayers(context.Background())
	if err != nil {
		return nil, err
	}
	players := make([]Player, 0, len(rows))
	for _, r := range rows {
		players = append(players, playerFromRow(r.Puuid, r.GameName, r.TagLine, r.Region, r.RankTier, r.RankDivision, r.LeaguePoints))
	}
	return players, nil
}

func (s *Store) AddPlayerChampion(puuid, champion string) error {
	if champion == "" {
		return nil
	}
	return s.q.AddPlayerChampion(context.Background(), store.AddPlayerChampionParams{PlayerPuuid: puuid, Champion: champion})
}

func (s *Store) PlayerChampions(puuid string) ([]string, error) {
	return s.q.ListPlayerChampions(context.Background(), puuid)
}

// CachedMatch decodes the stored Riot response of a match.
func (s *Store) CachedMatch(id string) (Match, bool, error) {
	compressed, err := s.q.GetMatchRaw(context.Background(), id)
	if errors.Is(err, sql.ErrNoRows) {
		return Match{}, false, nil
	}
	if err != nil {
		return Match{}, false, err
	}
	raw, err := store.DecompressRaw(compressed)
	if err != nil {
		return Match{}, false, err
	}
	var m Match
	if err := json.Unmarshal(raw, &m); err != nil {
		return Match{}, false, err
	}
	m.Raw = raw
	return m, true, nil
}

// SaveMatch stores the match with its compressed Riot response and refreshes
// the loadout of participants that already have rows.
func (s *Store) SaveMatch(m Match) error {
	if m.Metadata.MatchID == "" {
		return fmt.Errorf("match has no ID")
	}
	raw := m.Raw
	if len(raw) == 0 {
		var err error
		if raw, err = json.Marshal(m); err != nil {
			return err
		}
	}
	compressed, err := store.CompressRaw(raw)
	if err != nil {
		return err
	}
	major, minor := store.ParsePatch(m.Info.GameVersion)
	ctx := context.Background()
	return s.withTx(func(q *store.Queries) error {
		err := q.UpsertMatch(ctx, store.UpsertMatchParams{
			MatchID: m.Metadata.MatchID, Platform: m.Info.PlatformID, GameVersion: m.Info.GameVersion,
			PatchMajor: major, PatchMinor: minor, GameCreation: m.Info.GameCreation,
			GameDuration: m.Info.GameDuration, QueueID: int64(m.Info.QueueID),
			FetchedAt: time.Now().Unix(), RawGz: compressed})
		if err != nil {
			return err
		}
		for _, p := range m.Info.Participants {
			l := loadoutOf(p)
			err := q.UpdateParticipantLoadout(ctx, store.UpdateParticipantLoadoutParams{
				Summoner1ID: l.summoner1, Summoner2ID: l.summoner2, Keystone: l.keystone,
				SecondaryRune1: l.secondary1, SecondaryRune2: l.secondary2,
				MatchID: m.Metadata.MatchID, Puuid: p.PUUID})
			if err != nil {
				return err
			}
		}
		return nil
	})
}

type loadout struct {
	summoner1, summoner2, keystone, secondary1, secondary2 sql.NullInt64
}

func loadoutOf(p Participant) loadout {
	known := func(v int) sql.NullInt64 { return sql.NullInt64{Int64: int64(v), Valid: true} }
	perk := func(style, selection int) sql.NullInt64 {
		if style < len(p.Perks.Styles) && selection < len(p.Perks.Styles[style].Selections) {
			return known(p.Perks.Styles[style].Selections[selection].Perk)
		}
		return known(0)
	}
	return loadout{known(p.Summoner1ID), known(p.Summoner2ID), perk(0, 0), perk(1, 0), perk(1, 1)}
}

func (s *Store) MatchNeedsLoadoutRefresh(id string) (bool, error) {
	return s.q.MatchNeedsLoadout(context.Background(), id)
}

type cachedMatchRef struct {
	ID, Platform string
}

func (s *Store) MissingLoadoutMatches() ([]cachedMatchRef, error) {
	rows, err := s.q.ListMatchesMissingLoadout(context.Background())
	if err != nil {
		return nil, err
	}
	refs := make([]cachedMatchRef, 0, len(rows))
	for _, r := range rows {
		refs = append(refs, cachedMatchRef{ID: r.MatchID, Platform: r.Platform})
	}
	return refs, nil
}

func (s *Store) GameExists(matchID, puuid string) (bool, error) {
	return s.q.IsChecked(context.Background(), store.IsCheckedParams{MatchID: matchID, PlayerPuuid: puuid})
}

func (s *Store) MarkChecked(matchID, puuid string) error {
	return s.q.MarkChecked(context.Background(), store.MarkCheckedParams{MatchID: matchID, PlayerPuuid: puuid})
}

// SaveTrackedGame records a game from the tracked player's point of view,
// with participant rows for the player and the lane opponent o (if known).
func (s *Store) SaveTrackedGame(m Match, p Player, f Participant, o *Participant, status string) error {
	ctx := context.Background()
	id := m.Metadata.MatchID
	return s.withTx(func(q *store.Queries) error {
		participants := []Participant{f}
		var opp Participant
		if o != nil {
			opp = *o
			participants = append(participants, opp)
		}
		for _, part := range participants {
			l := loadoutOf(part)
			err := q.InsertParticipant(ctx, store.InsertParticipantParams{
				MatchID: id, Puuid: part.PUUID, RiotIDGameName: part.RiotIDGameName, RiotIDTagline: part.RiotIDTagline,
				Champion: part.ChampionName, TeamID: int64(part.TeamID), TeamPosition: part.TeamPosition,
				IndividualPosition: part.IndividualPosition, Lane: part.Lane, Win: part.Win,
				Kills: int64(part.Kills), Deaths: int64(part.Deaths), Assists: int64(part.Assists),
				Cs: int64(part.TotalMinionsKilled + part.NeutralMinionsKilled), Gold: int64(part.GoldEarned),
				Item0: int64(part.Item0), Item1: int64(part.Item1), Item2: int64(part.Item2), Item3: int64(part.Item3),
				Item4: int64(part.Item4), Item5: int64(part.Item5), Item6: int64(part.Item6),
				Summoner1ID: l.summoner1, Summoner2ID: l.summoner2, Keystone: l.keystone,
				SecondaryRune1: l.secondary1, SecondaryRune2: l.secondary2})
			if err != nil {
				return err
			}
		}
		return q.InsertTrackedGame(ctx, store.InsertTrackedGameParams{
			MatchID: id, PlayerPuuid: p.PUUID, OpponentPuuid: opp.PUUID,
			PlayerRankTier: p.Tier, PlayerRankDivision: p.Division,
			PlayerPosition: position(f), OpponentPosition: position(opp), MatchupStatus: status})
	})
}

// PruneOldPatches deletes matches from patches before the newest stored one:
// their replays can no longer be opened. It deliberately ignores the live
// patch from Data Dragon, which can lead a region's rollout by hours.
// It returns the number of matches removed.
func (s *Store) PruneOldPatches() (int64, error) {
	latest, err := s.newestStoredPatch()
	if err != nil || latest.IsZero() {
		return 0, err
	}
	ctx := context.Background()
	var removed int64
	err = s.withTx(func(q *store.Queries) error {
		if err := q.DeleteTrackedGamesBeforePatch(ctx, store.DeleteTrackedGamesBeforePatchParams{
			Major: latest.Major, Minor: latest.Minor}); err != nil {
			return err
		}
		if err := q.DeleteParticipantsBeforePatch(ctx, store.DeleteParticipantsBeforePatchParams{
			Major: latest.Major, Minor: latest.Minor}); err != nil {
			return err
		}
		removed, err = q.DeleteMatchesBeforePatch(ctx, store.DeleteMatchesBeforePatchParams{
			Major: latest.Major, Minor: latest.Minor})
		return err
	})
	return removed, err
}
