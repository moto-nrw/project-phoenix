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
transaction, production behavior or open #4642/#4725 file changes here.

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
  inventory omits this fact, so “each owner retains its authorization guards”
  is not established for this edge. Record the bypass, upstream check and
  required equivalence tests explicitly. Either prove Timetable's check
  covers the required action/cohort, or replace the bypass with a narrow
  guarded command. This decision does not choose a business authorization
  rule or claim a demonstrated unauthorized write.

### Lock and behavior locks remain authoritative

Keep class(shared/exclusive) → recurrence → transition where applicable;
child rows precede request/pickup rows in the corrected offering protocols.
Do not invent a single global row order from a partial inventory. E1's
request serialization and E2's corrected order are distinct protocols.
#4636/#4638 contract tests demonstrate real prior `40P01` failures; the open
#4725 addresses the remaining Care Plan request inversions. This ADR changes
neither their files nor ADR 0067 nor the shared inventory.

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
Only `policy.json` and `composition_inventory.json` remain as architecture JSON
files. ADR 0026's amendment and the README point to immutable
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
