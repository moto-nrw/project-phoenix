---
status: accepted
---

# Only the target is a rule; everything else is exact debt

Decision of 2026-10-15 under
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580), carried out
by [#4070](https://github.com/moto-nrw/project-phoenix/issues/4070) within the
consolidation [#4069](https://github.com/moto-nrw/project-phoenix/issues/4069).

## Context

#2580 keeps the target in `policy.json` and every existing violation as an
exact tuple in `legacy.jsonl`, so that the ratchet shows the real debt and can
only shrink. It warns that a generous allowlist would colour existing faults
green. An audit on `4072c83a48` found that this happened anyway:

- 1,346 rules, 9 with an `issue` field. The README defines that field as the
  mark of a temporary permission, so 1,337 rules claim to be target.
- On a copy of the policy without the 180 rules whose own description calls
  them temporary, the evaluator reports 300 violations instead of 34. Without
  the rules into another owner's implementation roles, from platforms into
  domain modules and into the date wrapper as well, it reports 575; without
  the `inbound` owners, 753.
- ADR 0030, 0035, 0036, 0037, 0038 and 0052 let the successors of dissolved
  legacy packages keep the reach the old packages had, and each of them
  rejected converting it into exact debt. 142 rules cite those tickets, 47 of
  them into another owner's implementation roles; all those tickets are
  closed.
- `models/*` packages are classified as the `domain` role of their target
  owner, so 62 rules let other owners import ORM rows.
- `policy_temporary_rules_ratchet_test.go` counts rules by phrase and sees 49
  of the 180.

The ratchet therefore measured how much debt had been moved out of
`legacy.jsonl`, not how much was gone.

## Decision

1. **A rule states the target.** A rule may only permit an import the end
   state of #2580 keeps. Not target, and therefore never a rule:
   - any role of owner X importing another owner's `application`, `domain`,
     `adapter`, `postgres` or `cli` role;
   - any import of a `models/*` package by an owner other than its own;
   - a platform owner importing a domain, projection or workflow owner;
   - any import that closes a cycle in the target graph
     ([ADR 0058](0058-owner-dependencies-form-a-dag.md)).
2. **Everything else that exists is exact debt.** It stands as a tuple in
   `legacy.jsonl` with an open carrier issue, as #2580 already requires.
   Rules no longer carry an `issue` field; a temporary allowance is a key.
3. **A shared runtime is classified, not granted.** When a package is
   legitimately used by many owners, such as the session token adapter, it
   gets the role that says so (`contract`, `public` or `http`). A standing
   grant into another owner's `adapter` role is not the way to express it.
4. **`models/*` get their own role**, which no foreign owner may target, until
   they are dissolved into their owners.
5. **Ratchets outside the policy only fall.** A raised budget in any ratchet
   test needs an ADR. The measurement covers `compose` packages, `legacy_*.go`
   and `modules/*/http` as well
   ([#4071](https://github.com/moto-nrw/project-phoenix/issues/4071)).
6. **The conversion makes the number rise.** #4070 deletes every rule that is
   not target and adds its tuples to `legacy.jsonl` through the existing
   conversion path. The ratchet is expected to rise to between 300 and 750.
   That is the honest starting point, not a regression. Rules with a cutover
   name whose target is a `public`, `contract`, `port` or `compose` role are
   renamed, not converted.
7. **Exceptions that can no longer fire are deleted.** The epoch-gated
   acceptors of finished cutovers in `internal/architecture/*_cutover.go`
   whose base condition is a deleted package go together with their tests.

## Consequences

- The replacement-rule clauses of ADR 0030, 0035, 0036, 0037, 0038 and 0052
  no longer grant target reach. What they let stand becomes exact debt under
  the carriers #4072 to #4078, #4081 and #4004.
- A future cutover may not hand its successor the old reach as a rule. It
  converts the reach into exact debt or removes it.
- Ratchet 0 now means what #2580 promised: no permitted violation is hidden
  in the policy.

## Addendum 2026-10-16: exact package restrictions (#4078)

Deleting an `inbound` pseudo-owner can merge its HTTP role into a real owner
that already has other HTTP packages or workflow test suites. A successor
selected only by owner and role would then grant unchanged packages reach
they did not have. The #4078 transition must not accept that widening.

Policy schema 4 therefore adds optional `source_package` and `target_package`
selectors. Each names one exact, classified module-relative package, not a
prefix, glob, alias or new owner. Explicit empty/null and unknown paths are
invalid. A package restriction is an intersection with the existing owner,
owner-kind, role and scope selectors. It cannot grant foreign private roles,
platform-to-domain dependencies or cycles, or replace exact legacy debt.
The source restriction may also narrow an external-class rule; the target
restriction is first-party only.

Adding a restriction is narrowing. Removing or changing a restriction must
pass the ordinary strictness comparison as a possible widening. Comparing
only owner/role points is insufficient: matching, overlap detection and
permission caches retain the selected package identity. The target graph
still contains every selected owner edge, so restrictions cannot hide cycles.

This chooses precise rule restrictions over rehosting runtime wiring and
workflow tests solely to avoid selector collisions. No production rule,
owner, role, scope, debt key or budget changes with the checker feature.
Successor PRs still delete their predecessors, keep the rule count flat or
lower, and list every deleted and successor rule. The #4078 transition and
its tests still disappear with the final inbound owner. Immutable schema-2/3
bases remain readable during rollout; only schema 4 candidates may use the
new fields. No policy epoch increase is needed for this narrowing mechanism.


## Addendum 2026-10-18: completed inbound cutover (#4078)

The inbound-owner transition and its tests are removed with the last two
engineering owners' move to the distinct `tool` kind (ADR 0060 addendum).
Live candidates use schema 5 and reject `inbound`; package selectors retain
all schema-4 restrictions. Immutable historical formats remain readable
without restoring any completed ownership or role-transition acceptor.
