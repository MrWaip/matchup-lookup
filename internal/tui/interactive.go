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
			err = promptRiotKey(store)
		case "find":
			selectionShown, err = interactiveFind(store)
		case "repeat":
			selectionShown, err = repeatLastSearch(store)
		case "import":
			err = interactiveImport(store)
		case "update":
			var key string
			key, err = interactiveKey(store)
			if err == nil {
				err = background.Start(api.NewRiotClient(key))
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

func interactiveKey(store *core.Store) (string, error) {
	key, err := core.ConfiguredRiotKey(store)
	if err != nil || key != "" {
		return key, err
	}
	if err := promptRiotKey(store); err != nil {
		return "", err
	}
	return store.RiotKey()
}

func promptRiotKey(store *core.Store) error {
	var key string
	err := huh.NewInput().Title("Riot API key (saved locally in SQLite)").EchoMode(huh.EchoModePassword).Value(&key).Run()
	if err != nil {
		return err
	}
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("Riot API key is required")
	}
	if err := store.SetRiotKey(strings.TrimSpace(key)); err != nil {
		return err
	}
	fmt.Println("Riot API key saved in the local database.")
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
	regions, err := store.ListRegions()
	if err != nil {
		return false, err
	}
	regionOptions := []huh.Option[string]{huh.NewOption("Any server", "")}
	for _, platform := range regions {
		regionOptions = append(regionOptions, huh.NewOption(core.PlatformLabel(platform)+" ("+platform+")", platform))
	}
	catalog, err := core.EnsureChampions(context.Background(), store)
	if err != nil {
		fmt.Println("Champion catalog unavailable; enter champion names manually:", err)
	}
	result, rank, kda := f.Result, f.Rank, f.KDACompare
	days := f.Days
	advanced := f.MinMinutes > 0 || f.Patch != "" || f.Player != ""
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
				huh.NewSelect[int]().Title("Date").Options(
					huh.NewOption("Last 1 day", 1), huh.NewOption("Last 3 days", 3), huh.NewOption("Last 7 days", 7),
					huh.NewOption("Last 14 days", 14), huh.NewOption("Last 30 days", 30), huh.NewOption("All stored", 0),
				).Value(&days),
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
			minutes, patch, player := "", f.Patch, f.Player
			if f.MinMinutes > 0 {
				minutes = strconv.Itoa(f.MinMinutes)
			}
			err := huh.NewForm(huh.NewGroup(
				huh.NewInput().Title("Minimum game duration (minutes; empty = any)").Value(&minutes),
				huh.NewInput().Title("Patch (empty = any)").Placeholder("16.19").Value(&patch),
				huh.NewInput().Title("Player name / PUUID (empty = any)").Value(&player),
			)).Run()
			if err != nil {
				return err
			}
			f.Patch, f.Player = strings.TrimSpace(patch), strings.TrimSpace(player)
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
	f.Result, f.KDACompare, f.Rank, f.Days = result, kda, rank, days
	if !advanced {
		f.MinMinutes, f.Patch, f.Player = 0, "", ""
	}
	rows, players, total, matching, wins, err := core.Search(store, f)
	if err != nil {
		return false, err
	}
	if err := store.SaveFilters(f); err != nil {
		return false, err
	}
	printInteractiveSearchSummary(rows, players, total, matching, wins)
	return len(rows) > 0, promptOpenReplay(rows)
}

func repeatLastSearch(store *core.Store) (bool, error) {
	filters, found, err := store.LastFilters()
	if err != nil {
		return false, err
	}
	if !found {
		return false, fmt.Errorf("no saved search yet; use Find matchups first")
	}
	rows, players, total, matching, wins, err := core.Search(store, filters)
	if err != nil {
		return false, err
	}
	printInteractiveSearchSummary(rows, players, total, matching, wins)
	return len(rows) > 0, promptOpenReplay(rows)
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

func printInteractiveSearchSummary(rows []core.Result, players, total, matching, wins int) {
	if len(rows) == 0 {
		PrintResults(rows, players, total, matching, wins)
		return
	}
	fmt.Printf("Players %d · Stored %d · Matching %d · Wins %d · Losses %d\n", players, total, matching, wins, matching-wins)
	fmt.Printf("Showing %d most recent matches. Choose one to see spells, runes, and replay options.\n\n", len(rows))
}
