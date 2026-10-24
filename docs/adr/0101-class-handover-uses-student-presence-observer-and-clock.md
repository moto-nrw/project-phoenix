---
status: accepted
---

# Class handover uses Student Presence's observer and clock

Decision of 2026-10-24 for
[#4794](https://github.com/moto-nrw/project-phoenix/issues/4794). It approves
three new command-effect diagnostics. It adds no owner, no policy rule and no
runtime exception.

## Context

The Student Presence application service runs every operation through its
operation wrapper, which reports duration, counts and error to the injected
observer `observe` (`compose.Dependencies.Observe`). A write that stamps a
moment reads the injected clock `now`. The binding-flow scan of
[ADR 0098](0098-command-effect-binding-flow-and-platform-protocols.md) cannot
follow either function field to a terminal, so every port that uses them
records the diagnostics
`unresolved:function:…/studentpresence/internal/application.Service.observe`
(binding digest `e227a8013824f329765590468b7e22d7`) and
`unresolved:function:…/studentpresence/internal/application.Service.now`
(binding digest `cd91850d3de0d051c94f2dc1d37acc63`). ADR 0098 approved the
existing ones and [ADR 0100](0100-morning-attendance-uses-student-presence-observer.md)
those of the morning attendance. An ADR already in the base cannot approve
new ones.

The class handover after lessons (#4794) adds two operations of the same
service:

| Port | Binding | Diagnostics |
|---|---|---|
| `studentpresence.Owner.ChangeClassHandover` | `application.Service.ChangeClassHandover` | `observe`, `now` |
| `studentpresence.Owner.ClassHandoversOn` | `application.Service.ClassHandoversOn` | `observe` |

## Decision

Both operations run through the service's operation wrapper like the
module's other operations, and `ChangeClassHandover` stamps the moment of the
handover with the service's clock. Their diagnostics cite this ADR as
`approval_adr`. The observer only reports measurements and the clock only
returns the current instant; neither reaches another owner's state, takes a
lock or widens a scope. The diagnostics carry the same binding digests as the
entries ADR 0098 approved, so neither assignment is changed.

Leaving the observer out would remove the handover from the module's runtime
metrics and break the service's single operation wrapper; reading the wall
clock directly would bypass the clock the tests fix. Resolving both function
fields for the whole module is a scanner change outside #4794.

After this ADR enters the base it cannot approve another diagnostic. A
changed observer or clock assignment changes the digest and needs fresh
approval.
