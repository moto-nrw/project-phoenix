# Final legacy cluster: round 8

Implementation base: `148e5aeb86cf22fdbfbffedd76ba070f52f80a3e`. The Round 7 record below is historical; its blockers and counts are not the current exit claim.

Integrated development: `e423ad19c732f17966b2f8332eafb5df01196f29`. Upstream retention changes and gocognit 1.3 scores are retained. Two upstream ADRs numbered 0072 were made unique by moving the retention ADR to 0073 and updating its references, without changing either decision.

## Literal exits

| Issue | Evidence and current result |
|---|---|
| #4086 | The remaining 16 non-root production implementation packages move under `internal`. The five private projection adapters move to their projection homes as well. No production Go package remains in `services`, `models` or `database/repositories`; no production legacy directory or legacy-prefixed Go file remains (the negative architecture fixture `testdata/semantic/invalid/legacy` is intentionally retained). `cmd` is a root-composition/CLI role, not an implementation carrier (ADR 0053 point 1; ADR 0059; ADR 0068 point 1). `templates/email` contains templates and external template tests, but no production Go file. |
| #4084 | Not proven. The complete role-based inventory still records runtime bodies, including constructor callbacks. The allowed extractions and reduced budgets below do not disguise that remainder. The native parent-announcement facade attempt passed Go tests but was reverted because internal `communication/adapter-test` suites cannot import `communication/application`. No test grant, alias, DTO copy or role change was added. |
| #4004 | Already complete in development. The legacy baseline stays empty. No key is added or retired in this round. |

## Approved ADR 0020 rename semantics

The projection guard consumes the canonical Git-verified package map. Owner, production role, both test roles, grant identity, table list and tenant-safety privilege must remain identical. A new table, changed privilege, changed identity, missing relocation or owner change is rejected by regression tests.

The transaction register uses the same verifier through the read-only `relocations` metadata command. Scope, function, operation, ordinal and reason are identical. Complete source-body bytes are compared directly as an injective fingerprint, excluding file paths; no hash collision can accept a changed body. Changed bodies/reasons/operations, additional/copied entries, unverified paths and scope changes fail. No register entry is added.

## Package relocation proof

| Previous package | Current package | Owner / role | Go files |
|---|---|---|---:|
| `auth/device` | `modules/devicefleet/internal/adapters/deviceauth` | `device-fleet / adapter` | 4 |
| `auth/device/devicetest` | `modules/devicefleet/devicetest` | `device-fleet / test-support` | 2 |
| `modules/communication/internal/adapters/audience` | `modules/staffannouncementaudience/internal/adapters/postgres` | `staff-announcement-audience / postgres` | 4 |
| `modules/communication/internal/adapters/parentaudience` | `modules/parentannouncementaudience/internal/adapters/postgres` | `parent-announcement-audience / postgres` | 7 |
| `modules/communication/internal/adapters/parentinbox` | `modules/parentmessageinbox/internal/adapters/postgres` | `parent-message-inbox / postgres` | 5 |
| `modules/communication/internal/adapters/staffinbox` | `modules/staffmessageinbox/internal/adapters/postgres` | `staff-message-inbox / postgres` | 4 |
| `modules/communication/projections/parentannouncementaudience` | `modules/parentannouncementaudience` | `parent-announcement-audience / compose` | 1 |
| `modules/communication/projections/parentmessageinbox` | `modules/parentmessageinbox` | `parent-message-inbox / compose` | 1 |
| `modules/communication/projections/staffannouncementaudience` | `modules/staffannouncementaudience` | `staff-announcement-audience / compose` | 1 |
| `modules/communication/projections/staffmessageinbox` | `modules/staffmessageinbox` | `staff-message-inbox / compose` | 1 |
| `modules/delivery/application/emailoutbox` | `modules/delivery/internal/emailoutbox` | `delivery-platform / application` | 7 |
| `modules/delivery/application/notifications` | `modules/delivery/internal/notifications` | `delivery-platform / application` | 51 |
| `modules/delivery/application/pwa` | `modules/delivery/internal/pwa` | `delivery-platform / application` | 9 |
| `modules/guardianlinkview` | `modules/guardianlinkview/internal/adapters/postgres` | `guardian-link-view / postgres` | 4 |
| `modules/organizationtenancy/internal/adapters/operatordashboard` | `modules/operatordashboard/internal/adapters/postgres` | `operator-dashboard-view / postgres` | 5 |
| `modules/organizationtenancy/projections/operatordashboard` | `modules/operatordashboard` | `operator-dashboard-view / compose` | 1 |
| `modules/parentrequestfeedprojection` | `modules/parentrequestfeedprojection/internal/adapters/postgres` | `parent-request-feed-view / postgres` | 3 |
| `modules/presenceprojection` | `modules/presenceprojection/internal/adapters/postgres` | `presence-legacy-view / postgres` | 9 |
| `modules/schedulerruntime/jobruntime` | `modules/schedulerruntime/internal/adapters/jobruntime` | `scheduler-runtime / postgres` | 2 |
| `modules/schedulerruntime/workerlease` | `modules/schedulerruntime/internal/adapters/workerlease` | `scheduler-runtime / postgres` | 3 |
| `modules/schoolsetupview` | `modules/schoolsetup/internal/adapters/schoolsetupview` | `school-setup / postgres` | 1 |
| `modules/studentdirectoryview` | `modules/studentdirectoryview/internal/adapters/postgres` | `student-directory-view / postgres` | 6 |
| `modules/timetableblockdisplay` | `modules/timetableblockdisplay/internal/adapters/postgres` | `timetable-block-display / postgres` | 4 |
| `modules/timetableenrollmentprovenance` | `modules/timetableenrollmentprovenance/internal/adapters/postgres` | `timetable-enrollment-provenance / postgres` | 3 |
| `modules/timetableprojection` | `modules/timetableprojection/internal/adapters/postgres` | `timetable-legacy-view / postgres` | 2 |
| `workflows/parentportal/care` | `workflows/parentportal/internal/care` | `parent-portal / application` | 90 |
| `workflows/parentportal/messaging` | `workflows/parentportal/internal/messaging` | `parent-portal / application` | 19 |

Owner and all three roles are unchanged. The canonical verifier checks the exact Go filename set. The five projection builders remain constructor-role packages at their module roots; they do not become implementation packages. The private runtime packages retain their cohesive names under `internal` rather than gaining another compatibility facade. Their pre-existing delegation scan coverage is not presented as an improvement or a retired debt.

## Compose work retained

- People Directory caregiver extraction was reverted after the unchanged production-router contract exposed a changed equality identity. The original comparable service/database binding remains; no assertion or golden changed.
- Timetable reorder validation/error handling move to application; composition only binds the registered executor.
- Communication platform-announcement response/error mapping moves to application, and the native SQL adapter owns ambient transaction validation. The separate parent-announcement facade remains in the inventory for the test-reach blocker.
- Audit context validation, handoff resolution and logging callbacks move to existing private adapter/application roles with constructor-supplied context providers.
- School Setup row mapping, School Structure substitution/error mapping, School Membership employment/count/scope mappings, Scheduler lease adaptation and Transaction Runtime pool lifecycle move to private owner roles.
- Grade Transition, Student Deletion and Staff Offboarding cross-owner authorization, audit, event and runtime binding move to the existing typed root. Existing owner dependency records remain at their seam; no root composition field is added.

## Exact remaining blockers and boundaries

| Surface | Exact constraint | Outcome |
|---|---|---|
| People Directory caregiver boundary | The unchanged `TestFullProductionRouterGolden/contracts/caregiver_wiring` contract compares separately constructed views for equality. Injected function fields made the value uncomparable; pointer embedding made it comparable but unequal. Preserving the exact existing binding is required. | Attempt reverted; the original boundary and its administrative register entry remain unchanged. |
| Parent-announcement shared application port | `internal_test` source `modules/communication/internal/staffannouncements`, owner `communication`, role `adapter-test`, target `modules/communication/internal/application`, role `application`, is forbidden. Production adapter-to-application is allowed, but no equivalent internal-test reach exists. The native seam passed its Go suites before this architecture rejection. | Attempt reverted; assertions, roles and grants remain unchanged. |
| Partial extraction of registered domain composition executors | The transaction register accepts whole Git-verified same-owner/role package renames only. Moving an individual boundary, such as `modules/facilities/compose/new.go:transaction.RunWrite:tenant.WithinCurrentTenant#1`, into an existing application package cannot supply that package proof; replacing it with a differently keyed invocation is still rejected. Constructor-only bindings are retained where possible; execution guards are not hidden by parentheses, operation renames or new allowances. | Remaining bodies are named in the complete inventory. No wider guard extension is made. |

The [complete compose audit](final-legacy-cluster-round-eight-compose.md) enumerates every non-constructor by package, source line and calls, and separately records function-literal bodies. It is the evidence for the outstanding exit, not a claim that a low LOC count makes runtime logic construction.

## Historical evidence

Local source links in earlier ticket evidence that genuinely moved are pinned to their immutable pre-move commit. Their behavioral claims, exact keys and thresholds are unchanged. Current round evidence uses the new paths.

---

# Round 7 historical record

Base: `121b0ee5130db5ec6b289965f495acdca6285744` after PR #4344. The obsolete held follow-up is not part of this branch. Integrated development `2fdb6d2c4cff1bd18069067f07316ed51dcae7c1` after the dependency-only PR #4375; backend source and policy are unchanged by that integration.

Final CI-repair integration: development `dcb2976982bda8a59ca777d4ba20aea112625e11`, including dependency batches PRs #4377, #4374 and #4376. Their upstream source/golden repairs are retained; the PR adds no route, middleware or golden change relative to that base.

## Measured cutover

| Surface | Before | After |
|---|---:|---:|
| Legacy keys | 1 | 0 |
| Composition field/setter targets | 0 | 0 |
| Compose LOC cap | 24,501 | 20,916 |
| Rules | 924 | 927 |
| ADR 0020 package relocations in this round | 0 | 17 |
| Outside-internal implementation packages, including CLI roles | 33 | 17 |
| Legacy-prefixed files and legacy directories | 0 | 0 |

The production Data Import authorization port is bound at the HTTP root using Security Runtime public decisions. Its internal decision tests retain their existing port double. `auth/authorize` is now Security Runtime internal application. The four public aliases that would otherwise expose its private types are removed: their actual context/audience definitions live in the native authorization port. The statistics facade returns a structural read capability rather than a private concrete type. No alias or row-schema clone is introduced.

The canonical architecture regression now requires a baseline-free check to succeed with exactly zero violations. This is the explicitly authorized #2580 zero-ratchet cutover, not a relaxed checker. `legacy.jsonl` is empty; the retirement evidence covers its last exact tuple.

## Compose extraction

Runtime capability checks, lifecycle conversion, recovery orchestration, planning-track validation and reads move to Timetable application. People Directory owns its route/person/student row conversions, retained repository read/write facades, caregiver JSON/error mapping and birthday view in application. Group-room row conversion is internal School Structure application. Root-only staff/person/group enrichment and the cross-owner staff invitation binding move to the typed root. Settings owns presence reads and snapshot/preload validation in application; snapshot interfaces move once to its native records. Appointments, Feedback, Meal Plan and School Calendar own their public response/error-mapping engines. Facilities and Device Fleet repository adapters become internal.

Communication staff SQL stores implement the native thread/message ports directly. The extra thread/message wrapper layer is deleted, rather than adding ten previously uncounted delegations. SQL, tenant filters, nil/empty results, timestamps and success-only message copyback remain unchanged. Two unused private row-value definitions are deleted.

The full [compose function inventory](final-legacy-cluster-round-seven-compose.md) records every remaining non-constructor function, including test-only functions and all registered compose boundaries. Budgets are not used as proof of the constructor-only exit.

## Literal exits and exact blockers

| Issue | Evidence and remaining condition |
|---|---|
| #4004 | Proven: zero legacy tuples and no forbidden cross-owner composition import. The final Data Import edge is gone and its production binding is root-owned. |
| #4084 | Not met. The complete inventory still contains runtime adapters and business/row-mapping bodies. In particular the caregiver administrative boundary and Timetable planning reorder are exact #4088 register entries. Their relocation is rejected by the unchanged shrink-only register. Communication announcement facade extraction also failed: `communication/adapter -> communication/public` is forbidden; moving that facade into application creates a Go cycle because the retained announcement adapter imports application. The attempted extraction was reverted. Other remaining bodies are explicitly inventoried, not silently called constructor-only. |
| #4086 | Not met. The remaining paths below are implementation packages outside internal. Projection-grant path changes and registered runtime moves were reverted; no grant, composition exception or guard extension was added. |

## Package relocations

| Old package | New package |
|---|---|
| `applog` | `observability/internal/adapters/applog` |
| `auth/authorize` | `modules/securityruntime/internal/application/authorize` |
| `auth/userpass` | `modules/securityruntime/internal/domain/userpass` |
| `auth/userpass/userpasstest` | `modules/securityruntime/userpasstest` |
| `database` | `modules/transactionruntime/internal/adapters/database` |
| `email` | `modules/delivery/internal/adapters/email` |
| `email/emailtest` | `modules/delivery/emailtest` |
| `localization` | `modules/delivery/internal/adapters/localization` |
| `modules/dataimport/fileformat` | `modules/dataimport/internal/adapters/fileformat` |
| `modules/delivery/application/emailbranding` | `modules/delivery/internal/application/emailbranding` |
| `modules/delivery/application/locales` | `modules/delivery/internal/application/locales` |
| `modules/delivery/application/realtimeevents` | `modules/delivery/internal/application/realtimeevents` |
| `modules/filestorage/documents` | `modules/filestorage/internal/adapters/documents` |
| `modules/peopledirectory/userscontract` | `modules/peopledirectory/internal/domain/userscontract` |
| `modules/schedulerruntime/cleanupruns` | `modules/schedulerruntime/internal/adapters/cleanupruns` |
| `seed/api` | `internal/devtools/seedapi` |
| `simulate` | `internal/devtools/simulate` |

Owner and all three roles are identical for these declarations. Source and destination Go filenames are checked against the immutable base by ADR 0020. The dead, unimported `constants` package is deleted, not relocated.

## Remaining outside-internal implementations

| Package | Owner / role | Exact blocker |
|---|---|
| `modules/studentdirectoryview` | `student-directory-view / postgres` | Exact tenant-safe projection grants are keyed by this path. The attempted ADR 0020 move was rejected as new grants by `readProjectionLoosenings`; no table/scope change was proposed. |
| `auth/device` | `device-fleet / adapter` | The attempted same-owner/role relocation produced new composition-surface setter keys for `LastSeenDebouncer.scheduleDeferredFlushLocked -> lastSeenDebounceState.flushContext` and the existing `heartbeatContextDirectory.RecordLastSeen -> contexts` test injection. The surface must stay zero; no normalization or exception was added. |
| `cmd` | `root-composition / cli` | The immutable #4088 register contains 2 site(s) at this path, e.g. `cmd/seed_parents.go:seedParentAccounts:tenant.WithinAdmin#1`. A renamed path is rejected by `transactionRegisterOnlyShrinks`. |
| `modules/delivery/application/emailoutbox` | `delivery-platform / application` | The immutable #4088 register contains 2 site(s) at this path, e.g. `modules/delivery/application/emailoutbox/transport.go:Transport.withinIntent:p.Tenants.WithinSystem#1`. A renamed path is rejected by `transactionRegisterOnlyShrinks`. |
| `modules/timetableprojection` | `timetable-legacy-view / postgres` | Exact tenant-safe projection grants are keyed by this path. The attempted ADR 0020 move was rejected as new grants by `readProjectionLoosenings`; no table/scope change was proposed. |
| `modules/timetableblockdisplay` | `timetable-block-display / postgres` | Exact tenant-safe projection grants are keyed by this path. The attempted ADR 0020 move was rejected as new grants by `readProjectionLoosenings`; no table/scope change was proposed. |
| `modules/presenceprojection` | `presence-legacy-view / postgres` | Exact tenant-safe projection grants are keyed by this path. The attempted ADR 0020 move was rejected as new grants by `readProjectionLoosenings`; no table/scope change was proposed. |
| `modules/timetableenrollmentprovenance` | `timetable-enrollment-provenance / postgres` | Exact tenant-safe projection grants are keyed by this path. The attempted ADR 0020 move was rejected as new grants by `readProjectionLoosenings`; no table/scope change was proposed. |
| `modules/parentrequestfeedprojection` | `parent-request-feed-view / postgres` | Exact tenant-safe projection grants are keyed by this path. The attempted ADR 0020 move was rejected as new grants by `readProjectionLoosenings`; no table/scope change was proposed. |
| `modules/delivery/application/notifications` | `delivery-platform / application` | The immutable #4088 register contains 13 site(s) at this path, e.g. `modules/delivery/application/notifications/preferences.go:*preferenceService.forEachGuardianTenant:tenant.WithAdminTx#1`. A renamed path is rejected by `transactionRegisterOnlyShrinks`. |
| `modules/delivery/application/pwa` | `delivery-platform / application` | The immutable #4088 register contains 3 site(s) at this path, e.g. `modules/delivery/application/pwa/usage.go:*usageService.ReportParent:tenant.WithAdminTx#1`. A renamed path is rejected by `transactionRegisterOnlyShrinks`. |
| `workflows/parentportal/care` | `parent-portal / application` | The immutable #4088 register contains 7 site(s) at this path, e.g. `workflows/parentportal/care/child.go:*Service.ResolvePermittedChild:s.Runtime.WithinAdmin#1`. A renamed path is rejected by `transactionRegisterOnlyShrinks`. |
| `workflows/parentportal/messaging` | `parent-portal / application` | The immutable #4088 register contains 15 site(s) at this path, e.g. `workflows/parentportal/messaging/parent_announcement_attachments.go:*Service.GuardianAnnouncementTenant:s.Runtime.WithinAdmin#1`. A renamed path is rejected by `transactionRegisterOnlyShrinks`. |
| `modules/schedulerruntime/jobruntime` | `scheduler-runtime / postgres` | The immutable #4088 register contains 3 site(s) at this path, e.g. `modules/schedulerruntime/jobruntime/runtime.go:Runtime.WithinAdmin:tenant.WithinAdmin#1`. A renamed path is rejected by `transactionRegisterOnlyShrinks`. |
| `modules/schedulerruntime/workerlease` | `scheduler-runtime / postgres` | The immutable #4088 register contains 1 site(s) at this path, e.g. `modules/schedulerruntime/workerlease/store.go:*Store.inAdminTransaction:tenant.WithAdminTx#1`. A renamed path is rejected by `transactionRegisterOnlyShrinks`. |
| `modules/guardianlinkview` | `guardian-link-view / postgres` | Exact tenant-safe projection grants are keyed by this path. The attempted ADR 0020 move was rejected as new grants by `readProjectionLoosenings`; no table/scope change was proposed. |
| `modules/schoolsetupview` | `school-setup / postgres` | Exact tenant-safe projection grants are keyed by this path. The attempted ADR 0020 move was rejected as new grants by `readProjectionLoosenings`; no table/scope change was proposed. |

The five projection adapters already physically under Communication/Organisation internal remain at their current paths as well: audience, parentaudience, parentinbox, staffinbox and operatordashboard. Moving them under their projection owner requires renaming exact `read_projections` grants; the same guard rejects that change. Their owner and role are not reclassified.

## Rules and one-to-one scan preservation

| Added rule | Narrow successor / precedent |
|---|---|
| `security-runtime.public.authorization-port` | `platform.public-application`, following `transaction-runtime.public.stats-port`; the actual authorization input types replace public application aliases. |
| `people-directory.http.authorization-port` | `people-directory.http.security-public`; same note-audience values, exact native port package only. |
| `people-directory.adapter-test.authorization-port` | `adapter-test.platform.public`; the same native note-audience values in the existing internal HTTP tests, exact native port package only. |

No key, grant, exception, hook allowance or budget grows. The function-length and complexity allowances for the four Data Import format files move 1:1 to `internal/adapters/fileformat`, with the same operations, limits and counts (four length entries, two complexity entries). Five realtime notice paths move 1:1 to Delivery internal application; operations, reasons and body fingerprints are unchanged, and the notice count stays 117. The existing 812 command and 257 administrative transaction-register entries and its guard are unchanged.

ADR 0064 scan preservation adds the exact native School Calendar feed-history file and the exact Device Fleet adapter subtree to the existing pass-through scan. The moved root staff-invitation file stays charged to Data Import. These extend scan coverage, not permission or budgets. The source methods were removed from the former counted composition trees; their owner attribution is unchanged.

The workflow selector’s one real-source locale path literal moves 1:1 from `backend/localization/locales.json` to `backend/modules/delivery/internal/adapters/localization/locales.json`. The three-pattern loop and both filter assertions stay unchanged (six assertions); its synthetic localization fixture paths are unchanged. This repairs the common pre-test cause of the two initial CI failures.

The locale JSON moves with its adapter. Three frontend script references and the CI locale filters point to the same data file at its new path. The SMTP demo test template reference changes 1:1 from `../templates` to `../../../../../templates`. Seed coverage CI and marketing filters track the same seed source files at the new tool-internal path.

## Verification

Required final results are recorded in the PR: architecture, all-ticket validation with retirement coverage, full affected tests without `--fast`, lint, full hooks and quorum. No benchmark, production request or manual HTTP smoke is claimed. Routes, middleware/IoT texts and router goldens are unchanged.

---
# Final legacy cluster: round 6

Integrated base: `466b4f956fa93c526b0f78fd0cb2a2ecd40f0d51` (development after PR #4341). The preserved work was committed before integration, as requested. Earlier rounds below are historical.

## Current measured result

| Surface | Before | After |
|---|---:|---:|
| Legacy keys | 1 | 1 |
| Conventional production carriers | 3 | 0 |
| Foreign compose aliases | 0 | 0 |
| Production type aliases removed in this round | 0 | 12 |
| Compose LOC budget | 27,061 | 24,971 |
| Counted delegation budget | 3,037 | 3,019 |
| Legacy-prefixed LOC budget | 2,666 | 0 |
| `legacy_*.go` files, including tests | 18 | 0 |
| Policy rules | 928 | 922 |
| Composition field/setter targets | 0 | 0 |

`services/scheduler`, `models/activities` and `models/schedule` have no production package or importer. `services/`, `database/repositories/` and `models/` have no production package; the existing test-only model interface pin remains. No `legacy/` directory remains. The deferred Data Import authorization key and baseline-free regression test are unchanged. No retirement key is removed, so there is no new retirement coverage to supply.

## Owner cutover

- Scheduler's 66 Go files move to `modules/schedulerruntime/internal/application`, retaining owner, application role, instance state, dependency injection, job IDs and test assertions. Compose only binds its constructor.
- Timetable owns the actual activity and schedule rows in `internal/domain/{activityrecords,schedulerecords}`. Its persistence interfaces have typed IDs and bounded operations, without a generic repository or production query-options API. Runtime instance queries use the existing bounded presence filter; participant status narrowing still occurs after pagination.
- The seven retained ActivityService files move to Timetable internal application. Its compose constructor returns the existing native activity contract. Canonical dates are `sharedkernel/calendar.Date`; the old date suites move with that capability, and retained parsing errors keep their exact text.
- `fixturetest` owns the old suites' query construction and native row handles. Handles embed the canonical row, rather than aliasing it or defining another row schema. The live, typed fixture bridges preserve mutation and optional capabilities; deadcode removed unused bridge operations. Fixture interfaces split into planning, lookup, attendance, excusal, retention and lifecycle capabilities; per-capability files stay below 800 lines. A dead pending-request lookup is deleted; the closing-day range behavior remains in owner test support. External integration suites no longer import private domain/application code. Existing owner decision dependency literals continue to use their already-authorized ports, without a new port grant or graph factory.
- People Directory and School Structure runtime context operations move into application; driver handling moves into the owner's Postgres adapter, with context operations supplied at construction. School Structure's redundant CRUD forwarding wrapper is deleted. Presence rows merge into the existing normal role file. Five test files receive byte-identical filename renames; the remaining test helper merges into its existing owner helper.
- Two moved validations exceeded the target complexity ceiling. Their metadata/partial-absence validation is split into private helpers, preserving order, mutation and error strings. No complexity allowance is added.

## One-to-one mechanical evidence

| Old path/reference | New path/reference | Proof |
|---|---|---|
| `services/scheduler/worker.go` and registry paths in `test/composition_inventory_test.go` | `modules/schedulerruntime/internal/application/worker.go` and corresponding registry paths | Same 66 files, 7 constructor calls and 27 jobs; fixed historical constructor evidence stays unchanged. |
| `services/scheduler/tenant_runtime.go:Scheduler.afterCommit:RegisterAfterCommit@0669f453e5ef89dcbbf01b3a0022619d#1` | `modules/schedulerruntime/internal/application/tenant_runtime.go:Scheduler.afterCommit:RegisterAfterCommit@0669f453e5ef89dcbbf01b3a0022619d#1` | Exactly one old entry and one new entry. Register 117 → 117; operation, reason and body fingerprint `0669f453e5ef89dcbbf01b3a0022619d` identical. Every other entry is byte-equivalent. |
| `modules/timetable/compose/legacy_supervisor_operations.go:buildReplacementSupervisorIDs` | `modules/timetable/internal/application/activity_supervisor_operations.go:buildReplacementSupervisorIDs` | One existing complexity reference moves 1:1; cap remains 22. |
| Scheduler source-discovery roots | Exact new Scheduler application subtree | Same scanned files; other removed roots are already covered by the existing `modules/` scan. |
| People/School Structure runtime in compose | Exact new owner Postgres runtime subtrees | Both moved implementations remain in the existing delegation scan, under the same owners. |
| Repository-interface scan `models/*/repository.go` | `modules/timetable/ports/*_persistence.go` | Two actual old files → two actual native capability files. Same two exception entries, none added; all 47 formerly scanned method names remain covered, with additional native methods checked. The obsolete optional config path is unchanged. |
| Two pure schedule model suites | Same owner `activity_instance_internal_test.go`, `pickup_schedule_internal_test.go` | Both are database-free internal domain suites. File renames only; assertions and fixture literals unchanged. They use the existing internal-unit-test convention, not a new skip entry. |

The approved notice guard extension only accepts removed entries as relocation sources. It consumes each source once and compares operation, reason and body fingerprint. Tests cover pure relocation, changed body/reason/operation, retained-entry mutation, duplicate relocation, an additional entry, and retirement alongside relocation. The old equal-sized opaque replacement regression stays unchanged. The additional repository scan repair changes only its source-path literal. No other checker logic changes.

Historical evidence sources in six existing ticket JSONs are pinned to this immutable integrated base where the original files still exist, rather than rewriting their historical claims. `validate-ticket --all` checks every ticket and reports 0 removed keys / 0 uncovered retirements.

## Rules and budget deltas

Three new native test-support rules, with precedents:

| Rule | Precedent / narrower boundary |
|---|---|
| `timetable-activities.test-support.activity-contract` | `delivery.test-support.native-contracts`: owner fixtures consume the actual owner contract. |
| `timetable-activities.test-support.native-domain` | `security-runtime.test-support.domain`: the exact owner fixture package constructs canonical owner rows/errors. Explicitly authorized Round 6 owner test-support reach. |
| `timetable-activities.test-support.school-calendar-public` | `timetable-activities.application.school-calendar.public`: the exact fixture package uses School Calendar's native dateframe filter, on the existing owner DAG edge. |

Nine stale rules are removed: `domain.application-legacy-model`, `domain.postgres-legacy-model`, `internal-test.same-owner.legacy-model`, `owner-compose.domain.legacy-model`, `timetable-activities.http-integration-test.application`, `timetable-activities.http-integration-test.owner-domain`, `timetable-activities.legacy-model.production.activity-contract`, `timetable-activities.port.legacy-model`, and `timetable-activities.integration-test.legacy-model`.

Compose caps: People Directory 2,576 → 2,504; School Structure 883 → 744; Timetable 3,777 → 1,898. Other caps do not rise. Delegation caps: School Structure 85 → 70; Timetable 491 → 488. Twelve production aliases and one test alias are deleted, none introduced.

## Literal issue-exit audit

| Issue | State / exact remaining condition |
|---|---|
| #4004 | Open. The last `dataimport/compose → auth/authorize` edge is explicitly deferred until the separate zero-ratchet cutover after #3021. |
| #4076, #4077, #4085 | Already completed in earlier merged rounds; no state change requested here. |
| #4084 | Open. All touched compose budgets are met and ActivityService is internal, but not every compose package only constructs/binds. For example `peopledirectory/compose/caregiver_capability_views.go` implements tenant execution/error mapping; `schoolstructure/compose/group_rooms.go:ListGroupRooms` maps rows; `timetable/compose/owner_rows.go:OperationTemplates` checks runtime capabilities. Passing the LOC ratchet does not prove their absence. |
| #4086 | Open. Legacy trees/files and conventional production carriers are gone, but the literal “every implementation package under internal” condition is false. `auth/authorize` cannot become private while the explicitly deferred foreign Data Import edge remains. Projection physical locations/grants stay unchanged by instruction. Other outside-internal implementations are enumerated below, not silently declared exempt or reclassified. |

## Remaining outside-internal implementation inventory

This is a policy-role/source-file inventory, not a claim that every row needs the same migration. Root/dev CLI entry points and the earlier-round projection locations are included rather than inventing exemptions from the issue wording. No relocation grant is rewritten in this round.

| Package | Owner | Role |
|---|---|---|
| `modules/studentdirectoryview` | `student-directory-view` | `postgres` |
| `modules/peopledirectory/userscontract` | `people-directory` | `domain` |
| `modules/filestorage/documents` | `file-storage` | `adapter` |
| `applog` | `observability` | `adapter` |
| `auth/authorize` | `security-runtime` | `application` |
| `auth/device` | `device-fleet` | `adapter` |
| `auth/userpass` | `security-runtime` | `domain` |
| `cmd` | `root-composition` | `cli` |
| `constants` | `legacy-shared` | `domain` |
| `database` | `transaction-runtime` | `postgres` |
| `email` | `delivery-platform` | `adapter` |
| `localization` | `delivery-platform` | `adapter` |
| `modules/dataimport/fileformat` | `data-import` | `adapter` |
| `seed/api` | `dev-tools` | `cli` |
| `modules/delivery/application/locales` | `delivery-platform` | `application` |
| `modules/delivery/application/emailbranding` | `delivery-platform` | `application` |
| `modules/delivery/application/emailoutbox` | `delivery-platform` | `application` |
| `modules/timetableprojection` | `timetable-legacy-view` | `postgres` |
| `modules/timetableblockdisplay` | `timetable-block-display` | `postgres` |
| `modules/presenceprojection` | `presence-legacy-view` | `postgres` |
| `modules/timetableenrollmentprovenance` | `timetable-enrollment-provenance` | `postgres` |
| `modules/parentrequestfeedprojection` | `parent-request-feed-view` | `postgres` |
| `modules/delivery/application/notifications` | `delivery-platform` | `application` |
| `modules/delivery/application/pwa` | `delivery-platform` | `application` |
| `modules/delivery/application/realtimeevents` | `delivery-platform` | `application` |
| `workflows/parentportal/care` | `parent-portal` | `application` |
| `workflows/parentportal/messaging` | `parent-portal` | `application` |
| `modules/schedulerruntime/jobruntime` | `scheduler-runtime` | `postgres` |
| `modules/schedulerruntime/cleanupruns` | `scheduler-runtime` | `postgres` |
| `modules/schedulerruntime/workerlease` | `scheduler-runtime` | `postgres` |
| `simulate` | `dev-tools` | `cli` |
| `modules/guardianlinkview` | `guardian-link-view` | `postgres` |
| `modules/schoolsetupview` | `school-setup` | `postgres` |

## Importer inventory at the round-6 base

The original owner/symbol inventory includes foreign users and the same-owner call sites needed for the atomic cutover. It is retained here as immutable pre-cutover evidence.

| Scope | Consumer owner / package | Old boundary | Selected symbols |
|---|---|---|---|
| external_test | `care-plan` / `modules/careplan/contracttest` | `modules/timetable/compose` | `NativeEnrollmentRecords`, `NativeExceptionRecords`, `NativeGroupRecords`, `NativeScheduleRecords`, `NativeTimeframeRecords`, `NewClassArrivalQueries`, `NewClassArrivals`, `NewConflictDetection`, `NewEnrollmentArrivalSchedules`, `NewEnrollmentPickupSchedules`, `NewInstanceStatusSlots`, `NewTimetableData`, `Observation` |
| external_test | `staff-offboarding` / `workflows/staffoffboarding` | `modules/timetable/compose` | `NewOffboarding`, `NewPresenceSessionFacts`, `Observation`, `OffboardingDependencies` |
| external_test | `timetable-activities` / `modules/timetable/compose/httpintegration` | `models/activities` | `GroupRepository`, `GroupTargetRepository`, `StudentEnrollmentRepository`, `TemplateFieldsUpdate`, `TemplateWeekdayRosterKindEmpty`, `TemplateWeekdayRosterKindProtectedStudent`, `TemplateWeekdayRosterKindStaff`, `TemplateWeekdayRosterKindStudent`, `TemplateWeekdayRosterRow` |
| external_test | `timetable-activities` / `modules/timetable/compose/httpintegration` | `models/schedule` | `ActivityCompletionSnapshot`, `ActivityExceptionCancelled`, `ActivityExceptionModified`, `ActivityExceptionRepository`, `ActivityException`, `ActivityInstanceRepository`, `ActivityInstance`, `ActivityRecoveryRepository`, `AttendanceFieldPatch`, `AttendanceStatusAbsent`, `AttendanceStatusExpected`, `AttendanceStatusPresent`, `AttendanceSubstatusExcused`, `AttendanceSubstatusFieldTrip`, `AttendanceSubstatusLate`, `AttendanceSubstatusSick`, `CalendarPeriodRepository`, `CalendarPeriod`, `ClosingDay`, `DateFromTime`, `Date`, `DateframeRepository`, `Dateframe`, `FrequencyDaily`, `FrequencyMonthly`, `FrequencyWeekly`, `InstanceIdempotencyRepository`, `InstanceStaffRepository`, `InstanceStaff`, `InstanceStatusActive`, `InstanceStatusCancelled`, `InstanceStatusCompleted`, `InstanceStatusPlanned`, `InstanceStudentKey`, `InstanceStudentNoteMaxLength`, `InstanceStudentRepository`, `InstanceStudent`, `NewDate`, `ParallelPresence`, `PartialAbsenceBlock`, `PeriodTypeCustom`, `PeriodTypeHoliday`, `PeriodTypeSchoolYear`, `PlanningTrackRepository`, `PlanningTrack`, `RecurrenceRuleRepository`, `RecurrenceRule`, `ScheduledInstanceRow`, `StaffShift`, `StudentArrivalExceptionRepository`, `StudentArrivalException`, `StudentArrivalNoteRepository`, `StudentArrivalNote`, `StudentArrivalScheduleRepository`, `StudentArrivalSchedule`, `StudentInstanceRef`, `StudentPickupExceptionRepository`, `StudentPickupException`, `StudentPickupNoteRepository`, `StudentPickupNote`, `StudentPickupScheduleRepository`, `StudentPickupSchedule`, `TimeframeRepository`, `Timeframe`, `WeekdayFriday`, `WeekdayMonday`, `WeekdayThursday`, `WeekdayTuesday`, `WeekdayWednesday` |
| external_test | `timetable-activities` / `modules/timetable/compose/httpintegration` | `modules/timetable/compose` | `NewCalendarPeriodUsageRepository`, `NewInstanceAutoEnd`, `NewInstanceLifecycle`, `NewMaterialization`, `NewOperations`, `NewPlanningCalendarPeriodUsage`, `NewPlanningTrackAdministration`, `NewRetainedActivityRecoveryRepository`, `NewTenantDatabase`, `NewTenantTransactions`, `OwnerRows`, `Planning` |
| external_test | `timetable-activities` / `modules/timetable/http` | `modules/timetable/compose` | `NewStaffNotices`, `StaffNoticeDependencies` |
| internal_test | `timetable-activities` / `modules/timetable/compose` | `models/activities` | `TemplateWeekdayRosterKindEmpty`, `TemplateWeekdayRosterKindProtectedStudent`, `TemplateWeekdayRosterKindStaff`, `TemplateWeekdayRosterKindStudent` |
| internal_test | `timetable-activities` / `modules/timetable/compose` | `models/schedule` | `ActivityExceptionCancelled`, `ActivityExceptionModified`, `ActivityInstanceIdempotencyKeyMaxLength`, `AttendanceStatusAbsent`, `AttendanceStatusExpected`, `AttendanceStatusPresent`, `AttendanceSubstatusExcused`, `AttendanceSubstatusFieldTrip`, `AttendanceSubstatusLate`, `AttendanceSubstatusOther`, `AttendanceSubstatusSick`, `ClassArrivalException`, `InstanceStaff`, `InstanceStatusActive`, `InstanceStatusCancelled`, `InstanceStatusCompleted`, `InstanceStatusPlanned`, `InstanceStudentNoteMaxLength`, `MaxTimetableReadRangeDays`, `StaffNoticePriorityImportant`, `StaffNoticePriorityInfo`, `StaffNotice` |
| internal_test | `timetable-activities` / `modules/timetable/http` | `modules/timetable/compose` | `NewInstanceSeriesConversion`, `NewOperations`, `NewTenantTransactions`, `NewTimetableData`, `Repositories` |
| internal_test | `timetable-activities` / `modules/timetable/internal/application` | `models/activities` | `CategoryRepository`, `ErrNotFound`, `GroupRepository`, `GroupTargetRepository`, `NullInt64`, `ScheduleRepository`, `StudentEnrollmentRepository`, `SupervisorPlannedRepository`, `TemplateCapacityOccurrence`, `TemplateFieldsUpdate`, `TemplateListRow`, `TemplateStartTime` |
| internal_test | `timetable-activities` / `modules/timetable/internal/application` | `models/schedule` | `ActivityCompletionSnapshot`, `ActivityExceptionCancelled`, `ActivityExceptionModified`, `ActivityExceptionRepository`, `ActivityException`, `ActivityInstanceRepository`, `ActivityInstance`, `AttendanceStatusAbsent`, `AttendanceStatusExpected`, `AttendanceStatusPresent`, `AttendanceSubstatusExcused`, `AttendanceSubstatusLate`, `AttendanceSubstatusSick`, `CalendarPeriodRepository`, `CalendarPeriod`, `Date`, `Dateframe`, `InstanceStaffRepository`, `InstanceStaff`, `InstanceStatusActive`, `InstanceStatusCancelled`, `InstanceStatusCompleted`, `InstanceStatusPlanned`, `InstanceStudentRepository`, `InstanceStudent`, `NewDate`, `PeriodTypeHoliday`, `PeriodTypeSchoolYear`, `StaffNoticeAck`, `StaffNoticeAudienceAll`, `StaffNoticeAudienceLehrkraft`, `StaffNoticeAudienceStaff`, `StaffNoticePriorityInfo`, `StaffNotice`, `StaffShift`, `StudentInstanceRef`, `TimeframeRepository`, `Timeframe`, `WeekPatternA`, `WeekPatternB` |
| production | `root-composition` / `api` | `modules/timetable/compose` | `ActivityInstanceDate`, `CalendarRows`, `CareScheduleRepositories`, `Dependencies`, `FindCalendarAssignments`, `FindCalendarInstances`, `FindCalendarShifts`, `ListCalendarShiftTypes`, `NativeMirrorRows`, `NewActivityCategorySeeder`, `NewActivityGroupRecords`, `NewActivityInstanceRepository`, `NewAttendanceCorrections`, `NewAttendanceHistory`, `NewAttendanceMirror`, `NewBlockDisplayMetadata`, `NewCalendarPeriodRepository`, `NewCallerActivityGroups`, `NewCareArrivalExceptionRows`, `NewCareArrivalScheduleRows`, `NewCarePickupExceptionRows`, `NewCarePickupNoteRows`, `NewCarePickupScheduleRows`, `NewCareScheduleRepositories`, `NewCatalog`, `NewCategoryCount`, `NewClassArrivalExceptionRepository`, `NewClassArrivalQueries`, `NewClassArrivals`, `NewClosingDayRepository`, `NewConflictDetection`, `NewCourseGroupQueries`, `NewDateframeRepository`, `NewEndedSessionCompletion`, `NewEnrollmentArrivalSchedules`, `NewEnrollmentPickupSchedules`, `NewInstanceLifecycle`, `NewInstanceSeriesConversion`, `NewInstanceStatusSlots`, `NewInstanceStudentRepository`, `NewLifecycleSchedulers`, `NewMaterialization`, `NewNativeMirrorRows`, `NewOffboarding`, `NewOperationLifecycle`, `NewOperations`, `NewPartialAbsencePreview`, `NewPlanningCalendarPeriodUsage`, `NewPlanningTrackAdministration`, `NewPresenceReads`, `NewPresenceSessionFacts`, `NewRecurrenceWriteLock`, `NewRepositories`, `NewRetainedActivityRecoveryRepository`, `NewRosterReconciler`, `NewSchoolCalendarRepositories`, `NewService`, `NewStaffDeviations`, `NewStaffNotices`, `NewSubstituteConflictsOverRows`, `NewTemplateAdministration`, `NewTenantDatabase`, `NewTenantTransactions`, `NewTimetableCleanup`, `NewTimetableData`, `NewTimetableGroupReads`, `NewTimetableInstanceReads`, `NewTimetableInstanceStaffReads`, `NewWorkforceShiftRows`, `NewWorkforceShiftTypeRows`, `New`, `Observation`, `OffboardingDependencies`, `OwnerRooms`, `OwnerRows`, `ParseActivityInstanceDate`, `PartialAbsencePreview`, `Planning`, `PresenceDatabase`, `PresenceReads`, `RecurrenceLockDependencies`, `Repositories`, `RepositoryDependencies`, `RosterStudents`, `SchoolCalendarRepositories`, `ShiftTypeCategoryLinker`, `StaffNoticeDependencies`, `StudentDirectoryFunc`, `StudentDirectory`, `SubstituteDayLock`, `TargetStudent`, `TemplateRows`, `WithMirrorAfterWrite`, `WrapDatabaseError` |
| production | `scheduler-runtime` / `modules/schedulerruntime/compose` | `services/scheduler` | `NewWorker`, `SettingKeys` |
| production | `test-support` / `modules/timetable/timetabletest` | `modules/timetable/compose` | `Dependencies`, `NewPresenceReads`, `New`, `Observation`, `PresenceReads`, `StudentDirectoryFunc`, `StudentDirectory`, `TargetStudent` |
| production | `timetable-activities` / `modules/timetable/compose` | `models/activities` | `CategoryRepository`, `GroupRepository`, `ScheduleRepository`, `StudentEnrollmentRepository`, `SupervisorPlannedRepository` |
| production | `timetable-activities` / `modules/timetable/compose` | `models/schedule` | `ActivityCompletionSnapshot`, `ActivityExceptionRepository`, `ActivityInstanceRepository`, `ActivityRecoveryRepository`, `CalendarPeriodRepository`, `CareScheduleChangeRequestRepository`, `ClassArrivalExceptionRepository`, `ClassArrivalException`, `ClosingDayRepository`, `Date`, `DateframeRepository`, `InstanceIdempotencyRepository`, `InstanceStaffRepository`, `InstanceStudentRepository`, `PartialAbsenceBlock`, `PlanningTrackRepository`, `RecurrenceRuleRepository`, `StaffNoticeRepository`, `StudentArrivalExceptionRepository`, `StudentArrivalNoteRepository`, `StudentArrivalScheduleRepository`, `StudentPickupExceptionRepository`, `StudentPickupNoteRepository`, `StudentPickupScheduleRepository`, `TenantRecurrenceLockKey`, `TimeframeRepository` |
| production | `timetable-activities` / `modules/timetable/internal/adapters/postgres` | `models/schedule` | `ClassArrivalException`, `Date`, `StaffNoticeAck`, `StaffNoticeAudienceAll`, `StaffNoticePriorityImportant`, `StaffNoticeRepository`, `StaffNotice`, `ValidStaffNoticeReader` |
| production | `timetable-activities` / `modules/timetable/internal/application` | `models/activities` | `CategoryRepository`, `GroupRepository`, `GroupTargetRepository`, `NullInt16`, `NullInt64`, `NullString`, `PlannedSupervisionBlocker`, `QueryOptions`, `ScheduleRepository`, `StudentEnrollmentQueryOptions`, `StudentEnrollmentRepository`, `SupervisorPlannedRepository`, `SupervisorQueryOptions`, `TemplateCapacityOccurrence`, `TemplateFieldsUpdate`, `TemplateListRow`, `TemplateStartTime`, `TemplateWeekdayRosterRow`, `WrapDatabaseError`, `WrapNotFoundDatabaseError` |
| production | `timetable-activities` / `modules/timetable/internal/application` | `models/schedule` | `ActivityCompletionSnapshot`, `ActivityExceptionCancelled`, `ActivityExceptionModified`, `ActivityExceptionReasonMaxLength`, `ActivityExceptionRepository`, `ActivityException`, `ActivityInstanceRepository`, `ActivityInstance`, `AttendanceFieldPatch`, `AttendanceStatusAbsent`, `AttendanceStatusExpected`, `AttendanceStatusPresent`, `AttendanceSubstatusExcused`, `AttendanceSubstatusSick`, `CalendarPeriodRepository`, `CalendarPeriodUsage`, `CalendarPeriod`, `CareRequestDecisionSnapshot`, `CareRequestStatusPending`, `CareScheduleChangeRequestRepository`, `CareScheduleChangeRequest`, `ClosingDayRepository`, `ClosingDay`, `CompletionAttendanceSnapshot`, `Date`, `DateframeRepository`, `Dateframe`, `ErrCalendarPeriodNameConflict`, `ErrCareRequestNotDecided`, `ErrCareRequestNotFound`, `ErrCareRequestNotPending`, `InstanceIdempotencyRepository`, `InstanceStaffRepository`, `InstanceStaff`, `InstanceStatusActive`, `InstanceStatusCancelled`, `InstanceStatusCompleted`, `InstanceStatusPlanned`, `InstanceStudentKey`, `InstanceStudentRepository`, `InstanceStudent`, `ParallelPresence`, `ParseDate`, `PartialAbsenceBlock`, `PeriodTypeSchoolYear`, `PlanningTrackRepository`, `PlanningTrack`, `RecurrenceRuleRepository`, `RecurrenceRule`, `ScheduledInstanceRow`, `ShiftType`, `StaffNoticeAudienceAll`, `StaffNoticePriorityInfo`, `StaffNoticeRepository`, `StaffNotice`, `StaffShift`, `StudentArrivalExceptionRepository`, `StudentArrivalException`, `StudentArrivalNoteRepository`, `StudentArrivalNote`, `StudentArrivalScheduleRepository`, `StudentArrivalSchedule`, `StudentInstanceRef`, `StudentPickupExceptionRepository`, `StudentPickupException`, `StudentPickupNoteRepository`, `StudentPickupNote`, `StudentPickupScheduleRepository`, `StudentPickupSchedule`, `TimeframeRepository`, `Timeframe`, `ValidStaffNoticeReader`, `WeekPatternEvery`, `WeekdayFriday`, `WeekdayMonday` |
| production | `timetable-activities` / `modules/timetable/ports` | `models/activities` | `CategoryRepository`, `GroupRepository`, `ScheduleRepository`, `StudentEnrollmentRepository`, `SupervisorPlannedRepository`, `TemplateCapacityOccurrence`, `TemplateListRow`, `TemplateStartTime`, `TemplateWeekdayRosterRow` |
| production | `timetable-activities` / `modules/timetable/ports` | `models/schedule` | `ActivityExceptionRepository`, `ActivityException`, `ActivityInstanceRepository`, `ActivityInstance`, `ActivityRecoveryRepository`, `AttendanceFieldPatch`, `CalendarPeriodRepository`, `CalendarPeriodUsage`, `CalendarPeriod`, `CareScheduleChangeRequestRepository`, `Date`, `InstanceIdempotencyRepository`, `InstanceStaffRepository`, `InstanceStaff`, `InstanceStudentRepository`, `InstanceStudent`, `ParallelPresence`, `ScheduledInstanceRow`, `ShiftType`, `StaffShift`, `StudentArrivalExceptionRepository`, `StudentArrivalException`, `StudentArrivalScheduleRepository`, `StudentInstanceRef`, `StudentPickupExceptionRepository`, `StudentPickupException`, `StudentPickupNoteRepository`, `StudentPickupScheduleRepository`, `TimeframeRepository` |

## Verification

Architecture, all-ticket validation, the full affected suite without `--fast`, lint, deadcode and full hooks are required before publication. Final command outcomes and offline quorum evidence belong in the PR. No route, middleware, response or IoT golden is changed.

---

# Final legacy cluster: round 5

Base: fresh development `b033bd74ff7f0c0c9d3377800818ecf49818f1ee` after PR #4338. The earlier rounds below are historical.

Integrated development: `10591926424937be480dd39ce3c54a44d97cad9a` after PR #4340, including PR #4337. Its architecture principles, export dispatch recovery, strict side-effect guard and budget reductions are preserved. Current before/after measurements use this integrated base.

## Current result

| Surface | Before | After |
|---|---:|---:|
| Legacy keys | 1 | 1 |
| Foreign compose aliases | 23 | 0 |
| Conventional carriers | 5 | 3 |
| Compose LOC | 27,101 | 27,061 |
| Legacy-prefixed LOC | 2,666 | 2,666 |
| Rules | 926 | 928 |
| Current composition field/setter targets | 0 | 0 |

`models/base` and `models/delivery` are gone. Scheduler remains at its original path for the blocker below. `database/repositories/base` was already absent at the base; it still has no package or consumer. The remaining conventional carriers are `models/activities`, `models/schedule` and `services/scheduler`. The retained activity service and every `legacy_*.go` file are outside this round.

The original Data Import authorization key and the zero-baseline regression test are unchanged. The separate zero-ratchet cutover follows #3021. Projection locations and grants are unchanged.

## Native Delivery boundaries

- `mail` defines the actual Message, address, Mailer, Dispatcher and result/error contracts. SMTP retains rendering and transport details; the rendered HTML/text are private adapter state. The existing `email/emailtest` doubles use the native mail values. No alias replaces any of the 23 deleted compose reexports.
- The former root Delivery package mixed intent/provider records and the Module/Worker facade. Actual records move once into `contract`; the facade lives in `runtime`. Both keep their original methods, fields, errors and validation. Consumers name the native definitions, not DTO copies. This separates the public facade from its typed contract instead of reclassifying the old mixed package or granting adapter reach to another existing point.
- `subscriptions` defines the actual tag-free registration value and validation. The persistence interface lives in `internal/ports`; the Postgres adapter still owns its row, ambient transaction, tenant filter and not-found translation. `subscriptiontest` constructs native fixtures for owner suites.
- The root MailTransport aggregate is removed. Builders receive only their positional mail, sender, push and hub dependencies. Outbox bindings use the existing Delivery runtime; a duplicate Delivery test-helper field is deleted. No root field or composition allowance is added.
- Audience selection moves from Delivery compose to its existing Notifications application, offsetting the mail factories. The redundant private subscription constructor and both unique-detector forwarding functions are removed. The Delivery compose cap falls from 774 to 770, its email subpackage cap of 36 disappears, and the total falls by 40. No pass-through budget rises.

## One-to-one relocation proof

| Old reference | New reference | Preserved count / behavior |
|---|---|---|
| Scheduler relocation probe | Reverted to `services/scheduler` | 66 Go files are byte-identical to integrated development; no relocation or notice allowance is added |
| Scheduler discovery and contract inventory | Original paths retained | 27 job IDs and all 7 constructor callers remain unchanged, including the four Worker tests; fixed-commit evidence remains unchanged |
| SQLSTATE skip prefix `models/base/` | `modules/transactionruntime/internal/adapters/postgres/unique_violation.go` | One prefix replaces one prefix and narrows to the canonical file; 4 non-comment SQLSTATE literal hits before and after; empty per-file allowance map stays empty |
| `models/base/db_errors.go` | `modules/transactionruntime/internal/adapters/postgres/unique_violation.go` | Exact bytes after normalizing only `package base` to `package postgres` |
| `database/tenant_runtime.go` | Transaction Runtime private Postgres adapter plus `protocol` errors | Same transaction context keys, role/tenant setup, commit/rollback/retry/savepoint behavior, error text and Unwrap/marker protocol |

The Scheduler contract register and its registry assertions remain unchanged at the original path. The Delivery contract register points to the native intent contract imported by its original public-contract suite. The 28-owner register count is unchanged. These are evidence-target repairs, not checker logic changes.

All route, middleware and IoT goldens, including the source of TestFullProductionRouterGolden, are byte-identical to the base. Test edits are imports, types, constructor calls and field-path repairs only. There is no test deletion or assertion change in this round.

## Foreign importer inventory at the base

The scoped mail/scheduler/base/subscription carriers have only one foreign consumer package: root `api` consumes the old email composition. The Scheduler, detector and subscription value/interface consumers are same-owner. The additional table rows record the existing Delivery facade/record consumers affected by the native contract split. Symbols are selected identifiers at the immutable base, grouped by owner and scope.

| Scope | Consumer owner / package | Old boundary | Selected symbols |
|---|---|---|---|
| external_test | `enrollment` / `modules/enrollment/http` | `modules/delivery` | `EmailIntent`, `EmailRecipient`, `Enqueued`, `Module`, `RelatedEntity`, `TransportEmail` |
| internal_test | `root-composition` / `api` | `modules/delivery` | `EmailIntent`, `EmailRecipient`, `Enqueued`, `Observation`, `RelatedEntity`, `Worker` |
| internal_test | `root-composition` / `api` | `modules/delivery/compose/email` | `Email`, `Message`, `NewEmail`, `TemplateDemoAccess`, `TemplateDemoLead` |
| production | `root-composition` / `api` | `modules/delivery` | `ClaimedIntent`, `DeferredPushIntent`, `EmailDelivery`, `EmailIntent`, `EmailRecipient`, `Enqueued`, `ErrCancelled`, `ErrDeferred`, `ErrIdempotencyConflict`, `GuardianDisplay`, `Module`, `Observation`, `ProviderAcceptedStatusCode`, `ProviderResult`, `PushDispatcher`, `RelatedEntity`, `Retention`, `SchoolSender`, `SenderOrDefault`, `StateCancelled`, `StateDeadLetter`, `StateSent`, `State`, `TransportEmail`, `TransportPush`, `WebPushConfig`, `Worker` |
| production | `root-composition` / `api` | `modules/delivery/compose/email` | `DeliveryMetadata`, `DeliveryObserver`, `DeliveryRequest`, `DeliveryResult`, `DeliveryStatusFailed`, `DeliveryStatusSent`, `Dispatcher`, `Email`, `ErrDeliveryUnavailable`, `IsDemoEnvironment`, `MailerConfig`, `Mailer`, `Message`, `MockMailer`, `NewDispatcher`, `NewEmail`, `NewMailer`, `NewMockMailer`, `ReplyToResolver`, `ResolveReplyToIdentity` |


## Rules

Seven native successor rules are added and five stale legacy-model rules are removed. All new targets are native public contracts; no private reach, data grant, checker exception or increased budget is introduced.

| Added rule | Native successor / precedent |
|---|---|
| `delivery.application.native-contracts` | `platform.application-legacy-model` and `delivery.application.same-owner-adapter`: native values replace old model/transport vocabulary. |
| `delivery.port.native-subscriptions` | The consumer-owned persistence port names the actual native subscription value; ADR 0058 contract direction, no private target. |
| `delivery.postgres.native-subscriptions` | `platform.postgres-legacy-model`: the same subscription values, now native and tag-free; persistence stays private. |
| `delivery.public.native-contracts` | The public Delivery facade names the actual native intent/provider values split from its former mixed contract package. |
| `delivery.test-support.native-contracts` | Existing `email/emailtest` mail doubles now use the actual native mail contract, not adapter aliases. |
| `delivery.workflow-integration-test.native-contracts` | `delivery-platform.workflow-integration-test.delivery-platform-legacy-model`: the same owner fixtures, now native contracts. |
| `transaction-runtime.postgres.native-protocol` | Native transaction-outcome errors replace adapter-local marker/error types; no driver in the contract. |

Removed rules: `delivery-platform.workflow-integration-test.delivery-platform-legacy-model`, `delivery.adapter-test.legacy-model`, `owner-compose.platform.legacy-model`, `platform.application-legacy-model`, `platform.postgres-legacy-model`.

## Verification and issue exits

Architecture must pass with the single unchanged key and zero current composition targets. `validate-ticket --all --base-ref 10591926424937be480dd39ce3c54a44d97cad9a` must report zero removed keys and zero uncovered retirements. Historical evidence whose file moved is pinned to that immutable base URL. The full affected suite, lint, dead-code gates and hooks are required before the authorized corrective follow-up push; final outcomes and offline review are recorded in the PR.

| Issue | Exit evidence |
|---|---|
| #4085 | The foreign compose alias register is empty and its total is 0; no vocabulary/retained alias package remains. |
| #4086 | Not met: two Timetable model carriers, Scheduler and legacy-prefixed implementations remain. |
| #4084 | Not met: retained composition/business and forwarding debt remains outside this round. |
| #4004 | One original key remains by the agreed zero-ratchet sequence after #3021. |

## Exact remaining blocker

| Family | Observed blocker | Safe retained state |
|---|---|---|
| Scheduler internal relocation | PR #4337 introduced `sideEffectRegisterOnlyShrinks` / `sideEffectRegisterSubset` in `backend/test/side_effect_register.go`. Its exact origin/development subset comparison rejects the moved `Scheduler.afterCommit:RegisterAfterCommit@0669f453e5ef89dcbbf01b3a0022619d#1` notice key, even with all 117 entries, fingerprints and reasons identical after one-to-one path normalization. ADR 0020 does not rename this JSON register. The authorized follow-up probe moved all 66 files and changed exactly one key prefix from `services/scheduler/tenant_runtime.go` to `modules/schedulerruntime/internal/application/tenant_runtime.go`. The 117 operations/reasons/fingerprints matched exactly after path normalization. `TestDurableSideEffectsRatchet` still rejected that key alone as an addition, not a changed body fingerprint; the user-directed fallback therefore applies. | The relocation is reverted; all 66 Scheduler files, its 7 constructor callers, 27 jobs and the 117-entry notice register match integrated development. No forwarding indirection, renamed hook, new allowance, checker edit or directory keep-alive is used. |

The first publication also exposed three scanners walking the deleted `services/` root only in clean checkouts; an empty local directory had masked those failures. One-to-one root repairs were verified locally, including 7/7 normalized constructor records. Retaining Scheduler for the independent strict-register blocker restores its real directory, so those extra scanner repairs are reverted too. The canonical detector's file-literal repair remains. The last key and Timetable work are deferred by explicit scope.

---

## Historical round 4

### Round-4 record

Base: development `dd92c93a8af3c4cc3d74d51769eb04d4922c85b8` after PR #4335. This section is historical; round 5 above is current.

Integrated development: `25c8d7368021768e253e1091ad297f01287e494c` (PR #4336). Its durable timeline transaction context/error returns are preserved in the native event contract and the relocated implementation. The report's before column is the initial round-4 base above; pre-push architecture and ticket validation use this integrated development SHA.

## Current result

| Surface | Before | Candidate |
|---|---:|---:|
| Exact legacy keys | 15 | 1 |
| #4004 | 11 | 1 |
| #4076 | 4 | 0 |
| Production composition targets | 0 | 0 |
| Foreign compose aliases | 30 | 23 |
| Conventional production package carriers | 8 | 5 |
| Legacy-tree LOC | 0 | 0 |
| Legacy-prefixed file LOC | 2,897 | 2,666 |
| Compose LOC | 31,160 | 27,190 |
| Policy rules | 902 | 926 |

Architecture passes at one retained key against the fixed base. The checker includes two unchanged test hooks in its combined surface measurement (`2 -> 2`); production remains zero. All 14 removed keys have retirement evidence in `final-legacy-round-four-4004.json` and `final-legacy-round-four-4076.json`. No key, alias, exception, grant, hook allowance, checker behavior or budget is added. Twenty-nine native-contract successor rules replace five stale rules.

## Implemented boundaries

1. The root constructs each Communication audience/inbox projection and the operator dashboard, then injects a positional consumer port. Projection rows are actual native definitions, not aliases or copied DTOs. The implementation owners and existing read grants are unchanged. The physical projection move remains blocked as described below.
2. Timetable HTTP moves to `modules/timetable/http/activities`. Authorization uses the existing Security Runtime public functions. Its transaction function is supplied by the root over the same Tenant runtime. Transport suites move to the existing adapter-test role with unchanged assertions. Activity values and availability behavior move to native owner contracts; CRUD/model compatibility is not republished as public API.
3. Timetable presence reads and Student Deletion enrollment counts receive root-built projection ports. Existing strict ambient-transaction checks stay in immutable root bindings. No consumer composition imports either foreign projection implementation.
4. Data Import internal decision tests construct a consumer-port fake, including `admin:*`, without a compose authorization call. Existing assertions are unchanged. Its production authorization edge is retained unchanged after the zero-key checker blocker below was demonstrated; the native root implementation is reverted, not kept alive as dead code.
5. Delivery owns its Bun subscription row, tenant filter and SQL-not-found translation. `deadcode` proved that the deleted generic CRUD/model API had no production caller. Its obsolete generic suites are deleted under the explicit round-4 authorization. Live error wrapping stays in Transaction Runtime internal postgres. The canonical unique-violation detector remains at its immutable helper-guard path; its original assertions run against that implementation through the existing native runtime facade. Existing Delivery subscription/portal suites and the new native error-chain contract protect the production path.
6. Actual Enrollment offering/availability values move to `offeringcontract`, preserving counts/default and validation behavior. The remaining decoded request state moves into the existing internal application role. The `models/enrollment` carrier is deleted.
7. Actual parent-event orchestration moves to Communication internal application by ADR 0020. Communication's native contract publishes an opaque JSON payload, not `map[string]any`. The payload is genuinely free-form; test support supplies the decoded view used by unchanged Care Plan assertions. Persistence uses `UseNumber` so large integer detail is not rounded during the internal JSON round trip. The `services/parentmessaging` carrier is deleted.
8. Seven foreign compose aliases disappear. Three redundant Communication delivery-recorder methods and seven Timetable reader/clock/end/assignment forwarding methods disappear through promotion of the actual native capabilities. These ten deletions offset the ten newly counted delegations. New HTTP/validation functions are split to the existing size/complexity/facade limits. Every improved budget is lowered; none is transferred to a new path.

## Golden normalization proof

`TestFullProductionRouterGolden` is unchanged. The route and IoT golden files are unchanged. Exactly 6 lines in `middleware_table.golden` change, all by the package path `modules/timetable/compose/httpadapter` to `modules/timetable/http/activities`.

```python
before = git_show("dd92c93a8af3c4cc3d74d51769eb04d4922c85b8:backend/api/testdata/middleware_table.golden")
after = read_bytes("backend/api/testdata/middleware_table.golden")
assert after.replace(b"modules/timetable/http/activities", b"modules/timetable/compose/httpadapter") == before
```

Normalized SHA-256: `8aac8ecab2b289d4672a132a762e8dea6b5f6d2400495c262c178cf322eece45`. The normalized diff is empty: no route, middleware ordering, permission or response change is claimed or accepted.

## Dissolved carriers

| Carrier | Outcome |
|---|---|
| `database/repositories/base` | Dead generic CRUD/context protocol deleted; Delivery owns its live tenant persistence and errors |
| `models/enrollment` | Native offering contract plus existing Enrollment internal application; original behavior suites move with the code |
| `services/parentmessaging` | Communication internal application, same owner/application role and original suite file set (ADR 0020) |

## Exact remaining boundaries

| Surface | Exact blocker / remaining work |
|---|---|
| Zero-key checkpoint | Unchanged `internal/architecture/cli_test.go:790`, `TestCanonicalArchitectureRatchetMatchesCommittedBaseline`, requires the baseline-free canonical check to fail with `imports.forbidden`. With a valid zero-key graph it succeeds, so the test fails. Changing this assertion is a forbidden checker edit. The original live Data Import compose → auth/authorize edge is retained as the single blocked consumer, rather than adding a synthetic violation, alias or keep-alive. |
| Projection physical relocation | Moving the five existing read implementations to their standalone module paths produced 33 new path-bound read-projection grants. ADR 0020 does not rewrite those grants. No grant or checker edit is made. Projection-owned composers therefore remain under the original Go-internal parent (`communication/projections/*`, `organizationtenancy/projections/operatordashboard`); the root still constructs the independent projection owners and injects only ports. |
| `models/base` | Generic model/filter/accessors are deleted, but unchanged `test/helper_consolidation_ratchet_test.go` permits SQLSTATE unique-violation literals only under `models/base/`. Moving the actual canonical detector produced four forbidden literal sites. It remains as the live canonical detector, reached by Transaction Runtime's existing private facade. No split literal, arithmetic encoding, new allowance or checker edit is used. |
| `services/scheduler` | Unchanged `test/composition_inventory_test.go` discovers `services/scheduler/worker.go` by literal path. Moving it requires a checker edit, which is forbidden. Constructor-injected clock/targets and instance emitter state from PR #4334 remain intact. |
| `models/activities` | Actual row/validation values are native now. Remaining callers still name generic repository interfaces, `QueryOptions`, template query rows and error compatibility. A direct public move would publish generic CRUD; a private port/domain move requires the missing external `timetable-activities/workflow-integration-test -> port/domain` reach. ADR 0020 cannot relabel `legacy-model`. Bounded native persistence capabilities and an owner fixture seam remain necessary; no private grant is introduced. |
| `models/schedule` | Timetable composition, application, ports and its external integration suites still construct retained ORM rows/repository contracts (`ActivityInstance`, `InstanceStudent`, recovery/snapshot, recurrence and school-calendar records). Public contracts cannot carry those Bun/generic APIs; external integration tests lack the private domain/port reach. A value/capability/fixture cutover is still required, not a policy-only package rename. |
| `models/delivery` | The live private row is separated from the now base-free subscription value. A public publication attempt needs new `delivery-platform/postgres -> contract` and integration-fixture reach at an existing policy point; the checker rejected it as loosening. No rule/grant is added. The remaining persistence capability needs a native port/fixture cutover before the model carrier can disappear. |
| Delivery email aliases | The 23 remaining foreign aliases are all in `modules/delivery/compose/email`. They expose actual provider `Message`, `Mailer`, `Dispatcher`, SMTP construction and test-mock types from `email` (adapter). Removing the facade requires native mail contracts and a test-support seam; redirecting roots to the private adapter would add private reach. These are retained explicitly, not claimed retired. |
| Compose and legacy files | There are still 27,190 compose LOC and 2,666 legacy-prefixed LOC. In particular Timetable's retained activity service, repository bindings and model contracts remain business/persistence code. Their removal depends on the bounded capability cutover above. They are not renamed outside the counted trees. |

## Round-4 baseline foreign-importer inventory

AST selector inventory at the immutable round-4 base. Own-owner callers are excluded from this foreign-owner table. Candidate imports of the five conventional carriers are all same-owner; their role/fixture blockers are listed above. Shared runtime rows are limited to the retiring consumers.

| Target | Foreign importer owner / package | Scope | Symbols |
|---|---|---|---|
| `api/common` | `timetable-activities` / `modules/timetable/compose/httpadapter` | production | `BindAction`, `CodeTimetableActivityNotOwner`, `CodeTimetableActivitySystemProtected`, `CodeTimetableActivityTemplateProtected`, `CodeTimetableOnlySupervisorReplacementRequired`, `DateFormatISO`, `ErrorBusinessRejectionOr`, `ErrorConflict`, `ErrorConflictWithCode`, `ErrorForbidden`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorRule`, `IDFetch`, `Middleware`, `MsgInvalidActivityID`, `MsgInvalidStudentID`, `MsgInvalidWeekday`, `PaginationParams`, `ParseID`, `ParseIDParam`, `ParsePagination`, `RenderError`, `RequiresAnyPermission`, `RequiresPermission`, `Respond`, `RespondPaginated`, `RespondWithError`, `RulesRenderer`, `Time`, `UnwrapRenderer` |
| `auth/authorize` | `data-import` / `modules/dataimport/compose` | production | `CanGrantRole`, `HasPermission` |
| `auth/authorize` | `timetable-activities` / `modules/timetable/compose/httpadapter` | production | `CanReadStudent`, `HasAdminWildcard` |
| `database/repositories/base` | `delivery-platform` / `modules/delivery/internal/adapters/deliverypostgres` | production | `EnsureTenantID`, `GetDB`, `NewRepository`, `Repository`, `TranslateNotFound`, `WithTenantFilter` |
| `models/base` | `delivery-platform` / `models/delivery` | production | `CRUDRepository`, `Model`, `TenantModel` |
| `models/enrollment` | `care-plan` / `modules/careplan/contracttest` | external_test | `AvailabilityMatchAll`, `AvailabilityOperatorIn`, `AvailabilitySourceGradeLevel`, `CareOffering`, `CareOfferingAvailabilityCondition`, `CareOfferingAvailabilityRule`, `ChildStatusApproved`, `ChildStatusSubmitted`, `DaysOfWeekModeFixed`, `DaysOfWeekModeParentChoice`, `OfferingChangeDate`, `OfferingChangeSnapshotEntry`, `OfferingChangeStatusApproved`, `OfferingChangeStatusPending`, `OfferingChangeStatusRejected`, `OfferingChangeStatusWithdrawn`, `PhaseCareOfferingSelectionAtLeastOne`, `PhaseCareOverflowWaitlist`, `PhaseKindSchoolYear`, `SelectionRuleAtLeastOne`, `SelectionRuleAtMostOne`, `SelectionRuleExactlyOne`, `SelectionRuleOptional` |
| `modules/communication/internal/adapters/audience` | `communication` / `modules/communication/composition` | production | `Announcement`, `Map`, `New`, `Rows`, `Stats`, `Target`, `ViewDetail` |
| `modules/communication/internal/adapters/parentaudience` | `communication` / `modules/communication/parentstore` | production | `Database`, `DeclarationChild`, `DeclarationHistoryLink`, `DeclarationSigner`, `New`, `ParentAnnouncementFeedItem`, `ParentAnnouncementFeedScope`, `ParentAnnouncementReminderRecipient`, `PendingAnnouncementApplicant`, `Projection` |
| `modules/communication/internal/adapters/parentinbox` | `communication` / `modules/communication/parentstore` | production | `Database`, `New`, `ParentInboxThread`, `ParentReadCursor`, `Projection` |
| `modules/communication/internal/adapters/staffinbox` | `communication` / `modules/communication/staffstore` | production | `Database`, `New`, `Projection` |
| `modules/identityaccess/sessiontoken` | `timetable-activities` / `modules/timetable/compose/httpadapter` | production | `PermissionsFromCtx` |
| `modules/organizationtenancy/internal/adapters/operatordashboard` | `organization-tenancy` / `modules/organizationtenancy/compose` | production | `Counts`, `Map`, `New`, `OrganizationSummary`, `PWAUsageRow`, `Rows`, `SchoolSummary` |
| `modules/presenceprojection` | `timetable-activities` / `modules/timetable/compose` | production | `CountNonAbsentParticipants`, `HasPlannedSlotsInRange`, `Instance`, `LegacyInstanceFilter`, `LegacyParticipantFilter`, `ListLegacyInstances`, `ListLegacyParticipants`, `ListParallelPresence`, `ListPartialAbsenceBlocks`, `ListScheduledInstancesForStudent`, `ParallelPresence`, `PartialAbsenceBlock`, `Participant`, `ScheduledInstance` |
| `modules/timetable/compose/httpadapter` | `root-composition` / `api` | production | `NewResource`, `NewSchedulesResource`, `TimeframeChangeGuard` |
| `modules/timetableprojection` | `student-deletion` / `workflows/studentdeletion/compose` | production | `CountStudentEnrollments` |
| `services/parentmessaging` | `care-plan` / `modules/careplan/contracttest` | external_test | `ChildEvent`, `Emitter`, `NewEmitterWithDecisions`, `PortalGuardian`, `TenantSettingsResolver` |

## Native successor rules

All new rules target native public/contract values, or the exact existing HTTP runtime targets of moved transport suites. No private reach is added. HTTP follows PR #4321's owner-http pattern. Fresh owner contract points follow ADR 0058 and the same native successor pattern used by PR #4335; projection/domain-to-native-public directions are pre-approved in the task. Test rules preserve scope and pin the moved source/target where applicable.

| Added rule | Precedent / reason |
|---|---|
| `parent-announcement-audience.postgres.parentannouncementaudience.native-contract` | Retired Communication/Organisation private-projection key; same projection owner publishes actual native public rows, root binds ports; ADR 0058 public directions |
| `communication.compose.parentannouncementaudience.native-contract` | Retired Communication/Organisation private-projection key; same projection owner publishes actual native public rows, root binds ports; ADR 0058 public directions |
| `parent-message-inbox.postgres.parentmessageinbox.native-contract` | Retired Communication/Organisation private-projection key; same projection owner publishes actual native public rows, root binds ports; ADR 0058 public directions |
| `communication.compose.parentmessageinbox.native-contract` | Retired Communication/Organisation private-projection key; same projection owner publishes actual native public rows, root binds ports; ADR 0058 public directions |
| `staff-message-inbox.postgres.staffmessageinbox.native-contract` | Retired Communication/Organisation private-projection key; same projection owner publishes actual native public rows, root binds ports; ADR 0058 public directions |
| `communication.compose.staffmessageinbox.native-contract` | Retired Communication/Organisation private-projection key; same projection owner publishes actual native public rows, root binds ports; ADR 0058 public directions |
| `operator-dashboard-view.postgres.native-contract` | Retired Communication/Organisation private-projection key; same projection owner publishes actual native public rows, root binds ports; ADR 0058 public directions |
| `staff-announcement-audience.postgres.staff-audience.native-public` | Retired Communication/Organisation private-projection key; same projection owner publishes actual native public rows, root binds ports; ADR 0058 public directions |
| `communication.compose.staff-audience.native-public` | Retired Communication/Organisation private-projection key; same projection owner publishes actual native public rows, root binds ports; ADR 0058 public directions |
| `care-plan.contracttest.enrollment-offering-contract` | Retired Care Plan legacy-model test key / existing Enrollment own-value validation and error identity; fresh native contract point |
| `enrollment.application.offering-contract` | Retired Care Plan legacy-model test key / existing Enrollment own-value validation and error identity; fresh native contract point |
| `enrollment.public.offering-contract` | Retired Care Plan legacy-model test key / existing Enrollment own-value validation and error identity; fresh native contract point |
| `communication.application.native-event-contract` | Retired Care Plan parentmessaging test key / same Communication event capability; fresh typed native contract, existing owner public event port |
| `communication.adapter.native-event-contract` | Retired Care Plan parentmessaging test key / same Communication event capability; fresh typed native contract, existing owner public event port |
| `communication.test-support.native-event-contract` | Retired Care Plan parentmessaging test key / same Communication event capability; fresh typed native contract, existing owner public event port |
| `care-plan.e2e-test.native-event-contract` | Retired Care Plan parentmessaging test key / same Communication event capability; fresh typed native contract, existing owner public event port |
| `communication.event-contract.public` | Retired Care Plan parentmessaging test key / same Communication event capability; fresh typed native contract, existing owner public event port |
| `timetable-activities.application.production.activity-contract` | Existing Timetable own-model and compose activity capability; fresh native contract values, no generic CRUD/ORM; owner HTTP pattern #4321 |
| `timetable-activities.legacy-model.production.activity-contract` | Existing Timetable own-model and compose activity capability; fresh native contract values, no generic CRUD/ORM; owner HTTP pattern #4321 |
| `timetable-activities.workflow-integration-test.internal_test.activity-contract` | Existing Timetable own-model and compose activity capability; fresh native contract values, no generic CRUD/ORM; owner HTTP pattern #4321 |
| `timetable-activities.activity-contract.public` | Existing Timetable own-model and compose activity capability; fresh native contract values, no generic CRUD/ORM; owner HTTP pattern #4321 |
| `timetable-activities.http.activity-contract` | Existing Timetable own-model and compose activity capability; fresh native contract values, no generic CRUD/ORM; owner HTTP pattern #4321 |
| `presence-legacy-view.postgres.native-presence-projection` | Retired Timetable presence key; actual native projection rows and consumer-owned read port, root binding; ADR 0058 public directions |
| `timetable-activities.application.native-presence-projection` | Retired Timetable presence key; actual native projection rows and consumer-owned read port, root binding; ADR 0058 public directions |
| `timetable-activities.port.native-presence-projection` | Retired Timetable presence key; actual native projection rows and consumer-owned read port, root binding; ADR 0058 public directions |
| `timetable-activities.port.activity-contract` | Existing Timetable own-model and compose activity capability; fresh native contract values, no generic CRUD/ORM; owner HTTP pattern #4321 |
| `timetable-activities.http.adapter-test.common` | Existing Timetable HTTP test runtime target and external-test scope; moved adapter-test role as PR #4321; exact target package |
| `timetable-activities.http.adapter-test.identity-runtime` | Existing Timetable HTTP test runtime target and external-test scope; moved adapter-test role as PR #4321; exact target package |
| `root-composition.internal-test.activity-contract` | Existing root internal tests constructed the same activity capability through compose; native contract only |

## Verification and issue exits

- Fixed-base architecture: passed, 1 retained key and unchanged production composition 0.
- Retirement validation: passed, all 14 removed keys covered; historical evidence paths for moved files use immutable base links.
- Source ratchets: passed after lowering the improved entries; no new budget entry.
- Full affected tests: passed all 221 affected packages without `--fast`, using `/tmp/orch2580/heavy.sh` and `GOCACHE=/tmp/gocache-final`.
- Full lint: 0 issues. Test-entry and production dead-code gates pass after deletion of the now-unused generic activity model/accessors.
- Normal pre-commit hooks pass. Full pre-push hooks are required before publication and their final outcome is reported with the PR.

| Issue | Exit evidence / intended state |
|---|---|
| #4004 | Ten keys retire; one original Data Import authorization key remains because the immutable canonical regression gate requires at least one forbidden import; open |
| #4076 | All 4 remaining carrier keys retire; no keys for this issue remain; the full affected tests, lint and retirement evidence pass |
| #4084 | Not met: composition still contains retained business/persistence code; open |
| #4085 | Not met: alias ratchet is 23, not 0; open |
| #4086 | Not met: five conventional production carriers, legacy-prefixed files and non-internal implementations remain; open |

The exits established by PR #4334 and PR #4335 remain intact. No broad graph, mutable injection setter or production composition target is restored.

## Historical round-3 record

### Final legacy cluster: round 3

Base: development `d8708d82c7c8e18774acc3ee800014649560d666` after PR #4334 and the clean integration of PR #4333. This is a partial contraction, not proof of the whole final-cluster exit.

## Result

| Surface | Before | Candidate |
|---|---:|---:|
| Exact legacy keys | 33 | 15 |
| #4004 | 22 | 11 |
| #4076 | 8 | 4 |
| #4077 | 3 | 0 |
| Production composition field/setter targets | 0 | 0 |
| Foreign compose aliases | 37 | 30 |
| Remaining package carriers from this round's 13 | 13 | 8 |
| Counted legacy-tree LOC | 547 | 0 |
| Counted compose LOC | 31,884 | 31,160 |
| Policy rules | 900 | 902 |

No new exact key, alias, exception, grant, hook allowance, checker edit or higher budget is introduced. Fifteen native successor rule IDs replace thirteen stale rules; twelve are production rules and three are existing test boundaries with narrower source packages.

## Native cutovers

- Collation is deterministic comparison/ordering code using only stdlib and x/text. Its sync.Pool caches temporary comparison state; it performs no I/O and carries no domain state. The actual package moves to sharedkernel/collation under the existing shared-kernel.contract rule. Seven keys retire, including the three remaining projection keys.
- Facilities handlers move to modules/facilities/http. The root mounts the same handlers, middleware and routes. The old rooms child package merges into the same HTTP role. The golden test and fixtures are unchanged.
- Request Review receives the root-built Care Plan, Enrollment and Timetable values. Actual review vocabulary moves to Care Plan public contracts, not aliases or DTO copies. Six foreign Request Review aliases disappear. Positional root constructors add no composition fields. The existing RLS test hook is narrowed to a test-only proof callback, not extended with new mutable slots.
- Device Scan receives an immutable native principal reader and People Directory error classifier from the root. Existing attendance context helpers stay with Device Fleet authentication. Timetable receives its native OperationCaller port from the root; its existing HTTP test binding supplies that caller without importing api from an internal test package.
- The Delivery/base cutover was reverted after the full dead-code gate exposed the entire generic repository as test-only, plus unused generic model accessors. Removing that obsolete implementation requires rewriting or deleting its generic behavioral suites, which this slice may not do. No keep-alive call, test-only relocation or checker exception is introduced; two original generic CRUD/model keys remain. The adapter independently owns its native store-error type, preserving the exact Error text, Unwrap chain and StoreFailure marker, retiring the direct models/base error import without disturbing the generic repository.
- Facilities and Device Fleet reuse their already-existing native public Room and Device values. The old model definitions and identity mappings disappear. Validation and unchanged model suites move with the native values. Device API key/archive JSON omission is preserved. Raw SQL fixture table names remain iot.devices.
- Facilities retires three read conveniences with no production consumers (GetRoom, FindRoomsByCategory, GetAvailableRooms). A test-only adapter preserves their existing assertions over the native Catalog. Its immutable occupancy function implements its read port directly, removing the service forwarding method. These four deleted runtime forwarders offset the four newly counted attendance adapters. Device Fleet presence mapping moves from compose to the actual application reader, offsetting constructor LOC. The color validator now uses the existing canonical reserved-color predicate, and obsolete row-copy helpers leave production.

## Package carriers

| Deleted carrier | Native destination / outcome |
|---|---|
| database/repositories/pwausage | observability/internal/adapters/pwausage, ADR 0020 same owner/postgres role; the existing Observability module root and composer remain in observability/, avoiding a new counted compose tree |
| models/facilities | Existing modules/facilities public Room and colors; obsolete repository/interface re-exports deleted |
| models/iot | Existing modules/devicefleet public Device; actual validation methods and suites move with it; obsolete repository interface deleted |
| modules/devicefleet/compose/legacy | Actual dashboard mapping/session-device policy in Device Fleet internal application; factories in the existing compose budget |
| modules/facilities/compose/legacy | Actual projections, settings handlers, attendance mapping and system-space adapters in the existing internal application; native public projection ports and existing compose factories |
| Retained carrier | Exact remaining boundary |
|---|---|
| database/repositories/base | Removing its last Delivery dependency produced 77 production dead-code findings outside test support, including the whole generic repository and model filter/accessor API. Deleting the generic implementation would require a non-mechanical rewrite/deletion of the preserved generic repository suites. The Delivery/base family is reverted rather than adding reach, keep-alives or an exclusion. |
| models/activities | Timetable HTTP integration tests cannot acquire timetable-activities/workflow-integration-test -> domain reach. ADR 0020 cannot change legacy-model into domain. Native fixture contracts are still needed. |
| models/schedule | Same Timetable native-fixture boundary; moving the model package alone is rejected. |
| models/base | Bare database/tenant_runtime.go and retained ambient error/transaction contracts still consume it. Move the actual runtime protocol/errors to native contracts before internalizing; no alias or legacy-model reclassification. |
| models/delivery | A public contract cannot carry ORM metadata or generic Upsert. Delivery postgres has own-domain reach, not own public/port reach. The native private row split also exposes external fixture-role reach. No new private grant is introduced. |
| models/enrollment | Care Plan fixtures require a native construction adapter preserving counts/default and availability behavior, as detailed below. |
| services/parentmessaging | Typed native payload and native test-support boundary required; public map leaks and private reach are not allowed. |
| services/scheduler | discoverWorkerJobIDs/discoverRequiredWorkerJobIDs in unchanged test/composition_inventory_test.go hard-code services/scheduler and worker.go. Relocation would require a forbidden checker edit. Constructor injection/instance state from PR #4334 remains intact. |

No legacy/ production directory remains. The existing legacy_*.go inventory remains at 2,897 LOC; this round does not claim its exit.

## Remaining exact keys and blockers

All fifteen keys remain under their original carriers. No key is transferred to another issue.

| Issue | Scope | Consumer owner and edge | Symbols | Exact blocker |
|---|---|---|---|---|
| #4076 | external_test | `care-plan`: `modules/careplan/contracttest` → `models/enrollment` | `AvailabilityMatchAll`, `AvailabilityOperatorIn`, `AvailabilitySourceGradeLevel`, `CareOffering`, `CareOfferingAvailabilityCondition`, `CareOfferingAvailabilityRule`, `ChildStatusApproved`, `ChildStatusSubmitted`, `DaysOfWeekModeFixed`, `DaysOfWeekModeParentChoice`, `OfferingChangeDate`, `OfferingChangeSnapshotEntry`, `OfferingChangeStatusApproved`, `OfferingChangeStatusPending`, `OfferingChangeStatusRejected`, `OfferingChangeStatusWithdrawn`, `PhaseCareOfferingSelectionAtLeastOne`, `PhaseCareOverflowWaitlist`, `PhaseKindSchoolYear`, `SelectionRuleAtLeastOne`, `SelectionRuleAtMostOne`, `SelectionRuleExactlyOne`, `SelectionRuleOptional` | Care Plan contract fixtures still construct CareOffering with CountsAsCareSet/default semantics and typed availability rules. The native public offering has explicit counts and JSON availability. A fixture-boundary conversion is required; no DTO clone, Enrollment private-domain test grant or test assertion change is used. |
| #4076 | external_test | `care-plan`: `modules/careplan/contracttest` → `services/parentmessaging` | `ChildEvent`, `Emitter`, `NewEmitterWithDecisions`, `PortalGuardian`, `TenantSettingsResolver` | The native-public probe reports contracts.filter-map for ChildEvent.Payload map[string]any (15 findings), plus communication/adapter -> communication/public and Care Plan test -> Communication/public import gaps. The probe is reverted. A typed native event payload and owner test-support seam remain necessary. |
| #4004 | production | `communication`: `modules/communication/composition` → `modules/communication/internal/adapters/audience` | `Announcement`, `Map`, `New`, `Rows`, `Stats`, `Target`, `ViewDetail` | The target is staff-announcement-audience/postgres, not Communication. Its named read projection and grants are path-bound. A layout-only store move does not remove the owner boundary; native projection contracts/root composition are still needed. |
| #4004 | production | `communication`: `modules/communication/parentstore` → `modules/communication/internal/adapters/parentaudience` | `Database`, `DeclarationChild`, `DeclarationHistoryLink`, `DeclarationSigner`, `New`, `ParentAnnouncementFeedItem`, `ParentAnnouncementFeedScope`, `ParentAnnouncementReminderRecipient`, `PendingAnnouncementApplicant`, `Projection` | The target is parent-announcement-audience/postgres, not Communication. Root access requires native projection records and composition without moving or adding the path-bound read grants. |
| #4004 | production | `communication`: `modules/communication/parentstore` → `modules/communication/internal/adapters/parentinbox` | `Database`, `New`, `ParentInboxThread`, `ParentReadCursor`, `Projection` | The target is parent-message-inbox/postgres, not Communication. A private store move alone still imports the foreign projection. No new postgres reach or projection grant is admitted. |
| #4004 | production | `communication`: `modules/communication/staffstore` → `modules/communication/internal/adapters/staffinbox` | `Database`, `New`, `Projection` | The target is staff-message-inbox/postgres, not Communication. The private projection cannot become Communication-owned by a policy-only reclassification. |
| #4004 | production | `data-import`: `modules/dataimport/compose` → `auth/authorize` | `CanGrantRole`, `HasPermission` | Data Import production can bind Security Runtime at the root. Its internal decision suites still construct the canonical authorization through compose. Importing api creates a cycle; a new mutable test slot, broader root or new test/private reach is excluded. The carrier remains, as recorded in the #4004 decision comments. |
| #4004 | production | `organization-tenancy`: `modules/organizationtenancy/compose` → `modules/organizationtenancy/internal/adapters/operatordashboard` | `Counts`, `Map`, `New`, `OrganizationSummary`, `PWAUsageRow`, `Rows`, `SchoolSummary` | The projection reads auth.account_roles/account_tenants/accounts/roles and iot.pwa_standalone_usage, not solely Organisation & Tenancy tables. The approved physical-own-postgres option therefore does not apply. A root-bindable projection contract/composition remains necessary; no reclassification or read grant is added. |
| #4004 | production | `timetable-activities`: `modules/timetable/compose/httpadapter` → `api/common` | `BindAction`, `CodeTimetableActivityNotOwner`, `CodeTimetableActivitySystemProtected`, `CodeTimetableActivityTemplateProtected`, `CodeTimetableOnlySupervisorReplacementRequired`, `DateFormatISO`, `ErrorBusinessRejectionOr`, `ErrorConflict`, `ErrorConflictWithCode`, `ErrorForbidden`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorRule`, `IDFetch`, `Middleware`, `MsgInvalidActivityID`, `MsgInvalidStudentID`, `MsgInvalidWeekday`, `PaginationParams`, `ParseID`, `ParseIDParam`, `ParsePagination`, `RenderError`, `RequiresAnyPermission`, `RequiresPermission`, `Respond`, `RespondPaginated`, `RespondWithError`, `RulesRenderer`, `Time`, `UnwrapRenderer` | Timetable HTTP relocation changes six non-normalized method middleware provenance lines in the unchanged production middleware golden: modules/timetable/compose/httpadapter.(*Resource).requireActivityModification-fm. No golden/checker change or compatibility wrapper is admitted. |
| #4004 | production | `timetable-activities`: `modules/timetable/compose/httpadapter` → `auth/authorize` | `CanReadStudent`, `HasAdminWildcard` | Data Import production can bind Security Runtime at the root. Its internal decision suites still construct the canonical authorization through compose. Importing api creates a cycle; a new mutable test slot, broader root or new test/private reach is excluded. The carrier remains, as recorded in the #4004 decision comments. |
| #4004 | production | `timetable-activities`: `modules/timetable/compose/httpadapter` → `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` | Same Timetable HTTP middleware provenance boundary as the api/common carrier; its three transport keys remain together. |
| #4004 | production | `timetable-activities`: `modules/timetable/compose` → `modules/presenceprojection` | `CountNonAbsentParticipants`, `HasPlannedSlotsInRange`, `Instance`, `LegacyInstanceFilter`, `LegacyParticipantFilter`, `ListLegacyInstances`, `ListLegacyParticipants`, `ListParallelPresence`, `ListPartialAbsenceBlocks`, `ListScheduledInstancesForStudent`, `ParallelPresence`, `PartialAbsenceBlock`, `Participant`, `ScheduledInstance` | The projection root is postgres, not a native contract. The current composition consumes legacy schedule rows and Bun-backed constructor aliases. A native projection port/root constructor must replace those values without a foreign postgres grant. |
| #4004 | production | `student-deletion`: `workflows/studentdeletion/compose` → `modules/timetableprojection` | `CountStudentEnrollments` | The projection root is postgres. Its CountStudentEnrollments read and constructor cannot be passed from the root through a public projection contract that does not yet exist. No compose-to-postgres grant is added. |
| #4076 | production | `delivery-platform`: `models/delivery` → `models/base` | `CRUDRepository`, `Model`, `TenantModel` | Delivery/base cutover reverted after the production dead-code gate exposed the generic repository and model API as test-only. Deleting/replacing their behavioral suites is outside the construction-only test repairs allowed here. |
| #4076 | production | `delivery-platform`: `modules/delivery/internal/adapters/deliverypostgres` → `database/repositories/base` | `EnsureTenantID`, `GetDB`, `NewRepository`, `Repository`, `TranslateNotFound`, `WithTenantFilter` | Same observed dead-code/test-preservation blocker. No exception, fake keep-alive or test-only implementation relocation is used. |

## Native successor rules and precedents

HTTP rules follow the owner HTTP shape used by PR #4321. They are pinned to modules/facilities/http and exact target packages. Internal application readers use the same already-existing composition public-owner directions permitted by ADR 0058. Test successors keep the existing scope/target and narrow the source package to the moved suite.

| Added rule ID | Existing precedent |
|---|---|
| `device-fleet.application.care-plan.public` | `device-fleet.compose.care-plan.public` |
| `device-fleet.application.student-presence.public` | `device-fleet.compose.student-presence-public` |
| `facilities.application.people-directory.public` | `facilities.compose.people-public` |
| `facilities.application.school-membership.public` | `facilities.compose.membership-public` |
| `facilities.application.student-presence.public` | `facilities.compose.student-presence-public` |
| `facilities.http.adapter-test.identity-adapter` | `facilities.integration-test.identity-adapter, same sessiontoken target/external_test scope, exact moved HTTP source` |
| `facilities.http.document-rendering.port` | `facilities.compose.document-records-port` |
| `facilities.http.identity-access.http` | `facilities.integration-test.identity-adapter and retired Facilities → sessiontoken key; #4321 owner HTTP identity shape` |
| `facilities.http.people-directory.public` | `facilities.compose.people-public` |
| `facilities.http.school-structure.contract` | `facilities.compose.school-structure-contract` |
| `facilities.http.security-runtime.contract` | `facilities.compose.security-contract` |
| `facilities.http.student-presence.public` | `facilities.compose.student-presence-public` |
| `facilities.http.tenant-runtime.http` | `Retired Facilities HTTP → api/common key; #4321 owner HTTP transport shape` |
| `facilities.module-internal-test.membership-public` | `facilities.integration-test.membership-public, same public target/internal_test scope, exact roomservice suite` |
| `facilities.module-internal-test.people-public` | `facilities.integration-test.people-public, same public target/internal_test scope, exact roomservice suite` |

## Foreign importer inventory for retained carriers

AST selector inventory across production, internal and external tests. Every foreign package importing one of the eight retained carrier packages is listed by owner, scope and symbol. Same-owner consumers remain relevant to relocation but do not count as foreign importers.

| Carrier | Foreign importer owner / package | Scope | Symbols |
|---|---|---|---|
| `models/enrollment` | `care-plan` / `modules/careplan/contracttest` | external_test | `AvailabilityMatchAll`, `AvailabilityOperatorIn`, `AvailabilitySourceGradeLevel`, `CareOffering`, `CareOfferingAvailabilityCondition`, `CareOfferingAvailabilityRule`, `ChildStatusApproved`, `ChildStatusSubmitted`, `DaysOfWeekModeFixed`, `DaysOfWeekModeParentChoice`, `OfferingChangeDate`, `OfferingChangeSnapshotEntry`, `OfferingChangeStatusApproved`, `OfferingChangeStatusPending`, `OfferingChangeStatusRejected`, `OfferingChangeStatusWithdrawn`, `PhaseCareOfferingSelectionAtLeastOne`, `PhaseCareOverflowWaitlist`, `PhaseKindSchoolYear`, `SelectionRuleAtLeastOne`, `SelectionRuleAtMostOne`, `SelectionRuleExactlyOne`, `SelectionRuleOptional` |
| `services/parentmessaging` | `care-plan` / `modules/careplan/contracttest` | external_test | `ChildEvent`, `Emitter`, `NewEmitterWithDecisions`, `PortalGuardian`, `TenantSettingsResolver` |
| `database/repositories/base` | `delivery-platform` / `modules/delivery/internal/adapters/deliverypostgres` | production | `EnsureTenantID`, `GetDB`, `NewRepository`, `Repository`, `TranslateNotFound`, `WithTenantFilter` |
| `models/base` | `delivery-platform` / `models/delivery` | production | `CRUDRepository`, `Model`, `TenantModel` |

## Verification and issue exits

- Architecture: passed against the full resolved base above, 15 keys and composition 0.
- Focused Delivery persistence/notification/PWA, Request Review RLS/Care Plan, repository relocations, collation race and root principal/caller/golden suites passed before the final integration. After the Delivery/base reversion, the final fixed-build architecture check passed at 15 keys and composition 0. validate-ticket --all covered all 18 retirements. The full 229-package affected-test run passed without --fast using the coordinated heavy runner and /tmp/gocache-final. For the final native Delivery error cut, the full 229-package run, fixed-build architecture at 15 keys/composition 0, all 18 retirement records and full lint passed. Full pre-commit/pre-push hooks, including source ratchets, dead-code, licenses and vulnerabilities, are required on the final published commit.
- Original HTTP golden test and golden files remain unchanged. Existing test assertions remain unchanged; imports, fixture types and construction bindings move mechanically.
- No production request, runtime benchmark, data/schema migration or credential change is part of this PR.

| Issue | Candidate exit / intended state |
|---|---|
| #4004 | Not met: 11 exact keys remain; open |
| #4076 | Not met: 4 exact keys remain; open |
| #4077 | All 3 remaining keys retire through shared-kernel collation. Only name-order imports change, with no projection query/access change. Existing Emergency route foreign-tenant rejection, Group Live TenantIsolation and SupervisionDashboard TenantIsolation suites are required in the full run; those suites passed in the final 229-package publication run, proving the exit |
| #4084 | Not met: counted compose/pass-through debt remains; open |
| #4085 | Not met: 30 foreign compose aliases remain; open |
| #4086 | Not met: 8 package carriers and legacy_*.go remain; open |

PR #4334's #2751 exit remains intact: no legacy key, factory, setter or broad composition is restored.

## Baseline foreign-importer inventory

Reconstructed from immutable development d8708d82c7c8e18774acc3ee800014649560d666 before the cutover. Every foreign package/scope importing the thirteen original carriers, collation or either HTTP adapter is listed with selected symbols. This is the before-state; the candidate inventory and blocker table above are authoritative for remaining work.

| Baseline target | Importer owner / package | Scope | Symbols |
|---|---|---|---|
| `database/repositories/base` | `delivery-platform` / `modules/delivery/internal/adapters/deliverypostgres` | production | `EnsureTenantID`, `GetDB`, `NewRepository`, `Repository`, `TranslateNotFound`, `WithTenantFilter` |
| `internal/collation` | `calendar-view` / `modules/supervisiondashboard/internal/adapters/nameorder` | production | `CompareGerman` |
| `internal/collation` | `emergency-snapshot` / `modules/emergencysnapshot/internal/adapters/nameorder` | production | `CompareGerman` |
| `internal/collation` | `enrollment` / `modules/enrollment/http` | production | `CompareSchoolClasses` |
| `internal/collation` | `facilities` / `modules/facilities/compose/httpadapter` | production | `CompareGerman` |
| `internal/collation` | `group-live-view` / `modules/grouplive/internal/adapters/nameorder` | production | `CompareGerman` |
| `internal/collation` | `people-directory` / `modules/peopledirectory/inbound/students` | production | `CompareGermanNames`, `CompareSchoolClasses` |
| `internal/collation` | `school-membership` / `modules/schoolmembership/internal/adapters/displayorder` | production | `CompareGermanNames`, `CompareSchoolClasses` |
| `models/base` | `delivery-platform` / `models/delivery` | production | `CRUDRepository`, `Model`, `TenantModel` |
| `models/base` | `delivery-platform` / `modules/delivery/internal/adapters/deliverypostgres` | production | `DatabaseError` |
| `models/enrollment` | `care-plan` / `modules/careplan/contracttest` | external_test | `AvailabilityMatchAll`, `AvailabilityOperatorIn`, `AvailabilitySourceGradeLevel`, `CareOffering`, `CareOfferingAvailabilityCondition`, `CareOfferingAvailabilityRule`, `ChildStatusApproved`, `ChildStatusSubmitted`, `DaysOfWeekModeFixed`, `DaysOfWeekModeParentChoice`, `OfferingChangeDate`, `OfferingChangeSnapshotEntry`, `OfferingChangeStatusApproved`, `OfferingChangeStatusPending`, `OfferingChangeStatusRejected`, `OfferingChangeStatusWithdrawn`, `PhaseCareOfferingSelectionAtLeastOne`, `PhaseCareOverflowWaitlist`, `PhaseKindSchoolYear`, `SelectionRuleAtLeastOne`, `SelectionRuleAtMostOne`, `SelectionRuleExactlyOne`, `SelectionRuleOptional` |
| `modules/devicefleet/compose/legacy` | `root-composition` / `api` | internal_test | `DashboardDependencies`, `NewDashboardSources` |
| `modules/devicefleet/compose/legacy` | `root-composition` / `api` | production | `DashboardDependencies`, `NewDashboardSources`, `SessionDeviceIDs`, `SessionDeviceRecords` |
| `modules/facilities/compose/httpadapter` | `root-composition` / `api` | production | `Dependencies`, `NewResource` |
| `modules/facilities/compose/legacy` | `root-composition` / `api` | production | `ActivityGroupName`, `HistoryProjection`, `NewAttendanceRooms`, `OccupancyProjection`, `OpenGroupCatalog`, `RoomOccupancyFact`, `RoomSessionFact`, `SettingHandlers` |
| `modules/timetable/compose/httpadapter` | `root-composition` / `api` | production | `NewResource`, `NewSchedulesResource`, `TimeframeChangeGuard` |
| `services/parentmessaging` | `care-plan` / `modules/careplan/contracttest` | external_test | `ChildEvent`, `Emitter`, `NewEmitterWithDecisions`, `PortalGuardian`, `TenantSettingsResolver` |

