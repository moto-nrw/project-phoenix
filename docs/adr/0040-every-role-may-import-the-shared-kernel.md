---
status: accepted
---

# Every role may import the shared kernel

Accepted for [#3447](https://github.com/moto-nrw/project-phoenix/issues/3447)
and [#3448](https://github.com/moto-nrw/project-phoenix/issues/3448) in the
policy epoch that registers the rule, following ADR 0014 and ADR 0039. The pull
request review is where it can still be rejected; the exception activates only
in the epoch that review merges.

## Context

The shared kernel (`sharedkernel/calendar`, owner `shared-kernel`, role
`contract`) holds the four values the policy declares as a closed list:
`Date`, `WallClock`, `TenantID` and `CorrelationID`. The retained
`internal/timezone` package (`legacy-shared`/`domain`) only forwards to it:
`type Date = calendar.Date`, and every function calls its kernel counterpart.
Moving a caller from the wrapper to the kernel changes no type and no
behaviour.

Before this decision, 15 rules pointed at `shared-kernel`, every one of them
owner-specific, and 83 at `legacy-shared`/`domain`. No rule let an arbitrary
owner import the kernel. An owner that wanted to drop the wrapper therefore
needed a new rule per role and scope, and the policy comparison reports each of
those as a loosening. The presence HTTP adapter and its tests (#3447), the
settings test support (#3448) and the enrollment postgres adapter (#2733) all
stopped at that point and kept the wrapper, or a temporary rule for it, only
because the permission for the kernel did not exist yet.

A kernel that every owner may use by definition gains nothing from per-owner
grants. Writing them down owner by owner is bookkeeping without design
content: each rule says the same thing, each needs its own review as a
loosening, and the wrapper survives in the meantime.

## Decision

Permit, in a reviewed epoch, exactly one owner-agnostic rule: no
`source_owner`, no `source_owner_kind`, no `source_role`, the scopes
`production`, `internal_test` and `external_test`, and the target
`shared-kernel`/`contract`. Every role of every owner may import the kernel's
contract.

The comparison in `backend/internal/architecture/policy_compare.go` admits this
shape, and only this shape, under a reviewed epoch (`reviewedSharedKernelRule`).
A source narrowed to an owner, an owner kind or a role, a subset of the scopes,
any other target, an owner-kind target, the same-owner flag and every external
class stay under the ordinary loosening guards.

A `same_owner` rule no longer selects a kernel owner. Without that, the
owner-agnostic rule and the generic `internal-test.same-owner.contract` and
`module-behavior-test.contract` rules would both grant the kernel's own tests
the same edge, and the loader rejects overlapping rules. The kernel has no
private side, so its tests lose nothing: they reach the contract through the
shared-kernel rule alone. Every other owner keeps its same-owner grants.

The decision does not permit:

- importing `internal/timezone` or any other `legacy-shared` package; the 83
  rules for the wrapper stay as they are and fall with its callers,
- adding a type to the kernel; `shared_kernel_types` remains a closed list,
- a kernel role other than `contract`; the loader still refuses one,
- converting a wrapper rule into exact debt. That debt would only disappear by
  reintroducing a target rule; the edge is moved to the kernel instead.

This decision changes no runtime write owner, table, HTTP route, status code or
error string. The date values are the same type.

## Consequences for #3447 and #3448

With the kernel open, the two presence `legacy-shared-domain` rules and
`settings-platform.test-support.legacy-shared-domain` fall: the presence HTTP
adapter, its adapter tests and the settings test support import
`sharedkernel/calendar` directly.

The two presence `inbound-common` rules become standing target rules without
an issue. `api/common` is the shared HTTP runtime (tenant middleware, response
rendering, error helpers) and has no replacement; 30 rules from 22 owners point
at it, and before this change only the two presence rules carried an issue.

## Policy registration

Epoch 25 registers `shared-kernel.contract`, removes the 15 owner-specific
kernel rules it subsumes and the three `legacy-shared-domain` rules of #3447 and
#3448, and rewrites the two presence `inbound-common` rules as standing rules.
The legacy baseline is unchanged at 597 entries.

## Verification

`internal/architecture/shared_kernel_rule_test.go` covers the shape (the rule
accepted in any scope order; a missing scope, a source owner, owner kind or
role, the wrapper, the shared HTTP runtime, an owner-kind target, another kernel
role, the same-owner flag and an external class all refused), the epoch anchor
(the repository policy passes against a base without the rule only with the
epoch increase), the guard (other owner-agnostic rules are still reported in
the reviewed epoch) and the same-owner exception (the kernel is not selected,
an ordinary owner still is, and the kernel's own test edge resolves to the
shared-kernel rule alone).
