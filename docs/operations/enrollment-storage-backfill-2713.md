# Request-child storage Backfill: operation and evidence

Migration `1.15.383` copies the legacy request-child rows into the Expand
targets for #2713. Applied Presence `1.15.381` and Staff `1.15.382` are
unchanged. All application traffic remains on
`enrollment.request_child_offerings`; no caller switch or dual write exists.

## Contract: nobody plans with other days

The copy must not change which days a child is planned for. With
`enrollment.bookings_authoritative` on, arrival and pickup planning read the
`selected_days` of the effective legacy interval. The targets therefore follow
the application, not a reconstructed history:

| Storage | Content |
| --- | --- |
| `care_offering_bookings` | One row per legacy row with the legacy ID, validity and timestamps. `manual_selected_days` and `automatic_selected_days` copy unchanged. Rows written before that breakdown carry their days only in `selected_days` (both parts NULL, JSON null or `[]`); those days are manual by construction and copy into `manual_selected_days`. |
| `request_child_offering_selections` | The Anmeldung of each child/offering pair: its earliest surviving interval without automatic additions. Later intervals are approved changes. |
| Legacy intervals and change history | Read-only; never deleted or rewritten by this job. |

Where a change effective from the original start deleted the original
interval, the earliest surviving interval stands in for the Anmeldung. No
planned day depends on the selection. Policy name:
`earliest-surviving-interval-v1`.

Verification requires, per school:

- equal booking and selection counts and SHA-256 canonical checksums,
- zero missing, different or orphaned rows,
- zero day differences: `manual ∪ automatic` of every booking equals the
  legacy `selected_days` as a set.

A day difference means the legacy row itself disagrees with its breakdown.
Recopying cannot repair it, so it blocks completion and the automatic
migration. The checkpoint records it separately as `day_differences`.

## Copy and recovery

1. Schools and source IDs run in ascending order. Every batch commits its
   booking and selection changes together with the high-water mark and
   counters. Changed and deleted legacy rows are reconciled in later passes.
2. A dedicated advisory session lock excludes concurrent run, verify-only,
   restart and down commands.
3. Deadlocks, serialization failures and lock timeouts retry up to five
   times per batch; lock timeout defaults to 5 s, a batch is bounded to 60 s.
4. Verification runs in one UTC `REPEATABLE READ` transaction and is persisted
   in `enrollment.request_child_storage_backfill_checkpoints`.
5. Starting any run invalidates old completion. Verify-only refreshes the
   evidence and leaves source and target data untouched.

## Commands and exit behavior

Run through the guarded maintenance connection, not a privileged serving app:

```bash
go run . migrate backfill request-child-storage
go run . migrate backfill request-child-storage --verify-only
go run . migrate backfill request-child-storage --tenant 42 --batch-size 200
go run . migrate backfill request-child-storage --restart
```

The CLI prints per-school evidence including day differences and returns
nonzero for errors, interruption or incomplete schools. Ctrl-C/SIGTERM lets
the current batch finish before stopping at a checkpoint boundary. The
automatic migration fails for any incomplete school; committed batches stay
resumable. Normal deployment stops the app before migrating.

## Reset and rollback

Before Cutover, `--restart` clears only selected target rows and checkpoints
and replays from zero; a replay reaches the same result. The down migration
clears both targets and drops the checkpoint table so Expand `1.15.375` can
roll back. Legacy rows are untouched.

## Acceptance

After the staging deploy, read the checkpoints:

```sql
SELECT tenant_id, complete, source_bookings, target_bookings,
       source_selections, target_selections, mismatch_count, orphan_count,
       day_differences, verified_at
FROM enrollment.request_child_storage_backfill_checkpoints
ORDER BY tenant_id;
```

Every school must report `complete = true` and `mismatch_count = 0`.

The bootstrap school exists before this migration even on a fresh database.
Its empty-source checkpoint is real migration evidence, so the checkpoint table
is not seed-exempt. Existing target-table exemptions remain: ordinary seeding
runs after migration and must not dual-write.
