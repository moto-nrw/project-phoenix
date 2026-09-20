-- Run with psql -v ON_ERROR_STOP=1 through the guarded maintenance connection.
-- No application rows change. Nothing here queries users.students: a probe of
-- the compatibility view would advance the very read counter #2760 gates on.
BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET LOCAL statement_timeout = '30s';
SET LOCAL lock_timeout = '5s';
SET LOCAL TIME ZONE 'UTC';

-- Compatibility hits. Both must trend to zero on the new image.
-- pg_sequence_last_value reports NULL while nextval has never been called;
-- last_value would report 1 for an untouched sequence.
SELECT clock_timestamp() AS sampled_at,
       coalesce(pg_sequence_last_value('users.student_compatibility_reads'), 0) AS compatibility_reads,
       coalesce(pg_sequence_last_value('users.student_compatibility_writes'), 0) AS compatibility_writes;

-- Frozen evidence from the final locked delta, not a comparison with today.
SELECT tenant_id, verified_at, stable,
       source_count = target_count
       AND source_checksum = target_checksum
       AND mismatch_count = 0
       AND guardian_mismatch_count = 0 AS cutover_checksums_equal,
       source_count, target_count, mismatch_count,
       guardian_mismatch_count, care_state_mismatch_count
FROM platform.storage_backfill_checkpoints
WHERE backfill = 'student-owner'
ORDER BY tenant_id;

-- Live completeness of the rollback shape, measured on the owner storage under
-- the view's own predicate.
--   students            what the previous image would read
--   hidden              profiles whose enrollment was retired after Cutover;
--                       expected to grow slowly, they left the school
--   broken              live enrollments without a care profile; must be zero,
--                       this is the one shape a rollback would silently lose
--   stale_archived_rows archive rows of children the owner storage no longer
--                       holds; the compatibility repair removes them
--   archived_missing    children enrolled after Cutover, which have no archive
--                       row and therefore read NULL legacy columns; expected
SELECT school.id AS tenant_id,
       (SELECT count(*) FROM users.student_profiles p
          JOIN users.student_school_memberships m
            ON m.tenant_id = p.tenant_id AND m.student_profile_id = p.id AND m.deleted_at IS NULL
          JOIN users.student_care_profiles c ON c.tenant_id = m.tenant_id AND c.membership_id = m.id
         WHERE p.tenant_id = school.id) AS students,
       (SELECT count(*) FROM users.student_profiles p WHERE p.tenant_id = school.id) AS profiles,
       (SELECT count(*) FROM users.student_profiles p
         WHERE p.tenant_id = school.id AND NOT EXISTS (
           SELECT 1 FROM users.student_school_memberships m
            WHERE m.tenant_id = p.tenant_id AND m.student_profile_id = p.id AND m.deleted_at IS NULL)) AS hidden,
       (SELECT count(*) FROM users.student_school_memberships m
         WHERE m.tenant_id = school.id AND m.deleted_at IS NULL AND NOT EXISTS (
           SELECT 1 FROM users.student_care_profiles c
            WHERE c.tenant_id = m.tenant_id AND c.membership_id = m.id)) AS broken,
       (SELECT count(*) FROM users.students_legacy l
         WHERE l.tenant_id = school.id AND NOT EXISTS (
           SELECT 1 FROM users.student_profiles p WHERE p.tenant_id = l.tenant_id AND p.id = l.id)) AS stale_archived_rows,
       (SELECT count(*) FROM users.student_profiles p
         WHERE p.tenant_id = school.id AND NOT EXISTS (
           SELECT 1 FROM users.students_legacy l WHERE l.tenant_id = p.tenant_id AND l.id = p.id)) AS archived_missing
FROM platform.schools school
ORDER BY school.id;

-- Repointed constraints whose existing rows are still unconfirmed. Empty after
-- a completed migration; a non-empty list means the post-commit validation was
-- interrupted and `backfill student-owner compatibility` should be rerun.
SELECT con.conrelid::regclass::text AS dependent_table, con.conname AS constraint_name
FROM pg_constraint AS con
WHERE con.confrelid = 'users.student_profiles'::regclass
  AND con.contype = 'f' AND NOT con.convalidated
ORDER BY 1, 2;

-- No dependent table may still point at the rollback archive.
SELECT con.conrelid::regclass::text AS dependent_table, con.conname AS constraint_name
FROM pg_constraint AS con
WHERE con.confrelid = 'users.students_legacy'::regclass AND con.contype = 'f'
ORDER BY 1, 2;

SELECT clock_timestamp() AS sampled_at, datname, deadlocks, stats_reset
FROM pg_stat_database WHERE datname = current_database();

SELECT wait_event_type, wait_event, count(*) AS sessions
FROM pg_stat_activity
WHERE datname = current_database() AND wait_event_type = 'Lock'
GROUP BY wait_event_type, wait_event ORDER BY wait_event;
COMMIT;
