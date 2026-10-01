package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

func RunInteractive(path string) error {
	store, err := OpenStore(path)
	if err != nil {
		return err
	}
	defer store.Close()
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	fmt.Println(title.Render("◆ MATCHUP LOOKUP"))
	fmt.Println("Database:", path)
	for {
		var action string
		err := huh.NewSelect[string]().Title("What would you like to do?").Options(
			huh.NewOption("Find matchups", "find"),
			huh.NewOption("Repeat last search", "repeat"),
			huh.NewOption("Update recent matches", "update"),
			huh.NewOption("Import players from GitHub / URL / file", "import"),
			huh.NewOption("Show tracked players", "players"),
			huh.NewOption("Set / replace Riot API key", "key"),
			huh.NewOption("Show database location", "path"),
			huh.NewOption("Exit", "exit"),
		).Value(&action).Run()
		if errors.Is(err, huh.ErrUserAborted) {
			return nil
		}
		if err != nil {
			return err
		}
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
			err = interactiveFind(store)
		case "repeat":
			err = repeatLastSearch(store)
		case "import":
			err = interactiveImport(store)
		case "update":
			err = interactiveUpdate(store)
		}
		if errors.Is(err, huh.ErrUserAborted) {
			continue
		}
		if err != nil {
			fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render("Error: " + err.Error()))
		}
		fmt.Println()
	}
}

func PrintPlayers(store *Store) error {
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
		fmt.Printf("  %s#%s  [%s]  %s  %s\n", p.GameName, p.TagLine, platformLabel(p.Region), rank, strings.Join(champions, ", "))
	}
	pending, err := store.PendingSeeds()
	if err != nil {
		return err
	}
	fmt.Printf("Pending Riot ID checks: %d\n", len(pending))
	for _, seed := range pending {
		fmt.Printf("  %s#%s  [%s]  pending  %s\n", seed.GameName, seed.TagLine, platformLabel(seed.Region), seed.Champion)
	}
	return nil
}

func interactiveKey(store *Store) (string, error) {
	key, err := configuredRiotKey(store)
	if err != nil || key != "" {
		return key, err
	}
	if err := promptRiotKey(store); err != nil {
		return "", err
	}
	return store.RiotKey()
}

func promptRiotKey(store *Store) error {
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

func interactiveImport(store *Store) error {
	source := DefaultPlayersSource
	if err := huh.NewInput().Title("JSON/CSV URL or local path").Value(&source).Run(); err != nil {
		return err
	}
	seeds, err := LoadSeeds(strings.TrimSpace(source))
	if err != nil {
		return err
	}
	return ImportSeeds(store, seeds)
}

func interactiveUpdate(store *Store) error {
	key, err := interactiveKey(store)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return UpdatePlayers(ctx, store, NewRiotClient(key))
}

func interactiveFind(store *Store) error {
	f, _, err := store.LastFilters()
	if err != nil {
		return err
	}
	regions, err := store.ListRegions()
	if err != nil {
		return err
	}
	regionOptions := []huh.Option[string]{huh.NewOption("Any server", "")}
	for _, platform := range regions {
		regionOptions = append(regionOptions, huh.NewOption(platformLabel(platform)+" ("+platform+")", platform))
	}
	if err := huh.NewSelect[string]().Title("Server").Options(regionOptions...).Value(&f.Region).Run(); err != nil {
		return err
	}
	catalog, err := EnsureChampions(context.Background(), store)
	if err != nil {
		fmt.Println("Champion catalog unavailable; enter champion names manually:", err)
		if err := huh.NewInput().Title("Your champion (empty = any)").Value(&f.Champion).Run(); err != nil {
			return err
		}
		if err := huh.NewInput().Title("Opponent champion (empty = any)").Value(&f.Opponent).Run(); err != nil {
			return err
		}
	} else {
		f.Champion, err = PickChampion("Your champion", catalog, f.Champion)
		if err != nil {
			return err
		}
		f.Opponent, err = PickChampion("Opponent champion", catalog, f.Opponent)
		if err != nil {
			return err
		}
	}
	result, rank, kda := f.Result, f.Rank, f.KDACompare
	days := f.Days
	err = huh.NewForm(huh.NewGroup(
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
	if err != nil {
		return err
	}
	f.Champion, f.Opponent = strings.TrimSpace(f.Champion), strings.TrimSpace(f.Opponent)
	f.Result, f.KDACompare, f.Rank, f.Days = result, kda, rank, days
	advanced := f.MinMinutes > 0 || f.Patch != "" || f.Player != ""
	if err := huh.NewConfirm().Title("More filters?").Value(&advanced).Run(); err != nil {
		return err
	}
	if advanced {
		minutes, patch, player := "", f.Patch, f.Player
		if f.MinMinutes > 0 {
			minutes = strconv.Itoa(f.MinMinutes)
		}
		err := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Minimum game duration (minutes; empty = any)").Value(&minutes),
			huh.NewInput().Title("Patch (empty = any)").Placeholder("16.19").Value(&patch),
			huh.NewInput().Title("Tracked player (empty = any)").Value(&player),
		)).Run()
		if err != nil {
			return err
		}
		f.Patch, f.Player = strings.TrimSpace(patch), strings.TrimSpace(player)
		if strings.TrimSpace(minutes) != "" {
			f.MinMinutes, err = strconv.Atoi(strings.TrimSpace(minutes))
			if err != nil {
				return fmt.Errorf("duration must be a number")
			}
		}
	}
	rows, players, total, matching, wins, err := Search(store, f)
	if err != nil {
		return err
	}
	if err := store.SaveFilters(f); err != nil {
		return err
	}
	PrintResults(rows, players, total, matching, wins)
	return nil
}

func repeatLastSearch(store *Store) error {
	filters, found, err := store.LastFilters()
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("no saved search yet; use Find matchups first")
	}
	rows, players, total, matching, wins, err := Search(store, filters)
	if err != nil {
		return err
	}
	PrintResults(rows, players, total, matching, wins)
	return nil
}
