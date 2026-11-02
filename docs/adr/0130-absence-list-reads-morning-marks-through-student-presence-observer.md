---
status: accepted
---

# Absence list reads the morning marks through Student Presence's observer

Decision of 2026-10-27 for
[#4793](https://github.com/moto-nrw/project-phoenix/issues/4793). It approves
one new command-effect diagnostic. It adds no owner, no policy rule and no
runtime exception.

## Context

The absence list of moto schule counts the absence days of a class over a
school half-year. It reads the morning marks of every child of the class for
the whole period at once. Student Presence had only the read of one day
(`MorningAttendanceOn`, [ADR 0100](0100-morning-attendance-uses-student-presence-observer.md)).
One read per day would grow with the period.

The Student Presence application service runs every operation through
`Service.run`, which reports to the injected observer `observe`. The
binding-flow scan of
[ADR 0098](0098-command-effect-binding-flow-and-platform-protocols.md) cannot
follow that function field, so every port of the service records the
diagnostic
`unresolved:function:…/studentpresence/internal/application.Service.observe`
(binding digest `e227a8013824f329765590468b7e22d7`). An ADR already in the
base cannot approve a new one.

The absence list adds one read of the same service:

| Port | Binding |
|---|---|
| `studentpresence.Owner.MorningAttendanceBetween` | `application.Service.MorningAttendanceBetween` |

## Decision

The read runs through `Service.run` like the module's other operations, and
its diagnostic cites this ADR as `approval_adr`. The observer only reports
measurements; it reaches no other owner's state, takes no lock and widens no
scope. The diagnostic carries the same binding digest as the entries ADR 0098
and ADR 0100 approved, so the observer's assignment is unchanged.
`MorningAttendanceOn` keeps its binding.

After this ADR enters the base it cannot approve another diagnostic. A
changed observer assignment changes the digest and needs fresh approval.
