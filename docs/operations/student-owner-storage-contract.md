# Student owner storage Contract (#2760)

## Current deployment policy (revised 2026-09-20)

Migration **1.15.399** runs during ordinary deployments, including upgrades of
existing databases. The user replaced the mandatory 24-hour/school-day waiting
policy with the normal staging-first release flow. No evidence JSON is required.
This is actual cleanup, not a skipped migration: the stored visit-count function
is rebound to owner storage and the old compatibility objects are removed.

The technical gates remain mandatory: zero compatibility counters, valid owner
links and RLS, reconciled guardian data, no unsupported function/dependency
references, bounded locks, and unchanged owner fingerprints across the atomic
transaction. Preflight checks the live database before stopping the old release;
execution repeats the checks under the DDL locks. New empty installations retain
their separate empty-replay verification.

Use the normal deployment pipeline: it stops the app, creates and verifies the
complete release-backup bundle, and only then migrates. Failures restore that
bundle and its matching previous image. A direct `migrate` invocation does not
create a backup; do not use it to bypass this deployment sequence. After a
successful destructive cleanup, an old image alone is not a rollback.

An explicitly supplied `--student-contract-evidence` file remains optional and
is still strictly validated, including its original observation-window policy.
Malformed or stale supplied evidence is not silently ignored. Without that flag,
there is no observation-window or school-day gate. Source review and the caller
inventory cover repository application code, not unknown external consumers.

The following evidence descriptions and historical verification results document
how the optional evidence path and the cleanup were tested. They do not restore
the superseded mandatory waiting policy. This change prepares a release; it does
not itself execute a staging or production deployment.

### Operating evidence

`school_day_start` and `school_day_end` are actual instants covering the complete
operating day, wholly inside `window_start`/`window_end`. They must share a Berlin
weekday. `school_day_evidence` must reference the reviewed school calendars,
complete opening/operating hours and observations for every affected school;
holidays, closures and a partial afternoon do not qualify merely because the
date is a weekday. `jobs_evidence` must identify all relevant application and
external jobs, their executions inside the same window, and zero old access.
These are reviewed operator assertions like the backup/caller artifacts, not
facts inferred from a date or an unverified filename.

Evidence and backup freshness are each capped at 24 hours as technical limits,
not additional waiting periods. The backup must follow the observed endpoint
and its exact contents must have been restored successfully. Refresh stale
endpoint evidence while preserving the original continuous observation window.
Live database identity, counters, tracking configuration and query fingerprint
are checked again immediately before DDL. No production action is authorized
by this implementation.

## Storage boundary

Contract removes `users.students`, its routing function and trigger,
`users.students_legacy` with its owned ID sequence, and the two compatibility
counter sequences. It also removes `users.expired_privacy_consents`: this
caller-free legacy view still reads guardian copies from the archive and
therefore directly depends on the storage being retired. The final caller
inventory must include this view, not only `users.students`.

The three owner tables remain unchanged, including Care Plan's `sick`,
`sick_since`, `excused`, and `excused_since`. The archive may lag legitimate
owner updates. Contract must not copy archive state back or require live
absence flags to match today's dated history. Guardian values are reconciled
against linked contacts using the existing backfill normalization; no guardian
copies are created.

## Transactional checks implemented

1. Reuse the backfill/repair advisory lock. Bound the transaction to 60 seconds
   and lock acquisition to 5 seconds. Lock the compatibility view and owner
   storage against writes and guardian storage against concurrent reconciliation
   changes.
2. Require the compatibility schema and all four Care Plan absence columns.
   Inspect sequence state without selecting the compatibility view. Any
   compatibility hit fails the database gate.
3. Refuse unvalidated owner FKs, archive-referencing FKs, disabled owner RLS,
   orphaned memberships/care rows, live memberships without care, duplicate
   live memberships for one profile within a school, or
   unreconciled guardian copies.
4. Capture all-column row counts and checksums by owner table and school.
   Use `DROP ... RESTRICT` in one transaction. Unexpected dependencies abort
   and restore earlier DDL. Recheck the owner fingerprints before commit.

The operating-evidence validator separately rejects missing measurements,
nonzero old callers/access counts, short or stale observations, statistics
resets/evictions, and missing or inconsistent backup/restore evidence. The
evidence-bearing transaction entry point checks the database name, database
OID, PostgreSQL system identifier, compatibility counters and query-statistics
epoch/evictions before locking and again inside the DDL transaction. When
supplying evidence, the caller must supply trusted executing-release metadata;
it must match the inventory's full commit. Operator assertions still need
raw-artifact review and deployment script wiring. Ordinary migration execution
uses the same transactional data checks without the optional evidence validator.

`scripts/deploy-remote.sh` accepts an optional second positional argument, an
absolute path to the reviewed evidence file (the first is the deployment
directory). It snapshots the file in a private temporary directory and binds
the same container-readable, read-only copy into both `migrate preflight` and
`migrate`. Missing/unreadable/empty input aborts before pulling or stopping.
Cleanup removes the temporary copy on success or failure. The operator must
retain the reviewed original with its raw source artifacts. The normal CI
invocation supplies no evidence and performs Contract using the mandatory live
data checks and normal release-backup sequence. Explicit evidence is optional;
there is no implicit file discovery.

All 23 release backup/deployment shell tests passed after this wiring,
including mutation of the original file after preflight: both phases still
read the initially reviewed bytes. `bash -n`, `scripts/env-check.sh` and
`git diff --check` passed. These are simulated Docker boundary tests, not a
production deployment.

The live gate also reads the successful `001015397` migration timestamp from
`public.bun_migrations`. Evidence must not date the cutover before that
timestamp; a missing migration record fails closed. Use that runner timestamp
or a later conservative observation boundary, not the slightly earlier
in-transaction backfill checkpoint timestamp. This binds the window to the
connected database rather than trusting the evidence file's date alone.
The regression test first reproduced acceptance of a cutover assertion before
the recorded boundary. After the fix, `TestStudentContract(Live|Evidence)`
passed on the ordinary local service (5.891s) and the separate preloaded
PostgreSQL instance (2.467s). The latter exercised successful destructive DDL
only on its disposable test clone. Its synthetic policy uses one nanosecond
to test evidence binding without an artificial operational waiting period;
it is not a production policy or a real backup/restore proof. Architecture
checks passed with 667 composition targets and 682 existing legacy findings.

The operational evidence JSON reader requires one object, rejects unknown
fields/trailing documents and invalid numeric field types, and caps input at
1 MiB. Tests cover these rejection paths and a complete round trip. Operating
policy is intentionally not accepted as part of the observation document;
parsing alone does not authorize execution or choose the agreed rollback
duration. Both `migrate` and `migrate preflight` now accept
`--student-contract-evidence /path/to/evidence.json` and carry the parsed
document into the migration context. They reject an evidence/build commit
mismatch before opening a database. When supplied, the registered precondition
and Up consume that context and apply the same optional
24-hour/school-day/jobs policy.

The first full backend run on the contracted schema found one real runtime
dependency that the Go caller inventory could not see:
`active.count_student_visits_for_deletion` stored a string SQL body referencing
the compatibility view. PostgreSQL's `DROP RESTRICT` did not protect this
dependency. Contract now replaces that function transactionally with the
equivalent live profile/membership/care joins, preserving its tenant guard,
SECURITY DEFINER mode, search path and execution grants. The cross-tenant
deletion workflow regression passed (6.553s); the complete affected student
API, user service, presence service and student-integration packages then
passed (17.810s, 10.363s, 2.666s and 1.640s respectively).

Preflight also inspects stored function bodies for explicit qualified names
of the retired relations, excluding only the routing function being dropped
and the visit-count function being replaced. A regression first demonstrated
silent acceptance of an additional SQL-function caller. This lexical check
does not prove absence of dynamically assembled SQL or external consumers.
Previous-image application compatibility remains tested on an isolated clone
with frozen 1.15.398 SQL definitions; current application tests assert that
rollback objects are absent. Remaining ORM-bound fixture queries were moved
to the corresponding owner tables without changing their assertions.

After these fixes, the full backend command
`scripts/run-go-toolchain.sh go -C backend test -p 10 -parallel 8 ./...`
exited successfully on the contracted schema. Go reused valid cached package
results; the affected application packages and focused migration regressions
also ran explicitly with `-count=1` as recorded above. The stored-function gate
and other focused Contract tests passed in 13.549s. The architecture check
passed at 667 composition targets / 682 existing legacy findings, and the
pinned `golangci-lint run --timeout 10m` reported `0 issues.` This does not
complete backup/image recovery evidence or settle the operating duration.

## Durable local metrics and database recovery

The later [durable-server cardinality run](evidence/student-owner-contract-2760/durable-cardinalities.txt)
used PostgreSQL 17.11 aarch64 with `fsync`, `synchronous_commit` and
`full_page_writes` all enabled. Observed Contract core duration: 98.609375 ms;
26 BUN query events (batches count once); pool-wait count/duration deltas 0/0s;
database deadlock counters 0 before and after. The original row-preservation
and retired-relation assertions passed. This measures the core, not the full
deployment or an approved operating window.

The [deletion workload](evidence/student-owner-contract-2760/deletion-runtime.txt)
ran 30 samples each of preview and execute after five warmups on that server.
Nearest-rank p50/p95: preview 4.011958/4.82925 ms; execute
10.458625/12.717 ms. Query events were 22/54 respectively; all pool-wait
samples and the deadlock delta were zero. The test passed in 7.550s.

A separate [real dump/restore exercise](evidence/student-owner-contract-2760/recovery.md)
preserved the synthetic pre-Contract database's six table fingerprints,
absence fields, RLS, validated owner FKs and readable rollback view. The actual
prior server image subsequently started against that restore, returned HTTP
200 on localhost health and left those fingerprints unchanged. This is a
backend startup smoke, not production backup freshness evidence or a complete
authenticated/frontend recovery test. The schema-3 record is
`backend/architecture/student-owner-contract-2760.json`; it explicitly records
these acceptance limits rather than inheriting a historical-gap exception.
The evidence validator passed against immutable base
`4aa36c315fc19e8ae8dda50193ffa34843e25575` (15 inventory files, no ratchet-key
removals). [Source hashes](evidence/student-owner-contract-2760/durable-source-sha256.txt)
identify the dirty-worktree implementation and test files used for the durable
measurements; this is not a claim that the base commit contains the changes.

The runner permits an initial replay only when it starts with no applied
migrations and no tables or views in the `users` schema. Before the destructive
DDL, under the same transaction locks, it also requires the archive and all
three owner tables to be empty. Empty student tables on an already initialized
database do not qualify. A local fresh bootstrap and all focused
`TestStudent(Contract|OwnerContract)` tests passed (8.663s), including missing
evidence, unconfigured policy, unexpected data during an initial replay, and
the registered migration's final schema. Historical tests explicitly restore
the pre-Contract schema rather than retaining it in the current template.
After correcting the historical repair/absence/guardian test arrangements,
the entire `database/migrations` package passed against the contracted default
template (30.167s). Focused runner/CLI tests passed (3.159s), as did the hermetic
fixture gate (6.986s). This is not yet a full backend-suite result.

The backend image build requires the full lowercase commit as the
`RELEASE_COMMIT` build argument. The build workflow passes `github.sha`; the
linker embeds it in the binary. It is not a runtime environment variable and
does not add a SOPS key. Unversioned local binaries may still run ordinary
migration commands, but cannot accept a Contract evidence file. Focused CLI
tests passed (5.878s), including malformed evidence, unknown policy fields,
missing build identity and mismatched release identity. `scripts/env-check.sh`
passed. A local production-Dockerfile build subsequently passed. The image
embedded the base SHA as a synthetic identity for a dirty-worktree build check,
not as deployable release provenance. With networking disabled, mismatched
evidence failed before database configuration; matching evidence passed the
identity check and then refused the deliberately absent `DB_DSN`. See
[build verification](evidence/student-owner-contract-2760/build-verification.md).

The local test server installs `pg_stat_statements` but does not preload it.
The live reader correctly refuses this setup rather than interpreting missing
statistics as zero use. Unit cases cover mismatched database/cluster/OID,
release, counters, reset and eviction; a transaction test confirms a failed
recheck preserves both rollback relations. `TestStudent(Contract|OwnerContract)`
passed. The successful live path was subsequently verified in a separate local
PostgreSQL 17 instance bound to `127.0.0.1:5531`, with
`shared_preload_libraries=pg_stat_statements`. No shared test service or
production settings were changed. The complete
`TestStudentContractLiveStateRequiresOperationalStatistics` passed there in
2.612 seconds: it reads real cluster/database/statistics identity, validates
matching synthetic operator evidence, runs the evidence-bearing Contract
entry point including its transactional recheck, verifies both retired
relations disappeared, and compares owner rows before/after. The ordinary
test-service run still verifies refusal when the extension is not preloaded.
This small-fixture check does not establish production-shaped timings, backup
recovery, the production acceptance period, or authenticity of operator files.

## Local verification, 2026-09-20

Observed on disposable per-test PostgreSQL clones, not production:

```text
scripts/run-go-toolchain.sh go -C backend test ./database/migrations \
  -run '^TestStudent(OwnerContract|ContractEvidence|OwnerCutoverRefusesUnreconciledGuardianValues)' -count=1
PASS (final run: 7.760s)

scripts/run-go-toolchain.sh go -C backend test ./test \
  -run '^TestHermeticTestPatterns$' -count=1
PASS

scripts/backend-architecture.sh check
PASS: 682 existing legacy findings; composition 667 -> 667
```

The tests observed successful removal of the listed objects with all owner
rows and dated status rows unchanged; preservation of live absence values
that differ from the archive; rejection of compatibility hits, missing care,
unvalidated/archive FKs and unreconciled guardian copies; cancellation before
DDL; and transaction rollback when an unexpected view depends on the archive.
The historical cutover guardian-reconciliation test also passed after sharing
its SQL normalization with Contract.

These small fixtures do not establish production-scale duration, lock-wait
behavior, full public-contract parity, restore recovery or a completed
schema-3 migration evidence record.

### Caller and fixture verification

The existing `TestStudentCutoverCallerInventory` no longer exempts the old
`users.Student` table tag. Its matcher also covers the archive, dependent
privacy view, counters, quoted identifiers and unqualified old-table SQL.
The old storage binding was removed; the DTO still carries API/service data
and supports scans of explicit projections. The test observed **2,830
application Go files and 85,114 string literals**, with zero detected old
storage callers or compatibility DTO tags. This lexical AST inventory does
not establish the absence of dynamically constructed SQL or external callers.

Shared fixture creation now inserts the profile, membership and care rows
atomically without the compatibility view. Shared group/lifecycle fixture
updates use the live membership's `student_profile_id`, not its independent
primary key. Only explicitly restored historical base-table schemas use the
old fixture insert/update. The fixture test first failed with three
compatibility writes, then passed with zero reads, writes and archive rows.
It uses distinct profile/membership IDs to exercise the identity boundary.

Observed checks after these changes:

- `./test`: fixture and hermetic gates passed.
- `./models/users`, `./database/repositories/users`,
  `./database/repositories/studentintegration`, and
  `./modules/peopledirectory/compose`: tests passed. The integration package
  includes owner operations with the compatibility view removed, tenant
  isolation, rollback and enrollment/lifecycle behavior.
- Focused historical student backfill, cutover and Contract migration tests
  passed after fixture insertion changed. These early results were later
  superseded by the full contracted-schema suite recorded above.
- Architecture check: passed, 682 existing findings and composition 667 -> 667.

## Historical verification log

### Individual test migration progress

The converted student API tests and the education, user services, enrollment
services and timetable test queries now address the owning tables. Membership queries resolve student
IDs through `student_profile_id` and the live membership, rather than assuming
membership and profile primary keys are equal. Enrollment approval rollback
snapshots include all three owner tables, including Care Plan, so replacing the
old joined view does not reduce rollback coverage.

Full package runs passed after these changes (`-parallel 8 -count=1`):

- `./api/students` and `./services/education`.
- `./services/enrollment` and `./services/users`.
- `./modules/timetable/legacy/timetableplanning` and
  `./modules/timetable/compose/httpintegration`.

These early runs used the then-current migrated schema with its compatibility
objects present. They did not replace the later full run after Contract
registration. The remaining conversions described next preceded that run.

The Care Plan, student-presence, statistics, school calendar, school portal,
photo purge, parent-message, data-import and meal-plan test SQL was subsequently
converted. The Class Day and emergency snapshot RLS matrices now check each of
the three owner tables under the non-bypass tenant role; Care Plan uses its
`membership_id` key. These complete package runs passed with `-parallel 8
-count=1` (multi-package runs bounded with `-p 4`):

- `./modules/careplan/{integration,inbound/parent,compose,legacy/careschedule,legacy/carelifecycle}`.
- `./modules/studentpresence/legacy/{services/active,repositories/active}`.
- `./modules/schoolcalendar/portal/compose`, `./modules/statistics/http`,
  `./modules/schoolportal`, `./modules/peopledirectory/compose`,
  `./modules/identityaccess/behavior`, and
  `./modules/communication/internal/parentmessages`.
- `./modules/classday/compose`, `./modules/emergencysnapshot/legacy`,
  `./modules/dataimport/inbound/compose`, and `./modules/mealplan/compose`.

The users/enrollment repository tests, parent-portal workflow tests and
booking-consistency test adapter were subsequently converted as well. Full
package runs passed for `./database/repositories/users`,
`./database/repositories/enrollment`, `./workflows/parentportal/legacy`, and
`./test`. Missing required Care Plan columns now fail the repository tests
rather than skipping coverage. The date-column gate records the old enrollment
dates as moved to membership and documents the existing adapter-local
`calendar.Date` fields; the People Directory Postgres adapter tests passed.
The composition inventory was regenerated and inspected: existing callers have
updated line numbers and table evidence, with no constructor additions.

Additional API membership/absence fixtures were converted; complete
`./api/students`, `./api/iot/data`, `./api/groups`, and `./api/timetable` runs
passed. The remaining API fixtures were then converted: contact tests arrange
linked guardian profiles instead of writing retired student contact copies;
wide student payloads are populated in profile and Care Plan separately;
runtime checkpoint volumes include all three owner tables. Student and group
API packages passed, and the complete `./api` router/golden package passed
after correcting two accidentally renamed `students_guardians` references.
The remaining old-name API occurrences are a comment and a query-classifier
literal, not database calls. Historical
cutover/backfill tests and scanner fixtures intentionally mention old storage
and need separate treatment from current-schema callers.

### Historical schema reconstruction

`RestoreStudentStorageBeforeCutover` can now rebuild the historical table after
Contract removed the archive, rather than assuming it still exists. Its
embedded schema-only fixture was captured from the local 1.15.398 test template
with PostgreSQL 17 `pg_dump`, retaining column definitions, constraints, indexes,
RLS and grants. It contains no student data and is not a production recovery
mechanism. The helper verifies the passed pool belongs to the current test's
isolated clone before executing DDL.

The new `TestStudentOwnerContractHistoricalFixtureRestoresAfterRemoval` first
failed on the missing old table, then passed after the helper change. It
executes Contract, reconstructs the historical schema, backfills two students,
runs cutover and absence compatibility, and executes Contract again. The
focused `TestStudent(Owner|Care|Contract)` migration run and
`Test.*StudentOwner` command run passed. Architecture checks remained at 682
legacy findings and 667 composition targets.

Previous-image compatibility tests and tests explicitly removing the view still
need arrangement for a default template in which Contract has already run.

### Integrity regression checks

A new duplicate-live-membership fixture first demonstrated that Contract
accepted ambiguous owner data if the unique index had been lost. The data gate
now explicitly rejects duplicate live memberships per school/profile. Tests
also exercise a missing profile link after removal of its FK and disabled
Care Plan RLS. `TestStudentOwnerContract` passed after these additions
(7.665 seconds); failed gates leave owner snapshots unchanged. The architecture
ratchet remains at 682 legacy findings and 667 composition targets.

### Synthetic production-cardinality check

`TestStudentOwnerContractObservedProductionCardinalities` builds 1,448 synthetic
profiles across twelve populated schools (plus the empty bootstrap school),
2,830 historical status days and sixteen live sick flags. It executes historical
backfill/cutover before Contract. The first measured Contract duration on the
ordinary local test server was **78.003833 ms**, with byte-equivalent JSON
snapshots of all three owner tables and dated status history before/after and
all three retired relations absent afterwards. The test completed in 3.65 s.

This matches observed row counts, not production value/index/cache distributions.
The ordinary test service disables durability. No lock contention, restore,
production I/O latency or authenticated old-image recovery was measured by this
run, so its duration is not a production runtime guarantee or the full schema-3
migration evidence record.

### Observed locking and explicit Down refusal

`TestStudentOwnerContractObservedLockWaitAndTimeout` observes the blocked
Contract session through `pg_stat_activity.wait_event_type = 'Lock'` and
`pg_blocking_pids`, tying it to the holding transaction. On the local test
server, the release case remained observed as blocked for at least
154.161750 ms and completed in 198.636708 ms. The held-lock case returned a
verified PostgreSQL lock-timeout error in 5.015356417 s and retained the
rollback view/archive. All owner/history snapshots remained equal in both
cases. These are small synthetic contention scenarios, not a production
lock-distribution measurement. Raw output: [lock-wait.txt](evidence/student-owner-contract-2760/lock-wait.txt).

The explicit Down function refuses automatic reconstruction and names the
coordinated pre-Contract backup/prior-image recovery. The production-cardinality
test also exercises that refusal and unchanged owner snapshots; its latest raw
output is [synthetic-cardinalities.txt](evidence/student-owner-contract-2760/synthetic-cardinalities.txt).
Migration registration and the later database/prior-image restore smoke are
documented above; this earlier lock measurement did not itself establish them.

### Delivery status and operational handoff

The SHA-256 revision has a fresh durable synthetic core measurement:
105.311875 ms, 26 BUN events, zero pool waits and zero deadlock delta. All owner
and history snapshots matched. See [output](evidence/student-owner-contract-2760/sha256-contract-cardinalities.txt)
and [source hashes](evidence/student-owner-contract-2760/sha256-contract-source.txt).
The 166-package changed-test run passed after the tracking-gate corrections and
a timestamp-collision fixture repair. Both review findings and the subsequent
tracking-configuration finding are addressed; local Loki/Grafana notification
delivery also passed. See [monitoring evidence](evidence/student-owner-contract-2760/monitoring.md).

1. The current deployment policy above supersedes the original mandatory
   waiting period. Live integrity and zero-compatibility-hit checks remain.
2. Review corrections, current broad tests, release-build
   verification and monitoring evidence are recorded in the
   [verification summary](evidence/student-owner-contract-2760/final-verification.md).
3. Deploy staging first and verify the application before promotion. Use the
   complete release-backup/rollback pipeline for both environments. Optional
   evidence documents do not replace that backup. These historical local
   results are not proof of a completed deployment or unknown external callers.
