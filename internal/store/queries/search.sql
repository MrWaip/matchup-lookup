-- name: SearchGames :many
-- Every tracked game is searchable from both sides: the tracked player's and,
-- unless the opponent is tracked too, the lane opponent's (listed second). The opponent's rank
-- is known only when the opponent is a tracked player. Empty/zero parameters
-- disable their filter. Results are newest first.
WITH perspectives AS (
    SELECT g.match_id, g.player_puuid, g.opponent_puuid,
           g.player_rank_tier AS rank_tier, g.player_rank_division AS rank_division,
           g.matchup_status, 0 AS opponent_view
    FROM tracked_games g
    UNION ALL
    SELECT g.match_id, g.opponent_puuid, g.player_puuid,
           COALESCE(rp.rank_tier, ''), COALESCE(rp.rank_division, ''),
           g.matchup_status, 1
    FROM tracked_games g
    LEFT JOIN players rp ON rp.puuid = g.opponent_puuid
    WHERE g.opponent_puuid <> ''
      AND NOT EXISTS (SELECT 1 FROM tracked_games own
                      WHERE own.match_id = g.match_id AND own.player_puuid = g.opponent_puuid)
)
SELECT m.match_id, m.platform, m.game_version, m.game_creation,
       me.riot_id_game_name, me.riot_id_tagline,
       COALESCE(p.game_name, '') AS player_game_name, COALESCE(p.tag_line, '') AS player_tag_line,
       g.rank_tier, g.rank_division, g.matchup_status,
       me.champion, me.win, me.kills, me.deaths, me.assists, me.cs,
       me.summoner1_id, me.summoner2_id, me.keystone, me.secondary_rune1, me.secondary_rune2,
       opp.champion AS opponent_champion,
       opp.riot_id_game_name AS opponent_game_name, opp.riot_id_tagline AS opponent_tag_line,
       opp.kills AS opponent_kills, opp.deaths AS opponent_deaths, opp.assists AS opponent_assists
FROM perspectives g
JOIN matches m ON m.match_id = g.match_id
JOIN match_participants me ON me.match_id = g.match_id AND me.puuid = g.player_puuid
LEFT JOIN match_participants opp ON opp.match_id = g.match_id AND opp.puuid = g.opponent_puuid
LEFT JOIN players p ON p.puuid = g.player_puuid
WHERE (CAST(@region AS TEXT) = '' OR m.platform = @region COLLATE NOCASE)
  AND (CAST(@champion AS TEXT) = '' OR me.champion = @champion COLLATE NOCASE)
  AND (CAST(@opponent AS TEXT) = '' OR opp.champion = @opponent COLLATE NOCASE)
  AND (CAST(@result AS TEXT) = '' OR me.win = (@result = 'win'))
  -- Exact KDA ratio comparison without floating point; unknown opponents never pass.
  AND (CAST(@kda AS TEXT) = '' OR (opp.kills IS NOT NULL AND (
        (@kda = 'ge' AND (me.kills + me.assists) * MAX(1, opp.deaths) >= (opp.kills + opp.assists) * MAX(1, me.deaths))
     OR (@kda = 'gt' AND (me.kills + me.assists) * MAX(1, opp.deaths) > (opp.kills + opp.assists) * MAX(1, me.deaths)))))
  AND (CAST(@min_rank AS INTEGER) = 0 OR CASE UPPER(g.rank_tier)
        WHEN 'EMERALD' THEN 1 WHEN 'DIAMOND' THEN 2 WHEN 'MASTER' THEN 3
        WHEN 'GRANDMASTER' THEN 4 WHEN 'CHALLENGER' THEN 5 ELSE 0 END >= @min_rank)
  AND m.game_creation >= @created_since
  AND m.game_duration >= @min_duration
  AND (CAST(@patch_major AS INTEGER) = 0 OR (m.patch_major = @patch_major AND m.patch_minor = @patch_minor))
  AND (CAST(@player AS TEXT) = '' OR me.riot_id_game_name = @player COLLATE NOCASE
       OR p.game_name = @player COLLATE NOCASE OR g.player_puuid = @player)
ORDER BY m.game_creation DESC, m.match_id DESC, g.opponent_view;
