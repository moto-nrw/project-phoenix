---
status: accepted
---

# The server root is the HTTP host

> Amended on 2026-10-16: the dev tools are a binary of their own
> ([ADR 0060](0060-the-dev-tools-are-a-binary-of-their-own.md), #4096).
> `cmd` no longer starts `seed`, `simulate` and `demo`, and the production
> image no longer ships them.

Decision for [#4063](https://github.com/moto-nrw/project-phoenix/issues/4063)
under [#2580](https://github.com/moto-nrw/project-phoenix/issues/2580). It
completes the root reach of
[ADR 0053](0053-owners-compose-themselves-and-the-roots-compose-the-owners.md)
for the HTTP and process runtime the typed roots assemble. The policy change
activates in epoch 46.

## Context

After #4065, 16 #2750 keys stood for imports the root needs to run the
process rather than to compose owners. ADR 0053 forbids the root these
reaches, and moving the code into an owner would need aliases (rejected in
ADR 0050 and in ADR 0053's considered options) or a role reclassification.

Measured on `79e2bfb51c`, policy epoch 44:

- The server root (`api`, `root-composition`/`compose`) builds the chi router,
  sets CORS, orders the global middleware chain, mounts every resource and
  runs `http.Server`: `net/http` (14 production files), `chi/v5` (12),
  `chi/v5/middleware` (2) and `go-chi/cors` (1). Seven owner `compose` roles
  already hold an owner-specific `*.compose.http-router` rule.
- `base.go` builds the global middleware chain from `api/common`
  (`inbound-common`/`http`, ADR 0046); six composition files bind owner
  failure kinds to its renderer. 14 files in all.
- The root mounts the session middleware (`Authenticator`, `Verifier`,
  `RateLimitKey`, `ParseAccessJWT`) and reads claims in seven files through
  the session token adapter (`identity-access`/`adapter`); ten internal tests
  put claims on requests. Only `newSessionTokenAuth` still built the signer
  itself, against ADR 0053 point 6.
- `middleware` (`tenant-runtime`/`adapter`: security headers, rate limiter,
  security logger, log redactors) and `analytics` (`observability`/`adapter`:
  tracker, core-action middleware, route table) have no importer besides the
  roots. ADR 0053 point 2 grants the root the `http` role of platform owners;
  #4064 wrote `root-composition.reach.platform.http` in epoch 45, but it does
  not cover an `adapter` role.
- The commands root (`cmd`, `cli`) starts the seeder, the demo and the
  simulator (`dev-tools`/`cli`, 17,347 LOC). They ship in the production
  binary; `assertDevOnlyTarget` and `validateDemoTarget` refuse non-dev
  targets at runtime. Its four internal tests drive cobra and viper, which
  production `cmd` already has (`external.cli-config.cli`).
- The Worker is filled with 37 exported symbols of `services/scheduler`
  (`scheduler-runtime`/`application`).

## Decision

### 1. The server root is the HTTP host

The server root hosts HTTP: the router, the server, CORS, the global
middleware chain and the route table. It holds no HTTP resources: a handler
with behaviour belongs to its owner's `http` role. Glue that binds an owner's
failure kinds to the shared renderer is binding, not a resource, and stays in
the root (ADR 0053 point 1). The two resources still in `api`,
`reportPWAUsage` and `serveStaffAvatar`, move to their owners under
[#4068](https://github.com/moto-nrw/project-phoenix/issues/4068).

### 2. Standing reaches of the host

Policy epoch 46 adds exactly these rules, without an `issue` field:

| Rule | Source role | Target | Scope |
|---|---|---|---|
| `root-composition.compose.http-router` | `compose` | class `http-router` | production |
| `root-composition.http-host.inbound-common` | `compose` | `inbound-common`/`http` | production |
| `root-composition.http-host.session-adapter` | `compose` | `identity-access`/`adapter` | production |
| `root-composition.internal-test.session-adapter` | `module-internal-test` | `identity-access`/`adapter` | internal test |
| `root-composition.internal-test.cli-config` | `module-internal-test` | class `cli-config` | internal test |
| `root-composition.cli.dev-tools` | `cli` | `dev-tools`/`cli` | production |

The session token adapter is the HTTP session runtime (ADR 0031, ADR 0046,
ADR 0053 point 7.3); every HTTP point and the shared fixtures reach it for
that reason, and the host does too. Its construction still comes from
Identity's composition: `compose.NewConfiguredSessionAuth` takes the signing
secret and the token lifetimes, and `NewConfiguredSessionTokens` builds the
codec from the same values. A JWT-agnostic principal (ADR 0054, option 2)
stays a separate decision.

### 3. Platform HTTP points

`root-composition.reach.platform.http` (ADR 0053 point 2, written by #4064)
covers the `http` role of platform owners in all three scopes. `middleware` becomes
`tenant-runtime`/`http` and `analytics` becomes `observability`/`http`: both
are HTTP middleware the host mounts, no other package imports them, and their
owners do not change. Whether host hardening belongs to the Tenant Runtime is
not part of this decision.

### 4. The Worker contract moves

The contract the root fills becomes defined types in a
`scheduler-runtime`/`contract` package, which the standing
`root-composition.reach.platform.contract` covers. Neither a grant into
`application` nor a `public` reclassification of the implementation is
acceptable. [#4067](https://github.com/moto-nrw/project-phoenix/issues/4067)
makes the move and removes `api -> services/scheduler`. The key keeps its
carrier #2750, because the baseline comparison rejects a changed issue on a
key; #4067 blocks #2750 instead.

### 5. How the checker accepts the reaches

`reviewedHTTPHostRule` (`internal/architecture/http_host_rule.go`) accepts
exactly the six rules of point 2, each pinned to its source role and scope,
only in an increased epoch, together with the first-party and external
imports they imply. Another source owner, an owner kind, another role,
target or class, `same_owner` or a wider scope stays a loosening. Point 3
adds no rule; the reclassified packages fall under the existing reach.

## Considered options

1. **Reclassify `api` as `root-composition`/`http`.** Rejected. The generic
   `external.http-router.http` rule would apply without a new rule, but `api`
   would lose its `compose` grants (`base.go` reads viper through
   `external.cli-config.compose`), and the host would look like an owner's
   HTTP adapter.
2. **Move hosting into `api/common` or a new owner.** Rejected. `api/common`
   would import every owner's `http` package, which imports `api/common`; a
   new hosting owner would be a catch-all module that #2580 rules out.
3. **Exact grants into `tenant-runtime`/`adapter` and
   `observability`/`adapter`.** Rejected. They break ADR 0053 point 2, while
   the `http` role describes both packages correctly.
4. **Move the claim reads to the validated principal of `api/common` and
   `security-runtime/contract`.** Not now: about 18 files, and it partly
   reopens ADR 0054 option 2.
5. **A separate binary for the dev tools.** Not part of this decision. It
   changes `go run . seed|demo|simulate` in the dev loop and the docs.

## Consequences

- 15 of the 16 keys leave `legacy.jsonl` (21 → 6 after #4064); `api -> services/scheduler`
  stays under #2750 until #4067. Composition surface 139 → 139.
- `api/demo_runtime.go` and `api/worker_lease.go` name `tenant.UnitOfWork`
  instead of the alias `api/common.TenantRuntime`.
- The security header and security logger tests run inside `middleware`.
  `api/middleware_ratelimit_test.go` and `api/base_utility_test.go` stay in
  `api`: they test the root's rate-limit wiring.
- `api -> api/operator`, the last #4064 key, is no longer blocked: the
  operator router is cross-owner glue the host may mount.

## Amendment of 2026-10-16

Point 4 named a `scheduler-runtime`/`contract` package. [#4067](https://github.com/moto-nrw/project-phoenix/issues/4067)
places the Worker's contract in `modules/schedulerruntime/ports` with the
`port` role instead. The package holds exactly what a port role is for: the
consumer-owned ports the Worker declares onto the owners
([ADR 0058](0058-owner-dependencies-form-a-dag.md) point 2), the results they
report and the dependency struct the root fills. The `contract` role would
have needed a new grant from the Worker's `application` role into its own
`contract`, which no standing reach covers; the Worker's own `port` is
already admitted by `platform.application-port`. Policy epoch 51 adds the
three standing ADR 0053 reaches the `port` role was missing for platform
owners, accepted by the existing acceptors: `root-composition.reach.platform.port`
(point 2), `owner-compose.platform.port` (point 3) and
`own-test.platform.port` (point 7.1). The constructors still come through
`modules/schedulerruntime/compose`; it stays within its LOC budget because
the retention run is built inside `NewWorker` and the root implements the
rollover runner itself. Neither a grant into `application` nor a `public`
reclassification is used, as point 4 requires.
