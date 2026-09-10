# Staff owner storage Backfill (#2752)

Migration `1.15.382` creates `platform.storage_backfill_checkpoints` and runs
the first copy of `users.staff` into `users.staff_school_memberships` and
`users.staff_employment_profiles`. `users.staff` remains the only application
authority: no caller switches, no trigger, view, or dual write exists, and the
old rows are never modified. The field mapping is unchanged from
[Expand #2715](staff-owner-storage-expand.md).

## How the copy works

- One checkpoint row per backfill and school holds the pass number, the
  high-water mark (last copied `users.staff.id`), cumulative counters, and the
  last verification result. Superuser CLI and migrations are the only writers;
  the application roles hold no grants on the table.
- One advisory session lock excludes simultaneous Staff backfill runs, reset,
  and rollback. A competing command fails without changing data. The lock is
  held on its own connection until the run and sequence alignment finish;
  worker and lock sampler use two additional connections.
- Each batch reads up to `--batch-size` rows (default 500) in `id` order after
  the high-water mark inside one `REPEATABLE READ` transaction with a 5 s lock
  timeout, upserts memberships and profiles by preserved identity
  (`membership_id = staff.id`), and advances the checkpoint in the same commit.
  Rows whose content already matches are counted as skipped, so a rerun of a
  completed batch writes nothing.
- Deadlocks (`40P01`), serialization failures (`40001`), and lock timeouts
  (`55P03`) retry the batch up to five times. A unique-index conflict
  (`23505`) rewinds the tenant's pass to zero, persisting the rewind first: it
  happens when a person is offboarded and rejoins between two batches of the
  same pass. Rewinds have their own budget of five per batch.
- A school whose batch fails with any other error is reported and skipped for
  this run; the remaining schools still run, and the command exits non-zero.
- Rows that reference a work-time model of another school are rejected, not
  copied. The superuser connection bypasses the tenant policy that rejects
  such assignments for the application role, so the backfill must surface the
  inconsistency instead of laundering it. Rejected rows keep the tenant
  unverified until the source is corrected.
- After each full pass the tenant's orphaned target rows (source physically
  deleted) are removed, then counts, canonical checksums
  (SHA-256 over ordered SHA-256 canonical JSON row digests on both sides) and
  a row-wise mismatch count are stored. All verification queries and their
  checkpoint update share one `REPEATABLE READ` transaction with local UTC;
  `verification_snapshot` identifies that snapshot. A tenant is stable when
  a pass changed nothing and verification matches. Unstable tenants get another
  pass, up to `--max-passes` (default 5) per run; at the limit the completed
  pass and its high-water mark are kept.
- A run on a tenant whose persisted pass is complete starts a new pass so old
  rows changed since the last verification are re-read. The membership
  sequence is kept above the preserved identities after every run; the
  hand-over from the `users.staff` sequence belongs to Cutover.
- The migration runs the first copy inline so every deployment leaves a
  populated target, at the cost of migration wall time proportional to
  `users.staff`. An error in one school fails the migration after the other
  schools have run; rerunning `migrate` resumes from the checkpoints.

## Commands

Run inside the server container.

```bash
go run . backfill staff-owner [--batch-size 500] [--max-passes 5]  # resume/re-read until stable; exit 1 while unstable
go run . backfill staff-owner status                                 # print checkpoints only; exit 1 while unstable
go run . backfill staff-owner reset                                  # truncate targets + checkpoints, restart from zero
```

Interrupting a run (SIGINT, deployment restart) keeps every committed batch and
its checkpoint. The next run resumes at the persisted high-water mark. Reset
and the migration rollback refuse once `users.staff` is no longer a base
table, because after Cutover #2753 the targets hold the authoritative rows.

## Runtime evidence

Every `run` and `status` prints one JSON object per school plus
`missing_tenants`, the schools without any checkpoint (after a reset, or
created after the last run). Missing schools count as unstable. Fields:

| Field | Meaning |
| --- | --- |
| `rows_scanned`, `rows_copied`, `rows_skipped`, `rows_rejected`, `rows_removed` | Cumulative source rows read, written (insert or changed update), unchanged, refused (foreign work-time model), and orphaned target rows deleted |
| `batches_completed`, `batches_retried`, `deadlocks`, `serialization_failures`, `lock_timeouts` | Cumulative batch outcomes and the SQLSTATE class of each retry |
| `pass`, `high_water_id`, `pass_writes`, `stable`, `stable_at` | Progress of the current pass and the final-delta checkpoint for Cutover |
| `source_count`, `target_count`, `source_checksum`, `target_checksum`, `mismatch_count`, `verified_at`, `verification_snapshot` | Last coherent UTC verification and its PostgreSQL snapshot |
| `oldest_unmigrated_at` | `updated_at` of the oldest source row still differing from the targets at the last verification; `null` when none |
| `batch_p95_ms`, `batch_max_ms` | p95 of the last run's batch transactions and the maximum across runs |
| `pool_wait_ms` | Pool-wide connection wait observed while the tenant's batches ran; the process pool is shared, so concurrent users of the same pool are included |
| `lock_wait_ms` | Cumulative sampled worker lock-wait time, including failed attempts; 10 ms sampling resolution |

The worker's `pg_stat_activity.wait_event_type` is sampled every 10 ms through
an independent connection. A successful wait below the 5 s `lock_timeout`
therefore still contributes to `lock_wait_ms`; sampling is approximate, not
statement duration. Missing or failed sampling fails the batch, not silently
recording zero. Batch statements also have a 60 s timeout. External observer
SQL remains useful for corroborating contention.

Deadlock, serialization-failure and timeout counts include the final failed
attempt; `batches_retried` counts only retries actually started. A successful
batch commits accumulated telemetry with its data checkpoint. Terminal errors
flush pending telemetry separately with a 5 s bound, without advancing row or
pass progress, including on graceful cancellation. SIGKILL or an unavailable
database can prevent that flush; the command reports persistence errors.
Partial failures still print the available per-tenant JSON report and return
the original error with a non-zero exit status.

## Rollback

`backfill staff-owner reset` and the migration's down step truncate only the
two target tables and delete this backfill's checkpoints; `users.staff` is not
touched. The down step drops the shared checkpoint table only when no other
backfill has rows in it. Stopping a run retains its checkpoint. The Expand
rollback follows after the targets are empty.

## Exit criterion for Cutover

`backfill staff-owner status` exits zero only when every school reports
`stable: true`, equal counts and checksums, `mismatch_count: 0`, and
`rows_rejected` explained. Cutover #2753 applies the final delta under its
own write lock using the same runner and re-verifies before switching callers.

## Staging acceptance record

**Not yet executed for this revision.** Normal deployment stops the application
before running the migration. Record that stopped-application migration path
on staging, then run `backfill staff-owner` until stable and attach the report.
Separately use an isolated staging copy: while the previous image serves
genuine Staff HTTP reads and create/edit/offboard/rejoin writes, run the new
backfill command against that copy; repeat the same reads/writes afterward.
Use dedicated synthetic identities and captured mail. Record source parity,
all-school checkpoints, interrupt/resume and reset/rerun on that isolated copy.

| Evidence | Result |
| --- | --- |
| Deployment SHA / previous image digest | Pending |
| Migration wall time (initial copy) | Pending |
| Per-tenant counts and checksums equal, mismatch 0 | Pending |
| `rows_rejected` per tenant and correction | Pending |
| `batch_p95_ms` / `batch_max_ms` / `pool_wait_ms` | Pending |
| `batches_retried` / `deadlocks` / `lock_timeouts` | Pending |
| Observer lock waits and blocked sessions during the copy | Pending |
| Interrupt during a pass and resume (SIGINT the container command) | Pending |
| Previous-image staff read/write parity during and after the copy | Pending |
| `reset` on an isolated clone followed by a full rerun | Pending |

## Local verification (2026-09-10)

The integration adds regression cases for concurrent source edits during UTC
verification, concurrent run/reset/down exclusion, measured successful lock
waits, durable terminal failure counters, and partial CLI report/output errors.
Focused test results and final integration checks are recorded in the PR
evidence; the historical checks below describe the original pre-integration
head and do not certify the revised head.

Migration tests cover interrupt and resume after every batch boundary, rerun
of completed batches, injected deadlock, serialization and lock-timeout
failures with retry accounting, one failing school not blocking the others,
changed-row re-read and orphan removal, foreign work-time model rejection with
the pass limit keeping the high-water mark, mid-pass rejoin conflict with a
persisted rewind, two-tenant RLS reads, index validity and index-backed plans
for the batch, checksum and mismatch queries with sequential scans disabled,
reset, and the guarded down/up sequence with the Expand rollback and shared
checkpoint retention. The CLI test drives run, status, reset and the unstable
exit path against an isolated clone.

- `scripts/test-changed.sh origin/development` (without `--fast`, pinned
  toolchain, `CGO_ENABLED=0`): `cmd`, `database/migrations`,
  `internal/architecture`, `seed/api`, `test` passed.
- `scripts/backend-architecture.sh check`: no new keys, composition
  796 → 796, 1,647 existing legacy violations remain; the new checkpoint
  table is owned by `migrations` in `policy.json`.
- `scripts/backend-architecture.sh validate-ticket --ticket backend/architecture/staff-owner-backfill-2752.json`: passed.
- `golangci-lint run` on `cmd`, `database/migrations`, `seed/api`: zero issues.

These results do not replace the staging record above.
