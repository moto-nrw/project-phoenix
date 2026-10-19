---
status: accepted
---

# Principle: owner dependencies point one way

Principle ADR of 2026-10-19 under
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580), written for
[#4091](https://github.com/moto-nrw/project-phoenix/issues/4091). It states the
rules for dependency direction and cycles that still hold. They come from
[ADR 0058](0058-owner-dependencies-form-a-dag.md) and its addenda, which keep
their measurements and history; this ADR changes none of them.

## Decision

1. **The target owner graph is acyclic.** The policy loader rejects a policy
   whose target projection has a cyclic component, and `scripts/backend-architecture.sh
   cycles` names it. There is no member list and no allowance; ADR 0058
   supersedes
   [ADR 0032](0032-the-target-may-keep-two-cyclic-components-and-they-only-shrink.md)
   (ADR 0058 point 1).
2. **Each owner kind depends in one direction.**
   - A platform depends on platforms and the shared kernel only. Where it needs
     domain facts, it declares a consumer-owned port that the root binds.
   - A domain depends on platforms and on other domains' `public` or
     `contract` roles, in one direction per pair.
   - A projection reads domains and may consume domain and platform `public`
     or `contract` roles; a domain that needs a projection's result declares
     its own reader port.
   - A workflow depends on domains, projections and platforms. Only roots
     depend on workflows.
   - No owner imports a root in production, so the roots add no cycle.
   (ADR 0058 point 2 and addendum #4314.)
3. **Two domains that need each other get one direction.** The other direction
   becomes a consumer-owned port bound in the root, or the shared behaviour
   moves into a workflow when it writes to both (ADR 0058 point 3).
4. **The recorded layering stands.** People Directory is the lowest of the
   coupled domains; Care Plan sits above People Directory, Enrollment above
   Care Plan, and Workforce above Student Presence (ADR 0058, addendum #3417).
5. **A domain's implementation follows the owner's existing edges.** A
   domain's `application`, `postgres` and `port` roles may reach the owner's
   own `public` role, the `public` role of Tenant Runtime and Transaction
   Runtime, and another domain's `public` role where the owner already has
   that edge; `postgres` may reach the owner's own `port` role. Such a rule
   moves an edge into the implementation and adds none to the owner graph; the
   reverse direction stays forbidden (ADR 0058, addendum #4084).
6. **The measurement counts every production package**, including test
   support compiled into production. Test scopes stay out (ADR 0058 point 4).
7. **Tools are outside the dependency graph.** `architecture-check` and
   `dev-tools` own no writable data and take no part in the domain DAG
   ([ADR 0060](0060-the-dev-tools-are-a-binary-of-their-own.md), addendum of
   2026-10-18).

## Consequences

- An acyclic owner graph has a build order, which the roots use to construct
  owners without late binding
  ([ADR 0068](0068-principle-owners-compose-themselves-and-the-roots-only-wire.md)).
  A dependency that seems to need late binding is a cycle and is resolved
  here.
- A rule that would close a cycle is not target and cannot be written
  ([ADR 0070](0070-principle-only-the-target-is-a-rule-and-debt-leaves-with-evidence.md)).
  Both the target and the migration graph currently have no cyclic component.

## References

ADR 0058 with its addenda carries the decision; ADR 0032 is superseded by it,
and ADR 0060 adds point 7. The full rule map is the
[traceability appendix](../../backend/architecture/README.md#appendix-principle-traceability).
