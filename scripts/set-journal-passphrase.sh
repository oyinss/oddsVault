#!/usr/bin/env bash
#
# Rotate the passphrase that unlocks the personal bet journal.
#
#   make journal-lock
#
# Sets JOURNAL_PASSPHRASE and a fresh JOURNAL_SESSION_SECRET on the Pages
# project, which invalidates every existing session.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROJECT="${CF_PAGES_PROJECT:-oddsvault-web}"

read -rsp "New journal passphrase (12+ characters): " PASS
echo
read -rsp "Repeat: " AGAIN
echo

if [ "$PASS" != "$AGAIN" ]; then
  echo "Passphrases do not match." >&2
  exit 1
fi
if [ "${#PASS}" -lt 12 ]; then
  echo "Use at least 12 characters." >&2
  exit 1
fi

(
  cd "$ROOT/apps/web"
  printf '%s' "$PASS" |
    npx wrangler pages secret put JOURNAL_PASSPHRASE --project-name "$PROJECT"
  openssl rand -hex 32 |
    npx wrangler pages secret put JOURNAL_SESSION_SECRET --project-name "$PROJECT"
)

echo "Passphrase updated. Every existing journal session is now invalid."
