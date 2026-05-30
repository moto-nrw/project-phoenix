# Phoenix Monitoring

This directory versions the capacity-monitoring layer for production and staging.

## Services

- Prometheus stores 30 days of metrics.
- Node Exporter captures host CPU, load, RAM, swap, disk, network, and file-descriptor pressure.
- cAdvisor captures Docker container CPU, memory, network, filesystem I/O, restarts, and OOM signals.
- Existing Grafana can import the Prometheus datasource and dashboard provisioning files from `grafana/provisioning`.

## Deploy

On the server, merge these files into `/root/monitoring` next to the existing Loki/Grafana/Alloy setup. Do not delete the existing Loki, Grafana, or Alloy services when adding this stack.

```bash
cd /root/monitoring
docker compose -f docker-compose.yml -f prometheus-compose.yml up -d
docker compose restart grafana
```

`prometheus-compose.yml` is an additive Compose overlay. Keep the server's existing `/root/monitoring/docker-compose.yml` as the base file because it owns Loki, Grafana, Alloy, and the healthcheck containers.

The Prometheus container reads `METRICS_BEARER_TOKEN` from `/root/monitoring/.env`. Add that key to the existing file next to `GRAFANA_ADMIN_PASSWORD`, using the same value deployed to the production and staging app `.env` files.

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

Prometheus runs with host networking so it can scrape the localhost-bound Caddy,
production, and staging ports without exposing those services publicly. cAdvisor
is published locally as `127.0.0.1:8082` to avoid colliding with the backend on
`127.0.0.1:8080`.

Useful queries:

```promql
histogram_quantile(0.95, sum by (le, route, env) (rate(phoenix_backend_http_request_duration_seconds_bucket[5m])))
sum by (env, tenant_id, outcome) (rate(phoenix_iot_requests_total[5m]))
phoenix_db_in_use_connections / phoenix_db_open_connections
phoenix_sse_clients
```
