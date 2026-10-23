---
status: accepted
---

# Ratchets protect invariants and expose command effects

Decision of 2026-10-23 for [#4548](https://github.com/moto-nrw/project-phoenix/issues/4548).
Keep the structural safety gates. Replace location-dependent shape accounting
with method-level evidence, and add typed command-effect accounting through
root bindings. Do not equate a permitted import with a safe business protocol.
The decisions below define follow-up cuts, not new checker permissions.

The [measurement record](../architecture/4548-ratchet-policy-measurement.md)
contains immutable SHAs, commands, counts and limits. The inspected development
is `2c7907c2fe725dd7e124a8b866d5dce2d8b09718`. No policy, budget, owner,
transaction or production behavior changes here. #4642 and #4725 are merged
and included in the PR head; this decision does not change their code.

## 1. Keep invariants; replace scaffolding only with its successor

An empty import baseline is not evidence that a source constraint is obsolete.
The five requested candidates have different contracts:

| Ratchet | Decision | Reason and exit condition |
|---|---|---|
| `module_legacy_budget` | **Change in a follow-up; keep now.** | Its legacy-tree register is zero, but it also protects one 246-LOC legacy file and 55 composition packages / 12,385 LOC. The zero tree check prevents reintroduction. Keep that ban and the exact legacy-file debt; replace compose LOC ceilings only after a typed construction-only gate covers owner compositions, roots and relocated code. Constructors have a nonzero floor, not a target of zero LOC. |
| `module_compose_alias` | **Keep.** | Its zero register prevents foreign aliases from hiding dependencies (ADR 0068 point 12). The import checker does not establish which vocabulary an alias leaks to consumers. Completion of #4085 is not a reason to allow regrowth. |
| `repo_method_caller` | **Change in a follow-up; keep now.** | A zero textual-caller register is useful but checks only Timetable persistence interfaces. Same-name collisions, comments, strings and testhook callers can make dead methods look live. Replace with typed production liveness across owners, including method values/interface dispatch, before retiring this migration-era name heuristic. Green is not proof of liveness. |
| `helper_consolidation` | **Keep.** | Unique-violation identity, checked URL IDs and one logger fallback remain useful outside the migration. The empty literal/parse registers prevent drift; syntax-based false positives should be repaired, not allowlisted. |
| `calendar_fixture` | **Keep.** | Clock-dependent fixture failures recur at date/week boundaries regardless of architecture migration. One exact, reasoned exception remains. This is test determinism, not migration debt. |

Disposition of the other `*_ratchet_test.go` files:

- **Keep:** `gdpr_log_pii`, `http_error_identity`, `durable_side_effects`,
  `transaction_boundaries`, `query_budget`, `toolchain_pins`, `handler_layer`,
  `module_facade_width`. They protect privacy, error identity, durable work,
  transaction starts, observable query cost, reproducibility and narrow seams.
- **Change scope, retain thresholds:** `module_file_size`,
  `module_function_length`, `module_complexity`, and their shared
  `module_source` scanner. ADR 0082 correctly added roots, but `internal/`,
  `database/`, `tenant/`, `realtime/`, middleware and other production packages
  still fall outside. Define the production source set from the policy/build,
  not an expanding directory list. Audit migrations, tools and generated
  sources explicitly; do not silently include all historical migrations or
  exempt every generated file. Generated error declarations are a separate
  measured source category, not permission to hide handwritten code.
- **Retire only after an equivalence proof:** the overlapping
  `handler_complexity` scan once the shared scanner proves default-production
  coverage and identical scoring. It currently excludes hooks while the module
  scanner includes them. No deletion in this PR.
- **Change:** `module_passthrough`, as point 3 below.

The 19-file glob is not the gate inventory: for example,
`TestModuleContractInventoryRatchet` is in `module_contract_inventory_test.go`.
Keep that 28-module contract-test inventory and the hermetic fixture gates too.
A successor must have positive and negative fixtures, a base comparison and
relocation tests before a predecessor disappears. ADR 0070's new-ADR requirement
for budget changes or measurement retirement remains in force.

## 2. Supplement structural checks with typed command effects

**Keep the Domain → Projection prohibition already implemented by #4540.**
`validateDomainProjectionRule` rejects every matching selector pair from epoch
91, including test scopes, public and contract targets. Its negative fixtures
are `TestPolicyRejectsDomainProjectionRules`; historical-base and reader-port
cases are in `TestDomainProjectionDirectionKeepsEarlierBasesAndValidReaders`.
Do not add a duplicate rule or describe this gap as still open.

**Add a typed effect inventory in a separate implementation.** Key entries by
resolved package/type/method, consumer port, concrete root binding and target
owner, not a method-name prefix or import alone. Follow embedded interfaces,
method values, adapters and function-valued bindings; list unresolved/dynamic
bindings as failures requiring an exact reviewed explanation, never as queries.
Cover every production package and root. Record separately:

1. Read-only queries versus mutations, lock acquisition, unit/scope changes
   and durable effects. A locking read is not a harmless query.
2. Caller/target tenant or admin scope and transaction participation, including
   savepoints, error propagation and best-effort boundaries.
3. Authorization responsibility, combined decisions, re-reads and lock order,
   including callbacks and target implementations supplied by workflows.

**Reject the blanket alternative “all foreign commands only from workflows”.**
ADR 0067's #4538 addendum permits guarded transaction-participating leaf
commands without a separate protocol. Coordinators require workflows; the
finite E1–E5 exceptions retain only their recorded protocols. The inventory
must distinguish these cases and detect additions/changed effects against the
base. It must not auto-approve the 40 manually inventoried edges as safe leaves.
Adding query/command annotations alone cannot prove ambient transaction,
authorization or lock correctness. Use annotations plus typed binding coverage
for effect completeness; behavior tests and review still establish semantics.

### Findings the successor must represent

- **Enrollment:** the issue's four-owner decision is still coordinated in the
  domain, but is now explicitly **E1**, not an undocumented green loophole.
  Preserve request/child serialization, class → recurrence gates,
  account-before-roster order and the decision outbox until its complete
  workflow cut. Count imports neither as commands nor as proof of guards.
- **Care source-booking round trip:** ADR 0075's latest amendment (#4531)
  already cut lock and end back to Care Plan's `OfferingBookings`. Snapshot
  and restore still use Enrollment-shaped data. Do not reopen the removed
  round trip or move lock ownership merely to lower a shape count.
- **Care resumption:** `careplan.CareResumption`, embedded by `CareExitCommands`,
  is implemented by `workflows/careexit`, not by the domain. Root binding and
  embedding avoid an import cycle but do not remove the runtime dependency.
  `roots/owners/care_lifecycle_owners.go:38` already binds `careexit.NewRoster`
  into Care Plan. Record both as workflow-backed domain ports, not leaf calls.
  Two follow-up options remain: split workflow entry commands from the domain's
  published lifecycle interface and route callers to the workflow, or retain
  that compatibility surface under a separately justified, bounded runtime
  exception. Prefer the explicit workflow entry seam; neither option is
  implemented or granted as a new general exception here.
- **The reported unused port is live through promotion.**
  `CareStudentDirectory.ResumeCare` in
  `modules/careplan/internal/ports/care_lifecycle.go:38` has no direct Care Plan
  application caller, but `roots/owners/care_resumption.go` embeds that
  interface in `careResumptionStudents`, which implements the workflow's
  `ResumptionStudents`. `workflows/careexit/resumption.go:resumeCare` calls
  `s.students.ResumeCare`; the promoted method reaches
  `roots/owners/care_students.go:116` and School Membership. Do not delete it
  as dead code. A narrower root-owned workflow input could remove it from
  Care Plan's port later, while preserving the actual command binding. This
  is a concrete requirement for typed liveness, not a new unused-port finding.
- **Delegated authorization:** Timetable's
  `operations_attendance.go:154` passes `BypassResourceChecks: true` to Presence.
  `presence/student_move.go:73` then omits the resource authorization branch.
  Timetable's comment says it authorized the target already. The #4538
  inventory now links this finding; “each owner retains its authorization
  guards” is not established for this edge. Record the bypass, upstream check and
  required equivalence tests explicitly. Either prove Timetable's check
  covers the required action/cohort, or replace the bypass with a narrow
  guarded command. This decision does not choose a business authorization
  rule or claim a demonstrated unauthorized write.

### Lock and behavior locks remain authoritative

Keep class(shared/exclusive) → recurrence → transition where applicable;
child rows precede request/pickup rows in the corrected offering protocols.
Do not invent a single global row order from a partial inventory. E1's
request serialization and E2's corrected order are distinct protocols.
#4636/#4638 contract tests demonstrate real prior `40P01` failures. Merged
#4725 (`e5ff1c467f`) fixes the Care Plan family-request inversions with
**class(shared) → child → family request**, as recorded in
[ADR 0067’s E2/E4 family-request correction](0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md#e2e4-family-request-lock-order-correction-4640).
Merged #4642 (`2ef86106e4`) is also included in the PR head. This ADR does not
change either fix or ADR 0067; the shared inventory links the bypass finding.

`care_resumption_workflow_test.go` pins ambient/nonambient rollback and tenant
isolation; `offering_lock_order_test.go` pins concurrent offering paths.
Enrollment's `TestDecisionService_ApprovalRollsBackEveryOwnerAfterMaterialization`
and `TestExistingStudentApprovalGrantsAccountBeforeClassRosterResync` pin E1.
The transaction-site and durable-notice ratchets complement them. No static
Command/Query label substitutes for these tests.

## 3. Change pass-through accounting, keep the current gate until replacement

Choose a **method-level, shape-aware register**, not owner-sum permission and
not warning-only enforcement. The current 1,408 hits in 30 owners include
42 `testhooks` hits. A same-owner swap is invisible; moving an unlisted binding
to a root can also hide it. Scope selectors are not compared by the numeric
budget checker. ADR 0064's requirement to follow moved code is therefore a
review rule with incomplete automation.

The replacement must:

- Scan policy-classified production packages and every root; compare the
  source set/selectors against the base, including relocation attribution.
  Preserve existing consumer attribution without double counting root hits.
- Register exact method identity, floor shape, reason and a normalized body
  fingerprint or equivalent structural evidence. New/changed sites require
  review; replacing a translation with a pure forward must fail even if the
  owner's count stays constant. A moved method retains its evidence.
- Separate default-production sites from `testhooks` and untagged fixture
  helpers. Report test-only shape cost without the production forwarding cap;
  import, tenant, composition and source-complexity gates still cover hooks.
  Do not merely subtract 42 from today's register before this split is tested.
- Preserve ADR 0075's transaction/observation, genuine consumer translation,
  Timetable §5, scanner-limit and owner-edge shapes and its exact exceptions.
  Nil guards and method renames alone are not translation. Pure forwards
  remain outside the floor; existing newly exposed root forwards require
  removal/deepening or an explicit finite debt decision, not a new blanket floor.

The unchanged root script finds 1,040 hits, but defaults unmatched bodies to
translation. At least seven additional nil-guard-only methods reduce its
claimed translation total from 830 to **at most 823** (root-composition subset:
823 to **at most 816**). ADR 0082's correction names them. This is not a full
semantic reclassification. The Device Fleet example is mostly valid: eight
`Get*` reads and `DetectNewDevices` call converters, but `mapping.go:10-11`
merely copies the same `devicefleet.Device`; `UpdateDeviceStatus` casts
`DeviceStatus` to itself. These ten sites do not translate domain vocabulary.
The eleventh, `ListDevices`, also validates/maps a filter map into a typed
`DeviceFilter`, so it is not an unchanged forward. Syntactic conversion alone
must not earn a translation classification. Preserve copy allocation, nil/error,
filter and collection behavior when evaluating deletion; this is a follow-up,
not permission to remove adapters by name.

Move the executable `docs/architecture/4542-root-passthroughs.py` into tested
measurement tooling sharing the scanner, or retire it when that tooling can
reproduce the snapshot. Its copied predicate, hard-coded 14-line threshold and
reviewed classification list can drift independently. Do not grow that script
into a second architecture checker. Keep it unchanged as reproducibility
input here, with the explicit classification limits above.

## 4. Keep explicit cross-owner permission; change documentation layout later

Reject the broad default “Domain may read every other Domain's public API”.
`public` contains commands too; two-way kind-wide grants also violate the DAG.
Even Query/Command labels would not authorize a new dependency direction,
projection grant, tenant scope or new runtime workflow dependency.

The 927 rules are not 927 explicit owner pairs: 690 select both owners;
91 target external classes, 92 use `same_owner`, and 116 use an owner-kind
selector (categories overlap). Standard rules already exist for shared kernel,
root surfaces and own-owner reach. Preserve exact package/table grants and
scope separation. A future compact policy notation is acceptable only if its
expanded permission relation is identical, has no new owner edge, and retains
strictness, overlap, stale-rule and negative-private-role tests. Measure the
actual saved edits first; do not trade a smaller JSON file for wider rights.

**Historical Cutover/Checkpoint JSONs: already removed, no further archive.**
The top level of `backend/architecture/` contains `policy.json` and
`composition.json` (the manifest read by `TestCompositionInventory`). Six
historical caller inventories also remain under `callers/` (#2734–#2749).
**Keep these six for now:** the README and `backend/architecture/WORKPLAN.md`
still cite them, and caller-inventory generators retain their output paths.
`scripts/backend-common-http-callers/main.go` scans Go source and writes
`callers/common-http-2738.json`; it does not read the JSONs as runtime or test
inputs. Archive them only with their historical references and generator
output paths in the separate documentation cut, not by deleting evidence here.
ADR 0026's amendment and the README point to immutable
`46506b00c695a6f102865cae2dc9f24010371b7c`; runtime measurements remain in
`docs/runtime-checkpoints/`. Do not restore deleted artifacts to move them.

**README: change in a separate documentation cut.** Its 5,060 lines mix live
contracts with historical evidence. Keep a short live entry point, commands,
current schema, guard semantics and principle traceability; move completed
cutover narratives to a clearly dated history document. Preserve old anchors
with links, historical paths and immutable measurements; do not move schema or
reviewed-procedure contracts merely because they mention an old issue.
This is not a safe blind section-range move. The #4082 historical filename
is restored separately in this PR; history is not retroactively renamed.

## 5. Keep deterministic gates; measure maintenance and latency separately

The local pinned-toolchain check took **55.49 s** wall time. Three recent
successful CI deterministic steps took **122, 168 and 197 s**; complete jobs
including checkout/cache/projection work took **166, 228 and 254 s**.
These are different machines and inputs, not a benchmark regression or an SLA.
Record checker-step and full-job duration separately before performance work;
keep full-source checks, caches and existing reuse rather than a diff-only
semantic scan that misses unchanged affected callers.

Three concrete feature/fix PRs (#4617, #4631, #4619) changed **zero** package,
rule, data-object or projection entries relative to their merge first parents.
That disproves a mandatory per-feature policy edit, not architecture overhead:
#4617 still crossed several read seams (ADR 0080). File/entry counts do not
measure human minutes. For future boundary-changing PRs record rules/packages/
grants added, removed and replaced, touched owners, and reported author time
separately. No unmeasured duration or extrapolated average justifies loosening.

## Delivery and follow-up cuts

No gate is removed or weakened here. Small repairs are separate commits:
AST-only database-operation detection with a failing-before regression test;
ADR-exception wording; historical filename; ADR 0082 measurement correction.
The setup/mock exemptions elsewhere in the hermetic scanner remain unchanged.

Suggested implementation slices, **not newly created issues**. Estimates are
engineering days for implementation/tests/review preparation, not commitments:

| Slice | Scope and completion evidence | Estimate |
|---|---|---|
| Typed command effects | Resolve roots and ports, effect annotations, unresolved-binding diagnostics, E1–E5 limits, positive/negative fixtures and reviewed inventory diff. No coordinator migration in the same PR. | 5–8 days |
| Pass-through replacement | Shared tested scanner, method/shape evidence, production/hook split, root/relocation coverage, selector-base checks, explicit treatment of exposed pure forwards. | 4–6 days |
| Complete source gates | Audit every production policy package, migrations/tools/generated categories, exact measured baselines; only then deduplicate handler complexity and replace compose LOC with construction checks. | 3–5 days |
| Typed liveness | Replace the Timetable-only textual caller heuristic across owners without losing interface/method-value uses; dead production API deletion is a separate cut. | 2–4 days |
| History and semantic seams | README extraction with preserved anchors: 1–2 days. Care workflow entry/root-owned port: 1–3 days after #4725. Authorization bypass proof or guarded command: 1–2 days, pending the concrete permission contract. Policy compaction: only if measured edit savings warrant a 2–3-day equivalence prototype. | Separate PRs |

ADRs 0057, 0066–0071, 0075, 0080–0082 remain the invariant basis. This ADR
narrows the interpretation of green gates; it grants no new exception to those
invariants and authorizes no ownership merger, alias or policy widening.


## Implementation addendum 2026-10-23 (#4739)

Decision of 2026-10-23 for the first implementation slice of
[#4739](https://github.com/moto-nrw/project-phoenix/issues/4739), implementing
part of [ADR 0083 §2](0083-ratchets-protect-invariants-and-expose-command-effects.md#2-supplement-structural-checks-with-typed-command-effects).
No coordinator, owner, permission, budget or runtime behavior moves.

### Cut and alternatives

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
load and `go/types` objects; add no dependency, second package load or independent production source set.

### Resolution and exact register

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
classification `unresolved`, cites ADR 0083’s #4739 addendum, and explains the boundary. These
entries remain visibly unresolved in JSON and the PR-check summary, not approved
leaves or queries.
New unresolved entries require a new approval ADR in PR mode.

### Effects and PR-base contract

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
a base without the register is permitted only while this exact #4739 addendum
is new against that immutable base. Only bootstrap may cite its amended ADR
0083 as `approval_adr`; once a base holds the register, future exception growth
still needs a newly added ADR number. Reusing an old addendum or changing an
old ADR after the register exists grants nothing.

### Limits and follow-up

- The scan stops at foreign owner contracts. Enrollment → People → Membership
  and Enrollment → Care Plan → Timetable are not flattened into invented direct
  edges. Record the actual first hop and reconcile the hand inventory separately.
- Other source owners, platform targets, direct foreign calls not supplied
  through an inventoried interface, generic/variadic and multi-result binding
  flow, reflection and runtime container/function dispatch need later slices.
  Function fields and unknown callable variables/parameters reached inside a
  scanned adapter are diagnosed, not resolved. Local static method/function
  values retain their typed selector/body evidence without a dynamic diagnostic.
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

## Domain-source expansion 2026-10-23 (#4739)

This second census extends the implementation addendum above to **every owner
whose policy kind is `domain`**, not a hardcoded module list. No production Go
file, coordinator, transaction, permission, owner or policy rule changes.
The [reconciliation](../architecture/4739-command-effect-reconciliation.md)
records all 40 hand rows, counts, known debt and the remaining inference cuts.

### Source identity and provider evidence

A register identity now includes `source_owner`. A native foreign contract can
be consumed by several domains; they must not overwrite one another's review.
Old entries without this field mean Enrollment for immutable-base comparison.
All current generated entries carry the field. A project with no discovered
foreign binding needs no empty register; any new discovered binding requires it,
and an existing register still rejects stale entries. Source discovery includes named
interfaces, aliases and foreign contract fields in every policy-classified
production package, including compiled fixture support and the additive
`testhooks` build. Zero discovered edges for an owner is a result, not an
exclusion from scanning. Type declarations/full method sets are still may-bind
surfaces, not proof that a domain application invokes every exposed method.

Keep the foreign contract as a first-hop terminal, but also follow its actual
typed providers when a different owner supplies it. Do not walk a foreign
owner's command body or guess structurally compatible implementations. This
exposes Care Plan's Communication `ShareVisibilityResolver` bound to Parent
Portal's `ShareRecipients` instead of hiding the workflow behind a read label.
Forwarded tuple returns bind corresponding result slots, including
`return factory()` returning `(contract, error)`. General tuple assignments,
variadic/container/generic instance flow remain separate follow-up work.
Anonymous-only source ports, foreign interfaces used solely as parameters and
embedded foreign dependency-object inputs also need that flow cut; anonymous
bindings reached behind the named source surface remain explicit diagnostics.

Builtin/external interfaces such as `error` are outside this policy-owned port
surface. Unioning every returned error's `Error` method invents unrelated domain
edges. Local-owner interface forwarding with no external provider is not itself
a cross-owner edge; do not invent an unresolved foreign command from that alone.
Dynamic root bindings, instantiated generic methods without indexed bodies,
function fields, anonymous interfaces and root forwarding cycles remain exact,
visible `unresolved` entries. The census does not claim to resolve them.

### Effect review, debt and canonical form

`check` requires the exact generator form: field order, omitted optional fields,
indentation, final newline and identity-sorted entries. It never rewrites the
file. Run `scripts/backend-architecture.sh effects`, then review new `unknown`
entries; generation does not infer labels from names. Formatting/reordering,
missing/stale entries, duplicate identities and conflicting effects for the same
terminal method fail. Pointer/value receiver spelling is one target method for
this consistency check. Different source protocols may still have different
review classifications for that method.

`query` includes read-only value, error, DTO-metadata and construction helpers.
It excludes locking reads, audited reads that persist an access record,
administrative/tenant scope transitions and durable delivery intent.
`notice` records the already exposed non-durable guardian SSE invalidation,
not a message, pill or outbox write. Its existing after-commit timing remains
required; this does not claim complete platform/after-commit effect analysis.

`debt` is an explicit **unapproved coordination finding**, not a sixth E
exception or a leaf. It cites this ADR (`0083`), carries exact evidence and a
proposed follow-up cut, and needs fresh approval evidence just like E1–E5 or
`unresolved` growth. Only protocols already named by ADR 0067 receive E1–E5.
In particular the Calendar portal's appointment lifecycle/reminders and the
Care family-request decision findings are not newly exempted by this
census. No follow-up issue is created here.

`workflow` describes a supplied workflow implementation, not permission for a
domain-application callback. Reasons distinguish outer HTTP workflow entries,
compiled support/contract-union surfaces, the finite Care Exit Roster/Bookings
E2 callbacks documented by ADR 0071 / PR #4753, and **workflow-backed, Schuld
#4825** for Share-Visibility and LiveStaffEmployment. Other confirmed callbacks
are separately recorded as direction debt; none is a normal guarded leaf.
ADR 0071 is not amended by this PR.

### One-time expansion review against the immutable base

The first census already occupies ADR 0083, so the original bootstrap cannot
approve this expansion. The new heading above permits `approval_adr: "0083"`
only while this exact domain-source addendum is absent from the immutable base:

- New exact entries of previously unscanned source owners may cite it.
- A newly resolved provider of an already reviewed port may cite it only with
  the same source owner, port, supplied binding, target owner, effect,
  classification and ADR as the reviewed boundary. This covers the extra
  Enrollment providers exposed by interface forwarding, not new Enrollment ports.
- Newly exposed `unresolved` diagnostics of an already reviewed source/port/binding
  may also cite this census, without downgrading or relabelling its existing
  contract terminal. New source ports do not qualify.
- Existing entry changes, including fingerprints and labels, receive no such
  approval. Once the addendum is in the base, it approves no further growth.

The counterprobe without this addendum must reject the census. The other
new-ADR rules and old bootstrap tests remain intact; this is a bounded census
review, not permission to append to an old decision for future exceptions.

### Fingerprint decision

**Retain traversed root-helper bodies**, not just the outer binding. A mapper
can change tenant IDs, arguments, error handling or execution order without
changing its terminal target set. A binding-only digest would miss such real
behavior changes. Proving that a particular mapper edit is harmless needs a
separate semantic summary; the current checker has none. The review cost of
re-approving an exception's helper change is therefore retained and explicit.
Formatting/comments/positions remain excluded. Fixtures cover real changes to
both the binding and a called helper while the target set stays the same.
The original 128 Enrollment fingerprints remain unchanged in this census.

The outstanding recognition work is the explicitly bounded binding-flow and
platform/protocol cuts, not additional source-domain coverage. This census does
not prove label truth, production liveness, guard equivalence, transaction
participation, after-commit durability or deadlock freedom.

## Gate-gap closure 2026-10-23 (#4836)

This implements the three gate repairs from
[#4836](https://github.com/moto-nrw/project-phoenix/issues/4836), not the complete
platform/protocol follow-up. No runtime owner, command, transaction, permission,
policy rule or shape budget changes. ADR 0071 remains unchanged; the finite
Roster/Bookings description in PR #4753 is context, not a merged decision.
[ADR 0092](0092-command-effect-gap-reconciliation.md) supplies the fresh-numbered
approval for the measured register reconciliation. Appending this heading to
ADR 0083 itself grants no approval.

### Assignment evidence and limits

Function fields and callable variables keep their exact `unresolved` identities,
but their fingerprint now includes their typed assignments, constructor
arguments, aliases and closure bodies. Referenced root helper bodies and typed
function signatures also contribute. The union is keyed by `go/types` objects,
not field names or structurally compatible signatures. Callable containers and
tuple-returning factory expressions retain assignment evidence for reached
diagnostics; this does not claim complete runtime function dispatch. This is
still conservative may-bind accounting, not instance-level dispatch resolution.

A closure or helper can change arguments, scope, order, errors or foreign calls
without changing the wrapper method or target set. Therefore semantic changes
to a still-unresolved binding need fresh ADR evidence, even for a purported
no-new-foreign-effect edit. The helper fixture exercises that trade-off with an
unchanged target set. A semantic effect-equivalence solver is out of scope.
Normalized comments, formatting/positions and unconnected closures do not
change evidence or require an ADR. No existing assertion is weakened.

### Container and tuple flow

Actual slice/array/map literals, including nested containers and map keys,
contribute element bindings. Container assignments/returns, variadic arguments
and expanded slices, append arguments, multi-result assignments/declarations, named-function calls
and sole tuple arguments bind the corresponding typed slots. Recursive named
containers terminate without inventing edges. No implementation is guessed
from assignability. These cases resolve the foreign terminal rather than
silently dropping it or calling it a query. Generation still leaves new
identities `unknown` for human classification.

### Workflow labels are not silent exceptions

New or changed `workflow` bindings now need a new `approval_adr`, just like
E1–E5, unresolved bindings and debt. Only the seven already recorded Care Exit
Roster/Bookings E2 command/lock identities retain that label. All other prior
workflow-labelled surfaces become `debt` with #4825, including HTTP entry and
compiled support may-bind surfaces. That conservative classification does not
assert those unused methods are live application callbacks. Read-only targets
remain `query`; their evidence still records direction debt where applicable.

### Verification and measured reconciliation

The three new regressions fail on development: changed closure reports
`0 changed`, container-only binding reports `0 entries`, and new `workflow`
passes without approval. After repair, the same fixture generation, base
comparison and CLI `check` reject all three. Additional fixtures cover assignment
and constructor aliases, helper changes, containers/variadics/tuples, comments,
unrelated wiring and recursive containers. The real repository counterprobe
removes ADR 0092 and this addendum, checks failure, then restores exact bytes.

Against `1f1d42ef1b`: **5,341 → 5,344 entries; 3 added, 0 removed**.
Container/tuple flow exposes no new unresolved identity. Named-function argument
flow exposes three existing File Storage → Care Plan cleanup-bookkeeping
providers (`StudentDocumentFileStore` → `StudentDocuments`: ActivateQueuedCleanup,
MarkFileDeleted, MarkQueuedCleanupCompleteByFilename). Each is `mutation/leaf`,
consistent with the same already inventoried owner commands; its exact reason
records the forwarding/context/error boundary. No new unresolved diagnostic
needs classification.
**48 fingerprints change**: all 40 function-field entries and 8 callable-variable
entries gain assignment evidence. **19 workflow → debt**, leaving **7 workflow**
and **73 → 92 debt** entries. The union of those changes is **70 reviewed entries**
carrying `approval_adr: "0092"`. Effects remain 2,779 query, 1,614 mutation,
298 lock, 7 scope, 1 durable, 5 notice and 640 unresolved. No policy or
pass-through entry changes.

Serial warm `check` before implementation: **51.33 s wall, 87.37 s user,
21.97 s system**, pinned Go 1.27.3, `-p=4`, `GOMAXPROCS=4`. Each timed sample
follows an untimed warmup; no own test/lint runs concurrently with the sample.
Other worktrees share this machine, so the comparison is observational, not a
causal benchmark or SLA. The no-new-ADR counterprobe exits 1 and rejects 67 existing-entry changes
(48 fingerprint changes and 19 relabels); restoring the exact ADR bytes
restores approval. The 3 new leaf provider entries require exact review but
no exception-growth approval. Final warm explicit-base `check`: **29.97 s wall, 87.62 s user, 18.48 s
system**, 5,344 entries, 3 added and 48 changed, 0 legacy violations,
composition surface 0 → 0 and 3,522 unchanged method pass-through sites. The
untimed default-base warmup and explicit merge-base check both pass. The wall
time difference is not attributed to this patch: shared-machine load and
cache state changed; CPU time is similar.

Generic instance-flow completeness, anonymous-only source discovery and direct
foreign calls outside supplied interfaces remain proposed binding-flow work.
Full platform effects, transaction/savepoint, authorization, lock-order and
after-commit protocol proofs remain a separate 2–4 day follow-up. Combining
those cuts here would require a broad new register review rather than a bounded
gate repair. No follow-up issue is created.

### Tuple-valued callable arguments (quorum follow-up)

The first quorum review of PR #4837 found that `makeBinding(supply())` lost
callback evidence when `supply` returned `(int, func())`: the sole AST argument
was assigned only to parameter zero. Two new fixtures reproduced the unchanged
fingerprint and missing fresh-ADR refusal, for ordinary and variadic callable
parameters. Callable collection now expands the typed tuple's parameter slots,
retaining the supplying expression as evidence for each callable slot. Both
fixtures reject the changed returned closure through generation, base comparison
and CLI `check`. Comments and unrelated wiring remain unaffected.
