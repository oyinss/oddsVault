#!/usr/bin/env bash
#
# Expose the local Go API through a free Cloudflare quick tunnel and point the
# Pages project at the new URL.
#
#   make tunnel                    # start the tunnel, update API_ORIGIN
#   CF_UPDATE_SECRET=0 make tunnel # start the tunnel only
#
# A quick tunnel gets a fresh *.trycloudflare.com hostname on every restart.
# That is why the deployed worker reads API_ORIGIN at request time instead of
# baking the host into the static build.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

PORT="${APP_PORT:-8080}"
PROJECT="${CF_PAGES_PROJECT:-oddsvault-web}"
UPDATE_SECRET="${CF_UPDATE_SECRET:-1}"
BIN="${CLOUDFLARED_BIN:-$ROOT/.local/bin/cloudflared}"
LOG="$ROOT/.local/logs/cloudflared.log"
ORIGIN_FILE="$ROOT/.local/api-origin.txt"

mkdir -p "$(dirname "$BIN")" "$(dirname "$LOG")"

if [ ! -x "$BIN" ]; then
  case "$(uname -m)" in
    x86_64 | amd64) arch=amd64 ;;
    aarch64 | arm64) arch=arm64 ;;
    *)
      echo "unsupported architecture: $(uname -m)" >&2
      exit 1
      ;;
  esac
  echo "Downloading cloudflared for linux-$arch …"
  curl -fsSL --retry 3 -o "$BIN" \
    "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-$arch"
  chmod +x "$BIN"
fi

if ! curl -fsS -m 3 "http://127.0.0.1:$PORT/healthz" >/dev/null; then
  echo "No API on http://127.0.0.1:$PORT — start it with 'make api' first." >&2
  exit 1
fi

: >"$LOG"
"$BIN" tunnel --no-autoupdate --url "http://127.0.0.1:$PORT" >"$LOG" 2>&1 &
TUNNEL_PID=$!
trap 'kill "$TUNNEL_PID" 2>/dev/null || true' EXIT INT TERM

URL=""
for _ in $(seq 1 60); do
  URL="$(grep -Eo 'https://[a-z0-9-]+\.trycloudflare\.com' "$LOG" | head -1 || true)"
  if [ -n "$URL" ]; then
    break
  fi
  sleep 0.5
done

if [ -z "$URL" ]; then
  echo "cloudflared did not report a tunnel URL. Last lines:" >&2
  tail -20 "$LOG" >&2
  exit 1
fi

printf '%s\n' "$URL" | tee "$ORIGIN_FILE"
echo "Local API:  http://127.0.0.1:$PORT"
echo "Public API: $URL"

if [ "$UPDATE_SECRET" = "1" ]; then
  echo "Setting API_ORIGIN on the $PROJECT Pages project …"
  if ! (
    cd apps/web &&
      printf '%s' "$URL" | npx wrangler pages secret put API_ORIGIN --project-name "$PROJECT"
  ); then
    echo "Could not set API_ORIGIN automatically. Set it by hand:" >&2
    echo "  make pages-origin URL='$URL'" >&2
  fi
fi

echo "Tunnel running. Keep this process alive (tmux or systemd); Ctrl-C stops it."
wait "$TUNNEL_PID"
