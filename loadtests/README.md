# Phoenix Capacity Tests

These tests answer one question: where does the current single-server architecture bend before school-start traffic bends it for us?

## What To Test

Use staging or a dedicated throwaway environment with production-like data:

| Scale | Schools | Users | Students | Devices |
| --- | ---: | ---: | ---: | ---: |
| Near term | 10 | 200 | 1,500 | 100 |
| Mid test | 25 | 500 | 3,750 | 250 |
| Year-1 target | 50 | 1,000 | 7,500 | 500 |

The first-day risk is not average traffic. It is overlapping morning bursts, open dashboards, SSE streams, and Postgres doing real work at the same time.

## Observability During Tests

The backend logs a `capacity snapshot` once per minute. It includes:

- Go runtime: goroutines, heap, GC cycles
- HTTP: active requests and the worst route/status buckets
- SSE: active client count
- Postgres pool: open, in-use, idle, wait count, wait duration

Useful Loki queries:

```logql
{env="staging", service="server"} |= "capacity snapshot"
{env="staging", service="server"} | json | msg = "capacity snapshot"
{env="staging", service="server"} | json | db_wait_count > 0
```

Postgres query diagnostics are enabled through `pg_stat_statements`. After a test run:

```bash
docker exec -i staging-postgres-1 psql -U postgres -d postgres < loadtests/sql/capacity-snapshot.sql
```

## Running The HTTP K6 Test

Install k6 locally or run it via Docker. This covers RFID/check-in bursts and authenticated dashboard reads. Keep secrets in your shell, not in git.

```bash
export API_BASE_URL="https://api-staging.moto-app.de"
export DEVICE_API_KEY="..."
export STAFF_PIN="..."
export WEB_JWT="..."
export RFIDS="04AABBCC,04DDEEFF,04112233"
export ROOM_IDS="101,102,103"

k6 run loadtests/k6/phoenix-capacity.js
```

Docker alternative:

```bash
docker run --rm -i \
  -e API_BASE_URL \
  -e DEVICE_API_KEY \
  -e STAFF_PIN \
  -e WEB_JWT \
  -e RFIDS \
  -e ROOM_IDS \
  -v "$PWD/loadtests:/loadtests:ro" \
  grafana/k6 run /loadtests/k6/phoenix-capacity.js
```

## Holding SSE Connections

Run the SSE holder in a second terminal while k6 is running. It uses Node's native `fetch()` to keep real EventSource-style HTTP streams open without turning intentional timeouts into failed k6 requests.

```bash
export API_BASE_URL="https://api-staging.moto-app.de"
export WEB_JWT="..."

SSE_CONNECTIONS=300 SSE_HOLD_SECONDS=900 node loadtests/node/sse-hold.mjs
```

## Parallel Supervision Acceptance (#2458)

Run this against staging or a dedicated throwaway environment. It opens one
SSE stream for each staff account, then both staff members check in ten distinct
children in parallel (sequentially per person). It fails on any 429, non-2xx
response, missing combined refresh, or SSE wire delivery over two seconds.

```bash
export API_BASE_URL="https://api-staging.moto-app.de"
export WEB_JWTS="<staff-1-jwt>,<staff-2-jwt>"
export TIMETABLE_INSTANCE_ID="<shared-instance>"
export ACTIVE_GROUP_ID="<shared-instance-active-group>"
export STUDENT_IDS="<10 comma-separated ids>;<10 comma-separated ids>"

node loadtests/node/parallel-supervision.mjs
```

The students must start at home. Both tokens must be assigned to the same active
timetable block and allowed to check in their ten students. Each action waits
for both writes in that round to reach the other staff member's SSE stream.
During the run, verify the
Grafana **Requests/s by tenant** panel stays below 20 RPS and the **Rate-limit
rejections** panel stays at zero. The script measures server-to-client SSE wire
latency; verify the open browser rosters render each update within two seconds
to close the end-to-end UI criterion.

### Request budgets

The changed hot paths are pinned by tests:

| Surface             |                                             Cold page / action budget |                          SSE budget |
| ------------------- | --------------------------------------------------------------------: | ----------------------------------: |
| Shared user context |                                                     1 backend request |               only on access change |
| Active supervision  | 1 aggregate request, plus 1 roster when a timetable block is selected |  1 aggregate revalidation per burst |
| Room detail         |                                           2 parallel backend requests | no extra page-specific subscription |
| OGS group           |                                        1 aggregated live-data request |         1 scoped group revalidation |

The unfiltered **Alle Kinder** search intentionally makes one slim-list request
after a movement because every row can show live location; filtered searches
skip events from other educational groups.

## Recommended Scenarios

Baseline:

```bash
SCAN_PEAK_RATE=20 WEB_VUS=50 k6 run loadtests/k6/phoenix-capacity.js
# Optional parallel SSE:
SSE_CONNECTIONS=50 node loadtests/node/sse-hold.mjs
```

Expected 50-school morning burst:

```bash
SCAN_PEAK_RATE=100 WEB_VUS=300 k6 run loadtests/k6/phoenix-capacity.js
# Parallel terminal:
SSE_CONNECTIONS=300 node loadtests/node/sse-hold.mjs
```

Headroom test:

```bash
SCAN_PEAK_RATE=200 WEB_VUS=500 k6 run loadtests/k6/phoenix-capacity.js
# Parallel terminal:
SSE_CONNECTIONS=500 node loadtests/node/sse-hold.mjs
```

Soak test:

```bash
SCAN_PEAK_RATE=50 WEB_VUS=250 \
SCAN_HOLD_DURATION=4h WEB_DURATION=4h \
k6 run loadtests/k6/phoenix-capacity.js

# Parallel terminal:
SSE_CONNECTIONS=250 SSE_HOLD_SECONDS=14400 node loadtests/node/sse-hold.mjs
```

## Pass Criteria

- Check-in p95 below 500 ms, p99 below 1 s
- Dashboard p95 below 300 ms, p99 below 1 s
- 5xx rate below 0.1 percent
- `db_wait_count` does not climb continuously
- Postgres CPU is not pinned for minutes at a time
- Heap does not grow steadily during a 4-hour soak
- SSE client count reaches the expected value and disconnects cleanly after the run

## How To Interpret Failures

- Go or Next.js CPU pinned: scale app containers or move services apart.
- DB wait count grows: increase DB pool only after checking Postgres CPU, locks, and slow queries.
- Slow dashboard reads: inspect `pg_stat_statements`, then add indexes or reduce query fan-out.
- SSE limits show first: consider bypassing the Next.js SSE proxy or isolating realtime traffic.
- Backups cause latency: move backup windows, reduce I/O contention, or split Postgres to its own host.
