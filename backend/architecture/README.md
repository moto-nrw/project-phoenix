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

`audit-issues` performs the network-dependent GitHub liveness check separately.
The wrapper supplies the committed baseline; callers must provide `--api-url`,
and `GITHUB_TOKEN` is optional for authenticated requests. A GitHub or network
error fails this audit and cannot change the deterministic `check` result or
appear as a green audit.

## Composition inventory

`composition.json` is the checked-in prerequisite inventory for the typed-root
migration. It is pinned to evidence commit
`8dc3a9ca8ac7cb8edbfa3c17760d92c02751bc3e` and records:

- the Serve, embedded Worker, and CLI roots, plus the affected packages'
  discoverable `TestMain` roots and one smoke test per root;
- every Cobra command path and scheduler job ID;
- every typed legacy-composition reference reported by this evaluator;
- every call to `api.New`, `api.NewServer`, `repositories.NewFactory`,
  `services.NewFactory`, `scheduler.NewScheduler`, and `SetupAPITest` under the
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
