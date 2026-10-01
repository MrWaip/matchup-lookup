package main

import (
	"os"
	"strings"
)

func configuredRiotKey(store *Store) (string, error) {
	if key := strings.TrimSpace(os.Getenv("RIOT_API_KEY")); key != "" {
		return key, nil
	}
	return store.RiotKey()
}
