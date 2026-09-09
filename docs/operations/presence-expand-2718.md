# Presence storage Expand (#2718)

## Scope and rollout order

1. Apply **1.15.377**, the separately approved prerequisite unique index on
   `schedule.instance_students (tenant_id, id)`. The existing global primary
   key already guarantees uniqueness. This migration changes no rows, columns,
   or write paths. It waits at most five seconds for a lock and limits the
   index statement to sixty seconds. Index creation can block old-table writes
   while building; measure this step separately from Expand and schedule it
   for a quiet window. Failure rolls back the index build; do not bypass the
   timeout without first investigating contention and table size.
2. Apply **1.15.378**, which creates empty `active.activity_sessions` and
   `active.activity_session_attendance` tables, their keys, indexes, grants and
   forced tenant RLS in one transaction. Lock acquisition is limited to five
   seconds. The migration does not copy data or change old-table definitions.
3. Keep the previous application image and its existing timetable traffic
   running. Neither target has an application writer in this release. Do not
   seed the targets or introduce compatibility views, routing triggers, or
   dual writes. PostgreSQL's internal FK triggers are not routing triggers.
4. Record staging evidence below before accepting the rollout. Backfill is
   [#2761](https://github.com/moto-nrw/project-phoenix/issues/2761); caller and
   status cutover is [#2762](https://github.com/moto-nrw/project-phoenix/issues/2762).

The prerequisite is an approved exception to the original ticket's restriction
on old-table changes: PostgreSQL rejected the required tenant-composite
participant FK with SQLSTATE `42830` until this index existed. Moving it to
Cutover would create a dependency cycle through Backfill and Expand.

## Field mapping for the later backfill

| Source | Empty target | Mapping |
| --- | --- | --- |
| `schedule.activity_instances.id` | `active.activity_sessions.schedule_instance_id` | One session per tenant/occurrence |
| Old occurrence `active` / `completed` status | Session `status` | Same value; these are the only session statuses |
| Occurrence runtime fields | Session fields with the same name | `active_group_id`, `started_by`, `started_at`, `completed_at`, `completed_by`, `reopen_until`, `completion_snapshot` |
| `schedule.instance_students.id` | `active.activity_session_attendance.instance_student_id` | One attendance row per tenant/planned participant |
| Participant attendance fields | Attendance fields with the same name | `status`, `substatus`, `note`, `checked_in_at`, `checked_out_at`, `is_unplanned`, `not_scheduled`, `manual_status_at`, `student_status_day_id`, `pickup_exception_id` |

The old occurrence retains **all** its fields and its four-value status
constraint during Expand. At Cutover, planning owns `date`, `activity_group_id`,
`calendar_period_id`, `title`, `description`, `start_time`, `end_time`, `room_id`,
`required_staff`, `list_kind`, `is_spontaneous`, `understaffed_ack`,
`understaffed_note`, `cancel_reason`, `notes`, and `created_by`; planned
participants retain `instance_id`, `student_id`, and `room_id`. Do not narrow
planning status or remove runtime fields in this release.

Moved fields keep their SQL types, nullability and attendance defaults.
`completed_by` remains a **global auth account** reference, unlike the
tenant-scoped staff reference in `started_by`. Optional link deletion clears
only the link column, not `tenant_id`. Deleting a planned occurrence or
participant cascades to its target row. Presence owns both new data objects;
all existing architecture owners and legacy ratchet keys remain unchanged.

## Staging evidence collection

Use the normal deployment migration runner and an approved database connection.
Do not run the seeder or reset a deployed database. No staging or production
HTTP requests are needed from the agent. Record the exact old image digest,
candidate commit, database name, and migration start/end timestamps.

Collect these observations before, during, and after **each** migration. Keep
the same ordinary old-table workload through the observation window.

```sql
-- Sample during migration. waitstart measures lock-wait age; blockers name
-- affected sessions without capturing their SQL or student data.
SELECT clock_timestamp() AS sampled_at, a.pid, a.application_name,
       a.state, a.wait_event_type, a.wait_event,
       l.mode, l.waitstart,
       clock_timestamp() - l.waitstart AS lock_wait_age,
       pg_blocking_pids(a.pid) AS blocked_by
FROM pg_stat_activity a
LEFT JOIN pg_locks l ON l.pid = a.pid AND NOT l.granted
WHERE a.datname = current_database()
  AND (a.wait_event_type = 'Lock' OR a.application_name LIKE '%migrat%');

-- Compare counters before and after, not lifetime values in isolation.
SELECT clock_timestamp() AS sampled_at, datname, deadlocks,
       xact_commit, xact_rollback, stats_reset
FROM pg_stat_database WHERE datname = current_database();

-- After both steps: record table, index and total bytes, including the
-- separately approved prerequisite's cost on the old participant table.
SELECT c.oid::regclass AS relation,
       pg_relation_size(c.oid) AS table_bytes,
       pg_indexes_size(c.oid) AS index_bytes,
       pg_total_relation_size(c.oid) AS total_bytes
FROM pg_class c
WHERE c.oid IN ('schedule.instance_students'::regclass,
                'active.activity_sessions'::regclass,
                'active.activity_session_attendance'::regclass);

SELECT pg_relation_size('schedule.uq_instance_students_tenant_id')
       AS prerequisite_index_bytes;

SELECT (SELECT count(*) FROM active.activity_sessions) AS sessions,
       (SELECT count(*) FROM active.activity_session_attendance) AS attendance;
```

Record migration elapsed time from the runner, maximum observed lock wait,
blocked-session count and duration, deadlock counter delta, and before/after
ordinary workload latency, error and throughput measurements. Exercise reads,
creation, attendance changes, completion and reopen with the **previous image**.
Targets must still be empty afterward. Record any workload failure instead of
interpreting a successful migration as evidence of application compatibility.

| Evidence | Status |
| --- | --- |
| Local migrated-clone column/FK/RLS/default/rollback tests | Automated in `001015377_instance_students_tenant_key_test.go` and `001015378_presence_expand_test.go` |
| Local old SQL shape preservation | Automated, including planning/attendance writes and empty-target assertions |
| Staging old image digest and workload result | Pending deployment; local SQL tests do not prove this |
| Staging migration durations, lock waits, blocked sessions, deadlock deltas | Pending deployment; no values fabricated |
| Staging table/index bytes and zero target rows after ordinary traffic | Pending deployment |

The migration tests retain the internal package because the active architecture
policy forbids external migration tests from importing the migration registry.
They invoke registered up/down entry points and use shared test support without
adding a policy exception. This follows the explicit architecture-policy
precedence in `backend/CLAUDE.md` over the general external-test guideline.

## Rollback

Rollback 1.15.378 before 1.15.377 using the migration runner's dependency order.
Expand rollback locks both targets, verifies that both are empty, and drops only
those tables and their dependent indexes, policies and owned sequences. It
leaves the prerequisite index and all old data intact. The separate prerequisite
rollback drops only its index and refuses while a target FK still depends on it.

If either target has rows, Expand rollback fails without dropping either table.
Stop and use the Backfill rollback procedure; do not truncate populated targets
or add `CASCADE` to force Expand rollback. A failed Expand transaction leaves no
partially provisioned target table.
