# Staff owner storage Expand (#2715)

Migration `1.15.376` creates empty target storage. `users.staff` remains the
only application authority. There is no backfill, caller switch, compatibility
view, or synchronization trigger.

## Field mapping

| Target | Source in `users.staff` |
| --- | --- |
| `staff_school_memberships.id` | `id` (future backfill preserves identity) |
| `staff_school_memberships.tenant_id`, `person_id` | Same columns |
| `staff_school_memberships.created_at`, `updated_at`, `deleted_at` | Same columns; lifecycle and soft deletion |
| `staff_employment_profiles.membership_id` | `id`, through the new membership FK |
| `staff_employment_profiles.tenant_id` | `tenant_id`, repeated for RLS and the composite membership FK |
| `staff_employment_profiles.staff_notes`, `employment_type`, `work_time_model_id`, `personnel_number`, `rotation_anchor_date`, `birthday_display_opt_out` | Same columns and storage types |

Membership owns lifecycle. Workforce owns employment fields. Existing policy
owners and application composition remain unchanged. No new models or
repositories are needed while the target tables have no application callers.

The membership/person and profile/membership FKs enforce tenant equality.
Work-time models have no `(tenant_id, id)` unique key. The profile keeps their
existing ID FK and adds a restrictive RLS check for tenant-role assignments,
without changing the work-time-model table. Superuser writes bypass RLS and
must validate the tenant during the later backfill.

**Approved Expand boundary:** personnel-number lookup is indexed, but the target
does not yet enforce active-membership uniqueness. The current partial unique
index depends on `users.staff.deleted_at`; it remains active and unchanged
there. A profile-only unique index would reject valid reuse after offboarding.
An index cannot use the membership table's deletion predicate.

The implementation owner approved deferring target enforcement to
[Cutover #2753](https://github.com/moto-nrw/project-phoenix/issues/2753).
Cutover must preserve concurrent duplicate rejection and reuse after
offboarding before enabling target writes. Expand leaves the authoritative
old-table constraint intact. [Backfill #2752](https://github.com/moto-nrw/project-phoenix/issues/2752)
and Cutover remain separate work.

## Rollback

Rollback locks both new tables, checks that both are empty, and drops them in
FK order in one transaction. Their owned sequence, indexes, and policies go
with them. It neither modifies nor drops existing tables. If either target
contains rows, rollback fails without deleting them: undo Cutover first.

## Staging acceptance record

**Not yet executed.** Local migration tests are not staging or previous-image
evidence. Run this on the agreed staging deployment through its normal
migration path. Do not use the dev seeder against staging.

1. Record the deployment SHA, previous image digest, database version, and
   start time. Keep the previous image serving its usual staff read/write
   traffic before, during, and after migration.
2. Record migration wall time from deployment logs. From a separate database
   observer, sample lock waits and blocked sessions during the migration.
   Record sample interval and peak counts, not only the final zero.
3. Compare database deadlock counters before and after. Record relation sizes
   and confirm both new tables contain zero rows after old-table writes.
4. Compare the previous image's staff read/write outcomes and latency before,
   during, and after. Record any error or changed response; redact personal
   data. Confirm the old schema, indexes, and triggers are unchanged.
5. On an isolated staging database clone, exercise down/up and the previous
   image again. Attach the logs and measured values to the PR before claiming
   the issue's runtime acceptance complete.

Observer SQL (read-only, through the approved staging database connection):

```sql
SELECT clock_timestamp(), pid, application_name, state,
       wait_event_type, wait_event, pg_blocking_pids(pid) AS blocking_pids,
       clock_timestamp() - query_start AS query_age
FROM pg_stat_activity
WHERE datname = current_database()
  AND (wait_event_type = 'Lock' OR cardinality(pg_blocking_pids(pid)) > 0);

SELECT clock_timestamp(), deadlocks, stats_reset
FROM pg_stat_database WHERE datname = current_database();

SELECT rel::text AS relation,
       pg_table_size(rel) AS table_bytes,
       pg_indexes_size(rel) AS index_bytes,
       pg_total_relation_size(rel) AS total_bytes
FROM unnest(ARRAY[
  'users.staff'::regclass,
  'users.staff_school_memberships'::regclass,
  'users.staff_employment_profiles'::regclass
]) AS rel;

SELECT 'memberships' AS target, count(*) FROM users.staff_school_memberships
UNION ALL
SELECT 'profiles', count(*) FROM users.staff_employment_profiles;
```

| Evidence | Result |
| --- | --- |
| Deployment SHA / previous image digest | Pending |
| Migration duration | Pending |
| Lock wait duration / sample interval | Pending |
| Peak blocked sessions | Pending |
| Deadlock delta / counter reset check | Pending |
| Table and index bytes | Pending |
| Empty targets after old-table traffic | Pending |
| Previous-image read/write parity and latency | Pending |
| Isolated down/up compatibility | Pending |

## Local verification (2026-09-09)

The following passed with the pinned toolchain and `CGO_ENABLED=0` (the local
macOS C linker could not resolve `libresolv`):

- `scripts/run-go-toolchain.sh scripts/test-backend.sh`: 25,691 tests reported,
  zero failures, two skips. Most unchanged packages used the Go test cache.
- `scripts/run-go-toolchain.sh scripts/test-changed.sh origin/development`:
  all six affected packages passed; no frontend changes.
- `scripts/backend-architecture.sh check`: 1,733 existing violations,
  no new keys; composition remained 798 → 798.
- `golangci-lint run --timeout 10m` through the pinned runner: zero issues.

The skipped tests require configured web push and a seeded stack respectively.
The seed coverage ratchet has **not** been verified against a seeded stack.
Migration tests verify empty targets, source column mapping, constraints,
two-tenant RLS, guarded down/up, and unchanged source-table catalog metadata.
These results do not replace the staging and previous-image checks above.
