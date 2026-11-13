-- 0021: strategies, frozen model runs and picks.
--
-- A pick is written once, with the data cutoff and odds used at generation
-- time. Later results never rewrite it; a new strategy version or run is added.

CREATE TABLE strategy_definitions (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug        text NOT NULL UNIQUE,
    name        text NOT NULL,
    description text,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE strategy_versions (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    strategy_id  bigint NOT NULL REFERENCES strategy_definitions (id) ON DELETE CASCADE,
    version      integer NOT NULL,
    rules        jsonb NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT strategy_versions_unique UNIQUE (strategy_id, version)
);

CREATE TABLE model_runs (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    strategy_version_id bigint NOT NULL REFERENCES strategy_versions (id) ON DELETE RESTRICT,
    generated_at        timestamptz NOT NULL DEFAULT now(),
    data_cutoff_at      timestamptz NOT NULL,
    status              text NOT NULL DEFAULT 'completed',
    notes               text,
    CONSTRAINT model_runs_status_check CHECK (status IN ('running', 'completed', 'failed'))
);

CREATE TABLE model_bets (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id         bigint NOT NULL REFERENCES model_runs (id) ON DELETE CASCADE,
    event_id       bigint NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    combined_odds  numeric(8, 3) NOT NULL,
    status         text NOT NULL DEFAULT 'pending',
    locked_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT model_bets_status_check CHECK (status IN (
        'pending', 'won', 'lost', 'void', 'partial', 'unknown'
    ))
);

CREATE TABLE model_bet_legs (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    model_bet_id bigint NOT NULL REFERENCES model_bets (id) ON DELETE CASCADE,
    event_id     bigint NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    market_type  text NOT NULL,
    selection    text NOT NULL,
    line         numeric(6, 2),
    odds         numeric(8, 3) NOT NULL,
    evidence     jsonb NOT NULL DEFAULT '{}'::jsonb
);

INSERT INTO strategy_definitions (slug, name, description)
VALUES ('match-result-edge', 'Match result edge',
        'Selects match-result prices where the settled historical hit rate in the same odds bucket exceeds the implied probability.')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO strategy_versions (strategy_id, version, rules)
SELECT id, 1, '{"market":"match_result","min_sample":20,"min_edge":0.03,"max_picks":5,"bucket":0.1}'::jsonb
FROM strategy_definitions WHERE slug = 'match-result-edge'
ON CONFLICT (strategy_id, version) DO NOTHING;
