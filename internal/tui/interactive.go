package tui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"matchup-lookup/internal/api"
	"matchup-lookup/internal/core"
)

func RunInteractive(path string) error {
	store, err := core.OpenStore(path)
	if err != nil {
		return err
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()
	background := core.NewBackgroundUpdate(path)
	defer background.Stop()
	for {
		action, err := pickMenuAction(background)
		if err != nil {
			return err
		}
		selectionShown := false
		switch action {
		case "exit":
			return nil
		case "path":
			fmt.Println(path)
		case "players":
			err = PrintPlayers(store)
		case "key":
			err = manageRiotKeys(store)
		case "find":
			selectionShown, err = interactiveFind(store)
		case "repeat":
			selectionShown, err = repeatLastSearch(store)
		case "import":
			err = interactiveImport(store)
		case "update":
			var keys []core.APIKey
			keys, err = interactiveKeys(store)
			if err == nil {
				err = background.Start(api.NewRiotClient(keys, func(key core.APIKey, reason string) {
					_ = store.MarkKeyRejected(key, reason) // best effort: rejected again next run
				}))
				if err == nil {
					fmt.Println("Update started in background. You can search while it runs.")
				}
			}
		case "cancel_update":
			if background.Snapshot().Running {
				background.Stop()
				fmt.Println("Update cancelled.")
			} else {
				fmt.Println("No update is running.")
			}
		case "db_export":
			var output string
			output = fmt.Sprintf("matchup-lookup-%s.db", time.Now().Format("20060102-150405"))
			err = huh.NewInput().Title("Export snapshot path").Value(&output).Run()
			if err == nil {
				err = core.ExportDatabase(store, strings.TrimSpace(output))
				if err == nil {
					fmt.Println("Database exported to", output)
				}
			}
		case "db_import":
			if background.Snapshot().Running {
				err = fmt.Errorf("cancel or finish the background update before importing a database")
				break
			}
			var source string
			err = huh.NewInput().Title("Snapshot .db path").Value(&source).Run()
			if err == nil {
				if closeErr := store.Close(); closeErr != nil {
					err = closeErr
				} else {
					store = nil
					var backup string
					backup, err = core.ImportDatabase(path, strings.TrimSpace(source))
					var openErr error
					store, openErr = core.OpenStore(path)
					if openErr != nil {
						return openErr
					}
					if err == nil {
						fmt.Println("Database imported from", filepath.Clean(source))
						if backup != "" {
							fmt.Println("Previous database backed up to", backup)
						}
					}
				}
			}
		}
		if errors.Is(err, huh.ErrUserAborted) {
			continue
		}
		if err != nil {
			fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render("Error: " + err.Error()))
		}
		if requiresReturnPrompt(action, selectionShown, err) {
			if err := waitForReturnToMenu(); err != nil {
				return err
			}
		}
	}
}

func requiresReturnPrompt(action string, selectionShown bool, err error) bool {
	return !((action == "find" || action == "repeat") && selectionShown && err == nil)
}

func PrintPlayers(store *core.Store) error {
	players, err := store.ListPlayers()
	if err != nil {
		return err
	}
	fmt.Printf("Tracked players: %d\n", len(players))
	for _, p := range players {
		champions, err := store.PlayerChampions(p.PUUID)
		if err != nil {
			return err
		}
		rank := strings.TrimSpace(p.Tier + " " + p.Division)
		if rank == "" {
			rank = "unranked/unknown"
		}
		fmt.Printf("  %s#%s  [%s]  %s  %s\n", p.GameName, p.TagLine, core.PlatformLabel(p.Region), rank, strings.Join(champions, ", "))
	}
	pending, err := store.PendingSeeds()
	if err != nil {
		return err
	}
	fmt.Printf("Pending Riot ID checks: %d\n", len(pending))
	for _, seed := range pending {
		fmt.Printf("  %s#%s  [%s]  pending  %s\n", seed.GameName, seed.TagLine, core.PlatformLabel(seed.Region), seed.Champion)
	}
	return nil
}

// interactiveKeys returns the usable keys, asking for a new one when there
// is none or Riot rejected all saved keys.
func interactiveKeys(store *core.Store) ([]core.APIKey, error) {
	keys, err := store.UsableRiotKeys()
	if err == nil {
		return keys, nil
	}
	fmt.Println(err)
	if err := promptNewKey(store); err != nil {
		return nil, err
	}
	return store.UsableRiotKeys()
}

// manageRiotKeys lists the keys in the order they are tried and lets the
// user add or remove one.
func manageRiotKeys(store *core.Store) error {
	keys, err := store.RiotKeys()
	if err != nil {
		return err
	}
	fmt.Println("Riot API keys, tried in this order (the next one takes over when Riot rejects a key):")
	if len(keys) == 0 {
		fmt.Println("  none yet")
	}
	for _, key := range keys {
		status := "ok"
		switch {
		case key.FromEnv:
			status = "from RIOT_API_KEY"
		case !key.Usable():
			status = "rejected by Riot (" + key.Rejection + ")"
		}
		fmt.Printf("  %s  %s  %s\n", key.Label, key.Masked(), status)
	}
	action := "add"
	options := []huh.Option[string]{huh.NewOption("Add a key", "add")}
	for _, key := range keys {
		if !key.FromEnv {
			options = append(options, huh.NewOption("Remove "+key.Label+" "+key.Masked(), strconv.FormatInt(key.ID, 10)))
		}
	}
	options = append(options, huh.NewOption("Back", "back"))
	if err := huh.NewSelect[string]().Title("Riot API keys").Options(options...).Value(&action).Run(); err != nil {
		return err
	}
	switch action {
	case "back":
		return nil
	case "add":
		return promptNewKey(store)
	}
	id, err := strconv.ParseInt(action, 10, 64)
	if err != nil {
		return err
	}
	if err := store.RemoveRiotKey(id); err != nil {
		return err
	}
	fmt.Println("Key removed.")
	return nil
}

// promptNewKey asks for a key, checks it with Riot and saves it. A key Riot
// rejects is not saved; without network the key is saved unchecked.
func promptNewKey(store *core.Store) error {
	var label, value string
	err := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Name (e.g. personal, dev)").Value(&label),
		huh.NewInput().Title("Riot API key (saved locally in SQLite)").EchoMode(huh.EchoModePassword).Value(&value),
	)).Run()
	if err != nil {
		return err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("Riot API key is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	checkErr := api.CheckKey(ctx, value, "euw1")
	if errors.Is(checkErr, core.ErrKeyRejected) {
		return fmt.Errorf("Riot rejected this key: it is expired or mistyped")
	}
	if err := store.AddRiotKey(label, value); err != nil {
		return err
	}
	if checkErr != nil {
		fmt.Println("Key saved without checking:", checkErr)
		return nil
	}
	fmt.Println("Key checked with Riot and saved.")
	return nil
}

func interactiveImport(store *core.Store) error {
	source := core.DefaultPlayersSource
	if err := huh.NewInput().Title("JSON/CSV URL or local path").Value(&source).Run(); err != nil {
		return err
	}
	seeds, err := core.LoadSeeds(strings.TrimSpace(source))
	if err != nil {
		return err
	}
	return ImportSeeds(store, seeds)
}

func interactiveFind(store *core.Store) (bool, error) {
	f, _, err := store.LastFilters()
	if err != nil {
		return false, err
	}
	servers, err := store.Servers()
	if err != nil {
		return false, err
	}
	regionOptions := []huh.Option[string]{huh.NewOption("Any server", "")}
	for _, server := range servers {
		label := server.Label
		if server.Players > 0 {
			label += fmt.Sprintf(" · %d players", server.Players)
		}
		regionOptions = append(regionOptions, huh.NewOption(label, server.Platform))
	}
	_ = core.RefreshLivePatch(context.Background(), store) // offline: use the last known patch
	catalog, err := core.EnsureChampions(context.Background(), store)
	if err != nil {
		fmt.Println("Champion catalog unavailable; enter champion names manually:", err)
	}
	result, rank, kda := f.Result, f.Rank, f.KDACompare
	advanced := f.MinMinutes > 0 || f.Player != ""
	steps := []func() error{
		func() error {
			return huh.NewSelect[string]().Title("Server").Options(regionOptions...).Value(&f.Region).Run()
		},
		func() error {
			if catalog == nil {
				return huh.NewInput().Title("Your champion (empty = any)").Value(&f.Champion).Run()
			}
			chosen, err := PickChampion("Your champion", catalog, f.Champion)
			if err == nil {
				f.Champion = chosen
			}
			return err
		},
		func() error {
			if catalog == nil {
				return huh.NewInput().Title("Opponent champion (empty = any)").Value(&f.Opponent).Run()
			}
			chosen, err := PickChampion("Opponent champion", catalog, f.Opponent)
			if err == nil {
				f.Opponent = chosen
			}
			return err
		},
		func() error {
			return huh.NewForm(huh.NewGroup(
				huh.NewSelect[string]().Title("Result").Options(
					huh.NewOption("Any", "any"), huh.NewOption("Win", "win"), huh.NewOption("Loss", "loss"),
				).Value(&result),
				huh.NewSelect[string]().Title("KDA vs lane opponent").Options(
					huh.NewOption("Any", "any"), huh.NewOption("At least opponent's KDA", "ge"), huh.NewOption("Better than opponent's KDA", "gt"),
				).Value(&kda),
				huh.NewSelect[string]().Title("Player rank").Options(
					huh.NewOption("Any", "any"), huh.NewOption("Emerald+", "emerald"), huh.NewOption("Diamond+", "diamond"),
					huh.NewOption("Master+", "master"), huh.NewOption("Grandmaster+", "grandmaster"), huh.NewOption("Challenger", "challenger"),
				).Value(&rank),
			)).Run()
		},
		func() error {
			chosen, err := promptYesNo("More filters?", advanced)
			advanced = chosen
			return err
		},
		func() error {
			if !advanced {
				return nil
			}
			minutes, player := "", f.Player
			if f.MinMinutes > 0 {
				minutes = strconv.Itoa(f.MinMinutes)
			}
			err := huh.NewForm(huh.NewGroup(
				huh.NewInput().Title("Minimum game duration (minutes; empty = any)").Value(&minutes),
				huh.NewInput().Title("Player name / PUUID (empty = any)").Value(&player),
			)).Run()
			if err != nil {
				return err
			}
			f.Player = strings.TrimSpace(player)
			f.MinMinutes = 0
			if strings.TrimSpace(minutes) != "" {
				f.MinMinutes, err = strconv.Atoi(strings.TrimSpace(minutes))
				if err != nil {
					return fmt.Errorf("duration must be a number")
				}
			}
			return nil
		},
	}
	err = runWizardSteps(steps)
	if err != nil {
		return false, err
	}
	f.Champion, f.Opponent = strings.TrimSpace(f.Champion), strings.TrimSpace(f.Opponent)
	f.Result, f.KDACompare, f.Rank = result, kda, rank
	if !advanced {
		f.MinMinutes, f.Player = 0, ""
	}
	found, err := core.Search(store, f)
	if err != nil {
		return false, err
	}
	if err := store.SaveFilters(f); err != nil {
		return false, err
	}
	printInteractiveSearchSummary(found)
	return len(found.Results) > 0, promptOpenReplay(found.Results)
}

func repeatLastSearch(store *core.Store) (bool, error) {
	filters, found, err := store.LastFilters()
	if err != nil {
		return false, err
	}
	if !found {
		return false, fmt.Errorf("no saved search yet; use Find matchups first")
	}
	result, err := core.Search(store, filters)
	if err != nil {
		return false, err
	}
	printInteractiveSearchSummary(result)
	return len(result.Results) > 0, promptOpenReplay(result.Results)
}

func promptOpenReplay(rows []core.Result) error {
	if len(rows) == 0 {
		return nil
	}
	cursor := 0
	for {
		index, err := browseReplay(rows, cursor)
		if err != nil {
			return err
		}
		if index < 0 {
			return nil
		}
		cursor = index
		row := rows[index]
		fmt.Println()
		printResult(index+1, row)
		launch, err := promptYesNo("Open this replay in League Client?", false)
		if err != nil {
			return err
		}
		if !launch {
			continue
		}
		client, err := api.ConnectLeagueClient()
		if err != nil {
			return err
		}
		fmt.Println("Opening replay", row.MatchID, "in League Client...")
		if err := api.OpenReplay(context.Background(), client, row.MatchID); err != nil {
			return err
		}
		fmt.Println("Replay launch requested in League Client.")
		return nil
	}
}

func printInteractiveSearchSummary(r core.SearchResult) {
	if len(r.Results) == 0 {
		PrintResults(r)
		return
	}
	fmt.Printf("Patch %s · Players %d · Stored %d · Matching %d · Wins %d · Losses %d\n", r.Patch, r.Players, r.Stored, r.Matching, r.Wins, r.Matching-r.Wins)
	fmt.Printf("Showing %d most recent matches. Choose one to see spells, runes, and replay options.\n\n", len(r.Results))
}
