# Final legacy cluster: round 8 compose audit

Base: `148e5aeb86cf22fdbfbffedd76ba070f52f80a3e`. This inventories **every policy package with role `compose`**, except the root-composition and dev-tools owners. Package role, not directory spelling, selects the inventory. Test-hook declarations are listed separately from production. A LOC reduction alone does not prove the exit.

Classification: **direct binding** has no local branch/loop and at most one delegated call; it supplies a native capability or type conversion without adding a decision. **construction/binding** identifies reviewed startup, callback-signature or runtime-capability wiring. **runtime retained** contains local orchestration, row/error mapping or execution checks and prevents the literal exit. **test-only** does not enter the production exit. Constructor callbacks are audited below too, so a `New` prefix cannot hide runtime logic.


## Package summary

| Compose package | Owner | Non-constructors | Runtime retained |
|---|---|---:|---:|
| `modules/appointments/compose` | `appointments` | 2 | 2 |
| `modules/auditlog/compose` | `audit-platform` | 0 | 0 |
| `modules/careplan/compose` | `care-plan` | 81 | 49 |
| `modules/careplan/requestfeed/compose` | `care-plan` | 2 | 0 |
| `modules/classday/compose` | `class-day-view` | 20 | 13 |
| `modules/communication/composition` | `communication` | 78 | 71 |
| `modules/communication/parentstore` | `communication` | 241 | 104 |
| `modules/communication/staffstore` | `communication` | 6 | 2 |
| `modules/dataimport/compose` | `data-import` | 10 | 6 |
| `modules/dataimport/compose/imports` | `data-import` | 0 | 0 |
| `modules/delivery/compose` | `delivery-platform` | 30 | 20 |
| `modules/devicefleet/compose` | `device-fleet` | 16 | 8 |
| `modules/devicefleet/compose/repositoryadapter` | `device-fleet` | 0 | 0 |
| `modules/devicescan/compose` | `process-device-scan` | 64 | 48 |
| `modules/emergencysnapshot/compose` | `emergency-snapshot` | 0 | 0 |
| `modules/enrollment/compose` | `enrollment` | 12 | 9 |
| `modules/exporttransfer/compose` | `export-transfer` | 6 | 5 |
| `modules/facilities/compose` | `facilities` | 11 | 4 |
| `modules/facilities/compose/repositoryadapter` | `facilities` | 0 | 0 |
| `modules/feedback/compose` | `feedback` | 3 | 0 |
| `modules/filestorage/compose` | `file-storage` | 19 | 8 |
| `modules/grouplive/compose` | `group-live-view` | 0 | 0 |
| `modules/guardianlinkview/compose` | `guardian-link-view` | 0 | 0 |
| `modules/identityaccess/compose` | `identity-access` | 40 | 19 |
| `modules/mealplan/compose` | `meal-plan` | 4 | 0 |
| `modules/operatordashboard` | `operator-dashboard-view` | 1 | 0 |
| `modules/organizationtenancy/compose` | `organization-tenancy` | 24 | 11 |
| `modules/parentannouncementaudience` | `parent-announcement-audience` | 0 | 0 |
| `modules/parentmessageinbox` | `parent-message-inbox` | 0 | 0 |
| `modules/parentrequestfeedprojection/compose` | `parent-request-feed-view` | 0 | 0 |
| `modules/peopledirectory/compose` | `people-directory` | 39 | 15 |
| `modules/presenceprojection/compose` | `presence-legacy-view` | 0 | 0 |
| `modules/requestreview/compose` | `request-review-view` | 40 | 37 |
| `modules/schedulerruntime/compose` | `scheduler-runtime` | 1 | 0 |
| `modules/schoolcalendar/compose` | `school-calendar` | 2 | 2 |
| `modules/schoolcalendar/portal/compose` | `school-calendar` | 5 | 4 |
| `modules/schoolmembership/compose` | `school-membership` | 2 | 0 |
| `modules/schoolsetup/compose` | `school-setup` | 2 | 1 |
| `modules/schoolstructure/compose` | `school-structure` | 23 | 0 |
| `modules/settings/compose` | `settings-platform` | 17 | 11 |
| `modules/staffannouncementaudience` | `staff-announcement-audience` | 1 | 0 |
| `modules/staffmessageinbox` | `staff-message-inbox` | 0 | 0 |
| `modules/studentdirectoryview/compose` | `student-directory-view` | 0 | 0 |
| `modules/studentpresence/compose` | `student-presence` | 42 | 31 |
| `modules/studentpresence/compose/presenceservice` | `student-presence` | 72 | 45 |
| `modules/supervisiondashboard/compose` | `calendar-view` | 1 | 0 |
| `modules/timetable/compose` | `timetable-activities` | 36 | 11 |
| `modules/timetableenrollmentprovenance/compose` | `timetable-enrollment-provenance` | 0 | 0 |
| `modules/timetableprojection/compose` | `timetable-legacy-view` | 0 | 0 |
| `modules/transactionruntime/compose` | `transaction-runtime` | 10 | 1 |
| `modules/workforce/compose` | `workforce` | 22 | 16 |
| `observability/compose` | `observability` | 1 | 0 |
| `workflows/gradetransition/compose` | `grade-transition` | 0 | 0 |
| `workflows/openroommove/compose` | `open-room-move` | 2 | 2 |
| `workflows/parentportal/compose` | `parent-portal` | 29 | 24 |
| `workflows/reminderdelivery/compose` | `reminder-delivery` | 0 | 0 |
| `workflows/sessionend/compose` | `session-end` | 8 | 8 |
| `workflows/shiftplansync/compose` | `shift-plan-sync` | 5 | 1 |
| `workflows/staffclock/compose` | `staff-clock` | 0 | 0 |
| `workflows/staffoffboarding/compose` | `staff-offboarding` | 0 | 0 |
| `workflows/studentdeletion/compose` | `student-deletion` | 0 | 0 |

## Every remaining non-constructor


### `modules/appointments/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `database` | [new.go:37](../../../backend/modules/appointments/compose/new.go#L37) | runtime retained | `tenant.FromContext`, `tenant.TransactionFromContext`, `fmt.Errorf`, `fmt.Errorf`, `fmt.Errorf` |
| `withinTenant` | [new.go:58](../../../backend/modules/appointments/compose/new.go#L58) | runtime retained | `tenant.TransactionFromContext`, `command`, `tenant.TenantFromContext`, `errors.New`, `tenant.WithinTenant` |

### `modules/auditlog/compose`

No non-constructor functions.


### `modules/careplan/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `TenantAmbientDatabase` | [ambient_database.go:14](../../../backend/modules/careplan/compose/ambient_database.go#L14) | runtime retained | `panic` |
| `arrivalScheduleRecords` | [arrival_schedules.go:61](../../../backend/modules/careplan/compose/arrival_schedules.go#L61) | runtime retained | `application.NewEffectiveRecords[careplan.ArrivalSchedule, *careplan.ArrivalSchedule]`, `application.Unlocked` |
| `arrivalExceptionRecords` | [arrival_schedules.go:66](../../../backend/modules/careplan/compose/arrival_schedules.go#L66) | direct binding | `application.NewExceptionRecords[careplan.ArrivalException, *careplan.ArrivalException]` |
| `arrivalNoteRecords` | [arrival_schedules.go:71](../../../backend/modules/careplan/compose/arrival_schedules.go#L71) | runtime retained | `application.NewNoteRecords[careplan.ArrivalNote, *careplan.ArrivalNote]`, `application.Unlocked` |
| `BookingRowNotFound` | [booking_materialization.go:36](../../../backend/modules/careplan/compose/booking_materialization.go#L36) | direct binding | `ports.BookingRowNotFound` |
| `bookingDependencies` | [booking_materialization.go:66](../../../backend/modules/careplan/compose/booking_materialization.go#L66) | runtime retained | `tenant.NewTransactionRunner`, `application.AnnounceOfferingPickupChange` |
| `documentFileRecords.MarkFileDeleted` | [care_lifecycle.go:136](../../../backend/modules/careplan/compose/care_lifecycle.go#L136) | direct binding | `r.records.MarkCareDocumentFileDeleted` |
| `documentFileRecords.MarkQueuedCleanupCompleteByFilename` | [care_lifecycle.go:140](../../../backend/modules/careplan/compose/care_lifecycle.go#L140) | direct binding | `r.records.CompleteCareDocumentCleanupByFilename` |
| `documentFileRecords.ActivateQueuedCleanup` | [care_lifecycle.go:144](../../../backend/modules/careplan/compose/care_lifecycle.go#L144) | direct binding | `r.records.ActivateCareDocumentCleanup` |
| `ownTenantTransaction` | [care_lifecycle.go:150](../../../backend/modules/careplan/compose/care_lifecycle.go#L150) | runtime retained | `tenant.WithTenantTx`, `tenant.FromContext`, `command` |
| `CatalogRowNotFound` | [care_offering_catalog.go:42](../../../backend/modules/careplan/compose/care_offering_catalog.go#L42) | direct binding | `ports.CatalogRowNotFound` |
| `catalogDependencies` | [care_offering_catalog.go:73](../../../backend/modules/careplan/compose/care_offering_catalog.go#L73) | direct binding | none |
| `cleanupRetention.DeleteFinishedStudentDocumentCleanups` | [cleanup_retention.go:29](../../../backend/modules/careplan/compose/cleanup_retention.go#L29) | direct binding | `r.deleteFinished` |
| `cleanupRetention.DeleteFinishedWithdrawalCompletions` | [cleanup_retention.go:33](../../../backend/modules/careplan/compose/cleanup_retention.go#L33) | direct binding | `r.deleteFinished` |
| `cleanupRetention.deleteFinished` | [cleanup_retention.go:37](../../../backend/modules/careplan/compose/cleanup_retention.go#L37) | runtime retained | `deleteBefore`, `int`, `window.Hours` |
| `retentionDatabase` | [cleanup_retention.go:47](../../../backend/modules/careplan/compose/cleanup_retention.go#L47) | runtime retained | `tenant.TenantFromContext`, `fmt.Errorf`, `tenant.TransactionFromContext`, `errors.New`, `fmt.Errorf`, `tenantID.Int64` |
| `dayLocks.LockStudentAndExceptionDay` | [day_locks.go:31](../../../backend/modules/careplan/compose/day_locks.go#L31) | runtime retained | `errors.New`, `l.database`, `fmt.Errorf`, `l.student`, `errors.Is`, `fmt.Errorf`, `l.LockExceptionDay` |
| `dayLocks.LockExceptionDay` | [day_locks.go:47](../../../backend/modules/careplan/compose/day_locks.go#L47) | runtime retained | `l.database`, `fmt.Errorf`, `postgres.LockExceptionDay`, `fmt.Errorf` |
| `TenantHooks.TenantID` | [excused_requests.go:78](../../../backend/modules/careplan/compose/excused_requests.go#L78) | direct binding | `tenant.FromContext` |
| `TenantHooks.AfterCommit` | [excused_requests.go:80](../../../backend/modules/careplan/compose/excused_requests.go#L80) | direct binding | `tenant.RegisterAfterCommit` |
| `guardianPickupDatabase` | [guardian_pickup.go:28](../../../backend/modules/careplan/compose/guardian_pickup.go#L28) | runtime retained | `tenant.CurrentTransaction[bun.IDB]`, `tenant.FromContext` |
| `guardianPickupPermissions.CreateGuardianPickupPermission` | [guardian_pickup.go:40](../../../backend/modules/careplan/compose/guardian_pickup.go#L40) | runtime retained | `time.Now`, `p.store.CreateGuardianPickupPermission`, `p.observe`, `time.Since` |
| `guardianPickupPermissions.ChangeGuardianPickupPermission` | [guardian_pickup.go:47](../../../backend/modules/careplan/compose/guardian_pickup.go#L47) | runtime retained | `time.Now`, `p.store.ChangeGuardianPickupPermission`, `p.observe`, `time.Since` |
| `*ManualPartialAbsences.Dates` | [manual_partial_absences.go:20](../../../backend/modules/careplan/compose/manual_partial_absences.go#L20) | runtime retained | `r.source.ListPickupExceptions`, `make`, `len`, `append` |
| `bindCareLocks` | [new.go:56](../../../backend/modules/careplan/compose/new.go#L56) | runtime retained | `careplanning.BindStudentLockForDB`, `careplanning.BindExceptionDayLockForDB`, `tenant.FromContext`, `errors.New`, `dependencies.AmbientDB`, `postgres.LockExceptionDay` |
| `carePlanDatabase` | [new.go:71](../../../backend/modules/careplan/compose/new.go#L71) | runtime retained | `tenant.TransactionFromContext`, `postgres.ResolveDatabase`, `tenant.FromContext` |
| `engineRuntime.WithinTenant` | [new.go:82](../../../backend/modules/careplan/compose/new.go#L82) | runtime retained | `tenant.TransactionFromContext`, `command`, `tenant.FromContext`, `tenant.NewTenantID`, `errors.New`, `tenant.WithinTenant` |
| `engineRuntime.LockStudentAndExceptionDay` | [new.go:94](../../../backend/modules/careplan/compose/new.go#L94) | runtime retained | `carePlanDatabase(r.db)`, `carePlanDatabase`, `careplanning.LockStudentAndExceptionDay` |
| `engineRuntime.LockExceptionDay` | [new.go:101](../../../backend/modules/careplan/compose/new.go#L101) | direct binding | `careplanning.LockExceptionDay` |
| `engineRuntime.LockStudent` | [new.go:105](../../../backend/modules/careplan/compose/new.go#L105) | direct binding | `careplanning.LockStudent` |
| `observeOfferingBookings` | [offering_booking_metrics.go:30](../../../backend/modules/careplan/compose/offering_booking_metrics.go#L30) | runtime retained | `offeringBookingMetricsOnce.Do`, `prometheus.MustRegister`, `careplan.ErrorCode`, `errors.As`, `databaseError.Field`, `offeringBookingOperations.WithLabelValues(event.Operation, event.Kind, outcome, code).Inc`, `offeringBookingOperations.WithLabelValues`, `offeringBookingDuration.WithLabelValues(event.Operation, event.Kind).Observe`, `offeringBookingDuration.WithLabelValues`, `event.Duration.Seconds`, `offeringBookingRows.WithLabelValues(event.Operation, "input").Observe`, `offeringBookingRows.WithLabelValues`, `float64`, `offeringBookingRows.WithLabelValues(event.Operation, "output").Observe`, `offeringBookingRows.WithLabelValues`, `float64` |
| `tenantUnitOfWork.TenantID` | [offering_changes.go:106](../../../backend/modules/careplan/compose/offering_changes.go#L106) | direct binding | `tenant.FromContext` |
| `tenantUnitOfWork.RunInTenantTx` | [offering_changes.go:108](../../../backend/modules/careplan/compose/offering_changes.go#L108) | runtime retained | `tenant.WithTenantTx`, `(*bun.DB)`, `tenant.FromContext`, `fn` |
| `tenantUnitOfWork.MarkRollback` | [offering_changes.go:114](../../../backend/modules/careplan/compose/offering_changes.go#L114) | direct binding | `tenant.MarkRollback` |
| `TenantHooks.MarkRollback` | [parent_requests.go:117](../../../backend/modules/careplan/compose/parent_requests.go#L117) | direct binding | `tenant.MarkRollback` |
| `excusedParentRequests.GetExcusedBulkCandidate` | [parent_requests.go:125](../../../backend/modules/careplan/compose/parent_requests.go#L125) | runtime retained | `p.requests.GetExcusedBulkCandidate`, `excusedParentRequestError` |
| `excusedParentRequests.LockExcusedBulkRequest` | [parent_requests.go:133](../../../backend/modules/careplan/compose/parent_requests.go#L133) | runtime retained | `excusedParentRequestError`, `p.requests.LockExcusedBulkRequest` |
| `excusedParentRequests.ApproveExcusedBulk` | [parent_requests.go:137](../../../backend/modules/careplan/compose/parent_requests.go#L137) | runtime retained | `excusedParentRequestError`, `p.requests.ApproveExcusedBulk` |
| `excusedParentRequests.ConflictCandidate` | [parent_requests.go:141](../../../backend/modules/careplan/compose/parent_requests.go#L141) | runtime retained | `p.requests.ConflictCandidate`, `excusedParentRequestError` |
| `excusedParentRequests.LockConflictRequest` | [parent_requests.go:149](../../../backend/modules/careplan/compose/parent_requests.go#L149) | runtime retained | `excusedParentRequestError`, `p.requests.LockConflictRequest` |
| `excusedParentRequests.DecideConflictRequest` | [parent_requests.go:153](../../../backend/modules/careplan/compose/parent_requests.go#L153) | runtime retained | `excusedParentRequestError`, `p.requests.DecideConflictRequest` |
| `excusedParentRequests.WriteStaffValue` | [parent_requests.go:162](../../../backend/modules/careplan/compose/parent_requests.go#L162) | runtime retained | `excusedParentRequestError`, `p.requests.WriteStaffValue` |
| `excusedParentRequestError` | [parent_requests.go:175](../../../backend/modules/careplan/compose/parent_requests.go#L175) | runtime retained | `errors.Is`, `errors.Is`, `errors.Is`, `parentRequestLifecycleError` |
| `parentRequestLifecycleError` | [parent_requests.go:192](../../../backend/modules/careplan/compose/parent_requests.go#L192) | runtime retained | `errors.Is` |
| `LockWeeklyPickupStudents` | [pickup_excusal.go:44](../../../backend/modules/careplan/compose/pickup_excusal.go#L44) | direct binding | `application.LockWeeklyPickupStudents` |
| `excusalTransaction.TenantID` | [pickup_excusal.go:60](../../../backend/modules/careplan/compose/pickup_excusal.go#L60) | direct binding | `tenant.FromContext` |
| `excusalTransaction.WithinTenant` | [pickup_excusal.go:61](../../../backend/modules/careplan/compose/pickup_excusal.go#L61) | runtime retained | `tenant.WithTenantTx`, `tenant.FromContext`, `fn` |
| `excusalTransaction.LockStudent` | [pickup_excusal.go:64](../../../backend/modules/careplan/compose/pickup_excusal.go#L64) | direct binding | `careplanning.LockStudent` |
| `excusalTransaction.LockStudentAndExceptionDay` | [pickup_excusal.go:67](../../../backend/modules/careplan/compose/pickup_excusal.go#L67) | runtime retained | `careplanning.LockStudentAndExceptionDay`, `errors.Is` |
| `excusalTransaction.IsNotFound` | [pickup_excusal.go:74](../../../backend/modules/careplan/compose/pickup_excusal.go#L74) | runtime retained | `errors.Is`, `errors.Is` |
| `pickupScheduleTransaction.LockStudentAndExceptionDay` | [pickup_schedules.go:51](../../../backend/modules/careplan/compose/pickup_schedules.go#L51) | direct binding | `careplanning.LockStudentAndExceptionDay` |
| `pickupScheduleTransaction.IsNotFound` | [pickup_schedules.go:54](../../../backend/modules/careplan/compose/pickup_schedules.go#L54) | runtime retained | `errors.Is`, `t.excusalTransaction.IsNotFound` |
| `pickupScheduleRecords` | [pickup_schedules.go:58](../../../backend/modules/careplan/compose/pickup_schedules.go#L58) | runtime retained | `application.NewEffectiveRecords[careplan.PickupSchedule, *careplan.PickupSchedule]`, `application.Unlocked` |
| `pickupExceptionRecords` | [pickup_schedules.go:63](../../../backend/modules/careplan/compose/pickup_schedules.go#L63) | direct binding | `application.NewExceptionRecords[careplan.PickupException, *careplan.PickupException]` |
| `pickupNoteRecords` | [pickup_schedules.go:68](../../../backend/modules/careplan/compose/pickup_schedules.go#L68) | runtime retained | `application.NewNoteRecords[careplan.PickupNote, *careplan.PickupNote]`, `application.Unlocked` |
| `parentStudentDirectory.ListStudentsByID` | [repository_adapter_parent_portal.go:69](../../../backend/modules/careplan/compose/repository_adapter_parent_portal.go#L69) | runtime retained | `d.directory.ListParentPortalStudents`, `make`, `len`, `append`, `student.EnrolledFrom.String`, `student.EnrolledUntil.String` |
| `parentGuardianDirectory.ListGuardianLinksByAccount` | [repository_adapter_parent_portal.go:85](../../../backend/modules/careplan/compose/repository_adapter_parent_portal.go#L85) | runtime retained | `d.directory.ListParentPortalGuardianLinks`, `make`, `len`, `append`, `parentRepo.DirectoryGuardianLink` |
| `ParentChildLinks.ListByAccount` | [repository_adapter_parent_portal.go:117](../../../backend/modules/careplan/compose/repository_adapter_parent_portal.go#L117) | runtime retained | `l.children.ListByAccount`, `make`, `len`, `append` |
| `ParentPortalRepositories.WithSchools` | [repository_adapter_parent_portal_schools.go:32](../../../backend/modules/careplan/compose/repository_adapter_parent_portal_schools.go#L32) | runtime retained | `panic` |
| `schoolChildRepository.ListByAccount` | [repository_adapter_parent_portal_schools.go:48](../../../backend/modules/careplan/compose/repository_adapter_parent_portal_schools.go#L48) | runtime retained | `r.ChildRepository.ListByAccount`, `enrichChildrenWithSchools`, `slices.SortStableFunc`, `strings.Compare`, `strings.Compare`, `strings.Compare` |
| `schoolChildRepository.FindForAccount` | [repository_adapter_parent_portal_schools.go:68](../../../backend/modules/careplan/compose/repository_adapter_parent_portal_schools.go#L68) | runtime retained | `r.ChildRepository.FindForAccount`, `enrichChildrenWithSchools` |
| `schoolEnrollablePhaseRepository.ListEnrollable` | [repository_adapter_parent_portal_schools.go:84](../../../backend/modules/careplan/compose/repository_adapter_parent_portal_schools.go#L84) | runtime retained | `r.EnrollablePhaseRepository.ListEnrollable`, `len`, `parentPortalSchoolsByID`, `phaseSchoolIDs`, `fmt.Errorf`, `make`, `len`, `fmt.Errorf`, `append`, `slices.SortStableFunc` |
| `compareEnrollablePhases` | [repository_adapter_parent_portal_schools.go:111](../../../backend/modules/careplan/compose/repository_adapter_parent_portal_schools.go#L111) | runtime retained | `strings.Compare`, `left.ServiceStartDate.Compare` |
| `schoolEnrollmentRequestRepository.ListByAccount` | [repository_adapter_parent_portal_schools.go:129](../../../backend/modules/careplan/compose/repository_adapter_parent_portal_schools.go#L129) | runtime retained | `r.EnrollmentRequestRepository.ListByAccount`, `len`, `make`, `len`, `append`, `parentPortalSchoolsByID`, `fmt.Errorf`, `make`, `len`, `fmt.Errorf`, `append` |
| `enrichChildrenWithSchools` | [repository_adapter_parent_portal_schools.go:158](../../../backend/modules/careplan/compose/repository_adapter_parent_portal_schools.go#L158) | runtime retained | `make`, `len`, `append`, `parentPortalSchoolsByID`, `fmt.Errorf`, `fmt.Errorf` |
| `phaseSchoolIDs` | [repository_adapter_parent_portal_schools.go:178](../../../backend/modules/careplan/compose/repository_adapter_parent_portal_schools.go#L178) | runtime retained | `make`, `len`, `append` |
| `parentPortalSchoolsByID` | [repository_adapter_parent_portal_schools.go:186](../../../backend/modules/careplan/compose/repository_adapter_parent_portal_schools.go#L186) | runtime retained | `make`, `len`, `make`, `len`, `append`, `directory.ListParentPortalSchools`, `make`, `len` |
| `requestConflictRecords.Find` | [request_conflicts.go:26](../../../backend/modules/careplan/compose/request_conflicts.go#L26) | direct binding | `r.FindCareScheduleRequest` |
| `requestCorrectionRecords.Find` | [request_corrections.go:40](../../../backend/modules/careplan/compose/request_corrections.go#L40) | direct binding | `r.FindCareScheduleRequest` |
| `requestCorrectionRecords.Redecide` | [request_corrections.go:43](../../../backend/modules/careplan/compose/request_corrections.go#L43) | direct binding | `r.RedecideCareScheduleRequest` |
| `requestCorrectionRecords.FindException` | [request_corrections.go:46](../../../backend/modules/careplan/compose/request_corrections.go#L46) | runtime retained | `r.FindPickupException` |
| `requestCorrectionRecords.DeleteException` | [request_corrections.go:53](../../../backend/modules/careplan/compose/request_corrections.go#L53) | direct binding | `r.DeletePickupException` |
| `requestCorrectionRecords.FindDepartureException` | [request_corrections.go:56](../../../backend/modules/careplan/compose/request_corrections.go#L56) | runtime retained | `r.ListDepartureExceptions`, `len` |
| `requestDecisionRecords.Find` | [request_decisions.go:59](../../../backend/modules/careplan/compose/request_decisions.go#L59) | direct binding | `r.FindCareScheduleRequest` |
| `requestDecisionRecords.Decide` | [request_decisions.go:62](../../../backend/modules/careplan/compose/request_decisions.go#L62) | direct binding | `r.DecideCareScheduleRequest` |
| `requestDecisionRecords.StoreSnapshot` | [request_decisions.go:65](../../../backend/modules/careplan/compose/request_decisions.go#L65) | runtime retained | `json.Marshal`, `r.UpdateCareScheduleRequestSnapshot` |
| `requestEditRecords.Find` | [request_edits.go:28](../../../backend/modules/careplan/compose/request_edits.go#L28) | direct binding | `r.FindCareScheduleRequest` |
| `requestEditRecords.UpdatePending` | [request_edits.go:31](../../../backend/modules/careplan/compose/request_edits.go#L31) | direct binding | `r.UpdatePendingCareScheduleRequest` |
| `requestReadRecords.ListCareRequests` | [request_reads.go:36](../../../backend/modules/careplan/compose/request_reads.go#L36) | direct binding | `r.ListCareScheduleRequests` |
| `requestSubmissionRecords.Create` | [request_submissions.go:27](../../../backend/modules/careplan/compose/request_submissions.go#L27) | direct binding | `r.CreateCareScheduleRequest` |
| `requestSubmissionRecords.IsPendingConflict` | [request_submissions.go:31](../../../backend/modules/careplan/compose/request_submissions.go#L31) | direct binding | `postgres.IsPendingCareScheduleConflict` |

### `modules/careplan/requestfeed/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `tokenAdapter.New` | [new.go:67](../../../backend/modules/careplan/requestfeed/compose/new.go#L67) | direct binding | `a.newToken` |
| `tokenAdapter.Hash` | [new.go:68](../../../backend/modules/careplan/requestfeed/compose/new.go#L68) | direct binding | `a.hashToken` |

### `modules/classday/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `mappedError.Error` | [classday.go:116](../../../backend/modules/classday/compose/classday.go#L116) | direct binding | `e.err.Error` |
| `mappedError.Unwrap` | [classday.go:117](../../../backend/modules/classday/compose/classday.go#L117) | direct binding | none |
| `mappedError.Is` | [classday.go:118](../../../backend/modules/classday/compose/classday.go#L118) | direct binding | none |
| `mapArrivalExceptionError` | [classday.go:135](../../../backend/modules/classday/compose/classday.go#L135) | runtime retained | `errors.Is` |
| `dayTimeBinding.DayDepartureModes` | [classday.go:151](../../../backend/modules/classday/compose/classday.go#L151) | direct binding | `b.pickups.GetBulkDayDepartureModesForDate` |
| `dayTimeBinding.EffectivePickups` | [classday.go:155](../../../backend/modules/classday/compose/classday.go#L155) | runtime retained | `b.pickups.GetBulkEffectivePickupTimesForDate`, `make`, `len` |
| `dayTimeBinding.EffectiveArrivals` | [classday.go:172](../../../backend/modules/classday/compose/classday.go#L172) | runtime retained | `b.arrivals.GetBulkEffectiveArrivalTimesForDate`, `make`, `len` |
| `classArrivalExceptionBinding.ListClassArrivalExceptions` | [classday.go:194](../../../backend/modules/classday/compose/classday.go#L194) | runtime retained | `b.store.ListClassArrivalExceptions`, `mapArrivalExceptionError`, `make`, `len`, `append`, `classArrivalException` |
| `classArrivalExceptionBinding.UpsertClassArrivalException` | [classday.go:208](../../../backend/modules/classday/compose/classday.go#L208) | runtime retained | `b.store.UpsertClassArrivalException`, `mapArrivalExceptionError`, `classArrivalException` |
| `classArrivalExceptionBinding.DeleteClassArrivalException` | [classday.go:218](../../../backend/modules/classday/compose/classday.go#L218) | runtime retained | `mapArrivalExceptionError`, `b.store.DeleteClassArrivalException` |
| `classArrivalException` | [classday.go:222](../../../backend/modules/classday/compose/classday.go#L222) | direct binding | none |
| `blockStartBinding.EarliestPlannedBlockStartForClass` | [classday.go:233](../../../backend/modules/classday/compose/classday.go#L233) | runtime retained | `b.starts.EarliestPlannedBlockStartForClass`, `mapArrivalExceptionError` |
| `callerBinding.AssignedClasses` | [classday.go:243](../../../backend/modules/classday/compose/classday.go#L243) | direct binding | `b.context.GetMySchoolClasses` |
| `callerBinding.StaffID` | [classday.go:252](../../../backend/modules/classday/compose/classday.go#L252) | runtime retained | `b.context.CurrentStaffID` |
| `careDayBinding.ResolveForDate` | [slotlists.go:115](../../../backend/modules/classday/compose/slotlists.go#L115) | runtime retained | `b.resolver.ResolveForDate`, `make`, `len`, `ports.CareDay` |
| `effectiveTimeBinding.PickupTimes` | [slotlists.go:132](../../../backend/modules/classday/compose/slotlists.go#L132) | runtime retained | `b.pickups.GetBulkEffectivePickupTimesForDate`, `make`, `len` |
| `effectiveTimeBinding.ArrivalTimes` | [slotlists.go:146](../../../backend/modules/classday/compose/slotlists.go#L146) | runtime retained | `b.arrivals.GetBulkEffectiveArrivalTimesForDate`, `make`, `len` |
| `pickupBaselineBinding.RegularPickupTimes` | [slotlists.go:162](../../../backend/modules/classday/compose/slotlists.go#L162) | runtime retained | `b.baselines.Project`, `make`, `len`, `projection.ForDate`, `row.PickupTime.Format` |
| `Rules.RowCareDay` | [slotlists.go:182](../../../backend/modules/classday/compose/slotlists.go#L182) | runtime retained | `ports.CareDay`, `careplan.AttendanceRowCareDay`, `careplan.CareDayStatus` |
| `Rules.EnrolledOn` | [slotlists.go:187](../../../backend/modules/classday/compose/slotlists.go#L187) | direct binding | `r` |

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
| `*parentAnnouncements.ListParentAnnouncements` | [parent_announcements.go:112](../../../backend/modules/communication/composition/parent_announcements.go#L112) | runtime retained | `p.service.List`, `mapParentAnnouncementError`, `make`, `len`, `append`, `mapParentAnnouncement` |
| `*parentAnnouncements.GetParentAnnouncement` | [parent_announcements.go:124](../../../backend/modules/communication/composition/parent_announcements.go#L124) | runtime retained | `p.service.Get`, `mapParentAnnouncementError`, `mapParentAnnouncement`, `row.IsDeclaration`, `p.service.DeclarationFrozen` |
| `*parentAnnouncements.CreateParentAnnouncement` | [parent_announcements.go:140](../../../backend/modules/communication/composition/parent_announcements.go#L140) | runtime retained | `p.service.Create`, `mapParentAnnouncementInput`, `mapParentAnnouncement`, `mapParentAnnouncementError` |
| `*parentAnnouncements.UpdateParentAnnouncement` | [parent_announcements.go:145](../../../backend/modules/communication/composition/parent_announcements.go#L145) | runtime retained | `p.service.Update`, `mapParentAnnouncementInput`, `mapParentAnnouncement`, `mapParentAnnouncementError` |
| `*parentAnnouncements.DeleteParentAnnouncement` | [parent_announcements.go:150](../../../backend/modules/communication/composition/parent_announcements.go#L150) | runtime retained | `mapParentAnnouncementError`, `p.service.Delete` |
| `*parentAnnouncements.PublishParentAnnouncement` | [parent_announcements.go:154](../../../backend/modules/communication/composition/parent_announcements.go#L154) | runtime retained | `p.service.Publish`, `mapParentAnnouncement`, `mapParentAnnouncementError` |
| `*parentAnnouncements.UnpublishParentAnnouncement` | [parent_announcements.go:159](../../../backend/modules/communication/composition/parent_announcements.go#L159) | runtime retained | `p.service.Unpublish`, `mapParentAnnouncement`, `mapParentAnnouncementError` |
| `*parentAnnouncements.ParentAnnouncementStats` | [parent_announcements.go:164](../../../backend/modules/communication/composition/parent_announcements.go#L164) | runtime retained | `p.service.Stats`, `mapParentAnnouncementError`, `mapParentAnnouncementError` |
| `*parentAnnouncements.ParentAnnouncementRecipients` | [parent_announcements.go:174](../../../backend/modules/communication/composition/parent_announcements.go#L174) | runtime retained | `p.service.Recipients`, `mapParentAnnouncementError`, `make`, `len`, `append` |
| `*parentAnnouncements.ParentAnnouncementPollResults` | [parent_announcements.go:189](../../../backend/modules/communication/composition/parent_announcements.go#L189) | runtime retained | `p.service.PollResults`, `mapParentAnnouncementError`, `make`, `len`, `append`, `mapParentAnnouncementError` |
| `*parentAnnouncements.ParentAnnouncementPollChildren` | [parent_announcements.go:206](../../../backend/modules/communication/composition/parent_announcements.go#L206) | runtime retained | `p.service.PollChildren`, `mapParentAnnouncementError`, `make`, `len`, `append` |
| `*parentAnnouncements.RemindParentAnnouncement` | [parent_announcements.go:222](../../../backend/modules/communication/composition/parent_announcements.go#L222) | runtime retained | `p.service.RemindOutstanding`, `mapParentAnnouncementError` |
| `*parentAnnouncements.ParentAnnouncementLetterStatus` | [parent_announcements.go:227](../../../backend/modules/communication/composition/parent_announcements.go#L227) | runtime retained | `p.service.LetterStatus`, `mapParentAnnouncementError`, `make`, `len`, `make`, `len`, `append`, `append`, `mapParentAnnouncementError` |
| `*parentAnnouncements.ResendParentAnnouncementEmails` | [parent_announcements.go:261](../../../backend/modules/communication/composition/parent_announcements.go#L261) | runtime retained | `p.service.ResendFailedEmails`, `mapParentAnnouncementError` |
| `*parentAnnouncements.UpdateParentAnnouncementReminder` | [parent_announcements.go:266](../../../backend/modules/communication/composition/parent_announcements.go#L266) | runtime retained | `p.service.UpdateReminder`, `mapParentAnnouncement`, `mapParentAnnouncementError` |
| `*parentAnnouncements.SendDueParentAnnouncementReminders` | [parent_announcements.go:271](../../../backend/modules/communication/composition/parent_announcements.go#L271) | runtime retained | `p.service.SendDueReminders`, `mapParentAnnouncementError` |
| `*parentAnnouncements.PublishCareCancellation` | [parent_announcements.go:276](../../../backend/modules/communication/composition/parent_announcements.go#L276) | runtime retained | `p.service.PublishCareCancellation`, `mapParentAnnouncementError`, `mapParentAnnouncementError` |
| `*parentAnnouncements.CareCancellationReachFor` | [parent_announcements.go:288](../../../backend/modules/communication/composition/parent_announcements.go#L288) | runtime retained | `p.service.CareCancellationReachFor`, `mapParentAnnouncementError`, `mapParentAnnouncementError` |
| `*parentAnnouncements.AnnouncementExists` | [parent_announcements.go:296](../../../backend/modules/communication/composition/parent_announcements.go#L296) | runtime retained | `p.service.AnnouncementExists`, `mapParentAnnouncementError` |
| `*parentAnnouncements.AnnouncementEditable` | [parent_announcements.go:301](../../../backend/modules/communication/composition/parent_announcements.go#L301) | runtime retained | `p.service.AnnouncementEditable`, `mapParentAnnouncementError` |
| `*parentAnnouncements.LockAnnouncementForAttachmentChange` | [parent_announcements.go:306](../../../backend/modules/communication/composition/parent_announcements.go#L306) | runtime retained | `p.service.LockAnnouncementForAttachmentChange`, `mapParentAnnouncementError` |
| `*parentAnnouncements.ResetAnnouncementEngagement` | [parent_announcements.go:311](../../../backend/modules/communication/composition/parent_announcements.go#L311) | runtime retained | `mapParentAnnouncementError`, `p.service.ResetAnnouncementEngagement` |
| `mapParentAnnouncementInput` | [parent_announcements.go:315](../../../backend/modules/communication/composition/parent_announcements.go#L315) | runtime retained | `make`, `len`, `append`, `staff.AnnouncementDeclarationSettings` |
| `mapParentAnnouncement` | [parent_announcements.go:331](../../../backend/modules/communication/composition/parent_announcements.go#L331) | runtime retained | `make`, `len`, `append`, `make`, `len`, `append`, `communication.ParentDeclarationSettings` |
| `*mappedParentAnnouncementError.Error` | [parent_announcements.go:364](../../../backend/modules/communication/composition/parent_announcements.go#L364) | direct binding | `e.original.Error` |
| `*mappedParentAnnouncementError.Unwrap` | [parent_announcements.go:365](../../../backend/modules/communication/composition/parent_announcements.go#L365) | direct binding | none |
| `mapParentAnnouncementError` | [parent_announcements.go:367](../../../backend/modules/communication/composition/parent_announcements.go#L367) | runtime retained | `errors.Is` |
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
| `schoolRolePolicy.Validate` | [school_role.go:15](../../../backend/modules/dataimport/compose/school_role.go#L15) | runtime retained | `identityaccess.ValidateAssignableSchoolRole`, `assignmentFacts` |
| `schoolRolePolicy.NeedsCaregiver` | [school_role.go:19](../../../backend/modules/dataimport/compose/school_role.go#L19) | runtime retained | `identityaccess.RoleNeedsCaregiverProfile`, `assignmentFacts` |
| `assignmentFacts` | [school_role.go:25](../../../backend/modules/dataimport/compose/school_role.go#L25) | runtime retained | none |
| `transactions.TenantID` | [transactions.go:15](../../../backend/modules/dataimport/compose/transactions.go#L15) | direct binding | `tenant.FromContext` |
| `transactions.HasTransaction` | [transactions.go:17](../../../backend/modules/dataimport/compose/transactions.go#L17) | runtime retained | `tenant.TransactionFromContext` |
| `transactions.WithinCurrent` | [transactions.go:22](../../../backend/modules/dataimport/compose/transactions.go#L22) | direct binding | `tenant.WithinCurrentTenant` |
| `transactions.WithinRetry` | [transactions.go:26](../../../backend/modules/dataimport/compose/transactions.go#L26) | runtime retained | `tenant.TenantFromContext`, `tenant.WithinTenantRetry` |
| `transactions.Savepoint` | [transactions.go:34](../../../backend/modules/dataimport/compose/transactions.go#L34) | direct binding | `tenant.WithSavepoint` |
| `transactions.AcquireLock` | [transactions.go:38](../../../backend/modules/dataimport/compose/transactions.go#L38) | direct binding | `tenant.AcquireLock` |
| `transactions.Observe` | [transactions.go:42](../../../backend/modules/dataimport/compose/transactions.go#L42) | runtime retained | `tenant.WithAdditionalUnitOfWorkObserver`, `observe` |

### `modules/dataimport/compose/imports`

No non-constructor functions.


### `modules/delivery/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `DrainEmailForTest` | [intent_worker_test_helpers.go:11](../../../backend/modules/delivery/compose/intent_worker_test_helpers.go#L11) | test-only | `application.DrainEmailForTest` |
| `DeleteSystemEmailForTest` | [intent_worker_test_helpers.go:14](../../../backend/modules/delivery/compose/intent_worker_test_helpers.go#L14) | test-only | `application.DeleteSystemEmailForTest` |
| `IsLoggingMailTransport` | [mail_transport.go:14](../../../backend/modules/delivery/compose/mail_transport.go#L14) | runtime retained | none |
| `*Runtime.EmailStatus` | [new.go:71](../../../backend/modules/delivery/compose/new.go#L71) | runtime retained | `errors.New`, `errors.New`, `r.store.EmailStatus`, `deliveryContract.State` |
| `guardianDirectoryAdapter.Resolve` | [new.go:87](../../../backend/modules/delivery/compose/new.go#L87) | runtime retained | `a.resolver.ResolveGuardianDisplays`, `make`, `len`, `domain.GuardianDisplay` |
| `moduleEngine.EnqueueEmail` | [new.go:104](../../../backend/modules/delivery/compose/new.go#L104) | runtime retained | `json.Marshal`, `fmt.Errorf`, `e.enqueue`, `domain.RelatedEntity` |
| `moduleEngine.EnqueueDeferredPush` | [new.go:116](../../../backend/modules/delivery/compose/new.go#L116) | runtime retained | `e.enqueue`, `domain.RelatedEntity`, `json.RawMessage` |
| `moduleEngine.EnqueuePush` | [new.go:136](../../../backend/modules/delivery/compose/new.go#L136) | runtime retained | `json.Marshal`, `fmt.Errorf`, `json.Marshal`, `fmt.Errorf`, `e.enqueue`, `domain.RelatedEntity` |
| `moduleEngine.enqueue` | [new.go:152](../../../backend/modules/delivery/compose/new.go#L152) | runtime retained | `e.service.Enqueue`, `errors.Is` |
| `moduleEngine.Cancel` | [new.go:163](../../../backend/modules/delivery/compose/new.go#L163) | runtime retained | `e.service.Cancel`, `domain.Transport`, `domain.RelatedEntity` |
| `moduleEngine.Statuses` | [new.go:167](../../../backend/modules/delivery/compose/new.go#L167) | runtime retained | `e.service.Statuses`, `domain.Transport`, `domain.RelatedEntity`, `make`, `len`, `len`, `json.Unmarshal`, `fmt.Errorf`, `append`, `deliveryContract.State` |
| `moduleEngine.ReplaceEmailDeliveries` | [new.go:190](../../../backend/modules/delivery/compose/new.go#L190) | runtime retained | `make`, `len`, `append`, `e.store.ReplaceEmailDeliveries` |
| `moduleEngine.DeleteEmailDeliveries` | [new.go:201](../../../backend/modules/delivery/compose/new.go#L201) | direct binding | `e.store.DeleteEmailDeliveries` |
| `moduleEngine.EmailDeliveryStatuses` | [new.go:205](../../../backend/modules/delivery/compose/new.go#L205) | runtime retained | `e.service.EmailDeliveryStatuses`, `domain.RelatedEntity`, `make`, `len`, `append` |
| `moduleEngine.AttachEmailOutbox` | [new.go:222](../../../backend/modules/delivery/compose/new.go#L222) | direct binding | `e.store.AttachEmailOutbox` |
| `moduleEngine.ClaimFailedEmailDelivery` | [new.go:226](../../../backend/modules/delivery/compose/new.go#L226) | direct binding | `e.store.ClaimFailedEmailDelivery` |
| `workerEngine.Backlog` | [new.go:235](../../../backend/modules/delivery/compose/new.go#L235) | direct binding | `e.store.Backlog` |
| `workerEngine.RunOnce` | [new.go:237](../../../backend/modules/delivery/compose/new.go#L237) | runtime retained | `e.Worker.RunOnce`, `deliveryContract.WorkerStats` |
| `providerAdapter.Send` | [new.go:244](../../../backend/modules/delivery/compose/new.go#L244) | runtime retained | `deliveryContract.Transport`, `json.Unmarshal`, `fmt.Errorf`, `p.provider.SendEmail`, `providerResult`, `json.Unmarshal`, `fmt.Errorf`, `json.Unmarshal`, `fmt.Errorf`, `p.provider.SendPush`, `providerResult`, `fmt.Errorf` |
| `providerResult` | [new.go:279](../../../backend/modules/delivery/compose/new.go#L279) | runtime retained | `errors.Is`, `fmt.Errorf`, `errors.Is`, `fmt.Errorf`, `domain.ProviderResult` |
| `containsPushSubscription` | [provider.go:63](../../../backend/modules/delivery/compose/provider.go#L63) | runtime retained | `subscription.UpdatedAt.Equal` |
| `DurableEmails.EnqueueEmail` | [provider.go:104](../../../backend/modules/delivery/compose/provider.go#L104) | runtime retained | `a.module.EnqueueEmail` |
| `DurableEmails.CancelEmail` | [provider.go:119](../../../backend/modules/delivery/compose/provider.go#L119) | direct binding | `a.module.Cancel` |
| `*MailPushProvider.SendEmail` | [provider.go:125](../../../backend/modules/delivery/compose/provider.go#L125) | runtime retained | `json.Unmarshal`, `fmt.Errorf`, `int64`, `p.Registry.Lookup`, `transport.Send`, `email.NewEmail`, `errors.Is`, `fmt.Errorf`, `errors.Is`, `fmt.Errorf` |
| `*MailPushProvider.SendPush` | [provider.go:162](../../../backend/modules/delivery/compose/provider.go#L162) | runtime retained | `errors.New`, `p.DeferredPush`, `errors.New`, `p.PushAuthorized`, `fmt.Errorf`, `p.Push.SendPush` |
| `QueueStaffTimeTrackingChanged` | [realtime.go:22](../../../backend/modules/delivery/compose/realtime.go#L22) | direct binding | `realtimeevents.QueueStaffTimeTrackingChanged` |
| `QueueInstanceStarted` | [realtime.go:32](../../../backend/modules/delivery/compose/realtime.go#L32) | direct binding | `realtimeevents.QueueInstanceStarted` |
| `QueueArrivalScheduleChanged` | [realtime.go:39](../../../backend/modules/delivery/compose/realtime.go#L39) | direct binding | `realtimeevents.QueueArrivalScheduleChanged` |
| `DurablePushOutbox.CancelPendingByRelatedEntity` | [tenant_transactions.go:33](../../../backend/modules/delivery/compose/tenant_transactions.go#L33) | runtime retained | `tenant.TenantFromContext`, `fmt.Errorf`, `a.module.Cancel`, `tenantID.Int64` |
| `DurablePushOutbox.EnqueuePush` | [tenant_transactions.go:42](../../../backend/modules/delivery/compose/tenant_transactions.go#L42) | runtime retained | `a.module.EnqueuePush`, `string` |

### `modules/devicefleet/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `activeTerminalCounts.CountActiveTerminalsByTenant` | [active_terminal_counts.go:23](../../../backend/modules/devicefleet/compose/active_terminal_counts.go#L23) | runtime retained | `tenant.TransactionFromContext`, `errors.New`, `tenant.IsAdminTx`, `errors.New`, `postgres.CountActiveTerminalsByTenant`, `postgres.CountActiveTerminalsByTenant`, `fmt.Errorf` |
| `databaseRuntime` | [new.go:131](../../../backend/modules/devicefleet/compose/new.go#L131) | runtime retained | `tenant.TenantFromContext`, `tenant.TransactionFromContext`, `tenant.IsAdminTx`, `errors.Join`, `postgres.ResolveDatabase`, `id.Int64`, `tenant.IsAdminTx` |
| `transaction.RunWrite` | [new.go:145](../../../backend/modules/devicefleet/compose/new.go#L145) | runtime retained | `tenant.TransactionFromContext`, `callback`, `tenant.TenantFromContext`, `errors.Join`, `tenant.WithinTenant` |
| `transaction.AdminScope` | [new.go:156](../../../backend/modules/devicefleet/compose/new.go#L156) | runtime retained | `tenant.TransactionFromContext`, `callback`, `tenant.WithAdminTx`, `callback` |
| `transaction.TenantScope` | [new.go:165](../../../backend/modules/devicefleet/compose/new.go#L165) | runtime retained | `tenant.WithTenantTx`, `callback` |
| `roomDirectory.ListRoomsByID` | [new.go:173](../../../backend/modules/devicefleet/compose/new.go#L173) | runtime retained | `d.rooms.ListRoomsByID`, `toDomainRooms` |
| `roomDirectory.ListRooms` | [new.go:181](../../../backend/modules/devicefleet/compose/new.go#L181) | runtime retained | `d.rooms.ListRooms`, `toDomainRooms` |
| `toDomainRooms` | [new.go:189](../../../backend/modules/devicefleet/compose/new.go#L189) | runtime retained | `make`, `len`, `append` |
| `uncomposedDashboard.ListActiveSessions` | [uncomposed.go:18](../../../backend/modules/devicefleet/compose/uncomposed.go#L18) | direct binding | none |
| `uncomposedDashboard.ListActivityTemplates` | [uncomposed.go:22](../../../backend/modules/devicefleet/compose/uncomposed.go#L22) | direct binding | none |
| `uncomposedDashboard.ListPlannedActivities` | [uncomposed.go:26](../../../backend/modules/devicefleet/compose/uncomposed.go#L26) | direct binding | none |
| `uncomposedDashboard.ListPickupTimes` | [uncomposed.go:30](../../../backend/modules/devicefleet/compose/uncomposed.go#L30) | direct binding | none |
| `uncomposedDashboard.ListOpenVisits` | [uncomposed.go:34](../../../backend/modules/devicefleet/compose/uncomposed.go#L34) | direct binding | none |
| `uncomposedDashboard.ListPresentStudents` | [uncomposed.go:38](../../../backend/modules/devicefleet/compose/uncomposed.go#L38) | direct binding | none |
| `uncomposedDashboard.School` | [uncomposed.go:42](../../../backend/modules/devicefleet/compose/uncomposed.go#L42) | direct binding | none |
| `uncomposedDashboard.DisplayEnabled` | [uncomposed.go:46](../../../backend/modules/devicefleet/compose/uncomposed.go#L46) | direct binding | none |

### `modules/devicefleet/compose/repositoryadapter`

No non-constructor functions.


### `modules/devicescan/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `AttendanceTenantID` | [attendance_principal.go:11](../../../backend/modules/devicescan/compose/attendance_principal.go#L11) | direct binding | `tenant.FromContext` |
| `MarkAttendanceRollback` | [attendance_principal.go:15](../../../backend/modules/devicescan/compose/attendance_principal.go#L15) | direct binding | `tenant.MarkRollback` |
| `groupDirectory.Find` | [catalog.go:14](../../../backend/modules/devicescan/compose/catalog.go#L14) | runtime retained | `g.education.GetGroup` |
| `pickupPlan.Effective` | [catalog.go:29](../../../backend/modules/devicescan/compose/catalog.go#L29) | runtime retained | `p.pickups.GetEffectivePickupTimeForDate`, `append` |
| `clock.Now` | [compose.go:142](../../../backend/modules/devicescan/compose/compose.go#L142) | direct binding | `c.now` |
| `clock.Day` | [compose.go:143](../../../backend/modules/devicescan/compose/compose.go#L143) | direct binding | `calendar.DateFromTime` |
| `unitOfWork.RequireTransaction` | [compose.go:148](../../../backend/modules/devicescan/compose/compose.go#L148) | runtime retained | `tenant.TransactionFromContext`, `errors.New`, `tenant.TenantFromContext` |
| `unitOfWork.MarkRollback` | [compose.go:156](../../../backend/modules/devicescan/compose/compose.go#L156) | direct binding | `tenant.MarkRollback` |
| `mappedError.Error` | [compose.go:165](../../../backend/modules/devicescan/compose/compose.go#L165) | direct binding | `e.err.Error` |
| `mappedError.Unwrap` | [compose.go:166](../../../backend/modules/devicescan/compose/compose.go#L166) | direct binding | none |
| `mappedError.Is` | [compose.go:167](../../../backend/modules/devicescan/compose/compose.go#L167) | direct binding | none |
| `mapError` | [compose.go:169](../../../backend/modules/devicescan/compose/compose.go#L169) | direct binding | none |
| `directoryPeople.Teachers` | [directory.go:33](../../../backend/modules/devicescan/compose/directory.go#L33) | runtime retained | `d.ListTeachersWithStaffAndPerson`, `make`, `len`, `append` |
| `feedbackStudents.LockFeedbackStudent` | [feedback_students.go:22](../../../backend/modules/devicescan/compose/feedback_students.go#L22) | runtime retained | `r.users.GetStudentByIDForUpdate`, `errors.Is` |
| `openRoomMover.MoveToOpenRoom` | [open_room.go:56](../../../backend/modules/devicescan/compose/open_room.go#L56) | runtime retained | `m.move`, `m.translate`, `slices.Contains` |
| `openRoomMover.translate` | [open_room.go:76](../../../backend/modules/devicescan/compose/open_room.go#L76) | runtime retained | `errors.AsType[*studentpresence.RoomCapacityError]`, `errors.Is`, `fmt.Errorf`, `errors.Is`, `fmt.Errorf`, `errors.Is`, `fmt.Errorf` |
| `people.FindPersonByTag` | [people.go:65](../../../backend/modules/devicescan/compose/people.go#L65) | runtime retained | `p.FindByTagID`, `p.IsPersonNotFound`, `mapError` |
| `people.FindStudentByPerson` | [people.go:79](../../../backend/modules/devicescan/compose/people.go#L79) | runtime retained | `p.GetStudentByPersonID`, `errors.Is` |
| `people.FindStaffByPerson` | [people.go:97](../../../backend/modules/devicescan/compose/people.go#L97) | runtime retained | `p.GetStaffByPersonID`, `errors.Is` |
| `classifyPresence` | [presence.go:18](../../../backend/modules/devicescan/compose/presence.go#L18) | runtime retained | `errors.Is`, `mapError`, `errors.Is`, `mapError`, `mapError`, `errors.Is`, `errors.Is`, `mapError`, `errors.Is`, `errors.Is`, `errors.Is`, `errors.Is`, `errors.Is`, `errors.Is`, `errors.Is`, `mapError`, `errors.Is`, `errors.Is`, `errors.Is`, `errors.Is`, `errors.Is`, `errors.Is`, `errors.Is`, `mapError`, `errors.Is`, `errors.Is`, `errors.Is`, `errors.Is`, `errors.Is`, `errors.Is`, `errors.Is`, `errors.Is`, `mapError` |
| `visits.Current` | [presence.go:62](../../../backend/modules/devicescan/compose/presence.go#L62) | runtime retained | `v.active.GetStudentCurrentVisitWithRoom`, `errors.Is` |
| `visits.End` | [presence.go:83](../../../backend/modules/devicescan/compose/presence.go#L83) | direct binding | `v.active.EndVisit` |
| `visits.Record` | [presence.go:85](../../../backend/modules/devicescan/compose/presence.go#L85) | runtime retained | `v.clock.Now`, `v.active.CreateVisit`, `errors.As`, `errors.Is`, `mapError`, `errors.Is`, `errors.Is`, `mapError` |
| `sessionFromGroup` | [presence.go:119](../../../backend/modules/devicescan/compose/presence.go#L119) | runtime retained | none |
| `sessions.ListOpenInRoom` | [presence.go:138](../../../backend/modules/devicescan/compose/presence.go#L138) | runtime retained | `s.presence.QueryLiveGroups`, `fmt.Errorf`, `make`, `len`, `append` |
| `sessions.FindDeviceSessionInRoom` | [presence.go:153](../../../backend/modules/devicescan/compose/presence.go#L153) | runtime retained | `s.active.FindDeviceActiveGroupInRoom`, `sessionFromGroup` |
| `sessions.Current` | [presence.go:165](../../../backend/modules/devicescan/compose/presence.go#L165) | runtime retained | `s.active.GetDeviceCurrentSession`, `errors.Is`, `sessionFromGroup` |
| `sessions.Start` | [presence.go:180](../../../backend/modules/devicescan/compose/presence.go#L180) | runtime retained | `s.clock.Now`, `s.active.CreateActiveGroup` |
| `sessions.EnsureRoomSession` | [presence.go:193](../../../backend/modules/devicescan/compose/presence.go#L193) | runtime retained | `s.active.EnsureOpenRoomSession`, `sessionFromGroup` |
| `sessions.Delete` | [presence.go:201](../../../backend/modules/devicescan/compose/presence.go#L201) | direct binding | `s.active.DeleteActiveGroup` |
| `sessions.End` | [presence.go:205](../../../backend/modules/devicescan/compose/presence.go#L205) | runtime retained | `s.active.EndActiveGroupSession`, `errors.Is` |
| `sessions.Touch` | [presence.go:213](../../../backend/modules/devicescan/compose/presence.go#L213) | direct binding | `s.active.UpdateSessionActivity` |
| `sessions.Supervisors` | [presence.go:217](../../../backend/modules/devicescan/compose/presence.go#L217) | runtime retained | `s.presence.QueryLiveGroups`, `len`, `s.clock.Day`, `s.clock.Now`, `today.String`, `s.presence.QueryGroupSupervisions`, `make`, `len`, `append`, `supervisionEnded`, `today.String` |
| `sessions.ReplaceSupervisors` | [presence.go:236](../../../backend/modules/devicescan/compose/presence.go#L236) | runtime retained | `s.active.UpdateActiveGroupSupervisors` |
| `attendance.Status` | [presence.go:245](../../../backend/modules/devicescan/compose/presence.go#L245) | runtime retained | `a.active.GetStudentAttendanceStatus` |
| `attendance.Toggle` | [presence.go:256](../../../backend/modules/devicescan/compose/presence.go#L256) | runtime retained | `a.active.ToggleStudentAttendance`, `classifyPresence` |
| `attendance.ConfirmDailyCheckout` | [presence.go:264](../../../backend/modules/devicescan/compose/presence.go#L264) | runtime retained | `a.active.ConfirmDailyCheckout`, `errors.Is`, `mapError`, `classifyPresence` |
| `supervisionQuery.ActiveSupervisions` | [session_lifecycle.go:48](../../../backend/modules/devicescan/compose/session_lifecycle.go#L48) | runtime retained | `calendar.TodayDate`, `today.String`, `q.source.QueryGroupSupervisions`, `make`, `len`, `append`, `supervisionEnded`, `today.String` |
| `lifecycleSession` | [session_lifecycle.go:67](../../../backend/modules/devicescan/compose/session_lifecycle.go#L67) | runtime retained | `lifecycleSupervisors` |
| `lifecycleSupervisors` | [session_lifecycle.go:81](../../../backend/modules/devicescan/compose/session_lifecycle.go#L81) | runtime retained | `calendar.TodayDate().String`, `calendar.TodayDate`, `make`, `len`, `append`, `supervisionEnded` |
| `supervisionEnded` | [session_lifecycle.go:92](../../../backend/modules/devicescan/compose/session_lifecycle.go#L92) | runtime retained | `calendar.ParseDate`, `calendar.ParseDate`, `day.Before` |
| `lifecycleStore.Start` | [session_lifecycle.go:107](../../../backend/modules/devicescan/compose/session_lifecycle.go#L107) | runtime retained | `s.active.ForceStartActivitySessionWithSupervisors`, `s.active.StartActivitySessionWithSupervisors`, `errors.Is`, `errors.Is`, `classifyPresence`, `lifecycleSession`, `classifyPresence` |
| `lifecycleStore.Current` | [session_lifecycle.go:121](../../../backend/modules/devicescan/compose/session_lifecycle.go#L121) | runtime retained | `s.active.GetDeviceCurrentSession`, `classifyPresence`, `errors.Is`, `mapError`, `lifecycleSession` |
| `lifecycleStore.Conflict` | [session_lifecycle.go:137](../../../backend/modules/devicescan/compose/session_lifecycle.go#L137) | runtime retained | `s.active.CheckActivityConflict`, `classifyPresence`, `strconv.ParseInt` |
| `lifecycleStore.Supervisors` | [session_lifecycle.go:151](../../../backend/modules/devicescan/compose/session_lifecycle.go#L151) | runtime retained | `s.presence.ActiveSupervisions`, `make`, `len`, `append` |
| `lifecycleStore.ReplaceSupervisors` | [session_lifecycle.go:163](../../../backend/modules/devicescan/compose/session_lifecycle.go#L163) | runtime retained | `s.active.UpdateActiveGroupSupervisors`, `lifecycleSession`, `classifyPresence` |
| `lifecycleStore.CountStudents` | [session_lifecycle.go:168](../../../backend/modules/devicescan/compose/session_lifecycle.go#L168) | direct binding | `s.active.CountActiveVisitsByActiveGroupID` |
| `lifecycleStore.Touch` | [session_lifecycle.go:172](../../../backend/modules/devicescan/compose/session_lifecycle.go#L172) | runtime retained | `classifyPresence`, `s.active.UpdateSessionActivity` |
| `lifecycleStore.Timeout` | [session_lifecycle.go:176](../../../backend/modules/devicescan/compose/session_lifecycle.go#L176) | runtime retained | `s.active.ProcessSessionTimeout`, `classifyPresence`, `errors.Is`, `mapError` |
| `lifecycleStore.ValidateTimeout` | [session_lifecycle.go:188](../../../backend/modules/devicescan/compose/session_lifecycle.go#L188) | runtime retained | `classifyPresence`, `s.active.ValidateSessionTimeout` |
| `lifecycleStore.TimeoutInfo` | [session_lifecycle.go:192](../../../backend/modules/devicescan/compose/session_lifecycle.go#L192) | runtime retained | `s.active.GetSessionTimeoutInfo`, `classifyPresence`, `int`, `info.InactivityDuration.Seconds`, `int`, `info.TimeUntilTimeout.Seconds` |
| `lifecyclePeople.StaffNames` | [session_lifecycle.go:202](../../../backend/modules/devicescan/compose/session_lifecycle.go#L202) | runtime retained | `p.people.GetStaffWithPersonByIDs`, `make`, `len` |
| `settings.PresenceMode` | [settings.go:32](../../../backend/modules/devicescan/compose/settings.go#L32) | direct binding | `s.active.GetPresenceMode` |
| `settings.FeedbackEnabled` | [settings.go:36](../../../backend/modules/devicescan/compose/settings.go#L36) | runtime retained | `s.settings.FeedbackEnabled` |
| `settings.CapacityDetailsDisclosed` | [settings.go:43](../../../backend/modules/devicescan/compose/settings.go#L43) | runtime retained | `s.settings.ActivityCapacityDetailsDisclosed`, `s.settings.RoomCapacityDetailsDisclosed` |
| `settings.DailyCheckoutTime` | [settings.go:54](../../../backend/modules/devicescan/compose/settings.go#L54) | runtime retained | `s.settings.DailyCheckoutTime` |
| `settings.PerStudentCheckoutEnabled` | [settings.go:61](../../../backend/modules/devicescan/compose/settings.go#L61) | runtime retained | `s.settings.PerStudentCheckoutEnabled` |
| `settings.PerStudentCheckoutDeltaMinutes` | [settings.go:68](../../../backend/modules/devicescan/compose/settings.go#L68) | runtime retained | `s.settings.PerStudentCheckoutDeltaMinutes` |
| `settings.DailyCheckoutFromAllRoomsEnabled` | [settings.go:75](../../../backend/modules/devicescan/compose/settings.go#L75) | runtime retained | `s.settings.DailyCheckoutFromAllRoomsEnabled` |
| `tagPeople.FindStaff` | [tag_assignment.go:26](../../../backend/modules/devicescan/compose/tag_assignment.go#L26) | runtime retained | `p.GetStaffByID` |
| `tagPeople.FindPerson` | [tag_assignment.go:34](../../../backend/modules/devicescan/compose/tag_assignment.go#L34) | runtime retained | `p.Get` |
| `tagPeople.LinkTag` | [tag_assignment.go:42](../../../backend/modules/devicescan/compose/tag_assignment.go#L42) | direct binding | `p.LinkToRFIDCard` |
| `tagPeople.UnlinkTag` | [tag_assignment.go:46](../../../backend/modules/devicescan/compose/tag_assignment.go#L46) | direct binding | `p.UnlinkFromRFIDCard` |
| `tagPeople.TeacherRole` | [tag_assignment.go:52](../../../backend/modules/devicescan/compose/tag_assignment.go#L52) | runtime retained | `p.GetTeacherByStaffID` |

### `modules/emergencysnapshot/compose`

No non-constructor functions.


### `modules/enrollment/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `tenantRuntime` | [application.go:49](../../../backend/modules/enrollment/compose/application.go#L49) | runtime retained | `tenant.NewTransactionRunner`, `errors.Is`, `tenant.TransactionFromContext`, `errors.Is`, `tenant.WithAdminTx`, `(*bun.DB)`, `fn`, `tenant.WithTenantTx`, `(*bun.DB)`, `fn`, `tenant.ContextWithoutAfterCommitHooks`, `tenant.ContextWithoutTransaction` |
| `*CareOfferingValues.List` | [care_offering_values.go:24](../../../backend/modules/enrollment/compose/care_offering_values.go#L24) | runtime retained | `v.rows.List`, `application.PublicCareOfferingList` |
| `*CareOfferingValues.ListByPhase` | [care_offering_values.go:33](../../../backend/modules/enrollment/compose/care_offering_values.go#L33) | runtime retained | `v.rows.ListByPhase`, `application.PublicCareOfferingList` |
| `*CareOfferingValues.GetByID` | [care_offering_values.go:42](../../../backend/modules/enrollment/compose/care_offering_values.go#L42) | runtime retained | `v.rows.GetByID`, `application.PublicCareOffering` |
| `*CareOfferingValues.Create` | [care_offering_values.go:51](../../../backend/modules/enrollment/compose/care_offering_values.go#L51) | runtime retained | `application.CareOfferingRowOf`, `v.rows.Create`, `application.PublicCareOffering` |
| `*CareOfferingValues.Update` | [care_offering_values.go:64](../../../backend/modules/enrollment/compose/care_offering_values.go#L64) | runtime retained | `application.CareOfferingRowOf`, `v.rows.Update`, `application.PublicCareOffering` |
| `*CareOfferingValues.Delete` | [care_offering_values.go:77](../../../backend/modules/enrollment/compose/care_offering_values.go#L77) | direct binding | `v.rows.Delete` |
| `*CareOfferingValues.Clone` | [care_offering_values.go:82](../../../backend/modules/enrollment/compose/care_offering_values.go#L82) | runtime retained | `v.rows.Clone`, `application.PublicCareOffering` |
| `*CareOfferingValues.ListBookingStats` | [care_offering_values.go:91](../../../backend/modules/enrollment/compose/care_offering_values.go#L91) | direct binding | `v.rows.ListBookingStats` |
| `withService` | [deletion_module.go:73](../../../backend/modules/enrollment/compose/deletion_module.go#L73) | runtime retained | `logger.With` |
| `PublicDecisions` | [intake.go:75](../../../backend/modules/enrollment/compose/intake.go#L75) | direct binding | `application.PublicDecisions` |
| `observeOfferingStorage` | [offering_storage_metrics.go:30](../../../backend/modules/enrollment/compose/offering_storage_metrics.go#L30) | runtime retained | `offeringStorageMetricsOnce.Do`, `prometheus.MustRegister`, `errors.Is`, `errors.As`, `databaseError.Field`, `offeringStorageOperations.WithLabelValues(event.Operation, event.Kind, outcome, code).Inc`, `offeringStorageOperations.WithLabelValues`, `offeringStorageDuration.WithLabelValues(event.Operation, event.Kind).Observe`, `offeringStorageDuration.WithLabelValues`, `event.Duration.Seconds`, `offeringStorageRows.WithLabelValues(event.Operation, "input").Observe`, `offeringStorageRows.WithLabelValues`, `float64`, `offeringStorageRows.WithLabelValues(event.Operation, "output").Observe`, `offeringStorageRows.WithLabelValues`, `float64` |

### `modules/exporttransfer/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `Settings.complete` | [new.go:33](../../../backend/modules/exporttransfer/compose/new.go#L33) | direct binding | none |
| `database` | [new.go:92](../../../backend/modules/exporttransfer/compose/new.go#L92) | runtime retained | `tenant.TransactionFromContext`, `errors.New`, `fmt.Errorf`, `tenant.TenantFromContext`, `tenantID.Int64` |
| `uploader.Prepare` | [new.go:116](../../../backend/modules/exporttransfer/compose/new.go#L116) | runtime retained | `u.client.Prepare` |
| `engine.Status` | [new.go:136](../../../backend/modules/exporttransfer/compose/new.go#L136) | runtime retained | `e.service.State`, `state.Ready` |
| `engine.Transfer` | [new.go:151](../../../backend/modules/exporttransfer/compose/new.go#L151) | runtime retained | `e.service.Transfer` |
| `journalRetention.DeleteExpiredTransfers` | [retention.go:24](../../../backend/modules/exporttransfer/compose/retention.go#L24) | runtime retained | `r.now`, `domain.JournalRetentionCutoff`, `r.journal.DeleteExpired`, `int`, `now.Sub(cutoff).Hours`, `now.Sub` |

### `modules/facilities/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `observer` | [new.go:55](../../../backend/modules/facilities/compose/new.go#L55) | runtime retained | `application.MapError`, `observe` |
| `databaseRuntime` | [new.go:62](../../../backend/modules/facilities/compose/new.go#L62) | runtime retained | `tenant.TenantFromContext`, `fmt.Errorf`, `tenant.TransactionFromContext`, `fmt.Errorf`, `tenantID.Int64`, `tenantID.Int64`, `fmt.Errorf`, `fmt.Errorf` |
| `engine.RequireTenant` | [new.go:88](../../../backend/modules/facilities/compose/new.go#L88) | runtime retained | `tenant.TenantFromContext` |
| `transaction.RunWrite` | [new.go:95](../../../backend/modules/facilities/compose/new.go#L95) | runtime retained | `tenant.TransactionFromContext`, `callback`, `tenant.WithinCurrentTenant` |
| `transaction.AcquireLock` | [new.go:102](../../../backend/modules/facilities/compose/new.go#L102) | direct binding | `tenant.AcquireLock` |
| `OccupancyProjection` | [projections.go:21](../../../backend/modules/facilities/compose/projections.go#L21) | direct binding | `facilitiesService.NewOccupancyProjection` |
| `HistoryProjection` | [projections.go:24](../../../backend/modules/facilities/compose/projections.go#L24) | direct binding | `facilitiesService.NewHistoryProjection` |
| `SettingHandlers` | [projections.go:27](../../../backend/modules/facilities/compose/projections.go#L27) | direct binding | `facilitiesService.SettingHandlers` |
| `FindRoomNames` | [room_rows.go:10](../../../backend/modules/facilities/compose/room_rows.go#L10) | direct binding | `facilitiesService.FindRoomNames` |
| `unusedRooms.HasActiveSessions` | [unobserved.go:27](../../../backend/modules/facilities/compose/unobserved.go#L27) | direct binding | none |
| `unusedRooms.RequiredByCareOffering` | [unobserved.go:28](../../../backend/modules/facilities/compose/unobserved.go#L28) | direct binding | none |

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
| `PersistenceDatabaseForTest` | [ambient_scope_test_helpers.go:11](../../../backend/modules/filestorage/compose/ambient_scope_test_helpers.go#L11) | test-only | `databaseRuntime` |
| `observer` | [new.go:162](../../../backend/modules/filestorage/compose/new.go#L162) | runtime retained | `application.MapPublicError`, `observe` |
| `databaseRuntime` | [new.go:169](../../../backend/modules/filestorage/compose/new.go#L169) | direct binding | none |
| `transaction.TenantID` | [new.go:175](../../../backend/modules/filestorage/compose/new.go#L175) | direct binding | `tenant.FromContext` |
| `transaction.RunWrite` | [new.go:177](../../../backend/modules/filestorage/compose/new.go#L177) | runtime retained | `tenant.TransactionFromContext`, `callback`, `tenant.WithinCurrentTenant` |
| `transaction.RunInTenant` | [new.go:184](../../../backend/modules/filestorage/compose/new.go#L184) | runtime retained | `tenant.NewTenantID`, `tenant.WithinTenant` |
| `transaction.AcquireLock` | [new.go:192](../../../backend/modules/filestorage/compose/new.go#L192) | direct binding | `tenant.AcquireLock` |
| `transaction.AfterCommit` | [new.go:196](../../../backend/modules/filestorage/compose/new.go#L196) | direct binding | `tenant.RegisterAfterCommit` |
| `transaction.Detach` | [new.go:200](../../../backend/modules/filestorage/compose/new.go#L200) | runtime retained | `context.WithoutCancel`, `tenant.ContextWithoutAfterCommitHooks`, `tenant.ContextWithoutTransaction` |
| `objectStore.Save` | [new.go:211](../../../backend/modules/filestorage/compose/new.go#L211) | direct binding | `s.backend.SavePrivate` |
| `objectStore.Open` | [new.go:215](../../../backend/modules/filestorage/compose/new.go#L215) | direct binding | `s.backend.OpenPrivate` |
| `objectStore.Remove` | [new.go:219](../../../backend/modules/filestorage/compose/new.go#L219) | direct binding | `s.backend.RemovePrivate` |
| `identity.HasActiveMembership` | [new.go:233](../../../backend/modules/filestorage/compose/new.go#L233) | direct binding | `i.module.HasActiveSchoolMembership` |
| `identity.ListAccountRoleIDs` | [new.go:237](../../../backend/modules/filestorage/compose/new.go#L237) | direct binding | `i.module.ListAccountRoleIDs` |
| `identity.ListShareableRoles` | [new.go:243](../../../backend/modules/filestorage/compose/new.go#L243) | runtime retained | `i.module.ListSchoolRoles`, `make`, `len`, `append` |
| `identity.ListActiveAccountIDs` | [new.go:258](../../../backend/modules/filestorage/compose/new.go#L258) | direct binding | `i.module.ListActiveSchoolAccountIDs` |
| `people.PersonNames` | [new.go:264](../../../backend/modules/filestorage/compose/new.go#L264) | runtime retained | `p.query.ListPersonsByAccount`, `fmt.Errorf`, `make`, `len`, `person.UpdatedAt.After`, `make`, `len` |
| `cleanupRetention.DeleteFinishedCleanups` | [retention.go:25](../../../backend/modules/filestorage/compose/retention.go#L25) | runtime retained | `r.retention.DeleteFinished`, `int`, `domain.CleanupRetentionWindow.Hours` |
| `retentionDatabase` | [retention.go:35](../../../backend/modules/filestorage/compose/retention.go#L35) | runtime retained | `tenant.TenantFromContext`, `fmt.Errorf`, `tenant.TransactionFromContext`, `errors.New`, `fmt.Errorf`, `tenantID.Int64` |

### `modules/grouplive/compose`

No non-constructor functions.


### `modules/guardianlinkview/compose`

No non-constructor functions.


### `modules/identityaccess/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `tenantRuntime.WithAdminTx` | [account_authentication.go:99](../../../backend/modules/identityaccess/compose/account_authentication.go#L99) | runtime retained | `tenant.WithinAdmin`, `r.attach` |
| `tenantRuntime.WithTenantTx` | [account_authentication.go:103](../../../backend/modules/identityaccess/compose/account_authentication.go#L103) | runtime retained | `tenant.WithTenantTx`, `r.attach`, `fn` |
| `tenantRuntime.RunInTx` | [account_authentication.go:112](../../../backend/modules/identityaccess/compose/account_authentication.go#L112) | runtime retained | `r.attach`, `tenant.TransactionFromContext`, `fn`, `tenant.ScopeFromContext`, `tenant.FromContext`, `tenant.OrgFromContext`, `tenant.WithinAdmin`, `r.runner.RunInTx` |
| `tenantRuntime.IsAdminTx` | [account_authentication.go:124](../../../backend/modules/identityaccess/compose/account_authentication.go#L124) | direct binding | `tenant.IsAdminTx` |
| `tenantRuntime.HasTransaction` | [account_authentication.go:126](../../../backend/modules/identityaccess/compose/account_authentication.go#L126) | runtime retained | `tenant.TransactionFromContext` |
| `tenantRuntime.HasAfterCommitHooks` | [account_authentication.go:131](../../../backend/modules/identityaccess/compose/account_authentication.go#L131) | direct binding | `tenant.HasAfterCommitHooks` |
| `tenantRuntime.RegisterAfterCommit` | [account_authentication.go:135](../../../backend/modules/identityaccess/compose/account_authentication.go#L135) | direct binding | `tenant.RegisterAfterCommit` |
| `tenantRuntime.TenantID` | [account_authentication.go:139](../../../backend/modules/identityaccess/compose/account_authentication.go#L139) | direct binding | `tenant.FromContext` |
| `tenantRuntime.Scope` | [account_authentication.go:141](../../../backend/modules/identityaccess/compose/account_authentication.go#L141) | direct binding | `tenant.ScopeFromContext` |
| `tenantRuntime.OrgID` | [account_authentication.go:143](../../../backend/modules/identityaccess/compose/account_authentication.go#L143) | direct binding | `tenant.OrgFromContext` |
| `tenantRuntime.WithTenantID` | [account_authentication.go:145](../../../backend/modules/identityaccess/compose/account_authentication.go#L145) | direct binding | `tenant.WithTenantID` |
| `tenantRuntime.Detach` | [account_authentication.go:149](../../../backend/modules/identityaccess/compose/account_authentication.go#L149) | runtime retained | `tenant.ContextWithoutAfterCommitHooks`, `tenant.ContextWithoutTenant`, `tenant.ContextWithoutTransaction` |
| `tenantRuntime.WithoutTransaction` | [account_authentication.go:153](../../../backend/modules/identityaccess/compose/account_authentication.go#L153) | direct binding | `tenant.ContextWithoutTransaction` |
| `tenantRuntime.AcquireLock` | [account_authentication.go:157](../../../backend/modules/identityaccess/compose/account_authentication.go#L157) | direct binding | `tenant.AcquireLock` |
| `rotationPolicy.RecoveryGrace` | [account_authentication.go:166](../../../backend/modules/identityaccess/compose/account_authentication.go#L166) | direct binding | none |
| `rotationPolicy.MaxRecoveryHops` | [account_authentication.go:168](../../../backend/modules/identityaccess/compose/account_authentication.go#L168) | direct binding | none |
| `rotationPolicy.RecoveryProofHash` | [account_authentication.go:170](../../../backend/modules/identityaccess/compose/account_authentication.go#L170) | direct binding | `rotation.RecoveryProofHash` |
| `rotationPolicy.MatchesRecoveryProof` | [account_authentication.go:174](../../../backend/modules/identityaccess/compose/account_authentication.go#L174) | direct binding | `rotation.MatchesRecoveryProof` |
| `rotationPolicy.FamilyFingerprint` | [account_authentication.go:178](../../../backend/modules/identityaccess/compose/account_authentication.go#L178) | direct binding | `rotation.FamilyFingerprint` |
| `guardianEnrollments` | [account_lifecycle.go:99](../../../backend/modules/identityaccess/compose/account_lifecycle.go#L99) | runtime retained | none |
| `requestMemo` | [caller_context.go:159](../../../backend/modules/identityaccess/compose/caller_context.go#L159) | runtime retained | `memo` |
| `composeDemoAccess` | [demo_access.go:36](../../../backend/modules/identityaccess/compose/demo_access.go#L36) | runtime retained | `errors.New`, `application.NewDemoAccess`, `newStore`, `identityaccess.NewDemoAccess` |
| `demoAccessExpiryEngine.ExpireDemoAccesses` | [demo_access.go:77](../../../backend/modules/identityaccess/compose/demo_access.go#L77) | runtime retained | `tenant.WithAdminTx`, `postgres.NewDemoAccessExpiryStore(tx).DeleteExpiredDemoAccesses`, `postgres.NewDemoAccessExpiryStore`, `e.now` |
| `guardianStudentAccess.GrantGuardianStudentAccess` | [guardian_student_access.go:32](../../../backend/modules/identityaccess/compose/guardian_student_access.go#L32) | runtime retained | `time.Now`, `a.GuardianStudentAccessStore.GrantGuardianStudentAccess`, `postgres.GuardianStudentAccessRecord`, `application.MapError`, `a.observe`, `time.Since` |
| `guardianStudentAccess.SetGuardianStudentPermissions` | [guardian_student_access.go:40](../../../backend/modules/identityaccess/compose/guardian_student_access.go#L40) | runtime retained | `time.Now`, `a.GuardianStudentAccessStore.SetGuardianStudentPermissions`, `application.MapError`, `a.observe`, `time.Since` |
| `RunMailStepInSavepoint` | [mail_transactions.go:22](../../../backend/modules/identityaccess/compose/mail_transactions.go#L22) | runtime retained | `tenant.WithSavepoint`, `errors.Is` |
| `RunMailStep` | [mail_transactions.go:32](../../../backend/modules/identityaccess/compose/mail_transactions.go#L32) | runtime retained | `tenant.TransactionFromContext`, `step`, `RunMailStepInSavepoint` |
| `savepointGuardianEnrollments.ClaimGuardianEnrollments` | [mail_transactions.go:54](../../../backend/modules/identityaccess/compose/mail_transactions.go#L54) | runtime retained | `tenant.WithSavepoint`, `e.backfill.BackfillGuardianAccountID`, `errors.Is`, `fmt.Errorf` |
| `AccountExists` | [maintenance.go:47](../../../backend/modules/identityaccess/compose/maintenance.go#L47) | runtime retained | `profiles.FindAccountMetadata`, `errors.Is` |
| `MaskEmail` | [mask_email.go:8](../../../backend/modules/identityaccess/compose/mask_email.go#L8) | direct binding | `domain.MaskEmail` |
| `withPasskeyFlows` | [mfa.go:139](../../../backend/modules/identityaccess/compose/mfa.go#L139) | runtime retained | `errors.New`, `mfaRuntime`, `application.NewAccountPasskeyFlows`, `application.NewOperatorPasskeyFlows` |
| `mfaRuntime` | [mfa.go:180](../../../backend/modules/identityaccess/compose/mfa.go#L180) | direct binding | `newTransactionRunner` |
| `operatorLoginGate` | [mfa.go:186](../../../backend/modules/identityaccess/compose/mfa.go#L186) | runtime retained | none |
| `bindTenantRuntime` | [new.go:162](../../../backend/modules/identityaccess/compose/new.go#L162) | runtime retained | `unit.IsZero`, `tenant.WithUnitOfWork` |
| `invitationLogger` | [school_invitation.go:81](../../../backend/modules/identityaccess/compose/school_invitation.go#L81) | runtime retained | `slog.Default` |
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
| `childQuotaLimits.ChildQuotaLimit` | [child_quota.go:26](../../../backend/modules/organizationtenancy/compose/child_quota.go#L26) | runtime retained | `tenant.TenantFromContext`, `fmt.Errorf`, `tenant.TransactionFromContext`, `errors.New`, `fmt.Errorf`, `postgres.ReadChildQuota`, `tenantID.Int64`, `application.MapError`, `organizationtenancy.ChildQuota{Bundles: *bundles, BundleSize: bundleSize}.Limit` |
| `demoQueueEngine.RequeueDeferredDemoSchoolOrders` | [demo_queue.go:27](../../../backend/modules/organizationtenancy/compose/demo_queue.go#L27) | direct binding | `e.store.RequeueDeferred` |
| `demoQueueEngine.RequeueDeferredDemoSchoolOrder` | [demo_queue.go:31](../../../backend/modules/organizationtenancy/compose/demo_queue.go#L31) | direct binding | `e.store.RequeueDeferred` |
| `demoQueueEngine.ReleaseDemoSchoolOrders` | [demo_queue.go:35](../../../backend/modules/organizationtenancy/compose/demo_queue.go#L35) | direct binding | `e.store.ReleaseClaims` |
| `demoQueueEngine.ClaimDemoSchoolOrder` | [demo_queue.go:39](../../../backend/modules/organizationtenancy/compose/demo_queue.go#L39) | runtime retained | `e.store.Claim` |
| `demoQueueEngine.FinishDemoSchoolOrder` | [demo_queue.go:50](../../../backend/modules/organizationtenancy/compose/demo_queue.go#L50) | direct binding | `e.store.Finish` |
| `demoQueueEngine.FailDemoSchoolOrder` | [demo_queue.go:54](../../../backend/modules/organizationtenancy/compose/demo_queue.go#L54) | direct binding | `e.store.Fail` |
| `demoQueueEngine.ReadyDemoSchools` | [demo_queue.go:58](../../../backend/modules/organizationtenancy/compose/demo_queue.go#L58) | direct binding | `e.store.ReadyNames` |
| `demoQueueEngine.ActiveDemoSchools` | [demo_queue.go:62](../../../backend/modules/organizationtenancy/compose/demo_queue.go#L62) | direct binding | `e.store.ActiveNames` |
| `demoOrderEngine.OrderDemoSchool` | [demo_queue.go:96](../../../backend/modules/organizationtenancy/compose/demo_queue.go#L96) | runtime retained | `make`, `io.ReadFull`, `fmt.Errorf`, `int`, `len`, `organizationtenancy.DemoSchoolSlug`, `string`, `e.store.Enqueue`, `errors.Is` |
| `demoOrderEngine.DemoSchoolProgress` | [demo_queue.go:115](../../../backend/modules/organizationtenancy/compose/demo_queue.go#L115) | runtime retained | `e.store.Progress` |
| `demoOrderEngine.MarkDemoSchoolUsed` | [demo_queue.go:126](../../../backend/modules/organizationtenancy/compose/demo_queue.go#L126) | direct binding | `e.store.MarkUsed` |
| `demoQueueEngine.ReturnDemoSchoolOrder` | [demo_queue.go:130](../../../backend/modules/organizationtenancy/compose/demo_queue.go#L130) | direct binding | `e.store.Return` |
| `demoQueueEngine.RetireDemoSchools` | [demo_queue.go:134](../../../backend/modules/organizationtenancy/compose/demo_queue.go#L134) | direct binding | `e.store.RetireMany` |
| `demoOrderEngine.RetireDemoSchool` | [demo_queue.go:138](../../../backend/modules/organizationtenancy/compose/demo_queue.go#L138) | direct binding | `e.store.Retire` |
| `demoStateEngine.LoadDemoSchool` | [demo_state.go:21](../../../backend/modules/organizationtenancy/compose/demo_state.go#L21) | runtime retained | `e.store.Load`, `[]byte` |
| `demoStateEngine.ReserveDemoSchool` | [demo_state.go:29](../../../backend/modules/organizationtenancy/compose/demo_state.go#L29) | direct binding | `e.store.Reserve` |
| `demoStateEngine.RememberDemoSchool` | [demo_state.go:33](../../../backend/modules/organizationtenancy/compose/demo_state.go#L33) | runtime retained | `e.store.Remember`, `string` |
| `demoStateEngine.UpdateDemoSchool` | [demo_state.go:37](../../../backend/modules/organizationtenancy/compose/demo_state.go#L37) | runtime retained | `e.store.Update`, `string` |
| `demoStateEngine.WithDemoLease` | [demo_state.go:41](../../../backend/modules/organizationtenancy/compose/demo_state.go#L41) | runtime retained | `e.store.WithLease` |
| `transaction.RunAdmin` | [new.go:47](../../../backend/modules/organizationtenancy/compose/new.go#L47) | direct binding | `tenant.WithinAdmin` |
| `transaction.RunTenant` | [new.go:51](../../../backend/modules/organizationtenancy/compose/new.go#L51) | runtime retained | `tenant.NewTenantID`, `tenant.WithinTenant` |
| `transaction.RunRead` | [new.go:59](../../../backend/modules/organizationtenancy/compose/new.go#L59) | runtime retained | `tenant.TransactionFromContext`, `callback`, `tenant.WithinAdmin` |
| `OperatorDatabase` | [provisioning.go:91](../../../backend/modules/organizationtenancy/compose/provisioning.go#L91) | runtime retained | `tenant.TransactionFromContext`, `errors.New`, `fmt.Errorf` |

### `modules/parentannouncementaudience`

No non-constructor functions.


### `modules/parentmessageinbox`

No non-constructor functions.


### `modules/parentrequestfeedprojection/compose`

No non-constructor functions.


### `modules/peopledirectory/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `requireBunTransaction` | [caregiver_capability_binding.go:20](../../../backend/modules/peopledirectory/compose/caregiver_capability_binding.go#L20) | runtime retained | `tenant.TransactionFromContext`, `fmt.Errorf`, `fmt.Errorf` |
| `CaregiverCapabilityViews.inSchool` | [caregiver_capability_views.go:25](../../../backend/modules/peopledirectory/compose/caregiver_capability_views.go#L25) | construction/binding | `tenant.WithAdminTx`, `callback` |
| `*GuardianProfileLoader.LoadForTenant` | [guardian_profile_loader.go:39](../../../backend/modules/peopledirectory/compose/guardian_profile_loader.go#L39) | runtime retained | `tenant.WithTenantTx`, `s.repo.LoadProfileWithChildren` |
| `guardianTransactions.RunInTx` | [guardian_service.go:25](../../../backend/modules/peopledirectory/compose/guardian_service.go#L25) | runtime retained | `tenant.NewTransactionRunner().RunInTx`, `tenant.NewTransactionRunner` |
| `guardianTransactions.WithTenantTx` | [guardian_service.go:29](../../../backend/modules/peopledirectory/compose/guardian_service.go#L29) | runtime retained | `tenant.WithTenantTx`, `tenant.FromContext`, `fn` |
| `guardianTransactions.TenantID` | [guardian_service.go:35](../../../backend/modules/peopledirectory/compose/guardian_service.go#L35) | direct binding | `tenant.FromContext` |
| `ApplyDefaultGuardianRole` | [guardian_service.go:40](../../../backend/modules/peopledirectory/compose/guardian_service.go#L40) | direct binding | `application.ApplyDefaultGuardianRole` |
| `transaction.RequireTransaction` | [new.go:150](../../../backend/modules/peopledirectory/compose/new.go#L150) | direct binding | `tenant.RequireTransaction` |
| `transaction.TenantID` | [new.go:155](../../../backend/modules/peopledirectory/compose/new.go#L155) | direct binding | `tenant.FromContext` |
| `transaction.RegisterAfterCommit` | [new.go:159](../../../backend/modules/peopledirectory/compose/new.go#L159) | direct binding | `tenant.RegisterAfterCommit` |
| `transaction.RunWrite` | [new.go:163](../../../backend/modules/peopledirectory/compose/new.go#L163) | runtime retained | `tenant.NewTransactionRunner().RunInTx`, `tenant.NewTransactionRunner` |
| `transaction.RunRead` | [new.go:167](../../../backend/modules/peopledirectory/compose/new.go#L167) | runtime retained | `tenant.NewTransactionRunner().RunInTx`, `tenant.NewTransactionRunner` |
| `transaction.RunSavepoint` | [new.go:171](../../../backend/modules/peopledirectory/compose/new.go#L171) | direct binding | `tenant.WithSavepoint` |
| `transaction.RunAdminRead` | [new.go:178](../../../backend/modules/peopledirectory/compose/new.go#L178) | runtime retained | `tenant.WithinAdmin`, `tenant.ContextWithoutTransaction` |
| `OperatorPersonStudentMembership` | [operator_person_students.go:12](../../../backend/modules/peopledirectory/compose/operator_person_students.go#L12) | direct binding | `usersRepo.FindOperatorPersonStudentMembership` |
| `defaultGuardianRole` | [repository_adapter_people.go:83](../../../backend/modules/peopledirectory/compose/repository_adapter_people.go#L83) | runtime retained | `ApplyDefaultGuardianRole` |
| `ParseRetainedCalendarDate` | [repository_adapter_record_errors.go:10](../../../backend/modules/peopledirectory/compose/repository_adapter_record_errors.go#L10) | direct binding | `usersRepo.ParseCalendarDate` |
| `CareStudentLock` | [repository_adapter_student_lock_test_helpers.go:14](../../../backend/modules/peopledirectory/compose/repository_adapter_student_lock_test_helpers.go#L14) | test-only | none |
| `StudentTagReleaser.ReleaseStudentTags` | [repository_adapter_student_tag_release.go:23](../../../backend/modules/peopledirectory/compose/repository_adapter_student_tag_release.go#L23) | runtime retained | `r.ReleaseStudentTagsByIDs`, `len` |
| `RetainedWithPortalMemberships` | [retained_repository_suites_test_helpers.go:63](../../../backend/modules/peopledirectory/compose/retained_repository_suites_test_helpers.go#L63) | test-only | `usersRepo.WithPortalMemberships` |
| `RetainedWithGuardianRelationshipMemberships` | [retained_repository_suites_test_helpers.go:69](../../../backend/modules/peopledirectory/compose/retained_repository_suites_test_helpers.go#L69) | test-only | `usersRepo.WithGuardianRelationshipMemberships` |
| `RetainedWithGuardianRelationshipOwners` | [retained_repository_suites_test_helpers.go:75](../../../backend/modules/peopledirectory/compose/retained_repository_suites_test_helpers.go#L75) | test-only | `usersRepo.WithGuardianRelationshipOwners` |
| `RetainedWithGuardianDefaultRole` | [retained_repository_suites_test_helpers.go:81](../../../backend/modules/peopledirectory/compose/retained_repository_suites_test_helpers.go#L81) | test-only | `usersRepo.WithGuardianDefaultRole` |
| `DisabledStudentPhotos` | [retained_repository_suites_test_helpers.go:88](../../../backend/modules/peopledirectory/compose/retained_repository_suites_test_helpers.go#L88) | test-only | none |
| `*studentAuditActorPort.RecordChanges` | [student_audit_port.go:33](../../../backend/modules/peopledirectory/compose/student_audit_port.go#L33) | direct binding | `s.recorder.RecordChanges` |
| `*studentAuditActorPort.RecordChangesForActor` | [student_audit_port.go:42](../../../backend/modules/peopledirectory/compose/student_audit_port.go#L42) | runtime retained | `s.RecordStudentChangesForActor`, `studentAuditSnapshot`, `studentAuditSnapshot` |
| `*studentAuditActorPort.RecordPickupPlanForActor` | [student_audit_port.go:50](../../../backend/modules/peopledirectory/compose/student_audit_port.go#L50) | runtime retained | `s.recorder.RecordPickupPlan`, `s.actorDisplayName` |
| `*studentAuditActorPort.RecordSystemStatusChange` | [student_audit_port.go:60](../../../backend/modules/peopledirectory/compose/student_audit_port.go#L60) | direct binding | `s.recorder.RecordSystemStatusChange` |
| `*studentAuditActorPort.actorDisplayName` | [student_audit_port.go:71](../../../backend/modules/peopledirectory/compose/student_audit_port.go#L71) | runtime retained | `s.actor` |
| `*studentAuditActorPort.RecordStudentChangesForActor` | [student_audit_port.go:81](../../../backend/modules/peopledirectory/compose/student_audit_port.go#L81) | runtime retained | `s.actor`, `s.recorder.RecordStudentChanges` |
| `studentAuditSnapshot` | [student_audit_port.go:92](../../../backend/modules/peopledirectory/compose/student_audit_port.go#L92) | runtime retained | `value.AuditSnapshot` |
| `disabledStudentPhotos.PhotoFeatureEnabled` | [student_photo.go:37](../../../backend/modules/peopledirectory/compose/student_photo.go#L37) | direct binding | none |
| `disabledStudentPhotos.ActingAccountID` | [student_photo.go:38](../../../backend/modules/peopledirectory/compose/student_photo.go#L38) | direct binding | none |
| `disabledStudentPhotos.QueueUnlinkStoredPhoto` | [student_photo.go:39](../../../backend/modules/peopledirectory/compose/student_photo.go#L39) | direct binding | none |
| `disabledStudentPhotos.UnlinkStoredPhoto` | [student_photo.go:40](../../../backend/modules/peopledirectory/compose/student_photo.go#L40) | direct binding | none |
| `disabledStudentPhotos.BroadcastPhotoChange` | [student_photo.go:41](../../../backend/modules/peopledirectory/compose/student_photo.go#L41) | direct binding | none |
| `disabledStudentPhotos.RecordPhotoConsent` | [student_photo.go:42](../../../backend/modules/peopledirectory/compose/student_photo.go#L42) | direct binding | none |
| `studentPhotoPort` | [student_photo.go:49](../../../backend/modules/peopledirectory/compose/student_photo.go#L49) | runtime retained | none |
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
| `PersistenceRuntimeFor` | [new.go:29](../../../backend/modules/schoolcalendar/compose/new.go#L29) | runtime retained | `panic`, `tenant.CurrentTransaction[bun.IDB]`, `fmt.Errorf` |
| `writeTransaction` | [new.go:120](../../../backend/modules/schoolcalendar/compose/new.go#L120) | runtime retained | `tenant.TransactionFromContext`, `fn`, `tenant.TenantFromContext`, `fmt.Errorf`, `tenant.WithinCurrentTenant` |

### `modules/schoolcalendar/portal/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `*TenantRuntime.TenantID` | [tenant_runtime.go:24](../../../backend/modules/schoolcalendar/portal/compose/tenant_runtime.go#L24) | direct binding | `tenant.FromContext` |
| `*TenantRuntime.WithinAppointmentWrite` | [tenant_runtime.go:28](../../../backend/modules/schoolcalendar/portal/compose/tenant_runtime.go#L28) | runtime retained | `tenant.TransactionFromContext`, `command`, `tenant.TenantFromContext`, `fmt.Errorf`, `tenant.WithinTenant` |
| `*TenantRuntime.WithinTenant` | [tenant_runtime.go:40](../../../backend/modules/schoolcalendar/portal/compose/tenant_runtime.go#L40) | runtime retained | `tenant.WithTenantTx`, `command` |
| `*TenantRuntime.WithinAdmin` | [tenant_runtime.go:45](../../../backend/modules/schoolcalendar/portal/compose/tenant_runtime.go#L45) | runtime retained | `tenant.WithAdminTx`, `command` |
| `*TenantRuntime.WithoutTransactionAndHooks` | [tenant_runtime.go:51](../../../backend/modules/schoolcalendar/portal/compose/tenant_runtime.go#L51) | runtime retained | `tenant.ContextWithoutAfterCommitHooks`, `tenant.ContextWithoutTransaction` |

### `modules/schoolmembership/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `StudentClassWriteGateQuery` | [student_class_gate.go:14](../../../backend/modules/schoolmembership/compose/student_class_gate.go#L14) | direct binding | `postgres.ResolveStudentClassWriteGate` |
| `StudentClassWriteGate` | [student_class_gate.go:20](../../../backend/modules/schoolmembership/compose/student_class_gate.go#L20) | direct binding | `StudentClassWriteGateQuery` |

### `modules/schoolsetup/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `ambientTx` | [compose.go:48](../../../backend/modules/schoolsetup/compose/compose.go#L48) | runtime retained | `schoolsetupview.AmbientTx`, `tenant.TransactionFromContext` |
| `AmbientRuntime.DB` | [compose.go:56](../../../backend/modules/schoolsetup/compose/compose.go#L56) | direct binding | `ambientTx` |

### `modules/schoolstructure/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `publicGroupError` | [group_values.go:9](../../../backend/modules/schoolstructure/compose/group_values.go#L9) | direct binding | `groups.PublicGroupError` |
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
| `*TeacherGroupIDs.TeacherGroupIDs` | [teacher_group_ids_test_helpers.go:21](../../../backend/modules/schoolstructure/compose/teacher_group_ids_test_helpers.go#L21) | test-only | `r.records.GetTeacherGroups`, `make`, `len`, `append` |

### `modules/settings/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `auditRetention.DeleteExpiredSettingChanges` | [audit_retention.go:24](../../../backend/modules/settings/compose/audit_retention.go#L24) | runtime retained | `tenant.TenantFromContext`, `fmt.Errorf`, `tenant.TransactionFromContext`, `errors.New`, `fmt.Errorf`, `r.now`, `now.AddDate`, `configRepo.NewSettingAuditRetention(transactionRuntime{tx: tx}).DeleteChangedBefore`, `configRepo.NewSettingAuditRetention`, `tenantID.Int64`, `int`, `now.Sub(cutoff).Hours`, `now.Sub` |
| `transactionRuntime.DB` | [audit_retention.go:49](../../../backend/modules/settings/compose/audit_retention.go#L49) | direct binding | none |
| `*operatorSchoolSettings.Schema` | [operator.go:65](../../../backend/modules/settings/compose/operator.go#L65) | runtime retained | `s.runtime.WithinTenant`, `s.settings.GetSchemaForOperator`, `json.Marshal` |
| `*operatorSchoolSettings.Reveal` | [operator.go:78](../../../backend/modules/settings/compose/operator.go#L78) | runtime retained | `s.runtime.WithinTenant`, `s.settings.Resolve` |
| `operatorSettingsRuntime.WithinTenant` | [operator.go:90](../../../backend/modules/settings/compose/operator.go#L90) | runtime retained | `tenant.WithTenantTx`, `fn` |
| `operatorSettingsRuntime.AfterCommit` | [operator.go:96](../../../backend/modules/settings/compose/operator.go#L96) | direct binding | `tenant.RegisterAfterCommit` |
| `operatorSettingsRuntime.Today` | [operator.go:100](../../../backend/modules/settings/compose/operator.go#L100) | runtime retained | `calendar.TodayDate`, `configModel.NewCalendarDate`, `today.Year`, `today.Month`, `today.Day` |
| `PresenceSettings` | [presence_settings.go:6](../../../backend/modules/settings/compose/presence_settings.go#L6) | direct binding | `config.NewPresenceSettings` |
| `TenantSettingsRuntime.TenantID` | [tenant_settings_runtime.go:28](../../../backend/modules/settings/compose/tenant_settings_runtime.go#L28) | direct binding | `tenant.FromContext` |
| `TenantSettingsRuntime.HasTransaction` | [tenant_settings_runtime.go:30](../../../backend/modules/settings/compose/tenant_settings_runtime.go#L30) | runtime retained | `tenant.TransactionFromContext` |
| `TenantSettingsRuntime.DB` | [tenant_settings_runtime.go:35](../../../backend/modules/settings/compose/tenant_settings_runtime.go#L35) | direct binding | `tenant.CurrentTransaction[bun.IDB]` |
| `TenantSettingsRuntime.LockStaffBalance` | [tenant_settings_runtime.go:39](../../../backend/modules/settings/compose/tenant_settings_runtime.go#L39) | runtime retained | `errors.New`, `r.TenantID`, `errors.New`, `r.AcquireLock`, `fmt.Sprintf`, `fmt.Errorf` |
| `TenantSettingsRuntime.TodayTime` | [tenant_settings_runtime.go:53](../../../backend/modules/settings/compose/tenant_settings_runtime.go#L53) | runtime retained | `timezone.TodayDate().UTCMidnight`, `timezone.TodayDate` |
| `TenantSettingsRuntime.WithinTenant` | [tenant_settings_runtime.go:55](../../../backend/modules/settings/compose/tenant_settings_runtime.go#L55) | runtime retained | `tenant.WithUnitOfWork`, `tenant.WithTenantTx`, `fn` |
| `TenantSettingsRuntime.WithinAdmin` | [tenant_settings_runtime.go:64](../../../backend/modules/settings/compose/tenant_settings_runtime.go#L64) | runtime retained | `tenant.WithUnitOfWork`, `tenant.WithAdminTx`, `fn` |
| `TenantSettingsRuntime.AcquireLock` | [tenant_settings_runtime.go:73](../../../backend/modules/settings/compose/tenant_settings_runtime.go#L73) | runtime retained | `tenant.WithUnitOfWork`, `tenant.AcquireLock` |
| `TenantSettingsRuntime.AfterCommit` | [tenant_settings_runtime.go:80](../../../backend/modules/settings/compose/tenant_settings_runtime.go#L80) | direct binding | `tenant.RegisterAfterCommit` |

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
| `reportMirrorPanic` | [attendance_mirror_report.go:26](../../../backend/modules/timetable/compose/attendance_mirror_report.go#L26) | runtime retained | `sentry.GetHubFromContext`, `sentry.CurrentHub`, `hub.WithScope`, `mirrorSchoolID`, `scope.SetTag`, `strconv.FormatInt`, `hub.RecoverWithContext`, `hub.Flush` |
| `mirrorSchoolID` | [attendance_mirror_report.go:42](../../../backend/modules/timetable/compose/attendance_mirror_report.go#L42) | runtime retained | `tenant.TenantFromContext`, `tenantID.Int64` |
| `CareScheduleRepositories.EnrollmentPickupSchedules` | [care_schedule_repositories_test_helpers.go:38](../../../backend/modules/timetable/compose/care_schedule_repositories_test_helpers.go#L38) | test-only | `application.NewEnrollmentPickupSchedules` |
| `CareScheduleRepositories.EnrollmentArrivalSchedules` | [care_schedule_repositories_test_helpers.go:44](../../../backend/modules/timetable/compose/care_schedule_repositories_test_helpers.go#L44) | test-only | `application.NewEnrollmentArrivalSchedules` |
| `classArrivalExceptionRepository.Upsert` | [class_arrival_exceptions_test_helpers.go:26](../../../backend/modules/timetable/compose/class_arrival_exceptions_test_helpers.go#L26) | test-only | `(transaction{}).RunWrite`, `r.store.UpsertClassArrivalException`, `postgres.TranslateNotFound` |
| `SubstituteDayLock` | [instance_lifecycle_ports.go:22](../../../backend/modules/timetable/compose/instance_lifecycle_ports.go#L22) | runtime retained | `postgres.New`, `databaseRuntime`, `store.AcquireTransactionLock`, `timetable.SubstituteDayLockKey`, `tenant.FromContext` |
| `WithMirrorAfterWrite` | [mirror_write_test_helpers.go:7](../../../backend/modules/timetable/compose/mirror_write_test_helpers.go#L7) | test-only | `application.WithMirrorAfterWrite` |
| `NativeGroupRecords` | [native_fixture_records.go:10](../../../backend/modules/timetable/compose/native_fixture_records.go#L10) | test-only | `application.NativeGroupRecords` |
| `NativeScheduleRecords` | [native_fixture_records.go:13](../../../backend/modules/timetable/compose/native_fixture_records.go#L13) | test-only | `application.NativeScheduleRecords` |
| `NativeEnrollmentRecords` | [native_fixture_records.go:16](../../../backend/modules/timetable/compose/native_fixture_records.go#L16) | test-only | `application.NativeEnrollmentRecords` |
| `NativeExceptionRecords` | [native_fixture_records.go:19](../../../backend/modules/timetable/compose/native_fixture_records.go#L19) | test-only | `application.NativeExceptionRecords` |
| `NativeTimeframeRecords` | [native_fixture_records.go:22](../../../backend/modules/timetable/compose/native_fixture_records.go#L22) | test-only | `application.NativeTimeframeRecords` |
| `databaseRuntime` | [new.go:51](../../../backend/modules/timetable/compose/new.go#L51) | runtime retained | `tenant.TenantFromContext`, `fmt.Errorf`, `tenant.TransactionFromContext`, `errors.New`, `tenantID.Int64`, `tenantID.Int64`, `errors.New`, `fmt.Errorf` |
| `transaction.RunWrite` | [new.go:77](../../../backend/modules/timetable/compose/new.go#L77) | runtime retained | `tenant.TransactionFromContext`, `callback`, `tenant.TenantFromContext`, `fmt.Errorf`, `tenant.WithinCurrentTenant`, `tenant.TenantFromContext`, `tenant.WithinTenantRetry` |
| `offboardingCountsToPublic` | [offboarding.go:39](../../../backend/modules/timetable/compose/offboarding.go#L39) | direct binding | none |
| `OwnerRows.SickCascadeRows` | [owner_rows.go:63](../../../backend/modules/timetable/compose/owner_rows.go#L63) | direct binding | `application.NewSickCascadeTimetableRows` |
| `*planningTrackAdministration.OrderPlanningTracks` | [planning_track_administration.go:21](../../../backend/modules/timetable/compose/planning_track_administration.go#L21) | construction/binding | `tenant.WithTenantTx`, `callback` |
| `PresenceDatabase` | [presence_reads.go:26](../../../backend/modules/timetable/compose/presence_reads.go#L26) | direct binding | `databaseRuntime` |
| `recurrenceWriteLock.LockRecurrenceWrites` | [recurrence_lock.go:53](../../../backend/modules/timetable/compose/recurrence_lock.go#L53) | runtime retained | `lockTenant`, `l.store.AcquireTransactionLock`, `schedule.TenantRecurrenceLockKey`, `fmt.Errorf` |
| `recurrenceWriteLock.LockRecurrenceWritesThenGradeTransitions` | [recurrence_lock.go:69](../../../backend/modules/timetable/compose/recurrence_lock.go#L69) | runtime retained | `l.LockRecurrenceWrites`, `lockTenant`, `l.store.AcquireTransactionLock`, `l.gradeTransitionLockKey`, `fmt.Errorf` |
| `lockTenant` | [recurrence_lock.go:83](../../../backend/modules/timetable/compose/recurrence_lock.go#L83) | runtime retained | `tenant.FromContext`, `errors.New`, `tenant.TransactionFromContext`, `errors.New` |
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
| `tenantTransactions.InTenant` | [tenant_transactions.go:15](../../../backend/modules/timetable/compose/tenant_transactions.go#L15) | runtime retained | `tenant.WithTenantTx`, `fn` |
| `blockDisplayMetadata.ListBlockDisplayMetadata` | [timetable_data_runtime.go:37](../../../backend/modules/timetable/compose/timetable_data_runtime.go#L37) | runtime retained | `tenant.TenantFromContext`, `blockDisplayDatabase`, `r.project`, `tenantID.Int64` |
| `blockDisplayDatabase` | [timetable_data_runtime.go:49](../../../backend/modules/timetable/compose/timetable_data_runtime.go#L49) | runtime retained | `tenant.TransactionFromContext`, `fmt.Errorf`, `fmt.Errorf`, `fmt.Errorf` |

### `modules/timetableenrollmentprovenance/compose`

No non-constructor functions.


### `modules/timetableprojection/compose`

No non-constructor functions.


### `modules/transactionruntime/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `OpenDatabase` | [database.go:32](../../../backend/modules/transactionruntime/compose/database.go#L32) | direct binding | `openDatabase` |
| `OpenServeDatabase` | [database.go:38](../../../backend/modules/transactionruntime/compose/database.go#L38) | runtime retained | `openDatabase`, `ObserveLockWaits`, `NewDatabase` |
| `OpenDemoDatabase` | [database.go:47](../../../backend/modules/transactionruntime/compose/database.go#L47) | direct binding | `openDatabase` |
| `bindDatabase` | [database.go:51](../../../backend/modules/transactionruntime/compose/database.go#L51) | construction/binding | `bind`, `database.NewBoundDatabase` |
| `openDatabase` | [database.go:59](../../../backend/modules/transactionruntime/compose/database.go#L59) | construction/binding | `open`, `bind`, `database.ClosePool` |
| `bindTenantRuntime` | [tenant_runtime.go:34](../../../backend/modules/transactionruntime/compose/tenant_runtime.go#L34) | construction/binding | `tenant.NewUnitOfWork`, `tenant.SavepointFunc`, `runtime.WithTransactionDetacher`, `runtime.WithContextAdapters` |
| `ObserveLockWaits` | [tenant_runtime.go:54](../../../backend/modules/transactionruntime/compose/tenant_runtime.go#L54) | direct binding | `database.AttachLockWaitHook` |
| `OpenPool` | [tenant_runtime.go:59](../../../backend/modules/transactionruntime/compose/tenant_runtime.go#L59) | direct binding | `database.DBConn` |
| `OpenServePool` | [tenant_runtime.go:62](../../../backend/modules/transactionruntime/compose/tenant_runtime.go#L62) | direct binding | `database.DBConnForServe` |
| `TenantCallback` | [tenant_runtime.go:65](../../../backend/modules/transactionruntime/compose/tenant_runtime.go#L65) | construction/binding | `fn` |

### `modules/workforce/compose`

| Function | Source | Classification | Calls |
|---|---|---|---|
| `transaction.LockStaffAbsence` | [absence_engine.go:14](../../../backend/modules/workforce/compose/absence_engine.go#L14) | runtime retained | `errors.New`, `tenant.FromContext`, `errors.New`, `t.acquireXactLock`, `fmt.Sprintf`, `fmt.Errorf` |
| `databaseRuntime` | [new.go:83](../../../backend/modules/workforce/compose/new.go#L83) | runtime retained | `tenant.TenantFromContext`, `tenant.TransactionFromContext`, `postgres.ResolveDatabase`, `id.Int64`, `tenant.IsAdminTx` |
| `transaction.RunWrite` | [new.go:99](../../../backend/modules/workforce/compose/new.go#L99) | runtime retained | `tenant.TransactionFromContext`, `callback`, `tenant.WithinCurrentTenant` |
| `transaction.LockStaffQualifications` | [new.go:106](../../../backend/modules/workforce/compose/new.go#L106) | runtime retained | `errors.New`, `tenant.FromContext`, `errors.New`, `t.acquireXactLock`, `fmt.Sprintf`, `fmt.Errorf` |
| `transaction.LockStaffBalance` | [new.go:123](../../../backend/modules/workforce/compose/new.go#L123) | runtime retained | `errors.New`, `tenant.FromContext`, `errors.New`, `t.acquireXactLock`, `fmt.Sprintf`, `fmt.Errorf` |
| `transaction.acquireXactLock` | [new.go:142](../../../backend/modules/workforce/compose/new.go#L142) | runtime retained | `tenant.TransactionFromContext`, `tenant.AcquireLock`, `errors.Is`, `t.lock` |
| `transaction.LockStaffShifts` | [new.go:152](../../../backend/modules/workforce/compose/new.go#L152) | runtime retained | `tenant.FromContext`, `errors.New`, `t.acquireXactLock`, `fmt.Sprintf`, `tenant.FromContext` |
| `assignments.AssignedStaffIDs` | [new.go:166](../../../backend/modules/workforce/compose/new.go#L166) | runtime retained | `a.store.StaffOnWorkTimeModel`, `len`, `a.live` |
| `assignments.RebaseAnchor` | [new.go:174](../../../backend/modules/workforce/compose/new.go#L174) | runtime retained | `len`, `a.store.RebaseStaffRotationAnchor` |
| `transaction.RunRead` | [new.go:183](../../../backend/modules/workforce/compose/new.go#L183) | direct binding | `t.RunWrite` |
| `retentionDatabase` | [retention.go:28](../../../backend/modules/workforce/compose/retention.go#L28) | runtime retained | `tenant.TenantFromContext`, `tenant.TransactionFromContext`, `postgres.ResolveDatabase`, `id.Int64` |
| `seriesOptions` | [shift_planning.go:236](../../../backend/modules/workforce/compose/shift_planning.go#L236) | runtime retained | `planning.WithStaffShiftSeriesBroadcaster`, `planning.WithStaffShiftSeriesToday`, `append`, `planning.WithStaffShiftSeriesNonWorkingDays`, `planning.SchoolCalendarNonWorkingDays` |
| `*ShiftPlanning.Planning` | [shift_planning.go:250](../../../backend/modules/workforce/compose/shift_planning.go#L250) | direct binding | `planning.NewStaffShiftPlanning` |
| `lockStaffShiftWrites` | [staff_shift_lock.go:27](../../../backend/modules/workforce/compose/staff_shift_lock.go#L27) | runtime retained | `tenant.FromContext`, `errors.New`, `fmt.Sprintf`, `(transaction{lock: postgres.New(databaseRuntime(db)).AcquireXactLock}).acquireXactLock`, `postgres.New`, `databaseRuntime`, `fmt.Errorf` |
| `*WorkSessionSide.WithSickCascade` | [time_tracking.go:145](../../../backend/modules/workforce/compose/time_tracking.go#L145) | runtime retained | `errors.New`, `s.clock.WithAbsences`, `timetracking.WorkSessionCapability`, `timetracking.TimeClockCapability`, `timetracking.StaffAbsenceCapability`, `timetracking.WorkTimeMonthCapability`, `timetracking.BalanceAdjustmentCapability`, `timetracking.MonthClosingCapability`, `timetracking.StaffOverviewCapability`, `timetracking.TimeTrackingAuditLogCapability`, `timetracking.StaffTimeExportCapability`, `timetracking.VacationTakeoverCapability`, `timetracking.OpeningBalanceBookingCapability` |
| `timeTrackingRows` | [time_tracking.go:175](../../../backend/modules/workforce/compose/time_tracking.go#L175) | runtime retained | `timetracking.MonthSnapshotCapability`, `timetracking.AbsenceTypes`, `timetracking.NewWorkSessionRows`, `timetracking.NewWorkSessionBreakRows`, `timetracking.NewBalanceAdjustmentRows`, `timetracking.NewVacationQuotaRows`, `timetracking.NewVacationOpeningRows` |
| `StaffScheduleAssignments` | [time_tracking_staff_schedule.go:23](../../../backend/modules/workforce/compose/time_tracking_staff_schedule.go#L23) | direct binding | none |
| `staffScheduleAssignments.ScheduleAssignment` | [time_tracking_staff_schedule.go:27](../../../backend/modules/workforce/compose/time_tracking_staff_schedule.go#L27) | runtime retained | `q.source.StaffEmployments`, `fmt.Errorf`, `timezone.Date` |
| `staffAllowanceTestDatabase` | [time_tracking_test_helpers.go:50](../../../backend/modules/workforce/compose/time_tracking_test_helpers.go#L50) | test-only | `databaseRuntime(db)`, `databaseRuntime`, `errors.New` |
| `WorkPresence` | [work_presence.go:18](../../../backend/modules/workforce/compose/work_presence.go#L18) | direct binding | none |
| `workPresence.GetTodayPresenceMap` | [work_presence.go:22](../../../backend/modules/workforce/compose/work_presence.go#L22) | direct binding | `p.capability.WorkPresenceMap` |
| `LegacyWorkTimeModelNotFound` | [work_time_model_not_found.go:13](../../../backend/modules/workforce/compose/work_time_model_not_found.go#L13) | runtime retained | `errors.Is` |

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

| Function | Source | Classification | Calls |
|---|---|---|---|
| `missingNames` | [new.go:126](../../../backend/workflows/shiftplansync/compose/new.go#L126) | runtime retained | `make`, `len`, `append`, `slices.Sort` |
| `shifts.ListStaffShifts` | [new.go:152](../../../backend/workflows/shiftplansync/compose/new.go#L152) | direct binding | `s.rows.ListStaffShifts` |
| `shifts.ApplyCancellation` | [new.go:156](../../../backend/workflows/shiftplansync/compose/new.go#L156) | direct binding | `s.planning.ApplyCancellation` |
| `shifts.SetStaffShiftSickAbsence` | [new.go:160](../../../backend/workflows/shiftplansync/compose/new.go#L160) | direct binding | `s.rows.SetStaffShiftSickAbsence` |
| `shifts.LockStaffShifts` | [new.go:164](../../../backend/workflows/shiftplansync/compose/new.go#L164) | direct binding | `s.lock` |

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
| `modules/appointments/compose` | `New` | [new.go:30](../../../backend/modules/appointments/compose/new.go#L30) | 2 | 0 | 2 | production |
| `modules/appointments/compose` | `database` | [new.go:38](../../../backend/modules/appointments/compose/new.go#L38) | 4 | 3 | 5 | production |
| `modules/careplan/compose` | `ownTenantTransaction` | [care_lifecycle.go:151](../../../backend/modules/careplan/compose/care_lifecycle.go#L151) | 1 | 0 | 3 | production |
| `modules/careplan/compose` | `ownTenantTransaction` | [care_lifecycle.go:152](../../../backend/modules/careplan/compose/care_lifecycle.go#L152) | 1 | 0 | 1 | production |
| `modules/careplan/compose` | `NewExcusedAbsenceRequests` | [excused_requests.go:62](../../../backend/modules/careplan/compose/excused_requests.go#L62) | 0 | 0 | 0 | production |
| `modules/careplan/compose` | `guardianPickupDatabase` | [guardian_pickup.go:29](../../../backend/modules/careplan/compose/guardian_pickup.go#L29) | 2 | 0 | 2 | production |
| `modules/careplan/compose` | `bindCareLocks` | [new.go:58](../../../backend/modules/careplan/compose/new.go#L58) | 5 | 2 | 4 | production |
| `modules/careplan/compose` | `carePlanDatabase` | [new.go:72](../../../backend/modules/careplan/compose/new.go#L72) | 2 | 0 | 3 | production |
| `modules/careplan/compose` | `observeOfferingBookings` | [offering_booking_metrics.go:31](../../../backend/modules/careplan/compose/offering_booking_metrics.go#L31) | 1 | 0 | 1 | production |
| `modules/careplan/compose` | `observeOfferingBookings` | [offering_booking_metrics.go:34](../../../backend/modules/careplan/compose/offering_booking_metrics.go#L34) | 6 | 4 | 14 | production |
| `modules/careplan/compose` | `newOfferingBookings` | [offering_bookings.go:20](../../../backend/modules/careplan/compose/offering_bookings.go#L20) | 7 | 3 | 5 | production |
| `modules/careplan/compose` | `RunInTenantTx` | [offering_changes.go:109](../../../backend/modules/careplan/compose/offering_changes.go#L109) | 1 | 0 | 1 | production |
| `modules/careplan/compose` | `LockWeeklyPickupStudents` | [pickup_excusal.go:45](../../../backend/modules/careplan/compose/pickup_excusal.go#L45) | 1 | 0 | 1 | production |
| `modules/careplan/compose` | `WithinTenant` | [pickup_excusal.go:62](../../../backend/modules/careplan/compose/pickup_excusal.go#L62) | 1 | 0 | 1 | production |
| `modules/careplan/compose` | `ListByAccount` | [repository_adapter_parent_portal_schools.go:56](../../../backend/modules/careplan/compose/repository_adapter_parent_portal_schools.go#L56) | 3 | 2 | 3 | production |
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
| `modules/dataimport/compose` | `NewOpeningReferences` | [opening_references.go:14](../../../backend/modules/dataimport/compose/opening_references.go#L14) | 11 | 6 | 10 | production |
| `modules/dataimport/compose` | `NewOpeningReferences` | [opening_references.go:41](../../../backend/modules/dataimport/compose/opening_references.go#L41) | 5 | 2 | 4 | production |
| `modules/dataimport/compose` | `NewOpeningReferences` | [opening_references.go:52](../../../backend/modules/dataimport/compose/opening_references.go#L52) | 5 | 2 | 4 | production |
| `modules/dataimport/compose` | `NewReferences` | [references.go:15](../../../backend/modules/dataimport/compose/references.go#L15) | 5 | 2 | 4 | production |
| `modules/dataimport/compose` | `NewReferences` | [references.go:26](../../../backend/modules/dataimport/compose/references.go#L26) | 5 | 2 | 4 | production |
| `modules/dataimport/compose` | `Observe` | [transactions.go:43](../../../backend/modules/dataimport/compose/transactions.go#L43) | 3 | 1 | 1 | production |
| `modules/delivery/compose` | `NewModule` | [new.go:50](../../../backend/modules/delivery/compose/new.go#L50) | 1 | 0 | 2 | production |
| `modules/delivery/compose` | `NewPushDispatcher` | [new.go:127](../../../backend/modules/delivery/compose/new.go#L127) | 3 | 1 | 3 | production |
| `modules/delivery/compose` | `NewPushAuthorizationChecker` | [provider.go:49](../../../backend/modules/delivery/compose/provider.go#L49) | 4 | 1 | 3 | production |
| `modules/delivery/compose` | `NewPushAuthorizationChecker` | [provider.go:51](../../../backend/modules/delivery/compose/provider.go#L51) | 3 | 0 | 1 | production |
| `modules/delivery/compose` | `NewExpiredPushSubscriptionCleaner` | [provider.go:78](../../../backend/modules/delivery/compose/provider.go#L78) | 1 | 0 | 2 | production |
| `modules/delivery/compose` | `NewExpiredPushSubscriptionCleaner` | [provider.go:79](../../../backend/modules/delivery/compose/provider.go#L79) | 5 | 0 | 1 | production |
| `modules/delivery/compose` | `NewTenantTransactions` | [tenant_transactions.go:17](../../../backend/modules/delivery/compose/tenant_transactions.go#L17) | 1 | 0 | 2 | production |
| `modules/delivery/compose` | `NewTenantTransactions` | [tenant_transactions.go:18](../../../backend/modules/delivery/compose/tenant_transactions.go#L18) | 1 | 0 | 1 | production |
| `modules/devicefleet/compose` | `New` | [new.go:107](../../../backend/modules/devicefleet/compose/new.go#L107) | 2 | 0 | 2 | production |
| `modules/devicefleet/compose` | `databaseRuntime` | [new.go:132](../../../backend/modules/devicefleet/compose/new.go#L132) | 4 | 1 | 7 | production |
| `modules/devicefleet/compose` | `AdminScope` | [new.go:160](../../../backend/modules/devicefleet/compose/new.go#L160) | 1 | 0 | 1 | production |
| `modules/devicefleet/compose` | `TenantScope` | [new.go:166](../../../backend/modules/devicefleet/compose/new.go#L166) | 1 | 0 | 1 | production |
| `modules/enrollment/compose` | `tenantRuntime` | [application.go:54](../../../backend/modules/enrollment/compose/application.go#L54) | 1 | 0 | 1 | production |
| `modules/enrollment/compose` | `tenantRuntime` | [application.go:56](../../../backend/modules/enrollment/compose/application.go#L56) | 2 | 0 | 1 | production |
| `modules/enrollment/compose` | `tenantRuntime` | [application.go:61](../../../backend/modules/enrollment/compose/application.go#L61) | 1 | 0 | 1 | production |
| `modules/enrollment/compose` | `tenantRuntime` | [application.go:64](../../../backend/modules/enrollment/compose/application.go#L64) | 1 | 0 | 3 | production |
| `modules/enrollment/compose` | `tenantRuntime` | [application.go:65](../../../backend/modules/enrollment/compose/application.go#L65) | 1 | 0 | 1 | production |
| `modules/enrollment/compose` | `tenantRuntime` | [application.go:67](../../../backend/modules/enrollment/compose/application.go#L67) | 1 | 0 | 3 | production |
| `modules/enrollment/compose` | `tenantRuntime` | [application.go:68](../../../backend/modules/enrollment/compose/application.go#L68) | 1 | 0 | 1 | production |
| `modules/enrollment/compose` | `tenantRuntime` | [application.go:73](../../../backend/modules/enrollment/compose/application.go#L73) | 1 | 0 | 2 | production |
| `modules/enrollment/compose` | `newStore` | [new.go:25](../../../backend/modules/enrollment/compose/new.go#L25) | 5 | 2 | 3 | production |
| `modules/enrollment/compose` | `newStore` | [new.go:35](../../../backend/modules/enrollment/compose/new.go#L35) | 3 | 1 | 2 | production |
| `modules/enrollment/compose` | `observeOfferingStorage` | [offering_storage_metrics.go:31](../../../backend/modules/enrollment/compose/offering_storage_metrics.go#L31) | 1 | 0 | 1 | production |
| `modules/enrollment/compose` | `observeOfferingStorage` | [offering_storage_metrics.go:34](../../../backend/modules/enrollment/compose/offering_storage_metrics.go#L34) | 6 | 5 | 14 | production |
| `modules/exporttransfer/compose` | `newModule` | [new.go:81](../../../backend/modules/exporttransfer/compose/new.go#L81) | 1 | 0 | 2 | production |
| `modules/exporttransfer/compose` | `database` | [new.go:93](../../../backend/modules/exporttransfer/compose/new.go#L93) | 7 | 3 | 5 | production |
| `modules/exporttransfer/compose` | `NewDispatchRecovery` | [recovery.go:16](../../../backend/modules/exporttransfer/compose/recovery.go#L16) | 5 | 2 | 3 | production |
| `modules/exporttransfer/compose` | `NewDispatchRecovery` | [recovery.go:27](../../../backend/modules/exporttransfer/compose/recovery.go#L27) | 1 | 0 | 3 | production |
| `modules/facilities/compose` | `observer` | [new.go:56](../../../backend/modules/facilities/compose/new.go#L56) | 2 | 0 | 2 | production |
| `modules/facilities/compose` | `databaseRuntime` | [new.go:63](../../../backend/modules/facilities/compose/new.go#L63) | 5 | 4 | 8 | production |
| `modules/facilities/compose` | `NewUnobserved` | [unobserved.go:16](../../../backend/modules/facilities/compose/unobserved.go#L16) | 1 | 0 | 0 | production |
| `modules/facilities/compose` | `NewUnobserved` | [unobserved.go:20](../../../backend/modules/facilities/compose/unobserved.go#L20) | 0 | 0 | 0 | production |
| `modules/feedback/compose` | `New` | [new.go:39](../../../backend/modules/feedback/compose/new.go#L39) | 7 | 3 | 5 | production |
| `modules/feedback/compose` | `New` | [new.go:54](../../../backend/modules/feedback/compose/new.go#L54) | 3 | 1 | 5 | production |
| `modules/feedback/compose` | `New` | [new.go:61](../../../backend/modules/feedback/compose/new.go#L61) | 2 | 0 | 2 | production |
| `modules/filestorage/compose` | `observer` | [new.go:163](../../../backend/modules/filestorage/compose/new.go#L163) | 2 | 0 | 2 | production |
| `modules/identityaccess/compose` | `WithTenantTx` | [account_authentication.go:104](../../../backend/modules/identityaccess/compose/account_authentication.go#L104) | 1 | 0 | 1 | production |
| `modules/identityaccess/compose` | `newAccountLifecycle` | [account_lifecycle.go:67](../../../backend/modules/identityaccess/compose/account_lifecycle.go#L67) | 1 | 0 | 0 | production |
| `modules/identityaccess/compose` | `NewCallerContext` | [caller_context.go:91](../../../backend/modules/identityaccess/compose/caller_context.go#L91) | 3 | 0 | 3 | production |
| `modules/identityaccess/compose` | `newParentRequestReview` | [caller_context.go:134](../../../backend/modules/identityaccess/compose/caller_context.go#L134) | 1 | 0 | 2 | production |
| `modules/identityaccess/compose` | `requestMemo` | [caller_context.go:160](../../../backend/modules/identityaccess/compose/caller_context.go#L160) | 3 | 2 | 1 | production |
| `modules/identityaccess/compose` | `ExpireDemoAccesses` | [demo_access.go:80](../../../backend/modules/identityaccess/compose/demo_access.go#L80) | 3 | 0 | 3 | production |
| `modules/identityaccess/compose` | `NewGuardianWelcomeOrder` | [guardian_welcome_order.go:38](../../../backend/modules/identityaccess/compose/guardian_welcome_order.go#L38) | 7 | 3 | 7 | production |
| `modules/identityaccess/compose` | `ClaimGuardianEnrollments` | [mail_transactions.go:56](../../../backend/modules/identityaccess/compose/mail_transactions.go#L56) | 3 | 0 | 1 | production |
| `modules/identityaccess/compose` | `NewInvitationMaintenance` | [maintenance.go:26](../../../backend/modules/identityaccess/compose/maintenance.go#L26) | 0 | 0 | 0 | production |
| `modules/identityaccess/compose` | `NewGuardianSeedAccess` | [maintenance.go:37](../../../backend/modules/identityaccess/compose/maintenance.go#L37) | 0 | 0 | 0 | production |
| `modules/identityaccess/compose` | `AccountExists` | [maintenance.go:48](../../../backend/modules/identityaccess/compose/maintenance.go#L48) | 3 | 1 | 2 | production |
| `modules/identityaccess/compose` | `NewSessionMaintenance` | [maintenance.go:91](../../../backend/modules/identityaccess/compose/maintenance.go#L91) | 0 | 0 | 0 | production |
| `modules/identityaccess/compose` | `NewSessionMaintenance` | [maintenance.go:98](../../../backend/modules/identityaccess/compose/maintenance.go#L98) | 1 | 0 | 0 | production |
| `modules/identityaccess/compose` | `New` | [new.go:81](../../../backend/modules/identityaccess/compose/new.go#L81) | 2 | 0 | 2 | production |
| `modules/identityaccess/compose` | `bindTenantRuntime` | [new.go:163](../../../backend/modules/identityaccess/compose/new.go#L163) | 1 | 0 | 0 | production |
| `modules/identityaccess/compose` | `bindTenantRuntime` | [new.go:168](../../../backend/modules/identityaccess/compose/new.go#L168) | 1 | 0 | 1 | production |
| `modules/identityaccess/compose` | `newCommandTransactions` | [new.go:184](../../../backend/modules/identityaccess/compose/new.go#L184) | 1 | 0 | 0 | production |
| `modules/identityaccess/compose` | `newStore` | [store.go:13](../../../backend/modules/identityaccess/compose/store.go#L13) | 1 | 0 | 3 | production |
| `modules/identityaccess/compose` | `newStore` | [store.go:16](../../../backend/modules/identityaccess/compose/store.go#L16) | 2 | 0 | 3 | production |
| `modules/mealplan/compose` | `New` | [new.go:41](../../../backend/modules/mealplan/compose/new.go#L41) | 7 | 3 | 5 | production |
| `modules/organizationtenancy/compose` | `NewBilling` | [billing.go:43](../../../backend/modules/organizationtenancy/compose/billing.go#L43) | 5 | 2 | 3 | production |
| `modules/organizationtenancy/compose` | `NewDemoSchoolOrders` | [demo_queue.go:74](../../../backend/modules/organizationtenancy/compose/demo_queue.go#L74) | 5 | 2 | 3 | production |
| `modules/organizationtenancy/compose` | `New` | [new.go:27](../../../backend/modules/organizationtenancy/compose/new.go#L27) | 5 | 2 | 3 | production |
| `modules/organizationtenancy/compose` | `New` | [new.go:38](../../../backend/modules/organizationtenancy/compose/new.go#L38) | 2 | 0 | 2 | production |
| `modules/organizationtenancy/compose` | `OperatorDatabase` | [provisioning.go:92](../../../backend/modules/organizationtenancy/compose/provisioning.go#L92) | 5 | 2 | 3 | production |
| `modules/peopledirectory/compose` | `inSchool` | [caregiver_capability_views.go:26](../../../backend/modules/peopledirectory/compose/caregiver_capability_views.go#L26) | 1 | 0 | 2 | production |
| `modules/peopledirectory/compose` | `inSchool` | [caregiver_capability_views.go:27](../../../backend/modules/peopledirectory/compose/caregiver_capability_views.go#L27) | 1 | 0 | 1 | production |
| `modules/peopledirectory/compose` | `LoadForTenant` | [guardian_profile_loader.go:45](../../../backend/modules/peopledirectory/compose/guardian_profile_loader.go#L45) | 4 | 1 | 1 | production |
| `modules/peopledirectory/compose` | `WithTenantTx` | [guardian_service.go:30](../../../backend/modules/peopledirectory/compose/guardian_service.go#L30) | 1 | 0 | 1 | production |
| `modules/peopledirectory/compose` | `NewWithGuardianMemberships` | [new.go:100](../../../backend/modules/peopledirectory/compose/new.go#L100) | 3 | 0 | 5 | production |
| `modules/peopledirectory/compose` | `NewWithGuardianMemberships` | [new.go:106](../../../backend/modules/peopledirectory/compose/new.go#L106) | 2 | 0 | 2 | production |
| `modules/peopledirectory/compose` | `MustNewReads` | [new.go:187](../../../backend/modules/peopledirectory/compose/new.go#L187) | 0 | 0 | 0 | production |
| `modules/peopledirectory/compose` | `NewMessageableStaff` | [repository_adapter_messageable.go:41](../../../backend/modules/peopledirectory/compose/repository_adapter_messageable.go#L41) | 5 | 2 | 5 | production |
| `modules/peopledirectory/compose` | `defaultGuardianRole` | [repository_adapter_people.go:87](../../../backend/modules/peopledirectory/compose/repository_adapter_people.go#L87) | 1 | 0 | 1 | production |
| `modules/requestreview/compose` | `Open` | [excused_queue.go:32](../../../backend/modules/requestreview/compose/excused_queue.go#L32) | 1 | 0 | 1 | production |
| `modules/schoolcalendar/compose` | `NewFeedHistory` | [feed_history.go:14](../../../backend/modules/schoolcalendar/compose/feed_history.go#L14) | 2 | 0 | 2 | production |
| `modules/schoolcalendar/compose` | `PersistenceRuntimeFor` | [new.go:34](../../../backend/modules/schoolcalendar/compose/new.go#L34) | 3 | 1 | 2 | production |
| `modules/schoolcalendar/compose` | `newService` | [new.go:106](../../../backend/modules/schoolcalendar/compose/new.go#L106) | 3 | 0 | 4 | production |
| `modules/schoolcalendar/compose` | `newService` | [new.go:111](../../../backend/modules/schoolcalendar/compose/new.go#L111) | 2 | 0 | 2 | production |
| `modules/schoolcalendar/portal/compose` | `WithinTenant` | [tenant_runtime.go:41](../../../backend/modules/schoolcalendar/portal/compose/tenant_runtime.go#L41) | 1 | 0 | 1 | production |
| `modules/schoolcalendar/portal/compose` | `WithinAdmin` | [tenant_runtime.go:46](../../../backend/modules/schoolcalendar/portal/compose/tenant_runtime.go#L46) | 1 | 0 | 1 | production |
| `modules/schoolmembership/compose` | `StudentClassWriteGate` | [student_class_gate.go:21](../../../backend/modules/schoolmembership/compose/student_class_gate.go#L21) | 1 | 0 | 1 | production |
| `modules/settings/compose` | `NewAnalyseFreigabe` | [analyse_freigabe.go:16](../../../backend/modules/settings/compose/analyse_freigabe.go#L16) | 3 | 1 | 5 | production |
| `modules/settings/compose` | `Schema` | [operator.go:67](../../../backend/modules/settings/compose/operator.go#L67) | 3 | 0 | 1 | production |
| `modules/settings/compose` | `Reveal` | [operator.go:80](../../../backend/modules/settings/compose/operator.go#L80) | 3 | 0 | 1 | production |
| `modules/settings/compose` | `WithinTenant` | [operator.go:91](../../../backend/modules/settings/compose/operator.go#L91) | 1 | 0 | 1 | production |
| `modules/settings/compose` | `WithinTenant` | [tenant_settings_runtime.go:59](../../../backend/modules/settings/compose/tenant_settings_runtime.go#L59) | 1 | 0 | 1 | production |
| `modules/settings/compose` | `WithinAdmin` | [tenant_settings_runtime.go:68](../../../backend/modules/settings/compose/tenant_settings_runtime.go#L68) | 1 | 0 | 1 | production |
| `modules/studentpresence/compose` | `FailureLogger` | [observers.go:8](../../../backend/modules/studentpresence/compose/observers.go#L8) | 1 | 1 | 2 | production |
| `modules/studentpresence/compose` | `DebugLogger` | [observers.go:21](../../../backend/modules/studentpresence/compose/observers.go#L21) | 1 | 0 | 1 | production |
| `modules/timetable/compose` | `reportMirrorPanic` | [attendance_mirror_report.go:31](../../../backend/modules/timetable/compose/attendance_mirror_report.go#L31) | 2 | 1 | 4 | production |
| `modules/timetable/compose` | `Upsert` | [class_arrival_exceptions_test_helpers.go:27](../../../backend/modules/timetable/compose/class_arrival_exceptions_test_helpers.go#L27) | 1 | 0 | 1 | test-only |
| `modules/timetable/compose` | `SubstituteDayLock` | [instance_lifecycle_ports.go:24](../../../backend/modules/timetable/compose/instance_lifecycle_ports.go#L24) | 1 | 0 | 3 | production |
| `modules/timetable/compose` | `New` | [new.go:42](../../../backend/modules/timetable/compose/new.go#L42) | 2 | 0 | 2 | production |
| `modules/timetable/compose` | `New` | [new.go:47](../../../backend/modules/timetable/compose/new.go#L47) | 1 | 0 | 2 | production |
| `modules/timetable/compose` | `databaseRuntime` | [new.go:52](../../../backend/modules/timetable/compose/new.go#L52) | 5 | 4 | 8 | production |
| `modules/timetable/compose` | `NewOffboarding` | [offboarding.go:24](../../../backend/modules/timetable/compose/offboarding.go#L24) | 2 | 0 | 2 | production |
| `modules/timetable/compose` | `NewOffboarding` | [offboarding.go:29](../../../backend/modules/timetable/compose/offboarding.go#L29) | 2 | 0 | 3 | production |
| `modules/timetable/compose` | `NewOffboarding` | [offboarding.go:33](../../../backend/modules/timetable/compose/offboarding.go#L33) | 2 | 0 | 3 | production |
| `modules/timetable/compose` | `OrderPlanningTracks` | [planning_track_administration.go:22](../../../backend/modules/timetable/compose/planning_track_administration.go#L22) | 1 | 0 | 2 | production |
| `modules/timetable/compose` | `OrderPlanningTracks` | [planning_track_administration.go:23](../../../backend/modules/timetable/compose/planning_track_administration.go#L23) | 1 | 0 | 1 | production |
| `modules/timetable/compose` | `InTenant` | [tenant_transactions.go:16](../../../backend/modules/timetable/compose/tenant_transactions.go#L16) | 1 | 0 | 1 | production |
| `modules/transactionruntime/compose` | `OpenServeDatabase` | [database.go:39](../../../backend/modules/transactionruntime/compose/database.go#L39) | 2 | 0 | 2 | production |
| `modules/transactionruntime/compose` | `NewDemoTenantRuntime` | [tenant_runtime.go:30](../../../backend/modules/transactionruntime/compose/tenant_runtime.go#L30) | 0 | 0 | 0 | production |
| `modules/transactionruntime/compose` | `NewDemoTenantRuntime` | [tenant_runtime.go:31](../../../backend/modules/transactionruntime/compose/tenant_runtime.go#L31) | 1 | 0 | 0 | production |
| `modules/transactionruntime/compose` | `TenantCallback` | [tenant_runtime.go:66](../../../backend/modules/transactionruntime/compose/tenant_runtime.go#L66) | 1 | 0 | 1 | production |
| `modules/workforce/compose` | `newApplication` | [new.go:62](../../../backend/modules/workforce/compose/new.go#L62) | 2 | 0 | 2 | production |
| `modules/workforce/compose` | `databaseRuntime` | [new.go:84](../../../backend/modules/workforce/compose/new.go#L84) | 3 | 0 | 5 | production |
| `modules/workforce/compose` | `NewOffboarding` | [offboarding.go:22](../../../backend/modules/workforce/compose/offboarding.go#L22) | 2 | 1 | 3 | production |
| `modules/workforce/compose` | `NewOffboarding` | [offboarding.go:28](../../../backend/modules/workforce/compose/offboarding.go#L28) | 2 | 0 | 3 | production |
| `modules/workforce/compose` | `NewOffboarding` | [offboarding.go:32](../../../backend/modules/workforce/compose/offboarding.go#L32) | 2 | 0 | 5 | production |
| `modules/workforce/compose` | `NewOffboarding` | [offboarding.go:33](../../../backend/modules/workforce/compose/offboarding.go#L33) | 1 | 0 | 2 | production |
| `modules/workforce/compose` | `NewStaffShiftLock` | [staff_shift_lock.go:22](../../../backend/modules/workforce/compose/staff_shift_lock.go#L22) | 1 | 0 | 1 | production |
| `modules/workforce/compose` | `NewStaffAdmin` | [staffadmin.go:49](../../../backend/modules/workforce/compose/staffadmin.go#L49) | 1 | 1 | 2 | production |
| `modules/workforce/compose` | `NewStaffEmployment` | [staffemployment.go:24](../../../backend/modules/workforce/compose/staffemployment.go#L24) | 0 | 0 | 0 | production |
| `modules/workforce/compose` | `staffAllowanceTestDatabase` | [time_tracking_test_helpers.go:51](../../../backend/modules/workforce/compose/time_tracking_test_helpers.go#L51) | 4 | 2 | 3 | test-only |
| `workflows/openroommove/compose` | `New` | [new.go:47](../../../backend/workflows/openroommove/compose/new.go#L47) | 1 | 0 | 1 | production |
