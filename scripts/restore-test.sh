#!/usr/bin/env bash
#
# Restore the newest backup into a throwaway database and check that the core
# tables survived. The real database is never touched.
#
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKUP_DIR="${BACKUP_DIR:-$ROOT/.local/backups}"

if [ -f "$ROOT/.env" ]; then
    while IFS='=' read -r key value; do
        case "$key" in '' | \#*) continue ;; esac
        [ -z "${!key+x}" ] && export "$key=$value"
    done <"$ROOT/.env"
fi

: "${DATABASE_URL:?DATABASE_URL is required}"

backup="${1:-}"
if [ -z "$backup" ]; then
    backup="$(find "$BACKUP_DIR" -name 'oddsvault-*.dump' -type f -printf '%T@ %p\n' | sort -nr | head -1 | cut -d' ' -f2-)"
fi
if [ -z "$backup" ] || [ ! -f "$backup" ]; then
    echo "no backup found in $BACKUP_DIR" >&2
    exit 1
fi

temp_db="oddsvault_restorecheck"
temp_url="$(python3 - "$DATABASE_URL" "$temp_db" <<'PY'
import sys, urllib.parse
url = urllib.parse.urlsplit(sys.argv[1])
print(urllib.parse.urlunsplit((url.scheme, url.netloc, "/" + sys.argv[2], url.query, url.fragment)))
PY
)"

echo "restoring $backup into $temp_db"
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -q -c "DROP DATABASE IF EXISTS $temp_db"
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -q -c "CREATE DATABASE $temp_db"
trap 'psql "$DATABASE_URL" -q -c "DROP DATABASE IF EXISTS $temp_db" >/dev/null 2>&1 || true' EXIT

psql "$temp_url" -v ON_ERROR_STOP=1 -q -c "CREATE EXTENSION IF NOT EXISTS timescaledb"
psql "$temp_url" -v ON_ERROR_STOP=1 -q -c "SELECT timescaledb_pre_restore()"
pg_restore --no-owner --dbname="$temp_url" "$backup"
psql "$temp_url" -v ON_ERROR_STOP=1 -q -c "SELECT timescaledb_post_restore()"

echo "row counts in the restored copy:"
psql "$temp_url" -tAc "SELECT 'events='||count(*) FROM events"
psql "$temp_url" -tAc "SELECT 'odds_snapshots='||count(*) FROM odds_snapshots"
psql "$temp_url" -tAc "SELECT 'personal_bets='||count(*) FROM personal_bets"

test "$(psql "$temp_url" -tAc 'SELECT count(*) FROM events')" -gt 0
test "$(psql "$temp_url" -tAc 'SELECT count(*) FROM personal_bets')" -gt 0
echo "restore test passed"
