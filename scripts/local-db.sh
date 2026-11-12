#!/usr/bin/env bash
#
# Local PostgreSQL + TimescaleDB cluster for OddsVault.
#
# The cluster lives inside the repository (.local/pgdata) so it can be started
# and stopped per work session without touching a system-wide service. Data
# survives stop/start; `reset` deletes it.
#
# Usage: scripts/local-db.sh {check|init|start|stop|status|reset|psql}

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOCAL_DIR="${LOCAL_DIR:-$ROOT/.local}"
PGDATA="${PGDATA:-$LOCAL_DIR/pgdata}"
PGLOG="${PGLOG:-$LOCAL_DIR/postgres.log}"
SOCKET_DIR="$LOCAL_DIR/run"

PGPORT="${PGPORT:-5432}"
PGUSER="${PGUSER:-oddsvault}"
PGPASSWORD="${PGPASSWORD:-oddsvault}"
PGDATABASE="${PGDATABASE:-oddsvault}"

export PGPASSWORD

die() {
    printf 'error: %s\n' "$*" >&2
    exit 1
}

require_tools() {
    local missing=()
    for tool in initdb pg_ctl pg_isready psql pg_config; do
        command -v "$tool" >/dev/null 2>&1 || missing+=("$tool")
    done
    if [ ${#missing[@]} -gt 0 ]; then
        die "missing PostgreSQL tools: ${missing[*]}.
Install PostgreSQL first (Arch: sudo pacman -S --needed postgresql)."
    fi
}

require_timescaledb() {
    local sharedir
    sharedir="$(pg_config --sharedir)"
    if [ ! -f "$sharedir/extension/timescaledb.control" ]; then
        die "TimescaleDB is not installed for $(pg_config --version).

The platform requires TimescaleDB, not plain PostgreSQL. Install the package
matching your PostgreSQL major version, then re-run:

    sudo pacman -S --needed timescaledb      # Arch
    # Debian/Ubuntu: follow https://docs.timescale.com/self-hosted/latest/install/

If you just installed it, no restart of the cluster is needed until 'init'."
    fi
}

is_initialised() {
    [ -f "$PGDATA/PG_VERSION" ]
}

is_running() {
    is_initialised && pg_ctl -D "$PGDATA" status >/dev/null 2>&1
}

wait_ready() {
    local tries=30
    while [ "$tries" -gt 0 ]; do
        if pg_isready -h 127.0.0.1 -p "$PGPORT" -U "$PGUSER" >/dev/null 2>&1; then
            return 0
        fi
        sleep 0.5
        tries=$((tries - 1))
    done
    return 1
}

cmd_check() {
    require_tools
    require_timescaledb
    printf 'PostgreSQL:  %s\n' "$(pg_config --version)"
    printf 'TimescaleDB: %s\n' "$(grep -m1 default_version "$(pg_config --sharedir)/extension/timescaledb.control" | cut -d"'" -f2)"
    printf 'Cluster:     %s (%s)\n' "$PGDATA" "$(is_initialised && echo initialised || echo absent)"
    if is_running; then
        printf 'State:       running on port %s\n' "$PGPORT"
    else
        printf 'State:       stopped\n'
    fi
}

cmd_init() {
    require_tools
    require_timescaledb

    if is_initialised; then
        die "cluster already exists at $PGDATA (use 'reset' to start over)"
    fi

    mkdir -p "$LOCAL_DIR" "$SOCKET_DIR"
    chmod 700 "$LOCAL_DIR"

    local pwfile
    pwfile="$(mktemp)"
    trap 'rm -f "$pwfile"' EXIT
    printf '%s' "$PGPASSWORD" >"$pwfile"

    printf 'initialising cluster at %s\n' "$PGDATA"
    initdb \
        --pgdata="$PGDATA" \
        --username="$PGUSER" \
        --pwfile="$pwfile" \
        --auth-local=trust \
        --auth-host=scram-sha-256 \
        --encoding=UTF8 \
        --locale=C \
        >/dev/null

    # Local-only cluster: never listens on a public interface.
    cat >>"$PGDATA/postgresql.conf" <<EOF

# --- OddsVault local development -------------------------------------------
listen_addresses = '127.0.0.1'
unix_socket_directories = '$SOCKET_DIR'
shared_preload_libraries = 'timescaledb'
max_connections = 40
log_min_duration_statement = 500
EOF

    start_cluster

    printf 'creating database %s\n' "$PGDATABASE"
    psql -h 127.0.0.1 -p "$PGPORT" -U "$PGUSER" -d postgres -v ON_ERROR_STOP=1 \
        -c "CREATE DATABASE \"$PGDATABASE\"" >/dev/null

    printf '\nready. next:\n  make migrate\n  make api\n'
}

start_cluster() {
    require_timescaledb
    if is_running; then
        printf 'cluster already running on port %s\n' "$PGPORT"
        return 0
    fi
    is_initialised || die "cluster not initialised at $PGDATA; run 'init' first"

    pg_ctl -D "$PGDATA" -l "$PGLOG" -w -t 30 \
        -o "-p $PGPORT -k $SOCKET_DIR" start >/dev/null
    wait_ready || die "cluster did not become ready; see $PGLOG"
    printf 'cluster running on 127.0.0.1:%s\n' "$PGPORT"
}

cmd_start() {
    require_tools
    start_cluster
}

cmd_stop() {
    if ! is_running; then
        printf 'cluster is not running\n'
        return 0
    fi
    pg_ctl -D "$PGDATA" -m fast -w stop >/dev/null
    printf 'cluster stopped (data kept in %s)\n' "$PGDATA"
}

cmd_status() {
    require_tools
    if is_running; then
        printf 'running: 127.0.0.1:%s (data: %s)\n' "$PGPORT" "$PGDATA"
        pg_isready -h 127.0.0.1 -p "$PGPORT" -U "$PGUSER" || true
    elif is_initialised; then
        printf 'stopped: %s\n' "$PGDATA"
    else
        printf 'not initialised: %s\n' "$PGDATA"
    fi
}

cmd_reset() {
    is_running && pg_ctl -D "$PGDATA" -m fast -w stop >/dev/null
    if [ -d "$PGDATA" ]; then
        rm -rf "$PGDATA"
        printf 'deleted %s\n' "$PGDATA"
    else
        printf 'nothing to delete at %s\n' "$PGDATA"
    fi
}

cmd_psql() {
    require_tools
    is_running || die "cluster is not running; run 'start' first"
    exec psql -h 127.0.0.1 -p "$PGPORT" -U "$PGUSER" -d "$PGDATABASE"
}

case "${1:-}" in
check) cmd_check ;;
init) cmd_init ;;
start) cmd_start ;;
stop) cmd_stop ;;
status) cmd_status ;;
reset) cmd_reset ;;
psql) cmd_psql ;;
*)
    printf 'usage: %s {check|init|start|stop|status|reset|psql}\n' "$0" >&2
    exit 2
    ;;
esac
