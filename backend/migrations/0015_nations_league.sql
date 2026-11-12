-- 0015: UEFA Nations League, covered for identity (bets and bookmaker odds).
INSERT INTO competitions (sport_id, name, slug, type)
SELECT id, 'UEFA Nations League', 'nations-league', 'international'
FROM sports WHERE slug = 'football'
ON CONFLICT (sport_id, slug) DO NOTHING;
