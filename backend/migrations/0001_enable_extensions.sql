-- 0001: required extensions.
--
-- TimescaleDB is a hard requirement: odds_snapshots becomes a hypertable in
-- Phase 2. Fail loudly rather than silently building on plain PostgreSQL.
DO $do$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_available_extensions WHERE name = 'timescaledb'
    ) THEN
        RAISE EXCEPTION
            'TimescaleDB is not installed on this PostgreSQL server. '
            'Install the package matching this major version (Arch: sudo pacman -S --needed timescaledb) '
            'and ensure shared_preload_libraries includes timescaledb (see scripts/local-db.sh).';
    END IF;
    EXECUTE 'CREATE EXTENSION IF NOT EXISTS timescaledb';
END
$do$;

-- gen_random_uuid() for identifiers that are not database-generated sequences.
CREATE EXTENSION IF NOT EXISTS pgcrypto;
