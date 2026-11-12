-- 0012: personal bet journal.
--
-- Stores what was actually played, exactly as the bookmaker reported it. The
-- odds taken are historical facts and are never replaced by later prices.

INSERT INTO bookmakers (name, slug, country_scope, base_url)
VALUES ('1xBet', '1xbet', 'nigeria', 'https://1xbet.ng')
ON CONFLICT (slug) DO NOTHING;

CREATE TABLE personal_bets (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    bookmaker_id     bigint NOT NULL REFERENCES bookmakers (id) ON DELETE RESTRICT,
    external_id      text NOT NULL,
    currency         char(3) NOT NULL DEFAULT 'NGN',
    placed_at        timestamptz,
    placed_at_raw    text,
    bet_type         text,
    stake            numeric(14, 2),
    combined_odds    numeric(10, 3),
    potential_return numeric(14, 2),
    status           text NOT NULL DEFAULT 'unknown',
    actual_return    numeric(14, 2),
    profit_loss      numeric(14, 2),
    source           text NOT NULL,
    raw              jsonb NOT NULL,
    imported_at      timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT personal_bets_status_check CHECK (status IN (
        'pending', 'won', 'lost', 'void', 'partial', 'unknown'
    )),
    CONSTRAINT personal_bets_stake_check CHECK (stake IS NULL OR stake >= 0),
    CONSTRAINT personal_bets_unique UNIQUE (bookmaker_id, external_id)
);

CREATE TRIGGER personal_bets_set_updated_at
    BEFORE UPDATE ON personal_bets
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX personal_bets_placed_idx ON personal_bets (placed_at DESC);

CREATE TABLE personal_bet_legs (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    personal_bet_id bigint NOT NULL REFERENCES personal_bets (id) ON DELETE CASCADE,
    leg_number      smallint NOT NULL,
    competition_raw text,
    event_raw       text NOT NULL,
    market_raw      text,
    selection_raw   text,
    odds_taken      numeric(10, 3),
    event_id        bigint REFERENCES events (id) ON DELETE SET NULL,
    identity_status text NOT NULL DEFAULT 'unresolved',
    leg_status      text NOT NULL DEFAULT 'unknown',
    source_result   text NOT NULL DEFAULT 'provider',
    created_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT personal_bet_legs_identity_check CHECK (identity_status IN (
        'confirmed', 'provisional', 'unresolved'
    )),
    CONSTRAINT personal_bet_legs_status_check CHECK (leg_status IN (
        'pending', 'won', 'lost', 'void', 'unknown'
    )),
    CONSTRAINT personal_bet_legs_unique UNIQUE (personal_bet_id, leg_number)
);

CREATE INDEX personal_bet_legs_event_idx ON personal_bet_legs (event_id);

CREATE TABLE personal_bet_versions (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    personal_bet_id bigint NOT NULL REFERENCES personal_bets (id) ON DELETE CASCADE,
    version         integer NOT NULL,
    snapshot        jsonb NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT personal_bet_versions_unique UNIQUE (personal_bet_id, version)
);
