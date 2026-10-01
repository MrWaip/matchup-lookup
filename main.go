package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
)

func main() {
	restoreConsole := initConsole()
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
	if len(os.Args) < 2 {
		path, err := defaultDBPath()
		if err != nil {
			return err
		}
		if custom := os.Getenv("MATCHUP_DB_PATH"); custom != "" {
			path = custom
		}
		return RunInteractive(path)
	}
	path, err := defaultDBPath()
	if err != nil {
		return err
	}
	if custom := os.Getenv("MATCHUP_DB_PATH"); custom != "" {
		path = custom
	}
	switch os.Args[1] {
	case "import":
		flags := flag.NewFlagSet("import", flag.ContinueOnError)
		source := flags.String("source", DefaultPlayersSource, "local JSON/CSV path or HTTP(S) URL")
		if err := flags.Parse(os.Args[2:]); err != nil {
			return err
		}
		seeds, err := LoadSeeds(*source)
		if err != nil {
			return err
		}
		store, err := OpenStore(path)
		if err != nil {
			return err
		}
		defer store.Close()
		fmt.Println("Database:", path)
		return ImportSeeds(store, seeds)
	case "update":
		if len(os.Args) != 2 {
			return fmt.Errorf("update takes no arguments; use import -source to add players")
		}
		store, err := OpenStore(path)
		if err != nil {
			return err
		}
		defer store.Close()
		key, err := configuredRiotKey(store)
		if err != nil {
			return err
		}
		if key == "" {
			return fmt.Errorf("set Riot API key in the interactive menu or RIOT_API_KEY")
		}
		fmt.Println("Database:", path)
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return UpdatePlayers(ctx, store, NewRiotClient(key))
	case "players":
		store, err := OpenStore(path)
		if err != nil {
			return err
		}
		defer store.Close()
		return PrintPlayers(store)
	case "find":
		flags := flag.NewFlagSet("find", flag.ContinueOnError)
		var f Filters
		flags.StringVar(&f.Champion, "champion", "", "tracked player's champion, e.g. Fiora")
		flags.StringVar(&f.Opponent, "opponent", "", "opponent champion, e.g. Darius")
		flags.StringVar(&f.Result, "result", "any", "any, win, loss")
		flags.StringVar(&f.KDACompare, "kda", "any", "any, ge (>= opponent), gt (> opponent)")
		flags.StringVar(&f.Rank, "rank", "any", "any, emerald, diamond, master, grandmaster, challenger")
		flags.IntVar(&f.Days, "days", 7, "last N days; 0 means all stored")
		flags.IntVar(&f.MinMinutes, "min-minutes", 0, "minimum game duration")
		flags.StringVar(&f.Patch, "patch", "", "patch, e.g. 16.19")
		flags.StringVar(&f.Player, "player", "", "tracked player game name or PUUID")
		flags.StringVar(&f.Region, "region", "", "platform (euw1, na1, etc.); empty = any")
		flags.IntVar(&f.Limit, "limit", 100, "maximum rows shown")
		if err := flags.Parse(os.Args[2:]); err != nil {
			return err
		}
		store, err := OpenStore(path)
		if err != nil {
			return err
		}
		defer store.Close()
		rows, players, total, matches, wins, err := Search(store, f)
		if err != nil {
			return err
		}
		if err := store.SaveFilters(f); err != nil {
			return err
		}
		PrintResults(rows, players, total, matches, wins)
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
		client, err := connectLeagueClient()
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return openReplay(ctx, client, *matchID)
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
		store, err := OpenStore(path)
		if err != nil {
			return err
		}
		defer store.Close()
		if err := ExportDatabase(store, *output); err != nil {
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
		backup, err := ImportDatabase(path, *source)
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
	fmt.Println(`matchup-lookup: recent Solo/Duo champion matchups

Run without a command for the interactive menu.

Commands for scripting:
  import [-source URL-or-path]     Import JSON/CSV players into SQLite
  players                         List stored player pool
  update                          Fetch recent matches for stored players
  find [filters]                   Search stored games (no API key needed)
  watch -match MATCH_ID            Open a replay in the running League Client
  path                             Show database location
  db-export -output PATH           Export a portable SQLite snapshot
  db-import -source PATH           Install a snapshot and back up existing data

Example:
  matchup-lookup find -champion Fiora -opponent Darius -result win -kda ge -rank diamond -days 7

Use "matchup-lookup import -h" or "matchup-lookup find -h" for flags.`)
}
