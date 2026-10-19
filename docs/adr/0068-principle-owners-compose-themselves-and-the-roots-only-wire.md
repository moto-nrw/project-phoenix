---
status: accepted
---

# Principle: owners compose themselves, and the roots only wire

Principle ADR of 2026-10-19 under
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580), written for
[#4091](https://github.com/moto-nrw/project-phoenix/issues/4091). It states the
construction and runtime rules that still hold. The ADRs it cites keep their
context, measurements and history; this ADR changes none of them.

## Decision

1. **There are two levels of composition.** An owner's `compose` package
   builds its own owner and nothing else; it may import every role of its own
   owner. The root composition (`root-composition`: `.`, `api`, `cmd`) is the
   only place that combines owners, and cross-owner glue, an adapter that binds
   one owner's consumer-owned port to another owner's facade, lives there
   ([ADR 0053](0053-owners-compose-themselves-and-the-roots-compose-the-owners.md)
   points 1 and 3).
2. **The root reaches only the surfaces of the owners.** Every role of
   `root-composition` may import the `compose`, `public`, `contract`, `port`
   and `http` roles of every domain, platform, workflow and projection owner,
   the migrations and its own packages. It never imports another owner's
   `application`, `domain`, `adapter`, `postgres`, `cli` or `legacy-model`
   role; when the root needs something from there, the owner's `compose`
   exposes it (ADR 0053 point 2).
3. **An owner composition reaches no other owner's implementation or
   composition.** It may name another owner's `public` or `contract` types
   where its own ports need them, within the directions of
   [ADR 0071](0071-principle-owner-dependencies-point-one-way.md). A reach into
   another owner's `compose` or `http` role is glue and belongs in the root
   ([ADR 0059](0059-the-roots-hold-no-service-graph.md) point 6).
4. **Compositions construct and bind; they do not decide.** A rule of one owner
   lives in that owner behind a consumer-owned port; a write across owners
   lives in a workflow
   ([ADR 0067](0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md)).
   The root holds wiring only: binding, type and error mapping, configuration,
   route mounting, nil guards and calls into owner predicates and commands
   (ADR 0053 point 4,
   [#4082 inventory proof](../architecture/4082-root-inventory-proof.md)).
5. **The roots hold no service graph.** No root struct holds values of several
   owners. Each typed root (server, worker, commands) builds the owners it runs
   as local values, in the build order of the acyclic owner graph, and passes
   them as constructor arguments; an owner with process state (cache, hub,
   limiter) is built once per root function. There is no late binding: no
   function slot, setter, stage or value read at call time stands in for a
   dependency. A missing dependency is a construction error, and production
   code has no branch that tolerates a nil dependency; tests pass explicit
   doubles (ADR 0059 points 1 to 5). The central factories (`services.Factory`,
   `database/repositories.Factory`, `api.API`, `api.serviceGraph`) are deleted,
   and the composition-surface guard keeps their names so they cannot return.
6. **The database reaches the roots as one value.**
   `modules/transactionruntime/compose` builds the pool and the tenant unit of
   work; the roots hand its parts to the owner compositions, and root code names
   no `bun` type (ADR 0053 point 5,
   [ADR 0047](0047-shared-fixtures-reach-the-security-and-settings-contracts.md)
   addendum #3970).
7. **There is one session token runtime.**
   `modules/identityaccess/sessiontoken` is Identity & Access's `http` role.
   Nobody adds a second home for session middleware or claims, and the package
   depends on no process configuration, tenant runtime or persistence
   ([ADR 0031](0031-the-session-token-adapter-stays-and-moves-last.md),
   [ADR 0054](0054-the-session-token-adapter-leaves-legacy-now.md)). Identity's
   composition builds the signer and the verifier from the signing secret and
   token lifetimes the root passes (ADR 0053 point 6). Session facts cross
   ports as values, never as a context service locator or an alias of JWT
   types ([ADR 0046](0046-api-common-is-the-shared-http-runtime.md), amendment
   of 2026-10-17). A JWT-agnostic principal is a separate, open decision
   (ADR 0054, option 2).
8. **A runtime many owners use is classified, not granted.** It gets the role
   that says so (`contract`, `public` or `http`), never a standing grant into
   another owner's `adapter` role
   ([ADR 0057](0057-only-the-target-is-a-rule.md) point 3). Response rendering,
   the error envelope, pagination, request parsing (`api/common`) and the
   hosting middleware (`middleware`) are Tenant Runtime's `http` role; the
   tracker middleware (`analytics`) is Observability's. Settings, byte storage
   and observation stay with Settings Platform, Delivery Platform and
   Observability. No shared-HTTP or hosting owner exists (ADR 0046 amendment,
   [ADR 0056](0056-the-server-root-is-the-http-host.md) point 3). Every
   owner's HTTP role may reach these standing runtimes
   ([ADR 0051](0051-guardian-portal-http-reaches-the-standing-http-runtimes.md)).
9. **The server root is the HTTP host.** It holds the router, the server,
   CORS, the global middleware chain and the route table, and no HTTP
   resources. A handler with behaviour belongs to the `http` role of the owner
   of the data and operations it serves, including projection and workflow
   owners; binding an owner's failure kinds to the shared renderer is glue
   (ADR 0056 point 1,
   [ADR 0058](0058-owner-dependencies-form-a-dag.md) addendum #4079/#4314).
10. **The Worker's contract is a port of Scheduler Runtime.**
    `modules/schedulerruntime/ports` holds the ports the Worker declares, their
    results and the dependency struct the root fills (ADR 0056 point 4 and its
    amendment). The Worker builds only what its jobs need (ADR 0059 point 5).
11. **The dev tools are a binary of their own.** `cmd/devtools` (`seed`,
    `simulate`, `demo`) is the only `dev-tools`/`compose` package; the
    production image ships the server only, and a new package in that role
    needs an architecture decision
    ([ADR 0060](0060-the-dev-tools-are-a-binary-of-their-own.md)).
12. **Aliases do not stand in for moved code.** An alias or re-export that
    satisfies a ratchet without moving code is rejected
    ([ADR 0050](0050-legacy-composition-names-retained-services-through-owner-compositions.md),
    ADR 0053 option 4). Remaining vocabulary aliases in owner compositions are
    debt, counted by `test/module_compose_alias_ratchet_test.go`.

## Consequences

- The composition surface counts every root struct that holds values of
  several owners. It is 0 since
  [#4080](https://github.com/moto-nrw/project-phoenix/issues/4080), and PR mode
  rejects every added key.
- One exact key remains against point 3:
  `modules/dataimport/compose -> auth/authorize`, carried by
  [#4004](https://github.com/moto-nrw/project-phoenix/issues/4004). The
  Worker's own root is
  [#4083](https://github.com/moto-nrw/project-phoenix/issues/4083); the
  composition vocabulary aliases go with
  [#4085](https://github.com/moto-nrw/project-phoenix/issues/4085).
- ADR 0041, ADR 0049 and ADR 0050 are historical records; their lasting part is
  point 1 (each owner constructs its own application and persistence) and
  point 12.

## References

ADR 0046, ADR 0051, ADR 0053, ADR 0056 and ADR 0059 carry the decisions;
ADR 0031 and ADR 0054 the session token runtime; ADR 0047, ADR 0057, ADR 0058
and ADR 0060 the points named above. The full rule map is the
[traceability appendix](../../backend/architecture/README.md#appendix-principle-traceability).
