---
status: accepted
---

# Command effects follow callable binding flow and record platform protocols

Decision of 2026-10-24 for [#4836](https://github.com/moto-nrw/project-phoenix/issues/4836).
It closes what [ADR 0094](0094-command-effect-gap-reconciliation.md) left
open: root closures behind domain-owned callable fields (M3), generic instance
flow, and platform/protocol effects, which ADR 0083 named as follow-up work.
No runtime owner, command, transaction, permission, policy rule or budget
changes. Only `internal/architecture` and the reviewed register change.

## 1. Callables are resolved, not only fingerprinted

Function-valued fields, callable variables and parameters are followed through
their typed values to the code that populates them. Each value resolves as follows:

- Root closures and root helpers are walked like root adapters.
- Method values and method expressions resolve their receiver like any binding.
- Named functions of another owner, and functions returned by its factories,
  are first-hop terminals. Kernels and libraries create no binding.
- Factory calls of the root or the source owner are followed to the callable
  expressions they return.
- Composite literals, conversions, generic instantiations and parentheses are
  followed through their elements and operands.

A resolved callable records its terminals, not an `unresolved:` diagnostic. The
assignment evidence from ADR 0094 stays in those terminals' fingerprints, so a
changed closure still fails as a changed binding. A diagnostic remains when any
value cannot be traced, for example a parameter whose callers are only in
`_test.go` (most `api` and `roots` `testhooks` helpers), a non-static
expression or an anonymous interface. Its fingerprint keeps every assignment,
so a changed traceable closure beside an untraceable source still needs fresh
approval. Function fields of container type (`[]func()`, `map[K]func()`) are
followed like single callables.

Two precision rules keep this a may-bind inventory rather than a union of all
code. First, a value the source owner computes or leaves unset is the owner's
own implementation, not a supplied binding. Second, the owner's own arguments
to its own parameters are summarized where they occur, in the caller reached
from the port. Without that rule a shared runner such as
`RunWrite(ctx, callback)` handed each port every other port's work: the first
measurement grew the register to 19,845 entries (1,214 of them false
Domain → Workflow hops); the precise rule kept the first measured register at 7,447.

## 2. Domain-owned callable fields (M3)

The scan used to stop at a concrete receiver of the source owner. It now
summarizes that receiver's method once per function, independently of the
port. A summary contains the callables the method reads or forwards, and the
owner's concrete methods and package functions it calls statically. Only those
callables are resolved for the port. The owner's own bodies are not binding
evidence. Own interfaces remain separate ports; foreign domain implementations
are still not walked. The ADR 0094 counterprobe now fails:
`GrantGuardianTenantAccess` inserted into the real `StaffColleagueReads` root
closure reports `effects.uninventoried` for both reachable ports and
`effects.changed` for the closure's existing terminals.

## 3. Generic flow, unexported helpers and forwarding cycles

Instantiated objects resolve through their `go/types` origin. This covers
bodies, callable values and parameters. A generic callee also binds its type
parameters from the argument types, so an interface reached through
`func wrap[W Port](w W)` resolves to the supplied implementation. Unexported
root helper methods are looked up in their declaring package; before, 152
diagnostics only reported a failed lookup.

Re-entering a binding identity already on the current path adds no target: the
first visit explores every source of the type-level union (least fixpoint).
The 296 `unresolved:cycle:` diagnostics therefore disappear without losing a
terminal.

## 4. Platform protocol effects

Owners of kind `platform` are now terminal targets. Methods of platform types
and interfaces are always inventoried. A platform package function enters a
protocol, and is inventoried, when its signature shows one of these traits:

- it runs or registers a callback (transaction, savepoint, after-commit, retry),
- it can fail (lock acquisition, current-unit lookup),
- or it returns nothing but takes a `context.Context` (rollback marks).

Pure value helpers such as `tenant.FromContext`, `realtime.NewEvent` and
`HasPermission`, and the context-free metric sinks `observability.Observe*`,
are not protocol effects. The rule reads the typed signature, not a name, so
it overapproximates: fallible helpers such as `tenant.TenantFromContext`,
`VerifyPassword` or `FillRandom` are inventoried too, reviewed as `query`.

Non-query platform entries use the new classification `platform` and cite this
ADR. It describes reviewed participation in an infrastructure protocol, not a
cross-owner command or an exception, so a new entry needs exact evidence but no
fresh approval ADR. Platform reads stay `query`. `check` rejects `platform` on
non-platform targets and on reads, and requires it for platform writes, locks,
scopes and deliveries. The reviewed effects are:

| Effect | Platform targets |
|---|---|
| `scope` | `RunInTx`, `WithinAdmin`, `WithinCurrentTenant`, `WithTenantTx`, `WithSavepoint`, `UnitOfWork`, `TenantCallback`, `RequireTransaction`, `ClaimScope.Bind*`, `MarkRollback`, `RegisterAfterCommit` |
| `lock` | `tenant.AcquireLock`, settings `Lock*`, `AuthEventRepository.LockStaffPreview` |
| `mutation` | audit `Append`/`Create*`/`DeleteOlderThan`/claims, delivery cancel/delete/claim, upload save/remove, push-subscription deletes, schedule and work-time writes |
| `durable` | outbox `Enqueue*`, `AttachEmailOutbox`, `ReplaceEmailDeliveries`, `DeliverEmail`, notification dispatch (`Notify*`), `exporttransfer.Transfer` |
| `notice` | `realtime` broadcasts, group-event queues, `analytics.Tracker.Capture`, `RequestIdentityCache.Evict` |
| `query` | settings resolvers and values, renderers, policy/permission checks, readers, value setters |

Notification dispatch runs outside the caller's rollback; it is `durable`
because its outcome survives the caller, not because every channel is an
outbox. The inventory records which platform protocol a binding enters. It does
not prove transaction participation, savepoint use, after-commit timing,
authorization equivalence or lock order. The ADR 0067 behavior and contract
tests remain the evidence for those properties.

## 5. Classification of newly exposed entries

Generation never labels entries. These rules were applied in review:

1. A target the base already reviewed keeps its effect.
2. A write or lock on a target the base reviewed as `leaf` stays `leaf`.
3. Other new Domain → Domain writes and locks are `debt`. E1–E5 grants cover
   only their recorded protocols and are not extended automatically.
4. Domain → Workflow hops are `debt` (#4825); HTTP entries stay `workflow`.
5. Everything else follows the platform table above or is a `query`.

The first 96 new debt entries are visible coordination findings, not approvals
(56 more follow in the follow-up section below):

- Care Plan, Enrollment and School Calendar take `LockRecurrenceWrites` (48).
- Enrollment decisions use Care Plan's weekly-pickup lock and excusal resync (12)
  and its companion-edge delete (2).
- Invitation acceptance prepares and resets Workforce employment (8); creating
  staff reinstates School Membership staff (2).
- Timetable template writes resync Care Plan rosters (8).
- People Directory queues File Storage cleanup (3) and takes School Membership,
  Timetable and Workforce locks (6).
- Workforce binds School Membership schedules and updates employment
  details (5); School Structure reaches Shift Plan Sync's after-commit (2).

Their protocol review is follow-up work under ADR 0067.

## Register delta and approval

Against development `1fb9eac4fa`, the register goes from 5,353 to 10,257
entries: 5,521 added, 617 removed and 25 changed fingerprints. The removed
entries are only diagnostics: 296 cycles, 152 failed helper lookups, 130
callables and 39 function fields. No previously resolved terminal disappears.
Added: 2,067 queries, 259 leaves, 152 debt, 737 platform (425 scope,
128 mutation, 115 notice, 60 durable, 9 lock) and 2,306 unresolved. The 25
changed entries are queries; their reasons record the new evidence.

The register then contains 2,331 unresolved entries across 56 identities.
Three observer callables account for 1,697 of them: People Directory (750),
Workforce (607) and Student Presence (340). A diagnostic run traced every
untraceable source to `api` `testhooks` helpers (`*_test_helpers.go`) and two
`roots/owners` clock parameters whose callers live only in `_test.go`. Effects:
4,855 query, 1,840 mutation, 616 lock, 432 scope, 122 notice, 61 durable and
2,331 unresolved. 2,458 entries (2,306 unresolved, 152 debt) cite
`approval_adr: "0098"`. The counterprobe that removes this ADR rejects exactly
those new exceptions. Once this ADR is in the base, it approves nothing further.

## Remaining limits

- Direct foreign calls in domain code outside a supplied interface remain
  governed by the policy import rules, not this inventory.
- Anonymous-only source ports and reflection are not discovered.
- Callables supplied only by `_test.go` stay `unresolved`.
- Static may-bind accounting can overapproximate which implementation reaches
  a port. It does not prove liveness or instance wiring.

## Callable resolution follow-ups (quorum review of PR #4879)

The own-argument precision rule first skipped too much:

- Local aliases (`saved := list`) are assignments, not call arguments, and
  stay traversable.
- A parameter reached through a stored field or a factory runs at
  construction, not on the port path. Its own arguments are followed too, so
  `NewReads(list) -> newReads(list) -> reads{list: list}` keeps the root
  binding.
- Only a parameter the owner invokes or forwards at port runtime leaves its own
  arguments to the caller's summary.

Factory results include named results at bare returns, sole tuple-valued
forwarding calls and root or source-owner factory methods. Active factories
stop direct and mutual recursion. Callable conversions follow their operand.
A source-owned named function used as a callable value is summarized like an
own method instead of counting as resolved. Fixtures cover each case; the
own-runner precision test is unchanged.

These repairs expose real reads that were silently missing, for example
Communication's injected People Directory and School Membership lookups. They
also expose 52 further debt entries:

- Workforce takes School Membership's `Query.FindStaffForMutation` row lock
  (41). The same method through `Capability`/`Owner` is a reviewed leaf; this
  interface is not, and no label is inferred from the method name.
- Student Presence takes Care Plan's `LockExceptionDay` (6).
- Facilities takes `LockRecurrenceWrites` (2).
- Workforce writes Timetable's `SetCategoryShiftTypeLinks` (2).
- People Directory takes School Membership's `StudentClassWriteGate`
  advisory lock (1).

A further review round found two more silent losses, both fixed with fixtures:

- A construction context is now kept for nested resolutions, such as a
  constructor storing `wrap(list)` whose closure calls `list`. The context is
  part of the callable cache identity.
- A root body calling the source owner's own function, such as
  `enrollment.Invoke(f)`, follows the callables that function invokes. It does
  not follow the owner's other effects.

Keeping construction context exposes real scope effects through stored runners
(for example Appointments → `tenant.WithinTenant`) and the observer diagnostics
above. It adds 4 debt entries: Timetable template writes reach Care Plan's
roster resync through one more interface each.

Policy, legacy baseline, composition inventory and runtime code remain
unchanged.
