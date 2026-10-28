---
status: accepted
---

# Offering-gap decision lives on the care offering

Issue [#5140](https://github.com/moto-nrw/project-phoenix/issues/5140): at
Schule am Berg 46 children of Klasse 3 and 4 booked „Randstunde“, but no
Regeltermin took them over. The Betreuungsplan now names an offering whose
booked children no Regeltermin of the planning period covers, and the school
may decide „Kein Regeltermin nötig“ so the hint disappears (decision
2026-10-28, option A).

## Decision

1. Detection is a Timetable read. `timetable.DetectOfferingGaps` compares
   Care Plan's approved bookings (`OfferingWeekdayBookings`) with the
   period's Regeltermine per offering × child × school day (Monday to
   Friday), date by date. The check looks at an active planning period only,
   from today (or the period start) to the period end. A booked date is
   covered when a Regeltermin the offering feeds runs on that date: its
   source rule with the Jahrgang or Klassen filter, or the offering's own
   link. "Runs" means the date lies between the schedule's first and last
   occurrence in the period, its split bounds and its series end; weeks an
   A/B pattern or a closing day skips inside that stretch count as covered.
   Each booking link brings its own validity and weekdays, so a series that
   only runs in November never covers a booking in December. Children on a
   hand-kept list or in another offering's Regeltermin stay uncovered; the
   roster review (ADR 0129) handles hand-kept lists.
2. The school's decision is stored on the offering row,
   `enrollment.care_offerings.template_gap_acks` (JSONB, planning period id
   → classes, NULL = no decision), as the classes the hint named in that
   period when the school decided. Care Plan owns the row and writes the
   period's entry through its catalog command
   `CareOfferingTemplateGapDecision.DecideOfferingTemplateGap`; Timetable
   reaches it through its consumer port `OfferingTemplateGapDecision`, bound
   to the catalog at the root. The command effect is an ADR 0067 leaf: one
   owner writes, inside the route's tenant transaction.
3. The decision hides the hint of its period while every uncovered class is
   one of the stored classes. A child of another class brings the hint back;
   deciding again stores the classes shown then. An empty list withdraws the
   period's decision. A decision never hides the hint of another period. The
   server computes the stored classes; the client sends only the offering,
   the period and yes or no.
4. An unknown or inactive planning period shows no hints and takes no
   decision (400), so no decision can stem from a period nobody plans in.
5. A new table was rejected. A tenant table needs a new school-erasure plan
   with copies of five erasure functions (ADR 0092), retention and seed
   registers, for a value that names periods and classes, no person, and
   belongs to one offering. The column goes with its offering and its school.

## Consequences

A copied offering of the next year starts without a decision. An entry for
a deleted planning period never matches again and stays until the offering
goes. The hint does not judge closing days or A/B weeks inside a running
Regeltermin; a booking on such a day counts as covered.
