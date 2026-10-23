---
status: accepted
---

# Enrollment interface bindings have reviewed command effects

Decision of 2026-10-23 for the first implementation slice of
[#4739](https://github.com/moto-nrw/project-phoenix/issues/4739), implementing
part of [ADR 0083 §2](0083-ratchets-protect-invariants-and-expose-command-effects.md#2-supplement-structural-checks-with-typed-command-effects).
No coordinator, owner, permission, budget or runtime behavior moves.

## Cut and alternatives

Start with **Enrollment's named interface binding surface to foreign domains,
workflows and projections**. Include interfaces declared in Enrollment, aliases
to foreign contracts and native contract fields declared by Enrollment. Derive
ownership from policy, not directory prefixes. All packages in the existing
fixed typed build supply bindings, including roots, relocated adapters and
`testhooks`; `_test.go` implementations are not production evidence.

This is a conservative **type-level may-bind inventory**, not a call graph or
proof of a live call. All actual assignments to the same interface type are
unioned, including assignments in other roots. The scan does not invent
implementations by testing every structurally compatible type. Some retained
bindings expose more methods than Enrollment calls. A port's full method set
is intentional: adding a command to its supplied interface must not evade the
gate merely because its first caller is added in another change.

A complete effect/data-flow analysis exceeds one reviewable PR. Scope the
first slice explicitly rather than silently calling unresolved values queries.
Do not infer effects from method-name prefixes. Use the existing `go/packages`
load and `go/types` objects; add no dependency or second production-source scan.

## Resolution and exact register

`backend/architecture/command-effects.json` is the new reviewed register allowed
by this decision (ADR 0070). `scripts/backend-architecture.sh effects` generates
its exact identities and fingerprints; reviewers supply effect/classification
and evidence. It preserves labels only on an exact identity match and never
invents labels for a new binding. The normal `check` validates it without
writing. The fingerprint uses `go/ast.Fprint` with positions, comments and parser object
links excluded, plus the existing standard-class `hash/fnv`. Missing, stale, duplicate, unknown-effect and changed-fingerprint
entries fail. The register is required whenever policy contains Enrollment packages.

The identity is resolved interface type/method, supplied type/method, terminal
type/method or function, and policy target owner. Aliases are canonicalized.
Typed struct literals, assignments, declarations, argument passing, explicit
conversions and returns establish interface bindings. Embedded interfaces use
complete method sets; promoted struct methods follow their embedded field.
Root adapters and root helper functions are followed recursively. Method
values count even without a direct call expression. Foreign owner interfaces
are contract terminals; the scan does not claim to inspect their guards.

A normalized fingerprint combines traversed root bodies and terminal contract
signatures. Formatting and file relocation do not erase the binding evidence;
a changed body with the same target set is still visible. It is not a digest
of every target implementation or a proof of the caller's protocol.

Unresolved bindings, cycles and function-valued fields produce exact
`unresolved:` targets. They fail unless an exact entry has effect and
classification `unresolved`, cites ADR 0089, and explains the boundary. These
entries remain visibly unresolved in JSON, not approved leaves or queries.
New unresolved entries require a new approval ADR in PR mode.

## Effects and PR-base contract

Effects are explicitly `query`, `mutation`, `lock`, `scope`, `durable` or
`unresolved`. The first slice's domain targets principally use query, mutation
and lock; it does not inventory platform outboxes, transaction runners or
non-durable notices. A locking read is `lock`, never `query`. All non-query
resolved entries need a classification and ADR 0067 evidence. `leaf` denotes
the exact transaction-participating command described by its reason, not an
owner-wide permission. `workflow` records a workflow-backed port. E1–E5 retain
the boundaries in ADR 0067, not a blanket permission to coordinate.

The initial register records Enrollment decision/approved-child-sync effects
as **E1**, not leaves. E2–E5 remain named classifications for later slices;
this decision does not grant them new sites. ADR 0067's class/recurrence/child/
request ordering, ambient units, savepoints, best-effort behavior and #4736
move-authorization decision remain authoritative.

The existing immutable `--base-ref` comparison also reads this register. New
leaves/queries need exact entries and reasons. New E1–E5 or unresolved entries,
changed effect/classification/ADR, and changed exception fingerprints require
`approval_adr` naming an ADR newly added by that PR. A changed body of an
existing ordinary entry requires updated review evidence. Removals are allowed
only when the current typed scan no longer finds the entry. Bootstrap against
a base without the register is permitted only while ADR 0089 is new.

## Limits and follow-up

- The scan stops at foreign owner contracts. Enrollment → People → Membership
  and Enrollment → Care Plan → Timetable are not flattened into invented direct
  edges. Record the actual first hop and reconcile the hand inventory separately.
- Other source owners, platform targets, direct foreign calls not supplied
  through an inventoried interface, generic/variadic and multi-result binding
  flow, reflection and runtime container/function dispatch need later slices.
  Function fields reached inside a scanned adapter are diagnosed, not resolved.
- Type-level assignment union can overapproximate which concrete implementation
  reaches Enrollment. It does not prove instance-level wiring, command liveness,
  transaction participation, authorization equivalence or deadlock freedom.
- One declared effect is a reviewed boundary label, not a complete set of all
  transitive effects. Fingerprints cannot replace the real transaction and
  concurrency tests cited by ADR 0067.

Extend source owners and target categories with fixtures and an inventory/base
comparison. Do not remove any existing ratchet. See the row-by-row
[reconciliation](../architecture/4739-command-effect-reconciliation.md) for
current counts, exact gaps and proposed follow-up scope.
