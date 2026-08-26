# claude-meter

Tiny self-hosted Claude Code usage accounting for teams sharing Claude accounts over SSH.

`claude-meter` transparently attributes each new Claude Code process to the machine that launched it, receives Claude Code's native OpenTelemetry `api_request` events, and keeps queryable history in SQLite.

Developers keep using `claude` normally. They do not run or configure `claude-meter` themselves.

## What it tracks

- person and Tailscale device
- Claude account UUID, account ID, and OAuth email
- Claude session ID and API request ID
- model, effort, query source, duration
- input/output/cache-read/cache-creation tokens
- Claude Code's estimated API-equivalent USD cost
- API errors
- permanent history by default (`retention_days: 0`)

It deliberately does not collect prompt text, assistant responses, tool arguments, or raw API bodies.

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
     transparent claude wrapper
             |
     claude-meter launch
             |
  identity + native Claude OTel
             |
             v
   172.16.0.69:8787
   Proxmox LXC: claude-meter serve
             |
           SQLite
             |
        web dashboard
```

Only the LXC runs the long-lived `claude-meter serve` process. `bughuntinc` only invokes `claude-meter launch` when somebody starts Claude.

The LXC does not need its own Tailscale daemon when its private subnet is already reachable through the Proxmox host's Tailscale/subnet-routing setup.

## Identity behavior

Resolution is deliberately fail-open. Metering must never prevent Claude Code from starting.

For a normal SSH launch:

```text
SSH_CONNECTION / SSH_CLIENT
        |
        v
Tailscale source IP
        |
        v
tailscale whois
        |
        +-- mapped device ----> configured person
        |
        +-- unmapped device --> person=unknown, device preserved
        |
        +-- whois failure ----> person=unknown, device=unresolved, source IP preserved
```

When running inside tmux, the resolver first checks the tmux session's current `SSH_CONNECTION`/`SSH_CLIENT`, then falls back to the process environment. This avoids stale SSH attribution in long-running tmux sessions when tmux has refreshed its session environment on attach.

When there is no remote connection metadata, such as a Proxmox web console, local shell, cron, or another non-SSH launch:

- `local_person` configured: attribute it to that person with `device=local`
- `local_person` empty: attribute it to `person=unknown`, `device=local`

A Proxmox browser console does not expose a trustworthy human identity inside the guest shell, so `local_person` is the explicit policy for that case.

The Claude account and human are separate dimensions. If Brandon switches Claude accounts, subsequent API events carry the other account UUID/email while `meter.person=brandon` stays the same.

## Failure behavior

The launcher is designed not to become a dependency for development work:

- unknown devices still launch Claude and are metered as `unknown`
- missing or failing `tailscale whois` still launches Claude
- missing SSH metadata still launches Claude
- malformed or missing launcher configuration falls back to the real Claude executable
- if the meter binary is missing, the shell wrapper directly launches the saved real Claude executable
- Claude command-line flags are passed through untouched
- an unavailable collector does not intentionally block Claude Code

Set `CLAUDE_METER_DEBUG=1` only when you want launcher fallback diagnostics on stderr.

## Recommended LXC

- unprivileged Debian 13 or Ubuntu 24.04 LTS
- 1 vCPU
- 512 MB RAM is ample
- 4 GB disk works; 8 GB gives comfortable update/history headroom
- no Tailscale inside the LXC when the Proxmox host already routes the LXC subnet over the tailnet
- keep port 8787 reachable only from trusted internal/tailnet paths

## Build

On Debian/Ubuntu:

```bash
sudo apt-get install -y build-essential libsqlite3-dev golang-go
make check
```

The resulting binary dynamically links the system SQLite library (`libsqlite3.so.0`).

## 1. Install the server in the LXC

```bash
sudo ./scripts/install-server.sh ./claude-meter
sudo openssl rand -hex 32
sudo nano /etc/claude-meter/server.json
sudo systemctl enable --now claude-meter
curl http://127.0.0.1:8787/healthz
```

Set the generated token as `ingest_token` and keep `retention_days: 0` for unlimited history.

For the current deployment the collector endpoint is:

```text
http://172.16.0.69:8787
```

## 2. Install the transparent launcher on `bughuntinc`

Capture the real Claude executable before installing the wrapper:

```bash
REAL_CLAUDE="$(readlink -f "$(command -v claude)")"
```

Then:

```bash
sudo ./scripts/install-launcher.sh ./claude-meter "$REAL_CLAUDE"
sudo nano /etc/claude-meter/launcher.json
```

Set:

- `endpoint` to the LXC endpoint
- `ingest_token` to exactly the same random token
- `claude_binary` to the real Claude Code executable
- `local_person` to the person who should own non-SSH/local-console launches, or leave it empty for `unknown`
- device mappings to known Tailscale machine names

Current starter mapping:

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

Verify attribution from an SSH session:

```bash
/usr/local/lib/claude-meter/claude-meter identify -config /etc/claude-meter/launcher.json
```

Then use Claude normally:

```bash
claude
claude --version
claude --dangerously-skip-permissions
```

The installer keeps the original real-Claude path in `/etc/claude-meter/real-claude`, installs the meter at `/usr/local/bin/claude`, and repoints the currently winning user-local Claude shim when appropriate.

Existing Claude processes are not retroactively instrumented. A Claude process must be started through the wrapper to receive `meter.*` attribution and exporter settings.

## Claude telemetry settings enforced by the launcher

```text
CLAUDE_CODE_ENABLE_TELEMETRY=1
OTEL_LOGS_EXPORTER=otlp
OTEL_METRICS_EXPORTER=none
OTEL_EXPORTER_OTLP_LOGS_PROTOCOL=http/json
OTEL_EXPORTER_OTLP_LOGS_ENDPOINT=<endpoint>/v1/logs
```

The launcher also disables content-bearing telemetry flags. Custom resource attributes carry `meter.person`, `meter.device`, `meter.node_id`, and `meter.source_ip` when available.

## Dashboard/API

- `GET /` dashboard
- `GET /api/summary?range=7d&person=brandon`
- `GET /api/history?range=30d&person=josiah&limit=200`
- `GET /healthz`
- `POST /v1/logs` authenticated OTLP ingestion

Ranges: `24h`, `7d`, `30d`, `all`.

Unknown devices naturally appear under the `unknown` person until they are mapped for future launches.

## Storage and history

SQLite runs in WAL mode. Every `api_request` is stored separately, so reports can be regrouped by person, device, Claude account, session, model, or date. Duplicate OTLP deliveries are deduplicated with a deterministic event key.

Back up:

```text
/var/lib/claude-meter/
```

## Security notes

- Keep port 8787 internal/tailnet-only.
- The ingestion endpoint requires a bearer token.
- A shared Unix account is cooperative accounting, not hostile or billing-grade enforcement.
- Anyone able to execute the real versioned Claude binary directly can bypass the wrapper.
- `cost_usd` is Claude Code's estimated API-equivalent cost, not the literal bill for a shared Pro/Max subscription.
