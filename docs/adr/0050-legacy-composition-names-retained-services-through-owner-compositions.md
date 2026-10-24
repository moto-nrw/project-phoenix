---
status: historical
---

# The legacy composition names retained services through owner compositions

> Historical record of a completed cutover. The rules that still hold
> are in [ADR 0068](0068-principle-owners-compose-themselves-and-the-roots-only-wire.md).

> Amended on 2026-10-15 by [ADR 0057](0057-only-the-target-is-a-rule.md): the
> `compose/vocabulary` and `retained` aliases this decision left in place are
> debt, not target. #4085 removes them.

Accepted for [#2747](https://github.com/moto-nrw/project-phoenix/issues/2747)
under [#2580](https://github.com/moto-nrw/project-phoenix/issues/2580), following
the precedent of ADR 0049. The pull request review is where it can still be
rejected; the exception activates only in the policy epoch that review merges
(37 → 38).

## Context

#2747 records 26 baseline keys of `services`
(`legacy-composition`/`compose`). Fourteen of them target an owner's
application or domain package (`services/activities`, `services/audit`,
`services/config`, `models/users`, …) or a platform package the legacy
composition may not import (`analytics`, `email`, `realtime`,
`internal/timezone`, `database/sql`). The legacy composition still binds
those services and their vocabulary, so the keys fall only if it reaches
them through a point it may import.

`modules/settings` already re-exports the Settings Platform's keys and
service types "under the contract's names" (#2736). Most owners' `compose`
points are reachable from `legacy-composition`/`compose` and may import their
own application and domain packages, so the same move works for them without
any policy change. Five pairs have no rule although both points exist at the
base, and PR mode reports a new rule between existing points as loosening:

| Source | Target | Needed for |
|---|---|---|
| legacy-composition/compose | document-rendering/contract | the list renderer and list vocabulary (`modules/documentrendering/lists`) |
| legacy-composition/compose | transaction-runtime/public | the dashboard database statistics and the missing-row sentinel (`modules/transactionruntime`) |
| data-import/compose | data-import/application | naming the retained import services (`modules/dataimport/compose/retained`) |
| audit-platform/compose | audit-platform/application | naming the append command, consent recorder and tag-scan log |
| audit-platform/compose | audit-platform/domain | re-exporting the audit records the legacy composition binds (`modules/auditlog/compose/vocabulary`) |

Two further rules anchor on a point the candidate creates
(`staff-clock`/`compose`, `workflows/staffclock/compose`) and need no
exception: `staff-clock.compose.application` and
`legacy-composition.to.staff-clock-compose`.

Alternatives considered:

1. **Aliases of `bun`, `tenant` or the JWT adapter** for the remaining three
   keys. Rejected: they would satisfy the ratchet without moving any code.
   Those keys need the legacy composition to stop holding a database handle,
   a tenant transaction or the request's JWT context.
2. **Moving the retained application packages into the owners' module
   trees.** Out of scope for #2747. A relocation (ADR 0020) renames keys and
   removes none.

## Decision

Grant exactly the five production pairs above in a reviewed epoch, anchored
on the base classifying `services` as `legacy-composition`/`compose` with
`module-internal-test` and `module-behavior-test`
(`internal/architecture/service_factory_cutover.go`). The matching rules
carry `issue: #2747`; they exist only while the legacy composition binds the
retained services, and `rules.stale` removes them with the last consumer.
Their descriptions avoid the marker phrases that
`TestPolicyTemporaryRulesRatchet` counts. Like ADR 0049 the exception forgives
no debt, changes no ownership, adds no external dependency, and grants
nothing to any other owner, role or scope. Because the policy classifies by
owner and role, the legacy-composition pairs also cover
`database/repositories/studentintegration`.

The re-exports live in the owners' compositions or contracts, never in
`legacy/` trees (`TestModuleLegacyBudgetRatchet`):
`modules/{settings,organizationtenancy,auditlog,peopledirectory}/compose/vocabulary`,
`modules/delivery/compose/{email,realtime}`, `modules/dataimport/compose/retained`,
`workflows/staffclock/compose`, and additions to existing compose and
contract packages. Each vocabulary package lists only names the legacy
composition uses and should shrink as consumers move to the owner's public
contract.

The database, tenant-transaction and JWT reads of the legacy composition move
into the owner compositions or are injected by the root as built values. No
exception is needed for that part.

## Verification

`internal/architecture/service_factory_cutover_test.go` covers the five
granted pairs in production, refusal in both test scopes, refusal of other
sources and targets, and refusal when the anchor fails: same epoch, `services`
missing from the base or reclassified, or a foreign module path.
`scripts/backend-architecture.sh check` passes with the candidate policy
against the merge base.

## Consequences

The baseline falls from 109 to 86 entries; 23 of the 26 #2747 keys are gone.
The `bun`, `tenant` and `legacy/jwt` keys of `services` remain. The builders
that compose several owners over one database (`services/*_roots.go`,
`command_roots.go`, `cleanup_composition.go`, `factory.go`) and about 90 test
hooks still need them. A legal home for that composition, whether the root
with new permissions or something else, is a separate decision.
