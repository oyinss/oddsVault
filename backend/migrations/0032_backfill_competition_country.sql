-- 0032: derive a competition's country from its league name.
--
-- Leagues discovered through 1xBet arrive with a name like
-- "Serbia. Srpska Liga-East". Where the country is still missing, the prefix
-- before the first dot is the country, and it is matched to the reference row
-- by slug. A prefix with no match is left alone rather than guessed.

UPDATE competitions c
SET country_id = co.id
FROM countries co
WHERE c.country_id IS NULL
  AND c.onexbet_league_id IS NOT NULL
  AND co.slug = trim(both '-' from lower(regexp_replace(
      CASE lower(split_part(c.name, '.', 1))
          WHEN 'uae' THEN 'united arab emirates'
          WHEN 'usa' THEN 'united states'
          WHEN 'dr congo' THEN 'dr-congo'
          ELSE split_part(c.name, '.', 1)
      END, '[^a-zA-Z0-9]+', '-', 'g')));
