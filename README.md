# Matchup Lookup

**Find League of Legends lane matchups you can actually study.** Matchup Lookup searches recent ranked Solo/Duo games from a player list you choose. Filter by champion, lane opponent, server, result, KDA, and rank; inspect the player's spells and runes; then open a current-patch replay in the League Client.

Use it to find a Fiora win against Darius, compare how strong players approach a difficult lane, or review a loss from the opponent's point of view. Results are sorted newest first, and the app remembers your filters.

## Get the app

Download [Matchup Lookup for Windows](https://github.com/MrWaip/matchup-lookup/releases/download/nightly/matchup-lookup-gui.exe) and open the `.exe`. Prefer a terminal? Download the [command-line version](https://github.com/MrWaip/matchup-lookup/releases/download/nightly/matchup-lookup.exe).

## Start finding games

1. Get a Riot API key from the [Riot Developer Portal](https://developer.riotgames.com/). Sign in with your Riot account, copy the key, and add it under **Settings → Riot API keys**.
2. Open **Settings → Players** and import the included Fiora player list, or provide your own JSON/CSV file, folder, or URL.
3. Click **Update matches** to fetch recent ranked Solo/Duo games for those players.
4. Choose **Your champion** and **Opponent**, then narrow the results with the other filters. Select a game to see its details and click **Watch replay**.

The League Client must be open on the match's server to watch a replay. Riot replays are available only for the current patch; Matchup Lookup searches that patch by default.

**Need a player list or a fresh key?** See the [setup and usage guide](DEVELOPMENT.md) for exact sources, file examples, and common fixes.

Matchup Lookup uses Riot Games data but is not endorsed by Riot Games.
