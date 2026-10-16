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

## Addendum (#3417): the layering of the remaining domain cycle

Decision of 2026-10-16. On `ced14cc9ff` the target graph had one cyclic
component of 14 owners. Without the domain → projection and domain → workflow
rules that point 2 already excludes, six domains still formed one component:
`care-plan`, `enrollment`, `people-directory`, `student-presence`,
`timetable-activities` and `workforce`. The directions are:

- People Directory is the lowest of them: it holds the master data the
  others read and depends on none of them.
- Care Plan sits above People Directory, Enrollment above Care Plan.
- Workforce sits above Student Presence.

Not target, therefore, and cut: People Directory's HTTP reach into Care
Plan, Enrollment and Student Presence (it moves into the root,
[#4079](https://github.com/moto-nrw/project-phoenix/issues/4079)),
`care-plan → enrollment` and `student-presence → workforce` (consumer-owned
ports the root binds,
[#4076](https://github.com/moto-nrw/project-phoenix/issues/4076)). The
remaining edges among the six (Care Plan → People Directory, Enrollment →
Care Plan, People Directory and Timetable & Activities, Student Presence →
Care Plan, Timetable & Activities → Care Plan and Student Presence,
Workforce → Student Presence and Timetable & Activities) are acyclic.

The 14 rules that closed the cycle are deleted and their 18 imports are
exact keys, the conversion point 5 prescribes. From policy epoch 48 the
policy loader rejects a target graph with a cyclic component.

## Addendum (#4084): the reach of a domain owner's implementation

Decision of 2026-10-16. #4084 moves domain logic, row mapping and repository
code out of the owner compositions into `application` and `postgres`. That
code names the owner's own `public` contract, the tenant runtime and the
public contracts of domains the owner already depends on. Measured on
`537bf3572e`, Timetable's `application` role had no rule into any of them,
while Care Plan, Enrollment, Student Presence, Workforce and five other
domains each carried a per-owner rule into their own `public` role.

From policy epoch 50 a reviewed epoch may add exactly these rules
(`reviewedInternalReachRule`, `internal/architecture/internal_reach_rule.go`):

1. `domain.application-public` and `domain.postgres-public`: a domain's
   `application` and `postgres` roles into its own `public` role, production
   only. They replace the nine per-owner rules of that shape.
2. Per owner, a domain's `application` or `postgres` role into the `public`
   role of `tenant-runtime` or `transaction-runtime`, production only. A
   domain depends on platforms (point 2); no other role of a platform is
   admitted.
3. Per owner, a domain's `application` or `postgres` role into another
   domain's `public` role, production only, when the base already grants the
   owner a production rule into that target. Such a rule adds no edge to the
   owner graph: it moves an edge the owner's composition or HTTP role holds
   into its implementation, so the direction recorded above stays, and the
   reverse direction stays forbidden.

4. The internal tests of that code (role `module-internal-test`, scope
   `internal_test` only) may reach what points 2 and 3 admit in production;
   they test unexported functions and move with the code.

A per-owner rule is added in the change that first uses it. The composition's
rule into the same target goes once the composition no longer needs it.
Projection, workflow and platform sources, other targets, external classes
and test scopes stay under the ordinary loosening guards.
