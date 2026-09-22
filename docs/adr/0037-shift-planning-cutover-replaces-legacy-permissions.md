---
status: accepted
---

# Shift planning retirement may replace its legacy permissions

Accepted for #3418 in the implementing change, following the precedent of
ADR 0030, ADR 0035 and ADR 0036. The pull request review is where it can
still be rejected; the exception activates only in the policy epoch that
review merges.

## Context

#3418 deletes `modules/workforce/legacy/shiftplanning`, the package #3219
moved the staff-shift cluster into file for file: 5,336 production LOC in 17
files and 9,106 test LOC in 20 files, classified `inbound-staff-shifts`/
`adapter` although `inbound-staff-shifts` is an inbound HTTP owner whose only
other package is `api/staff-shifts`. The package carried no `legacy.jsonl`
key; its 37 `inbound-staff-shifts.*` rules and the 7 rules naming
`inbound-staff-shifts`/`adapter` as a target were compatibility permissions
that hid every import edge from the ratchet. Deleting them is the outcome the
ticket exists for; converting them to exact debt is what it rules out.

The behaviour now sits with three owners:

- The shift, series, move, lock, planning facade, shift-type, assignment and
  Dienstplan-overview services are Workforce commands and queries in
  `modules/workforce/internal/planning` (`workforce`/`application`), bound
  by `modules/workforce/compose` behind the public `StaffShiftPlanning`,
  `ShiftTypeAdministration`, `StaffAssignmentQuery` and
  `StaffScheduleOverviewQuery` contracts. The retained `models/schedule`
  Dienstplan repository contracts and the adapter that served them over the
  Workforce facade (`database/repositories/workforce_shift_repositories.go`)
  are deleted; the planning package declares its own row ports and the
  compose package serves the retained rows to the two readers that still
  speak them (the Timetable coverage probe and staff pool, the staff calendar
  feed).
- The Tagesinformationen service and its route are Timetable's:
  `users.staff_notices` and `users.staff_notice_acks` were already recorded
  with `write_owner: timetable-activities` and persisted by the native
  repository in `modules/timetable/compose`, so the service joins it there
  behind the public `timetable.StaffNotices` contract and the route moves to
  `modules/timetable/compose/httpadapter`. The three-owner split the ticket
  names as not an outcome is closed by moving the code to the recorded owner,
  not by changing the write owner.
- The #1843 sick cascade and the schedule substitution moves cross an
  ownership line inside one tenant transaction (`schedule.staff_shifts` is
  Workforce's, `schedule.instance_staff` and `schedule.activity_instances`
  are Timetable's). They are the application workflow
  `workflows/shiftplansync` (owner `shift-plan-sync`, kind `workflow`). The
  absence service reaches the cascade through the port Workforce now declares
  publicly (`workforce.ShiftPlanSync`), the substitution module through the
  port `services/education` still owns; the two composition bridges in
  `services` that joined the halves are deleted, not renamed.

The PR-mode strictness check rejects every new permission between package-role
points that exist at the base SHA. Two shapes of edge are affected. First,
every consumer that imported the retired package must now name the owner that
holds the behaviour. Second, the code did not stop having dependencies when it
moved: the roles that received it keep the reach the retired package had. The
workflow's points are created by the candidate and need no exception.

## Decision

Permit this one cutover in a reviewed policy epoch, under these conditions:

1. The immutable base classifies `modules/workforce/legacy/shiftplanning`
   exactly as `inbound-staff-shifts`/`adapter` with the test roles
   `module-internal-test` and `module-behavior-test`.
2. The candidate classifies no package at that path or below it. Ordinary
   graph checks also reject a retained but unclassified source package.
3. A consumer gains access only where its own owner-role point could already
   reach the retired package in the same scope, and only to one of the
   replacement points: Workforce's `public` role and Timetable's `public`
   role. In `internal_test` and `external_test` scope only, that same prior
   permission may instead reach the owner's `compose` role, so a suite that
   constructed the retired service constructs its native replacement. No
   production composition access, no borrowing between scopes, no
   `application`, `port`, `postgres`, `domain` or `http` access.
4. The roles that received the retired code, Workforce's `application` and
   `compose` with the `module-internal-test`, `module-behavior-test` and
   `workflow-integration-test` roles of their suites, and Timetable's
   `compose` with its `workflow-integration-test` and `module-behavior-test`
   roles, may reach a target the retired package could reach at the base in
   the same scope. This carries dependencies, not new reach: the grant is
   refused for every target the package did not have, for every other owner,
   and across scopes.

The exception applies to import strictness only. It cannot change data
ownership, forgive a new debt key, expose an implementation role, permit an
external dependency, or bypass composition and runtime checks. Rules still
need to be narrow and live. The base package and epoch conditions prevent a
reuse after the cutover reaches the base.

Epoch 22 uses it for the rules listed in the implementing pull request and
recorded in `backend/architecture/policy.json` under the ids
`workforce.application.*`, `workforce.compose.*`, `workforce.behaviour-test.*`,
`workforce.internal-test.*`, `timetable-activities.compose.*`,
`timetable-activities.integration-test.*` and the consumer replacements
`document-rendering.adapter.workforce-public`,
`document-rendering.adapter-test.workforce-compose` and
`workforce.adapter-test.workforce-compose`. The same epoch deletes the 44
rules that carried #3418, the retired package entry, and the two bridges.

## Verification

`internal/architecture/shiftplanning_cutover_test.go` covers the consumer
replacement (public granted for both owners; every other role refused in
production; no lending between scopes; unrelated owners and sources without
the historical permission refused), test-scope composition (granted in the
historical scope only; the application refused), the retired code's reach
(granted for each receiving role in its scope, refused for roles that
received nothing, for foreign owners and for targets the package never had),
the moved suites (granted on the test roles, refused in production) and
incomplete retirement (same epoch, retained package or subpackage,
reclassified base, foreign module path). `scripts/backend-architecture.sh
check` runs the strictness comparison against the merge base and stays green
with the baseline unchanged at 586 entries.

## Readings of the ticket this cutover settles

- Item 3 asks for a named tenant-safe read projection for the timetable and
  room facts of the Dienstplan overview. The overview issues no SQL of its
  own: it reads the Timetable, Facilities and People Directory facts through
  consumer-owned reader ports the composition root binds to the owners'
  retained repositories, exactly as before. A projection owner would grant
  tables nobody joins; none is added. The plan export names the overview
  through Workforce's public `StaffScheduleOverviewQuery` and declares its
  own narrow sources for the rows it still reads itself.
- The Dienstplan rows still speak the retained `models/schedule` structs
  (`StaffShift`, `StaffShiftSeries`, `StaffShiftSeriesException`,
  `ShiftType`) inside the Workforce planning package, because the Timetable
  coverage probe and staff pool in `modules/timetable/legacy/timetableplanning`
  share that vocabulary and the ticket forbids pulling those files into
  Workforce. The repository contracts are gone; the structs go with #3424.
- The substitution port stays in `services/education` (`school-structure`/
  `application`) until #2742 dissolves that package. The workflow's rule to it
  is temporary and names #2742; the workflow's rules to
  `modules/timetable/legacy/timetableplanning` name #3424, as every other
  consumer's do.

## Consequences

This does not authorize general rule loosening; other legacy-to-owner cutovers
still need their own decision. The rules under this exception that reach
`inbound-timetable`/`adapter` stay temporary and name #3424; the workflow's
rule to `school-structure`/`application` names #2742. Every other rule added
under it is an ordinary target permission on a public or compose surface and
carries no cleanup issue, as the #3351, #3427 and #3422 cutover rules do not.
