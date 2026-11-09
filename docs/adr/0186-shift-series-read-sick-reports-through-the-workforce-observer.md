---
status: proposed
---

# Shift series read sick reports through the Workforce observer

Decision of 2026-11-09 for
[#6302](https://github.com/moto-nrw/project-phoenix/issues/6302). It approves
two new command-effect diagnostics. It adds no owner, no policy rule, no lock
and no runtime exception.

## Context

A shift series materializes its concrete shifts upfront. Before #6302 it did
not look at sick reports, so a shift created after a report stayed planned
while the sick cascade had cancelled the shifts that existed when the report
came in. The materializer now reads the staff member's open sick reports and
creates the occurrences on full sick days in the cascade's state.

The read goes through the Workforce application service, which runs every
operation through its operation wrapper and reports to the injected observer
`observe`. The binding-flow scan of
[ADR 0098](0098-command-effect-binding-flow-and-platform-protocols.md) cannot
follow that function value, so every port that uses it records the diagnostic
`unresolved:callable:…/workforce/internal/application.observe` (binding
digest `4a6c63bb7365af6cf9d81caa8ffa9d21`). ADR 0098 approved the existing
ones, [ADR 0117](0117-own-absence-allowances-use-workforce-observer.md) those
of the allowance read. An ADR already in the base cannot approve new ones.

| Port | Binding |
|---|---|
| `planning.SeriesSickReportReader.ListStaffAbsences` | `planning.Records.ListStaffAbsences` |
| `planning.Records.ListStaffAbsences` | `compose.WorkTime.ListStaffAbsences` |

## Decision

The series materializer reads sick reports through `planning.Records`, the
Workforce capability its composition already binds, like the shift reads
beside it. Its two diagnostics cite this ADR as `approval_adr`. The observer
only reports measurements; it reaches no other owner's state, takes no lock
and widens no scope. The diagnostics carry the same binding digest as the
entries ADR 0098 approved, so the observer assignment is unchanged.

A separate read around the operation wrapper would remove it from the
module's runtime metrics. Resolving the function value for the whole module
is a scanner change outside #6302.

After this ADR enters the base it cannot approve another diagnostic. A
changed observer assignment changes the digest and needs fresh approval.
