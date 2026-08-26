#!/bin/sh
set -eu
[ "$(id -u)" -eq 0 ] || { echo "run as root" >&2; exit 1; }
BIN="${1:-./claude-meter}"
REQUESTED_CLAUDE="${2:-$(command -v claude || true)}"
[ -n "$REQUESTED_CLAUDE" ] || { echo "Claude binary not found; pass it as argument 2" >&2; exit 1; }
REAL_CLAUDE="$(readlink -f "$REQUESTED_CLAUDE")"

install -Dm755 "$BIN" /usr/local/lib/claude-meter/claude-meter
install -d -m 0755 /etc/claude-meter

if [ "$REAL_CLAUDE" = "/usr/local/bin/claude" ]; then
  cp -a /usr/local/bin/claude /usr/local/lib/claude-meter/claude-real
  chmod 0755 /usr/local/lib/claude-meter/claude-real
  REAL_CLAUDE=/usr/local/lib/claude-meter/claude-real
fi

if [ ! -f /etc/claude-meter/launcher.json ]; then
  sed "s#\"/usr/local/bin/claude-real\"#\"$REAL_CLAUDE\"#" configs/launcher.example.json > /etc/claude-meter/launcher.json
  chmod 0644 /etc/claude-meter/launcher.json
fi

cat >/usr/local/bin/claude <<'SH'
#!/bin/sh
exec /usr/local/lib/claude-meter/claude-meter launch -config /etc/claude-meter/launcher.json "$@"
SH
chmod 0755 /usr/local/bin/claude

if [ "$REQUESTED_CLAUDE" != "/usr/local/bin/claude" ]; then
  rm -f "$REQUESTED_CLAUDE"
  ln -s /usr/local/bin/claude "$REQUESTED_CLAUDE"
fi

echo "Installed metered Claude launcher -> $REAL_CLAUDE"
echo "Edit /etc/claude-meter/launcher.json, then test from an SSH session with:"
echo "  /usr/local/lib/claude-meter/claude-meter identify -config /etc/claude-meter/launcher.json"
