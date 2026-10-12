# Visit retention backfill: one-off cleanup after #3835

Before #3835 the nightly visit retention only processed children with an
accepted privacy consent. Completed visits (`active.visits`) of every other
child were never deleted. The fixed cleanup applies:

| Child | Retention window |
| --- | --- |
| At least one accepted consent | Shortest `data_retention_days` of its accepted consents |
| No consent row, or only declined consents | Tenant setting `gdpr.privacy_consent_retention_days` (override, else registry default 30) |

The scheduler removes the backlog on its next run in every school with
`gdpr.data_cleanup_enabled`. The one-off run below also covers schools where
the scheduled cleanup is off, and records the count the issue asks for.

## Run once after the deploy

Run inside the server container, like every
[cleanup CLI](../agents/operations.md#cleanup-cli) command. The commands work
on every active school; inactive schools are skipped.

1. Record the count before deleting (dry run, no writes). Do this before
   the next scheduled cleanup (`gdpr.data_cleanup_time`, default 02:00). If
   the scheduler ran first, the audit query in step 3 is the count instead.

   ```bash
   go run . cleanup stats
   go run . cleanup preview --verbose
   ```

   `stats` prints per school the expired visits, the students affected and
   the oldest expired visit. Copy that output into the issue.

2. Delete:

   ```bash
   go run . cleanup visits
   ```

   There is no `cleanup visits preview`; extra arguments are ignored, so this
   command always deletes. It prints `Records deleted` per school and exits
   nonzero if a school fails. The run is idempotent, so a failed school can
   simply be rerun.

3. Verify and document:

   ```bash
   go run . cleanup stats
   ```

   Every school must report zero expired visits. Cross-check the audit trail:

   ```sql
   SELECT tenant_id, COUNT(*) AS students, SUM(records_deleted) AS visits
   FROM audit.data_deletions
   WHERE deletion_type = 'visit_retention' AND deleted_at >= '<run start>'
   GROUP BY tenant_id ORDER BY tenant_id;
   ```

   Post the per-school counts from step 1 and this query on #3835.

## Rollback

Deleted visits cannot be restored from the application. Restore from the
database backup only if the run deleted visits that were still inside their
window; see [release backup and rollback](release-backup-rollback.md).
