---
status: historical
---

# Time-tracking dissolution may replace its legacy permissions

> Historical record of a completed cutover. The rules that still hold
> are in [ADR 0070](0070-principle-only-the-target-is-a-rule-and-debt-leaves-with-evidence.md) and [ADR 0066](0066-principle-every-module-table-and-cross-owner-read-has-one-owner.md).

> Amended on 2026-10-15 by [ADR 0057](0057-only-the-target-is-a-rule.md): the
> replacement rules this decision allowed are not target. #4070 converts the
> reach they still grant into exact debt under its carriers (#4076 and others).

Accepted for #3413 in the implementing change, following the precedent of
ADR 0036 and ADR 0037. The pull request review is where it can still be
rejected; the exception activates only in the policy epoch that review
merges.

## Context

#3413 deletes `modules/workforce/legacy/timetracking`, the package
`54b04feb9d` moved out of `services/active` file for file: 13,327 production
LOC in 40 files and 20,529 test LOC, classified `workforce`/`adapter` with
`adapter-test` in both test scopes. The package carried no `legacy.jsonl`
key and no composition entry. Its stamping, absence, balance, month-close,
overview, export, audit-feed and cleanup services ran behind the public
`modules/workforce` contracts only through adapters in `services`, and the
public `WorkSessions`, `TimeClock`, `StaffAbsences` and `WorkTimeMonths`
contracts were implemented there, outside the module.

The services move to `modules/workforce/internal/application/timetracking`
(`workforce`/`application`, test roles `module-internal-test` and
`module-behavior-test`). The adapters that served the public contracts from
`services` (`workforce_time_tracking.go`, `workforce_time_clock.go`,
`workforce_month_snapshots.go`, `workforce_absence_types.go`,
`workforce_absence_type_reader.go`) move into the Workforce composition,
next to the `*_engine.go` files that already map the public types onto the
native application; `staff_target_override_weeks.go` joins the services.
`modules/workforce/compose.NewTimeTracking` builds the services over the
Workforce capability and the consumer-owned ports the root binds, and hands
the root the public contracts. The work-session repository shim
(`modules/workforce/legacy/worksession_repositories.go`), its five exported
constructors and every root binding of them are deleted: the mapping from the
capability onto the rows the services speak is private to `NewTimeTracking`
(`compose/worksession_rows.go`); the scheduler and the staff recipient
resolver read today's presence map from the capability itself, and the
suites reach the rows through `compose.NewTimeTrackingTestRows`
(`time_tracking_test_helpers.go`). Nothing in it issues SQL.

The PR-mode strictness check rejects every new permission between
package-role points that exist at the base SHA. The code did not stop having
dependencies when it moved: Workforce's `application` and `compose` roles and
the test roles of the moved suites now hold the imports the retired package
held (the `timerecords` rows, the public Student Presence session records,
the audit, transaction, timetable and settings vocabularies of its suites).
No consumer needs a replacement point: every consumer already reached
Workforce's `public` and `compose` roles.

## Decision

Permit this one cutover in a reviewed policy epoch, under these conditions:

1. The immutable base classifies `modules/workforce/legacy/timetracking`
   exactly as `workforce`/`adapter` with the test roles `adapter-test` and
   `adapter-test`.
2. The candidate classifies no package at that path or below it. Ordinary
   graph checks also reject a retained but unclassified source package.
3. The Workforce roles that received the retired code, `application`,
   `compose`, `module-internal-test` and `module-behavior-test`, may reach a
   target the retired package could reach at the base in the same scope. This
   carries dependencies, not new reach: the grant is refused for every target
   the package did not have, for every other owner and role, and across
   scopes.
   The grant is keyed by owner and role, as in ADR 0037, so every Workforce
   package in those roles inherits that reach, not only the moved package;
   the candidate's own rules still have to name each edge.

The exception applies to import strictness only. It cannot change data
ownership, forgive a new debt key, expose an implementation role, permit an
external dependency, or bypass composition and runtime checks. Rules still
need to be narrow and live. The base package and epoch conditions prevent a
reuse after the cutover reaches the base.

Epoch 40 uses it for `workforce.application.student-presence-public`,
`workforce.application.time-records`, `workforce.compose.time-records` and
the six `workforce.behaviour-test.*` rules of the moved suites
(`audit-domain`, `transaction-domain`, `timetable-domain`,
`settings-application`, `settings-support`, `time-records`). The same epoch
removes the nine rules the retired package alone used (three deleted, six
rewritten as the rules above) and the package entry.
The three `*.time-records` rules are temporary and name #3996, which retires
the `timerecords` vocabulary; the six rules that named #3413 for that
vocabulary are reassigned to #3996.

Policy epoch 40 applies this cutover after the guardian portal HTTP rules
entered development in epoch 39 (ADR 0051). Both exceptions remain bounded
by their existing selectors.

## Verification

`internal/architecture/timetracking_cutover_test.go` covers the retired code's
reach (granted for each receiving role in its scope, refused across scopes,
for roles that received nothing, for foreign owners and for targets the
package never had), the moved suites (granted on the receiving test roles,
refused in production and for foreign owners) and incomplete retirement
(same epoch, retained package or subpackage, reclassified base, foreign module
path). `scripts/backend-architecture.sh check` runs the strictness comparison
against the merge base and stays green with the baseline unchanged at 86
entries.

## Readings of the ticket this cutover settles

- The ticket names `modules/workforce/internal/application` as the target.
  The services live in its subpackage `internal/application/timetracking`
  rather than in the native CRUD package itself, because that package speaks
  the domain types and the moved services the `timerecords` rows; one package
  holding both would merge two vocabularies. The subpackage is scanned by the
  pass-through ratchet: the 70 counted adapter methods went to the
  composition, two unused reads were deleted, and the Workforce budget rises
  by the 15 service reads that keep the counted shape (47 to 62), as Care
  Plan's did with #3427.
- The shim's mapping survives as private code of the composition; its file,
  constructors and bindings are gone. The ticket itself establishes that it
  is no second write path, and the mapping goes with the rows it maps onto
  (#3996).
- The ticket's seven flows land in one cut. They share one constructor graph
  and one dependency set (`timetracking.New`); splitting them would have kept
  a half of the services in the retired package behind the same adapters.
- "Exports become a named tenant-safe read projection": the export and the
  overview issue no SQL of their own and read through the same
  consumer-owned ports as before, so a projection owner would grant tables
  nobody joins. None is added, as in ADR 0037.
- "The edit audit is a Workforce write": `audit.work_session_edits` stays
  with `audit-platform`. The exit criterion requires unchanged policy
  ownership, so the services keep writing it through the `WorkSessionAudit`
  port the root binds to the audit repository.
- The ticket counts 15 files importing `modules/studentpresence/legacy/models/active`.
  #3422 already replaced that import with the Workforce-owned `timerecords`
  rows and the public Student Presence records; nothing in the moved package
  imports a legacy model package.

## Consequences

This does not authorize general rule loosening; other legacy-to-owner
cutovers still need their own decision. The rules added under it that reach
`workforce`/`adapter` are temporary and name #3996; the Student Presence rule
is an ordinary target permission on a public surface and carries no cleanup
issue.
