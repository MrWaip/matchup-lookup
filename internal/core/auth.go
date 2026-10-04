package core

import (
	"os"
	"strings"
)

func ConfiguredRiotKey(store *Store) (string, error) {
	if key := strings.TrimSpace(os.Getenv("RIOT_API_KEY")); key != "" {
		return key, nil
	}
	return store.RiotKey()
}
