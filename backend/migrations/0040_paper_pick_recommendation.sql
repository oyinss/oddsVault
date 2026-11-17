-- 0040: freeze the board's own recommendation with each paper pick.
--
-- The card already suggested the best over and under line from the projection
-- band. Storing that at save time makes "did I follow the read" a recorded
-- fact rather than something reconstructed later from the band.
ALTER TABLE paper_picks
    ADD COLUMN rec_over_line  numeric(6, 2),
    ADD COLUMN rec_under_line numeric(6, 2);

COMMENT ON COLUMN paper_picks.rec_over_line IS
    'Highest standard over line below the low end of the projection band at save time, or null.';
COMMENT ON COLUMN paper_picks.rec_under_line IS
    'Lowest standard under line above the high end of the projection band at save time, or null.';

-- Backfill from the band already stored on older picks, using the same rule the
-- card applied, so their alignment can be read without guessing.
UPDATE paper_picks
SET rec_over_line = band.over_line,
    rec_under_line = band.under_line
FROM (
    SELECT p.id,
           (
               SELECT max(line)
               FROM generate_series(6.5::numeric, 15.5::numeric, 1) AS line
               WHERE p.projection_low IS NOT NULL AND line < p.projection_low
           ) AS over_line,
           (
               SELECT min(line)
               FROM generate_series(6.5::numeric, 15.5::numeric, 1) AS line
               WHERE p.projection_high IS NOT NULL AND line > p.projection_high
           ) AS under_line
    FROM paper_picks p
) AS band
WHERE band.id = paper_picks.id;
