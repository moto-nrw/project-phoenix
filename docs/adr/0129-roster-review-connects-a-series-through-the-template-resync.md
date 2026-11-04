---
status: accepted
---

# Roster review connects a series through the template resync

Issue [#5103](https://github.com/moto-nrw/project-phoenix/issues/5103) lets a
school switch a hand-kept Regeltermin to a Betreuungsangebot as source with
"Mit Angebot verbinden". The switch must reach every segment of a split series
that still has days on or after the chosen date: Schule am Berg had series
whose current segment ends on 30.10. while the successor starts that day.

## Decision

1. The Timetable owner offers the switch as its own command,
   `timetable.TemplateRosterReview.ConnectOfferingSource`, next to the reads
   that detect the deviation and preview the switch. It converts each future
   segment in place: target and source columns, the hand roster from the
   boundary on (history before it stays), then Care Plan's sourced-roster
   resync, then the already materialized planned occurrences. No segment is
   split and no scalar field changes.
2. The command participates Care Plan's guarded resync and care-offering
   checks exactly as the template update does (ADR 0067 addendum, command
   participation): the resync joins the route's tenant transaction, the
   command holds Timetable's recurrence gate before the first write, and Care
   Plan keeps its own validation. The command runs only inside the request's
   tenant transaction; it opens no unit of its own, so no second closure flows
   into the template writes' transaction helper.
3. The `backend/architecture/command-effects.json` entries of the new
   binding take the classification of the identical template-update entries:
   the two `ResyncTemplateOfferingRoster` targets stay `debt` (the
   coordination review ADR 0098 left open covers both commands), and the
   function-valued `ResyncOfferingRoster`, `ValidateOfferingSource` and
   `ValidateSeries` boundaries stay `unresolved`. This ADR is their approval;
   a changed closure needs a new one.
4. The read `OfferingSourceSupport.OfferingWeekdayBookings` is a query of
   Care Plan's approved bookings with their weekdays. Its budget is
   `services.enrollment.offering_weekday_bookings.reads` (6); the deviation
   list `api.timetable.templates.roster_deviations` stays flat at 16.

## Consequences

The protocol review of the template resync, still open since ADR 0098, now
covers two commands with the same shape. Moving the coordination into a
workflow later moves both together. No policy rule, baseline or ratchet budget
changes.
