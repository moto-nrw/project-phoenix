---
status: accepted
---

# Reviewed staff person repair adds narrow owner compositions

Decision for [#3397](https://github.com/moto-nrw/project-phoenix/issues/3397),
following ADR 0021 and the workflow boundary of ADR 0067. A reviewed repair
retains the account-bearing person and both employments. It must change People
Directory and School Membership together and append its operator audit in the
same transaction.

The two existing owners expose restricted administrative commands through new
constructors. Each constructor builds only its own store. The new data-less
`staff-person-merge` workflow composes those public commands, the audit port and
the standing runtime. It owns no table and executes no SQL. Reusing the full
owner compositions would pull unrelated student, employment and delivery
capabilities into this repair. Exposing their private stores at the root would
break the owner boundary.

HTTP uses the existing `TenantTxMiddleware` after the operator session gates.
The workflow requires its administrative transaction rather than opening
another one. All selected fingerprints are checked before writes, and a failed
write or audit rolls the entire batch back. No administrative transaction
register entry is added, and no scanner rule is changed.

Under ADR 0057 point 5 and ADR 0070 point 4, this decision permits exactly these
composition-budget additions:

- `modules/peopledirectory/compose`: 963 to 974 lines, for
  `NewStaffPersonMerge` (11 lines).
- `modules/schoolmembership/compose`: 185 to 196 lines, for
  `NewStaffPersonMerge` (11 lines).
- `workflows/staffpersonmerge/compose`: a new 31-line entry for construction,
  the ambient administrative-transaction guard and the school repair lock.
- `moduleComposeBudgetTotal`: 12,355 to 12,408, the same 53 lines in total.

This permits the named constructors and runtime binding only. It does not
raise a forwarding budget, authorize foreign implementation imports, move
application logic into composition, or loosen the transaction register. These
entries remain shrink-only after this addition. The ordinary architecture
check and the composition and transaction tests continue to enforce them.
