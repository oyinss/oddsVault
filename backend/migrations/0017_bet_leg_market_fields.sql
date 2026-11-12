-- 0017: structured market data for journal legs.
--
-- "Total. Under (7). Corners 1.39" becomes
--   family corners_total, side under, line 7.
-- market_raw is always kept as evidence.

ALTER TABLE personal_bet_legs
    ADD COLUMN market_family text,
    ADD COLUMN market_side text,
    ADD COLUMN market_line numeric(6, 2);

COMMENT ON COLUMN personal_bet_legs.market_family IS
    'Normalized family parsed from the provider text, e.g. corners_total.';
COMMENT ON COLUMN personal_bet_legs.market_line IS
    'Line/handicap value visible in the provider text; NULL when absent.';
