-- 0038: goal timelines and attacking stats.
--
-- Corner counts alone cannot say why a match produced them. Goal minutes let a
-- match be reconstructed: who led, when, and for how long. Shots, shots on
-- target, big chances and xG are the attacking pressure behind the corners.
-- Everything is provider data; missing values stay NULL.

CREATE TABLE match_goal_events (
    id                 bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id           bigint NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    minute             integer NOT NULL CHECK (minute >= 0 AND minute <= 130),
    is_home            boolean NOT NULL,
    home_score_before  integer,
    away_score_before  integer,
    player             text,
    own_goal           boolean NOT NULL DEFAULT false,
    penalty_shootout   boolean NOT NULL DEFAULT false,
    provider           text NOT NULL DEFAULT 'fotmob',
    UNIQUE (event_id, minute, is_home, player)
);

CREATE INDEX match_goal_events_event_idx ON match_goal_events (event_id, minute);

CREATE TABLE team_attack_stats (
    event_id        bigint NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    team_id         bigint NOT NULL REFERENCES teams (id) ON DELETE CASCADE,
    shots           integer,
    shots_on_target integer,
    big_chances     integer,
    passes          integer,
    xg              numeric(5, 2),
    possession      numeric(5, 2),
    provider        text NOT NULL DEFAULT 'fotmob',
    captured_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (event_id, team_id)
);
