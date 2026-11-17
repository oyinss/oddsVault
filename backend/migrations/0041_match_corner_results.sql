-- 0041: store verified final corner results and their sources.
--
-- Provider feeds miss whole competitions (friendlies, lower divisions), which
-- left bets pending with no way to settle without researching the same match
-- twice. A researched result lands here with the URL it came from, so the next
-- settlement reuses it instead of repeating the search.
CREATE TABLE match_corner_results (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id            bigint REFERENCES events (id) ON DELETE SET NULL,
    match_date          date NOT NULL,
    home_name           text NOT NULL,
    away_name           text NOT NULL,
    competition         text,
    home_goals          integer,
    away_goals          integer,
    home_corners        integer NOT NULL,
    away_corners        integer NOT NULL,
    total_corners       integer GENERATED ALWAYS AS (home_corners + away_corners) STORED,
    source_url          text NOT NULL,
    source_name         text NOT NULL,
    verification_status text NOT NULL,
    extracted_at        timestamptz NOT NULL DEFAULT now(),
    note                text,
    CONSTRAINT match_corner_results_status_check CHECK (verification_status IN (
        'verified', 'single_source', 'conflict', 'unavailable'
    )),
    CONSTRAINT match_corner_results_corners_check CHECK (home_corners >= 0 AND away_corners >= 0),
    CONSTRAINT match_corner_results_unique UNIQUE (match_date, home_name, away_name)
);

CREATE INDEX match_corner_results_date_idx ON match_corner_results (match_date DESC);
CREATE INDEX match_corner_results_event_idx ON match_corner_results (event_id);

COMMENT ON TABLE match_corner_results IS
    'Researched final corner counts with the URL they came from. Verified means two independent sources agreed.';
