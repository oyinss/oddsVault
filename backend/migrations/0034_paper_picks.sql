-- 0034: paper picks.
--
-- Paper picks are corner selections recorded by hand, with no stake. They are
-- deliberately separate from the model's agent picks and from the personal bet
-- journal so a pick recorded for learning can never be mistaken for money at
-- risk. The board context at save time is frozen with the pick: it is training
-- data, and back-filling it later would invent information the picker never had.

CREATE TABLE paper_picks (
    id                    bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id              bigint NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    competition_id        bigint NOT NULL REFERENCES competitions (id),
    home_team_id          bigint NOT NULL REFERENCES teams (id),
    away_team_id          bigint NOT NULL REFERENCES teams (id),
    market                text NOT NULL DEFAULT 'corners_total'
                          CHECK (market = 'corners_total'),
    side                  text NOT NULL CHECK (side IN ('over', 'under')),
    line                  numeric(5, 2) NOT NULL CHECK (line >= 0),
    odds                  numeric(6, 3) NOT NULL CHECK (odds > 1),
    -- Board state at the moment of recording.
    minute                integer,
    home_corners          integer,
    away_corners          integer,
    total_corners         integer,
    pace_per_minute       numeric(6, 3),
    projected_total       numeric(6, 2),
    projection_low        numeric(6, 2),
    projection_high       numeric(6, 2),
    needed                integer,
    needed_per_minute     numeric(6, 3),
    model_risk            boolean NOT NULL DEFAULT false,
    early_pace            boolean NOT NULL DEFAULT false,
    event_status_at_save  text NOT NULL,
    data_cutoff_at        timestamptz NOT NULL,
    created_at            timestamptz NOT NULL DEFAULT now(),
    -- Settlement.
    status                text NOT NULL DEFAULT 'pending'
                          CHECK (status IN ('pending', 'won', 'lost', 'push', 'void')),
    final_total           integer,
    settled_at            timestamptz,
    settlement_rule       text
);

CREATE INDEX paper_picks_status_idx ON paper_picks (status, created_at DESC);
CREATE INDEX paper_picks_event_idx ON paper_picks (event_id);
CREATE INDEX paper_picks_competition_idx ON paper_picks (competition_id);
