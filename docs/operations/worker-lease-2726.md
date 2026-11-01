# Worker lease and standalone Worker (#2726)

The background jobs run in exactly one process at a time. Every Worker, the
one embedded in `serve` and the standalone `worker` process, leads only while
it holds the lease `worker` in `platform.worker_leases` (migration 1.15.437).
A second Worker waits in standby and runs nothing.

## How the lease works

| Column | Meaning |
|---|---|
| `lease_name` | `worker`; one row per lease |
| `holder_id` | `host/pid/start-sequence` of the process that holds it |
| `fencing_token` | grows by one with every new holder, never reused |
| `lease_until` | database time; no process clock decides who leads |
| `updated_at` | last acquire, renewal or release |

- A Worker acquires the lease when it is free, expired, or its own, renews it
  every 10 s for 30 s, and retries every 5 s in standby.
- The leader stops on its own 25 s after its last successful renewal,
  measured on its monotonic clock from before the statement was sent. That is
  5 s before the database lets a standby in, whatever either clock says.
- Every transaction a job opens calls `platform.assert_worker_lease` as its
  last statement before commit. A term that ended, or that ends within 5 s,
  cannot commit; the job rolls back and the Worker steps down at once. The
  call share-locks the lease row until the commit, so a takeover waits for
  commits its predecessor already confirmed.
- The commit fence covers database writes only. Effects outside the
  database, such as a sent e-mail or a deleted file, are bounded by the
  leader's own deadline and the cancelled job context. An e-mail sent just
  before a takeover can be sent again by the new leader when the fence
  rejects the delivery record; delivery stays at least once.
- Losing the lease cancels the running jobs. A graceful stop cancels and
  drains the jobs while the lease stays renewed, then releases it.

Takeover bounds: a stopped leader is replaced within the 5 s retry interval;
a crashed or unreachable one within 35 s (TTL plus retry).

The administrative role acquires, renews and releases. Job transactions under
the tenant role reach the lease only through the `SECURITY DEFINER` function;
the tenant role has no grant on the table, and the table holds no tenant data.

## Processes

| Command | Runs |
|---|---|
| `./main serve` | HTTP API and the embedded Worker (unchanged default) |
| `./main serve --embedded-worker=false` | HTTP API only |
| `./main worker` | standalone Worker; same configuration as `serve` |

The standalone Worker listens on `PORT` and serves only:

- `/health`: 200 while the process lives.
- `/ready`: 200 while it holds the lease and runs jobs, 503 (`standby`)
  otherwise. Do not restart a process because it is not ready; a standby is
  healthy.
- `/internal/metrics`: Prometheus, behind `METRICS_BEARER_TOKEN`.

## Cutover to the standalone Worker

1. Deploy the image with migration 1.15.437. `serve` keeps leading.
2. Start `./main worker` with the server's environment. It logs no
   `worker lease acquired`, and `/ready` answers 503.
3. Restart `serve` with `--embedded-worker=false`. Its Worker drains, releases
   the lease and logs `worker lease ended reason=released`.
4. Within 5 s the standalone Worker logs `worker lease acquired` with the next
   fencing token, and `/ready` answers 200.

At no point do two Workers lead: step 2 waits for the lease, step 3 hands it
over. Starting the standalone Worker before the embedded one is turned off is
safe for the same reason.

## Rollback

1. Restart `serve` without `--embedded-worker=false`. It waits in standby.
2. Stop the standalone Worker. It releases the lease; if it cannot reach the
   database, the lease expires within 30 s.
3. The embedded Worker logs `worker lease acquired`.

Keep `platform.worker_leases`. Never edit `holder_id` or `fencing_token` to
force a second holder. Drop the table only in a later contract when no
process uses it.

## Runtime evidence

| Metric | Shows |
|---|---|
| `phoenix_worker_lease_held`, `phoenix_worker_lease_fencing_token`, `phoenix_worker_lease_expiry_timestamp_seconds` | owner (the process reporting 1), token and database-time expiry |
| `phoenix_worker_lease_operation_seconds{operation,outcome}` | acquire, renew and release latency; `outcome="error"` counts renewal failures |
| `phoenix_worker_leadership_changes_total{change}` | `acquired`, `lost`, `expired`, `fenced`, `released` |
| `phoenix_worker_standby_duration_seconds` | time a process waited before it led |
| `phoenix_worker_duplicate_suppressions_total{reason}` | runs skipped in standby, commits fenced after a takeover |
| `phoenix_worker_ready` | readiness of the process |
| `phoenix_worker_drain_seconds` | drain time of a graceful stop |
| `phoenix_worker_job_duration_seconds{job_id,outcome}`, `phoenix_worker_tenant_batch_*` | job starts, completions, failures, retries and backlog (unchanged) |

Expected steady state: exactly one process reports `phoenix_worker_lease_held 1`
and `phoenix_worker_ready 1`; `fenced` stays at zero outside a takeover.

Current holder from the database:

```sql
SELECT lease_name, holder_id, fencing_token, lease_until, lease_until > clock_timestamp() AS running
FROM platform.worker_leases;
```

## Realtime across processes (#5457)

The event transport uses PostgreSQL LISTEN/NOTIFY on the existing application
pool's database, not a new broker or durable outbox. Delivery Platform owns
transport and routing; process roots only construct and close it.

Transport invariants:

1. Producers retain their after-commit hooks. A committed Worker signal reaches
   subscribers of the same school in every API process listening at that time.
2. Each envelope carries the school ID and its original group, account or
   guardian addressing. Receiving a signal never broadens its audience.
   The unauthenticated channel carries allowlisted invalidations only, with
   numeric IDs and fixed reasons, never names, notes, text or links. Receivers
   reconstruct events and ignore unknown content. Tenant-wide producer contracts
   remain identity-free. Invalid envelopes are discarded.
3. Delivery is lossy: disconnects, oversized messages and full client buffers
   can lose signals, never redirect them. No replay or durable delivery is
   promised. The existing poll reconcile remains five minutes while SSE is
   connected, otherwise 60 seconds.
4. Every API has one dedicated listener connection. A standalone Worker only
   publishes. Local browsers receive immediately, even if NOTIFY fails. A
   nonblocking queue holds at most 128 outbound signals and logs overflow. Own
   origin echoes are ignored. Startup waits for a content-free readiness probe
   on the same listener before exposing the API hub; a socket write alone is
   not proof of LISTEN registration. Listeners reconnect; shutdown cancels queued
   publications and closes the listener before the pool.
5. Existing required database configuration is reused and still fails startup
   when missing. No new environment variable, fallback DSN, table or migration
   is needed. All processes must use the same database and a direct PostgreSQL
   connection (not transaction-mode pooling) for LISTEN.

Until the transport implementation is deployed to both API and Worker, a
standalone Worker has no connected browsers: cutting over disables realtime
job signals, and a second API cannot see the first API's local signals.

The relay covers the school-scoped Worker producers and school/group/account/
guardian broadcasts. Legacy unscoped API `BroadcastToAll` calls remain local;
they cannot safely enter the school-scoped channel without an audience contract. That migration is tracked in
[#5492](https://github.com/moto-nrw/project-phoenix/issues/5492).

Content-bearing `notification` events stay local until the tenant-bound reload
path in [#5504](https://github.com/moto-nrw/project-phoenix/issues/5504) is implemented.
They are never accepted from the unauthenticated channel.
