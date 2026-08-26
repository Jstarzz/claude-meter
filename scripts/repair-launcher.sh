#!/bin/sh
set -eu
WRAPPER=/usr/local/bin/claude
METER=/usr/local/lib/claude-meter/claude-meter
SHIM="$(cat /etc/claude-meter/claude-shim 2>/dev/null || true)"
[ -n "$SHIM" ] || exit 0
[ -x "$WRAPPER" ] || exit 0

write_real() {
  printf '%s\n' "$1" > /etc/claude-meter/real-claude.tmp
  chmod 0644 /etc/claude-meter/real-claude.tmp
  mv -f /etc/claude-meter/real-claude.tmp /etc/claude-meter/real-claude
  DIR="$(dirname "$1")"
  if [ "$(basename "$DIR")" = versions ]; then
    printf '%s\n' "$DIR" > /etc/claude-meter/versions-dir.tmp
    chmod 0644 /etc/claude-meter/versions-dir.tmp
    mv -f /etc/claude-meter/versions-dir.tmp /etc/claude-meter/versions-dir
  fi
}

if [ -L "$SHIM" ]; then
  CURRENT="$(readlink -f "$SHIM" 2>/dev/null || true)"
  if [ -n "$CURRENT" ] && [ "$CURRENT" != "$WRAPPER" ] && [ "$CURRENT" != "$METER" ] && [ -x "$CURRENT" ]; then
    write_real "$CURRENT"
  fi
elif [ -x "$SHIM" ] && [ "$SHIM" != "$WRAPPER" ]; then
  cp -f "$SHIM" /usr/local/lib/claude-meter/claude-recovered
  chmod 0755 /usr/local/lib/claude-meter/claude-recovered
  write_real /usr/local/lib/claude-meter/claude-recovered
fi

if [ -L "$SHIM" ] && [ "$(readlink -f "$SHIM" 2>/dev/null || true)" = "$WRAPPER" ]; then
  exit 0
fi

mkdir -p "$(dirname "$SHIM")"
TMP="${SHIM}.claude-meter.$$"
rm -f "$TMP"
ln -s "$WRAPPER" "$TMP"
mv -Tf "$TMP" "$SHIM"
