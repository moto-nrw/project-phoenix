# Guardian storage Expand (#2716)

## Scope and rollout order

1. Apply **1.15.387**, which creates the empty owner tables
   `users.student_guardian_relationships` (People Directory),
   `users.student_guardian_pickup_permissions` (Care Plan) and
   `auth.guardian_student_access` (Identity & Access) with their keys, indexes,
   grants and forced tenant RLS in one transaction. Lock acquisition is limited
   to five seconds. The migration does not copy data and does not alter
   `users.students_guardians`, its triggers, or its indexes.
2. Keep the previous application image and its guardian traffic running. None
   of the targets has an application writer in this release. Do not seed the
   targets or introduce compatibility views, routing triggers, or dual writes.
   PostgreSQL's internal FK triggers are not routing triggers.
3. Record staging evidence below before accepting the rollout. Backfill and
   caller cutover are separate tickets under
   [#2580](https://github.com/moto-nrw/project-phoenix/issues/2580).

No prerequisite change to the old table was needed: `users.students`,
`users.guardian_profiles` and `platform.schools` already expose the composite
`(tenant_id, id)` keys the target FKs reference.

## Field mapping for the later backfill

| Source column (`users.students_guardians`) | Empty target | Notes |
| --- | --- | --- |
| `id` | `users.student_guardian_relationships.id` | One relationship per tenant/student/guardian pair |
| `student_id`, `guardian_profile_id` | Relationship columns with the same name | Composite tenant FKs; student delete cascades, guardian delete stays `RESTRICT` (#819) |
| `relationship_type`, `guardian_role`, `is_primary`, `is_emergency_contact`, `emergency_priority`, `is_payer` | Relationship columns with the same name | Same SQL type, nullability and default |
| `can_pickup`, `pickup_notes` | `users.student_guardian_pickup_permissions` | One row per relationship via `relationship_id`; cascades with the relationship |
| `permissions` | `auth.guardian_student_access.permissions` | One row per relationship; must stay a JSON object |
| `users.guardian_profiles.account_id` | `auth.guardian_student_access.account_id` | Global `auth.accounts` reference, `NULL` without a portal account, `SET NULL` on account deletion |

Single-primary and single-payer per child are partial unique indexes on the
relationship table. The old table enforces the same invariants through its
demotion trigger and `uq_students_guardians_payer`, so the backfill copies
rows without conflict resolution. The old demotion behaviour itself is not
replicated in the database; the cutover owner implements it in the People
capability.

The targets carry no triggers at all, matching #2718: neither routing triggers
nor the old table's `updated_at` maintenance trigger. The cutover ticket adds
`updated_at` maintenance together with the first application writer; until
then `updated_at` only records creation. `permissions` gains a JSON-object
CHECK; every existing row already satisfies it because the model writes only
maps and an absent value falls back to `'{}'`.

The old table retains **all** its columns, its demotion and `updated_at`
triggers, the parent-consent and meal-participation permission triggers, and
every index during Expand. The relationship table deliberately carries no
CHECK on `relationship_type` or `guardian_role`: the old table has none, and the
model validates those values, so the backfill cannot fail on legacy rows.

## Staging evidence collection

Use the normal deployment migration runner and an approved database connection.
Do not run the seeder or reset a deployed database. No staging or production
HTTP requests are needed from the agent. Record the exact old image digest,
candidate commit, database name, and migration start/end timestamps.

Collect these observations before, during, and after the migration. Keep the
same ordinary guardian workload (student detail, guardian link edit, parent
portal login) through the observation window.

```sql
-- Sample during migration. waitstart measures lock-wait age; blockers name
-- affected sessions without capturing their SQL or guardian data.
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

-- After the migration: record table, index and total bytes.
SELECT c.oid::regclass AS relation,
       pg_relation_size(c.oid) AS table_bytes,
       pg_indexes_size(c.oid) AS index_bytes,
       pg_total_relation_size(c.oid) AS total_bytes
FROM pg_class c
WHERE c.oid IN ('users.students_guardians'::regclass,
                'users.student_guardian_relationships'::regclass,
                'users.student_guardian_pickup_permissions'::regclass,
                'auth.guardian_student_access'::regclass);

SELECT (SELECT count(*) FROM users.student_guardian_relationships) AS relationships,
       (SELECT count(*) FROM users.student_guardian_pickup_permissions) AS pickup_permissions,
       (SELECT count(*) FROM auth.guardian_student_access) AS access;
```

Record migration elapsed time from the runner, maximum observed lock wait,
blocked-session count and duration, deadlock counter delta, and before/after
ordinary workload latency, error and throughput measurements. Exercise guardian
reads, link creation, role and permission edits, pickup edits and parent-portal
login with the **previous image**. Targets must still be empty afterward.
Record any workload failure instead of interpreting a successful migration as
evidence of application compatibility.

| Evidence | Status |
| --- | --- |
| Local migrated-clone column/FK/RLS/default/rollback tests | Automated in `001015387_guardian_storage_expand_test.go` |
| Local old SQL shape preservation | Automated, including demotion trigger, permission writes and empty-target assertions |
| Local seeded stack (196 guardian links) migrated in 1.45 s wall clock including Go compilation; targets empty | Observed on a developer stack, not staging |
| Staging old image digest and workload result | Pending deployment; local SQL tests do not prove this |
| Staging migration duration, lock waits, blocked sessions, deadlock deltas | Pending deployment; no values fabricated |
| Staging table/index bytes and zero target rows after ordinary traffic | Pending deployment |

The migration tests retain the internal package because the active architecture
policy forbids external migration tests from importing the migration registry.
They invoke registered up/down entry points and use shared test support without
adding a policy exception.

## Rollback

Expand rollback locks all three targets, verifies that each is empty, and drops
only those tables and their dependent indexes, policies and owned sequences. It
leaves `users.students_guardians` and every old trigger and index untouched.

If any target has rows, rollback fails without dropping any table. Stop and use
the Backfill rollback procedure; do not truncate populated targets or add
`CASCADE` to force Expand rollback. A failed Expand transaction leaves no
partially provisioned target table.
