# Backend architecture policy

`policy.json` is the source for package and runtime write ownership, package
roles, tenant-safe read projections, legacy-composition symbols, dependency
classes, import scopes, and target import rules. Generated lint configs and
diagrams must derive from this file and must not be committed.

## Commands

Run commands from the repository root:

```bash
scripts/backend-architecture.sh check
scripts/backend-architecture.sh audit-issues \
  --api-url https://api.github.com
scripts/backend-architecture.sh explain \
  --scope production \
  --source github.com/moto-nrw/project-phoenix/services/mealplan \
  --target github.com/moto-nrw/project-phoenix/models/mealplan
scripts/backend-architecture.sh diagram
scripts/backend-architecture.sh dependencies \
  --focus module:meal-plan
scripts/backend-architecture.sh dependencies \
  --focus package:services/mealplan
scripts/backend-architecture.sh validate-ticket \
  --ticket backend/architecture/checkpoint-ticket-template.json
```

`check` loads packages with `GOOS=linux`, `GOARCH=amd64`, and `CGO_ENABLED=0`.
It checks production imports, same-package test imports, and external-package
test imports as separate scopes. Every package explicitly declares all three
roles. Test roles describe the seam (`module-internal-test`,
`module-behavior-test`, `workflow-decision-test`,
`workflow-integration-test`, `adapter-test`, or `e2e-test`), so production
permissions cannot leak into test scopes. A finding uses this stable key:

```text
scope|rule|source|target
```

The key identifies the violation. Source locations are evidence and do not
form part of that identity. Each location contains a project-relative Go file,
line, and the affected import, function, method, or declaration. `check` prints
all locations below the key. It sorts and deduplicates them, so the same source
tree produces the same output on every host.

Locations may change when code moves without changing the violation key. The
exact JSONL baseline therefore keeps only `scope`, `rule`, `source`, `target`,
and `issue`; it never stores locations. Generated JSON projections attach a
`locations` array to each violation instead. This lets migration tooling group
current evidence by the stable key without turning line changes into new debt.

Production analysis also uses Go syntax and type information to enforce these
semantic boundaries:

- a table has one runtime write owner;
- foreign reads require an owner query or an exact `read_projections` grant
  whose `tenant_safe` flag is true;
- foreign reads and writes are detected in BUN table/model calls, joins, SQL
  fragments, and raw SQL; writes also include `MERGE` and `TRUNCATE`;
- unresolved or unclassified table expressions remain findings;
- direct BUN/SQL access is restricted to Postgres, migration, test-support,
  and named projection adapters;
- public/contract packages cannot expose ORM, repository, internal-model,
  `map[string]any`, generic CRUD, or BUN-tagged types;
- references to the exact symbols in `legacy_composition` are findings outside
  the defining package.

`data_objects` contains target-owned runtime data, not every historical table
ever created by a migration. Access to an obsolete table with no target owner
therefore remains a `tables.unclassified` finding until that access and table
are removed; do not invent an owner merely to make the check quiet.

The current backend still violates the target policy, so the normal `check`
loads the committed exact baseline from `architecture/legacy.jsonl`. It passes
only when the current violation set is exactly equal to that baseline. Pull
request CI additionally reads the policy and baseline from the event's full
base-commit SHA and permits only removal of existing tuples. The required
status is `Backend architecture ratchet`.

## Generated projections

`diagram` evaluates the real graph once, loads the committed baseline by
default, and writes these files to a newly created system temporary directory:

- `target.svg` contains only owners and production edges declared by the
  target policy, including declared target modules that do not have packages
  yet.
- `migration.svg` condenses the current production graph by owner. Allowed
  edges are gray, violations present in `--baseline` are orange-red, and new
  violations are dashed red.
- `architecture.json` uses projection schema version 2 and contains both
  graphs plus exact violation keys, their
  source/target owners, and sorted source locations, so follow-up tooling can
  group ratchet work by owner and capability.
- `go-arch-lint.yml` projects the target policy into go-arch-lint's coarser
  owner-level model. It is an additional guard; the evaluator remains
  authoritative for roles, scopes, semantic checks, and exact edges.

`dependencies --focus ...` also loads the committed baseline and writes
`dependencies.svg`, `dependencies.json`, and `dependencies.goda`. Prefix a
focus with `module:` or `package:` when the same text names both. The generated
Goda query pins `GOOS=linux`, `GOARCH=amd64`, and `CGO_ENABLED=0` from the
policy. Unknown, ambiguous, and target-only modules without current packages
fail with a concrete error.

All files are generated artifacts. The default location and every accepted
`--output` location are inside the system temp tree; do not commit them.

## Exact legacy ratchet

The legacy baseline is canonical JSONL sorted by `scope|rule|source|target`.
Each record has exactly these fields in this order:

```json
{"scope":"production","rule":"imports.forbidden","source":"example/source","target":"example/target","issue":"https://github.com/moto-nrw/project-phoenix/issues/1234"}
```

The first four fields identify one exact violation. `issue` identifies its one
open migration ticket. Locations are non-identifying evidence and stay out of
this file. Wildcards, package-family patterns, blank lines,
duplicates, unsorted records, non-canonical JSON, and issue reassignment are
errors. The normal command has no init, approve, update, or rebaseline mode.

Local mode requires exact equality between the current violations and the
committed baseline. PR mode adds `--base-ref` with the event's full 40-character
base commit SHA and reads the baseline and policy directly from that Git object.
Candidate entries must be a subset of the base entries, unchanged entries must
keep their issue, and candidate policy, package classification, and ownership
changes may not weaken the checks enforced by the base policy.

PR mode allows a classification when the candidate adds the first Go file in
that exact package. Rules added with it must be anchored to an owner and role
used only by candidate-created packages; owner-kind rules remain forbidden.
Existing unclassified packages and modified packages do not qualify. A legacy
composition guard may be removed only after the guarded package declaration is
deleted.

One additive ownership case is allowed because otherwise the ratchet would
freeze the database schema: a candidate may add a `data_objects` entry when a
new Go file under `database/migrations/` in the same candidate creates that
exact schema-qualified table through a literal `NewRaw(... CREATE TABLE ...)`
statement. The table must not be mentioned by any migration at the base SHA,
the write owner must already exist, and changing or newly assigning ownership
for an existing table remains a policy loosening. Modified historical
migrations never qualify for this exception.

`audit-issues` performs the network-dependent GitHub liveness check separately.
The wrapper supplies the committed baseline; callers must provide `--api-url`,
and `GITHUB_TOKEN` is optional for authenticated requests. A GitHub or network
error fails this audit and cannot change the deterministic `check` result or
appear as a green audit.

## Migration evidence

Schema version 2 separates ordinary waves from runtime checkpoints. Convert
version 1 tickets explicitly. Both kinds retain prerequisites, owner/capability,
packages, tables, exact ratchet keys, atomic cutover, tests, rollback/cleanup,
and measurable exit criteria. Unknown fields and blank required evidence fail.

### Ordinary waves

Copy `migration-ticket-template.json` and replace all example/guidance values.
Set `ticket_kind` to `migration`. `checkpoint_reference` must be the canonical
issue URL of the latest accepted checkpoint in `runtime-checkpoints.json`.
Missing, malformed, future, unaccepted, and superseded references fail.

Record only flow-specific evidence needed for the cutover: raw source,
workload/environment/observation window, thresholds, query counts, stable
errors, failure-path and transaction rollback results, and smoke results.
Keep deployment rollback and cleanup in `rollback_and_cleanup`. Non-applicable
evidence needs a concrete reason. Additional metrics are allowed when the flow
needs them; a reference does not waive failure, rollback, query-budget, or smoke
checks. Ordinary waves do not repeat the full benchmark suite independently.

### Checkpoint measurements

Copy `checkpoint-ticket-template.json` and set `ticket_kind` to `checkpoint`.
Only these canonical checkpoint issue URLs are valid:

1. #3019 establishes the one-third baseline across completed migrations.
2. #3020 compares with #3019 after its blocking core migrations and before
   session-end, device-scan, and enrollment-acceptance workflow cutovers.
3. #3021 compares with both predecessors after contract/storage cutovers and
   before #2751 removes the remaining legacy composition.

The `checkpoint` object requires an exact commit, reproducible PostgreSQL 17
and production composition environment, toolchain, versioned workload, data
volume, concurrency, and warm-up. Cover successful and failing HTTP/module
flows and applicable Worker paths. `runs` contains exactly three measured runs
after warm-up; `median` and `worst` report every applicable metric. Each report
records source, workload, thresholds, query count, p50/p95, stable errors,
pool waits, measured lock waits, deadlocks/serialization retries, Worker
duration/retries/backlog, and affected rows. Non-applicable metrics need reasons.
Full statement duration is not lock-wait evidence.

`comparison` explains every metric against the required predecessors (or
establishes #3019's baseline). Keep workload/environment equivalent;
`workload_bridge` confirms equivalence or links old/new workloads measured on
the same commit. `regression_issues` links focused optimization issues for
material regressions; unexplained regressions block acceptance. `decision`
records completed blockers and acceptance/rejection; #3021 permits or blocks
#2751. Raw output and interpretation stay in the checkpoint issue or review
evidence, not the repository. Template values are examples, not measurements.

### Recording acceptance

`runtime-checkpoints.json` is a reviewed acceptance registry, not architecture
policy. It starts empty because no checkpoint has been accepted. Ordinary
waves cannot pass until #3019 is accepted; checkpoint measurements can be
validated before acceptance. After explicit acceptance in the checkpoint issue,
append an entry in a reviewed change, in order without gaps:

```json
{"issue": "https://github.com/moto-nrw/project-phoenix/issues/3019", "acceptance": "https://github.com/moto-nrw/project-phoenix/issues/3019#issuecomment-123"}
```

Replace the example comment ID with the actual acceptance comment. Validation
checks issue order and comment-link shape, not comment truth or measurement
accuracy. Review must verify acceptance, closed blockers, comparable runs,
and explanations. A ticket-local `accepted` flag cannot grant acceptance.
`--checkpoints path/to/registry.json` supports a reviewed registry snapshot
and isolated test fixtures; paths resolve from the repository root. Do not
use a self-authored registry to bypass acceptance.

No ownership or ratchet entries change. This contract sets no
machine-independent latency limits.

## Composition inventory

### Shrink-only fields and mutable wiring (#3030)

The normal `scripts/backend-architecture.sh check` compares composition source
with the merge-base of `HEAD` and `origin/development`. PR CI supplies the event's
full base SHA through `--base-ref`. Fetch `origin/development` before a local
check. Explicit project/baseline arguments remain available for isolated
analyzer fixtures; CI always supplies the real base commit.

The guard reads Go declarations directly from that Git object and the working
tree. It rejects new named or embedded fields on `services.Factory`,
`database/repositories.Factory`, and `api.API`, including local type aliases.
There is no accepted field/setter manifest and no approve, regenerate,
rebaseline, or wildcard option. Deletion spends the removed declaration's
budget permanently once it reaches the base branch.

Mutable wiring is identified by its destination, not by a `Set` prefix:
assignments to interface/function dependencies, dependency-containing bundles,
external pointer dependencies, and Worker/Scheduler scalar configuration are
guarded. Receiver methods and pointer-parameter wiring functions are covered,
including direct receiver aliases and whole-receiver replacement. The key
includes the destination field, so an existing setter cannot acquire another
dependency silently. Newly constructed local values are not mutable wiring.
Domain/model receivers, ordinary result records, and scheduler task state are
outside this guard. This is source analysis, not general interprocedural alias
or reflection analysis; review still checks indirect wiring.

Production, same-package tests, and external-package tests have separate keys.
Test declarations cannot authorize production growth. Parsing includes
build-tagged Go files, but excludes vendor, hidden directories, and fixture
`testdata` trees. Fixtures cover both additions and permitted non-composition
changes; the other architecture checks continue to enforce package boundaries.

The existing Factory-removal and composition-contract tickets retain ownership
of shrinking the current debt. This guard does not remove their work or change
runtime composition. The historical inventory below is separate evidence:
regenerating it cannot approve new fields or mutable wiring.

### Typed-root migration evidence

`composition.json` is the checked-in prerequisite inventory for the typed-root
migration. It is pinned to evidence commit
`8dc3a9ca8ac7cb8edbfa3c17760d92c02751bc3e` and records:

- the Serve, embedded Worker, and CLI roots, plus the affected packages'
  discoverable `TestMain` roots and one smoke test per root;
- every Cobra command path and scheduler job ID;
- every typed legacy-composition reference reported by this evaluator;
- every call to `api.New`, the evidence-only `api.NewServer`, the current
  `api.WithRuntime`, `repositories.NewFactory`,
  `services.NewFactory`, the evidence-only `scheduler.NewScheduler`, the current
  `scheduler.NewWorker`, and `SetupAPITest` under the
  affected production and test trees (`api`, `cmd`, `services`,
  `database/repositories`, and `test`); this is deliberately not a scan of
  unrelated unit-test packages or migration tests;
- each constructor caller's production/test-support scope, declaration, exact
  call lines, and concrete policy-owned tables; and
- the route count, cold Serve-root construction time, registered jobs, and
  full backend-suite duration measured at the evidence commit.

`evidence_legacy_callers` and `evidence_constructor_calls` preserve the exact
files, declarations, and lines from that commit. The unprefixed caller lists
mirror the current tree so the tests still reject newly unlisted callers after
the evidence commit; they must not be presented as historical measurements.

The runtime measurements are comparison evidence, not timing thresholds. The
composition tests fail when a root, caller, command, or job drifts. Typed
legacy references come from the architecture evaluator's existing `go/types`
analysis; the constructor inventory does not reimplement that analyzer.

After an intentional caller migration, regenerate the typed legacy locations,
then the discovered test-root, constructor-caller, and job sections with:

```bash
cd backend
go test ./internal/architecture \
  -run '^TestCompositionLegacyCallerInventory$' \
  -update-composition-legacy
go test ./test -run '^TestCompositionInventory$' \
  -update-composition-inventory
```

Review the manifest diff. Fixed evidence, production roots, command paths,
smoke-test names, and runtime measurements are deliberate evidence and are
never regenerated by these normal update flags.

## Changing the policy

1. Add or move the exact package classification and data-object owner.
2. Classify each new direct dependency. Do not use path wildcards.
3. Name every projection package, readable data object, and tenant-safety
   guarantee explicitly; projection grants never permit writes.
4. Add the narrowest owner, owner-kind, role, scope, and target selector that
   describes the intended seam.
5. Run `scripts/backend-architecture.sh check` and inspect every changed
   finding.
6. Run `cd backend && go test ./internal/architecture`.

The shared-kernel declaration is a closed list: `Date`, `WallClock`,
`TenantID`, and `CorrelationID`; kernel-owned packages must be contracts. The
loader rejects unknown JSON fields,
unsupported schema versions, invalid enum values, missing or conflicting table
owners, unsafe or overlapping projection grants, stale legacy symbols,
duplicate classifications, stale packages, unknown dependencies, and
logically overlapping rules. Change
`schema_version` only when the JSON shape changes. Change `policy_epoch` only
for a reviewed architecture decision; it does not approve or rebuild legacy
findings.
