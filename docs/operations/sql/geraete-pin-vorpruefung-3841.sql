-- Human-only pre-deploy inventory. No PINs or school identifiers are returned.
-- Run with a read-only-capable administrative role that can see every school
-- and setting row, not the tenant application role (RLS would hide rows).
BEGIN READ ONLY;
SELECT
    count(*) FILTER (WHERE v.id IS NULL) AS schools_without_override,
    count(*) FILTER (WHERE v.id IS NOT NULL AND
        (v.value = 'null'::jsonb OR v.value = '""'::jsonb)) AS schools_with_empty_override,
    count(*) FILTER (WHERE v.id IS NOT NULL AND
        (jsonb_typeof(v.value) <> 'string' OR
         COALESCE(v.value #>> '{}', '') !~ '^[0-9]{4}$')) AS schools_with_invalid_override
FROM platform.schools AS s
LEFT JOIN config.setting_values AS v
    ON v.tenant_id = s.id AND v.setting_key = 'security.ogs_device_pin'
WHERE s.deleted_at IS NULL;
COMMIT;
