# Student Presence #2691: local facade observation

This records development verification, not a deployment observation. The change
set is based on `ffd354dc2a2f0447e3b9afb98c0a3b99cce7f310`. The final acceptance
section maps the issue requirements to evidence. Earlier audit notes retain the
findings and verification state recorded as the migration progressed.

## Reproduce

From `backend/`, using the repository toolchain:

```sh
CGO_ENABLED=0 ../scripts/run-go-toolchain.sh go test ./modules/studentpresence/compose -run TestStudentPresenceMigrationRuntime -parallel 8 -count=1 -v
```

The test prints environment metadata and JSON samples under
`student-presence-runtime:`. It uses an isolated hermetic database, one
student, device, room and active group, and one attendance/visit pair per
iteration. Fixture deletion is outside measurement. Each flow has five warm-up
iterations followed by 30 measured iterations at concurrency one.

The recorded run used Go 1.27.0 and PostgreSQL 17.11. It ran within the full
Student Presence compose test package, alongside parallel tests. These numbers
are local smoke measurements, not an equivalent-load before/after comparison
or production latency limits. They do not replace checkpoint #3019.

## Flow measurements

| Flow | Queries/sample | p50 ms | p95 ms | DML rows/sample | Rollbacks/30 samples |
|---|---:|---:|---:|---:|---:|
| checkin-checkout | 14 | 4.595 | 11.239 | 4 | 0 |
| rollback-after-visit-close | 21 | 6.181 | 16.772 | 5 | 30 |
| rollback-after-attendance-close | 22 | 5.966 | 6.705 | 6 | 30 |

Each flow recorded 60 committed transactions and 30 prevented duplicate
attendance inserts. Rollback flows injected 30 failures, verified that both
records remained open, and completed the same checkout successfully on retry.
The final school-status read returned `checked_out`.

All flows recorded zero unexpected errors, transaction retries, deadlocks,
pool-wait count and pool-wait duration. DML rows are driver-reported successful
statement effects, including statements later rolled back; they are not net
committed rows. Query counts include transaction setup and outcome statements,
duplicate prevention and verification reads.

Lock sampling inspected PostgreSQL `pg_stat_activity.wait_event_type = 'Lock'`
in the isolated database. The three flows collected 133, 141, 125 samples,
respectively, and observed no waiting backends. Maximum sample gaps were
8.990, 4.970, 2.827 ms. Sampling cannot exclude shorter waits, and its window
includes fixture cleanup between iterations.

## Stable operation measurements

These durations come from the facade observer, not whole-flow timing.
Each observation reported one store query and no error. Transaction-injected
failures appear in rollback counters rather than as fabricated store errors.

| Operation | Samples | p50 ms | p95 ms | Errors |
|---|---:|---:|---:|---:|
| ensure_attendance | 180 | 0.377 | 0.633 | 0 |
| record_visit | 90 | 0.350 | 0.633 | 0 |
| close_visits | 150 | 0.386 | 0.612 | 0 |
| close_attendance | 120 | 0.398 | 0.539 | 0 |
| list_school_statuses | 90 | 0.356 | 0.615 | 0 |
| find_visit | 60 | 0.294 | 0.431 | 0 |
| find_attendance | 60 | 0.298 | 0.378 | 0 |

## HTTP visit-read failure contract

`TestDeviceCheckin_VisitReadFailureDoesNotCreateVisit` exercises the tenant
HTTP check-in route with real fixtures and an injected failure at the current
visit service boundary. Before the fix, that failure produced HTTP 200 and
created a visit. After the fix, the response is HTTP 500 with exactly
`{"status":"error","error":"Internal server error"}`. Public service reads
confirm no open visit and attendance status `not_checked_in`. Restoring the
lookup allows the same scan to succeed and creates one open visit.

The API and check-in service package tests passed with `-parallel 8 -count=1`.
This is failure-contract evidence, not a database-outage or latency measurement.
The kiosk uses its existing generic server-error contract; internal lookup
details are not included in the response.

## Adapter cleanup and rollback

Before removal, searches across all backend Go files found no callers of
`NewAttendanceRepository`, `NewVisitRecords`, or `NewVisitRepository`.
Only constructor definitions remained. The migrated facade tests and local
runtime workload formed the observed window for this development artifact.
The unused compatibility adapters and legacy visit repository were then
removed. The active attendance SQL repository had already been replaced.

Table shapes and tenant ownership are unchanged. Revert the migration change
set as a unit to restore the previous provider and its wiring. Retain the
migration commit and its parent as a rollback pair. No staging or production
tracer window has been observed here; rollout monitoring remains necessary.

## Historical verification and audit notes

The facade suite, repository packages, active/schedule/emergency services,
root service composition and cleanup CLI tests passed after adapter removal.
Lint, full `go vet ./...`, and the architecture check passed. The composition
surface changed from 829 at the base to 823. Further read-projection cutovers
reduced the repository-wide architecture baseline from 2364 at adapter removal
to 2360 legacy violations. These are repository-wide counts,
not a claim that all architecture migration work is complete.

Runtime consumers now use public attendance and visit DTOs, including visit
commands, checkout events and group-visit responses. Shared attendance fixtures
also use the module API. The affected fixture suites and backend test gates
passed after the composition inventory and tenant-isolation fixes. No new
repository-factory call remains in the rollback tests.

Kiosk and web checkout verification now reads attendance and visits through
tenant-scoped presence queries. Statistics, display, slot-list and parent-status
attendance fixtures also write through the module, including explicit-tenant
cases. Their package suites, full vet, scoped lint and backend test gates passed.
The composition inventory refresh changed constructor line numbers only.

The legacy attendance model and its model-only tests are removed. Persisted
field and lifecycle coverage remains in the facade suites. The module status
contract now explicitly covers all four yard/checkout combinations, including
checkout overriding retained yard history. Affected model, API, service,
repository and module suites, backend test gates, full vet and scoped lint passed.

Shared visit fixtures now use `RecordVisit` and return the public visit DTO,
including the explicit-tenant variant. All 18 consuming package suites passed.
The temporary active-service visit fixture was removed in favor of the shared
fixture; active-service tests, backend gates, full vet, scoped lint and the
architecture check passed after that cleanup.
Self-heal and kiosk visit readbacks now use presence queries, and schedule
instance fixtures use the shared visit writer. Five obsolete visit methods were
removed from service mocks. Affected suites and backend gates passed; the
inventory refresh changed only three constructor line numbers.
Active-service failure-injection mocks now use public visit DTOs and location
projections; both legacy conversion helpers are removed. Timeout fixtures use
the shared visit writer. Active-service tests, backend gates, full vet, scoped
lint and architecture checks passed after this migration.

The legacy visit model is also removed. Its validation cases now exercise the
public visit DTO validation and assert that both service write paths reject
invalid data. Facade lifecycle tests retain open/closed visit coverage. Affected
model, API, service, repository and module suites, backend gates, full vet,
scoped lint and the architecture check passed after removal.

The read-error audit found swallowed planned-status failures during check-in.
Single, visit and batch paths now propagate planned-status reads/clears and
student flag-update failures, classified as `ErrDatabaseOperation` at the service
boundary. Fault tests cover both planned-status read paths and the status-write,
student-read and student-write failures. Active service, HTTP/IoT suites, backend
gates, full vet, scoped lint and architecture checks passed. Nine database-backed
single/batch/visit scenarios now inject a read failure, failure after status clearing,
or failure after updating the student. They verify rollback of attendance,
status clearing and the original sickness flag/timestamp, no failed-operation
broadcast, successful retry and no duplicate attendance on repeat. The full
active-service suite, backend gates, vet, scoped lint and architecture checks
passed. Visit-entry cases also verify that failed operations leave no room visit;
successful retry creates one, and a repeat preserves the existing
`ErrStudentAlreadyActive` contract without duplicating attendance or visits.
Live sick/excused helpers now propagate student reads, history upserts/clears
and student-update failures. Four read/update regression tests first reproduced
the swallowed errors; additional tests cover both history-write stages for both
flags and prohibit a following student update. Single, visit and batch callers
classify these failures as `ErrDatabaseOperation`. Affected service, HTTP/IoT
suites, backend gates, full vet, scoped lint and architecture checks passed.
Eighteen database-backed live-flag scenarios now cover both sickness and
excused clearing across single, batch and visit check-in. Faults follow the
history upsert, history clear and student update. They verify rollback of
attendance, visits, all history rows and the original flag/timestamp, no failed
broadcast, successful retry and non-duplicating repeats. These and the nine
planned-status scenarios pass with the service suite, backend gates, full vet,
scoped lint and architecture check.

Clear-mode resolution now uses the settings service's tenant override or registry
default directly. Missing wiring and resolution failures propagate instead of
selecting a consumer fallback; no override-existence probe is required. Six
additional database-backed single/batch/visit scenarios inject sick/excused
setting failures and verify rollback, no broadcast, successful retry and no
duplicates. All 33 status rollback scenarios pass with the active-service,
settings, HTTP/IoT suites and backend gates. Scoped lint reports zero issues;
architecture remains at 2360 legacy entries and 823 composition targets. The
inventory refresh changes only constructor line locations, not callers.

Virtual web-device lookup failures previously became device ID zero, losing the
repository error. Three single/batch/visit fault scenarios reproduced this and
now verify the original error plus `ErrDatabaseOperation`, no persisted attendance
or visits, unchanged planned status and student flag, no broadcast, successful
retry and non-duplicating repeats. Missing virtual devices also fail explicitly.
The resulting 36 status/device rollback scenarios pass with the active-service
suite and backend gates; affected HTTP/IoT suites passed after the production
change. Architecture remains at 2360 legacy entries and 823 composition targets,
without a policy exception for the internal test-only device fault injector.

Visit check-in staff attribution also swallowed supervisor lookup failures.
A database-backed fault first reproduced a successful check-in despite a failed
lookup. The resolver now propagates repository errors while retaining device-only
attribution for the existing typed supervisor-unavailable outcome. The additional
rollback/retry scenario brings this matrix to 37 cases and verifies the recovered
attendance retains its device ID with no staff attribution. Active-service,
HTTP/IoT and backend-gate suites, full vet, scoped lint and architecture passed;
the explicit attribution assertions also passed in a focused rerun.

Presence-mode settings failures also produced successful single/batch/visit
check-ins. Three new database-backed scenarios reproduced that failure; those
entry points now resolve the mode strictly before their writes and propagate the
original error with `ErrDatabaseOperation`. They verify unchanged attendance,
visits, status and flag state, no broadcast, successful retry and no duplicates,
bringing this matrix to 40 cases. Resolver unit cases cover missing wiring,
repository errors, invalid/empty values and both supported modes. Active-service,
settings, HTTP/IoT and backend-gate suites, full vet, scoped lint and architecture
passed. This is a partial caller migration: `GetPresenceMode` still contains its
legacy fallback for checkout, move, cleanup and response-building callers. Those
callers and removal of the fallback remain required before acceptance.

Single and batch school checkout plus visit-only checkout now also resolve
presence mode strictly before writing. Five settings-failure cases first
reproduced successful checkout despite the failed lookup. The expanded ten-case
slot-checkout matrix covers those failures and the existing after-slot-write
failures in binary/detailed single/batch and visit-only flows. It verifies the
original error, database-error classification for settings failures, open
attendance/visits/slots after failure, no broadcast, successful retry and safe
repetition. Active-service, settings, HTTP/IoT and backend-gate suites passed,
as did full vet, scoped lint and architecture. The composition refresh changes
only the line of the existing daily-checkout fixture constructor.
Remaining production fallback callers are the three transit/move gates,
daily-session cleanup, OGS live locations, emergency locations, common/student
response locations and the IoT check-in handler.

The three transit/move gates now use strict presence-mode resolution. A real
assignment test reproduced a successful movement despite a settings failure;
move/transit already aborted later through strict checkout, but their initial
mode checks also no longer substitute a default. Three database-backed cases
verify error identity/classification, unchanged visits, no broadcast and safe
retry/repetition while preserving school attendance. Active-service, active HTTP
and backend-gate suites, full vet, scoped lint and architecture passed. Remaining
fallback consumers are daily-session cleanup and the five external location/
IoT response callers listed above; the public fallback is not yet removed.

Daily-session cleanup now uses strict presence-mode resolution too. Its added
database-backed case reproduced successful closure despite a settings failure;
the fixed path returns a failed cleanup result with the original error and
`ErrDatabaseOperation`, leaving the session and visit open and emitting no
broadcast. Recovery closes the visit once; a repeat preserves its departure
timestamp. Existing after-write rollback coverage remains intact. Active-service,
scheduler, active HTTP and backend-gate suites, full vet, scoped lint and
architecture passed. No active-service production caller now uses the legacy
`GetPresenceMode` fallback. The five external location/IoT callers and the
public fallback itself remain to migrate.

The public `GetPresenceMode` now returns `(string, error)` and the legacy
fallback is deleted. Every production caller propagates failures, including
common location snapshots, OGS live locations, emergency locations, student
response construction and IoT check-in. The student response builder now returns
errors through its four HTTP callers. Tests cover mode failures in each external
consumer; the IoT golden verifies the generic HTTP 500 response, no attendance
or visit writes, and successful retry. Strict resolver tests replace the old
fallback expectations and cover both modes, missing wiring, read errors and
invalid values.

The full backend run exposed a missing-settings CLI fixture and a missing import
in a new test. Both were repaired; affected packages were rerun. The IoT golden
also caught exposed cause text, corrected to the existing generic error wrapper.
Final affected suites, backend gates, full vet, full lint and architecture pass.
The composition diff contains only two existing CLI constructor line shifts;
no exception or caller was added. This is not a claim of a fresh all-green full
suite run after the last edit. Remaining read-error and full acceptance audits
still apply.

The student response location resolver also swallowed attendance, visit and
active-group read failures, presenting absence or transit as if the reads had
succeeded. Three regression cases reproduced those errors; they now propagate
through the response builder. Wrapped `ErrVisitNotFound` and a nil current visit
remain normal transit outcomes, with explicit coverage. The student API suite,
backend gates, full vet, scoped lint and architecture passed after this change.

## Two-table isolation coverage

The acceptance audit inspected the following tests in
`backend/modules/studentpresence/compose`:

| Evidence | Verified boundary |
| --- | --- |
| `TestAttendanceFacadeRejectsForeignReferencesAndCannotModifyForeignRows` | Attendance rejects foreign student references; foreign find/list/lock/existence/close/delete/revise attempts cannot expose or change the row. |
| `TestVisitFacadeCannotReadLockOrModifyAnotherTenant` | Foreign visit reads, location/occupancy projections, locking, close/delete/revise operations cannot expose or change the row; missing tenant and unscoped locking fail. |
| `TestVisitFacadeKeepsHostingSchoolBoundaryForCrossSchoolStudents` | Holiday visits intentionally support a foreign student while retaining the hosting school's row ownership and rejecting a foreign target group. |
| `TestPresenceQueriesAndCommandsRespectTwoTenantRLS` | Combined presence reads/locks/close preserve isolation across both named tables. |
| `TestPresenceTablesEnforceRLSWithoutTenantPredicates` | Direct SQL under a non-superuser, non-bypass role sees the owning row only; foreign UPDATE/DELETE affect zero rows, and the foreign tenant can still read its row afterward. |

The last test was added because facade tenant predicates alone do not prove
database RLS enforcement. Its SQL assertions live in shared database test support
(`backend/test/row_isolation.go`), not in the facade boundary. Provider and backend
gate suites, full vet, scoped lint and architecture pass with no added exception.
This proves two-table read/update/delete isolation alongside the facade contract
tests; it does not replace the remaining contract and authoritative-write audit.

## Authoritative-write audit: attendance commands

`TestAttendanceCommandsRollbackFacadeWritesAndRetry` injects immediately after
the single-student attendance insert and close statements, verifies rollback
and no broadcasts, then verifies successful and non-changing repeated actions.
The added `TestBatchAttendanceCommandsRollbackEveryRowAndRetry` covers the batch
counterparts. It confirms two returned row changes before each injected failure,
no retained inserts or closed attendance after rollback, no refresh broadcast,
successful retry for both students and unchanged repeated results without
duplicate rows. This closes the batch-statement gap that later visit/slot fault
tests did not pinpoint. The active-service suite, backend gates, full vet,
scoped lint and architecture pass.

The inspected provider tests also cover visit insert/close rollback and completed
history, transfer/bulk-close rollback and retry, and visit restoration rollback
on a mismatched snapshot. The complete maintenance/write inventory and its
caller mapping remain open; these inspected cases are not an exhaustive claim.

The stale-attendance maintenance audit found an unguarded update: a retry could
overwrite a completed checkout. `TestStaleAttendanceClosureRollsBackAndDoesNotRewriteDeparture`
reproduced changed departure and update timestamps on repetition. The provider
now updates only rows whose `check_out_time IS NULL`. The test proves rollback
after the authoritative update, successful retry, then zero changed rows and
unchanged timestamps on a later repeat. The cleanup caller already skips zero-row
results, preserving its counters. Provider, active-service, CLI and backend-gate
suites, full vet, scoped lint and architecture pass. Retention deletion and its
audit-write coverage still require inspection.

Retention coverage was found in `retention_presence_test.go`, including the
existing after-audit-append rollback case. That test now also injects immediately
after the actual visit DELETE. Both cases prove the expired visit is restored,
no audit survives, committed-deletion counters stay zero, successful retry
deletes one row and creates one audit, and repetition changes neither count.
Additional assertions preserve an open visit and a recent completed visit and
verify the audit's deleted-record count. Real write counters prove each fault
occurs after its named statement. Active-service and backend-gate suites, full
vet, scoped lint and architecture pass. No production change was required for
this transaction boundary.

Recovery now has a service-level fault after the complete snapshot restoration:
`TestInstance_Reopen_RollsBackRestorationAndRetries`. It proves the instance
and group remain completed, the same visit remains closed, and no broadcast
escapes the aborted transaction. Retry restores the visit; a repeated reopen
preserves the existing invalid-transition contract and does not duplicate it.
The history assertion uses the group visit query because the student visit
query intentionally returns only open visits. It also compares the completed
supervisor and both timetable-assignment rows before and after the aborted
restore, including their persisted fields. The schedule suite, backend gates,
scoped lint and architecture pass with these assertions. Injection after each
individual internal recovery statement remains a separate coverage gap.

The subsequent full-suite run exposed a pre-existing fixture collision in
`TestRequestSharingCurrentUsesAppendOrderNotTransactionTimestamp`: hardcoded
recipient IDs could equal the generated author ID. Real account fixtures now
preserve the append-order assertion without violating the no-self-sharing
constraint. The care-exit integration fixture also explicitly supplies binary
presence settings so it reaches the unchanged care-exit refusal assertion.

The next full run passed the privacy-event test but exposed the slot check-in
rollback fixtures after their 23:59 end time. Both tests now pin the existing
service clock inside the slot and assert that the stored timestamp matches it.
Batch check-in uses `s.now()` like single check-in; its default remains the real
clock. Active-service tests pass after this repair. The composition inventory
changes only reflect shifted call lines. The latest full-suite result is still
a failure from before this repair, not a final green verification.

The full run after the clock repair reached a different blocker: student-list
and OGS-live query budgets measured 33/32 and 42/41 statements respectively.
Both counts were constant for 3 and 10 students, so this evidence shows fixed
budget overruns rather than per-student growth. The run occurred on Monday,
2026-09-07, following the earlier Sunday runs; whether the weekday accounts
for the extra statement has not been proven. Budgets remain unchanged.
The snapshot-consumer audit also found that `LoadStudentDataSnapshot` still
logs and suppresses location-load failures for the student list and export.
That error path remains unresolved and prevents claiming no swallowed reads.

Both findings above are now repaired. The snapshot loader propagates person,
group and location errors and returns no partial snapshot; list/export return
HTTP 500 when the presence read fails. Loader tests preserve the injected cause,
and route tests prove both requests reach the failed attendance read. Narrow
person/group read interfaces remove two additional legacy application imports
without adding a policy exception (2358 remaining entries).

Student-list settings now resolve the booking-mode, presence-mode and photo
keys together, while OGS-live includes booking mode in its existing batch.
Batch-resolution failures return errors without a retry/fallback read chain.
The unchanged query budgets pass at 31 statements for student list and 41 for
OGS-live, each constant between 3 and 10 students. This proves the fixed
overruns are resolved; it does not establish a weekday cause for the earlier
overruns. After these repairs, the full backend suite passed with
`go test ./... -p 4 -parallel 8 -count=1`, including backend gates and both
end-to-end packages. Full vet, full lint (zero issues), architecture and
`git diff --check` also passed. Commands used the repository toolchain with
`CGO_ENABLED=0`; raw local logs are `/tmp/moto-2691-snapshot-full-tests.log`,
`/tmp/moto-2691-snapshot-full-vet.log`,
`/tmp/moto-2691-snapshot-full-lint.log` and
`/tmp/moto-2691-snapshot-final-architecture.log`.

## Review fixes

The two-axis review found no actionable standards violation and two spec gaps:
recovery did not inject after every write, and schedule conflict detection
suppressed presence-read errors. Follow-up review confirmed both fixes:

- Recovery tests seven stages: AFTER UPDATE of the active group, visits,
  supervisor, first assignment, second assignment and activity instance, plus
  failure after the complete restore. Each stage owns an isolated database
  clone. Targeted row triggers prove the real write was reached, then raise a
  database error. All completed state is compared after rollback, no broadcast
  escapes, retry succeeds, and repeat reopen retains its invalid-transition
  response without duplicating visits.
- `DetectStartConflicts` propagates failed student/presence reads as
  `ScheduleError` with the original cause. Manual start performs no writes or
  broadcast and can retry. Automatic start increments its failure count and
  returns the error, then succeeds on retry. Detected conflicts remain advisory
  for manual starts and continue to skip automatic starts.

## Acceptance audit

| Requirement | Evidence |
| --- | --- |
| Prerequisites | GitHub reports #2655, #2662, #2665, #2668, #2684 and #2685 closed, rechecked 2026-09-07. |
| One owner facade/provider | `modules/studentpresence` public Query/Command contracts delegate to its internal application/Postgres provider. Migrated entry points and composition use public attendance/visit types. |
| Old-provider removal | Backend Go search finds no `NewAttendanceRepository`, `NewVisitRecords`, `NewVisitRepository` or legacy attendance/visit DTO references. Old providers, models and factory fields are deleted. Pre-removal search and development observation are recorded above. |
| Ownership and ratchet | Compared with the base, `owners`, `data_objects` and `read_projections` are identical. All five issue keys are removed; no new violation key exists. Seventeen keys are removed in total, leaving 2358; composition targets shrink 829 → 823. |
| Atomic writes | Owner commands reuse the ambient tenant transaction; service/HTTP/worker transaction tests below abort actual writes, inspect persisted state, and retry. No application dual write to a legacy presence provider remains. |
| Stable errors | Presence/settings/read failures propagate through service, HTTP and automatic-start paths. Tests assert error causes or established HTTP failure envelopes; failed mutations do not publish events. Snapshot list/export failures return 500 rather than partial absence data. |
| Table and RLS behavior | Attendance/visit facade tests cover own/foreign reads and writes. `TestPresenceTablesEnforceRLSWithoutTenantPredicates` verifies least-privilege SELECT/UPDATE/DELETE isolation without explicit tenant predicates; foreign rows remain visible to their owning tenant. |
| Runtime metrics | Tables above record queries, p50/p95, stable-operation errors, pool waits, sampled lock waits, DML rows, rollback count, deadlocks/retries and duplicate conflicts. Ordinary-wave evidence references accepted checkpoint #3019; this is not a new full checkpoint or deployment measurement. |
| Rollback | Table shape is unchanged. Revert the migration commit as a unit to restore the parent provider and composition; preserve that commit/parent pair. No push or deployment is part of this implementation. |

### Authoritative-write coverage

Paths below are relative to `backend/`. Tests retain real provider writes and
inspect database state outside the failed transaction.

| Flow / writes | Fault and retry evidence |
| --- | --- |
| Single/batch attendance insert and checkout | `services/active/presence_command_rollback_test.go`; batch cases verify both student rows. |
| Visit creation plus attendance | `services/active/presence_create_rollback_test.go`; `presence_visit_slot_checkin_rollback_test.go` adds each timetable mirror write. |
| Visit/attendance close plus timetable mirror | `presence_close_rollback_test.go`, `presence_slot_checkout_rollback_test.go`; completed departure timestamps survive repetition. |
| Revision and deletion | `presence_revision_rollback_test.go` faults each visit/attendance/mirror write; `presence_delete_rollback_test.go` preserves repeated-delete not-found behavior. |
| Move and transfer | `presence_move_rollback_test.go`, `presence_transfer_rollback_test.go`, `modules/studentpresence/compose/visit_transfer_test.go`. |
| Status history and flags during check-in | `planned_status_checkin_rollback_test.go` covers single/batch/visit entry, planned/live sick/excused writes and required reads/settings/attribution. |
| Daily and stale cleanup | `presence_bulk_close_test.go`; `modules/studentpresence/compose/attendance_test.go` verifies stale rollback/retry and immutable completed checkout. |
| Retention delete and audit append | `services/active/retention_presence_test.go` faults each statement and preserves open/recent history and audit counts. |
| Care exit: attendance and visits | `modules/studentpresence/compose/care_exit_test.go` faults both writes and verifies rollback plus idempotent retry. |
| Recovery: group, visits, supervisor, two assignments, instance | `services/schedule/instance_service_integration_test.go:TestInstance_Reopen_RollsBackRestorationAndRetries` tests every boundary and final restoration. |

### Public contract evidence

These tests use explicit expected-value assertions rather than requiring
separate snapshot files for every mechanical caller migration.

| Surface | Evidence |
| --- | --- |
| HTTP check-in/out | `api/active/checkin_test.go`, `checkout_integration_test.go`: real routes, repeated checkout and unchanged history semantics. |
| HTTP visit representation | `api/active/visit_presence_contract_test.go`: IDs, status/group filters, checkout timestamp, message and tenant-field omission. |
| IoT scan and checkout | `api/iot/checkin/checkin_test.go`, `attendance_test.go`: successful scans, room transfer, capacity rollback, repeated scans, attendance toggles and daily checkout. |
| IoT failure envelope | `api/iot/checkin/visit_read_failure_test.go`: exact generic 500 JSON, no presence mutation, successful retry. |
| Student list/export | `api/students/snapshot_read_failure_test.go`: both routes return 500 on failed presence reads. List/OGS query budgets pass at 31/41 statements for 3 and 10 students. |
| Worker and CLI | Daily-session, retention and automatic-start tests plus `cmd/presence_cleanup_test.go`: failed-school reporting, tenant isolation, commit-only publication and retry. |
| Module/location semantics | Attendance/visit facade tests, `compose/school_status_test.go`, OGS-live status/planning tests: latest stay, yard/checkout precedence and public ID serialization. |

### Final verification

The final post-review full backend suite passed using
`go test ./... -p 4 -parallel 8 -count=1`, including architecture/unit gates and
both end-to-end packages. Full `go vet ./...`, full
`golangci-lint run --timeout 10m` (zero issues), the architecture ratchet and
`git diff --check` passed. Go commands used the repository toolchain with
`CGO_ENABLED=0`. Final raw local logs are
`/tmp/moto-2691-final-full-tests.log`, `/tmp/moto-2691-final-full-lint.log`,
`/tmp/moto-2691-reviewed-vet.log` and `/tmp/moto-2691-reviewed-architecture.log`.

The commit hook additionally required `goimports` grouping in 38 files. A
comparison with the staged versions confirmed these were import-block-only
changes. The composition refresh changed six existing call line numbers only;
the architecture ratchet passed again after formatting.

Standards review: zero confirmed findings. Spec review: both confirmed findings
fixed and re-reviewed. Review coverage is the migration core and representative
callers, complemented by the full suite and architecture checks, not a claim
that tests prove every possible behavior.
