---
status: accepted
---

# Morning attendance uses Student Presence's observer

Decision of 2026-10-24 for
[#4790](https://github.com/moto-nrw/project-phoenix/issues/4790). It approves
three new command-effect diagnostics. It adds no owner, no policy rule and no
runtime exception.

## Context

The Student Presence application service runs every operation through
`Service.run`, which reports duration, counts and error to the injected
observer `observe` (`compose.Dependencies.Observe`). The binding-flow scan of
[ADR 0098](0098-command-effect-binding-flow-and-platform-protocols.md) cannot
follow that function field to a terminal. Every port of the service therefore
records the diagnostic
`unresolved:function:…/studentpresence/internal/application.Service.observe`
(binding digest `e227a8013824f329765590468b7e22d7`). ADR 0098 approved the
existing ones. An ADR already in the base cannot approve new ones.

Morning attendance (#4790) adds three operations of the same service:

| Port | Binding |
|---|---|
| `studentpresence.Owner.RecordMorningAttendance` | `application.Service.RecordMorningAttendance` |
| `studentpresence.Owner.MorningAttendanceOn` | `application.Service.MorningAttendanceOn` |
| `studentpresence.Owner.DeleteExpiredMorningAttendance` | `application.Service.DeleteExpiredMorningAttendance` |

## Decision

The three operations run through `Service.run` like the module's other
operations, and their diagnostics cite this ADR as `approval_adr`. The
observer only reports measurements; it reaches no other owner's state, takes
no lock and widens no scope. The diagnostics carry the same binding digest as
the entries ADR 0098 approved, so the observer's assignment is unchanged.

Leaving the observer out would remove the morning attendance from the
module's runtime metrics and break the service's single operation wrapper.
Resolving the observer for the whole module is a scanner change outside
#4790.

After this ADR enters the base it cannot approve another diagnostic. A
changed observer assignment changes the digest and needs fresh approval.
