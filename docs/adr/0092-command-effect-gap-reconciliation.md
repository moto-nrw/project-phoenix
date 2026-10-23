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

## Development merge reconciliation

Development commit `3e625a1f5d` already uses ADR 0091 for external caregiver
records. This decision therefore uses the fresh number 0092; unchanged base
approvals continue to cite 0091. Regeneration removes all 15 obsolete branch
identities, including the old GuestRepository callbacks, rather than preserving
them as debt. Replacement identities retain the base's reviewed classifications.

Against that development commit the merged register contains 5,350 entries:
3 added, 0 removed and 48 changed fingerprints. There are 7 workflow, 92 debt
and 642 unresolved entries. The 70 gap-repair approvals cite 0092, including
the renamed callable whose assignment fingerprint changes. The original
pre-merge measurements in ADR 0083 remain historical evidence.

The subsequent integrations of `c69f73f218` (#4755) and `b8c42a3cda` (#4832) leave these register counts
unchanged. The tuple-argument evidence correction adds no production identity
or digest change; its normal/variadic fixtures are red before and green after.

Final tuple-corrected warm `check` against full merge-base
`b8c42a3cdac13d1270017eee39fcd79a207b8fba`: 33.80 s wall, 93.69 s user,
20.87 s system, after an untimed warmup and without concurrent own tests/lint.
The fresh-base no-new-ADR counterprobe remains red (67 existing changes).
