---
status: accepted
---

# The roots hold no service graph

Decision of 2026-10-15 under
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580). It replaces
the consequence of
[ADR 0053](0053-owners-compose-themselves-and-the-roots-compose-the-owners.md)
that "`api` grows while `services` drains" and defers splitting the root, and
it decides [#4004](https://github.com/moto-nrw/project-phoenix/issues/4004).
[#4080](https://github.com/moto-nrw/project-phoenix/issues/4080) carries it
out.

## Context

#2747 deleted `services.Factory` and moved its fields into
`api.serviceGraph`. Measured on `4072c83a48`:

- `api.serviceGraph` has 91 fields, the same names the Factory had. The
  composition ratchet maps the one onto the other
  (`internal/architecture/composition_ratchet.go`), so it reports 139.
- Construction runs in 33 stages. Helper structs (`graphBuild`,
  `serviceGraphInputs`, nine `*Wiring` structs, `IdentityMaterial`) hold about
  400 more fields that no ratchet counts.
- Services read late-bound values at call time. When such a value is nil,
  Care Plan and Enrollment save the change and log a warning instead of
  failing, and 14 production paths accept nil so that tests can run without
  the full wiring.
- `api` grew from 1,270 to about 49,000 production LOC.

#2580 requires that central service, repository and API factories are gone.
A struct that holds every service under another name is still one.

## Decision

1. **No root struct holds services of several owners.** The composition
   surface counts every such struct in `root-composition` packages
   ([#4071](https://github.com/moto-nrw/project-phoenix/issues/4071)) and
   reaches 0.
2. **Roots build owners as local values.** Each typed root (server, worker,
   commands) constructs the owners it needs in the build order of the
   acyclic owner graph ([ADR 0058](0058-owner-dependencies-form-a-dag.md)) and
   passes them as constructor arguments. An owner that holds process state,
   such as a cache, hub or limiter, is built once in the root function and
   passed to every mount that needs it.
3. **No late binding.** No function slot, setter, stage or value read at call
   time stands in for a dependency. A dependency that seems to need one is a
   cycle and is resolved under ADR 0058.
4. **A missing dependency is a construction error.** Production code has no
   branch that tolerates a nil dependency; tests pass explicit doubles.
5. **Each root builds what it runs.** The server builds what it mounts
   (#2745). The worker gets its own root and builds only what its jobs need
   ([#4083](https://github.com/moto-nrw/project-phoenix/issues/4083)).
   `NewServiceGraphForTests` goes; tests build route-sized routers or the
   typed roots (ADR 0053 point 7).
6. **Owner compositions reach no other owner's implementation or
   composition (#4004).** An owner's `compose` may name another owner's
   `public` or `contract` types where its own ports need them, within the
   directions of ADR 0058. Reaches into another owner's `application`,
   `domain`, `adapter` or `postgres` role are exact debt
   ([ADR 0057](0057-only-the-target-is-a-rule.md)). Reaches into another
   owner's `compose` or `http` role are glue and move into the root.

## Consequences

- The DoD line of #2580 on central factories now covers `api.serviceGraph`
  and its helper structs.
- Decisions found in the root while it is taken apart move to their owner or
  into a workflow first ([#4082](https://github.com/moto-nrw/project-phoenix/issues/4082)).
