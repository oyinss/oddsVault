-- 0004: core sports identity — teams, events, participants, provider links.
--
-- Provider names are kept as observations. Canonical teams are matched by a
-- normalized name key and explicit aliases; an unmatchable fixture is stored as
-- identity_status = 'provisional' rather than guessed.

-- Raw response bodies are needed for CSV and text payloads, which jsonb cannot
-- hold faithfully.
ALTER TABLE provider_request_cache ADD COLUMN body text;

-- ---------------------------------------------------------------------------
-- Teams
-- ---------------------------------------------------------------------------
CREATE TABLE teams (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    sport_id    bigint NOT NULL REFERENCES sports (id) ON DELETE RESTRICT,
    country_id  bigint REFERENCES countries (id) ON DELETE SET NULL,
    name        text NOT NULL,
    name_key    text NOT NULL,
    short_name  text,
    slug        text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT teams_sport_slug_unique UNIQUE (sport_id, slug)
);

COMMENT ON TABLE teams IS 'Canonical teams. name_key is the normalized match key for provider names.';
COMMENT ON COLUMN teams.name_key IS 'Unicode-folded, suffix-stripped key; matching only, never displayed.';

CREATE UNIQUE INDEX teams_sport_name_key_unique ON teams (sport_id, name_key);
CREATE TRIGGER teams_set_updated_at
    BEFORE UPDATE ON teams
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Team provider ids
-- ---------------------------------------------------------------------------
CREATE TABLE team_provider_ids (
    team_id           bigint NOT NULL REFERENCES teams (id) ON DELETE CASCADE,
    provider          text NOT NULL,
    provider_team_id  text NOT NULL,
    provider_name     text,
    created_at        timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, provider_team_id)
);

CREATE INDEX team_provider_ids_team_idx ON team_provider_ids (team_id);

-- ---------------------------------------------------------------------------
-- Team aliases
-- ---------------------------------------------------------------------------
CREATE TABLE team_aliases (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    team_id     bigint NOT NULL REFERENCES teams (id) ON DELETE CASCADE,
    provider    text NOT NULL DEFAULT '',
    alias       text NOT NULL,
    alias_key   text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT team_aliases_unique UNIQUE (alias_key, provider)
);

COMMENT ON TABLE team_aliases IS
    'Explicit name variants (Man Utd / Manchester United) that resolve to one canonical team.';

CREATE INDEX team_aliases_team_idx ON team_aliases (team_id);

-- ---------------------------------------------------------------------------
-- Events
-- ---------------------------------------------------------------------------
CREATE TABLE events (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    sport_id          bigint NOT NULL REFERENCES sports (id) ON DELETE RESTRICT,
    competition_id    bigint NOT NULL REFERENCES competitions (id) ON DELETE RESTRICT,
    season_id         bigint REFERENCES seasons (id) ON DELETE SET NULL,
    start_date        date,
    start_time        timestamptz,
    start_time_local  text,
    round_label       text,
    status            text NOT NULL DEFAULT 'scheduled',
    identity_status   text NOT NULL DEFAULT 'provisional',
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT events_status_check CHECK (status IN (
        'scheduled', 'live', 'finished', 'postponed', 'cancelled', 'abandoned', 'unknown'
    )),
    CONSTRAINT events_identity_status_check CHECK (identity_status IN (
        'confirmed', 'provisional', 'unresolved'
    ))
);

COMMENT ON TABLE events IS 'Sport-neutral events. start_time stays NULL when the source has no verified timezone.';

CREATE INDEX events_sport_start_idx ON events (sport_id, start_date);
CREATE INDEX events_competition_start_idx ON events (competition_id, season_id, start_date);
CREATE TRIGGER events_set_updated_at
    BEFORE UPDATE ON events
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Event participants
-- ---------------------------------------------------------------------------
CREATE TABLE event_participants (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id       bigint NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    team_id        bigint REFERENCES teams (id) ON DELETE SET NULL,
    role           text NOT NULL,
    display_order  smallint NOT NULL DEFAULT 0,
    provider_name  text,
    CONSTRAINT event_participants_role_check CHECK (role IN ('home', 'away', 'participant')),
    CONSTRAINT event_participants_unique UNIQUE (event_id, role),
    CONSTRAINT event_participants_order_check CHECK (display_order >= 0)
);

CREATE INDEX event_participants_team_idx ON event_participants (team_id);

-- ---------------------------------------------------------------------------
-- Event provider ids
-- ---------------------------------------------------------------------------
CREATE TABLE event_provider_ids (
    event_id             bigint NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    provider             text NOT NULL,
    provider_event_id    text NOT NULL,
    observed_start_time  timestamptz,
    observed_status      text,
    match_method         text,
    linked_at            timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, provider_event_id)
);

COMMENT ON TABLE event_provider_ids IS
    'Maps an external fixture id (or provider-derived key) to one internal event.';

CREATE INDEX event_provider_ids_event_idx ON event_provider_ids (event_id);

-- ---------------------------------------------------------------------------
-- Raw provider payloads
-- ---------------------------------------------------------------------------
CREATE TABLE raw_provider_payloads (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    provider            text NOT NULL,
    entity_type         text NOT NULL,
    provider_entity_id  text NOT NULL,
    captured_at         timestamptz NOT NULL DEFAULT now(),
    payload             jsonb NOT NULL,
    payload_hash        text NOT NULL,
    schema_version      text NOT NULL DEFAULT 'v1',
    CONSTRAINT raw_provider_payloads_unique
        UNIQUE (provider, entity_type, provider_entity_id, payload_hash)
);

CREATE INDEX raw_provider_payloads_entity_idx
    ON raw_provider_payloads (provider, entity_type, provider_entity_id);
