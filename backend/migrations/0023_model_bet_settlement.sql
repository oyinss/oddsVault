-- 0023: link model pick legs to the selection they priced, so a pick can be
-- resolved from the same deterministic settlement as any other selection.
--
-- Existing legs keep a null selection_id and stay pending: a pick is never
-- back-filled with a selection that was not recorded at generation time.

ALTER TABLE model_bet_legs
    ADD COLUMN selection_id bigint REFERENCES market_selections (id) ON DELETE SET NULL;

ALTER TABLE model_bets
    ADD COLUMN resolved_at timestamptz;

CREATE INDEX model_bet_legs_selection_idx ON model_bet_legs (selection_id);
