# MatchupFinder.gg setup and usage

MatchupFinder.gg finds recent League of Legends ranked Solo/Duo lane matchups among the players you track. Add a Riot API key and a player list once, update matches, and search for games worth watching.

## Get a Riot API key

1. Sign in at the [Riot Developer Portal](https://developer.riotgames.com/). Riot gives you a key on your portal dashboard.
2. In MatchupFinder.gg, open **Settings → Riot API keys**. Paste the key and click **Add key**.
3. If an update says the key expired or was rejected, return to the portal, reset or copy a current key, and add it in Settings. Data already collected remains available to search.

Riot's standard portal key expires every 24 hours. For longer personal use, Riot describes [personal API keys and how to request one](https://developer.riotgames.com/docs/portal). Your key is saved on this computer; keep database exports private because they include it.

## Choose players to track

**Try the included list:** In **Players**, keep the source already shown in the import field and click **Import**. It contains 200 Fiora players on EUW. Importing a list only adds Riot IDs; **Update matches** fetches their games.

**Make your own list:** Collect Riot IDs from your League friends list, match history, or players you want to study. A Riot ID has a game name and tag, such as `ExamplePlayer#EUW`. The tag is part of the ID; the `region` field is the server where that player plays. [Riot explains the two parts of a Riot ID](https://support.riotgames.com/en-us/riot/account/changing-your-riot-id).

Save a UTF-8 CSV file such as `players.csv`:

```csv
gameName,tagLine,region,champion
ExamplePlayer,EUW,euw1,Fiora
AnotherPlayer,NA1,na1,Garen
```

Or save a JSON file such as `players.json`:

```json
[
  {"gameName":"ExamplePlayer","tagLine":"EUW","region":"euw1","champion":"Fiora"},
  {"gameName":"AnotherPlayer","tagLine":"NA1","region":"na1","champion":"Garen"}
]
```

Replace the example IDs with real ones. Enter the file path in **Players** and click **Import**. You can also import a folder of JSON/CSV files or an HTTP(S) link to one. `champion` is an optional label for organizing your list; search filters use the champion actually played in each match. Reimport a list when you add players. You can also add one Riot ID directly in **Players** and set optional tags and champions. Use Pause to skip a player in future updates without losing saved games; select multiple players for bulk Pause, Enable, or Delete.

## Search and watch

Click **Update matches** after importing players. The first update can take a while for a large list because Riot limits request rates. Later updates keep what has already been collected.

Set **Your champion** and **Opponent** to find a specific lane, then use **Result**, **KDA vs lane opponent**, **Player rank**, **Server**, or **More filters** to narrow the games. For example, choose Fiora vs Darius, Win, and Diamond+ to find recent wins to review. You can also search from the opponent's side of a stored match.

Select a result to see the Riot IDs, match stats, summoner spells, and runes. Click **Watch replay** with the League Client running on the same server as that game. A replay may be unavailable if Riot has removed it or the patch has changed.

If you see no results, clear some filters, check that your player list imported, and run **Update matches**. After a new patch, update again to find games with watchable replays.
