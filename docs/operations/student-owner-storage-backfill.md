# Student owner storage Backfill (#2758)

Migration `1.15.394` widens `platform.storage_backfill_checkpoints` with two
student-specific verdicts and runs the first copy of `users.students` into
`users.student_profiles`, `users.student_school_memberships` and
`users.student_care_profiles`. `users.students` remains the only application
authority: no caller switches, no trigger, view, or dual write exists, and the
old rows are never modified. The field mapping is unchanged from
[Expand #2717](../../backend/database/migrations/001015393_student_owner_storage_expand.go).

Two groups of old columns have no target, and the backfill reports rather than
copies them:

- `guardian_name`, `guardian_contact`, `guardian_email` and `guardian_phone`
  belong to `users.guardian_profiles` / `users.guardian_phone_numbers`. A value
  is reconciled when a guardian linked to that child carries it; the rest are
  counted in `guardian_mismatch_count`.
- `sick`, `sick_since`, `excused` and `excused_since` are superseded by
  `active.student_status_days`. A raised flag is equivalent when an uncleared
  status day for today (Europe/Berlin) carries the matching status
  (`class_trip` counts as excused); the rest are counted in
  `care_state_mismatch_count`.

Both are Cutover blockers, so a school reports `stable: true` only once every
legacy value has an owner and every raised flag has its day.

## How the copy works

- One checkpoint row per backfill and school holds the pass number, the
  high-water mark (last copied `users.students.id`), cumulative counters, and
  the last verification result. The table is shared with the Staff backfill
  (#2752); superuser CLI and migrations are the only writers, and the
  application roles hold no grants on it.
- One advisory session lock excludes simultaneous Student backfill runs, reset,
  and rollback. A competing command fails without changing data. The lock is
  held on its own connection until the run and sequence alignment finish;
  worker and lock sampler use two additional connections.
- Each batch reads up to `--batch-size` rows (default 500) in `id` order after
  the high-water mark inside one `REPEATABLE READ` transaction with a 5 s lock
  timeout, upserts the profile, the membership and the care profile by
  preserved identity (`student_profiles.id` = `student_school_memberships.id` =
  `student_care_profiles.membership_id` = `students.id`), and advances the
  checkpoint in the same commit. Rows whose content already matches are counted
  as skipped, so a rerun of a completed batch writes nothing.
- The membership's `deleted_at` is written as NULL. `users.students` has no soft
  deletion, so while it stays authoritative no enrollment it still holds is
  retired, and the verification reports a stray retirement as a mismatch.
- Deadlocks (`40P01`), serialization failures (`40001`), and lock timeouts
  (`55P03`) retry the batch up to five times. A unique-index conflict (`23505`)
  rewinds the tenant's pass to zero, persisting the rewind first: it happens
  when a child's enrollment is deleted and recreated between two batches of the
  same pass, because the new student ID meets the profile of the deleted one on
  the per-person unique key. Rewinds have their own budget of five per batch.
  Before rewinding, the same transaction removes tenant profiles whose source
  row was physically deleted and records their removal count; the membership and
  care profile follow through the cascade.
- A school whose batch fails with any other error is reported and skipped for
  this run; the remaining schools still run, and the command exits non-zero.
- Rows whose person belongs to another school are rejected, not copied.
  `users.students` still carries the single-column person foreign key while
  `users.student_profiles` references `users.persons(tenant_id, id)`, and the
  superuser connection would otherwise launder the inconsistency into People
  storage. Rejected rows keep the tenant unverified until the source is
  corrected. `group_id` needs no such check: the old table already has the
  composite `education.groups(tenant_id, id)` foreign key.
- After each full pass the tenant's orphaned target rows (source physically
  deleted) are removed, then counts, canonical checksums (SHA-256 over ordered
  SHA-256 canonical JSON row digests on both sides), a row-wise mismatch count,
  the guardian reconciliation and the care-state equivalence are stored. All
  verification queries and their checkpoint update share one `REPEATABLE READ`
  transaction with local UTC; `verification_snapshot` identifies that snapshot.
  A tenant is stable when a pass changed nothing and every verification matches.
  Unstable tenants get another pass, up to `--max-passes` (default 5) per run;
  at the limit the completed pass and its high-water mark are kept.
- A run on a tenant whose persisted pass is complete starts a new pass so old
  rows changed since the last verification are re-read. Both target sequences
  are kept above the preserved identities after every run; the hand-over from
  the `users.students` sequence belongs to Cutover.
- The migration runs the first copy inline so every deployment leaves a
  populated target, at the cost of migration wall time proportional to
  `users.students`. An error in one school fails the migration after the other
  schools have run; rerunning `migrate` resumes from the checkpoints.

## Commands

Run inside the server container.

```bash
go run . backfill student-owner [--batch-size 500] [--max-passes 5]  # resume/re-read until stable; exit 1 while unstable
go run . backfill student-owner status                                # print checkpoints only; exit 1 while unstable
go run . backfill student-owner reset                                 # truncate targets + checkpoints, restart from zero
```

Interrupting a run (SIGINT, deployment restart) keeps every committed batch and
its checkpoint. The next run resumes at the persisted high-water mark. Reset
and the migration rollback refuse once `users.students` is no longer a base
table, because after Cutover #2759 the targets hold the authoritative rows.

## Resolving the two reported backlogs

Neither counter is cleared by rerunning the backfill; both name data work.

- **`guardian_mismatch_count`** — list the affected children and decide per
  value whether the contact belongs on an existing linked guardian, needs a new
  `users.guardian_profiles` row and link, or is stale and may be cleared on
  `users.students`. Normalization stops at case and whitespace for names and
  e-mails and at digits for phone numbers, so the same number written `+49 170
  1234567` on the guardian and `0170 1234567` on the child is reported, not
  assumed equal. Correcting the guardian row or the legacy column clears it.
- **`care_state_mismatch_count`** — a raised flag without an open status day is
  an absence the split would drop. Run the end-of-day status archiver, which
  writes the day and clears the flag, or clear the flag directly when the child
  is no longer absent.

## Runtime evidence

Every `run` and `status` prints one JSON object per school plus
`missing_tenants`, the schools without any checkpoint (after a reset, or
created after the last run). Missing schools count as unstable. Fields:

| Field | Meaning |
| --- | --- |
| `rows_scanned`, `rows_copied`, `rows_skipped`, `rows_rejected`, `rows_removed` | Cumulative source rows read, written (insert or changed update), unchanged, refused (cross-tenant person), and orphaned target rows deleted |
| `batches_completed`, `batches_retried`, `deadlocks`, `serialization_failures`, `lock_timeouts` | Cumulative batch outcomes and the SQLSTATE class of each retry |
| `pass`, `high_water_id`, `pass_writes`, `stable`, `stable_at` | Progress of the current pass and the final-delta checkpoint for Cutover |
| `source_count`, `target_count`, `source_checksum`, `target_checksum`, `mismatch_count`, `verified_at`, `verification_snapshot` | Last coherent UTC verification of the mapped columns and its PostgreSQL snapshot |
| `guardian_mismatch_count` | Students whose legacy guardian name, contact, e-mail or phone has no counterpart among the guardians linked to that child |
| `care_state_mismatch_count` | Students whose legacy sick/excused flag has no equivalent uncleared status day for today |
| `oldest_unmigrated_at` | `updated_at` of the oldest source row still reported by any of the three checks; `null` when none |
| `batch_p95_ms`, `batch_max_ms` | p95 of the last run's batch transactions and the maximum across runs |
| `pool_wait_ms` | Pool-wide connection wait observed while the tenant's batches ran; the process pool is shared, so concurrent users of the same pool are included |
| `lock_wait_ms` | Cumulative sampled worker lock-wait time, including failed attempts; 10 ms sampling resolution |

The worker's `pg_stat_activity.wait_event_type` is sampled every 10 ms through
an independent connection. A successful wait below the 5 s `lock_timeout`
therefore still contributes to `lock_wait_ms`; sampling is approximate, not
statement duration. Missing or failed sampling fails the batch, not silently
recording zero. Batch statements also have a 60 s timeout.

Deadlock, serialization-failure and timeout counts include the final failed
attempt; `batches_retried` counts only retries actually started. A successful
batch commits accumulated telemetry with its data checkpoint. Terminal errors
flush pending telemetry separately with a 5 s bound, without advancing row or
pass progress, including on graceful cancellation. SIGKILL or an unavailable
database can prevent that flush; the command reports persistence errors.
Partial failures still print the available per-tenant JSON report and return
the original error with a non-zero exit status.

## Rollback

`backfill student-owner reset` and the migration's down step truncate only the
three target tables and delete this backfill's checkpoints; `users.students` is
not touched. The down step drops the shared checkpoint table, and with it the
two student-specific columns, only when no other backfill has rows in it — the
up step is idempotent and recreates both. Stopping a run retains its
checkpoint. The Expand rollback follows after the targets are empty.

## Exit criterion for Cutover

`backfill student-owner status` exits zero only when every school reports
`stable: true`, equal counts and checksums, `mismatch_count: 0`,
`guardian_mismatch_count: 0`, `care_state_mismatch_count: 0`, and
`rows_rejected` explained. Cutover #2759 applies the final delta under its own
write lock using the same runner and re-verifies before switching callers.

## Staging acceptance record

**Not yet executed for this revision.** Normal deployment stops the application
before running the migration. Record that stopped-application migration path on
staging, then run `backfill student-owner` until stable and attach the report.
Separately use an isolated staging copy: while the previous image serves genuine
student HTTP reads and create/edit/enroll/graduate writes, run the new backfill
command against that copy; repeat the same reads/writes afterward. Use dedicated
synthetic identities and captured mail. Record source parity, all-school
checkpoints, interrupt/resume and reset/rerun on that isolated copy.

| Evidence | Result |
| --- | --- |
| Deployment SHA / previous image digest | Pending |
| Migration wall time (initial copy) | Pending |
| Per-tenant counts and checksums equal, mismatch 0 | Pending |
| `guardian_mismatch_count` per tenant and correction | Pending |
| `care_state_mismatch_count` per tenant and correction | Pending |
| `rows_rejected` per tenant and correction | Pending |
| `batch_p95_ms` / `batch_max_ms` / `pool_wait_ms` | Pending |
| `batches_retried` / `deadlocks` / `lock_timeouts` | Pending |
| Observer lock waits and blocked sessions during the copy | Pending |
| Interrupt during a pass and resume (SIGINT the container command) | Pending |
| Previous-image student read/write parity during and after the copy | Pending |
| `reset` on an isolated clone followed by a full rerun | Pending |

## Local verification (2026-09-18)

Migration tests cover interrupt and resume after every batch boundary, rerun of
completed batches, injected deadlock, serialization and lock-timeout failures
with retry accounting and durable terminal counters, graceful cancellation, one
failing school not blocking the others, changed-row re-read and orphan removal,
cross-tenant person rejection with the pass limit keeping the high-water mark,
a mid-pass rejoin unique-conflict restart, unreconciled guardian values and
stale absence flags with their corrections, concurrent run/reset/down exclusion,
a measured successful lock wait, one coherent UTC verification snapshot,
two-tenant RLS reads with the checkpoint grant denial, index validity and
index-backed plans for the batch, checksum and mismatch queries with sequential
scans disabled, reset, and the guarded down/up sequence with the Expand
rollback, shared checkpoint retention and the post-Cutover view guard. The CLI
test drives run, status, reset and the unstable exit path against an isolated
clone.

These results do not replace the staging record above.
