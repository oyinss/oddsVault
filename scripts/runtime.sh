#!/usr/bin/env bash
#
# OddsVault local runtime.
#
#   runtime.sh up        start whatever is not already healthy
#   runtime.sh down      stop processes this script started
#   runtime.sh status    one-line summary per component
#   runtime.sh supervise run forever, restarting failed children (systemd uses this)
#
# Processes already healthy on the expected ports are reused rather than
# duplicated, so this can coexist with a manually started `make dev` session.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOCAL="$ROOT/.local"
BIN="$LOCAL/bin"
RUN="$LOCAL/run"
LOG="$LOCAL/log"
WEB_DIR="$ROOT/apps/web"
WEB_OUT="$WEB_DIR/out"

API_PORT="${API_PORT:-8080}"
WEB_PORT="${WEB_PORT:-3000}"
CORS="${CORS_ALLOWED_ORIGINS:-http://localhost:${WEB_PORT},http://127.0.0.1:${WEB_PORT}}"

mkdir -p "$BIN" "$RUN" "$LOG"

# Load local configuration (DATABASE_URL, JOURNAL_TOKEN, ...) without exporting
# anything the caller already set.
if [ -f "$ROOT/.env" ]; then
    while IFS='=' read -r key value; do
        case "$key" in
        '' | \#*) continue ;;
        esac
        if [ -z "${!key+x}" ]; then
            export "$key=$value"
        fi
    done <"$ROOT/.env"
fi

: "${DATABASE_URL:?DATABASE_URL is required (see .env)}"

log() { printf '[runtime] %s\n' "$*"; }

pid_file() { printf '%s/%s.pid' "$RUN" "$1"; }

alive() {
    local file
    file="$(pid_file "$1")"
    [ -f "$file" ] || return 1
    local pid
    pid="$(cat "$file" 2>/dev/null || true)"
    [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null
}

http_ok() {
    curl -sf -o /dev/null --max-time 2 "http://localhost:$1$2" 2>/dev/null
}

start_bg() {
    local name="$1"
    shift
    if alive "$name"; then
        log "$name already running (pid $(cat "$(pid_file "$name")"))"
        return 0
    fi
    setsid nohup "$@" >"$LOG/$name.log" 2>&1 </dev/null &
    echo $! >"$(pid_file "$name")"
    log "$name started (pid $!)"
}

build_backend() {
    if [ ! -x "$BIN/api" ]; then
        log "building Go binaries"
        (cd "$ROOT/backend" && go build -o "$BIN/api" ./cmd/api)
        (cd "$ROOT/backend" && go build -o "$BIN/sports-sync" ./cmd/sports-sync)
        (cd "$ROOT/backend" && go build -o "$BIN/settlement-worker" ./cmd/settlement-worker)
        (cd "$ROOT/backend" && go build -o "$BIN/bookmaker-collector" ./cmd/bookmaker-collector)
        (cd "$ROOT/backend" && go build -o "$BIN/import-bets" ./cmd/import-bets)
        (cd "$ROOT/backend" && go build -o "$BIN/model-picks" ./cmd/model-picks)
        (cd "$ROOT/backend" && go build -o "$BIN/dedupe-live" ./cmd/dedupe-live)
    fi
}

build_web() {
    if [ "${WEB_REBUILD:-0}" = "1" ] || [ ! -f "$WEB_OUT/index.html" ]; then
        log "building web export"
        (cd "$WEB_DIR" && NEXT_PUBLIC_API_BASE_URL="http://localhost:${API_PORT}" npm run build >"$LOG/web-build.log" 2>&1)
    fi
}

start_db() {
    if pg_isready -h 127.0.0.1 -p 5432 >/dev/null 2>&1; then
        return 0
    fi
    log "starting database"
    "$ROOT/scripts/local-db.sh" start >>"$LOG/db.log" 2>&1
}

start_api() {
    if http_ok "$API_PORT" /healthz; then
        log "api already serving on :$API_PORT"
        return 0
    fi
    build_backend
    start_bg api env \
        DATABASE_URL="$DATABASE_URL" \
        JOURNAL_TOKEN="${JOURNAL_TOKEN:-}" \
        CORS_ALLOWED_ORIGINS="$CORS" \
        APP_ENV="${APP_ENV:-development}" \
        HTTP_ADDR=":$API_PORT" \
        ODDSVAULT_BIN="$BIN" \
        ODDSVAULT_ROOT="$ROOT" \
        "$BIN/api"
}

start_web() {
    if http_ok "$WEB_PORT" /; then
        log "web already serving on :$WEB_PORT"
        return 0
    fi
    build_web
    start_bg web python3 -m http.server "$WEB_PORT" --bind 127.0.0.1 --directory "$WEB_OUT"
}

start_worker() {
    local name="$1"
    if pgrep -f "cmd/$name|$BIN/$name" >/dev/null 2>&1; then
        log "$name already running (external process)"
        return 0
    fi
    build_backend
    start_bg "$name" env \
        DATABASE_URL="$DATABASE_URL" \
        APP_ENV="${APP_ENV:-development}" \
        "$BIN/$name"
}

wait_ready() {
    local tries="${1:-30}"
    while [ "$tries" -gt 0 ]; do
        if http_ok "$API_PORT" /readyz && http_ok "$WEB_PORT" /; then
            return 0
        fi
        sleep 1
        tries=$((tries - 1))
    done
    return 1
}

cmd_up() {
    start_db
    start_api
    start_web
    start_worker sports-sync
    start_worker settlement-worker
    start_worker bookmaker-collector
    if wait_ready 30; then
        log "ready: http://localhost:$WEB_PORT (api :$API_PORT)"
    else
        log "warning: not all components became ready; see $LOG"
        return 1
    fi
}

cmd_down() {
    local name pid file
    for name in web api sports-sync settlement-worker bookmaker-collector; do
        file="$(pid_file "$name")"
        [ -f "$file" ] || continue
        pid="$(cat "$file" 2>/dev/null || true)"
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            kill "$pid" 2>/dev/null || true
            log "$name stopped (pid $pid)"
        fi
        rm -f "$file"
    done
}

cmd_status() {
    local name state
    for name in api web sports-sync settlement-worker bookmaker-collector; do
        if alive "$name"; then
            state="running (pid $(cat "$(pid_file "$name")"))"
        elif [ "$name" = "api" ] && http_ok "$API_PORT" /healthz; then
            state="external (api :$API_PORT)"
        elif [ "$name" = "web" ] && http_ok "$WEB_PORT" /; then
            state="external (:port $WEB_PORT)"
        else
            state="stopped"
        fi
        printf '%-20s %s\n' "$name" "$state"
    done
    printf '%-20s %s\n' database "$(pg_isready -h 127.0.0.1 -p 5432 2>/dev/null || echo down)"
}

cmd_supervise() {
    trap 'cmd_down; exit 0' INT TERM
    log "supervising"
    while true; do
        cmd_up || true
        # Re-check every 15 seconds; rebuild if a child died.
        sleep 15
        if ! alive api || ! alive web || ! alive sports-sync ||
            ! alive settlement-worker || ! alive bookmaker-collector; then
            if ! http_ok "$API_PORT" /readyz || ! http_ok "$WEB_PORT" /; then
                log "component unhealthy; restarting"
                cmd_down
                sleep 2
            fi
        fi
    done
}

case "${1:-}" in
up) cmd_up ;;
down) cmd_down ;;
status) cmd_status ;;
supervise) cmd_supervise ;;
*)
    printf 'usage: %s {up|down|status|supervise}\n' "$0" >&2
    exit 2
    ;;
esac
