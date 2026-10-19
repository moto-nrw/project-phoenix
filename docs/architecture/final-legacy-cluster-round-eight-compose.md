# Final legacy cluster: round 8 compose audit

Base: `148e5aeb86cf22fdbfbffedd76ba070f52f80a3e`. This inventories **every policy package with role `compose`**, except the root-composition and dev-tools owners. Package role, not directory spelling, selects the inventory. Test-hook declarations are listed separately from production. A LOC reduction alone does not prove the exit.

Integrated development: `e423ad19c732f17966b2f8332eafb5df01196f29`.

Classification: **direct binding** has no local branch/loop and at most one delegated call; it supplies a native capability or type conversion without adding a decision. **construction/binding** identifies reviewed startup, callback-signature or runtime-capability wiring. **runtime retained** contains local orchestration, row/error mapping or execution checks and prevents the literal exit. **test-only** does not enter the production exit. Constructor callbacks are audited below too, so a `New` prefix cannot hide runtime logic.


## Round 9 owner set B

Re-audited against `12608383f0f793e811f4379ec293f61efd6150c9`. A-package inventories remain at their prior base until the parallel change is integrated. Counts below are production runtime-retained non-constructors; remaining constructor callbacks in B only bind or forward native values. Transaction operations stay bound at their existing registered compose declarations and execute in named internal functions. No transaction-register keys or router goldens changed. Caregiver bindings retain comparable value fields.

| Compose package | Runtime before → after | Compose LOC before → after |
|---|---:|---:|
| `modules/appointments/compose` | 2 → 0 | 67 → 45 |
| `modules/careplan/compose` | 49 → 0 | 2460 → 1646 |
| `modules/classday/compose` | 13 → 0 | 461 → 213 |
| `modules/dataimport/compose` | 6 → 0 | 202 → 85 |
| `modules/delivery/compose` | 20 → 0 | 759 → 363 |
| `modules/devicefleet/compose` | 8 → 0 | 353 → 219 |
| `modules/devicescan/compose` | 48 → 30 | 1228 → 1053 |
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

B runtime retained: **266 → 30**. Global compose LOC ceiling: **20,119 → 16445** before parallel integration. Native implementations live in each owner's existing application/Postgres packages; Data Import's consumer-row adapters use its adapter package and cross-owner reference lookup lives in `roots/owners`.

### Exact remaining blockers

Only DeviceScan retains runtime. Its native application has consumer ports and its own public contract, but no production rule to the foreign contracts listed below. The reviewed successor shapes in [ADR 0058](../adr/0058-owner-dependencies-form-a-dag.md) expressly exclude workflow sources. Moving these implementations unchanged into native code requires prohibited new grants. Their source-row and error contracts must first become workflow-owned public port contracts, with the foreign conversions bound at the typed root. No new grant, alias, exception or checker accommodation was added for them.

| Function | Blocking native dependency |
|---|---|
| `attendance.ConfirmDailyCheckout` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |
| `attendance.Status` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |
| `attendance.Toggle` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |
| `classifyPresence` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |
| `groupDirectory.Find` | `process-device-scan.application → school-structure.contract`: `SchoolGroups.GetGroup` returns `contract.Group`. |
| `lifecycleSession` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |
| `lifecycleStore.Conflict` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |
| `lifecycleStore.Current` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |
| `lifecycleStore.ReplaceSupervisors` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |
| `lifecycleStore.Start` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |
| `lifecycleStore.Timeout` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |
| `lifecycleStore.TimeoutInfo` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |
| `lifecycleStore.Touch` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |
| `lifecycleStore.ValidateTimeout` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |
| `lifecycleSupervisors` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |
| `openRoomMover.MoveToOpenRoom` | `process-device-scan.application → student-presence.public`: `StudentMoveSkipNotPresent`, `RoomCapacityError` and `ErrStudentsNotPresent` require the foreign presence contract. |
| `openRoomMover.translate` | `process-device-scan.application → student-presence.public`: `StudentMoveSkipNotPresent`, `RoomCapacityError` and `ErrStudentsNotPresent` require the foreign presence contract. |
| `pickupPlan.Effective` | `process-device-scan.application → care-plan.public`: `PickupReader` returns the effective pickup and day-note records. |
| `sessionFromGroup` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |
| `sessions.Current` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |
| `sessions.End` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |
| `sessions.EnsureRoomSession` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |
| `sessions.FindDeviceSessionInRoom` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |
| `sessions.ListOpenInRoom` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |
| `sessions.ReplaceSupervisors` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |
| `sessions.Start` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |
| `sessions.Supervisors` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |
| `supervisionQuery.ActiveSupervisions` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |
| `visits.Current` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |
| `visits.Record` | `process-device-scan.application → student-presence.public`: presence/session/supervision source rows, commands or error classifications. A native adapter role has no successor grant for this workflow either. |

## Package summary

| Compose package | Owner | Non-constructors | Runtime retained |
|---|---|---:|---:|
| `modules/appointments/compose` | `appointments` | 2 | 0 |
| `modules/auditlog/compose` | `audit-platform` | 0 | 0 |
| `modules/careplan/compose` | `care-plan` | 27 | 0 |
| `modules/careplan/requestfeed/compose` | `care-plan` | 2 | 0 |
| `modules/classday/compose` | `class-day-view` | 2 | 0 |
| `modules/communication/composition` | `communication` | 79 | 71 |
| `modules/communication/parentstore` | `communication` | 241 | 104 |
| `modules/communication/staffstore` | `communication` | 6 | 2 |
| `modules/dataimport/compose` | `data-import` | 10 | 0 |
| `modules/dataimport/compose/imports` | `data-import` | 0 | 0 |
| `modules/delivery/compose` | `delivery-platform` | 9 | 0 |
| `modules/devicefleet/compose` | `device-fleet` | 3 | 0 |
| `modules/devicefleet/compose/repositoryadapter` | `device-fleet` | 0 | 0 |
| `modules/devicescan/compose` | `process-device-scan` | 64 | 30 |
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
| `modules/requestreview/compose` | `request-review-view` | 40 | 37 |
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
| `modules/studentpresence/compose` | `student-presence` | 42 | 31 |
| `modules/studentpresence/compose/presenceservice` | `student-presence` | 72 | 45 |
| `modules/supervisiondashboard/compose` | `calendar-view` | 1 | 0 |
| `modules/timetable/compose` | `timetable-activities` | 29 | 0 |
| `modules/timetableenrollmentprovenance/compose` | `timetable-enrollment-provenance` | 0 | 0 |
| `modules/timetableprojection/compose` | `timetable-legacy-view` | 0 | 0 |
| `modules/transactionruntime/compose` | `transaction-runtime` | 10 | 0 |
| `modules/workforce/compose` | `workforce` | 15 | 0 |
| `observability/compose` | `observability` | 1 | 0 |
| `workflows/gradetransition/compose` | `grade-transition` | 0 | 0 |
| `workflows/openroommove/compose` | `open-room-move` | 2 | 2 |
| `workflows/parentportal/compose` | `parent-portal` | 29 | 24 |
| `workflows/reminderdelivery/compose` | `reminder-delivery` | 0 | 0 |
| `workflows/sessionend/compose` | `session-end` | 8 | 8 |
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
| `tokenAdapter.New` | [new.go:67](../../../backend/modules/careplan/requestfeed/compose/new.go#L67) | direct binding | `a.newToken` |
| `tokenAdapter.Hash` | [new.go:68](../../../backend/modules/careplan/requestfeed/compose/new.go#L68) | direct binding | `a.hashToken` |

### `modules/classday/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `Rules.RowCareDay` | [rules_test_helpers.go:13](../../../backend/modules/classday/compose/rules_test_helpers.go#L13) | test-only | `application.Rules`, `.RowCareDay` |
| `Rules.EnrolledOn` | [rules_test_helpers.go:16](../../../backend/modules/classday/compose/rules_test_helpers.go#L16) | test-only | `application.Rules`, `.EnrolledOn` |


### `modules/communication/composition`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `*parentMessaging.ListParentMessageInbox` | [messages.go:78](../../../backend/modules/communication/composition/messages.go#L78) | runtime retained | `observeResult`, `m.service.ListInbox`, `mapParentMessageInbox`, `mapParentMessagingError` |
| `*parentMessaging.CountUnreadParentMessages` | [messages.go:85](../../../backend/modules/communication/composition/messages.go#L85) | runtime retained | `observeResult`, `m.service.UnreadMessageCount`, `mapParentMessagingError` |
| `*parentMessaging.GetParentMessageThread` | [messages.go:92](../../../backend/modules/communication/composition/messages.go#L92) | runtime retained | `observeResult`, `m.service.GetThread`, `mapParentMessagingError`, `mapParentMessageThread` |
| `*parentMessaging.ListMessageableGuardians` | [messages.go:102](../../../backend/modules/communication/composition/messages.go#L102) | runtime retained | `observeResult`, `m.service.ListGuardians`, `mapParentMessagingError`, `make`, `len`, `append` |
| `*parentMessaging.ListParentMessageThreadsForStudent` | [messages.go:119](../../../backend/modules/communication/composition/messages.go#L119) | runtime retained | `observeResult`, `m.service.ListStudentThreads`, `mapParentMessageInbox`, `mapParentMessagingError` |
| `*parentMessaging.StartParentMessageThread` | [messages.go:126](../../../backend/modules/communication/composition/messages.go#L126) | runtime retained | `observeResult`, `m.service.StartThread`, `mapParentMessagingError`, `mapParentMessageThread` |
| `*parentMessaging.OpenParentMessageThread` | [messages.go:136](../../../backend/modules/communication/composition/messages.go#L136) | runtime retained | `observeResult`, `m.service.OpenThread`, `mapParentMessagingError`, `mapParentMessageThread` |
| `*parentMessaging.PostParentMessage` | [messages.go:146](../../../backend/modules/communication/composition/messages.go#L146) | runtime retained | `observeResult`, `m.service.PostMessage`, `mapParentMessagingError`, `mapParentMessages` |
| `*parentMessaging.MarkParentMessageThreadUnread` | [messages.go:156](../../../backend/modules/communication/composition/messages.go#L156) | runtime retained | `observeResult`, `mapParentMessagingError`, `m.service.MarkUnread` |
| `*parentMessaging.MarkAllParentMessagesRead` | [messages.go:163](../../../backend/modules/communication/composition/messages.go#L163) | runtime retained | `observeResult`, `m.service.MarkAllRead`, `mapParentMessagingError` |
| `*parentMessaging.ParentMessageCountScope` | [messages.go:170](../../../backend/modules/communication/composition/messages.go#L170) | runtime retained | `observeResult`, `m.service.CountScope`, `mapParentMessagingError`, `m.service.HasOwnGroups`, `mapParentMessagingError` |
| `*parentMessaging.SetParentMessageCountScope` | [messages.go:184](../../../backend/modules/communication/composition/messages.go#L184) | runtime retained | `observeResult`, `mapParentMessagingError`, `m.service.SetCountScope` |
| `*staffMessaging.ListStaffMessageInbox` | [messages.go:221](../../../backend/modules/communication/composition/messages.go#L221) | runtime retained | `observeResult`, `m.service.ListInbox`, `mapStaffMessagingError`, `make`, `len`, `append` |
| `*staffMessaging.CountUnreadStaffMessages` | [messages.go:240](../../../backend/modules/communication/composition/messages.go#L240) | runtime retained | `observeResult`, `m.service.UnreadMessageCount`, `mapStaffMessagingError` |
| `*staffMessaging.ListMessageableStaff` | [messages.go:247](../../../backend/modules/communication/composition/messages.go#L247) | runtime retained | `observeResult`, `m.service.ListMessageableStaff`, `mapStaffMessagingError`, `make`, `len`, `append` |
| `*staffMessaging.OpenStaffMessageThread` | [messages.go:261](../../../backend/modules/communication/composition/messages.go#L261) | runtime retained | `observeResult`, `m.service.OpenThread`, `mapStaffMessageThread`, `mapStaffMessagingError` |
| `*staffMessaging.GetStaffMessageThread` | [messages.go:268](../../../backend/modules/communication/composition/messages.go#L268) | runtime retained | `observeResult`, `m.service.GetThread`, `mapStaffMessageThread`, `mapStaffMessagingError` |
| `*staffMessaging.PostStaffMessage` | [messages.go:275](../../../backend/modules/communication/composition/messages.go#L275) | runtime retained | `observeResult`, `m.service.PostMessage`, `mapStaffMessagingError`, `mapStaffMessage`, `mapStaffMessagingError` |
| `*staffMessaging.CleanupExpiredStaffMessages` | [messages.go:286](../../../backend/modules/communication/composition/messages.go#L286) | runtime retained | `observeResult`, `m.service.CleanupExpiredMessages`, `mapStaffMessagingError` |
| `observeResult` | [messages.go:297](../../../backend/modules/communication/composition/messages.go#L297) | runtime retained | `time.Now`, `communicationPostgres.WithMessageQueryStats`, `func() {
	if observe != nil {
		stats := queryStats.Snapshot()
		if err != nil {
			stats.Rows = 0
		}
		observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	}
}`, `queryStats.Snapshot`, `observe`, `time.Since`, `run` |
| `mapParentMessageInbox` | [messages.go:312](../../../backend/modules/communication/composition/messages.go#L312) | runtime retained | `make`, `len`, `append` |
| `mapParentMessageThread` | [messages.go:325](../../../backend/modules/communication/composition/messages.go#L325) | runtime retained | `mapParentMessages` |
| `mapParentMessages` | [messages.go:339](../../../backend/modules/communication/composition/messages.go#L339) | runtime retained | `make`, `len`, `mapParentMessage`, `append` |
| `mapParentMessage` | [messages.go:351](../../../backend/modules/communication/composition/messages.go#L351) | runtime retained | `json.Marshal`, `fmt.Errorf` |
| `mapStaffMessageThread` | [messages.go:369](../../../backend/modules/communication/composition/messages.go#L369) | runtime retained | `make`, `len`, `append`, `mapStaffMessage` |
| `mapStaffMessage` | [messages.go:384](../../../backend/modules/communication/composition/messages.go#L384) | direct binding | none |
| `mapParentMessagingError` | [messages.go:391](../../../backend/modules/communication/composition/messages.go#L391) | runtime retained | `errors.Is`, `fmt.Errorf`, `errors.Is`, `fmt.Errorf`, `errors.Is`, `fmt.Errorf`, `errors.Is`, `fmt.Errorf`, `errors.Is`, `fmt.Errorf`, `errors.Is`, `fmt.Errorf`, `errors.Is`, `fmt.Errorf`, `errors.Is`, `fmt.Errorf`, `errors.Is`, `fmt.Errorf` |
| `mapStaffMessagingError` | [messages.go:418](../../../backend/modules/communication/composition/messages.go#L418) | runtime retained | `errors.Is`, `fmt.Errorf`, `errors.Is`, `fmt.Errorf`, `errors.Is`, `fmt.Errorf`, `errors.Is`, `fmt.Errorf`, `errors.Is`, `fmt.Errorf`, `errors.Is`, `fmt.Errorf`, `errors.Is`, `fmt.Errorf`, `errors.Is`, `fmt.Errorf`, `errors.Is`, `fmt.Errorf`, `errors.Is`, `fmt.Errorf` |
| `AuditFunc.AppendAnnouncementAudit` | [new.go:35](../../../backend/modules/communication/composition/new.go#L35) | direct binding | `f` |
| `transaction.RunAdmin` | [new.go:75](../../../backend/modules/communication/composition/new.go#L75) | runtime retained | `tenant.TransactionFromContext`, `callback`, `tenant.WithinAdminRetry` |
| `targets.CountOrganizationsByID` | [new.go:86](../../../backend/modules/communication/composition/new.go#L86) | runtime retained | `t.capability.CountOrganizationsByID`, `int64`, `errors.Is`, `fmt.Errorf` |
| `targets.CountSchoolsByID` | [new.go:95](../../../backend/modules/communication/composition/new.go#L95) | runtime retained | `t.capability.CountSchoolsByID`, `int64`, `errors.Is`, `fmt.Errorf` |
| `audit.Append` | [new.go:106](../../../backend/modules/communication/composition/new.go#L106) | runtime retained | `time.Now`, `a.port.AppendAnnouncementAudit`, `time.Since` |
| `AudienceMapping` | [new.go:119](../../../backend/modules/communication/composition/new.go#L119) | construction/binding | `domain.Announcement`, `domain.AnnouncementViewDetail` |
| `AnnouncementDatabase` | [new.go:133](../../../backend/modules/communication/composition/new.go#L133) | construction/binding | `postgres.NewAnnouncementDatabase` |
| `*notificationConsent.StoredConsent` | [notification_consent.go:37](../../../backend/modules/communication/composition/notification_consent.go#L37) | runtime retained | `observeResult`, `c.store.StoredConsent` |
| `*notificationConsent.RecordConsent` | [notification_consent.go:43](../../../backend/modules/communication/composition/notification_consent.go#L43) | runtime retained | `observeResult`, `c.store.RecordConsent` |
| `*notificationConsent.DisableConsent` | [notification_consent.go:50](../../../backend/modules/communication/composition/notification_consent.go#L50) | runtime retained | `observeResult`, `c.store.DisableConsent` |
| `*notificationConsent.HasAnyOptedIn` | [notification_consent.go:57](../../../backend/modules/communication/composition/notification_consent.go#L57) | runtime retained | `observeResult`, `c.store.HasAnyOptedIn` |
| `*notificationConsent.FilterOptedIn` | [notification_consent.go:63](../../../backend/modules/communication/composition/notification_consent.go#L63) | runtime retained | `observeResult`, `c.store.FilterOptedIn` |
| `*notificationConsent.ListOptedIn` | [notification_consent.go:69](../../../backend/modules/communication/composition/notification_consent.go#L69) | runtime retained | `observeResult`, `c.store.ListOptedIn` |
| `*notificationConsent.FilterOptedInByType` | [notification_consent.go:75](../../../backend/modules/communication/composition/notification_consent.go#L75) | runtime retained | `observeResult`, `c.store.FilterOptedInByType` |
| `*notificationConsent.FilterNotOptedOut` | [notification_consent.go:81](../../../backend/modules/communication/composition/notification_consent.go#L81) | runtime retained | `observeResult`, `c.store.FilterNotOptedOut` |
| `parentAnnouncementDeliveryAdapter.ReplaceForEntity` | [parent_announcements.go:76](../../../backend/modules/communication/composition/parent_announcements.go#L76) | runtime retained | `make`, `len`, `append`, `a.ParentAnnouncementDeliveryRecorder.ReplaceForEntity` |
| `parentAnnouncementDeliveryAdapter.ListForEntity` | [parent_announcements.go:90](../../../backend/modules/communication/composition/parent_announcements.go#L90) | runtime retained | `a.ParentAnnouncementDeliveryRecorder.ListForEntity`, `make`, `len`, `append` |
| `*parentAnnouncements.DeleteExpiredParentAnnouncements` | [parent_announcements.go:112](../../../backend/modules/communication/composition/parent_announcements.go#L112) | direct binding | `p.service.DeleteExpiredParentAnnouncements` |
| `*parentAnnouncements.ListParentAnnouncements` | [parent_announcements.go:116](../../../backend/modules/communication/composition/parent_announcements.go#L116) | runtime retained | `p.service.List`, `mapParentAnnouncementError`, `make`, `len`, `append`, `mapParentAnnouncement` |
| `*parentAnnouncements.GetParentAnnouncement` | [parent_announcements.go:128](../../../backend/modules/communication/composition/parent_announcements.go#L128) | runtime retained | `p.service.Get`, `mapParentAnnouncementError`, `mapParentAnnouncement`, `row.IsDeclaration`, `p.service.DeclarationFrozen` |
| `*parentAnnouncements.CreateParentAnnouncement` | [parent_announcements.go:144](../../../backend/modules/communication/composition/parent_announcements.go#L144) | runtime retained | `p.service.Create`, `mapParentAnnouncementInput`, `mapParentAnnouncement`, `mapParentAnnouncementError` |
| `*parentAnnouncements.UpdateParentAnnouncement` | [parent_announcements.go:149](../../../backend/modules/communication/composition/parent_announcements.go#L149) | runtime retained | `p.service.Update`, `mapParentAnnouncementInput`, `mapParentAnnouncement`, `mapParentAnnouncementError` |
| `*parentAnnouncements.DeleteParentAnnouncement` | [parent_announcements.go:154](../../../backend/modules/communication/composition/parent_announcements.go#L154) | runtime retained | `mapParentAnnouncementError`, `p.service.Delete` |
| `*parentAnnouncements.PublishParentAnnouncement` | [parent_announcements.go:158](../../../backend/modules/communication/composition/parent_announcements.go#L158) | runtime retained | `p.service.Publish`, `mapParentAnnouncement`, `mapParentAnnouncementError` |
| `*parentAnnouncements.UnpublishParentAnnouncement` | [parent_announcements.go:163](../../../backend/modules/communication/composition/parent_announcements.go#L163) | runtime retained | `p.service.Unpublish`, `mapParentAnnouncement`, `mapParentAnnouncementError` |
| `*parentAnnouncements.ParentAnnouncementStats` | [parent_announcements.go:168](../../../backend/modules/communication/composition/parent_announcements.go#L168) | runtime retained | `p.service.Stats`, `mapParentAnnouncementError`, `mapParentAnnouncementError` |
| `*parentAnnouncements.ParentAnnouncementRecipients` | [parent_announcements.go:178](../../../backend/modules/communication/composition/parent_announcements.go#L178) | runtime retained | `p.service.Recipients`, `mapParentAnnouncementError`, `make`, `len`, `append` |
| `*parentAnnouncements.ParentAnnouncementPollResults` | [parent_announcements.go:193](../../../backend/modules/communication/composition/parent_announcements.go#L193) | runtime retained | `p.service.PollResults`, `mapParentAnnouncementError`, `make`, `len`, `append`, `mapParentAnnouncementError` |
| `*parentAnnouncements.ParentAnnouncementPollChildren` | [parent_announcements.go:210](../../../backend/modules/communication/composition/parent_announcements.go#L210) | runtime retained | `p.service.PollChildren`, `mapParentAnnouncementError`, `make`, `len`, `append` |
| `*parentAnnouncements.RemindParentAnnouncement` | [parent_announcements.go:226](../../../backend/modules/communication/composition/parent_announcements.go#L226) | runtime retained | `p.service.RemindOutstanding`, `mapParentAnnouncementError` |
| `*parentAnnouncements.ParentAnnouncementLetterStatus` | [parent_announcements.go:231](../../../backend/modules/communication/composition/parent_announcements.go#L231) | runtime retained | `p.service.LetterStatus`, `mapParentAnnouncementError`, `make`, `len`, `make`, `len`, `append`, `append`, `mapParentAnnouncementError` |
| `*parentAnnouncements.ResendParentAnnouncementEmails` | [parent_announcements.go:265](../../../backend/modules/communication/composition/parent_announcements.go#L265) | runtime retained | `p.service.ResendFailedEmails`, `mapParentAnnouncementError` |
| `*parentAnnouncements.UpdateParentAnnouncementReminder` | [parent_announcements.go:270](../../../backend/modules/communication/composition/parent_announcements.go#L270) | runtime retained | `p.service.UpdateReminder`, `mapParentAnnouncement`, `mapParentAnnouncementError` |
| `*parentAnnouncements.SendDueParentAnnouncementReminders` | [parent_announcements.go:275](../../../backend/modules/communication/composition/parent_announcements.go#L275) | runtime retained | `p.service.SendDueReminders`, `mapParentAnnouncementError` |
| `*parentAnnouncements.PublishCareCancellation` | [parent_announcements.go:280](../../../backend/modules/communication/composition/parent_announcements.go#L280) | runtime retained | `p.service.PublishCareCancellation`, `mapParentAnnouncementError`, `mapParentAnnouncementError` |
| `*parentAnnouncements.CareCancellationReachFor` | [parent_announcements.go:292](../../../backend/modules/communication/composition/parent_announcements.go#L292) | runtime retained | `p.service.CareCancellationReachFor`, `mapParentAnnouncementError`, `mapParentAnnouncementError` |
| `*parentAnnouncements.AnnouncementExists` | [parent_announcements.go:300](../../../backend/modules/communication/composition/parent_announcements.go#L300) | runtime retained | `p.service.AnnouncementExists`, `mapParentAnnouncementError` |
| `*parentAnnouncements.AnnouncementEditable` | [parent_announcements.go:305](../../../backend/modules/communication/composition/parent_announcements.go#L305) | runtime retained | `p.service.AnnouncementEditable`, `mapParentAnnouncementError` |
| `*parentAnnouncements.LockAnnouncementForAttachmentChange` | [parent_announcements.go:310](../../../backend/modules/communication/composition/parent_announcements.go#L310) | runtime retained | `p.service.LockAnnouncementForAttachmentChange`, `mapParentAnnouncementError` |
| `*parentAnnouncements.ResetAnnouncementEngagement` | [parent_announcements.go:315](../../../backend/modules/communication/composition/parent_announcements.go#L315) | runtime retained | `mapParentAnnouncementError`, `p.service.ResetAnnouncementEngagement` |
| `mapParentAnnouncementInput` | [parent_announcements.go:319](../../../backend/modules/communication/composition/parent_announcements.go#L319) | runtime retained | `make`, `len`, `append`, `staff.AnnouncementDeclarationSettings` |
| `mapParentAnnouncement` | [parent_announcements.go:335](../../../backend/modules/communication/composition/parent_announcements.go#L335) | runtime retained | `make`, `len`, `append`, `make`, `len`, `append`, `communication.ParentDeclarationSettings` |
| `*mappedParentAnnouncementError.Error` | [parent_announcements.go:368](../../../backend/modules/communication/composition/parent_announcements.go#L368) | direct binding | `e.original.Error` |
| `*mappedParentAnnouncementError.Unwrap` | [parent_announcements.go:369](../../../backend/modules/communication/composition/parent_announcements.go#L369) | direct binding | none |
| `mapParentAnnouncementError` | [parent_announcements.go:371](../../../backend/modules/communication/composition/parent_announcements.go#L371) | runtime retained | `errors.Is` |
| `*parentAnnouncements.ParentDeclarationStatus` | [parent_declarations.go:11](../../../backend/modules/communication/composition/parent_declarations.go#L11) | runtime retained | `p.service.ParentDeclarationStatus`, `mapParentAnnouncementError`, `mapDeclarationStatus` |
| `mapDeclarationStatus` | [parent_declarations.go:19](../../../backend/modules/communication/composition/parent_declarations.go#L19) | runtime retained | `communication.ParentDeclarationSettings`, `append`, `mapDeclarationVersion`, `mapDeclarationVersion`, `make`, `len`, `append`, `communication.DeclarationSignerStatus`, `append`, `append`, `communication.DeclarationSubmissionRecord` |
| `mapDeclarationVersion` | [parent_declarations.go:48](../../../backend/modules/communication/composition/parent_declarations.go#L48) | runtime retained | `make`, `len`, `append`, `communication.DeclarationAttachment` |
| `parentMessageRetention.CleanupExpiredParentMessages` | [parent_message_retention.go:28](../../../backend/modules/communication/composition/parent_message_retention.go#L28) | runtime retained | `r.retention.CleanupExpiredMessages` |
| `parentDatabase` | [parent_store.go:26](../../../backend/modules/communication/composition/parent_store.go#L26) | runtime retained | `tenant.TransactionFromContext`, `parentpostgres.ResolveDatabase`, `parentpostgres.ScopeFor`, `tenant.FromContext`, `tenant.IsAdminTx` |
| `InstrumentMessageQueries` | [query_observer.go:11](../../../backend/modules/communication/composition/query_observer.go#L11) | direct binding | `communicationPostgres.InstrumentMessageQueries` |

### `modules/communication/parentstore`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `ParentAnnouncementFixtures.Create` | [announcement_fixtures_test_helpers.go:24](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L24) | test-only | `announcementFixtureToRow`, `f.repo.Create`, `announcementFixtureFromRow` |
| `ParentAnnouncementFixtures.FindByID` | [announcement_fixtures_test_helpers.go:33](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L33) | test-only | `f.repo.FindByID`, `announcementFixtureFromRow` |
| `ParentAnnouncementFixtures.Delete` | [announcement_fixtures_test_helpers.go:38](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L38) | test-only | `f.repo.Delete` |
| `ParentAnnouncementFixtures.SetPublished` | [announcement_fixtures_test_helpers.go:41](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L41) | test-only | `f.repo.SetPublished` |
| `ParentAnnouncementFixtures.ReplaceTargets` | [announcement_fixtures_test_helpers.go:45](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L45) | test-only | `announcementTargetFixtureRows`, `f.repo.ReplaceTargets`, `announcementTargetFixture` |
| `ParentAnnouncementFixtures.ReplaceOptions` | [announcement_fixtures_test_helpers.go:56](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L56) | test-only | `announcementOptionFixtureRows`, `f.repo.ReplaceOptions`, `announcementOptionFixture` |
| `ParentAnnouncementFixtures.ListOptions` | [announcement_fixtures_test_helpers.go:67](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L67) | test-only | `f.repo.ListOptions`, `make`, `len`, `announcementOptionFixture` |
| `ParentAnnouncementFixtures.InsertDeclarationVersion` | [announcement_fixtures_test_helpers.go:80](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L80) | test-only | `f.repo.InsertDeclarationVersion` |
| `ParentAnnouncementFixtures.ListDeclarationSubmissions` | [announcement_fixtures_test_helpers.go:88](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L88) | test-only | `f.repo.ListDeclarationSubmissions` |
| `announcementFixtureToRow` | [announcement_fixtures_test_helpers.go:93](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L93) | test-only | `announcementTargetFixtureRows`, `announcementOptionFixtureRows`, `communication.PortalAnnouncementDeclarationSettings` |
| `announcementFixtureFromRow` | [announcement_fixtures_test_helpers.go:122](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L122) | test-only | `communication.ParentDeclarationSettings`, `make`, `len`, `announcementTargetFixture`, `make`, `len`, `announcementOptionFixture` |
| `announcementTargetFixtureRows` | [announcement_fixtures_test_helpers.go:165](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L165) | test-only | `make`, `len` |
| `announcementTargetFixture` | [announcement_fixtures_test_helpers.go:187](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L187) | test-only | none |
| `announcementOptionFixtureRows` | [announcement_fixtures_test_helpers.go:201](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L201) | test-only | `make`, `len` |
| `announcementOptionFixture` | [announcement_fixtures_test_helpers.go:222](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L222) | test-only | none |
| `ParentAnnouncementPublishedError` | [announcement_fixtures_test_helpers.go:236](../../../backend/modules/communication/parentstore/announcement_fixtures_test_helpers.go#L236) | test-only | none |
| `*announcementRepository.Create` | [announcements.go:53](../../../backend/modules/communication/parentstore/announcements.go#L53) | runtime retained | `stampAnnouncementKind`, `announcementValue`, `r.store.Create` |
| `*announcementRepository.Update` | [announcements.go:69](../../../backend/modules/communication/parentstore/announcements.go#L69) | runtime retained | `stampAnnouncementKind`, `announcementValue`, `r.store.UpdateDraft`, `publishedError` |
| `*announcementRepository.FindByID` | [announcements.go:79](../../../backend/modules/communication/parentstore/announcements.go#L79) | runtime retained | `r.store.FindByID`, `announcementModel` |
| `*announcementRepository.FindByIDForUpdate` | [announcements.go:84](../../../backend/modules/communication/parentstore/announcements.go#L84) | runtime retained | `r.store.FindByIDForUpdate`, `announcementModel` |
| `*announcementRepository.Delete` | [announcements.go:89](../../../backend/modules/communication/parentstore/announcements.go#L89) | direct binding | `r.store.Delete` |
| `*announcementRepository.ListForTenant` | [announcements.go:93](../../../backend/modules/communication/parentstore/announcements.go#L93) | runtime retained | `r.store.ListForTenant`, `make`, `len`, `append`, `announcementModel` |
| `*announcementRepository.SetPublished` | [announcements.go:105](../../../backend/modules/communication/parentstore/announcements.go#L105) | direct binding | `r.store.SetPublished` |
| `*announcementRepository.ClearEngagement` | [announcements.go:109](../../../backend/modules/communication/parentstore/announcements.go#L109) | direct binding | `r.store.ClearEngagement` |
| `*announcementRepository.PublishIfDraft` | [announcements.go:113](../../../backend/modules/communication/parentstore/announcements.go#L113) | direct binding | `r.store.PublishIfDraft` |
| `*announcementRepository.SetReminder` | [announcements.go:117](../../../backend/modules/communication/parentstore/announcements.go#L117) | direct binding | `r.store.SetReminder` |
| `*announcementRepository.ListDueReminders` | [announcements.go:121](../../../backend/modules/communication/parentstore/announcements.go#L121) | runtime retained | `r.store.ListDueReminders`, `make`, `len`, `append`, `announcementModel` |
| `*announcementRepository.ClaimReminder` | [announcements.go:133](../../../backend/modules/communication/parentstore/announcements.go#L133) | direct binding | `r.store.ClaimReminder` |
| `*announcementRepository.ReleaseReminderClaim` | [announcements.go:137](../../../backend/modules/communication/parentstore/announcements.go#L137) | direct binding | `r.store.ReleaseReminderClaim` |
| `*announcementRepository.ReplaceTargets` | [announcements.go:141](../../../backend/modules/communication/parentstore/announcements.go#L141) | runtime retained | `make`, `len`, `append`, `r.store.ReplaceTargets`, `publishedError` |
| `*announcementRepository.ListTargets` | [announcements.go:161](../../../backend/modules/communication/parentstore/announcements.go#L161) | runtime retained | `r.store.ListTargets`, `make`, `len`, `append`, `targetModel` |
| `*announcementRepository.ReplaceOptions` | [announcements.go:173](../../../backend/modules/communication/parentstore/announcements.go#L173) | runtime retained | `make`, `len`, `append`, `r.store.ReplaceOptions`, `publishedError` |
| `*announcementRepository.ListOptions` | [announcements.go:194](../../../backend/modules/communication/parentstore/announcements.go#L194) | runtime retained | `r.store.ListOptions`, `optionModels` |
| `*announcementRepository.ListOptionsForAnnouncements` | [announcements.go:199](../../../backend/modules/communication/parentstore/announcements.go#L199) | runtime retained | `r.store.ListOptionsForAnnouncements`, `optionModels` |
| `*announcementRepository.AnswerableChildren` | [announcements.go:204](../../../backend/modules/communication/parentstore/announcements.go#L204) | runtime retained | `r.audience.AnswerableChildren`, `make`, `len`, `append` |
| `*announcementRepository.AccountMayAnswerForStudent` | [announcements.go:219](../../../backend/modules/communication/parentstore/announcements.go#L219) | direct binding | `r.audience.AccountMayAnswerForStudent` |
| `*announcementRepository.SetResponse` | [announcements.go:232](../../../backend/modules/communication/parentstore/announcements.go#L232) | runtime retained | `r.store.LockResponse`, `r.audience.HoldAnswerPermission`, `r.store.DeleteResponse`, `len`, `r.store.InsertResponse` |
| `*announcementRepository.PollResults` | [announcements.go:247](../../../backend/modules/communication/parentstore/announcements.go#L247) | runtime retained | `r.audience.PollResults`, `make`, `len`, `append` |
| `*announcementRepository.PollChildren` | [announcements.go:264](../../../backend/modules/communication/parentstore/announcements.go#L264) | runtime retained | `r.audience.PollChildren`, `make`, `len`, `append` |
| `*announcementRepository.UnansweredReminderRecipients` | [announcements.go:280](../../../backend/modules/communication/parentstore/announcements.go#L280) | runtime retained | `r.audience.UnansweredReminderRecipients`, `reminderModels` |
| `*announcementRepository.UnacknowledgedReminderRecipients` | [announcements.go:285](../../../backend/modules/communication/parentstore/announcements.go#L285) | runtime retained | `r.audience.UnacknowledgedReminderRecipients`, `reminderModels` |
| `*announcementRepository.CountAudience` | [announcements.go:290](../../../backend/modules/communication/parentstore/announcements.go#L290) | direct binding | `r.audience.CountAudience` |
| `*announcementRepository.AccountMatchesAnnouncement` | [announcements.go:294](../../../backend/modules/communication/parentstore/announcements.go#L294) | runtime retained | `r.audience.AccountMatchesAnnouncement`, `r.store.ListDeclarationSubmissions`, `r.audience.HistoricalDeclarationAccess`, `declarationHistoryLinksForAccount` |
| `*announcementRepository.ResolveAudienceEmails` | [announcements.go:306](../../../backend/modules/communication/parentstore/announcements.go#L306) | runtime retained | `r.audience.ResolveAudienceEmails`, `make`, `len`, `append` |
| `*announcementRepository.LetterChildStatuses` | [announcements.go:320](../../../backend/modules/communication/parentstore/announcements.go#L320) | runtime retained | `r.audience.LetterChildStatuses`, `make`, `len`, `append` |
| `*announcementRepository.ResolveDeliveryRecipients` | [announcements.go:336](../../../backend/modules/communication/parentstore/announcements.go#L336) | runtime retained | `r.audience.ResolveDeliveryRecipients`, `make`, `len`, `append` |
| `*announcementRepository.SchoolName` | [announcements.go:354](../../../backend/modules/communication/parentstore/announcements.go#L354) | direct binding | `errors.New` |
| `*announcementRepository.AudienceRecipients` | [announcements.go:358](../../../backend/modules/communication/parentstore/announcements.go#L358) | runtime retained | `r.audience.AudienceRecipients`, `make`, `len`, `append` |
| `*announcementRepository.ListFeedForAccount` | [announcements.go:373](../../../backend/modules/communication/parentstore/announcements.go#L373) | runtime retained | `feedScope`, `r.audience.ListFeedForAccount`, `r.store.ListDeclarationSubmissionsForAccountInTenants`, `feedTenantIDs`, `r.audience.HistoricalDeclarationFeed`, `declarationHistoryLinks`, `mergeFeedItems`, `make`, `len`, `append`, `valuesPolicy.AnnouncementResponseIsPoll` |
| `declarationHistoryLinksForAccount` | [announcements.go:403](../../../backend/modules/communication/parentstore/announcements.go#L403) | runtime retained | `make`, `len`, `append`, `declarationHistoryLinks` |
| `mergeFeedItems` | [announcements.go:413](../../../backend/modules/communication/parentstore/announcements.go#L413) | runtime retained | `make`, `len`, `len`, `make`, `len`, `append`, `sort.Slice`, `feedItemTime(merged[i]).After`, `feedItemTime`, `feedItemTime`, `feedItemTime(merged[i]).Equal`, `feedItemTime`, `feedItemTime` |
| `feedItemTime` | [announcements.go:432](../../../backend/modules/communication/parentstore/announcements.go#L432) | runtime retained | `item.ReminderSentAt.After` |
| `*announcementRepository.CountUnreadForAccount` | [announcements.go:442](../../../backend/modules/communication/parentstore/announcements.go#L442) | runtime retained | `r.audience.CountOutstandingForAccount`, `feedScope` |
| `*announcementRepository.CountReachableGuardiansForStudents` | [announcements.go:446](../../../backend/modules/communication/parentstore/announcements.go#L446) | direct binding | `r.audience.CountReachableGuardiansForStudents` |
| `*announcementRepository.MarkRead` | [announcements.go:450](../../../backend/modules/communication/parentstore/announcements.go#L450) | direct binding | `r.store.MarkRead` |
| `*announcementRepository.MarkAcknowledged` | [announcements.go:454](../../../backend/modules/communication/parentstore/announcements.go#L454) | direct binding | `r.store.MarkAcknowledged` |
| `*announcementRepository.Stats` | [announcements.go:458](../../../backend/modules/communication/parentstore/announcements.go#L458) | runtime retained | `r.audience.Stats` |
| `publishedError` | [announcements.go:470](../../../backend/modules/communication/parentstore/announcements.go#L470) | runtime retained | `errors.Is` |
| `feedScope` | [announcements.go:477](../../../backend/modules/communication/parentstore/announcements.go#L477) | direct binding | none |
| `feedTenantIDs` | [announcements.go:481](../../../backend/modules/communication/parentstore/announcements.go#L481) | runtime retained | `make`, `len`, `len`, `append`, `append` |
| `announcementValue` | [announcements.go:487](../../../backend/modules/communication/parentstore/announcements.go#L487) | runtime retained | `domain.DeclarationSettings` |
| `announcementModel` | [announcements.go:502](../../../backend/modules/communication/parentstore/announcements.go#L502) | runtime retained | `communication.PortalAnnouncementDeclarationSettings`, `make`, `len`, `append`, `targetModel`, `optionModels`, `stampAnnouncementKind` |
| `targetModel` | [announcements.go:532](../../../backend/modules/communication/parentstore/announcements.go#L532) | runtime retained | none |
| `optionModels` | [announcements.go:541](../../../backend/modules/communication/parentstore/announcements.go#L541) | runtime retained | `make`, `len`, `append` |
| `reminderModels` | [announcements.go:557](../../../backend/modules/communication/parentstore/announcements.go#L557) | runtime retained | `make`, `len`, `append` |
| `stampAnnouncementKind` | [announcements.go:573](../../../backend/modules/communication/parentstore/announcements.go#L573) | runtime retained | `valuesPolicy.AnnouncementIsPoll`, `(*valuesPolicy.ParentAnnouncement)`, `valuesPolicy.AnnouncementIsDeclaration`, `(*valuesPolicy.ParentAnnouncement)` |
| `*announcementRepository.LatestDeclarationVersion` | [declarations.go:16](../../../backend/modules/communication/parentstore/declarations.go#L16) | runtime retained | `r.store.LatestDeclarationVersion`, `declarationVersionModel` |
| `*announcementRepository.ListDeclarationVersions` | [declarations.go:24](../../../backend/modules/communication/parentstore/declarations.go#L24) | runtime retained | `r.store.ListDeclarationVersions`, `make`, `len`, `append`, `declarationVersionModel` |
| `*announcementRepository.LatestDeclarationVersions` | [declarations.go:36](../../../backend/modules/communication/parentstore/declarations.go#L36) | runtime retained | `r.store.LatestDeclarationVersions`, `make`, `len`, `declarationVersionModel` |
| `*announcementRepository.InsertDeclarationVersion` | [declarations.go:48](../../../backend/modules/communication/parentstore/declarations.go#L48) | runtime retained | `append`, `domain.DeclarationAttachmentDigest`, `r.store.InsertDeclarationVersion` |
| `*announcementRepository.DeclarationSettings` | [declarations.go:65](../../../backend/modules/communication/parentstore/declarations.go#L65) | runtime retained | `r.store.DeclarationSettings`, `make`, `len`, `communication.PortalAnnouncementDeclarationSettings` |
| `*announcementRepository.CountDeclarationSubmissions` | [declarations.go:77](../../../backend/modules/communication/parentstore/declarations.go#L77) | direct binding | `r.store.CountDeclarationSubmissions` |
| `*announcementRepository.ListDeclarationSubmissions` | [declarations.go:81](../../../backend/modules/communication/parentstore/declarations.go#L81) | runtime retained | `r.store.ListDeclarationSubmissions`, `declarationSubmissionModels` |
| `*announcementRepository.ListDeclarationSubmissionsForStudents` | [declarations.go:86](../../../backend/modules/communication/parentstore/declarations.go#L86) | runtime retained | `r.store.ListDeclarationSubmissionsForStudents`, `declarationSubmissionModels` |
| `*announcementRepository.LockDeclarationChild` | [declarations.go:91](../../../backend/modules/communication/parentstore/declarations.go#L91) | direct binding | `r.store.LockDeclarationChild` |
| `*announcementRepository.InsertDeclarationSubmission` | [declarations.go:95](../../../backend/modules/communication/parentstore/declarations.go#L95) | runtime retained | `domain.DeclarationSubmission`, `r.store.InsertDeclarationSubmission` |
| `*announcementRepository.DeclarationChildren` | [declarations.go:105](../../../backend/modules/communication/parentstore/declarations.go#L105) | runtime retained | `r.store.ListDeclarationSubmissions`, `r.audience.DeclarationChildren`, `declarationHistoryLinks`, `declarationChildModels` |
| `*announcementRepository.DeclarationSigners` | [declarations.go:114](../../../backend/modules/communication/parentstore/declarations.go#L114) | runtime retained | `r.audience.DeclarationSigners`, `declarationSignerModels` |
| `*announcementRepository.DeclarationChildrenForAccount` | [declarations.go:119](../../../backend/modules/communication/parentstore/declarations.go#L119) | runtime retained | `r.store.ListDeclarationSubmissionsForAccountAndAnnouncements`, `r.audience.DeclarationChildrenForAccount`, `declarationHistoryLinks`, `declarationChildModels` |
| `*announcementRepository.DeclarationSignersForStudents` | [declarations.go:128](../../../backend/modules/communication/parentstore/declarations.go#L128) | runtime retained | `r.audience.DeclarationSignersForStudents`, `declarationSignerModels` |
| `*announcementRepository.HoldDeclarationSigner` | [declarations.go:133](../../../backend/modules/communication/parentstore/declarations.go#L133) | runtime retained | `r.audience.HoldDeclarationSigner`, `communication.PortalDeclarationSignerContext` |
| `*announcementRepository.HoldHistoricalDeclarationSigner` | [declarations.go:142](../../../backend/modules/communication/parentstore/declarations.go#L142) | runtime retained | `r.store.ListDeclarationSubmissions`, `hasDeclarationSubmission`, `r.audience.HoldHistoricalDeclarationSigner`, `communication.PortalDeclarationSignerContext` |
| `declarationHistoryLinks` | [declarations.go:155](../../../backend/modules/communication/parentstore/declarations.go#L155) | runtime retained | `make`, `len`, `make`, `len`, `append` |
| `hasDeclarationSubmission` | [declarations.go:171](../../../backend/modules/communication/parentstore/declarations.go#L171) | runtime retained | none |
| `*announcementRepository.ReadOpenDeclarations` | [declarations.go:180](../../../backend/modules/communication/parentstore/declarations.go#L180) | direct binding | `r.store.ReadOpenDeclarations` |
| `declarationVersionModel` | [declarations.go:184](../../../backend/modules/communication/parentstore/declarations.go#L184) | runtime retained | `make`, `len`, `append`, `communication.PortalDeclarationAttachmentDigest` |
| `declarationSubmissionModels` | [declarations.go:196](../../../backend/modules/communication/parentstore/declarations.go#L196) | runtime retained | `make`, `len`, `communication.PortalDeclarationSubmission`, `append` |
| `declarationChildModels` | [declarations.go:205](../../../backend/modules/communication/parentstore/declarations.go#L205) | runtime retained | `make`, `len`, `communication.PortalDeclarationChild`, `append` |
| `declarationSignerModels` | [declarations.go:214](../../../backend/modules/communication/parentstore/declarations.go#L214) | runtime retained | `make`, `len`, `communication.PortalDeclarationSigner`, `append` |
| `*messageRepository.Create` | [messages.go:22](../../../backend/modules/communication/parentstore/messages.go#L22) | runtime retained | `messageValue`, `values.ParentPayloadFromFacts`, `r.store.Create` |
| `*messageRepository.Update` | [messages.go:46](../../../backend/modules/communication/parentstore/messages.go#L46) | runtime retained | `messageValue`, `r.store.Update` |
| `*messageRepository.FindByID` | [messages.go:54](../../../backend/modules/communication/parentstore/messages.go#L54) | runtime retained | `r.store.FindByID`, `messageModel` |
| `*messageRepository.FindByIDForUpdate` | [messages.go:63](../../../backend/modules/communication/parentstore/messages.go#L63) | runtime retained | `r.store.FindByIDForUpdate`, `messageModel` |
| `*messageRepository.FindEventByRef` | [messages.go:72](../../../backend/modules/communication/parentstore/messages.go#L72) | runtime retained | `r.store.FindEventByRef`, `messageModel` |
| `*messageRepository.ListByThread` | [messages.go:81](../../../backend/modules/communication/parentstore/messages.go#L81) | runtime retained | `r.store.ListByThread`, `make`, `len`, `messageModel`, `append` |
| `messageValue` | [messages.go:97](../../../backend/modules/communication/parentstore/messages.go#L97) | runtime retained | `values.ParentPayloadToFacts` |
| `messageModel` | [messages.go:119](../../../backend/modules/communication/parentstore/messages.go#L119) | runtime retained | `values.ParentPayloadFromFacts` |
| `*threadRepository.Create` | [messages.go:163](../../../backend/modules/communication/parentstore/messages.go#L163) | runtime retained | `threadValue`, `r.store.Create` |
| `*threadRepository.Update` | [messages.go:175](../../../backend/modules/communication/parentstore/messages.go#L175) | runtime retained | `r.store.Update`, `threadValue` |
| `*threadRepository.FindByID` | [messages.go:179](../../../backend/modules/communication/parentstore/messages.go#L179) | runtime retained | `r.store.FindByID`, `threadModel` |
| `*threadRepository.FindByStudentGuardian` | [messages.go:184](../../../backend/modules/communication/parentstore/messages.go#L184) | runtime retained | `r.store.FindByStudentGuardian`, `threadModel` |
| `*threadRepository.GetOrCreate` | [messages.go:189](../../../backend/modules/communication/parentstore/messages.go#L189) | runtime retained | `r.store.GetOrCreate`, `threadModel` |
| `*threadRepository.LockForMessageAppend` | [messages.go:197](../../../backend/modules/communication/parentstore/messages.go#L197) | direct binding | `r.store.LockForMessageAppend` |
| `*threadRepository.TouchLastMessage` | [messages.go:201](../../../backend/modules/communication/parentstore/messages.go#L201) | direct binding | `r.store.TouchLastMessage` |
| `*threadRepository.ClaimStaffMessageNotification` | [messages.go:205](../../../backend/modules/communication/parentstore/messages.go#L205) | direct binding | `r.store.ClaimStaffMessageNotification` |
| `*threadRepository.ListGuardiansForStudent` | [messages.go:209](../../../backend/modules/communication/parentstore/messages.go#L209) | direct binding | `r.guardians` |
| `threadValue` | [messages.go:213](../../../backend/modules/communication/parentstore/messages.go#L213) | runtime retained | none |
| `threadModel` | [messages.go:230](../../../backend/modules/communication/parentstore/messages.go#L230) | runtime retained | none |
| `*announcementRepository.AnnouncementClearEngagement` | [native_announcement_records.go:12](../../../backend/modules/communication/parentstore/native_announcement_records.go#L12) | direct binding | `r.ClearEngagement` |
| `*announcementRepository.AnnouncementSetReminder` | [native_announcement_records.go:15](../../../backend/modules/communication/parentstore/native_announcement_records.go#L15) | direct binding | `r.SetReminder` |
| `*announcementRepository.AnnouncementListDeclarationSubmissions` | [native_announcement_records.go:18](../../../backend/modules/communication/parentstore/native_announcement_records.go#L18) | direct binding | `r.ListDeclarationSubmissions` |
| `*announcementRepository.AnnouncementListDueReminders` | [native_announcement_records.go:21](../../../backend/modules/communication/parentstore/native_announcement_records.go#L21) | direct binding | `r.ListDueReminders` |
| `*announcementRepository.AnnouncementClaimReminder` | [native_announcement_records.go:24](../../../backend/modules/communication/parentstore/native_announcement_records.go#L24) | direct binding | `r.ClaimReminder` |
| `*announcementRepository.AnnouncementCountDeclarationSubmissions` | [native_announcement_records.go:27](../../../backend/modules/communication/parentstore/native_announcement_records.go#L27) | direct binding | `r.CountDeclarationSubmissions` |
| `*announcementRepository.AnnouncementAudienceRecipients` | [native_announcement_records.go:30](../../../backend/modules/communication/parentstore/native_announcement_records.go#L30) | direct binding | `r.AudienceRecipients` |
| `*announcementRepository.AnnouncementReplaceOptions` | [native_announcement_records.go:33](../../../backend/modules/communication/parentstore/native_announcement_records.go#L33) | direct binding | `r.ReplaceOptions` |
| `*announcementRepository.AnnouncementLatestDeclarationVersion` | [native_announcement_records.go:36](../../../backend/modules/communication/parentstore/native_announcement_records.go#L36) | direct binding | `r.LatestDeclarationVersion` |
| `*announcementRepository.AnnouncementReplaceTargets` | [native_announcement_records.go:39](../../../backend/modules/communication/parentstore/native_announcement_records.go#L39) | direct binding | `r.ReplaceTargets` |
| `*announcementRepository.AnnouncementSetPublished` | [native_announcement_records.go:42](../../../backend/modules/communication/parentstore/native_announcement_records.go#L42) | direct binding | `r.SetPublished` |
| `*announcementRepository.AnnouncementInsertDeclarationVersion` | [native_announcement_records.go:45](../../../backend/modules/communication/parentstore/native_announcement_records.go#L45) | direct binding | `r.InsertDeclarationVersion` |
| `*announcementRepository.AnnouncementFindByIDForUpdate` | [native_announcement_records.go:48](../../../backend/modules/communication/parentstore/native_announcement_records.go#L48) | direct binding | `r.FindByIDForUpdate` |
| `*announcementRepository.AnnouncementDeclarationSigners` | [native_announcement_records.go:51](../../../backend/modules/communication/parentstore/native_announcement_records.go#L51) | direct binding | `r.DeclarationSigners` |
| `*announcementRepository.AnnouncementPollResults` | [native_announcement_records.go:54](../../../backend/modules/communication/parentstore/native_announcement_records.go#L54) | direct binding | `r.PollResults` |
| `*announcementRepository.AnnouncementLetterChildStatuses` | [native_announcement_records.go:57](../../../backend/modules/communication/parentstore/native_announcement_records.go#L57) | direct binding | `r.LetterChildStatuses` |
| `*announcementRepository.AnnouncementDeclarationChildren` | [native_announcement_records.go:60](../../../backend/modules/communication/parentstore/native_announcement_records.go#L60) | direct binding | `r.DeclarationChildren` |
| `*announcementRepository.AnnouncementReleaseReminderClaim` | [native_announcement_records.go:63](../../../backend/modules/communication/parentstore/native_announcement_records.go#L63) | direct binding | `r.ReleaseReminderClaim` |
| `*announcementRepository.AnnouncementDelete` | [native_announcement_records.go:66](../../../backend/modules/communication/parentstore/native_announcement_records.go#L66) | direct binding | `r.Delete` |
| `*announcementRepository.AnnouncementPublishIfDraft` | [native_announcement_records.go:69](../../../backend/modules/communication/parentstore/native_announcement_records.go#L69) | direct binding | `r.PublishIfDraft` |
| `*announcementRepository.AnnouncementListDeclarationVersions` | [native_announcement_records.go:72](../../../backend/modules/communication/parentstore/native_announcement_records.go#L72) | direct binding | `r.ListDeclarationVersions` |
| `*announcementRepository.AnnouncementUnansweredReminderRecipients` | [native_announcement_records.go:75](../../../backend/modules/communication/parentstore/native_announcement_records.go#L75) | direct binding | `r.UnansweredReminderRecipients` |
| `*announcementRepository.AnnouncementSchoolName` | [native_announcement_records.go:78](../../../backend/modules/communication/parentstore/native_announcement_records.go#L78) | direct binding | `r.SchoolName` |
| `*announcementRepository.AnnouncementResolveAudienceEmails` | [native_announcement_records.go:81](../../../backend/modules/communication/parentstore/native_announcement_records.go#L81) | direct binding | `r.ResolveAudienceEmails` |
| `*announcementRepository.AnnouncementStats` | [native_announcement_records.go:84](../../../backend/modules/communication/parentstore/native_announcement_records.go#L84) | direct binding | `r.Stats` |
| `*announcementRepository.AnnouncementListTargets` | [native_announcement_records.go:87](../../../backend/modules/communication/parentstore/native_announcement_records.go#L87) | direct binding | `r.ListTargets` |
| `*announcementRepository.AnnouncementPollChildren` | [native_announcement_records.go:90](../../../backend/modules/communication/parentstore/native_announcement_records.go#L90) | direct binding | `r.PollChildren` |
| `*announcementRepository.AnnouncementUpdate` | [native_announcement_records.go:93](../../../backend/modules/communication/parentstore/native_announcement_records.go#L93) | direct binding | `r.Update` |
| `*announcementRepository.AnnouncementResolveDeliveryRecipients` | [native_announcement_records.go:96](../../../backend/modules/communication/parentstore/native_announcement_records.go#L96) | direct binding | `r.ResolveDeliveryRecipients` |
| `*announcementRepository.AnnouncementCountReachableGuardiansForStudents` | [native_announcement_records.go:99](../../../backend/modules/communication/parentstore/native_announcement_records.go#L99) | direct binding | `r.CountReachableGuardiansForStudents` |
| `*announcementRepository.AnnouncementUnacknowledgedReminderRecipients` | [native_announcement_records.go:102](../../../backend/modules/communication/parentstore/native_announcement_records.go#L102) | direct binding | `r.UnacknowledgedReminderRecipients` |
| `*announcementRepository.AnnouncementListForTenant` | [native_announcement_records.go:105](../../../backend/modules/communication/parentstore/native_announcement_records.go#L105) | direct binding | `r.ListForTenant` |
| `*announcementRepository.AnnouncementCreate` | [native_announcement_records.go:108](../../../backend/modules/communication/parentstore/native_announcement_records.go#L108) | direct binding | `r.Create` |
| `*announcementRepository.IsParentAnnouncementPublishedConflict` | [native_announcement_records.go:111](../../../backend/modules/communication/parentstore/native_announcement_records.go#L111) | direct binding | `errors.Is` |
| `*messageRepository.ParentMessageCreate` | [native_parent_records.go:10](../../../backend/modules/communication/parentstore/native_parent_records.go#L10) | direct binding | `r.Create` |
| `*messageRepository.ParentMessageListByThread` | [native_parent_records.go:13](../../../backend/modules/communication/parentstore/native_parent_records.go#L13) | direct binding | `r.ListByThread` |
| `*messageRepository.ParentMessageFindEventByRef` | [native_parent_records.go:16](../../../backend/modules/communication/parentstore/native_parent_records.go#L16) | direct binding | `r.FindEventByRef` |
| `*threadRepository.ParentThreadLockForMessageAppend` | [native_parent_records.go:19](../../../backend/modules/communication/parentstore/native_parent_records.go#L19) | direct binding | `r.LockForMessageAppend` |
| `*threadRepository.ParentThreadTouchLastMessage` | [native_parent_records.go:22](../../../backend/modules/communication/parentstore/native_parent_records.go#L22) | direct binding | `r.TouchLastMessage` |
| `*threadRepository.ParentThreadFindByID` | [native_parent_records.go:25](../../../backend/modules/communication/parentstore/native_parent_records.go#L25) | direct binding | `r.FindByID` |
| `*threadRepository.ParentThreadFindByStudentGuardian` | [native_parent_records.go:28](../../../backend/modules/communication/parentstore/native_parent_records.go#L28) | direct binding | `r.FindByStudentGuardian` |
| `*threadRepository.ParentThreadGetOrCreate` | [native_parent_records.go:31](../../../backend/modules/communication/parentstore/native_parent_records.go#L31) | direct binding | `r.GetOrCreate` |
| `*threadRepository.ParentThreadListGuardiansForStudent` | [native_parent_records.go:34](../../../backend/modules/communication/parentstore/native_parent_records.go#L34) | direct binding | `r.ListGuardiansForStudent` |
| `parentMessageStaffRepository.ParentReadMarkReadUpTo` | [native_parent_records.go:37](../../../backend/modules/communication/parentstore/native_parent_records.go#L37) | direct binding | `r.MarkReadUpTo` |
| `parentMessageStaffRepository.ParentReadClearUnreadForStaff` | [native_parent_records.go:40](../../../backend/modules/communication/parentstore/native_parent_records.go#L40) | direct binding | `r.ClearUnreadForStaff` |
| `parentMessageStaffRepository.ParentReadMarkStaffHandledUpTo` | [native_parent_records.go:43](../../../backend/modules/communication/parentstore/native_parent_records.go#L43) | direct binding | `r.MarkStaffHandledUpTo` |
| `parentMessageStaffRepository.ParentReadMarkStaffUnread` | [native_parent_records.go:46](../../../backend/modules/communication/parentstore/native_parent_records.go#L46) | direct binding | `r.MarkStaffUnread` |
| `parentMessageStaffRepository.ParentReadClearStaffUnreadMark` | [native_parent_records.go:49](../../../backend/modules/communication/parentstore/native_parent_records.go#L49) | direct binding | `r.ClearStaffUnreadMark` |
| `parentMessageStaffRepository.ParentReadUnreadMessageCountForStaff` | [native_parent_records.go:52](../../../backend/modules/communication/parentstore/native_parent_records.go#L52) | direct binding | `r.UnreadMessageCountForStaff` |
| `parentMessageStaffRepository.ParentReadUnreadMessageCountForStaffInGroups` | [native_parent_records.go:55](../../../backend/modules/communication/parentstore/native_parent_records.go#L55) | direct binding | `r.UnreadMessageCountForStaffInGroups` |
| `parentMessageStaffRepository.ParentReadListInboxForStaff` | [native_parent_records.go:58](../../../backend/modules/communication/parentstore/native_parent_records.go#L58) | direct binding | `r.ListInboxForStaff` |
| `parentMessageStaffRepository.ParentReadListThreadsForStudent` | [native_parent_records.go:61](../../../backend/modules/communication/parentstore/native_parent_records.go#L61) | direct binding | `r.ListThreadsForStudent` |
| `parentMessageStaffRepository.ParentReadFindThreadHeader` | [native_parent_records.go:64](../../../backend/modules/communication/parentstore/native_parent_records.go#L64) | direct binding | `r.FindThreadHeader` |
| `parentMessageStaffRepository.ParentReadLatestReadCursorByOther` | [native_parent_records.go:67](../../../backend/modules/communication/parentstore/native_parent_records.go#L67) | direct binding | `r.LatestReadCursorByOther` |
| `parentMessageStaffRepository.ParentReadGuardianReadCursor` | [native_parent_records.go:70](../../../backend/modules/communication/parentstore/native_parent_records.go#L70) | direct binding | `r.GuardianReadCursor` |
| `schoolParentMessageReadRepository.ParentReadListInboxForStaff` | [native_parent_records.go:73](../../../backend/modules/communication/parentstore/native_parent_records.go#L73) | direct binding | `r.ListInboxForStaff` |
| `schoolParentMessageReadRepository.ParentReadListThreadsForStudent` | [native_parent_records.go:76](../../../backend/modules/communication/parentstore/native_parent_records.go#L76) | direct binding | `r.ListThreadsForStudent` |
| `groupParentMessageReadRepository.ParentReadListInboxForStaff` | [native_parent_records.go:79](../../../backend/modules/communication/parentstore/native_parent_records.go#L79) | direct binding | `r.ListInboxForStaff` |
| `groupParentMessageReadRepository.ParentReadListThreadsForStudent` | [native_parent_records.go:82](../../../backend/modules/communication/parentstore/native_parent_records.go#L82) | direct binding | `r.ListThreadsForStudent` |
| `resolve` | [parentstore.go:24](../../../backend/modules/communication/parentstore/parentstore.go#L24) | runtime retained | `errors.New`, `tenant.FromContext`, `tenant.IsAdminTx`, `tenant.TransactionFromContext`, `errors.New`, `errors.New` |
| `postgresDatabase` | [parentstore.go:54](../../../backend/modules/communication/parentstore/parentstore.go#L54) | runtime retained | `resolve`, `parentpostgres.ScopeFor`, `tenant.IsAdminTx`, `scope.Validate` |
| `ProjectionDatabase` | [parentstore.go:66](../../../backend/modules/communication/parentstore/parentstore.go#L66) | direct binding | `resolve` |
| `*announcementRepository.PortalDeclarationChildrenForAccount` | [portal_announcement_declarations_ports.go:9](../../../backend/modules/communication/parentstore/portal_announcement_declarations_ports.go#L9) | direct binding | `r.DeclarationChildrenForAccount` |
| `*announcementRepository.PortalDeclarationSettings` | [portal_announcement_declarations_ports.go:12](../../../backend/modules/communication/parentstore/portal_announcement_declarations_ports.go#L12) | direct binding | `r.DeclarationSettings` |
| `*announcementRepository.PortalDeclarationSignersForStudents` | [portal_announcement_declarations_ports.go:15](../../../backend/modules/communication/parentstore/portal_announcement_declarations_ports.go#L15) | direct binding | `r.DeclarationSignersForStudents` |
| `*announcementRepository.PortalLatestDeclarationVersion` | [portal_announcement_declarations_ports.go:18](../../../backend/modules/communication/parentstore/portal_announcement_declarations_ports.go#L18) | direct binding | `r.LatestDeclarationVersion` |
| `*announcementRepository.PortalLatestDeclarationVersions` | [portal_announcement_declarations_ports.go:21](../../../backend/modules/communication/parentstore/portal_announcement_declarations_ports.go#L21) | direct binding | `r.LatestDeclarationVersions` |
| `*announcementRepository.PortalListDeclarationSubmissionsForStudents` | [portal_announcement_declarations_ports.go:24](../../../backend/modules/communication/parentstore/portal_announcement_declarations_ports.go#L24) | direct binding | `r.ListDeclarationSubmissionsForStudents` |
| `*announcementRepository.PortalListDeclarationVersions` | [portal_announcement_declarations_ports.go:27](../../../backend/modules/communication/parentstore/portal_announcement_declarations_ports.go#L27) | direct binding | `r.ListDeclarationVersions` |
| `*announcementRepository.PortalHoldDeclarationSigner` | [portal_announcement_declarations_ports.go:30](../../../backend/modules/communication/parentstore/portal_announcement_declarations_ports.go#L30) | direct binding | `r.HoldDeclarationSigner` |
| `*announcementRepository.PortalHoldHistoricalDeclarationSigner` | [portal_announcement_declarations_ports.go:33](../../../backend/modules/communication/parentstore/portal_announcement_declarations_ports.go#L33) | direct binding | `r.HoldHistoricalDeclarationSigner` |
| `*announcementRepository.PortalInsertDeclarationSubmission` | [portal_announcement_declarations_ports.go:36](../../../backend/modules/communication/parentstore/portal_announcement_declarations_ports.go#L36) | direct binding | `r.InsertDeclarationSubmission` |
| `*announcementRepository.PortalAnswerableChildren` | [portal_announcement_feed_ports.go:9](../../../backend/modules/communication/parentstore/portal_announcement_feed_ports.go#L9) | direct binding | `r.AnswerableChildren` |
| `*announcementRepository.PortalCountUnreadForAccount` | [portal_announcement_feed_ports.go:12](../../../backend/modules/communication/parentstore/portal_announcement_feed_ports.go#L12) | direct binding | `r.CountUnreadForAccount` |
| `*announcementRepository.PortalFindByID` | [portal_announcement_feed_ports.go:15](../../../backend/modules/communication/parentstore/portal_announcement_feed_ports.go#L15) | direct binding | `r.FindByID` |
| `*announcementRepository.PortalListFeedForAccount` | [portal_announcement_feed_ports.go:18](../../../backend/modules/communication/parentstore/portal_announcement_feed_ports.go#L18) | direct binding | `r.ListFeedForAccount` |
| `*announcementRepository.PortalListOptions` | [portal_announcement_feed_ports.go:21](../../../backend/modules/communication/parentstore/portal_announcement_feed_ports.go#L21) | direct binding | `r.ListOptions` |
| `*announcementRepository.PortalListOptionsForAnnouncements` | [portal_announcement_feed_ports.go:24](../../../backend/modules/communication/parentstore/portal_announcement_feed_ports.go#L24) | direct binding | `r.ListOptionsForAnnouncements` |
| `schoolParentAnnouncementRepository.PortalListFeedForAccount` | [portal_announcement_feed_ports.go:27](../../../backend/modules/communication/parentstore/portal_announcement_feed_ports.go#L27) | direct binding | `r.ListFeedForAccount` |
| `portalDeclarationPolicy.PortalCanonicalRecord` | [portal_declaration_policy.go:16](../../../backend/modules/communication/parentstore/portal_declaration_policy.go#L16) | runtime retained | `values.CanonicalDeclarationRecord`, `(*values.DeclarationSubmission)` |
| `portalDeclarationPolicy.PortalCanonicalContent` | [portal_declaration_policy.go:19](../../../backend/modules/communication/parentstore/portal_declaration_policy.go#L19) | runtime retained | `values.CanonicalDeclarationContent`, `(*values.DeclarationVersion)` |
| `*messageRepository.PortalListByThread` | [portal_message_read_ports.go:9](../../../backend/modules/communication/parentstore/portal_message_read_ports.go#L9) | direct binding | `r.ListByThread` |
| `parentMessageStaffRepository.PortalListThreadsForGuardianStudent` | [portal_message_read_ports.go:12](../../../backend/modules/communication/parentstore/portal_message_read_ports.go#L12) | direct binding | `r.ListThreadsForGuardianStudent` |
| `parentMessageStaffRepository.PortalListThreadsForGuardianTenants` | [portal_message_read_ports.go:15](../../../backend/modules/communication/parentstore/portal_message_read_ports.go#L15) | direct binding | `r.ListThreadsForGuardianTenants` |
| `schoolParentMessageReadRepository.PortalListThreadsForGuardianStudent` | [portal_message_read_ports.go:18](../../../backend/modules/communication/parentstore/portal_message_read_ports.go#L18) | direct binding | `r.ListThreadsForGuardianStudent` |
| `schoolParentMessageReadRepository.PortalListThreadsForGuardianTenants` | [portal_message_read_ports.go:21](../../../backend/modules/communication/parentstore/portal_message_read_ports.go#L21) | direct binding | `r.ListThreadsForGuardianTenants` |
| `groupParentMessageReadRepository.PortalListThreadsForGuardianStudent` | [portal_message_read_ports.go:24](../../../backend/modules/communication/parentstore/portal_message_read_ports.go#L24) | direct binding | `r.ListThreadsForGuardianStudent` |
| `groupParentMessageReadRepository.PortalListThreadsForGuardianTenants` | [portal_message_read_ports.go:27](../../../backend/modules/communication/parentstore/portal_message_read_ports.go#L27) | direct binding | `r.ListThreadsForGuardianTenants` |
| `*threadRepository.PortalFindByStudentGuardian` | [portal_thread_ports.go:9](../../../backend/modules/communication/parentstore/portal_thread_ports.go#L9) | direct binding | `r.FindByStudentGuardian` |
| `*threadRepository.PortalGetOrCreate` | [portal_thread_ports.go:12](../../../backend/modules/communication/parentstore/portal_thread_ports.go#L12) | direct binding | `r.GetOrCreate` |
| `*ParentMessageReads.MarkReadUpTo` | [reads.go:38](../../../backend/modules/communication/parentstore/reads.go#L38) | direct binding | `r.cursors.MarkReadUpTo` |
| `*ParentMessageReads.ClearUnreadForStaff` | [reads.go:42](../../../backend/modules/communication/parentstore/reads.go#L42) | runtime retained | `make`, `len`, `append`, `r.cursors.ClearUnreadForStaff` |
| `*ParentMessageReads.MarkStaffHandledUpTo` | [reads.go:52](../../../backend/modules/communication/parentstore/reads.go#L52) | direct binding | `r.threads.MarkStaffHandledUpTo` |
| `*ParentMessageReads.MarkStaffUnread` | [reads.go:56](../../../backend/modules/communication/parentstore/reads.go#L56) | direct binding | `r.threads.MarkStaffUnread` |
| `*ParentMessageReads.ClearStaffUnreadMark` | [reads.go:60](../../../backend/modules/communication/parentstore/reads.go#L60) | direct binding | `r.threads.ClearStaffUnreadMark` |
| `*ParentMessageReads.UnreadMessageCountForStaff` | [reads.go:64](../../../backend/modules/communication/parentstore/reads.go#L64) | direct binding | `r.inbox.UnreadMessageCountForStaff` |
| `*ParentMessageReads.UnreadMessageCountForStaffInGroups` | [reads.go:68](../../../backend/modules/communication/parentstore/reads.go#L68) | direct binding | `r.inbox.UnreadMessageCountForStaffInGroups` |
| `*ParentMessageReads.ListInboxForStaff` | [reads.go:72](../../../backend/modules/communication/parentstore/reads.go#L72) | runtime retained | `r.inbox.ListInboxForStaff`, `inboxModels` |
| `*ParentMessageReads.ListThreadsForStudent` | [reads.go:81](../../../backend/modules/communication/parentstore/reads.go#L81) | runtime retained | `r.inbox.ListThreadsForStudent`, `inboxModels` |
| `*ParentMessageReads.ListThreadsForGuardianStudentWithStaff` | [reads.go:90](../../../backend/modules/communication/parentstore/reads.go#L90) | runtime retained | `r.inbox.ListThreadsForGuardianStudent`, `inboxModels` |
| `*ParentMessageReads.ListThreadsForGuardianTenantsWithStaff` | [reads.go:99](../../../backend/modules/communication/parentstore/reads.go#L99) | runtime retained | `r.inbox.ListThreadsForGuardianTenants`, `inboxModels` |
| `*ParentMessageReads.UnreadMessageCountForGuardianTenants` | [reads.go:108](../../../backend/modules/communication/parentstore/reads.go#L108) | direct binding | `r.inbox.UnreadMessageCountForGuardianTenants` |
| `*ParentMessageReads.FindThreadHeader` | [reads.go:112](../../../backend/modules/communication/parentstore/reads.go#L112) | runtime retained | `r.inbox.FindThreadHeader` |
| `*ParentMessageReads.LatestReadCursorByOtherStaff` | [reads.go:123](../../../backend/modules/communication/parentstore/reads.go#L123) | runtime retained | `r.inbox.LatestReadCursorByOtherStaff`, `cursorModel` |
| `*ParentMessageReads.GuardianReadCursor` | [reads.go:128](../../../backend/modules/communication/parentstore/reads.go#L128) | runtime retained | `r.inbox.GuardianReadCursor`, `cursorModel` |
| `cursorModel` | [reads.go:133](../../../backend/modules/communication/parentstore/reads.go#L133) | runtime retained | none |
| `inboxModels` | [reads.go:140](../../../backend/modules/communication/parentstore/reads.go#L140) | runtime retained | `make`, `len`, `json.Unmarshal`, `len`, `values.ParentPayloadFromFacts`, `append` |
| `WithInboxSchoolNames` | [school_projection.go:22](../../../backend/modules/communication/parentstore/school_projection.go#L22) | direct binding | none |
| `WithAnnouncementSchoolNames` | [school_projection.go:28](../../../backend/modules/communication/parentstore/school_projection.go#L28) | direct binding | none |
| `schoolParentAnnouncementRepository.AnnouncementSchoolName` | [school_projection.go:37](../../../backend/modules/communication/parentstore/school_projection.go#L37) | direct binding | `r.SchoolName` |
| `schoolParentAnnouncementRepository.SchoolName` | [school_projection.go:41](../../../backend/modules/communication/parentstore/school_projection.go#L41) | runtime retained | `r.schools.SchoolName`, `fmt.Errorf` |
| `schoolParentAnnouncementRepository.ListFeedForAccount` | [school_projection.go:49](../../../backend/modules/communication/parentstore/school_projection.go#L49) | runtime retained | `r.ParentAnnouncementRepository.ListFeedForAccount`, `len`, `make`, `len`, `append`, `r.schools.SchoolNames`, `fmt.Errorf`, `fmt.Errorf` |
| `schoolParentMessageReadRepository.ListInboxForStaff` | [school_projection.go:77](../../../backend/modules/communication/parentstore/school_projection.go#L77) | runtime retained | `r.ParentMessageReadRepository.ListInboxForStaff`, `enrichInboxSchools` |
| `schoolParentMessageReadRepository.ListThreadsForStudent` | [school_projection.go:82](../../../backend/modules/communication/parentstore/school_projection.go#L82) | runtime retained | `r.ParentMessageReadRepository.ListThreadsForStudent`, `enrichInboxSchools` |
| `schoolParentMessageReadRepository.ListThreadsForGuardianStudent` | [school_projection.go:87](../../../backend/modules/communication/parentstore/school_projection.go#L87) | runtime retained | `r.ParentMessageReadRepository.ListThreadsForGuardianStudent`, `enrichInboxSchools` |
| `schoolParentMessageReadRepository.ListThreadsForGuardianTenants` | [school_projection.go:92](../../../backend/modules/communication/parentstore/school_projection.go#L92) | runtime retained | `r.ParentMessageReadRepository.ListThreadsForGuardianTenants`, `enrichInboxSchools` |
| `enrichInboxSchools` | [school_projection.go:97](../../../backend/modules/communication/parentstore/school_projection.go#L97) | runtime retained | `len`, `make`, `len`, `append`, `schools.SchoolNames`, `fmt.Errorf`, `fmt.Errorf` |
| `MessagingRepositories.WithSchoolNames` | [school_projection.go:131](../../../backend/modules/communication/parentstore/school_projection.go#L131) | runtime retained | `WithAnnouncementSchoolNames`, `WithInboxSchoolNames` |
| `WithInboxGroupNames` | [staff_projection.go:41](../../../backend/modules/communication/parentstore/staff_projection.go#L41) | direct binding | none |
| `CurrentTenantStaffAccounts` | [staff_projection.go:48](../../../backend/modules/communication/parentstore/staff_projection.go#L48) | runtime retained | `staffAccountsByTenant`, `make`, `len`, `append` |
| `staffAccountsByTenant` | [staff_projection.go:68](../../../backend/modules/communication/parentstore/staff_projection.go#L68) | runtime retained | `listMembers`, `fmt.Errorf`, `len`, `make`, `len`, `append`, `persons`, `fmt.Errorf`, `make`, `make`, `len`, `append` |
| `parentMessageStaffRepository.staffAccounts` | [staff_projection.go:109](../../../backend/modules/communication/parentstore/staff_projection.go#L109) | direct binding | `staffAccountsByTenant` |
| `parentMessageStaffRepository.ListThreadsForGuardianStudent` | [staff_projection.go:113](../../../backend/modules/communication/parentstore/staff_projection.go#L113) | runtime retained | `r.staffAccounts`, `r.ListThreadsForGuardianStudentWithStaff` |
| `parentMessageStaffRepository.ListThreadsForGuardianTenants` | [staff_projection.go:121](../../../backend/modules/communication/parentstore/staff_projection.go#L121) | runtime retained | `len`, `staffAccountsByTenant`, `r.ListThreadsForGuardianTenantsWithStaff` |
| `parentMessageStaffRepository.LatestReadCursorByOther` | [staff_projection.go:132](../../../backend/modules/communication/parentstore/staff_projection.go#L132) | runtime retained | `r.staffAccounts`, `r.LatestReadCursorByOtherStaff` |
| `groupParentMessageReadRepository.ListInboxForStaff` | [staff_projection.go:145](../../../backend/modules/communication/parentstore/staff_projection.go#L145) | runtime retained | `r.ParentMessageReadRepository.ListInboxForStaff`, `enrichInboxGroups` |
| `groupParentMessageReadRepository.ListThreadsForStudent` | [staff_projection.go:150](../../../backend/modules/communication/parentstore/staff_projection.go#L150) | runtime retained | `r.ParentMessageReadRepository.ListThreadsForStudent`, `enrichInboxGroups` |
| `groupParentMessageReadRepository.ListThreadsForGuardianStudent` | [staff_projection.go:155](../../../backend/modules/communication/parentstore/staff_projection.go#L155) | runtime retained | `r.ParentMessageReadRepository.ListThreadsForGuardianStudent`, `enrichInboxGroups` |
| `groupParentMessageReadRepository.ListThreadsForGuardianTenants` | [staff_projection.go:160](../../../backend/modules/communication/parentstore/staff_projection.go#L160) | runtime retained | `r.ParentMessageReadRepository.ListThreadsForGuardianTenants`, `enrichInboxGroups` |
| `enrichInboxGroups` | [staff_projection.go:168](../../../backend/modules/communication/parentstore/staff_projection.go#L168) | runtime retained | `len`, `make`, `len`, `append`, `inboxGroupID`, `groupNames`, `fmt.Errorf`, `inboxGroupID` |
| `inboxGroupID` | [staff_projection.go:189](../../../backend/modules/communication/parentstore/staff_projection.go#L189) | runtime retained | none |
| `*StudentConversations.CountStudentConversationRecords` | [student_conversations.go:24](../../../backend/modules/communication/parentstore/student_conversations.go#L24) | direct binding | `c.store.CountRecords` |
| `*StudentConversations.LockStudentMessageThreads` | [student_conversations.go:31](../../../backend/modules/communication/parentstore/student_conversations.go#L31) | direct binding | `c.store.LockThreads` |

### `modules/communication/staffstore`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `*readRepository.MarkStaffReadUpTo` | [repositories.go:34](../../../backend/modules/communication/staffstore/repositories.go#L34) | direct binding | `r.cursors.MarkReadUpTo` |
| `*readRepository.StaffUnreadCount` | [repositories.go:37](../../../backend/modules/communication/staffstore/repositories.go#L37) | direct binding | `r.inbox.UnreadCount` |
| `*readRepository.ListStaffInbox` | [repositories.go:40](../../../backend/modules/communication/staffstore/repositories.go#L40) | runtime retained | `r.inbox.ListInbox`, `make`, `len`, `append` |
| `resolve` | [staffstore.go:33](../../../backend/modules/communication/staffstore/staffstore.go#L33) | runtime retained | `errors.New`, `tenant.FromContext`, `tenant.IsAdminTx`, `tenant.TransactionFromContext`, `errors.New`, `fmt.Errorf` |
| `postgresDatabase` | [staffstore.go:62](../../../backend/modules/communication/staffstore/staffstore.go#L62) | direct binding | `resolve` |
| `ProjectionDatabase` | [staffstore.go:67](../../../backend/modules/communication/staffstore/staffstore.go#L67) | direct binding | `resolve` |

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
| `groupDirectory.Find` | [catalog.go:14](../../../backend/modules/devicescan/compose/catalog.go#L14) | runtime retained | `g.education.GetGroup` |
| `pickupPlan.Effective` | [catalog.go:29](../../../backend/modules/devicescan/compose/catalog.go#L29) | runtime retained | `p.pickups.GetEffectivePickupTimeForDate`, `append` |
| `clock.Now` | [compose.go:141](../../../backend/modules/devicescan/compose/compose.go#L141) | direct binding | `c.now` |
| `clock.Day` | [compose.go:142](../../../backend/modules/devicescan/compose/compose.go#L142) | direct binding | `calendar.DateFromTime` |
| `unitOfWork.RequireTransaction` | [compose.go:147](../../../backend/modules/devicescan/compose/compose.go#L147) | direct binding | `application.RequireTransaction` |
| `unitOfWork.MarkRollback` | [compose.go:151](../../../backend/modules/devicescan/compose/compose.go#L151) | direct binding | `tenant.MarkRollback` |
| `mappedError.Error` | [compose.go:160](../../../backend/modules/devicescan/compose/compose.go#L160) | direct binding | `e.err.Error` |
| `mappedError.Unwrap` | [compose.go:161](../../../backend/modules/devicescan/compose/compose.go#L161) | direct binding | none |
| `mappedError.Is` | [compose.go:162](../../../backend/modules/devicescan/compose/compose.go#L162) | direct binding | none |
| `mapError` | [compose.go:164](../../../backend/modules/devicescan/compose/compose.go#L164) | direct binding | none |
| `directoryPeople.Teachers` | [directory.go:32](../../../backend/modules/devicescan/compose/directory.go#L32) | direct binding | `application.TeachersBinding` |
| `feedbackStudents.LockFeedbackStudent` | [feedback_students.go:23](../../../backend/modules/devicescan/compose/feedback_students.go#L23) | direct binding | `application.LockFeedbackStudentBinding` |
| `openRoomMover.MoveToOpenRoom` | [open_room.go:56](../../../backend/modules/devicescan/compose/open_room.go#L56) | runtime retained | `m.move`, `m.translate`, `slices.Contains` |
| `openRoomMover.translate` | [open_room.go:76](../../../backend/modules/devicescan/compose/open_room.go#L76) | runtime retained | `errors.AsType`, `errors.Is`, `fmt.Errorf` |
| `people.FindPersonByTag` | [people.go:29](../../../backend/modules/devicescan/compose/people.go#L29) | direct binding | `application.FindPersonByTagBinding` |
| `people.FindStudentByPerson` | [people.go:33](../../../backend/modules/devicescan/compose/people.go#L33) | direct binding | `application.FindStudentByPersonBinding` |
| `people.FindStaffByPerson` | [people.go:37](../../../backend/modules/devicescan/compose/people.go#L37) | direct binding | `application.FindStaffByPersonBinding` |
| `classifyPresence` | [presence.go:18](../../../backend/modules/devicescan/compose/presence.go#L18) | runtime retained | `errors.Is`, `mapError`, `err.` |
| `visits.Current` | [presence.go:62](../../../backend/modules/devicescan/compose/presence.go#L62) | runtime retained | `v.active.GetStudentCurrentVisitWithRoom`, `errors.Is` |
| `visits.End` | [presence.go:83](../../../backend/modules/devicescan/compose/presence.go#L83) | direct binding | `v.active.EndVisit` |
| `visits.Record` | [presence.go:85](../../../backend/modules/devicescan/compose/presence.go#L85) | runtime retained | `v.clock.Now`, `v.active.CreateVisit`, `errors.As`, `errors.Is`, `mapError` |
| `sessionFromGroup` | [presence.go:119](../../../backend/modules/devicescan/compose/presence.go#L119) | runtime retained | none |
| `sessions.ListOpenInRoom` | [presence.go:138](../../../backend/modules/devicescan/compose/presence.go#L138) | runtime retained | `s.presence.QueryLiveGroups`, `fmt.Errorf`, `make`, `len`, `append` |
| `sessions.FindDeviceSessionInRoom` | [presence.go:153](../../../backend/modules/devicescan/compose/presence.go#L153) | runtime retained | `s.active.FindDeviceActiveGroupInRoom`, `sessionFromGroup` |
| `sessions.Current` | [presence.go:165](../../../backend/modules/devicescan/compose/presence.go#L165) | runtime retained | `s.active.GetDeviceCurrentSession`, `errors.Is`, `sessionFromGroup` |
| `sessions.Start` | [presence.go:180](../../../backend/modules/devicescan/compose/presence.go#L180) | runtime retained | `s.clock.Now`, `s.active.CreateActiveGroup` |
| `sessions.EnsureRoomSession` | [presence.go:193](../../../backend/modules/devicescan/compose/presence.go#L193) | runtime retained | `s.active.EnsureOpenRoomSession`, `sessionFromGroup` |
| `sessions.Delete` | [presence.go:201](../../../backend/modules/devicescan/compose/presence.go#L201) | direct binding | `s.active.DeleteActiveGroup` |
| `sessions.End` | [presence.go:205](../../../backend/modules/devicescan/compose/presence.go#L205) | runtime retained | `s.active.EndActiveGroupSession`, `errors.Is` |
| `sessions.Touch` | [presence.go:213](../../../backend/modules/devicescan/compose/presence.go#L213) | direct binding | `s.active.UpdateSessionActivity` |
| `sessions.Supervisors` | [presence.go:217](../../../backend/modules/devicescan/compose/presence.go#L217) | runtime retained | `s.presence.QueryLiveGroups`, `len`, `s.clock.Day`, `s.clock.Now`, `today.String`, `s.presence.QueryGroupSupervisions`, `make`, `append`, `supervisionEnded` |
| `sessions.ReplaceSupervisors` | [presence.go:236](../../../backend/modules/devicescan/compose/presence.go#L236) | runtime retained | `s.active.UpdateActiveGroupSupervisors` |
| `attendance.Status` | [presence.go:245](../../../backend/modules/devicescan/compose/presence.go#L245) | runtime retained | `a.active.GetStudentAttendanceStatus` |
| `attendance.Toggle` | [presence.go:256](../../../backend/modules/devicescan/compose/presence.go#L256) | runtime retained | `a.active.ToggleStudentAttendance`, `classifyPresence` |
| `attendance.ConfirmDailyCheckout` | [presence.go:264](../../../backend/modules/devicescan/compose/presence.go#L264) | runtime retained | `a.active.ConfirmDailyCheckout`, `errors.Is`, `mapError`, `classifyPresence` |
| `supervisionQuery.ActiveSupervisions` | [session_lifecycle.go:40](../../../backend/modules/devicescan/compose/session_lifecycle.go#L40) | runtime retained | `calendar.TodayDate`, `today.String`, `q.source.QueryGroupSupervisions`, `make`, `len`, `append`, `supervisionEnded` |
| `lifecycleSession` | [session_lifecycle.go:59](../../../backend/modules/devicescan/compose/session_lifecycle.go#L59) | runtime retained | `lifecycleSupervisors` |
| `lifecycleSupervisors` | [session_lifecycle.go:73](../../../backend/modules/devicescan/compose/session_lifecycle.go#L73) | runtime retained | `calendar.TodayDate`, `.String`, `make`, `len`, `append`, `supervisionEnded` |
| `supervisionEnded` | [session_lifecycle.go:84](../../../backend/modules/devicescan/compose/session_lifecycle.go#L84) | direct binding | `application.SupervisionEnded` |
| `lifecycleStore.Start` | [session_lifecycle.go:88](../../../backend/modules/devicescan/compose/session_lifecycle.go#L88) | runtime retained | `s.active.ForceStartActivitySessionWithSupervisors`, `s.active.StartActivitySessionWithSupervisors`, `errors.Is`, `classifyPresence`, `lifecycleSession` |
| `lifecycleStore.Current` | [session_lifecycle.go:102](../../../backend/modules/devicescan/compose/session_lifecycle.go#L102) | runtime retained | `s.active.GetDeviceCurrentSession`, `classifyPresence`, `errors.Is`, `mapError`, `lifecycleSession` |
| `lifecycleStore.Conflict` | [session_lifecycle.go:118](../../../backend/modules/devicescan/compose/session_lifecycle.go#L118) | runtime retained | `s.active.CheckActivityConflict`, `classifyPresence`, `strconv.ParseInt` |
| `lifecycleStore.Supervisors` | [session_lifecycle.go:132](../../../backend/modules/devicescan/compose/session_lifecycle.go#L132) | direct binding | `application.LifecycleSupervisorsBinding` |
| `lifecycleStore.ReplaceSupervisors` | [session_lifecycle.go:136](../../../backend/modules/devicescan/compose/session_lifecycle.go#L136) | runtime retained | `s.active.UpdateActiveGroupSupervisors`, `lifecycleSession`, `classifyPresence` |
| `lifecycleStore.CountStudents` | [session_lifecycle.go:141](../../../backend/modules/devicescan/compose/session_lifecycle.go#L141) | direct binding | `s.active.CountActiveVisitsByActiveGroupID` |
| `lifecycleStore.Touch` | [session_lifecycle.go:145](../../../backend/modules/devicescan/compose/session_lifecycle.go#L145) | runtime retained | `classifyPresence`, `s.active.UpdateSessionActivity` |
| `lifecycleStore.Timeout` | [session_lifecycle.go:149](../../../backend/modules/devicescan/compose/session_lifecycle.go#L149) | runtime retained | `s.active.ProcessSessionTimeout`, `classifyPresence`, `errors.Is`, `mapError` |
| `lifecycleStore.ValidateTimeout` | [session_lifecycle.go:161](../../../backend/modules/devicescan/compose/session_lifecycle.go#L161) | runtime retained | `classifyPresence`, `s.active.ValidateSessionTimeout` |
| `lifecycleStore.TimeoutInfo` | [session_lifecycle.go:165](../../../backend/modules/devicescan/compose/session_lifecycle.go#L165) | runtime retained | `s.active.GetSessionTimeoutInfo`, `classifyPresence`, `int`, `info.InactivityDuration.Seconds`, `info.TimeUntilTimeout.Seconds` |
| `lifecyclePeople.StaffNames` | [session_lifecycle.go:175](../../../backend/modules/devicescan/compose/session_lifecycle.go#L175) | direct binding | `application.StaffNamesBinding` |
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
| `TransactionFunc.Run` | [new.go:19](../../../backend/modules/feedback/compose/new.go#L19) | direct binding | `fn` |
| `*Settings.FeedbackEnabled` | [new.go:84](../../../backend/modules/feedback/compose/new.go#L84) | direct binding | `s.enabled` |
| `*Settings.FeedbackRetentionDays` | [new.go:86](../../../backend/modules/feedback/compose/new.go#L86) | direct binding | `s.retention` |

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
| `TransactionFunc.Run` | [new.go:19](../../../backend/modules/mealplan/compose/new.go#L19) | direct binding | `fn` |
| `*Settings.MealPlanEnabled` | [new.go:76](../../../backend/modules/mealplan/compose/new.go#L76) | direct binding | `s.mealPlanEnabled` |
| `*Settings.MealRegistrationEnabled` | [new.go:78](../../../backend/modules/mealplan/compose/new.go#L78) | direct binding | `s.mealRegistrationEnabled` |
| `*Settings.MealRegistrationCutoff` | [new.go:82](../../../backend/modules/mealplan/compose/new.go#L82) | direct binding | `s.mealRegistrationCutoff` |

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

| Function | Source | Classification | Calls |
|---|---|---|---|
| `access.Caller` | [access.go:25](../../../backend/modules/requestreview/compose/access.go#L25) | direct binding | `a.principal` |
| `access.ReviewAccess` | [access.go:29](../../../backend/modules/requestreview/compose/access.go#L29) | runtime retained | `a.level` |
| `careQueueFilter` | [care_queue.go:29](../../../backend/modules/requestreview/compose/care_queue.go#L29) | runtime retained | none |
| `careQueueCursor` | [care_queue.go:37](../../../backend/modules/requestreview/compose/care_queue.go#L37) | runtime retained | none |
| `careScheduleQueue.Open` | [care_queue.go:44](../../../backend/modules/requestreview/compose/care_queue.go#L44) | runtime retained | `q.query.ListPending`, `careQueueFilter`, `careplan.ParseDate`, `q.today`, `make`, `len`, `append`, `excusedrequests.ParentRequestVersion`, `careplan.CareReviewUrgentOn`, `careplan.CareReviewPastOn`, `careplan.CareReviewConflictKeys`, `requestreview.ToCareRequestResponse`, `careQueueCursor` |
| `careScheduleQueue.History` | [care_queue.go:66](../../../backend/modules/requestreview/compose/care_queue.go#L66) | runtime retained | `q.query.ListHistory`, `careQueueFilter`, `make`, `len`, `append`, `excusedrequests.ParentRequestVersion`, `requestreview.HistoryDecidedAt`, `requestreview.ToCareRequestHistoryResponse`, `careQueueCursor` |
| `careScheduleQueue.OpenCount` | [care_queue.go:83](../../../backend/modules/requestreview/compose/care_queue.go#L83) | runtime retained | `q.query.ListPending`, `today.String`, `len` |
| `correctionLog.History` | [corrections.go:31](../../../backend/modules/requestreview/compose/corrections.go#L31) | runtime retained | `c.query.ListDirectCorrections`, `fmt.Errorf`, `len`, `len`, `make`, `len`, `len`, `make`, `len`, `append`, `c.people.FindStudents`, `fmt.Errorf`, `append`, `c.people.PersonNames`, `fmt.Errorf`, `c.allowed`, `strings.TrimSpace`, `careplan.ReviewCorrectionDiff`, `slog.Warn`, `make`, `len`, `append`, `strconv.FormatInt`, `requestreview.GermanOfferingDiffLabel`, `requestreview.GermanOfferingDiffLabel`, `append`, `strconv.FormatInt`, `strconv.FormatInt`, `row.ActorName` |
| `directory.GroupNames` | [directory.go:36](../../../backend/modules/requestreview/compose/directory.go#L36) | runtime retained | `d.people.ListStudentsByID`, `make`, `len`, `append`, `d.groups.ListGroupsByID`, `make`, `len`, `make`, `len` |
| `excusedQueue.Open` | [excused_queue.go:23](../../../backend/modules/requestreview/compose/excused_queue.go#L23) | runtime retained | `q.service.ListPending`, `careQueueFilter`, `excusedrequests.ParseDate`, `q.today`, `mapRows`, `excusedPendingRow`, `careQueueCursor` |
| `excusedQueue.History` | [excused_queue.go:37](../../../backend/modules/requestreview/compose/excused_queue.go#L37) | runtime retained | `q.service.ListHistory`, `careQueueFilter`, `mapRows`, `careQueueCursor` |
| `excusedQueue.OpenCount` | [excused_queue.go:46](../../../backend/modules/requestreview/compose/excused_queue.go#L46) | runtime retained | `q.service.ListPending`, `today.String`, `len` |
| `excusedPendingRow` | [excused_queue.go:54](../../../backend/modules/requestreview/compose/excused_queue.go#L54) | runtime retained | `item.Request.UrgentOn`, `excusedrequests.ParentRequestVersion`, `item.Request.PastOn`, `item.Request.ConflictKeys`, `requestreview.ToStaffExcusedRequestResponse` |
| `excusedHistoryRow` | [excused_queue.go:76](../../../backend/modules/requestreview/compose/excused_queue.go#L76) | runtime retained | `excusedrequests.ParentRequestVersion`, `requestreview.HistoryDecidedAt`, `item.Request.CanCorrect`, `requestreview.ToStaffExcusedHistoryResponse` |
| `familyProtection.Protected` | [family_protection.go:22](../../../backend/modules/requestreview/compose/family_protection.go#L22) | direct binding | `f.query.CurrentFamilyProtection` |
| `masterDataDirectory.ReviewFields` | [master_data_directory.go:31](../../../backend/modules/requestreview/compose/master_data_directory.go#L31) | runtime retained | `make`, `len`, `append`, `d.fields.ReviewStudentFields`, `make`, `len`, `reviewStudent` |
| `reviewStudent` | [master_data_directory.go:59](../../../backend/modules/requestreview/compose/master_data_directory.go#L59) | runtime retained | `student.IsAlumnus`, `careplan.Date` |
| `reviewDirectory.FindStudents` | [master_data_directory.go:66](../../../backend/modules/requestreview/compose/master_data_directory.go#L66) | runtime retained | `d.people.ListStudentsByID`, `make`, `len`, `reviewStudent` |
| `reviewDirectory.PersonNames` | [master_data_directory.go:78](../../../backend/modules/requestreview/compose/master_data_directory.go#L78) | runtime retained | `d.people.ListPersonsByID`, `make`, `len` |
| `reviewDirectory.ReviewerNames` | [master_data_directory.go:90](../../../backend/modules/requestreview/compose/master_data_directory.go#L90) | runtime retained | `d.people.ListPersonsByAccount`, `make`, `len` |
| `masterDataQueue.Open` | [master_data_queue.go:24](../../../backend/modules/requestreview/compose/master_data_queue.go#L24) | runtime retained | `q.service.ListPending`, `careQueueFilter`, `mapRows`, `careQueueCursor` |
| `masterDataQueue.History` | [master_data_queue.go:32](../../../backend/modules/requestreview/compose/master_data_queue.go#L32) | runtime retained | `q.service.ListHistory`, `careQueueFilter`, `mapRows`, `careQueueCursor` |
| `masterDataQueue.OpenCount` | [master_data_queue.go:40](../../../backend/modules/requestreview/compose/master_data_queue.go#L40) | runtime retained | `q.service.ListPending`, `today.String`, `len` |
| `mapRows` | [master_data_queue.go:45](../../../backend/modules/requestreview/compose/master_data_queue.go#L45) | runtime retained | `make`, `len`, `append`, `build` |
| `masterDataPendingRow` | [master_data_queue.go:53](../../../backend/modules/requestreview/compose/master_data_queue.go#L53) | runtime retained | `excusedrequests.ParentRequestVersion`, `item.Request.ConflictKeys`, `requestreview.ToMasterDataChangeRequestResponse` |
| `masterDataHistoryRow` | [master_data_queue.go:71](../../../backend/modules/requestreview/compose/master_data_queue.go#L71) | runtime retained | `excusedrequests.ParentRequestVersion`, `requestreview.HistoryDecidedAt`, `item.Request.CanCorrect`, `requestreview.ToMasterDataHistoryResponse` |
| `offeringQueue.Open` | [offering_queue.go:24](../../../backend/modules/requestreview/compose/offering_queue.go#L24) | runtime retained | `q.query.ListPending`, `careQueueFilter`, `careplan.ParseDate`, `q.today`, `make`, `len`, `append`, `excusedrequests.ParentRequestVersion`, `careplan.OfferingReviewUrgentOn`, `careplan.OfferingReviewPastOn`, `careplan.OfferingReviewConflictKeys`, `ToOfferingRequestResponse`, `careQueueCursor` |
| `offeringQueue.History` | [offering_queue.go:43](../../../backend/modules/requestreview/compose/offering_queue.go#L43) | runtime retained | `q.query.ListHistory`, `careQueueFilter`, `make`, `len`, `append`, `excusedrequests.ParentRequestVersion`, `requestreview.HistoryDecidedAt`, `ToOfferingRequestHistoryResponse`, `careQueueCursor` |
| `offeringQueue.OpenCount` | [offering_queue.go:57](../../../backend/modules/requestreview/compose/offering_queue.go#L57) | runtime retained | `q.query.PendingCount`, `careplan.Date`, `today.String` |
| `OfferingRequestDiffLines` | [offering_responses.go:10](../../../backend/modules/requestreview/compose/offering_responses.go#L10) | runtime retained | `make`, `len`, `strconv.FormatInt`, `requestreview.GermanOfferingDiffLabel`, `requestreview.GermanOfferingDiffLabel`, `len`, `requestreview.GermanOfferingDiffLabel`, `len`, `requestreview.GermanOfferingDiffLabel`, `len`, `len`, `requestreview.GermanOfferingDiffLabel`, `append`, `strconv.FormatInt`, `append` |
| `ToOfferingRequestResponse` | [offering_responses.go:40](../../../backend/modules/requestreview/compose/offering_responses.go#L40) | runtime retained | `OfferingRequestDiffLines`, `make`, `len`, `append`, `strconv.FormatInt`, `requestreview.GermanOfferingDiffLabel`, `strconv.FormatInt`, `strconv.FormatInt`, `len` |
| `ToOfferingRequestHistoryResponse` | [offering_responses.go:81](../../../backend/modules/requestreview/compose/offering_responses.go#L81) | runtime retained | `make`, `len`, `append`, `strconv.FormatInt`, `requestreview.GermanOfferingDiffLabel`, `ToOfferingRequestResponse`, `requestreview.HistoryDecidedAt` |
| `offeringReviewDirectory.SearchStudentIDs` | [offering_reviews.go:26](../../../backend/modules/requestreview/compose/offering_reviews.go#L26) | runtime retained | `strings.TrimSpace`, `make`, `d.search.SearchPersons`, `append`, `len`, `make`, `len`, `d.search.ListStudentsByPersonID`, `student.IsAlumnus`, `slices.Contains`, `append` |
| `offeringReviewEnrollment.Children` | [offering_reviews.go:65](../../../backend/modules/requestreview/compose/offering_reviews.go#L65) | runtime retained | `e.query.ChildrenByID`, `make`, `len`, `append` |
| `offeringReviewEnrollment.Requests` | [offering_reviews.go:82](../../../backend/modules/requestreview/compose/offering_reviews.go#L82) | runtime retained | `e.query.RequestsByID`, `make`, `len`, `append` |
| `offeringReviewEnrollment.Phases` | [offering_reviews.go:95](../../../backend/modules/requestreview/compose/offering_reviews.go#L95) | runtime retained | `e.query.PhasesByID`, `make`, `len`, `append`, `careplan.Date`, `careplan.Date` |
| `offeringReviewEnrollment.Selections` | [offering_reviews.go:108](../../../backend/modules/requestreview/compose/offering_reviews.go#L108) | runtime retained | `e.bookings.CareOfferingBookingsAtDates`, `make`, `len`, `append`, `row.EffectiveSelectedDays` |
| `offeringReviewCourses.CourseGroups` | [offering_reviews.go:122](../../../backend/modules/requestreview/compose/offering_reviews.go#L122) | runtime retained | `day.String`, `append`, `append`, `append`, `c.query.ListCourseGroups`, `append`, `slices.Clone`, `append` |
| `scheduleReviewDirectory.DepartureModes` | [schedule_reviews.go:22](../../../backend/modules/requestreview/compose/schedule_reviews.go#L22) | direct binding | `d.departures.ListStudentDepartureModes` |
| `scheduleReviewBookings.ApprovedForStudents` | [schedule_reviews.go:33](../../../backend/modules/requestreview/compose/schedule_reviews.go#L33) | runtime retained | `to.Before`, `b.query.ApprovedOfferingChildrenForStudents`, `make`, `len`, `make`, `len`, `append`, `child.StudentID`, `b.bookings.CareOfferingBookingHistory`, `make`, `len`, `from.Before`, `to.Before`, `row.EffectiveSelectedDays`, `append` |

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
| `retainedGroupError` | [group_values_test_helpers.go:29](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L29) | test-only | `legacyGroupError`, `legacyGroupError`, `groups.MapGroupErrorCauses` |
| `legacyGroupQuery` | [group_values_test_helpers.go:74](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L74) | test-only | `schoolstructure.GroupListQuery` |
| `LegacyGroupManagement` | [group_values_test_helpers.go:90](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L90) | test-only | none |
| `*legacyGroupManagement.GetGroup` | [group_values_test_helpers.go:94](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L94) | test-only | `s.capability.GetGroup`, `groups.LegacyGroup`, `legacyGroupError` |
| `*legacyGroupManagement.FindGroupWithRoom` | [group_values_test_helpers.go:98](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L98) | test-only | `s.capability.FindGroupWithRoom`, `groups.LegacyGroup`, `legacyGroupError` |
| `*legacyGroupManagement.GetGroupsByIDs` | [group_values_test_helpers.go:102](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L102) | test-only | `s.capability.GetGroupsByIDs`, `legacyGroupError`, `make`, `len`, `groups.LegacyGroup`, `legacyGroupError` |
| `*legacyGroupManagement.GetGroupsWithRoomsByIDs` | [group_values_test_helpers.go:113](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L113) | test-only | `s.capability.GetGroupsWithRoomsByIDs`, `legacyGroupError`, `make`, `len`, `groups.LegacyGroup`, `legacyGroupError` |
| `*legacyGroupManagement.CreateGroup` | [group_values_test_helpers.go:124](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L124) | test-only | `groups.PublicGroup`, `s.capability.CreateGroup`, `groups.LegacyGroup`, `legacyGroupError` |
| `*legacyGroupManagement.UpdateGroup` | [group_values_test_helpers.go:132](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L132) | test-only | `groups.PublicGroup`, `s.capability.UpdateGroup`, `groups.LegacyGroup`, `legacyGroupError` |
| `*legacyGroupManagement.DeleteGroup` | [group_values_test_helpers.go:140](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L140) | test-only | `legacyGroupError`, `s.capability.DeleteGroup` |
| `*legacyGroupManagement.ListGroups` | [group_values_test_helpers.go:143](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L143) | test-only | `s.capability.ListGroups`, `legacyGroupQuery`, `legacyGroupError`, `make`, `len`, `groups.LegacyGroup`, `legacyGroupError` |
| `*legacyGroupManagement.CountGroups` | [group_values_test_helpers.go:154](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L154) | test-only | `s.capability.CountGroups`, `legacyGroupQuery`, `legacyGroupError` |
| `*legacyGroupManagement.GetTeacherGroups` | [group_values_test_helpers.go:158](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L158) | test-only | `s.capability.GetTeacherGroups`, `legacyGroupError`, `make`, `len`, `groups.LegacyGroup`, `legacyGroupError` |
| `*legacyGroupManagement.GetGroupTeachers` | [group_values_test_helpers.go:169](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L169) | test-only | `s.capability.GetGroupTeachers`, `legacyGroupError`, `make`, `len`, `legacyTeacher`, `legacyGroupError` |
| `*legacyGroupManagement.GetTeachersForGroups` | [group_values_test_helpers.go:180](../../../backend/modules/schoolstructure/compose/group_values_test_helpers.go#L180) | test-only | `s.capability.GetTeachersForGroups`, `legacyGroupError`, `make`, `len`, `make`, `len`, `legacyTeacher`, `legacyGroupError` |
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
| `sessionAttendancePorts` | [new.go:47](../../../backend/modules/studentpresence/compose/new.go#L47) | runtime retained | none |
| `databaseRuntime` | [new.go:63](../../../backend/modules/studentpresence/compose/new.go#L63) | direct binding | none |
| `transaction.Run` | [new.go:67](../../../backend/modules/studentpresence/compose/new.go#L67) | runtime retained | `tenant.NewTransactionRunner().RunInTx`, `tenant.NewTransactionRunner` |
| `transaction.Require` | [new.go:71](../../../backend/modules/studentpresence/compose/new.go#L71) | runtime retained | `tenant.TenantFromContext`, `tenant.TransactionFromContext`, `errors.New` |
| `FailureLogger` | [observers.go:7](../../../backend/modules/studentpresence/compose/observers.go#L7) | runtime retained | `slog.Default().Warn`, `slog.Default` |
| `DebugLogger` | [observers.go:20](../../../backend/modules/studentpresence/compose/observers.go#L20) | direct binding | `logger.Debug` |
| `plannedRoster.ListPlannedParticipants` | [session_attendance.go:13](../../../backend/modules/studentpresence/compose/session_attendance.go#L13) | runtime retained | `r.query.ListPlannedParticipants`, `studentpresence.PlannedRosterFilter`, `make`, `len`, `append`, `ports.PlannedParticipant` |
| `carePlanDirectory.FindPickupException` | [session_attendance.go:29](../../../backend/modules/studentpresence/compose/session_attendance.go#L29) | runtime retained | `d.query.FindPickupException`, `ports.PickupException` |
| `carePlanDirectory.ListPickupExceptions` | [session_attendance.go:38](../../../backend/modules/studentpresence/compose/session_attendance.go#L38) | runtime retained | `d.query.ListPickupExceptions`, `studentpresence.SessionPickupExceptionFilter`, `make`, `len`, `append`, `ports.PickupException` |
| `carePlanDirectory.FindStudentStatusDay` | [session_attendance.go:47](../../../backend/modules/studentpresence/compose/session_attendance.go#L47) | runtime retained | `d.query.FindStudentStatusDay`, `ports.StudentStatusDay` |
| `carePlanDirectory.ListStudentStatusDays` | [session_attendance.go:56](../../../backend/modules/studentpresence/compose/session_attendance.go#L56) | runtime retained | `d.query.ListStudentStatusDays`, `studentpresence.SessionStudentStatusDayFilter`, `make`, `len`, `append`, `ports.StudentStatusDay` |
| `SessionRepositories` | [session_records.go:87](../../../backend/modules/studentpresence/compose/session_records.go#L87) | runtime retained | `panic`, `fmt.Sprintf` |
| `*SessionRecords.RoomDirectory` | [session_records.go:97](../../../backend/modules/studentpresence/compose/session_records.go#L97) | direct binding | `r.groups.RoomDirectory` |
| `*SessionRecords.SupervisionStaff` | [session_records.go:104](../../../backend/modules/studentpresence/compose/session_records.go#L104) | direct binding | none |
| `*SessionRecords.CreateSession` | [session_records.go:108](../../../backend/modules/studentpresence/compose/session_records.go#L108) | runtime retained | `r.groups.Create`, `activeGroupRow`, `r.groups.Create`, `livePointer` |
| `*SessionRecords.FindSession` | [session_records.go:120](../../../backend/modules/studentpresence/compose/session_records.go#L120) | runtime retained | `r.groups.FindByID`, `livePointer` |
| `*SessionRecords.FindByIDForUpdate` | [session_records.go:125](../../../backend/modules/studentpresence/compose/session_records.go#L125) | runtime retained | `r.groups.FindByIDForUpdate`, `livePointer` |
| `*SessionRecords.FindByIDs` | [session_records.go:130](../../../backend/modules/studentpresence/compose/session_records.go#L130) | runtime retained | `r.groups.FindByIDs`, `make`, `len`, `sessionDetailOf` |
| `*SessionRecords.FindActiveByRoomID` | [session_records.go:142](../../../backend/modules/studentpresence/compose/session_records.go#L142) | runtime retained | `r.groups.FindActiveByRoomID`, `livePointers` |
| `*SessionRecords.FindActiveByDeviceIDWithNames` | [session_records.go:147](../../../backend/modules/studentpresence/compose/session_records.go#L147) | runtime retained | `r.groups.FindActiveByDeviceIDWithNames`, `sessionDetailOf` |
| `*SessionRecords.FindActiveGroups` | [session_records.go:152](../../../backend/modules/studentpresence/compose/session_records.go#L152) | runtime retained | `r.groups.FindActiveGroups`, `livePointers` |
| `*SessionRecords.CheckRoomConflict` | [session_records.go:157](../../../backend/modules/studentpresence/compose/session_records.go#L157) | runtime retained | `r.groups.CheckRoomConflict`, `livePointer` |
| `*SessionRecords.UpdateLastActivity` | [session_records.go:162](../../../backend/modules/studentpresence/compose/session_records.go#L162) | direct binding | `r.groups.UpdateLastActivity` |
| `*SessionRecords.GetOccupiedActivityGroupIDs` | [session_records.go:166](../../../backend/modules/studentpresence/compose/session_records.go#L166) | direct binding | `r.groups.GetOccupiedActivityGroupIDs` |
| `*SessionRecords.SessionRows` | [session_records.go:172](../../../backend/modules/studentpresence/compose/session_records.go#L172) | runtime retained | `make`, `len`, `len`, `r.groups.FindByIDs`, `append` |
| `*SessionRecords.CreateSupervision` | [session_records.go:189](../../../backend/modules/studentpresence/compose/session_records.go#L189) | runtime retained | `r.supervisors.Create`, `supervisorRow`, `r.supervisors.Create`, `supervisionOf` |
| `*SessionRecords.FindByActiveGroupID` | [session_records.go:204](../../../backend/modules/studentpresence/compose/session_records.go#L204) | runtime retained | `r.supervisors.FindByActiveGroupID`, `staffedSupervisions` |
| `*SessionRecords.FindByActiveGroupIDs` | [session_records.go:209](../../../backend/modules/studentpresence/compose/session_records.go#L209) | runtime retained | `r.supervisors.FindByActiveGroupIDs`, `staffedSupervisions` |
| `*SessionRecords.FindActiveByStaffID` | [session_records.go:214](../../../backend/modules/studentpresence/compose/session_records.go#L214) | runtime retained | `r.supervisors.FindActiveByStaffID`, `make`, `len`, `supervisionOf`, `append` |
| `*SessionRecords.EndByActiveGroupAndStaffID` | [session_records.go:227](../../../backend/modules/studentpresence/compose/session_records.go#L227) | direct binding | `r.supervisors.EndByActiveGroupAndStaffID` |
| `*SessionRecords.EndAllActiveByStaffID` | [session_records.go:231](../../../backend/modules/studentpresence/compose/session_records.go#L231) | direct binding | `r.supervisors.EndAllActiveByStaffID` |
| `*SessionRecords.GetStaffIDsWithSupervisionToday` | [session_records.go:235](../../../backend/modules/studentpresence/compose/session_records.go#L235) | direct binding | `r.supervisors.GetStaffIDsWithSupervisionToday` |
| `*SessionRecords.ListActiveSupervisionBlockers` | [session_records.go:239](../../../backend/modules/studentpresence/compose/session_records.go#L239) | runtime retained | `r.supervisors.ListActiveSupervisionBlockers`, `make`, `len`, `append`, `studentpresence.SupervisionBlocker` |
| `*SessionRecords.ListActiveSupervisedRooms` | [session_records.go:251](../../../backend/modules/studentpresence/compose/session_records.go#L251) | runtime retained | `r.supervisors.ListActiveSupervisedRooms`, `make`, `len`, `append`, `studentpresence.StaffRoomSupervision` |
| `livePointer` | [session_records.go:263](../../../backend/modules/studentpresence/compose/session_records.go#L263) | runtime retained | none |
| `livePointers` | [session_records.go:274](../../../backend/modules/studentpresence/compose/session_records.go#L274) | runtime retained | `make`, `len`, `livePointer` |
| `activeGroupRow` | [session_records.go:285](../../../backend/modules/studentpresence/compose/session_records.go#L285) | direct binding | none |
| `sessionDetailOf` | [session_records.go:293](../../../backend/modules/studentpresence/compose/session_records.go#L293) | runtime retained | `append`, `supervisionOf` |
| `supervisionOf` | [session_records.go:319](../../../backend/modules/studentpresence/compose/session_records.go#L319) | runtime retained | `row.StartDate.String`, `row.EndDate.String` |
| `staffedSupervisions` | [session_records.go:331](../../../backend/modules/studentpresence/compose/session_records.go#L331) | runtime retained | `make`, `len`, `supervisionOf`, `row.Staff.Person.GetFullName`, `append` |
| `supervisorRow` | [session_records.go:348](../../../backend/modules/studentpresence/compose/session_records.go#L348) | runtime retained | `application.SupervisionToRecord` |

### `modules/studentpresence/compose/presenceservice`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `WithPresenceSettings` | [presence.go:53](../../../backend/modules/studentpresence/compose/presenceservice/presence.go#L53) | direct binding | `presence.WithSettings` |
| `WithPresenceTenantRuntime` | [presence.go:59](../../../backend/modules/studentpresence/compose/presenceservice/presence.go#L59) | direct binding | `presence.WithTenantRuntime` |
| `WithGuardianWaker` | [presence.go:64](../../../backend/modules/studentpresence/compose/presenceservice/presence.go#L64) | direct binding | `presence.WithGuardianWaker` |
| `presenceEngineOf` | [presence.go:75](../../../backend/modules/studentpresence/compose/presenceservice/presence.go#L75) | runtime retained | none |
| `*presenceFacade.Engine` | [presence_facade.go:53](../../../backend/modules/studentpresence/compose/presenceservice/presence_facade.go#L53) | direct binding | none |
| `*presenceFacade.GetActiveGroup` | [presence_facade.go:57](../../../backend/modules/studentpresence/compose/presenceservice/presence_facade.go#L57) | runtime retained | `p.presenceEngine.GetActiveGroup`, `sessionDetail` |
| `*presenceFacade.GetActiveGroupsByIDs` | [presence_facade.go:62](../../../backend/modules/studentpresence/compose/presenceservice/presence_facade.go#L62) | runtime retained | `p.presenceEngine.GetActiveGroupsByIDs`, `make`, `len`, `sessionDetail` |
| `*presenceFacade.GetUnclaimedActiveGroups` | [presence_facade.go:74](../../../backend/modules/studentpresence/compose/presenceservice/presence_facade.go#L74) | runtime retained | `p.presenceEngine.GetUnclaimedActiveGroups`, `make`, `len`, `sessionDetail` |
| `*presenceFacade.GetRoomsByIDs` | [presence_facade.go:86](../../../backend/modules/studentpresence/compose/presenceservice/presence_facade.go#L86) | runtime retained | `p.rooms.FindByIDs`, `make`, `len`, `sessionRoom` |
| `*presenceFacade.HasOpenAttendanceOn` | [presence_facade.go:100](../../../backend/modules/studentpresence/compose/presenceservice/presence_facade.go#L100) | runtime retained | `p.attendance.HasAttendance`, `date.String`, `date.String` |
| `*presenceFacade.StartActivitySessionWithSupervisors` | [presence_facade.go:104](../../../backend/modules/studentpresence/compose/presenceservice/presence_facade.go#L104) | runtime retained | `p.presenceEngine.StartActivitySessionWithSupervisors`, `sessionDetail` |
| `*presenceFacade.ForceStartActivitySessionWithSupervisors` | [presence_facade.go:109](../../../backend/modules/studentpresence/compose/presenceservice/presence_facade.go#L109) | runtime retained | `p.presenceEngine.ForceStartActivitySessionWithSupervisors`, `sessionDetail` |
| `*presenceFacade.CheckActivityConflict` | [presence_facade.go:114](../../../backend/modules/studentpresence/compose/presenceservice/presence_facade.go#L114) | runtime retained | `p.presenceEngine.CheckActivityConflict`, `liveGroup` |
| `*presenceFacade.GetDeviceCurrentSession` | [presence_facade.go:130](../../../backend/modules/studentpresence/compose/presenceservice/presence_facade.go#L130) | runtime retained | `p.presenceEngine.GetDeviceCurrentSession`, `sessionDetail` |
| `*presenceFacade.FindDeviceActiveGroupInRoom` | [presence_facade.go:135](../../../backend/modules/studentpresence/compose/presenceservice/presence_facade.go#L135) | runtime retained | `p.presenceEngine.FindDeviceActiveGroupInRoom`, `sessionDetail` |
| `*presenceFacade.UpdateActiveGroupSupervisors` | [presence_facade.go:140](../../../backend/modules/studentpresence/compose/presenceservice/presence_facade.go#L140) | runtime retained | `p.presenceEngine.UpdateActiveGroupSupervisors`, `sessionDetail` |
| `*presenceFacade.EnsureOpenRoomSession` | [presence_facade.go:145](../../../backend/modules/studentpresence/compose/presenceservice/presence_facade.go#L145) | runtime retained | `p.presenceEngine.EnsureOpenRoomSession`, `sessionDetail` |
| `*presenceFacade.CreateActiveGroup` | [presence_facade.go:152](../../../backend/modules/studentpresence/compose/presenceservice/presence_facade.go#L152) | runtime retained | `p.presenceEngine.CreateActiveGroup`, `groupRow`, `p.presenceEngine.CreateActiveGroup`, `liveGroup` |
| `*presenceFacade.CreateGroupSupervisor` | [presence_facade.go:166](../../../backend/modules/studentpresence/compose/presenceservice/presence_facade.go#L166) | runtime retained | `p.presenceEngine.CreateGroupSupervisor`, `supervisorRow`, `p.presenceEngine.CreateGroupSupervisor`, `groupSupervision` |
| `liveGroup` | [presence_facade.go:181](../../../backend/modules/studentpresence/compose/presenceservice/presence_facade.go#L181) | direct binding | none |
| `groupRow` | [presence_facade.go:189](../../../backend/modules/studentpresence/compose/presenceservice/presence_facade.go#L189) | runtime retained | `row.SetTenantID` |
| `sessionDetail` | [presence_facade.go:204](../../../backend/modules/studentpresence/compose/presenceservice/presence_facade.go#L204) | runtime retained | `sessionRoom`, `make`, `len`, `append`, `groupSupervision` |
| `sessionRoom` | [presence_facade.go:231](../../../backend/modules/studentpresence/compose/presenceservice/presence_facade.go#L231) | runtime retained | none |
| `groupSupervision` | [presence_facade.go:238](../../../backend/modules/studentpresence/compose/presenceservice/presence_facade.go#L238) | runtime retained | `row.StartDate.String`, `row.EndDate.String` |
| `supervisorRow` | [presence_facade.go:250](../../../backend/modules/studentpresence/compose/presenceservice/presence_facade.go#L250) | runtime retained | `calendar.ParseDate`, `supervisor.SetTenantID`, `calendar.ParseDate` |
| `PresenceOperations.StartSession` | [presence_operations.go:44](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L44) | runtime retained | `groupRow`, `p.active.CreateActiveGroup`, `liveGroup` |
| `PresenceOperations.ReviseSession` | [presence_operations.go:52](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L52) | runtime retained | `groupRow`, `p.active.UpdateActiveGroup`, `liveGroup` |
| `PresenceOperations.RemoveSession` | [presence_operations.go:60](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L60) | direct binding | `p.active.DeleteActiveGroup` |
| `PresenceOperations.EndSession` | [presence_operations.go:64](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L64) | direct binding | `p.active.EndActiveGroupSession` |
| `PresenceOperations.TouchSession` | [presence_operations.go:68](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L68) | direct binding | `p.active.UpdateSessionActivity` |
| `PresenceOperations.AdmitVisit` | [presence_operations.go:72](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L72) | runtime retained | `p.active.CreateVisit` |
| `PresenceOperations.AmendVisit` | [presence_operations.go:79](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L79) | direct binding | `p.active.UpdateVisit` |
| `PresenceOperations.RemoveVisit` | [presence_operations.go:83](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L83) | direct binding | `p.active.DeleteVisit` |
| `PresenceOperations.EndVisit` | [presence_operations.go:87](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L87) | direct binding | `p.active.EndVisit` |
| `PresenceOperations.PresenceMode` | [presence_operations.go:91](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L91) | direct binding | `p.active.GetPresenceMode` |
| `PresenceOperations.AssignSupervision` | [presence_operations.go:95](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L95) | runtime retained | `supervisorRow`, `p.active.CreateGroupSupervisor`, `groupSupervision` |
| `PresenceOperations.AmendSupervision` | [presence_operations.go:106](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L106) | runtime retained | `supervisorRow`, `p.active.UpdateGroupSupervisor` |
| `PresenceOperations.RemoveSupervisionRecord` | [presence_operations.go:114](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L114) | direct binding | `p.active.DeleteGroupSupervisor` |
| `PresenceOperations.EndSupervision` | [presence_operations.go:118](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L118) | direct binding | `p.active.EndSupervision` |
| `PresenceOperations.ClaimSupervision` | [presence_operations.go:122](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L122) | runtime retained | `p.active.ClaimActiveGroup`, `row.StartDate.String` |
| `PresenceOperations.UnclaimedSessions` | [presence_operations.go:133](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L133) | runtime retained | `p.active.GetUnclaimedActiveGroups`, `make`, `len`, `append` |
| `PresenceOperations.CreateCombination` | [presence_operations.go:156](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L156) | runtime retained | `p.active.CreateCombinedGroupWithGroups` |
| `PresenceOperations.AmendCombination` | [presence_operations.go:163](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L163) | runtime retained | `p.active.UpdateCombinedGroup` |
| `PresenceOperations.RemoveCombination` | [presence_operations.go:170](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L170) | direct binding | `p.active.DeleteCombinedGroup` |
| `PresenceOperations.CloseCombination` | [presence_operations.go:174](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L174) | direct binding | `p.active.EndCombinedGroup` |
| `attendanceStatus` | [presence_operations.go:178](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L178) | runtime retained | `status.Date.String` |
| `PresenceOperations.StudentAttendanceStatus` | [presence_operations.go:189](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L189) | runtime retained | `p.active.GetStudentAttendanceStatus`, `attendanceStatus` |
| `PresenceOperations.StudentsAttendanceStatuses` | [presence_operations.go:197](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L197) | runtime retained | `p.active.GetStudentsAttendanceStatuses`, `make`, `len`, `attendanceStatus` |
| `PresenceOperations.CheckOutStudent` | [presence_operations.go:209](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L209) | runtime retained | `p.active.CheckOutStudent` |
| `studentMoveResult` | [presence_operations.go:217](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L217) | runtime retained | `make`, `len`, `append` |
| `PresenceOperations.AssignTransitStudents` | [presence_operations.go:228](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L228) | runtime retained | `p.active.AssignTransitStudentsToActiveGroupAuthorized`, `make`, `len`, `append` |
| `PresenceOperations.MoveStudentsToSession` | [presence_operations.go:242](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L242) | runtime retained | `p.active.MoveStudentsToActiveGroupAuthorized`, `studentMoveResult` |
| `PresenceOperations.MoveStudentsToTransit` | [presence_operations.go:250](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L250) | runtime retained | `p.active.MoveStudentsToTransitAuthorized`, `studentMoveResult` |
| `PresenceOperations.SessionVisitsWithDisplay` | [presence_operations.go:258](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L258) | runtime retained | `p.active.GetActiveGroupVisitsWithDisplay`, `make`, `len`, `append` |
| `PresenceOperations.SessionRooms` | [presence_operations.go:276](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L276) | runtime retained | `p.sessions.GetRoomsByIDs`, `make`, `len`, `append` |
| `PresenceOperations.DashboardAnalytics` | [presence_operations.go:291](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L291) | runtime retained | `p.active.GetDashboardAnalytics`, `make`, `len`, `make`, `len`, `make`, `len`, `splitAtSchoolOrKeepHome`, `append`, `append`, `append` |
| `splitAtSchoolOrKeepHome` | [presence_operations.go:328](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L328) | runtime retained | `splitAtSchoolFromHome`, `logger.Warn` |
| `splitAtSchoolFromHome` | [presence_operations.go:340](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L340) | runtime retained | `len`, `counter.CountAtSchoolToday`, `max` |
| `PresenceOperations.CrossTenantStudents` | [presence_operations.go:351](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L351) | runtime retained | `p.active.GetCrossTenantStudents`, `make`, `len`, `append` |
| `PresenceOperations.TrackingIndicators` | [presence_operations.go:365](../../../backend/modules/studentpresence/compose/presenceservice/presence_operations.go#L365) | direct binding | `p.active.GetTrackingIndicators` |
| `statusDays.DeleteStatusDay` | [status_days.go:30](../../../backend/modules/studentpresence/compose/presenceservice/status_days.go#L30) | direct binding | `s.DeleteByID` |
| `statusDays.GetActiveByStudentIDsAndDate` | [status_days.go:36](../../../backend/modules/studentpresence/compose/presenceservice/status_days.go#L36) | direct binding | `s.repo.FindActiveByStudentIDsAndDate` |
| `statusDays.GetSignedOffByStudentIDsAndDate` | [status_days.go:43](../../../backend/modules/studentpresence/compose/presenceservice/status_days.go#L43) | direct binding | `s.repo.FindSignedOffByStudentIDsAndDate` |
| `statusDays.GetActiveByStudentAndDateRange` | [status_days.go:49](../../../backend/modules/studentpresence/compose/presenceservice/status_days.go#L49) | direct binding | `s.repo.FindActiveByStudentAndDateRange` |
| `statusDays.GetByStudentAndDateRange` | [status_days.go:55](../../../backend/modules/studentpresence/compose/presenceservice/status_days.go#L55) | direct binding | `s.repo.FindByStudentAndDateRange` |
| `statusDays.MarkCleared` | [status_days.go:60](../../../backend/modules/studentpresence/compose/presenceservice/status_days.go#L60) | direct binding | `s.repo.MarkCleared` |
| `studentHistory.GetSlotAttendanceByStudentAndDateRange` | [student_history.go:34](../../../backend/modules/studentpresence/compose/presenceservice/student_history.go#L34) | direct binding | `h.slots.Slots` |
| `studentHistory.HasPlannedSlotsInRange` | [student_history.go:38](../../../backend/modules/studentpresence/compose/presenceservice/student_history.go#L38) | direct binding | `h.slots.HasPlannedSlots` |
| `studentHistory.RecordDataAccess` | [student_history.go:42](../../../backend/modules/studentpresence/compose/presenceservice/student_history.go#L42) | runtime retained | `h.accessLog.Create`, `dataAccessEvent` |
| `dataAccessEvent` | [student_history.go:48](../../../backend/modules/studentpresence/compose/presenceservice/student_history.go#L48) | runtime retained | none |
| `noHistorySlots.Slots` | [student_history.go:69](../../../backend/modules/studentpresence/compose/presenceservice/student_history.go#L69) | direct binding | none |
| `noHistorySlots.HasPlannedSlots` | [student_history.go:73](../../../backend/modules/studentpresence/compose/presenceservice/student_history.go#L73) | direct binding | none |

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
| `Repositories.GroupReads` | [repository_adapter_repositories.go:84](../../../backend/modules/timetable/compose/repository_adapter_repositories.go#L84) | direct binding | `application.NewTimetableGroupReads` |
| `Repositories.NativeRecords` | [repository_adapter_repositories.go:89](../../../backend/modules/timetable/compose/repository_adapter_repositories.go#L89) | direct binding | none |
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

| Function | Source | Classification | Calls |
|---|---|---|---|
| `roomSessions.EnsureOpenRoomSession` | [new.go:57](../../../backend/workflows/openroommove/compose/new.go#L57) | runtime retained | `r.presence.EnsureOpenRoomSession` |
| `moves.MoveIntoRoomSession` | [new.go:67](../../../backend/workflows/openroommove/compose/new.go#L67) | runtime retained | `m.presence.MoveStudentsToOpenRoomSessionAuthorized`, `append` |

### `workflows/parentportal/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `careConfig` | [new.go:115](../../../backend/workflows/parentportal/compose/new.go#L115) | runtime retained | `messagingFlows.ChildFlowSharing` |
| `messagingConfig` | [new.go:146](../../../backend/workflows/parentportal/compose/new.go#L146) | direct binding | none |
| `bindOwners` | [new.go:170](../../../backend/workflows/parentportal/compose/new.go#L170) | runtime retained | none |
| `guardianRecords.CreateGuardianContact` | [owner_bindings.go:26](../../../backend/workflows/parentportal/compose/owner_bindings.go#L26) | runtime retained | `g.people.CreateGuardianContact`, `peopledirectory.GuardianContactRecord`, `guardianError` |
| `guardianRecords.UpdateGuardianContact` | [owner_bindings.go:31](../../../backend/workflows/parentportal/compose/owner_bindings.go#L31) | runtime retained | `guardianError`, `g.people.UpdateGuardianContact`, `peopledirectory.GuardianContactRecord` |
| `guardianRecords.ReplaceGuardianPhones` | [owner_bindings.go:35](../../../backend/workflows/parentportal/compose/owner_bindings.go#L35) | runtime retained | `make`, `len`, `append`, `peopledirectory.GuardianPhoneRecord`, `g.people.ReplaceGuardianPhones` |
| `guardianRecords.AddGuardianPhoneRecord` | [owner_bindings.go:43](../../../backend/workflows/parentportal/compose/owner_bindings.go#L43) | runtime retained | `g.people.AddGuardianPhoneRecord`, `peopledirectory.GuardianPhoneRecord` |
| `guardianRecords.SetGuardianPhoneNumber` | [owner_bindings.go:47](../../../backend/workflows/parentportal/compose/owner_bindings.go#L47) | direct binding | `g.people.SetGuardianPhoneNumber` |
| `guardianRecords.DeleteGuardianPhoneRecord` | [owner_bindings.go:51](../../../backend/workflows/parentportal/compose/owner_bindings.go#L51) | direct binding | `g.people.DeleteGuardianPhoneRecord` |
| `guardianRecords.LinkGuardianContact` | [owner_bindings.go:55](../../../backend/workflows/parentportal/compose/owner_bindings.go#L55) | runtime retained | `g.people.LinkGuardianContact`, `peopledirectory.GuardianContactLink` |
| `guardianRecords.PatchGuardianLinkPickup` | [owner_bindings.go:59](../../../backend/workflows/parentportal/compose/owner_bindings.go#L59) | runtime retained | `g.people.PatchGuardianLinkPickup`, `peopledirectory.GuardianLinkPickupPatch` |
| `guardianRecords.SetGuardianPortalLocale` | [owner_bindings.go:63](../../../backend/workflows/parentportal/compose/owner_bindings.go#L63) | direct binding | `g.people.SetGuardianPortalLocale` |
| `guardianRecords.ListGuardianProfileTenants` | [owner_bindings.go:67](../../../backend/workflows/parentportal/compose/owner_bindings.go#L67) | runtime retained | `g.people.ListGuardiansByAccount`, `make`, `len`, `make`, `len`, `append` |
| `guardianError` | [owner_bindings.go:84](../../../backend/workflows/parentportal/compose/owner_bindings.go#L84) | runtime retained | `errors.Is`, `errors.Join` |
| `studentRecords.SetStudentHealthInfo` | [owner_bindings.go:98](../../../backend/workflows/parentportal/compose/owner_bindings.go#L98) | runtime retained | `s.care.SetStudentHealthInfo`, `requireCareProfileWrite` |
| `studentRecords.SetStudentLiveAbsence` | [owner_bindings.go:105](../../../backend/workflows/parentportal/compose/owner_bindings.go#L105) | runtime retained | `s.care.SetStudentLiveStatus`, `requireCareProfileWrite` |
| `requireCareProfileWrite` | [owner_bindings.go:116](../../../backend/workflows/parentportal/compose/owner_bindings.go#L116) | runtime retained | `fmt.Errorf` |
| `studentRecords.SetStudentPhotoConsent` | [owner_bindings.go:126](../../../backend/workflows/parentportal/compose/owner_bindings.go#L126) | runtime retained | `s.people.SetStudentPhotoConsent`, `peopledirectory.StudentPhotoState` |
| `guardianChanges.RecordGuardianChanges` | [owner_bindings.go:134](../../../backend/workflows/parentportal/compose/owner_bindings.go#L134) | runtime retained | `make`, `len`, `append`, `auditlog.GuardianChange`, `g.log.RecordGuardianChanges` |
| `mealPlan.Available` | [platform_bindings.go:28](../../../backend/workflows/parentportal/compose/platform_bindings.go#L28) | runtime retained | `m.provider.Available`, `mealPlanError` |
| `mealPlan.Week` | [platform_bindings.go:33](../../../backend/workflows/parentportal/compose/platform_bindings.go#L33) | runtime retained | `m.provider.Week`, `mealplanModule.Date`, `monday.String`, `mealPlanError`, `make`, `len`, `append`, `string` |
| `mealPlan.RegistrationAvailable` | [platform_bindings.go:45](../../../backend/workflows/parentportal/compose/platform_bindings.go#L45) | runtime retained | `m.provider.RegistrationAvailable`, `mealPlanError` |
| `mealPlan.Participation` | [platform_bindings.go:50](../../../backend/workflows/parentportal/compose/platform_bindings.go#L50) | runtime retained | `m.provider.Participation`, `mealplanModule.Date`, `from.String`, `mealplanModule.Date`, `to.String`, `mealPlanError`, `string`, `make`, `len`, `make`, `len`, `append`, `care.MealWeekday`, `append`, `string`, `string` |
| `mealPlan.ReplaceParticipationSchedule` | [platform_bindings.go:70](../../../backend/workflows/parentportal/compose/platform_bindings.go#L70) | runtime retained | `make`, `len`, `append`, `mealplanModule.Weekday`, `m.provider.ReplaceParticipationSchedule`, `string`, `mealPlanError` |
| `mealPlan.SetParticipationForDay` | [platform_bindings.go:81](../../../backend/workflows/parentportal/compose/platform_bindings.go#L81) | runtime retained | `mealPlanError`, `m.provider.SetParticipationForDay`, `mealParticipationDay` |
| `mealPlan.ClearParticipationForDay` | [platform_bindings.go:85](../../../backend/workflows/parentportal/compose/platform_bindings.go#L85) | runtime retained | `mealParticipationDay`, `mealPlanError`, `m.provider.ClearParticipationForDay` |
| `mealParticipationDay` | [platform_bindings.go:91](../../../backend/workflows/parentportal/compose/platform_bindings.go#L91) | runtime retained | `mealplanModule.Date`, `change.Date.String` |
| `mealPlanError` | [platform_bindings.go:98](../../../backend/workflows/parentportal/compose/platform_bindings.go#L98) | runtime retained | `errors.Is`, `errors.Join`, `errors.Is`, `errors.Join`, `errors.Is`, `errors.Join`, `errors.Is`, `errors.Join` |
| `children.ResolvePermittedChild` | [platform_bindings.go:119](../../../backend/workflows/parentportal/compose/platform_bindings.go#L119) | direct binding | `(*c.flows).ResolvePermittedChild` |

### `workflows/reminderdelivery/compose`

No non-constructor functions.


### `workflows/sessionend/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `*notifier.log` | [notifier.go:32](../../../backend/workflows/sessionend/compose/notifier.go#L32) | runtime retained | `slog.Default` |
| `*notifier.SessionEnded` | [notifier.go:39](../../../backend/workflows/sessionend/compose/notifier.go#L39) | runtime retained | `strconv.FormatInt`, `n.announceCheckouts`, `n.announceActivityEnd`, `n.announceInstanceCompleted`, `n.wakeGuardians` |
| `*notifier.announceCheckouts` | [notifier.go:47](../../../backend/workflows/sessionend/compose/notifier.go#L47) | runtime retained | `len`, `make`, `len`, `make`, `make`, `strconv.FormatInt`, `append`, `append`, `append`, `append`, `strconv.FormatInt`, `len`, `n.toGroup`, `realtime.NewEvent`, `strconv.FormatInt`, `realtime.NewEvent`, `n.toGroup`, `n.refreshDashboards` |
| `*notifier.announceActivityEnd` | [notifier.go:89](../../../backend/workflows/sessionend/compose/notifier.go#L89) | runtime retained | `strconv.FormatInt`, `realtime.NewEvent`, `n.toGroup`, `n.refreshDashboards` |
| `*notifier.announceInstanceCompleted` | [notifier.go:103](../../../backend/workflows/sessionend/compose/notifier.go#L103) | runtime retained | `strconv.FormatInt`, `strconv.FormatInt`, `n.broadcaster.BroadcastToTenant`, `realtime.NewEvent`, `n.log().Warn`, `n.log`, `slog.String`, `err.Error`, `slog.Int64`, `slog.String` |
| `*notifier.wakeGuardians` | [notifier.go:124](../../../backend/workflows/sessionend/compose/notifier.go#L124) | runtime retained | `n.guardians.BroadcastChildUpdateToGuardians` |
| `*notifier.refreshDashboards` | [notifier.go:133](../../../backend/workflows/sessionend/compose/notifier.go#L133) | runtime retained | `len`, `n.broadcaster.BroadcastToTenant`, `realtime.NewEvent`, `n.log().Warn`, `n.log`, `slog.String`, `err.Error`, `slog.String`, `slog.String` |
| `*notifier.toGroup` | [notifier.go:147](../../../backend/workflows/sessionend/compose/notifier.go#L147) | runtime retained | `n.broadcaster.BroadcastToGroup`, `n.log().Warn`, `n.log`, `slog.String`, `err.Error`, `slog.String`, `string`, `slog.String` |

### `workflows/shiftplansync/compose`

No non-constructor functions.


### `workflows/staffclock/compose`

No non-constructor functions.


### `workflows/staffoffboarding/compose`

No non-constructor functions.


### `workflows/studentdeletion/compose`

No non-constructor functions.


## Callback bodies, including constructors

A callback that only forwards native values is binding. Multiple statements, branches or iterations below are retained runtime unless the enclosing factory only assembles the callback itself. These entries are not claimed as constructor-only evidence.

| Package | Enclosing declaration | Source line | Statements | Branches/loops | Calls | Scope |
|---|---|---|---:|---:|---:|---|
| `modules/careplan/compose` | `NewExcusedAbsenceRequests` | [excused_requests.go:63](../../../backend/modules/careplan/compose/excused_requests.go#L63) | 0 | 0 | 0 | production |
| `modules/careplan/compose` | `LockWeeklyPickupStudents` | [pickup_excusal.go:44](../../../backend/modules/careplan/compose/pickup_excusal.go#L44) | 1 | 0 | 1 | production |
| `modules/dataimport/compose` | `Observe` | [transactions.go:39](../../../backend/modules/dataimport/compose/transactions.go#L39) | 1 | 0 | 1 | production |
| `modules/delivery/compose` | `NewPushAuthorizationChecker` | [provider.go:40](../../../backend/modules/delivery/compose/provider.go#L40) | 1 | 0 | 1 | production |
| `modules/delivery/compose` | `NewExpiredPushSubscriptionCleaner` | [provider.go:48](../../../backend/modules/delivery/compose/provider.go#L48) | 1 | 0 | 1 | production |
| `modules/delivery/compose` | `NewTenantTransactions` | [tenant_transactions.go:23](../../../backend/modules/delivery/compose/tenant_transactions.go#L23) | 1 | 0 | 2 | production |
| `modules/delivery/compose` | `NewTenantTransactions` | [tenant_transactions.go:24](../../../backend/modules/delivery/compose/tenant_transactions.go#L24) | 1 | 0 | 1 | production |
| `modules/enrollment/compose` | `tenantRuntime` | [application.go:156](../../../backend/modules/enrollment/compose/application.go#L156) | 1 | 0 | 3 | production |
| `modules/enrollment/compose` | `tenantRuntime` | [application.go:157](../../../backend/modules/enrollment/compose/application.go#L157) | 1 | 0 | 1 | production |
| `modules/enrollment/compose` | `tenantRuntime` | [application.go:159](../../../backend/modules/enrollment/compose/application.go#L159) | 1 | 0 | 3 | production |
| `modules/enrollment/compose` | `tenantRuntime` | [application.go:160](../../../backend/modules/enrollment/compose/application.go#L160) | 1 | 0 | 1 | production |
| `modules/exporttransfer/compose` | `retainedNewDetachedContext` | [compose_bindings_callback_detached_context.go:10](../../../backend/modules/exporttransfer/compose/compose_bindings_callback_detached_context.go#L10) | 1 | 0 | 2 | production |
| `modules/facilities/compose` | `NewUnobserved` | [unobserved.go:17](../../../backend/modules/facilities/compose/unobserved.go#L17) | 1 | 0 | 0 | production |
| `modules/facilities/compose` | `NewUnobserved` | [unobserved.go:21](../../../backend/modules/facilities/compose/unobserved.go#L21) | 0 | 0 | 0 | production |
| `modules/identityaccess/compose` | `newAccountLifecycle` | [account_lifecycle.go:60](../../../backend/modules/identityaccess/compose/account_lifecycle.go#L60) | 1 | 0 | 0 | production |
| `modules/identityaccess/compose` | `NewInvitationMaintenance` | [maintenance.go:25](../../../backend/modules/identityaccess/compose/maintenance.go#L25) | 0 | 0 | 0 | production |
| `modules/identityaccess/compose` | `NewGuardianSeedAccess` | [maintenance.go:36](../../../backend/modules/identityaccess/compose/maintenance.go#L36) | 0 | 0 | 0 | production |
| `modules/identityaccess/compose` | `NewSessionMaintenance` | [maintenance.go:77](../../../backend/modules/identityaccess/compose/maintenance.go#L77) | 0 | 0 | 0 | production |
| `modules/identityaccess/compose` | `NewSessionMaintenance` | [maintenance.go:84](../../../backend/modules/identityaccess/compose/maintenance.go#L84) | 1 | 0 | 0 | production |
| `modules/identityaccess/compose` | `withPasskeyFlows` | [mfa.go:91](../../../backend/modules/identityaccess/compose/mfa.go#L91) | 1 | 0 | 1 | production |
| `modules/identityaccess/compose` | `newCommandTransactions` | [new.go:163](../../../backend/modules/identityaccess/compose/new.go#L163) | 1 | 0 | 0 | production |
| `modules/peopledirectory/compose` | `LoadForTenant` | [guardian_profile_loader.go:39](../../../backend/modules/peopledirectory/compose/guardian_profile_loader.go#L39) | 1 | 0 | 0 | production |
| `modules/peopledirectory/compose` | `LoadForTenant` | [guardian_profile_loader.go:39](../../../backend/modules/peopledirectory/compose/guardian_profile_loader.go#L39) | 1 | 0 | 0 | production |
| `modules/peopledirectory/compose` | `MustNewReads` | [new.go:145](../../../backend/modules/peopledirectory/compose/new.go#L145) | 0 | 0 | 0 | production |
| `modules/settings/compose` | `Schema` | [operator.go:65](../../../backend/modules/settings/compose/operator.go#L65) | 1 | 0 | 0 | production |
| `modules/settings/compose` | `Reveal` | [operator.go:69](../../../backend/modules/settings/compose/operator.go#L69) | 1 | 0 | 0 | production |
| `modules/transactionruntime/compose` | `NewDemoTenantRuntime` | [tenant_runtime.go:30](../../../backend/modules/transactionruntime/compose/tenant_runtime.go#L30) | 0 | 0 | 0 | production |
| `modules/transactionruntime/compose` | `NewDemoTenantRuntime` | [tenant_runtime.go:31](../../../backend/modules/transactionruntime/compose/tenant_runtime.go#L31) | 1 | 0 | 0 | production |
| `modules/transactionruntime/compose` | `TenantCallback` | [tenant_runtime.go:66](../../../backend/modules/transactionruntime/compose/tenant_runtime.go#L66) | 1 | 0 | 1 | production |
| `modules/workforce/compose` | `NewStaffShiftLock` | [staff_shift_lock.go:19](../../../backend/modules/workforce/compose/staff_shift_lock.go#L19) | 1 | 0 | 1 | production |
| `modules/workforce/compose` | `NewStaffEmployment` | [staffemployment.go:23](../../../backend/modules/workforce/compose/staffemployment.go#L23) | 0 | 0 | 0 | production |
| `modules/careplan/requestfeed/compose` | `New` | [new.go:32](../../../backend/modules/careplan/requestfeed/compose/new.go#L32) | 4 | 3 | 3 | production |
| `modules/communication/composition` | `ListParentMessageInbox` | [messages.go:79](../../../backend/modules/communication/composition/messages.go#L79) | 2 | 0 | 3 | production |
| `modules/communication/composition` | `CountUnreadParentMessages` | [messages.go:86](../../../backend/modules/communication/composition/messages.go#L86) | 2 | 0 | 2 | production |
| `modules/communication/composition` | `GetParentMessageThread` | [messages.go:93](../../../backend/modules/communication/composition/messages.go#L93) | 3 | 1 | 3 | production |
| `modules/communication/composition` | `ListMessageableGuardians` | [messages.go:103](../../../backend/modules/communication/composition/messages.go#L103) | 5 | 2 | 5 | production |
| `modules/communication/composition` | `ListParentMessageThreadsForStudent` | [messages.go:120](../../../backend/modules/communication/composition/messages.go#L120) | 2 | 0 | 3 | production |
| `modules/communication/composition` | `StartParentMessageThread` | [messages.go:127](../../../backend/modules/communication/composition/messages.go#L127) | 3 | 1 | 3 | production |
| `modules/communication/composition` | `OpenParentMessageThread` | [messages.go:137](../../../backend/modules/communication/composition/messages.go#L137) | 3 | 1 | 3 | production |
| `modules/communication/composition` | `PostParentMessage` | [messages.go:147](../../../backend/modules/communication/composition/messages.go#L147) | 3 | 1 | 3 | production |
| `modules/communication/composition` | `MarkParentMessageThreadUnread` | [messages.go:157](../../../backend/modules/communication/composition/messages.go#L157) | 1 | 0 | 2 | production |
| `modules/communication/composition` | `MarkAllParentMessagesRead` | [messages.go:164](../../../backend/modules/communication/composition/messages.go#L164) | 2 | 0 | 2 | production |
| `modules/communication/composition` | `ParentMessageCountScope` | [messages.go:171](../../../backend/modules/communication/composition/messages.go#L171) | 5 | 2 | 4 | production |
| `modules/communication/composition` | `SetParentMessageCountScope` | [messages.go:185](../../../backend/modules/communication/composition/messages.go#L185) | 1 | 0 | 2 | production |
| `modules/communication/composition` | `ListStaffMessageInbox` | [messages.go:222](../../../backend/modules/communication/composition/messages.go#L222) | 5 | 2 | 5 | production |
| `modules/communication/composition` | `CountUnreadStaffMessages` | [messages.go:241](../../../backend/modules/communication/composition/messages.go#L241) | 2 | 0 | 2 | production |
| `modules/communication/composition` | `ListMessageableStaff` | [messages.go:248](../../../backend/modules/communication/composition/messages.go#L248) | 5 | 2 | 5 | production |
| `modules/communication/composition` | `OpenStaffMessageThread` | [messages.go:262](../../../backend/modules/communication/composition/messages.go#L262) | 2 | 0 | 3 | production |
| `modules/communication/composition` | `GetStaffMessageThread` | [messages.go:269](../../../backend/modules/communication/composition/messages.go#L269) | 2 | 0 | 3 | production |
| `modules/communication/composition` | `PostStaffMessage` | [messages.go:276](../../../backend/modules/communication/composition/messages.go#L276) | 4 | 1 | 4 | production |
| `modules/communication/composition` | `CleanupExpiredStaffMessages` | [messages.go:287](../../../backend/modules/communication/composition/messages.go#L287) | 2 | 0 | 2 | production |
| `modules/communication/composition` | `observeResult` | [messages.go:300](../../../backend/modules/communication/composition/messages.go#L300) | 1 | 2 | 3 | production |
| `modules/communication/composition` | `AudienceMapping` | [new.go:121](../../../backend/modules/communication/composition/new.go#L121) | 1 | 0 | 1 | production |
| `modules/communication/composition` | `AudienceMapping` | [new.go:122](../../../backend/modules/communication/composition/new.go#L122) | 1 | 0 | 0 | production |
| `modules/communication/composition` | `AudienceMapping` | [new.go:125](../../../backend/modules/communication/composition/new.go#L125) | 1 | 0 | 1 | production |
| `modules/communication/composition` | `AudienceMapping` | [new.go:126](../../../backend/modules/communication/composition/new.go#L126) | 1 | 0 | 0 | production |
| `modules/communication/composition` | `StoredConsent` | [notification_consent.go:38](../../../backend/modules/communication/composition/notification_consent.go#L38) | 1 | 0 | 1 | production |
| `modules/communication/composition` | `RecordConsent` | [notification_consent.go:44](../../../backend/modules/communication/composition/notification_consent.go#L44) | 1 | 0 | 1 | production |
| `modules/communication/composition` | `DisableConsent` | [notification_consent.go:51](../../../backend/modules/communication/composition/notification_consent.go#L51) | 1 | 0 | 1 | production |
| `modules/communication/composition` | `HasAnyOptedIn` | [notification_consent.go:58](../../../backend/modules/communication/composition/notification_consent.go#L58) | 1 | 0 | 1 | production |
| `modules/communication/composition` | `FilterOptedIn` | [notification_consent.go:64](../../../backend/modules/communication/composition/notification_consent.go#L64) | 1 | 0 | 1 | production |
| `modules/communication/composition` | `ListOptedIn` | [notification_consent.go:70](../../../backend/modules/communication/composition/notification_consent.go#L70) | 1 | 0 | 1 | production |
| `modules/communication/composition` | `FilterOptedInByType` | [notification_consent.go:76](../../../backend/modules/communication/composition/notification_consent.go#L76) | 1 | 0 | 1 | production |
| `modules/communication/composition` | `FilterNotOptedOut` | [notification_consent.go:82](../../../backend/modules/communication/composition/notification_consent.go#L82) | 1 | 0 | 1 | production |
| `modules/communication/composition` | `parentDatabase` | [parent_store.go:27](../../../backend/modules/communication/composition/parent_store.go#L27) | 2 | 0 | 5 | production |
| `modules/communication/parentstore` | `mergeFeedItems` | [announcements.go:425](../../../backend/modules/communication/parentstore/announcements.go#L425) | 1 | 0 | 6 | production |
| `modules/communication/parentstore` | `postgresDatabase` | [parentstore.go:55](../../../backend/modules/communication/parentstore/parentstore.go#L55) | 4 | 1 | 4 | production |
| `modules/communication/parentstore` | `ProjectionDatabase` | [parentstore.go:67](../../../backend/modules/communication/parentstore/parentstore.go#L67) | 1 | 0 | 1 | production |
| `modules/communication/parentstore` | `CurrentTenantStaffAccounts` | [staff_projection.go:49](../../../backend/modules/communication/parentstore/staff_projection.go#L49) | 5 | 2 | 4 | production |
| `modules/communication/staffstore` | `postgresDatabase` | [staffstore.go:63](../../../backend/modules/communication/staffstore/staffstore.go#L63) | 1 | 0 | 1 | production |
| `modules/communication/staffstore` | `ProjectionDatabase` | [staffstore.go:68](../../../backend/modules/communication/staffstore/staffstore.go#L68) | 1 | 0 | 1 | production |
| `modules/feedback/compose` | `New` | [new.go:39](../../../backend/modules/feedback/compose/new.go#L39) | 7 | 3 | 5 | production |
| `modules/feedback/compose` | `New` | [new.go:54](../../../backend/modules/feedback/compose/new.go#L54) | 3 | 1 | 5 | production |
| `modules/feedback/compose` | `New` | [new.go:61](../../../backend/modules/feedback/compose/new.go#L61) | 2 | 0 | 2 | production |
| `modules/mealplan/compose` | `New` | [new.go:41](../../../backend/modules/mealplan/compose/new.go#L41) | 7 | 3 | 5 | production |
| `modules/requestreview/compose` | `Open` | [excused_queue.go:32](../../../backend/modules/requestreview/compose/excused_queue.go#L32) | 1 | 0 | 1 | production |
| `modules/schoolmembership/compose` | `StudentClassWriteGate` | [student_class_gate.go:21](../../../backend/modules/schoolmembership/compose/student_class_gate.go#L21) | 1 | 0 | 1 | production |
| `modules/studentpresence/compose` | `FailureLogger` | [observers.go:8](../../../backend/modules/studentpresence/compose/observers.go#L8) | 1 | 1 | 2 | production |
| `modules/studentpresence/compose` | `DebugLogger` | [observers.go:21](../../../backend/modules/studentpresence/compose/observers.go#L21) | 1 | 0 | 1 | production |
| `workflows/openroommove/compose` | `New` | [new.go:47](../../../backend/workflows/openroommove/compose/new.go#L47) | 1 | 0 | 1 | production |
