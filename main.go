package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		usage()
		return nil
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
		key := strings.TrimSpace(os.Getenv("RIOT_API_KEY"))
		if key == "" {
			return fmt.Errorf("set RIOT_API_KEY in your environment")
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
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return ImportSeeds(ctx, store, NewRiotClient(key), seeds)
	case "update":
		if len(os.Args) != 2 {
			return fmt.Errorf("update takes no arguments; use import -source to add players")
		}
		key := strings.TrimSpace(os.Getenv("RIOT_API_KEY"))
		if key == "" {
			return fmt.Errorf("set RIOT_API_KEY in your environment")
		}
		store, err := OpenStore(path)
		if err != nil {
			return err
		}
		defer store.Close()
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
		players, err := store.ListPlayers()
		if err != nil {
			return err
		}
		fmt.Printf("Tracked players: %d\n", len(players))
		for _, p := range players {
			fmt.Printf("  %s#%s  [%s]  %s %s (%d LP)\n", p.GameName, p.TagLine, p.Region, p.Tier, p.Division, p.LP)
		}
		return nil
	case "find":
		flags := flag.NewFlagSet("find", flag.ContinueOnError)
		var f Filters
		flags.StringVar(&f.Opponent, "opponent", "", "opponent champion, e.g. Darius")
		flags.StringVar(&f.Result, "result", "any", "any, win, loss")
		flags.StringVar(&f.Rank, "rank", "any", "any, emerald, diamond, master, grandmaster, challenger")
		flags.IntVar(&f.Days, "days", 7, "last N days; 0 means all stored")
		flags.IntVar(&f.MinMinutes, "min-minutes", 0, "minimum game duration")
		flags.StringVar(&f.Patch, "patch", "", "patch, e.g. 16.19")
		flags.StringVar(&f.Player, "player", "", "Fiora game name or PUUID")
		flags.StringVar(&f.Region, "region", "euw1", "platform (euw1)")
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
		PrintResults(rows, players, total, matches, wins)
		return nil
	case "path":
		fmt.Println(path)
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
	fmt.Println(`matchup-lookup: recent EUW Fiora Solo/Duo matchups

Commands:
  import [-source URL-or-path]     Import JSON/CSV players into SQLite
  players                         List stored player pool
  update                          Fetch recent matches for stored players
  find [filters]                   Search stored games (no API key needed)
  path                             Show database location

Example:
  matchup-lookup find -opponent Darius -result win -rank diamond -days 7

Use "matchup-lookup import -h" or "matchup-lookup find -h" for flags.`)
}
