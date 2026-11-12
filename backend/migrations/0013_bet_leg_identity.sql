-- 0013: parse bet legs and add the first extra competition needed to link them.

ALTER TABLE personal_bet_legs
    ADD COLUMN home_raw text,
    ADD COLUMN away_raw text,
    ADD COLUMN market_hint text,
    ADD COLUMN resolution_note text;

COMMENT ON COLUMN personal_bet_legs.market_hint IS
    'Market family visible in the provider wording (e.g. corners). Never a selection.';

INSERT INTO competitions (sport_id, country_id, name, slug, type)
SELECT s.id, c.id, 'Copa del Rey', 'copa-del-rey', 'cup'
FROM sports s, countries c
WHERE s.slug = 'football' AND c.slug = 'spain'
ON CONFLICT (sport_id, slug) DO NOTHING;
