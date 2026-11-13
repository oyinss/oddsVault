-- 0026: moneyline for two-way sports.
--
-- Basketball and American football price a winner, not a 1X2. The market is
-- one shared type with no sport owner: the same settlement rule applies to any
-- two-way event.

INSERT INTO market_types (slug, name, sport_id)
VALUES ('moneyline', 'Moneyline', NULL)
ON CONFLICT (slug) DO NOTHING;
