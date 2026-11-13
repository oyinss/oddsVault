-- 0028: ESPN league codes on competitions.
--
-- ESPN identifies a competition by a code such as eng.1 or caf.nations_qual.
-- Storing it lets live discovery find the canonical competition instead of
-- creating a duplicate row every time it meets the same league.

ALTER TABLE competitions ADD COLUMN espn_code text;

CREATE UNIQUE INDEX competitions_espn_code_idx
    ON competitions (espn_code)
    WHERE espn_code IS NOT NULL;

UPDATE competitions c
SET espn_code = v.code
FROM (VALUES
    ('premier-league', 'eng.1'),
    ('la-liga', 'esp.1'),
    ('serie-a', 'ita.1'),
    ('bundesliga', 'ger.1'),
    ('ligue-1', 'fra.1'),
    ('copa-del-rey', 'esp.copa_del_rey'),
    ('nations-league', 'uefa.nations'),
    ('champions-league', 'uefa.champions'),
    ('argentina-primera', 'arg.1'),
    ('chile-primera', 'chi.1'),
    ('paraguay-primera', 'par.1'),
    ('uruguay-primera', 'uru.1'),
    ('italy-serie-c', 'ita.3')
) AS v(slug, code)
WHERE c.slug = v.slug
  AND c.sport_id = (SELECT id FROM sports WHERE slug = 'football');
