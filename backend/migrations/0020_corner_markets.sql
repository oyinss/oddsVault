-- 0020: corner markets.
--
-- Corner results already live in team_match_stats (verification). This adds the
-- canonical market types so imported corner prices can be settled by the same
-- deterministic engine.

INSERT INTO market_types (slug, name, sport_id)
SELECT v.slug, v.name, s.id
FROM (VALUES
    ('total_corners',    'Total corners'),
    ('corners_handicap', 'Corners handicap')
) AS v(slug, name)
CROSS JOIN sports s
WHERE s.slug = 'football'
ON CONFLICT (slug) DO NOTHING;
