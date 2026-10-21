# Final legacy cluster: round 8 compose audit

> **Historical record.** State at `ae894d7538` (2026-10-19). #2580 and #4069 are closed;
> counts, budgets and open issues below describe that state, not today.
> Final state: [`backend/architecture/WORKPLAN.md`](../../backend/architecture/WORKPLAN.md#endstand).

Base: `148e5aeb86cf22fdbfbffedd76ba070f52f80a3e`. This inventories **every policy package with role `compose`**, except the root-composition and dev-tools owners. Package role, not directory spelling, selects the inventory. Test-hook declarations are listed separately from production. A LOC reduction alone does not prove the exit.

Integrated development: `e423ad19c732f17966b2f8332eafb5df01196f29`.

Classification: **direct binding** has no local branch/loop and at most one delegated call; it supplies a native capability or type conversion without adding a decision. **construction/binding** identifies reviewed startup, callback-signature or runtime-capability wiring. **runtime retained** contains local orchestration, row/error mapping or execution checks and prevents the literal exit. **test-only** does not enter the production exit. Constructor callbacks are audited below too, so a `New` prefix cannot hide runtime logic.


## Round 9 owner set B

Re-audited against `12608383f0f793e811f4379ec293f61efd6150c9`. Set A and the student-notes follow-up are integrated through development `545bbe9f27`; all 61 compose packages were re-audited after integration. Counts below are production runtime-retained non-constructors; remaining constructor callbacks in B only bind or forward native values. Transaction operations stay bound at their existing registered compose declarations and execute in named internal functions. No transaction-register keys or router goldens changed. Caregiver bindings retain comparable value fields.

| Compose package | Runtime before → after | Compose LOC before → after |
|---|---:|---:|
| `modules/appointments/compose` | 2 → 0 | 67 → 45 |
| `modules/careplan/compose` | 49 → 0 | 2460 → 1646 |
| `modules/classday/compose` | 13 → 0 | 461 → 213 |
| `modules/dataimport/compose` | 6 → 0 | 202 → 85 |
| `modules/delivery/compose` | 20 → 0 | 759 → 363 |
| `modules/devicefleet/compose` | 8 → 0 | 353 → 219 |
| `modules/devicescan/compose` | 48 → 0 | 1228 → 531 |
| `modules/enrollment/compose` | 9 → 0 | 714 → 540 |
| `modules/exporttransfer/compose` | 5 → 0 | 234 → 168 |
| `modules/facilities/compose` | 4 → 0 | 222 → 184 |
| `modules/filestorage/compose` | 7 → 0 | 341 → 217 |
| `modules/identityaccess/compose` | 19 → 0 | 1729 → 1332 |
| `modules/organizationtenancy/compose` | 11 → 0 | 474 → 254 |
| `modules/peopledirectory/compose` | 19 → 0 | 1168 → 974 |
| `modules/schoolcalendar/compose` | 2 → 0 | 165 → 148 |
| `modules/settings/compose` | 11 → 0 | 380 → 308 |
| `modules/timetable/compose` | 11 → 0 | 1365 → 1182 |
| `modules/transactionruntime/compose` | 1 → 0 | 137 → 133 |
| `modules/workforce/compose` | 16 → 0 | 1164 → 931 |
| `modules/schoolcalendar/portal/compose` | 4 → 0 | 119 → 112 |
| `workflows/shiftplansync/compose` | 1 → 0 | 166 → 127 |

B runtime retained: **266 → 0**. Global runtime-retained functions and constructor callbacks: **0**. Global compose LOC ceiling: **20,119 → 12,491**; current development cap before this PR: **16,742**.

### Final integrated boundaries

DeviceScan publishes its workflow-owned port records. Foreign Student Presence, School Structure and Care Plan conversions now live in `roots/devicescanbinding`, using existing typed-root permissions. Root adapters consume operation-sized ports, not a multi-owner service graph. Native application decisions remain internal. Delegations moved into roots and public ports remain in the existing ADR 0064 scan through exact path literals; predicates and budgets are not loosened.

Communication's application caller joins an ambient transaction through its bound `HasTransaction` capability, otherwise invokes the original registered admin-retry binding. The transaction register accepts the narrowed branch-free `transaction.RunAdmin` body. Student Presence's byte-identical `transaction.Run` is consistently classified as a direct binding, like the other branch-free tenant forwards. Neither registered operation is relocated or replaced.

Feedback, Meal Plan and Request Feed constructor callbacks now use named native factories with injected context lookups; no tenant dependency grant is added. No exact blockers remain. The student-notes merge adds no compose runtime. People Directory delegations measure 298 and the total cap is 2,925, both below development (302 / 2,970). No new alias, grant, exception, transaction-register key, checker allowance or router golden change.

## Package summary

| Compose package | Owner | Non-constructors | Runtime retained |
|---|---|---:|---:|
| `modules/appointments/compose` | `appointments` | 2 | 0 |
| `modules/auditlog/compose` | `audit-platform` | 0 | 0 |
| `modules/careplan/compose` | `care-plan` | 27 | 0 |
| `modules/careplan/requestfeed/compose` | `care-plan` | 2 | 0 |
| `modules/classday/compose` | `class-day-view` | 2 | 0 |
| `modules/communication/composition` | `communication` | 6 | 0 |
| `modules/communication/parentstore` | `communication` | 23 | 0 |
| `modules/communication/staffstore` | `communication` | 2 | 0 |
| `modules/dataimport/compose` | `data-import` | 10 | 0 |
| `modules/dataimport/compose/imports` | `data-import` | 0 | 0 |
| `modules/delivery/compose` | `delivery-platform` | 9 | 0 |
| `modules/devicefleet/compose` | `device-fleet` | 3 | 0 |
| `modules/devicefleet/compose/repositoryadapter` | `device-fleet` | 0 | 0 |
| `modules/devicescan/compose` | `process-device-scan` | 26 | 0 |
| `modules/emergencysnapshot/compose` | `emergency-snapshot` | 0 | 0 |
| `modules/enrollment/compose` | `enrollment` | 2 | 0 |
| `modules/exporttransfer/compose` | `export-transfer` | 6 | 0 |
| `modules/facilities/compose` | `facilities` | 7 | 0 |
| `modules/facilities/compose/repositoryadapter` | `facilities` | 0 | 0 |
| `modules/feedback/compose` | `feedback` | 3 | 0 |
| `modules/filestorage/compose` | `file-storage` | 7 | 0 |
| `modules/grouplive/compose` | `group-live-view` | 0 | 0 |
| `modules/guardianlinkview/compose` | `guardian-link-view` | 0 | 0 |
| `modules/identityaccess/compose` | `identity-access` | 29 | 0 |
| `modules/mealplan/compose` | `meal-plan` | 4 | 0 |
| `modules/operatordashboard` | `operator-dashboard-view` | 1 | 0 |
| `modules/organizationtenancy/compose` | `organization-tenancy` | 5 | 0 |
| `modules/parentannouncementaudience` | `parent-announcement-audience` | 0 | 0 |
| `modules/parentmessageinbox` | `parent-message-inbox` | 0 | 0 |
| `modules/parentrequestfeedprojection/compose` | `parent-request-feed-view` | 0 | 0 |
| `modules/peopledirectory/compose` | `people-directory` | 27 | 0 |
| `modules/presenceprojection/compose` | `presence-legacy-view` | 0 | 0 |
| `modules/requestreview/compose` | `request-review-view` | 0 | 0 |
| `modules/schedulerruntime/compose` | `scheduler-runtime` | 1 | 0 |
| `modules/schoolcalendar/compose` | `school-calendar` | 2 | 0 |
| `modules/schoolcalendar/portal/compose` | `school-calendar` | 5 | 0 |
| `modules/schoolmembership/compose` | `school-membership` | 2 | 0 |
| `modules/schoolsetup/compose` | `school-setup` | 2 | 0 |
| `modules/schoolstructure/compose` | `school-structure` | 23 | 0 |
| `modules/settings/compose` | `settings-platform` | 16 | 0 |
| `modules/staffannouncementaudience` | `staff-announcement-audience` | 1 | 0 |
| `modules/staffmessageinbox` | `staff-message-inbox` | 0 | 0 |
| `modules/studentdirectoryview/compose` | `student-directory-view` | 0 | 0 |
| `modules/studentpresence/compose` | `student-presence` | 7 | 0 |
| `modules/studentpresence/compose/presenceservice` | `student-presence` | 3 | 0 |
| `modules/supervisiondashboard/compose` | `calendar-view` | 1 | 0 |
| `modules/timetable/compose` | `timetable-activities` | 29 | 0 |
| `modules/timetableenrollmentprovenance/compose` | `timetable-enrollment-provenance` | 0 | 0 |
| `modules/timetableprojection/compose` | `timetable-legacy-view` | 0 | 0 |
| `modules/transactionruntime/compose` | `transaction-runtime` | 10 | 0 |
| `modules/workforce/compose` | `workforce` | 15 | 0 |
| `observability/compose` | `observability` | 1 | 0 |
| `workflows/gradetransition/compose` | `grade-transition` | 0 | 0 |
| `workflows/openroommove/compose` | `open-room-move` | 0 | 0 |
| `workflows/parentportal/compose` | `parent-portal` | 3 | 0 |
| `workflows/reminderdelivery/compose` | `reminder-delivery` | 0 | 0 |
| `workflows/sessionend/compose` | `session-end` | 0 | 0 |
| `workflows/shiftplansync/compose` | `shift-plan-sync` | 0 | 0 |
| `workflows/staffclock/compose` | `staff-clock` | 0 | 0 |
| `workflows/staffoffboarding/compose` | `staff-offboarding` | 0 | 0 |
| `workflows/studentdeletion/compose` | `student-deletion` | 0 | 0 |

## Every remaining non-constructor


### `modules/appointments/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `database` | [composition_database_test_helpers.go:10](../../../backend/modules/appointments/compose/composition_database_test_helpers.go#L10) | test-only | `postgres.DatabaseBinding` |
| `withinTenant` | [new.go:33](../../../backend/modules/appointments/compose/new.go#L33) | direct binding | `postgres.WithinTenantBinding` |


### `modules/auditlog/compose`

No non-constructor functions.


### `modules/careplan/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `TenantAmbientDatabase` | [ambient_database.go:8](../../../backend/modules/careplan/compose/ambient_database.go#L8) | direct binding | `postgresNative.TenantAmbientDatabase` |
| `BookingRowNotFound` | [booking_materialization.go:36](../../../backend/modules/careplan/compose/booking_materialization.go#L36) | direct binding | `ports.BookingRowNotFound` |
| `bookingDependencies` | [booking_materialization.go:38](../../../backend/modules/careplan/compose/booking_materialization.go#L38) | construction/binding | `tenant.NewTransactionRunner`, `application.AnnounceOfferingPickupChange` |
| `ownTenantTransaction` | [care_lifecycle.go:125](../../../backend/modules/careplan/compose/care_lifecycle.go#L125) | direct binding | `postgres.OwnTenantTransactionBinding` |
| `CatalogRowNotFound` | [care_offering_catalog.go:42](../../../backend/modules/careplan/compose/care_offering_catalog.go#L42) | direct binding | `ports.CatalogRowNotFound` |
| `catalogDependencies` | [care_offering_catalog.go:73](../../../backend/modules/careplan/compose/care_offering_catalog.go#L73) | direct binding | none |
| `retainedRequestSubmissionRecordsAdapter.Create` | [compose_bindings_request_submissions.go:15](../../../backend/modules/careplan/compose/compose_bindings_request_submissions.go#L15) | direct binding | `r.CreateCareScheduleRequest` |
| `retainedRequestSubmissionRecordsAdapter.IsPendingConflict` | [compose_bindings_request_submissions.go:18](../../../backend/modules/careplan/compose/compose_bindings_request_submissions.go#L18) | direct binding | `postgres.IsPendingCareScheduleConflict` |
| `TenantHooks.TenantID` | [excused_requests.go:77](../../../backend/modules/careplan/compose/excused_requests.go#L77) | direct binding | `tenant.FromContext` |
| `TenantHooks.AfterCommit` | [excused_requests.go:78](../../../backend/modules/careplan/compose/excused_requests.go#L78) | direct binding | `tenant.RegisterAfterCommit` |
| `TenantHooks.MarkRollback` | [excused_requests.go:79](../../../backend/modules/careplan/compose/excused_requests.go#L79) | direct binding | `tenant.MarkRollback` |
| `engineRuntime.WithinTenant` | [new.go:60](../../../backend/modules/careplan/compose/new.go#L60) | direct binding | `postgres.EngineRuntimeWithinTenantBinding` |
| `engineRuntime.LockStudentAndExceptionDay` | [new.go:64](../../../backend/modules/careplan/compose/new.go#L64) | direct binding | `postgres.LockEngineStudentDay` |
| `engineRuntime.LockExceptionDay` | [new.go:68](../../../backend/modules/careplan/compose/new.go#L68) | direct binding | `careplanning.LockExceptionDay` |
| `engineRuntime.LockStudent` | [new.go:72](../../../backend/modules/careplan/compose/new.go#L72) | direct binding | `careplanning.LockStudent` |
| `tenantUnitOfWork.TenantID` | [offering_changes.go:105](../../../backend/modules/careplan/compose/offering_changes.go#L105) | direct binding | `tenant.FromContext` |
| `tenantUnitOfWork.RunInTenantTx` | [offering_changes.go:107](../../../backend/modules/careplan/compose/offering_changes.go#L107) | direct binding | `boundaryBinding.TenantUnitOfWorkRunInTenantTxBinding` |
| `tenantUnitOfWork.MarkRollback` | [offering_changes.go:111](../../../backend/modules/careplan/compose/offering_changes.go#L111) | direct binding | `tenant.MarkRollback` |
| `LockWeeklyPickupStudents` | [pickup_excusal.go:43](../../../backend/modules/careplan/compose/pickup_excusal.go#L43) | direct binding | `application.LockWeeklyPickupStudents` |
| `excusalTransaction.TenantID` | [pickup_excusal.go:59](../../../backend/modules/careplan/compose/pickup_excusal.go#L59) | direct binding | `tenant.FromContext` |
| `excusalTransaction.WithinTenant` | [pickup_excusal.go:61](../../../backend/modules/careplan/compose/pickup_excusal.go#L61) | direct binding | `boundaryBinding.ExcusalTransactionWithinTenantBinding` |
| `excusalTransaction.LockStudent` | [pickup_excusal.go:65](../../../backend/modules/careplan/compose/pickup_excusal.go#L65) | direct binding | `careplanning.LockStudent` |
| `excusalTransaction.LockStudentAndExceptionDay` | [pickup_excusal.go:69](../../../backend/modules/careplan/compose/pickup_excusal.go#L69) | direct binding | `boundaryBinding.LockExcusalStudentDay` |
| `excusalTransaction.IsNotFound` | [pickup_excusal.go:73](../../../backend/modules/careplan/compose/pickup_excusal.go#L73) | direct binding | `boundaryBinding.IsExcusalNotFound` |
| `pickupScheduleTransaction.LockStudentAndExceptionDay` | [pickup_schedules.go:30](../../../backend/modules/careplan/compose/pickup_schedules.go#L30) | direct binding | `careplanning.LockStudentAndExceptionDay` |
| `pickupScheduleTransaction.IsNotFound` | [pickup_schedules.go:34](../../../backend/modules/careplan/compose/pickup_schedules.go#L34) | direct binding | `compositionBinding.IsPickupScheduleNotFound` |
| `ParentPortalRepositories.WithSchools` | [repository_adapter_parent_portal_schools.go:5](../../../backend/modules/careplan/compose/repository_adapter_parent_portal_schools.go#L5) | direct binding | `r.ParentPortalRepositories.WithSchools` |


### `modules/careplan/requestfeed/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `tokenAdapter.New` | [new.go:52](../../../backend/modules/careplan/requestfeed/compose/new.go#L52) | direct binding | `a.newToken` |
| `tokenAdapter.Hash` | [new.go:53](../../../backend/modules/careplan/requestfeed/compose/new.go#L53) | direct binding | `a.hashToken` |


### `modules/classday/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `Rules.RowCareDay` | [rules_test_helpers.go:13](../../../backend/modules/classday/compose/rules_test_helpers.go#L13) | test-only | `application.Rules`, `.RowCareDay` |
| `Rules.EnrolledOn` | [rules_test_helpers.go:16](../../../backend/modules/classday/compose/rules_test_helpers.go#L16) | test-only | `application.Rules`, `.EnrolledOn` |


### `modules/communication/composition`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `transaction.RunAdmin` | [new.go:54](../../../backend/modules/communication/composition/new.go#L54) | direct binding | `tenant.WithinAdminRetry` |
| `AudienceMapping` | [new.go:58](../../../backend/modules/communication/composition/new.go#L58) | construction/binding | `domain.Announcement`, `domain.AnnouncementViewDetail` |
| `AnnouncementDatabase` | [new.go:72](../../../backend/modules/communication/composition/new.go#L72) | construction/binding | `postgres.NewAnnouncementDatabase` |
| `transaction.HasTransaction` | [new.go:76](../../../backend/modules/communication/composition/new.go#L76) | direct binding | `application.TransactionPresent` |
| `parentDatabase` | [parent_store.go:8](../../../backend/modules/communication/composition/parent_store.go#L8) | direct binding | `parentpostgres.NewCompositionParentDatabase` |
| `InstrumentMessageQueries` | [query_observer.go:11](../../../backend/modules/communication/composition/query_observer.go#L11) | direct binding | `communicationPostgres.InstrumentMessageQueries` |


### `modules/communication/parentstore`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `ParentAnnouncementFixtures.Create` | [announcement_fixtures_test_helpers.go:26](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L26) | test-only | `announcementFixtureToRow`, `f.repo.Create`, `announcementFixtureFromRow` |
| `ParentAnnouncementFixtures.FindByID` | [announcement_fixtures_test_helpers.go:35](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L35) | test-only | `f.repo.FindByID`, `announcementFixtureFromRow` |
| `ParentAnnouncementFixtures.Delete` | [announcement_fixtures_test_helpers.go:40](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L40) | test-only | `f.repo.Delete` |
| `ParentAnnouncementFixtures.SetPublished` | [announcement_fixtures_test_helpers.go:43](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L43) | test-only | `f.repo.SetPublished` |
| `ParentAnnouncementFixtures.ReplaceTargets` | [announcement_fixtures_test_helpers.go:47](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L47) | test-only | `announcementTargetFixtureRows`, `f.repo.ReplaceTargets`, `announcementTargetFixture` |
| `ParentAnnouncementFixtures.ReplaceOptions` | [announcement_fixtures_test_helpers.go:58](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L58) | test-only | `announcementOptionFixtureRows`, `f.repo.ReplaceOptions`, `announcementOptionFixture` |
| `ParentAnnouncementFixtures.ListOptions` | [announcement_fixtures_test_helpers.go:69](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L69) | test-only | `f.repo.ListOptions`, `make`, `len`, `announcementOptionFixture` |
| `ParentAnnouncementFixtures.InsertDeclarationVersion` | [announcement_fixtures_test_helpers.go:82](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L82) | test-only | `f.repo.InsertDeclarationVersion` |
| `ParentAnnouncementFixtures.ListDeclarationSubmissions` | [announcement_fixtures_test_helpers.go:90](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L90) | test-only | `f.repo.ListDeclarationSubmissions` |
| `announcementFixtureToRow` | [announcement_fixtures_test_helpers.go:95](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L95) | test-only | `announcementTargetFixtureRows`, `announcementOptionFixtureRows`, `communication.PortalAnnouncementDeclarationSettings` |
| `announcementFixtureFromRow` | [announcement_fixtures_test_helpers.go:124](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L124) | test-only | `communication.ParentDeclarationSettings`, `make`, `len`, `announcementTargetFixture`, `announcementOptionFixture` |
| `announcementTargetFixtureRows` | [announcement_fixtures_test_helpers.go:167](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L167) | test-only | `make`, `len` |
| `announcementTargetFixture` | [announcement_fixtures_test_helpers.go:189](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L189) | test-only | none |
| `announcementOptionFixtureRows` | [announcement_fixtures_test_helpers.go:203](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L203) | test-only | `make`, `len` |
| `announcementOptionFixture` | [announcement_fixtures_test_helpers.go:224](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L224) | test-only | none |
| `ParentAnnouncementPublishedError` | [announcement_fixtures_test_helpers.go:238](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L238) | test-only | none |
| `postgresDatabase` | [parentstore.go:17](../../../backend/modules/communication/parentstore/parentstore.go#L17) | direct binding | `parentpostgres.NewParentRecordDatabase` |
| `ProjectionDatabase` | [parentstore.go:20](../../../backend/modules/communication/parentstore/parentstore.go#L20) | direct binding | `parentpostgres.NewProjectionDatabase` |
| `MessagingRepositories.WithSchoolNames` | [school_projection.go:21](../../../backend/modules/communication/parentstore/school_projection.go#L21) | construction/binding | `WithAnnouncementSchoolNames`, `WithInboxSchoolNames` |
| `WithInboxSchoolNames` | [school_projection.go:29](../../../backend/modules/communication/parentstore/school_projection.go#L29) | direct binding | `parentpostgres.WithInboxSchoolNames` |
| `WithAnnouncementSchoolNames` | [school_projection.go:32](../../../backend/modules/communication/parentstore/school_projection.go#L32) | direct binding | `parentpostgres.WithAnnouncementSchoolNames` |
| `WithInboxGroupNames` | [staff_projection.go:14](../../../backend/modules/communication/parentstore/staff_projection.go#L14) | direct binding | `parentpostgres.WithInboxGroupNames` |
| `CurrentTenantStaffAccounts` | [staff_projection.go:17](../../../backend/modules/communication/parentstore/staff_projection.go#L17) | direct binding | `parentpostgres.CurrentTenantStaffAccounts` |


### `modules/communication/staffstore`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `postgresDatabase` | [staffstore.go:11](../../../backend/modules/communication/staffstore/staffstore.go#L11) | direct binding | `staffpostgres.NewAmbientDatabase` |
| `ProjectionDatabase` | [staffstore.go:12](../../../backend/modules/communication/staffstore/staffstore.go#L12) | direct binding | `staffpostgres.NewAmbientDatabase` |


### `modules/dataimport/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `schoolRolePolicy.Validate` | [school_role.go:16](../../../backend/modules/dataimport/compose/school_role.go#L16) | direct binding | `identityaccess.ValidateAssignableSchoolRole`, `assignmentFacts` |
| `schoolRolePolicy.NeedsCaregiver` | [school_role.go:20](../../../backend/modules/dataimport/compose/school_role.go#L20) | direct binding | `identityaccess.RoleNeedsCaregiverProfile`, `assignmentFacts` |
| `assignmentFacts` | [school_role.go:26](../../../backend/modules/dataimport/compose/school_role.go#L26) | direct binding | `binding.AssignmentFacts` |
| `transactions.TenantID` | [transactions.go:16](../../../backend/modules/dataimport/compose/transactions.go#L16) | direct binding | `tenant.FromContext` |
| `transactions.HasTransaction` | [transactions.go:18](../../../backend/modules/dataimport/compose/transactions.go#L18) | direct binding | `app.ImportTransactionPresent` |
| `transactions.WithinCurrent` | [transactions.go:22](../../../backend/modules/dataimport/compose/transactions.go#L22) | direct binding | `tenant.WithinCurrentTenant` |
| `transactions.WithinRetry` | [transactions.go:26](../../../backend/modules/dataimport/compose/transactions.go#L26) | direct binding | `app.RetryImportTransaction` |
| `transactions.Savepoint` | [transactions.go:30](../../../backend/modules/dataimport/compose/transactions.go#L30) | direct binding | `tenant.WithSavepoint` |
| `transactions.AcquireLock` | [transactions.go:34](../../../backend/modules/dataimport/compose/transactions.go#L34) | direct binding | `tenant.AcquireLock` |
| `transactions.Observe` | [transactions.go:38](../../../backend/modules/dataimport/compose/transactions.go#L38) | direct binding | `tenant.WithAdditionalUnitOfWorkObserver`, `app.ObserveImportTransaction` |


### `modules/dataimport/compose/imports`

No non-constructor functions.


### `modules/delivery/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `DrainEmailForTest` | [intent_worker_test_helpers.go:11](../../../backend/modules/delivery/compose/intent_worker_test_helpers.go#L11) | test-only | `application.DrainEmailForTest` |
| `DeleteSystemEmailForTest` | [intent_worker_test_helpers.go:14](../../../backend/modules/delivery/compose/intent_worker_test_helpers.go#L14) | test-only | `application.DeleteSystemEmailForTest` |
| `IsLoggingMailTransport` | [mail_transport.go:15](../../../backend/modules/delivery/compose/mail_transport.go#L15) | direct binding | `email.IsLoggingMailTransport` |
| `*Runtime.EmailStatus` | [new.go:34](../../../backend/modules/delivery/compose/new.go#L34) | direct binding | `.EmailStatus` |
| `*MailPushProvider.SendEmail` | [provider.go:30](../../../backend/modules/delivery/compose/provider.go#L30) | direct binding | `.SendEmail` |
| `*MailPushProvider.SendPush` | [provider.go:33](../../../backend/modules/delivery/compose/provider.go#L33) | direct binding | `.SendPush` |
| `QueueStaffTimeTrackingChanged` | [realtime.go:22](../../../backend/modules/delivery/compose/realtime.go#L22) | direct binding | `realtimeevents.QueueStaffTimeTrackingChanged` |
| `QueueInstanceStarted` | [realtime.go:32](../../../backend/modules/delivery/compose/realtime.go#L32) | direct binding | `realtimeevents.QueueInstanceStarted` |
| `QueueArrivalScheduleChanged` | [realtime.go:39](../../../backend/modules/delivery/compose/realtime.go#L39) | direct binding | `realtimeevents.QueueArrivalScheduleChanged` |


### `modules/devicefleet/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `transaction.RunWrite` | [new.go:128](../../../backend/modules/devicefleet/compose/new.go#L128) | direct binding | `postgres.TransactionRunWriteBinding` |
| `transaction.AdminScope` | [new.go:132](../../../backend/modules/devicefleet/compose/new.go#L132) | direct binding | `postgres.TransactionAdminScopeBinding` |
| `transaction.TenantScope` | [new.go:136](../../../backend/modules/devicefleet/compose/new.go#L136) | direct binding | `postgres.TransactionTenantScopeBinding` |


### `modules/devicefleet/compose/repositoryadapter`

No non-constructor functions.


### `modules/devicescan/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `AttendanceTenantID` | [attendance_principal.go:11](../../../backend/modules/devicescan/compose/attendance_principal.go#L11) | direct binding | `tenant.FromContext` |
| `MarkAttendanceRollback` | [attendance_principal.go:15](../../../backend/modules/devicescan/compose/attendance_principal.go#L15) | direct binding | `tenant.MarkRollback` |
| `clock.Now` | [compose.go:129](../../../backend/modules/devicescan/compose/compose.go#L129) | direct binding | `c.now` |
| `clock.Day` | [compose.go:130](../../../backend/modules/devicescan/compose/compose.go#L130) | direct binding | `calendar.DateFromTime` |
| `unitOfWork.RequireTransaction` | [compose.go:135](../../../backend/modules/devicescan/compose/compose.go#L135) | direct binding | `application.RequireTransaction` |
| `unitOfWork.MarkRollback` | [compose.go:139](../../../backend/modules/devicescan/compose/compose.go#L139) | direct binding | `tenant.MarkRollback` |
| `directoryPeople.Teachers` | [directory.go:32](../../../backend/modules/devicescan/compose/directory.go#L32) | direct binding | `application.TeachersBinding` |
| `feedbackStudents.LockFeedbackStudent` | [feedback_students.go:23](../../../backend/modules/devicescan/compose/feedback_students.go#L23) | direct binding | `application.LockFeedbackStudentBinding` |
| `people.FindPersonByTag` | [people.go:29](../../../backend/modules/devicescan/compose/people.go#L29) | direct binding | `application.FindPersonByTagBinding` |
| `people.FindStudentByPerson` | [people.go:33](../../../backend/modules/devicescan/compose/people.go#L33) | direct binding | `application.FindStudentByPersonBinding` |
| `people.FindStaffByPerson` | [people.go:37](../../../backend/modules/devicescan/compose/people.go#L37) | direct binding | `application.FindStaffByPersonBinding` |
| `lifecyclePeople.StaffNames` | [session_lifecycle.go:31](../../../backend/modules/devicescan/compose/session_lifecycle.go#L31) | direct binding | `application.StaffNamesBinding` |
| `SupervisionEnded` | [session_lifecycle.go:35](../../../backend/modules/devicescan/compose/session_lifecycle.go#L35) | direct binding | `application.SupervisionEnded` |
| `LifecycleSupervisorsBinding` | [session_lifecycle.go:39](../../../backend/modules/devicescan/compose/session_lifecycle.go#L39) | direct binding | `application.LifecycleSupervisorsBinding` |
| `settings.PresenceMode` | [settings.go:25](../../../backend/modules/devicescan/compose/settings.go#L25) | direct binding | `s.active.GetPresenceMode` |
| `settings.FeedbackEnabled` | [settings.go:29](../../../backend/modules/devicescan/compose/settings.go#L29) | direct binding | `application.FeedbackEnabledBinding` |
| `settings.CapacityDetailsDisclosed` | [settings.go:33](../../../backend/modules/devicescan/compose/settings.go#L33) | direct binding | `application.CapacityDetailsDisclosedBinding` |
| `settings.DailyCheckoutTime` | [settings.go:38](../../../backend/modules/devicescan/compose/settings.go#L38) | direct binding | `application.DailyCheckoutTimeBinding` |
| `settings.PerStudentCheckoutEnabled` | [settings.go:42](../../../backend/modules/devicescan/compose/settings.go#L42) | direct binding | `application.PerStudentCheckoutEnabledBinding` |
| `settings.PerStudentCheckoutDeltaMinutes` | [settings.go:46](../../../backend/modules/devicescan/compose/settings.go#L46) | direct binding | `application.PerStudentCheckoutDeltaMinutesBinding` |
| `settings.DailyCheckoutFromAllRoomsEnabled` | [settings.go:50](../../../backend/modules/devicescan/compose/settings.go#L50) | direct binding | `application.DailyCheckoutFromAllRoomsEnabledBinding` |
| `tagPeople.FindStaff` | [tag_assignment.go:26](../../../backend/modules/devicescan/compose/tag_assignment.go#L26) | direct binding | `application.FindStaffBinding` |
| `tagPeople.FindPerson` | [tag_assignment.go:30](../../../backend/modules/devicescan/compose/tag_assignment.go#L30) | direct binding | `application.FindPersonBinding` |
| `tagPeople.LinkTag` | [tag_assignment.go:34](../../../backend/modules/devicescan/compose/tag_assignment.go#L34) | direct binding | `p.LinkToRFIDCard` |
| `tagPeople.UnlinkTag` | [tag_assignment.go:38](../../../backend/modules/devicescan/compose/tag_assignment.go#L38) | direct binding | `p.UnlinkFromRFIDCard` |
| `tagPeople.TeacherRole` | [tag_assignment.go:44](../../../backend/modules/devicescan/compose/tag_assignment.go#L44) | direct binding | `application.TeacherRoleBinding` |


### `modules/emergencysnapshot/compose`

No non-constructor functions.


### `modules/enrollment/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `tenantRuntime` | [application.go:144](../../../backend/modules/enrollment/compose/application.go#L144) | construction/binding | `tenant.NewTransactionRunner`, `application.NewErrorMatcher`, `application.NewTransactionPresence`, `tenant.WithAdminTx`, `fn`, `tenant.WithTenantTx`, `application.NewDetachedContext` |
| `PublicDecisions` | [intake.go:74](../../../backend/modules/enrollment/compose/intake.go#L74) | direct binding | `application.PublicDecisions` |


### `modules/exporttransfer/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `retainedNewDetachedContext` | [compose_bindings_callback_detached_context.go:9](../../../backend/modules/exporttransfer/compose/compose_bindings_callback_detached_context.go#L9) | direct binding | `tenant.ContextWithoutAfterCommitHooks`, `tenant.ContextWithoutTransaction` |
| `retainedDatabase` | [compose_bindings_new.go:17](../../../backend/modules/exporttransfer/compose/compose_bindings_new.go#L17) | direct binding | `postgres.NewScopedDatabase` |
| `retainedUploader.Prepare` | [compose_bindings_new.go:23](../../../backend/modules/exporttransfer/compose/compose_bindings_new.go#L23) | direct binding | `sftp.PreparePortUpload`, `sftp.Target` |
| `retainedEngine.Status` | [compose_bindings_new.go:29](../../../backend/modules/exporttransfer/compose/compose_bindings_new.go#L29) | direct binding | `application.StatusBinding` |
| `retainedEngine.Transfer` | [compose_bindings_new.go:32](../../../backend/modules/exporttransfer/compose/compose_bindings_new.go#L32) | direct binding | `application.TransferBinding`, `domain.Request` |
| `Settings.complete` | [new.go:31](../../../backend/modules/exporttransfer/compose/new.go#L31) | direct binding | none |


### `modules/facilities/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `databaseRuntime` | [composition_database_test_helpers.go:10](../../../backend/modules/facilities/compose/composition_database_test_helpers.go#L10) | test-only | `postgres.DatabaseRuntime` |
| `transaction.RunWrite` | [new.go:55](../../../backend/modules/facilities/compose/new.go#L55) | direct binding | `postgres.TransactionRunWriteBinding` |
| `transaction.AcquireLock` | [new.go:59](../../../backend/modules/facilities/compose/new.go#L59) | direct binding | `tenant.AcquireLock` |
| `OccupancyProjection` | [projections.go:21](../../../backend/modules/facilities/compose/projections.go#L21) | direct binding | `facilitiesService.NewOccupancyProjection` |
| `HistoryProjection` | [projections.go:24](../../../backend/modules/facilities/compose/projections.go#L24) | direct binding | `facilitiesService.NewHistoryProjection` |
| `SettingHandlers` | [projections.go:27](../../../backend/modules/facilities/compose/projections.go#L27) | direct binding | `facilitiesService.SettingHandlers` |
| `FindRoomNames` | [room_rows.go:10](../../../backend/modules/facilities/compose/room_rows.go#L10) | direct binding | `facilitiesService.FindRoomNames` |


### `modules/facilities/compose/repositoryadapter`

No non-constructor functions.


### `modules/feedback/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `TransactionFunc.Run` | [new.go:17](../../../backend/modules/feedback/compose/new.go#L17) | direct binding | `fn` |
| `*Settings.FeedbackEnabled` | [new.go:59](../../../backend/modules/feedback/compose/new.go#L59) | direct binding | `s.enabled` |
| `*Settings.FeedbackRetentionDays` | [new.go:61](../../../backend/modules/feedback/compose/new.go#L61) | direct binding | `s.retention` |


### `modules/filestorage/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `PersistenceDatabaseForTest` | [ambient_scope_test_helpers.go:12](../../../backend/modules/filestorage/compose/ambient_scope_test_helpers.go#L12) | test-only | `postgresNative.DatabaseRuntime` |
| `transaction.TenantID` | [new.go:169](../../../backend/modules/filestorage/compose/new.go#L169) | direct binding | `tenant.FromContext` |
| `transaction.RunWrite` | [new.go:171](../../../backend/modules/filestorage/compose/new.go#L171) | direct binding | `postgres.TransactionRunWriteBinding` |
| `transaction.RunInTenant` | [new.go:175](../../../backend/modules/filestorage/compose/new.go#L175) | direct binding | `postgres.TransactionRunInTenantBinding` |
| `transaction.AcquireLock` | [new.go:179](../../../backend/modules/filestorage/compose/new.go#L179) | direct binding | `tenant.AcquireLock` |
| `transaction.AfterCommit` | [new.go:183](../../../backend/modules/filestorage/compose/new.go#L183) | direct binding | `tenant.RegisterAfterCommit` |
| `transaction.Detach` | [new.go:187](../../../backend/modules/filestorage/compose/new.go#L187) | direct binding | `postgres.DetachedContext` |


### `modules/grouplive/compose`

No non-constructor functions.


### `modules/guardianlinkview/compose`

No non-constructor functions.


### `modules/identityaccess/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `tenantRuntime.WithAdminTx` | [account_authentication.go:57](../../../backend/modules/identityaccess/compose/account_authentication.go#L57) | direct binding | `boundaryBinding.TenantRuntimeWithAdminTxBinding` |
| `tenantRuntime.WithTenantTx` | [account_authentication.go:61](../../../backend/modules/identityaccess/compose/account_authentication.go#L61) | direct binding | `boundaryBinding.TenantRuntimeWithTenantTxBinding` |
| `tenantRuntime.RunInTx` | [account_authentication.go:68](../../../backend/modules/identityaccess/compose/account_authentication.go#L68) | direct binding | `boundaryBinding.TenantRuntimeRunInTxBinding` |
| `tenantRuntime.IsAdminTx` | [account_authentication.go:72](../../../backend/modules/identityaccess/compose/account_authentication.go#L72) | direct binding | `tenant.IsAdminTx` |
| `tenantRuntime.HasTransaction` | [account_authentication.go:74](../../../backend/modules/identityaccess/compose/account_authentication.go#L74) | direct binding | `boundaryBinding.HasTenantTransaction` |
| `tenantRuntime.HasAfterCommitHooks` | [account_authentication.go:78](../../../backend/modules/identityaccess/compose/account_authentication.go#L78) | direct binding | `tenant.HasAfterCommitHooks` |
| `tenantRuntime.RegisterAfterCommit` | [account_authentication.go:82](../../../backend/modules/identityaccess/compose/account_authentication.go#L82) | direct binding | `tenant.RegisterAfterCommit` |
| `tenantRuntime.TenantID` | [account_authentication.go:86](../../../backend/modules/identityaccess/compose/account_authentication.go#L86) | direct binding | `tenant.FromContext` |
| `tenantRuntime.Scope` | [account_authentication.go:88](../../../backend/modules/identityaccess/compose/account_authentication.go#L88) | direct binding | `tenant.ScopeFromContext` |
| `tenantRuntime.OrgID` | [account_authentication.go:90](../../../backend/modules/identityaccess/compose/account_authentication.go#L90) | direct binding | `tenant.OrgFromContext` |
| `tenantRuntime.WithTenantID` | [account_authentication.go:92](../../../backend/modules/identityaccess/compose/account_authentication.go#L92) | direct binding | `tenant.WithTenantID` |
| `tenantRuntime.Detach` | [account_authentication.go:96](../../../backend/modules/identityaccess/compose/account_authentication.go#L96) | direct binding | `boundaryBinding.DetachCallerTransaction` |
| `tenantRuntime.WithoutTransaction` | [account_authentication.go:100](../../../backend/modules/identityaccess/compose/account_authentication.go#L100) | direct binding | `tenant.ContextWithoutTransaction` |
| `tenantRuntime.AcquireLock` | [account_authentication.go:104](../../../backend/modules/identityaccess/compose/account_authentication.go#L104) | direct binding | `tenant.AcquireLock` |
| `retainedGuardianStudentAccess.GrantGuardianStudentAccess` | [compose_bindings_guardian_student_access.go:18](../../../backend/modules/identityaccess/compose/compose_bindings_guardian_student_access.go#L18) | direct binding | `application.ObserveGuardianAccessGrant`, `postgres.GuardianStudentAccessRecord` |
| `retainedGuardianStudentAccess.SetGuardianStudentPermissions` | [compose_bindings_guardian_student_access.go:21](../../../backend/modules/identityaccess/compose/compose_bindings_guardian_student_access.go#L21) | direct binding | `application.ObserveGuardianPermissionChange` |
| `composeDemoAccess` | [demo_access.go:33](../../../backend/modules/identityaccess/compose/demo_access.go#L33) | direct binding | `application.NewDemoAccessBinding`, `postgres.NewStore` |
| `demoAccessExpiryEngine.ExpireDemoAccesses` | [demo_access.go:47](../../../backend/modules/identityaccess/compose/demo_access.go#L47) | direct binding | `postgres.ExpireDemoAccessesBinding` |
| `RunMailStepInSavepoint` | [mail_transactions.go:25](../../../backend/modules/identityaccess/compose/mail_transactions.go#L25) | direct binding | `applicationBindings.RunMailStepInSavepoint` |
| `RunMailStep` | [mail_transactions.go:29](../../../backend/modules/identityaccess/compose/mail_transactions.go#L29) | direct binding | `applicationBindings.RunMailStep` |
| `AccountExists` | [maintenance.go:96](../../../backend/modules/identityaccess/compose/maintenance.go#L96) | direct binding | `application.AccountExists` |
| `MaskEmail` | [mask_email.go:8](../../../backend/modules/identityaccess/compose/mask_email.go#L8) | direct binding | `domain.MaskEmail` |
| `withPasskeyFlows` | [mfa.go:81](../../../backend/modules/identityaccess/compose/mfa.go#L81) | direct binding | `application.NewPasskeyFlowsBinding`, `mfaRuntime` |
| `mfaRuntime` | [mfa.go:94](../../../backend/modules/identityaccess/compose/mfa.go#L94) | direct binding | `application.NewTransactionRunner` |
| `TenantUnitOfWork.WithinCurrentTenant` | [tenant_unit_of_work.go:16](../../../backend/modules/identityaccess/compose/tenant_unit_of_work.go#L16) | direct binding | `tenant.WithinCurrentTenant` |
| `TenantUnitOfWork.WithinAdmin` | [tenant_unit_of_work.go:21](../../../backend/modules/identityaccess/compose/tenant_unit_of_work.go#L21) | direct binding | `tenant.WithinAdmin` |
| `TenantUnitOfWork.MarkRollback` | [tenant_unit_of_work.go:27](../../../backend/modules/identityaccess/compose/tenant_unit_of_work.go#L27) | direct binding | `tenant.MarkRollback` |
| `TenantUnitOfWork.WithTenantID` | [tenant_unit_of_work.go:32](../../../backend/modules/identityaccess/compose/tenant_unit_of_work.go#L32) | direct binding | `tenant.WithTenantID` |
| `TenantUnitOfWork.TenantID` | [tenant_unit_of_work.go:37](../../../backend/modules/identityaccess/compose/tenant_unit_of_work.go#L37) | direct binding | `tenant.FromContext` |


### `modules/mealplan/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `TransactionFunc.Run` | [new.go:18](../../../backend/modules/mealplan/compose/new.go#L18) | direct binding | `fn` |
| `*Settings.MealPlanEnabled` | [new.go:61](../../../backend/modules/mealplan/compose/new.go#L61) | direct binding | `s.mealPlanEnabled` |
| `*Settings.MealRegistrationEnabled` | [new.go:63](../../../backend/modules/mealplan/compose/new.go#L63) | direct binding | `s.mealRegistrationEnabled` |
| `*Settings.MealRegistrationCutoff` | [new.go:67](../../../backend/modules/mealplan/compose/new.go#L67) | direct binding | `s.mealRegistrationCutoff` |


### `modules/operatordashboard`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `Map` | [new.go:14](../../../backend/modules/operatordashboard/new.go#L14) | direct binding | `postgres.Map` |


### `modules/organizationtenancy/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `retainedChildQuotaLimits.ChildQuotaLimit` | [compose_bindings_child_quota.go:12](../../../backend/modules/organizationtenancy/compose/compose_bindings_child_quota.go#L12) | direct binding | `postgres.ReadComposedChildQuota` |
| `transaction.RunAdmin` | [new.go:33](../../../backend/modules/organizationtenancy/compose/new.go#L33) | direct binding | `postgres.TransactionRunAdminBinding` |
| `transaction.RunTenant` | [new.go:37](../../../backend/modules/organizationtenancy/compose/new.go#L37) | direct binding | `postgres.TransactionRunTenantBinding` |
| `transaction.RunRead` | [new.go:41](../../../backend/modules/organizationtenancy/compose/new.go#L41) | direct binding | `postgres.TransactionRunReadBinding` |
| `OperatorDatabase` | [provisioning.go:87](../../../backend/modules/organizationtenancy/compose/provisioning.go#L87) | direct binding | `postgresNative.OperatorDatabase` |


### `modules/parentannouncementaudience`

No non-constructor functions.


### `modules/parentmessageinbox`

No non-constructor functions.


### `modules/parentrequestfeedprojection/compose`

No non-constructor functions.


### `modules/peopledirectory/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `CaregiverCapabilityViews.GetSchoolAccountCaregiverCapability` | [caregiver_capability_views.go:25](../../../backend/modules/peopledirectory/compose/caregiver_capability_views.go#L25) | direct binding | `application.GetSchoolAccountCaregiverCapability` |
| `CaregiverCapabilityViews.EnableSchoolAccountCaregiverCapability` | [caregiver_capability_views.go:31](../../../backend/modules/peopledirectory/compose/caregiver_capability_views.go#L31) | direct binding | `application.EnableSchoolAccountCaregiverCapability` |
| `CaregiverCapabilityViews.DisableSchoolAccountCaregiverCapability` | [caregiver_capability_views.go:37](../../../backend/modules/peopledirectory/compose/caregiver_capability_views.go#L37) | direct binding | `application.DisableSchoolAccountCaregiverCapability` |
| `CaregiverCapabilityViews.inSchool` | [caregiver_capability_views.go:41](../../../backend/modules/peopledirectory/compose/caregiver_capability_views.go#L41) | direct binding | `boundaryBinding.CaregiverCapabilityViewsInSchoolBinding` |
| `studentPhotoPort` | [compose_binding_test_helpers.go:12](../../../backend/modules/peopledirectory/compose/compose_binding_test_helpers.go#L12) | test-only | `application.StudentPhotoPort` |
| `studentPhotoRuntimeUnboundForTest` | [compose_binding_test_helpers.go:15](../../../backend/modules/peopledirectory/compose/compose_binding_test_helpers.go#L15) | test-only | none |
| `ApplyDefaultGuardianRole` | [compose_binding_test_helpers.go:20](../../../backend/modules/peopledirectory/compose/compose_binding_test_helpers.go#L20) | test-only | `application.ApplyDefaultGuardianRole` |
| `*GuardianProfileLoader.LoadForTenant` | [guardian_profile_loader.go:38](../../../backend/modules/peopledirectory/compose/guardian_profile_loader.go#L38) | direct binding | `boundaryBinding.GuardianProfileLoaderLoadForTenantBinding` |
| `guardianTransactions.RunInTx` | [guardian_service.go:23](../../../backend/modules/peopledirectory/compose/guardian_service.go#L23) | direct binding | `boundaryBinding.GuardianTransactionsRunInTxBinding`, `tenant.NewTransactionRunner` |
| `guardianTransactions.WithTenantTx` | [guardian_service.go:27](../../../backend/modules/peopledirectory/compose/guardian_service.go#L27) | direct binding | `boundaryBinding.GuardianTransactionsWithTenantTxBinding` |
| `guardianTransactions.TenantID` | [guardian_service.go:31](../../../backend/modules/peopledirectory/compose/guardian_service.go#L31) | direct binding | `tenant.FromContext` |
| `transaction.RequireTransaction` | [new.go:154](../../../backend/modules/peopledirectory/compose/new.go#L154) | direct binding | `tenant.RequireTransaction` |
| `transaction.TenantID` | [new.go:159](../../../backend/modules/peopledirectory/compose/new.go#L159) | direct binding | `tenant.FromContext` |
| `transaction.RegisterAfterCommit` | [new.go:163](../../../backend/modules/peopledirectory/compose/new.go#L163) | direct binding | `tenant.RegisterAfterCommit` |
| `transaction.RunWrite` | [new.go:167](../../../backend/modules/peopledirectory/compose/new.go#L167) | direct binding | `postgres.TransactionRunWriteBinding`, `tenant.NewTransactionRunner` |
| `transaction.RunRead` | [new.go:171](../../../backend/modules/peopledirectory/compose/new.go#L171) | direct binding | `postgres.TransactionRunReadBinding`, `tenant.NewTransactionRunner` |
| `transaction.RunSavepoint` | [new.go:175](../../../backend/modules/peopledirectory/compose/new.go#L175) | direct binding | `tenant.WithSavepoint` |
| `transaction.RunAdminRead` | [new.go:182](../../../backend/modules/peopledirectory/compose/new.go#L182) | direct binding | `postgres.TransactionRunAdminReadBinding` |
| `OperatorPersonStudentMembership` | [operator_person_students.go:12](../../../backend/modules/peopledirectory/compose/operator_person_students.go#L12) | direct binding | `usersRepo.FindOperatorPersonStudentMembership` |
| `ParseRetainedCalendarDate` | [repository_adapter_record_errors.go:10](../../../backend/modules/peopledirectory/compose/repository_adapter_record_errors.go#L10) | direct binding | `usersRepo.ParseCalendarDate` |
| `CareStudentLock` | [repository_adapter_student_lock_test_helpers.go:14](../../../backend/modules/peopledirectory/compose/repository_adapter_student_lock_test_helpers.go#L14) | test-only | none |
| `RetainedWithPortalMemberships` | [retained_repository_suites_test_helpers.go:62](../../../backend/modules/peopledirectory/compose/retained_repository_suites_test_helpers.go#L62) | test-only | `usersRepo.WithPortalMemberships` |
| `RetainedWithGuardianRelationshipMemberships` | [retained_repository_suites_test_helpers.go:68](../../../backend/modules/peopledirectory/compose/retained_repository_suites_test_helpers.go#L68) | test-only | `usersRepo.WithGuardianRelationshipMemberships` |
| `RetainedWithGuardianRelationshipOwners` | [retained_repository_suites_test_helpers.go:74](../../../backend/modules/peopledirectory/compose/retained_repository_suites_test_helpers.go#L74) | test-only | `usersRepo.WithGuardianRelationshipOwners` |
| `RetainedWithGuardianDefaultRole` | [retained_repository_suites_test_helpers.go:80](../../../backend/modules/peopledirectory/compose/retained_repository_suites_test_helpers.go#L80) | test-only | `usersRepo.WithGuardianDefaultRole` |
| `DisabledStudentPhotos` | [retained_repository_suites_test_helpers.go:87](../../../backend/modules/peopledirectory/compose/retained_repository_suites_test_helpers.go#L87) | test-only | none |
| `VerifyRetainedStudentSchema` | [student_schema.go:12](../../../backend/modules/peopledirectory/compose/student_schema.go#L12) | direct binding | `usersRepo.VerifyStudentSchema` |


### `modules/presenceprojection/compose`

No non-constructor functions.


### `modules/requestreview/compose`

No non-constructor functions.


### `modules/schedulerruntime/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `SettingKeys` | [compose.go:21](../../../backend/modules/schedulerruntime/compose/compose.go#L21) | direct binding | `scheduler.SettingKeys` |


### `modules/schoolcalendar/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `PersistenceRuntimeFor` | [new.go:28](../../../backend/modules/schoolcalendar/compose/new.go#L28) | construction/binding | `panic`, `postgres.NewPersistenceDatabase` |
| `writeTransaction` | [new.go:101](../../../backend/modules/schoolcalendar/compose/new.go#L101) | direct binding | `postgres.WriteTransactionBinding` |


### `modules/schoolcalendar/portal/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `*TenantRuntime.TenantID` | [tenant_runtime.go:24](../../../backend/modules/schoolcalendar/portal/compose/tenant_runtime.go#L24) | direct binding | `tenant.FromContext` |
| `*TenantRuntime.WithinAppointmentWrite` | [tenant_runtime.go:28](../../../backend/modules/schoolcalendar/portal/compose/tenant_runtime.go#L28) | direct binding | `runtimeBinding.PortalAppointmentWrite` |
| `*TenantRuntime.WithinTenant` | [tenant_runtime.go:33](../../../backend/modules/schoolcalendar/portal/compose/tenant_runtime.go#L33) | direct binding | `runtimeBinding.PortalTenantWrite` |
| `*TenantRuntime.WithinAdmin` | [tenant_runtime.go:38](../../../backend/modules/schoolcalendar/portal/compose/tenant_runtime.go#L38) | direct binding | `runtimeBinding.PortalAdminWrite` |
| `*TenantRuntime.WithoutTransactionAndHooks` | [tenant_runtime.go:44](../../../backend/modules/schoolcalendar/portal/compose/tenant_runtime.go#L44) | direct binding | `runtimeBinding.PortalDetachedContext` |


### `modules/schoolmembership/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `StudentClassWriteGateQuery` | [student_class_gate.go:14](../../../backend/modules/schoolmembership/compose/student_class_gate.go#L14) | direct binding | `postgres.ResolveStudentClassWriteGate` |
| `StudentClassWriteGate` | [student_class_gate.go:20](../../../backend/modules/schoolmembership/compose/student_class_gate.go#L20) | direct binding | `StudentClassWriteGateQuery` |


### `modules/schoolsetup/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `ambientTx` | [compose.go:48](../../../backend/modules/schoolsetup/compose/compose.go#L48) | construction/binding | `schoolsetupview.AmbientTx`, `tenant.TransactionFromContext` |
| `AmbientRuntime.DB` | [compose.go:56](../../../backend/modules/schoolsetup/compose/compose.go#L56) | construction/binding | `ambientTx` |


### `modules/schoolstructure/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `legacyTeacher` | [group_values_test_helpers.go:13](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L13) | test-only | `groups.TeacherPerson` |
| `legacyGroupError` | [group_values_test_helpers.go:25](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L25) | test-only | `retainedGroupError` |
| `retainedGroupError` | [group_values_test_helpers.go:29](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L29) | test-only | `err.`, `legacyGroupError`, `groups.MapGroupErrorCauses` |
| `legacyGroupQuery` | [group_values_test_helpers.go:74](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L74) | test-only | `schoolstructure.GroupListQuery` |
| `LegacyGroupManagement` | [group_values_test_helpers.go:90](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L90) | test-only | none |
| `*legacyGroupManagement.GetGroup` | [group_values_test_helpers.go:94](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L94) | test-only | `s.capability.GetGroup`, `groups.LegacyGroup`, `legacyGroupError` |
| `*legacyGroupManagement.FindGroupWithRoom` | [group_values_test_helpers.go:98](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L98) | test-only | `s.capability.FindGroupWithRoom`, `groups.LegacyGroup`, `legacyGroupError` |
| `*legacyGroupManagement.GetGroupsByIDs` | [group_values_test_helpers.go:102](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L102) | test-only | `s.capability.GetGroupsByIDs`, `legacyGroupError`, `make`, `len`, `groups.LegacyGroup` |
| `*legacyGroupManagement.GetGroupsWithRoomsByIDs` | [group_values_test_helpers.go:113](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L113) | test-only | `s.capability.GetGroupsWithRoomsByIDs`, `legacyGroupError`, `make`, `len`, `groups.LegacyGroup` |
| `*legacyGroupManagement.CreateGroup` | [group_values_test_helpers.go:124](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L124) | test-only | `groups.PublicGroup`, `s.capability.CreateGroup`, `groups.LegacyGroup`, `legacyGroupError` |
| `*legacyGroupManagement.UpdateGroup` | [group_values_test_helpers.go:132](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L132) | test-only | `groups.PublicGroup`, `s.capability.UpdateGroup`, `groups.LegacyGroup`, `legacyGroupError` |
| `*legacyGroupManagement.DeleteGroup` | [group_values_test_helpers.go:140](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L140) | test-only | `legacyGroupError`, `s.capability.DeleteGroup` |
| `*legacyGroupManagement.ListGroups` | [group_values_test_helpers.go:143](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L143) | test-only | `s.capability.ListGroups`, `legacyGroupQuery`, `legacyGroupError`, `make`, `len`, `groups.LegacyGroup` |
| `*legacyGroupManagement.CountGroups` | [group_values_test_helpers.go:154](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L154) | test-only | `s.capability.CountGroups`, `legacyGroupQuery`, `legacyGroupError` |
| `*legacyGroupManagement.GetTeacherGroups` | [group_values_test_helpers.go:158](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L158) | test-only | `s.capability.GetTeacherGroups`, `legacyGroupError`, `make`, `len`, `groups.LegacyGroup` |
| `*legacyGroupManagement.GetGroupTeachers` | [group_values_test_helpers.go:169](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L169) | test-only | `s.capability.GetGroupTeachers`, `legacyGroupError`, `make`, `len`, `legacyTeacher` |
| `*legacyGroupManagement.GetTeachersForGroups` | [group_values_test_helpers.go:180](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L180) | test-only | `s.capability.GetTeachersForGroups`, `legacyGroupError`, `make`, `len`, `legacyTeacher` |
| `*legacyGroupManagement.RemoveTeacherFromGroup` | [group_values_test_helpers.go:199](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L199) | test-only | `legacyGroupError`, `s.capability.RemoveTeacherFromGroup` |
| `*legacyGroupManagement.UpdateGroupTeachers` | [group_values_test_helpers.go:202](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L202) | test-only | `legacyGroupError`, `s.capability.UpdateGroupTeachers` |
| `*legacyGroupManagement.GetStaffSchoolClasses` | [group_values_test_helpers.go:205](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L205) | test-only | `s.capability.GetStaffSchoolClasses`, `legacyGroupError` |
| `*legacyGroupManagement.SetStaffSchoolClasses` | [group_values_test_helpers.go:209](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L209) | test-only | `legacyGroupError`, `s.capability.SetStaffSchoolClasses` |
| `publicGroupError` | [group_values_test_helpers.go:242](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L242) | test-only | `groups.PublicGroupError` |
| `*TeacherGroupIDs.TeacherGroupIDs` | [teacher_group_ids_test_helpers.go:21](../../../backend/modules/schoolstructure/compose/teacher_group_ids_test_helpers.go#L21) | test-only | `r.records.GetTeacherGroups`, `make`, `len`, `append` |


### `modules/settings/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `retainedAuditRetention.DeleteExpiredSettingChanges` | [compose_bindings_audit_retention.go:14](../../../backend/modules/settings/compose/compose_bindings_audit_retention.go#L14) | direct binding | `configRepo.DeleteComposedSettingAudit` |
| `*operatorSchoolSettings.Schema` | [operator.go:64](../../../backend/modules/settings/compose/operator.go#L64) | direct binding | `configSvc.OperatorSettingSchema` |
| `*operatorSchoolSettings.Reveal` | [operator.go:68](../../../backend/modules/settings/compose/operator.go#L68) | direct binding | `configSvc.OperatorSettingReveal` |
| `operatorSettingsRuntime.WithinTenant` | [operator.go:74](../../../backend/modules/settings/compose/operator.go#L74) | direct binding | `tenant.WithTenantTx`, `configSvc.SettingContextCallback` |
| `operatorSettingsRuntime.AfterCommit` | [operator.go:78](../../../backend/modules/settings/compose/operator.go#L78) | direct binding | `tenant.RegisterAfterCommit` |
| `operatorSettingsRuntime.Today` | [operator.go:82](../../../backend/modules/settings/compose/operator.go#L82) | direct binding | `configSvc.OperatorSettingToday` |
| `PresenceSettings` | [presence_settings.go:6](../../../backend/modules/settings/compose/presence_settings.go#L6) | direct binding | `config.NewPresenceSettings` |
| `TenantSettingsRuntime.TenantID` | [tenant_settings_runtime.go:27](../../../backend/modules/settings/compose/tenant_settings_runtime.go#L27) | direct binding | `tenant.FromContext` |
| `TenantSettingsRuntime.HasTransaction` | [tenant_settings_runtime.go:29](../../../backend/modules/settings/compose/tenant_settings_runtime.go#L29) | direct binding | `app.SettingTransactionPresent` |
| `TenantSettingsRuntime.DB` | [tenant_settings_runtime.go:33](../../../backend/modules/settings/compose/tenant_settings_runtime.go#L33) | direct binding | `tenant.CurrentTransaction` |
| `TenantSettingsRuntime.LockStaffBalance` | [tenant_settings_runtime.go:37](../../../backend/modules/settings/compose/tenant_settings_runtime.go#L37) | direct binding | `app.LockSettingStaffBalance` |
| `TenantSettingsRuntime.TodayTime` | [tenant_settings_runtime.go:41](../../../backend/modules/settings/compose/tenant_settings_runtime.go#L41) | direct binding | `timezone.TodayDate`, `.UTCMidnight` |
| `TenantSettingsRuntime.WithinTenant` | [tenant_settings_runtime.go:43](../../../backend/modules/settings/compose/tenant_settings_runtime.go#L43) | direct binding | `tenant.WithTenantTx`, `app.AttachSettingUnit`, `app.SettingContextCallback` |
| `TenantSettingsRuntime.WithinAdmin` | [tenant_settings_runtime.go:47](../../../backend/modules/settings/compose/tenant_settings_runtime.go#L47) | direct binding | `tenant.WithAdminTx`, `app.AttachSettingUnit`, `app.SettingContextCallback` |
| `TenantSettingsRuntime.AcquireLock` | [tenant_settings_runtime.go:51](../../../backend/modules/settings/compose/tenant_settings_runtime.go#L51) | direct binding | `tenant.AcquireLock`, `app.AttachSettingUnit` |
| `TenantSettingsRuntime.AfterCommit` | [tenant_settings_runtime.go:55](../../../backend/modules/settings/compose/tenant_settings_runtime.go#L55) | direct binding | `tenant.RegisterAfterCommit` |


### `modules/staffannouncementaudience`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `Map` | [new.go:10](../../../backend/modules/staffannouncementaudience/new.go#L10) | direct binding | `postgres.Map` |


### `modules/staffmessageinbox`

No non-constructor functions.


### `modules/studentdirectoryview/compose`

No non-constructor functions.


### `modules/studentpresence/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `PersistenceDatabaseForTest` | [ambient_scope_test_helpers.go:11](../../../backend/modules/studentpresence/compose/ambient_scope_test_helpers.go#L11) | test-only | `databaseRuntime` |
| `sessionAttendancePorts` | [new.go:47](../../../backend/modules/studentpresence/compose/new.go#L47) | construction/binding | `application.NewPlannedRoster`, `application.NewCarePlanDirectory` |
| `databaseRuntime` | [new.go:63](../../../backend/modules/studentpresence/compose/new.go#L63) | construction/binding | none |
| `transaction.Run` | [new.go:69](../../../backend/modules/studentpresence/compose/new.go#L69) | direct binding | `tenant.NewTransactionRunner`, `.RunInTx` |
| `FailureLogger` | [observers.go:9](../../../backend/modules/studentpresence/compose/observers.go#L9) | direct binding | `application.FailureLogger` |
| `DebugLogger` | [observers.go:10](../../../backend/modules/studentpresence/compose/observers.go#L10) | direct binding | `application.DebugLogger` |
| `SessionRepositories` | [session_records.go:76](../../../backend/modules/studentpresence/compose/session_records.go#L76) | direct binding | `application.SessionRepositories` |


### `modules/studentpresence/compose/presenceservice`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `WithPresenceSettings` | [presence.go:53](../../../backend/modules/studentpresence/compose/presenceservice/presence.go#L53) | direct binding | `presence.WithSettings` |
| `WithPresenceTenantRuntime` | [presence.go:59](../../../backend/modules/studentpresence/compose/presenceservice/presence.go#L59) | direct binding | `presence.WithTenantRuntime` |
| `WithGuardianWaker` | [presence.go:64](../../../backend/modules/studentpresence/compose/presenceservice/presence.go#L64) | direct binding | `presence.WithGuardianWaker` |


### `modules/supervisiondashboard/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `CompareRoomNames` | [compose.go:9](../../../backend/modules/supervisiondashboard/compose/compose.go#L9) | direct binding | `legacy.CompareNames` |


### `modules/timetable/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `CareScheduleRepositories.EnrollmentPickupSchedules` | [care_schedule_repositories_test_helpers.go:38](../../../backend/modules/timetable/compose/care_schedule_repositories_test_helpers.go#L38) | test-only | `application.NewEnrollmentPickupSchedules` |
| `CareScheduleRepositories.EnrollmentArrivalSchedules` | [care_schedule_repositories_test_helpers.go:44](../../../backend/modules/timetable/compose/care_schedule_repositories_test_helpers.go#L44) | test-only | `application.NewEnrollmentArrivalSchedules` |
| `classArrivalExceptionRepository.Upsert` | [class_arrival_exceptions_test_helpers.go:26](../../../backend/modules/timetable/compose/class_arrival_exceptions_test_helpers.go#L26) | test-only | `.RunWrite`, `r.Store.UpsertClassArrivalException`, `postgres.TranslateNotFound` |
| `mirrorSchoolID` | [compose_binding_test_helpers.go:11](../../../backend/modules/timetable/compose/compose_binding_test_helpers.go#L11) | test-only | `application.MirrorSchoolID` |
| `blockDisplayDatabase` | [composition_database_test_helpers.go:12](../../../backend/modules/timetable/compose/composition_database_test_helpers.go#L12) | test-only | `postgres.BlockDisplayDatabase` |
| `SubstituteDayLock` | [instance_lifecycle_ports.go:17](../../../backend/modules/timetable/compose/instance_lifecycle_ports.go#L17) | direct binding | `postgres.SubstituteDayLock` |
| `WithMirrorAfterWrite` | [mirror_write_test_helpers.go:7](../../../backend/modules/timetable/compose/mirror_write_test_helpers.go#L7) | test-only | `application.WithMirrorAfterWrite` |
| `NativeGroupRecords` | [native_fixture_records.go:10](../../../backend/modules/timetable/compose/native_fixture_records.go#L10) | test-only | `application.NativeGroupRecords` |
| `NativeScheduleRecords` | [native_fixture_records.go:13](../../../backend/modules/timetable/compose/native_fixture_records.go#L13) | test-only | `application.NativeScheduleRecords` |
| `NativeEnrollmentRecords` | [native_fixture_records.go:16](../../../backend/modules/timetable/compose/native_fixture_records.go#L16) | test-only | `application.NativeEnrollmentRecords` |
| `NativeExceptionRecords` | [native_fixture_records.go:19](../../../backend/modules/timetable/compose/native_fixture_records.go#L19) | test-only | `application.NativeExceptionRecords` |
| `NativeTimeframeRecords` | [native_fixture_records.go:22](../../../backend/modules/timetable/compose/native_fixture_records.go#L22) | test-only | `application.NativeTimeframeRecords` |
| `transaction.RunWrite` | [new.go:47](../../../backend/modules/timetable/compose/new.go#L47) | direct binding | `postgres.TransactionRunWriteBinding` |
| `OwnerRows.SickCascadeRows` | [owner_rows.go:63](../../../backend/modules/timetable/compose/owner_rows.go#L63) | direct binding | `application.NewSickCascadeTimetableRows` |
| `*planningTrackAdministration.OrderPlanningTracks` | [planning_track_administration.go:22](../../../backend/modules/timetable/compose/planning_track_administration.go#L22) | construction/binding | `boundaryBinding.PlanningTrackAdministrationOrderPlanningTracksBinding` |
| `PresenceDatabase` | [presence_reads.go:26](../../../backend/modules/timetable/compose/presence_reads.go#L26) | direct binding | `postgresNative.DatabaseRuntime` |
| `Repositories.InstanceReads` | [repositories_test_helpers.go:8](../../../backend/modules/timetable/compose/repositories_test_helpers.go#L8) | test-only | `application.NewTimetableInstanceReads` |
| `Repositories.InstanceStaffReads` | [repositories_test_helpers.go:12](../../../backend/modules/timetable/compose/repositories_test_helpers.go#L12) | test-only | `application.NewTimetableInstanceStaffReads` |
| `Repositories.GroupReads` | [repositories.go:71](../../../backend/modules/timetable/compose/repositories.go#L71) | direct binding | `application.NewTimetableGroupReads` |
| `Repositories.NativeRecords` | [repositories.go:76](../../../backend/modules/timetable/compose/repositories.go#L76) | direct binding | none |
| `ActivityInstanceDate` | [retained_rows.go:18](../../../backend/modules/timetable/compose/retained_rows.go#L18) | direct binding | `calendar.Date` |
| `ParseActivityInstanceDate` | [retained_rows.go:23](../../../backend/modules/timetable/compose/retained_rows.go#L23) | direct binding | `application.ParseActivityInstanceDate` |
| `WrapDatabaseError` | [retained_rows.go:28](../../../backend/modules/timetable/compose/retained_rows.go#L28) | direct binding | `application.WrapDatabaseError` |
| `FindCalendarAssignments` | [retained_rows.go:33](../../../backend/modules/timetable/compose/retained_rows.go#L33) | direct binding | `application.FindCalendarAssignments` |
| `FindCalendarInstances` | [retained_rows.go:38](../../../backend/modules/timetable/compose/retained_rows.go#L38) | direct binding | `application.FindCalendarInstances` |
| `FindCalendarShifts` | [retained_rows.go:43](../../../backend/modules/timetable/compose/retained_rows.go#L43) | direct binding | `application.FindCalendarShifts` |
| `ListCalendarShiftTypes` | [retained_rows.go:48](../../../backend/modules/timetable/compose/retained_rows.go#L48) | direct binding | `application.ListCalendarShiftTypes` |
| `ShiftTypeCategoryLinker` | [retained_rows.go:139](../../../backend/modules/timetable/compose/retained_rows.go#L139) | direct binding | `application.ShiftTypeCategoryLinker` |
| `tenantTransactions.InTenant` | [tenant_transactions.go:32](../../../backend/modules/timetable/compose/tenant_transactions.go#L32) | direct binding | `postgres.TenantTransactionsInTenantBinding` |


### `modules/timetableenrollmentprovenance/compose`

No non-constructor functions.


### `modules/timetableprojection/compose`

No non-constructor functions.


### `modules/transactionruntime/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `OpenDatabase` | [database.go:31](../../../backend/modules/transactionruntime/compose/database.go#L31) | direct binding | `openDatabase` |
| `OpenServeDatabase` | [database.go:37](../../../backend/modules/transactionruntime/compose/database.go#L37) | direct binding | `openDatabase`, `database.ObservedBinder` |
| `OpenDemoDatabase` | [database.go:43](../../../backend/modules/transactionruntime/compose/database.go#L43) | direct binding | `openDatabase` |
| `bindDatabase` | [database.go:47](../../../backend/modules/transactionruntime/compose/database.go#L47) | construction/binding | `bind`, `database.NewBoundDatabase` |
| `openDatabase` | [database.go:55](../../../backend/modules/transactionruntime/compose/database.go#L55) | construction/binding | `open`, `bind`, `database.ClosePool` |
| `bindTenantRuntime` | [tenant_runtime.go:34](../../../backend/modules/transactionruntime/compose/tenant_runtime.go#L34) | construction/binding | `tenant.NewUnitOfWork`, `tenant.SavepointFunc`, `runtime.WithTransactionDetacher`, `runtime.WithContextAdapters` |
| `ObserveLockWaits` | [tenant_runtime.go:54](../../../backend/modules/transactionruntime/compose/tenant_runtime.go#L54) | direct binding | `database.AttachLockWaitHook` |
| `OpenPool` | [tenant_runtime.go:59](../../../backend/modules/transactionruntime/compose/tenant_runtime.go#L59) | direct binding | `database.DBConn` |
| `OpenServePool` | [tenant_runtime.go:62](../../../backend/modules/transactionruntime/compose/tenant_runtime.go#L62) | direct binding | `database.DBConnForServe` |
| `TenantCallback` | [tenant_runtime.go:65](../../../backend/modules/transactionruntime/compose/tenant_runtime.go#L65) | construction/binding | `fn` |


### `modules/workforce/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `transaction.LockStaffAbsence` | [absence_engine.go:20](../../../backend/modules/workforce/compose/absence_engine.go#L20) | direct binding | `compositionBinding.LockStaffAbsenceGate` |
| `retainedStaffScheduleAssignments.ScheduleAssignment` | [compose_bindings_time_tracking_staff_schedule.go:17](../../../backend/modules/workforce/compose/compose_bindings_time_tracking_staff_schedule.go#L17) | direct binding | `timetracking.ReadScheduleAssignment` |
| `transaction.RunWrite` | [new.go:85](../../../backend/modules/workforce/compose/new.go#L85) | direct binding | `postgres.TransactionRunWriteBinding` |
| `transaction.LockStaffQualifications` | [new.go:89](../../../backend/modules/workforce/compose/new.go#L89) | direct binding | `postgres.LockStaffQualificationsGate` |
| `transaction.LockStaffBalance` | [new.go:96](../../../backend/modules/workforce/compose/new.go#L96) | direct binding | `postgres.LockStaffBalanceGate` |
| `transaction.acquireXactLock` | [new.go:105](../../../backend/modules/workforce/compose/new.go#L105) | direct binding | `postgres.AcquireXactLock` |
| `transaction.LockStaffShifts` | [new.go:109](../../../backend/modules/workforce/compose/new.go#L109) | direct binding | `postgres.LockStaffShiftsGate` |
| `transaction.RunRead` | [new.go:115](../../../backend/modules/workforce/compose/new.go#L115) | direct binding | `postgres.TransactionRunReadBinding` |
| `*ShiftPlanning.Planning` | [shift_planning.go:237](../../../backend/modules/workforce/compose/shift_planning.go#L237) | direct binding | `planning.NewStaffShiftPlanning` |
| `*WorkSessionSide.WithSickCascade` | [time_tracking.go:123](../../../backend/modules/workforce/compose/time_tracking.go#L123) | direct binding | `s.WorkSessionSide.WithSickCascade` |
| `StaffScheduleAssignments` | [time_tracking_staff_schedule.go:18](../../../backend/modules/workforce/compose/time_tracking_staff_schedule.go#L18) | direct binding | none |
| `staffAllowanceTestDatabase` | [time_tracking_test_helpers.go:51](../../../backend/modules/workforce/compose/time_tracking_test_helpers.go#L51) | test-only | `postgresNative.DatabaseRuntime`, `errors.New` |
| `WorkPresence` | [work_presence.go:18](../../../backend/modules/workforce/compose/work_presence.go#L18) | direct binding | none |
| `workPresence.GetTodayPresenceMap` | [work_presence.go:22](../../../backend/modules/workforce/compose/work_presence.go#L22) | direct binding | `p.capability.WorkPresenceMap` |
| `LegacyWorkTimeModelNotFound` | [work_time_model_not_found.go:5](../../../backend/modules/workforce/compose/work_time_model_not_found.go#L5) | direct binding | `postgresBindings.LegacyWorkTimeModelNotFound` |


### `observability/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `InstallProcessLogger` | [process_logger.go:12](../../../backend/observability/compose/process_logger.go#L12) | construction/binding | `newProcessLogger`, `applog.ConfigureDefault` |


### `workflows/gradetransition/compose`

No non-constructor functions.


### `workflows/openroommove/compose`

No non-constructor functions.


### `workflows/parentportal/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `careConfig` | [new.go:115](../../../backend/workflows/parentportal/compose/new.go#L115) | construction/binding | `newGuardianChanges`, `messagingFlows.ChildFlowSharing`, `newMealPlan` |
| `messagingConfig` | [new.go:146](../../../backend/workflows/parentportal/compose/new.go#L146) | construction/binding | none |
| `bindOwners` | [new.go:170](../../../backend/workflows/parentportal/compose/new.go#L170) | construction/binding | `newGuardianRecords`, `newStudentRecords` |


### `workflows/reminderdelivery/compose`

No non-constructor functions.


### `workflows/sessionend/compose`

No non-constructor functions.


### `workflows/shiftplansync/compose`

No non-constructor functions.


### `workflows/staffclock/compose`

No non-constructor functions.


### `workflows/staffoffboarding/compose`

No non-constructor functions.


### `workflows/studentdeletion/compose`

No non-constructor functions.


## Callback bodies, including constructors

All production callbacks below bind or forward native values without runtime branches or iteration. Runtime callbacks formerly in Feedback, Meal Plan and Request Feed now have named native factories.

| Package | Enclosing declaration | Source line | Statements | Branches/loops | Calls | Scope |
|---|---|---|---:|---:|---:|---|
| `modules/enrollment/compose` | `tenantRuntime` | [application.go:156](../../../backend/modules/enrollment/compose/application.go#L156) | 1 | 0 | 3 | production |
| `modules/enrollment/compose` | `tenantRuntime` | [application.go:157](../../../backend/modules/enrollment/compose/application.go#L157) | 1 | 0 | 1 | production |
| `modules/enrollment/compose` | `tenantRuntime` | [application.go:159](../../../backend/modules/enrollment/compose/application.go#L159) | 1 | 0 | 3 | production |
| `modules/enrollment/compose` | `tenantRuntime` | [application.go:160](../../../backend/modules/enrollment/compose/application.go#L160) | 1 | 0 | 1 | production |
| `modules/exporttransfer/compose` | `retainedNewDetachedContext` | [compose_bindings_callback_detached_context.go:10](../../../backend/modules/exporttransfer/compose/compose_bindings_callback_detached_context.go#L10) | 1 | 0 | 2 | production |
| `modules/dataimport/compose` | `Observe` | [transactions.go:39](../../../backend/modules/dataimport/compose/transactions.go#L39) | 1 | 0 | 1 | production |
| `modules/delivery/compose` | `NewPushAuthorizationChecker` | [provider.go:40](../../../backend/modules/delivery/compose/provider.go#L40) | 1 | 0 | 1 | production |
| `modules/delivery/compose` | `NewExpiredPushSubscriptionCleaner` | [provider.go:48](../../../backend/modules/delivery/compose/provider.go#L48) | 1 | 0 | 1 | production |
| `modules/delivery/compose` | `NewTenantTransactions` | [tenant_transactions.go:23](../../../backend/modules/delivery/compose/tenant_transactions.go#L23) | 1 | 0 | 2 | production |
| `modules/delivery/compose` | `NewTenantTransactions` | [tenant_transactions.go:24](../../../backend/modules/delivery/compose/tenant_transactions.go#L24) | 1 | 0 | 1 | production |
| `modules/peopledirectory/compose` | `LoadForTenant` | [guardian_profile_loader.go:39](../../../backend/modules/peopledirectory/compose/guardian_profile_loader.go#L39) | 1 | 0 | 0 | production |
| `modules/peopledirectory/compose` | `LoadForTenant` | [guardian_profile_loader.go:39](../../../backend/modules/peopledirectory/compose/guardian_profile_loader.go#L39) | 1 | 0 | 0 | production |
| `modules/peopledirectory/compose` | `MustNewReads` | [new.go:145](../../../backend/modules/peopledirectory/compose/new.go#L145) | 0 | 0 | 0 | production |
| `modules/facilities/compose` | `NewUnobserved` | [unobserved.go:17](../../../backend/modules/facilities/compose/unobserved.go#L17) | 1 | 0 | 0 | production |
| `modules/facilities/compose` | `NewUnobserved` | [unobserved.go:21](../../../backend/modules/facilities/compose/unobserved.go#L21) | 0 | 0 | 0 | production |
| `modules/workforce/compose` | `NewStaffShiftLock` | [staff_shift_lock.go:19](../../../backend/modules/workforce/compose/staff_shift_lock.go#L19) | 1 | 0 | 1 | production |
| `modules/workforce/compose` | `NewStaffEmployment` | [staffemployment.go:23](../../../backend/modules/workforce/compose/staffemployment.go#L23) | 0 | 0 | 0 | production |
| `workflows/openroommove/compose` | `New` | [new.go:40](../../../backend/workflows/openroommove/compose/new.go#L40) | 1 | 0 | 1 | production |
| `workflows/parentportal/compose` | `newGuardianRecords` | [owner_bindings.go:23](../../../backend/workflows/parentportal/compose/owner_bindings.go#L23) | 1 | 0 | 1 | production |
| `workflows/parentportal/compose` | `newGuardianRecords` | [owner_bindings.go:26](../../../backend/workflows/parentportal/compose/owner_bindings.go#L26) | 1 | 0 | 1 | production |
| `workflows/parentportal/compose` | `newGuardianRecords` | [owner_bindings.go:29](../../../backend/workflows/parentportal/compose/owner_bindings.go#L29) | 1 | 0 | 1 | production |
| `workflows/parentportal/compose` | `newGuardianRecords` | [owner_bindings.go:32](../../../backend/workflows/parentportal/compose/owner_bindings.go#L32) | 1 | 0 | 1 | production |
| `workflows/parentportal/compose` | `newGuardianRecords` | [owner_bindings.go:35](../../../backend/workflows/parentportal/compose/owner_bindings.go#L35) | 1 | 0 | 0 | production |
| `workflows/parentportal/compose` | `newStudentRecords` | [owner_bindings.go:40](../../../backend/workflows/parentportal/compose/owner_bindings.go#L40) | 1 | 0 | 0 | production |
| `workflows/parentportal/compose` | `newStudentRecords` | [owner_bindings.go:43](../../../backend/workflows/parentportal/compose/owner_bindings.go#L43) | 1 | 0 | 1 | production |
| `workflows/parentportal/compose` | `newGuardianChanges` | [owner_bindings.go:49](../../../backend/workflows/parentportal/compose/owner_bindings.go#L49) | 1 | 0 | 1 | production |
| `workflows/parentportal/compose` | `newMealPlan` | [platform_bindings.go:24](../../../backend/workflows/parentportal/compose/platform_bindings.go#L24) | 1 | 0 | 1 | production |
| `workflows/parentportal/compose` | `newMealPlan` | [platform_bindings.go:27](../../../backend/workflows/parentportal/compose/platform_bindings.go#L27) | 1 | 0 | 1 | production |
| `workflows/parentportal/compose` | `newMealPlan` | [platform_bindings.go:30](../../../backend/workflows/parentportal/compose/platform_bindings.go#L30) | 1 | 0 | 2 | production |
| `workflows/parentportal/compose` | `newMealPlan` | [platform_bindings.go:33](../../../backend/workflows/parentportal/compose/platform_bindings.go#L33) | 1 | 0 | 0 | production |
| `workflows/parentportal/compose` | `newMealPlan` | [platform_bindings.go:36](../../../backend/workflows/parentportal/compose/platform_bindings.go#L36) | 1 | 0 | 0 | production |
| `modules/schoolmembership/compose` | `StudentClassWriteGate` | [student_class_gate.go:21](../../../backend/modules/schoolmembership/compose/student_class_gate.go#L21) | 1 | 0 | 1 | production |
| `modules/communication/composition` | `AudienceMapping` | [new.go:60](../../../backend/modules/communication/composition/new.go#L60) | 1 | 0 | 1 | production |
| `modules/communication/composition` | `AudienceMapping` | [new.go:61](../../../backend/modules/communication/composition/new.go#L61) | 1 | 0 | 0 | production |
| `modules/communication/composition` | `AudienceMapping` | [new.go:64](../../../backend/modules/communication/composition/new.go#L64) | 1 | 0 | 1 | production |
| `modules/communication/composition` | `AudienceMapping` | [new.go:65](../../../backend/modules/communication/composition/new.go#L65) | 1 | 0 | 0 | production |
| `modules/careplan/compose` | `NewExcusedAbsenceRequests` | [excused_requests.go:63](../../../backend/modules/careplan/compose/excused_requests.go#L63) | 0 | 0 | 0 | production |
| `modules/careplan/compose` | `LockWeeklyPickupStudents` | [pickup_excusal.go:44](../../../backend/modules/careplan/compose/pickup_excusal.go#L44) | 1 | 0 | 1 | production |
| `modules/identityaccess/compose` | `newAccountLifecycle` | [account_lifecycle.go:60](../../../backend/modules/identityaccess/compose/account_lifecycle.go#L60) | 1 | 0 | 0 | production |
| `modules/identityaccess/compose` | `NewInvitationMaintenance` | [maintenance.go:25](../../../backend/modules/identityaccess/compose/maintenance.go#L25) | 0 | 0 | 0 | production |
| `modules/identityaccess/compose` | `NewGuardianSeedAccess` | [maintenance.go:36](../../../backend/modules/identityaccess/compose/maintenance.go#L36) | 0 | 0 | 0 | production |
| `modules/identityaccess/compose` | `NewSessionMaintenance` | [maintenance.go:77](../../../backend/modules/identityaccess/compose/maintenance.go#L77) | 0 | 0 | 0 | production |
| `modules/identityaccess/compose` | `NewSessionMaintenance` | [maintenance.go:84](../../../backend/modules/identityaccess/compose/maintenance.go#L84) | 1 | 0 | 0 | production |
| `modules/identityaccess/compose` | `withPasskeyFlows` | [mfa.go:91](../../../backend/modules/identityaccess/compose/mfa.go#L91) | 1 | 0 | 1 | production |
| `modules/identityaccess/compose` | `newCommandTransactions` | [new.go:163](../../../backend/modules/identityaccess/compose/new.go#L163) | 1 | 0 | 0 | production |
| `modules/settings/compose` | `Schema` | [operator.go:65](../../../backend/modules/settings/compose/operator.go#L65) | 1 | 0 | 0 | production |
| `modules/settings/compose` | `Reveal` | [operator.go:69](../../../backend/modules/settings/compose/operator.go#L69) | 1 | 0 | 0 | production |
| `modules/transactionruntime/compose` | `NewDemoTenantRuntime` | [tenant_runtime.go:30](../../../backend/modules/transactionruntime/compose/tenant_runtime.go#L30) | 0 | 0 | 0 | production |
| `modules/transactionruntime/compose` | `NewDemoTenantRuntime` | [tenant_runtime.go:31](../../../backend/modules/transactionruntime/compose/tenant_runtime.go#L31) | 1 | 0 | 0 | production |
| `modules/transactionruntime/compose` | `TenantCallback` | [tenant_runtime.go:66](../../../backend/modules/transactionruntime/compose/tenant_runtime.go#L66) | 1 | 0 | 1 | production |
