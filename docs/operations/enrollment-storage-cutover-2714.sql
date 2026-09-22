-- HISTORICAL: requires the 1.15.385 compatibility schema, which migration
-- 1.15.413 (#2719) removed. It no longer executes against current databases.
-- Run with psql -v ON_ERROR_STOP=1 through the guarded maintenance connection.
-- No application rows change. The compatibility probe advances its read counter.
BEGIN ISOLATION LEVEL REPEATABLE READ;
SET LOCAL statement_timeout = '30s';
SET LOCAL lock_timeout = '5s';
SET LOCAL TIME ZONE 'UTC';

-- Sample these counters separately when measuring application compatibility hits.
SELECT clock_timestamp() AS sampled_at,
       (SELECT CASE WHEN is_called THEN last_value ELSE 0 END
        FROM enrollment.request_child_compatibility_reads) AS compatibility_reads,
       (SELECT CASE WHEN is_called THEN last_value ELSE 0 END
        FROM enrollment.request_child_compatibility_writes) AS compatibility_writes;

-- Frozen evidence from the final locked delta, not a comparison to today's data.
SELECT tenant_id, verified_at, complete,
       source_bookings = target_bookings
       AND source_selections = target_selections
       AND source_bookings_checksum = target_bookings_checksum
       AND source_selections_checksum = target_selections_checksum
       AND mismatch_count = 0 AND orphan_count = 0 AND day_differences = 0
       AS cutover_checksums_equal
FROM enrollment.request_child_storage_backfill_checkpoints
ORDER BY tenant_id;

-- Live effective-state comparison in one snapshot. Archived submission notes
-- intentionally differ from later bookings and are not an effective-state field.
WITH rows AS (
    SELECT 'target' AS provider, tenant_id, id, request_child_id, care_offering_id,
           manual_selected_days, automatic_selected_days, valid_from, valid_until,
           created_at, updated_at,
           CASE WHEN manual_selected_days IS NULL AND automatic_selected_days IS NULL THEN NULL ELSE COALESCE((
               SELECT jsonb_agg(day ORDER BY array_position(ARRAY['mon','tue','wed','thu','fri','sat','sun'], day), day)
               FROM (SELECT DISTINCT jsonb_array_elements_text(
                   CASE WHEN jsonb_typeof(manual_selected_days) = 'array' THEN manual_selected_days ELSE '[]'::jsonb END ||
                   CASE WHEN jsonb_typeof(automatic_selected_days) = 'array' THEN automatic_selected_days ELSE '[]'::jsonb END
               ) AS day) normalized_days
           ), '[]'::jsonb) END AS effective_days
    FROM enrollment.care_offering_bookings
    UNION ALL
    SELECT 'compatibility', tenant_id, id, request_child_id, care_offering_id,
           CASE WHEN COALESCE(manual_selected_days, '[]'::jsonb) IN ('[]'::jsonb, 'null'::jsonb)
                AND COALESCE(automatic_selected_days, '[]'::jsonb) IN ('[]'::jsonb, 'null'::jsonb)
                THEN selected_days ELSE manual_selected_days END,
           automatic_selected_days, valid_from, valid_until, created_at, updated_at,
           CASE WHEN selected_days IS NULL AND NULL::jsonb IS NULL THEN NULL ELSE COALESCE((
               SELECT jsonb_agg(day ORDER BY array_position(ARRAY['mon','tue','wed','thu','fri','sat','sun'], day), day)
               FROM (SELECT DISTINCT jsonb_array_elements_text(
                   CASE WHEN jsonb_typeof(selected_days) = 'array' THEN selected_days ELSE '[]'::jsonb END ||
                   CASE WHEN jsonb_typeof(NULL::jsonb) = 'array' THEN NULL::jsonb ELSE '[]'::jsonb END
               ) AS day) normalized_days
           ), '[]'::jsonb) END
    FROM enrollment.request_child_offerings
), checksums AS (
    SELECT provider, tenant_id, count(*) AS row_count,
           encode(sha256(convert_to(string_agg(jsonb_build_array(
               id, request_child_id, care_offering_id, manual_selected_days,
               automatic_selected_days, valid_from, valid_until, created_at,
               updated_at, effective_days)::text, E'\n' ORDER BY id), 'UTF8')), 'hex') AS checksum
    FROM rows GROUP BY provider, tenant_id
)
SELECT COALESCE(target.tenant_id, compatibility.tenant_id) AS tenant_id,
       target.row_count AS target_rows, compatibility.row_count AS compatibility_rows,
       target.checksum AS target_checksum, compatibility.checksum AS compatibility_checksum,
       target.row_count IS NOT DISTINCT FROM compatibility.row_count
       AND target.checksum IS NOT DISTINCT FROM compatibility.checksum AS effective_checksums_equal
FROM (SELECT * FROM checksums WHERE provider = 'target') target
FULL JOIN (SELECT * FROM checksums WHERE provider = 'compatibility') compatibility USING (tenant_id)
ORDER BY tenant_id;

SELECT clock_timestamp() AS sampled_at, datname, deadlocks, stats_reset
FROM pg_stat_database WHERE datname = current_database();

SELECT wait_event_type, wait_event, count(*) AS sessions
FROM pg_stat_activity
WHERE datname = current_database() AND wait_event_type = 'Lock'
GROUP BY wait_event_type, wait_event ORDER BY wait_event;
COMMIT;
