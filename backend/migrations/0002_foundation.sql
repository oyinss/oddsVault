-- 0002: reference catalogue and ingestion operations.
--
-- This migration contains structure only. Reference rows (sports) are inserted
-- by 0003; no sports events, odds, results or bets are ever seeded.

-- ---------------------------------------------------------------------------
-- Shared trigger: keep updated_at honest.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END
$$;

-- ---------------------------------------------------------------------------
-- Countries
-- ---------------------------------------------------------------------------
CREATE TABLE countries (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    iso2        char(2),
    iso3        char(3),
    name        text NOT NULL,
    slug        text NOT NULL UNIQUE,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT countries_iso2_unique UNIQUE (iso2),
    CONSTRAINT countries_iso3_unique UNIQUE (iso3)
);

COMMENT ON TABLE countries IS 'Canonical countries used by competitions, teams and players.';

CREATE TRIGGER countries_set_updated_at
    BEFORE UPDATE ON countries
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Sports
-- ---------------------------------------------------------------------------
CREATE TABLE sports (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name          text NOT NULL UNIQUE,
    slug          text NOT NULL UNIQUE,
    display_order integer NOT NULL DEFAULT 0,
    active        boolean NOT NULL DEFAULT true,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE sports IS 'Sport-neutral catalogue. Events reference a sport regardless of shape.';

CREATE TRIGGER sports_set_updated_at
    BEFORE UPDATE ON sports
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Competitions
-- ---------------------------------------------------------------------------
CREATE TABLE competitions (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    sport_id    bigint NOT NULL REFERENCES sports (id) ON DELETE RESTRICT,
    country_id  bigint REFERENCES countries (id) ON DELETE SET NULL,
    name        text NOT NULL,
    slug        text NOT NULL,
    type        text NOT NULL DEFAULT 'league',
    active      boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT competitions_sport_slug_unique UNIQUE (sport_id, slug),
    CONSTRAINT competitions_type_check
        CHECK (type IN ('league', 'cup', 'international', 'tournament', 'friendly', 'other'))
);

COMMENT ON TABLE competitions IS 'League, cup or tournament within a sport.';

CREATE INDEX competitions_country_idx ON competitions (country_id);
CREATE TRIGGER competitions_set_updated_at
    BEFORE UPDATE ON competitions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Seasons
-- ---------------------------------------------------------------------------
CREATE TABLE seasons (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    competition_id  bigint NOT NULL REFERENCES competitions (id) ON DELETE CASCADE,
    name            text NOT NULL,
    start_date      date,
    end_date        date,
    is_current      boolean NOT NULL DEFAULT false,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT seasons_competition_name_unique UNIQUE (competition_id, name),
    CONSTRAINT seasons_dates_check
        CHECK (start_date IS NULL OR end_date IS NULL OR start_date <= end_date)
);

COMMENT ON TABLE seasons IS 'At most one current season per competition (enforced below).';

CREATE UNIQUE INDEX seasons_one_current_per_competition
    ON seasons (competition_id)
    WHERE is_current;

CREATE TRIGGER seasons_set_updated_at
    BEFORE UPDATE ON seasons
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Ingestion run log (master plan section 10)
-- ---------------------------------------------------------------------------
CREATE TABLE import_runs (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    provider          text NOT NULL,
    job_type          text NOT NULL,
    started_at        timestamptz NOT NULL DEFAULT now(),
    finished_at       timestamptz,
    status            text NOT NULL DEFAULT 'running',
    records_received  integer NOT NULL DEFAULT 0 CHECK (records_received >= 0),
    records_inserted  integer NOT NULL DEFAULT 0 CHECK (records_inserted >= 0),
    records_updated   integer NOT NULL DEFAULT 0 CHECK (records_updated >= 0),
    records_skipped   integer NOT NULL DEFAULT 0 CHECK (records_skipped >= 0),
    error_message     text,
    metadata          jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT import_runs_status_check
        CHECK (status IN ('running', 'success', 'partial', 'failed'))
);

COMMENT ON TABLE import_runs IS 'One row per ingestion operation, surfaced in the admin data-health page.';

CREATE INDEX import_runs_provider_started_idx ON import_runs (provider, started_at DESC);
CREATE INDEX import_runs_status_idx ON import_runs (status, started_at DESC);

-- ---------------------------------------------------------------------------
-- Provider response cache (master plan section 8)
-- ---------------------------------------------------------------------------
CREATE TABLE provider_request_cache (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    provider       text NOT NULL,
    request_key    text NOT NULL,
    request_hash   text NOT NULL,
    status_code    integer,
    content_type   text,
    payload        jsonb,
    payload_hash   text,
    size_bytes     integer CHECK (size_bytes IS NULL OR size_bytes >= 0),
    fetched_at     timestamptz NOT NULL DEFAULT now(),
    expires_at     timestamptz,
    etag           text,
    last_modified  text,
    created_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT provider_request_cache_unique UNIQUE (provider, request_hash)
);

COMMENT ON TABLE provider_request_cache IS
    'Normalized upstream responses keyed by provider + request hash. Completed historical responses may be reused indefinitely.';

CREATE INDEX provider_request_cache_expiry_idx ON provider_request_cache (expires_at);

-- ---------------------------------------------------------------------------
-- Provider quota observations (master plan section 9)
-- ---------------------------------------------------------------------------
CREATE TABLE provider_quota_observations (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    provider          text NOT NULL,
    observed_at       timestamptz NOT NULL DEFAULT now(),
    daily_limit       integer CHECK (daily_limit IS NULL OR daily_limit >= 0),
    daily_remaining   integer CHECK (daily_remaining IS NULL OR daily_remaining >= 0),
    minute_limit      integer CHECK (minute_limit IS NULL OR minute_limit >= 0),
    minute_remaining  integer CHECK (minute_remaining IS NULL OR minute_remaining >= 0),
    metadata          jsonb NOT NULL DEFAULT '{}'::jsonb
);

COMMENT ON TABLE provider_quota_observations IS
    'Counter snapshots reported by free-tier APIs. NULL means unknown, 0 means a reported zero.';

CREATE INDEX provider_quota_observations_provider_idx
    ON provider_quota_observations (provider, observed_at DESC);
