# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Matchup Lookup finds recent League of Legends Solo/Duo games of tracked players by lane matchup and opens their replays in the League Client. Windows is the target platform; development happens on macOS. The README and the user's messages are in Russian.

## Commands

All workflows go through `just` (see `justfile`):

- `just check` — `go vet ./...` + `go test ./...`
- `just check-js` — `npm ci` + TypeScript 7 type-check of the GUI's JSDoc-typed JS (`tsc -p .`, no emit)
- `just build` — CLI/TUI binary (`go build .`)
- `just build-gui` / `just gui` — Wails desktop app build / dev mode (needs `wails` v2.16 CLI on PATH, usually `~/go/bin`)
- `just generate` — regenerate `internal/store/*_gen.go` with `sqlc` after editing migrations or queries
- Single test: `go test ./internal/core -run TestSearchShowsOnlyCurrentPatch -count=1`
- Windows cross-check from macOS: `GOOS=windows go vet ./...`

On this machine the default `GOPROXY` points to an unreachable corporate proxy; prefix module downloads with `GOPROXY=https://proxy.golang.org,direct`. npm uses the public registry via the root `.npmrc` (npm only reads it next to `package.json`, which is why `package.json`/`tsconfig.json` live at the repo root).

`justfile` recipes run under PowerShell 5.1 on Windows: no `&&`; use the `[working-directory: ...]` attribute instead of `cd`.

## Architecture

Two executables share one core, because a Windows binary is either console or GUI:

- `main.go` — CLI commands (`import`, `update`, `find`, `watch`, `db-export`, ...) and, without arguments, the terminal menu.
- `cmd/matchup-gui` — Wails entry point embedding `frontend/`; Go methods live in `internal/gui`.

Package dependencies point one way: `main`/`cmd` → `tui`/`gui` → `core` → `store`; `api` → `core`.

- `internal/core` — domain: models, `Store` repository, collector/update, search, patch logic, Data Dragon catalog and icons. `core` and `api` must not print; progress, logs and rate-limit waits go through callbacks (`UpdateReporter`, `ImportSeeds` `onSaved`, `WaitReporter`/`RiotClient.ReportWaits`), and `tui`/`gui` render them.
- `internal/api` — Riot HTTP client with a shared rate limiter (18/s, 90/2 min, below Riot's 20/100) and the unofficial local League Client API used to download/launch replays.
- `internal/store` — SQLite schema and queries. Goose migrations in `migrations/` are embedded and applied by `store.Open` on every start, after copying an existing DB to `*.pre-migration-<ts>.db`. Queries are SQL files in `queries/` compiled by sqlc (`sqlc.yaml` at the root) into `*_gen.go`; never edit generated files. Migration 00003 is a Go migration registered in `open.go`. Add new migrations rather than editing shipped ones. `core.Store` maps generated rows to domain types and holds no SQL (tests may use `store.db.Exec` for setup).
- `internal/tui` — Bubble Tea/huh terminal UI and plain CLI output.
- `internal/gui` — `App` methods bound to the frontend as `window.go.gui.App`, with camelCase JSON DTOs.

Data model points that span several files:

- `matches.raw_gz` keeps the full Riot response gzip-compressed (`Match.Raw`); searchable fields live in `match_participants`, only for tracked players and their lane opponents. `tracked_games` stores the tracked player's point of view (opponent PUUID, rank snapshot, positions).
- Search shows each tracked game from both sides (the opponent's side unless the opponent is tracked too) via the CTE in `queries/search.sql`.
- Replays expire with each patch. Search always covers the *current patch*: the newer of Data Dragon's live version (cached hourly in `app_settings`) and the newest stored match. Pruning of older patches deliberately uses only the newest *stored* patch, since Data Dragon can lead a region's rollout. Pruned IDs stay in `match_checks` so they are not re-downloaded.
- Riot API keys (`riot_keys`, plus `RIOT_API_KEY` first) are fallbacks tried in order, never pooled (Riot forbids that). `api.RiotClient` reports a 401/403 once through `onRejected` (callers persist it via `Store.MarkKeyRejected`) and retries with the next key; with none left it returns `core.ErrKeyRejected` without network calls, and `UpdatePlayers` stops on it instead of counting per-item failures.
- `core.Filters` is persisted as JSON with Go field names (no tags) in `app_settings.last_filters`; renaming fields breaks saved filters, and removed fields are ignored on load.

## GUI frontend

`cmd/matchup-gui/frontend` is plain HTML/CSS/ES modules with no framework or bundler. Keep JS modern and simple: types via JSDoc, no type casts, no defensive checks beyond what TS strict mode needs (`$()` in `dom.js` is the typed element lookup; build DOM with `h()` so Riot IDs are never parsed as HTML). Wails v2 generates `.ts` models that cannot be loaded without a bundler, so `frontend/api.js` mirrors the Go DTOs by hand: change `internal/gui` and the JSDoc typedefs together, then run `just check-js`. `wails.d.ts` only declares the injected `window.go`.

Wails quirks:

- `wails build` passes `-buildvcs=false`, so `build-gui` injects the version via `-ldflags -X core.buildCommit/buildDate`; the CLI gets it from Go's VCS stamping (`core.Version`). `go run` shows `dev`.
- `wailsjsdir` is set to `build` so generated runtime files stay out of `frontend/`.
- Opening the `wails dev` server (localhost:34115) in a regular browser is unreliable: its dev IPC script fails before `<body>` exists. For visual checks, serve a copy of `frontend/` with a mocked `window.go` (a classic, synchronous script) and screenshot with headless Chrome.

## CI

`.github/workflows/ci.yml` runs on `windows-latest`: check, check-js, both builds, an artifact upload, and on pushes to `main` replaces the assets of the rolling `nightly` release that the README download links point to.
