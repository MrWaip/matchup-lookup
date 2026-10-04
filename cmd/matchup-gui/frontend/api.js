// Typed access to the Go methods of internal/gui.App. Wails exposes them as
// window.go.gui.App; every call returns a promise that rejects with the Go
// error message.

/**
 * @typedef {object} Region
 * @property {string} id platform such as "euw1"
 * @property {string} label display name such as "EUW"
 */

/**
 * @typedef {object} Overview
 * @property {string} dbPath
 * @property {boolean} hasKey
 * @property {boolean} keyFromEnv
 * @property {number} players
 * @property {number} pending
 * @property {Region[]} regions
 * @property {string} playersSource
 */

/**
 * @typedef {object} Filters
 * @property {string} region
 * @property {string} champion
 * @property {string} opponent
 * @property {string} result "any", "win" or "loss"
 * @property {string} kda "any", "ge" (at least the opponent's KDA) or "gt" (better)
 * @property {string} rank "any" or the lowest tier, e.g. "diamond"
 * @property {number} days 0 means all stored matches
 * @property {number} minMinutes
 * @property {string} player
 */

/**
 * @typedef {object} Game
 * @property {string} matchId
 * @property {string} date RFC 3339
 * @property {string} region
 * @property {string} patch
 * @property {boolean} win
 * @property {string} playerId
 * @property {string} champion
 * @property {string} rank
 * @property {string} kda
 * @property {number} cs
 * @property {string} opponent
 * @property {string} opponentId
 * @property {string} opponentKda
 * @property {string} status "confirmed", "lane_fallback" or "ambiguous"
 * @property {string} spells
 * @property {string} keystone
 * @property {string} secondaryRunes
 */

/**
 * @typedef {object} SearchResult
 * @property {Game[]} games newest first, at most 500
 * @property {number} players
 * @property {number} stored
 * @property {number} matching
 * @property {number} wins
 */

/**
 * @typedef {object} Champion
 * @property {string} id
 * @property {string} name
 */

/**
 * @typedef {object} UpdateStatus
 * @property {boolean} running
 * @property {boolean} cancelled
 * @property {boolean} finished
 * @property {string} error
 * @property {number} percent
 * @property {number} resolved
 * @property {number} resolveTotal
 * @property {number} players
 * @property {number} playerTotal
 * @property {number} matches
 * @property {number} matchTotal
 * @property {number} waitSeconds
 * @property {string} waitReason
 */

/**
 * @typedef {object} PlayerRow
 * @property {string} riotId
 * @property {string} region
 * @property {string} rank
 * @property {string[]} champions
 * @property {boolean} pending
 */

/**
 * @typedef {object} GoApp
 * @property {() => Promise<Overview>} Overview
 * @property {() => Promise<Filters>} LastFilters
 * @property {(filters: Filters) => Promise<SearchResult>} Search
 * @property {(query: string, limit: number) => Promise<Champion[]>} Champions
 * @property {() => Promise<UpdateStatus>} UpdateStatus
 * @property {() => Promise<void>} StartUpdate
 * @property {() => Promise<void>} CancelUpdate
 * @property {(key: string) => Promise<void>} SetRiotKey
 * @property {(source: string) => Promise<number>} ImportPlayers
 * @property {() => Promise<PlayerRow[]>} Players
 * @property {(matchId: string) => Promise<void>} OpenReplay
 * @property {() => Promise<string>} ExportDatabase
 * @property {() => Promise<string>} ImportDatabase
 */

// Wails fills window.go once its runtime loads, so look it up on each call.
const app = () => window.go.gui.App;

export const overview = () => app().Overview();
export const lastFilters = () => app().LastFilters();
/** @param {Filters} filters */
export const search = (filters) => app().Search(filters);
/** @param {string} query @param {number} limit */
export const champions = (query, limit) => app().Champions(query, limit);
export const updateStatus = () => app().UpdateStatus();
export const startUpdate = () => app().StartUpdate();
export const cancelUpdate = () => app().CancelUpdate();
/** @param {string} key */
export const setRiotKey = (key) => app().SetRiotKey(key);
/** @param {string} source */
export const importPlayers = (source) => app().ImportPlayers(source);
export const players = () => app().Players();
/** @param {string} matchId */
export const openReplay = (matchId) => app().OpenReplay(matchId);
export const exportDatabase = () => app().ExportDatabase();
export const importDatabase = () => app().ImportDatabase();
