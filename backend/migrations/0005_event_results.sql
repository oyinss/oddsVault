-- 0005: verified event results.
--
-- NULL means the provider did not report that period. 0 means the provider
-- explicitly reported zero. Never substitute one for the other.

CREATE TABLE event_results (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id          bigint NOT NULL UNIQUE REFERENCES events (id) ON DELETE CASCADE,
    home_score        integer,
    away_score        integer,
    home_score_ht     integer,
    away_score_ht     integer,
    home_score_et     integer,
    away_score_et     integer,
    home_penalties    integer,
    away_penalties    integer,
    provider          text NOT NULL,
    source_updated_at timestamptz,
    captured_at       timestamptz NOT NULL DEFAULT now(),
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT event_results_home_check CHECK (home_score IS NULL OR home_score >= 0),
    CONSTRAINT event_results_away_check CHECK (away_score IS NULL OR away_score >= 0),
    CONSTRAINT event_results_ht_check CHECK (
        (home_score_ht IS NULL OR home_score_ht >= 0) AND
        (away_score_ht IS NULL OR away_score_ht >= 0)
    ),
    CONSTRAINT event_results_et_check CHECK (
        (home_score_et IS NULL OR home_score_et >= 0) AND
        (away_score_et IS NULL OR away_score_et >= 0)
    ),
    CONSTRAINT event_results_pen_check CHECK (
        (home_penalties IS NULL OR home_penalties >= 0) AND
        (away_penalties IS NULL OR away_penalties >= 0)
    )
);

COMMENT ON TABLE event_results IS
    'Verified final scores. Provider values only; calculated market outcomes live elsewhere.';

CREATE TRIGGER event_results_set_updated_at
    BEFORE UPDATE ON event_results
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
