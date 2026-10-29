---
status: accepted
---

# Soll und Ist je Kind is a statistics read over a swappable Soll port

Issue [#4774](https://github.com/moto-nrw/project-phoenix/issues/4774): the
school sees per child and day whether the actual attendance (Ist) stayed
within the booked care time (Soll), and the school management sees every
child with overruns in a window, as a list and as PDF and Excel export.

## Decision

1. Student Presence owns the comparison, next to the Statistik report
   (#2606) and the occupancy by time of day (#4764) in
   `modules/studentpresence/internal/application/statistics`. No new owner,
   table, write path or port is added.
2. The Soll is the port the occupancy already reads,
   `ports.PlannedCareDays`, bound at the root to Care Plan's
   `CareDayQuery.ResolvePlannedForRange`. Its value `PlannedCareDay` gains
   `NoPlan` and `Cancelled`, so the comparison can tell a day without any
   care plan, a cancelled day and a day that was never booked apart; the
   occupancy keeps reading `Expected`. A care contract in the Träger module
   (#4683) can replace the binding without touching the comparison.
3. The Ist is Student Presence's own attendance through the occupancy's
   `ports.AttendanceIntervals`: first check-in to last check-out per child
   and day. Like the occupancy, the comparison reads no further back than
   `gdpr.attendance_retention_days` and `gdpr.status_day_retention_days`;
   older days could have lost their check-ins or sick notes and would read
   as false shortfalls. The first compared day travels as `data_from`.
4. The overrun list and its export are management reports and share the
   Statistik report's permissions (`config:read` and `users:read`) and its
   data-access log (scope `care_times`). One child's comparison needs the
   children read right (`users:read`), like the child's care-time tab.
5. Command effects: the three new bindings reach the same reviewed targets
   as `Report` and `ReportForExport`. Their unresolved boundary is the same
   clock (`statistics.Config.Now`, ADR 0098). This ADR is its approval for
   `StudentCareTimes`, `CareTimeOverruns` and `CareTimeOverrunsForExport`;
   a changed closure needs a new one.

## Consequences

- The definitions are binding and named on the screen, in the export and in
  the help guide: an overrun is an arrival earlier or a pickup later than
  booked by more than 15 minutes (`CareTimeToleranceMinutes`), or attendance
  on a day without a booking. A booked day without attendance counts as
  shorter than booked unless a sick note, an absence, a cancellation, a
  closing day or a public holiday explains it.
- A school without any care plan on file sees its attendance with
  "Keine Zeiten hinterlegt" and no overruns, so missing data never produces
  a false overrun.
- The window follows the Statistik report: at most a school year, never in
  the future.
