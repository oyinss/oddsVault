-- 0014: media assets, starting with team logos.
--
-- The row records where the asset came from. Bytes are cached by our API so the
-- browser only ever talks to our origin.

CREATE TABLE media_assets (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    entity_type      text NOT NULL,
    entity_id        bigint NOT NULL,
    asset_type       text NOT NULL,
    source_provider  text NOT NULL,
    source_url       text NOT NULL,
    storage_url      text,
    width            integer,
    height           integer,
    mime_type        text,
    fetched_at       timestamptz NOT NULL DEFAULT now(),
    license_metadata jsonb,
    active           boolean NOT NULL DEFAULT true,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT media_assets_entity_check CHECK (entity_type IN (
        'team', 'competition', 'player', 'bookmaker', 'venue', 'sport'
    )),
    CONSTRAINT media_assets_asset_check CHECK (asset_type IN (
        'team_logo', 'competition_logo', 'player_photo',
        'bookmaker_logo', 'venue_photo', 'sport_icon'
    )),
    CONSTRAINT media_assets_unique UNIQUE (entity_type, entity_id, asset_type)
);

CREATE INDEX media_assets_entity_idx ON media_assets (entity_type, entity_id);

CREATE TRIGGER media_assets_set_updated_at
    BEFORE UPDATE ON media_assets
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
