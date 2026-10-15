---
status: accepted
---

# Owners compose themselves, and the typed roots compose the owners

> Amended on 2026-10-15 by [ADR 0059](0059-the-roots-hold-no-service-graph.md):
> the roots hold no service graph, and the consequence that `api` grows while
> `services` drains no longer stands. Point 3's open cross-owner reach is
> decided there (#4004). A grant into another owner's `adapter` role is
> expressed as a classification, not a rule
> ([ADR 0057](0057-only-the-target-is-a-rule.md), point 3).

Decision for [#2747](https://github.com/moto-nrw/project-phoenix/issues/2747)
under [#2580](https://github.com/moto-nrw/project-phoenix/issues/2580). It
also settles the role of the Identity behaviour suites
([#3446](https://github.com/moto-nrw/project-phoenix/issues/3446)) and the
shared-fixture reach of the session token adapter
([#2748](https://github.com/moto-nrw/project-phoenix/issues/2748)). The policy
change activates in the epoch that
[#4001](https://github.com/moto-nrw/project-phoenix/issues/4001) merges.

## Context

ADR 0050 removed 23 of the 26 #2747 keys. Three remain: `services -> bun`,
`services -> tenant` and `services -> modules/identityaccess/legacy/jwt`. The
code that still needs them composes several owners over one database, and the
policy names no place where such code may live.

Measured on `6343f9082d`, policy epoch 40:

- The top-level `services` package has 260 production files with 35,439 LOC.
  The static closure of `newFactory` reaches 240 of them. Nine files import
  `bun`, `tenant` or the session token adapter.
- 104 `*_test_helpers.go` files (8,053 LOC) compile as production code. 72 of
  them build two or more owners and serve 422 test files of 39 owners. 88
  import `bun`, 34 `tenant`, one the session token adapter. Two of them are
  called by production builders.
- `newFactory` holds seven decisions that belong to an owner: the room
  deletion guard, the substitution visibility rule, the welcome-mail
  predecessor status, the pickup auto-excusal resync (which opens the only
  tenant transaction in `services`), the student lock order of the weekly
  pickup approval, the enrollment guards, and the parent request review
  binding.
- The root composition has 84 rules, almost all of the form
  `root-composition -> <owner>/compose|public|http`. Each new pair is a new rule
  between existing points, so PR mode reports it as loosening. Moving the
  builders into the root as written would need about 44 more pairs, eleven of
  them outside `compose`, `public` and `contract`: implementation roles of
  other owners, two `port` roles, the session token adapter and the ORM.
- 206 rules let an owner's `compose` reach another owner: 82 into `public`, 81
  into `domain`, `application`, `postgres` or `adapter`.

#2580 already sets the target. Server, Worker and Commands get typed
composition roots. Writes across modules run in an application workflow with
an outer unit of work. Tests build the typed roots. What is missing is the
policy shape for that target and a migration path that needs no exception.

## Decision

### 1. Two levels of composition

An owner's `compose` package builds its own owner and nothing else. The root
composition (`root-composition`: `.`, `api`, `cmd`) is the only place that
combines several owners. There is no new composition owner and no successor to
`legacy-composition`. Cross-owner glue, meaning an adapter that binds owner A's
consumer-owned port to owner B's facade, lives in the root.

### 2. The standing reach of the root

Every role of `root-composition`, in all three scopes, may import:

- the `compose`, `public`, `contract`, `port` and `http` roles of every owner of
  kind `domain`, `platform`, `workflow` or `projection`;
- `migrations`/`migration`, because the commands root runs the migrations;
- its own packages.

It may not import another owner's `application`, `domain`, `adapter`,
`postgres` or `cli` role, the legacy composition, or an `inbound`-kind owner
beyond the enumerated rules that already exist. When the root needs something
from one of those roles, the owner's `compose` exposes it.

The enumerated root rules that this reach covers are deleted. The three target
rules from the root into implementation roles
(`root-composition.to.workforce-adapter`,
`root-composition.compose.production.data-import.adapter`,
`root-composition.compose.scheduler-runtime-postgres`) become exact debt under
[#4003](https://github.com/moto-nrw/project-phoenix/issues/4003).

### 3. The standing reach of an owner's composition

An owner's `compose` may import every role of its own owner in production. This
replaces the enumerated same-owner compose rules. It grants nothing across
owners. No new cross-owner reach is added to an owner's `compose`; glue moving
out of `services` binds in the root. The 206 existing cross-owner reaches stay
until [#4004](https://github.com/moto-nrw/project-phoenix/issues/4004) decides
them.

### 4. Compositions construct and bind, they do not decide

A rule that belongs to one owner moves into that owner, behind a
consumer-owned port. A write across owners moves into a workflow under
`workflows/` with the outer unit of work. A composition holds neither. This
applies to the seven decisions listed above; each slice ticket names its own.

### 5. The database reaches the roots as one value

`modules/transactionruntime/compose` returns one defined type, not an alias,
that holds the pool and the tenant unit of work and can close the pool and
register query hooks. The roots hold only that value and hand its parts to the
owner compositions, whose signatures stay as they are. Root code names no
`bun` type. This removes `api -> bun`, `cmd -> bun` and both
`database.direct-access` keys of #2750, and it stops them from coming back
while code moves from `services` into the root
([#4002](https://github.com/moto-nrw/project-phoenix/issues/4002)).

### 6. The session codec comes from Identity's composition

The root passes configuration values: the signing secret and the token
lifetimes. `modules/identityaccess/compose` builds the signer and the verifier
through its own session token adapter (point 3). No root package and no
legacy-composition package imports the adapter for that.

### 7. Tests

These mirror the test seams of #2580.

1. **A test of one owner uses that owner.** Every test package of owner X, in
   both test scopes, may import X's `public`, `contract`, `compose`, `port`,
   `adapter` and `test-support` roles. An owner's test-support package
   (production role `test-support`) may import its own `public`, `contract`,
   `compose`, `port` and `adapter`. Other owners appear as fakes of
   consumer-owned ports or as rows created by the shared fixtures. Another
   owner's implementation roles stay forbidden.
2. **A test of several owners builds the typed root.** Every test package, in
   both test scopes, may import `root-composition`'s `compose` role. The rule
   `e2e-test.compose` narrows to `legacy-composition` and carries #2747 until
   `services` is deleted. Go still forbids an internal test from importing a
   root that imports its own package.
3. **The shared fixtures create rows and never compose owners.** ADR 0039's
   fixture targets gain `identity-access`/`adapter`. The session token adapter
   is the HTTP session runtime (ADR 0031, ADR 0046), and the fixtures reach it
   for the same reason they reach the tenant runtime.
4. `api/testutil` keeps only owner-agnostic HTTP helpers. It stops re-exporting
   the legacy composition.

The Identity behaviour suites keep their point (`identity-access`/
`test-support`, `e2e-test` in both test scopes). They are Identity's own
suites. Under 7.1 they may import Identity's public contract and its session
token adapter. `auth/rotation` is Identity's `application` role and stays
forbidden, so that import goes in code (#3446).

### 8. The way out of `services`

No bridge rule and no exception are needed. The way out follows from the
direction of the existing rules.

- Owner compositions are neutral ground: both `services` and the root may call
  them. Wiring of a single owner moves there first.
- Code that still calls `services` may live only in `api` or `cmd`, under the
  existing #2750 keys `api -> services` and `cmd -> services`. Code moves from
  the top down: a function leaves `services` only when no remaining `services`
  code calls it. No new package may import `services`.
- The server root builds each resource where it mounts it (#2745). It may hand
  a value it built into `services.NewFactoryWithModules` in place of what the
  Factory built itself, as ADR 0050 did. The composition surface stays equal
  or falls in every change. No Factory field, setter or capability is added.
- Moved code takes the Transaction Runtime value (point 5) and names no `bun`
  type.
- The test hooks move first
  ([#4005](https://github.com/moto-nrw/project-phoenix/issues/4005)). Once
  they sit in the root or in owner test support, the production slices
  ([#4006](https://github.com/moto-nrw/project-phoenix/issues/4006) to
  [#4013](https://github.com/moto-nrw/project-phoenix/issues/4013)) no longer
  pull the tests along.
- #2747 deletes the empty Factory and the top-level `services` package. It
  removes the remaining #2747 keys and the coupled keys `api/testutil ->
  services` (#2748), `api -> services`, `cmd -> services` and
  `api -> services.Factory` (#2750) in the same change. The classified packages
  under `services/` (`activities`, `audit`, `config`, `database`,
  `facilities`, `import`, `iot`, `listexport`, `parentmessaging`,
  `scheduler`) belong to their owners. Their path is not part of this
  decision.

### 9. How the checker accepts the reaches

Each reach in points 2, 3 and 7 is a finite selector set that the comparison
accepts only in an increased epoch, implemented the way
`reviewedCommonHTTPRule` (ADR 0046) and `reviewedSharedKernelRule` (ADR 0040)
are. Each has positive and negative fixtures. None of the new rules carries an
`issue` field: they are the target, not compatibility allowances.

## Considered options

1. **Enumerate about 40 new root rules.** Rejected. It is per-pair bookkeeping,
   each pair needs another epoch exception, and eleven pairs would point at
   implementation roles.
2. **A shared composition owner between the owners and the root.** Rejected.
   It would be a second legacy composition under another name. #2580 rules out
   catch-all modules, and a new owner needs its own decision.
3. **Glue in the consumer's composition.** Rejected for new glue. It adds
   owner-to-owner edges and can grow the cyclic component of ADR 0032. Binding
   in the root adds none, because no other owner imports the root in
   production.
4. **Aliases of `bun`, `tenant` or the session token adapter.** Rejected, as in
   ADR 0050: they satisfy the ratchet without moving code.
5. **A temporary rule that lets `services` import root packages.** Rejected. It
   would be an exception, and the top-down move works without it.

## Consequences

- #4001 removes the baseline entries that the new reaches allow. Expected,
  subject to measurement: the #2750 entries `api -> auth/authorize/permissions`,
  `api -> realtime` and `api -> tenant` (internal test), `api -> observability`,
  `cmd -> api`, `cmd -> tenant`, `cmd -> database/migrations` (production and
  internal test) and `. -> cmd`; the #3446 entries
  `behavior -> modules/identityaccess` and `behavior -> legacy/jwt`; the #2748
  entry `test -> legacy/jwt`. It adds three exact-debt entries under #4003.
- The cycle report reads production edges only, and no other owner imports the
  root in production. The reaches add no cyclic component.
- `api` grows while `services` drains. Splitting the root into more packages is
  a later, separate refactor; such a package may never import `services`.
- ADR 0054 renames the session token adapter after #4001, so fewer keys need
  renaming.
