-- 0029: FotMob league id on competitions.
--
-- FotMob identifies a league by a numeric id. Discovery uses it as the
-- competition key, so a league seen once is never duplicated on the next poll.

ALTER TABLE competitions ADD COLUMN fotmob_league_id bigint;

CREATE UNIQUE INDEX competitions_fotmob_league_idx
    ON competitions (fotmob_league_id)
    WHERE fotmob_league_id IS NOT NULL;
