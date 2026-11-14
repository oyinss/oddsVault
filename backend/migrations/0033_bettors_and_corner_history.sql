-- 0033: bettor attribution and corner history.
--
-- Bets above NGN 2,500 were placed by Emife, the rest by Oyins. Agent picks
-- will use the same column later. Corner snapshots record what the board showed
-- while a match was live, so projections can be scored against the final count.

ALTER TABLE personal_bets ADD COLUMN placed_by text;

UPDATE personal_bets
SET placed_by = CASE WHEN stake > 2500 THEN 'emife' ELSE 'oyins' END;

CREATE INDEX personal_bets_placed_by_idx ON personal_bets (placed_by, placed_at DESC);

CREATE TABLE corner_snapshots (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id        bigint NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    minute          integer,
    home_corners    integer,
    away_corners    integer,
    total_corners   integer NOT NULL,
    projected_total numeric(6, 2),
    captured_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX corner_snapshots_event_idx
    ON corner_snapshots (event_id, captured_at DESC);
