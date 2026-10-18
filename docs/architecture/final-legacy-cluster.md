# Final legacy cluster

Inventory base: `a51ece49deb11a42951491853e85a7968a2d39ff`. Branch: `final-legacy-cluster`.

## Candidate result

**Partial, not ready for a PR.** Seven of the 42 exact keys retire. Thirty-five remain. No push, PR, merge, issue update, grant, checker change, alias or larger budget is included. The required final exits are not claimed.

| Carrier | Base | Candidate | Retired |
|---|---:|---:|---:|
| #4004 | 29 | 24 | 5 |
| #4076 | 8 | 8 | 0 |
| #4077 | 3 | 3 | 0 |
| #2751 | 2 | 0 | 2 |

Completed boundaries and package dissolutions:

1. `database/repositories/config` is retained: the full test build exposed four internal Settings service suites outside the Go internal boundary. Its attempted relocation is reverted. School Setup still receives its existing store ports from Root, so its foreign repository key retires.
2. `database/repositories/parent` → `modules/careplan/internal/adapters/parentpostgres`, ADR 0020, unchanged owner and all roles. Five setters become constructor parameters; two long methods are split without changing their predicates or outputs.
3. `models/users` → People Directory private rows. The eight conventional-entity compile assertions move beside them. Their local structural interface has exactly the original `base.Entity` methods; all eight assertions remain. `models` retains only its test-package documentation because the unchanged checker scenario `composition_reach_rule_test.go:283` requires that classified path; it contains no runtime provider, imports, factories, setters or broad composition.
4. `models/parent` → Care Plan private rows. Dates and existing tests retain their contracts.
5. `models/calendar` disappears. Its only consumers were tests of already-native Appointments values and Calendar event constants. Those imports now name the real owners; validation tests move to Appointments unchanged.
6. `services/audit` is split into Audit’s internal application and the existing persistence adapter. The days-to-timestamp scan adapter and its tests stay together. The shared retention-clock margin is one native record constant. Correction reads own their existing observation, deleting the compose pass-through and offsetting the newly counted Append method.
7. `services/database` disappears. The actual statistics types move to the native Transaction Runtime `stats` port, and the implementation/tests move to its internal application. All wire tags and assertions stay unchanged. The public factory names only the native port; no aliases or DTO copies remain.
8. `modules/delivery/compose/realtime` disappears. Root uses the existing public `realtime` types directly, removing 17 aliases.

Student Presence publishes the original `SessionRoom` value without an alias or clone. Facilities no longer imports Presence composition for room projections. Reminder HTTP runtime and Workforce/Delivery offboarding bindings move to Root from existing values, without a new root field.

Foreign composition aliases: **128 → 102** (Audit 8, realtime 17, Settings 1). Root composition: **1638 → 1633**. The candidate keeps the Audit pass-through budget at 1. Other improved budgets are lowered in the source registers.

One rule is added: `transaction-runtime.public.stats-port`, production-only, same owner. Its precedent is the existing `platform.public-application` reach: native statistics port types replace the facade’s direct application aliases. The fixed-build checker accepts this narrowing. Rules fall from 906 to 898 overall.

## Reverted native-model probes

These are observed failures against the immutable inventory base, not proposed allowances. The four model moves and their proposed rules were reverted. No failed rule remains in `policy.json`.

| Retained package | Consumer / exact rejected reach | Consequence |
|---|---|---|
| `models/activities` | `modules/timetable/compose/httpintegration`, external test: `timetable-activities/workflow-integration-test → timetable-activities/domain` | Native domain import is rejected. Replacing the legacy-model test rule with the native domain point also exposes existing `internal/domain` packages and is rejected as loosening. Tests need owner-native fixture contracts, not a private-row grant. |
| `models/schedule` | Same Timetable HTTP integration seam | Same native-fixture prerequisite; changing only the package location is insufficient. |
| `models/iot` | `modules/devicefleet/compose/httpadapter`, external test: `device-fleet/workflow-integration-test → device-fleet/domain` | Even a target-package-restricted native successor is rejected by the existing role-point guard. Mutable retained-repository fixtures remain. |
| `models/facilities` | `compose/legacy` and `compose/repositoryadapter`, internal test: `facilities/workflow-integration-test → facilities/domain`; native rows also require `facilities/domain → facilities/public` | Both rules are rejected as new permissions; keep the original model and consumers. |

A native Security Runtime probe for `dataimport/compose → auth/authorize` was also reverted. Its existing public permission functions preserve the required decisions, but the checker rejects the new `data-import/compose → security-runtime/public` rule and both compose package reaches. The documented projection-public approval does not cover this workflow composition role. The probe added no surviving rule or changed authorization behavior.

The complete remaining scope is not implemented. The tables below identify the retained callers and their boundary prerequisites; they are not assertions that every alternative has been exhausted.

## Remaining exact-key consumers

| Scope | Consumer → provider | Selected symbols | Required boundary |
|---|---|---|---|
| `external_test` | `modules/careplan/contracttest` → `models/enrollment` | `AvailabilityMatchAll`, `AvailabilityOperatorIn`, `AvailabilitySourceGradeLevel`, `CareOffering`, `CareOfferingAvailabilityCondition`, `CareOfferingAvailabilityRule`, `ChildStatusApproved`, `ChildStatusSubmitted`, `DaysOfWeekModeFixed`, `DaysOfWeekModeParentChoice`, `OfferingChangeDate`, `OfferingChangeSnapshotEntry`, `OfferingChangeStatusApproved`, `OfferingChangeStatusPending`, `OfferingChangeStatusRejected`, `OfferingChangeStatusWithdrawn`, `PhaseCareOfferingSelectionAtLeastOne`, `PhaseCareOverflowWaitlist`, `PhaseKindSchoolYear`, `SelectionRuleAtLeastOne`, `SelectionRuleAtMostOne`, `SelectionRuleExactlyOne`, `SelectionRuleOptional` | Care Plan native offering fixtures; Care Plan cannot acquire an Enrollment dependency against the DAG. |
| `external_test` | `modules/careplan/contracttest` → `services/parentmessaging` | `ChildEvent`, `Emitter`, `NewEmitter`, `PortalGuardian`, `TenantSettingsResolver` | Communication-native emitter/event contract; preserve nil behavior, payloads and decision delivery. |
| `production` | `models/delivery` → `models/base` | `CRUDRepository`, `Model`, `TenantModel` | Replace Delivery generic CRUD/row/error dependencies with owner-native persistence and error contracts before internalization. |
| `production` | `modules/communication/composition` → `modules/communication/internal/adapters/audience` | `Announcement`, `Map`, `New`, `Rows`, `Stats`, `Target`, `ViewDetail` | Projection-native public contracts or a root-bound port. Existing read-projection paths/grants must not be widened. |
| `production` | `modules/communication/parentstore` → `modules/communication/internal/adapters/parentaudience` | `Database`, `DeclarationChild`, `DeclarationHistoryLink`, `DeclarationSigner`, `New`, `ParentAnnouncementFeedItem`, `ParentAnnouncementFeedScope`, `ParentAnnouncementReminderRecipient`, `PendingAnnouncementApplicant`, `Projection` | Projection-native public contracts or a root-bound port. Existing read-projection paths/grants must not be widened. |
| `production` | `modules/communication/parentstore` → `modules/communication/internal/adapters/parentinbox` | `Database`, `New`, `ParentInboxThread`, `ParentReadCursor`, `Projection` | Projection-native public contracts or a root-bound port. Existing read-projection paths/grants must not be widened. |
| `production` | `modules/communication/staffstore` → `modules/communication/internal/adapters/staffinbox` | `Database`, `New`, `Projection` | Projection-native public contracts or a root-bound port. Existing read-projection paths/grants must not be widened. |
| `production` | `modules/dataimport/compose` → `auth/authorize` | `CanGrantRole`, `HasPermission` | Root authorization binding plus the 19 retained internal-test callers; their root-hook alternative currently reaches the forbidden application seam. |
| `production` | `modules/delivery/internal/adapters/deliverypostgres` → `database/repositories/base` | `EnsureTenantID`, `GetDB`, `NewRepository`, `Repository`, `TranslateNotFound`, `WithTenantFilter` | Replace Delivery generic CRUD/row/error dependencies with owner-native persistence and error contracts before internalization. |
| `production` | `modules/delivery/internal/adapters/deliverypostgres` → `models/base` | `DatabaseError` | Replace Delivery generic CRUD/row/error dependencies with owner-native persistence and error contracts before internalization. |
| `production` | `modules/devicescan/compose` → `auth/device` | `AuthenticatedDevice`, `AuthenticatedStaff`, `CtxDevice`, `CtxIsIoTDevice`, `CtxStaff`, `DeviceFromCtx`, `IsIoTDeviceRequest`, `StaffFromCtx` | Native device principal contract or root-bound context port, including the existing testhooks helper. |
| `production` | `modules/devicescan/compose` → `modules/peopledirectory/userscontract` | `ErrPersonNotFound` | Retained and public person-not-found sentinels are distinct instances; use an owner-native classifier without changing error identity. |
| `production` | `modules/emergencysnapshot/internal/adapters/nameorder` → `internal/collation` | `CompareGerman` | Canonical native ordering plus consumer ports; a private legacy-shared import is not a target grant. |
| `production` | `modules/enrollment/http` → `internal/collation` | `CompareSchoolClasses` | Canonical native ordering plus consumer ports; a private legacy-shared import is not a target grant. |
| `production` | `modules/facilities/compose/httpadapter` → `api/common` | `CodeAttendanceLogDisabled`, `CodeRoomsColorAlreadyInUse`, `CodeRoomsNotFound`, `ErrorConflict`, `ErrorConflictWithCode`, `ErrorForbidden`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFoundWithCode`, `HasAdminWildcard`, `PaginationParams`, `ParseID`, `ParsePagination`, `RenderError`, `RequiresPermission`, `Respond`, `RespondNoContent`, `RespondPaginated` | Separate transport/runtime bindings from native owner values and renderer ports; a composition must not retain HTTP/runtime/private implementation reach. |
| `production` | `modules/facilities/compose/httpadapter` → `internal/collation` | `CompareGerman` | Canonical native ordering plus consumer ports; a private legacy-shared import is not a target grant. |
| `production` | `modules/facilities/compose/httpadapter` → `modules/facilities/http/rooms` | `ErrHistoryDisabled`, `ExportFile`, `FailureConflict`, `FailureForbidden`, `FailureInvalid`, `FailureKind`, `FailureNotFound`, `InvalidExportError`, `Middleware`, `NewResource`, `Pagination`, `Resource`, `RoomSessionEntry`, `RoomView`, `Runtime`, `SnapshotRequest` | Separate transport/runtime bindings from native owner values and renderer ports; a composition must not retain HTTP/runtime/private implementation reach. |
| `production` | `modules/facilities/compose/httpadapter` → `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` | Separate transport/runtime bindings from native owner values and renderer ports; a composition must not retain HTTP/runtime/private implementation reach. |
| `production` | `modules/facilities/compose/httpadapter` → `services/listexport` | `Column`, `ColumnChecklist`, `ColumnID`, `ColumnRoomActivity`, `ColumnRoomBuilding`, `ColumnRoomChildCount`, `ColumnRoomFloor`, `ColumnRoomName`, `ColumnRoomStatus`, `ColumnRoomSupervision`, `ColumnStudentClass`, `ColumnStudentGroup`, `ColumnStudentName`, `Document`, `Format`, `RendererService`, `Row` | Separate transport/runtime bindings from native owner values and renderer ports; a composition must not retain HTTP/runtime/private implementation reach. |
| `production` | `modules/grouplive/internal/adapters/nameorder` → `internal/collation` | `CompareGerman` | Canonical native ordering plus consumer ports; a private legacy-shared import is not a target grant. |
| `production` | `modules/organizationtenancy/compose` → `modules/organizationtenancy/internal/adapters/operatordashboard` | `Counts`, `Map`, `New`, `OrganizationSummary`, `PWAUsageRow`, `Rows`, `SchoolSummary` | Projection-native public contracts or a root-bound port. Existing read-projection paths/grants must not be widened. |
| `production` | `modules/peopledirectory/inbound/students` → `internal/collation` | `CompareGermanNames`, `CompareSchoolClasses` | Canonical native ordering plus consumer ports; a private legacy-shared import is not a target grant. |
| `production` | `modules/requestreview/compose` → `modules/careplan/compose` | `MasterDataDirectory`, `MasterDataFieldChange`, `MasterDataFieldFacts`, `MasterDataReviewDependencies`, `NewMasterDataReviews`, `NewOfferingBookings`, `NewOfferingReviews`, `NewScheduleReviews`, `NewStudentDataRequestQueries`, `Observation`, `OfferingReviewBooking`, `OfferingReviewChild`, `OfferingReviewCourse`, `OfferingReviewCourseRef`, `OfferingReviewDependencies`, `OfferingReviewPhase`, `OfferingReviewRequest`, `PersonName`, `PickupReviewBlock`, `PickupReviewBlocks`, `PickupReviewImpact`, `ReviewBooking`, `ReviewScope`, `ReviewScopeResolver`, `ReviewStudent`, `ScheduleReviewDependencies`, `StudentDirectory` | Publish native review vocabulary, then bind builders positionally in Root; retain four test seams without private composition imports or hook growth. |
| `production` | `modules/requestreview/compose` → `modules/enrollment/compose` | `New` | Publish native review vocabulary, then bind builders positionally in Root; retain four test seams without private composition imports or hook growth. |
| `production` | `modules/requestreview/compose` → `modules/timetable/compose` | `NewClassArrivalQueries`, `NewCourseGroupQueries`, `Observation` | Publish native review vocabulary, then bind builders positionally in Root; retain four test seams without private composition imports or hook growth. |
| `production` | `modules/schoolmembership/internal/adapters/displayorder` → `internal/collation` | `CompareGermanNames`, `CompareSchoolClasses` | Canonical native ordering plus consumer ports; a private legacy-shared import is not a target grant. |
| `production` | `modules/supervisiondashboard/internal/adapters/nameorder` → `internal/collation` | `CompareGerman` | Canonical native ordering plus consumer ports; a private legacy-shared import is not a target grant. |
| `production` | `modules/timetable/compose/httpadapter` → `api/common` | `BindAction`, `CodeTimetableActivityNotOwner`, `CodeTimetableActivitySystemProtected`, `CodeTimetableActivityTemplateProtected`, `CodeTimetableOnlySupervisorReplacementRequired`, `DateFormatISO`, `ErrorBusinessRejectionOr`, `ErrorConflict`, `ErrorConflictWithCode`, `ErrorForbidden`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorRule`, `IDFetch`, `Middleware`, `MsgInvalidActivityID`, `MsgInvalidStudentID`, `MsgInvalidWeekday`, `PaginationParams`, `ParseID`, `ParseIDParam`, `ParsePagination`, `RenderError`, `RequiresAnyPermission`, `RequiresPermission`, `Respond`, `RespondPaginated`, `RespondWithError`, `RulesRenderer`, `Time`, `UnwrapRenderer` | Separate transport/runtime bindings from native owner values and renderer ports; a composition must not retain HTTP/runtime/private implementation reach. |
| `production` | `modules/timetable/compose/httpadapter` → `auth/authorize` | `CanReadStudent`, `HasAdminWildcard` | Separate transport/runtime bindings from native owner values and renderer ports; a composition must not retain HTTP/runtime/private implementation reach. |
| `production` | `modules/timetable/compose/httpadapter` → `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` | Separate transport/runtime bindings from native owner values and renderer ports; a composition must not retain HTTP/runtime/private implementation reach. |
| `production` | `modules/timetable/compose` → `auth/authorize` | `HasAdminWildcard`, `HasOperationalOverviewForResolvedStaff`, `HasPermission`, `SchoolWideActionScope` | Root-bound caller and named native presence read ports; preserve session/tenant and authorization behavior. |
| `production` | `modules/timetable/compose` → `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `PermissionsFromCtx` | Root-bound caller and named native presence read ports; preserve session/tenant and authorization behavior. |
| `production` | `modules/timetable/compose` → `modules/peopledirectory/userscontract` | `ErrPersonNotFound` | Retained and public person-not-found sentinels are distinct instances; use an owner-native classifier without changing error identity. |
| `production` | `modules/timetable/compose` → `modules/presenceprojection` | `CountNonAbsentParticipants`, `HasPlannedSlotsInRange`, `Instance`, `LegacyInstanceFilter`, `LegacyParticipantFilter`, `ListLegacyInstances`, `ListLegacyParticipants`, `ListParallelPresence`, `ListPartialAbsenceBlocks`, `ListScheduledInstancesForStudent`, `ParallelPresence`, `PartialAbsenceBlock`, `Participant`, `ScheduledInstance` | Root-bound caller and named native presence read ports; preserve session/tenant and authorization behavior. |
| `production` | `workflows/studentdeletion/compose` → `modules/timetableprojection` | `CountStudentEnrollments` | Owner-native public/contract capability or a root-bound consumer port; no new private-role grant. |

## Package-exit accounting

The inventory base has **28 direct production-package carriers** under the requested model/service/repository/legacy-compose paths. Six disappear in this candidate; **22 remain**, more than half of that package scope. Test-only packages, empty `services/iot`, already-dissolved packages and legacy-prefixed files are not counted as additional package exits. This is an exit-status count, not a claim that every possible redesign has been exhausted.

| Base package | Candidate exit |
|---|---|
| `database/repositories/base` | Retained; prerequisite below |
| `database/repositories/config` | Retained; prerequisite below |
| `database/repositories/parent` | Dissolved or relocated |
| `database/repositories/pwausage` | Retained; prerequisite below |
| `models/activities` | Retained; prerequisite below |
| `models/base` | Retained; prerequisite below |
| `models/calendar` | Dissolved or relocated |
| `models/delivery` | Retained; prerequisite below |
| `models/enrollment` | Retained; prerequisite below |
| `models/facilities` | Retained; prerequisite below |
| `models/iot` | Retained; prerequisite below |
| `models/parent` | Dissolved or relocated |
| `models/schedule` | Retained; prerequisite below |
| `models/users` | Dissolved or relocated |
| `modules/devicefleet/compose/legacy` | Retained; prerequisite below |
| `modules/facilities/compose/legacy` | Retained; prerequisite below |
| `services/audit` | Dissolved or relocated |
| `services/config` | Retained; prerequisite below |
| `services/config/defaults` | Retained; prerequisite below |
| `services/config/settingstest` | Retained; prerequisite below |
| `services/config/sideeffects` | Retained; prerequisite below |
| `services/database` | Dissolved or relocated |
| `services/import` | Retained; prerequisite below |
| `services/import/ports` | Retained; prerequisite below |
| `services/iot/staffclock` | Retained; prerequisite below |
| `services/listexport` | Retained; prerequisite below |
| `services/parentmessaging` | Retained; prerequisite below |
| `services/scheduler` | Retained; prerequisite below |

## Remaining package prerequisites

The earlier [relocation inventory](4086-relocation-inventory.md) contains the recorded source-level diagnostics for the packages below. They are retained, not newly presented as completed relocations. This candidate does not add an allowance for any of them.

| Package / family | Retained prerequisite |
|---|---|
| `models/{activities,schedule,facilities,iot}` | Reverted probes above. |
| `database/repositories/config` | Full test loading fails in `services/config/attendance_scope_test.go`, `operational_overview_scope_test.go`, `settings_multi_tenant_test.go` and `settings_request_cache_db_test.go`: Go internal visibility. They are internal suites, so importing Settings compose or Root would create a cycle. The relocation is reverted; the store port binding remains. |
| `models/{base,delivery}` and `database/repositories/base` | Delivery is still a foreign consumer of generic rows, errors and persistence. `database` also needs the repository context handoff from base. |
| `models/enrollment` | Care Plan’s foreign fixture consumer remains, plus Enrollment’s own mutable offering-row adapters. |
| `services/config`, `defaults`, `sideeffects`, `settingstest` | Native snapshot/error contracts, immutable observer/side-effect wiring, and Workforce’s foreign settings doubles. Earlier probe records six service hooks plus Registry registration/observer targets. The 28 remaining Settings composition aliases still name retained services. |
| `services/database` | Completed: native `stats` port and internal implementation replace all five public application re-exports. |
| `services/import`, `services/import/ports`, `compose/retained` | Earlier probe records twelve new path-keyed fake mutations. Preserve assertions while separating owner test support and replacing the retained aliases; no hook allowance. |
| `services/iot/staffclock` | `StampError.Error()` must retain `Cause.Error()` behavior. A counted application delegation has no owner budget; a native error-contract split needs a lawful reach. |
| `services/listexport` | Facilities is a foreign consumer. Native list/record types must replace the current application re-exports before the implementation becomes internal. |
| `services/parentmessaging` | Care Plan’s foreign emitter/event test seam above. |
| `services/scheduler` | Earlier probe records five test-injection targets and production `rotateOverdueCacheIfNewDay → overdueEmittedDay`; no path-keyed hook allowance may be added. |
| `database/repositories/pwausage` | Its composer is `observability/compose`, outside `modules/observability`; merely moving persistence there violates Go internal visibility. |
| `modules/{devicefleet,facilities}/compose/legacy` | Root still names their constructors/types. Publish native contracts and keep moved delegations measured; a filename-only move is not dissolution. |

## Foreign importer inventory

Generated once from Go AST imports and selector expressions at the inventory base. Production, same-package tests and external tests are separate. This syntactic inventory includes testhooks and conservatively includes platform-specific Go files; architecture checks remain authoritative for the fixed Linux build. Imports inside architecture fixture trees are excluded.

### care-plan

| Importer file | Scope | Provider | Symbols |
|---|---|---|---|
| `backend/api/display_http_test.go` | `internal_test` | `modules/careplan/compose` | `NewPickupSchedules` |
| `backend/api/display_pickup_fixture_test.go` | `internal_test` | `modules/careplan/compose` | `NewPickupBaselines` |
| `backend/api/active_test_helpers.go` | `production` | `modules/careplan/compose` | `CareDayDependencies`, `NewArrivalSchedules`, `NewCareDays`, `NewPickupAutoExcusal`, `NewPickupBaselines`, `NewPickupSchedules`, `PickupExcusalDependencies`, `WireCareParticipation` |
| `backend/api/auth_repository_test_helpers.go` | `production` | `modules/careplan/compose` | `NewParentEnrollmentRequestRepository`, `ParentPortalRepositories` |
| `backend/api/base.go` | `production` | `modules/careplan/compose` | `Dependencies`, `New`, `NewStudentProfiles`, `Observation`, `ReviewScope`, `StatusSlotDirectory`, `TenantAmbientDatabase`, `Today` |
| `backend/api/booking_materialization_test_helpers.go` | `production` | `modules/careplan/compose` | `BookingWithdrawals`, `NewPickupBaselines` |
| `backend/api/calendar_facts.go` | `production` | `modules/careplan/compose` | `ParentChildLink` |
| `backend/api/calendar_repository_test_helpers.go` | `production` | `modules/careplan/compose` | `NewParentChildLinks`, `ParentChildLinks`, `ParentPortalRepositories` |
| `backend/api/care_companion_students.go` | `production` | `modules/careplan/compose` | `CompanionStudent`, `CompanionStudents` |
| `backend/api/care_exit_enrollment.go` | `production` | `modules/careplan/compose` | `CareExitApplication`, `CareExitOfferingLink`, `CareExitOfferingRestore`, `CareExitOfferingSnapshot` |
| `backend/api/care_exit_enrollment_bookings.go` | `production` | `modules/careplan/compose` | `NewOfferingBookings` |
| `backend/api/care_lifecycle_owners.go` | `production` | `modules/careplan/compose` | `CareEndRecorder`, `CareExitBooking`, `CareExitBookingCap`, `CareExitBookingChanges`, `CareExitBookingRestore`, `CareLifecycleOwners` |
| `backend/api/care_lifecycle_repository_test_helpers.go` | `production` | `modules/careplan/compose` | `CareLifecycleDependencies`, `NewCareLifecycle`, `NewCompanions` |
| `backend/api/care_lifecycle_test_helpers.go` | `production` | `modules/careplan/compose` | `NewStudentDocuments` |
| `backend/api/care_offering_catalog_test_helpers.go` | `production` | `modules/careplan/compose` | `PickupResyncer`, `SourcedTemplateResyncer` |
| `backend/api/care_plan_arrival_baselines.go` | `production` | `modules/careplan/compose` | `ArrivalBaselineDependencies`, `ArrivalClassPlan`, `NewArrivalBaselines` |
| `backend/api/care_plan_arrival_schedules.go` | `production` | `modules/careplan/compose` | `ArrivalClassPlans`, `ArrivalScheduleDependencies`, `ArrivalScheduleRules`, `NewArrivalScheduleRules` |
| `backend/api/care_plan_arrival_students.go` | `production` | `modules/careplan/compose` | `ArrivalBulkStudent`, `PickupBulkStudent` |
| `backend/api/care_plan_booking_materialization.go` | `production` | `modules/careplan/compose` | `ApprovedOfferingChild`, `BookingChild`, `BookingCommands`, `BookingMaterializationDependencies`, `BookingPhase`, `BookingRequest`, `BookingRowNotFound`, `BookingStudent`, `BookingWithdrawals`, `NewBookingMaterialization`, `PickupWeekdayRow`, `RosterEnrollment`, `SourcedTemplate` |
| `backend/api/care_plan_care_conflict_port.go` | `production` | `modules/careplan/compose` | `ConflictCandidate`, `ConflictDecision`, `ParentRequestConflicts`, `StaffValueWrite` |
| `backend/api/care_plan_care_request_service.go` | `production` | `modules/careplan/compose` | `CareRequestEffectPorts`, `CareRequestEffects`, `NewCareRequestEffects`, `NewRequestConflicts`, `NewRequestCorrections`, `NewRequestDecisions`, `NewRequestDiffs`, `NewRequestEdits`, `NewRequestReads`, `NewRequestSubmissions`, `RequestCorrectionDependencies`, `RequestDecisionDependencies`, `RequestMessenger` |
| `backend/api/care_plan_class_exceptions.go` | `production` | `modules/careplan/compose` | `NewClassArrivalExceptions` |
| `backend/api/care_plan_class_times.go` | `production` | `modules/careplan/compose` | `ArrivalClassPlan`, `ArrivalClassPlans` |
| `backend/api/care_plan_documents.go` | `production` | `modules/careplan/compose` | `StudentDocumentDependencies`, `StudentDocumentPermissions` |
| `backend/api/care_plan_excused_request_students.go` | `production` | `modules/careplan/compose` | `AbsenceNotifier`, `ExcusedAbsenceRequests`, `ExcusedRequestDependencies`, `NewExcusedAbsenceRequests`, `Observation`, `PersonName`, `RequestLedger`, `RequestMessenger`, `ReviewScopeResolver`, `ReviewStudent`, `ShareVisibility`, `StaffBroadcaster`, `Today` |
| `backend/api/care_plan_excused_requests.go` | `production` | `modules/careplan/compose` | `AbsenceNotifier`, `AbsenceReport`, `ExcusedAbsenceRequests`, `Observation`, `RequestLedger`, `RequestMessenger`, `ReviewScope`, `ReviewScopeResolver`, `ShareVisibility`, `StaffBroadcaster` |
| `backend/api/care_plan_graph_wiring.go` | `production` | `modules/careplan/compose` | `ArrivalScheduleDependencies`, `ArrivalScheduleRules`, `BookingWithdrawals`, `CareLifecycleOwners`, `ExcusedAbsenceRequests`, `PartialAbsencePreview`, `PickupBulkStudents`, `PickupExtensions`, `PickupStudentLocks`, `ShareVisibility` |
| `backend/api/care_plan_offering_catalog.go` | `production` | `modules/careplan/compose` | `CareOfferingCatalogDependencies`, `CatalogRecords`, `CatalogRowNotFound`, `NewCareOfferingCatalog`, `OfferingGradeCount`, `OfferingSourceInactive`, `OfferingSourceMixedPhases`, `OfferingSourceNotFound`, `OfferingSourceOutsidePeriod`, `OfferingSourceRefusal`, `OfferingSourceRefusalKind`, `OfferingSourceTooMany`, `PickupResyncer`, `SourcedTemplateResyncer` |
| `backend/api/care_plan_pickup_adjustments.go` | `production` | `modules/careplan/compose` | `NewPickupAdjustments`, `OfferingChangeSettings`, `PickupAdjustmentDependencies` |
| `backend/api/care_plan_pickup_approval_adapter.go` | `production` | `modules/careplan/compose` | `NewPickupApprovalExceptions`, `NewPickupApprovals`, `PickupApprovalDependencies` |
| `backend/api/care_plan_pickup_excusal_timetable.go` | `production` | `modules/careplan/compose` | `PickupDayExtension`, `PickupWeekdayExtension` |
| `backend/api/care_plan_pickup_schedules.go` | `production` | `modules/careplan/compose` | `PickupBulkStudents` |
| `backend/api/care_plan_pickup_students.go` | `production` | `modules/careplan/compose` | `PickupBulkStudent` |
| `backend/api/care_plan_request_correction_adapter.go` | `production` | `modules/careplan/compose` | `RequestCorrectionEvent` |
| `backend/api/care_plan_request_decision_adapter.go` | `production` | `modules/careplan/compose` | `ReviewStudent` |
| `backend/api/care_plan_request_records.go` | `production` | `modules/careplan/compose` | `RequestCorrectionRecords`, `RequestDecisionRecords`, `RequestEditRecords`, `RequestReadRecords`, `RequestSubmissionRecords` |
| `backend/api/care_plan_request_submission_adapter.go` | `production` | `modules/careplan/compose` | `PersonName` |
| `backend/api/care_plan_schedule_records.go` | `production` | `modules/careplan/compose` | `Today` |
| `backend/api/care_plan_status_directories.go` | `production` | `modules/careplan/compose` | `StatusSlotDirectory`, `StatusStudent`, `StatusStudentDirectory` |
| `backend/api/care_plan_test_helpers.go` | `production` | `modules/careplan/compose` | `Dependencies`, `New`, `NewStudentProfiles`, `Observation`, `TenantAmbientDatabase` |
| `backend/api/care_plan_weekly_approval_adapter.go` | `production` | `modules/careplan/compose` | `NewWeeklyApprovals` |
| `backend/api/care_plan_wiring.go` | `production` | `modules/careplan/compose` | `ExcusedAbsenceRequests`, `NewCompanions`, `NewStudentDocuments`, `Observation` |
| `backend/api/care_students.go` | `production` | `modules/careplan/compose` | `CarePerson`, `CareStudent` |
| `backend/api/class_day_test_helpers.go` | `production` | `modules/careplan/compose` | `WireCareParticipation` |
| `backend/api/enrollment_booking_repository_test_helpers.go` | `production` | `modules/careplan/compose` | `NewOfferingBookings` |
| `backend/api/enrollment_bookings.go` | `production` | `modules/careplan/compose` | `NewOfferingBookings` |
| `backend/api/enrollment_flow_test_helpers.go` | `production` | `modules/careplan/compose` | `NewPickupAutoExcusal`, `PickupExcusalDependencies` |
| `backend/api/excused_requests_test_helpers.go` | `production` | `modules/careplan/compose` | `ExcusedAbsenceRequests`, `ReviewScopeResolver` |
| `backend/api/manual_partial_absences.go` | `production` | `modules/careplan/compose` | `ManualPartialAbsences`, `NewManualPartialAbsences`, `PickupExceptionReader` |
| `backend/api/master_data_decisions.go` | `production` | `modules/careplan/compose` | `DepartureModes`, `MasterDataDecisionDependencies`, `MasterDataDecisions`, `MasterDataFieldChange`, `MasterDataFieldFacts`, `MasterDataPerson`, `MasterDataPersonChange`, `MasterDataRecords`, `MasterDataStudent`, `MasterDataStudentChange`, `MasterDataWriteAudit`, `MasterDataWriteError`, `MasterDataWriteLoad`, `MasterDataWriteUpdate`, `NewMasterDataDecisions`, `NewParentRequestCoordinator`, `ParentRequestConflicts`, `ParentRequestCoordinatorDependencies`, `ParentRequestRights`, `ReviewScope`, `ReviewScopeResolver`, `ReviewStudent`, `Today` |
| `backend/api/master_data_decisions_test_helpers.go` | `production` | `modules/careplan/compose` | `MasterDataDecisions`, `ReviewScopeResolver`, `ShareVisibility` |
| `backend/api/offering_change_composition.go` | `production` | `modules/careplan/compose` | `BookingPhase`, `BookingRequest`, `ConflictCandidate`, `ConflictDecision`, `DecisionAudience`, `NewOfferingChanges`, `OfferingCarePeriod`, `OfferingCatalogState`, `OfferingChangeCatalog`, `OfferingChangeChild`, `OfferingChangeDependencies`, `ParentRequestConflicts`, `RequestEvent`, `RequestLedger`, `RequestLedgerEntry`, `RequestMessenger`, `ReviewScope`, `ReviewScopeResolver`, `ReviewStudent`, `StaffValueWrite` |
| `backend/api/offering_change_planning.go` | `production` | `modules/careplan/compose` | `CourseGroup`, `CourseOfferingReference`, `ManualPlanningOccurrence`, `OfferingChangePlanning`, `TenantAmbientDatabase` |
| `backend/api/offering_change_test_helpers.go` | `production` | `modules/careplan/compose` | `OfferingChangeCatalog` |
| `backend/api/offering_review_test_helpers.go` | `production` | `modules/careplan/compose` | `NewOfferingBookings`, `NewOfferingReviews`, `Observation`, `OfferingReviewBooking`, `OfferingReviewChild`, `OfferingReviewCourse`, `OfferingReviewCourseRef`, `OfferingReviewDependencies`, `OfferingReviewPhase`, `OfferingReviewRequest`, `PersonName`, `ReviewScope`, `ReviewStudent` |
| `backend/api/owner_roots.go` | `production` | `modules/careplan/compose` | `NewParentPortalRepositories`, `ParentPortalDependencies`, `ParentPortalRepositories`, `ParentPortalSchool` |
| `backend/api/parent_care_plan_commands.go` | `production` | `modules/careplan/compose` | `NewGuardianAbsences`, `NewGuardianPickupExceptions` |
| `backend/api/parent_care_request_identity.go` | `production` | `modules/careplan/compose` | `NewParentCareRequestQueries` |
| `backend/api/parent_care_schedule_test_helpers.go` | `production` | `modules/careplan/compose` | `NewStudentProfiles`, `Observation` |
| `backend/api/parent_portal_read_test_helpers.go` | `production` | `modules/careplan/compose` | `NewParentChildQueries`, `NewParentEnrollmentQueries`, `NewParentPhaseQueries` |
| `backend/api/parent_route_test_helpers.go` | `production` | `modules/careplan/compose` | `NewParentChildLinks`, `NewParentChildRepository`, `ParentChildLinks`, `ParentPortalRepositories`, `ReviewScope` |
| `backend/api/people_directory_composition.go` | `production` | `modules/careplan/compose` | `CompanionRecords` |
| `backend/api/people_directory_roots.go` | `production` | `modules/careplan/compose` | `NewCompanionRecords`, `NewGuardianPickupPermissions`, `NewStudentProfiles`, `Observation` |
| `backend/api/people_wiring.go` | `production` | `modules/careplan/compose` | `ReviewScopeResolver` |
| `backend/api/presence_bindings.go` | `production` | `modules/careplan/compose` | `DayLocks` |
| `backend/api/presence_projections.go` | `production` | `modules/careplan/compose` | `PickupReviewBlock`, `PickupReviewImpact` |
| `backend/api/presence_reads.go` | `production` | `modules/careplan/compose` | `TenantAmbientDatabase` |
| `backend/api/reminders_test_helpers.go` | `production` | `modules/careplan/compose` | `NewPickupAutoExcusal`, `NewPickupBaselines`, `NewPickupSchedules`, `PickupExcusalDependencies` |
| `backend/api/retention_test_helpers.go` | `production` | `modules/careplan/compose` | `NewCleanupRetention` |
| `backend/api/server.go` | `production` | `modules/careplan/compose` | `NewCleanupRetention` |
| `backend/api/service_graph_compose.go` | `production` | `modules/careplan/compose` | `ExcusedAbsenceRequests` |
| `backend/api/service_graph_compose_requests.go` | `production` | `modules/careplan/compose` | `CareLifecycleDependencies`, `LockWeeklyPickupStudents`, `NewCareLifecycle`, `NewCareOfferings`, `NewParentChildLinks`, `NewParentChildQueries`, `NewParentEnrollmentQueries`, `NewParentPhaseQueries`, `NewStudentProfiles`, `Observation`, `WireCareParticipation` |
| `backend/api/service_graph_compose_timetable.go` | `production` | `modules/careplan/compose` | `CareDayDependencies`, `NewArrivalSchedules`, `NewCareDays`, `NewPartialAbsences`, `NewPickupAutoExcusal`, `NewPickupBaselines`, `NewPickupSchedules`, `PickupExcusalDependencies` |
| `backend/api/service_graph_wiring.go` | `production` | `modules/careplan/compose` | `MasterDataDecisions`, `OfferingChangeCatalog`, `OfferingChangePlanning`, `ParentChildLink`, `ParentRequestConflicts`, `ReviewScopeResolver`, `ShareVisibility` |
| `backend/api/student_http_test_helpers.go` | `production` | `modules/careplan/compose` | `PickupReviewBlocks` |
| `backend/api/student_owner_http_test_helpers.go` | `production` | `modules/careplan/compose` | `StudentDirectory` |
| `backend/api/student_presence_roots.go` | `production` | `modules/careplan/compose` | `NewDayLocks`, `NewExceptionQueries`, `Observation`, `PickupReviewBlocks` |
| `backend/api/student_test_helpers.go` | `production` | `modules/careplan/compose` | `LockWeeklyPickupStudents`, `NewPartialAbsences`, `NewPickupAutoExcusal`, `NewPickupBaselines`, `PickupExcusalDependencies`, `WireCareParticipation` |
| `backend/api/timetable_http_test_helpers.go` | `production` | `modules/careplan/compose` | `NewPickupBaselines` |
| `backend/api/timetable_test_helpers.go` | `production` | `modules/careplan/compose` | `CareDayDependencies`, `NewCareDays`, `NewPickupBaselines`, `WireCareParticipation` |
| `backend/database/repositories/studentintegration/student_owner_rollback_test.go` | `external_test` | `modules/careplan/compose` | `NewStudentProfiles`, `Observation` |
| `backend/database/repositories/studentintegration/student_projection_cutover_test.go` | `external_test` | `modules/careplan/compose` | `NewStudentProfiles`, `Observation` |
| `backend/modules/requestreview/compose/master_data.go` | `production` | `modules/careplan/compose` | `MasterDataDirectory`, `MasterDataReviewDependencies`, `NewMasterDataReviews`, `NewStudentDataRequestQueries`, `Observation`, `ReviewScope`, `ReviewScopeResolver`, `StudentDirectory` |
| `backend/modules/requestreview/compose/master_data_directory.go` | `production` | `modules/careplan/compose` | `MasterDataFieldChange`, `MasterDataFieldFacts`, `PersonName`, `ReviewStudent` |
| `backend/modules/requestreview/compose/offering_reviews.go` | `production` | `modules/careplan/compose` | `NewOfferingBookings`, `NewOfferingReviews`, `OfferingReviewBooking`, `OfferingReviewChild`, `OfferingReviewCourse`, `OfferingReviewCourseRef`, `OfferingReviewDependencies`, `OfferingReviewPhase`, `OfferingReviewRequest`, `ReviewScopeResolver` |
| `backend/modules/requestreview/compose/schedule_reviews.go` | `production` | `modules/careplan/compose` | `NewOfferingBookings`, `NewScheduleReviews`, `PickupReviewBlock`, `PickupReviewBlocks`, `PickupReviewImpact`, `ReviewBooking`, `ReviewScopeResolver`, `ScheduleReviewDependencies`, `StudentDirectory` |
| `backend/workflows/parentportal/care_exception_auto_excusal_test.go` | `external_test` | `modules/careplan/compose` | `NewPickupAutoExcusal`, `PickupExcusalDependencies` |
| `backend/workflows/parentportal/pickup_baseline_fixture_test.go` | `external_test` | `modules/careplan/compose` | `NewPickupBaselines` |
| `backend/workflows/parentportal/care/parent_today_status_service_test.go` | `external_test` | `modules/careplan/compose` | `NewArrivalSchedules`, `NewPickupSchedules` |
| `backend/workflows/parentportal/care/pickup_baseline_fixture_test.go` | `external_test` | `modules/careplan/compose` | `NewPickupBaselines` |

### communication

| Importer file | Scope | Provider | Symbols |
|---|---|---|---|
| `backend/modules/careplan/contracttest/care_request_pickup_pill_test.go` | `external_test` | `services/parentmessaging` | `ChildEvent`, `NewEmitter`, `PortalGuardian` |
| `backend/modules/careplan/contracttest/care_request_service_test.go` | `external_test` | `services/parentmessaging` | `Emitter`, `TenantSettingsResolver` |

### delivery-platform

| Importer file | Scope | Provider | Symbols |
|---|---|---|---|
| `backend/workflows/staffoffboarding/compose/new.go` | `production` | `modules/delivery/application/realtimeevents` | `QueueGroupAccessChanged` |

### device-fleet

| Importer file | Scope | Provider | Symbols |
|---|---|---|---|
| `backend/api/display_http_test.go` | `internal_test` | `modules/devicefleet/compose/legacy` | `DashboardDependencies`, `NewDashboardSources` |
| `backend/api/cleanup_roots.go` | `production` | `modules/devicefleet/compose/legacy` | `SessionDeviceRecords` |
| `backend/api/service_graph_compose_timetable.go` | `production` | `modules/devicefleet/compose/legacy` | `DashboardDependencies`, `NewDashboardSources` |
| `backend/api/session_devices.go` | `production` | `modules/devicefleet/compose/legacy` | `SessionDeviceIDs`, `SessionDeviceRecords` |
| `backend/modules/devicescan/compose/attendance_device_test_helpers.go` | `production` | `auth/device` | `AuthenticatedDevice`, `CtxDevice`, `CtxIsIoTDevice` |
| `backend/modules/devicescan/compose/attendance_principal.go` | `production` | `auth/device` | `AuthenticatedStaff`, `CtxStaff`, `DeviceFromCtx`, `IsIoTDeviceRequest`, `StaffFromCtx` |
| `backend/modules/devicescan/compose/principals.go` | `production` | `auth/device` | `DeviceFromCtx` |

### document-rendering

| Importer file | Scope | Provider | Symbols |
|---|---|---|---|
| `backend/modules/facilities/compose/httpadapter/export.go` | `production` | `services/listexport` | `Column`, `ColumnChecklist`, `ColumnID`, `ColumnRoomActivity`, `ColumnRoomBuilding`, `ColumnRoomChildCount`, `ColumnRoomFloor`, `ColumnRoomName`, `ColumnRoomStatus`, `ColumnRoomSupervision`, `ColumnStudentClass`, `ColumnStudentGroup`, `ColumnStudentName`, `Document`, `Format`, `Row` |
| `backend/modules/facilities/compose/httpadapter/runtime.go` | `production` | `services/listexport` | `RendererService` |

### enrollment

| Importer file | Scope | Provider | Symbols |
|---|---|---|---|
| `backend/api/worker_retention_enrollment_test.go` | `internal_test` | `modules/enrollment/compose` | `New` |
| `backend/api/auth_repository_test_helpers.go` | `production` | `modules/enrollment/compose` | `New` |
| `backend/api/booking_materialization_test_helpers.go` | `production` | `modules/enrollment/compose` | `NewApprovedOfferingProjection` |
| `backend/api/care_plan_graph_wiring.go` | `production` | `modules/enrollment/compose` | `ApprovedOfferingProjection`, `CompanionGraphCoordinator` |
| `backend/api/care_plan_offering_catalog.go` | `production` | `modules/enrollment/compose` | `CareOfferingCatalogAdministration`, `CareOfferingValues`, `NewCareOfferingValues` |
| `backend/api/careplan_contract_test_helpers.go` | `production` | `modules/enrollment/compose` | `DecisionGuardianAccess` |
| `backend/api/class_day_test_helpers.go` | `production` | `modules/enrollment/compose` | `NewCareOfferingRecords` |
| `backend/api/class_list_entry_composition.go` | `production` | `modules/enrollment/compose` | `ClassListEntries`, `ClassListEntry` |
| `backend/api/delivery_test_helpers.go` | `production` | `modules/enrollment/compose` | `AdminMailSubscribers` |
| `backend/api/enrollment_application_composition.go` | `production` | `modules/enrollment/compose` | `Mail`, `NewNotifications`, `NotificationDependencies`, `NotificationModePin` |
| `backend/api/enrollment_bookings.go` | `production` | `modules/enrollment/compose` | `New` |
| `backend/api/enrollment_decision_composition.go` | `production` | `modules/enrollment/compose` | `DecisionDependencies`, `Decisions`, `ExportAccessLog`, `NewDecisions`, `OfferingAdjustmentTrail`, `PayerAudit`, `PayerChange`, `PeopleDirectory`, `Restoration`, `RestorationAudit`, `Student`, `StudentAudit`, `StudentConsents` |
| `backend/api/enrollment_decision_people.go` | `production` | `modules/enrollment/compose` | `CompanionEdge`, `DecisionStudentEnrollment`, `GuardianPhone`, `GuardianProfile`, `GuardianRole`, `GuardianRoleEmergency`, `GuardianRolePickupOnly`, `GuardianRolePrimary`, `Person`, `Student`, `StudentGuardian` |
| `backend/api/enrollment_deletion_composition.go` | `production` | `modules/enrollment/compose` | `DeletionActorSystem`, `DeletionModule`, `DeletionModuleDependencies`, `DeletionScopeChild`, `DirectoryGuardian`, `EnrollmentDeletionEvent`, `NewDeletionModule` |
| `backend/api/enrollment_intake_composition.go` | `production` | `modules/enrollment/compose` | `AdminMailSubscribers`, `ChangeRequestDependencies`, `Intake`, `IntakeDependencies`, `IntakeSettings`, `LegalSettings`, `MailOutbox`, `NewChangeRequests`, `NewIntake`, `PeopleDirectory` |
| `backend/api/enrollment_intake_test_helpers.go` | `production` | `modules/enrollment/compose` | `CareBookingChanges` |
| `backend/api/enrollment_phase_expiry.go` | `production` | `modules/enrollment/compose` | `OfferingStudent` |
| `backend/api/enrollment_report_composition.go` | `production` | `modules/enrollment/compose` | `ExportAccess`, `GuardianContactRow`, `NewReports`, `ReportDependencies`, `RosterPerson`, `RosterStudent` |
| `backend/api/enrollment_rollover_composition.go` | `production` | `modules/enrollment/compose` | `CapacityOfferings`, `CapacityPeaks`, `NewOfferingCapacity`, `NewRollovers`, `RolloverDependencies` |
| `backend/api/enrollment_test_helpers.go` | `production` | `modules/enrollment/compose` | `ApprovedOfferingProjection`, `ApprovedSelections`, `NewApprovedOfferingProjection` |
| `backend/api/enrollment_wiring.go` | `production` | `modules/enrollment/compose` | `CalendarPeriods`, `GuardianInvitationAvailability`, `NewApprovedOfferingProjection`, `NewCaptcha`, `NewCareOfferingRecords`, `NewFormSchemas`, `NewMailRenderers`, `NewPhaseExpirySnapshots`, `NewPhaseExpiryWarnings`, `NewPhases`, `PhaseDependencies`, `Phases` |
| `backend/api/filestore_test_helpers.go` | `production` | `modules/enrollment/compose` | `New` |
| `backend/api/parent_portal_enrollments_test_helpers.go` | `production` | `modules/enrollment/compose` | `New` |
| `backend/api/people_repository_test_helpers.go` | `production` | `modules/enrollment/compose` | `New` |
| `backend/api/platform_wiring.go` | `production` | `modules/enrollment/compose` | `EnrollmentDeletionDelivery`, `New` |
| `backend/api/retention_test_helpers.go` | `production` | `modules/enrollment/compose` | `New` |
| `backend/api/server.go` | `production` | `modules/enrollment/compose` | `New` |
| `backend/api/service_graph.go` | `production` | `modules/enrollment/compose` | `Intake` |
| `backend/api/service_graph_compose.go` | `production` | `modules/enrollment/compose` | `CareOfferingRecords`, `Decisions`, `Phases` |
| `backend/api/service_graph_compose_requests.go` | `production` | `modules/enrollment/compose` | `NewCareOfferingRecords`, `NewOfferingChangeRecords`, `PublicDecisions`, `WeeklyPickupHooks` |
| `backend/api/service_graph_wiring.go` | `production` | `modules/enrollment/compose` | `AdminMailSubscribers`, `ApprovedOfferingProjection`, `CareBookingChanges`, `CareBookingGates`, `CareOfferingCatalog`, `CareParticipation`, `CareWithdrawalReconciler`, `ChangeRequestRecords`, `ClassListEntries`, `CompanionGraphCoordinator`, `DecisionBookings`, `DecisionChildren`, `DecisionGuardianAccess`, `DecisionGuardians`, `DecisionLateInvites`, `DecisionPhases`, `DecisionRequests`, `DecisionSchemas`, `Decisions`, `DeletionModule`, `DeletionOwner`, `EnrollmentDeletionDelivery`, `GuardianDirectory`, `GuardianInvitationAvailability`, `GuardianStudentAuthorizer`, `Intake`, `IntakeCatalog`, `IntakeChildren`, `IntakeGuardians`, `IntakeLateInvites`, `IntakeOfferings`, `IntakeRequests`, `MailOutbox`, `ManualEnrollmentDecider`, `PhaseEligibilityGuard`, `Phases`, `PickupSchedules`, `ReportChildren`, `ReportGuardians`, `ReportOfferings`, `ReportPhases`, `ReportRequests`, `ReportSchemas`, `RequestParentChanges`, `RolloverCatalogCloner`, `RolloverChildren`, `RolloverDecider`, `RolloverPhases`, `RolloverRequests`, `RosterCompanions`, `RosterGroups`, `StudentMatches`, `SubmissionRateLimiter`, `WeeklyArrivalSchedules`, `WeeklyPickupHooks`, `WeeklyPickupSchedules` |
| `backend/api/settings_repository_test_helpers.go` | `production` | `modules/enrollment/compose` | `New` |
| `backend/api/student_deletion_owners.go` | `production` | `modules/enrollment/compose` | `New` |
| `backend/api/student_test_helpers.go` | `production` | `modules/enrollment/compose` | `NewApprovedOfferingProjection`, `NewCareOfferingRecords`, `PublicDecisions`, `WeeklyPickupHooks` |
| `backend/api/timetable_repository_test_helpers.go` | `production` | `modules/enrollment/compose` | `New` |
| `backend/modules/careplan/contracttest/care_offering_catalog_guards_test.go` | `external_test` | `models/enrollment` | `CareOffering`, `DaysOfWeekModeFixed`, `PhaseCareOverflowWaitlist`, `PhaseKindSchoolYear` |
| `backend/modules/careplan/contracttest/care_offering_catalog_test.go` | `external_test` | `models/enrollment` | `CareOffering`, `DaysOfWeekModeFixed`, `DaysOfWeekModeParentChoice`, `PhaseCareOverflowWaitlist`, `PhaseKindSchoolYear`, `SelectionRuleAtLeastOne`, `SelectionRuleAtMostOne`, `SelectionRuleExactlyOne`, `SelectionRuleOptional` |
| `backend/modules/careplan/contracttest/care_offering_records_fixture_test.go` | `external_test` | `models/enrollment` | `CareOffering`, `OfferingChangeStatusPending` |
| `backend/modules/careplan/contracttest/care_offering_repository_test.go` | `external_test` | `models/enrollment` | `CareOffering`, `DaysOfWeekModeParentChoice` |
| `backend/modules/careplan/contracttest/enrollment_decision_env_test.go` | `external_test` | `models/enrollment` | `CareOffering`, `DaysOfWeekModeParentChoice`, `PhaseCareOverflowWaitlist`, `PhaseKindSchoolYear` |
| `backend/modules/careplan/contracttest/enrollment_fixture_helpers_test.go` | `external_test` | `models/enrollment` | `CareOffering`, `DaysOfWeekModeFixed`, `PhaseCareOverflowWaitlist`, `PhaseKindSchoolYear` |
| `backend/modules/careplan/contracttest/lifecycle_care_booking_authority_integration_test.go` | `external_test` | `models/enrollment` | `ChildStatusApproved`, `ChildStatusSubmitted` |
| `backend/modules/careplan/contracttest/offering_adjustment_complete_withdrawal_test.go` | `external_test` | `models/enrollment` | `CareOffering` |
| `backend/modules/careplan/contracttest/offering_adjustment_dated_test.go` | `external_test` | `models/enrollment` | `CareOffering` |
| `backend/modules/careplan/contracttest/offering_arrival_projection_test.go` | `external_test` | `models/enrollment` | `CareOffering`, `DaysOfWeekModeFixed` |
| `backend/modules/careplan/contracttest/offering_change_full_withdrawal_test.go` | `external_test` | `models/enrollment` | `CareOffering`, `OfferingChangeStatusApproved`, `OfferingChangeStatusPending`, `PhaseCareOfferingSelectionAtLeastOne` |
| `backend/modules/careplan/contracttest/offering_change_history_test.go` | `external_test` | `models/enrollment` | `OfferingChangeStatusRejected`, `OfferingChangeStatusWithdrawn` |
| `backend/modules/careplan/contracttest/offering_change_request_automatic_test.go` | `external_test` | `models/enrollment` | `AvailabilityMatchAll`, `AvailabilityOperatorIn`, `AvailabilitySourceGradeLevel`, `CareOffering`, `CareOfferingAvailabilityCondition`, `CareOfferingAvailabilityRule`, `OfferingChangeSnapshotEntry`, `OfferingChangeStatusPending`, `OfferingChangeStatusRejected` |
| `backend/modules/careplan/contracttest/offering_change_request_service_test.go` | `external_test` | `models/enrollment` | `CareOffering`, `DaysOfWeekModeFixed`, `OfferingChangeDate`, `OfferingChangeStatusApproved`, `OfferingChangeStatusPending`, `OfferingChangeStatusRejected` |
| `backend/modules/careplan/contracttest/offering_pickup_projection_test.go` | `external_test` | `models/enrollment` | `CareOffering`, `DaysOfWeekModeFixed` |
| `backend/modules/careplan/contracttest/offering_source_service_test.go` | `external_test` | `models/enrollment` | `CareOffering`, `DaysOfWeekModeFixed`, `PhaseCareOverflowWaitlist`, `PhaseKindSchoolYear` |
| `backend/modules/requestreview/compose/offering_reviews.go` | `production` | `modules/enrollment/compose` | `New` |
| `backend/modules/requestreview/compose/schedule_reviews.go` | `production` | `modules/enrollment/compose` | `New` |

### facilities

| Importer file | Scope | Provider | Symbols |
|---|---|---|---|
| `backend/api/active_test_helpers.go` | `production` | `modules/facilities/compose/legacy` | `NewAttendanceRooms`, `OpenGroupCatalog` |
| `backend/api/groups_test_helpers.go` | `production` | `modules/facilities/compose/legacy` | `NewAttendanceRooms` |
| `backend/api/iot_data_test_helpers.go` | `production` | `modules/facilities/compose/legacy` | `HistoryProjection`, `OccupancyProjection` |
| `backend/api/presence_suite_test_helpers.go` | `production` | `modules/facilities/compose/legacy` | `NewAttendanceRooms` |
| `backend/api/presence_wiring.go` | `production` | `modules/facilities/compose/legacy` | `HistoryProjection`, `OccupancyProjection`, `OpenGroupCatalog` |
| `backend/api/room_activity_names.go` | `production` | `modules/facilities/compose/legacy` | `ActivityGroupName` |
| `backend/api/room_history_presence.go` | `production` | `modules/facilities/compose/legacy` | `RoomSessionFact` |
| `backend/api/room_occupancy_presence.go` | `production` | `modules/facilities/compose/legacy` | `RoomOccupancyFact` |
| `backend/api/service_graph_compose_timetable.go` | `production` | `modules/facilities/compose/legacy` | `NewAttendanceRooms` |
| `backend/api/settings_bindings.go` | `production` | `modules/facilities/compose/legacy` | `RegisterSettingsSideEffects`, `SettingHandler` |
| `backend/api/timetable_test_helpers.go` | `production` | `modules/facilities/compose/legacy` | `NewAttendanceRooms` |

### identity-access

| Importer file | Scope | Provider | Symbols |
|---|---|---|---|
| `backend/api/active_authorization_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `CtxPermissions` |
| `backend/api/authorization_observer_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims` |
| `backend/api/base_utility_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `NewTokenAuthWithDurations`, `TokenAuth` |
| `backend/api/care_schedule_authority_router_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/api/class_list_entries_composition_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/api/core_action_analytics_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/api/display_http_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/api/enrollment_contract_golden_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/api/guardians_composition_payment_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/api/guardians_composition_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/api/notification_email_subscription_handlers_internal_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/api/operator_booking_authority_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/api/parent_scope_middleware_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims` |
| `backend/api/readonly_preview_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims` |
| `backend/api/runtime_checkpoint_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/api/runtime_checkpoint_v2_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `MFAChallengeClaims`, `MFAChallengeScopeTenant`, `TokenAuth` |
| `backend/api/school_portal_school_api_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `MFAChallengeClaims`, `MFAChallengeScopeTenant`, `MFAEnrollmentClaims`, `MFAEnrollmentScopeSchool`, `MFAEnrollmentScopeTenant` |
| `backend/api/school_portal_school_arrival_exceptions_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/api/school_portal_school_notifications_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/api/school_portal_school_supervisions_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/api/school_scope_matrix_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/api/school_scope_middleware_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims` |
| `backend/api/security_principal_middleware_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims` |
| `backend/api/sentry_session_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `NewTokenAuthWithDurations` |
| `backend/api/statistics_http_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/api/supervision_dashboard_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims`, `CtxPermissions` |
| `backend/api/tenant_scope_middleware_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims` |
| `backend/api/active_authorization.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |
| `backend/api/base.go` | `production` | `modules/identityaccess/sessiontoken` | `ActorAccountIDFromCtx`, `Authenticator`, `ClaimsFromCtx`, `NewOpaqueCapabilityToken`, `OpaqueCapabilityFingerprint`, `PermissionsFromCtx`, `TokenAuth` |
| `backend/api/class_day_slot_lists.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |
| `backend/api/class_list_entries_composition.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/api/core_action_analytics.go` | `production` | `modules/identityaccess/sessiontoken` | `AppClaims`, `TokenAuth` |
| `backend/api/data_import_http_runtime.go` | `production` | `modules/identityaccess/sessiontoken` | `AppClaims`, `Authenticator`, `ClaimsFromCtx`, `CtxClaims` |
| `backend/api/display_http.go` | `production` | `modules/identityaccess/sessiontoken` | `Authenticator` |
| `backend/api/group_live.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |
| `backend/api/guardians_composition.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `PermissionsFromCtx` |
| `backend/api/notification_http_runtime.go` | `production` | `modules/identityaccess/sessiontoken` | `ActorAccountIDFromCtx`, `ClaimsFromCtx`, `ErrTokenUnauthorized`, `PermissionsFromCtx` |
| `backend/api/operator_booking_authority.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/api/portal_scope.go` | `production` | `modules/identityaccess/sessiontoken` | `ParentMiddleware`, `SchoolMiddleware`, `TenantMiddleware` |
| `backend/api/pwa_usage.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `ErrTokenUnauthorized` |
| `backend/api/readonly_preview.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/api/security_principal_middleware.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/api/sentry_session.go` | `production` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/api/session_route_assembly.go` | `production` | `modules/identityaccess/sessiontoken` | `Authenticator` |
| `backend/api/session_token_bindings.go` | `production` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims`, `NewOpaqueCapabilityToken`, `OpaqueCapabilityFingerprint`, `PermissionsFromCtx` |
| `backend/api/sse_http_runtime.go` | `production` | `modules/identityaccess/sessiontoken` | `Authenticator`, `ClaimsFromCtx`, `ParentMiddleware`, `PermissionsFromCtx`, `SchoolMiddleware`, `TenantMiddleware` |
| `backend/api/staff_composition.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `PermissionsFromCtx` |
| `backend/api/statistics_http_runtime.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/api/student_owner_http_test_helpers.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |
| `backend/api/supervision_dashboard.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `PermissionsFromCtx` |
| `backend/api/time_tracking_composition.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `PermissionsFromCtx` |
| `backend/api/workforce_shift_planning_http.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |
| `backend/modules/careplan/contracttest/care_request_pickup_pill_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims`, `CtxPermissions`, `PermissionsFromCtx` |
| `backend/modules/careplan/contracttest/care_request_service_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims`, `CtxPermissions` |
| `backend/modules/careplan/contracttest/lifecycle_student_companion_service_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims` |
| `backend/modules/careplan/contracttest/offering_change_request_service_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `CtxPermissions` |
| `backend/modules/careplan/http/care_request_detail_handlers_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/careplan/http/departure_exception_handlers_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/careplan/http/export_pickup_routes_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/careplan/http/open_care_absence_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `CtxPermissions` |
| `backend/modules/careplan/http/student_fixture_helpers_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/careplan/http/absence_notification_callback_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims` |
| `backend/modules/careplan/http/master_data_review_handlers_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims` |
| `backend/modules/careplan/http/offering_request_review_handlers_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims` |
| `backend/modules/careplan/http/parent_request_bulk_handlers_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims`, `CtxPermissions` |
| `backend/modules/careplan/http/parent_request_conflict_handlers_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims`, `CtxPermissions` |
| `backend/modules/careplan/http/parent_request_lifecycle_handlers_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims`, `CtxPermissions` |
| `backend/modules/careplan/http/arrival_schedule_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |
| `backend/modules/careplan/http/authorization.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `PermissionsFromCtx` |
| `backend/modules/careplan/http/care_exit_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/careplan/http/care_request_review_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/careplan/http/change_history_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/careplan/http/class_arrival_exception_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |
| `backend/modules/careplan/http/companion_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/careplan/http/documents_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `PermissionsFromCtx` |
| `backend/modules/careplan/http/excused_request_review_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/careplan/http/master_data_review_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/careplan/http/offering_request_review_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/careplan/http/parent_request_bulk_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/careplan/http/parent_request_conflict_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/careplan/http/parent_request_lifecycle_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/careplan/http/partial_absence_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |
| `backend/modules/careplan/http/pickup_adjustment_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `PermissionsFromCtx` |
| `backend/modules/careplan/http/pickup_schedule_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `PermissionsFromCtx` |
| `backend/modules/careplan/http/student_broadcasts.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/careplan/http/student_create_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |
| `backend/modules/careplan/http/student_update_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ActorAccountIDFromCtx`, `PermissionsFromCtx` |
| `backend/modules/classday/http/class_day_api_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/classday/http/class_day_arrival_exceptions_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/classday/http/class_list_entries_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/classday/http/api.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/classday/http/arrival_exceptions.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |
| `backend/modules/communication/http/operatorannouncements/announcements_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims` |
| `backend/modules/communication/http/operatorannouncements/announcements.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/communication/http/parentannouncements/api.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/communication/http/staffmessages/school_routes_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/communication/http/staffmessages/errors.go` | `production` | `modules/identityaccess/sessiontoken` | `ActorAccountIDFromCtx` |
| `backend/modules/communication/internal/parentmessages/count_scope_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/communication/internal/parentmessages/service_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims`, `CtxPermissions` |
| `backend/modules/communication/internal/parentmessages/staff_isolation_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims` |
| `backend/modules/communication/internal/parentmessages/staff_service_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims` |
| `backend/modules/communication/internal/parentmessages/service.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `PermissionsFromCtx` |
| `backend/modules/communication/internal/staffmessages/service.go` | `production` | `modules/identityaccess/sessiontoken` | `ActorAccountIDFromCtx` |
| `backend/modules/enrollment/http/change_request_review_list_router_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/enrollment/http/decision_offering_guard_router_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/enrollment/http/request_reads_router_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/enrollment/http/admin_data_correction_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/enrollment/http/admin_decision_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/enrollment/http/admin_deletion_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/enrollment/http/admin_late_invite_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/enrollment/http/admin_offering_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/enrollment/http/change_request_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/enrollment/http/export_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/enrollment/http/me_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/enrollment/http/report_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/enrollment/http/request_read_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/enrollment/http/rollover_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/enrollment/http/schema_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/facilities/compose/httpadapter/runtime_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/facilities/compose/httpadapter/runtime.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |
| `backend/modules/filestorage/http/files/api.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `PermissionsFromCtx` |
| `backend/modules/filestorage/http/files/attachments.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/organizationtenancy/inbound/operator/provisioning_internal_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims` |
| `backend/modules/organizationtenancy/inbound/operator/billing.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/organizationtenancy/inbound/operator/provisioning.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/peopledirectory/contracttest/caregiver_capability_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims` |
| `backend/modules/peopledirectory/contracttest/student_audit_composition_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims` |
| `backend/modules/peopledirectory/inbound/birthdays/api.go` | `production` | `modules/identityaccess/sessiontoken` | `ActorAccountIDFromCtx` |
| `backend/modules/peopledirectory/inbound/students/birthdays_api_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/peopledirectory/inbound/students/photo_routes_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/peopledirectory/inbound/students/student_fixture_helpers_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/peopledirectory/inbound/students/family_protection_handlers_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims`, `CtxPermissions` |
| `backend/modules/peopledirectory/inbound/students/master_data_review_handlers_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims` |
| `backend/modules/peopledirectory/inbound/students/authorization.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |
| `backend/modules/peopledirectory/inbound/students/change_history_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |
| `backend/modules/peopledirectory/inbound/students/family_protection_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `PermissionsFromCtx` |
| `backend/modules/peopledirectory/inbound/students/note_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `PermissionsFromCtx` |
| `backend/modules/presenceprojection/http/care_exit_handlers_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/presenceprojection/http/export_birthday_routes_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/presenceprojection/http/health_list_export_routes_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/presenceprojection/http/student_fixture_helpers_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/presenceprojection/http/attendance_history_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `PermissionsFromCtx` |
| `backend/modules/presenceprojection/http/authorization.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `PermissionsFromCtx` |
| `backend/modules/presenceprojection/http/day_log_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `PermissionsFromCtx` |
| `backend/modules/presenceprojection/http/day_planning.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |
| `backend/modules/presenceprojection/http/health_list_export.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/presenceprojection/http/list_helpers.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |
| `backend/modules/presenceprojection/http/status_day_overview_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/presenceprojection/http/student_create_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |
| `backend/modules/presenceprojection/http/student_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |
| `backend/modules/requestreview/http/change_request_access_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/requestreview/http/change_request_review_golden_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/requestreview/http/parent_absence_review_scope_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/requestreview/http/parent_request_review_scope_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/requestreview/http/student_fixture_helpers_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/requestreview/http/change_request_list_handlers_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims`, `CtxPermissions` |
| `backend/modules/requestreview/http/request_review_identity_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims`, `CtxPermissions` |
| `backend/modules/requestreview/http/change_request_access_handler.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |
| `backend/modules/requestreview/http/request_review_identity.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `PermissionsFromCtx` |
| `backend/modules/schoolcalendar/portal/compose/service_integration_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims` |
| `backend/modules/schoolstructure/http/api.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |
| `backend/modules/studentpresence/http/student_fixture_helpers_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/studentpresence/http/status_day_internal_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims`, `CtxPermissions` |
| `backend/modules/studentpresence/http/authorization.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `PermissionsFromCtx` |
| `backend/modules/studentpresence/http/pickup_schedule_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/studentpresence/http/status_day_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `PermissionsFromCtx` |
| `backend/modules/timetable/compose/operation_caller.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `PermissionsFromCtx` |
| `backend/modules/timetable/compose/httpadapter/enrollment_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |
| `backend/modules/timetable/compose/httpadapter/helpers.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |
| `backend/modules/timetable/compose/httpintegration/activity_access_control_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/timetable/compose/httpintegration/category_api_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/timetable/http/betreuungsplan_export_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `CtxPermissions` |
| `backend/modules/timetable/http/conflict_acks_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims` |
| `backend/modules/timetable/http/instances_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims` |
| `backend/modules/timetable/http/operations_unit_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/modules/timetable/http/participants_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `CtxPermissions` |
| `backend/modules/timetable/http/pickup_extensions_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `CtxPermissions` |
| `backend/modules/timetable/http/student_day_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `CtxPermissions` |
| `backend/modules/timetable/http/templates_split_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `CtxPermissions` |
| `backend/modules/timetable/http/betreuungsplan_export.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |
| `backend/modules/timetable/http/conflict_acks.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/timetable/http/deviations.go` | `production` | `modules/identityaccess/sessiontoken` | `ActorAccountIDFromCtx` |
| `backend/modules/timetable/http/instances.go` | `production` | `modules/identityaccess/sessiontoken` | `ActorAccountIDFromCtx`, `AppClaims`, `ClaimsFromCtx`, `CtxClaims` |
| `backend/modules/timetable/http/instances_bulk_cancel.go` | `production` | `modules/identityaccess/sessiontoken` | `ActorAccountIDFromCtx` |
| `backend/modules/timetable/http/instances_convert.go` | `production` | `modules/identityaccess/sessiontoken` | `ActorAccountIDFromCtx` |
| `backend/modules/timetable/http/instances_list.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/timetable/http/instances_update.go` | `production` | `modules/identityaccess/sessiontoken` | `ActorAccountIDFromCtx` |
| `backend/modules/timetable/http/move_staff.go` | `production` | `modules/identityaccess/sessiontoken` | `ActorAccountIDFromCtx` |
| `backend/modules/timetable/http/operations.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `PermissionsFromCtx` |
| `backend/modules/timetable/http/participants.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |
| `backend/modules/timetable/http/replan_week.go` | `production` | `modules/identityaccess/sessiontoken` | `ActorAccountIDFromCtx` |
| `backend/modules/timetable/http/school_supervisions.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/modules/timetable/http/student_day.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |
| `backend/modules/timetable/http/substitutions_bulk.go` | `production` | `modules/identityaccess/sessiontoken` | `ActorAccountIDFromCtx` |
| `backend/modules/timetable/http/templates_split.go` | `production` | `modules/identityaccess/sessiontoken` | `ActorAccountIDFromCtx` |
| `backend/modules/timetable/http/understaffed.go` | `production` | `modules/identityaccess/sessiontoken` | `ActorAccountIDFromCtx` |
| `backend/test/fixtures.go` | `production` | `modules/identityaccess/sessiontoken` | `NewTokenAuthWithDurations`, `TokenAuth` |
| `backend/test/hooks_test_helpers.go` | `production` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/test/identity_context.go` | `production` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims`, `CtxPermissions` |
| `backend/test/portal_session_gates.go` | `production` | `modules/identityaccess/sessiontoken` | `Authenticator`, `ClaimsFromCtx`, `TenantMiddleware` |
| `backend/test/request_actor.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/test/request_review_fixture.go` | `production` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims`, `CtxPermissions` |
| `backend/test/route_claims.go` | `production` | `modules/identityaccess/sessiontoken` | `AppClaims`, `ClaimsFromCtx`, `CtxClaims`, `CtxEnrollmentClaims`, `CtxPermissions`, `CtxRefreshToken`, `MFAEnrollmentClaims`, `MFAEnrollmentScopePlatform`, `MFAEnrollmentScopeTenant`, `OpaqueCapabilityFingerprint`, `PermissionsFromCtx`, `TokenAuth` |
| `backend/test/session_verifier.go` | `production` | `modules/identityaccess/sessiontoken` | `NewTokenAuthWithDurations`, `TokenAuth` |
| `backend/workflows/parentportal/excused_request_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `CtxPermissions` |
| `backend/workflows/parentportal/parent_master_data_service_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims` |
| `backend/workflows/parentportal/http/child_write_handlers_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/workflows/parentportal/http/me_handlers_test.go` | `internal_test` | `modules/identityaccess/sessiontoken` | `AppClaims`, `CtxClaims` |
| `backend/workflows/parentportal/http/child_write_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `ErrTokenUnauthorized` |
| `backend/workflows/parentportal/http/enrollment_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `ErrTokenUnauthorized` |
| `backend/workflows/parentportal/http/me_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `ErrTokenUnauthorized` |
| `backend/workflows/parentportal/http/notification_preference_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx` |
| `backend/workflows/parentportal/http/push_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `ErrTokenUnauthorized` |
| `backend/workflows/parentportal/http/pwa_usage_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `ClaimsFromCtx`, `ErrTokenUnauthorized` |
| `backend/workflows/reminderdelivery/compose/http_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/workflows/studentdeletion/http/student_fixture_helpers_test.go` | `external_test` | `modules/identityaccess/sessiontoken` | `AppClaims` |
| `backend/workflows/studentdeletion/http/care_exit_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |
| `backend/workflows/studentdeletion/http/student_delete_handlers.go` | `production` | `modules/identityaccess/sessiontoken` | `PermissionsFromCtx` |

### legacy-shared

| Importer file | Scope | Provider | Symbols |
|---|---|---|---|
| `backend/modules/emergencysnapshot/internal/adapters/nameorder/legacy.go` | `production` | `internal/collation` | `CompareGerman` |
| `backend/modules/enrollment/http/care_usage_export.go` | `production` | `internal/collation` | `CompareSchoolClasses` |
| `backend/modules/enrollment/http/class_roster_export.go` | `production` | `internal/collation` | `CompareSchoolClasses` |
| `backend/modules/facilities/compose/httpadapter/export.go` | `production` | `internal/collation` | `CompareGerman` |
| `backend/modules/grouplive/internal/adapters/nameorder/legacy.go` | `production` | `internal/collation` | `CompareGerman` |
| `backend/modules/peopledirectory/inbound/students/student_list_handlers.go` | `production` | `internal/collation` | `CompareSchoolClasses` |
| `backend/modules/peopledirectory/inbound/students/student_resource.go` | `production` | `internal/collation` | `CompareGermanNames`, `CompareSchoolClasses` |
| `backend/modules/schoolmembership/internal/adapters/displayorder/displayorder.go` | `production` | `internal/collation` | `CompareGermanNames`, `CompareSchoolClasses` |
| `backend/modules/supervisiondashboard/internal/adapters/nameorder/legacy.go` | `production` | `internal/collation` | `CompareGerman` |

### operator-dashboard-view

| Importer file | Scope | Provider | Symbols |
|---|---|---|---|
| `backend/modules/organizationtenancy/compose/provisioning.go` | `production` | `modules/organizationtenancy/internal/adapters/operatordashboard` | `Counts`, `Map`, `New`, `OrganizationSummary`, `PWAUsageRow`, `Rows`, `SchoolSummary` |

### parent-announcement-audience

| Importer file | Scope | Provider | Symbols |
|---|---|---|---|
| `backend/modules/communication/parentstore/announcements.go` | `production` | `modules/communication/internal/adapters/parentaudience` | `Database`, `DeclarationHistoryLink`, `New`, `ParentAnnouncementFeedItem`, `ParentAnnouncementFeedScope`, `ParentAnnouncementReminderRecipient`, `PendingAnnouncementApplicant`, `Projection` |
| `backend/modules/communication/parentstore/declarations.go` | `production` | `modules/communication/internal/adapters/parentaudience` | `DeclarationChild`, `DeclarationHistoryLink`, `DeclarationSigner` |

### parent-message-inbox

| Importer file | Scope | Provider | Symbols |
|---|---|---|---|
| `backend/modules/communication/parentstore/parentstore.go` | `production` | `modules/communication/internal/adapters/parentinbox` | `Database` |
| `backend/modules/communication/parentstore/reads.go` | `production` | `modules/communication/internal/adapters/parentinbox` | `New`, `ParentInboxThread`, `ParentReadCursor`, `Projection` |

### people-directory

| Importer file | Scope | Provider | Symbols |
|---|---|---|---|
| `backend/models/entity_interfaces_test.go` | `internal_test` | `models/users` | `GuardianProfile`, `Guest`, `Person`, `PrivacyConsent`, `Staff`, `Student`, `StudentGuardian`, `Teacher` |
| `backend/modules/devicescan/compose/people.go` | `production` | `modules/peopledirectory/userscontract` | `ErrPersonNotFound` |
| `backend/modules/timetable/compose/operation_caller.go` | `production` | `modules/peopledirectory/userscontract` | `ErrPersonNotFound` |

### presence-legacy-view

| Importer file | Scope | Provider | Symbols |
|---|---|---|---|
| `backend/modules/timetable/compose/presence_reads.go` | `production` | `modules/presenceprojection` | `CountNonAbsentParticipants`, `HasPlannedSlotsInRange`, `Instance`, `LegacyInstanceFilter`, `LegacyParticipantFilter`, `ListLegacyInstances`, `ListLegacyParticipants`, `ListParallelPresence`, `ListPartialAbsenceBlocks`, `ListScheduledInstancesForStudent`, `ParallelPresence`, `PartialAbsenceBlock`, `Participant`, `ScheduledInstance` |

### security-runtime

| Importer file | Scope | Provider | Symbols |
|---|---|---|---|
| `backend/modules/dataimport/compose/authorization.go` | `production` | `auth/authorize` | `CanGrantRole`, `HasPermission` |
| `backend/modules/timetable/compose/operation_caller.go` | `production` | `auth/authorize` | `HasAdminWildcard`, `HasOperationalOverviewForResolvedStaff`, `HasPermission`, `SchoolWideActionScope` |
| `backend/modules/timetable/compose/httpadapter/enrollment_handlers.go` | `production` | `auth/authorize` | `CanReadStudent` |
| `backend/modules/timetable/compose/httpadapter/helpers.go` | `production` | `auth/authorize` | `HasAdminWildcard` |

### settings-platform

| Importer file | Scope | Provider | Symbols |
|---|---|---|---|
| `backend/modules/schoolsetup/compose/compose.go` | `production` | `database/repositories/config` | `NewSchoolSetupRepository`, `NewStaffOnboardingRepository` |
| `backend/modules/workforce/internal/application/timetracking/staff_time_export_datev_test.go` | `external_test` | `services/config/settingstest` | `New`, `UnitDays`, `UnitHours`, `Values` |
| `backend/modules/workforce/internal/application/timetracking/work_schedule_targets_adapter_test.go` | `external_test` | `services/config/settingstest` | `ScheduleRow`, `Schedules` |
| `backend/modules/workforce/internal/application/timetracking/work_time_target_model_adapter_test.go` | `external_test` | `services/config/settingstest` | `Template`, `TemplateEntry`, `Templates` |

### staff-announcement-audience

| Importer file | Scope | Provider | Symbols |
|---|---|---|---|
| `backend/modules/communication/composition/new.go` | `production` | `modules/communication/internal/adapters/audience` | `Announcement`, `Map`, `New`, `Rows`, `Stats`, `Target`, `ViewDetail` |

### staff-message-inbox

| Importer file | Scope | Provider | Symbols |
|---|---|---|---|
| `backend/modules/communication/staffstore/repositories.go` | `production` | `modules/communication/internal/adapters/staffinbox` | `New`, `Projection` |
| `backend/modules/communication/staffstore/staffstore.go` | `production` | `modules/communication/internal/adapters/staffinbox` | `Database` |

### student-presence

| Importer file | Scope | Provider | Symbols |
|---|---|---|---|
| `backend/api/data_access_audit_test.go` | `internal_test` | `modules/studentpresence/compose/presenceservice` | `DataAccessEvent` |
| `backend/api/deletion_audit_test.go` | `internal_test` | `modules/studentpresence/compose/presenceservice` | `DeletionEvent` |
| `backend/api/active_test_helpers.go` | `production` | `modules/studentpresence/compose/presenceservice` | `NewPresence`, `NewPresenceOperations`, `PresenceDependencies`, `PresenceOperations`, `WithPresenceSettings` |
| `backend/api/attendance_education_groups.go` | `production` | `modules/studentpresence/compose/presenceservice` | `AttendanceEducationGroups`, `EducationGroupRoom` |
| `backend/api/attendance_staff.go` | `production` | `modules/studentpresence/compose/presenceservice` | `AttendanceStaff` |
| `backend/api/attendance_staff_names.go` | `production` | `modules/studentpresence/compose/presenceservice` | `AttendanceStaffNames` |
| `backend/api/base.go` | `production` | `modules/studentpresence/compose/presenceservice` | `NewPresenceOperations` |
| `backend/api/data_access_audit.go` | `production` | `modules/studentpresence/compose/presenceservice` | `DataAccessAudit`, `DataAccessEvent` |
| `backend/api/deletion_audit.go` | `production` | `modules/studentpresence/compose/presenceservice` | `DeletionAudit`, `DeletionEvent` |
| `backend/api/groups_test_helpers.go` | `production` | `modules/studentpresence/compose/presenceservice` | `NewPresence`, `PresenceDependencies` |
| `backend/api/history_room_names.go` | `production` | `modules/studentpresence/compose/presenceservice` | `HistoryRoomReader` |
| `backend/api/history_slots.go` | `production` | `modules/studentpresence/compose/presenceservice` | `HistorySlotReader` |
| `backend/api/people_directory_roots.go` | `production` | `modules/studentpresence/compose/presenceservice` | `PresenceStudents` |
| `backend/api/people_wiring.go` | `production` | `modules/studentpresence/compose/presenceservice` | `StatusDayOverviewPeople` |
| `backend/api/presence_wiring.go` | `production` | `modules/studentpresence/compose/presenceservice` | `AttendanceStaffNames`, `NewStatusDays`, `SessionDeviceDirectory` |
| `backend/api/school_projection_adapters.go` | `production` | `modules/studentpresence/compose/presenceservice` | `School`, `SchoolQuery` |
| `backend/api/service_graph_compose_requests.go` | `production` | `modules/studentpresence/compose/presenceservice` | `NewPresenceCleanup` |
| `backend/api/service_graph_compose_timetable.go` | `production` | `modules/studentpresence/compose/presenceservice` | `NewPresence`, `PresenceDependencies`, `WithGuardianWaker`, `WithPresenceSettings`, `WithPresenceTenantRuntime` |
| `backend/api/service_graph_compose_views.go` | `production` | `modules/studentpresence/compose/presenceservice` | `NewStatusDayOverviews`, `NewStudentHistory` |
| `backend/api/service_graph_wiring.go` | `production` | `modules/studentpresence/compose/presenceservice` | `AttendanceActivityGroups`, `AttendanceEducationGroups`, `AttendanceStaff`, `AttendanceStaffNames`, `CrossTenantRepo`, `DataAccessAudit`, `DeletionAudit`, `HistoryRoomReader`, `HistorySlotReader`, `PresenceStudents`, `SchoolQuery`, `SessionDeviceDirectory`, `StatusDayOverviewPeople`, `StudentDisplayReader` |
| `backend/api/session_devices.go` | `production` | `modules/studentpresence/compose/presenceservice` | `SessionDeviceDirectory` |
| `backend/api/student_display_projection.go` | `production` | `modules/studentpresence/compose/presenceservice` | `StudentDisplayFacts` |
| `backend/api/student_owner_http_test_helpers.go` | `production` | `modules/studentpresence/compose/presenceservice` | `NewStatusDayOverviews`, `NewStatusDays`, `NewStudentHistory` |
| `backend/api/student_records.go` | `production` | `modules/studentpresence/compose/presenceservice` | `PresenceStudents`, `StatusDayOverviewPeople` |
| `backend/api/student_status_day_http_test_helpers.go` | `production` | `modules/studentpresence/compose/presenceservice` | `NewStatusDays`, `StudentStatusDayRepository` |
| `backend/api/student_test_helpers.go` | `production` | `modules/studentpresence/compose/presenceservice` | `DataAccessAudit`, `HistorySlotReader`, `ManualPartialAbsenceReader`, `NewStatusDays`, `StatusDayOverviewPeople` |
| `backend/api/timetable_test_helpers.go` | `production` | `modules/studentpresence/compose/presenceservice` | `NewPresence`, `PresenceDependencies`, `WithPresenceSettings` |
| `backend/cmd/cleanup_test.go` | `internal_test` | `modules/studentpresence/compose/presenceservice` | `NewPresence`, `PresenceDependencies`, `WithPresenceSettings` |
| `backend/cmd/cleanup_roots.go` | `production` | `modules/studentpresence/compose/presenceservice` | `NewPresence`, `NewPresenceCleanup`, `PresenceDependencies`, `WithPresenceSettings`, `WithPresenceTenantRuntime` |
| `backend/modules/careplan/contracttest/lifecycle_care_lifecycle_integration_test.go` | `external_test` | `modules/studentpresence/compose/presenceservice` | `NewPresence`, `PresenceDependencies`, `WithPresenceSettings` |
| `backend/modules/facilities/compose/legacy/attendance_rooms.go` | `production` | `modules/studentpresence/compose/presenceservice` | `AttendanceRooms`, `SessionRoom` |
| `backend/workflows/parentportal/parent_write_service_test.go` | `external_test` | `modules/studentpresence/compose/presenceservice` | `NewStatusDays` |

### tenant-runtime

| Importer file | Scope | Provider | Symbols |
|---|---|---|---|
| `backend/api/server_error_reporting_test.go` | `internal_test` | `api/common` | `ErrUnauthorized`, `ErrorBadGatewayWrap`, `ErrorClientClosed`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorTooManyRequests`, `ErrorUnauthorized`, `RenderError`, `RespondWithError`, `SkipServerErrorReport` |
| `backend/api/settings_composition_test.go` | `internal_test` | `api/common` | `CodeGeneralInput`, `CodeSettingsAttendanceRetentionBelowVisible`, `CodeSettingsAttendanceVisibleExceedsRetention`, `RenderError` |
| `backend/api/authorization_observer_test_helpers.go` | `production` | `api/common` | `RequiresPermission` |
| `backend/api/base.go` | `production` | `api/common` | `CodeFeedbackDisabled`, `CodeMealsParticipationCutoffPassed`, `CodeMealsPlanDisabled`, `CodeMealsRegistrationDisabled`, `ErrResponse`, `ErrorConflict`, `ErrorConflictWithCode`, `ErrorForbiddenWithCode`, `ErrorInternalServerRenderer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorRule`, `FixedCodedRenderer`, `FixedRenderer`, `IsLocalSeedRequest`, `Middleware`, `OperatorInternal`, `OperatorInvalidRequest`, `PaginationParams`, `ParseInt64IDWithError`, `ParsePagination`, `ProblemResponseMiddleware`, `RenderError`, `RenderStatus`, `RequiresAnyPermission`, `RequiresPermission`, `Respond`, `RespondNoContent`, `RespondPaginated`, `RespondWithJSON`, `RoutePattern`, `RulesRenderer`, `ServeFile`, `ServeImage`, `TenantRequestEvent`, `TenantRequestObserverMiddleware`, `TenantRuntimeMiddleware`, `TenantRuntimeMissingTenant`, `TenantRuntimeObservation`, `TenantRuntimeObserverMiddleware`, `TenantRuntimeResponseWrite`, `TenantRuntimeTransaction` |
| `backend/api/class_list_entries_composition.go` | `production` | `api/common` | `CodeStudentsClassListEntryAssignMismatch`, `CodeStudentsClassListEntryDuplicate`, `CodeStudentsClassListEntryStudentExists`, `CodeStudentsClassListEntryStudentNotFound`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFound`, `Middleware`, `RenderError`, `RequiresPermission`, `Respond` |
| `backend/api/data_import_http_runtime.go` | `production` | `api/common` | `BusinessRejection`, `ErrResponse`, `ErrorForbiddenMessage`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorUnauthorized`, `RenderError`, `RequiresAnyPermission`, `Respond`, `TenantOperationMiddleware`, `TenantTxMiddleware` |
| `backend/api/display_http.go` | `production` | `api/common` | `CodeDisplayDisabled`, `ErrorForbidden`, `ErrorForbiddenWithCode`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorNotFound`, `ParseInt64IDWithError`, `RenderError`, `RequiresAnyPermission`, `RequiresPermission`, `TenantTxMiddleware` |
| `backend/api/enrollment_route_ports.go` | `production` | `api/common` | `GuardianProfileChildData`, `GuardianProfileData` |
| `backend/api/group_live.go` | `production` | `api/common` | `HasEffectiveAdminScope`, `IsAssignmentBoundPortal` |
| `backend/api/guardians_composition.go` | `production` | `api/common` | `CodeStudentsGuardianAccountHolderTooLong`, `CodeStudentsGuardianEmailTaken`, `CodeStudentsGuardianIbanInvalid`, `ErrorConflict`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidOnField`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFound`, `ErrorUnauthorized`, `HasAdminWildcard`, `HasPermission`, `PaginationParams`, `ParsePagination`, `RenderError`, `RequiresPermission`, `Respond`, `RespondPaginated`, `ShouldExposeSeedInvitationToken` |
| `backend/api/iot_http_composition.go` | `production` | `api/common` | `Middleware`, `TenantTxMiddleware` |
| `backend/api/iot_http_metrics.go` | `production` | `api/common` | `RoutePattern` |
| `backend/api/iot_http_runtime.go` | `production` | `api/common` | `CodeGeneralBusinessRejection`, `ErrorBadGatewayWrap`, `ErrorConflict`, `ErrorConflictOnField`, `ErrorConflictWithCode`, `ErrorConflictWithDetails`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInternalServerWrapWithCode`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFound`, `ErrorNotFoundWithCode`, `ErrorServiceUnavailable`, `ErrorTooManyRequests`, `ErrorUnauthorizedWithCode`, `ParseIDParam`, `RenderError`, `RequiresPermission`, `Respond`, `SkipServerErrorReport` |
| `backend/api/notification_http_runtime.go` | `production` | `api/common` | `CodeCommunicationNotificationsDisabled`, `ErrorConflict`, `ErrorConflictWithCode`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorUnauthorized`, `Middleware`, `RenderError`, `Respond`, `TenantOperationMiddleware` |
| `backend/api/operator_booking_authority.go` | `production` | `api/common` | `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/api/owner_http_mounts.go` | `production` | `api/common` | `Middleware` |
| `backend/api/owner_http_test_helpers.go` | `production` | `api/common` | `Middleware` |
| `backend/api/pwa_usage.go` | `production` | `api/common` | `ErrorInternalServerWrap`, `ErrorUnauthorized`, `Middleware`, `RenderError`, `Respond` |
| `backend/api/readonly_preview.go` | `production` | `api/common` | `ReadOnlyPreviewGuard` |
| `backend/api/readonly_preview_test_helpers.go` | `production` | `api/common` | `CodeIdentityReadOnlyPreview` |
| `backend/api/request_id.go` | `production` | `api/common` | `ProblemResponseMiddleware`, `RenderStatus` |
| `backend/api/runtime_metrics.go` | `production` | `api/common` | `NewStatusRecorder`, `RenderStatus`, `RoutePattern` |
| `backend/api/school_setup_composition.go` | `production` | `api/common` | `CurrentPrincipal`, `ErrorConflictWithCode`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `RenderError`, `RequiresAnyPermission`, `Respond` |
| `backend/api/school_test_helpers.go` | `production` | `api/common` | `MFAResendResponse` |
| `backend/api/security_principal_middleware.go` | `production` | `api/common` | `PrincipalFromFacts` |
| `backend/api/security_principal_test_helpers.go` | `production` | `api/common` | `CurrentPrincipal`, `RequiresAllPermissions`, `RequiresAnyPermission`, `RequiresPermission` |
| `backend/api/sentry_session_test_helpers.go` | `production` | `api/common` | `ErrResponse`, `ErrorInternalServer`, `RenderError` |
| `backend/api/server.go` | `production` | `api/common` | `RenderStatus` |
| `backend/api/server_error_reporting.go` | `production` | `api/common` | `NewServerErrorCapture`, `RoutePattern`, `UnmatchedRoute`, `WithServerErrorCapture` |
| `backend/api/session_route_assembly.go` | `production` | `api/common` | `Middleware`, `ProtectedParentGroup`, `ProtectedSchoolRoutes`, `ProtectedTenantRoutes` |
| `backend/api/settings_composition.go` | `production` | `api/common` | `CodeSettingsAttendanceRetentionBelowVisible`, `CodeSettingsAttendanceVisibleExceedsRetention`, `CurrentPrincipal`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithDetails`, `ErrorNotFound`, `ParseImage`, `ParsePDFWithLimits`, `RenderError`, `Respond`, `RespondNoContent`, `TenantOperationMiddleware` |
| `backend/api/settings_permissions.go` | `production` | `api/common` | `CurrentPrincipal`, `Middleware`, `RequiresAllPermissions`, `RequiresAnyPermission` |
| `backend/api/staff_composition.go` | `production` | `api/common` | `ErrorConflictMessage`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorUnauthorized`, `HasPermission`, `IsNotFound`, `Middleware`, `RenderError`, `RenderStatus`, `RequiresAnyPermission`, `RequiresPermission`, `Respond`, `ServeImage` |
| `backend/api/statistics_http_runtime.go` | `production` | `api/common` | `RequiresAllPermissions` |
| `backend/api/student_owner_http_test_helpers.go` | `production` | `api/common` | `Middleware` |
| `backend/api/supervision_dashboard.go` | `production` | `api/common` | `HasEffectiveAdminScope`, `IsAssignmentBoundPortal` |
| `backend/api/upload_ports.go` | `production` | `api/common` | `SaveImage`, `SavePDF` |
| `backend/api/workforce_absence_types_http.go` | `production` | `api/common` | `ErrorConflict`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorUnauthorized`, `ParseID`, `ParseIDParam`, `RenderError`, `RequiresAnyPermission`, `RequiresPermission`, `Respond` |
| `backend/api/workforce_shift_planning_http.go` | `production` | `api/common` | `CanExportInternalPlan`, `CodeWorkforceReplacementOutsideOrigin`, `CodeWorkforceShiftChanged`, `CodeWorkforceShiftDuplicate`, `CodeWorkforceShiftHasReplacements`, `CodeWorkforceShiftOverlap`, `CodeWorkforceShiftSeriesNoOccurrences`, `CodeWorkforceShiftSeriesOutsidePeriod`, `CodeWorkforceShiftSeriesWeekCycleMissing`, `CodeWorkforceShiftTypeInactive`, `ErrorConflict`, `ErrorConflictWithCode`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFound`, `ErrorUnauthorized`, `ParseID`, `RenderError`, `RequiresPermission`, `Respond` |
| `backend/api/workforce_substitutions_http.go` | `production` | `api/common` | `ErrResponse`, `FieldError`, `RenderError`, `Respond` |
| `backend/api/workforce_work_time_models_http.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ParseID`, `RenderError`, `RequiresPermission`, `Respond` |
| `backend/modules/careplan/http/care_request_detail_handlers_test.go` | `external_test` | `api/common` | `CodeCareRequestNotFound` |
| `backend/modules/careplan/http/companion_stale_plan_test.go` | `external_test` | `api/common` | `CodeStudentsCompanionsChanged` |
| `backend/modules/careplan/http/student_fixture_helpers_test.go` | `external_test` | `api/common` | `Middleware` |
| `backend/modules/careplan/http/care_exit_error_codes_test.go` | `internal_test` | `api/common` | `CodeStudentsCareExitAlreadyEffective`, `CodeStudentsCareExitBlocked`, `CodeStudentsCareExitDayInPast`, `CodeStudentsCareExitInvalidReason`, `CodeStudentsCareExitNoStudents`, `CodeStudentsCareExitNotPlanned`, `CodeStudentsCareExitNoteNotAllowed`, `CodeStudentsCareExitNoteRequired`, `CodeStudentsCareExitNoteTooLong`, `CodeStudentsCareExitPreviewChanged`, `CodeStudentsCareExitTooManyStudents`, `CodeStudentsCareResumeMissing`, `CodeStudentsCareResumeNotChecked`, `CodeStudentsCareResumeNotEnded`, `CodeStudentsCareResumeStartInPast`, `CodeStudentsCareWithdrawalAfterGap`, `CodeStudentsCareWithdrawalAlreadyResolved`, `CodeStudentsCareWithdrawalDateInvalid`, `CodeStudentsCareWithdrawalNotFound`, `ErrResponse` |
| `backend/modules/careplan/http/care_plan_decision_errors_golden_test.go` | `internal_test` | `api/common` | `CodeStudentsStaffValueInvalid`, `ErrResponse`, `ErrorClassCode` |
| `backend/modules/careplan/http/companion_lock_error_test.go` | `internal_test` | `api/common` | `CodeStudentsCompanionLockBusy`, `CodeStudentsCompanionsChanged`, `ErrResponse` |
| `backend/modules/careplan/http/companion_plan_error_test.go` | `internal_test` | `api/common` | `CodeStudentsCompanionLockBusy` |
| `backend/modules/careplan/http/parent_request_conflict_handlers_test.go` | `internal_test` | `api/common` | `CodeStudentsChangeRequestStale`, `CodeStudentsConflictKindUnsupported`, `CodeStudentsReasonRequired`, `CodeStudentsStaffValueInvalid`, `CodeStudentsStaffValueUnsupported` |
| `backend/modules/careplan/http/arrival_schedule_handlers.go` | `production` | `api/common` | `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `RenderError`, `Respond` |
| `backend/modules/careplan/http/arrival_settings_handlers.go` | `production` | `api/common` | `ErrorInternalServer`, `Respond` |
| `backend/modules/careplan/http/authorization.go` | `production` | `api/common` | `StudentAccessSource` |
| `backend/modules/careplan/http/care_exit_handlers.go` | `production` | `api/common` | `CodeStudentsCareExitAlreadyEffective`, `CodeStudentsCareExitBlocked`, `CodeStudentsCareExitDayInPast`, `CodeStudentsCareExitInvalidReason`, `CodeStudentsCareExitNoStudents`, `CodeStudentsCareExitNotPlanned`, `CodeStudentsCareExitNoteNotAllowed`, `CodeStudentsCareExitNoteRequired`, `CodeStudentsCareExitNoteTooLong`, `CodeStudentsCareExitPreviewChanged`, `CodeStudentsCareExitTooManyStudents`, `CodeStudentsCareResumeMissing`, `CodeStudentsCareResumeNotChecked`, `CodeStudentsCareResumeNotEnded`, `CodeStudentsCareResumeStartInPast`, `CodeStudentsCareWithdrawalAfterGap`, `CodeStudentsCareWithdrawalAlreadyResolved`, `CodeStudentsCareWithdrawalDateInvalid`, `CodeStudentsCareWithdrawalNotFound`, `ErrorBusinessRejection`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorRule`, `IsBusinessRejection`, `ParsePositiveInt64IDWithError`, `Respond`, `RulesRenderer` |
| `backend/modules/careplan/http/care_request_detail_handlers.go` | `production` | `api/common` | `CodeCareRequestNotFound`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorNotFoundWithCode`, `ErrorRule`, `ParsePositiveInt64IDWithError`, `RulesRenderer` |
| `backend/modules/careplan/http/care_request_review_handlers.go` | `production` | `api/common` | `CodeStudentsCareDayManagedByBooking`, `CodeStudentsChangeRequestNotPending`, `CodeStudentsGuardianAccessRevoked`, `CodeStudentsPickupChangeCompleted`, `CodeStudentsPickupChangeConflict`, `CodeStudentsPickupChangeExpired`, `CodeStudentsPickupChangeImpactChanged`, `ErrorConflictWithCode`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFound`, `ErrorNotFoundWithCode`, `ErrorRule`, `ParsePositiveInt64IDWithError`, `RulesRenderer` |
| `backend/modules/careplan/http/class_arrival_exception_handlers.go` | `production` | `api/common` | `CodeClassdayArrivalExceptionClassNotFound`, `CodeClassdayArrivalExceptionEditorRequired`, `CodeClassdayArrivalExceptionPastDate`, `CodeClassdayArrivalExceptionWeekend`, `ErrorForbidden`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFound`, `ErrorNotFoundWithCode`, `ErrorRule`, `Respond`, `RulesRenderer` |
| `backend/modules/careplan/http/companion_handlers.go` | `production` | `api/common` | `CodeStudentsCompanionLockBusy`, `CodeStudentsCompanionWouldLoseDeparture`, `CodeStudentsCompanionsChanged`, `ErrResponse`, `ErrorConflictWithCode`, `ErrorForbidden`, `ErrorInternalServerWrap`, `ErrorInvalidRequestWithCode`, `Respond`, `StudentAccessContext` |
| `backend/modules/careplan/http/departure_exception_handlers.go` | `production` | `api/common` | `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `Respond` |
| `backend/modules/careplan/http/documents_handlers.go` | `production` | `api/common` | `CloseFile`, `DocumentFileExtension`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorUnauthorized`, `ErrorUpload`, `IsNotFound`, `MsgInvalidStudentID`, `ParseDocumentWithLimits`, `ParseInt64IDWithError`, `RenderError`, `Respond`, `UploadedFile` |
| `backend/modules/careplan/http/effective_time_handler_core.go` | `production` | `api/common` | `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `Respond` |
| `backend/modules/careplan/http/errors.go` | `production` | `api/common` | `CodeStudentsCareExceptionRaced`, `CodeStudentsPartialAbsenceRequiresDedicatedAction`, `CodeStudentsPickupResetRequiresOffering`, `CodeStudentsStaffProfileRequired`, `ErrorConflictWithCode`, `ErrorForbidden`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorNotFound`, `ErrorRule`, `RenderError`, `RulesRenderer` |
| `backend/modules/careplan/http/excused_request_review_handlers.go` | `production` | `api/common` | `CodeStudentsChangeRequestNotPending`, `CodeStudentsExcusedRequestStatusConflict`, `CodeStudentsGuardianAccessRevoked`, `ErrorConflictWithCode`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorRule`, `ParsePositiveInt64IDWithError`, `RulesRenderer` |
| `backend/modules/careplan/http/list_helpers.go` | `production` | `api/common` | `DetermineStudentAccess`, `StudentAccessContext` |
| `backend/modules/careplan/http/master_data_review_handlers.go` | `production` | `api/common` | `CodeStudentsChangeRequestNotPending`, `CodeStudentsChangeRequestStale`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorRule`, `ParsePositiveInt64IDWithError`, `RulesRenderer` |
| `backend/modules/careplan/http/offering_request_review_handlers.go` | `production` | `api/common` | `CodeEnrollmentCompleteWithdrawalConfirmationRequired`, `CodeStudentsCareOfferingsDisabled`, `CodeStudentsChangeRequestNotPending`, `CodeStudentsOfferingChangeCapacityFull`, `CodeStudentsOfferingChangeDateOutOfRange`, `CodeStudentsOfferingChangesNoEnrollment`, `ErrorForbidden`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFound`, `ErrorRule`, `ParsePositiveInt64IDWithError`, `Respond`, `RulesRenderer` |
| `backend/modules/careplan/http/parent_request_bulk_handlers.go` | `production` | `api/common` | `CodeStudentsBulkApprovalIneligible`, `CodeStudentsChangeRequestStale`, `ErrorConflictWithCode`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorRule`, `Respond`, `RulesRenderer` |
| `backend/modules/careplan/http/parent_request_conflict_handlers.go` | `production` | `api/common` | `CodeStudentsChangeRequestNotPending`, `CodeStudentsConflictKindUnsupported`, `CodeStudentsStaffValueInvalid`, `CodeStudentsStaffValueUnsupported`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestMessageWithCode`, `ErrorNotFound`, `ErrorRule`, `Respond`, `RulesRenderer` |
| `backend/modules/careplan/http/parent_request_errors.go` | `production` | `api/common` | `CodeStudentsAbsenceReadRequired`, `CodeStudentsChangeRequestStale`, `CodeStudentsCorrectionUnsupported`, `CodeStudentsReasonRequired`, `CodeStudentsRequestNotDecided`, `CodeStudentsRequestNotPast`, `CodeStudentsRequestPast`, `ErrorConflictMessageWithCode`, `ErrorConflictWithCode`, `ErrorForbiddenMessageWithCode`, `ErrorInternalServer`, `ErrorInvalidRequestMessageWithCode`, `ErrorRule`, `RulesRenderer` |
| `backend/modules/careplan/http/parent_request_lifecycle_handlers.go` | `production` | `api/common` | `CodeStudentsChangeRequestNotPending`, `CodeStudentsCorrectionUnsupported`, `CodeStudentsRequestNotDecided`, `ErrorConflictMessageWithCode`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorRule`, `ParsePositiveInt64IDWithError`, `Respond`, `RulesRenderer` |
| `backend/modules/careplan/http/partial_absence_handlers.go` | `production` | `api/common` | `ErrorConflict`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorRule`, `IsNotFound`, `Respond`, `RulesRenderer` |
| `backend/modules/careplan/http/people_directory_reads.go` | `production` | `api/common` | `StudentAccessContext` |
| `backend/modules/careplan/http/pickup_adjustment_handlers.go` | `production` | `api/common` | `CodeEnrollmentCompleteWithdrawalConfirmationRequired`, `CodePickupFutureManualReset`, `CodePickupInvalid`, `CodePickupOfferingCapacityFull`, `CodePickupOfferingsDisabled`, `CodePickupPreviewStale`, `CodePickupResolutionRequired`, `ErrorConflictWithCode`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequestWithCode`, `ErrorNotFound`, `Respond` |
| `backend/modules/careplan/http/pickup_schedule_handlers.go` | `production` | `api/common` | `CodePickupBulkExceptionConfirmationRequired`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFound`, `ParseIDParam`, `Respond` |
| `backend/modules/careplan/http/portal_scope_test_helpers.go` | `production` | `api/common` | `PrincipalFactsReader`, `PrincipalFromFacts`, `ProtectedTenantRoutes`, `ReadOnlyPreviewGuard` |
| `backend/modules/careplan/http/response_helpers.go` | `production` | `api/common` | `ErrorInternalServerWrap` |
| `backend/modules/careplan/http/student_create_handlers.go` | `production` | `api/common` | `CodeStudentsGuardianEmailTaken`, `ErrorBusinessRejection`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `IsBusinessRejection` |
| `backend/modules/careplan/http/student_handlers.go` | `production` | `api/common` | `ErrorInvalidRequest`, `ErrorNotFound`, `MsgInvalidStudentID`, `ParseID` |
| `backend/modules/careplan/http/student_resource.go` | `production` | `api/common` | `Middleware`, `RequiresAnyPermission`, `RequiresPermission` |
| `backend/modules/careplan/http/student_update_handlers.go` | `production` | `api/common` | `CodeStudentsCompanionPlanConflict`, `CodeStudentsPartialAbsenceConflict`, `CodeStudentsSickExcusedConflict`, `ErrResponse`, `ErrorConflictWithCode`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound` |
| `backend/modules/classday/http/api.go` | `production` | `api/common` | `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorUnauthorized`, `Middleware`, `RenderError`, `RequiresPermission`, `Respond` |
| `backend/modules/classday/http/arrival_exceptions.go` | `production` | `api/common` | `CodeClassdayArrivalExceptionClassNotFound`, `CodeClassdayArrivalExceptionPastDate`, `CodeClassdayArrivalExceptionWeekend`, `CodeClassdaySchoolWriteDisabled`, `ErrorForbidden`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFound`, `ErrorNotFoundWithCode`, `ErrorRule`, `HasPermission`, `RenderError`, `Respond`, `RulesRenderer` |
| `backend/modules/classday/http/slot_lists.go` | `production` | `api/common` | `CodeTimetableSlotListDrifted`, `CodeTimetableSlotListGroupingInvalid`, `CodeTimetableSlotListPickupPastDate`, `CodeTimetableSlotListReconciliationFutureDate`, `CodeTimetableSlotListsDisabled`, `ErrorConflictWithCode`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidOnField`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorRule`, `Middleware`, `RenderError`, `RenderWithRules`, `RequiresAllPermissions` |
| `backend/modules/communication/http/operatorannouncements/errors_test.go` | `external_test` | `api/common` | `CodeCommunicationAnnouncementNotFound`, `ErrResponse` |
| `backend/modules/communication/http/operatorannouncements/announcements.go` | `production` | `api/common` | `CodeCommunicationAnnouncementNotFound`, `CodeGeneralInput`, `IDFetch`, `OperatorAuditedIDAction`, `OperatorInternal`, `OperatorInvalidField`, `OperatorInvalidRequest`, `OperatorRejection`, `ParseClientIP`, `ParseInt64IDWithError`, `RenderError`, `Respond` |
| `backend/modules/communication/http/parentannouncements/api.go` | `production` | `api/common` | `CodeCommunicationAnnouncementNotPublished`, `CodeCommunicationAnnouncementPublishedImmutable`, `CodeCommunicationAnnouncementReminderSent`, `CodeCommunicationDeclarationHasSubmissions`, `CodeCommunicationParentNewsDisabled`, `CodeCommunicationPollNotOpen`, `CodeCommunicationSystemAnnouncementImmutable`, `ErrorConflictWithCode`, `ErrorForbiddenWithCode`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorNotFound`, `Middleware`, `ParsePositiveInt64IDWithError`, `RenderError`, `RequiresPermission`, `Respond` |
| `backend/modules/communication/http/parentannouncements/declarations.go` | `production` | `api/common` | `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/modules/communication/http/parentmessages/api.go` | `production` | `api/common` | `ErrorConflictMessage`, `ErrorForbidden`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorNotFound`, `Middleware`, `ParsePositiveInt64IDWithError`, `RenderError`, `RequiresPermission`, `Respond` |
| `backend/modules/communication/http/staffmessages/api.go` | `production` | `api/common` | `ErrorInvalidRequest`, `ErrorInvalidRequestMessage`, `Middleware`, `ParsePositiveInt64IDWithError`, `RenderError`, `Respond` |
| `backend/modules/communication/http/staffmessages/errors.go` | `production` | `api/common` | `CodeCommunicationStaffCounterpartUnavailable`, `CodeCommunicationStaffMessagingDisabled`, `ErrorConflictMessage`, `ErrorConflictMessageWithCode`, `ErrorForbiddenMessage`, `ErrorForbiddenMessageWithCode`, `ErrorInternalServerWrap`, `ErrorInvalidRequestMessage`, `ErrorNotFoundMessage`, `ErrorRule`, `RenderError`, `RulesRenderer` |
| `backend/modules/emergencysnapshot/http/api.go` | `production` | `api/common` | `ErrorInternalServer`, `Middleware`, `RenderError`, `RequiresPermission` |
| `backend/modules/emergencysnapshot/http/portal_scope_test_helpers.go` | `production` | `api/common` | `PrincipalFactsReader`, `PrincipalFromFacts`, `ProtectedTenantRoutes`, `ReadOnlyPreviewGuard` |
| `backend/modules/enrollment/http/admin_data_correction_handlers.go` | `production` | `api/common` | `CodeEnrollmentChildBirthDateInvalid`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ParsePositiveInt64IDWithError`, `RenderError`, `Respond` |
| `backend/modules/enrollment/http/admin_decision_handlers.go` | `production` | `api/common` | `CodeEnrollmentApprovalCareOfferingExactlyOne`, `CodeEnrollmentApprovalCareOfferingMissing`, `CodeEnrollmentApprovalDataInvalid`, `CodeEnrollmentCareOfferingFull`, `CodeEnrollmentDecisionAlreadyFinal`, `CodeEnrollmentGuardianAccountMismatch`, `CodeEnrollmentRequestNotFound`, `CodeEnrollmentRestoreDuplicate`, `CodeEnrollmentRestoreNothingWithdrawn`, `CodeEnrollmentRestoreOfferingClosed`, `CodeEnrollmentRestorePhaseInactive`, `CodeEnrollmentWaitlistDisabled`, `ErrorClientClosed`, `ErrorConflictWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFoundWithCode`, `ErrorRequestTimeout`, `ErrorRule`, `ErrorServiceUnavailable`, `ParsePositiveInt64IDWithError`, `RenderError`, `RenderWithRules`, `Respond` |
| `backend/modules/enrollment/http/admin_deletion_handlers.go` | `production` | `api/common` | `CodeEnrollmentChildDeletionNotAllowed`, `CodeEnrollmentDeletionReasonInvalid`, `CodeEnrollmentRequestNotFound`, `CodeEnrollmentStudentExists`, `ErrorConflictWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFoundWithCode`, `ParsePositiveInt64IDWithError`, `RenderError`, `Respond` |
| `backend/modules/enrollment/http/admin_late_invite_handlers.go` | `production` | `api/common` | `CodeEnrollmentApprovalDataInvalid`, `CodeEnrollmentDecisionAlreadyFinal`, `CodeEnrollmentDisabled`, `CodeEnrollmentGuardianAccountMismatch`, `CodeEnrollmentInvalidEmail`, `CodeEnrollmentRequestNotFound`, `ErrorConflictWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFoundWithCode`, `ErrorServiceUnavailable`, `ErrorUnauthorized`, `ParsePositiveInt64IDWithError`, `RenderError`, `Respond` |
| `backend/modules/enrollment/http/admin_offering_handlers.go` | `production` | `api/common` | `CodeEnrollmentCareOfferingClosed`, `CodeEnrollmentCareOfferingExactlyOne`, `CodeEnrollmentCareOfferingMissing`, `CodeEnrollmentCareOfferingsDisabled`, `CodeEnrollmentCompleteWithdrawalConfirmationRequired`, `CodeEnrollmentOfferingAdjustmentInvalid`, `CodeEnrollmentRequestNotFound`, `CodeEnrollmentRequiredCareOfferingMissing`, `ErrorConflictWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFoundWithCode`, `ErrorRule`, `ParsePositiveInt64IDWithError`, `RenderError`, `Respond`, `RulesRenderer` |
| `backend/modules/enrollment/http/api.go` | `production` | `api/common` | `RequiresAllPermissions`, `RequiresPermission`, `UploadStorage` |
| `backend/modules/enrollment/http/care_offering_handlers.go` | `production` | `api/common` | `CodeEnrollmentCareOfferingDaysRequired`, `CodeEnrollmentCareOfferingInUse`, `CodeEnrollmentCareOfferingNotFound`, `CodeEnrollmentCareOfferingPickupTimesRequired`, `CodeEnrollmentCareOfferingSelectionInvalid`, `CodeEnrollmentCareOfferingTemplatePeriodMismatch`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFoundWithCode`, `ErrorRule`, `ParsePositiveInt64IDWithError`, `RenderError`, `Respond`, `RespondNoContent`, `RulesRenderer` |
| `backend/modules/enrollment/http/change_request_handlers.go` | `production` | `api/common` | `CodeEnrollmentChangeRequestChildLocked`, `CodeEnrollmentChangeRequestConflict`, `CodeEnrollmentChangeRequestNotAllowed`, `CodeEnrollmentChangeRequestNotFound`, `CodeEnrollmentChangeRequestStatusChanged`, `CodeEnrollmentStatusLinkInvalid`, `CodeStudentsCompanionLockBusy`, `CodeStudentsCompanionWouldLoseDeparture`, `ErrorConflictWithCode`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFoundWithCode`, `ParsePositiveInt64IDWithError`, `RenderError`, `Respond` |
| `backend/modules/enrollment/http/change_request_review_list_handlers.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/modules/enrollment/http/export_handlers.go` | `production` | `api/common` | `CodeEnrollmentExportTooLarge`, `CodeEnrollmentPhaseNotFound`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFound`, `ErrorNotFoundWithCode`, `ParsePositiveInt64IDWithError`, `RenderError` |
| `backend/modules/enrollment/http/legal_document_handlers.go` | `production` | `api/common` | `CloseFile`, `CodeEnrollmentLegalDocumentInvalid`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithDetails`, `ParsePDFWithLimits`, `RenderError`, `Respond`, `RespondNoContent`, `SavePDF`, `UploadStorage`, `ValidateFilename` |
| `backend/modules/enrollment/http/legal_handlers.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/modules/enrollment/http/me_handlers.go` | `production` | `api/common` | `BuildGuardianProfileResponse`, `ErrorForbidden`, `ErrorInternalServer`, `GuardianProfileChildData`, `GuardianProfileData`, `GuardianProfileDefaults`, `RenderError`, `Respond` |
| `backend/modules/enrollment/http/parent_forms.go` | `production` | `api/common` | `Respond` |
| `backend/modules/enrollment/http/phase_expiry_handlers.go` | `production` | `api/common` | `ErrorInternalServer`, `RenderError`, `Respond` |
| `backend/modules/enrollment/http/phase_handlers.go` | `production` | `api/common` | `CodeEnrollmentPhaseCareOfferingConflict`, `CodeEnrollmentPhaseNameExists`, `CodeEnrollmentPhaseNotFound`, `CodeEnrollmentPhaseServicePeriodInvalid`, `CodeEnrollmentPhaseWindowInvalid`, `ErrorConflictOnField`, `ErrorConflictWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFoundWithCode`, `ParsePositiveInt64IDWithError`, `RenderError`, `Respond`, `RespondNoContent` |
| `backend/modules/enrollment/http/phase_response_handlers.go` | `production` | `api/common` | `CodeEnrollmentPhaseNotFound`, `ErrorInternalServer`, `ErrorNotFoundWithCode`, `ParsePositiveInt64IDWithError`, `RenderError`, `Respond` |
| `backend/modules/enrollment/http/portal_scope_test_helpers.go` | `production` | `api/common` | `PrincipalFactsReader`, `PrincipalFromFacts`, `ReadOnlyPreviewGuard` |
| `backend/modules/enrollment/http/public_form_handlers.go` | `production` | `api/common` | `CodeEnrollmentDisabled`, `CodeEnrollmentFormNotFound`, `CodeEnrollmentLateInviteInvalid`, `CodeEnrollmentWindowClosed`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFoundWithCode`, `ParsePositiveInt64IDWithError`, `RenderError`, `Respond` |
| `backend/modules/enrollment/http/report_handlers.go` | `production` | `api/common` | `CodeEnrollmentPhaseNotFound`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFoundWithCode`, `RenderError`, `Respond` |
| `backend/modules/enrollment/http/request_read_handlers.go` | `production` | `api/common` | `ErrorInternalServer`, `ParsePositiveInt64IDWithError`, `RenderError`, `Respond` |
| `backend/modules/enrollment/http/rollover_handlers.go` | `production` | `api/common` | `CodeEnrollmentPhaseServicePeriodInvalid`, `CodeEnrollmentPhaseWindowInvalid`, `CodeRolloverDeadlineRequired`, `CodeRolloverDuplicateName`, `CodeRolloverInvalidRequest`, `CodeRolloverReviewInvalid`, `CodeRolloverReviewNotFound`, `CodeRolloverSourceAlreadyRolled`, `CodeRolloverSourceNotFound`, `ErrorConflictOnField`, `ErrorConflictWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFoundWithCode`, `HasInputRejection`, `ParsePositiveInt64IDWithError`, `RenderError`, `Respond` |
| `backend/modules/enrollment/http/schema_handlers.go` | `production` | `api/common` | `CodeEnrollmentSchemaHasPhases`, `CodeEnrollmentSchemaHasRequests`, `CodeEnrollmentSchemaNameExists`, `CodeEnrollmentSchemaNameRequired`, `CodeEnrollmentSchemaNotFound`, `ErrorConflictOnField`, `ErrorConflictWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorNotFoundWithCode`, `ErrorUnauthorized`, `ParsePositiveInt64IDWithError`, `RenderError`, `Respond`, `RespondNoContent` |
| `backend/modules/enrollment/http/status_handlers.go` | `production` | `api/common` | `CodeEnrollmentEditNotAllowed`, `CodeEnrollmentInvalidPhone`, `CodeEnrollmentStatusLinkInvalid`, `CodeEnrollmentWithdrawNotAllowed`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFoundWithCode`, `RenderError`, `Respond`, `RespondNoContent` |
| `backend/modules/enrollment/http/student_extra_fields.go` | `production` | `api/common` | `ErrorForbidden`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `Middleware`, `RenderError`, `RequiresPermission`, `Respond` |
| `backend/modules/enrollment/http/submission_handlers.go` | `production` | `api/common` | `CodeEnrollmentChildBirthDateInvalid`, `CodeEnrollmentFormNotFound`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFoundWithCode`, `GetClientIPString`, `RenderError`, `Respond` |
| `backend/modules/enrollment/http/submit_errors.go` | `production` | `api/common` | `CodeEnrollmentCaptchaFailed`, `CodeEnrollmentCaptchaRequired`, `CodeEnrollmentCareOfferingClosed`, `CodeEnrollmentCareOfferingExactlyOne`, `CodeEnrollmentCareOfferingFull`, `CodeEnrollmentCareOfferingMissing`, `CodeEnrollmentCareOfferingUnavailable`, `CodeEnrollmentCareOfferingsDisabled`, `CodeEnrollmentChildAlreadyEnrolled`, `CodeEnrollmentChildAlreadyRequested`, `CodeEnrollmentChildAmbiguous`, `CodeEnrollmentChildNotEnrolled`, `CodeEnrollmentChildNotPermitted`, `CodeEnrollmentClassNotEligible`, `CodeEnrollmentDaySelectionNotAllowed`, `CodeEnrollmentDaySelectionRequired`, `CodeEnrollmentDepartureModeLimit`, `CodeEnrollmentDisabled`, `CodeEnrollmentGradeNotEligible`, `CodeEnrollmentInvalidEmail`, `CodeEnrollmentInvalidPhone`, `CodeEnrollmentLateInviteInvalid`, `CodeEnrollmentPhaseNotEligible`, `CodeEnrollmentPickupTimeNotAllowed`, `CodeEnrollmentRequestDuplicate`, `CodeEnrollmentRequiredCareOfferingMissing`, `CodeEnrollmentSelectedDayNotAvailable`, `CodeEnrollmentSubmissionRateLimited`, `CodeEnrollmentWindowClosed`, `ErrResponse`, `ErrorConflictWithCode`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorRule`, `ErrorTooManyRequestsWithCode`, `RenderError`, `RulesRenderer` |
| `backend/modules/facilities/compose/httpadapter/runtime.go` | `production` | `api/common` | `CodeAttendanceLogDisabled`, `CodeRoomsColorAlreadyInUse`, `CodeRoomsNotFound`, `ErrorConflict`, `ErrorConflictWithCode`, `ErrorForbidden`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFoundWithCode`, `HasAdminWildcard`, `PaginationParams`, `ParseID`, `ParsePagination`, `RenderError`, `RequiresPermission`, `Respond`, `RespondNoContent`, `RespondPaginated` |
| `backend/modules/filestorage/http/files/api_test.go` | `external_test` | `api/common` | `JSONID` |
| `backend/modules/filestorage/http/files/runtime_evidence_test.go` | `external_test` | `api/common` | `JSONID` |
| `backend/modules/filestorage/http/files/api.go` | `production` | `api/common` | `CloseFile`, `CodeFilesFolderNameTaken`, `CodeFilesQuotaExceeded`, `DocumentFileExtension`, `ErrorConflictWithCode`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `JSONID`, `Middleware`, `ParseOfficeFileWithLimits`, `RenderError`, `RequiresPermission` |
| `backend/modules/filestorage/http/files/attachments.go` | `production` | `api/common` | `CodeFilesAnnouncementPublished`, `CodeFilesAttachmentLimitReached`, `ErrorConflictWithCode`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorUnauthorized`, `ErrorUpload`, `Middleware`, `ParseInt64IDWithError`, `RenderError`, `RequiresPermission`, `Respond` |
| `backend/modules/filestorage/http/files/handlers.go` | `production` | `api/common` | `ErrorInvalidRequest`, `ErrorUnauthorized`, `ErrorUpload`, `JSONID`, `ParseInt64IDWithError`, `RenderError`, `Respond` |
| `backend/modules/grouplive/http/handlers.go` | `production` | `api/common` | `ErrorForbidden`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `Fetch`, `Middleware`, `RenderError`, `RequiresPermission`, `Respond` |
| `backend/modules/identityaccess/inbound/account/account_handlers.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorRule`, `MsgInvalidAccountID`, `NewCaregiverCapabilityBlockedResponse`, `ParseInt64IDWithError`, `RenderError`, `Respond`, `RespondNoContent`, `UnwrapRenderer` |
| `backend/modules/identityaccess/inbound/account/api.go` | `production` | `api/common` | `AuthorizationForbidden`, `ErrorInternalServer`, `IDAction`, `MsgInvalidAccountID`, `MsgInvalidParentAccountID`, `MsgInvalidPermissionID`, `MsgInvalidRoleID`, `RenderError`, `RequiresAnyPermission`, `RequiresPermission`, `TenantTxMiddleware`, `TwoIDAction` |
| `backend/modules/identityaccess/inbound/account/demo_access.go` | `production` | `api/common` | `CodeIdentityDemoAccessExpired`, `CodeIdentityDemoAccessInvalid`, `CodeIdentityDemoAccessRateLimited`, `CodeIdentityDemoAccessUnknown`, `CodeIdentityDemoCapacityReached`, `CodeIdentityDemoSchoolPreparing`, `ErrResponse`, `ErrorInternalServer`, `ErrorRule`, `RenderError`, `RenderWithRules` |
| `backend/modules/identityaccess/inbound/account/guardian_invitation_handlers.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/modules/identityaccess/inbound/account/invitation_handlers.go` | `production` | `api/common` | `CodeIdentityAccountAlreadyHasTenantAccess`, `CodeIdentityAccountInactive`, `CodeIdentityEmailAlreadyExists`, `CodeIdentityInvitationAccountLoginRequired`, `CodeIdentityInvitationAccountMismatch`, `CodeIdentityInvitationExpired`, `CodeIdentityInvitationNotFound`, `CodeIdentityPasswordTooWeak`, `ErrorConflict`, `ErrorConflictOnField`, `ErrorForbidden`, `ErrorForbiddenWithCode`, `ErrorGoneWithCode`, `ErrorInternalServer`, `ErrorInvalidOnField`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorNotFoundWithCode`, `ErrorRule`, `ErrorUnauthorizedWithCode`, `FixedRenderer`, `JSONID`, `RenderError`, `RenderWithRules`, `Respond`, `RespondNoContent` |
| `backend/modules/identityaccess/inbound/account/mfa_admin_handlers.go` | `production` | `api/common` | `ErrUnauthorized`, `ErrorInvalidRequest`, `ErrorUnauthorized`, `MsgInvalidAccountID`, `RenderError` |
| `backend/modules/identityaccess/inbound/account/mfa_handlers.go` | `production` | `api/common` | `CodeIdentityInvalidCredentials`, `CodeIdentityMfaBlocked`, `CodeIdentityMfaCodeInvalid`, `CodeIdentitySessionAccountInactive`, `CodeIdentityUseSchoolPortal`, `ErrUnauthorized`, `ErrorConflict`, `ErrorForbidden`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorServiceUnavailable`, `ErrorTooManyRequestsWithCode`, `ErrorUnauthorized`, `ErrorUnauthorizedWithCode`, `MFAEnrollConfirmRequest`, `MFAResendRequest`, `MFAResendResponse`, `MFAVerifyRequest`, `MapTrustedDevices`, `ParseClientIP`, `RenderError`, `RequireDependency`, `TrustedDeviceRow` |
| `backend/modules/identityaccess/inbound/account/parent_account_handlers.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `MsgInvalidParentAccountID`, `ParseIntIDWithError`, `RenderError`, `Respond`, `RespondNoContent` |
| `backend/modules/identityaccess/inbound/account/passkey_handlers.go` | `production` | `api/common` | `CodeIdentityMfaBlocked`, `CodeIdentityMfaCodeInvalid`, `CodeIdentityPasskeyLoginFailed`, `CodeIdentitySessionAccountInactive`, `CodeIdentityUseParentPortal`, `CodeIdentityUseSchoolPortal`, `ErrorForbiddenWithCode`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorServiceUnavailable`, `ErrorTooManyRequestsWithCode`, `ErrorUnauthorized`, `ErrorUnauthorizedWithCode`, `PasskeyRegisterOptionsRequest`, `PasskeyRegisterVerifyRequest`, `PasskeyVerifyRequest`, `RenderError`, `RequireDependency`, `RespondNoContent` |
| `backend/modules/identityaccess/inbound/account/password_handlers.go` | `production` | `api/common` | `CodeIdentityPasswordResetLinkInvalid`, `CodeIdentityPasswordTooWeak`, `ErrorInternalServer`, `ErrorInvalidOnField`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorServiceUnavailable`, `MsgInvalidAccountID`, `ParseIntIDWithError`, `RenderError`, `Respond` |
| `backend/modules/identityaccess/inbound/account/password_reset_errors.go` | `production` | `api/common` | `CodeIdentityPasswordResetRateLimited`, `ErrorTooManyRequestsWithCode`, `RenderError` |
| `backend/modules/identityaccess/inbound/account/portal_scope_test_helpers.go` | `production` | `api/common` | `PrincipalFactsReader`, `PrincipalFromFacts`, `ReadOnlyPreviewGuard` |
| `backend/modules/identityaccess/inbound/account/preview_handlers.go` | `production` | `api/common` | `CodeIdentityPreviewTargetNotPreviewable`, `CodeIdentityPreviewTargetSchoolPortal`, `CodeIdentityPreviewTokenInvalid`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `RenderError`, `Respond` |
| `backend/modules/identityaccess/inbound/account/rbac_handlers.go` | `production` | `api/common` | `CodeIdentityRoleNameTaken`, `ErrorConflict`, `ErrorConflictMessage`, `ErrorConflictOnField`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorRule`, `MsgInvalidAccountID`, `MsgInvalidPermissionID`, `MsgInvalidRoleID`, `ParseIntIDWithError`, `RenderError`, `Respond`, `RespondNoContent`, `UnwrapRenderer` |
| `backend/modules/identityaccess/inbound/account/session_handlers.go` | `production` | `api/common` | `CodeIdentityCurrentPasswordWrong`, `CodeIdentityEmailAlreadyExists`, `CodeIdentityInvalidCredentials`, `CodeIdentityMfaBlocked`, `CodeIdentityPasswordTooWeak`, `CodeIdentitySessionAccountInactive`, `CodeIdentityUseParentPortal`, `CodeIdentityUseSchoolPortal`, `ErrorConflict`, `ErrorForbidden`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidOnField`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorServiceUnavailable`, `ErrorTooManyRequestsWithCode`, `ErrorUnauthorized`, `ErrorUnauthorizedOnField`, `ErrorUnauthorizedWithCode`, `GetClientIPString`, `RenderError`, `Respond`, `RespondNoContent` |
| `backend/modules/identityaccess/inbound/account/session_validation.go` | `production` | `api/common` | `ErrorInvalidRequest`, `ErrorServiceUnavailable`, `ErrorUnauthorized`, `RenderError` |
| `backend/modules/identityaccess/inbound/account/tenant_handlers.go` | `production` | `api/common` | `CodeIdentityDemoSession`, `CodeIdentityTenantAccessDenied`, `CodeIdentityUseSchoolPortal`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorServiceUnavailable`, `ErrorUnauthorized`, `ErrorUnauthorizedWithCode`, `RenderError`, `Respond` |
| `backend/modules/identityaccess/inbound/account/types.go` | `production` | `api/common` | `JSONID` |
| `backend/modules/identityaccess/inbound/me/api.go` | `production` | `api/common` | `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorRule`, `ErrorUnauthorized`, `NewResponse`, `ParseIDParam`, `RenderError`, `RulesRenderer`, `TenantTxMiddleware`, `UploadStorage` |
| `backend/modules/identityaccess/inbound/me/portal_scope_test_helpers.go` | `production` | `api/common` | `PrincipalFactsReader`, `PrincipalFromFacts`, `ReadOnlyPreviewGuard` |
| `backend/modules/identityaccess/inbound/me/profile.go` | `production` | `api/common` | `CloseFile`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorUnauthorized`, `ParseImage`, `RenderError`, `SaveImage`, `ServeImage` |
| `backend/modules/identityaccess/inbound/operator/account_access.go` | `production` | `api/common` | `JSONID`, `ParseClientIP`, `ParseInt64IDWithError`, `RenderError`, `Respond` |
| `backend/modules/identityaccess/inbound/operator/auth.go` | `production` | `api/common` | `ParseClientIP`, `RenderError`, `Respond` |
| `backend/modules/identityaccess/inbound/operator/errors.go` | `production` | `api/common` | `CodeIdentityAccountAlreadyHasTenantAccess`, `CodeIdentityAccountInactive`, `CodeIdentityAccountNotFound`, `CodeIdentityCurrentPasswordWrong`, `CodeIdentityLehrkraftRoleImmutable`, `CodeIdentityMfaBlocked`, `CodeIdentityMfaCodeInvalid`, `CodeIdentityTenantAccessNotFound`, `CodeProvisioningSchoolAlreadyDeleted`, `CodeProvisioningSchoolNotFound`, `OperatorInternal`, `OperatorInvalidCredentials`, `OperatorInvalidField`, `OperatorInvalidRequest`, `OperatorRejection`, `OperatorServiceUnavailable` |
| `backend/modules/identityaccess/inbound/operator/invitations.go` | `production` | `api/common` | `CodeGeneralInput`, `CodeIdentityEmailAlreadyExists`, `CodeIdentityInvitationNotFound`, `CodeIdentityInvitationRateLimited`, `CodeIdentityOperatorInvitationInvalid`, `CodeIdentityPasswordTooWeak`, `OperatorInternal`, `OperatorInvalidRequest`, `OperatorRejection`, `RenderError`, `Respond` |
| `backend/modules/identityaccess/inbound/operator/mfa.go` | `production` | `api/common` | `CodeIdentityInvalidCredentials`, `CodeIdentityMfaAlreadyEnrolled`, `CodeIdentityMfaBlocked`, `CodeIdentityMfaCodeInvalid`, `CodeIdentityMfaNotEnrolled`, `ErrResponse`, `MFAEnrollConfirmRequest`, `MFAResendRequest`, `MFAResendResponse`, `MFAVerifyRequest`, `MapTrustedDevices`, `OperatorForbidden`, `OperatorInternal`, `OperatorInvalidRequest`, `OperatorRejection`, `OperatorServiceUnavailable`, `OperatorUnauthorized`, `ParseClientIP`, `RenderError`, `RequireDependency`, `Respond`, `RespondNoContent`, `TrustedDeviceRow` |
| `backend/modules/identityaccess/inbound/operator/mfa_admin.go` | `production` | `api/common` | `CodeIdentityAccountNotInSchool`, `MsgInvalidAccountID`, `OperatorForbidden`, `OperatorInternal`, `OperatorInvalidRequest`, `OperatorRejection`, `OperatorUnauthorized`, `ParseInt64IDWithError`, `RenderError`, `Respond`, `RespondNoContent` |
| `backend/modules/identityaccess/inbound/operator/middleware.go` | `production` | `api/common` | `OperatorForbidden`, `OperatorServiceUnavailable`, `OperatorUnauthorized`, `RenderError` |
| `backend/modules/identityaccess/inbound/operator/operator_profile.go` | `production` | `api/common` | `CodeIdentityAccountNotFound`, `CodeIdentityCurrentPasswordWrong`, `CodeIdentityEmailAlreadyExists`, `CodeIdentityEmailChangeLinkInvalid`, `CodeIdentityEmailChangeRateLimited`, `CodeIdentityEmailChangeSameEmail`, `OperatorInternal`, `OperatorInvalidField`, `OperatorInvalidRequest`, `OperatorRejection`, `RenderError`, `Respond` |
| `backend/modules/identityaccess/inbound/operator/operator_status.go` | `production` | `api/common` | `CodeIdentityAccountNotFound`, `CodeIdentityOperatorSelfDeactivation`, `OperatorInternal`, `OperatorInvalidRequest`, `OperatorRejection`, `RenderError`, `Respond` |
| `backend/modules/identityaccess/inbound/operator/passkeys.go` | `production` | `api/common` | `CodeIdentityMfaBlocked`, `CodeIdentityMfaCodeInvalid`, `CodeIdentityPasskeyLoginFailed`, `CodeIdentityPasskeyNotFound`, `ErrorInvalidRequest`, `OperatorRejection`, `ParseClientIP`, `PasskeyRegisterOptionsRequest`, `PasskeyRegisterVerifyRequest`, `PasskeyVerifyRequest`, `RenderError`, `RequireDependency`, `RespondNoContent` |
| `backend/modules/identityaccess/inbound/operator/profile.go` | `production` | `api/common` | `RenderError`, `Respond` |
| `backend/modules/identityaccess/inbound/operator/resource.go` | `production` | `api/common` | `OperatorConflict`, `OperatorForbidden`, `OperatorInvalidCredentials`, `OperatorInvalidRequest`, `OperatorNotFound`, `OperatorUnauthorized` |
| `backend/modules/organizationtenancy/inbound/operator/billing_test.go` | `internal_test` | `api/common` | `CodeBillingInvalidKeyDay`, `CodeGeneralInput` |
| `backend/modules/organizationtenancy/inbound/operator/provisioning_error_codes_internal_test.go` | `internal_test` | `api/common` | `CaregiverCapabilityBlockedResponse`, `CodeIdentityAccountNotFound`, `CodeIdentityEmailAlreadyExists`, `CodeIdentityPasswordMismatch`, `CodeIdentityPasswordTooWeak`, `CodeIdentityUsernameTaken`, `CodeProvisioningCaregiverCapabilityBlocked`, `CodeProvisioningDeviceActiveSession`, `CodeProvisioningDeviceApiKeyTaken`, `CodeProvisioningDeviceIdTaken`, `CodeProvisioningDeviceInUse`, `CodeProvisioningDeviceNotFound`, `CodeProvisioningDeviceOnline`, `CodeProvisioningDeviceOtherOrganization`, `CodeProvisioningDeviceProtected`, `CodeProvisioningDeviceSameSchool`, `CodeProvisioningDeviceTransferProtected`, `CodeProvisioningOrganizationAlreadyDeleted`, `CodeProvisioningOrganizationDeleted`, `CodeProvisioningOrganizationHasSchools`, `CodeProvisioningOrganizationNotDeleted`, `CodeProvisioningOrganizationNotFound`, `CodeProvisioningOrganizationSlugTaken`, `CodeProvisioningPersonHasSupervisions`, `CodeProvisioningPersonNotFound`, `CodeProvisioningSchoolAlreadyDeleted`, `CodeProvisioningSchoolInactive`, `CodeProvisioningSchoolNotDeleted`, `CodeProvisioningSchoolNotFound`, `CodeProvisioningSchoolSlugTaken`, `CodeProvisioningSchoolSubdomainTaken`, `ErrResponse` |
| `backend/modules/organizationtenancy/inbound/operator/provisioning_internal_test.go` | `internal_test` | `api/common` | `CaregiverCapabilityBlockedResponse`, `ErrResponse`, `JSONID`, `NewCaregiverCapabilityBlockedResponse` |
| `backend/modules/organizationtenancy/inbound/operator/provisioning_test.go` | `internal_test` | `api/common` | `ErrResponse` |
| `backend/modules/organizationtenancy/inbound/operator/billing.go` | `production` | `api/common` | `CodeBillingInvalidKeyDay`, `OperatorForbidden`, `OperatorInternal`, `OperatorInvalidField`, `OperatorInvalidRequest`, `RenderError`, `Respond` |
| `backend/modules/organizationtenancy/inbound/operator/provisioning.go` | `production` | `api/common` | `IDFetch`, `JSONID`, `OperatorInternal`, `OperatorInvalidRequest`, `ParseClientIP`, `ParseInt64IDWithError`, `RenderError`, `Respond`, `RespondNoContent`, `ShouldExposeSeedInvitationToken` |
| `backend/modules/organizationtenancy/inbound/operator/provisioning_error_renderer.go` | `production` | `api/common` | `CodeIdentityAccountNotFound`, `CodeIdentityEmailAlreadyExists`, `CodeIdentityPasswordMismatch`, `CodeIdentityPasswordTooWeak`, `CodeIdentityUsernameTaken`, `CodeProvisioningCaregiverCapabilityBlocked`, `CodeProvisioningDeviceActiveSession`, `CodeProvisioningDeviceApiKeyTaken`, `CodeProvisioningDeviceIdTaken`, `CodeProvisioningDeviceInUse`, `CodeProvisioningDeviceNotFound`, `CodeProvisioningDeviceOnline`, `CodeProvisioningDeviceOtherOrganization`, `CodeProvisioningDeviceProtected`, `CodeProvisioningDeviceSameSchool`, `CodeProvisioningDeviceTransferProtected`, `CodeProvisioningOrganizationAlreadyDeleted`, `CodeProvisioningOrganizationDeleted`, `CodeProvisioningOrganizationHasSchools`, `CodeProvisioningOrganizationNotDeleted`, `CodeProvisioningOrganizationNotFound`, `CodeProvisioningOrganizationSlugTaken`, `CodeProvisioningPersonHasSupervisions`, `CodeProvisioningPersonNotFound`, `CodeProvisioningSchoolAlreadyDeleted`, `CodeProvisioningSchoolInactive`, `CodeProvisioningSchoolNotDeleted`, `CodeProvisioningSchoolNotFound`, `CodeProvisioningSchoolSlugTaken`, `CodeProvisioningSchoolSubdomainTaken`, `NewCaregiverCapabilityBlockedResponse`, `OperatorConflict`, `OperatorInternal`, `OperatorInvalidField`, `OperatorInvalidInput`, `OperatorInvalidRequest`, `OperatorRejection`, `OperatorRejectionOnField`, `OperatorRejectionWithDetails` |
| `backend/modules/organizationtenancy/inbound/operator/provisioning_summaries.go` | `production` | `api/common` | `IDFetch`, `RenderError`, `Respond` |
| `backend/modules/peopledirectory/inbound/birthdays/api.go` | `production` | `api/common` | `CodeWorkforceStaffProfileMissing`, `DetermineStudentAccess`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFoundWithCode`, `ErrorUnauthorized`, `Middleware`, `RenderError`, `RequiresAnyPermission`, `RequiresPermission`, `Respond`, `StudentAccessSource` |
| `backend/modules/peopledirectory/inbound/birthdays/export_handlers.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `RenderError` |
| `backend/modules/peopledirectory/inbound/students/api_photo_paths_test.go` | `external_test` | `api/common` | `StudentPhotoStoredURLPrefix` |
| `backend/modules/peopledirectory/inbound/students/birthdays_api_test.go` | `external_test` | `api/common` | `CodeWorkforceStaffProfileMissing` |
| `backend/modules/peopledirectory/inbound/students/common_not_found_test.go` | `external_test` | `api/common` | `IsNotFound` |
| `backend/modules/peopledirectory/inbound/students/create_student_guardians_middleware_test.go` | `external_test` | `api/common` | `CodeStudentsGuardianEmailTaken` |
| `backend/modules/peopledirectory/inbound/students/photo_routes_test.go` | `external_test` | `api/common` | `BuildStudentPhotoServeURL`, `StudentPhotoStoredURLPrefix` |
| `backend/modules/peopledirectory/inbound/students/student_fixture_helpers_test.go` | `external_test` | `api/common` | `Middleware` |
| `backend/modules/peopledirectory/inbound/students/photo_unlinker_test.go` | `internal_test` | `api/common` | `StudentPhotoStoredURLPrefix` |
| `backend/modules/peopledirectory/inbound/students/tenant_middleware_integration_test.go` | `internal_test` | `api/common` | `TenantRuntimeMiddleware`, `TenantTxMiddleware` |
| `backend/modules/peopledirectory/inbound/students/authorization.go` | `production` | `api/common` | `StudentAccessSource` |
| `backend/modules/peopledirectory/inbound/students/change_history_handlers.go` | `production` | `api/common` | `ErrorForbidden`, `ErrorInternalServer`, `Respond` |
| `backend/modules/peopledirectory/inbound/students/child_quota_handler.go` | `production` | `api/common` | `ErrorInternalServer`, `Respond` |
| `backend/modules/peopledirectory/inbound/students/errors.go` | `production` | `api/common` | `RenderError` |
| `backend/modules/peopledirectory/inbound/students/family_protection_handlers.go` | `production` | `api/common` | `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorRule`, `ParsePositiveInt64IDWithError`, `Respond`, `RulesRenderer` |
| `backend/modules/peopledirectory/inbound/students/note_handlers.go` | `production` | `api/common` | `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorRule`, `ParsePositiveInt64IDWithError`, `Respond`, `RulesRenderer` |
| `backend/modules/peopledirectory/inbound/students/photo.go` | `production` | `api/common` | `BuildStudentPhotoServeURL`, `CloseFile`, `CodeStudentsPhotoConsentRequired`, `CodeStudentsPhotoConsentWithdrawn`, `CodeStudentsPhotosDisabled`, `ErrorConflictWithCode`, `ErrorForbidden`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFound`, `MsgInvalidStudentID`, `ParseID`, `ParseImageWithLimits`, `Respond`, `SaveImage`, `ServeImage`, `StudentPhotoStoredURLPrefix` |
| `backend/modules/peopledirectory/inbound/students/photo_unlinker.go` | `production` | `api/common` | `StudentPhotoStoredURLPrefix`, `UploadStorage` |
| `backend/modules/peopledirectory/inbound/students/response_helpers.go` | `production` | `api/common` | `ErrorInternalServerWrap` |
| `backend/modules/peopledirectory/inbound/students/rfid_handlers.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorUnauthorized`, `Respond` |
| `backend/modules/peopledirectory/inbound/students/student_handlers.go` | `production` | `api/common` | `ErrorInvalidRequest`, `ErrorNotFound`, `MsgInvalidStudentID`, `ParseID` |
| `backend/modules/peopledirectory/inbound/students/student_list_handlers.go` | `production` | `api/common` | `ErrorInternalServer`, `Respond` |
| `backend/modules/peopledirectory/inbound/students/student_resource.go` | `production` | `api/common` | `Middleware`, `RequiresPermission`, `TenantTxMiddleware`, `UploadStorage` |
| `backend/modules/peopledirectory/inbound/students/student_values.go` | `production` | `api/common` | `ErrorUnauthorized`, `Middleware` |
| `backend/modules/presenceprojection/http/student_fixture_helpers_test.go` | `external_test` | `api/common` | `Middleware` |
| `backend/modules/presenceprojection/http/day_log_codes_internal_test.go` | `internal_test` | `api/common` | `CodeStudentsDayLogNoGroups` |
| `backend/modules/presenceprojection/http/export_handlers_test.go` | `internal_test` | `api/common` | `CodeStudentsExportSelectionTooLarge`, `ErrResponse` |
| `backend/modules/presenceprojection/http/attendance_history_export.go` | `production` | `api/common` | `CodeAttendanceLogDisabled`, `ErrorForbidden`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest` |
| `backend/modules/presenceprojection/http/attendance_history_handlers.go` | `production` | `api/common` | `CodeAttendanceLogDisabled`, `ErrorForbidden`, `ErrorForbiddenWithCode`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `Respond` |
| `backend/modules/presenceprojection/http/authorization.go` | `production` | `api/common` | `StudentAccessSource` |
| `backend/modules/presenceprojection/http/companion_handlers.go` | `production` | `api/common` | `StudentAccessContext` |
| `backend/modules/presenceprojection/http/companion_list_enrichment.go` | `production` | `api/common` | `StudentAccessContext` |
| `backend/modules/presenceprojection/http/day_log_export.go` | `production` | `api/common` | `CodeStudentsDayLogDisabled`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest` |
| `backend/modules/presenceprojection/http/day_log_handlers.go` | `production` | `api/common` | `CodeStudentsDayLogDisabled`, `CodeStudentsDayLogNoGroups`, `ErrorForbidden`, `ErrorForbiddenWithCode`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `Respond` |
| `backend/modules/presenceprojection/http/errors.go` | `production` | `api/common` | `RenderError` |
| `backend/modules/presenceprojection/http/export_handlers.go` | `production` | `api/common` | `CodeStudentsExportSelectionTooLarge`, `ErrResponse`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `StudentAccessContext` |
| `backend/modules/presenceprojection/http/health_list_export.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInternalServerWrap` |
| `backend/modules/presenceprojection/http/list_helpers.go` | `production` | `api/common` | `DetermineStudentAccess`, `ErrorForbidden`, `ErrorInvalidRequest`, `ParsePagination`, `StudentAccessContext` |
| `backend/modules/presenceprojection/http/people_directory_reads.go` | `production` | `api/common` | `StudentAccessContext` |
| `backend/modules/presenceprojection/http/response_helpers.go` | `production` | `api/common` | `BuildStudentPhotoServeURL`, `ErrorInternalServerWrap`, `StudentAccessContext` |
| `backend/modules/presenceprojection/http/status_day_handlers.go` | `production` | `api/common` | `ErrorForbidden`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `Respond` |
| `backend/modules/presenceprojection/http/status_day_overview_handlers.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ParsePagination`, `Respond` |
| `backend/modules/presenceprojection/http/student_create_handlers.go` | `production` | `api/common` | `ErrorInternalServer`, `Respond` |
| `backend/modules/presenceprojection/http/student_handlers.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `MsgInvalidStudentID`, `ParseID`, `Respond` |
| `backend/modules/presenceprojection/http/student_list_handlers.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `PaginationParams`, `RespondPaginated`, `StudentAccessContext` |
| `backend/modules/presenceprojection/http/student_resource.go` | `production` | `api/common` | `Middleware`, `RequiresPermission` |
| `backend/modules/presenceprojection/http/student_update_handlers.go` | `production` | `api/common` | `ErrorInternalServer`, `Respond` |
| `backend/modules/presenceprojection/http/visit_handlers.go` | `production` | `api/common` | `ErrorForbidden`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `MsgInvalidStudentID`, `ParseID`, `Respond` |
| `backend/modules/requestreview/http/student_fixture_helpers_test.go` | `external_test` | `api/common` | `Middleware` |
| `backend/modules/requestreview/http/change_request_access_handler.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `MsgInvalidStudentID`, `Respond` |
| `backend/modules/requestreview/http/change_request_list_handlers.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `Respond` |
| `backend/modules/requestreview/http/change_request_pending_count_handler.go` | `production` | `api/common` | `ErrorInternalServer`, `Respond` |
| `backend/modules/requestreview/http/decision_responses.go` | `production` | `api/common` | `Respond` |
| `backend/modules/requestreview/http/errors.go` | `production` | `api/common` | `RenderError` |
| `backend/modules/requestreview/http/student_resource.go` | `production` | `api/common` | `Middleware`, `RequiresAnyPermission` |
| `backend/modules/schoolcalendar/http/api.go` | `production` | `api/common` | `ErrorConflict`, `ErrorForbidden`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorNotFound`, `ParsePositiveInt64IDWithError`, `RenderError`, `RequiresPermission`, `Respond`, `TenantTxMiddleware` |
| `backend/modules/schoolcalendar/http/caldav.go` | `production` | `api/common` | `RenderStatus` |
| `backend/modules/schoolcalendar/http/public_feed.go` | `production` | `api/common` | `RenderStatus` |
| `backend/modules/schoolportal/api.go` | `production` | `api/common` | `Middleware` |
| `backend/modules/schoolportal/auth_handlers.go` | `production` | `api/common` | `CodeIdentityAccountNotFound`, `CodeIdentityInvalidCredentials`, `CodeIdentityMfaBlocked`, `CodeIdentityMfaCodeInvalid`, `CodeIdentitySessionAccountInactive`, `CodeIdentityTenantAccessDenied`, `CodeSchoolNoSchoolPortalRole`, `ErrUnauthorized`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorServiceUnavailable`, `ErrorTooManyRequestsWithCode`, `ErrorUnauthorized`, `ErrorUnauthorizedWithCode`, `GetClientIPString`, `MFAResendRequest`, `MFAResendResponse`, `MFAVerifyRequest`, `ParseClientIP`, `RenderError`, `RequireDependency` |
| `backend/modules/schoolportal/password_handlers.go` | `production` | `api/common` | `ErrorGone`, `ErrorInternalServer`, `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/modules/schoolportal/password_reset_runtime.go` | `production` | `api/common` | `ErrorTooManyRequests`, `RenderError` |
| `backend/modules/schoolportal/router_test_helpers.go` | `production` | `api/common` | `Middleware` |
| `backend/modules/schoolstructure/http/api.go` | `production` | `api/common` | `ErrorConflictMessage`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `Middleware`, `MsgGroupNotFound`, `MsgInvalidGroupID`, `PaginationParams`, `ParseID`, `ParseInt64IDWithError`, `ParsePagination`, `RenderError`, `RequiresPermission`, `Respond`, `RespondPaginated` |
| `backend/modules/schoolstructure/http/errors.go` | `production` | `api/common` | `ErrorConflict`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorRule`, `UnwrapRenderer` |
| `backend/modules/schoolstructure/http/students.go` | `production` | `api/common` | `ErrorForbidden`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorNotFound`, `MsgGroupNotFound`, `MsgInvalidGroupID`, `ParseInt64IDWithError`, `RenderError`, `Respond` |
| `backend/modules/settings/inbound/operator/settings.go` | `production` | `api/common` | `CodeSettingsAdminOnly`, `CodeSettingsAttendanceRetentionBelowVisible`, `CodeSettingsAttendanceVisibleExceedsRetention`, `CodeSettingsBookingAuthorityBlocked`, `CodeSettingsInvalidValue`, `CodeSettingsManagedByUpload`, `CodeSettingsNotFound`, `CodeSettingsPresenceModeSwitchBlocked`, `ErrorInternalServerWrap`, `OperatorForbidden`, `OperatorInvalidRequest`, `OperatorInvalidRequestWithCode`, `OperatorRejection`, `OperatorRejectionWithDetails`, `ParseInt64IDWithError`, `RenderError`, `Respond`, `RespondNoContent` |
| `backend/modules/statistics/http/api.go` | `production` | `api/common` | `CodePresenceStatisticsRangeInvalid`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorRule`, `Middleware`, `RenderError`, `Respond`, `RulesRenderer` |
| `backend/modules/studentpresence/http/student_fixture_helpers_test.go` | `external_test` | `api/common` | `Middleware` |
| `backend/modules/studentpresence/http/students_test.go` | `external_test` | `api/common` | `CodeStudentsSickExcusedConflict` |
| `backend/modules/studentpresence/http/school_checkin_handlers_test.go` | `internal_test` | `api/common` | `CodeGeneralBusinessRejection`, `CodeStudentsCheckinCareEnded`, `ErrResponse` |
| `backend/modules/studentpresence/http/authorization.go` | `production` | `api/common` | `StudentAccessSource` |
| `backend/modules/studentpresence/http/errors.go` | `production` | `api/common` | `RenderError` |
| `backend/modules/studentpresence/http/privacy_handlers.go` | `production` | `api/common` | `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `Respond` |
| `backend/modules/studentpresence/http/school_checkin_handlers.go` | `production` | `api/common` | `CodeGeneralBusinessRejection`, `CodeStudentsCheckinCareEnded`, `CodeStudentsCheckoutNoteTooLong`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFoundWithCode`, `RenderError`, `Respond` |
| `backend/modules/studentpresence/http/status_day_handlers.go` | `production` | `api/common` | `CodeStudentsPartialAbsenceConflict`, `CodeStudentsStatusDayConflict`, `ErrResponse`, `ErrorConflictWithCode`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorNotFound`, `IsNotFound`, `ParsePositiveInt64IDWithError`, `Respond` |
| `backend/modules/studentpresence/http/student_handlers.go` | `production` | `api/common` | `ErrorInvalidRequest`, `ErrorNotFound`, `MsgInvalidStudentID`, `ParseID` |
| `backend/modules/studentpresence/http/student_resource.go` | `production` | `api/common` | `Middleware`, `RequireWebAttendanceEnabled`, `RequiresAnyPermission`, `RequiresPermission` |
| `backend/modules/studentpresence/inbound/presence/active_test.go` | `external_test` | `api/common` | `CodeAttendanceWebDisabled` |
| `backend/modules/studentpresence/inbound/presence/errors_test.go` | `external_test` | `api/common` | `ErrResponse` |
| `backend/modules/studentpresence/inbound/presence/wire_format_test.go` | `external_test` | `api/common` | `RenderError` |
| `backend/modules/studentpresence/inbound/presence/active_internal_test.go` | `internal_test` | `api/common` | `ErrResponse` |
| `backend/modules/studentpresence/inbound/presence/handlers_unit_test.go` | `internal_test` | `api/common` | `ErrResponse` |
| `backend/modules/studentpresence/inbound/presence/resource_test.go` | `internal_test` | `api/common` | `Middleware` |
| `backend/modules/studentpresence/inbound/presence/visit_display_test.go` | `internal_test` | `api/common` | `StudentAccessContext` |
| `backend/modules/studentpresence/inbound/presence/analytics_handlers.go` | `production` | `api/common` | `ErrorInternalServer`, `RenderError`, `Respond` |
| `backend/modules/studentpresence/inbound/presence/api.go` | `production` | `api/common` | `Middleware`, `RequireActiveGroupAssign`, `RequireActiveGroupCreate`, `RequireActiveGroupDelete`, `RequireActiveGroupRead`, `RequireActiveGroupUpdate`, `RequireVisitUpdate`, `RequireWebAttendanceEnabled` |
| `backend/modules/studentpresence/inbound/presence/checkin.go` | `production` | `api/common` | `CurrentPrincipal`, `RenderError`, `RespondWithError`, `RespondWithJSON` |
| `backend/modules/studentpresence/inbound/presence/checkout.go` | `production` | `api/common` | `CurrentPrincipal`, `RespondWithError`, `RespondWithJSON` |
| `backend/modules/studentpresence/inbound/presence/combined_handlers.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `ParseID`, `RenderError`, `Respond` |
| `backend/modules/studentpresence/inbound/presence/cross_tenant_handlers.go` | `production` | `api/common` | `RenderError`, `Respond`, `RespondWithError` |
| `backend/modules/studentpresence/inbound/presence/errors.go` | `production` | `api/common` | `ErrorBusinessRejection`, `ErrorBusinessRejectionOr`, `ErrorConflict`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorRule`, `RulesRenderer` |
| `backend/modules/studentpresence/inbound/presence/groups_handlers.go` | `production` | `api/common` | `BuildStudentPhotoServeURL`, `CurrentPrincipal`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorUnauthorized`, `ParseID`, `ParseIDParam`, `RenderError`, `Respond`, `StudentAccessContext` |
| `backend/modules/studentpresence/inbound/presence/mappings_handlers.go` | `production` | `api/common` | `ErrorInvalidRequest`, `ParseIDParam`, `RenderError`, `Respond` |
| `backend/modules/studentpresence/inbound/presence/move_authorization.go` | `production` | `api/common` | `CanUseSchoolWideAttendance`, `HasEffectiveAdminScope` |
| `backend/modules/studentpresence/inbound/presence/move_handlers.go` | `production` | `api/common` | `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/modules/studentpresence/inbound/presence/operational_overview.go` | `production` | `api/common` | `HasEffectiveAdminScope`, `IsAssignmentBoundPortal` |
| `backend/modules/studentpresence/inbound/presence/portal_scope_test_helpers.go` | `production` | `api/common` | `PrincipalFactsReader`, `PrincipalFromFacts`, `ProtectedTenantRoutes`, `ReadOnlyPreviewGuard` |
| `backend/modules/studentpresence/inbound/presence/supervisors_handlers.go` | `production` | `api/common` | `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ParseID`, `ParseIDParam`, `RenderError`, `Respond` |
| `backend/modules/studentpresence/inbound/presence/tracking_handlers.go` | `production` | `api/common` | `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/modules/studentpresence/inbound/presence/transit_handlers.go` | `production` | `api/common` | `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/modules/studentpresence/inbound/presence/unclaimed_handlers.go` | `production` | `api/common` | `CurrentPrincipal`, `RenderError`, `Respond`, `RespondWithError` |
| `backend/modules/studentpresence/inbound/presence/visit_access.go` | `production` | `api/common` | `AuthorizationForbidden`, `ErrorInternalServerWrap`, `ErrorUnauthorized`, `ParseID`, `RenderError`, `VisitReadAccess` |
| `backend/modules/studentpresence/inbound/presence/visits_handlers.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `ParseID`, `ParseIDParam`, `RenderError`, `Respond` |
| `backend/modules/supervisiondashboard/http/handlers.go` | `production` | `api/common` | `ErrorForbidden`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `Middleware`, `RenderError`, `RequireActiveGroupRead`, `Respond` |
| `backend/modules/supervisiondashboard/http/schulhof_handlers.go` | `production` | `api/common` | `ErrorForbidden`, `ErrorInternalServerWrap`, `Middleware`, `RenderError`, `RequireActiveGroupRead`, `Respond` |
| `backend/modules/timetable/compose/httpadapter/activity_handlers.go` | `production` | `api/common` | `CodeTimetableActivityNotOwner`, `ErrorForbidden`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `MsgInvalidActivityID`, `ParseID`, `RenderError`, `Respond`, `RespondWithError` |
| `backend/modules/timetable/compose/httpadapter/category_handlers.go` | `production` | `api/common` | `ErrorInvalidRequest`, `ParseIDParam`, `RenderError`, `Respond` |
| `backend/modules/timetable/compose/httpadapter/enrollment_handlers.go` | `production` | `api/common` | `ErrorInvalidRequest`, `MsgInvalidStudentID`, `ParseIDParam`, `RenderError`, `Respond` |
| `backend/modules/timetable/compose/httpadapter/errors.go` | `production` | `api/common` | `CodeTimetableActivitySystemProtected`, `CodeTimetableActivityTemplateProtected`, `CodeTimetableOnlySupervisorReplacementRequired`, `ErrorConflict`, `ErrorConflictWithCode`, `ErrorForbidden`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorRule`, `RulesRenderer`, `UnwrapRenderer` |
| `backend/modules/timetable/compose/httpadapter/helpers.go` | `production` | `api/common` | `CodeTimetableActivityNotOwner`, `ErrorForbiddenWithCode`, `ErrorInvalidRequest`, `MsgInvalidActivityID`, `ParseID`, `RenderError` |
| `backend/modules/timetable/compose/httpadapter/http.go` | `production` | `api/common` | `BindAction`, `IDFetch`, `Middleware`, `RequiresAnyPermission`, `RequiresPermission` |
| `backend/modules/timetable/compose/httpadapter/schedule_handlers.go` | `production` | `api/common` | `ErrorForbidden`, `ErrorInvalidRequest`, `MsgInvalidWeekday`, `ParseIDParam`, `RenderError`, `Respond`, `RespondWithError` |
| `backend/modules/timetable/compose/httpadapter/schedules.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `Middleware`, `PaginationParams`, `ParseID`, `ParsePagination`, `RenderError`, `RequiresPermission`, `Respond`, `RespondPaginated`, `Time` |
| `backend/modules/timetable/compose/httpadapter/schedules_errors.go` | `production` | `api/common` | `ErrorBusinessRejectionOr`, `ErrorConflict`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorRule`, `RulesRenderer` |
| `backend/modules/timetable/compose/httpadapter/schedules_ports.go` | `production` | `api/common` | `DateFormatISO` |
| `backend/modules/timetable/compose/httpadapter/supervisor_handlers.go` | `production` | `api/common` | `ErrorForbidden`, `ErrorInvalidRequest`, `ParseIDParam`, `RenderError`, `Respond`, `RespondWithError` |
| `backend/modules/timetable/compose/httpintegration/errors_test.go` | `external_test` | `api/common` | `ErrResponse` |
| `backend/modules/timetable/compose/httpintegration/schedules_errors_test.go` | `external_test` | `api/common` | `ErrResponse` |
| `backend/modules/timetable/http/api.go` | `production` | `api/common` | `CodeTimetableCalendarPeriodCareOfferingConflict`, `CodeTimetableCalendarPeriodEndBeforeStart`, `CodeTimetableCalendarPeriodInvalid`, `CodeTimetableCalendarPeriodNameTaken`, `CodeTimetableCalendarPeriodNotFound`, `CodeTimetableCalendarPeriodOverlapConflict`, `CodeTimetableCalendarPeriodRosterConflict`, `CodeTimetableCalendarPeriodWeekCycleAnchorMissing`, `ErrorConflictMessageWithCode`, `ErrorConflictOnField`, `ErrorConflictWithCode`, `ErrorConflictWithDetails`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFoundWithCode`, `ParseID`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/attendance_correction.go` | `production` | `api/common` | `CodeTimetableAttendanceEntryNotFound`, `CodeTimetableCorrectionCancelled`, `CodeTimetableCorrectionNotCompleted`, `CodeTimetableCorrectionReasonMissing`, `CodeTimetableCorrectionReasonTooLong`, `ErrorConflictWithCode`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorNotFoundWithCode`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/attendance_web.go` | `production` | `api/common` | `CodeTimetableInstanceNotFound`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFoundWithCode`, `ErrorServiceUnavailable`, `ParseID`, `RenderError`, `RequireWebAttendanceEnabled` |
| `backend/modules/timetable/http/betreuungsplan_export.go` | `production` | `api/common` | `CanExportInternalPlan`, `CodeTimetableExportEndBeforeStart`, `CodeTimetableExportInternalForbidden`, `CodeTimetableExportRangeTooLarge`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidOnField`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithDetails`, `RenderError` |
| `backend/modules/timetable/http/closing_days.go` | `production` | `api/common` | `CodeTimetableClosingDayEndBeforeStart`, `CodeTimetableClosingDayInvalid`, `CodeTimetableClosingDayNotFound`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFoundWithCode`, `ParseID`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/conflict_acks.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorUnauthorized`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/conflicts.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/dates.go` | `production` | `api/common` | `CodeTimetableDateInPast`, `CodeTimetableWindowEndBeforeStart`, `CodeTimetableWindowTooLarge`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorInvalidRequestWithDetails`, `RenderError` |
| `backend/modules/timetable/http/deviation_history.go` | `production` | `api/common` | `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/deviations.go` | `production` | `api/common` | `ErrResponse`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `FieldError`, `ParseID`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/edited_in_window.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/error_codes.go` | `production` | `api/common` | `ErrResponse`, `ErrorConflictWithCode`, `ErrorForbiddenWithCode`, `ErrorInvalidOnField`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorInvalidRequestWithDetails`, `ErrorNotFoundWithCode`, `FieldError` |
| `backend/modules/timetable/http/exception_conflicts.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInternalServerWrap`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/gaps.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInternalServerWrap`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/instance_students.go` | `production` | `api/common` | `CodeTimetableAttendanceEntryNotFound`, `CodeTimetableAttendanceFrozen`, `CodeTimetableAttendanceInvalid`, `CodeTimetableInstanceNotFound`, `ErrResponse`, `ErrorConflictWithCode`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorNotFoundWithCode`, `ErrorValidation`, `FieldError`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/instances.go` | `production` | `api/common` | `CodeTimetableAmbiguousTemplateInstanceDelete`, `CodeTimetableDuplicateInstance`, `CodeTimetableGuardianNoticeInvalid`, `CodeTimetableInstanceMoved`, `CodeTimetableUnderstaffedStillStaffed`, `ErrorConflictWithCode`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorRule`, `HasEffectiveAdminScope`, `ParseID`, `RenderError`, `RenderWithRules`, `Respond`, `RespondNoContent` |
| `backend/modules/timetable/http/instances_bulk_cancel.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/instances_convert.go` | `production` | `api/common` | `CodeTimetableInstanceNotConvertible`, `CodeTimetableInstanceNotFound`, `CodeTimetableInstanceOutsidePeriod`, `CodeTimetableInstanceWeekend`, `CodeTimetableTemplateInvalid`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorRule`, `ParseID`, `RenderError`, `RenderWithRules`, `Respond` |
| `backend/modules/timetable/http/instances_create.go` | `production` | `api/common` | `CodeTimetableDuplicateInstance`, `CodeTimetableIdempotencyKeyReused`, `CodeTimetableInstanceEndBeforeStart`, `CodeTimetableInstanceInvalid`, `CodeTimetableInstanceOutsidePeriod`, `CodeTimetableInstanceWeekend`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorRule`, `RenderError`, `RenderWithRules`, `Respond` |
| `backend/modules/timetable/http/instances_list.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `HasEffectiveAdminScope`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/instances_update.go` | `production` | `api/common` | `CodeTimetableInstanceEndBeforeStart`, `CodeTimetableInstanceInvalid`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ParseID`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/materialize.go` | `production` | `api/common` | `CodeTimetableWindowEndBeforeStart`, `CodeTimetableWindowTooLarge`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/move_staff.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `ParseID`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/offering_sources.go` | `production` | `api/common` | `CodeTimetableOfferingSourceInvalid`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/operation_errors.go` | `production` | `api/common` | `CodeTimetableCompleteTooEarly`, `CodeTimetableCompletionConfirmationStale`, `CodeTimetableGroupAlreadyEnded`, `CodeTimetableInstanceNotFound`, `CodeTimetableInstanceOutsidePeriod`, `CodeTimetableInstanceWeekend`, `CodeTimetableInvalidTransition`, `CodeTimetableNoStaffProfile`, `CodeTimetableOperationNotPlanned`, `CodeTimetableOperationStale`, `CodeTimetableRoomOccupied`, `CodeTimetableStartTooEarly`, `CodeTimetableStartWindowExpired`, `CodeTimetableStudentAlreadyActive`, `CodeTimetableStudentCareEnded`, `CodeTimetableStudentGraduated`, `CodeTimetableStudentNotCheckedIn`, `CodeTimetableStudentNotFound`, `CodeTimetableStudentsNotPresent`, `ErrorBusinessRejection`, `ErrorBusinessRejectionOr`, `ErrorConflictWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFoundWithCode`, `ErrorRule`, `RenderWithRules` |
| `backend/modules/timetable/http/operations.go` | `production` | `api/common` | `CodeTimetableNoStaffProfile`, `CodeTimetableRoomOccupied`, `CodeTimetableSpontaneousActivitiesDisabled`, `CodeTimetableSpontaneousCategoryArchived`, `ErrorConflictWithCode`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `HasEffectiveAdminScope`, `IsAssignmentBoundPortal`, `JSONID`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/participants.go` | `production` | `api/common` | `CodeTimetableInstanceNotFound`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorNotFoundWithCode`, `ParseID`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/pickup_extensions.go` | `production` | `api/common` | `CodeTimetablePickupExtensionBlockGone`, `CodeTimetablePickupExtensionInvalid`, `CodeTimetablePickupExtensionNotFound`, `CodeTimetablePickupExtensionTooManyBlocks`, `ErrorConflictWithCode`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFoundWithCode`, `ParseID`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/planning_tracks.go` | `production` | `api/common` | `CodeTimetablePlanningTrackArchived`, `CodeTimetablePlanningTrackInvalid`, `CodeTimetablePlanningTrackNameTaken`, `CodeTimetablePlanningTrackNotFound`, `ErrorConflictOnField`, `ErrorInternalServer`, `ErrorInternalServerRenderer`, `ErrorInternalServerWrap`, `ErrorInvalidOnField`, `ErrorInvalidRequest`, `ErrorRule`, `ParsePositiveInt64IDWithError`, `RenderError`, `RenderWithRules`, `Respond` |
| `backend/modules/timetable/http/portal_scope_test_helpers.go` | `production` | `api/common` | `PrincipalFactsReader`, `PrincipalFromFacts`, `ProtectedTenantRoutes`, `ReadOnlyPreviewGuard` |
| `backend/modules/timetable/http/replan_week.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/router.go` | `production` | `api/common` | `Middleware`, `RequireWebAttendanceEnabled`, `RequiresAllPermissions`, `RequiresAnyPermission`, `RequiresPermission` |
| `backend/modules/timetable/http/school_supervisions.go` | `production` | `api/common` | `ErrorForbidden`, `ErrorInternalServer`, `ErrorNotFound`, `Middleware`, `RenderError`, `RequireWebAttendanceEnabled`, `RequiresPermission`, `Respond` |
| `backend/modules/timetable/http/shift_coverage.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/staff_notices.go` | `production` | `api/common` | `CodeCommunicationStaffNoticeOutdated`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFound`, `Middleware`, `RenderError`, `RequiresPermission` |
| `backend/modules/timetable/http/staff_pool.go` | `production` | `api/common` | `CodeTimetableInstanceNotFound`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorNotFoundWithCode`, `ParseID`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/student_day.go` | `production` | `api/common` | `CodeTimetableStudentNotFound`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorNotFoundWithCode`, `IsNotFound`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/substitutions_bulk.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/template_errors.go` | `production` | `api/common` | `CodeTimetableOfferingSourceInvalid`, `CodeTimetableTemplateCategoryUnavailable`, `CodeTimetableTemplateEducationGroupInvalid`, `CodeTimetableTemplateGradeAboveMax`, `CodeTimetableTemplatePlanningTrackUnavailable`, `CodeTimetableTemplateSplitInvalid`, `CodeTimetableTemplateWeekend`, `ErrorInvalidOnField`, `ErrorRule`, `RenderWithRules` |
| `backend/modules/timetable/http/templates_create.go` | `production` | `api/common` | `CodeTimetableTemplateInvalid`, `CodeTimetableTemplateWeekend`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/templates_end.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/templates_list.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/templates_people.go` | `production` | `api/common` | `CodeTimetableTemplateEducationGroupInvalid`, `CodeTimetableTemplateEndBeforeStart`, `CodeTimetableTemplateMaxParticipantsInvalid`, `CodeTimetableTemplateStartOutsidePeriod`, `CodeTimetableTemplateWeekPatternInvalid`, `ErrorInternalServerWrap`, `ErrorInvalidOnField`, `ErrorInvalidRequest`, `RenderError` |
| `backend/modules/timetable/http/templates_series_end.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `RenderError` |
| `backend/modules/timetable/http/templates_split.go` | `production` | `api/common` | `CodeTimetableTemplateCareOfferingConflict`, `CodeTimetableTemplateInvalid`, `CodeTimetableTemplateRosterRebaseConflict`, `ErrorConflictWithCode`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/templates_update.go` | `production` | `api/common` | `CodeTimetableTemplateInvalid`, `CodeTimetableTemplateNotFound`, `CodeTimetableTemplateStartInPast`, `CodeTimetableTemplateStartNotEarlier`, `CodeTimetableTemplateStartPredecessorOverlap`, `CodeTimetableTemplateWeekend`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFoundWithCode`, `ParsePositiveInt64IDWithError`, `RenderError`, `Respond` |
| `backend/modules/timetable/http/understaffed.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `ParseID`, `RenderError`, `Respond` |
| `backend/modules/workforce/http/timetracking/api.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorUnauthorized`, `Middleware`, `ParseInt64IDWithError`, `RenderError`, `RequiresAnyPermission`, `RequiresPermission`, `Respond`, `RespondNoContent` |
| `backend/modules/workforce/http/timetracking/errors.go` | `production` | `api/common` | `CodeIotDeviationReasonRequired`, `CodeIotPlannedStartNotReached`, `CodeWorkforceAbsenceAlreadyDecided`, `CodeWorkforceAbsenceNoWorkingDays`, `CodeWorkforceAbsenceNotCancelable`, `CodeWorkforceAbsenceNotFound`, `CodeWorkforceAbsenceNotOwned`, `CodeWorkforceAbsenceOverlap`, `CodeWorkforceAbsenceTypeInactive`, `CodeWorkforceAlreadyCheckedIn`, `CodeWorkforceAlreadyCheckedOut`, `CodeWorkforceBreakAlreadyActive`, `CodeWorkforceManagerControlledAbsence`, `CodeWorkforceNoActiveBreak`, `CodeWorkforceNoActiveSession`, `CodeWorkforceNoSessionForToday`, `CodeWorkforceSessionNotFound`, `CodeWorkforceSessionNotOwned`, `CodeWorkforceSessionNoteRequired`, `CodeWorkforceSessionTimesInvalid`, `CodeWorkforceShiftOverlap`, `CodeWorkforceVacationQuotaExceeded`, `CodeWorkforceVacationRequestInPast`, `CodeWorkforceWorkSessionOverlap`, `ErrorConflictWithDetails`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFoundWithCode`, `ErrorRule`, `RenderWithRules` |
| `backend/modules/workforce/http/timetracking/month_summary.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorUnauthorized`, `RenderError`, `Respond` |
| `backend/modules/workforce/http/timetracking/own_absences.go` | `production` | `api/common` | `ErrorInvalidRequest`, `ErrorUnauthorized`, `ParseInt64IDWithError`, `RenderError`, `Respond`, `RespondNoContent` |
| `backend/modules/workforce/http/timetracking/planning_calendar.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/modules/workforce/http/timetracking/portal_scope_test_helpers.go` | `production` | `api/common` | `PrincipalFactsReader`, `PrincipalFromFacts`, `ProtectedTenantRoutes`, `ReadOnlyPreviewGuard` |
| `backend/modules/workforce/http/timetracking/schedule_targets.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorUnauthorized`, `RenderError`, `Respond` |
| `backend/modules/workforce/http/timetracking/staff_admin.go` | `production` | `api/common` | `Middleware`, `RequiresAnyPermission`, `RequiresPermission`, `UploadStorage` |
| `backend/modules/workforce/http/timetracking/staff_admin_adjustments.go` | `production` | `api/common` | `CodeWorkforceAdjustmentInClosedMonth`, `CodeWorkforceBalanceAdjustmentExceedsBalance`, `CodeWorkforceBalanceAlreadyReset`, `CodeWorkforceDependentBalanceReset`, `CodeWorkforceOpeningBalanceAlreadyExists`, `ErrorConflictWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFound`, `ErrorRule`, `ErrorUnauthorized`, `IsNotFound`, `ParseID`, `RenderError`, `RenderWithRules`, `Respond` |
| `backend/modules/workforce/http/timetracking/staff_admin_audit_log.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorRule`, `RenderError`, `RenderWithRules`, `Respond` |
| `backend/modules/workforce/http/timetracking/staff_admin_documents.go` | `production` | `api/common` | `CloseFile`, `DocumentFileExtension`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorUnauthorized`, `ErrorUpload`, `IsNotFound`, `MsgInvalidStaffID`, `ParseDocumentWithLimits`, `ParseInt64IDWithError`, `RenderError`, `Respond`, `SavePrivateNamedFile`, `ServeFile`, `UploadStorage`, `ValidateFilename` |
| `backend/modules/workforce/http/timetracking/staff_admin_export.go` | `production` | `api/common` | `CodeWorkforcePayrollConfigIncomplete`, `ErrorConflictWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorRule`, `RenderError`, `RenderWithRules`, `Respond` |
| `backend/modules/workforce/http/timetracking/staff_admin_export_transfer.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `RenderError`, `RenderWithRules`, `Respond` |
| `backend/modules/workforce/http/timetracking/staff_admin_month.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ParseID`, `RenderError`, `Respond` |
| `backend/modules/workforce/http/timetracking/staff_admin_month_close.go` | `production` | `api/common` | `CodeWorkforceLaterMonthClosed`, `CodeWorkforceMonthNotClosable`, `CodeWorkforceMonthNotClosed`, `ErrorConflictWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFoundWithCode`, `ErrorRule`, `ErrorUnauthorized`, `ParseID`, `RenderError`, `RenderWithRules`, `Respond` |
| `backend/modules/workforce/http/timetracking/staff_admin_overview.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorRule`, `RenderError`, `RenderWithRules`, `Respond` |
| `backend/modules/workforce/http/timetracking/staff_admin_payroll.go` | `production` | `api/common` | `CodeWorkforcePersonnelNumberInvalid`, `CodeWorkforcePersonnelNumberTaken`, `ErrorConflictWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFound`, `ErrorUnauthorized`, `IsNotFound`, `MsgInvalidStaffID`, `ParseInt64IDWithError`, `RenderError`, `Respond` |
| `backend/modules/workforce/http/timetracking/staff_admin_schedule.go` | `production` | `api/common` | `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `HasPermission`, `ParseID`, `RenderError`, `Respond` |
| `backend/modules/workforce/http/timetracking/staff_admin_schedule_targets.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ParseID`, `RenderError`, `Respond` |
| `backend/modules/workforce/http/timetracking/staff_admin_sessions.go` | `production` | `api/common` | `CodeWorkforceAbsenceAllowanceExceeded`, `CodeWorkforceAbsenceAlreadyDecided`, `CodeWorkforceAbsenceNoWorkingDays`, `CodeWorkforceAbsenceOverlap`, `CodeWorkforceAbsenceRebookingBlocked`, `CodeWorkforceAbsenceTypeInactive`, `CodeWorkforceDecisionNoteRequired`, `CodeWorkforceShiftOverlap`, `CodeWorkforceVacationQuotaBelowUsed`, `CodeWorkforceVacationQuotaExceeded`, `CodeWorkforceVacationQuotaReasonRequired`, `CodeWorkforceWorkSessionOverlap`, `ErrorBusinessRejectionOr`, `ErrorConflictWithCode`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFound`, `ErrorRule`, `ErrorUnauthorized`, `ParseID`, `ParseIDParam`, `RenderError`, `RenderWithRules`, `Respond`, `RespondNoContent` |
| `backend/modules/workforce/http/timetracking/staff_admin_stammdaten.go` | `production` | `api/common` | `CodeWorkforceStammdatenInvalid`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFound`, `ErrorUnauthorized`, `IsNotFound`, `MsgInvalidStaffID`, `ParseInt64IDWithError`, `RenderError`, `Respond` |
| `backend/modules/workforce/http/timetracking/staff_admin_target_overrides.go` | `production` | `api/common` | `ErrorConflictMessage`, `ErrorConflictWithDetails`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestMessage`, `ErrorInvalidRequestWithDetails`, `ErrorNotFound`, `Middleware`, `ParseID`, `RenderError`, `RequiresPermission`, `Respond` |
| `backend/modules/workforce/http/timetracking/staff_admin_vacation.go` | `production` | `api/common` | `CodeWorkforceVacationOpeningAbsencesBeforeCutoff`, `CodeWorkforceVacationOpeningAlreadyExists`, `ErrorConflictWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorUnauthorized`, `ParseID`, `RenderError`, `Respond` |
| `backend/workflows/gradetransition/http/grade_transitions.go` | `production` | `api/common` | `CodeGradeTransitionGraduatesCheckedIn`, `CodeGradeTransitionNotApplied`, `CodeGradeTransitionNotDraft`, `CodeGradeTransitionNotLatestTransition`, `CodeGradeTransitionPreviewStale`, `ErrorConflictWithCode`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `Middleware`, `PaginationParams`, `ParseInt64IDWithError`, `ParsePagination`, `RenderError`, `RequiresPermission`, `Respond`, `RespondPaginated` |
| `backend/workflows/openroommove/http/handlers.go` | `production` | `api/common` | `CodeRoomsNotReleased`, `ErrorConflictWithCode`, `ErrorInvalidRequest`, `ErrorNotFound`, `Middleware`, `RenderError`, `RequireVisitUpdate`, `RequireWebAttendanceEnabled`, `Respond` |
| `backend/workflows/parentportal/http/announcement_handlers.go` | `production` | `api/common` | `ErrorInvalidRequest`, `ParsePositiveInt64IDWithError`, `RenderError`, `Respond` |
| `backend/workflows/parentportal/http/auth_handlers.go` | `production` | `api/common` | `CodeCareInvalidCredentials`, `CodeCareNotAGuardian`, `CodeIdentityPasswordResetLinkInvalid`, `CodeIdentityPasswordTooWeak`, `CodeIdentitySessionAccountInactive`, `ErrorForbiddenWithCode`, `ErrorGoneWithCode`, `ErrorInternalServer`, `ErrorInvalidOnField`, `ErrorInvalidRequest`, `ErrorUnauthorizedWithCode`, `GetClientIPString`, `RenderError`, `Respond` |
| `backend/workflows/parentportal/http/calendar_handlers.go` | `production` | `api/common` | `ErrorConflict`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorNotFound`, `ParsePositiveInt64IDWithError`, `RenderError`, `Respond` |
| `backend/workflows/parentportal/http/care_exception_handlers.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/workflows/parentportal/http/care_offerings_handlers.go` | `production` | `api/common` | `CodeCareOfferingChangeInvalid`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `RenderError`, `Respond` |
| `backend/workflows/parentportal/http/care_schedule_handlers.go` | `production` | `api/common` | `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/workflows/parentportal/http/child_messages_handlers.go` | `production` | `api/common` | `ErrorInvalidRequest`, `ParsePositiveInt64IDWithError`, `RenderError`, `Respond` |
| `backend/workflows/parentportal/http/child_write_handlers.go` | `production` | `api/common` | `CodeCareAnnouncementAckNotRequired`, `CodeCareAnnouncementNotAPoll`, `CodeCareAnnouncementStale`, `CodeCareCannotRemoveOwnAccess`, `CodeCareCareExceptionAlreadyLeft`, `CodeCareCareExceptionConflict`, `CodeCareCareExceptionNoTime`, `CodeCareCareExceptionPastDate`, `CodeCareCareExceptionReasonRequired`, `CodeCareCareExceptionReasonTooLong`, `CodeCareCareExceptionTooFar`, `CodeCareCareRequestAlreadyPending`, `CodeCareCareRequestBookingsAuthoritative`, `CodeCareCareRequestFieldDisabled`, `CodeCareChildCareEnded`, `CodeCareChildNotLinked`, `CodeCareExcusedRequestNotPending`, `CodeCareExcusedRequestOverlap`, `CodeCareGuardianContactInvalid`, `CodeCareGuardianEmailConflict`, `CodeCareGuardianHasOwnAccount`, `CodeCareGuardianManagementDisabled`, `CodeCareGuardianNoChange`, `CodeCareGuardianNotLinked`, `CodeCareGuardianPermissionDenied`, `CodeCareGuardianRelationshipInvalid`, `CodeCareGuardianRoleManaged`, `CodeCareGuardianSharedAcrossFamilies`, `CodeCareGuardianSocialWorkerManaged`, `CodeCareInvalidPollResponse`, `CodeCareInvalidRequestPayload`, `CodeCareInviteDisabled`, `CodeCareMasterDataDuplicatePending`, `CodeCareMasterDataEditDisabled`, `CodeCareMasterDataFieldNotEditable`, `CodeCareMasterDataInvalidValue`, `CodeCareMasterDataNoChanges`, `CodeCareMasterDataRequestDisabled`, `CodeCareNotesDisabled`, `CodeCareOfferingChangeAlreadyPending`, `CodeCareOfferingChangeForbidden`, `CodeCareOfferingChangeInvalid`, `CodeCareOfferingChangesDisabled`, `CodeCarePayerGuardianProtected`, `CodeCarePickupChangeCutoffPassed`, `CodeCarePickupChangeDisabled`, `CodeCarePollClosed`, `CodeCarePrimaryGuardianProtected`, `CodeCareRemoveDisabled`, `CodeCareRequestNotOpen`, `CodeCareSickNoteDisabled`, `CodeCareStaffManagedGuardianProtected`, `CodeEnrollmentCompleteWithdrawalConfirmationRequired`, `CodeMealsInvalidParticipation`, `CodeMealsParticipationCutoffPassed`, `CodeMealsParticipationOutOfRange`, `CodeMealsPlanDisabled`, `CodeMealsPlanWeekOutOfRange`, `CodeMealsRegistrationDisabled`, `CodeStudentsCareExceptionRaced`, `CodeStudentsCareOfferingsDisabled`, `CodeStudentsOfferingChangeCapacityFull`, `CodeStudentsOfferingChangesNoEnrollment`, `ErrorConflictWithCode`, `ErrorConflictWithDetails`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFound`, `ErrorUnauthorized`, `ParsePositiveInt64IDWithError`, `RenderError`, `Respond` |
| `backend/workflows/parentportal/http/consent_handlers.go` | `production` | `api/common` | `CodeCareChildCareEnded`, `CodeCareChildNotLinked`, `CodeCareGuardianPermissionDenied`, `CodeCarePhotoConsentNotWithdrawn`, `ErrorConflictWithCode`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorRule`, `RenderError`, `Respond`, `RulesRenderer` |
| `backend/workflows/parentportal/http/courses_handlers.go` | `production` | `api/common` | `CodeCareCourseAlreadyBooked`, `CodeCareCourseRequestInvalid`, `CodeCareCourseRequestNotOwn`, `CodeCareCourseRequestsDisabled`, `ErrorForbiddenWithCode`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFound`, `ParsePositiveInt64IDWithError`, `RenderError`, `Respond` |
| `backend/workflows/parentportal/http/declaration_handlers.go` | `production` | `api/common` | `CodeCareChildCareEnded`, `CodeCareDeclarationActionNotAllowed`, `CodeCareDeclarationClosed`, `CodeCareDeclarationNotPermitted`, `CodeCareDeclarationPasswordIncorrect`, `CodeCareDeclarationPasswordRequired`, `CodeCareDeclarationVersionChanged`, `ErrorConflictWithCode`, `ErrorForbiddenWithCode`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/workflows/parentportal/http/enrollment_handlers.go` | `production` | `api/common` | `BuildGuardianProfileResponse`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorUnauthorized`, `GuardianProfileData`, `GuardianProfileDefaults`, `ParsePositiveInt64IDWithError`, `RenderError`, `Respond` |
| `backend/workflows/parentportal/http/guardian_handlers.go` | `production` | `api/common` | `ErrorInvalidRequest`, `ParsePositiveInt64IDWithError`, `RenderError`, `Respond` |
| `backend/workflows/parentportal/http/master_data_handlers.go` | `production` | `api/common` | `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/workflows/parentportal/http/me_handlers.go` | `production` | `api/common` | `ErrorConflict`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorUnauthorized`, `RenderError`, `Respond` |
| `backend/workflows/parentportal/http/notification_preference_handlers.go` | `production` | `api/common` | `ErrorInternalServer`, `ErrorInvalidRequest`, `RenderError`, `Respond` |
| `backend/workflows/parentportal/http/password_reset_runtime.go` | `production` | `api/common` | `CodeIdentityPasswordResetRateLimited`, `ErrorTooManyRequestsWithCode`, `RenderError` |
| `backend/workflows/parentportal/http/push_handlers.go` | `production` | `api/common` | `ErrorConflict`, `ErrorInternalServerWrap`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorUnauthorized`, `RenderError`, `Respond` |
| `backend/workflows/parentportal/http/pwa_usage_handlers.go` | `production` | `api/common` | `ErrorInternalServerWrap`, `ErrorUnauthorized`, `RenderError`, `Respond` |
| `backend/workflows/parentportal/http/related_accounts_handlers.go` | `production` | `api/common` | `ErrorInvalidRequest`, `ParsePositiveInt64IDWithError`, `RenderError`, `Respond` |
| `backend/workflows/parentportal/http/request_edit_handlers.go` | `production` | `api/common` | `CodeCareInvalidRecipients`, `CodeCareOfferingChangeInvalid`, `CodeStudentsChangeRequestStale`, `CodeStudentsReasonRequired`, `ErrorConflictWithCode`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ParsePositiveInt64IDWithError`, `RenderError`, `Respond` |
| `backend/workflows/parentportal/http/request_sharing_handlers.go` | `production` | `api/common` | `CodeCareFamilyProtection`, `CodeCareRequestNotFound`, `CodeCareRequestSharingInvalid`, `ErrorForbiddenWithCode`, `ErrorInternalServer`, `ErrorInvalidRequestWithCode`, `ErrorNotFoundWithCode`, `ErrorRule`, `ParsePositiveInt64IDWithError`, `RenderError`, `Respond`, `RulesRenderer` |
| `backend/workflows/parentportal/http/today_status_handlers.go` | `production` | `api/common` | `Respond` |
| `backend/workflows/reminderdelivery/compose/http.go` | `production` | `api/common` | `ErrorForbidden`, `ErrorInternalServer`, `HasEffectiveAdminScope`, `Middleware`, `RenderError`, `RequiresPermission`, `Respond` |
| `backend/workflows/studentdeletion/http/student_fixture_helpers_test.go` | `external_test` | `api/common` | `Middleware` |
| `backend/workflows/studentdeletion/http/companion_lock_error_test.go` | `internal_test` | `api/common` | `ErrResponse` |
| `backend/workflows/studentdeletion/http/student_deletion_error_codes_test.go` | `internal_test` | `api/common` | `CodeStudentsCareWithdrawalAlreadyResolved`, `CodeStudentsCareWithdrawalNotFound`, `CodeStudentsDeletionAcknowledgementRequired`, `CodeStudentsDeletionAlumnus`, `CodeStudentsDeletionCompanionBlocked`, `CodeStudentsDeletionCompanionLockBusy`, `CodeStudentsDeletionConfirmationMismatch`, `CodeStudentsDeletionInvalidReason`, `CodeStudentsDeletionPreviewChanged`, `CodeStudentsDeletionRetentionNotEnded` |
| `backend/workflows/studentdeletion/http/authorization.go` | `production` | `api/common` | `StudentAccessSource` |
| `backend/workflows/studentdeletion/http/care_exit_handlers.go` | `production` | `api/common` | `CodeStudentsCareWithdrawalAlreadyResolved`, `CodeStudentsCareWithdrawalNotFound`, `ErrorConflictWithCode`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorNotFoundWithCode`, `ParsePositiveInt64IDWithError`, `Respond` |
| `backend/workflows/studentdeletion/http/errors.go` | `production` | `api/common` | `RenderError` |
| `backend/workflows/studentdeletion/http/student_delete_handlers.go` | `production` | `api/common` | `CodeStudentsCareWithdrawalAlreadyResolved`, `CodeStudentsCareWithdrawalNotFound`, `CodeStudentsDeletionAcknowledgementRequired`, `CodeStudentsDeletionAlumnus`, `CodeStudentsDeletionCompanionBlocked`, `CodeStudentsDeletionCompanionLockBusy`, `CodeStudentsDeletionConfirmationMismatch`, `CodeStudentsDeletionConstraintsChanged`, `CodeStudentsDeletionInvalidReason`, `CodeStudentsDeletionPreviewChanged`, `CodeStudentsDeletionRetentionNotEnded`, `ErrorConflictMessage`, `ErrorConflictWithCode`, `ErrorForbidden`, `ErrorInternalServer`, `ErrorInvalidRequest`, `ErrorInvalidRequestWithCode`, `ErrorNotFound`, `ErrorNotFoundWithCode`, `ErrorRule`, `RenderWithRules`, `Respond` |
| `backend/workflows/studentdeletion/http/student_handlers.go` | `production` | `api/common` | `ErrorInvalidRequest`, `ErrorNotFound`, `MsgInvalidStudentID`, `ParseID` |
| `backend/workflows/studentdeletion/http/student_resource.go` | `production` | `api/common` | `Middleware`, `RequiresPermission` |

### timetable-activities

| Importer file | Scope | Provider | Symbols |
|---|---|---|---|
| `backend/api/active_test_helpers.go` | `production` | `modules/timetable/compose` | `NewCategoryCount`, `NewClassArrivals`, `NewOperationLifecycle`, `NewRetainedActivityRecoveryRepository`, `Observation` |
| `backend/api/activities_test_helpers.go` | `production` | `modules/timetable/compose` | `ActivityService`, `NewService` |
| `backend/api/attendance_sync_test_helpers.go` | `production` | `modules/timetable/compose` | `NewActivityInstanceRepository`, `NewInstanceStudentRepository`, `NewNativeMirrorRows`, `OwnerRows`, `WithMirrorAfterWrite` |
| `backend/api/base.go` | `production` | `modules/timetable/compose` | `Dependencies`, `New`, `NewCalendarPeriodRepository`, `NewRecurrenceWriteLock`, `NewStaffNotices`, `Observation`, `RecurrenceLockDependencies`, `Repositories`, `StaffNoticeDependencies` |
| `backend/api/calendar_facts.go` | `production` | `modules/timetable/compose` | `FindCalendarAssignments`, `FindCalendarInstances`, `FindCalendarShifts`, `ListCalendarShiftTypes` |
| `backend/api/care_plan_pickup_excusal_timetable.go` | `production` | `modules/timetable/compose` | `PartialAbsencePreview` |
| `backend/api/care_plan_request_adapters.go` | `production` | `modules/timetable/compose` | `NewCareScheduleRequestRows` |
| `backend/api/care_plan_status_directories.go` | `production` | `modules/timetable/compose` | `NewInstanceStatusSlots` |
| `backend/api/care_plan_test_helpers.go` | `production` | `modules/timetable/compose` | `NewInstanceStudentRepository` |
| `backend/api/care_plan_wiring.go` | `production` | `modules/timetable/compose` | `NewClassArrivals`, `Observation` |
| `backend/api/classday_timetable_rows.go` | `production` | `modules/timetable/compose` | `PresenceReads` |
| `backend/api/cleanup_roots.go` | `production` | `modules/timetable/compose` | `NewActivityInstanceRepository` |
| `backend/api/database_stats_reader.go` | `production` | `modules/timetable/compose` | `Repositories` |
| `backend/api/device_scan_catalog.go` | `production` | `modules/timetable/compose` | `ActivityService`, `NewCatalog` |
| `backend/api/enrollment_bookings.go` | `production` | `modules/timetable/compose` | `NewPlanningCalendarPeriodUsage` |
| `backend/api/facilities_activity_catalog.go` | `production` | `modules/timetable/compose` | `ActivityService`, `NewCatalog` |
| `backend/api/history_slots.go` | `production` | `modules/timetable/compose` | `NewAttendanceHistory` |
| `backend/api/iot_session_mirror_test_helpers.go` | `production` | `modules/timetable/compose` | `ActivityService` |
| `backend/api/kiosk_timetable.go` | `production` | `modules/timetable/compose` | `ActivityService`, `NewCatalog`, `Repositories` |
| `backend/api/owner_roots_test_helpers.go` | `production` | `modules/timetable/compose` | `OwnerRows`, `SubstituteDayLock` |
| `backend/api/owner_schedule_fixture_test_helpers.go` | `production` | `modules/timetable/compose` | `NewClassArrivals`, `Observation` |
| `backend/api/parent_route_test_helpers.go` | `production` | `modules/timetable/compose` | `NewInstanceStudentRepository` |
| `backend/api/partial_absence_preview.go` | `production` | `modules/timetable/compose` | `NewPartialAbsencePreview`, `PartialAbsencePreview`, `PresenceReads` |
| `backend/api/people_repository_test_helpers.go` | `production` | `modules/timetable/compose` | `NewPlanningCalendarPeriodUsage`, `NewSchoolCalendarRepositories`, `SchoolCalendarRepositories` |
| `backend/api/presence_projections.go` | `production` | `modules/timetable/compose` | `PresenceReads` |
| `backend/api/presence_session_records.go` | `production` | `modules/timetable/compose` | `NewActivityGroupRecords`, `NewTimetableGroupReads` |
| `backend/api/presence_suite_test_helpers.go` | `production` | `modules/timetable/compose` | `NewCategoryCount` |
| `backend/api/reminder_timetable.go` | `production` | `modules/timetable/compose` | `ParseActivityInstanceDate`, `WrapDatabaseError` |
| `backend/api/room_repository_test_helpers.go` | `production` | `modules/timetable/compose` | `OwnerRooms` |
| `backend/api/scheduler_ports.go` | `production` | `modules/timetable/compose` | `ActivityInstanceDate`, `Repositories` |
| `backend/api/service_graph.go` | `production` | `modules/timetable/compose` | `ActivityService`, `Planning` |
| `backend/api/service_graph_compose.go` | `production` | `modules/timetable/compose` | `NewCalendarPeriodRepository`, `NewCallerActivityGroups`, `NewPlanningCalendarPeriodUsage`, `NewRosterReconciler`, `OwnerRows`, `TemplateRows` |
| `backend/api/service_graph_compose_requests.go` | `production` | `modules/timetable/compose` | `NewEnrollmentArrivalSchedules`, `NewEnrollmentPickupSchedules`, `NewTimetableInstanceReads`, `NewTimetableInstanceStaffReads` |
| `backend/api/service_graph_compose_timetable.go` | `production` | `modules/timetable/compose` | `NewCategoryCount`, `NewLifecycleSchedulers`, `NewMaterialization`, `NewOperationLifecycle`, `NewRetainedActivityRecoveryRepository`, `NewService`, `NewStaffDeviations`, `NewSubstituteConflictsOverRows`, `NewTenantDatabase`, `NewTenantTransactions`, `NewTimetableGroupReads`, `NewTimetableInstanceReads`, `NewTimetableInstanceStaffReads`, `OwnerRows`, `ShiftTypeCategoryLinker` |
| `backend/api/service_graph_compose_views.go` | `production` | `modules/timetable/compose` | `NewActivityCategorySeeder`, `NewBlockDisplayMetadata`, `NewInstanceSeriesConversion`, `NewTemplateAdministration`, `NewTenantTransactions`, `NewTimetableGroupReads`, `NewTimetableInstanceReads`, `NewTimetableInstanceStaffReads`, `Planning`, `SubstituteDayLock` |
| `backend/api/service_graph_wiring.go` | `production` | `modules/timetable/compose` | `OwnerRows`, `PartialAbsencePreview`, `Repositories`, `TemplateRows` |
| `backend/api/staff_offboarding_owners.go` | `production` | `modules/timetable/compose` | `NewOffboarding`, `NewPresenceSessionFacts`, `Observation`, `OffboardingDependencies` |
| `backend/api/staff_offboarding_test_helpers.go` | `production` | `modules/timetable/compose` | `ActivityInstanceDate` |
| `backend/api/staff_route_repository_test_helpers.go` | `production` | `modules/timetable/compose` | `Repositories` |
| `backend/api/staff_route_test_helpers.go` | `production` | `modules/timetable/compose` | `ActivityService`, `NewService`, `NewTimetableGroupReads`, `NewTimetableInstanceReads`, `NewTimetableInstanceStaffReads`, `ShiftTypeCategoryLinker` |
| `backend/api/student_presence_roots.go` | `production` | `modules/timetable/compose` | `NewPresenceReads`, `NewPresenceSessionFacts`, `PartialAbsencePreview`, `PresenceReads` |
| `backend/api/student_schedule_adapters.go` | `production` | `modules/timetable/compose` | `NewCareArrivalExceptionRows`, `NewCareArrivalScheduleRows`, `NewCarePickupExceptionRows`, `NewCarePickupScheduleRows` |
| `backend/api/student_schedule_test_helpers.go` | `production` | `modules/timetable/compose` | `CareScheduleRepositories`, `NewCarePickupNoteRows`, `NewCareScheduleRepositories` |
| `backend/api/student_test_helpers.go` | `production` | `modules/timetable/compose` | `NewEnrollmentArrivalSchedules`, `NewEnrollmentPickupSchedules`, `NewRosterReconciler`, `NewTimetableInstanceReads`, `NewTimetableInstanceStaffReads` |
| `backend/api/students_owner_ports.go` | `production` | `modules/timetable/compose` | `ActivityService` |
| `backend/api/timetable_cleanup_composition.go` | `production` | `modules/timetable/compose` | `NewTimetableCleanup` |
| `backend/api/timetable_conflict_composition.go` | `production` | `modules/timetable/compose` | `NewConflictDetection` |
| `backend/api/timetable_data_composition.go` | `production` | `modules/timetable/compose` | `NewTimetableData` |
| `backend/api/timetable_deviation_composition.go` | `production` | `modules/timetable/compose` | `NativeMirrorRows`, `NewAttendanceCorrections`, `NewAttendanceMirror`, `NewNativeMirrorRows`, `OwnerRows` |
| `backend/api/timetable_http_test_helpers.go` | `production` | `modules/timetable/compose` | `NewAttendanceCorrections`, `NewBlockDisplayMetadata`, `NewClassArrivalQueries`, `NewConflictDetection`, `NewMaterialization`, `NewRetainedActivityRecoveryRepository`, `NewTemplateAdministration`, `NewTenantTransactions`, `NewTimetableData`, `Observation`, `OwnerRows` |
| `backend/api/timetable_lifecycle_composition.go` | `production` | `modules/timetable/compose` | `NewInstanceLifecycle` |
| `backend/api/timetable_operations_composition.go` | `production` | `modules/timetable/compose` | `NewOperations` |
| `backend/api/timetable_people_views.go` | `production` | `modules/timetable/compose` | `RosterStudents` |
| `backend/api/timetable_repositories.go` | `production` | `modules/timetable/compose` | `NewRepositories`, `Observation`, `Repositories`, `RepositoryDependencies` |
| `backend/api/timetable_repository_test_helpers.go` | `production` | `modules/timetable/compose` | `CalendarRows`, `CareScheduleRepositories`, `NativeMirrorRows`, `NewCalendarPeriodRepository`, `NewClassArrivalExceptionRepository`, `NewClosingDayRepository`, `NewDateframeRepository`, `NewPlanningCalendarPeriodUsage`, `NewRecurrenceWriteLock`, `NewRosterReconciler`, `OwnerRows`, `RecurrenceLockDependencies`, `Repositories`, `TemplateRows` |
| `backend/api/timetable_session_end_composition.go` | `production` | `modules/timetable/compose` | `NewEndedSessionCompletion`, `OwnerRows` |
| `backend/api/timetable_students.go` | `production` | `modules/timetable/compose` | `StudentDirectory`, `StudentDirectoryFunc`, `TargetStudent` |
| `backend/api/timetable_test_helpers.go` | `production` | `modules/timetable/compose` | `NewBlockDisplayMetadata`, `NewClassArrivalQueries`, `NewMaterialization`, `NewPlanningTrackAdministration`, `NewRetainedActivityRecoveryRepository`, `NewStaffDeviations`, `NewSubstituteConflictsOverRows`, `NewTemplateAdministration`, `NewTenantDatabase`, `NewTenantTransactions`, `Observation`, `Planning` |
| `backend/api/timetable_wiring.go` | `production` | `modules/timetable/compose` | `NewPlanningTrackAdministration`, `NewPresenceReads`, `NewRecurrenceWriteLock`, `RecurrenceLockDependencies` |
| `backend/api/usercontext_test_helpers.go` | `production` | `modules/timetable/compose` | `NewCallerActivityGroups` |
| `backend/api/workforce_shift_rows.go` | `production` | `modules/timetable/compose` | `NewWorkforceShiftRows`, `NewWorkforceShiftTypeRows` |
| `backend/api/workforce_test_helpers.go` | `production` | `modules/timetable/compose` | `NewTimetableGroupReads`, `NewTimetableInstanceReads`, `NewTimetableInstanceStaffReads`, `SubstituteDayLock` |
| `backend/modules/careplan/contracttest/arrival_baseline_service_test.go` | `external_test` | `modules/timetable/compose` | `NewClassArrivalQueries`, `NewClassArrivals`, `Observation` |
| `backend/modules/careplan/contracttest/arrival_class_bulk_test.go` | `external_test` | `modules/timetable/compose` | `NewClassArrivals`, `Observation` |
| `backend/modules/careplan/contracttest/arrival_class_exception_test.go` | `external_test` | `modules/timetable/compose` | `NewClassArrivals`, `Observation` |
| `backend/modules/careplan/contracttest/care_offering_catalog_guards_test.go` | `external_test` | `modules/timetable/compose` | `NativeExceptionRecords`, `NativeGroupRecords`, `NativeScheduleRecords`, `NativeTimeframeRecords` |
| `backend/modules/careplan/contracttest/care_offering_catalog_test.go` | `external_test` | `modules/timetable/compose` | `NativeGroupRecords`, `NativeScheduleRecords` |
| `backend/modules/careplan/contracttest/enrollment_decision_env_test.go` | `external_test` | `modules/timetable/compose` | `NewEnrollmentArrivalSchedules`, `NewEnrollmentPickupSchedules` |
| `backend/modules/careplan/contracttest/lifecycle_care_lifecycle_integration_test.go` | `external_test` | `modules/timetable/compose` | `NativeEnrollmentRecords` |
| `backend/modules/careplan/contracttest/lifecycle_care_lifecycle_service_test.go` | `external_test` | `modules/timetable/compose` | `NativeEnrollmentRecords` |
| `backend/modules/careplan/contracttest/offering_arrival_projection_test.go` | `external_test` | `modules/timetable/compose` | `NewClassArrivalQueries`, `Observation` |
| `backend/modules/careplan/contracttest/offering_change_request_service_test.go` | `external_test` | `modules/timetable/compose` | `NativeGroupRecords` |
| `backend/modules/careplan/contracttest/offering_source_service_test.go` | `external_test` | `modules/timetable/compose` | `NativeEnrollmentRecords`, `NativeGroupRecords` |
| `backend/modules/careplan/contracttest/pickup_auto_excusal_test.go` | `external_test` | `modules/timetable/compose` | `NewInstanceStatusSlots` |
| `backend/modules/careplan/contracttest/template_roster_maintenance_test.go` | `external_test` | `modules/timetable/compose` | `NativeGroupRecords` |
| `backend/modules/careplan/contracttest/timetable_arrival_projection_test.go` | `external_test` | `modules/timetable/compose` | `NativeExceptionRecords`, `NewConflictDetection`, `NewTimetableData` |
| `backend/modules/requestreview/compose/offering_reviews.go` | `production` | `modules/timetable/compose` | `NewCourseGroupQueries` |
| `backend/modules/requestreview/compose/schedule_reviews.go` | `production` | `modules/timetable/compose` | `NewClassArrivalQueries`, `Observation` |
| `backend/modules/timetable/timetabletest/timetable.go` | `production` | `modules/timetable/compose` | `Dependencies`, `New`, `NewPresenceReads`, `Observation`, `PresenceReads`, `StudentDirectory`, `StudentDirectoryFunc`, `TargetStudent` |
| `backend/workflows/staffoffboarding/offboarding_test.go` | `external_test` | `modules/timetable/compose` | `NewOffboarding`, `NewPresenceSessionFacts`, `Observation`, `OffboardingDependencies` |

### timetable-legacy-view

| Importer file | Scope | Provider | Symbols |
|---|---|---|---|
| `backend/workflows/studentdeletion/compose/new.go` | `production` | `modules/timetableprojection` | `CountStudentEnrollments` |

### transaction-runtime

| Importer file | Scope | Provider | Symbols |
|---|---|---|---|
| `backend/models/entity_interfaces_test.go` | `internal_test` | `models/base` | `Entity` |
| `backend/models/delivery/push_subscription.go` | `production` | `models/base` | `CRUDRepository`, `Model`, `TenantModel` |
| `backend/modules/delivery/internal/adapters/deliverypostgres/push_subscription.go` | `production` | `database/repositories/base` | `EnsureTenantID`, `GetDB`, `NewRepository`, `Repository`, `TranslateNotFound`, `WithTenantFilter` |
| `backend/modules/delivery/internal/adapters/deliverypostgres/push_subscription.go` | `production` | `models/base` | `DatabaseError` |

### workforce

| Importer file | Scope | Provider | Symbols |
|---|---|---|---|
| `backend/api/plan_export_dienstplan_test.go` | `internal_test` | `modules/workforce/compose` | `Dependencies`, `New`, `NewStaffScheduleOverview`, `Observation`, `RoomReader`, `StaffScheduleOverviewDependencies` |
| `backend/api/time_tracking_absence_email_test.go` | `internal_test` | `modules/workforce/compose` | `AbsenceEmailMessage` |
| `backend/api/time_tracking_audits_internal_test.go` | `internal_test` | `modules/workforce/compose` | `TimeTrackingAuditCursor`, `TimeTrackingAuditEntry`, `TimeTrackingAuditFilter`, `WorkSessionEdit` |
| `backend/api/time_tracking_documents_internal_test.go` | `internal_test` | `modules/workforce/compose` | `PayrollExportCategory`, `PayrollExportConfiguration` |
| `backend/api/auth_persistence_test_helpers.go` | `production` | `modules/workforce/compose` | `MustNewStaffEmployment` |
| `backend/api/base.go` | `production` | `modules/workforce/compose` | `Dependencies`, `New`, `NewAbsenceTypeAdministration`, `NewStaffEmployment`, `Observation` |
| `backend/api/data_access_audit.go` | `production` | `modules/workforce/compose` | `DataAccessAudit`, `DataAccessEvent` |
| `backend/api/deletion_audit.go` | `production` | `modules/workforce/compose` | `DeletionAudit`, `DeletionEvent` |
| `backend/api/delivery_staff_recipients.go` | `production` | `modules/workforce/compose` | `WorkPresenceQuery` |
| `backend/api/document_cleanup_test_helpers.go` | `production` | `modules/workforce/compose` | `NewDocumentCleanup` |
| `backend/api/membership_repositories.go` | `production` | `modules/workforce/compose` | `MustNewStaffEmployment` |
| `backend/api/membership_test_helpers.go` | `production` | `modules/workforce/compose` | `Dependencies`, `New`, `Observation` |
| `backend/api/people_directory_roots.go` | `production` | `modules/workforce/compose` | `MustNewStaffEmployment` |
| `backend/api/people_repository_test_helpers.go` | `production` | `modules/workforce/compose` | `StaffAbsenceRepository` |
| `backend/api/presence_work_sessions.go` | `production` | `modules/workforce/compose` | `AutoCheckIn` |
| `backend/api/retention_test_helpers.go` | `production` | `modules/workforce/compose` | `NewCleanupRetention`, `NewSickNoteRetention` |
| `backend/api/scheduler_ports.go` | `production` | `modules/workforce/compose` | `WorkPresence` |
| `backend/api/scheduler_time_tracking_cleanup.go` | `production` | `modules/workforce/compose` | `TimeTrackingCleanupService` |
| `backend/api/school_membership.go` | `production` | `modules/workforce/compose` | `NewStaffEmployment` |
| `backend/api/server.go` | `production` | `modules/workforce/compose` | `NewCleanupRetention`, `NewSickNoteRetention` |
| `backend/api/service_graph.go` | `production` | `modules/workforce/compose` | `TimeTracking`, `TimeTrackingCleanupService` |
| `backend/api/service_graph_compose.go` | `production` | `modules/workforce/compose` | `ShiftPlanning` |
| `backend/api/service_graph_compose_requests.go` | `production` | `modules/workforce/compose` | `WorkPresence` |
| `backend/api/service_graph_compose_timetable.go` | `production` | `modules/workforce/compose` | `NewShiftPlanning`, `NewTimeTrackingCleanup`, `ShiftPlanningDependencies`, `TimeTrackingCleanupDependencies` |
| `backend/api/service_graph_compose_views.go` | `production` | `modules/workforce/compose` | `NewStaffShiftLock` |
| `backend/api/service_graph_wiring.go` | `production` | `modules/workforce/compose` | `DeletionAudit`, `HolidayDatesReader`, `ShiftPlanSyncer`, `TimeTracking` |
| `backend/api/staff_admin_composition.go` | `production` | `modules/workforce/compose` | `NewStaffAdmin`, `StaffAdminDependencies` |
| `backend/api/staff_projection_education.go` | `production` | `modules/workforce/compose` | `GroupSubstitution`, `NewGroupSubstitutionRepository`, `SubstitutionBlocker` |
| `backend/api/staff_route_test_helpers.go` | `production` | `modules/workforce/compose` | `NewAbsenceTypeAdministration`, `NewShiftPlanning`, `ShiftPlanningDependencies` |
| `backend/api/time_tracking_absence_email.go` | `production` | `modules/workforce/compose` | `AbsenceEmailContact`, `AbsenceEmailMessage` |
| `backend/api/time_tracking_audits.go` | `production` | `modules/workforce/compose` | `TimeTrackingAuditEntry`, `TimeTrackingAuditFilter`, `TimeTrackingAuditReader`, `TimeTrackingDeletionAudit`, `TimeTrackingDeletionEvent`, `WorkSessionAudit`, `WorkSessionEdit` |
| `backend/api/time_tracking_composition.go` | `production` | `modules/workforce/compose` | `NewDocumentCleanup` |
| `backend/api/time_tracking_documents.go` | `production` | `modules/workforce/compose` | `PayrollExportCategory`, `PayrollExportConfiguration`, `TimeTrackingDocument` |
| `backend/api/time_tracking_events.go` | `production` | `modules/workforce/compose` | `EventPublisher` |
| `backend/api/time_tracking_ports.go` | `production` | `modules/workforce/compose` | `NewTimeTracking`, `NewTimeTrackingShifts`, `StaffEmploymentQuery`, `StaffScheduleAssignments`, `StaffScheduleBinding`, `StaffScheduleBindingLookup`, `TimeTracking`, `TimeTrackingPorts` |
| `backend/api/time_tracking_staff.go` | `production` | `modules/workforce/compose` | `MonthCloseStaffQuery`, `OverviewStaff`, `OverviewStaffQuery`, `StaffDisplayNameQuery`, `StaffEmploymentQuery`, `StaffScheduleAssignment`, `StaffScheduleAssignments`, `StaffScheduleBinding`, `TimeExportStaff`, `TimeExportStaffQuery`, `WorkSessionStaff`, `WorkSessionStaffName` |
| `backend/api/time_tracking_test_helpers.go` | `production` | `modules/workforce/compose` | `EventPublisher`, `HolidayDatesReader`, `MustNewStaffEmployment`, `NewStaffAbsenceAuditRepository`, `ShiftPlanSyncer`, `TimeTracking`, `TimeTrackingPorts` |
| `backend/api/work_session_repository_test_helpers.go` | `production` | `modules/workforce/compose` | `StaffAbsenceRepository` |
| `backend/api/work_session_test_helpers.go` | `production` | `modules/workforce/compose` | `TimeTracking` |
| `backend/api/worker_retention.go` | `production` | `modules/workforce/compose` | `MustNewStaffEmployment`, `NewRetiredStaffRecords` |
| `backend/api/workforce_admin_capabilities.go` | `production` | `modules/workforce/compose` | `TimeTracking` |
| `backend/api/workforce_composition.go` | `production` | `modules/workforce/compose` | `AbsenceEmailDeps`, `Dependencies`, `MustNewStaffEmployment`, `New`, `NewStaffAbsenceAuditRepository`, `Observation`, `StaffAdminDependencies`, `TimeTracking`, `TimeTrackingPorts` |
| `backend/api/workforce_legacy_schedule_repositories.go` | `production` | `modules/workforce/compose` | `LegacyWorkTimeModelNotFound` |
| `backend/api/workforce_repository_test_helpers.go` | `production` | `modules/workforce/compose` | `NewStaffAbsenceAuditRepository`, `NewStaffAbsenceTypeAllowanceChangeRepository`, `NewStaffAbsenceTypeAllowanceRepository`, `StaffAbsenceAuditRepository`, `StaffAbsenceTypeAllowanceChangeRepository`, `StaffAbsenceTypeAllowanceRepository` |
| `backend/api/workforce_schedule_records.go` | `production` | `modules/workforce/compose` | `WorkScheduleRow`, `WorkScheduleTargets`, `WorkTimeTargetModel`, `WorkTimeTemplate`, `WorkTimeTemplateEntry` |
| `backend/api/workforce_staff_absences.go` | `production` | `modules/workforce/compose` | `AbsenceRequestFilter`, `AbsenceRequestRow`, `NewStaffAbsenceRepository`, `StaffAbsenceRepository` |
| `backend/api/workforce_test_helpers.go` | `production` | `modules/workforce/compose` | `AbsenceRequestFilter`, `MustNewStaffEmployment`, `NewShiftPlanning`, `NewStaffShiftLock`, `Observation`, `ShiftPlanningDependencies`, `StaffAbsence`, `StaffAdminDependencies`, `TimeTracking` |
| `backend/cmd/cleanup.go` | `production` | `modules/workforce/compose` | `TimeTrackingCleanupPreview`, `TimeTrackingCleanupResult`, `TimeTrackingCleanupStats` |
| `backend/cmd/cleanup_helpers.go` | `production` | `modules/workforce/compose` | `TimeTrackingCleanupService` |
| `backend/cmd/cleanup_roots.go` | `production` | `modules/workforce/compose` | `NewStaffAbsenceRepository`, `NewTimeTrackingCleanup`, `TimeTrackingCleanupDependencies`, `TimeTrackingCleanupService` |
| `backend/modules/schoolstructure/internal/adapters/staffgroups/staffgroups_test.go` | `external_test` | `modules/workforce/compose` | `Dependencies`, `New`, `NewStaffEmployment`, `Observation` |
| `backend/workflows/shiftplansync/compose/workflow_env_test.go` | `external_test` | `modules/workforce/compose` | `NewShiftPlanning`, `NewStaffShiftLock`, `ShiftPlanningDependencies` |
| `backend/workflows/staffoffboarding/offboarding_test.go` | `external_test` | `modules/workforce/compose` | `Dependencies`, `New`, `NewOffboarding`, `NewStaffEmployment`, `Observation` |
| `backend/workflows/staffoffboarding/compose/new.go` | `production` | `modules/workforce/compose` | `Dependencies`, `NewOffboarding`, `Observation` |

## Legacy-prefixed files retained

No file-only rename is counted as a completed dissolution. The following production files still require their owner-native split:

- `backend/modules/peopledirectory/compose/legacy_repository_runtime.go`
- `backend/modules/presenceprojection/legacy_rows.go`
- `backend/modules/schoolstructure/compose/legacy_group_catalog.go`
- `backend/modules/schoolstructure/compose/legacy_group_contract.go`
- `backend/modules/schoolstructure/compose/legacy_group_management_test_helpers.go`
- `backend/modules/schoolstructure/compose/legacy_repository_runtime.go`
- `backend/modules/timetable/compose/legacy_activity_service.go`
- `backend/modules/timetable/compose/legacy_category_operations.go`
- `backend/modules/timetable/compose/legacy_enrollment_operations.go`
- `backend/modules/timetable/compose/legacy_errors.go`
- `backend/modules/timetable/compose/legacy_group_update.go`
- `backend/modules/timetable/compose/legacy_interface.go`
- `backend/modules/timetable/compose/legacy_schedule_operations.go`
- `backend/modules/timetable/compose/legacy_supervisor_operations.go`

## Verification and publication

Focused Audit behavior tests and source ratchets passed before the final full run. Full-check outcomes are recorded in the local handoff after completion; this document does not claim an uncompleted command passed. `TestFullProductionRouterGolden` source and golden fixtures are unchanged.

Publication is held for #4080. No PR or merge commit exists for this branch. After an authorized continuation, integrate the confirmed merged development head, repeat the required checks, and push only once.
