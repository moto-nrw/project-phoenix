---
status: accepted
---

# A booking change reports hand-maintained Regeltermine

Decision of 2026-10-27 for
[#5102](https://github.com/moto-nrw/project-phoenix/issues/5102). It approves
five command-effect diagnostics and raises two composition LOC budgets by six
lines. It adds no owner, no policy rule and no runtime exception.

## Context

After a booking change, the offering resync rewrites only the roster rows a
booking created (`activities.student_enrollments.enrollment_request_child_id`).
A Regeltermin whose children staff picked by hand or through a Klasse,
Jahrgang or Gruppe target keeps the child on weekdays the booking no longer
has. The office learned nothing about it.

Care Plan now answers every booking change (`AdjustOfferings`) with its
roster impact: the Regeltermine the resync rewrote and those that keep the
child on a weekday the change took away. Timetable names the latter through
the new read `timetable.StudentTemplateDays.ManualStudentPlan`. Care Plan
reaches it through its consumer port `BookingManualPlans`, bound at the root.
The office then confirms the removal per Regeltermin. Timetable runs it as
its own command `RemoveStudentTemplateDays` under the tenant recurrence gate.
It writes only Timetable's rows: the child's hand roster rows and planned
occurrences of that series.

The binding-flow scan of
[ADR 0098](0098-command-effect-binding-flow-and-platform-protocols.md) records
two kinds of diagnostics this change touches:

| Port | Diagnostic | Why |
|---|---|---|
| `careplan.OfferingChangeCapability.Decide` via `roots/owners.offeringChangeLifecycle.Decide` | `unresolved:callable:api.now`, `unresolved:callable:api.s`, `unresolved:function:…/careplan/internal/application.OfferingChangeDependencies.Today` | `Decide` returns the roster impact of an approval, so the root binding's body and signature changed. The callables are the same clock and settings values ADR 0098 approved. |
| `timetable.StudentTemplateDays.RemoveStudentTemplateDays` | `unresolved:callable:roots/owners.now`, `unresolved:function:…/timetable/ports.StudentTemplateDaysDependencies.Today` | The removal refuses a date before today. The day comes from the root clock so tests can fix it. |

An ADR already in the base cannot approve them.

## Decision

The five diagnostics cite this ADR as `approval_adr`. The callables only
supply the clock and the settings read; they reach no other owner's state,
take no lock and widen no scope. `ManualStudentPlan` is an ordinary query
entry with review evidence.

Leaving the clock out would compare the removal against the wall clock and
make the behaviour test depend on the day it runs. Returning the impact
through a second decide port would add the same diagnostics under another
name.

The roots construct the new Timetable capability through
`compose.NewStudentTemplateDays` (`modules/timetable/compose`, +5 LOC) and
hand Care Plan its port through `BookingMaterializationDependencies.ManualPlans`
(`modules/careplan/compose`, +1 LOC). Both are construction only. The budgets
and their total rise by exactly these lines.

After this ADR enters the base it cannot approve another diagnostic. A
changed clock or settings assignment changes the digest and needs fresh
approval.
