-- 0006: canonical football reference rows.
--
-- Countries and competitions are product configuration, not sporting events.
-- Seasons are created by the importer when a provider season is first synced.

INSERT INTO countries (iso2, iso3, name, slug) VALUES
    ('GB', 'GBR', 'England',        'england'),
    ('ES', 'ESP', 'Spain',          'spain'),
    ('IT', 'ITA', 'Italy',          'italy'),
    ('DE', 'DEU', 'Germany',        'germany'),
    ('FR', 'FRA', 'France',         'france')
ON CONFLICT DO NOTHING;

INSERT INTO competitions (sport_id, country_id, name, slug, type)
SELECT s.id, c.id, v.name, v.slug, v.type
FROM (VALUES
    ('Premier League', 'premier-league', 'league',     'england'),
    ('La Liga',        'la-liga',        'league',     'spain'),
    ('Serie A',        'serie-a',        'league',     'italy'),
    ('Bundesliga',     'bundesliga',     'league',     'germany'),
    ('Ligue 1',        'ligue-1',        'league',     'france')
) AS v(name, slug, type, country_slug)
JOIN sports s ON s.slug = 'football'
JOIN countries c ON c.slug = v.country_slug
ON CONFLICT (sport_id, slug) DO NOTHING;
