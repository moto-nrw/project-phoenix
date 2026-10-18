# Final legacy cluster: round 5

Base: fresh development `b033bd74ff7f0c0c9d3377800818ecf49818f1ee` after PR #4338. The earlier rounds below are historical.

## Current result

| Surface | Before | After |
|---|---:|---:|
| Legacy keys | 1 | 1 |
| Foreign compose aliases | 23 | 0 |
| Conventional carriers | 5 | 2 |
| Compose LOC | 27,190 | 27,150 |
| Legacy-prefixed LOC | 2,666 | 2,666 |
| Rules | 926 | 928 |
| Current composition field/setter targets | 0 | 0 |

`services/scheduler`, `models/base` and `models/delivery` are gone. `database/repositories/base` was already absent at the base; it still has no package or consumer. Only `models/activities` and `models/schedule` remain as conventional carriers. The retained activity service and every `legacy_*.go` file are outside this round.

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
| `services/scheduler` | `modules/schedulerruntime/internal/application` | 66 Go files, identical file set and normalized bytes; same owner/application/test roles via ADR 0020 |
| Worker registry directory and `services/scheduler/worker.go` literals in composition discovery | Corresponding internal Scheduler directory and `worker.go` | 27 job IDs before and after, identical order; constructor identities and fixed-commit evidence remain unchanged |
| SQLSTATE skip prefix `models/base/` | `modules/transactionruntime/internal/adapters/postgres/` | One prefix replaces one prefix; 4 non-comment SQLSTATE literal hits before and after; empty per-file allowance map stays empty |
| `models/base/db_errors.go` | `modules/transactionruntime/internal/adapters/postgres/unique_violation.go` | Exact bytes after normalizing only `package base` to `package postgres` |
| `database/tenant_runtime.go` | Transaction Runtime private Postgres adapter plus `protocol` errors | Same transaction context keys, role/tenant setup, commit/rollback/retry/savepoint behavior, error text and Unwrap/marker protocol |

The Scheduler contract register now points to its existing public lease contract test. Its original registry contract assertions still run unchanged in the relocated internal suite. The Delivery contract register points to the native intent contract imported by its original public-contract suite. The 28-owner register count is unchanged. These are evidence-target repairs, not checker logic changes.

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

Architecture must pass with the single unchanged key and zero current composition targets. `validate-ticket --all --base-ref b033bd74ff7f0c0c9d3377800818ecf49818f1ee` must report zero removed keys and zero uncovered retirements. Historical evidence whose file moved is pinned to that immutable base URL. The full affected suite, lint, dead-code gates and hooks are required before the single publication push; final outcomes and offline review are recorded in the PR.

| Issue | Exit evidence |
|---|---|
| #4085 | The foreign compose alias register is empty and its total is 0; no vocabulary/retained alias package remains. |
| #4086 | Not met: two Timetable model carriers and legacy-prefixed implementations remain for round 6. |
| #4084 | Not met: retained composition/business and forwarding debt remains outside this round. |
| #4004 | One original key remains by the agreed zero-ratchet sequence after #3021. |

No scoped family remains blocked. The last key and Timetable work are deferred by explicit scope, not hidden through allowances.

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
