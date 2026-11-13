---
status: proposed
---

# Approved and withdrawn absences run through the plan-sync entry

Decision of 2026-11-13 for
[#6847](https://github.com/moto-ogs/project-phoenix/issues/6847). It approves
two new command-effect workflow bindings and updates the reviewed effects of
the materializer's absence read. It adds no owner, no policy rule and no new
kind of runtime exception.

## Context

Only a full-day sick report reached the Dienstplan and the Betreuungsplan. An
approved vacation, a training, comp time, "Sonstige" and the school's own
Abwesenheitsarten left shifts planned and blocks staffed, so the staffing
plans were wrong.

[ADR 0099](0099-domain-applications-no-longer-call-workflows.md) made the
absence writes that cascade into the plans HTTP transport entries: the
consuming `modules/workforce/http/timetracking` package declares the port
(`OwnAbsenceWrites`, `AdminAbsenceWrites`), and the root binds the Shift Plan
Sync `SickReports` entry. That entry takes the absence and plan locks, runs
Workforce's write and applies the `SickCascadePlan` the write returns in one
tenant transaction. A request takes plan effect only when a manager approves
it, and loses it when the staff member withdraws the approved request; until
now both writes ran outside the entry.

| Port | Binding |
|---|---|
| `timetracking.AdminAbsenceWrites.ApproveAbsence` | `shiftplansync.SickReports.ApproveAbsence` |
| `timetracking.OwnAbsenceWrites.CancelAbsence` | `shiftplansync.SickReports.CancelAbsence` |

## Decision

Every absence in force cascades: a reported entry at once, a request with its
approval. `workforce.PlanEffectiveAbsenceStatuses` names the two statuses.
The approval and the withdrawal join the existing `SickReports` writes under
the same ADR 0099 shape: Workforce returns the plan, the entry applies it
fail-closed. Both bindings cite this ADR as `approval_adr`.

The approval names no subject, so the entry's lock reads it from the stored
absence (`lockFor(0, absenceID, …)`). The lock order of
[ADR 0186](0186-shift-writes-read-sick-reports-through-the-workforce-observer.md)
and #6232 is unchanged.

The materializer's read (`roots/owners.timetableSickReports.OpenSickReports`)
now lists every absence type with plan effect. Its two new value helpers,
`PlanEffectiveAbsenceStatuses` and `StaffAbsence.HalfBoundaryDays`, are
classified `query`: no database access, no lock.

The plans name only the kind: a sick report keeps "Krankheit" and
"Krankmeldung", every other absence writes "Abwesenheit" and logs the
ordinary `absence` and `return_to_presence` events. A school's own art name
and the note never reach the team-visible plans. A single half day still
never cascades, as decided for #1843.

After this ADR enters the base it cannot approve another binding.
