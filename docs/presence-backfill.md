# Timetable → Presence backfill (#2761)

This is the backfill between Expand #2718 and Cutover #2762. It does not
switch application callers, narrow the old status constraint, or add dual
writes. `schedule.activity_instances` and `schedule.instance_students` remain
authoritative. Planned occurrences and assignments stay in those tables.

## Run and resume

Apply migrations first. Use the existing migration CLI connection and database
configuration; no new environment variables are required. The runner needs two
available pool connections: one worker and one lock-wait sampler.

Commands below run against the local Compose server configuration:

```bash
docker compose run --rm server go run . migrate
docker compose run --rm server go run . migrate presence-backfill run --all-tenants --batch-size 500
docker compose run --rm server go run . migrate presence-backfill status --all-tenants
```

For one school, replace `--all-tenants` with `--tenant-id SCHOOL_ID`.
`--max-batches 1` deliberately stops after one committed batch with a nonzero
exit unless that batch completes verification. Rerun the same command without
the limit to resume. Killing the process rolls back its current transaction;
earlier batches and their checkpoints remain committed. A lost connection after
commit is resolved by reading the checkpoint, not assuming the batch failed.

The CLI emits one JSON object per batch with `checkpoint`. It includes `metrics`
on status, restart, and completed-pass output. Live source-age inspection scans
the whole tenant, so ordinary copy batches do not repeat it. Use `status` from
another process to inspect live metrics during a long run.
Database/CLI startup diagnostics may precede these records. A failed output
write can occur after a committed batch; rerunning remains safe.

## Copy and verification contract

1. Schools are visited in ID order. Within each school, sessions and attendance
   are scanned separately by `(tenant_id, id)` keysets. Each transaction holds
   that school's checkpoint row lock and uses repeatable-read isolation.
2. Old `active`/`completed` occurrences become Presence sessions with the same
   status and all execution fields. Old `planned`/`cancelled` occurrences create
   no session. Every old participant produces a Presence attendance row,
   including expected attendance attached to a planning-only occurrence.
3. Upserts use tenant/source IDs, not target surrogate IDs. Identical payloads
   are skipped. Source deletions cascade through the Expand FKs; a return to
   planning-only state removes the target session on the next pass. Changed
   active-group bindings are released in targets before copying replacements.
4. Verification compares every moved field, including NULLs, timestamps,
   provenance IDs, and JSON completion snapshots, in one repeatable-read
   snapshot. Counts and checksums are per tenant. Canonical checksums are
   `hex(SHA256(concat(SHA256(UTF8(JSONB field-array text)) ORDER BY source_id)))`,
   concatenating the binary row digests, with UTC timestamp serialization and
   empty bytes for no rows. Exact field
   comparison also counts mismatches, independently of checksum collisions.
5. A mismatch restarts both ID cursors. Full rescans catch changes behind the
   high-water mark even if an old writer did not update `updated_at`. A rerun of
   an already-complete checkpoint also starts a new pass. No update-time
   watermark, trigger, compatibility view, or application dual write is used.

Planning evidence contains the count/checksum of the retained occurrence and
assignment fields. Its occurrence-status projection maps `active`/`completed`
to `planned` without writing that change into the old table. There is no second
copy of planned rows to compare: they already occupy their target-owner tables.

## Reading evidence

| Field | Meaning |
| --- | --- |
| `rows_scanned/copied/skipped/removed` | Cumulative source rows examined, inserted/changed target payloads, no-copy source rows, and removed planning-only target sessions. Rescans count again. |
| `session_high_water`, `attendance_high_water`, `phase`, `pass` | Last committed source IDs and current pass. Phase-only transitions are also committed batches. |
| `sessions`, `attendance`, `mismatches`, `orphans` | Results of the last completed verification query, not a promise that old writers have stopped. |
| `oldest_unmigrated_seconds` in `metrics` | Live age, from source `created_at`, of the oldest missing or unequal target payload. Zero when none exists. This read may observe writes later than the checkpoint snapshot. |
| `batches`, `retries`, `deadlocks` | Durable committed-batch counts. Retries cover SQLSTATE `40001`, `40P01`, and `55P03`, at most five retries with bounded backoff. Terminal failures report retry/deadlock counts in the error and retain the previous checkpoint. |
| `batch_p95_ms`, `batch_max_ms` | Exact aggregates over stored batch durations, including successful retries/backoff, ending just before evidence insertion/commit. |
| `pool_wait_ms` | Time acquiring this runner's worker and monitor connections, including retry attempts; not shared pool-wide counters. |
| `lock_wait_ms`, `lock_sample_interval_ms` | Worker-PID `pg_stat_activity` lock-wait samples at 10 ms, including retry attempts. Sub-interval waits may be missed. Sampling failure fails the batch; SQL duration is not substituted for lock wait. |
| `complete`, `verified_at`, `final_delta_snapshot` | Zero-mismatch snapshot evidence for a completed pass. Snapshot text is a transaction visibility boundary, not a WAL/CDC cursor or a durable exported snapshot. |

Checkpoints and batch timings are stored in
`active.presence_backfill_checkpoints` and `active.presence_backfill_batches`.
Both have forced tenant RLS. Tenant sessions can read their own evidence but
cannot write it. The migration CLI uses its existing administrative connection.
These one-time migration ledgers are not demo-seeder data.

## Final delta and rollback

`complete: true` proves equality at the recorded verification snapshot, not
future equality. Before #2762 switches callers, that cutover must fence old
writers, rerun the backfill for every school, and require complete checkpoints,
equal counts/checksums, and zero mismatches/orphans. This command does not fence
writers or perform the switch.

Before cutover only, the following clears the selected school's Presence
targets and batch evidence and resets its existing checkpoint to ID zero:

```bash
docker compose run --rm server go run . migrate presence-backfill restart --tenant-id SCHOOL_ID
```

It does not delete or mutate old authoritative rows and cannot restart all
schools in one invocation. Stop competing runners before using it. Ordinary
stopping needs no reset. Schema rollback refuses to discard any checkpoint;
Expand rollback still refuses populated targets. After cutover, remove this
backfill/restart entry point as part of the contract cleanup, never use it to
erase authoritative Presence data.

## Verification

The PostgreSQL tests cover interruption/resume, completed-pass replay, changes
behind cursors, field mapping, planning-only rows, injected serialization and
deadlock failures, transaction rollback, measured lock waits, tenant isolation,
target-only restart, migration up/down, source deletion, and keyset query plans.
CLI tests cover selection validation, JSON progress, and batch-limit exit.

```bash
CGO_ENABLED=0 scripts/run-go-toolchain.sh go -C backend test ./database/migrations ./cmd -run '^TestPresenceBackfill' -count=1 -parallel 8
scripts/backend-architecture.sh check
```

Owner/capability: migration adapter for Timetable → Student Presence backfill.
No existing data owner, target import permission, legacy baseline entry, or
Factory/API field/setter changes. New ledger objects belong to Student Presence.
