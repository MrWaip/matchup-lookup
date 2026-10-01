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
			huh.NewOption("Update recent matches", "update"),
			huh.NewOption("Import players from GitHub / URL / file", "import"),
			huh.NewOption("Show tracked players", "players"),
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
		case "find":
			err = interactiveFind(store)
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
		rank := strings.TrimSpace(p.Tier + " " + p.Division)
		if rank == "" {
			rank = "unranked/unknown"
		}
		fmt.Printf("  %s#%s  [%s]  %s\n", p.GameName, p.TagLine, p.Region, rank)
	}
	return nil
}

func interactiveKey() (string, error) {
	if key := strings.TrimSpace(os.Getenv("RIOT_API_KEY")); key != "" {
		return key, nil
	}
	var key string
	err := huh.NewInput().Title("Riot API key (used only for this session)").EchoMode(huh.EchoModePassword).Value(&key).Run()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("Riot API key is required")
	}
	return strings.TrimSpace(key), nil
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
	key, err := interactiveKey()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return ImportSeeds(ctx, store, NewRiotClient(key), seeds)
}

func interactiveUpdate(store *Store) error {
	key, err := interactiveKey()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return UpdatePlayers(ctx, store, NewRiotClient(key))
}

func interactiveFind(store *Store) error {
	f := Filters{Region: "euw1", Limit: 100}
	var champion, opponent, result, rank, kda string
	var days int
	err := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Your champion (empty = any)").Placeholder("Fiora").Value(&champion),
		huh.NewInput().Title("Opponent champion (empty = any)").Placeholder("Darius").Value(&opponent),
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
	f.Champion, f.Opponent, f.Result, f.KDACompare, f.Rank, f.Days = strings.TrimSpace(champion), strings.TrimSpace(opponent), result, kda, rank, days
	var advanced bool
	if err := huh.NewConfirm().Title("More filters?").Value(&advanced).Run(); err != nil {
		return err
	}
	if advanced {
		var minutes, patch, player string
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
	PrintResults(rows, players, total, matching, wins)
	return nil
}
