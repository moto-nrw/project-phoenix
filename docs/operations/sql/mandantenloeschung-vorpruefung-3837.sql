-- Human-only pre-deploy inventory for 1.15.481. Counts only, no row contents.
-- Use the approved migration role with visibility across schools and RLS bypass.
BEGIN READ ONLY;
SET LOCAL row_security = off;
WITH orphan_counts(source, orphaned_rows) AS (
    SELECT 'active.presence_backfill_batches.tenant_id', count(*)
    FROM active.presence_backfill_batches r
    WHERE r.tenant_id IS NOT NULL AND NOT EXISTS (SELECT FROM platform.schools s WHERE s.id=r.tenant_id)
    UNION ALL
    SELECT 'audit.room_color_migration_backup.tenant_id', count(*)
    FROM audit.room_color_migration_backup r
    WHERE r.tenant_id IS NOT NULL AND NOT EXISTS (SELECT FROM platform.schools s WHERE s.id=r.tenant_id)
    UNION ALL
    SELECT 'audit.wc_alias_migration_backup.tenant_id', count(*)
    FROM audit.wc_alias_migration_backup r
    WHERE r.tenant_id IS NOT NULL AND NOT EXISTS (SELECT FROM platform.schools s WHERE s.id=r.tenant_id)
    UNION ALL
    SELECT 'auth.mfa_email_challenges.tenant_id', count(*)
    FROM auth.mfa_email_challenges r
    WHERE r.tenant_id IS NOT NULL AND NOT EXISTS (SELECT FROM platform.schools s WHERE s.id=r.tenant_id)
    UNION ALL
    SELECT 'auth.parent_declaration_permission_grants.tenant_id', count(*)
    FROM auth.parent_declaration_permission_grants r
    WHERE r.tenant_id IS NOT NULL AND NOT EXISTS (SELECT FROM platform.schools s WHERE s.id=r.tenant_id)
)
SELECT source, orphaned_rows, sum(orphaned_rows) OVER () = 0 AS all_school_references_valid
FROM orphan_counts
ORDER BY source;
COMMIT;
