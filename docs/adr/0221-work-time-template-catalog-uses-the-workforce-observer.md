---
status: proposed
---

# Work-time template catalog uses the Workforce observer

Decision of 2026-11-13 for
[#6891](https://github.com/moto-ogs/project-phoenix/issues/6891). It approves
six new command-effect diagnostics. It adds no owner, no policy rule, no lock
and no runtime exception.

## Context

A school manages its Arbeitszeitmodell-Vorlagen on a catalog page (#6891). The
page shows how many people each template reaches and lets the school stop
offering a template for new assignments and offer it again. Workforce serves
this through the new public interface `workforce.WorkTimeModelCatalog`, bound
to the same application service as `workforce.Capability`.

The service runs every operation through its operation wrapper, which reports
duration, counts and error to the injected observer `observe`. The
binding-flow scan of
[ADR 0098](0098-command-effect-binding-flow-and-platform-protocols.md) cannot
follow that function value, so every port that uses it records
`unresolved:callable:…/workforce/internal/application.observe` (binding digest
`a4203d183cd9c0f618bb13f73d8c59af`, the digest of every existing observer
entry). ADR 0098, [ADR 0117](0117-own-absence-allowances-use-workforce-observer.md)
and [ADR 0186](0186-shift-writes-read-sick-reports-through-the-workforce-observer.md)
approved the existing ones. An ADR already in the base cannot approve new
diagnostics.

| Port | Binding |
|---|---|
| `workforce.WorkTimeModelCatalog.WorkTimeModelUsage` | `compose.WorkTime.WorkTimeModelUsage` |
| `workforce.WorkTimeModelCatalog.ArchiveWorkTimeModel` | `compose.WorkTime.ArchiveWorkTimeModel` |
| `workforce.WorkTimeModelCatalog.RestoreWorkTimeModel` | `compose.WorkTime.RestoreWorkTimeModel` |
| `compose.WorkTime.WorkTimeModelUsage` | `application.Service.WorkTimeModelUsage` |
| `compose.WorkTime.ArchiveWorkTimeModel` | `application.Service.ArchiveWorkTimeModel` |
| `compose.WorkTime.RestoreWorkTimeModel` | `application.Service.RestoreWorkTimeModel` |

## Decision

The three operations run through the service's operation wrapper like the
other template operations (`ListWorkTimeModels`, `DeleteWorkTimeModel`). Their
diagnostics cite this ADR as `approval_adr`. The observer only reports
measurements; it reaches no other owner's state, takes no lock and widens no
scope. The diagnostics carry the same binding digest as the approved entries,
so the observer assignment is unchanged.

The usage count reads School Membership's liveness through the existing
`StaffAssignments` port, as the template edit and delete already do; those
reads are classified `query`. Archiving changes one row of
`config.work_time_models` and no Soll, inside the unit of work the wrapper
opens or joins.

The interface is separate from `workforce.Capability` because the facade
method-set ratchet freezes that interface's width.

Leaving the observer out would remove the operations from the module's
runtime metrics and break the service's single operation wrapper. Resolving
the function value for the whole module is a scanner change outside #6891.

After this ADR enters the base it cannot approve another diagnostic. A
changed observer assignment changes the digest and needs fresh approval.
