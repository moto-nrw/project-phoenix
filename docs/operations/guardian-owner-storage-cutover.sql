-- Run with psql -v ON_ERROR_STOP=1 through the guarded maintenance connection.
-- No application rows change. Reads of the mirror are not counted; the write
-- counter counts rows the previous image routed into the owner tables.
BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET LOCAL statement_timeout = '30s';
SET LOCAL lock_timeout = '5s';
SET LOCAL TIME ZONE 'UTC';

-- Compatibility writes. Must trend to zero on the new image.
-- pg_sequence_last_value reports NULL while nextval has never been called.
SELECT clock_timestamp() AS sampled_at,
       coalesce(pg_sequence_last_value('users.students_guardians_compatibility_writes'), 0) AS compatibility_writes;

-- Frozen evidence of the final locked delta (#2756), not a comparison with
-- today: the switch wrote its own verdict into the backfill checkpoint.
SELECT tenant_id, verified_at, pass_completed, stable,
       source_count = target_count AND source_checksum = target_checksum
       AND mismatch_count = 0 AND guardian_access_mismatch_count = 0 AS cutover_checksums_equal,
       source_count, target_count, mismatch_count, guardian_access_mismatch_count, rows_rejected
FROM platform.storage_backfill_checkpoints
WHERE backfill = 'guardian-owner'
ORDER BY tenant_id;

-- Live comparison of the rollback mirror with the joined owners, per school.
-- Timestamps are left out: each owner maintains its own updated_at.
-- mirror_drift counts relationships whose mirror row differs from the owners
-- (or is missing, or has no owner row); binding_drift counts access rows not
-- bound to the guardian's current account; incomplete_links counts
-- relationships missing a pickup permission or an access row.
SELECT school.id AS tenant_id,
       (SELECT count(*)
        FROM users.student_guardian_relationships AS r
        LEFT JOIN users.student_guardian_pickup_permissions AS p
          ON p.tenant_id = r.tenant_id AND p.relationship_id = r.id
        LEFT JOIN auth.guardian_student_access AS a
          ON a.tenant_id = r.tenant_id AND a.relationship_id = r.id
        FULL JOIN users.students_guardians AS sg
          ON sg.tenant_id = r.tenant_id AND sg.id = r.id
        WHERE coalesce(r.tenant_id, sg.tenant_id) = school.id
          AND ROW(r.student_id, r.guardian_profile_id, r.relationship_type, r.guardian_role, r.is_primary,
                  r.is_emergency_contact, r.emergency_priority, r.is_payer, p.can_pickup, p.pickup_notes, a.permissions)
              IS DISTINCT FROM
              ROW(sg.student_id, sg.guardian_profile_id, sg.relationship_type, sg.guardian_role, sg.is_primary,
                  sg.is_emergency_contact, sg.emergency_priority, sg.is_payer, sg.can_pickup, sg.pickup_notes, sg.permissions)
       ) AS mirror_drift,
       (SELECT count(*)
        FROM auth.guardian_student_access AS a
        JOIN users.student_guardian_relationships AS r ON r.tenant_id = a.tenant_id AND r.id = a.relationship_id
        JOIN users.guardian_profiles AS g ON g.tenant_id = r.tenant_id AND g.id = r.guardian_profile_id
        WHERE a.tenant_id = school.id AND a.account_id IS DISTINCT FROM g.account_id
       ) AS binding_drift,
       (SELECT count(*)
        FROM users.student_guardian_relationships AS r
        WHERE r.tenant_id = school.id
          AND (NOT EXISTS (SELECT 1 FROM users.student_guardian_pickup_permissions AS p
                           WHERE p.tenant_id = r.tenant_id AND p.relationship_id = r.id)
               OR NOT EXISTS (SELECT 1 FROM auth.guardian_student_access AS a
                              WHERE a.tenant_id = r.tenant_id AND a.relationship_id = r.id))
       ) AS incomplete_links
FROM platform.schools AS school
ORDER BY school.id;

-- The rollback shape must still be installed during the rollback window.
SELECT tgname, tgrelid::regclass AS on_table
FROM pg_trigger
WHERE NOT tgisinternal AND tgname IN (
  'students_guardians_route_compatibility', 'student_guardian_relationships_mirror',
  'student_guardian_pickup_permissions_mirror', 'guardian_student_access_mirror',
  'guardian_profiles_bind_student_access')
ORDER BY tgname;

-- Deadlocks since the last statistics reset, and current lock waits on the
-- guardian storage.
SELECT datname, deadlocks, stats_reset FROM pg_stat_database WHERE datname = current_database();
SELECT pid, wait_event_type, wait_event, now() - query_start AS waiting_for, left(query, 120) AS query
FROM pg_stat_activity
WHERE wait_event_type = 'Lock'
  AND (query ILIKE '%student_guardian%' OR query ILIKE '%guardian_student_access%' OR query ILIKE '%students_guardians%');

COMMIT;
