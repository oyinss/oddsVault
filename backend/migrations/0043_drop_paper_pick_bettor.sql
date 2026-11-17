-- 0043: remove the paper pick bettor column.
--
-- Attribution was added for a slip import but paper picks are board recordings
-- with no owner, so the column goes rather than sitting there empty.
ALTER TABLE paper_picks DROP COLUMN IF EXISTS placed_by;
