# Guardian storage cutover and rollback window (#2756)

Migration 1.15.417 applies the final delta of the
[guardian owner backfill](guardian-owner-storage-backfill.md) under one write
lock, verifies every school, and makes the three owner tables authoritative:

- `users.student_guardian_relationships` (People Directory): `student_id`,
  `guardian_profile_id`, `relationship_type`, `guardian_role`, `is_primary`,
  `is_emergency_contact`, `emergency_priority`, `is_payer`.
- `users.student_guardian_pickup_permissions` (Care Plan): `can_pickup`,
  `pickup_notes`, one row per relationship.
- `auth.guardian_student_access` (Identity & Access): the guardian account
  binding (`account_id`) and the parents-portal `permissions`, one row per
  relationship.

`users.students_guardians` stays a base table under its old name as a
rollback-only mirror. Triggers on the three owner tables copy every owner
write onto it; a routing trigger on it copies a previous image's writes into
the owners and counts them in `users.students_guardians_compatibility_writes`.
The old name cannot become a view: the previous image links guardians with
`INSERT ... ON CONFLICT`, which PostgreSQL refuses through a view (the same
reason as the presence cutover #2762).

The same migration moves every database object that named the old table:

- The guardian foreign key of the relationship goes back to `ON DELETE
  RESTRICT` (#819); the backfill window had relaxed it to `CASCADE`.
- The consent and meal-participation grant keys point at the relationship,
  and their invalidation triggers fire on the Identity row's `permissions`.
- A trigger on `users.guardian_profiles` keeps every access row bound to the
  guardian's current account when either image links, moves or drops it.
- `platform.delivery_push_subscriptions` reads the push audience from the
  relationship and the Identity permissions.
- The mirror's id default draws from the relationship sequence, so both
  images allocate from one sequence.
- The owner tables get the `updated_at` triggers the old table had.

Current application code reads and writes the relationship only through the
owners or the tenant-safe guardian-link projection (`modules/guardianlinkview`,
owner `guardian-link-view`). People Directory's relationship store runs the
unit of work: it writes the relationship, then Care Plan's pickup permission
and Identity & Access's portal access through consumer-owned ports, inside one
savepoint that holds the relationship row lock. `TestGuardianStorageCallerInventory`
fails the build when a provider names the mirror or the counter again. Keep the
mirror, the triggers and the counter for the
[rollback window](../agents/operations.md#rollback-window-of-a-storage-cutover);
#2757 removes them once its three conditions hold. There is no waiting period.

## Release and rollback

1. Save the previous image identifier and the backfill evidence:
   `phoenix backfill guardian-owner status` reports a stable, verified
   checkpoint for every school. The migration's preflight refuses before the
   application stops when a school with links has no completed pass, or when
   a link can never be copied (a guardian of another school, a second primary
   guardian of the same child). Correct such rows first; the guide of the
   backfill names them.
2. Stop application writers and apply migrations. The cutover takes the
   backfill's advisory lock, locks the four guardian tables and
   `users.guardian_profiles` (`lock_timeout` 5 s, `statement_timeout` 60 s),
   drops target rows whose source changed a unique claim, copies the
   remainder, compares counts, checksums, rows and account bindings per
   school, and installs the compatibility shape in the same transaction. A
   failing school, a lock timeout or an error in the install aborts the whole
   transaction; nothing changed. The grant keys are validated after the
   commit; an interrupted validation resumes on the next `migrate`.
3. Deploy the new image. Exercise: link an existing guardian to a child and
   create a child with a new guardian; edit a relationship (role, pickup,
   primary); set and clear the payer; unlink; a parent editing pickup flags
   and adding a contact in the parents portal; a guardian invitation accepted
   by a parent; an enrollment approval that links guardians; the parent
   announcement audience and a push to a parent. Save the evidence below.
4. If the new image misbehaves, follow the
   [complete release rollback](release-backup-rollback.md) and deploy the
   previous image. It reads and writes the mirror; the routing trigger keeps
   the owners current, so the same data serves the next forward deployment.
   Do not run a down migration (it refuses), do not drop the triggers, the
   counter or the mirror, and do not truncate the owner tables.

After a committed switch the backfill, its `reset` and its rollback refuse to
run: a batch would copy the mirror onto itself and a reset would erase the
authoritative relationships. Drift observed afterwards is corrected in the
owner tables through the application; the mirror follows through the
triggers.

A previous image that moves a link to another child while it is primary
(`UPDATE ... SET student_id`) is refused by the relationship's
single-primary index; the old table's demotion trigger only fired on
`is_primary`, so the old image could leave two primaries. No application path
does this.

## Capture database evidence

Run [the observation SQL](guardian-owner-storage-cutover.sql) through the
guarded maintenance connection with psql, `ON_ERROR_STOP=1`, and its file
flag. Save the output before rollout, after the smoke tests, and once more
before #2757 runs. The script uses one repeatable-read, read-only
snapshot with bounded timeouts and changes no rows.

| Evidence | Required result |
| --- | --- |
| Final-delta checkpoint | Every school has `cutover_checksums_equal` true. These checksums describe the switch, not later changes. |
| Live mirror comparison | `mirror_drift`, `binding_drift` and `incomplete_links` are zero for every school. The comparison leaves out the timestamps, which each owner maintains on its own. |
| Compatibility writes | The counter delta trends to zero on the new image. A nonzero delta means a previous image, or a script, still writes the mirror. |
| Rollback shape | All five triggers are listed. |
| Deadlocks and current lock waits | Save `deadlocks` with `stats_reset`; investigate increases against operation errors. |

The counter counts routed rows, including rows of a transaction that rolled
back later and the demotions the old single-primary trigger routes. Reads of
the mirror cannot be counted on a base table; the caller inventory test and
the deployed image identifiers are the read evidence. Sample the counter
twice during an interval without maintenance traffic to measure the
application-only delta. Never reset the sequence.

## Capture application evidence

The relationship store of the serving graph reports its Identity & Access
commands (`grant_guardian_student_access`, `set_guardian_student_permissions`)
through the identity-access operation observer the root already binds
(duration, queries, rows, error code). People Directory records the
parents-portal link writes as `link_guardian_contact` and
`patch_guardian_link_pickup`, including the owner commands inside them. Care
Plan's pickup commands (`create_guardian_pickup_permission`,
`change_guardian_pickup_permission`) carry an observation seam that the
composition leaves unobserved, like the companion slice; their latency is
inside the two People Directory operations and the relationship store's
callers. Record before #2757 runs the p95 and the error counts of these
operations and of the guardian list endpoints; the query-budget register
(`backend/test/query_budgets.go`) is unchanged by the cutover because every
retained read joins the three owners in one statement through the projection.

## Local evidence

Disposable PostgreSQL clones, 2026-09-23, PR branch:

| Measurement | Result |
| --- | --- |
| Migration tests | `TestGuardianOwnerCutover*` and `TestGuardianCompatibility*` in `backend/database/migrations`: contract golden, final delta (edit, delete, newcomer, moved primary, new account binding, target-only edit), refusals (no pass, uncopyable row, second switch), rollback with a failing switch and a clean retry, routing of previous-image inserts including `ON CONFLICT`, updates, promotions and deletes, mirroring of owner writes, student deletion without counted hits, RESTRICT on a linked guardian, the push audience, two-school isolation. The Expand and Backfill contracts run against a restored pre-cutover clone. |
| Unit of work | `TestGuardianRelationship*` in `backend/database/repositories/users` and `TestGuardianPortalLinkRollsBackAfterEveryOwnerCommand` in `backend/modules/peopledirectory/compose`: a failure injected after each owner command leaves no half of the link, also when the caller commits; the retry writes the link once; promotion demotes the previous primary; another school sees and changes nothing. |
| Owner commands | `TestGuardianPickupPermissionsWriteOnlyTheirSchool` (Care Plan) and `TestGuardianStudentAccessWritesOnlyItsSchool` (Identity & Access): two-school isolation, refusal of a command naming another school, account binding following the profile. |
| Caller inventory | `TestGuardianStorageCallerInventory`: zero application literals name the mirror or the counter. |
| Architecture ratchet | `scripts/backend-architecture.sh check` passes with the 568 baseline violations unchanged (no key removed or added) and no policy loosening. |

Staging acceptance is still to be recorded:

| Staging | Result |
| --- | --- |
| Switch wall time | |
| Previous-image smoke against the mirror | |
| Counter delta on the new image under production load (zero is the gate, not the clock) | |
