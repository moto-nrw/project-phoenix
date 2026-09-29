# Guardian owner storage Contract (#2757)

Migration `1.15.429` removes the rollback mirror that Cutover `1.15.417`
([#2756](guardian-owner-storage-cutover.md)) kept for previous-image rollback.
It follows the [student](student-owner-storage-contract.md),
[request-child](enrollment-storage-contract-2719.md) and
[staff](staff-owner-storage-contract.md) Contracts: one backend and one
frontend per environment, both stopped before migration, no rolling
mixed-version rollout and no observation window.

## Before the release

The issue names three conditions. Check them before the release that carries
`1.15.429`:

1. The cutover shipped in an earlier production release: `1.15.417` is
   recorded in `public.bun_migrations` and the owner tables are in use.
2. Nobody uses the old path. Run the
   [observation SQL](guardian-owner-storage-cutover.sql) on the production
   database while the current image serves requests: `compatibility_writes`
   must be zero, and `mirror_drift`, `binding_drift` and `incomplete_links`
   must be zero for every school. The caller inventory
   (`TestGuardianStorageCallerInventory`) is green in CI.
3. A restorable release backup is verified, not merely present. The release
   sequence below takes and verifies it.

The migration does not read the counter itself. As for the earlier Contracts,
historical hits are diagnostic, not a cleanup gate: the previous image is
stopped before the migration runs, and a failure after the backup restores the
complete release.

`incomplete_links` matters here because the mirror triggers never mirrored a
standalone `DELETE` of one pickup permission or access row (#3539 review). No
code path issues one; the relationship delete removes both halves by
foreign-key cascade. A relationship with a missing half would show up as an
incomplete link, so the Contract refuses it instead of patching the triggers.

## Release sequence

1. Pull the new images and run the read-only migration data preflight while
   the old release still serves requests. An integrity failure aborts before
   downtime; preflight neither removes objects nor changes rows or the counter.
2. Stop both application services, including the backend's embedded jobs.
3. Create and verify the complete release backup.
4. Run the migration. It takes the guardian backfill's advisory lock, rechecks
   integrity under `ACCESS EXCLUSIVE` locks on the mirror, the three owner
   tables and `users.guardian_profiles` (5 s lock timeout, 60 s statement
   timeout) and removes the compatibility objects in one transaction. Row
   counts and SHA-256 fingerprints of the three owner tables per school must be
   unchanged before it commits.
5. Start both new images and verify health. A failure after the backup
   restores the complete saved release.

After the Contract an old image alone is not a rollback: restore the
pre-Contract backup and the prior images together. Down is refused.

## Preflight refusals

- The completed cutover schema is required: `users.students_guardians` a base
  table, the write counter, the routing, mirror and single-primary functions,
  the two pre-Cutover grant invalidation functions and all four compatibility
  triggers present.
- A stored function other than the removed ones that names
  `users.students_guardians`, a view that depends on it and a foreign key into
  it. `DROP ... RESTRICT` also refuses any unknown dependency.
- Unvalidated foreign keys on or into the owner tables, and owner RLS that is
  not enabled and forced.
- An incomplete link (relationship without pickup permission or access row),
  an access row not bound to the guardian's current account, and mirror drift
  (a mirror row that differs from the joined owners, or is missing).
- A fresh initial replay is identified by the migration runner and verifies
  that the guardian storage is empty before cleanup.

## Removed and kept

Removed: the table `users.students_guardians` with its triggers
(`students_guardians_route_compatibility`,
`enforce_single_primary_student_guardian_trigger`,
`update_students_guardians_updated_at`) and its owned sequence
`users.students_guardians_id_seq`; the owner mirror triggers
`student_guardian_relationships_mirror`,
`student_guardian_pickup_permissions_mirror` and
`guardian_student_access_mirror`; the functions
`users.route_students_guardians_compatibility()`,
`users.mirror_student_guardian_relationship()`,
`users.mirror_student_guardian_pickup_permission()`,
`auth.mirror_guardian_student_access()`,
`users.enforce_single_primary_student_guardian()`,
`meta.invalidate_parent_student_consent_permission_grant()` and
`meta.invalidate_meal_participation_permission_grant()`; and the counter
`users.students_guardians_compatibility_writes`.

Kept, because the current image depends on them:

- the owner tables and their indexes, including the one-primary and one-payer
  partial unique indexes on `users.student_guardian_relationships`;
- the owners' `updated_at` triggers;
- the account binding `guardian_profiles_bind_student_access`;
- the grant invalidation triggers on `auth.guardian_student_access` and their
  functions `meta.invalidate_*_access_grant()`;
- the consent and meal-participation grant foreign keys onto the relationship;
- the frozen `guardian-owner` backfill checkpoints and the
  `backfill guardian-owner` command, which refuses to run once
  `users.students_guardians` is gone.

Owner rows are not changed. Mirror data is never copied over owner state.

## Verification

`database/migrations/guardian_owner_contract_test.go` rebuilds the pre-Contract
world from the frozen mirror schema in a disposable clone, runs the real
preflight and migration runner, and checks object removal, unchanged owner
snapshots, retained owner triggers, binding and grant foreign keys, owner
writes and cascades after the Contract, refused Down, lock wait and timeout,
rollback on an unexpected dependency or fingerprint change, and every
integrity refusal, including a standalone pickup or access delete. The caller
inventory rejects the retired names in application code, fixtures and behavior
tests. Older migration tests restore the mirror through
`RestoreGuardianStorageBeforeContract`, and the Expand, Backfill and Cutover
contracts restore the authoritative table through
`RestoreGuardianStorageBeforeCutover`.
