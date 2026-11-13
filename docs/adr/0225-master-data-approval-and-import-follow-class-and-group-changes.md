---
status: accepted
---

# Master-data approval and import follow class and group changes

Decision of 2026-11-13 for
[#6951](https://github.com/moto-ogs/project-phoenix/issues/6951), the
follow-up of [ADR 0224](0224-class-and-group-changes-follow-on-targeted-regeltermine.md).
It adds no owner, no capability and no policy rule. It raises the
composition LOC budget of `modules/careplan/compose` by three lines and
approves the new command effects of the Stammdaten decision.

## Context

ADR 0224 let every writer of a child's Klasse or Gruppe re-plan the
Regeltermine in its own transaction, except two: the approval of a parent's
Stammdaten request (single, bulk and conflict resolution with a staff value)
and the student import in update or upsert mode. Both wrote the new Klasse
(the import also the Gruppe) and left the child on the old occurrences and
off the offering-sourced and targeted Regeltermine of the new one.

## Decision

The Stammdaten decision takes the Klasse follow-up through its own
consumer port `MasterDataClassChanges` (Lock, Follow), which the root binds
to Care Plan's `LockOfferingDerivedWrites` and, after the write, to the
offering resync and `timetable.CohortRosters.FollowCohortChanges`. A request
whose target and field name the Klasse takes the gates before its child row:
the single decision and the conflict lock when they lock the request's
child, the bulk approval before its child frontier
(`MasterDataBulkPort.LockBulkClassChanges`). Requests for other fields take
no gate. The follow-up runs only when the write changed the Klasse; a bulk
approval collects its children and follows them in one call after its last
decision (`MasterDataBulkPort.FollowBulkClassChanges`), so the offering
resync runs once per bulk.

The command-effect inventory records the decision's recurrence locks as
lock debt and its unresolved function values under this ADR, the resync's
transaction runner as platform participation, and the cohort follow-up as
E4: ADR 0067 names the master-data approval in the E4 Care surface, and the
follow-up preserves its class, recurrence and child order.

The student import declares its own ports in `StudentImportDeps`
(`LockCohortWrites`, `FollowCohortChanges`). An import config may implement
`ports.RowWriteFollowUp`: the batch attempt calls it before its first row
write and finishes it after its last row, before the checkpoint, in the
attempt's transaction. The student config takes the gates for update and
upsert attempts only, collects every child whose Klasse or Gruppe a row
changed, and follows them in one call per batch: each batch commits on its
own, so a run of several batches follows once per batch. A failed row write
ends the attempt, so every collected change commits with its follow-up.

The roots construct the decision's port in `roots/owners` and the import's
in `api`; the Care Plan composition re-exports the port type and passes the
field through (`modules/careplan/compose`, +3 LOC). The budget and its total
rise by exactly these lines.

## Consequences

Every writer of a child's Klasse or Gruppe now re-plans the Regeltermine in
the same transaction. A Stammdaten approval of a Klasse and an update import
wait for running timetable writes on the recurrence gate. A create import
takes no gate: new children reach only occurrences materialized afterwards.
