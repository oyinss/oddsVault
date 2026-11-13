-- 0030: 1xBet league id on competitions.
--
-- 1xBet is the bookmaker the user actually plays, and its live feed covers
-- leagues no other accessible provider lists. The league id keeps one row per
-- league instead of a new competition on every poll.

ALTER TABLE competitions ADD COLUMN onexbet_league_id bigint;

CREATE UNIQUE INDEX competitions_onexbet_league_idx
    ON competitions (onexbet_league_id)
    WHERE onexbet_league_id IS NOT NULL;
