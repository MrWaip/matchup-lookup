package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"matchup-lookup/internal/store"
)

// ErrKeyRejected means Riot answered 401/403: the key expired (development
// keys last 24 hours), was revoked or is mistyped. Clients wrap it, and an
// update stops on it instead of failing every remaining request.
var ErrKeyRejected = errors.New("Riot rejected the API key (expired or invalid)")

// APIKey is a Riot API key in priority order. The RIOT_API_KEY environment
// variable, when set, comes first with ID 0 and is never stored.
type APIKey struct {
	ID         int64
	Label      string
	Value      string
	FromEnv    bool
	RejectedAt time.Time // zero while the key is usable
	Rejection  string
}

func (k APIKey) Usable() bool { return k.RejectedAt.IsZero() }

// Masked shows enough of the key to tell keys apart.
func (k APIKey) Masked() string {
	if len(k.Value) <= 10 {
		return "…"
	}
	return k.Value[:6] + "…" + k.Value[len(k.Value)-4:]
}

// RiotKeys returns all keys, usable or not, in the order they are tried.
func (s *Store) RiotKeys() ([]APIKey, error) {
	rows, err := s.q.ListRiotKeys(context.Background())
	if err != nil {
		return nil, err
	}
	var keys []APIKey
	if value := strings.TrimSpace(os.Getenv("RIOT_API_KEY")); value != "" {
		keys = append(keys, APIKey{Label: "RIOT_API_KEY", Value: value, FromEnv: true})
	}
	for _, r := range rows {
		key := APIKey{ID: r.ID, Label: r.Label, Value: r.Value, Rejection: r.Rejection}
		if r.RejectedAt != 0 {
			key.RejectedAt = time.Unix(r.RejectedAt, 0)
		}
		keys = append(keys, key)
	}
	return keys, nil
}

// UsableRiotKeys returns the keys an update may try, or an error explaining
// what the user has to do when there is none.
func (s *Store) UsableRiotKeys() ([]APIKey, error) {
	keys, err := s.RiotKeys()
	if err != nil {
		return nil, err
	}
	var usable []APIKey
	for _, key := range keys {
		if key.Usable() {
			usable = append(usable, key)
		}
	}
	switch {
	case len(usable) > 0:
		return usable, nil
	case len(keys) > 0:
		return nil, fmt.Errorf("%w: every saved key was rejected", ErrKeyRejected)
	default:
		return nil, fmt.Errorf("no Riot API key yet")
	}
}

// AddRiotKey appends a key, or renames a known one and clears its rejection.
func (s *Store) AddRiotKey(label, value string) error {
	label, value = strings.TrimSpace(label), strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("Riot API key cannot be empty")
	}
	if label == "" {
		label = "Key"
	}
	return s.q.AddRiotKey(context.Background(), store.AddRiotKeyParams{Label: label, Value: value, AddedAt: time.Now().Unix()})
}

func (s *Store) RemoveRiotKey(id int64) error {
	return s.q.DeleteRiotKey(context.Background(), id)
}

// MarkKeyRejected remembers that Riot rejected a stored key; the environment
// key cannot be changed here and is only skipped for the current run.
func (s *Store) MarkKeyRejected(key APIKey, reason string) error {
	if key.FromEnv {
		return nil
	}
	return s.q.RejectRiotKey(context.Background(), store.RejectRiotKeyParams{
		RejectedAt: time.Now().Unix(), Rejection: reason, ID: key.ID})
}
