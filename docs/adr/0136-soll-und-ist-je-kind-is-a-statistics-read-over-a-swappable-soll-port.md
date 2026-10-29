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
   (#2606) in `modules/studentpresence/internal/application/statistics`.
   The Ist is its own data (`active.attendance`: first check-in to last
   check-out of the day). The care-day calendar (weekdays minus public
   holidays, closing days and holiday periods) and the status days are the
   inputs the Statistik report already reads. No new owner, table or write
   path is added.
2. The Soll is a consumer-owned port, `ports.CareTimeTargets`, bound at the
   root to Care Plan's new read capability `careplan.CareTimeQuery`
   (`ResolveCareTimes`). Care Plan answers from the derivation the care-day
   verdict already uses (`CareDayQueries`), so the verdict and the booked
   arrival and pickup times can never disagree. A care contract in the
   Träger module (#4683) can replace the binding without touching the
   comparison.
3. To keep the composition packages at their LOC budgets, the Soll port
   travels with the status days as `ports.CarePlanFacts` and the attendance
   span read with the attendance days on `ports.AttendanceDays`. Care Plan's
   composed care-day derivation is exposed as `careplan.CareDays`, the union
   of `CareDayQuery` and `CareTimeQuery`; every existing consumer keeps its
   narrower interface.
4. The overrun list and its export are management reports and share the
   Statistik report's permissions (`config:read` and `users:read`) and its
   data-access log (scope `care_times`). One child's comparison needs the
   children read right (`users:read`), like the child's care-time tab.
5. Command effects: the new bindings are reads. The four unresolved
   boundaries are the clock (`statistics.Config.Now`) of
   `StudentCareTimes`, `CareTimeOverruns` and `CareTimeOverrunsForExport`
   and the observation callback (`application.Service.observe`) of
   `ListAttendanceSpans`. They are the same closures the reviewed
   `Report`, `ReportForExport` and `ListAttendanceDays` bindings already
   reach (ADR 0098). This ADR is their approval; a changed closure needs a
   new one.

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
