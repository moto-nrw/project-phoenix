# Student-owner storage cleanup (#2760)

## Deployment policy (revised 2026-09-20)

Migration 1.15.399 removes obsolete student compatibility storage during the
ordinary staging-first deployment. There is one backend, including its jobs,
and one frontend per environment; both are stopped before migration. This is
not a rolling mixed-version rollout. No observation window, school-day wait,
operator evidence JSON, query-statistics configuration or zero historical
compatibility-counter value is required. The evidence CLI flag and its optional
deployment mount have been removed, not bypassed or silently ignored.

The historical read/write sequences record past compatibility activity. A
nonzero value does not identify a consumer of the new release and must not
permanently block unrelated releases. Migration does not reset them to pretend
there was no activity: it removes them with the obsolete compatibility objects.

## Normal release sequence

1. Pull the new images and run their read-only migration data preflight while
   the old release still serves requests. A data-integrity failure aborts before
   downtime; preflight neither removes objects nor checks old-access counters.
2. Stop both application services, including the backend's embedded jobs.
3. Create and verify the complete release backup: database, roles, uploads,
   configuration and matching image identities.
4. Execute migration. Recheck integrity under bounded database locks, replace
   the stored deletion-count function and remove the compatibility objects in
   one transaction. Verify unchanged owner fingerprints before committing.
5. Start both new application images and verify health. A failure after backup
   invokes complete rollback to the saved release.

Use the deployment pipeline, not a direct migration invocation on a running
environment. The migration CLI does not stop services or create backups. After
successful cleanup, an old image alone is not a rollback: restore the complete
matching backup and prior images. Automatic Down remains refused.

## Retained protections

- Completed cutover and Care Plan absence schema are required.
- Owner foreign keys, RLS, guardian reconciliation, owner links and dependency
  checks remain mandatory before deployment and again under locks.
- Unexpected stored functions referring to retired storage fail preflight.
  DROP RESTRICT prevents removal of unknown dependent objects.
- The transaction has a 60-second bound and a 5-second lock timeout. Owner
  row counts and SHA-256 fingerprints must remain unchanged per school.
- Fresh initial replay is identified by the migration runner and separately
  verifies that student storage is empty before cleanup.

## Removed objects and preserved data

Cleanup removes users.students, users.students_legacy, the compatibility routing
function/trigger, the archive's owned sequence, both compatibility counters and
users.expired_privacy_consents. The stored visit-count function is first rebound
to the current owner tables.

People Directory profiles, School Membership rows and Care Plan rows remain
unchanged, including sick, sick_since, excused and excused_since. Status-day
history is unchanged. Archive data is never copied over newer owner state.

## Verification and historical evidence

The ordinary-upgrade regression uses a populated pre-cleanup database with 566
historical reads and 12 historical writes. It exercises the real preflight and
migration runner, checks actual object removal and function replacement, and
compares all owner/history snapshots. Failure-path tests retain unknown
functions/views, integrity refusal, lock timeout and transactional rollback.
The application caller inventory rejects references to retired student storage;
normal backend behavior tests run against the already-cleaned schema.

Release-script tests cover preflight, application stop, complete backup,
migration, startup and full restore. They enforce stop/backup-before-migration
ordering without any operator evidence file.

The artifacts under [historical verification](evidence/student-owner-contract-2760/)
record the original implementation and local recovery experiments. References
there to mandatory or optional evidence, zero counters and waiting periods are
historical, not current deployment requirements. Their measurements are not
claims of a new staging or production rollout. Optional old-access monitoring
remains diagnostic only and is not required to deploy.
