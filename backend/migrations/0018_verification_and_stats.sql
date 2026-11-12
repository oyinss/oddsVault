-- 0018: independent verification of journal legs against provider stats.
--
-- Corner totals are the first verified market: team_match_stats stores the
-- provider's corner counts, and each journal leg gets the result computed from
-- those numbers alongside the bookmaker's own verdict.

CREATE TABLE team_match_stats (
    event_id    bigint NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    team_id     bigint NOT NULL REFERENCES teams (id) ON DELETE CASCADE,
    corners     integer,
    provider    text NOT NULL,
    captured_at timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT team_match_stats_corners_check CHECK (corners IS NULL OR corners >= 0),
    PRIMARY KEY (event_id, team_id)
);

CREATE TRIGGER team_match_stats_set_updated_at
    BEFORE UPDATE ON team_match_stats
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE personal_bet_legs
    ADD COLUMN verified_status text,
    ADD COLUMN verified_value text,
    ADD COLUMN verification_note text,
    ADD COLUMN verified_at timestamptz;

COMMENT ON COLUMN personal_bet_legs.verified_status IS
    'Result recomputed from provider stats; NULL when no verified data exists.';
