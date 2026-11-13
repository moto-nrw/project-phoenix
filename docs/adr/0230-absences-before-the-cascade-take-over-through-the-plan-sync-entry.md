---
status: proposed
---

# Absences before the cascade take over through the plan-sync entry

Decision of 2026-11-13 for
[#7021](https://github.com/moto-ogs/project-phoenix/issues/7021). It approves
one new command-effect workflow binding and adds no owner, no policy rule and
no new kind of runtime exception.

## Context

Since [ADR 0229](0229-approved-and-withdrawn-absences-run-through-the-plan-sync-entry.md)
every absence in force cascades into Dienstplan and Betreuungsplan when it is
written. An absence approved or reported before that change left the shifts
and blocks planned at that time alone; only rows planned later take its
effect. A vacation approved in October for December thus shows the person in
the Dienstplan while newly materialized blocks of the same days show them
absent.

Running the cascade over every school at deploy time would add "Abwesenheit"
entries to the Änderungsprotokoll the Leitung did not cause, on days the
Leitung may already have planned by hand.

## Decision

The Leitung decides. `GET /api/staff/absences/plan-gaps` lists the absences in
force whose shifts and blocks from today on still lack their effect, with the
number of each; it is the dry run of `SickCascade.MarkSickForRange` and reads
set-based. `POST /api/staff/absences/{absenceId}/plan-sync` takes one absence
over: the Shift Plan Sync entry takes the locks of an absence write
(`LockSickReportWrite(0, absenceID, …)`, lock order of
[ADR 0186](0186-shift-writes-read-sick-reports-through-the-workforce-observer.md)
and #6232 unchanged) and re-applies `MarkSickForRange` to the stored absence,
in the request's tenant transaction. One absence per request: the runtime
lock-order guard rejects the locks of a second absence after the first one's
day locks, and a batch transaction would hold every day lock of the school.
The screen takes several absences over one after the other. An absence no
longer in force or not of the school is skipped.

The cascade keeps every hand decision: a cancelled shift, a cover, an already
absent block row and past days stay as they are. Both routes take
`vacation:approve` or `time_tracking:manage`, the permissions whose writes
cascade absences today.

| Port | Binding |
|---|---|
| `timetracking.AdminAbsenceWrites.SyncAbsencePlan` | `shiftplansync.SickReports.SyncAbsencePlan` |

The read `timetracking.AdminAbsenceWrites.AbsencePlanGaps` is classified
`query`. Both cite this ADR. The read needs the block assignments of several
staff members over a date range in one statement; Timetable's
`TimetableInstanceStaffReads.FindByStaffIDsAndDateRange` serves it, which
raises the pass-through budget of `modules/timetable` and the total by one.

After this ADR enters the base it cannot approve another binding.
