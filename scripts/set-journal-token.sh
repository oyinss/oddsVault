#!/usr/bin/env bash
#
# Rotate the shared token that lets the Pages proxy read the journal from the
# Go API. The tunnel URL is public, so the API refuses /api/v1/my-bets without
# this token.
#
#   make journal-token
#
# Writes the token to .env (git-ignored) and to the Pages project. Restart the
# API afterwards so it picks up the new value.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROJECT="${CF_PAGES_PROJECT:-oddsvault-web}"
TOKEN="$(openssl rand -hex 32)"

cd "$ROOT"
touch .env
if grep -q '^JOURNAL_TOKEN=' .env; then
  sed -i "s|^JOURNAL_TOKEN=.*|JOURNAL_TOKEN=$TOKEN|" .env
else
  printf '\nJOURNAL_TOKEN=%s\n' "$TOKEN" >>.env
fi

(
  cd apps/web
  printf '%s' "$TOKEN" |
    npx wrangler pages secret put JOURNAL_TOKEN --project-name "$PROJECT"
)

echo "Token updated in .env and on $PROJECT."
echo "Restart the API (make api) so it enforces the new token."
