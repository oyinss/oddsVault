-- 0019: coverage for competitions that appear in the personal bet journal.
--
-- Fixtures and results come from ESPN; these rows exist so journal legs can be
-- linked to real events and verified.

INSERT INTO countries (iso2, iso3, name, slug) VALUES
    ('AR', 'ARG', 'Argentina', 'argentina'),
    ('CL', 'CHL', 'Chile',     'chile'),
    ('PY', 'PRY', 'Paraguay',  'paraguay'),
    ('UY', 'URY', 'Uruguay',   'uruguay'),
    ('IT', 'ITA', 'Italy',     'italy')
ON CONFLICT DO NOTHING;

INSERT INTO competitions (sport_id, country_id, name, slug, type)
SELECT s.id, c.id, v.name, v.slug, 'league'
FROM (VALUES
    ('Argentine Primera División', 'argentina-primera', 'argentina'),
    ('Chilean Primera División',   'chile-primera',     'chile'),
    ('Paraguayan Primera División','paraguay-primera',  'paraguay'),
    ('Uruguayan Primera División', 'uruguay-primera',   'uruguay'),
    ('Italian Serie C',            'italy-serie-c',     'italy')
) AS v(name, slug, country_slug)
JOIN sports s ON s.slug = 'football'
JOIN countries c ON c.slug = v.country_slug
ON CONFLICT (sport_id, slug) DO NOTHING;
