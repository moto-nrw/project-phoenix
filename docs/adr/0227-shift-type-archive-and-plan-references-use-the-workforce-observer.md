---
status: accepted
---

# Shift-type archive and plan references use the Workforce observer

Decision of 2026-11-13 for
[#6845](https://github.com/moto-ogs/project-phoenix/issues/6845). It approves
seven new command-effect diagnostics. It adds no owner, no policy rule and no
runtime exception.

## Context

Deleting a Dienstart set `shift_type_id` to NULL on every planned shift and
series, so past shifts lost their classification. Deleting a calendar period
that Dienstplan series still used failed on the series FK, while the deletion
preview showed the period as unused. Workforce now serves both through the
new public interfaces `workforce.ShiftTypeRetirement` (`RemoveShiftType`) and
`workforce.ShiftPlanReferenceQuery` (`ShiftPlanReferences`), bound to the same
application service as `workforce.Capability`.

The service runs every operation through its operation wrapper, which reports
duration, counts and error to the injected observer `observe`. The
binding-flow scan of
[ADR 0098](0098-command-effect-binding-flow-and-platform-protocols.md) cannot
follow that function value, so every port that uses it records
`unresolved:callable:…/workforce/internal/application.observe`. ADR 0098,
[ADR 0117](0117-own-absence-allowances-use-workforce-observer.md),
[ADR 0186](0186-shift-writes-read-sick-reports-through-the-workforce-observer.md)
and [ADR 0221](0221-work-time-template-catalog-uses-the-workforce-observer.md)
approved the existing ones. An ADR already in the base cannot approve new
diagnostics.

| Port | Binding |
|---|---|
| `workforce.ShiftPlanReferenceQuery.ShiftPlanReferences` | `compose.WorkTime.ShiftPlanReferences` |
| `compose.WorkTime.RemoveShiftType` | `application.Service.RemoveShiftType` |
| `compose.WorkTime.ShiftPlanReferences` | `application.Service.ShiftPlanReferences` |
| `planning.Records.RemoveShiftType` | `compose.WorkTime.RemoveShiftType` |
| `planning.Records.ShiftPlanReferences` | `compose.WorkTime.ShiftPlanReferences` |
| `planning.shiftTypeOperations.RemoveShiftType` | `planning.Records.RemoveShiftType` |
| `planning.shiftTypeOperations.ShiftPlanReferences` | `planning.Records.ShiftPlanReferences` |

## Decision

Both operations run through the service's operation wrapper like the other
shift-type operations (`ListShiftTypes`, `DeleteShiftType`). Their diagnostics
cite this ADR as `approval_adr`. The observer only reports measurements; it
reaches no other owner's state, takes no lock and widens no scope.

`RemoveShiftType` locks the type row, counts the shifts and series carrying
it and then archives or deletes it, all inside the unit of work the wrapper
opens or joins. `ShiftPlanReferences` is one tenant-scoped SELECT over
`schedule.staff_shifts` and `schedule.staff_shift_series`. The timetable
calendar period usage reads it through a root adapter; that binding is
classified `query`.

The interfaces are separate from `workforce.Capability` because the facade
method-set ratchet freezes that interface's width.

Leaving the observer out would remove the operations from the module's
runtime metrics and break the service's single operation wrapper. Resolving
the function value for the whole module is a scanner change outside #6845.

After this ADR enters the base it cannot approve another diagnostic. A
changed observer assignment changes the digest and needs fresh approval.
