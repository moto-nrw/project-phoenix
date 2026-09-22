# Request-child storage cleanup (#2719)

Migration 1.15.414 removes the request-child compatibility storage that
Cutover 1.15.385 (#2714) retained for previous-image rollback. It follows the
[student-owner cleanup](student-owner-storage-contract.md) policy: one backend
and one frontend per environment, both stopped before migration, no rolling
mixed-version rollout, no observation window, operator evidence file or zero
historical compatibility-counter value required.

## Normal release sequence

1. Pull the new images and run the read-only migration data preflight while
   the old release still serves requests. An integrity failure aborts before
   downtime; preflight neither removes objects nor checks old-access counters.
2. Stop both application services, including the backend's embedded jobs.
3. Create and verify the complete release backup.
4. Execute migration. It rechecks integrity under bounded locks and removes
   the compatibility objects in one transaction. Owner row counts and SHA-256
   fingerprints per school must be unchanged before it commits.
5. Start both new images and verify health. A failure after backup invokes
   complete rollback to the saved release.

After successful cleanup an old image alone is not a rollback: restore the
complete matching backup and prior images together. Automatic Down is refused.

## Retained protections

- The completed cutover schema (view, archive, routing function, counters)
  is required; an older or partially cleaned schema fails preflight.
- Stored functions referring to the retired names, views depending on them,
  and foreign keys into the archive fail preflight. DROP RESTRICT prevents
  removal of unknown dependent objects.
- Owner foreign keys must be validated, owner RLS enabled and forced, and
  every booking and selection must reference an existing request child and
  care offering of the same school.
- The transaction has a 60-second bound and a 5-second lock timeout.
- A fresh initial replay is identified by the migration runner and separately
  verifies that request-child storage is empty before cleanup.

## Removed objects and preserved data

Cleanup removes the view `enrollment.request_child_offerings`, the archive
`enrollment.request_child_offerings_legacy` with its owned id sequence, the
routing function and INSTEAD OF trigger, both compatibility normalization
functions and both hit counters. The compatibility repair CLI
(`migrate backfill request-child-compatibility`) and the
[observation SQL](enrollment-storage-cutover-2714.sql) no longer apply and were
retired with it.

`enrollment.request_child_offering_selections`, `enrollment.care_offering_bookings`
and the frozen backfill checkpoints remain unchanged. Archive data is never
copied over newer owner state.

## Verification

Migration tests rebuild the pre-Contract world from the frozen historical
table in a disposable clone, run the real preflight and migration runner, and
check object removal, unchanged owner snapshots, refused Down, lock timeout,
transactional rollback on an unexpected dependency or fingerprint change, and
every integrity refusal. The caller inventory rejects the retired names in
application code, fixtures and behavior tests; the Expand, Backfill and Cutover
contracts restore the historical table through `RestoreRequestChildStorageBeforeCutover`.
