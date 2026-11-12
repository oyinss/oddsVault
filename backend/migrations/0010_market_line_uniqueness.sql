-- 0010: markets with a NULL line must still be unique.
--
-- PostgreSQL treats NULLs as distinct in unique constraints, which would allow
-- two "match result" markets for one event. NULLS NOT DISTINCT closes that.

ALTER TABLE markets DROP CONSTRAINT markets_unique;
ALTER TABLE markets
    ADD CONSTRAINT markets_unique
    UNIQUE NULLS NOT DISTINCT (event_id, market_type_id, period, line, subject);
