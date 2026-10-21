---
status: accepted
---

# Pass-throughs keep a per-owner floor of four shapes

Decision of 2026-10-21 for
[#4459](https://github.com/moto-nrw/project-phoenix/issues/4459), part of
[#4403](https://github.com/moto-nrw/project-phoenix/issues/4403) under
[#3423](https://github.com/moto-nrw/project-phoenix/issues/3423). It records
the owner decision of 2026-10-20 on #4403 and changes no budget, no rule and
no scanner condition.

## Context

`TestModulePassthroughRatchet` counts a shape: an exported method of at most
14 body lines with exactly one call through a field of its receiver. Rule 8 of
`.claude/rules/backend-conventions.md` forbids a service method that only
forwards; it does not forbid every method of that shape. After the
facade-collapse rollout of #4403 (#4492, #4506, #4507, #4509, #4511; #4414,
#4480 and #4495 closed) the register reads 1,555 hits in 31 owners at
development `1ff680016b`, down from 2,925 on 2026-10-20. Every entry equals
its measured count.

The owner decided on #4403 not to drive the register to zero: about 600 of the
then 2,925 hits wrapped a tenant transaction and observation around one store
call, and the consumer-port adapters are the boundary ADR 0067 asks for.
Removing either would delete behaviour or break the boundary. The owner
decision on #4480 (section 5) and the #4495 measurement in #4507 named the
further items of this floor.

## Decision

1. **Four shapes form the floor.** A counted method belongs to the floor when
   it has one of these shapes:
   1. *Transaction and observation wrapper.* The field call sits in a function
      literal handed to the owner's tenant, administrative or platform
      transaction helper or its observation helper (`s.run`, `s.runWrite`,
      `runValue`, `requestCommand`, `withinTenant` and their siblings), or the
      field call is the transaction runner itself (`RunInTx`,
      `WithinTenant`). The wrapper is the unit of work; inlining it moves the
      transaction to every caller.
   2. *Consumer-port adapter* ([ADR 0067](0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md)
      point 2). The field is another owner's contract, the owner's
      root-facing `ports` package
      ([ADR 0064](0064-moved-code-stays-in-the-pass-through-scan.md)), or the
      method sits in a binding the root owns (`roots/*binding`, `api/`, the
      parent portal's care adapters), and the method translates values,
      errors or arguments on the way.
   3. *The Timetable adapters of #4480 section 5.* `CalendarPeriodRecords`
      over School Calendar, the enrollment schedules over Care Plan and the
      Records-to-public reads for Workforce stay as consumer-port adapters
      (owner decision on #4480).
   4. *Scanner limitation 5 of the ratchet header.* `Error()` on an error type
      that forwards to its wrapped cause, a fail-closed log line, a clock
      read. None of them reaches a repository or a port.
2. **Two documented items stay without a counted hit.**
   - `careplan.OfferingBookings` stays in the public package (#4495, #4507
     section 3). Moving it into the application layer turns
     `LockCareOfferingBookings` (observation plus one store call) into a
     counted hop and would raise `modules/careplan` from 163 to 164.
   - Identity & Access keeps the `engine` value that bundles its services for
     the `Owner` interface and its `DemoAccess()` accessor, which returns a
     field and is not counted (#4507 section 2).
3. **The Care Plan lock round trip is a contract-cut candidate, not a floor.**
   `CareLifecycle.lockPlanning` reaches its own booking lock through
   `ports.CareExitEnrollment.LockCareExitOfferingLinks` →
   `roots/owners` (`careExitEnrollment`, `EnrollmentBookingProjection`) →
   `workflows/careexit.Bookings` →
   `OfferingBookings.LockCareOfferingBookings`: Care Plan to Enrollment's port,
   the roots, the care exit workflow and back to Care Plan. Cutting it removes
   a tested cross-owner contract
   (`modules/careplan/contracttest/care_exit_storage_cutover_test.go`) and the
   `lock` booking observation, so it needs its own decision with the care exit
   contract. Until then the chain stays as it is.
4. **Everything else is outside the floor.** 789 hits have none of the four
   shapes. The appendix lists them per owner and file in three groups:
   - *pure forward* (80): body is `return s.x.M(params...)` with the
     parameters and the result unchanged, inside the owner;
   - *binding without translation* (60): the same pure forward, but over
     another owner's contract or a root-facing port;
   - *owner-edge translation* (649): one call inside the owner with
     validation, or with internal values or sentinels mapped onto the owner's
     public contract. The group also holds seams whose bound owner the scan
     cannot establish: a locally declared interface or an `internal/ports`
     interface may be a store or another owner's port.

   Rule 8 names validation and error mapping as encapsulation, but #4459
   admits only transaction wrappers and consumer-port adapters (shapes 1 to
   3; shape 4 is no forward at all), so this ADR does not admit the
   owner-edge translations. Each hit outside the floor leaves by removal, or
   joins the floor by a later decision that names its shape and the
   evidence.
5. **The register stays as it is.** The scanner keeps its four conditions,
   every entry keeps its measured count and may only fall, and no entry,
   alias, allowlist or exception is added. The floor is a record, not a
   budget: an entry may fall below its floor when a floor hit goes away.

## Measurement

The classification is mechanical and leans conservative: a hit enters the
floor only on the evidence named in point 1, so the floor per owner is a lower
bound and the "outside" count an upper bound. It was taken over the
ratchet's own scan (`modulePassthroughScan`, `modulePassthroughOwnerKey`,
`modulePassthroughIsDelegation`) at `1ff680016b`, with the receiver's field
types resolved from the struct declarations in the hit's package:

- *Transaction and observation wrapper:* the field call sits in a function
  literal passed to a helper that opens a transaction or records an
  observation (each of the 56 helper and package pairs was read), or the call
  is `RunInTx` or `WithinTenant`.
- *Consumer-port adapter:* the field type comes from another owner's package
  or from `modules/<owner>/ports`, or the file lies in `roots/`, `api/` or
  `workflows/parentportal/internal/care/`; and the method is not a pure
  forward.
- *Scanner limitation:* method `Error` whose call ends in `.Error`, or a call
  through `logger`, `clock` or a `Now` field.
- *Pure forward:* one statement, `return` or an expression statement, of the
  field call with the method's parameters in order and nothing else.

Two counted files are fixture helpers without a `_test.go` suffix
(`modules/delivery/internal/application/intent_worker_test_helpers.go`,
`modules/timetable/compose/class_arrival_exceptions_test_helpers.go`); they
are counted like production code.

### Floor per owner

Columns: budget = measured count; the four floor shapes (Tx/obs, port
adapter, #4480 section 5, scanner limitation) and their sum; then the three
groups outside the floor (pure forward, binding without translation,
owner-edge translation) and their sum.

| Owner | Budget | Tx/obs | Port adapter | #4480 §5 | Scanner | Floor | Pure | Binding | Edge | Outside |
|---|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|
| `modules/appointments` | 21 | 21 | 0 | 0 | 0 | 21 | 0 | 0 | 0 | 0 |
| `modules/auditlog` | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 1 |
| `modules/careplan` | 163 | 115 | 7 | 0 | 0 | 122 | 7 | 0 | 34 | 41 |
| `modules/classday` | 18 | 0 | 3 | 0 | 1 | 4 | 4 | 0 | 10 | 14 |
| `modules/communication` | 58 | 22 | 3 | 0 | 1 | 26 | 1 | 0 | 31 | 32 |
| `modules/dataimport` | 1 | 0 | 1 | 0 | 0 | 1 | 0 | 0 | 0 | 0 |
| `modules/delivery` | 6 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 5 | 6 |
| `modules/devicefleet` | 35 | 7 | 2 | 0 | 3 | 12 | 2 | 0 | 21 | 23 |
| `modules/devicescan` | 41 | 0 | 25 | 0 | 3 | 28 | 0 | 6 | 7 | 13 |
| `modules/enrollment` | 31 | 7 | 0 | 0 | 0 | 7 | 7 | 0 | 17 | 24 |
| `modules/exporttransfer` | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 1 |
| `modules/facilities` | 13 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 13 | 13 |
| `modules/feedback` | 2 | 2 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 |
| `modules/filestorage` | 9 | 1 | 0 | 0 | 0 | 1 | 3 | 0 | 5 | 8 |
| `modules/identityaccess` | 251 | 64 | 58 | 0 | 4 | 126 | 6 | 27 | 92 | 125 |
| `modules/mealplan` | 3 | 3 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | 0 |
| `modules/organizationtenancy` | 12 | 11 | 0 | 0 | 0 | 11 | 1 | 0 | 0 | 1 |
| `modules/peopledirectory` | 242 | 58 | 47 | 0 | 1 | 106 | 11 | 21 | 104 | 136 |
| `modules/requestreview` | 17 | 0 | 15 | 0 | 0 | 15 | 0 | 2 | 0 | 2 |
| `modules/schedulerruntime` | 4 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 4 | 4 |
| `modules/schoolcalendar` | 18 | 9 | 0 | 0 | 0 | 9 | 2 | 0 | 7 | 9 |
| `modules/schoolmembership` | 16 | 14 | 0 | 0 | 0 | 14 | 0 | 0 | 2 | 2 |
| `modules/schoolsetup` | 2 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 1 | 2 |
| `modules/schoolstructure` | 43 | 0 | 0 | 0 | 1 | 1 | 0 | 0 | 42 | 42 |
| `modules/settings` | 19 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 18 | 19 |
| `modules/studentpresence` | 145 | 63 | 0 | 0 | 0 | 63 | 25 | 0 | 57 | 82 |
| `modules/timetable` | 211 | 85 | 45 | 21 | 1 | 152 | 1 | 1 | 57 | 59 |
| `modules/workforce` | 151 | 20 | 0 | 0 | 4 | 24 | 7 | 0 | 120 | 127 |
| `workflows/openroommove` | 1 | 0 | 1 | 0 | 0 | 1 | 0 | 0 | 0 | 0 |
| `workflows/parentportal` | 17 | 0 | 17 | 0 | 0 | 17 | 0 | 0 | 0 | 0 |
| `workflows/shiftplansync` | 3 | 0 | 0 | 0 | 0 | 0 | 0 | 3 | 0 | 3 |
| **Total** | **1555** | **502** | **224** | **21** | **19** | **766** | **80** | **60** | **649** | **789** |

## Consequences

- #4403 has its recorded result: a floor of 766 in four shapes, 789 hits
  outside it, and a total that keeps falling. Whether #4403 and #3423 close
  with hits outside the floor is the owner's call; this ADR closes neither.
- A pure forward or a binding without translation is the next Rule 8 work.
  The owner-edge translations shrink owner by owner where the rollout's rule
  applies: a domain copy that only duplicates a public value field by field
  is deleted ([ADR 0074](0074-delivery-and-export-transfer-speak-their-own-contract.md)
  context).
- A new hit has to fit one of the four shapes; the budget still rejects
  growth, so the floor never licenses a raise.

## Appendix: hits outside the floor

Per owner and file (paths relative to the owner; `app/` is
`internal/application/`), measured at `1ff680016b`. Owners without an entry
have every hit in the floor.

- `modules/auditlog`: owner-edge translation: `app/command.go` 1
- `modules/careplan`: pure forward: `app/compose_bindings_care_lifecycle.go` 3, `app/baseline_records.go` 2, `app/arrival_schedule_rules.go` 1, `app/pickup_schedules.go` 1 · owner-edge translation: `app/compose_bindings_parent_requests.go` 7, `app/baseline_records.go` 3, `app/care_documents.go` 3, `app/companion_records.go` 3, `app/companions.go` 3, `app/excused_request_coordination.go` 2, `app/pickup_excusal.go` 2, `app/arrival_schedules.go` 1, `app/care_day_facts.go` 1, `app/class_arrival_exceptions.go` 1, `app/compose_bindings_repository_adapter_parent_portal.go` 1, `app/compose_bindings_repository_adapter_parent_portal_schools.go` 1, `app/parent_portal_people.go` 1, `app/partial_absence.go` 1, `app/request_conflicts.go` 1, `app/request_reads.go` 1, `compose/repository_adapter_parent_portal_schools.go` 1, `requestfeed/app/service.go` 1
- `modules/classday`: pure forward: `app/classday.go` 2, `app/compose_bindings_classday.go` 2 · owner-edge translation: `app/compose_bindings_slotlists.go` 4, `app/arrival_exceptions.go` 3, `app/compose_bindings_classday.go` 2, `app/classday.go` 1
- `modules/communication`: pure forward: `app/parent_announcements_facade.go` 1 · owner-edge translation: `app/parent_announcements_facade.go` 19, `app/staff_attachments.go` 4, `app/parentmessaging/emitter.go` 3, `app/staff_poll.go` 2, `app/parent_declarations.go` 1, `app/parent_message_retention.go` 1, `app/staff_declaration.go` 1
- `modules/delivery`: pure forward: `app/worker.go` 1 · owner-edge translation: `app/service.go` 3, `app/compose_bindings_new.go` 1, `app/intent_worker_test_helpers.go` 1
- `modules/devicefleet`: pure forward: `internal/adapters/repositoryadapter/device_repository.go` 2 · owner-edge translation: `app/deviceservice/iot_service.go` 12, `internal/adapters/repositoryadapter/device_repository.go` 6, `app/presence_reader.go` 2, `app/online.go` 1
- `modules/devicescan`: binding without translation: `roots/devicescanbinding/presence.go` 3, `api/kiosk_timetable.go` 1, `compose/settings.go` 1, `roots/devicescanbinding/session_lifecycle.go` 1 · owner-edge translation: `app/session_lifecycle.go` 5, `app/school_name.go` 1, `app/service.go` 1
- `modules/enrollment`: pure forward: `app/compose_bindings_care_offering_values.go` 2, `app/service_request_locks.go` 2, `app/phases.go` 1, `app/service_enrollment.go` 1, `app/service_phase_queries.go` 1 · owner-edge translation: `app/compose_bindings_care_offering_values.go` 6, `app/form_schemas.go` 3, `app/captcha.go` 2, `app/phases.go` 2, `app/service_enrollment.go` 2, `app/approved_offerings.go` 1, `app/service_account_requests.go` 1
- `modules/exporttransfer`: owner-edge translation: `app/service.go` 1
- `modules/facilities`: owner-edge translation: `app/roomservice/facility_service.go` 5, `app/roomservice/attendance_rooms.go` 4, `app/roomservice/room_names.go` 4
- `modules/filestorage`: pure forward: `app/native_compose_bindings_new.go` 3 · owner-edge translation: `app/compose_bindings_new.go` 3, `app/native_compose_bindings_new.go` 1, `app/retention.go` 1
- `modules/identityaccess`: pure forward: `app/caller_context.go` 3, `app/role_administration.go` 2, `app/mfa.go` 1 · binding without translation: `app/account_lifecycle.go` 15, `app/account_authentication.go` 3, `app/caller_context.go` 3, `app/operator_authentication.go` 2, `app/operator_provisioning.go` 2, `app/demo_access.go` 1, `app/role_administration.go` 1 · owner-edge translation: `app/account_mfa_capability.go` 35, `app/caller_engine.go` 19, `app/demo_access.go` 6, `app/caller_context.go` 5, `app/mfa.go` 4, `app/account_mfa_flow.go` 3, `app/operator_provisioning.go` 3, `app/account_mfa_admin.go` 2, `app/account_mfa_composition.go` 2, `app/account_passkey_flow.go` 2, `app/operator_mfa_flow.go` 2, `app/operator_passkey_flow.go` 2, `app/password_reset.go` 2, `app/session_cleanup.go` 2, `app/rfid.go` 1, `app/session_revocation.go` 1, `app/staff_offboarding.go` 1
- `modules/organizationtenancy`: pure forward: `app/provisioning.go` 1
- `modules/peopledirectory`: pure forward: `app/capability_account_lifecycle_commands.go` 5, `app/guardian_service.go` 3, `app/guardian_phones.go` 2, `app/attendance_route_identity.go` 1 · binding without translation: `app/staff_directory.go` 10, `app/person_directory.go` 6, `app/student_route_persons.go` 2, `app/student_seams.go` 2, `app/group_route_people.go` 1 · owner-edge translation: `app/capability_student_directory.go` 17, `app/student_records_boundary.go` 17, `app/capability_guardian_portal_commands.go` 10, `app/capability_student.go` 9, `app/student_writes_boundary.go` 6, `app/capability_guardians.go` 5, `app/capability_student_photo.go` 5, `app/capability_student_write.go` 5, `app/student_directory_boundary.go` 5, `app/capability_student_enrollment.go` 4, `app/birthday_view.go` 3, `app/capability_student_audit.go` 3, `app/capability_student_deletion.go` 3, `app/capability_family_protection.go` 2, `app/contract_student_audit.go` 2, `app/root_read_ports.go` 2, `app/attendance_route_identity.go` 1, `app/capability_departure_plans.go` 1, `app/capability_guardian_retention.go` 1, `app/guardian_links.go` 1, `app/person_rows_boundary.go` 1, `app/student_audited_update.go` 1
- `modules/requestreview`: binding without translation: `roots/requestreviewbinding/family_protection.go` 1, `roots/requestreviewbinding/schedule_reviews.go` 1
- `modules/schedulerruntime`: owner-edge translation: `app/lease_store.go` 4
- `modules/schoolcalendar`: pure forward: `app/service.go` 1, `internal/adapters/postgres/native_feed_history.go` 1 · owner-edge translation: `app/non_working_days.go` 4, `portal/app/calendar_renderer.go` 2, `portal/app/feed_cleanup.go` 1
- `modules/schoolmembership`: owner-edge translation: `app/employment_binding.go` 2
- `modules/schoolsetup`: pure forward: `app/service.go` 1 · owner-edge translation: `app/progress.go` 1
- `modules/schoolstructure`: owner-edge translation: `app/groups/facade_group_management.go` 15, `compose/group_values_test_helpers.go` 15, `app/groups/facade_group_reads.go` 6, `app/groups/substitution_boundary.go` 3, `app/group_rooms.go` 1, `app/groups/group_names.go` 1, `compose/teacher_group_ids_test_helpers.go` 1
- `modules/settings`: pure forward: `app/tenant_operations.go` 1 · owner-edge translation: `app/presence_settings.go` 16, `app/tenant_operations.go` 2
- `modules/studentpresence`: pure forward: `app/presence/facade_presence_operations.go` 11, `app/session_record_facade.go` 7, `app/presence/facade_status_days.go` 5, `app/presence/facade_student_history.go` 2 · owner-edge translation: `app/presence/facade_presence_operations.go` 17, `app/presence/facade_presence_facade.go` 11, `app/session_record_facade.go` 11, `app/presence/visit_reads.go` 5, `app/session_attendance_bindings.go` 5, `app/presence/combined_group_service.go` 2, `app/presence/active_service.go` 1, `app/presence/facade_student_history.go` 1, `app/presence/session_groups.go` 1, `app/presence/session_service.go` 1, `app/presence/student_status_day_service.go` 1, `app/presence/transit_service.go` 1
- `modules/timetable`: pure forward: `app/instance_replan_match.go` 1 · binding without translation: `app/timetable_data_spontaneous.go` 1 · owner-edge translation: `app/activity_service.go` 5, `app/catalog.go` 5, `app/planning_track_views.go` 5, `app/activity_category_operations.go` 4, `app/class_arrivals.go` 4, `app/template_administration.go` 4, `app/native_fixture_exception.go` 3, `app/native_fixture_timeframe.go` 3, `app/activity_schedule_operations.go` 2, `app/activity_supervisor_operations.go` 2, `app/instance_lifecycle_deviation_port.go` 2, `app/instance_replan_match.go` 2, `app/native_fixture_enrollment.go` 2, `app/session_records.go` 2, `app/workforce_shift_rows.go` 2, `app/workforce_shift_type_rows.go` 2, `app/activity_category_seeder.go` 1, `app/activity_recovery.go` 1, `app/category_count.go` 1, `app/native_fixture_schedule.go` 1, `app/participant_fixture_rows.go` 1, `app/room_directory.go` 1, `app/session_facts.go` 1, `app/template_end.go` 1
- `modules/workforce`: pure forward: `app/staffadmin_engine.go` 4, `app/timetracking/public_work_sessions.go` 1, `app/timetracking/work_session_schedule.go` 1, `compose/work_presence.go` 1 · owner-edge translation: `app/staffadmin_engine.go` 32, `app/timetracking/public_absences.go` 19, `app/timetracking/public_work_sessions.go` 15, `app/timetracking/public_capabilities.go` 10, `app/timetracking/public_time_clock.go` 7, `app/timetracking/work_session_history.go` 6, `internal/planning/planning.go` 5, `app/timetracking/capability_absence_type_reader.go` 4, `app/timetracking/public_absence_type_administration.go` 4, `app/timetracking/public_month_snapshots.go` 4, `internal/planning/shifttypes.go` 4, `app/offboarding.go` 1, `app/timetracking/staff_absence_service.go` 1, `app/timetracking/staff_balance_adjustment_service.go` 1, `app/timetracking/staff_month_close_service.go` 1, `app/timetracking/staff_vacation_opening.go` 1, `app/timetracking/work_session_maintenance.go` 1, `compose/time_tracking.go` 1, `internal/planning/ports.go` 1, `internal/planning/shift_type_service.go` 1, `internal/planning/staff_shift_service.go` 1
- `workflows/shiftplansync`: binding without translation: `app/compose_bindings_new.go` 3
