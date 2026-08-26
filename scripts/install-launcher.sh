#!/bin/sh
set -eu
[ "$(id -u)" -eq 0 ] || { echo "run as root" >&2; exit 1; }
BIN="${1:-./claude-meter}"
CURRENT_CLAUDE="$(command -v claude || true)"
REQUESTED_CLAUDE="${2:-$CURRENT_CLAUDE}"
[ -n "$REQUESTED_CLAUDE" ] || { echo "Claude binary not found; pass it as argument 2" >&2; exit 1; }
REAL_CLAUDE="$(readlink -f "$REQUESTED_CLAUDE")"
CURRENT_REAL=""
[ -z "$CURRENT_CLAUDE" ] || CURRENT_REAL="$(readlink -f "$CURRENT_CLAUDE")"

install -Dm755 "$BIN" /usr/local/lib/claude-meter/claude-meter
install -Dm755 scripts/repair-launcher.sh /usr/local/lib/claude-meter/repair-launcher.sh
install -d -m 0755 /etc/claude-meter

if [ "$REAL_CLAUDE" = "/usr/local/bin/claude" ] && [ -s /etc/claude-meter/real-claude ]; then
  REAL_CLAUDE="$(cat /etc/claude-meter/real-claude)"
elif [ "$REAL_CLAUDE" = "/usr/local/bin/claude" ]; then
  cp -a /usr/local/bin/claude /usr/local/lib/claude-meter/claude-real
  chmod 0755 /usr/local/lib/claude-meter/claude-real
  REAL_CLAUDE=/usr/local/lib/claude-meter/claude-real
fi

printf '%s\n' "$REAL_CLAUDE" > /etc/claude-meter/real-claude
chmod 0644 /etc/claude-meter/real-claude
VERSION_DIR="$(dirname "$REAL_CLAUDE")"
if [ "$(basename "$VERSION_DIR")" = versions ]; then
  printf '%s\n' "$VERSION_DIR" > /etc/claude-meter/versions-dir
  chmod 0644 /etc/claude-meter/versions-dir
fi

SHIM=""
if [ -n "$CURRENT_CLAUDE" ] && [ "$CURRENT_CLAUDE" != "/usr/local/bin/claude" ]; then
  SHIM="$CURRENT_CLAUDE"
  printf '%s\n' "$SHIM" > /etc/claude-meter/claude-shim
  chmod 0644 /etc/claude-meter/claude-shim
fi

if [ ! -f /etc/claude-meter/launcher.json ]; then
  sed "s#\"/usr/local/bin/claude-real\"#\"$REAL_CLAUDE\"#" configs/launcher.example.json > /etc/claude-meter/launcher.json
  chmod 0644 /etc/claude-meter/launcher.json
fi

cat >/usr/local/bin/claude <<'SH'
#!/bin/sh
METER=/usr/local/lib/claude-meter/claude-meter
REAL_CLAUDE="$(cat /etc/claude-meter/real-claude 2>/dev/null || true)"
VERSION_DIR="$(cat /etc/claude-meter/versions-dir 2>/dev/null || true)"
if [ -n "$VERSION_DIR" ] && [ -d "$VERSION_DIR" ]; then
  NEWEST="$(find "$VERSION_DIR" -maxdepth 1 -type f -perm /111 -print 2>/dev/null | sort -V | tail -n 1)"
  if [ -n "$NEWEST" ] && [ -x "$NEWEST" ]; then
    REAL_CLAUDE="$NEWEST"
  fi
fi
if [ -x "$METER" ]; then
  exec "$METER" launch -config /etc/claude-meter/launcher.json -fallback-bin "$REAL_CLAUDE" -- "$@"
fi
if [ -n "$REAL_CLAUDE" ] && [ -x "$REAL_CLAUDE" ]; then
  exec "$REAL_CLAUDE" "$@"
fi
if [ -x /usr/local/lib/claude-meter/claude-recovered ]; then
  exec /usr/local/lib/claude-meter/claude-recovered "$@"
fi
if [ -x /usr/local/lib/claude-meter/claude-real ]; then
  exec /usr/local/lib/claude-meter/claude-real "$@"
fi
printf '%s\n' 'claude: real Claude binary unavailable' >&2
exit 127
SH
chmod 0755 /usr/local/bin/claude

if [ -n "$CURRENT_CLAUDE" ] && [ "$CURRENT_CLAUDE" != "/usr/local/bin/claude" ] && [ "$CURRENT_REAL" = "$(readlink -f "$REQUESTED_CLAUDE")" ]; then
  rm -f "$CURRENT_CLAUDE"
  ln -s /usr/local/bin/claude "$CURRENT_CLAUDE"
fi

if [ -n "$SHIM" ]; then
  cat >/etc/systemd/system/claude-meter-repair.service <<'UNIT'
[Unit]
Description=Repair Claude meter launcher
After=local-fs.target

[Service]
Type=oneshot
ExecStart=/usr/local/lib/claude-meter/repair-launcher.sh
UNIT

  cat >/etc/systemd/system/claude-meter-repair.path <<UNIT
[Unit]
Description=Watch Claude launcher for updates

[Path]
PathChanged=$SHIM
Unit=claude-meter-repair.service

[Install]
WantedBy=multi-user.target
UNIT

  cat >/etc/systemd/system/claude-meter-repair.timer <<'UNIT'
[Unit]
Description=Periodically verify Claude meter launcher

[Timer]
OnBootSec=10s
OnUnitActiveSec=60s
AccuracySec=10s
Persistent=true
Unit=claude-meter-repair.service

[Install]
WantedBy=timers.target
UNIT

  systemctl daemon-reload
  systemctl enable --now claude-meter-repair.path claude-meter-repair.timer
  /usr/local/lib/claude-meter/repair-launcher.sh
fi

echo "Installed metered Claude launcher -> $REAL_CLAUDE"
echo "Edit /etc/claude-meter/launcher.json, then test from an SSH session with:"
echo "  /usr/local/lib/claude-meter/claude-meter identify -config /etc/claude-meter/launcher.json"
