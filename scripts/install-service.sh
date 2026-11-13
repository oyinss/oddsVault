#!/usr/bin/env bash
#
# Install (or refresh) the user-level systemd units that keep OddsVault running
# and back the database up nightly. No sudo is required; linger is already
# enabled for this user.
#
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
UNIT_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
mkdir -p "$UNIT_DIR"

cat >"$UNIT_DIR/oddsvault.service" <<EOF
[Unit]
Description=OddsVault runtime (api, web, workers)
After=network-online.target

[Service]
Type=simple
WorkingDirectory=$ROOT
ExecStart=$ROOT/scripts/runtime.sh supervise
Restart=always
RestartSec=5
KillMode=control-group
TimeoutStopSec=20

[Install]
WantedBy=default.target
EOF

cat >"$UNIT_DIR/oddsvault-model-picks.service" <<EOF
[Unit]
Description=OddsVault model picks run
Wants=oddsvault.service

[Service]
Type=oneshot
WorkingDirectory=$ROOT
EnvironmentFile=-$ROOT/.env
ExecStart=$ROOT/.local/bin/model-picks
EOF

cat >"$UNIT_DIR/oddsvault-model-picks.timer" <<EOF
[Unit]
Description=Refresh OddsVault model picks

[Timer]
OnCalendar=*-*-* 00/6:15:00
Persistent=true

[Install]
WantedBy=timers.target
EOF

cat >"$UNIT_DIR/oddsvault-backup.service" <<EOF
[Unit]
Description=OddsVault database backup

[Service]
Type=oneshot
WorkingDirectory=$ROOT
ExecStart=$ROOT/scripts/backup.sh
EOF

cat >"$UNIT_DIR/oddsvault-backup.timer" <<EOF
[Unit]
Description=Daily OddsVault backup

[Timer]
OnCalendar=*-*-* 03:00:00
Persistent=true
RandomizedDelaySec=15m

[Install]
WantedBy=timers.target
EOF

systemctl --user daemon-reload
systemctl --user enable --now oddsvault-backup.timer
systemctl --user enable --now oddsvault-model-picks.timer

if [ "${SKIP_RUNTIME:-0}" != "1" ]; then
    systemctl --user enable --now oddsvault.service
fi

echo "installed:"
systemctl --user --no-pager status oddsvault.service | head -6 || true
systemctl --user --no-pager list-timers oddsvault-backup.timer --all | head -3 || true
