# Request-child storage Backfill: operation and evidence

Migration `1.15.383` implements the resumable booking copy and fail-closed
submission-provenance gate for #2713. Applied Presence `1.15.381` and Staff
`1.15.382` are unchanged. All application traffic remains on
`enrollment.request_child_offerings`; no caller switch or dual write exists.

## Binding provenance contract

Unknown original submission history **blocks verification**. Current or earliest
surviving intervals, matching target rows, a previous successful checkpoint and
partial change-request snapshots do not prove the original selected days,
per-offering notes, submission time and tenant-linked identity. Approved
pre-acceptance amendments also lack an established immutable version contract.
A later approved booking interval must never become a guessed original.

No universal full-field immutable authority has been established in this code.
Policy `immutable-submission-v1-unresolved-legacy` therefore resolves **every
nonempty legacy child/offering pair as unresolved**, including singletons.
The reason is `missing-authoritative-submission`. There is no heuristic,
manual checkpoint override or fallback resolver that can turn this into proof.
Introducing trusted authority requires a separately established durable source
and reviewed provenance policy; current target selections are never that source.

| Storage | Behavior |
| --- | --- |
| `care_offering_bookings` | Preserve legacy IDs, manual/automatic days, half-open validity and timestamps |
| `request_child_offering_selections` | Do not insert guessed submissions; copy reconciliation removes untrusted target-only copies |
| Legacy intervals and change history | Read-only and never deleted or rewritten by this job |

Before Cutover these targets have no application writer/consumer. Reset and
copy cleanup affect targets only. Every run and down step refuses if the legacy
source is no longer a base table.

## Copy, recovery and verification

1. Schools and source IDs are processed in ascending order. Every batch commits
   its booking changes and high-water/counters together. Changed and physically
   deleted legacy rows are reconciled in later passes. Booking copy semantics
   remain unchanged; progress can resume after interruption.
2. A dedicated advisory session lock excludes concurrent run, verify-only,
   restart and down commands. Worker and lock sampler have separate
   connections, so three connections are needed while copying.
3. Worker `pg_stat_activity` lock waits are sampled every 10 ms. The measured
   `lock_wait_ms` includes successful waits below the timeout and failed
   attempts, not statement duration. Missing/failed sampling fails the attempt.
   Lock timeout defaults to 5 s; a batch has a 60 s execution bound.
4. Deadlocks, serialization failures and lock timeouts retry up to five times.
   Event counters include the final failed attempt; `batches_retried` counts
   actual retries. Successful batches commit pending telemetry with progress.
   Terminal failures flush telemetry separately with a 5 s bound without
   advancing rows/high-water. SIGKILL or an unavailable database can prevent
   that flush, and persistence errors remain visible.
5. Counts, SHA-256 canonical JSON checksums, row differences, unresolved counts,
   reason counts, policy, snapshot and completion are verified and persisted in
   one UTC `REPEATABLE READ` transaction. Calendar dates remain SQL DATEs.
   `oldest_unmigrated_seconds` includes unresolved-origin backlog.
6. Starting any run invalidates old completion, including checkpoints without
   the current policy. Verify-only refreshes the authoritative acceptance
   evidence and can invalidate old success; it leaves source and target data
   untouched. Cumulative copy metrics are preserved.
7. `stable` means the copy reconcile pass found no more target changes. It is
   **not** submission proof. `complete` additionally requires the current
   policy, snapshot/time, zero unresolved origins, equal counts/checksums and
   zero missing/different/orphan rows. Empty-source schools can verify; schools
   with any legacy pair cannot currently pass.
8. SQL failures retain partial reports and do not prevent later schools from
   running unless cancellation stops the command. An unavailable database-wide
   deadlock sample is reported as unavailable, never as a measured zero.

## Commands and exit behavior

Run through the guarded maintenance connection, not a privileged serving app:

```bash
go run . migrate backfill request-child-storage
go run . migrate backfill request-child-storage --verify-only
go run . migrate backfill request-child-storage --tenant 42 --batch-size 200
go run . migrate backfill request-child-storage --restart
```

The CLI prints visited-school evidence including unresolved counts/reasons and
returns nonzero for errors, interruption or incomplete schools. Ctrl-C/SIGTERM
lets the current bounded batch finish before stopping at a checkpoint boundary.

**Automatic migration still returns an error for any incomplete school.**
Committed batches remain resumable, but no successful migration or full
verification may be claimed. Normal deployment stops the app before migrating.
The shared staging census has five unproven singleton pairs in school 1.
Consequently this migration is expected to block shared rollout until genuine
authority is established; do not replace that error with a warning, restrict
the school set, or call booking parity submission proof.

## Reset and rollback

Before Cutover, `--restart` clears only selected target rows/checkpoints and
replays from zero. The down migration clears both targets and drops its
checkpoint table so Expand `1.15.375` can roll back. Source intervals and
historical amendment records are untouched. Restart/replay must reproduce the
same unresolved decisions, independent of when an older backfill first ran.

## Evidence and acceptance

Record the exact candidate/previous-image digests, PostgreSQL snapshot, workload
and all-school rows. Use an isolated staging copy for previous-image genuine
Enrollment HTTP reads/writes during the backfill and before/after parity.
Include interrupt/resume, terminal retry evidence, successful sub-timeout lock
wait, reset/replay and source preservation. Capture mail on the isolated copy.
Local tests do not replace this evidence.

```sql
SELECT tenant_id, complete, stable, high_water_mark,
       source_bookings, target_bookings, source_selections, target_selections,
       source_bookings_checksum, target_bookings_checksum,
       mismatch_count, orphan_count, unresolved_origins, unresolved_reasons,
       provenance_policy, verification_snapshot, verified_at,
       oldest_unmigrated_seconds, batches_retried, deadlocks,
       serialization_failures, lock_timeouts, lock_wait_ms
FROM enrollment.request_child_storage_backfill_checkpoints
ORDER BY tenant_id;
```

The bootstrap school exists before this migration even on a fresh database.
Its empty-source checkpoint is real migration evidence, so the checkpoint table
is not seed-exempt. Existing target-table exemptions remain: ordinary seeding
runs after migration and must not invent submissions or dual writes.

Regression coverage includes singleton/deleted-origin history, matching guessed
targets plus old success, partial/foreign approved-change snapshots, later
approved intervals, snapshot interleaving, verify-only invalidation, empty
schools, automatic migration failure, replay/reset, booking timestamps and
bounds, RLS, retry exhaustion, writer exclusion and measured lock waits.
No trusted full-evidence test is fabricated without an established authority.
Final-head local/CI and isolated runtime results belong in the PR evidence.
