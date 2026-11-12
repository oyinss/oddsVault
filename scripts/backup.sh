#!/usr/bin/env bash
#
# Dump the database, verify the archive, keep 14 days, and optionally copy it
# off the machine.
#
#   BACKUP_REMOTE=user@host:/path scripts/backup.sh
#
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKUP_DIR="${BACKUP_DIR:-$ROOT/.local/backups}"
RETENTION_DAYS="${RETENTION_DAYS:-14}"

if [ -f "$ROOT/.env" ]; then
    while IFS='=' read -r key value; do
        case "$key" in '' | \#*) continue ;; esac
        [ -z "${!key+x}" ] && export "$key=$value"
    done <"$ROOT/.env"
fi

: "${DATABASE_URL:?DATABASE_URL is required}"

mkdir -p "$BACKUP_DIR"
stamp="$(date +%Y%m%d-%H%M%S)"
final="$BACKUP_DIR/oddsvault-$stamp.dump"
tmp="$final.part"

echo "dumping to $final"
pg_dump "$DATABASE_URL" --format=custom --no-owner --file="$tmp"

# Prove the archive is readable before deleting the partial extension.
if ! pg_restore --list "$tmp" >/dev/null 2>&1; then
    echo "backup archive failed validation; leaving $tmp" >&2
    exit 1
fi
mv "$tmp" "$final"
echo "verified $(du -h "$final" | cut -f1) archive"

find "$BACKUP_DIR" -name 'oddsvault-*.dump' -type f -mtime +"$RETENTION_DAYS" -print -delete

if [ -n "${BACKUP_REMOTE:-}" ]; then
    echo "copying to $BACKUP_REMOTE"
    rsync -a "$BACKUP_DIR/" "$BACKUP_REMOTE/"
fi

echo "$final"
