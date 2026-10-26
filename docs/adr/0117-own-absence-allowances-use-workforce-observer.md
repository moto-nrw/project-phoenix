---
status: accepted
---

# Own absence allowances use the Workforce observer

Decision of 2026-10-26 for
[#4971](https://github.com/moto-nrw/project-phoenix/issues/4971). It approves
four new command-effect diagnostics. It adds no owner, no policy rule and no
runtime exception.

## Context

The Workforce application service runs every operation through its operation
wrapper, which reports duration, counts and error to the injected observer
`observe`. The binding-flow scan of
[ADR 0098](0098-command-effect-binding-flow-and-platform-protocols.md) cannot
follow that function value to a terminal, so every port that uses it records
the diagnostic
`unresolved:callable:…/workforce/internal/application.observe` (binding
digest `4a6c63bb7365af6cf9d81caa8ffa9d21`). ADR 0098 approved the existing
ones. An ADR already in the base cannot approve new ones.

Staff members read their own Kontingente of the school's absence types
(#4971) through one new read of the same service:

| Port | Binding |
|---|---|
| `workforce.OwnAbsenceAllowances.AllowanceAccounts` | `workforce.Capability.AllowanceAccounts` |
| `workforce.Query.AllowanceAccounts` | `workforce.Capability.AllowanceAccounts` |
| `workforce.Capability.AllowanceAccounts` | `compose.WorkTime.AllowanceAccounts` |
| `compose.WorkTime.AllowanceAccounts` | `application.Service.AllowanceAccounts` |

## Decision

`AllowanceAccounts` runs its reads through the service's operation wrapper
like `AllowanceSummary`, whose numbers it repeats for every active type with
an allowance. Its diagnostics cite this ADR as `approval_adr`. The observer
only reports measurements; it reaches no other owner's state, takes no lock
and widens no scope. The diagnostics carry the same binding digest as the
entries ADR 0098 approved, so the observer assignment is unchanged.

Leaving the observer out would remove the read from the module's runtime
metrics and break the service's single operation wrapper. Resolving the
function value for the whole module is a scanner change outside #4971.

After this ADR enters the base it cannot approve another diagnostic. A
changed observer assignment changes the digest and needs fresh approval.
