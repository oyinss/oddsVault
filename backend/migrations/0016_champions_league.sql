-- 0016: UEFA Champions League reference row.
INSERT INTO competitions (sport_id, name, slug, type)
SELECT id, 'UEFA Champions League', 'champions-league', 'cup'
FROM sports WHERE slug = 'football'
ON CONFLICT (sport_id, slug) DO NOTHING;
