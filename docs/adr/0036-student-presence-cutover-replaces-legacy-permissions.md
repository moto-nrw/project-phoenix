---
status: accepted
---

# Student Presence retirement may replace its legacy permissions

Accepted for #3422 in the implementing change, following the precedent of
ADR 0030 and ADR 0035. The pull request review is where it can still be
rejected; the exception activates only in the policy epoch that review merges.

## Context

#3422 deletes `modules/studentpresence/legacy`, the four packages #3214 moved
into a nest the architecture evaluator could not see: `services/active`
(`student-presence`/`adapter`), `models/active` (`student-presence`/`domain`),
`repositories/active` (`student-presence`/`postgres`) and `statistics`
(`student-presence`/`adapter`). The presence behaviour now sits in Student
Presence's own public contract, application, ports and Postgres adapter. The
rows the nest held for other owners went to those owners: the nine
`active.staff_*` and `active.work_session*` tables to
`modules/workforce/adapters/timerecords` and the two Care Plan tables to
`modules/careplan/absencerecords`.

The PR-mode strictness check rejects every new permission between package-role
points that exist at the base SHA. Three shapes of edge are affected.

First, 125 production consumers and 80 external test files reached the nest and
must now name the owner that holds the behaviour. `student-presence`/`adapter`
and `student-presence`/`domain` existed only for the nest, so deleting the
rules that name them — 96 when the ticket was written, 79 still standing at
the merge base — is the required outcome of the ticket, not a side effect;
converting them to exact debt is what the ticket rules out.

Second, the nest's code did not stop having dependencies when it moved. The
roles that received it — Student Presence's `public`, `application`, `port`,
`postgres`, `compose` and its test roles — keep exactly the reach the retired
packages had, and the nest's own behaviour and repository suites moved into
the packages they test, carrying their test role with them.

Third, the handed-back row types cross an owner boundary that did not exist
before. A consumer that named a Workforce row under `student-presence`/`domain`
now names it under `workforce`/`adapter`. That is the same private row package
under its real owner, not a wider grant.

ADR 0030 settled the same situation for the care-schedule cutover (#3351) and
ADR 0035 for the care-lifecycle adapter (#3427). This decision applies that
shape to the Student Presence nest.

## Decision

Permit this one cutover in a reviewed policy epoch, under these conditions:

1. The immutable base classifies all four retired packages exactly:
   `modules/studentpresence/legacy/services/active` as
   `student-presence`/`adapter` with `adapter-test` and `e2e-test`,
   `.../legacy/models/active` as `student-presence`/`domain` with
   `adapter-test` and `module-behavior-test`, `.../legacy/repositories/active`
   as `student-presence`/`postgres` with `module-internal-test` and
   `e2e-test`, and `.../legacy/statistics` as `student-presence`/`adapter`
   with `adapter-test` and `module-behavior-test`.
2. The candidate classifies no package at or below
   `modules/studentpresence/legacy`. Ordinary graph checks also reject a
   retained but unclassified source package.
3. A consumer gains access only where its own owner-role point could already
   reach one of the retired packages in the same scope, and only to one of the
   replacement points: Student Presence's `public` role; Care Plan's
   `contract` role; Workforce's `public` role and the `adapter` role holding
   the handed-back rows. In `internal_test` and `external_test` scope only,
   that same prior permission may instead reach Student Presence's `compose`
   role, so a suite that constructed the retired service constructs its native
   replacement. Care Plan's `test-support` role is reachable only from a
   `test-support` source, so the shared fixture catalog inserts the
   handed-back fixture rows and nothing else does. There is one pinned
   production exception: `facilities`/`compose` may reach
   `student-presence`/`compose`, because the retired adapter declared the
   attendance-room port and its row type, those declarations landed in
   `modules/studentpresence/compose/presenceservice`, and the port filters
   through a `map[string]any`, which the contract checks forbid on a public
   package. Facilities implements that port; it constructs nothing. No other
   production composition access, no borrowing between scopes, no
   `application`, `port`, `postgres` or `domain` access for a foreign owner.
4. The roles that received the retired code — Student Presence's `public`,
   `compose`, `application`, `port`, `postgres`, `test-support` and its four
   test roles — may reach a target one of the retired packages could reach at
   the base in the same scope. This carries dependencies, not new reach: the
   grant is refused for every target the nest did not have, for every other
   owner, and across scopes.
5. The nest's own suites moved into the packages they test, so a Student
   Presence package the candidate classifies with a test role one of the
   retired packages carried may reach the replacement points of condition 3
   in that test scope. The anchor is the nest itself: the retired packages'
   suites reached the retired packages at the base. This is how the native
   Postgres adapter's external test role became `e2e-test`, the role the
   repository suites carried in the nest.
6. Two pinned production bindings carry a handed-back row type into the
   contract that names it, because at the base both sides were one package:
   `student-presence`/`public` may name `care-plan`/`contract`, since the
   public status-day contract carries the Care Plan rows the retired domain
   package held, and `care-plan`/`contract` may name `care-plan`/`contract`,
   so the relocated rows alias Care Plan's own excused-request vocabulary
   instead of redeclaring the persisted values. Both are production only.
7. Care Plan's and Workforce's row packages inherit exactly one dependency of
   the retired packages, the canonical calendar-date value
   (`legacy-shared`/`domain`), and only where a retired package had it in the
   same scope. They received data, not behaviour, so they inherit nothing
   else.

The exception applies to import strictness only. It cannot change data
ownership, forgive a new debt key, expose an implementation role, permit an
external dependency, or bypass composition and runtime checks. Rules still
need to be narrow and live. The base package and epoch conditions prevent a
reuse after the cutover reaches the base.

Epoch 21 uses it for exactly these rules:

| Scope | Source owner/role | Target owner/role |
|---|---|---|
| external_test | inbound-timetable/e2e-test | care-plan/contract |
| external_test | inbound-timetable/module-behavior-test | care-plan/contract |
| external_test | parent-portal/workflow-decision-test | student-presence/public |
| external_test | people-directory/module-behavior-test | student-presence/public |
| external_test | people-directory/module-behavior-test | workforce/adapter |
| external_test | people-directory/module-behavior-test | workforce/public |
| external_test | student-presence/e2e-test | care-plan/contract |
| external_test | student-presence/e2e-test | student-presence/application |
| external_test | student-presence/e2e-test | student-presence/port |
| external_test | student-presence/e2e-test | workforce/adapter |
| external_test | student-presence/e2e-test | workforce/public |
| internal_test | enrollment/module-internal-test | care-plan/contract |
| internal_test | inbound-students/adapter-test | student-presence/compose |
| internal_test | scheduler-runtime/module-internal-test | workforce/public |
| internal_test | student-presence/adapter-test | student-presence/port |
| internal_test | student-presence/adapter-test | workforce/adapter |
| internal_test | student-presence/module-internal-test | care-plan/contract |
| internal_test | student-presence/module-internal-test | legacy-shared/domain |
| internal_test | workforce/adapter-test | legacy-shared/domain |
| production | calendar-view/adapter | student-presence/public |
| production | care-plan/contract | care-plan/contract |
| production | care-plan/contract | legacy-shared/domain |
| production | care-plan/test-support | legacy-shared/domain |
| production | delivery-platform/application | care-plan/contract |
| production | delivery-platform/application | workforce/public |
| production | enrollment/application | care-plan/contract |
| production | facilities/compose | student-presence/compose |
| production | facilities/compose | student-presence/public |
| production | group-live-view/adapter | care-plan/contract |
| production | inbound-common/http | student-presence/public |
| production | inbound-groups/http | student-presence/public |
| production | inbound-operator/http | student-presence/public |
| production | inbound-parent/http | care-plan/contract |
| production | inbound-statistics/http | student-presence/public |
| production | open-room-move/compose | student-presence/public |
| production | people-directory/application | care-plan/contract |
| production | people-directory/application | student-presence/public |
| production | root-composition/cli | student-presence/public |
| production | scheduler-runtime/application | care-plan/contract |
| production | scheduler-runtime/application | student-presence/public |
| production | scheduler-runtime/application | workforce/public |
| production | settings-platform/http | student-presence/public |
| production | student-presence/application | care-plan/contract |
| production | student-presence/application | delivery-platform/application |
| production | student-presence/application | legacy-shared/domain |
| production | student-presence/application | student-presence/public |
| production | student-presence/application | tenant-runtime/public |
| production | student-presence/application | transaction-runtime/domain |
| production | student-presence/application | workforce/adapter |
| production | student-presence/application | workforce/public |
| production | student-presence/public | care-plan/contract |
| production | student-presence/public | legacy-shared/domain |
| production | test-support/test-support | care-plan/test-support |
| production | test-support/test-support | workforce/public |
| production | workforce/adapter | student-presence/public |
| production | workforce/adapter | workforce/adapter |
| production | workforce/postgres | workforce/adapter |

One rule the slices had added is not in that table and was deleted before this
epoch: `legacy-composition.module-behavior-test.student-presence-compose`. It
had no historical permission to replace, so the two repository-factory tests
behind it read the composed session records through the shared fixtures
instead (`test.SessionRooms`, `test.SupervisionRowStaff`) and name no module
composition themselves.

The same epoch and the four slices before it delete the four retired package
entries and every rule that named `student-presence`/`adapter` or
`student-presence`/`domain`: 79 at the merge base, 96 when the ticket was
written, 0 in the candidate. The owner's package roles are `public`,
`compose`, `http`, `application`, `port` and `postgres`, plus `test-support`
on one fixture package (see below).

## Verification

`internal/architecture/studentpresence_cutover_test.go` covers the consumer
replacement (public granted; the implementation, composition and test-support
roles refused in production; no lending between scopes; unrelated owners and
sources without the historical permission refused), the row hand-back (Care
Plan's contract and both Workforce points granted, other roles of both owners
refused, the fixture target refused without a `test-support` source), the row
value reach (the calendar-date target granted for the row packages only, and
nothing else inherited), the retired code's reach (granted for a receiving
role, refused for a role that received nothing, for a foreign owner, across
scopes and for a target the nest never had), the moved suites (granted in the
test scopes on the candidate's own role, refused in production, refused for a
target outside the replacement points and for a foreign owner's suite), the
test-scope composition (granted in the historical scope only), the pinned
Facilities point (granted for Facilities, refused for another composition),
the pinned hand-back bindings (granted in production only, refused for other
roles of both owners) and incomplete retirement (same or lower epoch, retained
nest, retained subtree or subpackage, reclassified base, a retired package
missing from the base, foreign module path).
`scripts/backend-architecture.sh check` runs the strictness comparison against
the merge base and stays green with the baseline unchanged at 624 entries.

## Readings of the ticket this cutover settles

- The ticket's exit criterion lists `public`, `compose`, `http`,
  `application`, `port` and `postgres` as the owner's roles after the change.
  One package keeps a seventh, `test-support`:
  `modules/studentpresence/sessionrecordstest`, the BUN fixture rows of
  `active.groups` and `active.group_supervisors`. `TestDateColumnTypes` maps
  DATE columns only from bun-tagged structs under `models/` and `modules/`,
  and after the move the Postgres adapter reads plain values through
  `ModelTableExpr`, so those rows are the only mapping of
  `active.group_supervisors.start_date` and `.end_date`. Folding them into
  `backend/test` would drop the mapping and require two new
  `unmappedDateColumns` allowlist entries, which is growth on a shrink-only
  allowlist. The role is a test role on a fixture package, the shape #3422
  already accepted for `modules/careplan/absencerecordstest`, and the roles
  the criterion is about — `adapter` and `domain` — are gone from every
  Student Presence package and from every rule. Its four rules
  (`student-presence.test-support.calendar-date`,
  `test-support.fixtures.student-presence-session-rows`,
  `test-support.e2e-test.student-presence-session-rows` and the Timetable
  suite's `inbound-timetable.module-behavior-test.student-presence-session-rows`)
  need no exception: the role point is created by the candidate, so PR mode
  does not treat them as loosening.
- "All 96 rules naming those two roles are gone, not converted" is read
  literally: no rule and no `legacy.jsonl` entry replaces them. The 57 rules
  above are the consumers' new, narrower permissions on the owners that hold
  the behaviour, each of them derived from a permission the base already
  granted.
- The nest's path still appears once in the repository, as the constant
  `studentPresenceLegacyPath` in the evaluator's exception. That is the shape
  #3427 left behind for its own retired path; the exit criterion is about
  imports, and no Go file imports the module path any more.

## Consequences

This does not authorize general rule loosening; other legacy-to-owner cutovers
still need their own decision. The permissions to `workforce`/`adapter` stay
temporary and name #3413, which dissolves
`modules/workforce/legacy/timetracking` and the work-session shim; the three
Timetable suite permissions name #3424. Every other rule in the table is an
ordinary target permission on a public or contract surface and carries no
cleanup issue, as the #3351 and #3427 cutover rules do not.
