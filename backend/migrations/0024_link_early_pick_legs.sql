-- 0024: link early model picks to the selection they priced.
--
-- Runs created before 0023 stored the event, market and selection but not the
-- selection id. The pick itself is unchanged: this only records which row was
-- priced, and only where exactly one selection matches, so an ambiguous leg
-- stays unlinked and pending rather than being guessed.

UPDATE model_bet_legs ml
SET selection_id = match.only_id
FROM (
    SELECT ml2.id AS leg_id, min(se.id) AS only_id
    FROM model_bet_legs ml2
    JOIN markets m ON m.event_id = ml2.event_id
    JOIN market_types mt ON mt.id = m.market_type_id AND mt.slug = ml2.market_type
    JOIN market_selections se ON se.market_id = m.id AND se.selection = ml2.selection
    WHERE ml2.selection_id IS NULL
    GROUP BY ml2.id
    HAVING count(*) = 1
) match
WHERE ml.id = match.leg_id;
