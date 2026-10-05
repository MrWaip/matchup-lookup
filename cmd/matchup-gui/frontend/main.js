import * as api from "./api.js";
import { championCombobox } from "./combobox.js";
import { $, h, icon, showError, toast } from "./dom.js";

/** @import { Filters, Game, Key, Overview, PlayerRow, SearchResult, UpdateStatus } from "./api.js" */

const form = $("#filters", HTMLFormElement);
const regionSelect = $("#region", HTMLSelectElement);
const gamesBody = $("#games", HTMLTableSectionElement);
const tableWrap = $("#table-wrap", HTMLElement);
const details = $("#details", HTMLElement);
const settings = $("#settings", HTMLDialogElement);
const playerView = $("#player-view", HTMLElement);
const matchLayout = $(".layout", HTMLElement);
const updateButton = $("#update-button", HTMLButtonElement);
const playerForm = $("#player-form", HTMLFormElement);
/** @type {PlayerRow | undefined} */
let editingPlayer;

const dateFormat = new Intl.DateTimeFormat("en-GB", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" });
const longDateFormat = new Intl.DateTimeFormat("en-GB", { dateStyle: "medium", timeStyle: "short" });

const themePreference = window.matchMedia("(prefers-color-scheme: light)");
const themeSelect = $("#theme", HTMLSelectElement);
const savedTheme = localStorage.getItem("matchupfinder.theme");
themeSelect.value = savedTheme === "light" || savedTheme === "dark" ? savedTheme : "system";
function applyTheme() {
  document.documentElement.dataset.theme = themeSelect.value === "system"
    ? themePreference.matches ? "light" : "dark"
    : themeSelect.value;
}
themeSelect.addEventListener("change", () => {
  localStorage.setItem("matchupfinder.theme", themeSelect.value);
  applyTheme();
});
themePreference.addEventListener("change", applyTheme);
applyTheme();

/** @type {Game[]} */
let games = [];
let selected = -1;
let searchRequest = 0;
/** @type {Overview | undefined} */
let currentOverview;
/** @type {PlayerRow[]} */
let listedPlayers = [];
/** @type {PlayerRow[]} */
let visiblePlayers = [];
const selectedPlayers = new Set();
let selectionAnchor = "";

const champion = championCombobox($("#champion", HTMLInputElement), $("#champion-list", HTMLUListElement), () => runSearch());
const opponent = championCombobox($("#opponent", HTMLInputElement), $("#opponent-list", HTMLUListElement), () => runSearch());

// ── Filters ────────────────────────────────────────────────────────────────

/** @returns {Filters} */
function readFilters() {
  const data = new FormData(form);
  const text = (/** @type {string} */ name) => String(data.get(name) ?? "").trim();
  return {
    region: text("region"),
    champion: text("champion"),
    opponent: text("opponent"),
    result: text("result"),
    kda: text("kda"),
    rank: text("rank"),
    minMinutes: Number(text("minMinutes")),
    player: text("player"),
  };
}

/** @param {Filters} filters */
function applyFilters(filters) {
  regionSelect.value = filters.region;
  champion.set(filters.champion);
  opponent.set(filters.opponent);
  for (const radio of form.querySelectorAll("input")) {
    if (radio.name === "result") radio.checked = radio.value === (filters.result || "any");
  }
  $("select[name=kda]", HTMLSelectElement).value = filters.kda || "any";
  $("select[name=rank]", HTMLSelectElement).value = filters.rank || "any";
  $("input[name=minMinutes]", HTMLInputElement).value = filters.minMinutes > 0 ? String(filters.minMinutes) : "";
  $("input[name=player]", HTMLInputElement).value = filters.player;
  $("details.more", HTMLDetailsElement).open = filters.minMinutes > 0 || filters.player !== "";
}

form.addEventListener("change", (event) => {
  // The comboboxes report their own changes once a champion is picked.
  if (event.target instanceof HTMLInputElement && event.target.role === "combobox") return;
  runSearch();
});
form.addEventListener("submit", (event) => event.preventDefault());
form.addEventListener("reset", () => {
  // Form controls are reset after this event, so search on the next tick.
  setTimeout(() => {
    champion.set("");
    opponent.set("");
    runSearch();
  });
});

// ── Search results ─────────────────────────────────────────────────────────

async function runSearch() {
  const request = ++searchRequest;
  const previous = games[selected]?.matchId;
  try {
    const result = await api.search(readFilters());
    if (request !== searchRequest) return;
    games = result.games;
    renderSummary(result);
    renderGames(result);
    select(Math.max(0, games.findIndex((game) => game.matchId === previous)));
  } catch (error) {
    showError(error);
  }
}

/** @param {SearchResult} result */
function renderSummary({ matching, wins, players, stored, games, patch }) {
  const losses = matching - wins;
  const rate = matching > 0 ? Math.round((wins / matching) * 100) : 0;
  const parts = [
    h("span", { className: "patch", title: "Only the current patch is searched: replays of older patches can no longer be opened." }, patch ? `Patch ${patch}` : "Patch unknown"),
    h("strong", {}, `${matching} ${matching === 1 ? "match" : "matches"}`),
    h("span", { className: "win" }, `${wins} W`),
    h("span", { className: "loss" }, `${losses} L`),
    h("span", {}, matching > 0 ? `${rate}% win rate` : "—"),
    h("span", { className: "muted" }, `${players} players · ${stored} games this patch`),
  ];
  if (games.length < matching) parts.push(h("span", { className: "muted" }, `showing newest ${games.length}`));
  $("#summary", HTMLElement).replaceChildren(...parts);
}

/** @param {SearchResult} result */
function renderGames({ stored, patch }) {
  gamesBody.replaceChildren(...games.map((game, index) => h("tr", {
    onclick: () => select(index),
    ondblclick: () => launchReplay(game),
  },
    h("td", { className: "muted" }, dateFormat.format(new Date(game.date))),
    h("td", {}, h("span", { className: `badge ${game.win ? "win" : "loss"}` }, game.win ? "Win" : "Loss")),
    h("td", {}, h("span", { className: "matchup" },
      icon(game.championIcon), h("b", {}, game.champion),
      h("span", { className: "muted" }, "vs"),
      icon(game.opponentIcon), game.opponent)),
    h("td", { className: "ellipsis col-player", title: game.playerId }, game.playerId),
    h("td", { className: "col-rank" }, game.rank),
    h("td", { className: "num" }, game.kda),
    h("td", { className: "num muted col-opp" }, game.opponentKda),
    h("td", { className: "num" }, String(game.cs)),
  )));
  const empty = $("#empty", HTMLElement);
  empty.hidden = games.length > 0;
  empty.textContent = currentOverview?.players === 0
    ? "No tracked players yet. Import players in Settings, then update matches."
    : stored === 0
      ? `No games of patch ${patch || "the current patch"} stored yet. Run Update matches to fetch them.`
      : "No games match these filters. Try fewer filters or a broader rank.";
}

/** @param {number} index */
function select(index) {
  selected = games.length === 0 ? -1 : Math.min(Math.max(index, 0), games.length - 1);
  for (const [i, row] of [...gamesBody.rows].entries()) row.classList.toggle("selected", i === selected);
  gamesBody.rows[selected]?.scrollIntoView({ block: "nearest" });
  renderDetails(games[selected]);
}

/** @param {Game | undefined} game */
function renderDetails(game) {
  details.hidden = !game;
  if (!game) return;
  /** @param {string} label @param {...(Node | string)} value */
  const field = (label, ...value) => h("div", { className: "field" }, h("dt", {}, label), h("dd", {}, ...value));
  /** @param {string[]} icons @param {string} names "A + B" */
  const pair = (icons, names) => [...icons.map((src) => icon(src)), " ", names];
  const copy = h("button", {
    type: "button",
    className: "link",
    onclick: async () => {
      await navigator.clipboard.writeText(game.matchId);
      toast("Match ID copied");
    },
  }, "Copy");
  details.replaceChildren(
    h("header", {},
      icon(game.championIcon, game.champion, "portrait"),
      h("span", { className: "muted" }, "vs"),
      icon(game.opponentIcon, game.opponent, "portrait"),
      h("div", {},
        h("h2", {}, `${game.champion} vs ${game.opponent}`),
        h("span", { className: `badge ${game.win ? "win" : "loss"}` }, game.win ? "Win" : "Loss")),
      h("button", { type: "button", className: "primary", onclick: (event) => launchReplay(game, event.currentTarget) }, "▶ Watch replay"),
    ),
    h("dl", {},
      field("Player", `${game.playerId} · ${game.rank}`),
      field("Opponent", game.opponentId),
      field("K/D/A", `${game.kda} vs ${game.opponentKda} · ${game.cs} CS`),
      field("Spells", ...pair(game.spellIcons, game.spells)),
      field("Keystone", icon(game.keystoneIcon), " ", game.keystone),
      field("Secondary", ...pair(game.secondaryIcons, game.secondaryRunes)),
      field("Played", `${longDateFormat.format(new Date(game.date))} · ${game.region} · patch ${game.patch}`),
      h("div", { className: "field" }, h("dt", {}, "Match ID"), h("dd", {}, game.matchId, " ", copy)),
    ),
  );
  if (game.status !== "confirmed") {
    details.append(h("p", { className: "muted note" }, `Lane opponent is ${game.status.replace("_", " ")}: positions in this match were not conclusive.`));
  }
}

/**
 * @param {Game} game
 * @param {EventTarget | null} [button]
 */
async function launchReplay(game, button = null) {
  const target = button instanceof HTMLButtonElement ? button : null;
  if (target) {
    target.disabled = true;
    target.textContent = "Opening…";
  }
  try {
    await api.openReplay(game.matchId);
    toast("Replay requested in the League Client");
  } catch (error) {
    showError(error);
  } finally {
    if (target) {
      target.disabled = false;
      target.textContent = "▶ Watch replay";
    }
  }
}

tableWrap.addEventListener("keydown", (event) => {
  const game = games[selected];
  switch (event.key) {
    case "ArrowDown":
    case "ArrowUp":
      event.preventDefault();
      select(selected + (event.key === "ArrowDown" ? 1 : -1));
      break;
    case "Enter":
      if (game) launchReplay(game);
      break;
  }
});

// ── Overview, banner and background update ─────────────────────────────────

async function refreshOverview() {
  const overview = await api.overview();
  currentOverview = overview;
  $("#version", HTMLElement).textContent = overview.version;
  const current = regionSelect.value;
  regionSelect.replaceChildren(
    h("option", { value: "" }, "Any server"),
    ...overview.servers.map((server) => h("option", { value: server.id },
      server.players > 0 ? `${server.label} · ${server.players} players` : server.label)),
  );
  regionSelect.value = current;

  const banner = $("#banner", HTMLElement);
  const message = overview.keys.length === 0
    ? "Add your Riot API key to download matches."
    : !overview.hasUsableKey
      ? "Riot rejected your API key: it probably expired. Add a new one to keep updating."
      : overview.players + overview.pending === 0
      ? "Import players to start tracking their matches."
      : "";
  banner.hidden = message === "";
  $("#banner-text", HTMLElement).textContent = message;
  $("#banner-action", HTMLButtonElement).textContent = "Open settings";
}

$("#banner-action", HTMLButtonElement).addEventListener("click", openSettings);

/** @param {number} seconds */
const clock = (seconds) => `${String(Math.floor(seconds / 60)).padStart(2, "0")}:${String(seconds % 60).padStart(2, "0")}`;

/** @param {UpdateStatus} status */
function renderUpdate(status) {
  const box = $("#update", HTMLElement);
  const label = $("#update-label", HTMLElement);
  const detail = $("#update-detail", HTMLElement);
  box.dataset.state = status.running ? "running" : status.error ? "error" : status.finished && !status.cancelled ? "done" : "idle";
  $("#update-bar", HTMLElement).style.width = `${status.running || status.finished ? status.percent : 0}%`;
  updateButton.textContent = status.running ? "Cancel update" : "Update matches";
  updateButton.classList.toggle("primary", !status.running);
  box.title = status.error;

  if (status.running) {
    label.textContent = `Updating · ${status.percent}%`;
    const steps = status.resolved < status.resolveTotal
      ? `Riot IDs ${status.resolved}/${status.resolveTotal}`
      : `players ${status.players}/${status.playerTotal} · matches ${status.matches}/${status.matchTotal}`;
    detail.textContent = status.waitSeconds > 0 ? `Riot limit ${clock(status.waitSeconds)} · ${status.waitReason}` : steps;
  } else if (status.cancelled) {
    label.textContent = "Update cancelled";
    detail.textContent = "";
  } else if (status.keyRejected) {
    label.textContent = "Riot API key expired or invalid";
    detail.textContent = "data is saved · add a key in ⚙";
  } else if (status.error) {
    label.textContent = "Update finished with errors";
    detail.textContent = status.error;
  } else if (status.finished) {
    label.textContent = "Update complete";
    detail.textContent = `${status.matches} matches checked`;
  } else {
    label.textContent = "No update running";
    detail.textContent = "";
  }
}

let wasRunning = false;

async function pollUpdate() {
  try {
    const status = await api.updateStatus();
    renderUpdate(status);
    if (wasRunning && !status.running) {
      await refreshOverview();
      await runSearch();
      if (status.keyRejected) toast("Riot rejected the API key. Add a new key in Settings; nothing fetched so far is lost.", "error");
    }
    wasRunning = status.running;
  } catch (error) {
    showError(error);
  }
  setTimeout(pollUpdate, wasRunning ? 500 : 2000);
}

updateButton.addEventListener("click", async () => {
  try {
    if (wasRunning) {
      await api.cancelUpdate();
    } else {
      await api.startUpdate();
      wasRunning = true;
    }
    renderUpdate(await api.updateStatus());
  } catch (error) {
    showError(error);
    if (String(error).includes("key")) openSettings();
  }
});

// ── Settings ───────────────────────────────────────────────────────────────

async function openSettings() {
  settings.showModal();
  await renderSettings();
}

async function renderSettings() {
  try {
    const overview = await api.overview();
    renderKeys(overview.keys);
    $("#db-path", HTMLElement).textContent = overview.dbPath;
  } catch (error) {
    showError(error);
  }
}

async function openPlayerView() {
  if (settings.open) settings.close();
  matchLayout.hidden = true;
  playerView.hidden = false;
  await renderPlayers();
}

$("#players-button", HTMLButtonElement).addEventListener("click", openPlayerView);
$("#settings-players", HTMLButtonElement).addEventListener("click", openPlayerView);
$("#back-to-matches", HTMLButtonElement).addEventListener("click", () => {
  playerView.hidden = true;
  matchLayout.hidden = false;
});

async function renderPlayers() {
  try {
    const [overview, players] = await Promise.all([api.overview(), api.players()]);
    listedPlayers = players;
    const available = new Set(players.map(playerKey));
    for (const key of selectedPlayers) if (!available.has(key)) selectedPlayers.delete(key);
    if (!available.has(selectionAnchor)) selectionAnchor = "";
    const source = $("#import-form input[name=source]", HTMLInputElement);
    source.value ||= overview.playersSource;
    const region = $("#player-region", HTMLSelectElement);
    const selectedRegion = region.value || "euw1";
    region.replaceChildren(...overview.servers.map((server) => h("option", { value: server.id }, server.label)));
    region.value = selectedRegion;
    renderPlayerList();
  } catch (error) {
    showError(error);
  }
}

/** @param {string} value */
const normalized = (value) => value.toLocaleLowerCase("en-US").normalize("NFKD").replace(/[^\p{L}\p{N}]/gu, "");

/** @param {string} query @param {string} value */
function fuzzyScore(query, value) {
  const q = normalized(query);
  const text = normalized(value);
  if (!q) return 0;
  if (text === q) return 1000;
  if (text.startsWith(q)) return 800 - text.length;
  const index = text.indexOf(q);
  if (index >= 0) return 600 - index - text.length;
  let matched = 0, last = -2, score = 300;
  for (let i = 0; i < text.length && matched < q.length; i++) {
    if (text[i] !== q[matched]) continue;
    score += i === last + 1 ? 12 : -(i - last - 1);
    last = i;
    matched++;
  }
  return matched === q.length ? score - text.length : -1;
}

function renderPlayerList() {
  const query = $("#player-search", HTMLInputElement).value.trim();
  $("#players-summary", HTMLElement).textContent = `${listedPlayers.length} players · ${listedPlayers.filter((player) => player.enabled).length} enabled`;
  const ranked = listedPlayers.map((player) => ({ player, score: Math.max(...[player.riotId, player.server, ...player.tags, ...player.champions].map((value) => fuzzyScore(query, value))) }))
    .filter(({ score }) => score >= 0)
    .sort((a, b) => b.score - a.score || a.player.riotId.localeCompare(b.player.riotId));
  visiblePlayers = ranked.map(({ player }) => player);
  $("#players", HTMLUListElement).replaceChildren(...ranked.map(({ player }) => {
      const [gameName, tagLine] = splitRiotID(player.riotId);
      return h("li", { className: `${player.enabled ? "" : "disabled"}${selectedPlayers.has(playerKey(player)) ? " selected" : ""}`.trim(), onclick: (event) => {
        if (event.target instanceof Element && event.target.closest("button")) return;
        selectPlayer(player, event);
      } },
        h("input", { type: "checkbox", checked: selectedPlayers.has(playerKey(player)), ariaLabel: `Select ${player.riotId}`, onclick: (event) => {
          event.stopPropagation();
          selectPlayer(player, event);
        } }),
        h("div", { className: "player-info" },
          h("b", {}, player.riotId),
          h("span", { className: "muted" }, ` · ${player.server} · ${player.pending ? "pending" : player.rank}${player.enabled ? "" : " · paused"}`),
          player.tags.length || player.champions.length ? h("span", { className: "tags" },
            [player.tags.length ? `Tags: ${player.tags.join(", ")}` : "", player.champions.length ? `Champions: ${player.champions.join(", ")}` : ""].filter(Boolean).join(" · ")) : "",
        ),
        h("div", { className: "player-actions" },
          h("button", { type: "button", className: "link", onclick: () => editPlayer(player) }, "Edit"),
          h("button", { type: "button", className: "link", onclick: () => togglePlayer(player) }, player.enabled ? "Pause" : "Enable"),
          h("button", { type: "button", className: "link loss", onclick: () => removePlayer(gameName, tagLine, player.region) }, "Delete"),
        ),
      );
    }));
  if (ranked.length === 0) $("#players", HTMLUListElement).append(h("li", { className: "muted" }, "No players found"));
  renderSelection();
}

$("#player-search", HTMLInputElement).addEventListener("input", renderPlayerList);

/** @param {PlayerRow} player */
const playerKey = (player) => `${player.region}\u0000${player.riotId.toLowerCase()}`;

/** @param {PlayerRow} player @param {MouseEvent} event */
function selectPlayer(player, event) {
  const key = playerKey(player);
  const index = visiblePlayers.findIndex((item) => playerKey(item) === key);
  const anchor = visiblePlayers.findIndex((item) => playerKey(item) === selectionAnchor);
  if (event.shiftKey && anchor >= 0) {
    for (let i = Math.min(index, anchor); i <= Math.max(index, anchor); i++) {
      const item = visiblePlayers[i];
      if (item) selectedPlayers.add(playerKey(item));
    }
  } else {
    if (selectedPlayers.has(key)) selectedPlayers.delete(key);
    else selectedPlayers.add(key);
    selectionAnchor = key;
  }
  renderPlayerList();
}

function renderSelection() {
  const count = selectedPlayers.size;
  $("#selected-count", HTMLElement).textContent = `${count} selected`;
  for (const id of ["pause-selected", "enable-selected", "delete-selected"]) $("#" + id, HTMLButtonElement).disabled = count === 0;
}

$("#select-visible", HTMLButtonElement).addEventListener("click", () => {
  for (const player of visiblePlayers) selectedPlayers.add(playerKey(player));
  const first = visiblePlayers[0];
  selectionAnchor = first ? playerKey(first) : "";
  renderPlayerList();
});
$("#clear-selection", HTMLButtonElement).addEventListener("click", () => {
  selectedPlayers.clear();
  selectionAnchor = "";
  renderPlayerList();
});

/** @returns {import("./api.js").PlayerIdentity[]} */
function selectedIdentities() {
  return listedPlayers.filter((player) => selectedPlayers.has(playerKey(player))).map((player) => {
    const [gameName, tagLine] = splitRiotID(player.riotId);
    return { gameName, tagLine, region: player.region };
  });
}

/** @param {boolean} enabled */
async function setSelectedEnabled(enabled) {
  const identities = selectedIdentities();
  if (identities.length === 0) return;
  try {
    await api.setPlayersEnabled(identities, enabled);
    selectedPlayers.clear();
    await Promise.all([renderPlayers(), refreshOverview()]);
    toast(`${identities.length} players ${enabled ? "enabled" : "paused"}`);
  } catch (error) { showError(error); }
}

$("#pause-selected", HTMLButtonElement).addEventListener("click", () => setSelectedEnabled(false));
$("#enable-selected", HTMLButtonElement).addEventListener("click", () => setSelectedEnabled(true));
$("#delete-selected", HTMLButtonElement).addEventListener("click", async () => {
  const identities = selectedIdentities();
  if (identities.length === 0 || !window.confirm(`Delete ${identities.length} selected players and their saved results?`)) return;
  try {
    await api.deletePlayers(identities);
    selectedPlayers.clear();
    resetPlayerForm();
    await Promise.all([renderPlayers(), refreshOverview()]);
    await runSearch();
    toast(`${identities.length} players deleted`);
  } catch (error) { showError(error); }
});

/** @param {string} riotId @returns {[string, string]} */
function splitRiotID(riotId) {
  const separator = riotId.lastIndexOf("#");
  return [riotId.slice(0, separator), riotId.slice(separator + 1)];
}

/** @param {PlayerRow} player */
function editPlayer(player) {
  editingPlayer = player;
  const [gameName, tagLine] = splitRiotID(player.riotId);
  $("#player-form input[name=gameName]", HTMLInputElement).value = gameName;
  $("#player-form input[name=tagLine]", HTMLInputElement).value = tagLine;
  $("#player-region", HTMLSelectElement).value = player.region;
  $("#player-form input[name=tags]", HTMLInputElement).value = player.tags.join(", ");
  $("#player-form input[name=champions]", HTMLInputElement).value = player.champions.join(", ");
  $("#player-form input[name=enabled]", HTMLInputElement).checked = player.enabled;
  $("#player-form input[name=gameName]", HTMLInputElement).readOnly = true;
  $("#player-form input[name=tagLine]", HTMLInputElement).readOnly = true;
  $("#player-region", HTMLSelectElement).disabled = true;
  $("#save-player", HTMLButtonElement).textContent = "Save player";
  $("#cancel-player-edit", HTMLButtonElement).hidden = false;
  playerForm.scrollIntoView({ block: "nearest" });
}

function resetPlayerForm() {
  editingPlayer = undefined;
  playerForm.reset();
  $("#player-form input[name=gameName]", HTMLInputElement).readOnly = false;
  $("#player-form input[name=tagLine]", HTMLInputElement).readOnly = false;
  $("#player-region", HTMLSelectElement).disabled = false;
  $("#player-region", HTMLSelectElement).value = "euw1";
  $("#save-player", HTMLButtonElement).textContent = "Add player";
  $("#cancel-player-edit", HTMLButtonElement).hidden = true;
}

$("#cancel-player-edit", HTMLButtonElement).addEventListener("click", resetPlayerForm);

/** @param {string} value */
const labels = (value) => value.split(",").map((item) => item.trim()).filter(Boolean);

playerForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const data = new FormData(playerForm);
  const button = $("#save-player", HTMLButtonElement);
  button.disabled = true;
  try {
    const old = editingPlayer;
    const [oldName, oldTag] = old ? splitRiotID(old.riotId) : ["", ""];
    await api.savePlayer(old ? oldName : String(data.get("gameName")), old ? oldTag : String(data.get("tagLine")),
      old ? old.region : String(data.get("region")), data.has("enabled"), labels(String(data.get("tags") ?? "")), labels(String(data.get("champions") ?? "")));
    resetPlayerForm();
    toast(old ? "Player saved" : "Player added. Run Update matches to fetch games.");
    await Promise.all([renderPlayers(), refreshOverview()]);
  } catch (error) {
    showError(error);
  } finally {
    button.disabled = false;
  }
});

/** @param {PlayerRow} player */
async function togglePlayer(player) {
  const [gameName, tagLine] = splitRiotID(player.riotId);
  try {
    await api.setPlayersEnabled([{ gameName, tagLine, region: player.region }], !player.enabled);
    await Promise.all([renderPlayers(), refreshOverview()]);
    toast(player.enabled ? "Player paused" : "Player enabled");
  } catch (error) { showError(error); }
}

/** @param {string} gameName @param {string} tagLine @param {string} region */
async function removePlayer(gameName, tagLine, region) {
  if (!window.confirm(`Delete ${gameName}#${tagLine} from tracked players and remove their saved results?`)) return;
  try {
    await api.deletePlayer(gameName, tagLine, region);
    resetPlayerForm();
    await Promise.all([renderPlayers(), refreshOverview()]);
    await runSearch();
    toast("Player deleted");
  } catch (error) { showError(error); }
}

$("#settings-button", HTMLButtonElement).addEventListener("click", openSettings);

/** @param {Key[]} keys */
function renderKeys(keys) {
  const list = $("#keys", HTMLUListElement);
  list.replaceChildren(...keys.map((key) => h("li", { className: key.rejected ? "rejected" : "" },
    h("b", {}, key.label),
    h("span", { className: "path" }, key.masked),
    h("span", { className: "key-status" }, key.fromEnv ? "from RIOT_API_KEY" : key.rejected ? `rejected · ${key.rejection}` : "ok"),
    key.fromEnv ? "" : h("button", { type: "button", className: "link", onclick: () => removeKey(key) }, "Remove"),
  )));
  if (keys.length === 0) list.append(h("li", { className: "muted" }, "No keys yet."));
}

/** @param {Key} key */
async function removeKey(key) {
  try {
    await api.removeRiotKey(key.id);
    await Promise.all([renderSettings(), refreshOverview()]);
  } catch (error) {
    showError(error);
  }
}

$("#key-form", HTMLFormElement).addEventListener("submit", async (event) => {
  event.preventDefault();
  const keyForm = $("#key-form", HTMLFormElement);
  const button = $("#key-form button", HTMLButtonElement);
  const data = new FormData(keyForm);
  button.disabled = true;
  try {
    const note = await api.addRiotKey(String(data.get("label") ?? ""), String(data.get("key") ?? ""));
    keyForm.reset();
    toast(note || "Key checked with Riot and saved");
    await Promise.all([renderSettings(), refreshOverview()]);
  } catch (error) {
    showError(error);
  } finally {
    button.disabled = false;
  }
});

$("#import-form", HTMLFormElement).addEventListener("submit", async (event) => {
  event.preventDefault();
  const button = $("#import-form button", HTMLButtonElement);
  button.disabled = true;
  try {
    const count = await api.importPlayers(String(new FormData($("#import-form", HTMLFormElement)).get("source") ?? ""));
    toast(`Imported ${count} Riot IDs. Run Update matches to fetch their games.`);
    await Promise.all([renderPlayers(), refreshOverview()]);
  } catch (error) {
    showError(error);
  } finally {
    button.disabled = false;
  }
});

$("#export-db", HTMLButtonElement).addEventListener("click", async () => {
  try {
    const path = await api.exportDatabase();
    if (path) toast(`Database exported to ${path}`);
  } catch (error) {
    showError(error);
  }
});

$("#import-db", HTMLButtonElement).addEventListener("click", async () => {
  try {
    const backup = await api.importDatabase();
    toast(backup ? `Database imported. Previous data backed up to ${backup}` : "Database imported");
    await Promise.all([renderSettings(), refreshOverview()]);
    if (!playerView.hidden) await renderPlayers();
    await runSearch();
  } catch (error) {
    showError(error);
  }
});

// ── Start ──────────────────────────────────────────────────────────────────

try {
  const [, filters] = await Promise.all([refreshOverview(), api.lastFilters()]);
  applyFilters(filters);
  await runSearch();
} catch (error) {
  showError(error);
}
pollUpdate();
