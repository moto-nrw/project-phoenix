---
status: accepted
---

# The operator status lifecycle raises the Identity & Access compose budget

Decision of 2026-10-16 under
[#4047](https://github.com/moto-nrw/project-phoenix/issues/4047). It raises two
registers that [ADR 0057](0057-only-the-target-is-a-rule.md) point 5 lets only
fall.

## Context

#4047 gives the accounts of moto team members who left a fixed deletion
window. Another operator deactivates or reactivates the account in the
operator dashboard, and the hourly cleanup deletes accounts deactivated before
1 January of last year. The HTTP adapter and the scheduler reach Identity &
Access only through its public module, so the engine in
`modules/identityaccess/compose` needs one method per operation. Each method
checks that the operator provisioning flow is wired, attaches the runtime,
converts the types and maps the errors, like the nine operator provisioning
methods next to it.

Before raising anything, the change was cut down:

- Deactivation and reactivation are one operation, `ChangeOperatorStatus`,
  with an `Active` field. This saves one engine method and 8 lines.
- None of the 323 engine methods in the package is unused in production code
  and could go to make room.
- Moving the forwarding into a helper or a local alias would lower the
  passthrough count without removing a delegation. The header of
  `test/module_passthrough_ratchet_test.go` names this as a gap, not a way out.

What remains are two engine methods, `ChangeOperatorStatus` and
`DeleteDeactivatedOperators`, and one sentinel mapping: 17 lines and 2
single-call delegations.

## Decision

1. `moduleComposeBudgets["modules/identityaccess/compose"]` rises from 7,339
   to 7,356 and `moduleComposeBudgetTotal` rises by the same 17 lines.
2. `modulePassthroughOwnerBudgets["modules/identityaccess"]` rises from 600
   to 602 and `modulePassthroughOwnerBudgetTotal` rises by the same two. The
   two delegations sit in `modules/identityaccess/compose`, which this entry
   counts together with the application layer since
   [ADR 0061](0061-pass-throughs-are-counted-per-owner.md).
3. The raise covers exactly the methods named above. Every other number keeps
   falling under ADR 0057 point 5.

## Consequences

- The operator status lifecycle has the same shape as the rest of the operator
  provisioning flow. It adds no new composition pattern.
- If the engine later drops its per-method forwarding, for example once the
  public module calls the application layer directly, these methods go with
  the rest and the budgets fall again.
