---
status: accepted
---

# A relocated package keeps its debt

The backend migration of
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580) moves packages
for a living: a legacy package leaves its old path and lands inside the module
that owns it, with the same owner, the same role and the same imports. The
exact ratchet identifies a violation by `scope|rule|source|target`, so a move
renames every key the package appears in — as source and as target. The base
comparison then reports the package's own debt as resolved and every consumer's
debt as new, although nothing about the dependency changed.

We decide: a policy epoch may declare a package relocation, and the base
baseline is then read at the candidate's package paths. The declaration renames
keys. It grants nothing.

## Why the existing mechanisms do not cover it

Neither route worked, and both were measured against
[#3226](https://github.com/moto-nrw/project-phoenix/issues/3226), which moves
`auth/jwt`, `models/auth` and `database/repositories/auth` into
`modules/identityaccess/legacy`:

1. **Recording the renamed keys as debt.** The import-debt conversion admits a
   tuple only when the base policy allowed that exact import. A moved target
   was forbidden at the base, so all 65 renamed entries are rejected as new
   violations.
2. **Permitting the renamed edges with rules.** A rule names an owner and a
   role, never a package. `identity-access/domain` and `identity-access/postgres`
   are also held by `modules/identityaccess/internal/domain` and
   `modules/identityaccess/internal/adapters/postgres`, and every
   identity-access package shares `module-internal-test` and
   `module-behavior-test`. The 61 rules the move would need widened 179 edges
   that no package has, and the loosening guard rejected them — correctly.

Both alternatives asked the ticket to change what it moves. The third one —
relocating the module's own clean Postgres adapter, or relabelling repository
tests as workflow-decision tests, so that the role points become
candidate-only — would have damaged the target architecture to satisfy the
tool.

The compatibility permissions the earlier relocations did buy are not free
either: 458 rule descriptions in `policy.json` read "compatibility permission",
and each one is committed to a later conversion back to exact debt. A
relocation that keeps its keys makes that work unnecessary by construction, and
it keeps each consumer's debt visible under the consumer's own migration issue
instead of dropping it and re-deriving it later.

## What a declaration may do

`policy.json` gains an optional `relocations` array of `{from, to, issue}`,
sorted by `from`. A declaration is active while `from` is still classified in
the base policy; afterwards it is a historical record and cannot replay. An
active declaration must prove all of:

- the policy epoch increased, as for every reviewed mechanism
  ([ADR 0014](0014-test-roles-may-import-test-infrastructure.md),
  [ADR 0015](0015-owners-adopt-existing-unowned-tables.md),
  [ADR 0022](0022-parent-portal-is-an-application-workflow.md));
- `from` is classified in the base policy and gone from the candidate, and `to`
  is classified in the candidate and absent from the base;
- owner, production role, internal-test role and external-test role are
  identical on both sides;
- `to` holds exactly the Go files `from` held at the immutable base commit,
  read from Git rather than from the declaration.

The last condition compares file *names*, and it cannot compare contents: a
relocation must rewrite the import paths and may rename the package clause, so
the bytes necessarily differ. A wholesale rewrite that kept every file name
would pass it. It is a sanity gate on the declaration, not the safety property.

The safety property is the unchanged violation-key set, and it holds whatever
the files contain: an import the target has beyond the renamed base entries is
still a new violation, and a renamed base entry the target no longer earns is
still stale. A declaration can therefore rename keys onto a package, but it can
never give that package a dependency it did not earn.

## What it cannot do

Renaming the base keys changes nothing else, so every other guard still
decides:

- an import the candidate adds has no renamed twin and stays a new violation;
- an owner or role change fails the declaration, so a move may not become a
  reclassification;
- a changed migration issue is still a reassignment and is still rejected;
- a base entry whose import disappeared is still stale and must be removed;
- ownership, read projections, composition surface and the semantic table
  rules are untouched.

The declaration is not an input to rule anchoring. Whether a package counts as
candidate-created still comes from Git, exactly as it does for a package the
candidate writes from scratch, so a relocation neither gains nor loses the
anchor it would have had without the declaration. It buys only the renamed
keys.

## Consequences

- `#3226` moves all three packages with 0 new rules, 0 owner or role changes
  and 690 baseline entries whose issues are unchanged; 65 of them are simply
  read at the new paths.
- The remaining relocation-shaped tickets under #2580 —
  [#3230](https://github.com/moto-nrw/project-phoenix/issues/3230),
  [#3231](https://github.com/moto-nrw/project-phoenix/issues/3231),
  #3349–#3356 and most of Block 10 — use the same declaration instead of a new
  compatibility-permission tail.
- The issue audit keeps working: a relocated entry still names one open
  migration issue, and that issue is still the consumer's.
- Declarations accumulate in `policy.json` as the record of which path became
  which. A new package may not reoccupy a relocated path while its declaration
  stands; validation rejects that.
- A branch whose base predates a relocation still has `from` classified in its
  base policy, so the declaration stays active for it and the file-set check
  runs against that branch's own tree. Merging `development` into the branch
  settles it, because the merge-base then contains the relocation and the
  declaration goes inert. The failure, if one is ever reached, is loud and
  fails closed.

This decision was authorized in the implementation session on 2026-09-18.
