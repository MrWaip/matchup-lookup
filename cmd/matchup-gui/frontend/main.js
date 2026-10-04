import * as api from "./api.js";
import { championCombobox } from "./combobox.js";
import { $, h, icon, showError, toast } from "./dom.js";

/** @import { Filters, Game, Overview, SearchResult, UpdateStatus } from "./api.js" */

const form = $("#filters", HTMLFormElement);
const regionSelect = $("#region", HTMLSelectElement);
const gamesBody = $("#games", HTMLTableSectionElement);
const tableWrap = $("#table-wrap", HTMLElement);
const details = $("#details", HTMLElement);
const settings = $("#settings", HTMLDialogElement);
const updateButton = $("#update-button", HTMLButtonElement);

const dateFormat = new Intl.DateTimeFormat(undefined, { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" });
const longDateFormat = new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" });

/** @type {Game[]} */
let games = [];
let selected = -1;
let searchRequest = 0;
/** @type {Overview | undefined} */
let currentOverview;

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
  const message = !overview.hasKey
    ? "Add your Riot API key to download matches."
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
    if (String(error).includes("API key")) openSettings();
  }
});

// ── Settings ───────────────────────────────────────────────────────────────

async function openSettings() {
  settings.showModal();
  await renderSettings();
}

async function renderSettings() {
  try {
    const [overview, players] = await Promise.all([api.overview(), api.players()]);
    $("#key-status", HTMLElement).textContent = overview.keyFromEnv
      ? "Using RIOT_API_KEY from the environment; a saved key is used only when it is unset."
      : overview.hasKey
        ? "A key is saved. Development keys expire after 24 hours; paste a new one here."
        : "No key yet. Get one at developer.riotgames.com.";
    const source = $("#import-form input[name=source]", HTMLInputElement);
    source.value ||= overview.playersSource;
    $("#db-path", HTMLElement).textContent = overview.dbPath;
    $("#players-summary", HTMLElement).textContent = `Tracked players: ${overview.players}` + (overview.pending > 0 ? ` · ${overview.pending} pending Riot ID checks` : "");
    $("#players", HTMLUListElement).replaceChildren(...players.map((player) => h("li", {},
      h("b", {}, player.riotId),
      h("span", { className: "muted" }, ` ${player.region} · ${player.pending ? "pending" : player.rank}`),
      player.champions.length > 0 ? h("span", { className: "tags" }, player.champions.join(", ")) : "",
    )));
  } catch (error) {
    showError(error);
  }
}

$("#settings-button", HTMLButtonElement).addEventListener("click", openSettings);

$("#key-form", HTMLFormElement).addEventListener("submit", async (event) => {
  event.preventDefault();
  const keyForm = $("#key-form", HTMLFormElement);
  try {
    await api.setRiotKey(String(new FormData(keyForm).get("key") ?? ""));
    keyForm.reset();
    toast("Riot API key saved");
    await Promise.all([renderSettings(), refreshOverview()]);
  } catch (error) {
    showError(error);
  }
});

$("#import-form", HTMLFormElement).addEventListener("submit", async (event) => {
  event.preventDefault();
  const button = $("#import-form button", HTMLButtonElement);
  button.disabled = true;
  try {
    const count = await api.importPlayers(String(new FormData($("#import-form", HTMLFormElement)).get("source") ?? ""));
    toast(`Imported ${count} Riot IDs. Run Update matches to fetch their games.`);
    await Promise.all([renderSettings(), refreshOverview()]);
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
