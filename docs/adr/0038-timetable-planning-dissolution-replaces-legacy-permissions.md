---
status: accepted
---

# Timetable planning dissolution may replace its legacy permissions slice by slice

Accepted for #3424 in the implementing change of its first slice, following
the precedent of ADR 0030, ADR 0035, ADR 0036 and ADR 0037. The pull request review is
where it can still be rejected; the exception activates only in the policy
epoch that review merges, and every later slice of #3424 reuses it under the
same conditions.

## Context

#3424 dissolves `modules/timetable/legacy/timetableplanning`, the 23,300 LOC
nest #3218 relocated out of `services/schedule` and classified as
`inbound-timetable`/`adapter`. Unlike the three earlier nests it is too large
for one assignment: the ticket cuts it into six slices, each of which moves
behaviour behind the public capability of its real owner, switches the
consumers to that contract and deletes its own files and compatibility rules.
The nest therefore still exists at the base of every slice but the last.

Slice S6 moves calendar periods, closing days, statutory holidays and
dateframes to the School Calendar owner (`modules/schoolcalendar`), whose
`policy.json` entries already own `schedule.calendar_periods`,
`schedule.closing_days` and `schedule.dateframes`. The persistence of those
tables had reached the owner with #2666; the application behaviour, the A/B
week engine and the model-typed facades had not. After the slice:

- the period administration (name uniqueness, the same-type overlap invariant
  #1837, the recurrence gate and the care-offering guard) is
  `schoolcalendar.CalendarPeriodAdministration`;
- the tenant's non-working days are `schoolcalendar.NonWorkingDayQuery`;
- the A/B week decision is `schoolcalendar.WeekPatternApplies`, the single
  engine shifts, staff notices, care-offering validation and materialization
  decide through;
- dateframes are served by the schedules HTTP composition from the School
  Calendar, timeframes and recurrence rules from the Timetable owner.

The PR-mode strictness check rejects every new permission between package-role
points that exist at the base SHA. The consumers that reached the nest for
this behaviour — `api/timetable`, the schedules HTTP composition, the
retained shift planning services, Enrollment and two behaviour suites — must
now name the School Calendar's public contract, and the retained nest itself
must consume the engine that moved out of it. Deleting the nest's
compatibility permissions is the required outcome of the ticket; converting
them to exact debt is what the ticket rules out.

## Decision

Permit the dissolution in reviewed policy epochs, under these conditions:

1. The immutable base classifies `modules/timetable/legacy/timetableplanning`
   exactly as `inbound-timetable`/`adapter` with the test roles
   `module-internal-test` and `module-behavior-test`.
2. The candidate's policy epoch is higher than the base's. The candidate may
   still classify the nest: the exception is for a dissolution in slices and
   ends when the last slice removes the package from the base.
3. A consumer gains access only where its own owner-role point could already
   reach the nest in the same scope, and only to the School Calendar's
   `public` role. No production composition access, no borrowing between
   scopes, no `application`, `port`, `postgres` or `domain` access for a
   foreign owner.
4. The nest itself, in its `adapter` role and its two test roles, may reach
   the School Calendar's `public` role for the parts already moved out of it.
   Nothing else about the nest's reach changes; its remaining compatibility
   rules fall with the slices that move the behaviour they cover.

The exception applies to import strictness only. It cannot change data
ownership, forgive a new debt key, expose an implementation role, permit an
external dependency, or bypass composition and runtime checks. Rules still
need to be narrow and live; the slice that empties a rule deletes it.

Epoch 22 uses it for exactly these rules:

| Scope | Source owner/role | Target owner/role |
|---|---|---|
| production | inbound-timetable/http | school-calendar/public |
| production | inbound-timetable/adapter | school-calendar/public |
| production | timetable-activities/compose | school-calendar/public |
| production | inbound-staff-shifts/adapter | school-calendar/public |
| production | enrollment/application | school-calendar/public |
| external_test | enrollment/module-behavior-test | school-calendar/public |
| internal_test | test-support/e2e-test | school-calendar/public |

The same epoch deletes the three rules slice S6 emptied:
`timetable-activities.compose.inbound-timetable-adapter`,
`timetable-activities.workflow-integration-test.inbound-timetable-adapter`
and `care-schedule-cutover.inbound-timetable.module-behavior-test.timetable-activities.public`.

### Slice S4: conflict detection and staffing (#3550)

Slice S4 moves the start and planning conflict checks, the exception
conflicts, the staff availability pool, the shift-coverage probe and the
staffing rules (capacity, understaffing, slot precedence) to the Timetable &
Activities owner. The public contract is `timetable.ConflictDetectionCapability`
(`StartConflictQuery`, `PlanningConflictQuery`, `StaffingQuery`) plus the
pure staffing and interval functions in `modules/timetable`; the
implementation sits in `modules/timetable/compose`. The replacement point of
condition 3 and 4 therefore extends to the Timetable owner's `public` role,
and only to it: its composition, application, port, Postgres and domain
roles stay closed to former consumers of the nest.

Epoch 26 uses the extended exception for exactly these rules:

| Scope | Source owner/role | Target owner/role |
|---|---|---|
| production | workforce/application | timetable-activities/public |
| production | calendar-view/adapter | timetable-activities/public |
| internal_test | workforce/module-internal-test | timetable-activities/public |
| external_test | workforce/module-behavior-test | timetable-activities/public |
| external_test | inbound-timetable/module-behavior-test | timetable-activities/public |

`api/timetable` and the nest itself already held a permission to the
Timetable contract. The same epoch deletes the four rules slice S4 emptied:
`document-rendering.adapter.inbound-timetable-adapter`,
`workforce.application.inbound-timetable-adapter`,
`workforce.internal-test.inbound-timetable-adapter` and
`workforce.behaviour-test.inbound-timetable-adapter`.

### Slice S5: timetable reads, operations and cleanup (#3551)

Slice S5 moves the planner's reads, the operational day, the retention
cleanup, the ended-session completion and the planning-track
administration to the Timetable & Activities owner. The public contracts
are `timetable.TimetableDataCapability`, `timetable.OperationCapability`,
`timetable.TimetableCleanup`, `timetable.EndedSessionCompletion` and
`timetable.PlanningTrackAdministration`, with the lifecycle clock policy,
the attendance-patch rules and the reopen gate as pure functions beside
them; the implementation sits in `modules/timetable/compose`. The
replacement point stays the Timetable owner's `public` role.

Epoch 27 uses the exception for exactly these rules:

| Scope | Source owner/role | Target owner/role |
|---|---|---|
| production | scheduler-runtime/application | timetable-activities/public |
| internal_test | scheduler-runtime/module-internal-test | timetable-activities/public |
| internal_test | test-support/e2e-test | timetable-activities/public |

`api/timetable`, the supervision dashboard and the nest already held a
permission to the Timetable contract; the command line reaches the cleanup
through the legacy composition and the kiosk session mirror through
consumer-owned ports. The same epoch deletes the seventeen rules slice S5
emptied: `calendar-view.adapter.inbound-timetable-adapter`,
`calendar-view.adapter-test.inbound-timetable-adapter`,
`process-device-scan.compose.inbound-timetable-adapter`,
`root-composition.cli.inbound-timetable-adapter`,
`test-support.e2e-test.inbound-timetable-adapter`,
`inbound-timetable.e2e-test.inbound-timetable-adapter`,
`care-schedule-cutover.inbound-timetable.module-internal-test.care-plan.public`
and the ten reach rules of the nest itself the moved files needed
(`inbound-timetable.adapter.{device-fleet-adapter,identity-access-adapter,people-directory-application,security-runtime-application,settings-platform-application}`,
`inbound-timetable.module-behavior-test.{audit-platform-postgres,settings-platform-application,settings-platform-domain}`,
`inbound-timetable.module-internal-test.{people-directory-application,settings-platform-domain}`).

### Slice S2: templates, materialization and roster maintenance (#3552)

Slice S2 moves the template writes (create, update, split, end, the
weekday rosters, the offering source and the grade-limit checks), the
materialization with its edited-occurrence detection, the roster
maintenance and the tenant recurrence gate to the Timetable & Activities
owner. The public contracts are `timetable.TemplateAdministration`,
`timetable.MaterializationCapability`, `timetable.RosterMaintenance` and
`timetable.RecurrenceWriteLock`, with `timetable.OfferingRosterResyncInput`,
`timetable.ErrOfferingSourceInvalid` and the template errors beside them;
the implementation sits in `modules/timetable/compose`. The recurrence gate
keeps its lock order: the recurrence key first, the grade-transition key
second. The replacement point stays the Timetable owner's `public` role.

Epoch 28 uses the exception for exactly this rule:

| Scope | Source owner/role | Target owner/role |
|---|---|---|
| production | enrollment/application | timetable-activities/public |

`api/timetable`, the scheduler, the nest and the timetable end-to-end flows
already held a permission to the Timetable contract; the root binds School
Structure's class rules, Enrollment's care-offering checks and the realtime
announcement through consumer-owned ports. The same epoch deletes the
thirteen rules slice S2 emptied:
`enrollment.application.inbound-timetable-adapter`,
`enrollment.module-behavior-test.inbound-timetable-adapter`,
`inbound-students.adapter-test.inbound-timetable-adapter`,
`people-directory.module-behavior-test.inbound-timetable-adapter`,
`root-composition.compose.inbound-timetable-adapter`,
`school-structure.module-behavior-test.inbound-timetable-adapter`,
`timetable-planning-cutover.inbound-timetable.adapter.school-calendar.public`
(the week-pattern decision left the nest with the materialization) and the
six reach rules of the nest itself the moved files needed
(`inbound-timetable.adapter.enrollment-domain`,
`inbound-timetable.module-behavior-test.{enrollment-application,enrollment-domain,enrollment-public,school-structure-application}`,
`inbound-timetable.module-internal-test.school-structure-domain`).

### Slice S3: deviations, substitutions and the attendance mirror (#3553)

Slice S3 moves the deviation, substitution and sick-report writes, the
substitute time-overlap advisory, the attendance correction of completed
blocks and the Student Presence attendance mirror to the Timetable &
Activities owner. The public contracts are `timetable.StaffDeviations`,
`timetable.SubstituteConflictQuery`, `timetable.AttendanceCorrections` and
`timetable.AttendanceMirror`; the implementation sits in
`modules/timetable/compose`. The mirror keeps one runtime write owner for
`schedule.instance_students`: Student Presence triggers the Timetable command
through its own attendance syncer port, bound at the composition root, in
its tenant transaction. The replacement point stays the Timetable owner's
`public` role.

Epoch 29 uses the exception for exactly this rule:

| Scope | Source owner/role | Target owner/role |
|---|---|---|
| production | shift-plan-sync/application | timetable-activities/public |

`api/timetable`, the composition root and the nest already held a
permission to the Timetable contract; the root binds the Audit Platform's
trails, the retained lifecycle and the realtime activity update through
consumer-owned ports. The same epoch deletes the five rules slice S3
emptied: `shift-plan-sync.application.inbound-timetable-adapter`,
`shift-plan-sync.compose.inbound-timetable-adapter`,
`student-presence.e2e-test.inbound-timetable-adapter` (the Presence suites
bind the mirror through the root) and the two reach rules of the nest's
behaviour suites the moved attendance tests needed
(`inbound-timetable.module-behavior-test.{inbound-timetable-test-support,transaction-runtime-domain}`).

## Verification

`internal/architecture/timetableplanning_cutover_test.go` covers the
consumer replacement (public granted; the composition and implementation
roles refused; no lending between scopes; unrelated owners and sources
without the historical permission refused), the nest's own reach (granted in
all three scopes, refused for the composition and for unrelated owners, not
extended to other roles of the same owner), the slice S4 replacement by the
Timetable contract for the Workforce, supervision dashboard and nest points
(public only; the other Timetable roles refused), the slice S5 replacement
for the scheduler and the timetable end-to-end flows (public only, each in
its own scope), the slice S2 replacement for Enrollment (public only, in
production only), the slice S3 replacement for the shift-plan-sync workflow
(public only, in production only, no other role of the workflow) and the
anchor (same or
lower epoch, a reclassified or missing nest and a foreign module all
refuse).
