# People Directory contract cluster

Inventory base: `a52dccfc01e977e18c112d164949b819eda552f3` (development, including Timetable PR #4324). Parsed Go imports and selector expressions, not text matches. Production, internal tests and external tests are separate scopes. Composition vocabulary imports are included.

## Outcome

- Foreign consumers use native People Directory values and read/command ports. Care withdrawal, companion edges and data-change request records use Care Plan values. Native snapshots already published by an owner are reused, not cloned.
- Care Plan and Student Presence test consumers no longer import `models/activities` or `models/schedule`. Existing Timetable grants are reused. School Calendar fixtures are bound by root functions from existing database values; no root composition field was added.
- Cross-owner Care Plan suites formerly in People Directory contracttest now live in Care Plan contracttest. Student deletion/retention suites live beside their workflow. Expected values, assertions and scenarios are retained; ORM fixture writes and JSON/date/clock projections adapt the native field representations.
- Pure guest/staff/teacher tests move to People Directory public values; optional-phone tests move to its contact package. Obsolete legacy methods, codecs and unused test-double stubs are deleted. Historical migration evidence paths follow the moved tests.
- `modules/peopledirectory/compose/vocabulary` is deleted: 88 foreign aliases removed. No replacement aliases, allowances, rules or checker changes.
- `database/repositories/users` was already relocated on the inventory base to People Directory’s `internal/adapters/peoplepostgres`, package `users`. It does not exist on either side of this change.

## Retained dissolution blocker

`backend/models/entity_interfaces_test.go` is retained unchanged. Its eight `base.Entity` compile-time checks use Guest, GuardianProfile, Person, PrivacyConsent, Staff, Student, StudentGuardian and Teacher. The `models` internal-test scope is legacy-composition and cannot import People Directory public. Rehosting in People Directory postgres would require its forbidden dependency on `models/base`. Neither route has an existing predecessor rule permitting a same-or-narrower successor; a legacy exception is not a grant. No rule or exception was added.

Per the agreed fallback, only dissolution is deferred: `models/users` remains, with the original `models` → `models/users` and `models` → `models/base` keys. The SQL adapter and owner-local model tests retain private row usage. All foreign consumer changes remain. The sole remaining foreign importer of `models/users` is this unchanged test. The package was not relabelled as a port.

## Foreign users/vocabulary consumers at the base

122 importing files across 7 package/scope groups. The current base has 15 Care Plan files, rather than the earlier attempt’s 18.

### `api` (root-composition, internal_test)

| File | Imported surface | Symbols |
|---|---|---|
| `communication_staff_sender_test.go` | `modules/peopledirectory/compose/vocabulary` | `Person` |
| `emergency_snapshot_test.go` | `modules/peopledirectory/compose/vocabulary` | `GuardianEmergencyContactRow`, `GuardianPhoneNumber`, `Student` |
| `student_consent_composition_test.go` | `modules/peopledirectory/compose/vocabulary` | `Student` |

### `api` (root-composition, production)

| File | Imported surface | Symbols |
|---|---|---|
| `attendance_education_groups.go` | `modules/peopledirectory/compose/vocabulary` | `Student` |
| `attendance_route_identity.go` | `modules/peopledirectory/compose/vocabulary` | `Person`, `Staff`, `Student`, `Teacher` |
| `attendance_staff.go` | `modules/peopledirectory/compose/vocabulary` | `Staff` |
| `attendance_staff_names.go` | `modules/peopledirectory/compose/vocabulary` | `Person`, `Staff` |
| `birthday_composition.go` | `modules/peopledirectory/compose/vocabulary` | `BirthdayEntry`, `MonthDay`, `Person`, `Staff` |
| `calendar_facts.go` | `modules/peopledirectory/compose/vocabulary` | `GuardianProfile`, `Person`, `Staff`, `Student`, `StudentGuardian`, `StudentStatusActive`, `StudentWithGroupInfo` |
| `care_companion_students.go` | `modules/peopledirectory/compose/vocabulary` | `AccompaniedWeekdays`, `Student` |
| `care_exit_enrollment.go` | `modules/peopledirectory/compose/vocabulary` | `CalendarDate` |
| `care_lifecycle_owners.go` | `modules/peopledirectory/compose/vocabulary` | `CalendarDate`, `Student` |
| `care_lifecycle_repository_test_helpers.go` | `modules/peopledirectory/compose/vocabulary` | `CalendarDate`, `CareWithdrawalCompletionRepository`, `CompanionWeekdayKeys`, `ErrCompanionInvalidWeekday`, `ErrCompanionSelfLink`, `ErrCompanionStudentIDRequired`, `StudentCompanion` |
| `care_plan_booking_materialization.go` | `modules/peopledirectory/compose/vocabulary` | `Person` |
| `care_plan_care_request_service.go` | `modules/peopledirectory/compose/vocabulary` | `Student` |
| `care_plan_documents.go` | `modules/peopledirectory/compose/vocabulary` | `Student` |
| `care_plan_excused_request_students.go` | `modules/peopledirectory/compose/vocabulary` | `Person`, `Student` |
| `care_plan_graph_wiring.go` | `modules/peopledirectory/compose/vocabulary` | `Student` |
| `care_plan_pickup_adjustments.go` | `modules/peopledirectory/compose/vocabulary` | `Student` |
| `care_plan_request_adapters.go` | `modules/peopledirectory/compose/vocabulary` | `ErrChangeRequestNotDecided`, `ErrChangeRequestNotFound`, `ErrChangeRequestNotPending` |
| `care_plan_request_decision_adapter.go` | `modules/peopledirectory/compose/vocabulary` | `Student` |
| `care_plan_request_people.go` | `modules/peopledirectory/compose/vocabulary` | `ErrCompanionLockBusy`, `ErrCompanionWouldLoseDeparture`, `ErrStudentRowMissing`, `OptionalCalendarDate`, `Student`, `StudentStatus` |
| `care_plan_student_record_adapters.go` | `modules/peopledirectory/compose/vocabulary` | `CompanionLink`, `ErrCompanionInvalidWeekday`, `Student`, `StudentCompanion`, `StudentStatus`, `StudentStatusActive`, `StudentStatusAlumnus`, `StudentStatusInactive`, `StudentStatusPending` |
| `care_plan_weekly_approval_adapter.go` | `modules/peopledirectory/compose/vocabulary` | `AllowedDepartureModes`, `ContextWithCompanionChangeRecorder`, `DepartureMode` |
| `care_students.go` | `modules/peopledirectory/compose/vocabulary` | `CalendarDate`, `Person`, `RenderCalendarDate`, `Student` |
| `care_withdrawal_completions.go` | `modules/peopledirectory/compose/vocabulary` | `CalendarDate`, `CareWithdrawalCompletion`, `CareWithdrawalCompletionFilter`, `CareWithdrawalCompletionRepository`, `Student` |
| `care_withdrawal_test_helpers.go` | `modules/peopledirectory/compose/vocabulary` | `CareExitSourceOffering`, `CareWithdrawalCompletion`, `CareWithdrawalCompletionRepository`, `CareWithdrawalStatePending` |
| `caregiver_blocker_adapters.go` | `modules/peopledirectory/compose/vocabulary` | `BlockerSupervision` |
| `class_day_composition.go` | `modules/peopledirectory/compose/vocabulary` | `GuardianEmergencyContactRow`, `Person`, `Student` |
| `communication_parent_message_people.go` | `modules/peopledirectory/compose/vocabulary` | `GuardianProfile`, `Person`, `Student` |
| `communication_staff_sender.go` | `modules/peopledirectory/compose/vocabulary` | `Person` |
| `delivery_staff_recipients.go` | `modules/peopledirectory/compose/vocabulary` | `Student` |
| `device_scan_people.go` | `modules/peopledirectory/compose/vocabulary` | `Person`, `Staff`, `Student`, `StudentWithGroupInfo`, `Teacher` |
| `education_service_ports.go` | `modules/peopledirectory/compose/vocabulary` | `ActiveCaregiver`, `Guest`, `Staff` |
| `education_teachers.go` | `modules/peopledirectory/compose/vocabulary` | `Teacher` |
| `emergency_snapshot.go` | `modules/peopledirectory/compose/vocabulary` | `GuardianEmergencyContactRow`, `Student` |
| `enrollment_decision_composition.go` | `modules/peopledirectory/compose/vocabulary` | `ErrCompanionLockBusy`, `ErrCompanionWouldLoseDeparture`, `ErrStudentGuardianNotFound` |
| `enrollment_decision_people.go` | `modules/peopledirectory/compose/vocabulary` | `DeparturePlanSnapshot`, `ErrGuardianProfileNotFound`, `GuardianPhoneNumber`, `GuardianProfile`, `Person`, `PhoneType`, `Student`, `StudentGuardian`, `StudentStatus` |
| `enrollment_flow_repository_test_helpers.go` | `modules/peopledirectory/compose/vocabulary` | `CareWithdrawalCompletionRepository` |
| `enrollment_intake_composition.go` | `modules/peopledirectory/compose/vocabulary` | `ErrCompanionLockBusy` |
| `enrollment_rows.go` | `modules/peopledirectory/compose/vocabulary` | `CareWithdrawalCompletion`, `CareWithdrawalCompletionFilter`, `GuardianEmergencyContactRow`, `GuardianPhoneNumber`, `GuardianProfile`, `Person`, `Staff`, `Student`, `StudentGuardian` |
| `excused_requests_test_helpers.go` | `modules/peopledirectory/compose/vocabulary` | `ParentRequestEventRepository` |
| `group_projection.go` | `modules/peopledirectory/compose/vocabulary` | `BirthdayEntry`, `MonthDay`, `StudentWithGroupInfo` |
| `guardian_repository_test_helpers.go` | `modules/peopledirectory/compose/vocabulary` | `GuardianFinancialDataRepository` |
| `identity_account_lifecycle.go` | `modules/peopledirectory/compose/vocabulary` | `GuardianProfile`, `Person`, `Staff`, `StudentGuardian` |
| `identity_caller_rows.go` | `modules/peopledirectory/compose/vocabulary` | `Staff`, `Student`, `Teacher` |
| `identity_rows.go` | `modules/peopledirectory/compose/vocabulary` | `GuardianProfile`, `Person`, `Staff`, `Student`, `StudentGuardian`, `Teacher` |
| `master_data_decisions.go` | `modules/peopledirectory/compose/vocabulary` | `AllowedDepartureModes`, `DepartureMode`, `Person`, `Student` |
| `master_data_decisions_test_helpers.go` | `modules/peopledirectory/compose/vocabulary` | `ParentRequestEventRepository` |
| `offering_change_composition.go` | `modules/peopledirectory/compose/vocabulary` | `CareWithdrawalCompletionFilter`, `ParentRequestEvent`, `Student` |
| `operator_provisioning_adapters.go` | `modules/peopledirectory/compose/vocabulary` | `Person`, `Staff` |
| `operator_settings_test_helpers.go` | `modules/peopledirectory/compose/vocabulary` | `CareWithdrawalCompletionRepository` |
| `owner_roots.go` | `modules/peopledirectory/compose/vocabulary` | `PersonAccount` |
| `parent_people_fixtures_test_helpers.go` | `modules/peopledirectory/compose/vocabulary` | `GuardianProfile`, `ParentRequestEventRepository` |
| `parent_request_review_policy.go` | `modules/peopledirectory/compose/vocabulary` | `Student` |
| `parent_request_review_test_helpers.go` | `modules/peopledirectory/compose/vocabulary` | `Student` |
| `parent_store_values_test_helpers.go` | `modules/peopledirectory/compose/vocabulary` | `ParentMessageSenderGuardian` |
| `people_bindings.go` | `modules/peopledirectory/compose/vocabulary` | `EnrolledOn`, `Student`, `StudentStatus` |
| `people_read_ports.go` | `modules/peopledirectory/compose/vocabulary` | `Person`, `Student` |
| `people_repository_test_helpers.go` | `modules/peopledirectory/compose/vocabulary` | `CareWithdrawalCompletionRepository`, `GuardianFinancialDataRepository`, `StudentCompanion`, `StudentGuardian` |
| `plan_export_staff_names.go` | `modules/peopledirectory/compose/vocabulary` | `Staff` |
| `reminder_people.go` | `modules/peopledirectory/compose/vocabulary` | `Person`, `Student` |
| `service_graph_wiring.go` | `modules/peopledirectory/compose/vocabulary` | `BlockerGroup`, `Person`, `Student`, `StudentCompanion`, `StudentConsentState`, `StudentStatus` |
| `staff_admin_composition.go` | `modules/peopledirectory/compose/vocabulary` | `Person`, `Staff` |
| `staff_membership_adapters.go` | `modules/peopledirectory/compose/vocabulary` | `BirthdayEntry`, `BirthdayKindStaff`, `ErrPersonnelNumberConflict`, `MonthDay`, `Person`, `Staff`, `StaffWithRoleInfo` |
| `staff_membership_guest.go` | `modules/peopledirectory/compose/vocabulary` | `Guest` |
| `staff_membership_models.go` | `modules/peopledirectory/compose/vocabulary` | `Staff`, `Teacher` |
| `staff_membership_teacher.go` | `modules/peopledirectory/compose/vocabulary` | `ActiveCaregiver`, `Staff`, `Teacher` |
| `staff_projection_education.go` | `modules/peopledirectory/compose/vocabulary` | `BlockerSubstitution` |
| `statistics_inputs.go` | `modules/peopledirectory/compose/vocabulary` | `EnrolledOn`, `StudentWithGroupInfo` |
| `status_day_students_test_helpers.go` | `modules/peopledirectory/compose/vocabulary` | `Student` |
| `student_audit_composition.go` | `modules/peopledirectory/compose/vocabulary` | `Student`, `StudentStatus` |
| `student_consent_composition.go` | `modules/peopledirectory/compose/vocabulary` | `Student`, `StudentConsentState` |
| `student_data_request_fixture_test_helpers.go` | `modules/peopledirectory/compose/vocabulary` | `DataChangeStatusApproved`, `DataChangeStatusAutoApplied`, `DataChangeStatusPending`, `DataChangeStatusRejected`, `StudentDataChangeRequest` |
| `student_http_test_helpers.go` | `modules/peopledirectory/compose/vocabulary` | `CareWithdrawalCompletion`, `CareWithdrawalCompletionFilter`, `CareWithdrawalCompletionRepository` |
| `student_photo_composition.go` | `modules/peopledirectory/compose/vocabulary` | `Student` |
| `student_records.go` | `modules/peopledirectory/compose/vocabulary` | `Person`, `Student` |
| `student_repository_test_helpers.go` | `modules/peopledirectory/compose/vocabulary` | `CareWithdrawalCompletionRepository`, `ParentRequestEventRepository` |
| `student_root_reads.go` | `modules/peopledirectory/compose/vocabulary` | `Student`, `StudentStatus`, `StudentWithGroupInfo` |
| `teaching_assignment_adapters.go` | `modules/peopledirectory/compose/vocabulary` | `BlockerGroup` |
| `time_tracking_absence_email.go` | `modules/peopledirectory/compose/vocabulary` | `StaffWithRoleInfo` |
| `time_tracking_ports.go` | `modules/peopledirectory/compose/vocabulary` | `Staff` |
| `time_tracking_staff.go` | `modules/peopledirectory/compose/vocabulary` | `Staff` |
| `timetable_people_views.go` | `modules/peopledirectory/compose/vocabulary` | `Person`, `Staff`, `Student` |
| `workforce_admin_capabilities.go` | `modules/peopledirectory/compose/vocabulary` | `Person`, `Staff` |
| `workforce_plan_staff.go` | `modules/peopledirectory/compose/vocabulary` | `Staff` |

### `models` (legacy-composition, internal_test)

| File | Imported surface | Symbols |
|---|---|---|
| `entity_interfaces_test.go` | `models/users` | `GuardianProfile`, `Guest`, `Person`, `PrivacyConsent`, `Staff`, `Student`, `StudentGuardian`, `Teacher` |

### `modules/careplan/contracttest` (care-plan, external_test)

| File | Imported surface | Symbols |
|---|---|---|
| `care_request_alumni_test.go` | `models/users` | `StudentStatusAlumnus` |
| `care_request_co_guardian_notice_test.go` | `models/users` | `Student` |
| `care_request_service_test.go` | `models/users` | `AllowedDepartureModes`, `DepartureAccompanied`, `DepartureBus`, `DepartureMode`, `DeparturePickup`, `ParentMessageEventRequestCreated`, `ParentMessageEventRequestStatus`, `ParentMessageRequestStatusRejected`, `PickupDayOrder`, `Student`, `StudentCompanion` |
| `enrollment_decision_env_test.go` | `models/users` | `Student` |
| `lifecycle_care_booking_authority_integration_test.go` | `models/users` | `CareWithdrawalCompletion`, `CareWithdrawalCompletionFilter`, `CareWithdrawalCompletionRepository`, `CareWithdrawalObsoleteRebooked`, `CareWithdrawalStateObsolete`, `CareWithdrawalTriggerBookingExpired`, `CareWithdrawalTriggerDirectSchool`, `StudentStatusAlumnus`, `StudentStatusInactive` |
| `lifecycle_care_lifecycle_service_test.go` | `models/users` | `Student`, `StudentStatusActive`, `StudentStatusAlumnus`, `StudentStatusInactive`, `StudentStatusPending` |
| `lifecycle_care_withdrawal_lifecycle_test.go` | `models/users` | `CareWithdrawalCompletion`, `CareWithdrawalTriggerDirectSchool`, `StudentStatusActive` |
| `lifecycle_student_companion_service_test.go` | `models/users` | `AccompaniedWeekdays`, `AllowedDepartureModes`, `DepartureAccompanied`, `DepartureMode`, `ErrDepartureCompanionNoteRequired`, `Student`, `WithAccompaniedDays` |
| `lifecycle_student_document_service_test.go` | `models/users` | `Staff` |
| `master_data_corrections_test.go` | `models/users` | `AllowedDepartureModes`, `DataChangeStatusAutoApplied`, `DataChangeStatusPending`, `DataChangeStatusRejected`, `DataChangeTargetDeparture`, `DataChangeTargetPerson`, `DepartureAccompanied`, `DepartureBus`, `DepartureMode`, `PickupDayTuesday`, `StudentCompanion`, `StudentDataChangeRequest`, `StudentStatusAlumnus` |
| `master_data_decisions_test.go` | `models/users` | `AllowedDepartureModes`, `DataChangeStatusApproved`, `DataChangeStatusPending`, `DataChangeStatusRejected`, `DataChangeTargetDeparture`, `DataChangeTargetPerson`, `DataChangeTargetStudent`, `DepartureBus`, `DepartureMode`, `DeparturePickup`, `ParentMessageEventRequestStatus`, `ParentMessageRequestStatusDone`, `ParentMessageRequestStatusRejected`, `ParentMessageSenderStaff`, `Person`, `PickupDayMonday`, `PickupDayTuesday`, `PickupDayWednesday`, `StudentDataChangeRequest` |
| `native_request_people_test.go` | `models/users` | `AllowedDepartureModes`, `BusDays`, `DepartureBus`, `DepartureMode`, `DeparturePickup`, `PickupDays`, `Student` |
| `offering_adjustment_complete_withdrawal_test.go` | `models/users` | `CareExitReasonNoCareNeed`, `CareWithdrawalCompletion`, `CareWithdrawalCompletionFilter`, `CareWithdrawalCompletionRepository` |
| `offering_change_full_withdrawal_test.go` | `models/users` | `CareWithdrawalCompletionFilter` |
| `offering_change_request_service_test.go` | `models/users` | `CareWithdrawalCompletion`, `CareWithdrawalTriggerDirectSchool` |

### `modules/enrollment/http` (enrollment, external_test)

| File | Imported surface | Symbols |
|---|---|---|
| `change_request_review_list_router_test.go` | `models/users` | `Person` |
| `change_request_service_test.go` | `models/users` | `GuardianProfile` |
| `decision_approval_rollback_test.go` | `models/users` | `StudentStatusInactive` |
| `decision_child_quota_router_test.go` | `models/users` | `StudentStatus`, `StudentStatusActive`, `StudentStatusInactive` |
| `decision_class_switch_test.go` | `models/users` | `StudentStatusActive`, `StudentStatusInactive`, `StudentStatusPending` |
| `decision_companion_sync_test.go` | `models/users` | `AccompaniedWeekdays`, `AllowedDepartureModes`, `DepartureAccompanied`, `DepartureMode`, `ErrCompanionWouldLoseDeparture`, `PickupDayTuesday`, `StudentCompanion` |
| `decision_service_test.go` | `models/users` | `BusDayMonday`, `CareWithdrawalCompletion`, `CareWithdrawalTriggerDirectSchool`, `DepartureAccompanied`, `DepartureBus`, `DeparturePickup`, `GuardianProfile`, `PickupDayMonday`, `PickupDayWednesday`, `PickupStatusAccompanied`, `Student`, `StudentGuardian`, `StudentStatusActive`, `StudentStatusPending` |
| `deletion_service_integration_test.go` | `models/users` | `GuardianProfile` |
| `flow_fixture_helpers_test.go` | `models/users` | `CareWithdrawalCompletion`, `CareWithdrawalCompletionFilter`, `CareWithdrawalCompletionRepository`, `StudentGuardian` |
| `phase_expiry_decision_integration_test.go` | `models/users` | `Student`, `StudentStatusInactive`, `StudentStatusPending` |
| `phase_expiry_projection_test.go` | `models/users` | `StudentStatus`, `StudentStatusActive`, `StudentStatusInactive`, `StudentStatusPending` |
| `phase_expiry_successor_projection_test.go` | `models/users` | `StudentStatusActive` |
| `rollover_autoapprove_integration_test.go` | `models/users` | `Person`, `Student`, `StudentStatusActive`, `StudentStatusInactive`, `StudentStatusPending` |
| `rollover_child_quota_test.go` | `models/users` | `StudentStatusActive`, `StudentStatusInactive` |
| `status_takeover_lock_router_test.go` | `models/users` | `GuardianProfile`, `Person` |

### `modules/timetable/compose` (timetable, production)

| File | Imported surface | Symbols |
|---|---|---|
| `caregiver_blockers.go` | `models/users` | `BlockerActivity` |
| `legacy_enrollment_operations.go` | `models/users` | `Person`, `Student`, `StudentStatusAlumnus` |
| `legacy_interface.go` | `models/users` | `Student` |
| `roster_students.go` | `models/users` | `Student` |

### `modules/timetable/compose/httpadapter` (timetable, production)

| File | Imported surface | Symbols |
|---|---|---|
| `http.go` | `models/users` | `Staff`, `Teacher` |

## Last foreign Timetable test consumers at the base


### `modules/careplan/contracttest` (external_test)

| File | Imported surface | Symbols |
|---|---|---|
| `activity_date_test.go` | `models/activities` | `Date` |
| `arrival_baseline_service_test.go` | `models/schedule` | `ArrivalScheduleSourceClassSchedule`, `ArrivalScheduleSourceStaff`, `WeekdayMonday`, `WeekdayTuesday`, `WeekdayWednesday` |
| `arrival_class_bulk_test.go` | `models/schedule` | `ArrivalScheduleSourceClassSchedule`, `ArrivalScheduleSourceStaff`, `StudentArrivalSchedule`, `WeekdayFriday`, `WeekdayMonday`, `WeekdayTuesday` |
| `arrival_class_exception_test.go` | `models/schedule` | `ArrivalScheduleSourceClassException`, `ArrivalScheduleSourceClassSchedule`, `ClassArrivalException`, `Date`, `WeekdayMonday`, `WeekdayTuesday` |
| `arrival_service_test.go` | `models/schedule` | `NewDate`, `WeekdayFriday`, `WeekdayMonday`, `WeekdayThursday`, `WeekdayTuesday`, `WeekdayWednesday` |
| `care_offering_catalog_guards_test.go` | `models/activities` | `Group`, `GroupTypeCare`, `Schedule`, `WeekdayMonday`, `WeekdayTuesday` |
| `care_offering_catalog_guards_test.go` | `models/schedule` | `ActivityException`, `ActivityExceptionCancelled`, `ActivityExceptionModified`, `CalendarPeriod`, `Date`, `NewDate`, `PeriodTypeCustom`, `Timeframe` |
| `care_offering_catalog_test.go` | `models/activities` | `Group`, `GroupTypeCare`, `Schedule`, `ScheduleRepository`, `WeekdayMonday`, `WeekdayTuesday` |
| `care_offering_catalog_test.go` | `models/schedule` | `CalendarPeriod`, `Date`, `NewDate`, `PeriodTypeCustom` |
| `care_request_alumni_test.go` | `models/schedule` | `CareRequestStatusPending` |
| `care_request_decision_snapshot_test.go` | `models/schedule` | `CareRequestStatusWithdrawn` |
| `care_request_history_test.go` | `models/schedule` | `CareRequestKindPickupChange`, `CareRequestStatusRejected`, `CareRequestStatusWithdrawn` |
| `care_request_pickup_pill_test.go` | `models/schedule` | `CareRequestStatusPending`, `CareRequestStatusRejected` |
| `care_request_service_test.go` | `models/schedule` | `CareRequestStatusApproved`, `CareRequestStatusPending`, `CareRequestStatusRejected`, `Date`, `ExceptionSourceGuardian`, `PickupScheduleSourceCareOffering`, `StudentPickupException`, `TenantModel` |
| `enrollment_decision_env_test.go` | `models/activities` | `StudentEnrollment` |
| `lifecycle_care_lifecycle_integration_test.go` | `models/activities` | `Date`, `StudentEnrollment` |
| `lifecycle_care_lifecycle_integration_test.go` | `models/schedule` | `AttendanceStatusExpected`, `AttendanceStatusPresent`, `WeekdayMonday` |
| `lifecycle_care_lifecycle_service_test.go` | `models/activities` | `Date`, `StudentEnrollment` |
| `lifecycle_care_lifecycle_service_test.go` | `models/schedule` | `AttendanceStatusAbsent`, `AttendanceStatusExpected` |
| `lifecycle_care_withdrawal_lifecycle_test.go` | `models/schedule` | `AttendanceStatusPresent` |
| `offering_adjustment_dated_test.go` | `models/activities` | `Date`, `StudentEnrollment` |
| `offering_arrival_projection_test.go` | `models/schedule` | `WeekdayMonday`, `WeekdayThursday`, `WeekdayTuesday` |
| `offering_change_request_service_test.go` | `models/activities` | `Date`, `Group`, `GroupTypeActivity`, `GroupTypeCare`, `StudentEnrollment`, `TargetGroupTypeAngebot`, `TargetGroupTypeNone` |
| `offering_change_request_service_test.go` | `models/schedule` | `InstanceStatusCancelled`, `InstanceStatusPlanned` |
| `offering_pickup_projection_test.go` | `models/schedule` | `WeekdayMonday` |
| `offering_source_service_test.go` | `models/activities` | `Date`, `Group`, `Schedule`, `StudentEnrollment`, `TargetGroupTypeAngebot`, `TemplateWeekdayRosterKindProtectedStudent`, `WeekdayMonday`, `WeekdayTuesday` |
| `offering_source_service_test.go` | `models/schedule` | `AttendanceStatusExpected`, `CalendarPeriod` |
| `partial_absence_pending_request_test.go` | `models/schedule` | `Date` |
| `pickup_auto_excusal_test.go` | `models/schedule` | `AttendanceStatusAbsent`, `AttendanceStatusExpected`, `AttendanceSubstatusExcused`, `AttendanceSubstatusSick`, `Date`, `PartialAbsenceBlock`, `StudentPickupException`, `WeekdayMonday` |
| `pickup_baseline_fixture_test.go` | `models/schedule` | `PickupScheduleSourceCareOffering` |
| `pickup_change_cutoff_test.go` | `models/schedule` | `CareRequestStatusApproved`, `CareRequestStatusPending`, `Date`, `ExceptionSourceStaff` |
| `pickup_change_request_service_test.go` | `models/schedule` | `AttendanceStatusAbsent`, `AttendanceStatusExpected`, `AttendanceSubstatusExcused`, `CareRequestKindPickupChange`, `CareRequestStatusApproved`, `CareRequestStatusPending`, `CareRequestStatusRejected`, `CareScheduleChangeRequest`, `Date`, `ExceptionSourceGuardian`, `ExceptionSourceStaff`, `StudentPickupException`, `TenantModel` |
| `pickup_days_test.go` | `models/schedule` | `WeekdayThursday` |
| `pickup_extension_trigger_test.go` | `models/schedule` | `WeekdayMonday`, `WeekdayWednesday` |
| `pickup_schedule_provenance_test.go` | `models/schedule` | `PickupScheduleSourceCareOffering`, `PickupScheduleSourceStaff`, `StudentPickupSchedule`, `WeekdayMonday`, `WeekdayTuesday` |
| `pickup_schedule_service_test.go` | `models/schedule` | `Date`, `ExceptionSourceGuardian`, `ExceptionSourceStaff`, `NewDate`, `StudentPickupException`, `WeekdayFriday`, `WeekdayMonday`, `WeekdayThursday`, `WeekdayTuesday`, `WeekdayWednesday` |
| `template_roster_maintenance_test.go` | `models/activities` | `WeekdayMonday` |
| `timetable_arrival_projection_test.go` | `models/schedule` | `ActivityException`, `ActivityExceptionCancelled`, `ActivityExceptionModified`, `ArrivalScheduleSourceClassSchedule`, `AttendanceStatusExpected`, `Date`, `WeekdayThursday` |

### `modules/studentpresence/internal/application/presence` (external_test)

| File | Imported surface | Symbols |
|---|---|---|
| `attendance_mirror_integration_test.go` | `models/schedule` | `ActivityInstance`, `ActivityInstanceRepository`, `AttendanceStatusAbsent`, `AttendanceStatusExpected`, `AttendanceStatusPresent`, `AttendanceSubstatusExcused`, `AttendanceSubstatusLate`, `AttendanceSubstatusSick`, `InstanceStatusActive`, `InstanceStatusPlanned`, `InstanceStudent`, `InstanceStudentRepository`, `NewDate` |
| `attendance_sse_enrichment_test.go` | `models/schedule` | `AttendanceStatusExpected`, `AttendanceStatusPresent`, `InstanceStatusActive`, `InstanceStudent` |
| `presence_revision_rollback_test.go` | `models/schedule` | `AttendanceStatusExpected`, `InstanceStatusActive`, `InstanceStudentRepository` |
| `presence_slot_checkin_rollback_test.go` | `models/schedule` | `AttendanceStatusExpected`, `InstanceStudentKey`, `InstanceStudentRepository` |
| `presence_slot_checkout_rollback_test.go` | `models/schedule` | `AttendanceStatusPresent`, `InstanceStudentKey`, `InstanceStudentRepository` |
| `presence_visit_slot_checkin_rollback_test.go` | `models/schedule` | `AttendanceStatusExpected`, `InstanceStudent`, `InstanceStudentRepository` |
| `session_conflict_test.go` | `models/schedule` | `ActivityInstance`, `Date`, `InstanceStatusActive`, `InstanceStatusCompleted` |

## Register reductions

| Register | Before | After |
|---|---:|---:|
| Composition surface | 1638 | 1638 |
| Foreign compose aliases | 216 | 128 |
| People Directory vocabulary aliases | 88 | 0 |
| Compose LOC total | 32764 | 32613 |
| Care Plan compose LOC | 2556 | 2551 |
| People Directory compose LOC | 2654 | 2617 |
| People Directory vocabulary LOC | 106 | deleted |
| Timetable compose LOC | 4173 | 4171 |
| Timetable HTTP adapter compose LOC | 3563 | 3562 |
| Legacy-file LOC total | 2898 | 2897 |
| Timetable legacy interface LOC | 85 | 84 |

No increased budgets or new rule IDs. Three stale People Directory rules and the deleted vocabulary package classification are removed. Timetable forwarding methods and compose getters were removed to offset the new native fixture bindings.

### Retired exact keys

| Issue | Scope | Source | Target |
|---|---|---|---|
| #4076 | `external_test` | `modules/careplan/contracttest` | `models/activities` |
| #4076 | `external_test` | `modules/careplan/contracttest` | `models/schedule` |
| #4076 | `external_test` | `modules/careplan/contracttest` | `models/users` |
| #4076 | `external_test` | `modules/enrollment/http` | `models/users` |
| #4076 | `external_test` | `modules/studentpresence/internal/application/presence` | `models/schedule` |
| #4004 | `production` | `modules/timetable/compose/httpadapter` | `models/users` |
| #4004 | `production` | `modules/timetable/compose` | `models/users` |

The base has no live foreign users key for #4077 or #2751 that becomes stale in this slice. Those consumers were already behind the #4085 vocabulary route, which is removed here. Live unrelated keys are retained.

## Verification

Final commands and observed results are recorded in the PR and execution report. HTTP route registration and `TestFullProductionRouterGolden` source are not changed. No production or staging requests, schema changes, grants, hook allowances, or new root composition fields.
