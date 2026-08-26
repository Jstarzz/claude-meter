# claude-meter

Tiny self-hosted Claude Code usage accounting for teams sharing Claude accounts over SSH.

`claude-meter` attributes each Claude Code process to the Tailscale device that opened the SSH connection, receives Claude Code's native OpenTelemetry `api_request` events, and keeps queryable history in SQLite.

## What it tracks

- person and Tailscale device
- Claude account UUID, account ID, and OAuth email
- Claude session ID and API request ID
- model, effort, query source, duration
- input/output/cache-read/cache-creation tokens
- Claude Code's estimated API-equivalent USD cost
- API errors
- permanent history by default (`retention_days: 0`)

It deliberately does **not** collect prompt text, assistant responses, tool arguments, or raw API bodies.

## Architecture

```text
Josiah / Brandon / Yhaneal / Richard
             |
          Tailscale
             |
             v
        bughuntinc
       SSH + Claude Code
             |
     claude-meter launch
   SSH_CONNECTION -> tailscale whois
             |
     meter.person/device tags
             |
     OTLP HTTP/JSON /v1/logs
             v
   Proxmox LXC: claude-meter serve
             |
           SQLite
             |
        web dashboard
```

The Claude account and human are separate dimensions. If Brandon switches Claude accounts, subsequent API events carry the other account UUID/email while `meter.person=brandon` stays the same.

## Recommended Proxmox LXC

- unprivileged Debian 13 or Ubuntu 24.04 LTS
- 1 vCPU
- 512 MB RAM (256 MB works; 512 MB gives comfortable headroom)
- 8 GB disk (4 GB works; 8 GB gives history/update headroom)
- no swap required; 256-512 MB swap is harmless
- Tailscale installed in the LXC
- bind the service only where your tailnet/firewall can reach it

The service itself idles around ~11 MB RSS in a local release build; Tailscale and the OS will use more than the meter. The SQLite database is tiny at this team size, but 8 GB avoids having to think about package updates and long-term history.

For an unprivileged Proxmox LXC, pass `/dev/net/tun` through before installing Tailscale. On current Proxmox this can be done from **Resources → Add → Device Passthrough → `dev/net/tun`**, or from the host with `pct set CTID --dev0 /dev/net/tun`.

## Build

On Debian/Ubuntu:

```bash
sudo apt-get install -y build-essential libsqlite3-dev
make check
```

The resulting binary dynamically links the system SQLite library (`libsqlite3.so.0`) and otherwise keeps the service dependency-light.

## 1. Install the server in the LXC

```bash
sudo ./scripts/install-server.sh ./claude-meter
sudo openssl rand -hex 32
sudo nano /etc/claude-meter/server.json
sudo systemctl enable --now claude-meter
curl http://127.0.0.1:8787/healthz
```

Set the generated token as `ingest_token` and keep `retention_days: 0` for unlimited history.

Use the LXC's MagicDNS name or Tailscale IP as the endpoint, for example `http://claude-meter:8787`.

## 2. Install the launcher on `bughuntinc`

Copy the same binary/repository to `bughuntinc`, then:

```bash
sudo ./scripts/install-launcher.sh ./claude-meter /absolute/path/to/the/real/claude
sudo nano /etc/claude-meter/launcher.json
```

Set:

- `endpoint` to the LXC endpoint
- `ingest_token` to exactly the same random token
- `claude_binary` to the real Claude Code executable
- device mappings to the names returned by `tailscale whois`

Current starter mapping is already in `configs/launcher.example.json`:

```json
{
  "desktop-josiah": "josiah",
  "brandons-macbook-pro": "brandon",
  "him-092507": "yhaneal",
  "rich23": "richard",
  "richards-macbook-air": "richard",
  "richards-pc": "richard"
}
```

From an SSH session, verify attribution before touching Claude:

```bash
/usr/local/lib/claude-meter/claude-meter identify -config /etc/claude-meter/launcher.json
```

The installer places the metered launcher at `/usr/local/bin/claude`, which normally wins PATH lookup over user-local installs. It preserves a conflicting real `/usr/local/bin/claude` under `/usr/local/lib/claude-meter/claude-real` to avoid recursion. After attribution succeeds, run `claude` normally.

## Claude telemetry settings enforced by the launcher

The launcher sets logs-only OTLP over HTTP/JSON:

```text
CLAUDE_CODE_ENABLE_TELEMETRY=1
OTEL_LOGS_EXPORTER=otlp
OTEL_METRICS_EXPORTER=none
OTEL_EXPORTER_OTLP_LOGS_PROTOCOL=http/json
OTEL_EXPORTER_OTLP_LOGS_ENDPOINT=http://claude-meter:8787/v1/logs
```

It also explicitly disables content-bearing telemetry flags. Custom resource attributes carry `meter.person`, `meter.device`, `meter.node_id`, and `meter.source_ip`.

## Dashboard/API

- `GET /` dashboard
- `GET /api/summary?range=7d&person=brandon`
- `GET /api/history?range=30d&person=josiah&limit=200`
- `GET /healthz`
- `POST /v1/logs` authenticated OTLP ingestion

Ranges: `24h`, `7d`, `30d`, `all`.

## Storage and history

SQLite runs in WAL mode. Every `api_request` is stored separately, so reports can be regrouped later by person, device, Claude account, session, model, or date. Duplicate OTLP deliveries are deduplicated with a deterministic event key.

Back up one directory:

```text
/var/lib/claude-meter/
```

For a consistent live backup, use SQLite's backup command or stop the service briefly before copying `usage.db`.

## Security notes

- Keep port 8787 tailnet-only; do not expose it with Tailscale Funnel.
- The ingestion endpoint requires a bearer token.
- A shared Unix account is adequate for cooperative accounting, not hostile/billing-grade enforcement. A user who can directly execute the real Claude binary can bypass the launcher.
- For stronger enforcement, use separate Unix users and root-managed Claude settings/launcher paths.
- `cost_usd` is Claude Code's estimated API-equivalent cost. On a shared Pro/Max subscription it is not the literal amount billed by Anthropic.
