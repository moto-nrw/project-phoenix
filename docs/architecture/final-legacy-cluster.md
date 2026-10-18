# Final legacy cluster: round 3

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
