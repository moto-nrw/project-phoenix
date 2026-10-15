---
status: accepted
---

# Owner dependencies form a DAG

Decision of 2026-10-15 under
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580). It supersedes
[ADR 0032](0032-the-target-may-keep-two-cyclic-components-and-they-only-shrink.md).
[#3417](https://github.com/moto-nrw/project-phoenix/issues/3417) carries it
out.

## Context

ADR 0032 allowed the target graph two cyclic components, of 17 and 5 owners,
that would only shrink. Its subset check was never written and its follow-ups
are open. The first owner outside its lists joined on 2026-09-21
(`ea5880362d`), and #2731 merged both components. On `4072c83a48` the target
graph has one cyclic component of 32 of 58 owners with 139 edges; the
migration graph has one of 39.

ADR 0032 argued that removing domain cycles was modelling work beyond the
migration. Without it the modules cannot be changed, tested or built in
order on their own, which is what the migration is for. An audit measured
which rules hold the component together: removing the cross-owner reach of
owner compositions, the platform-to-domain rules and the cross-owner rules of
domain `http` roles leaves three pairs (`people-directory` with
`student-directory-view`, `care-plan` with `enrollment` and
`timetable-legacy-view`, `student-presence` with `workforce`).

## Decision

1. **The target graph is acyclic.** `scripts/backend-architecture.sh cycles
   --graph target` reports no cyclic component. CI fails on any component;
   there is no member list and no allowance.
2. **Directions.**
   - A platform depends on platforms and the shared kernel only. Where it
     needs domain facts it declares a consumer-owned port that the root binds.
   - A domain depends on platforms and on other domains' `public` or
     `contract` roles, in one direction per pair.
   - A projection reads domains. A domain that needs a projection's result
     declares its own reader port; the root binds it.
   - A workflow depends on domains, projections and platforms. Only roots
     depend on workflows.
   - Cross-owner glue lives in the roots (ADR 0053 point 1). No owner imports
     a root in production, so roots add no cycle.
3. **Two domains that need each other** get one target direction. The other
   direction becomes a consumer-owned port bound in the root, or the shared
   behaviour moves into a workflow when it writes to both. #3417 decides the
   direction for each of the three remaining pairs.
4. **The measurement counts every production package**, including
   test-support code compiled into production. Test scopes stay out.
5. **Transition.** By [ADR 0057](0057-only-the-target-is-a-rule.md) a rule
   that closes a cycle is not target. #4070 and #3417 convert those rules into
   exact debt, so the target graph is acyclic when #3417 merges, and the CI
   check is hard from then on. The migration graph, which includes the debt,
   may stay cyclic until the keys are gone.
6. The platform triangle edge `organization-tenancy -> delivery-platform`
   (ADR 0032) is cut as decided there, under
   [#4072](https://github.com/moto-nrw/project-phoenix/issues/4072).

## Consequences

- An acyclic owner graph has a build order. The roots can construct owners in
  that order without late binding
  ([ADR 0059](0059-the-roots-hold-no-service-graph.md)).
- The Definition of Done of #2580 contains this criterion in place of the
  ADR 0032 sentence.
