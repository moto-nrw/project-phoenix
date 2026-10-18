# Final legacy cluster

Integrated development base: `2dc7af533610495387765d0f78c8d84af75d866b` (PRs #4330 and #4332). Original inventory base: `a51ece49deb11a42951491853e85a7968a2d39ff`. Branch: `final-legacy-cluster`.

## Candidate result

This PR retires **7 keys relative to merged development: 40 → 33**. The original 42-key inventory also includes the two keys retired by PR #4330. This is a partial final-cluster implementation, not a claim that all carrier exits are met.

| Carrier | Original inventory | Merged development | Candidate | This PR retires |
|---|---:|---:|---:|---:|
| #4004 | 29 | 27 | 22 | 5 |
| #4076 | 8 | 8 | 8 | 0 |
| #4077 | 3 | 3 | 3 | 0 |
| #2751 | 2 | 2 | 0 | 2 |

Composition stays **0**. The fixed-build cross-policy comparison reports `36 → 0` because the old classifier treats newly native foreign dependency types in root bundles as composition; 36 is not the merged base’s stock surface. No service graph or new root composition field returns. Foreign composition aliases decrease **124 → 37**. Rules change **901 → 902**; the nine native successors below add no exception or private-role grant.

## Completed owner boundaries

1. **Scheduler:** application code owns its clock and per-instance overdue emission state. Five test-injection targets become construction values; emitter-day rotation is a method on instance state. Job phases, fail-closed ordering, logs and all assertions stay unchanged.
2. **Data Import:** private application, native construction ports and real constructors replace `services/import`, `import/ports` and `compose/retained`. Integration suites live in the existing owner test-support role at `importtest`. Decision-test functional adapters stay test-only beside private decision tests, so there is no new test-support grant. Twelve fake mutations become constructor-bound functions with the original recording state and assertions. Batch admission, retry attempts and checkpoint receipts are explicit steps. The existing authorization factory remains: moving its canonical policy would require new reach for the private decision tests. Both the root-bound and native-public probes are reverted.
3. **Settings:** native opaque snapshots, errors, observation functions and constructor ports live in Settings records. Four DB suites move with the private service, removing the config-repository cycle. Workforce uses Settings test support. Immutable construction keeps PR #4330’s removal of side-effect registration/observer setters. Defaults are ordered native definition tables. Registry access policy and operator capabilities replace forwarding methods.
4. **Document Rendering:** the actual list/record values and renderer port live in native records. Facilities consumes them. Renderers, assets and tests are private application code; XLSX/PDF output behavior and assertions stay unchanged.
5. **Staff Clock:** the workflow publishes the actual command/error contract. `StampError.Error()` still returns `Cause.Error()`. The public boundary now reads clock/day/open-session state directly, removing the old state delegation and offsetting the counted error delegation.
6. **Earlier branch work:** People Directory/Care Plan private rows, Appointments values, Audit command/scan adapter, Transaction Runtime statistics, Care Plan parent persistence, root-bound School Setup/Staff Offboarding dependencies and native Presence room values remain. Delivery’s realtime alias package is gone.

`models` contains only test-package documentation, with no import, provider, factory, setter or broad composition. All eight original conventional-entity assertions live beside People Directory rows with the same structural method set. Both remaining keys of #2751 are absent.

## Native successor rules

Every added rule is production-only; no test-support rule, hook allowance, checker change or wider budget is added.

| Rule | Precedent and narrowed contract |
|---|---|
| `data-import.port.audit-consents` | `data-import.application.audit-consent-contract`: the same native consent types now occur in constructor ports. |
| `data-import.port.native-public` | `data-import.application.public`: native import runtime/row values are named by the constructor port, not an implementation alias. |
| `document-rendering.contract.document-records-port` | `document-rendering.contract.list-renderer`: only native records replace application vocabulary. |
| `document-rendering.public.document-records-port` | `platform.public-application`: the public renderer names its own records port instead of application types. |
| `facilities.compose.document-records-port` | The retired Facilities HTTP adapter → list renderer key and established renderer contract reach; only native document values/renderer port are exposed. |
| `staff-clock.application.public` | The existing Staff Clock application definitions become the workflow’s native public command/error contract. |
| `staff-clock.to.process-device-scan.native-contract` | `staff-clock.to.process-device-scan`: the same device-scan public types move into the native workflow contract. |
| `staff-clock.to.workforce.native-contract` | `staff-clock.to.workforce`: the same Workforce public types move into the native workflow contract. |
| `transaction-runtime.public.stats-port` | `platform.public-application`: native statistics port replaces public implementation aliases; explicitly accepted for this branch. |

## Reverted native-model probes

These are observed failures against both the original inventory base and the merged development base, not proposed allowances. The four model moves and their proposed rules were reverted. No failed rule remains in `policy.json`.

| Retained package | Consumer / exact rejected reach | Consequence |
|---|---|---|
| `models/activities` | `modules/timetable/compose/httpintegration`, external test: `timetable-activities/workflow-integration-test → timetable-activities/domain` | Native domain import is rejected. Replacing the legacy-model test rule with the native domain point also exposes existing `internal/domain` packages and is rejected as loosening. Tests need owner-native fixture contracts, not a private-row grant. |
| `models/schedule` | Same Timetable HTTP integration seam | Same native-fixture prerequisite; changing only the package location is insufficient. |
| `models/iot` | `modules/devicefleet/compose/httpadapter`, external test: `device-fleet/workflow-integration-test → device-fleet/domain` | Even a target-package-restricted native successor is rejected by the existing role-point guard. Mutable retained-repository fixtures remain. |
| `models/facilities` | `compose/legacy` and `compose/repositoryadapter`, internal test: `facilities/workflow-integration-test → facilities/domain`; native rows also require `facilities/domain → facilities/public` | Both rules are rejected as new permissions; keep the original model and consumers. |

## Round 2 blockers and retained work

| Family / consumer | Exact blocker or remaining work |
|---|---|
| Scheduler package path | `discoverWorkerJobIDs` and `discoverRequiredWorkerJobIDs` in the unchanged `test/composition_inventory_test.go` hard-code `services/scheduler`. Its constructor-injection and per-instance emission-state work stays, but the package relocation is reverted rather than editing the checker. |
| Data Import authorization | Native compose/public policy probes require `data-import → security-runtime/public` rules rejected as loosening. A root-only binding leaves private decision tests without their canonical policy unless a new test grant or broad test root is added. The original consumer remains, with no new key or rule. |
| PWA usage repository | Moving `observability/compose` into `modules/observability/compose` would make the private repository reachable, but creates a new counted compose tree with no budget entry. A new compose-budget entry is forbidden. Existing paths remain; no rule or allowance is added. |
| Native model probes | The table above records the observed private-domain test-role rejections. The probes compile but are reverted. Integration fixtures must first use native owner contracts; no private-row test grant is added. |
| Device Fleet / Facilities `compose/legacy` | The merged roots remove the old service-graph prerequisite, but current callers still name legacy composer contracts. Constructors must move into existing composition budgets while actual facts/ports become owner-native. These trees remain; this round does not claim their exit. |
| Remaining glue and generic carriers | The exact-key table and foreign importer inventory below name the surviving consumers and symbols. No new allowance is used to hide them. |

## Remaining exact-key consumers

| Carrier | Scope | Consumer owner and edge | Imported symbols | Required boundary |
|---|---|---|---|---|
| #4076 | `external_test` | `care-plan`: `modules/careplan/contracttest` → `models/enrollment` | `AvailabilityMatchAll`, `AvailabilityOperatorIn`, `AvailabilitySourceGradeLevel`, `CareOffering`, `CareOfferingAvailabilityCondition`, `CareOfferingAvailabilityRule`, `ChildStatusApproved`, `ChildStatusSubmitted`, `DaysOfWeekModeFixed`, `DaysOfWeekModeParentChoice`, `OfferingChangeDate`, `OfferingChangeSnapshotEntry`, `OfferingChangeStatusApproved`, `OfferingChangeStatusPending`, `OfferingChangeStatusRejected`, `OfferingChangeStatusWithdrawn`, `PhaseCareOfferingSelectionAtLeastOne`, `PhaseCareOverflowWaitlist`, `PhaseKindSchoolYear`, `SelectionRuleAtLeastOne`, `SelectionRuleAtMostOne`, `SelectionRuleExactlyOne`, `SelectionRuleOptional` | Care Plan native offering fixtures; Care Plan cannot acquire an Enrollment dependency against the DAG. |
| #4076 | `external_test` | `care-plan`: `modules/careplan/contracttest` → `services/parentmessaging` | `ChildEvent`, `Emitter`, `NewEmitterWithDecisions`, `PortalGuardian`, `TenantSettingsResolver` | Communication-native emitter/event contract; preserve nil behavior, payloads and decision delivery. |
| #4076 | `production` | `delivery-platform`: `models/delivery` → `models/base` | `CRUDRepository`, `Model`, `TenantModel` | Replace Delivery generic CRUD/row/error dependencies with owner-native persistence and error contracts before internalization. |
| #4004 | `production` | `communication`: `modules/communication/composition` → `modules/communication/internal/adapters/audience` | `Announcement`, `Map`, `New`, `Rows`, `Stats`, `Target`, `ViewDetail` | Projection-native public contracts or a root-bound port. Existing read-projection paths/grants must not be widened. |
| #4004 | `production` | `communication`: `modules/communication/parentstore` → `modules/communication/internal/adapters/parentaudience` | `Database`, `DeclarationChild`, `DeclarationHistoryLink`, `DeclarationSigner`, `New`, `ParentAnnouncementFeedItem`, `ParentAnnouncementFeedScope`, `ParentAnnouncementReminderRecipient`, `PendingAnnouncementApplicant`, `Projection` | Projection-native public contracts or a root-bound port. Existing read-projection paths/grants must not be widened. |
| #4004 | `production` | `communication`: `modules/communication/parentstore` → `modules/communication/internal/adapters/parentinbox` | `Database`, `New`, `ParentInboxThread`, `ParentReadCursor`, `Projection` | Projection-native public contracts or a root-bound port. Existing read-projection paths/grants must not be widened. |
| #4004 | `production` | `communication`: `modules/communication/staffstore` → `modules/communication/internal/adapters/staffinbox` | `Database`, `New`, `Projection` | Projection-native public contracts or a root-bound port. Existing read-projection paths/grants must not be widened. |
| #4004 | `production` | `data-import`: `modules/dataimport/compose` → `auth/authorize` | `CanGrantRole`, `HasPermission` | Needs an owner-native public contract or an existing-value root-bound port; no additional private-role reach is admitted. |
| #4076 | `production` | `delivery-platform`: `modules/delivery/internal/adapters/deliverypostgres` → `database/repositories/base` | `EnsureTenantID`, `GetDB`, `NewRepository`, `Repository`, `TranslateNotFound`, `WithTenantFilter` | Replace Delivery generic CRUD/row/error dependencies with owner-native persistence and error contracts before internalization. |
| #4076 | `production` | `delivery-platform`: `modules/delivery/internal/adapters/deliverypostgres` → `models/base` | `DatabaseError` | Replace Delivery generic CRUD/row/error dependencies with owner-native persistence and error contracts before internalization. |
| #4004 | `production` | `process-device-scan`: `modules/devicescan/compose` → `auth/device` | `AuthenticatedDevice`, `AuthenticatedStaff`, `CtxDevice`, `CtxIsIoTDevice`, `CtxStaff`, `DeviceFromCtx`, `IsIoTDeviceRequest`, `StaffFromCtx` | Native device principal contract or root-bound context port, including the existing testhooks helper. |
| #4004 | `production` | `process-device-scan`: `modules/devicescan/compose` → `modules/peopledirectory/userscontract` | `ErrPersonNotFound` | Retained and public person-not-found sentinels are distinct instances; use an owner-native classifier without changing error identity. |
| #4077 | `production` | `emergency-snapshot`: `modules/emergencysnapshot/internal/adapters/nameorder` → `internal/collation` | `CompareGerman` | Canonical native ordering plus consumer ports; a private legacy-shared import is not a target grant. |
| #4076 | `production` | `enrollment`: `modules/enrollment/http` → `internal/collation` | `CompareSchoolClasses` | Canonical native ordering plus consumer ports; a private legacy-shared import is not a target grant. |
| #4004 | `production` | `facilities`: `modules/facilities/compose/httpadapter` → `api/common` | `CodeAttendanceLogDisabled`, `CodeRoomsColorAlreadyInUse`, `CodeRoomsNotFound`, `ErrorConflict`, `ErrorConflictWithCode`, `ErrorForbidden`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFoundWithCode`, `HasAdminWildcard`, `PaginationParams`, `ParseID`, `ParsePagination`, `RenderError`, `RequiresPermission`, `Respond`, `RespondNoContent`, `RespondPaginated` | Separate transport/runtime bindings from native owner values and renderer ports; a composition must not retain HTTP/runtime/private implementation reach. |
| #4004 | `production` | `facilities`: `modules/facilities/compose/httpadapter` → `internal/collation` | `CompareGerman` | Canonical native ordering plus consumer ports; a private legacy-shared import is not a target grant. |
| #4004 | `production` | `facilities`: `modules/facilities/compose/httpadapter` → `modules/facilities/http/rooms` | `ErrHistoryDisabled`, `ExportFile`, `FailureConflict`, `FailureForbidden`, `FailureInvalid`, `FailureKind`, `FailureNotFound`, `InvalidExportError`, `Middleware`, `NewResource`, `Pagination`, `Resource`, `RoomSessionEntry`, `RoomView`, `Runtime`, `SnapshotRequest` | Separate transport/runtime bindings from native owner values and renderer ports; a composition must not retain HTTP/runtime/private implementation reach. |
| #4004 | `production` | `facilities`: `modules/facilities/compose/httpadapter` → `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` | Separate transport/runtime bindings from native owner values and renderer ports; a composition must not retain HTTP/runtime/private implementation reach. |
| #4077 | `production` | `group-live-view`: `modules/grouplive/internal/adapters/nameorder` → `internal/collation` | `CompareGerman` | Canonical native ordering plus consumer ports; a private legacy-shared import is not a target grant. |
| #4004 | `production` | `organization-tenancy`: `modules/organizationtenancy/compose` → `modules/organizationtenancy/internal/adapters/operatordashboard` | `Counts`, `Map`, `New`, `OrganizationSummary`, `PWAUsageRow`, `Rows`, `SchoolSummary` | Projection-native public contracts or a root-bound port. Existing read-projection paths/grants must not be widened. |
| #4076 | `production` | `people-directory`: `modules/peopledirectory/inbound/students` → `internal/collation` | `CompareGermanNames`, `CompareSchoolClasses` | Canonical native ordering plus consumer ports; a private legacy-shared import is not a target grant. |
| #4004 | `production` | `request-review-view`: `modules/requestreview/compose` → `modules/careplan/compose` | `MasterDataDirectory`, `MasterDataFieldChange`, `MasterDataFieldFacts`, `MasterDataReviewDependencies`, `NewMasterDataReviews`, `NewOfferingBookings`, `NewOfferingReviews`, `NewScheduleReviews`, `NewStudentDataRequestQueries`, `Observation`, `OfferingReviewBooking`, `OfferingReviewChild`, `OfferingReviewCourse`, `OfferingReviewCourseRef`, `OfferingReviewDependencies`, `OfferingReviewPhase`, `OfferingReviewRequest`, `PersonName`, `PickupReviewBlock`, `PickupReviewBlocks`, `PickupReviewImpact`, `ReviewBooking`, `ReviewScope`, `ReviewScopeResolver`, `ReviewStudent`, `ScheduleReviewDependencies`, `StudentDirectory` | Publish native review vocabulary, then bind builders positionally in Root; retain four test seams without private composition imports or hook growth. |
| #4004 | `production` | `request-review-view`: `modules/requestreview/compose` → `modules/enrollment/compose` | `New` | Publish native review vocabulary, then bind builders positionally in Root; retain four test seams without private composition imports or hook growth. |
| #4004 | `production` | `request-review-view`: `modules/requestreview/compose` → `modules/timetable/compose` | `NewClassArrivalQueries`, `NewCourseGroupQueries`, `Observation` | Publish native review vocabulary, then bind builders positionally in Root; retain four test seams without private composition imports or hook growth. |
| #4076 | `production` | `school-membership`: `modules/schoolmembership/internal/adapters/displayorder` → `internal/collation` | `CompareGermanNames`, `CompareSchoolClasses` | Canonical native ordering plus consumer ports; a private legacy-shared import is not a target grant. |
| #4077 | `production` | `calendar-view`: `modules/supervisiondashboard/internal/adapters/nameorder` → `internal/collation` | `CompareGerman` | Canonical native ordering plus consumer ports; a private legacy-shared import is not a target grant. |
| #4004 | `production` | `timetable-activities`: `modules/timetable/compose/httpadapter` → `api/common` | `BindAction`, `CodeTimetableActivityNotOwner`, `CodeTimetableActivitySystemProtected`, `CodeTimetableActivityTemplateProtected`, `CodeTimetableOnlySupervisorReplacementRequired`, `DateFormatISO`, `ErrorBusinessRejectionOr`, `ErrorConflict`, `ErrorConflictWithCode`, `ErrorForbidden`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorRule`, `IDFetch`, `Middleware`, `MsgInvalidActivityID`, `MsgInvalidStudentID`, `MsgInvalidWeekday`, `PaginationParams`, `ParseID`, `ParseIDParam`, `ParsePagination`, `RenderError`, `RequiresAnyPermission`, `RequiresPermission`, `Respond`, `RespondPaginated`, `RespondWithError`, `RulesRenderer`, `Time`, `UnwrapRenderer` | Separate transport/runtime bindings from native owner values and renderer ports; a composition must not retain HTTP/runtime/private implementation reach. |
| #4004 | `production` | `timetable-activities`: `modules/timetable/compose/httpadapter` → `auth/authorize` | `CanReadStudent`, `HasAdminWildcard` | Separate transport/runtime bindings from native owner values and renderer ports; a composition must not retain HTTP/runtime/private implementation reach. |
| #4004 | `production` | `timetable-activities`: `modules/timetable/compose/httpadapter` → `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` | Separate transport/runtime bindings from native owner values and renderer ports; a composition must not retain HTTP/runtime/private implementation reach. |
| #4004 | `production` | `timetable-activities`: `modules/timetable/compose` → `auth/authorize` | `HasAdminWildcard`, `HasOperationalOverviewForResolvedStaff`, `HasPermission`, `SchoolWideActionScope` | Root-bound caller and named native presence read ports; preserve session/tenant and authorization behavior. |
| #4004 | `production` | `timetable-activities`: `modules/timetable/compose` → `modules/peopledirectory/userscontract` | `ErrPersonNotFound` | Retained and public person-not-found sentinels are distinct instances; use an owner-native classifier without changing error identity. |
| #4004 | `production` | `timetable-activities`: `modules/timetable/compose` → `modules/presenceprojection` | `CountNonAbsentParticipants`, `HasPlannedSlotsInRange`, `Instance`, `LegacyInstanceFilter`, `LegacyParticipantFilter`, `ListLegacyInstances`, `ListLegacyParticipants`, `ListParallelPresence`, `ListPartialAbsenceBlocks`, `ListScheduledInstancesForStudent`, `ParallelPresence`, `PartialAbsenceBlock`, `Participant`, `ScheduledInstance` | Root-bound caller and named native presence read ports; preserve session/tenant and authorization behavior. |
| #4004 | `production` | `student-deletion`: `workflows/studentdeletion/compose` → `modules/timetableprojection` | `CountStudentEnrollments` | Owner-native public/contract capability or a root-bound consumer port; no new private-role grant. |

## Package-exit accounting

Of the original 28 direct production-package carriers, **15 disappear and 13 remain**. Additional alias-only package exits are `modules/delivery/compose/realtime` and `modules/dataimport/compose/retained`; owner integration tests move with their support boundary. Empty packages and file-only renames are not extra exits.

| Original carrier | Candidate |
|---|---|
| `database/repositories/base` | Retained; exact consumers below |
| `database/repositories/config` | Dissolved or re-created in the native owner role |
| `database/repositories/parent` | Dissolved or re-created in the native owner role |
| `database/repositories/pwausage` | Retained; exact consumers below |
| `models/activities` | Retained; exact consumers below |
| `models/base` | Retained; exact consumers below |
| `models/calendar` | Dissolved or re-created in the native owner role |
| `models/delivery` | Retained; exact consumers below |
| `models/enrollment` | Retained; exact consumers below |
| `models/facilities` | Retained; exact consumers below |
| `models/iot` | Retained; exact consumers below |
| `models/parent` | Dissolved or re-created in the native owner role |
| `models/schedule` | Retained; exact consumers below |
| `models/users` | Dissolved or re-created in the native owner role |
| `modules/devicefleet/compose/legacy` | Retained; exact consumers below |
| `modules/facilities/compose/legacy` | Retained; exact consumers below |
| `services/audit` | Dissolved or re-created in the native owner role |
| `services/config` | Dissolved or re-created in the native owner role |
| `services/config/defaults` | Dissolved or re-created in the native owner role |
| `services/config/settingstest` | Dissolved or re-created in the native owner role |
| `services/config/sideeffects` | Dissolved or re-created in the native owner role |
| `services/database` | Dissolved or re-created in the native owner role |
| `services/import` | Dissolved or re-created in the native owner role |
| `services/import/ports` | Dissolved or re-created in the native owner role |
| `services/iot/staffclock` | Dissolved or re-created in the native owner role |
| `services/listexport` | Dissolved or re-created in the native owner role |
| `services/parentmessaging` | Retained; exact consumers below |
| `services/scheduler` | Retained; exact consumers below |

## Foreign importer inventory

Current-tree Go AST inventory, grouped by target owner. This includes remaining legacy carriers and the compositions carrying aliases, not imports merely mentioned in source-code scanner fixtures.

### audit-platform

| Consumer owner | File | Scope | Target | Symbols |
|---|---|---|---|---|
| `root-composition` | `backend/api/audit_test_helpers.go` | `production` | `modules/auditlog/compose` | `AppendObservation`, `MustNewPersistence`, `NewCommand`, `Persistence`, `PersistenceDependencies` |
| `root-composition` | `backend/api/auth_test_helpers.go` | `production` | `modules/auditlog/compose` | `AppendObservation`, `MustNewPersistence`, `NewCommand`, `PersistenceDependencies` |
| `root-composition` | `backend/api/booking_consistency_audit.go` | `production` | `modules/auditlog/compose` | `CareOffering`, `DirectoryStudent`, `NewPersistence`, `PersistenceDependencies` |
| `root-composition` | `backend/api/booking_materialization_test_helpers.go` | `production` | `modules/auditlog/compose` | `AppendObservation`, `NewCommand` |
| `root-composition` | `backend/api/calendar_test_helpers.go` | `production` | `modules/auditlog/compose` | `AppendObservation`, `NewCommand` |
| `root-composition` | `backend/api/care_lifecycle_test_helpers.go` | `production` | `modules/auditlog/compose` | `AppendObservation`, `NewCommand` |
| `root-composition` | `backend/api/class_day_test_helpers.go` | `production` | `modules/auditlog/compose` | `AppendObservation`, `NewCommand` |
| `root-composition` | `backend/api/class_list_test_helpers.go` | `production` | `modules/auditlog/compose` | `AppendObservation`, `NewCommand` |
| `root-composition` | `backend/api/cleanup_roots.go` | `production` | `modules/auditlog/compose` | `MustNewPersistence`, `NewHandoffPersistence`, `NewObservedCommand`, `PersistenceDependencies` |
| `root-composition` | `backend/api/filestore_test_helpers.go` | `production` | `modules/auditlog/compose` | `AppendObservation`, `NewCommand` |
| `root-composition` | `backend/api/grade_transition_test_helpers.go` | `production` | `modules/auditlog/compose` | `AppendObservation`, `NewCommand` |
| `root-composition` | `backend/api/guardian_invitation_test_helpers.go` | `production` | `modules/auditlog/compose` | `AppendObservation`, `MustNewPersistence`, `NewCommand`, `PersistenceDependencies` |
| `root-composition` | `backend/api/guardian_test_helpers.go` | `production` | `modules/auditlog/compose` | `AppendObservation`, `NewCommand` |
| `root-composition` | `backend/api/identity_cleanup.go` | `production` | `modules/auditlog/compose` | `MustNewPersistence`, `PersistenceDependencies` |
| `root-composition` | `backend/api/import_composition.go` | `production` | `modules/auditlog/compose` | `NewConsentRecorder` |
| `root-composition` | `backend/api/import_test_helpers.go` | `production` | `modules/auditlog/compose` | `AppendObservation`, `NewCommand`, `NewConsentRecorder` |
| `root-composition` | `backend/api/offering_change_test_helpers.go` | `production` | `modules/auditlog/compose` | `AppendObservation`, `NewCommand` |
| `root-composition` | `backend/api/offering_review_test_helpers.go` | `production` | `modules/auditlog/compose` | `AppendObservation`, `NewCommand` |
| `root-composition` | `backend/api/owner_roots.go` | `production` | `modules/auditlog/compose` | `MustNewPersistence`, `PersistenceDependencies` |
| `root-composition` | `backend/api/parent_guardian_changes.go` | `production` | `modules/auditlog/compose` | `NewGuardianChangeLog` |
| `root-composition` | `backend/api/people_directory_roots.go` | `production` | `modules/auditlog/compose` | `MustNewPersistence`, `PersistenceDependencies` |
| `root-composition` | `backend/api/request_review_test_helpers.go` | `production` | `modules/auditlog/compose` | `NewCorrectionQueries` |
| `root-composition` | `backend/api/retention_test_helpers.go` | `production` | `modules/auditlog/compose` | `AppendObservation`, `Command`, `NewCommand` |
| `root-composition` | `backend/api/root_platform.go` | `production` | `modules/auditlog/compose` | `NewObservedCommand`, `NewTenantPersistence`, `Persistence` |
| `root-composition` | `backend/api/school_structure_suite_test_helpers.go` | `production` | `modules/auditlog/compose` | `MustNewPersistence`, `PersistenceDependencies` |
| `root-composition` | `backend/api/serve_mounts.go` | `production` | `modules/auditlog/compose` | `NewCorrectionQueries` |
| `root-composition` | `backend/api/statistics_test_helpers.go` | `production` | `modules/auditlog/compose` | `AppendObservation`, `NewCommand` |
| `root-composition` | `backend/api/student_deletion_owners.go` | `production` | `modules/auditlog/compose` | `NewTenantPersistence` |
| `root-composition` | `backend/api/student_test_helpers.go` | `production` | `modules/auditlog/compose` | `AppendObservation`, `NewCommand` |
| `root-composition` | `backend/api/timetable_http_test_helpers.go` | `production` | `modules/auditlog/compose` | `MustNewPersistence`, `NewFixedRuntime`, `PersistenceDependencies` |
| `root-composition` | `backend/api/timetable_scenario_test_helpers.go` | `production` | `modules/auditlog/compose` | `AppendObservation`, `NewCommand` |
| `root-composition` | `backend/api/workflow_audits.go` | `production` | `modules/auditlog/compose` | `AppendObservation`, `NewCommand`, `NewTenantPersistence` |
| `root-composition` | `backend/api/workforce_test_helpers.go` | `production` | `modules/auditlog/compose` | `AppendObservation`, `NewCommand` |
| `root-composition` | `backend/cmd/cleanup_roots.go` | `production` | `modules/auditlog/compose` | `MustNewPersistence`, `NewFixedRuntime`, `NewPersistence`, `PersistenceDependencies` |

### communication

| Consumer owner | File | Scope | Target | Symbols |
|---|---|---|---|---|
| `care-plan` | `backend/modules/careplan/contracttest/care_request_pickup_pill_test.go` | `external_test` | `services/parentmessaging` | `ChildEvent`, `NewEmitterWithDecisions`, `PortalGuardian` |
| `care-plan` | `backend/modules/careplan/contracttest/care_request_service_test.go` | `external_test` | `services/parentmessaging` | `Emitter`, `TenantSettingsResolver` |

### device-fleet

| Consumer owner | File | Scope | Target | Symbols |
|---|---|---|---|---|
| `root-composition` | `backend/api/cleanup_roots.go` | `production` | `modules/devicefleet/compose/legacy` | `SessionDeviceRecords` |
| `root-composition` | `backend/api/display_http_test.go` | `internal_test` | `modules/devicefleet/compose/legacy` | `DashboardDependencies`, `NewDashboardSources` |
| `root-composition` | `backend/api/root_presence_services.go` | `production` | `modules/devicefleet/compose/legacy` | `DashboardDependencies`, `NewDashboardSources` |
| `root-composition` | `backend/api/session_devices.go` | `production` | `modules/devicefleet/compose/legacy` | `SessionDeviceIDs`, `SessionDeviceRecords` |

### enrollment

| Consumer owner | File | Scope | Target | Symbols |
|---|---|---|---|---|
| `care-plan` | `backend/modules/careplan/contracttest/care_offering_catalog_guards_test.go` | `external_test` | `models/enrollment` | `CareOffering`, `DaysOfWeekModeFixed`, `PhaseCareOverflowWaitlist`, `PhaseKindSchoolYear` |
| `care-plan` | `backend/modules/careplan/contracttest/care_offering_catalog_test.go` | `external_test` | `models/enrollment` | `CareOffering`, `DaysOfWeekModeFixed`, `DaysOfWeekModeParentChoice`, `PhaseCareOverflowWaitlist`, `PhaseKindSchoolYear`, `SelectionRuleAtLeastOne`, `SelectionRuleAtMostOne`, `SelectionRuleExactlyOne`, `SelectionRuleOptional` |
| `care-plan` | `backend/modules/careplan/contracttest/care_offering_records_fixture_test.go` | `external_test` | `models/enrollment` | `CareOffering`, `OfferingChangeStatusPending` |
| `care-plan` | `backend/modules/careplan/contracttest/care_offering_repository_test.go` | `external_test` | `models/enrollment` | `CareOffering`, `DaysOfWeekModeParentChoice` |
| `care-plan` | `backend/modules/careplan/contracttest/enrollment_decision_env_test.go` | `external_test` | `models/enrollment` | `CareOffering`, `DaysOfWeekModeParentChoice`, `PhaseCareOverflowWaitlist`, `PhaseKindSchoolYear` |
| `care-plan` | `backend/modules/careplan/contracttest/enrollment_fixture_helpers_test.go` | `external_test` | `models/enrollment` | `CareOffering`, `DaysOfWeekModeFixed`, `PhaseCareOverflowWaitlist`, `PhaseKindSchoolYear` |
| `care-plan` | `backend/modules/careplan/contracttest/lifecycle_care_booking_authority_integration_test.go` | `external_test` | `models/enrollment` | `ChildStatusApproved`, `ChildStatusSubmitted` |
| `care-plan` | `backend/modules/careplan/contracttest/offering_adjustment_complete_withdrawal_test.go` | `external_test` | `models/enrollment` | `CareOffering` |
| `care-plan` | `backend/modules/careplan/contracttest/offering_adjustment_dated_test.go` | `external_test` | `models/enrollment` | `CareOffering` |
| `care-plan` | `backend/modules/careplan/contracttest/offering_arrival_projection_test.go` | `external_test` | `models/enrollment` | `CareOffering`, `DaysOfWeekModeFixed` |
| `care-plan` | `backend/modules/careplan/contracttest/offering_change_full_withdrawal_test.go` | `external_test` | `models/enrollment` | `CareOffering`, `OfferingChangeStatusApproved`, `OfferingChangeStatusPending`, `PhaseCareOfferingSelectionAtLeastOne` |
| `care-plan` | `backend/modules/careplan/contracttest/offering_change_history_test.go` | `external_test` | `models/enrollment` | `OfferingChangeStatusRejected`, `OfferingChangeStatusWithdrawn` |
| `care-plan` | `backend/modules/careplan/contracttest/offering_change_request_automatic_test.go` | `external_test` | `models/enrollment` | `AvailabilityMatchAll`, `AvailabilityOperatorIn`, `AvailabilitySourceGradeLevel`, `CareOffering`, `CareOfferingAvailabilityCondition`, `CareOfferingAvailabilityRule`, `OfferingChangeSnapshotEntry`, `OfferingChangeStatusPending`, `OfferingChangeStatusRejected` |
| `care-plan` | `backend/modules/careplan/contracttest/offering_change_request_service_test.go` | `external_test` | `models/enrollment` | `CareOffering`, `DaysOfWeekModeFixed`, `OfferingChangeDate`, `OfferingChangeStatusApproved`, `OfferingChangeStatusPending`, `OfferingChangeStatusRejected` |
| `care-plan` | `backend/modules/careplan/contracttest/offering_pickup_projection_test.go` | `external_test` | `models/enrollment` | `CareOffering`, `DaysOfWeekModeFixed` |
| `care-plan` | `backend/modules/careplan/contracttest/offering_source_service_test.go` | `external_test` | `models/enrollment` | `CareOffering`, `DaysOfWeekModeFixed`, `PhaseCareOverflowWaitlist`, `PhaseKindSchoolYear` |

### facilities

| Consumer owner | File | Scope | Target | Symbols |
|---|---|---|---|---|
| `root-composition` | `backend/api/active_test_helpers.go` | `production` | `modules/facilities/compose/legacy` | `NewAttendanceRooms`, `OpenGroupCatalog` |
| `root-composition` | `backend/api/groups_test_helpers.go` | `production` | `modules/facilities/compose/legacy` | `NewAttendanceRooms` |
| `root-composition` | `backend/api/iot_data_test_helpers.go` | `production` | `modules/facilities/compose/legacy` | `HistoryProjection`, `OccupancyProjection` |
| `root-composition` | `backend/api/operator_settings_test_helpers.go` | `production` | `modules/facilities/compose/legacy` | `SettingHandlers` |
| `root-composition` | `backend/api/presence_suite_test_helpers.go` | `production` | `modules/facilities/compose/legacy` | `NewAttendanceRooms` |
| `root-composition` | `backend/api/room_activity_names.go` | `production` | `modules/facilities/compose/legacy` | `ActivityGroupName` |
| `root-composition` | `backend/api/room_history_presence.go` | `production` | `modules/facilities/compose/legacy` | `RoomSessionFact` |
| `root-composition` | `backend/api/room_occupancy_presence.go` | `production` | `modules/facilities/compose/legacy` | `RoomOccupancyFact` |
| `root-composition` | `backend/api/root_presence.go` | `production` | `modules/facilities/compose/legacy` | `HistoryProjection`, `OccupancyProjection`, `OpenGroupCatalog` |
| `root-composition` | `backend/api/root_presence_services.go` | `production` | `modules/facilities/compose/legacy` | `NewAttendanceRooms` |
| `root-composition` | `backend/api/root_settings_writes.go` | `production` | `modules/facilities/compose/legacy` | `SettingHandlers` |
| `root-composition` | `backend/api/timetable_test_helpers.go` | `production` | `modules/facilities/compose/legacy` | `NewAttendanceRooms` |

### settings-platform

| Consumer owner | File | Scope | Target | Symbols |
|---|---|---|---|---|
| `root-composition` | `backend/api/account_route_test_helpers.go` | `production` | `modules/settings/compose` | `NewTenantSettingsRuntime`, `WithSettingsRequestCache` |
| `root-composition` | `backend/api/active_test_helpers.go` | `production` | `modules/settings/compose` | `BatchSettingsService`, `PresenceSettings` |
| `root-composition` | `backend/api/auth_test_helpers.go` | `production` | `modules/settings/compose` | `SettingsService` |
| `root-composition` | `backend/api/device_configuration_internal_test.go` | `internal_test` | `modules/settings/compose` | `SettingsSnapshot` |
| `root-composition` | `backend/api/device_fleet_settings.go` | `production` | `modules/settings/compose` | `ResolveIntOrDefault` |
| `root-composition` | `backend/api/display_http_test.go` | `internal_test` | `modules/settings/compose` | `SettingsService` |
| `root-composition` | `backend/api/filestore_composition.go` | `production` | `modules/settings/compose` | `SettingsService` |
| `root-composition` | `backend/api/filestore_test_helpers.go` | `production` | `modules/settings/compose` | `SettingsService` |
| `root-composition` | `backend/api/identity_account_authentication.go` | `production` | `modules/settings/compose` | `SettingsService` |
| `root-composition` | `backend/api/identity_guardian_invitation.go` | `production` | `modules/settings/compose` | `ResolveIntOrDefault`, `SettingsService` |
| `root-composition` | `backend/api/identity_mfa.go` | `production` | `modules/settings/compose` | `SettingsService` |
| `root-composition` | `backend/api/iot_composition_test.go` | `internal_test` | `modules/settings/compose` | `SettingsService` |
| `root-composition` | `backend/api/operator_provisioning_composition.go` | `production` | `modules/settings/compose` | `SettingsService` |
| `root-composition` | `backend/api/operator_settings_composition.go` | `production` | `modules/settings/compose` | `SettingsChangedNotifier` |
| `root-composition` | `backend/api/operator_settings_test_helpers.go` | `production` | `modules/settings/compose` | `BatchSettingsService`, `NewOperatorSchoolSettings`, `NewSideEffects`, `OperatorDependencies`, `OperatorValueSetHook`, `SettingSideEffect`, `SettingsChangedNotifier` |
| `root-composition` | `backend/api/presence_suite_test_helpers.go` | `production` | `modules/settings/compose` | `PresenceSettings` |
| `root-composition` | `backend/api/reminder_readers.go` | `production` | `modules/settings/compose` | `SettingsService`, `SettingsSnapshot`, `WithSettingsSnapshot` |
| `root-composition` | `backend/api/retention_test_helpers.go` | `production` | `modules/settings/compose` | `BatchSettingsService`, `NewAuditRetention` |
| `root-composition` | `backend/api/root_careplan.go` | `production` | `modules/settings/compose` | `SettingsService` |
| `root-composition` | `backend/api/root_careplan_offerings.go` | `production` | `modules/settings/compose` | `SettingsService` |
| `root-composition` | `backend/api/root_careplan_requests.go` | `production` | `modules/settings/compose` | `BatchSettingsService` |
| `root-composition` | `backend/api/root_communication.go` | `production` | `modules/settings/compose` | `BatchSettingsService` |
| `root-composition` | `backend/api/root_delivery.go` | `production` | `modules/settings/compose` | `BatchSettingsService` |
| `root-composition` | `backend/api/root_enrollment_services.go` | `production` | `modules/settings/compose` | `BatchSettingsService` |
| `root-composition` | `backend/api/root_env.go` | `production` | `modules/settings/compose` | `SettingsLookupObserver`, `SideEffectFailureObserver` |
| `root-composition` | `backend/api/root_identity.go` | `production` | `modules/settings/compose` | `BatchSettingsService` |
| `root-composition` | `backend/api/root_people_services.go` | `production` | `modules/settings/compose` | `SettingsService` |
| `root-composition` | `backend/api/root_planning.go` | `production` | `modules/settings/compose` | `BatchSettingsService`, `NewPayrollStatusService`, `PayrollStatusGetter` |
| `root-composition` | `backend/api/root_platform.go` | `production` | `modules/settings/compose` | `BatchSettingsService`, `NewHomeLayoutRepository`, `NewHomeLayoutService`, `NewSettingAuditRepository`, `NewSettingValueRepository`, `NewSettings`, `NewTenantSettingsRuntime`, `SettingsDependencies`, `SettingsService`, `TenantSettingsRuntime` |
| `root-composition` | `backend/api/root_portal.go` | `production` | `modules/settings/compose` | `BatchSettingsService`, `SettingsService` |
| `root-composition` | `backend/api/root_presence_services.go` | `production` | `modules/settings/compose` | `BatchSettingsService`, `PresenceSettings` |
| `root-composition` | `backend/api/root_serve.go` | `production` | `modules/settings/compose` | `PayrollStatusGetter` |
| `root-composition` | `backend/api/root_serve_owners.go` | `production` | `modules/settings/compose` | `SettingsService` |
| `root-composition` | `backend/api/root_settings_writes.go` | `production` | `modules/settings/compose` | `NewSideEffects`, `NewTenantOperations`, `PayrollStatusGetter`, `SettingSideEffect` |
| `root-composition` | `backend/api/root_test_root.go` | `production` | `modules/settings/compose` | `BatchSettingsService`, `PayrollStatusGetter` |
| `root-composition` | `backend/api/root_timetable_services.go` | `production` | `modules/settings/compose` | `BatchSettingsService` |
| `root-composition` | `backend/api/root_views.go` | `production` | `modules/settings/compose` | `BatchSettingsService`, `PresenceSettings`, `SettingsService` |
| `root-composition` | `backend/api/root_worker.go` | `production` | `modules/settings/compose` | `BatchSettingsService`, `NewAuditRetention`, `PayrollStatusGetter` |
| `root-composition` | `backend/api/root_workforce.go` | `production` | `modules/settings/compose` | `BatchSettingsService`, `PayrollStatusGetter`, `PresenceSettings`, `SettingsService` |
| `root-composition` | `backend/api/scheduler_ports.go` | `production` | `modules/settings/compose` | `BatchSettingsService`, `NewTenantSnapshots`, `TenantSnapshot`, `VerifyPreloadKeys` |
| `root-composition` | `backend/api/school_setup_composition.go` | `production` | `modules/settings/compose` | `NewSchoolSetupRepository`, `NewStaffOnboardingRepository` |
| `root-composition` | `backend/api/school_setup_store_test_helpers.go` | `production` | `modules/settings/compose` | `NewSchoolSetupRepository`, `NewStaffOnboardingRepository` |
| `root-composition` | `backend/api/serve_host.go` | `production` | `modules/settings/compose` | `NewAnalyseFreigabe`, `SettingsService` |
| `root-composition` | `backend/api/serve_mounts.go` | `production` | `modules/settings/compose` | `NewOperatorSchoolSettings`, `OperatorDependencies`, `SettingsService` |
| `root-composition` | `backend/api/server.go` | `production` | `modules/settings/compose` | `BatchSettingsService` |
| `root-composition` | `backend/api/settings_adapters.go` | `production` | `modules/settings/compose` | `SchoolSettingsStore` |
| `root-composition` | `backend/api/settings_bindings.go` | `production` | `modules/settings/compose` | `ResolveBoolOrDefault`, `ResolveIntOrDefault`, `SettingsSnapshot`, `WithSettingsSnapshot` |
| `root-composition` | `backend/api/settings_callbacks_test_helpers.go` | `production` | `modules/settings/compose` | `NewSideEffects`, `NewTenantOperations`, `SettingSideEffect`, `SettingsService` |
| `root-composition` | `backend/api/settings_repository_test_helpers.go` | `production` | `modules/settings/compose` | `NewSettingAuditRepository`, `NewSettingValueRepository`, `NewSettingsService`, `PersistenceRuntime`, `SettingsRuntime`, `SettingsService` |
| `root-composition` | `backend/api/settings_test_helpers.go` | `production` | `modules/settings/compose` | `BatchSettingsService`, `NewHomeLayoutRepository`, `NewHomeLayoutService`, `NewPayrollStatusService`, `NewSettings`, `NewTenantOperations`, `NewTenantSettingsRuntime`, `NewTenantSnapshots`, `PayrollStatusGetter`, `SettingsDependencies`, `TenantOperationsRuntime`, `TenantSnapshots` |
| `root-composition` | `backend/api/staff_composition.go` | `production` | `modules/settings/compose` | `NewSFTPExportSettings`, `SettingsService` |
| `root-composition` | `backend/api/student_photo_bootstrap_test.go` | `internal_test` | `modules/settings/compose` | `NewSideEffects`, `SettingSideEffect` |
| `root-composition` | `backend/api/student_photo_settings_test.go` | `internal_test` | `modules/settings/compose` | `NewSideEffects`, `SettingSideEffect` |
| `root-composition` | `backend/api/time_tracking_composition.go` | `production` | `modules/settings/compose` | `SettingsService` |
| `root-composition` | `backend/api/time_tracking_documents.go` | `production` | `modules/settings/compose` | `PayrollStatusGetter` |
| `root-composition` | `backend/api/time_tracking_documents_internal_test.go` | `internal_test` | `modules/settings/compose` | `NewPayrollStatusService`, `PayrollStatusGetter` |
| `root-composition` | `backend/api/time_tracking_ports.go` | `production` | `modules/settings/compose` | `PayrollStatusGetter`, `PresenceSettings` |
| `root-composition` | `backend/api/time_tracking_test_helpers.go` | `production` | `modules/settings/compose` | `NewPayrollStatusService`, `PayrollStatusGetter`, `SettingsService` |
| `root-composition` | `backend/api/timetable_test_helpers.go` | `production` | `modules/settings/compose` | `PresenceSettings` |
| `root-composition` | `backend/api/worker_retention.go` | `production` | `modules/settings/compose` | `VerifyPreloadKeys` |
| `root-composition` | `backend/api/workforce_admin_capabilities.go` | `production` | `modules/settings/compose` | `SettingsService` |
| `root-composition` | `backend/cmd/cleanup_roots.go` | `production` | `modules/settings/compose` | `NewSettingAuditRepository`, `NewSettingValueRepository`, `NewSettingsService`, `NewTenantSettingsRuntime`, `PresenceSettings`, `SettingsService` |
| `root-composition` | `backend/cmd/settings.go` | `production` | `modules/settings/compose` | `NewStoredOverrideReader` |

### transaction-runtime

| Consumer owner | File | Scope | Target | Symbols |
|---|---|---|---|---|
| `delivery-platform` | `backend/modules/delivery/internal/adapters/deliverypostgres/push_subscription.go` | `production` | `database/repositories/base` | `EnsureTenantID`, `GetDB`, `NewRepository`, `Repository`, `TranslateNotFound`, `WithTenantFilter` |
| `delivery-platform` | `backend/models/delivery/push_subscription.go` | `production` | `models/base` | `CRUDRepository`, `Model`, `TenantModel` |
| `delivery-platform` | `backend/modules/delivery/internal/adapters/deliverypostgres/push_subscription.go` | `production` | `models/base` | `DatabaseError` |

### workforce

| Consumer owner | File | Scope | Target | Symbols |
|---|---|---|---|---|
| `root-composition` | `backend/api/auth_persistence_test_helpers.go` | `production` | `modules/workforce/compose` | `MustNewStaffEmployment` |
| `root-composition` | `backend/api/base.go` | `production` | `modules/workforce/compose` | `NewStaffEmployment`, `Observation` |
| `root-composition` | `backend/api/data_access_audit.go` | `production` | `modules/workforce/compose` | `DataAccessAudit`, `DataAccessEvent` |
| `root-composition` | `backend/api/deletion_audit.go` | `production` | `modules/workforce/compose` | `DeletionAudit`, `DeletionEvent` |
| `root-composition` | `backend/api/delivery_staff_recipients.go` | `production` | `modules/workforce/compose` | `WorkPresenceQuery` |
| `root-composition` | `backend/api/document_cleanup_test_helpers.go` | `production` | `modules/workforce/compose` | `NewDocumentCleanup` |
| `root-composition` | `backend/api/membership_repositories.go` | `production` | `modules/workforce/compose` | `MustNewStaffEmployment` |
| `root-composition` | `backend/api/membership_test_helpers.go` | `production` | `modules/workforce/compose` | `Dependencies`, `New`, `Observation` |
| `root-composition` | `backend/api/people_directory_roots.go` | `production` | `modules/workforce/compose` | `MustNewStaffEmployment`, `StaffDayStatus` |
| `root-composition` | `backend/api/people_repository_test_helpers.go` | `production` | `modules/workforce/compose` | `StaffAbsenceRepository` |
| `root-composition` | `backend/api/personal_reminder_notifications.go` | `production` | `modules/workforce/compose` | `WorkPresence` |
| `root-composition` | `backend/api/plan_export_dienstplan_test.go` | `internal_test` | `modules/workforce/compose` | `Dependencies`, `New`, `NewStaffScheduleOverview`, `Observation`, `RoomReader`, `StaffScheduleOverviewDependencies` |
| `root-composition` | `backend/api/presence_work_sessions.go` | `production` | `modules/workforce/compose` | `AutoCheckIn` |
| `root-composition` | `backend/api/retention_test_helpers.go` | `production` | `modules/workforce/compose` | `NewCleanupRetention`, `NewSickNoteRetention` |
| `root-composition` | `backend/api/root_delivery.go` | `production` | `modules/workforce/compose` | `WorkPresence` |
| `root-composition` | `backend/api/root_people_services.go` | `production` | `modules/workforce/compose` | `StaffAdminDependencies` |
| `root-composition` | `backend/api/root_planning.go` | `production` | `modules/workforce/compose` | `NewShiftPlanning`, `ShiftPlanning`, `ShiftPlanningDependencies` |
| `root-composition` | `backend/api/root_presence_services.go` | `production` | `modules/workforce/compose` | `AutoCheckIn` |
| `root-composition` | `backend/api/root_serve.go` | `production` | `modules/workforce/compose` | `ShiftPlanning`, `WorkSessionSide` |
| `root-composition` | `backend/api/root_serve_owners.go` | `production` | `modules/workforce/compose` | `Dependencies`, `New`, `Observation` |
| `root-composition` | `backend/api/root_test_root.go` | `production` | `modules/workforce/compose` | `WorkSessionSide` |
| `root-composition` | `backend/api/root_views.go` | `production` | `modules/workforce/compose` | `ShiftPlanning` |
| `root-composition` | `backend/api/root_worker.go` | `production` | `modules/workforce/compose` | `NewCleanupRetention`, `NewDocumentCleanup`, `NewSickNoteRetention`, `NewStaffDocumentFileRecovery`, `ShiftPlanning`, `TimeTracking`, `WorkSessionSide` |
| `root-composition` | `backend/api/root_workforce.go` | `production` | `modules/workforce/compose` | `AbsenceEmailDeps`, `MustNewStaffEmployment`, `NewStaffAbsenceAuditRepository`, `NewStaffShiftLock`, `NewTimeTrackingCleanup`, `NewWorkSessionSide`, `ShiftPlanning`, `TimeTracking`, `TimeTrackingCleanupDependencies`, `TimeTrackingCleanupService`, `TimeTrackingPorts`, `WorkSessionSide` |
| `root-composition` | `backend/api/scheduler_time_tracking_cleanup.go` | `production` | `modules/workforce/compose` | `TimeTrackingCleanupService` |
| `root-composition` | `backend/api/school_membership.go` | `production` | `modules/workforce/compose` | `NewStaffEmployment` |
| `root-composition` | `backend/api/serve_mounts.go` | `production` | `modules/workforce/compose` | `NewAbsenceTypeAdministration`, `ShiftPlanning`, `TimeTracking` |
| `root-composition` | `backend/api/staff_admin_composition.go` | `production` | `modules/workforce/compose` | `NewStaffAdmin`, `StaffAdminDependencies` |
| `root-composition` | `backend/api/staff_composition.go` | `production` | `modules/workforce/compose` | `TimeTracking` |
| `root-composition` | `backend/api/staff_membership_composition.go` | `production` | `modules/workforce/compose` | `StaffDayStatus` |
| `root-composition` | `backend/api/staff_offboarding_workforce.go` | `production` | `modules/workforce/compose` | `Dependencies`, `NewOffboarding`, `Observation` |
| `root-composition` | `backend/api/staff_projection_education.go` | `production` | `modules/workforce/compose` | `GroupSubstitution`, `NewGroupSubstitutionRepository`, `SubstitutionBlocker` |
| `root-composition` | `backend/api/staff_route_test_helpers.go` | `production` | `modules/workforce/compose` | `NewAbsenceTypeAdministration`, `NewShiftPlanning`, `ShiftPlanningDependencies` |
| `root-composition` | `backend/api/time_tracking_absence_email.go` | `production` | `modules/workforce/compose` | `AbsenceEmailContact`, `AbsenceEmailMessage` |
| `root-composition` | `backend/api/time_tracking_absence_email_test.go` | `internal_test` | `modules/workforce/compose` | `AbsenceEmailMessage` |
| `root-composition` | `backend/api/time_tracking_audits.go` | `production` | `modules/workforce/compose` | `TimeTrackingAuditEntry`, `TimeTrackingAuditFilter`, `TimeTrackingAuditReader`, `TimeTrackingDeletionAudit`, `TimeTrackingDeletionEvent`, `WorkSessionAudit`, `WorkSessionEdit` |
| `root-composition` | `backend/api/time_tracking_audits_internal_test.go` | `internal_test` | `modules/workforce/compose` | `TimeTrackingAuditCursor`, `TimeTrackingAuditEntry`, `TimeTrackingAuditFilter`, `WorkSessionEdit` |
| `root-composition` | `backend/api/time_tracking_composition.go` | `production` | `modules/workforce/compose` | `NewDocumentCleanup`, `NewStaffDocumentFileRecovery`, `TimeTracking` |
| `root-composition` | `backend/api/time_tracking_documents.go` | `production` | `modules/workforce/compose` | `PayrollExportCategory`, `PayrollExportConfiguration`, `TimeTrackingDocument` |
| `root-composition` | `backend/api/time_tracking_documents_internal_test.go` | `internal_test` | `modules/workforce/compose` | `PayrollExportCategory`, `PayrollExportConfiguration` |
| `root-composition` | `backend/api/time_tracking_events.go` | `production` | `modules/workforce/compose` | `EventPublisher` |
| `root-composition` | `backend/api/time_tracking_ports.go` | `production` | `modules/workforce/compose` | `NewTimeTrackingShifts`, `StaffEmploymentQuery`, `StaffScheduleAssignments`, `StaffScheduleBinding`, `StaffScheduleBindingLookup`, `TimeTrackingPorts` |
| `root-composition` | `backend/api/time_tracking_staff.go` | `production` | `modules/workforce/compose` | `MonthCloseStaffQuery`, `OverviewStaff`, `OverviewStaffQuery`, `StaffDisplayNameQuery`, `StaffEmploymentQuery`, `StaffScheduleAssignment`, `StaffScheduleAssignments`, `StaffScheduleBinding`, `TimeExportStaff`, `TimeExportStaffQuery`, `WorkSessionStaff`, `WorkSessionStaffName` |
| `root-composition` | `backend/api/time_tracking_test_helpers.go` | `production` | `modules/workforce/compose` | `EventPublisher`, `HolidayDatesReader`, `MustNewStaffEmployment`, `NewStaffAbsenceAuditRepository`, `NewWorkSessionSide`, `ShiftPlanSyncer`, `StaffEmploymentQuery`, `TimeTracking`, `TimeTrackingPorts`, `WorkSessionSide` |
| `root-composition` | `backend/api/work_session_repository_test_helpers.go` | `production` | `modules/workforce/compose` | `StaffAbsenceRepository` |
| `root-composition` | `backend/api/work_session_test_helpers.go` | `production` | `modules/workforce/compose` | `TimeTracking` |
| `root-composition` | `backend/api/worker_retention.go` | `production` | `modules/workforce/compose` | `MustNewStaffEmployment`, `NewRetiredStaffRecords` |
| `root-composition` | `backend/api/workforce_admin_capabilities.go` | `production` | `modules/workforce/compose` | `TimeTracking` |
| `root-composition` | `backend/api/workforce_composition.go` | `production` | `modules/workforce/compose` | `Dependencies`, `New`, `Observation` |
| `root-composition` | `backend/api/workforce_legacy_schedule_repositories.go` | `production` | `modules/workforce/compose` | `LegacyWorkTimeModelNotFound` |
| `root-composition` | `backend/api/workforce_repository_test_helpers.go` | `production` | `modules/workforce/compose` | `NewStaffAbsenceAuditRepository`, `NewStaffAbsenceTypeAllowanceChangeRepository`, `NewStaffAbsenceTypeAllowanceRepository`, `StaffAbsenceAuditRepository`, `StaffAbsenceTypeAllowanceChangeRepository`, `StaffAbsenceTypeAllowanceRepository` |
| `root-composition` | `backend/api/workforce_schedule_records.go` | `production` | `modules/workforce/compose` | `WorkScheduleRow`, `WorkScheduleTargets`, `WorkTimeTargetModel`, `WorkTimeTemplate`, `WorkTimeTemplateEntry` |
| `root-composition` | `backend/api/workforce_staff_absences.go` | `production` | `modules/workforce/compose` | `AbsenceRequestFilter`, `AbsenceRequestRow`, `NewStaffAbsenceRepository`, `StaffAbsenceRepository` |
| `root-composition` | `backend/api/workforce_test_helpers.go` | `production` | `modules/workforce/compose` | `AbsenceRequestFilter`, `MustNewStaffEmployment`, `NewShiftPlanning`, `NewStaffShiftLock`, `Observation`, `ShiftPlanningDependencies`, `StaffAbsence`, `StaffAdminDependencies`, `TimeTracking` |
| `root-composition` | `backend/cmd/cleanup.go` | `production` | `modules/workforce/compose` | `TimeTrackingCleanupPreview`, `TimeTrackingCleanupResult`, `TimeTrackingCleanupService`, `TimeTrackingCleanupStats` |
| `root-composition` | `backend/cmd/cleanup_helpers.go` | `production` | `modules/workforce/compose` | `TimeTrackingCleanupService` |
| `root-composition` | `backend/cmd/cleanup_roots.go` | `production` | `modules/workforce/compose` | `NewStaffAbsenceRepository`, `NewTimeTrackingCleanup`, `TimeTrackingCleanupDependencies`, `TimeTrackingCleanupService` |
| `school-structure` | `backend/modules/schoolstructure/internal/adapters/staffgroups/staffgroups_test.go` | `external_test` | `modules/workforce/compose` | `Dependencies`, `New`, `NewStaffEmployment`, `Observation` |
| `shift-plan-sync` | `backend/workflows/shiftplansync/compose/workflow_env_test.go` | `external_test` | `modules/workforce/compose` | `NewShiftPlanning`, `NewStaffShiftLock`, `ShiftPlanningDependencies` |
| `staff-offboarding` | `backend/workflows/staffoffboarding/offboarding_test.go` | `external_test` | `modules/workforce/compose` | `Dependencies`, `New`, `NewOffboarding`, `NewStaffEmployment`, `Observation` |

## Legacy-prefixed files retained

These still need an owner-native split; renaming alone is not counted as a dissolution.

- `backend/internal/architecture/legacy_test.go`
- `backend/modules/careplan/http/legacy_contact_contract_test.go`
- `backend/modules/grouplive/internal/adapters/nameorder/legacy_test.go`
- `backend/modules/peopledirectory/compose/legacy_repository_runtime.go`
- `backend/modules/presenceprojection/http/legacy_contact_contract_test.go`
- `backend/modules/presenceprojection/legacy_rows.go`
- `backend/modules/schoolstructure/compose/legacy_group_catalog.go`
- `backend/modules/schoolstructure/compose/legacy_group_contract.go`
- `backend/modules/schoolstructure/compose/legacy_group_management_test_helpers.go`
- `backend/modules/schoolstructure/compose/legacy_repository_runtime.go`
- `backend/modules/supervisiondashboard/internal/adapters/nameorder/legacy_test.go`
- `backend/modules/timetable/compose/legacy_activity_service.go`
- `backend/modules/timetable/compose/legacy_category_operations.go`
- `backend/modules/timetable/compose/legacy_enrollment_operations.go`
- `backend/modules/timetable/compose/legacy_errors.go`
- `backend/modules/timetable/compose/legacy_group_update.go`
- `backend/modules/timetable/compose/legacy_interface.go`
- `backend/modules/timetable/compose/legacy_schedule_operations.go`
- `backend/modules/timetable/compose/legacy_supervisor_operations.go`

## Verification and publication

Focused Data Import application/integration, Settings application/defaults/HTTP, Scheduler, Document Rendering and Staff Clock suites passed with unchanged assertions. Source ratchets and the fixed-build architecture check passed. `TestFullProductionRouterGolden` and all golden fixtures are unchanged. Historical evidence source paths moved here are pinned to the immutable development commit, rather than left as missing local files.

Full affected tests passed before the latest clean development merge (219 packages, no `--fast`, `GOCACHE=/tmp/gocache-final`); the merged head is rechecked before push. Retirement evidence validates all seven removals against the immutable development SHA. Lint passed with zero issues. Full hooks run on the committed head before the single authorized push; the PR records their outcome. PR #4330 is integrated; there is no remaining hold. After push, run offline quorum with `--no-resolve-conflicts` and merge only after the required green checks.
