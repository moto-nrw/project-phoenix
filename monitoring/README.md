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
