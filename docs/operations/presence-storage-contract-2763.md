# Presence storage Contract (#2763)

Migration `1.15.432` removes the rollback mirror that Cutover `1.15.415`
([#2762](presence-storage-cutover-2762.md)) kept for previous-image rollback.
It follows the [student](student-owner-storage-contract.md),
[request-child](enrollment-storage-contract-2719.md),
[staff](staff-owner-storage-contract.md) and
[guardian](guardian-owner-storage-contract.md) Contracts: one backend and one
frontend per environment, both stopped before migration, no rolling
mixed-version rollout and no observation window.

After the Contract the storage is:

- `schedule.activity_instances`: the plan of a block (date, template, period,
  title, description, times, room, required staff, list kind, spontaneous
  marker, understaffed acknowledgement, cancel reason, notes, creator,
  idempotency key). Its `status` accepts only `planned` and `cancelled`.
- `active.activity_sessions`: the execution of a block, status `active` or
  `completed`, live group, starter, start, completion, completer, reopen
  window and completion snapshot.
- `schedule.instance_students`: the planned participants (block, child, room).
- `active.activity_session_attendance`: the attendance of a participant. A
  participant without a row is expected.

## Before the release

The issue names three conditions. Check them before the release that carries
`1.15.432`:

1. The cutover shipped in an earlier production release: `1.15.415` is
   recorded in `public.bun_migrations` and the owner tables are in use. The
   migration preflight refuses a release that would apply both migrations at
   once.
2. Nobody uses the old path. Run the
   [observation SQL](presence-storage-cutover-2762.sql) on the production
   database while the current image serves requests: the
   `compatibility_writes` delta between two samples must be zero and
   `attendance_drift` must be zero for every school. The caller inventory
   (`TestPresenceStorageCallerInventory`) is green in CI. The authoritative
   drift gate is the migration data preflight (release step 1). Its
   execution check is narrower than `execution_drift` in the observation
   SQL, which also counts execution values left on a planned or cancelled
   block without a session; those are not drift for the Contract (see
   below). A nonzero `execution_drift` alone does not block the release when
   the preflight passes.
3. A restorable release backup is verified, not merely present. The release
   sequence below takes and verifies it.

The migration does not read the counter itself. As for the earlier Contracts,
historical hits are diagnostic, not a cleanup gate: the previous image is
stopped before the migration runs, and a failure after the backup restores the
complete release.

## Release sequence

1. Pull the new images and run the read-only migration data preflight while
   the old release still serves requests. An integrity failure aborts before
   downtime; preflight neither removes objects nor changes rows or the counter.
2. Stop both application services, including the backend's embedded jobs.
3. Create and verify the complete release backup.
4. Run the migration. It rechecks integrity under `ACCESS EXCLUSIVE` locks on
   the two plan tables and the two owner tables (5 s lock timeout, 60 s
   statement timeout), narrows the planning status and removes the
   compatibility objects in one transaction. Row counts and SHA-256
   fingerprints per school of both owner tables and of the retained plan
   columns must be unchanged before it commits.
5. Start both new images and verify health: start a block on a device, check
   children in and out, end the block, reopen it, and open the class-day
   lists. A failure after the backup restores the complete saved release.

After the Contract an old image alone is not a rollback: restore the
pre-Contract backup and the prior images together. Down is refused.

## Preflight refusals

- The completed cutover schema is required: the counter, the four mirror and
  routing functions, their four triggers and every mirrored column present.
  In a deployment preflight the cutover must not be pending in the same
  release.
- A stored function other than the removed ones that names a plan table and
  a mirrored column (`status` and `note` excepted, because retained columns
  share them), and a view that depends on a mirrored column. `DROP ...
  RESTRICT` also refuses any unknown dependency.
- Unvalidated foreign keys on or into the four tables, and RLS that is not
  enabled and forced on any of them.
- Execution mirror drift: a block the mirror shows as active or completed
  without a session, or a session whose mirrored columns differ.
- Attendance mirror drift: a participant whose mirrored columns differ from
  its attendance row, or from expected attendance when it has none.
- A fresh initial replay is identified by the migration runner and verifies
  that the timetable and presence storage is empty before cleanup.

Execution values a previous image left on a planned or cancelled block (for
example a start on a block it cancelled) are not drift. The backfill never
mapped such a block to a session, so the Contract drops them with the column.

## Removed and kept

Removed: the columns `active_group_id`, `started_by`, `started_at`,
`completed_at`, `completed_by`, `reopen_until` and `completion_snapshot` of
`schedule.activity_instances` with the index
`idx_activity_instances_active_group_unique`, the index
`idx_activity_instances_reopenable` and the foreign keys
`fk_activity_instances_active_group_tenant`,
`fk_activity_instances_started_by_tenant` and
`activity_instances_completed_by_fkey`; the columns `status`, `substatus`,
`note`, `checked_in_at`, `checked_out_at`, `is_unplanned`, `not_scheduled`,
`manual_status_at`, `student_status_day_id` and `pickup_exception_id` of
`schedule.instance_students` with their checks, the indexes
`idx_instance_students_status`, `idx_instance_students_status_day` and
`idx_instance_students_pickup_exception` and the foreign keys
`fk_instance_students_status_day` and `fk_instance_students_pickup_exception`;
the triggers `activity_sessions_mirror`, `activity_session_attendance_mirror`,
`activity_instances_route_compatibility` and
`instance_students_route_compatibility`; the functions
`active.mirror_activity_session()`,
`active.mirror_activity_session_attendance()`,
`schedule.route_activity_instance_compatibility()` and
`schedule.route_instance_student_compatibility()`; and the counter
`active.presence_compatibility_writes`.

Changed: `check_activity_instance_status` accepts `planned` and `cancelled`
only. Blocks the mirror showed as `active` or `completed` keep their session
and read `planned` in the plan.

Kept, because the current image depends on them:

- both owner tables with their keys, checks, indexes, RLS and foreign keys;
- the plan tables' remaining columns, keys and indexes, including
  `idx_activity_instances_status` on the planning status and the calendar
  feed tombstone trigger;
- the frozen `active.presence_backfill_checkpoints` and batch evidence and the
  `migrate presence-backfill` command: `status` reads the checkpoints, `run`
  and `restart` refuse once the cutover or the Contract ran. The earlier plan
  to remove the entry point was dropped in favour of the guardian Contract's
  pattern (#2757): the refusal already rules out erasing Presence data, and
  `status` remains the operator's read of the frozen evidence.
- the composed legacy DTOs `models/schedule.ActivityInstance` and
  `InstanceStudent`. The presence projection fills their execution and
  attendance fields from the owner tables for the retained legacy callers;
  they are no longer a storage mapping for those fields. Writing either DTO
  straight into a plan table must exclude them (`test/presence_rows.go`).

Owner rows are not changed. Mirror data is never copied over owner state.

## Verification

`database/migrations/presence_contract_test.go` rebuilds the pre-Contract
world from the backfilled, cut-over storage in a disposable clone and checks
object removal, unchanged owner and plan fingerprints, the narrowed status
check, tenant isolation and cascades after the Contract, refused Down, the
refused backfill, lock wait and timeout, cancellation, rollback on a changed
fingerprint, every integrity refusal, historical counter hits, the ordinary
upgrade path, the preflight's same-release refusal and the fresh replay. The
caller inventory rejects the retired columns and objects in application
code, fixtures and behavior tests. Older migration tests restore the mirror
through `RestorePresenceStorageBeforeContract`, and the Expand, Backfill and
Cutover contracts restore the authoritative columns through
`RestorePresenceStorageBeforeCutover`.
