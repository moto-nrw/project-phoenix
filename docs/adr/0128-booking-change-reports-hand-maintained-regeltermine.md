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

A booking now carries a Regeltermin on its weekday when the offering fits the
Regeltermin's kind and the Regeltermin ends no later than the offering's
pickup time that day. The kind is the one the roster review (#5103) uses,
`RosterOfferingKindFor`: a Mensa list takes offerings with lunch, another
care block offerings that count as care, an AG course offerings, anything
else every offering. Care Plan names the weekdays on which the change drops a
booking, those on which it drops more offerings than it newly books (net
loss), and the bookings that remain, with their kind flags and pickup times.
Timetable reports a hand-maintained Regeltermin on a dropped weekday when no
remaining booking carries it, also if none carried it before.

The end of a Regeltermin is the end of its occurrence, or of its schedule for
a hand roster row; the latest end of the weekday counts. A schedule names a
weekday and an end only when it is valid on a date of that weekday from the
Stichtag on, by materialization's own rule `scheduleValidOn` (validity dates,
the series' last day, the planning period and the A/B week), and when one of
the child's hand roster rows of the template is valid on that same date by
materialization's `isEnrollmentValidOn` (its validity, period and weekday
scope). A schedule whose validity or period has ended, or that never meets
the child's row, plans nothing the change touches.

The times decide only when the Regeltermin has an end and every remaining
booking of the day has a pickup time. Otherwise the day keeps the first rule
exactly, whatever the kinds: the Regeltermin counts as lost when the change
drops more offerings on the day than it newly books. Schools book a
Randstunde and Ganztag variants without pickup times; there the first
rule's notices stay as they were. A day without any remaining booking is a
net loss under both rules.

Decided for #5342: where the bookings of the day set pickup times, a
Randstunde dropped beside a Ganztag that stays is not reported: the Ganztag
carries the class Randstunde, like the Ganztag children the class list
already holds. The rule assumes that the booked care of a day runs without a
gap until the pickup time. A care offering before
lessons, such as a Frühbetreuung, would break that assumption; no school
books one yet.

The binding-flow diagnostics of this ADR are unchanged. The read gained query
fields and Timetable dependencies on its timeframes and active calendar
periods, not a callable; the root binding that maps Care Plan's day change
onto the query is reviewed again as a query. Materialization's per-date rule
`candidateSlot` now calls `scheduleValidOn` for its validity part, without a
change in behaviour.

## Addendum 2026-10-31: the student's other enrollments carry as well (#5366)

A student may hold approved enrollments of several phases at once, for
example a custom phase for existing students beside the school year. The
first addendum read only the bookings of the changed enrollment, so an
unchanged booking of another enrollment that carries the Regeltermin did not
prevent the notice.

Care Plan now also names the bookings of the student's other approved
request children, read through Enrollment's existing approved-selection
projection (`ApprovedSelectionsForStudents`) behind its consumer port
`BookingEnrollment.StudentSelectionsAt`. A selection counts when it is in
force on the first rewritten day, its phase's service window holds that day
and its offering is active. Timetable receives them as `OtherBookings` of
the query.

Such a booking carries a Regeltermin by the same rule: a fitting kind and a
pickup time no earlier than the end. Decided for #5366: the times decide only
when the Regeltermin has an end and every remaining booking of the child's
day has a pickup time, in any enrollment. Where one of them lacks it, the day
keeps the earlier rule across all enrollments, whatever the kinds: the
Regeltermin counts as lost when the change drops more offerings on the day
than it newly books. An enrollment the change leaves as it is adds nothing to
that balance. A booking without a pickup time therefore decides alike in the
changed and in another enrollment.

The decision keeps the known limit of the earlier rule: a net loss reports a
Regeltermin although another booking of the day provably carries it, when a
pickup time of that day is missing. With a single enrollment and in schools
without pickup times every notice stays as it was. No school has children
with parallel approved enrollments today, and none mixes offerings with and
without pickup times (read-only production check of 2026-10-31).

The read adds no owner edge, no write and no binding-flow diagnostic.

## Addendum 2026-11-01: the report stays until somebody decides (#5102)

A pre-deploy test on a copy of production data showed the gap the first
decision left: the list exists only in the save response. A Korrektur took a
child's Thursday away, nobody clicked a button, and after a reload the child
stayed on two hand-maintained Regeltermine every Thursday while the
deviation list, the gap list and the home page were empty.

Timetable now derives the report again from what is stored, in the read
`GET /api/timetable/templates/booking-leftovers` (pure rule
`timetable.DetectBookingLeftovers`). Its anchor is a booking link that ended
by a change or a withdrawal: Care Plan's existing period read
`OfferingWeekdayBookings`, asked from the period start, marks such a window
`Ended` when the link's own end, not its phase, bounds it. For every hand
roster row (no `enrollment_request_child_id`) the rule takes the row's
first occurrence of that weekday from the later of the booking end and
tomorrow on. The occurrences are materialization's own projection
(`MaterializationCapability.ProjectOccurrences`, as for the offering gaps of
#5343): holidays, closing days, exceptions, A/B weeks and the series' end
are already applied, the rule computes no dates of its own. Whether the row
fills an occurrence is materialization's roster rule (`appliesOn`). On that
occurrence the carry rule of the first addendum decides with the bookings
in force; where a pickup time or the end is missing, the day counts as lost
when it has fewer offerings on the weekday than the day before the booking
ended. Of several ended bookings of a weekday, the earliest such day
without a carrying booking starts the report. A child the
Regeltermin plans through a Klasse, Jahrgang or Gruppe target is left out:
removing the hand row would not hold, and the deviation hint covers those
lists. A Regeltermin the deviation hint already shows is left out as well.
Nothing is stored for the report.

The school's answer is the only stored part. "Entfernen" is
`RemoveStudentTemplateDays` from that first planned day.
"Bleibt so" is Timetable's new command `KeepStudentTemplateDays`: it writes
the latest booking end of the reported days, per ISO weekday, into the new JSONB column
`activities.student_enrollments.kept_without_booking` of the child's hand
rows of the series, for example `{"4": "2026-11-02"}`: every hand row of
the series that plans the weekday, so the row of each gap carries it. It
writes the asked weekdays key by key inside the row's UPDATE, so two
decisions on the same row never replace each other, and it takes the
tenant recurrence gate like the commands that rewrite rows. A decision on
or after a weekday's booking end hides that weekday's report; a later
booking end shows it again; other weekdays keep their own decision, also on
undo. It changes no plan. The commands that rewrite hand rows carry the
decision onto the rows that replace them: saving a series, editing its
roster from a date, splitting it and the continuation rows of a removal.
Per student and weekday a decision survives only when every replaced row
that plans the weekday carried one, with the earliest date, so a rewrite
never hides more than before. Booking-created rows never carry it.

`RemoveStudentTemplateDays` now starts tomorrow at the earliest, also when
asked for today: today's occurrences and rows stay as they are. The command
`KeepStudentTemplateDays` reads no clock, so it adds no binding-flow
diagnostic; the read reaches no other owner beyond the existing ports.

## Addendum 2026-11-01: "Entfernen" covers the gap only (#5497)

A report had a start and no end. When the same offering was booked again
later, for example a Thursday booking that ends on 02.11. and starts again
on 16.11., "Entfernen" took the child off every Thursday from 05.11. on,
also from 19.11., where the new booking carries the Regeltermin again. The
dialog named only the start, so nobody could see it.

Decided (product decision of 2026-11-01): "Entfernen" covers the gap until
a later booking carries the Regeltermin again, and the screens name that
period. The report gets an end, `until` (exclusive): the start of the
latest booking in force on the next planned day of the weekday that a
booking carries again, but after the last planned day without one. Without
such a day the end stays open, as before. Weekdays of one Regeltermin and
child share a report only when their gaps start and end on the same days.

`RemoveStudentTemplateDays` takes the optional end. It ends the hand rows at
the start as before and continues the removed weekdays with weekday-scoped
rows from the end on; occurrences from the end on keep the child. A row
that starts on or after the end stays as it is. An end on or before the
start is refused.

The save response follows the same rule. It used to see only the bookings
in force on the change's date. Care Plan now also hands over bookings of the
student's other enrollments that start within the next school year, each
with its start. They do not carry the change's date. For a lost weekday the
first such start from which the carry rule holds again ends the gap; where
the times do not decide, a later booking of the weekday offsets the dropped
one, like a switch on the change's date. A series whose weekdays end at
different days is listed once per end, so each removal takes nothing a
booking carries. A booking change rewrites the changed enrollment from its
date on as a whole, so only other enrollments can start later there.
