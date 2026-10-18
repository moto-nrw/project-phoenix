# Timetable contract cluster

Refs #4004, #4076, #4086. Inventory base: `caee1cbd5b` on development.

## Result

All three production consumer owners now receive consumer-owned ports bound
in the root from Timetable's native `Catalog`, `TemplateCatalog` and
`SessionRecords`. They reuse the existing activity service and repository
values. No root graph field, rule, role, grant, alias, exception or hook
allowance was added. Root composition remains 1,644.

The native contracts reuse `Group`, `GroupInput`, `Category`, `CreateCategory`
and `ScheduledInstance`. `OccupiedGroup` adds only occupancy to `Group`.
No new row-shaped clone or alias of a legacy model was introduced. Catalog
creation preserves the established service's validation and error wrapping;
instance/session commands still use the same joined adapter, context and
transaction. The dashboard reads the same category-enriched template list
and joined instances. Kiosk mirroring still creates the plan, active session
and staff with the same timestamps and tenant context.

`services/activities` was an alias-only barrel. All of its own-owner callers
now name the existing composition directly; the obsolete package and its
`application -> compose` rule are deleted instead of relocating aliases.
The activities and schedule repositories were already internal at this base.

## Foreign importers before the change

Each row identifies a file, scope, imported package and selected symbols.
No foreign importer of either matching `database/repositories` package
remains at this base. Same-owner Timetable consumers are not foreign edges.

### care-plan

| Importer | Scope | Legacy package | Symbols |
|---|---|---|---|
| `modules/careplan/contracttest/activity_date_test.go` | `external_test` | `models/activities` | `Date` |
| `modules/careplan/contracttest/arrival_baseline_service_test.go` | `external_test` | `models/schedule` | `ArrivalScheduleSourceClassSchedule`, `ArrivalScheduleSourceStaff`, `WeekdayMonday`, `WeekdayTuesday`, `WeekdayWednesday` |
| `modules/careplan/contracttest/arrival_class_bulk_test.go` | `external_test` | `models/schedule` | `ArrivalScheduleSourceClassSchedule`, `ArrivalScheduleSourceStaff`, `StudentArrivalSchedule`, `WeekdayFriday`, `WeekdayMonday`, `WeekdayTuesday` |
| `modules/careplan/contracttest/arrival_class_exception_test.go` | `external_test` | `models/schedule` | `ArrivalScheduleSourceClassException`, `ArrivalScheduleSourceClassSchedule`, `ClassArrivalException`, `Date`, `WeekdayMonday`, `WeekdayTuesday` |
| `modules/careplan/contracttest/arrival_service_test.go` | `external_test` | `models/schedule` | `NewDate`, `WeekdayFriday`, `WeekdayMonday`, `WeekdayThursday`, `WeekdayTuesday`, `WeekdayWednesday` |
| `modules/careplan/contracttest/care_offering_catalog_guards_test.go` | `external_test` | `models/activities` | `Group`, `GroupTypeCare`, `Schedule`, `WeekdayMonday`, `WeekdayTuesday` |
| `modules/careplan/contracttest/care_offering_catalog_guards_test.go` | `external_test` | `models/schedule` | `ActivityException`, `ActivityExceptionCancelled`, `ActivityExceptionModified`, `CalendarPeriod`, `Date`, `NewDate`, `PeriodTypeCustom`, `Timeframe` |
| `modules/careplan/contracttest/care_offering_catalog_test.go` | `external_test` | `models/activities` | `Group`, `GroupTypeCare`, `Schedule`, `ScheduleRepository`, `WeekdayMonday`, `WeekdayTuesday` |
| `modules/careplan/contracttest/care_offering_catalog_test.go` | `external_test` | `models/schedule` | `CalendarPeriod`, `Date`, `NewDate`, `PeriodTypeCustom` |
| `modules/careplan/contracttest/care_request_alumni_test.go` | `external_test` | `models/schedule` | `CareRequestStatusPending` |
| `modules/careplan/contracttest/care_request_decision_snapshot_test.go` | `external_test` | `models/schedule` | `CareRequestStatusWithdrawn` |
| `modules/careplan/contracttest/care_request_history_test.go` | `external_test` | `models/schedule` | `CareRequestKindPickupChange`, `CareRequestStatusRejected`, `CareRequestStatusWithdrawn` |
| `modules/careplan/contracttest/care_request_pickup_pill_test.go` | `external_test` | `models/schedule` | `CareRequestStatusPending`, `CareRequestStatusRejected` |
| `modules/careplan/contracttest/care_request_service_test.go` | `external_test` | `models/schedule` | `CareRequestStatusApproved`, `CareRequestStatusPending`, `CareRequestStatusRejected`, `Date`, `ExceptionSourceGuardian`, `PickupScheduleSourceCareOffering`, `StudentPickupException`, `TenantModel` |
| `modules/careplan/contracttest/enrollment_decision_env_test.go` | `external_test` | `models/activities` | `StudentEnrollment` |
| `modules/careplan/contracttest/lifecycle_care_lifecycle_integration_test.go` | `external_test` | `models/activities` | `Date`, `StudentEnrollment` |
| `modules/careplan/contracttest/lifecycle_care_lifecycle_integration_test.go` | `external_test` | `models/schedule` | `AttendanceStatusExpected`, `AttendanceStatusPresent`, `WeekdayMonday` |
| `modules/careplan/contracttest/lifecycle_care_lifecycle_service_test.go` | `external_test` | `models/activities` | `Date`, `StudentEnrollment` |
| `modules/careplan/contracttest/lifecycle_care_lifecycle_service_test.go` | `external_test` | `models/schedule` | `AttendanceStatusAbsent`, `AttendanceStatusExpected` |
| `modules/careplan/contracttest/lifecycle_care_withdrawal_lifecycle_test.go` | `external_test` | `models/schedule` | `AttendanceStatusPresent` |
| `modules/careplan/contracttest/offering_adjustment_dated_test.go` | `external_test` | `models/activities` | `Date`, `StudentEnrollment` |
| `modules/careplan/contracttest/offering_arrival_projection_test.go` | `external_test` | `models/schedule` | `WeekdayMonday`, `WeekdayThursday`, `WeekdayTuesday` |
| `modules/careplan/contracttest/offering_change_request_service_test.go` | `external_test` | `models/activities` | `Date`, `Group`, `GroupTypeActivity`, `GroupTypeCare`, `StudentEnrollment`, `TargetGroupTypeAngebot`, `TargetGroupTypeNone` |
| `modules/careplan/contracttest/offering_change_request_service_test.go` | `external_test` | `models/schedule` | `InstanceStatusCancelled`, `InstanceStatusPlanned` |
| `modules/careplan/contracttest/offering_pickup_projection_test.go` | `external_test` | `models/schedule` | `WeekdayMonday` |
| `modules/careplan/contracttest/offering_source_service_test.go` | `external_test` | `models/activities` | `Date`, `Group`, `Schedule`, `StudentEnrollment`, `TargetGroupTypeAngebot`, `TemplateWeekdayRosterKindProtectedStudent`, `WeekdayMonday`, `WeekdayTuesday` |
| `modules/careplan/contracttest/offering_source_service_test.go` | `external_test` | `models/schedule` | `AttendanceStatusExpected`, `CalendarPeriod` |
| `modules/careplan/contracttest/partial_absence_pending_request_test.go` | `external_test` | `models/schedule` | `Date` |
| `modules/careplan/contracttest/pickup_auto_excusal_test.go` | `external_test` | `models/schedule` | `AttendanceStatusAbsent`, `AttendanceStatusExpected`, `AttendanceSubstatusExcused`, `AttendanceSubstatusSick`, `Date`, `PartialAbsenceBlock`, `StudentPickupException`, `WeekdayMonday` |
| `modules/careplan/contracttest/pickup_baseline_fixture_test.go` | `external_test` | `models/schedule` | `PickupScheduleSourceCareOffering` |
| `modules/careplan/contracttest/pickup_change_cutoff_test.go` | `external_test` | `models/schedule` | `CareRequestStatusApproved`, `CareRequestStatusPending`, `Date`, `ExceptionSourceStaff` |
| `modules/careplan/contracttest/pickup_change_request_service_test.go` | `external_test` | `models/schedule` | `AttendanceStatusAbsent`, `AttendanceStatusExpected`, `AttendanceSubstatusExcused`, `CareRequestKindPickupChange`, `CareRequestStatusApproved`, `CareRequestStatusPending`, `CareRequestStatusRejected`, `CareScheduleChangeRequest`, `Date`, `ExceptionSourceGuardian`, `ExceptionSourceStaff`, `StudentPickupException`, `TenantModel` |
| `modules/careplan/contracttest/pickup_days_test.go` | `external_test` | `models/schedule` | `WeekdayThursday` |
| `modules/careplan/contracttest/pickup_extension_trigger_test.go` | `external_test` | `models/schedule` | `WeekdayMonday`, `WeekdayWednesday` |
| `modules/careplan/contracttest/pickup_schedule_provenance_test.go` | `external_test` | `models/schedule` | `PickupScheduleSourceCareOffering`, `PickupScheduleSourceStaff`, `StudentPickupSchedule`, `WeekdayMonday`, `WeekdayTuesday` |
| `modules/careplan/contracttest/pickup_schedule_service_test.go` | `external_test` | `models/schedule` | `Date`, `ExceptionSourceGuardian`, `ExceptionSourceStaff`, `NewDate`, `StudentPickupException`, `WeekdayFriday`, `WeekdayMonday`, `WeekdayThursday`, `WeekdayTuesday`, `WeekdayWednesday` |
| `modules/careplan/contracttest/template_roster_maintenance_test.go` | `external_test` | `models/activities` | `WeekdayMonday` |
| `modules/careplan/contracttest/timetable_arrival_projection_test.go` | `external_test` | `models/schedule` | `ActivityException`, `ActivityExceptionCancelled`, `ActivityExceptionModified`, `ArrivalScheduleSourceClassSchedule`, `AttendanceStatusExpected`, `Date`, `WeekdayThursday` |

### device-fleet

| Importer | Scope | Legacy package | Symbols |
|---|---|---|---|
| `modules/devicefleet/compose/legacy/dashboard_sources.go` | `production` | `models/activities` | `GroupRepository` |
| `modules/devicefleet/compose/legacy/dashboard_sources.go` | `production` | `models/schedule` | `ActivityInstanceRepository`, `Date`, `InstanceStatusPlanned` |

### facilities

| Importer | Scope | Legacy package | Symbols |
|---|---|---|---|
| `modules/facilities/compose/legacy/system_adapters.go` | `production` | `models/activities` | `Category`, `Group`, `GroupListQuery`, `Schedule` |
| `modules/facilities/compose/legacy/system_adapters.go` | `production` | `services/activities` | `ActivityService` |

### process-device-scan

| Importer | Scope | Legacy package | Symbols |
|---|---|---|---|
| `modules/devicescan/compose/catalog.go` | `production` | `models/activities` | `Category`, `Group`, `GroupListQuery`, `Schedule` |
| `modules/devicescan/compose/catalog.go` | `production` | `services/activities` | `ActivityService` |
| `modules/devicescan/compose/compose.go` | `production` | `services/activities` | `ActivityService` |
| `modules/devicescan/compose/directory.go` | `production` | `services/activities` | `ActivityService` |
| `modules/devicescan/compose/session_mirror.go` | `production` | `models/schedule` | `ActivityInstance`, `InstanceStaff`, `InstanceStatusActive`, `ParseDate` |
| `modules/devicescan/compose/session_mirror.go` | `production` | `services/activities` | `ActivityService` |

### student-presence

| Importer | Scope | Legacy package | Symbols |
|---|---|---|---|
| `modules/studentpresence/internal/application/presence/attendance_mirror_integration_test.go` | `external_test` | `models/schedule` | `ActivityInstance`, `ActivityInstanceRepository`, `AttendanceStatusAbsent`, `AttendanceStatusExpected`, `AttendanceStatusPresent`, `AttendanceSubstatusExcused`, `AttendanceSubstatusLate`, `AttendanceSubstatusSick`, `InstanceStatusActive`, `InstanceStatusPlanned`, `InstanceStudent`, `InstanceStudentRepository`, `NewDate` |
| `modules/studentpresence/internal/application/presence/attendance_sse_enrichment_test.go` | `external_test` | `models/schedule` | `AttendanceStatusExpected`, `AttendanceStatusPresent`, `InstanceStatusActive`, `InstanceStudent` |
| `modules/studentpresence/internal/application/presence/presence_revision_rollback_test.go` | `external_test` | `models/schedule` | `AttendanceStatusExpected`, `InstanceStatusActive`, `InstanceStudentRepository` |
| `modules/studentpresence/internal/application/presence/presence_slot_checkin_rollback_test.go` | `external_test` | `models/schedule` | `AttendanceStatusExpected`, `InstanceStudentKey`, `InstanceStudentRepository` |
| `modules/studentpresence/internal/application/presence/presence_slot_checkout_rollback_test.go` | `external_test` | `models/schedule` | `AttendanceStatusPresent`, `InstanceStudentKey`, `InstanceStudentRepository` |
| `modules/studentpresence/internal/application/presence/presence_visit_slot_checkin_rollback_test.go` | `external_test` | `models/schedule` | `AttendanceStatusExpected`, `InstanceStudent`, `InstanceStudentRepository` |
| `modules/studentpresence/internal/application/presence/session_conflict_test.go` | `external_test` | `models/schedule` | `ActivityInstance`, `Date`, `InstanceStatusActive`, `InstanceStatusCompleted` |

## Consumer decisions

1. **Device Scan:** the root binds its existing catalog/directory and session
   mirror ports. Consumer catalog and session values move to Device Scan's
   existing public package; operation interfaces stay in its application.
   Existing tests receive only import/type-name repairs. The root keeps its
   anonymous catalog port, so no generic CRUD operation becomes a public
   domain contract.
2. **Device Fleet:** its existing dashboard dependency fields now accept
   narrow template and planned-activity readers. The root maps the native
   Timetable values. Interface embedding removes redundant delegations;
   capacity `<= 0` remains unlimited (`nil`).
3. **Facilities:** its existing system-activity values and catalog port are
   defined once in its existing public package. The root binds the native
   Timetable catalog; the retained room service consumes the same port.
4. **Care Plan contract tests: retained unchanged.** These tests instantiate
   legacy rows and pass them to the retained repository methods. For example,
   `care_offering_catalog_test.go` names `ScheduleRepository` and constructs
   `Group`, `Schedule`, `CalendarPeriod` and `Timeframe` rows. Native owner
   values have flat persistence metadata and string date/clock boundaries;
   repository `Create(ctx, *row) error` differs from the native input/result
   command contract. Substituting imports/types alone cannot preserve the
   fixture calls and their mutations. A native-fixture/API migration exceeds
   the mechanical test-repair constraint. Both #4076 keys remain.
5. **Student Presence integration tests: retained unchanged.**
   `attendance_mirror_integration_test.go` stores
   `ActivityInstanceRepository` and `InstanceStudentRepository`; its fixtures
   and rollback suites exercise retained row mutation, attendance and execution
   state together. The native query/command ports have different signatures
   and return shapes. Replacing them requires fixture/API-call changes, not
   mechanical imports/types. Its #4076 key remains. No reverse production
   owner edge is added.

Two of five foreign consumer owners remain blocked, not more than half.
No assertions, HTTP routes, error codes or golden files were changed.

## Relocation decision

`models/activities` and `models/schedule` remain in place. Their policy role
is `legacy-model`; `validateLegacyModelPackages` rejects that role outside
`models/` (ADR 0057), while an ADR 0020 relocation must keep the role identical.
Changing them to `domain` or record types would be a role change and is not
performed. The two unchanged foreign test consumers would also fail Go's
internal visibility. This is the instructed consumer-work fallback, not a
claim that the model relocation is complete. No relocation declaration is
needed for deleting the empty alias-only service barrel.

## Ratchet evidence

- Exact keys: 56 → 49. Seven production #4004 keys disappear: Device Scan
  three, Device Fleet two, Facilities two. #4076, #4077 and #2751 lose no key.
- Rules: one obsolete `timetable-activities.compatibility-application.compose`
  rule removed; no rule added. Owner/role classifications unchanged for
  retained packages; the deleted service classification disappears.
- Compose LOC: Device Scan 1,441 → 1,272; Timetable 4,218 → 4,173;
  total 33,081 → 32,867.
- Legacy-tree LOC: Device Fleet 143 → 109; Facilities 517 → 446;
  total 660 → 555.
- Legacy-file LOC: activity service 537 → 527; category operations 138 → 98;
  interface 90 → 85; total 2,953 → 2,898.
- Pass-throughs: Device Scan stays 48; owner total stays 3,052. The scan
  follows its eight moved delegations into `api/device_scan_catalog.go` and
  `api/kiosk_timetable.go` and attributes them to Device Scan (ADR 0064).
  The moved-tree register now records the attributed owner explicitly; this
  expands measurement and grants nothing. No assertion or allowance changes.
  Timetable remains 491: nine new native adapter delegations are offset by
  deleting the unused category read, three identical recovery wrappers,
  four template-administration wrappers and one deviation-lifecycle wrapper.
  Existing interfaces/methods are promoted directly, without aliases or
  moving delegations outside the scan. Unchanged owner-row conversions move
  from compose to application to offset the native composition bindings.

Final verification and integration base are recorded in the PR.
