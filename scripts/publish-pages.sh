#!/usr/bin/env bash
#
# Publish the API origin to the Pages project.
#
# The quick tunnel gets a new hostname whenever it restarts, and the worker binds
# API_ORIGIN per deployment, so a new origin needs the secret updated and a fresh
# deploy. This does that in one place, with its own log and a verification step,
# so the watcher can call it and a person can run it by hand.
#
#   scripts/publish-pages.sh https://example.trycloudflare.com
#
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT" || exit 1

URL="${1:-$(cat .local/api-origin.txt 2>/dev/null || true)}"
PROJECT="${CF_PAGES_PROJECT:-oddsvault-web}"
LOG="$ROOT/.local/logs/publish-pages.log"
BUILD_LOG="$ROOT/.local/logs/publish-build.log"
LOCK="/tmp/oddsvault-deploy.lock"

if [ -z "$URL" ]; then
    echo "no origin given and .local/api-origin.txt is empty" >&2
    exit 2
fi

mkdir -p "$ROOT/.local/logs"
: >"$LOG"
log() { printf '%s %s\n' "$(date -Is)" "$*" | tee -a "$LOG"; }

log "publishing origin $URL"

# Serialise with any other build, so two publishes never fight over out/.
exec 9>"$LOCK" || exit 1
flock -w 600 9 || { log "could not take the deploy lock"; exit 3; }

printf '%s\n' "$URL" | (cd apps/web && npx wrangler pages secret put API_ORIGIN --project-name "$PROJECT") >>"$LOG" 2>&1 \
    || log "warning: setting API_ORIGIN failed"

TOKEN="$(grep -E '^JOURNAL_TOKEN=' .env 2>/dev/null | cut -d= -f2- || true)"
if [ -n "$TOKEN" ]; then
    printf '%s\n' "$TOKEN" | (cd apps/web && npx wrangler pages secret put JOURNAL_TOKEN --project-name "$PROJECT") >>"$LOG" 2>&1 \
        || log "warning: setting JOURNAL_TOKEN failed"
fi

cd apps/web || exit 1
unset NEXT_PUBLIC_API_BASE_URL
if ! npm run build >"$BUILD_LOG" 2>&1; then
    log "build failed; last lines:"
    tail -8 "$BUILD_LOG" >>"$LOG"
    exit 4
fi
log "build ok"

if ! npx wrangler pages deploy out --project-name "$PROJECT" --branch=main --commit-dirty=true \
    >>"$LOG" 2>&1; then
    log "deploy command failed"
    exit 5
fi
log "deploy command ok"

code=""
for _ in $(seq 1 15); do
    code="$(/usr/bin/curl -sS -m 8 -o /dev/null -w '%{http_code}' "https://$PROJECT.pages.dev/svc/healthz" 2>/dev/null || true)"
    [ "$code" = "200" ] && break
    sleep 4
done
if [ "$code" = "200" ]; then
    log "verified: https://$PROJECT.pages.dev/svc/healthz answered 200"
else
    log "warning: /svc/healthz answered ${code:-nothing} after the deploy"
fi

# Put the local export back on the local API so :3000 keeps working.
if NEXT_PUBLIC_API_BASE_URL="http://localhost:${APP_PORT:-8080}" npm run build >"$ROOT/.local/logs/publish-local.log" 2>&1; then
    log "local export rebuilt"
else
    log "warning: local export rebuild failed"
fi

exit 0
