---
status: accepted
---

# The dev tools are a binary of their own

Decision for [#4096](https://github.com/moto-nrw/project-phoenix/issues/4096)
under [#2580](https://github.com/moto-nrw/project-phoenix/issues/2580). It
decides the separate binary that
[ADR 0056](0056-the-server-root-is-the-http-host.md) left open (considered
option 5). The policy change activates in epoch 49.

## Context

The commands root `cmd` (`root-composition`/`cli`) started the seeder
(`seed/api`), the simulator (`simulate`) and the demo process. Both packages
are `dev-tools`/`cli`, another owner's private role
([ADR 0057](0057-only-the-target-is-a-rule.md)), so #4070 turned the rule
`root-composition.cli.dev-tools` into the keys `cmd -> seed/api` and
`cmd -> simulate`. The production image shipped the dev tools; only the
runtime guards `assertDevOnlyTarget` and `validateDemoTarget` kept them off
staging and production.

Measured on `01b434087e`, policy epoch 47:

- No rule lets any package import `dev-tools`/`cli`, not even another
  `dev-tools`/`cli` package, and `dev-tools`/`cli` may not import `net/http`
  (class `http-router`). The seeder's HTTP client lived in the root
  (`api/command_adapter.go`) and reached `seed/api` through its `Adapter`
  interface.
- The demo process is deployed: the `demo-runtime` sidecar of
  `environments/demo.compose.yml` runs it from the server image. It keeps its
  state on its own `phoenix_demo` connection (`DEMO_DB_DSN`), the only role
  that may read `platform.demo_school_states.seed_state`, which holds
  credentials (migrations 1.15.403 and 1.15.406). The serving backend's role
  cannot read that column. `api.NewDemoRuntime` composed the four owners the
  sidecar needs on that connection.

## Decision

1. **The dev tools are the binary `cmd/devtools`** with the commands `seed`,
   `simulate` and `demo`. The commands root no longer imports `seed/api` or
   `simulate`. The HTTP client, the demo runtime with its lease, and the
   secure random source of the seeder move from `api` into the binary. The
   runtime guards stay; the demo keeps its `phoenix_demo` connection.
2. **`cmd/devtools` is `dev-tools`/`compose`, the only package in that role.**
   Policy epoch 49 adds exactly these rules, each from `dev-tools`/`compose`
   in production:

   | Rule | Target |
   |---|---|
   | `dev-tools.compose.cli` | `dev-tools`/`cli` |
   | `dev-tools.compose.http-client` | class `http-router` |
   | `dev-tools.compose.transaction-runtime` | `transaction-runtime`/`compose` |
   | `dev-tools.compose.organization-tenancy` | `organization-tenancy`/`compose` |
   | `dev-tools.compose.identity-access` | `identity-access`/`compose` |
   | `dev-tools.compose.student-presence` | `student-presence`/`compose` |
   | `dev-tools.compose.tenant-runtime` | `tenant-runtime`/`public` |
   | `dev-tools.compose.security-runtime` | `security-runtime`/`public` |

   The first is the owner's own reach. The others are the reach of an
   `inbound` owner's composition, which ADR 0057 point 1 and ADR 0059 point 6
   do not count as an owner composition. The binary names no owner's
   `public` type besides those two: it binds the owner calls whose signatures
   carry owner types as functions inside `NewDemoRuntime`; the state saves call
   `RememberDemoSchoolFields` and `UpdateDemoSchoolFields` on the demo-school
   capability the composition returns; Organisation & Tenancy builds its
   state type from those explicit fields.

   `dev-tools.compose.security-runtime` is there for entropy. The seeder
   generates the staff passwords of demo schools, which are reachable from
   the internet, so they come from a cryptographic source. Security Runtime
   owns that source (#3364, `securityruntime.FillRandom`), as it does for
   the server's demo orders. Importing `crypto/rand` directly is rejected:
   the class `crypto` is granted per owner, and a grant here would bypass the
   owner #3364 made responsible for entropy. Reading `/dev/urandom` through
   `os` is rejected as well: it reaches the same source while evading the
   class the policy tracks it by.
3. **The production image does not ship the dev tools.** `backend/Dockerfile`
   builds `cmd/devtools` only for its `demo` target; `production` stays the
   default target and contains the server binary only. The build workflow
   selects `demo` for the demo environment. The sidecar runs
   `./devtools demo`. Development and CI build or run it from source
   (`go run ./cmd/devtools <command>`).

## Considered options

1. **Internal server endpoints for the demo state.** Rejected. The serving
   backend would need the `phoenix_demo` credential and would serve the
   credential-bearing `seed_state` over HTTP, which the isolation of
   migrations 1.15.403 and 1.15.406 keeps out of its reach. The lease, a
   transaction-scoped advisory lock held for the life of the process, would
   become a long-held HTTP request.
2. **A child-process proxy.** The root keeps the HTTP client, the demo
   runtime and the lease and starts the dev tools as a child process that
   reaches them over pipes. No rule needed, but the dev tools could not run
   on their own, and the protocol exists only to satisfy the checker.
   Rejected.
3. **Keep the two keys as debt.** Rejected: the production image would keep
   shipping the dev tools, and #4096 would stay open without a path.
4. **A non-private role for `seed/api` and `simulate`** (option 2 of #4096).
   Rejected: the root's standing reach covers domain, platform, projection
   and workflow owners, not the `inbound` owner `dev-tools`, so it needs rules
   as well, and it would keep the dev tools in the production binary.

## Consequences

- `cmd -> seed/api` and `cmd -> simulate` leave `legacy.jsonl` (492 → 490 on `development` at merge time).
  Composition surface 1740 → 1724: the moved test helper of the HTTP client
  builds its adapter instead of setting a field on a pointer argument.
- `go run . seed|simulate|demo` becomes `go run ./cmd/devtools
  seed|simulate|demo`; CI builds a second binary for the seed smoke, the
  nightly performance run and the product screenshots.
- A new package in `dev-tools`/`compose` needs an architecture decision: the
  rules above name the role, not the package.

## Addendum 2026-10-18: tooling is not an inbound owner (#4078)

The final two owners of kind `inbound` are engineering tools, not HTTP
adapters. Schema 5 gives `architecture-check` and `dev-tools` the distinct
kind `tool`. Their six package classifications keep their owner IDs,
production roles and both test roles: the checker remains `application`,
the binary `compose`, the commands `cli` and the two value packages
`contract`. No package moves and no executable behavior changes.

Tools own no writable data and stay outside the domain dependency DAG, as
the binary already did. The existing, explicitly named dev-tools composition
rules keep their IDs and reach. `own-test.tool.contract` is the same-scope,
same-owner successor of `own-test.inbound.contract`. No other permission,
legacy key, policy epoch, composition surface or budget changes.

Live policies reject `inbound`, both as an owner kind and in rule selectors.
The immutable-base reader first validates the old wire format. Only the
final schema-4 inventory containing no actual inbound HTTP owner translates
the two canonical engineering owners and their kind selectors to schema-5
vocabulary. Earlier HTTP inventories remain historical data, not tools.
This is a wire-format migration before the ordinary comparison, not an alias
or a new ownership/permission acceptor. Existing kind changes, package-owner
changes and widened scopes still fail the ordinary guards.

The completed inbound-owner and epoch-47 role reclassification acceptors,
their tests, and the retired Common-specific acceptor disappear. Parent and
HTTP-host guards lose only their retired Common branches; current session,
tenant, router and CLI guards and their negative coverage remain.
