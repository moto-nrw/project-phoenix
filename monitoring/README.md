# Phoenix Monitoring

This directory versions the capacity and data-integrity monitoring layer for
production and staging.

## Services

- Prometheus stores 30 days of metrics.
- Node Exporter captures host CPU, load, RAM, swap, disk, network, and file-descriptor pressure.
- cAdvisor captures Docker container CPU, memory, network, filesystem I/O, restarts, and OOM signals.
- Existing Grafana can import the Prometheus datasource and dashboard provisioning files from `grafana/provisioning`.
- Grafana provisions the Loki datasource, booking-consistency dashboard, and
  the booking-consistency, server-error, and Postgres-error alert rules from
  `grafana/provisioning`.

The stack does not run `postgres_exporter`. Phoenix exports connection-pool
metrics from the backend process. Do not enable the exporter's table-statistics
collector: its repeated `pg_stat_user_tables` scan costs more database time than
the application workload. Disable that collector in any older server-local
monitoring setup before applying this overlay.

## Deploy

For the prepared Grafana version upgrade, use the separate
[`grafana-upgrade/README.md`](grafana-upgrade/README.md) release and full-volume
rollback procedure. It preserves the server-local base and provisioning.

On the server, merge these files into `/root/monitoring` next to the existing Loki/Grafana/Alloy setup. Do not delete the existing Loki, Grafana, or Alloy services when adding this stack.

```bash
cd /root/monitoring
docker compose -f docker-compose.yml -f prometheus-compose.yml up -d
docker compose restart grafana
```

`prometheus-compose.yml` is an additive Compose overlay. Keep the server's existing `/root/monitoring/docker-compose.yml` as the base file because it owns Loki, Grafana, Alloy, and the healthcheck containers.

The Prometheus container reads `METRICS_BEARER_TOKEN` from `/root/monitoring/.env`. Add that key to the existing file next to `GRAFANA_ADMIN_PASSWORD`, using the same value deployed to the production and staging app `.env` files.

Copy the complete `grafana/provisioning` directory before restarting Grafana.
The alert rules use the existing `Phoenix Alerts` folder and the existing
default notification policy; they do not provision or overwrite contact points
or notification policies.

## Booking consistency

Deploy the backend containing the reduced audit log contract before provisioning
the dashboard and rules. The monitoring configuration intentionally ignores the
removed counters and uses only:

- `pickup_projection_missing_days`
- `approved_without_required_offering`
- `approved_without_optional_offering` (review only)
- `total_findings`

The drift rule compares the latest result with the documented Production
baseline in the runbook. Tenants without a documented non-zero baseline use
zero. It excludes tenants where bookings are not the authoritative source, so
their projection counters remain visible in the dashboard without triggering
an alarm. Update both the rule and the runbook when an accepted baseline or the
authoritative tenant set changes.
The runbook is
[`runbooks/booking-consistency.md`](runbooks/booking-consistency.md).

After copying the files and restarting Grafana, verify provisioning without
printing credentials:

```bash
docker compose logs grafana | grep -E "provision|alert"
curl -s http://localhost:3100/ready
```

Then follow the post-deployment checks in the runbook. Test alert delivery in
staging; never inject a synthetic audit failure into Production data.

## Server and Postgres errors

`grafana/provisioning/alerting/server-errors.yml` replaces the hand-made
"Error Spike" rule (#2953). The server rule ignores `transaction_failure`
lines without a 5xx status; the Postgres rule counts `ERROR:` and `PANIC:`
lines from the database container. Both read the `service` label that Alloy
derives from the journald tags `prod-server` and `prod-postgres`. After
copying the file:

```bash
docker compose restart grafana
```

Then delete or pause the manually created "Error Spike" rule in the Grafana
UI so the two do not fire side by side. Verify in Explore that
`{env="prod",service="postgres"}` returns lines before trusting the Postgres
rule; a different label means the Alloy mapping differs and both rules need
the same correction. Runbook: [`runbooks/server-errors.md`](runbooks/server-errors.md).

## Error codes

`grafana/provisioning/dashboards/json/phoenix-error-codes.json` answers
"which error code, how often, at which school" from the frontend client logs
(#2512). The browser writes one `error_displayed` entry per failure a person
saw (`surface`: `toast`, `form`, `load`, `page`). Failed background loads
(`swr_fetch_failed`, `swr_fetch_rate_limited`) and dropped live-update
streams (`sse connection error`) get their own table. All of them carry only
identifiers: `error_code`, `error_class`, `status`, `request_id`,
`error_name`, and field names. `error_class` is the registry class of the
code, `crash` for an exception without an API answer, or `local` for a
failure the screen found itself. A form's own input check counts as
`general.input`. A failed load counts once while SWR retries it. An SSE
error after the server answered (`ready_state` 2) carries no class. The client log route adds `tenant_id`,
`portal` and `user_id` from the verified session. The list of fields that can
name a person and are never logged is in `frontend/src/lib/error-log.ts`.

The queries read `{env="$env",service="frontend"}` and parse the JSON line;
`tenant_id` and `msg` are no Loki labels. Parent sessions are not bound to one
school, so parent portal rows carry no `tenant_id`. Join `request_id` with
the server's request line to find the API route.

Deploy and restart from a checkout with SSH access to the monitoring host:

```bash
scripts/deploy-grafana-provisioning.sh --dry-run <ssh-host> dashboards/json/phoenix-error-codes.json
scripts/deploy-grafana-provisioning.sh <ssh-host> dashboards/json/phoenix-error-codes.json
```

The script copies only the named files below `grafana/provisioning`, saves
their previous versions to `/root/monitoring/backups/grafana-provisioning-<UTC>`,
restarts Grafana, waits for `/api/health` and fails on a `logger=provisioning`
error in the Grafana log. It prints the restore command when a check fails.
Name files one by one on purpose: the host carries dashboards that exist only
there, and copying the whole directory would also switch on alert rules
whose prerequisites (see their sections) are not met yet.

## Provisioned state on the monitoring host

Since 2026-10-23 every dashboard under `grafana/provisioning/dashboards/json`
matches the host, including the five that used to exist only there
(`iot-checkin-performance`, `phoenix-overview`, `phoenix-production`,
`phoenix-scaling`, `phoenix-staging`). Change a dashboard here and deploy it
with `scripts/deploy-grafana-provisioning.sh`, not in the Grafana UI: a
provisioned dashboard edited in the UI is overwritten on the next restart.

`alerting/server-errors.yml` is deployed. `alerting/student-contract.yml` is
not deployed: its collector is not installed, and the rule alerts on missing
data.

`alerting/infrastructure.yml` (backup, host, DB pool, event loop, Postgres
connections) and `alerting/production.yml` (5xx, DB connection, server down)
hold the rules that were created in the Grafana UI, moved to files on
2026-10-23 with their UIDs. The hand-made "Error Spike" rule was removed; the
server error rule replaces it. Every alert rule on the host is now provisioned
from this directory. Create a new rule here, not in the UI: Grafana refuses to
provision a file rule whose UID already exists in the UI, and it does not
allow editing a file rule there. Contact points and the notification policy
stay on the host because they name recipients.

## Student storage Contract observation

Before the #2760 rollback window, install the read-only collector and Grafana
rules described in [the Student Contract runbook](runbooks/student-contract.md).
The Node Exporter overlay includes its textfile collector mount. Missing or stale
observations alert rather than implying zero access; a separate PostgreSQL-log
rule detects old-object query errors after Contract. Provisioning and actual
notification delivery must be checked by the operator before claiming coverage.

## Caddy

`caddy/Caddyfile.observability.example` enables Caddy Prometheus metrics and GDPR-safer JSON access logs:

- no request or response bodies
- no Authorization, Cookie, X-Device-Key, or X-Staff-PIN headers
- query values removed
- IPs masked
- upstream address and upstream latency added for bottleneck analysis

After editing `/etc/caddy/Caddyfile`:

```bash
caddy validate --config /etc/caddy/Caddyfile
systemctl reload caddy
curl -s http://localhost:2019/metrics | head
```

## Grafana Checks

Prometheus targets:

```bash
curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | {job: .labels.job, health: .health}'
```

Prometheus is published on `127.0.0.1:9090` only and also joins the production
and staging Docker networks to scrape the app containers directly. The app
targets use the Compose container names (`production-server-1`,
`production-frontend-1`, `staging-server-1`, `staging-frontend-1`) instead of
host loopback ports.

Caddy is not scraped by default because it runs as a systemd service on the
host. Enable it only after binding Caddy's admin metrics listener to an address
that is reachable from Docker without exposing it publicly.

Useful queries:

```promql
histogram_quantile(0.95, sum by (le, route, env) (rate(phoenix_backend_http_request_duration_seconds_bucket[5m])))
sum by (env, tenant_id, outcome) (rate(phoenix_iot_requests_total[5m]))
phoenix_db_in_use_connections / phoenix_db_open_connections
phoenix_sse_clients
sum by (env, tenant_id) (rate(phoenix_tenant_http_requests_total[1m]))
sum by (env, bucket) (increase(phoenix_rate_limit_rejections_total[5m]))
```
