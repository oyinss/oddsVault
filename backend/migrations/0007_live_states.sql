-- 0007: live match state.
--
-- One current row per event: the source of truth for what the provider reports
-- right now. Missing clock/period values stay NULL; 0-0 is a reported score.

CREATE TABLE event_live_states (
    event_id       bigint PRIMARY KEY REFERENCES events (id) ON DELETE CASCADE,
    provider       text NOT NULL,
    status         text NOT NULL,
    home_score     integer,
    away_score     integer,
    minute         text,
    period         text,
    clock          text,
    detail         text,
    source_updated_at timestamptz,
    captured_at    timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT event_live_states_status_check CHECK (status IN (
        'scheduled', 'live', 'finished', 'postponed', 'cancelled', 'abandoned', 'unknown'
    )),
    CONSTRAINT event_live_states_home_check CHECK (home_score IS NULL OR home_score >= 0),
    CONSTRAINT event_live_states_away_check CHECK (away_score IS NULL OR away_score >= 0)
);

COMMENT ON TABLE event_live_states IS 'Latest provider-reported state for an in-progress or recently finished event.';

CREATE TRIGGER event_live_states_set_updated_at
    BEFORE UPDATE ON event_live_states
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
