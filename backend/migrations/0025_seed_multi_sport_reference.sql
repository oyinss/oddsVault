-- 0025: basketball and American football reference rows.
--
-- Only reference data: a country and two competitions. No games, teams or
-- results are seeded; those are imported from the provider.

INSERT INTO countries (iso2, iso3, name, slug)
VALUES ('US', 'USA', 'United States', 'usa')
ON CONFLICT DO NOTHING;

INSERT INTO competitions (sport_id, country_id, name, slug, type)
SELECT s.id, c.id, v.name, v.slug, 'league'
FROM (VALUES
    ('NBA', 'nba', 'basketball'),
    ('NFL', 'nfl', 'american-football')
) AS v(name, slug, sport_slug)
JOIN sports s ON s.slug = v.sport_slug
JOIN countries c ON c.slug = 'usa'
ON CONFLICT (sport_id, slug) DO NOTHING;
