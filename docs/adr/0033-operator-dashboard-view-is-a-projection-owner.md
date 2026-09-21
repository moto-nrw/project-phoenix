---
status: accepted
---

# Operator dashboard view is a projection owner

[PR #3297](https://github.com/moto-nrw/project-phoenix/pull/3297) introduced
the projection owner `operator-dashboard-view` without the architecture
decision that
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580) requires for a
new owner. [#3414](https://github.com/moto-nrw/project-phoenix/issues/3414)
asked to record one or dissolve the owner. We keep it and record the decision
here. No production code and no policy entry changes.

## Why a separate projection owner

The operator dashboard counts organisations, schools, accounts and PWA usage
across the installation. Its one package,
`modules/organizationtenancy/internal/adapters/operatordashboard`, lives in the
Organisation & Tenancy tree, but it reads seven tables of three write owners:

| Table | Write owner |
|---|---|
| `platform.organizations`, `platform.schools` | `organization-tenancy` |
| `auth.accounts`, `auth.account_tenants`, `auth.account_roles`, `auth.roles` | `identity-access` |
| `iot.pwa_standalone_usage` | `observability` |

Only an owner of kind `projection` may hold a `read_projections` entry, and
that entry is the cross-owner read grant. Classifying the package as
`organization-tenancy` would delete the grant for the five foreign tables. The
replacement would be owner queries on `identity-access` and `observability`
behind consumer-owned ports, for a read-only operator table that joins all
three owners in one statement. That cost buys nothing, so the owner stays.

It owns no table and writes nothing: its reads are compile-time-constant
`SELECT` statements, and no data object names it as write owner. It is not a
persistent read model, so the #2580 condition that persistent read models need
a measured demand does not apply.

## What `tenant_safe` asserts here

The projection reads every tenant by design, so `"tenant_safe": true` cannot
mean a tenant predicate. It asserts that the read is reachable only from the
operator scope:

- every call runs inside `Provisioning.inAdmin` → `tx.RunAdmin`, the
  cross-tenant administrative transaction, which clears the tenant from the
  context;
- the HTTP routes that reach it sit behind `RequiresOperatorScope` and
  `RequiresActiveOperator`.

The evaluator checks only that the flag is `true`, not these two conditions.
They are the invariant. A caller outside the administrative transaction, or a
route outside the operator group, breaks this decision and needs a new one.

## The duplicated role name

`guardianRoleName = "guardian"` repeats an Identity & Access role name as a
literal, because a projection reads another owner's rows without importing
that owner's package. This is the accepted price. Renaming the base role in
Identity & Access must update the literal in the same change; no check
enforces that today.

## Policy registration

Unchanged: owner `operator-dashboard-view` (kind `projection`), read projection
`operator-dashboard`, and the rules `operator-dashboard-view.domain`,
`organization-tenancy.compose.operator-dashboard`,
`operator-dashboard-view.integration-test.postgres` and
`operator-dashboard-view.integration-test.domain`. The owner id and the
projection id differ on purpose, like `parent-request-feed-view` and
`parent-request-feed-read-model`; do not add a third spelling.

## Consequences

`operator-dashboard-view` and `organization-tenancy` import each other at owner
level: composition constructs the projection, and the projection returns
Organisation & Tenancy domain values. That is the projection pattern, and
[ADR 0032](0032-the-target-may-keep-two-cyclic-components-and-they-only-shrink.md)
lists both owners in component 1.
