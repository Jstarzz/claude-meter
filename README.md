# claude-meter

Tiny self-hosted Claude Code usage accounting for teams sharing Claude accounts over SSH.

`claude-meter` transparently attributes each new Claude Code process to the machine that launched it, receives Claude Code's native OpenTelemetry `api_request` events, and keeps queryable history in SQLite.

Developers keep using `claude` normally. They do not run or configure `claude-meter` themselves.

## What it tracks

- person and Tailscale device
- Claude account UUID, account ID, and OAuth email
- Claude session ID and API request ID
- model, effort, query source, duration
- input, output, cache-read, and cache-creation tokens
- Claude Code's estimated API-equivalent USD cost
- API errors
- permanent history by default with `retention_days: 0`

It deliberately does not collect prompt text, assistant responses, tool arguments, or raw API bodies.

## Architecture

```text
Developer machines
       |
    Tailscale
       |
       v
 Shared Claude host
   SSH + Claude Code
       |
 transparent claude wrapper
       |
 claude-meter launch
       |
 identity + Claude OTel
       |
       v
 Collector LXC or VM
 claude-meter serve
       |
     SQLite
       |
 web dashboard
```

Only the collector runs the long-lived `claude-meter serve` process. The shared Claude host only invokes `claude-meter launch` when somebody starts Claude.

The collector does not need its own Tailscale daemon when its private subnet is already reachable from the shared Claude host.

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

When running inside tmux, the resolver first checks the tmux session's current `SSH_CONNECTION` or `SSH_CLIENT`, then falls back to the process environment. This reduces stale SSH attribution in long-running tmux sessions when tmux has refreshed its session environment on attach.

When there is no remote connection metadata, such as a hypervisor web console, local shell, cron, or another non-SSH launch:

- `local_person` configured: attribute it to that person with `device=local`
- `local_person` empty: attribute it to `person=unknown`, `device=local`

A browser-based console usually does not expose a trustworthy human identity inside the guest shell, so `local_person` is the explicit policy for that case.

The Claude account and human are separate dimensions. If a developer switches Claude accounts, subsequent API events carry the new account UUID and email while `meter.person` stays mapped to the developer identity.

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

## Claude updates and self-healing

Claude Code updates frequently, so the launcher does not depend on one permanently pinned version path.

The installer records the discovered Claude versions directory and installs a repair service, path watcher, and timer. The launcher prefers the newest valid executable in the versions directory and avoids resolving back into its own wrapper.

If Claude's updater replaces the user-local `claude` shim, the repair unit records the new real target and restores the metered shim. A periodic timer verifies the launcher as a backup.

Existing Claude processes are not retroactively instrumented. A Claude process must be started through the wrapper to receive `meter.*` attribution and exporter settings.

## Recommended collector

- unprivileged Debian 13 or Ubuntu 24.04 LTS
- 1 vCPU
- 512 MB RAM is ample
- 4 GB disk works, 8 GB gives comfortable update and history headroom
- no Tailscale inside the collector when the surrounding network already routes the collector subnet over the tailnet
- keep port 8787 reachable only from trusted internal or tailnet paths

## Build

On Debian or Ubuntu:

```bash
sudo apt-get install -y build-essential libsqlite3-dev golang-go
make check
```

The resulting binary dynamically links the system SQLite library, `libsqlite3.so.0`.

## 1. Install the collector

```bash
sudo ./scripts/install-server.sh ./claude-meter
sudo openssl rand -hex 32
sudo nano /etc/claude-meter/server.json
sudo systemctl enable --now claude-meter
curl http://127.0.0.1:8787/healthz
```

Set the generated token as `ingest_token` and keep `retention_days: 0` for unlimited history.

Example collector endpoint:

```text
http://10.0.0.50:8787
```

## 2. Install the transparent launcher

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

- `endpoint` to the collector endpoint
- `ingest_token` to exactly the same random token
- `claude_binary` to the real Claude Code executable
- `local_person` to the person who should own non-SSH or local-console launches, or leave it empty for `unknown`
- device mappings to known Tailscale machine names

Example mapping:

```json
{
  "workstation-a": "developer-a",
  "laptop-b": "developer-b",
  "desktop-c": "developer-c"
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

## Claude telemetry settings enforced by the launcher

```text
CLAUDE_CODE_ENABLE_TELEMETRY=1
OTEL_LOGS_EXPORTER=otlp
OTEL_METRICS_EXPORTER=none
OTEL_EXPORTER_OTLP_LOGS_PROTOCOL=http/json
OTEL_EXPORTER_OTLP_LOGS_ENDPOINT=<endpoint>/v1/logs
```

The launcher also disables content-bearing telemetry flags. Custom resource attributes carry `meter.person`, `meter.device`, `meter.node_id`, and `meter.source_ip` when available.

## Dashboard and API

- `GET /` dashboard
- `GET /api/summary?range=7d&person=developer-a`
- `GET /api/history?range=30d&person=developer-b&limit=200`
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

- Keep port 8787 internal or tailnet-only.
- The ingestion endpoint requires a bearer token.
- A shared Unix account is cooperative accounting, not hostile or billing-grade enforcement.
- Anyone able to execute the real versioned Claude binary directly can bypass the wrapper.
- `cost_usd` is Claude Code's estimated API-equivalent cost, not the literal bill for a shared Pro or Max subscription.
