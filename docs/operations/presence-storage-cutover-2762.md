# Presence storage cutover and rollback window (#2762)

Migration 1.15.415 applies the final delta of the
[Timetable → Presence backfill](../presence-backfill.md) under one write lock,
verifies every school, and makes the two Presence targets authoritative:

- `active.activity_sessions` owns the execution of a block: `status`
  (`active`/`completed`), `active_group_id`, `started_by`, `started_at`,
  `completed_at`, `completed_by`, `reopen_until`, `completion_snapshot`.
- `active.activity_session_attendance` owns the attendance of a participant:
  `status`, `substatus`, `note`, `checked_in_at`, `checked_out_at`,
  `is_unplanned`, `not_scheduled`, `manual_status_at`,
  `student_status_day_id`, `pickup_exception_id`.

`schedule.activity_instances` and `schedule.instance_students` keep the plan
(dates, times, rooms, participants) and stay base tables under their old
names. The old execution and attendance columns remain in place as a
rollback-only mirror: two triggers copy every owner write onto them, two
routing triggers copy a previous image's writes of those columns into the
owner tables and count them in `active.presence_compatibility_writes`. The
old names cannot become views because the previous image books participants
with `INSERT ... ON CONFLICT`, which PostgreSQL refuses through a view.

Current application code reads and writes the execution and the attendance
only through the Student Presence owner (`modules/studentpresence`) or the
tenant-safe presence projection (`modules/presenceprojection`) that joins
the plan with the owner tables for the retained list endpoints.
`TestPresenceStorageCallerInventory` fails the build when a provider names a
mirrored column or the counter again. Keep the triggers, the counter and the
columns for the
[rollback window](../agents/operations.md#rollback-window-of-a-storage-cutover);
#2763 removes them once its three conditions hold. There is no waiting period.

## Release and rollback

1. Save the previous image identifier and the completed backfill evidence:
   `presence-backfill status --all-tenants` reports `complete: true` for every
   school with timetable rows. The migration's preflight names the schools
   without a completed pass and refuses before the application stops.
2. Stop application writers and apply migrations. The cutover locks the four
   tables (`lock_timeout` 5 s, `statement_timeout` 60 s), copies the
   remainder the backfill has not seen, compares counts and checksums per
   school, and installs the mirror and routing triggers in the same
   transaction. Unequal targets, a missing pass or a lock timeout abort the
   whole transaction; nothing changed. Fix the cause, rerun the backfill until
   `status` reports a completed pass, and migrate again. Running the migration
   again after a committed switch is a no-op.
3. Deploy the new image. Exercise a block start on a device, check-ins and
   checkouts, a manual attendance decision, a reported day status, a partial
   pickup excusal, the block end, a reopen and the class-day lists. Save the
   evidence below with the image identifiers and sampling times.
4. If the new image misbehaves, follow the
   [complete release rollback](release-backup-rollback.md) and deploy the
   previous image. It reads and writes the mirrored columns; the routing
   triggers keep the owner tables current, so the same data serves the next
   forward deployment. Do not run a down migration (it refuses), do not drop
   the triggers or the counter, and do not truncate the owner tables.

After a committed switch the pre-cutover backfill and its `restart` refuse to
run: a batch would copy the mirror onto itself and a restart would erase
authoritative Presence data. Drift observed afterwards is corrected in the
owner tables through the application; the mirror follows through the
triggers.

## Capture database evidence

Run [the observation SQL](presence-storage-cutover-2762.sql) through the
guarded maintenance connection with psql, `ON_ERROR_STOP=1`, and its file
flag. Save the output before rollout, after the smoke tests, and once more
before #2763 runs. The script uses one repeatable-read, read-only
snapshot with bounded timeouts and changes no rows.

| Evidence | Required result |
| --- | --- |
| Final-delta checkpoint | Every reported school has `complete` true and `cutover_checksums_equal` true. These checksums describe the switch, not later changes. |
| Live mirror comparison | `execution_drift` and `attendance_drift` are zero for every school: the mirrored columns equal the owner rows. |
| Compatibility writes | The counter delta trends to zero on the new image. A nonzero delta means a previous image, or a script, still writes the mirrored columns. |
| Deadlocks and current lock waits | Save `deadlocks` with `stats_reset`; investigate increases against operation errors. |

The counter counts routed rows, including rows of a transaction that rolled
back later. Reads of the mirrored columns cannot be counted on a base table;
the caller inventory test and the deployed image identifiers are the read
evidence. Sample the counter twice during an interval without maintenance
traffic to measure the application-only delta. Never reset the sequence.

## Capture application evidence

Timetable & Activities exposes its operations on the existing Prometheus
endpoint under `phoenix_timetable_activities_*` (operations, duration,
queries, rows, statement duration). Student Presence operations are logged
at debug level (`student presence operation` with `operation`, `duration`,
`queries`, `rows`, `error`) by the serving root; they carry no school IDs and
no child data. Record before #2763 runs:

- p95 of `start_activity_session`, `complete_activity_session`,
  `check_in_participants`, `check_out_participants`,
  `patch_session_attendance`, `apply_status_day`, `apply_partial_absence`
  and `session_execution`, and their error counts, from the log lines;
- p95 of `resolve_pickup_extension`, `patch_activity_instance` and the list
  operations from the Timetable histograms;
- the query-budget register (`backend/test/query_budgets.go`) is unchanged
  by the cutover: every retained list keeps its statement count.

## Local evidence

Disposable PostgreSQL clones, 2026-09-22, PR branch:

| Measurement | Result |
| --- | --- |
| Migration tests | `TestPresenceCutover*` and `TestPresenceCompatibility*` in `backend/database/migrations`: final delta, rollback with a failing switch, unequal targets refused, mirror and routing in both directions, tenant isolation of the routing. |
| Owner tests | `TestActivitySessionStartsOncePerBlock`, `TestSessionStorageIsTenantIsolated`, `TestSessionStorageWritesRollBackWithTheCallerTransaction` in `backend/modules/studentpresence/compose`. |
| Composition tests | `TestActivityInstanceCreateInExecutionStateJoinsTheCallerTransaction`, `TestActivityInstanceCreateRollsBackThePlanWhenTheSessionCannotStart` in `backend/modules/timetable/compose/httpintegration/legacy_presence_cutover_composition_test.go`. |
| Caller inventory | `TestPresenceStorageCallerInventory`: zero application literals name a mirrored column or the counter. |
| Query budgets | Unchanged register; the retained lists read the plan and the owner rows in one joined statement each. |
| Architecture ratchet | `scripts/backend-architecture.sh check` passes with the 568 baseline violations unchanged (no key removed or added) and no policy loosening. |

Staging acceptance (switch wall time on real data, previous image against the
switched schema, counter trend under load) is still to be
recorded:

| Staging | Result |
| --- | --- |
| Switch wall time | |
| Previous-image smoke against the mirror | |
| Counter delta after 24 h on the new image | |
