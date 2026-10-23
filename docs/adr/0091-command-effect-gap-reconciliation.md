---
status: accepted
---

# Reconcile callable evidence and workflow debt without a reusable gate exemption

Decision of 2026-10-23 for [#4836](https://github.com/moto-nrw/project-phoenix/issues/4836).
The [ADR 0083 gap-closure addendum](0083-ratchets-protect-invariants-and-expose-command-effects.md#gate-gap-closure-2026-10-23-4836)
records the scanner changes, exact register delta, evidence and limitations.

Existing callable diagnostics gain assignment/closure evidence. Existing
`workflow` labels outside the seven Care Exit Roster/Bookings E2 command/lock
identities become `debt`, citing #4825. Neither change introduces a runtime
command, a new exception or a permission to call workflows from domains.
The retained seven labels record the bounded E2 protocol described by ADR 0067
and the proposed ADR 0071 addendum in unmerged PR #4753. This decision does not
claim that proposed addendum is merged or amend ADR 0071.

The changed entries cite this newly numbered ADR as `approval_adr`. This uses
the existing immutable-base approval contract rather than teaching the checker
another one-time census heading or a fingerprint-migration bypass. After this
ADR enters the base it cannot approve another exception, changed callback or
new workflow label. The counterprobe removes this decision and must fail.

A semantic edit to a still-unresolved closure requires fresh ADR evidence even
if its author believes no foreign effect changed. We retain that review cost:
without resolving runtime dispatch the checker cannot prove that claim.
Comments, formatting and unrelated closures do not change its fingerprint.
