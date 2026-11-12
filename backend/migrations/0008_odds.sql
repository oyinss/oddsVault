-- 0008: bookmakers, canonical markets and immutable odds snapshots.
--
-- source_kind distinguishes how a price was observed:
--   'closing' — the provider published a closing price for a finished match
--   'live'    — our own collector captured the price at captured_at
-- Both are real observations; they are never mixed in the same comparison
-- without saying so.

CREATE TABLE bookmakers (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name          text NOT NULL UNIQUE,
    slug          text NOT NULL UNIQUE,
    country_scope text,
    base_url      text,
    active        boolean NOT NULL DEFAULT true,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER bookmakers_set_updated_at
    BEFORE UPDATE ON bookmakers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE bookmaker_events (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    bookmaker_id      bigint NOT NULL REFERENCES bookmakers (id) ON DELETE CASCADE,
    provider_event_id text NOT NULL,
    raw_name          text,
    scheduled_at      timestamptz,
    matched_event_id  bigint REFERENCES events (id) ON DELETE SET NULL,
    identity_status   text NOT NULL DEFAULT 'unresolved',
    first_seen_at     timestamptz NOT NULL DEFAULT now(),
    last_seen_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT bookmaker_events_identity_check CHECK (identity_status IN (
        'confirmed', 'provisional', 'unresolved'
    )),
    CONSTRAINT bookmaker_events_unique UNIQUE (bookmaker_id, provider_event_id)
);

CREATE TABLE market_types (
    id        bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug      text NOT NULL UNIQUE,
    name      text NOT NULL,
    sport_id  bigint REFERENCES sports (id) ON DELETE SET NULL,
    active    boolean NOT NULL DEFAULT true
);

CREATE TABLE markets (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id       bigint NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    market_type_id bigint NOT NULL REFERENCES market_types (id) ON DELETE RESTRICT,
    period         text NOT NULL DEFAULT 'full_game',
    line           numeric(6, 2),
    subject        text NOT NULL DEFAULT 'match',
    created_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT markets_unique UNIQUE (event_id, market_type_id, period, line, subject)
);

CREATE INDEX markets_event_idx ON markets (event_id);

CREATE TABLE market_selections (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    market_id    bigint NOT NULL REFERENCES markets (id) ON DELETE CASCADE,
    selection    text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT market_selections_unique UNIQUE (market_id, selection)
);

CREATE INDEX market_selections_market_idx ON market_selections (market_id);

CREATE TABLE snapshot_batches (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    bookmaker_id      bigint NOT NULL REFERENCES bookmakers (id) ON DELETE CASCADE,
    started_at        timestamptz NOT NULL DEFAULT now(),
    completed_at      timestamptz,
    status            text NOT NULL DEFAULT 'running',
    source_kind       text NOT NULL DEFAULT 'live',
    event_count       integer NOT NULL DEFAULT 0,
    market_count      integer NOT NULL DEFAULT 0,
    offer_count       integer NOT NULL DEFAULT 0,
    collector_version text,
    error_summary     text,
    CONSTRAINT snapshot_batches_status_check CHECK (status IN ('running', 'success', 'partial', 'failed')),
    CONSTRAINT snapshot_batches_kind_check CHECK (source_kind IN ('live', 'closing'))
);

CREATE INDEX snapshot_batches_bookmaker_idx ON snapshot_batches (bookmaker_id, started_at DESC);

-- Odds observations. Converted to a hypertable below.
CREATE TABLE odds_snapshots (
    captured_at       timestamptz NOT NULL,
    snapshot_batch_id bigint NOT NULL REFERENCES snapshot_batches (id) ON DELETE CASCADE,
    bookmaker_id      bigint NOT NULL REFERENCES bookmakers (id) ON DELETE CASCADE,
    event_id          bigint REFERENCES events (id) ON DELETE SET NULL,
    bookmaker_event_id bigint REFERENCES bookmaker_events (id) ON DELETE SET NULL,
    market_id         bigint NOT NULL REFERENCES markets (id) ON DELETE CASCADE,
    selection_id      bigint NOT NULL REFERENCES market_selections (id) ON DELETE CASCADE,
    decimal_odds      numeric(8, 3) NOT NULL CHECK (decimal_odds > 1),
    availability_status text NOT NULL DEFAULT 'available',
    price_kind        text NOT NULL DEFAULT 'live',
    source_hash       text,
    CONSTRAINT odds_snapshots_kind_check CHECK (price_kind IN ('live', 'closing')),
    CONSTRAINT odds_snapshots_availability_check CHECK (availability_status IN (
        'available', 'suspended', 'removed'
    ))
);

SELECT create_hypertable('odds_snapshots', 'captured_at', if_not_exists => TRUE);

CREATE INDEX odds_snapshots_event_selection_idx
    ON odds_snapshots (event_id, selection_id, captured_at DESC);
CREATE INDEX odds_snapshots_selection_idx
    ON odds_snapshots (selection_id, captured_at DESC);
CREATE INDEX odds_snapshots_batch_idx
    ON odds_snapshots (snapshot_batch_id);

-- Seed canonical market types used by the importer and settlement engine.
INSERT INTO market_types (slug, name, sport_id)
SELECT v.slug, v.name, s.id
FROM (VALUES
    ('match_result',       'Match result (1X2)'),
    ('double_chance',      'Double chance'),
    ('draw_no_bet',        'Draw no bet'),
    ('total_goals',        'Total goals'),
    ('both_teams_to_score','Both teams to score')
) AS v(slug, name)
CROSS JOIN sports s
WHERE s.slug = 'football'
ON CONFLICT (slug) DO NOTHING;

-- Seed the first bookmaker: Bet365 closing prices via football-data.co.uk.
INSERT INTO bookmakers (name, slug, country_scope, base_url)
VALUES ('Bet365', 'bet365', 'international', 'https://www.bet365.com')
ON CONFLICT (slug) DO NOTHING;
