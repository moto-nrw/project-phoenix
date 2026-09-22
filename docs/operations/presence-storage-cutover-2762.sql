-- Run with psql -v ON_ERROR_STOP=1 through the guarded maintenance connection.
-- No application rows change. Reads of the mirrored columns are not counted;
-- the write counter counts rows the previous image routed into the owner tables.
BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET LOCAL statement_timeout = '30s';
SET LOCAL lock_timeout = '5s';
SET LOCAL TIME ZONE 'UTC';

-- Compatibility writes. Must trend to zero on the new image.
-- pg_sequence_last_value reports NULL while nextval has never been called.
SELECT clock_timestamp() AS sampled_at,
       coalesce(pg_sequence_last_value('active.presence_compatibility_writes'), 0) AS compatibility_writes;

-- Frozen evidence from the final locked delta, not a comparison with today.
SELECT tenant_id,
       (state ->> 'verified_at')::timestamptz AS verified_at,
       (state ->> 'complete')::boolean AS complete,
       (state -> 'sessions' ->> 'source_count') = (state -> 'sessions' ->> 'target_count')
       AND (state -> 'sessions' ->> 'source_checksum') = (state -> 'sessions' ->> 'target_checksum')
       AND (state -> 'attendance' ->> 'source_count') = (state -> 'attendance' ->> 'target_count')
       AND (state -> 'attendance' ->> 'source_checksum') = (state -> 'attendance' ->> 'target_checksum')
       AND (state ->> 'mismatches')::bigint = 0
       AND (state ->> 'orphans')::bigint = 0 AS cutover_checksums_equal,
       (state -> 'sessions' ->> 'target_count')::bigint AS sessions,
       (state -> 'attendance' ->> 'target_count')::bigint AS attendance,
       (state ->> 'mismatches')::bigint AS mismatches,
       (state ->> 'orphans')::bigint AS orphans
FROM active.presence_backfill_checkpoints
ORDER BY tenant_id;

-- Live comparison of the rollback mirror with the owner rows, per school.
-- execution_drift counts blocks whose mirrored execution columns differ from
-- their session (or that carry an execution without a session);
-- attendance_drift counts participants whose mirrored attendance columns
-- differ from their attendance row (a participant without a row is expected).
SELECT school.id AS tenant_id,
       (SELECT count(*)
        FROM schedule.activity_instances AS instance
        LEFT JOIN active.activity_sessions AS session
          ON session.tenant_id = instance.tenant_id AND session.schedule_instance_id = instance.id
        WHERE instance.tenant_id = school.id
          AND (
            (session.id IS NULL AND (instance.status IN ('active', 'completed') OR instance.active_group_id IS NOT NULL
               OR instance.started_at IS NOT NULL OR instance.completed_at IS NOT NULL))
            OR (session.id IS NOT NULL AND (
               instance.status IS DISTINCT FROM session.status
               OR instance.active_group_id IS DISTINCT FROM session.active_group_id
               OR instance.started_by IS DISTINCT FROM session.started_by
               OR instance.started_at IS DISTINCT FROM session.started_at
               OR instance.completed_at IS DISTINCT FROM session.completed_at
               OR instance.completed_by IS DISTINCT FROM session.completed_by
               OR instance.reopen_until IS DISTINCT FROM session.reopen_until
               OR instance.completion_snapshot IS DISTINCT FROM session.completion_snapshot))
          )) AS execution_drift,
       (SELECT count(*)
        FROM schedule.instance_students AS participant
        LEFT JOIN active.activity_session_attendance AS attendance
          ON attendance.tenant_id = participant.tenant_id AND attendance.instance_student_id = participant.id
        WHERE participant.tenant_id = school.id
          AND (
            (attendance.id IS NULL AND (participant.status <> 'expected' OR participant.substatus IS NOT NULL
               OR participant.note IS NOT NULL OR participant.checked_in_at IS NOT NULL OR participant.checked_out_at IS NOT NULL
               OR participant.is_unplanned OR participant.not_scheduled OR participant.manual_status_at IS NOT NULL
               OR participant.student_status_day_id IS NOT NULL OR participant.pickup_exception_id IS NOT NULL))
            OR (attendance.id IS NOT NULL AND (
               participant.status IS DISTINCT FROM attendance.status
               OR participant.substatus IS DISTINCT FROM attendance.substatus
               OR participant.note IS DISTINCT FROM attendance.note
               OR participant.checked_in_at IS DISTINCT FROM attendance.checked_in_at
               OR participant.checked_out_at IS DISTINCT FROM attendance.checked_out_at
               OR participant.is_unplanned IS DISTINCT FROM attendance.is_unplanned
               OR participant.not_scheduled IS DISTINCT FROM attendance.not_scheduled
               OR participant.manual_status_at IS DISTINCT FROM attendance.manual_status_at
               OR participant.student_status_day_id IS DISTINCT FROM attendance.student_status_day_id
               OR participant.pickup_exception_id IS DISTINCT FROM attendance.pickup_exception_id))
          )) AS attendance_drift
FROM platform.schools AS school
ORDER BY school.id;

-- The rollback shape must still be installed during the rollback window.
SELECT tgname AS trigger_name, tgrelid::regclass::text AS table_name
FROM pg_trigger
WHERE NOT tgisinternal
  AND tgname IN ('activity_sessions_mirror', 'activity_session_attendance_mirror',
                 'activity_instances_route_compatibility', 'instance_students_route_compatibility')
ORDER BY 1;

SELECT clock_timestamp() AS sampled_at, datname, deadlocks, stats_reset
FROM pg_stat_database WHERE datname = current_database();

SELECT wait_event_type, wait_event, count(*) AS sessions
FROM pg_stat_activity
WHERE datname = current_database() AND wait_event_type = 'Lock'
GROUP BY wait_event_type, wait_event ORDER BY wait_event;
COMMIT;
