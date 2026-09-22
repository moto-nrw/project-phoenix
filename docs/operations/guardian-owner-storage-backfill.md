# Guardian owner storage Backfill (#2755)

Migration `1.15.413` widens `platform.storage_backfill_checkpoints` with the
guardian-access verdict and runs the first copy of `users.students_guardians`
into `users.student_guardian_relationships` (People Directory),
`users.student_guardian_pickup_permissions` (Care Plan) and
`auth.guardian_student_access` (Identity & Access). `users.students_guardians`
remains the only application authority: no caller switches, no trigger, view,
or dual write exists, and the old rows are never modified. The field mapping is
unchanged from
[Expand #2716](../../backend/database/migrations/001015387_guardian_storage_expand.go).

One value has no column on the old table: the guardian's account binding.
`auth.guardian_student_access.account_id` mirrors
`users.guardian_profiles.account_id` at copy time. A guardian who gains or
loses a portal account afterwards edits the profile, not the link, so the copy
re-reads the binding on every pass and verification reports drift as
`guardian_access_mismatch_count` until the next pass closes it.

## How the copy works

- One checkpoint row per backfill and school holds the pass number, the
  high-water mark (last copied `users.students_guardians.id`), cumulative
  counters, and the last verification result. The table is shared with the
  Staff (#2752) and Student (#2758) backfills; superuser CLI and migrations
  are the only writers, and the application roles hold no grants on it.
- One advisory session lock excludes simultaneous guardian backfill runs,
  reset, and rollback. A competing command fails without changing data. The
  lock is held on its own connection until the run and sequence alignment
  finish; worker and lock sampler use two additional connections.
- Each batch reads up to `--batch-size` rows (default 500) in `id` order after
  the high-water mark inside one `REPEATABLE READ` transaction with a 5 s lock
  timeout, joins the guardian profile for the account binding, upserts the
  relationship by preserved identity (`student_guardian_relationships.id` =
  `students_guardians.id`) and the pickup permission and access row by
  `(tenant_id, relationship_id)`, and advances the checkpoint in the same
  commit. Rows whose content already matches are counted as skipped, so a
  rerun of a completed batch writes nothing. Preserving the link id keeps every
  foreign key that names a link today (parent-student consents, meal
  participation) valid after Cutover.
- Deadlocks (`40P01`), serialization failures (`40001`), and lock timeouts
  (`55P03`) retry the batch up to five times. A unique-index conflict (`23505`)
  rewinds the tenant's pass to zero, persisting the rewind first. The
  relationship table carries three unique keys: one `(student, guardian)` pair
  per school and at most one primary and one payer per child. A conflict means
  some target row still claims a key its own source row no longer holds:
  either the link was deleted and recreated under a new id, or the
  primary/payer flag moved to another guardian of the same child and the batch
  reached the newly flagged, lower id before the demoted, higher one. Rewinds
  have their own budget of five per batch. Before rewinding, the same
  transaction removes this school's relationships whose source row is gone
  *or* now names a different pair or primary/payer flag, and records their
  removal count; the pickup permission and access row follow through the
  cascade, and the restarted pass rebuilds all three from the source. The
  end-of-pass sweep keeps the narrower rule and only removes relationships
  whose source row is gone.
- A school whose batch fails with any other error is reported and skipped for
  this run; the remaining schools still run, and the command exits non-zero.
- Rows whose guardian profile is missing or belongs to another school are
  rejected, not copied. The old table already carries composite tenant foreign
  keys for both the student and the guardian, so this cannot happen on
  validated data; the check keeps a forced or not-yet-validated row from being
  laundered into Identity storage with an empty binding. Rejected rows keep
  the tenant unverified until the source is corrected.
- After each full pass the tenant's orphaned target rows (source physically
  deleted) are removed, then counts, canonical checksums (SHA-256 over ordered
  SHA-256 canonical JSON row digests on both sides), a row-wise mismatch count
  and the guardian-access verdict are stored. All verification queries and
  their checkpoint update share one `REPEATABLE READ` transaction with local
  UTC; `verification_snapshot` identifies that snapshot. A tenant is stable
  when a pass changed nothing and every verification matches. Unstable tenants
  get another pass, up to `--max-passes` (default 5) per run; at the limit the
  completed pass and its high-water mark are kept.
- A separate transaction before that one reads the three targets as
  `phoenix_tenant`, scoped to the school, and fails the pass unless that role
  sees exactly this school's rows and no other school's. The copy runs as
  superuser and bypasses the very policies the rows depend on; after Cutover
  these rows decide which parent may see which child, so a policy that Expand
  created and a later change dropped, disabled or widened must fail the run.
  The count it saw is recorded as `rows_visible_to_tenant`.
- A run on a tenant whose persisted pass is complete starts a new pass so old
  rows changed since the last verification are re-read. The relationship
  sequence is kept above the preserved identities after every run; the
  hand-over from the `users.students_guardians` sequence belongs to Cutover.
- The migration runs the first copy inline so every deployment leaves a
  populated target, at the cost of migration wall time proportional to
  `users.students_guardians`. An error in one school fails the migration after
  the other schools have run; rerunning `migrate` resumes from the checkpoints.

## Commands

Run inside the server container.

```bash
go run . backfill guardian-owner [--batch-size 500] [--max-passes 5]  # resume/re-read until stable; exit 1 while unstable
go run . backfill guardian-owner status                                # print checkpoints only; exit 1 while unstable
go run . backfill guardian-owner reset                                 # truncate targets + checkpoints, restart from zero
```

Interrupting a run (SIGINT, deployment restart) keeps every committed batch and
its checkpoint. The next run resumes at the persisted high-water mark. Reset
and the migration rollback refuse once `users.students_guardians` is no longer
a base table, because after Cutover #2756 the targets hold the authoritative
rows.

## Resolving the reported backlogs

`guardian_access_mismatch_count` and `mismatch_count` are mechanical: another
pass closes them unless the source keeps changing. `rows_rejected` names data
work. Run the query as a superuser against the school's `tenant_id`; it reads
only and mirrors `guardianOwnerEligibleBatch` in
`backend/database/migrations/guardian_owner_backfill.go`.

### `rows_rejected`

A rejected row points at a guardian profile that is missing or belongs to
another school. Repoint `users.students_guardians.guardian_profile_id` at this
school's guardian, or delete the link.

```sql
SELECT sg.id AS link_id, sg.tenant_id AS link_tenant, sg.student_id, sg.guardian_profile_id, g.tenant_id AS guardian_tenant
FROM users.students_guardians sg
LEFT JOIN users.guardian_profiles g ON g.id = sg.guardian_profile_id
WHERE sg.tenant_id = :tenant_id
  AND (g.id IS NULL OR g.tenant_id <> sg.tenant_id)
ORDER BY sg.id;
```

## Runtime evidence

Every `run` and `status` prints one JSON object per school plus
`missing_tenants`, the schools without any checkpoint (after a reset, or
created after the last run). Missing schools count as unstable. Fields:

| Field | Meaning |
| --- | --- |
| `rows_scanned`, `rows_copied`, `rows_skipped`, `rows_rejected`, `rows_removed` | Cumulative source rows read, written (insert or changed update), unchanged, refused (guardian missing or of another school), and orphaned target rows deleted |
| `batches_completed`, `batches_retried`, `deadlocks`, `serialization_failures`, `lock_timeouts` | Cumulative batch outcomes and the SQLSTATE class of each retry |
| `pass`, `high_water_id`, `pass_writes`, `stable`, `stable_at` | Progress of the current pass and the final-delta checkpoint for Cutover |
| `source_count`, `target_count`, `source_checksum`, `target_checksum`, `mismatch_count`, `verified_at`, `verification_snapshot` | Last coherent UTC verification of the mapped columns plus the account binding, and its PostgreSQL snapshot |
| `guardian_access_mismatch_count` | Links whose access row is missing, bound to a different account than the guardian profile names, or carrying different portal permissions |
| `rows_visible_to_tenant` | Target rows the `phoenix_tenant` role saw for this school at the last verification, across all three tables. The copy is superuser and bypasses the policies, so this is the checkpoint's only evidence that the tenant boundary still holds |
| `oldest_unmigrated_at` | Latest edit of the oldest source row or guardian profile still reported by either check; `null` when none |
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

`backfill guardian-owner reset` and the migration's down step truncate only
the three target tables and delete this backfill's checkpoints;
`users.students_guardians` is not touched. The down step drops the shared
checkpoint table, and with it the guardian-specific column, only when no other
backfill has rows in it; the up step is idempotent and recreates both.
Stopping a run retains its checkpoint. The Expand rollback follows after the
targets are empty.

## Exit criterion for Cutover

`backfill guardian-owner status` exits zero only when every school reports
`stable: true`, equal counts and checksums, `mismatch_count: 0`,
`guardian_access_mismatch_count: 0`, and `rows_rejected` explained. A school
whose tenant policies no longer hold never reaches a verdict at all: the run
fails that school's pass before verification. Cutover #2756 applies the final
delta under its own write lock using the same copy statement and re-verifies
before switching callers.

## Staging acceptance record

**Not yet executed for this revision.** Normal deployment stops the application
before running the migration. Record that stopped-application migration path on
staging, then run `backfill guardian-owner` until stable and attach the report.
Separately use an isolated staging copy: while the previous image serves genuine
guardian HTTP reads and link/unlink/role/pickup writes, run the new backfill
command against that copy; repeat the same reads/writes afterward. Use dedicated
synthetic identities and captured mail. Record source parity, all-school
checkpoints, interrupt/resume and reset/rerun on that isolated copy.

| Evidence | Result |
| --- | --- |
| Deployment SHA / previous image digest | Pending |
| Migration wall time (initial copy) | Pending |
| Per-tenant counts and checksums equal, mismatch 0 | Pending |
| `guardian_access_mismatch_count` per tenant after the last pass | Pending |
| `rows_rejected` per tenant and correction | Pending |
| `batch_p95_ms` / `batch_max_ms` / `pool_wait_ms` | Pending |
| `batches_retried` / `deadlocks` / `lock_timeouts` | Pending |
| Observer lock waits and blocked sessions during the copy | Pending |
| Interrupt during a pass and resume (SIGINT the container command) | Pending |
| Previous-image guardian read/write parity during and after the copy | Pending |
| `reset` on an isolated clone followed by a full rerun | Pending |

## Local verification (2026-09-22)

Migration tests cover column mapping including role permissions and the account
binding, interrupt and resume after every batch boundary, rerun of completed
batches, injected deadlock, serialization and lock-timeout failures with retry
accounting and durable terminal counters, graceful cancellation, one failing
school not blocking the others, changed-row re-read and orphan removal,
account binding gained and released after the copy, cross-tenant guardian
rejection with the pass limit keeping the high-water mark, a mid-pass link
recreation unique-conflict restart, a primary/payer swap between two live links
of one child that no orphan sweep can clear, concurrent run/reset/down
exclusion, a measured successful lock wait, one coherent UTC verification
snapshot, two-tenant RLS reads with the checkpoint grant denial, runtime
detection of a dropped and of a widened tenant policy, index validity and
index-backed plans for the batch, checksum, mismatch and access queries with
sequential scans disabled, reset, and the guarded down/up sequence with the
Expand rollback, shared checkpoint retention and the post-Cutover view guard.
The CLI test drives run, status, reset and the unstable exit path against an
isolated clone.

These results do not replace the staging record above.
