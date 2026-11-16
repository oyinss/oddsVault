-- 0039: the score when a paper pick was recorded.
--
-- The minute and corner count alone do not say whether the favourite was
-- leading. Storing the score at save makes the 60-minute game-state finding
-- usable on our own picks, not only on finished matches.

ALTER TABLE paper_picks ADD COLUMN home_score_at_save integer;
ALTER TABLE paper_picks ADD COLUMN away_score_at_save integer;
