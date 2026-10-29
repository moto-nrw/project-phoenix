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
child without a booking that carries them. Timetable names the latter through
the new read `timetable.StudentTemplateDays.ManualStudentPlan`. Care Plan
reaches it through its consumer port `BookingManualPlans`, bound at the root.
Which booking carries a Regeltermin follows from the pickup time
([addendum for #5342](#addendum-2026-10-29-a-booking-carries-a-regeltermin-until-its-pickup-time-5342)).
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

## Addendum 2026-10-29: a booking carries a Regeltermin until its pickup time (#5342)

The first rule counted a weekday as taken away when the change dropped more
offerings on it than it newly booked. In production it reported class
Randstunden (11:45 to 12:45) as left behind when a child dropped Ganztag and
kept the Randstunde. Those class lists hold every child of the class,
including the children who book only Ganztag: Ganztag care runs past 12:45.
The rule also hid a real loss when a switch offset it, for example Ganztag
until 16:00 switched to 14:30 beside a hand-kept block until 16:00.

A booking now carries a Regeltermin on its weekday when the Regeltermin ends
no later than the offering's pickup time that day. A care Regeltermin needs
an offering that counts as care; other Regeltermine take any offering. The
lunch flag does not decide for Mensa lists, because schools keep Mensa lists
without flagging an offering with lunch. Care Plan names the weekdays on
which the change drops a booking, those on which it drops more offerings
than it newly books (net loss), and the bookings that remain, with their
pickup times. Timetable reports a hand-maintained Regeltermin on a dropped
weekday when no remaining booking carries it, also if none carried it
before. The end of a Regeltermin is the end of its occurrence, or of its
schedule for a hand roster row; a schedule that ended before the date gives
no end, and the latest end of the weekday counts.

When a fitting remaining booking has no pickup time, or the Regeltermin has
no end, the times cannot decide. Such a day keeps the first rule: the
Regeltermin counts as lost only on a net-loss weekday. Schools book a
Randstunde and Ganztag variants without pickup times; there a booking
without a pickup time proves nothing about the end of care, and treating it
as carrying would drop notices the first rule gave. A remaining booking
with a pickup time that reaches the end still carries the Regeltermin.

Decided for #5342: where both offerings set pickup times, a Randstunde
dropped beside a Ganztag that stays is not reported: the Ganztag carries the
class Randstunde, like the Ganztag children the class list already holds. The rule assumes that the booked care
of a day runs without a gap until the pickup time. A care offering before
lessons, such as a Frühbetreuung, would break that assumption; no school
books one yet.

The binding-flow diagnostics of this ADR are unchanged. The read gained query
fields and a Timetable dependency on its timeframes, not a callable; the
root binding that maps Care Plan's day change onto the query is reviewed
again as a query.
