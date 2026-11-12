---
status: proposed
---

# Shift writes read sick reports through the Workforce observer

Decision of 2026-11-09 for
[#6302](https://github.com/moto-ogs/project-phoenix/issues/6302). It approves
eleven new command-effect diagnostics. It adds no owner, no policy rule, no
lock and no runtime exception.

## Context

The sick cascade cancels the shifts that exist when a sick report comes in.
Before #6302 every shift planned later stayed planned: occurrences a shift
series materialized, a single shift created for the sick day, and a shift moved
or edited onto it. An edit that took a sick-cancelled shift to a healthy day
kept it cancelled. A sick person could also cover a colleague's cancelled shift. These
writes now read the staff member's open sick reports under the shift lock they
already hold. A new shift on a full sick day takes the cascade's state, a shift edited away
from one leaves it, and a sick cover is refused. Past days stay history, as the cascade leaves them, so
the single shift writes compare against the calendar-day clock.

Two function values in that flow are opaque to the binding-flow scan of
[ADR 0098](0098-command-effect-binding-flow-and-platform-protocols.md):

- `observe`: the Workforce application service runs every operation through
  its operation wrapper and reports to this injected observer. Every port that
  uses it records `unresolved:callable:…/workforce/internal/application.observe`
  (binding digest `4a6c63bb7365af6cf9d81caa8ffa9d21`). ADR 0098 approved the
  existing ones, [ADR 0117](0117-own-absence-allowances-use-workforce-observer.md)
  those of the allowance read.
- `api.now`: the calendar-day clock the series service already reads,
  approved there by ADR 0176. The shift service now reads it too.

An ADR already in the base cannot approve new diagnostics.

| Port | Binding | Boundary |
|---|---|---|
| `planning.SickReportReader.ListStaffAbsences` | `planning.Records.ListStaffAbsences` | `observe` |
| `planning.Records.ListStaffAbsences` | `compose.WorkTime.ListStaffAbsences` | `observe` |
| `workforce.StaffShiftPlanning.CreateShift`, `.MoveShift`, `.UpdateShift`, `.ApplyCancellation` | `planning.staffShiftPlanning` | `api.now` |
| `planning.StaffShiftService.CreateShift`, `.MoveShift`, `.UpdateShift`, `.UpdateShiftWithOptions`, `.ApplyCancellation` | `planning.staffShiftService` | `api.now` |

## Decision

The shift writes read sick reports through `planning.Records`, the Workforce
capability their composition already binds, like the shift reads beside them.
The diagnostics cite this ADR as `approval_adr`. The observer only reports
measurements, and the clock only returns the calendar day. Neither reaches
another owner's state, takes a lock or widens a scope. The observer
diagnostics carry the same binding digest as the entries ADR 0098 approved, so
the observer assignment is unchanged.

A separate read around the operation wrapper would remove it from the
module's runtime metrics. Resolving the function values for the whole module
is a scanner change outside #6302.

After this ADR enters the base it cannot approve another diagnostic. A
changed observer or clock assignment changes the digest and needs fresh
approval.
