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

## Verification

`internal/architecture/timetableplanning_cutover_test.go` covers the
consumer replacement (public granted; the composition and implementation
roles refused; no lending between scopes; unrelated owners and sources
without the historical permission refused; the Timetable owner deliberately
not granted by this slice), the nest's own reach (granted in all three
scopes, refused for the composition and for unrelated owners, not extended
to other roles of the same owner) and the anchor (same or lower epoch, a
reclassified or missing nest and a foreign module all refuse).
