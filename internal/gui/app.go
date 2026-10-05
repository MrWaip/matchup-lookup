// Package gui is the desktop interface: a Wails window whose frontend
// (cmd/matchup-gui/frontend) calls the exported methods of App.
package gui

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"matchup-lookup/internal/api"
	"matchup-lookup/internal/core"
)

// App is bound to the frontend as window.go.gui.App. Wails calls its methods
// concurrently; mu guards store, which ImportDatabase replaces.
type App struct {
	ctx        context.Context
	path       string
	mu         sync.RWMutex
	store      *core.Store
	background *core.BackgroundUpdate

	iconsMu    sync.Mutex
	icons      *core.Icons // nil until Data Dragon answers
	iconsTried time.Time
}

// Run opens the window and blocks until it is closed.
func Run(path string, assets fs.FS) error {
	store, err := core.OpenStore(path)
	if err != nil {
		return err
	}
	app := &App{path: path, store: store, background: core.NewBackgroundUpdate(path)}
	return wails.Run(&options.App{
		Title:            "MatchupFinder.gg",
		Width:            1280,
		Height:           800,
		MinWidth:         900,
		MinHeight:        560,
		BackgroundColour: &options.RGBA{R: 14, G: 17, B: 23, A: 255},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        func(ctx context.Context) { app.ctx = ctx },
		OnShutdown:       app.shutdown,
		Bind:             []any{app},
	})
}

func (a *App) shutdown(context.Context) {
	a.background.Stop()
	a.mu.Lock()
	defer a.mu.Unlock()
	a.store.Close()
}

// withStore runs fn while no database import can swap the store.
func withStore[T any](a *App, fn func(*core.Store) (T, error)) (T, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return fn(a.store)
}

type Server struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Players int    `json:"players"`
}

// Key describes a Riot API key without exposing its value.
type Key struct {
	ID        int64  `json:"id"`
	Label     string `json:"label"`
	Masked    string `json:"masked"`
	FromEnv   bool   `json:"fromEnv"`
	Rejected  bool   `json:"rejected"`
	Rejection string `json:"rejection"`
}

type Overview struct {
	Version       string   `json:"version"`
	DBPath        string   `json:"dbPath"`
	Keys          []Key    `json:"keys"`
	HasUsableKey  bool     `json:"hasUsableKey"`
	Players       int      `json:"players"`
	Pending       int      `json:"pending"`
	Servers       []Server `json:"servers"`
	PlayersSource string   `json:"playersSource"`
}

func (a *App) Overview() (Overview, error) {
	return withStore(a, func(s *core.Store) (Overview, error) {
		_ = core.RefreshLivePatch(a.ctx, s) // offline: search uses the last known patch
		a.loadIcons(s)
		o := Overview{Version: core.Version(), DBPath: a.path, PlayersSource: core.DefaultPlayersSource}
		keys, err := s.RiotKeys()
		if err != nil {
			return o, err
		}
		o.Keys = make([]Key, len(keys))
		for i, k := range keys {
			o.Keys[i] = Key{ID: k.ID, Label: k.Label, Masked: k.Masked(), FromEnv: k.FromEnv,
				Rejected: !k.Usable(), Rejection: k.Rejection}
			o.HasUsableKey = o.HasUsableKey || k.Usable()
		}
		players, err := s.ListPlayers()
		if err != nil {
			return o, err
		}
		pending, err := s.PendingSeeds()
		if err != nil {
			return o, err
		}
		o.Players, o.Pending = len(players), len(pending)
		servers, err := s.Servers()
		if err != nil {
			return o, err
		}
		o.Servers = make([]Server, len(servers))
		for i, server := range servers {
			o.Servers[i] = Server{ID: server.Platform, Label: server.Label, Players: server.Players}
		}
		return o, nil
	})
}

// loadIcons fetches Data Dragon icon tables once, retrying a failed attempt
// after a few minutes; until then results simply carry no icon URLs.
func (a *App) loadIcons(s *core.Store) {
	a.iconsMu.Lock()
	defer a.iconsMu.Unlock()
	if a.icons != nil || time.Since(a.iconsTried) < 5*time.Minute {
		return
	}
	a.iconsTried = time.Now()
	if icons, err := core.LoadIcons(a.ctx, s); err == nil {
		a.icons = icons
	}
}

func (a *App) currentIcons() *core.Icons {
	a.iconsMu.Lock()
	defer a.iconsMu.Unlock()
	return a.icons
}

// Filters mirrors core.Filters with JSON names for the frontend; core.Filters
// itself is persisted with Go field names.
type Filters struct {
	Region     string `json:"region"`
	Champion   string `json:"champion"`
	Opponent   string `json:"opponent"`
	Result     string `json:"result"`
	KDACompare string `json:"kda"`
	Rank       string `json:"rank"`
	MinMinutes int    `json:"minMinutes"`
	Player     string `json:"player"`
}

func (a *App) LastFilters() (Filters, error) {
	return withStore(a, func(s *core.Store) (Filters, error) {
		f, _, err := s.LastFilters()
		return Filters{Region: f.Region, Champion: f.Champion, Opponent: f.Opponent, Result: f.Result,
			KDACompare: f.KDACompare, Rank: f.Rank, MinMinutes: f.MinMinutes, Player: f.Player}, err
	})
}

type Game struct {
	MatchID        string   `json:"matchId"`
	ChampionIcon   string   `json:"championIcon"`
	OpponentIcon   string   `json:"opponentIcon"`
	SpellIcons     []string `json:"spellIcons"`
	KeystoneIcon   string   `json:"keystoneIcon"`
	SecondaryIcons []string `json:"secondaryIcons"`
	Date           string   `json:"date"`
	Region         string   `json:"region"`
	Patch          string   `json:"patch"`
	Win            bool     `json:"win"`
	PlayerID       string   `json:"playerId"`
	Champion       string   `json:"champion"`
	Rank           string   `json:"rank"`
	KDA            string   `json:"kda"`
	CS             int      `json:"cs"`
	Opponent       string   `json:"opponent"`
	OpponentID     string   `json:"opponentId"`
	OpponentKDA    string   `json:"opponentKda"`
	Status         string   `json:"status"`
	Spells         string   `json:"spells"`
	Keystone       string   `json:"keystone"`
	SecondaryRunes string   `json:"secondaryRunes"`
}

type SearchResult struct {
	Games    []Game `json:"games"`
	Patch    string `json:"patch"`
	Players  int    `json:"players"`
	Stored   int    `json:"stored"`
	Matching int    `json:"matching"`
	Wins     int    `json:"wins"`
}

const searchLimit = 500

// Search runs a search and remembers its filters for the next session.
func (a *App) Search(f Filters) (SearchResult, error) {
	return withStore(a, func(s *core.Store) (SearchResult, error) {
		filters := core.Filters{Region: f.Region, Champion: f.Champion, Opponent: f.Opponent, Result: f.Result,
			KDACompare: f.KDACompare, Rank: f.Rank, MinMinutes: f.MinMinutes, Player: f.Player,
			Limit: searchLimit}
		found, err := core.Search(s, filters)
		if err != nil {
			return SearchResult{}, err
		}
		if err := s.SaveFilters(filters); err != nil {
			return SearchResult{}, err
		}
		icons := a.currentIcons()
		games := make([]Game, len(found.Results))
		for i, r := range found.Results {
			games[i] = Game{MatchID: r.MatchID,
				ChampionIcon: icons.Champion(r.Champion), OpponentIcon: icons.Champion(r.Opponent),
				SpellIcons:     []string{icons.Spell(r.SpellIDs[0]), icons.Spell(r.SpellIDs[1])},
				KeystoneIcon:   icons.Rune(r.KeystoneID),
				SecondaryIcons: []string{icons.Rune(r.SecondaryIDs[0]), icons.Rune(r.SecondaryIDs[1])}, Date: r.Date.Format(time.RFC3339), Region: r.Region, Patch: r.Patch,
				Win: r.Win, PlayerID: r.PlayerID, Champion: r.Champion, Rank: r.Rank, KDA: r.KDA, CS: r.CS,
				Opponent: r.Opponent, OpponentID: r.OpponentID, OpponentKDA: r.OpponentKDA, Status: r.Status,
				Spells: r.Spells, Keystone: r.Keystone, SecondaryRunes: r.SecondaryRunes}
		}
		patch := ""
		if !found.Patch.IsZero() {
			patch = found.Patch.String()
		}
		return SearchResult{Games: games, Patch: patch, Players: found.Players, Stored: found.Stored,
			Matching: found.Matching, Wins: found.Wins}, nil
	})
}

type Champion struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// Champions returns up to limit champions fuzzily matching query.
func (a *App) Champions(query string, limit int) ([]Champion, error) {
	return withStore(a, func(s *core.Store) ([]Champion, error) {
		all, err := core.EnsureChampions(a.ctx, s)
		if err != nil {
			return nil, err
		}
		ranked := core.RankChampions(all, query)
		icons := a.currentIcons()
		result := make([]Champion, 0, min(limit, len(ranked)))
		for _, c := range ranked[:min(limit, len(ranked))] {
			result = append(result, Champion{ID: c.ID, Name: c.Name, Icon: icons.Champion(c.ID)})
		}
		return result, nil
	})
}

type UpdateStatus struct {
	Running      bool   `json:"running"`
	Cancelled    bool   `json:"cancelled"`
	Finished     bool   `json:"finished"`
	Error        string `json:"error"`
	KeyRejected  bool   `json:"keyRejected"`
	Percent      int    `json:"percent"`
	Resolved     int    `json:"resolved"`
	ResolveTotal int    `json:"resolveTotal"`
	Players      int    `json:"players"`
	PlayerTotal  int    `json:"playerTotal"`
	Matches      int    `json:"matches"`
	MatchTotal   int    `json:"matchTotal"`
	WaitSeconds  int    `json:"waitSeconds"`
	WaitReason   string `json:"waitReason"`
}

func (a *App) UpdateStatus() UpdateStatus {
	s := a.background.Snapshot()
	p := s.Progress
	status := UpdateStatus{Running: s.Running, Cancelled: s.Cancelled, Finished: !s.Finished.IsZero(),
		Percent: p.Percent, Resolved: p.Resolved, ResolveTotal: p.ResolveTotal, Players: p.Players,
		PlayerTotal: p.PlayerTotal, Matches: p.Matches, MatchTotal: p.MatchTotal}
	if s.Err != nil && !s.Cancelled {
		status.Error = s.Err.Error()
		status.KeyRejected = errors.Is(s.Err, core.ErrKeyRejected)
	}
	if wait := time.Until(s.WaitUntil); s.Running && wait > 0 {
		status.WaitSeconds = int(wait.Round(time.Second).Seconds())
		status.WaitReason = s.WaitReason
	}
	return status
}

func (a *App) StartUpdate() error {
	keys, err := withStore(a, (*core.Store).UsableRiotKeys)
	if err != nil {
		return err
	}
	return a.background.Start(api.NewRiotClient(keys, func(key core.APIKey, reason string) {
		// Best effort: if this write fails, the key is rejected again next run.
		_, _ = withStore(a, func(s *core.Store) (struct{}, error) { return struct{}{}, s.MarkKeyRejected(key, reason) })
	}))
}

func (a *App) CancelUpdate() { a.background.Stop() }

// AddRiotKey checks a key with Riot and saves it after the existing ones.
// A key Riot rejects is not saved. If Riot cannot be reached, the key is
// saved anyway and the returned note says it was not checked.
func (a *App) AddRiotKey(label, value string) (string, error) {
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()
	checkErr := api.CheckKey(ctx, strings.TrimSpace(value), "euw1")
	if errors.Is(checkErr, core.ErrKeyRejected) {
		return "", fmt.Errorf("Riot rejected this key: it is expired or mistyped")
	}
	_, err := withStore(a, func(s *core.Store) (struct{}, error) { return struct{}{}, s.AddRiotKey(label, value) })
	if err != nil || checkErr == nil {
		return "", err
	}
	return "Saved without checking: " + checkErr.Error(), nil
}

func (a *App) RemoveRiotKey(id int64) error {
	_, err := withStore(a, func(s *core.Store) (struct{}, error) { return struct{}{}, s.RemoveRiotKey(id) })
	return err
}

// ImportPlayers queues Riot IDs from a JSON/CSV URL or path and returns their count.
func (a *App) ImportPlayers(source string) (int, error) {
	seeds, err := core.LoadSeeds(strings.TrimSpace(source))
	if err != nil {
		return 0, err
	}
	return withStore(a, func(s *core.Store) (int, error) {
		return len(seeds), core.ImportSeeds(s, seeds, nil)
	})
}

type PlayerRow struct {
	RiotID    string   `json:"riotId"`
	Region    string   `json:"region"`
	Server    string   `json:"server"`
	Rank      string   `json:"rank"`
	Champions []string `json:"champions"`
	Tags      []string `json:"tags"`
	Enabled   bool     `json:"enabled"`
	Pending   bool     `json:"pending"`
}

func (a *App) Players() ([]PlayerRow, error) {
	return withStore(a, func(s *core.Store) ([]PlayerRow, error) {
		players, err := s.ListPlayers()
		if err != nil {
			return nil, err
		}
		pending, err := s.AllPendingSeeds()
		if err != nil {
			return nil, err
		}
		rows := make([]PlayerRow, 0, len(players)+len(pending))
		for _, p := range players {
			seed := core.Seed{GameName: p.GameName, TagLine: p.TagLine, Region: p.Region}
			control, err := s.Control(seed)
			if err != nil {
				return nil, err
			}
			champions, err := s.PlayerChampions(p.PUUID)
			if err != nil {
				return nil, err
			}
			rank := strings.TrimSpace(p.Tier + " " + p.Division)
			if rank == "" {
				rank = "Unranked"
			}
			rows = append(rows, PlayerRow{RiotID: p.GameName + "#" + p.TagLine, Region: p.Region, Server: core.PlatformLabel(p.Region),
				Rank: rank, Champions: champions, Tags: control.Tags, Enabled: control.Enabled})
		}
		for _, seed := range pending {
			control, err := s.Control(seed)
			if err != nil {
				return nil, err
			}
			champions, err := s.SeedChampions(seed)
			if err != nil {
				return nil, err
			}
			rows = append(rows, PlayerRow{RiotID: seed.GameName + "#" + seed.TagLine, Region: seed.Region, Server: core.PlatformLabel(seed.Region),
				Champions: champions, Tags: control.Tags, Enabled: control.Enabled, Pending: true})
		}
		return rows, nil
	})
}

func (a *App) SavePlayer(gameName, tagLine, region string, enabled bool, tags, champions []string) error {
	_, err := withStore(a, func(s *core.Store) (struct{}, error) {
		return struct{}{}, s.SaveManagedPlayer(core.Seed{GameName: gameName, TagLine: tagLine, Region: region}, enabled, tags, champions)
	})
	return err
}

func (a *App) DeletePlayer(gameName, tagLine, region string) error {
	_, err := withStore(a, func(s *core.Store) (struct{}, error) {
		return struct{}{}, s.DeleteManagedPlayer(core.Seed{GameName: gameName, TagLine: tagLine, Region: region})
	})
	return err
}

func (a *App) SetPlayersEnabled(players []core.PlayerIdentity, enabled bool) error {
	_, err := withStore(a, func(s *core.Store) (struct{}, error) {
		return struct{}{}, s.SetPlayersEnabled(players, enabled)
	})
	return err
}

func (a *App) DeletePlayers(players []core.PlayerIdentity) error {
	_, err := withStore(a, func(s *core.Store) (struct{}, error) {
		return struct{}{}, s.DeleteManagedPlayers(players)
	})
	return err
}

// OpenReplay asks the running League Client to download and play a replay.
func (a *App) OpenReplay(matchID string) error {
	client, err := api.ConnectLeagueClient()
	if err != nil {
		return err
	}
	return api.OpenReplay(a.ctx, client, matchID)
}

var dbFilter = []runtime.FileFilter{{DisplayName: "SQLite database (*.db)", Pattern: "*.db"}}

// ExportDatabase asks where to save a snapshot and returns the chosen path,
// or "" if the dialog was cancelled.
func (a *App) ExportDatabase() (string, error) {
	output, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export database",
		DefaultFilename: "matchupfinder-" + time.Now().Format("20060102-150405") + ".db",
		Filters:         dbFilter,
	})
	if err != nil || output == "" {
		return "", err
	}
	return withStore(a, func(s *core.Store) (string, error) {
		return output, core.ExportDatabase(s, output)
	})
}

// ImportDatabase replaces the local database with a chosen snapshot and
// returns the backup path of the previous database ("" if there was none or
// the dialog was cancelled).
func (a *App) ImportDatabase() (string, error) {
	if a.background.Snapshot().Running {
		return "", fmt.Errorf("cancel or finish the update before importing a database")
	}
	source, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "Import database", Filters: dbFilter})
	if err != nil || source == "" {
		return "", err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.store.Close(); err != nil {
		return "", err
	}
	backup, importErr := core.ImportDatabase(a.path, source)
	store, err := core.OpenStore(a.path)
	if err != nil {
		return "", err
	}
	a.store = store
	return backup, importErr
}
