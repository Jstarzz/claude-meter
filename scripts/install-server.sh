#!/bin/sh
set -eu
[ "$(id -u)" -eq 0 ] || { echo "run as root" >&2; exit 1; }
BIN="${1:-./claude-meter}"
id claude-meter >/dev/null 2>&1 || useradd --system --home /var/lib/claude-meter --shell /usr/sbin/nologin claude-meter
install -Dm755 "$BIN" /usr/local/bin/claude-meter
install -d -o claude-meter -g claude-meter -m 0750 /var/lib/claude-meter
install -d -m 0750 /etc/claude-meter
if [ ! -f /etc/claude-meter/server.json ]; then
  install -o root -g claude-meter -m 0640 configs/server.example.json /etc/claude-meter/server.json
fi
chown root:claude-meter /etc/claude-meter/server.json
chmod 0640 /etc/claude-meter/server.json
install -m 0644 deploy/systemd/claude-meter.service /etc/systemd/system/claude-meter.service
systemctl daemon-reload
echo "Edit /etc/claude-meter/server.json, then: systemctl enable --now claude-meter"
