---
status: accepted
---

# Delivery and Export Transfer speak their own contract

Decision of 2026-10-20 under
[#4461](https://github.com/moto-nrw/project-phoenix/issues/4461), part of the
facade-collapse rollout
[#4403](https://github.com/moto-nrw/project-phoenix/issues/4403). It adds
three rules under [ADR 0057](0057-only-the-target-is-a-rule.md) and
[ADR 0070](0070-principle-only-the-target-is-a-rule-and-debt-leaves-with-evidence.md)
point 7.

## Context

The rollout removes each owner's `Module` and `Engine` layers: the
application service implements the public capability itself and speaks the
owner's published values, and domain copies that only duplicate those values
field by field are deleted. #4451 left two platform owners out because the
target policy has no rule for it:

- **Delivery.** `domain.ProviderResult`, `domain.EmailDeliveryStatus`,
  `domain.GuardianDisplay`, `domain.RelatedEntity` and `domain.WorkerStats`
  copy values of `modules/delivery/contract`. The provider and guardian
  ports and the Postgres store return three of them. The platform rules allow
  `port` and `postgres` to reach `domain` only; the two existing
  same-owner contract rules are pinned to `modules/delivery/subscriptions`.
- **Export Transfer.** The service returns domain copies that a composition
  engine maps onto the request, outcome and status values of
  `modules/exporttransfer`. The platform rules allow `application` to reach
  `domain` and `port` only.

The owner decision on #4461 approves exactly three same-owner directions,
modelled on `care-plan.application.care-plan.contract` and
`care-plan.port.care-plan.contract`: `delivery-platform/port` and
`delivery-platform/postgres` into `delivery-platform/contract`, and
`export-transfer/application` into Export Transfer's published package. The
decision names that target `export-transfer/public`; the policy classifies
`modules/exporttransfer` as the owner's `contract` role, and the owner has no
`public` package.

## Decision

1. Policy epoch 88 to 89 adds three production rules, each pinned to the
   owner's one contract package:
   - `delivery-platform.port.delivery-platform.contract` and
     `delivery-platform.postgres.delivery-platform.contract`, target package
     `modules/delivery/contract`;
   - `export-transfer.application.export-transfer.contract`, target package
     `modules/exporttransfer`.
2. The named acceptor `reviewedSameOwnerContractRule`
   (`backend/internal/architecture/same_owner_contract_rule.go`) admits
   exactly these three owner, source role and target package triples, with
   positive and negative fixtures. It admits no other owner, role, package,
   selector or scope.
3. Delivery's application service implements the public `Capability` of
   `modules/delivery/runtime`, including the input validation of the former
   `Module`. `Module`, `ModuleEngine`, `WorkerEngine`, the guardian directory
   adapter and the five copied domain values are deleted.
4. Export Transfer's service implements `exporttransfer.Capability`.
   `Module`, the composition engine, its result binders and the copied
   domain request and result are deleted.

## Consequences

- The targets stay inside their owner, so the owner graph gains no edge and
  no foreign implementation role is reachable.
- Delivery's pass-through entry falls from 12 to 6 and its composition LOC
  budget from 363 to 361; Export Transfer's composition LOC budget falls from
  168 to 149, and its pass-through entry stays at 1.
- Delivery keeps its internal `Intent`, `EmailDelivery`, `Enqueued` and
  `Observation` records, which carry row and metric fields the contract does
  not publish, and the `Transport` and `State` enums that type its store's
  state machine. Replacing those enums is not part of this decision.
- A further same-owner contract direction needs its own decision and its own
  acceptor entry.
