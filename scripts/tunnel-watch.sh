#!/usr/bin/env bash
#
# Keeps the quick tunnel and the Pages origin in step.
#
# The free trycloudflare tunnel is not stable: it expires server-side after a
# while, and when it dies every /svc call returns 530. This watcher checks the
# current origin, and when it stops answering it starts a new tunnel, updates
# local state, pushes the new API_ORIGIN secret to Pages, redeploys, and rebuilds
# the local export.
#
# Run it in the background: scripts/tunnel-watch.sh
# It logs to .local/logs/tunnel-watch.log.

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

ORIGIN_FILE=".local/api-origin.txt"
LOG=".local/logs/tunnel-watch.log"
BIN=".local/bin/cloudflared"
TUNNEL_LOG=".local/logs/cloudflared.log"
PROJECT="oddsvault-web"
API_LOCAL="http://127.0.0.1:8080/healthz"
CHECK_INTERVAL="${CHECK_INTERVAL:-30}"

mkdir -p .local/logs

log() {
  printf '%s %s\n' "$(date -Is)" "$*" >>"$LOG"
}

tunnel_up() {
  local url
  url="$(cat "$ORIGIN_FILE" 2>/dev/null || true)"
  [ -n "$url" ] || return 1
  curl -fsS -m 8 "$url/healthz" >/dev/null 2>&1
}

local_api_up() {
  curl -fsS -m 5 "$API_LOCAL" >/dev/null 2>&1
}

start_tunnel() {
  local old_pid
  for old_pid in $(pgrep -f "[c]loudflared tunnel --no-autoupdate --url http://127.0.0.1:8080" 2>/dev/null); do
    kill "$old_pid" 2>/dev/null
  done
  sleep 1
  : >"$TUNNEL_LOG"
  setsid nohup "$BIN" tunnel --no-autoupdate --url http://127.0.0.1:8080 \
    >"$TUNNEL_LOG" 2>&1 </dev/null &
  log "cloudflared started"

  local url="" attempt
  for attempt in $(seq 1 30); do
    url="$(grep -Eo 'https://[a-z0-9-]+\.trycloudflare\.com' "$TUNNEL_LOG" | head -1)"
    [ -n "$url" ] && break
    sleep 2
  done
  if [ -z "$url" ]; then
    log "no tunnel url after 60s"
    return 1
  fi
  printf '%s\n' "$url" >"$ORIGIN_FILE"
  log "tunnel url $url"

  local code="000" attempt
  for attempt in $(seq 1 30); do
    code="$(curl -sS -m 5 -o /dev/null -w '%{http_code}' "$url/healthz" 2>/dev/null || true)"
    [ "$code" = "200" ] && break
    sleep 2
  done
  if [ "$code" != "200" ]; then
    log "tunnel did not answer health after 60s (last $code)"
    return 1
  fi
  log "tunnel healthy"
  return 0
}

publish_origin() {
  local url token
  url="$(cat "$ORIGIN_FILE")"
  token="$(grep -E '^JOURNAL_TOKEN=' .env 2>/dev/null | cut -d= -f2- || true)"

  printf '%s\n' "$url" | (cd apps/web && npx wrangler pages secret put API_ORIGIN --project-name "$PROJECT" >/dev/null 2>&1) \
    || log "secret API_ORIGIN failed"
  if [ -n "$token" ]; then
    printf '%s\n' "$token" | (cd apps/web && npx wrangler pages secret put JOURNAL_TOKEN --project-name "$PROJECT" >/dev/null 2>&1) \
      || log "secret JOURNAL_TOKEN failed"
  fi

  # A build that collides with another build in the same directory fails fast,
  # and a failed deploy leaves Pages pointing at the dead origin. Serialise on
  # one lock, record the reason, and try the whole thing twice.
  local attempt ok=0
  for attempt in 1 2; do
    if (
      flock -w 600 9 || exit 90
      cd "$ROOT/apps/web" || exit 1
      unset NEXT_PUBLIC_API_BASE_URL
      npm run build >/tmp/oddsvault-watch-build.log 2>&1 || { tail -5 /tmp/oddsvault-watch-build.log >>"$LOG"; exit 1; }
      npx wrangler pages deploy out --project-name "$PROJECT" --branch=main --commit-dirty=true \
        >>"$LOG" 2>&1 || exit 1
    ) 9>/tmp/oddsvault-deploy.lock; then
      ok=1
      break
    fi
    log "pages deploy attempt $attempt failed"
    sleep 5
  done
  if [ "$ok" != "1" ]; then
    log "pages deploy failed for $url; site will keep the old origin until it succeeds"
    return 1
  fi

  local code="" wait
  for wait in $(seq 1 15); do
    code="$(curl -sS -m 8 -o /dev/null -w '%{http_code}' "https://$PROJECT.pages.dev/svc/healthz" 2>/dev/null || true)"
    [ "$code" = "200" ] && break
    sleep 4
  done
  if [ "$code" != "200" ]; then
    log "deployed but https://$PROJECT.pages.dev/svc/healthz answered $code"
  else
    log "pages deployed for $url and verified"
  fi

  # Restore the local export so localhost:3000 keeps talking to localhost:8080.
  (cd apps/web && NEXT_PUBLIC_API_BASE_URL=http://localhost:8080 npm run build \
    >/tmp/oddsvault-watch-local.log 2>&1) || log "local rebuild failed"
  log "local export rebuilt"
}

log "watcher started (pid $$)"
while true; do
  if ! local_api_up; then
    log "local api is not answering; not touching the tunnel"
  elif ! tunnel_up; then
    log "tunnel is unhealthy; restarting"
    if start_tunnel; then
      publish_origin
    fi
  fi
  sleep "$CHECK_INTERVAL"
done
