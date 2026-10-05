package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"matchup-lookup/internal/api"
	"matchup-lookup/internal/core"
	"matchup-lookup/internal/tui"
)

func main() {
	restoreConsole := tui.InitConsole()
	err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
	}
	restoreConsole()
	if err != nil {
		os.Exit(1)
	}
}

func run() error {
	path, err := core.DBPath()
	if err != nil {
		return err
	}
	if len(os.Args) < 2 {
		return tui.RunInteractive(path)
	}
	switch os.Args[1] {
	case "import":
		flags := flag.NewFlagSet("import", flag.ContinueOnError)
		source := flags.String("source", core.DefaultPlayersSource, "local JSON/CSV path or HTTP(S) URL")
		if err := flags.Parse(os.Args[2:]); err != nil {
			return err
		}
		seeds, err := core.LoadSeeds(*source)
		if err != nil {
			return err
		}
		store, err := core.OpenStore(path)
		if err != nil {
			return err
		}
		defer store.Close()
		fmt.Println("Database:", path)
		return tui.ImportSeeds(store, seeds)
	case "update":
		if len(os.Args) != 2 {
			return fmt.Errorf("update takes no arguments; use import -source to add players")
		}
		store, err := core.OpenStore(path)
		if err != nil {
			return err
		}
		defer store.Close()
		keys, err := store.UsableRiotKeys()
		if err != nil {
			return fmt.Errorf("%w (add keys in the interactive menu under Riot API keys, or set RIOT_API_KEY)", err)
		}
		fmt.Println("Database:", path)
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		client := api.NewRiotClient(keys, func(key core.APIKey, reason string) {
			fmt.Printf("\n[KEY] Riot rejected key %s (%s); trying the next one\n", key.Label, reason)
			_ = store.MarkKeyRejected(key, reason) // best effort: rejected again next run
		})
		err = tui.UpdatePlayers(ctx, store, client)
		if errors.Is(err, core.ErrKeyRejected) {
			return fmt.Errorf("%w. Data fetched so far is saved; add a new key in the interactive menu or set RIOT_API_KEY", err)
		}
		return err
	case "players":
		store, err := core.OpenStore(path)
		if err != nil {
			return err
		}
		defer store.Close()
		return tui.PrintPlayers(store)
	case "find":
		flags := flag.NewFlagSet("find", flag.ContinueOnError)
		var f core.Filters
		flags.StringVar(&f.Champion, "champion", "", "tracked player's champion, e.g. Fiora")
		flags.StringVar(&f.Opponent, "opponent", "", "opponent champion, e.g. Darius")
		flags.StringVar(&f.Result, "result", "any", "any, win, loss")
		flags.StringVar(&f.KDACompare, "kda", "any", "any, ge (>= opponent), gt (> opponent)")
		flags.StringVar(&f.Rank, "rank", "any", "any, emerald, diamond, master, grandmaster, challenger")
		flags.IntVar(&f.MinMinutes, "min-minutes", 0, "minimum game duration")
		flags.StringVar(&f.Player, "player", "", "tracked player game name or PUUID")
		flags.StringVar(&f.Region, "region", "", "platform (euw1, na1, etc.); empty = any")
		flags.IntVar(&f.Limit, "limit", 100, "maximum rows shown")
		if err := flags.Parse(os.Args[2:]); err != nil {
			return err
		}
		store, err := core.OpenStore(path)
		if err != nil {
			return err
		}
		defer store.Close()
		_ = core.RefreshLivePatch(context.Background(), store) // offline: use the last known patch
		result, err := core.Search(store, f)
		if err != nil {
			return err
		}
		if err := store.SaveFilters(f); err != nil {
			return err
		}
		tui.PrintResults(result)
		return nil
	case "version":
		fmt.Println(core.Version())
		return nil
	case "path":
		fmt.Println(path)
		return nil
	case "watch":
		flags := flag.NewFlagSet("watch", flag.ContinueOnError)
		matchID := flags.String("match", "", "match ID, e.g. EUW1_123456789")
		if err := flags.Parse(os.Args[2:]); err != nil {
			return err
		}
		if *matchID == "" {
			return fmt.Errorf("watch requires -match MATCH_ID")
		}
		client, err := api.ConnectLeagueClient()
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return api.OpenReplay(ctx, client, *matchID)
	case "db-export":
		flags := flag.NewFlagSet("db-export", flag.ContinueOnError)
		output := flags.String("output", "", "destination .db file")
		if err := flags.Parse(os.Args[2:]); err != nil {
			return err
		}
		if *output == "" {
			return fmt.Errorf("db-export requires -output PATH")
		}
		if _, err := os.Stat(path); err != nil {
			return err
		}
		store, err := core.OpenStore(path)
		if err != nil {
			return err
		}
		defer store.Close()
		if err := core.ExportDatabase(store, *output); err != nil {
			return err
		}
		fmt.Println("Database exported to", *output)
		return nil
	case "db-import":
		flags := flag.NewFlagSet("db-import", flag.ContinueOnError)
		source := flags.String("source", "", "SQLite snapshot .db file")
		if err := flags.Parse(os.Args[2:]); err != nil {
			return err
		}
		if *source == "" {
			return fmt.Errorf("db-import requires -source PATH")
		}
		backup, err := core.ImportDatabase(path, *source)
		if err != nil {
			return err
		}
		fmt.Println("Database imported to", path)
		if backup != "" {
			fmt.Println("Previous database backed up to", backup)
		}
		return nil
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command %q", os.Args[1])
	}
}

func usage() {
	fmt.Println("MatchupFinder.gg " + core.Version() + `: recent Solo/Duo champion matchups

Run without a command for the interactive menu.

Commands for scripting:
  import [-source URL-or-path]     Import JSON/CSV players into SQLite
  players                         List stored player pool
  update                          Fetch recent matches for stored players
  find [filters]                   core.Search stored games (no API key needed)
  watch -match MATCH_ID            Open a replay in the running League Client
  path                             Show database location
  version                          Show build date and commit
  db-export -output PATH           Export a portable SQLite snapshot
  db-import -source PATH           Install a snapshot and back up existing data

Example:
  matchup-lookup find -champion Fiora -opponent Darius -result win -kda ge -rank diamond

Use "matchup-lookup import -h" or "matchup-lookup find -h" for flags.`)
}
