# Staff owner storage Contract (#2754)

Migration `1.15.427` removes the staff compatibility storage that Cutover
`1.15.409` ([#2753](staff-owner-storage-cutover.md)) kept for previous-image
rollback. It follows the [student-owner cleanup](student-owner-storage-contract.md)
and the [request-child cleanup](enrollment-storage-contract-2719.md): one
backend and one frontend per environment, both stopped before migration, no
rolling mixed-version rollout and no observation window.

## Before the release

The issue names three conditions. Check them before the release that carries
`1.15.427`:

1. The cutover shipped in an earlier production release: `1.15.409` is
   recorded in `public.bun_migrations` and the owner tables are in use.
2. Nobody uses the old path. Run the
   [observation SQL](staff-owner-storage-cutover.sql) on the production
   database while the current image serves requests: both
   `compatibility_reads` and `compatibility_writes` must be zero, and
   `broken`, `live_numbers` and `unvalidated_staff_foreign_keys` must be zero.
   The caller inventory (`TestStaffCutoverCallerInventory`) is green in CI.
3. A restorable release backup is verified, not merely present. The release
   sequence below takes and verifies it.

The migration does not read the counters itself. As for the student and
request-child Contracts, historical hits are diagnostic, not a cleanup gate:
the previous image is stopped before the migration runs, and a failure after
the backup restores the complete release.

## Release sequence

1. Pull the new images and run the read-only migration data preflight while
   the old release still serves requests. An integrity failure aborts before
   downtime; preflight neither removes objects nor touches the view or its
   counters.
2. Stop both application services, including the backend's embedded jobs.
3. Create and verify the complete release backup.
4. Run the migration. It takes the staff backfill's advisory lock, rechecks
   integrity under `ACCESS EXCLUSIVE` locks (5 s lock timeout, 60 s statement
   timeout) and removes the compatibility objects in one transaction. Row
   counts and SHA-256 fingerprints of both owner tables per school must be
   unchanged before it commits.
5. Start both new images and verify health. A failure after the backup
   restores the complete saved release.

After the Contract an old image alone is not a rollback: restore the
pre-Contract backup and the prior images together. Down is refused.

## Preflight refusals

- The completed cutover schema is required: `users.staff` a view,
  `users.staff_legacy` a base table, both counters and the routing function
  present.
- A stored function that names `users.staff` or `users.staff_legacy`, a view
  that depends on either (other than the compatibility view itself) and a
  foreign key into the archive. `DROP ... RESTRICT` also refuses any unknown
  dependency.
- Unvalidated foreign keys on or into the owner tables, and owner RLS that is
  not enabled and forced.
- A membership without employment profile or person, a profile without
  membership, a person with two live memberships, and a personnel number held
  by two live staff members of one school.
- A fresh initial replay is identified by the migration runner and verifies
  that the staff storage is empty before cleanup.

## Removed and kept

Removed: the view `users.staff` with its `INSTEAD OF` trigger, the routing
function `users.route_staff_compatibility()`, the archive `users.staff_legacy`
with its owned sequence `users.staff_id_seq`, and the counters
`users.staff_compatibility_reads` and `users.staff_compatibility_writes`. The
`models/users.Staff` DTO no longer names a table.

Kept, because the current image depends on them:

- the personnel-number trigger `staff_employment_profiles_personnel_number`,
  which enforces one live staff member per number and school and raises
  `uq_staff_tenant_personnel_number`;
- the membership index `idx_staff_tenant_person`, whose name School Membership
  classifies a duplicate staff member by;
- the membership `updated_at` trigger;
- the frozen `staff-owner` backfill checkpoints and the `backfill staff-owner`
  command, which refuses to run once `users.staff` is not a base table.

`users.staff_school_memberships` and `users.staff_employment_profiles` are not
changed. Archive data is never copied over newer owner state.

## Verification

`database/migrations/staff_owner_contract_test.go` rebuilds the pre-Contract
world from the frozen archive schema in a disposable clone, runs the real
preflight and migration runner, and checks object removal, unchanged owner
snapshots, retained dependent foreign keys and invariants, refused Down, lock
wait and timeout, rollback on an unexpected dependency or fingerprint change,
and every integrity refusal. The caller inventory rejects the retired names in
application code, fixtures and behavior tests. The Expand, Backfill and
Cutover contracts, and older repairs that wrote `users.staff`, restore the
historical table through `RestoreStaffStorageBeforeCutover`.
