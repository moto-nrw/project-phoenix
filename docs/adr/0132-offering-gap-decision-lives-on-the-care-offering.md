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
   Care Plan's approved bookings (`OfferingWeekdayBookings`, from today on)
   with the period's Regeltermine per offering × child × school day
   (Monday to Friday). Only a Regeltermin the offering feeds covers a
   booking: its source rule with the Jahrgang or Klassen filter, or the
   offering's own link. Children on a hand-kept list or in another offering's
   Regeltermin stay uncovered; the roster review (ADR 0129) handles hand-kept
   lists.
2. The school's decision is stored on the offering row,
   `enrollment.care_offerings.template_gap_ack_classes` (TEXT[], NULL = no
   decision), as the classes the hint named when the school decided. Care
   Plan owns the row and writes the column through its catalog command
   `CareOfferingTemplateGapDecision.DecideOfferingTemplateGap`; Timetable
   reaches it through its consumer port `OfferingTemplateGapDecision`, bound
   to the catalog at the root. The command effect is an ADR 0067 leaf: one
   owner writes, inside the route's tenant transaction.
3. The decision hides the hint while every uncovered class is one of the
   stored classes. A child of another class brings the hint back; deciding
   again stores the classes shown then. An empty list withdraws the decision.
   The server computes the stored classes; the client sends only the offering,
   the period and yes or no.
4. A new table was rejected. A tenant table needs a new school-erasure plan
   with copies of five erasure functions (ADR 0092), retention and seed
   registers, for a value that names classes, no person, and belongs to one
   offering. The column goes with its offering and its school.

## Consequences

An offering phase usually spans one school year, so the decision holds for
the planning periods that phase fits. A copied offering of the next year
starts without a decision. A decision on an offering without bookings in a
period has no effect and stays until the school withdraws it from a period
where the hint shows.
