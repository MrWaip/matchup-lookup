// Package gui is the desktop interface: a Wails window whose frontend
// (cmd/matchup-gui/frontend) calls the exported methods of App.
package gui

import (
	"context"
	"fmt"
	"io/fs"
	"os"
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
}

// Run opens the window and blocks until it is closed.
func Run(path string, assets fs.FS) error {
	store, err := core.OpenStore(path)
	if err != nil {
		return err
	}
	app := &App{path: path, store: store, background: core.NewBackgroundUpdate(path)}
	return wails.Run(&options.App{
		Title:            "Matchup Lookup",
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

type Overview struct {
	DBPath        string   `json:"dbPath"`
	HasKey        bool     `json:"hasKey"`
	KeyFromEnv    bool     `json:"keyFromEnv"`
	Players       int      `json:"players"`
	Pending       int      `json:"pending"`
	Servers       []Server `json:"servers"`
	PlayersSource string   `json:"playersSource"`
}

func (a *App) Overview() (Overview, error) {
	return withStore(a, func(s *core.Store) (Overview, error) {
		_ = core.RefreshLivePatch(a.ctx, s) // offline: search uses the last known patch
		o := Overview{DBPath: a.path, PlayersSource: core.DefaultPlayersSource,
			KeyFromEnv: strings.TrimSpace(os.Getenv("RIOT_API_KEY")) != ""}
		key, err := core.ConfiguredRiotKey(s)
		if err != nil {
			return o, err
		}
		o.HasKey = key != ""
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
	MatchID        string `json:"matchId"`
	Date           string `json:"date"`
	Region         string `json:"region"`
	Patch          string `json:"patch"`
	Win            bool   `json:"win"`
	PlayerID       string `json:"playerId"`
	Champion       string `json:"champion"`
	Rank           string `json:"rank"`
	KDA            string `json:"kda"`
	CS             int    `json:"cs"`
	Opponent       string `json:"opponent"`
	OpponentID     string `json:"opponentId"`
	OpponentKDA    string `json:"opponentKda"`
	Status         string `json:"status"`
	Spells         string `json:"spells"`
	Keystone       string `json:"keystone"`
	SecondaryRunes string `json:"secondaryRunes"`
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
		games := make([]Game, len(found.Results))
		for i, r := range found.Results {
			games[i] = Game{MatchID: r.MatchID, Date: r.Date.Format(time.RFC3339), Region: r.Region, Patch: r.Patch,
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
}

// Champions returns up to limit champions fuzzily matching query.
func (a *App) Champions(query string, limit int) ([]Champion, error) {
	return withStore(a, func(s *core.Store) ([]Champion, error) {
		all, err := core.EnsureChampions(a.ctx, s)
		if err != nil {
			return nil, err
		}
		ranked := core.RankChampions(all, query)
		result := make([]Champion, 0, min(limit, len(ranked)))
		for _, c := range ranked[:min(limit, len(ranked))] {
			result = append(result, Champion{ID: c.ID, Name: c.Name})
		}
		return result, nil
	})
}

type UpdateStatus struct {
	Running      bool   `json:"running"`
	Cancelled    bool   `json:"cancelled"`
	Finished     bool   `json:"finished"`
	Error        string `json:"error"`
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
	}
	if wait := time.Until(s.WaitUntil); s.Running && wait > 0 {
		status.WaitSeconds = int(wait.Round(time.Second).Seconds())
		status.WaitReason = s.WaitReason
	}
	return status
}

func (a *App) StartUpdate() error {
	key, err := withStore(a, core.ConfiguredRiotKey)
	if err != nil {
		return err
	}
	if key == "" {
		return fmt.Errorf("set a Riot API key first")
	}
	return a.background.Start(api.NewRiotClient(key))
}

func (a *App) CancelUpdate() { a.background.Stop() }

func (a *App) SetRiotKey(key string) error {
	_, err := withStore(a, func(s *core.Store) (struct{}, error) {
		return struct{}{}, s.SetRiotKey(strings.TrimSpace(key))
	})
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
	Rank      string   `json:"rank"`
	Champions []string `json:"champions"`
	Pending   bool     `json:"pending"`
}

func (a *App) Players() ([]PlayerRow, error) {
	return withStore(a, func(s *core.Store) ([]PlayerRow, error) {
		players, err := s.ListPlayers()
		if err != nil {
			return nil, err
		}
		pending, err := s.PendingSeeds()
		if err != nil {
			return nil, err
		}
		rows := make([]PlayerRow, 0, len(players)+len(pending))
		for _, p := range players {
			champions, err := s.PlayerChampions(p.PUUID)
			if err != nil {
				return nil, err
			}
			rank := strings.TrimSpace(p.Tier + " " + p.Division)
			if rank == "" {
				rank = "Unranked"
			}
			rows = append(rows, PlayerRow{RiotID: p.GameName + "#" + p.TagLine, Region: core.PlatformLabel(p.Region),
				Rank: rank, Champions: champions})
		}
		for _, seed := range pending {
			rows = append(rows, PlayerRow{RiotID: seed.GameName + "#" + seed.TagLine, Region: core.PlatformLabel(seed.Region),
				Champions: []string{}, Pending: true})
		}
		return rows, nil
	})
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
		DefaultFilename: "matchup-lookup-" + time.Now().Format("20060102-150405") + ".db",
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
