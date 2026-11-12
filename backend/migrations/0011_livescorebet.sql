-- 0011: first live bookmaker.
--
-- LiveScoreBet's public gateway returns pre-match markets with decimal odds,
-- so this can be a real collector rather than a browser capture.

INSERT INTO bookmakers (name, slug, country_scope, base_url)
VALUES ('LiveScoreBet', 'livescorebet', 'nigeria', 'https://www.livescorebet.com/ng/')
ON CONFLICT (slug) DO NOTHING;

-- The provider publishes a wall-clock time with no timezone. Keep it verbatim
-- and leave scheduled_at NULL until a UTC kickoff is verified.
ALTER TABLE bookmaker_events ADD COLUMN raw_start_time text;
