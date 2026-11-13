---
status: accepted
---

# Class and group changes follow on targeted Regeltermine

Decision of 2026-11-13 for
[#6841](https://github.com/moto-ogs/project-phoenix/issues/6841). It adds one
Timetable capability and raises two composition LOC budgets by seven lines in
total. It approves the Enrollment decision bindings of the new capability as
E1 coordination. It adds no owner and no policy rule.

## Context

A Regeltermin can plan its children through a Klasse, Jahrgang or Gruppe
target. The materializer resolves the target only when it creates an
occurrence, and saving a Regeltermin materializes its whole planning period.
A child who moved from 3a to 3b therefore stayed on every future 3a
occurrence and was missing on the 3b ones until somebody re-planned by hand.
Offering-sourced Regeltermine already follow such a change: every writer of a
child's class re-runs Care Plan's offering resync under the recurrence gate
in its own transaction (#2147).

## Decision

Timetable owns the follow-up as `timetable.CohortRosters.FollowCohortChanges`.
The caller names each child and the Klasse or Gruppe before its write.
Timetable reads the current values from the People Directory, resolves which
targeted templates the child joined or left, and diffs only those children on
the planned occurrences from the given day on. It writes only Timetable rows,
in one batch, through the writes the offering resync already uses. It keeps a
row an enrollment of the template still plans and any row recording a
hand decision or an observation. Templates whose target the child still
matches stay untouched, so a hand removal there survives.

Every writer that already runs the offering resync after a class change calls
it next, under the gates it already holds: the student edit (which now also
takes the gates for a Gruppe edit), the existing-student approval, the
approved-child edit, the planned class switch, and the grade transition's
apply and revert. Each binds the capability through its own consumer port at
the root. The command-effect inventory records Timetable's reads as queries
and its tenant protocol as platform participation. The Enrollment decision
bindings join the existing E1 decision surface of
[ADR 0067](0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md)
and cite this ADR as `approval_adr`: they run inside the decision's tenant
unit, after its offering resync, under the gates it already holds.

The roots construct the capability through `compose.NewCohortRosters`
(`modules/timetable/compose`, +5 LOC) and hand the grade transition its port
through `Dependencies.FollowClassChanges` (`workflows/gradetransition/compose`,
+2 LOC). Both are construction only. The budgets and their total rise by
exactly these lines.

## Consequences

A Klasse or Gruppe change reaches the planned future occurrences in the same
transaction. Children created later still reach only occurrences materialized
afterwards, as the roster indicator says. The master-data review and the data
import write a child's class without either follow-up; they are tracked in
[#6951](https://github.com/moto-ogs/project-phoenix/issues/6951).
