-- 0035: the bookmaker behind a paper pick.
--
-- A paper pick records the price that was actually available, and a price only
-- means something with the bookmaker that offered it. The list comes from the
-- same bookmakers table the odds snapshots use, so the picker picks a name
-- instead of typing one.

ALTER TABLE paper_picks
    ADD COLUMN provider_slug text NOT NULL DEFAULT '1xbet'
        REFERENCES bookmakers (slug) ON UPDATE CASCADE;

CREATE INDEX paper_picks_provider_idx ON paper_picks (provider_slug, created_at DESC);
