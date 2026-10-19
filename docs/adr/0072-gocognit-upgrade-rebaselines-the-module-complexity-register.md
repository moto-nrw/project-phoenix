---
status: accepted
---

# The gocognit upgrade rebaselines the module complexity register

Decision of 2026-10-19 under
[#4386](https://github.com/moto-nrw/project-phoenix/issues/4386). It raises a
register that [ADR 0057](0057-only-the-target-is-a-rule.md) point 5 lets only
fall.

## Context

`TestModuleComplexityRatchet` scores every function under `modules/` and
`workflows/` with `github.com/uudashr/gocognit` and compares the scores over
15 with `moduleComplexityAllowlist`. Dependabot proposed gocognit 1.3.0 in
#4374; it was held at 1.2.1 there because the new version fails the ratchet
without a single changed line. The scoring changed:

- 1.2.2 corrects the nesting increment of `else`
  ([uudashr/gocognit#44](https://github.com/uudashr/gocognit/pull/44)) and
  logical operators inside parentheses
  ([uudashr/gocognit#45](https://github.com/uudashr/gocognit/pull/45)).
- 1.3.0 counts recursion once per method in a cycle and includes indirect
  recursion ([uudashr/gocognit#46](https://github.com/uudashr/gocognit/pull/46)).
  Its changelog asks users to review their thresholds.

Measured at `148e5aeb86`, 1.3.0 raises thirteen entries by one to five
points, scores three functions outside the register at 16 and lowers two
entries by one. `TestHandlerComplexityRatchet` stays at zero.

The rule "never raise a number, never add an entry" guards against code
that grows more complex. Here the code is unchanged and only the measuring
instrument moved. Refactoring the affected functions, most of them login,
MFA, session and invitation flows in Identity & Access, would put security
paths at risk for a dependency bump; that work belongs to
[#3423](https://github.com/moto-nrw/project-phoenix/issues/3423).

## Decision

1. gocognit moves to 1.3.0 in `backend/go.mod`.
2. `moduleComplexityAllowlist` is re-measured with 1.3.0: thirteen entries
   rise, three entries are added at 16
   (`(*service).deliverReminder`, `(*Service).ensurePickupWeekdayEnrollment`,
   `validStudentEnrollment`), two fall. The register's sum rises from 5,460
   to 5,527 over 228 entries.
3. The raise covers exactly this re-measurement. Every entry keeps falling
   under ADR 0057 point 5, and a later gocognit upgrade that moves scores
   again needs an ADR of its own.

## Consequences

- The ratchet measures with the current gocognit, and Dependabot can keep it
  current.
- The register reads 67 points higher without any added debt. Comparing
  numbers across the upgrade compares two instruments, not two codebases.
