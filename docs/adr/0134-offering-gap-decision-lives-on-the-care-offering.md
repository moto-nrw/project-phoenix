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
   Care Plan's approved bookings (`OfferingWeekdayBookings`, each link with
   its validity, bounded by the phase's service days) with the period's
   Regeltermine per offering × child × school day (Monday to Friday), date
   by date, in an active planning period from today (or the period start)
   to the period end. **A day is judged exactly as materialization creates
   it:** the dates of a Regeltermin come from
   `MaterializationCapability.ProjectOccurrences`, which runs the
   materializer's own per-date rule (`candidateSlot`: schedule bounds,
   series end, period selection including the lowest id among overlapping
   unpinned periods, the A/B anchor of the selected period, holidays,
   closing days and exceptions) over the inputs the materializer loads
   (`loadOccurrenceRules`). The check builds no recurrence of its own. A
   booked date is covered when a Regeltermin the offering feeds has an
   occurrence on it: its source rule with the Jahrgang or Klassen filter,
   or the offering's own link. Holidays and closing days carry no care and
   are not checked. Children on a hand-kept list or in another offering's
   Regeltermin stay uncovered; the roster review (ADR 0129) handles
   hand-kept lists.
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
goes. A weekly booking against a Regeltermin in week A only is open in the
B weeks: that is what materialization creates. Closing days are not
checked, also for series that include them. The projection reads one
window of up to `MaxOccurrenceProjectionDays` (400); materialization itself
keeps its 56-day write window.
