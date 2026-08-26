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

if [ ! -f /etc/claude-meter/launcher.json ]; then
  sed "s#\"/usr/local/bin/claude-real\"#\"$REAL_CLAUDE\"#" configs/launcher.example.json > /etc/claude-meter/launcher.json
  chmod 0644 /etc/claude-meter/launcher.json
fi

cat >/usr/local/bin/claude <<'SH'
#!/bin/sh
METER=/usr/local/lib/claude-meter/claude-meter
REAL_CLAUDE="$(cat /etc/claude-meter/real-claude 2>/dev/null || true)"
if [ -x "$METER" ]; then
  exec "$METER" launch -config /etc/claude-meter/launcher.json -fallback-bin "$REAL_CLAUDE" -- "$@"
fi
if [ -n "$REAL_CLAUDE" ] && [ -x "$REAL_CLAUDE" ]; then
  exec "$REAL_CLAUDE" "$@"
fi
printf '%s\n' 'claude: real Claude binary unavailable' >&2
exit 127
SH
chmod 0755 /usr/local/bin/claude

if [ -n "$CURRENT_CLAUDE" ] && [ "$CURRENT_CLAUDE" != "/usr/local/bin/claude" ] && [ "$CURRENT_REAL" = "$(readlink -f "$REQUESTED_CLAUDE")" ]; then
  rm -f "$CURRENT_CLAUDE"
  ln -s /usr/local/bin/claude "$CURRENT_CLAUDE"
fi

echo "Installed metered Claude launcher -> $REAL_CLAUDE"
echo "Edit /etc/claude-meter/launcher.json, then test from an SSH session with:"
echo "  /usr/local/lib/claude-meter/claude-meter identify -config /etc/claude-meter/launcher.json"
