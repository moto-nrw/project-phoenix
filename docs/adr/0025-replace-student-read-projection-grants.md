---
status: accepted
---

# Replace the three student read grants through an exact checker exception

For [#3432](https://github.com/moto-nrw/project-phoenix/issues/3432), permit
one bounded replacement of the three existing `users.students` read-projection
grants. Encode the exact replacements in the checker, require a reviewed epoch
increase, and preserve the compatibility view's row-selection semantics by
joining all three owner tables. The user confirmed both decisions on
2026-09-19. This records the design, not an implemented or verified cutover.

## Why an epoch is not enough

At `development` commit `eaabd67ca68e4f8c31710bcfa37be87325721f4a`, policy
epoch 15, `readProjectionLoosenings` in
`backend/internal/architecture/policy_compare.go` rejects new package/table
grants for existing projection owners, independently of the epoch. A review
note and an epoch bump cannot make the proposed table changes pass PR mode.
The grant key contains only package and table, so the exception must also
check projection identity and package classification.

This narrowly extends [ADR 0023](0023-student-enrollment-owner-and-directory-projection.md)'s
projection-grant rule. It does not introduce an epoch-wide bypass, a generic
policy migration schema, new projection owners, or package moves to evade the
existing guard. Those alternatives would permit more change than this ticket
needs.

## Exact replacement contract

Only these existing tuples qualify in the canonical moto module:

| Projection ID and owner | Package | Production role |
|---|---|---|
| `parent-message-inbox` | `modules/communication/internal/adapters/parentinbox` | `postgres` |
| `parent-announcement-audience` | `modules/communication/internal/adapters/parentaudience` | `postgres` |
| `care-exit-view` | `modules/careplan/legacy/careexitview` | `postgres` |

For each tuple, replace `users.students` with exactly:

| Data object | Unchanged write owner | Read purpose |
|---|---|---|
| `users.student_profiles` | `people-directory` | Public student ID and person reference |
| `users.student_school_memberships` | `school-membership` | Class, lifecycle and live-membership filter |
| `users.student_care_profiles` | `care-plan` | Existing care-row existence filter only |

The checker may exempt those new package/table grants only when all of these
conditions hold against the immutable PR base policy:

1. The named projection and its `users.students` grant exist in the base.
   Package, projection ID, owner, owner kind `projection`, production role and
   both test roles stay unchanged; `tenant_safe` is true on both sides.
2. The candidate epoch is greater than the base epoch. Do not hard-code epoch
   16: other reviewed changes may land first.
3. The candidate projection's data-object set equals the base set minus
   `users.students` plus the three targets above. All other grants in that
   projection remain unchanged. Target tables exist in both policies and keep
   the write owners above.
4. The ordinary comparison still checks every other new grant, and all policy
   validation, write-ownership, import, baseline and composition guards still
   run. A matching replacement must not skip checks for the rest of a
   projection or candidate.

Each tuple can qualify independently. Once its old grant is absent from the
base, that tuple cannot activate the exception again. An unchanged migrated
policy remains valid under ordinary comparison. A removed care-exit projection
does not activate an exception or authorize a replacement under another name.
The fixed exception can be removed after all three old grants have left
`development`; keeping it until then does not grant a replay path.

## Preserve the joined read

The compatibility view in
`backend/database/migrations/student_owner_compatibility.go` uses this shape:

```sql
FROM users.student_profiles AS p
JOIN users.student_school_memberships AS m
  ON m.tenant_id = p.tenant_id
 AND m.student_profile_id = p.id
 AND m.deleted_at IS NULL
JOIN users.student_care_profiles AS c
  ON c.tenant_id = m.tenant_id
 AND c.membership_id = m.id
```

Preserve these joins inside each existing statement and retain its tenant,
authorization and business predicates. Public `student_id` remains `p.id`,
not `m.id`; the initial backfill's equal IDs do not establish that identity.
`person_id` comes from `p`, and class, status and enrollment bounds from `m`.
`student_profiles` has no `deleted_at` filter. Existing `users.persons` grants
stay in place.

The care-to-membership foreign key does not require every membership to have
a care row. Omitting the third join would expose rows the view excludes.
Keep that existence filter without selecting additional care fields or adding
a new database invariant. Preserve `FOR UPDATE OF person` in the care-exit
read and `FOR SHARE OF sg, gp, act` in announcement response authorization.
Relationship-level `parent_portal.*` permissions and tenant-scoped RLS remain
mandatory; membership alone does not authorize a parent to read a child.

## Delivery and evidence

Ship the checker exception, policy replacements and adapter queries in one
implementation PR. #3432 carries the executable test and verification scope.
Positive comparator tests must accept each exact replacement, all three
together, and an unchanged post-cutover policy. A PR-mode fixture with an
immutable Git base must accept the intended migration through the real checker.

Negative tests must still reject extra grants on the migrated and other
existing packages, changed identities or roles, missing or retained old grants,
wrong or incomplete target sets, a missing epoch increase and replay. An epoch
increase alone must not authorize an unrelated addition. Keep unsafe-projection
and forbidden-write coverage. Query tests must prove parity with distinct
profile/membership IDs, missing care rows, missing or deleted memberships,
parent permissions, cross-tenant access, lock scope and unchanged query budgets.

Coordinate with [#3427](https://github.com/moto-nrw/project-phoenix/issues/3427):
if it actually removes `care-exit-view` first, migrate only the two remaining
projections and prove the old care-exit grant is gone. Otherwise migrate the
existing care-exit package without dissolving it. Do not assume that closing
#3427 alone proves removal. This work blocks
[#2760](https://github.com/moto-nrw/project-phoenix/issues/2760), but does not
perform its DDL or #2759's caller migration.

No `legacy.jsonl` changes, composition growth, write-owner changes, persistent
read model, query-budget increases or general legacy cleanup belong here.
[ADR 0024](0024-retire-legacy-student-contact-fields.md) stays separate: these
reads do not need the retired guardian copies, archive fallbacks or contact
reconciliation. No glossary change is needed for this policy mechanism.
