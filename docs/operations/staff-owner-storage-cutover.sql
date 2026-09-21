-- Run with psql -v ON_ERROR_STOP=1 through the guarded maintenance connection.
-- No application rows change. Nothing here queries users.staff: a probe of the
-- compatibility view would advance the very read counter #2754 gates on.
BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET LOCAL statement_timeout = '30s';
SET LOCAL lock_timeout = '5s';
SET LOCAL TIME ZONE 'UTC';

-- Compatibility hits. Both must trend to zero on the new image.
-- pg_sequence_last_value reports NULL while nextval has never been called;
-- last_value would report 1 for an untouched sequence.
SELECT clock_timestamp() AS sampled_at,
       coalesce(pg_sequence_last_value('users.staff_compatibility_reads'), 0) AS compatibility_reads,
       coalesce(pg_sequence_last_value('users.staff_compatibility_writes'), 0) AS compatibility_writes;

-- Frozen evidence from the final locked delta, not a comparison with today.
SELECT tenant_id, verified_at, stable,
       source_count = target_count
       AND source_checksum = target_checksum
       AND mismatch_count = 0 AS cutover_checksums_equal,
       source_count, target_count, mismatch_count, rows_copied, rows_removed
FROM platform.storage_backfill_checkpoints
WHERE backfill = 'staff-owner'
ORDER BY tenant_id;

-- Live completeness of the rollback shape on the owner storage.
--   staff        what the previous image would read (live and retired)
--   broken       memberships without an employment profile; must be zero,
--                the inner join would hide them from the previous image
--   live_numbers personnel numbers held by more than one live staff member;
--                must be zero
SELECT school.id AS tenant_id,
       (SELECT count(*) FROM users.staff_school_memberships m
          JOIN users.staff_employment_profiles p ON p.tenant_id = m.tenant_id AND p.membership_id = m.id
         WHERE m.tenant_id = school.id) AS staff,
       (SELECT count(*) FROM users.staff_school_memberships m
         WHERE m.tenant_id = school.id AND NOT EXISTS (
           SELECT 1 FROM users.staff_employment_profiles p
            WHERE p.tenant_id = m.tenant_id AND p.membership_id = m.id)) AS broken,
       (SELECT count(*) FROM (
          SELECT p.personnel_number FROM users.staff_employment_profiles p
            JOIN users.staff_school_memberships m ON m.tenant_id = p.tenant_id AND m.id = p.membership_id
           WHERE p.tenant_id = school.id AND p.personnel_number IS NOT NULL AND m.deleted_at IS NULL
           GROUP BY p.personnel_number HAVING count(*) > 1) duplicate) AS live_numbers
FROM platform.schools school
ORDER BY school.id;

-- Repointed constraints still waiting for VALIDATE; must be zero once
-- 1.15.409 is recorded.
SELECT count(*) AS unvalidated_staff_foreign_keys
FROM pg_constraint
WHERE confrelid = 'users.staff_school_memberships'::regclass AND contype = 'f' AND NOT convalidated;

-- Lock waits and deadlocks while the release runs (sample repeatedly).
SELECT clock_timestamp() AS sampled_at,
       (SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock') AS lock_waiters,
       (SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()) AS deadlocks_total;

COMMIT;
