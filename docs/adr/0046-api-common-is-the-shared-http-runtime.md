---
status: accepted
---

# api/common is the shared HTTP runtime

Decision for [#2738](https://github.com/moto-nrw/project-phoenix/issues/2738)
and [#3898](https://github.com/moto-nrw/project-phoenix/issues/3898). The user
expanded #2738 to include this decision, targeted policy changes and caller
changes. The implementation remains subject to PR review.

## Context

The native capability blockers are closed, but `api/common` still binds the
shared HTTP runtime: portal gates, validated principals, tenant transactions,
request memos, response rendering and uploads. These are live responsibilities,
not obsolete business providers. Its existing `inbound-common/http` point had
only the Presence snapshot permission. A Contract restricted to deletion could
neither preserve the runtime nor import its existing owners.

## Decision

Keep `api/common` and its existing owner and role. It owns HTTP adaptation,
not business decisions, SQL error details, byte persistence or retained rows.
Policy epoch 33 adds exactly these production reaches:

| Target | HTTP purpose |
| --- | --- |
| `identity-access/adapter` | Permanent session adapter, scope gates and identity memo, as decided in ADR 0031 |
| `tenant-runtime/public` | Claim binding, transactions, rollback markers and observations |
| `security-runtime/contract` | Validated principal and permission vocabulary |
| `security-runtime/public` | Authorization decisions, seed-request gates and filename entropy |
| `settings-platform/public` | Setting keys, snapshots and tenant-keyed request memo |
| `delivery-platform/public` | Byte-object storage |
| `transaction-runtime/public` | Driver-free database failure classification |

The `inbound-common/adapter-test` point also gains `identity-access/adapter` in
both test scopes for session construction. Other public/contract test reaches
already exist. These are standing target rules, not compatibility allowances.
The comparison accepts only this finite selector set in an increased epoch;
owner-kind selectors, other source roles, application/ORM/model targets,
external classes, expanded scopes and `same_owner` remain forbidden.

## Contract and cleanup

- Settings use the existing public owner contract, with the same snapshot and
  memo identities. The retained settings service remains that owner's provider;
  no alternative resolver, environment fallback or cache is introduced.
- Guardian rendering consumes HTTP input values assembled from already-authorized
  loader results. Relationship-level submit rights and lifecycle status pass
  through unchanged. Student access takes the existing nil-safe authorization
  view instead of a persistence row.
- SQL error classification moves unchanged behind Transaction Runtime's public
  facade and private Postgres adapter. HTTP exposes neither a driver nor a DB.
- Byte storage becomes Delivery's native `objects` capability. The old
  `internal/storage` provider is deleted. The owner, path layout, private modes,
  traversal checks, cancellation and failed-write cleanup remain unchanged;
  public methods name byte-object operations and return stream metadata.
- `io.Reader.Read` is standard byte I/O, not entity CRUD. The evaluator still
  inspects its types/results and rejects application-owned `Read`, generic
  entity CRUD and ORM/internal-model leaks. A CLI fixture covers both sides.
- The unused DB-bearing `ProtectedTenantGroup` and `ProtectedSchoolGroup`
  wrappers are deleted. Callers use the existing database-free routes helpers.
  Their security/memo/transaction middleware order is unchanged. HTTP resources
  and composition constructors also stop taking or capturing the unused DB;
  actual query/transaction providers keep theirs. This retires eight stale
  production ORM rules and twelve HTTP-ORM ratchet entries. Constructor guards
  still reject missing live capabilities, identity functions and other required
  collaborators; an unused DB is no longer a dependency.
- The class-number renderer uses the same first ASCII digit run, and the
  Presence batch loader deduplicates IDs in first-occurrence order locally.
  Delete the now-unreachable `schoolclass.GradePrefix`; its grammar cases move
  to the active HTTP parser, with additional first-run/ASCII/overflow checks.

No owner, table, projection grant, schema, route, permission value, error body,
JWT validation rule or transaction boundary changes. No relocation declaration,
new debt or temporary rule stands in for retiring the 20 remaining keys.
Composition field/setter targets stay 596; the ledger falls from 262 to 242 on current development. The original
task-base run was 265 to 245; #3757 independently retired three calendar keys
before this change was rebased.

## Verification and rollback

The common HTTP suites, owner and route suites, unchanged production-router
goldens, storage failure/privacy tests, query budgets and architecture checks
are the acceptance surface. Selector/epoch negative tests keep the policy
change bounded. The generated caller inventory records zero references to the
retired wrappers/provider. The migration record supplies results and the latest
accepted runtime checkpoint. Revert the code-only change if a hidden caller is
found; no data migration or cleanup job is required.
