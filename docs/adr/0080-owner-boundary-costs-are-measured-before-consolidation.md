---
status: accepted
---

# Owner boundary costs are measured before consolidation

Decision of 2026-10-23 for [#4546](https://github.com/moto-nrw/project-phoenix/issues/4546),
after #2580. Keep explicit cross-owner bindings; reduce duplicate vocabulary
inside an owner, not the owner boundary. This decision changes no ownership,
policy rule, ratchet budget or runtime code. Projection consolidation and the
large-domain cuts belong to [#4547](https://github.com/moto-nrw/project-phoenix/issues/4547).

## Decisions

1. **Projection candidates: defer the cut to #4547.** The inventory below
   gives costs for every small projection named in #4546, including glue
   outside its directory. Small size alone is not a reason to transfer SQL
   grants to a domain. Preserve the exact joined-table grants and tenant-safe
   reads of [ADR 0066](0066-principle-every-module-table-and-cross-owner-read-has-one-owner.md)
   points 4 to 7 and the dependency directions of
   [ADR 0071](0071-principle-owner-dependencies-point-one-way.md). No candidate
   is merged, renamed or reclassified here.
2. **Wiring: keep explicit bindings, do not add a generic binding framework.**
   The three measured seams cost 1.21, 1.00 and 0.34 wiring LOC per read-
   implementation LOC. They solve different problems: row-source selection,
   ambient-transaction enforcement, and result conversion. Reuse the existing
   ambient database runtime and direct constructors where their contracts fit.
   The existing typed `Map` helpers in Staff Announcement Audience and Operator
   Dashboard are local solutions, not evidence for a new shared abstraction.
   Reflection, generated bindings or a generic row-mapper would not remove the
   consumer's vocabulary, tenant checks or SQL ownership. There is no measured
   repeated mechanism here that needs a new abstraction.
3. **Pass-throughs: later, narrow same-owner vocabulary cleanup.** Prefer a
   single published value inside Identity & Access when the internal copy adds
   neither an invariant nor storage detail. The sampled `SchoolDirectory` and
   `StaffDirectory` adapters contain 12 counted translation sites. They are
   candidates, not a promise that 12 hits or their whole files can disappear.
   Preserve `School.Live`, lifecycle orchestration, nil/empty collection
   behavior and sentinel mappings. Apply the principle already used by
   [ADR 0074](0074-delivery-and-export-transfer-speak-their-own-contract.md),
   without sharing values between independent owners or introducing aliases.
   The change crosses internal ports, domain values, implementations and
   fixtures, so it is a separate implementation proposal, not part of this
   measurement PR. It does not block #4547.
4. **Developer cost: keep the explicit read seam.** PR #4617 changed 29 files,
   23 in the backend, of which 15 are production Go files. Two read owners,
   the root and test support were touched. This is a concrete maintenance
   cost, not a measured duration or evidence that every feature has this cost.
   Do not consolidate owners or add a gate from one sample. Reuse the
   file/owner accounting below when #4547 compares proposed cuts; review the
   vocabulary and selection rules, not just the number of files.

[ADR 0067](0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md)
continues to require consumer-owned ports/contracts and workflows for writes
across owners. [ADR 0068](0068-principle-owners-compose-themselves-and-the-roots-only-wire.md)
continues to require owner-local construction and root-only cross-owner glue.
[ADR 0075](0075-pass-throughs-keep-a-per-owner-floor.md), including its amendments,
is unchanged: the five floor shapes and four named exceptions do not license
budget growth or removal of transaction/validation behavior.

## Measurement basis and limits

Fetched `origin/development` before branching. The measured tree is
`068511c21700c6dac166cfc3c22a29d524bf7e7e` (23 October 2026); the issue's
reference is `ebad9bbaac50e0b9b2151a4ec14c3929bb07a899`. Issue #4546 had no
comments when read. All current numbers below refer to the measured tree,
not to the later documentation commit or a moving branch name.

LOC means physical lines, including comments and blanks, in tracked backend
`.go` files, excluding `_test.go` and files with a `//go:build` line containing
`testhooks`. It is a source-size proxy, not semantic business statements,
compiled binary size, runtime cost or developer time. Untagged fixture helpers,
engineering tools and migration Go files remain included. This matches the
issue's reproducible 759k denominator. The architecture/ratchet scan deliberately
uses a different, hook-enabled superset; do not apply the LOC exclusions to it.

### Repository totals

| Surface | Issue reference, remeasured | Current |
|---|---:|---:|
| All backend Go, with migrations | 758,888 | 755,591 |
| Without `database/migrations/` | 684,938 | 681,527 |
| `roots/` | 35,875 | 36,026 |
| Every directory segment `ports/` | 15,239 | 15,229 |
| Every directory segment `compose/` | 11,469 | 11,515 |
| Union of the preceding three sets | 62,583 | 62,770 |
| Union / all backend Go | 8.25% | 8.31% |
| Union / backend Go without migrations | 9.14% | 9.21% |
| Projection / domain / workflow / platform owners | 19 / 18 / 12 / 10 | 19 / 18 / 13 / 10 |
| Timetable / Care Plan / Workforce / Identity & Access LOC | 70,125 / 62,965 / 53,572 / 51,513 | 68,266 / 63,231 / 53,552 / 51,446 |

The three directory sets do not overlap at either measured SHA. Their union
is not all wiring: it misses adapters in `api/`, owners' application packages
and directory names such as `composition/`. Conversely it includes comments,
configuration and correctness checks that are not removable ceremony. The
issue's "about 9% of 759k" mixes rough rounding with the denominator: about
8.3% includes migrations; about 9.2% excludes them.

The extra workflow is `staff-person-merge`, recorded by
[ADR 0076](0076-reviewed-staff-person-repair-adds-narrow-compositions.md) and the
policy diff between the two SHAs. The small projection sizes are all unchanged.
The issue snapshot already has a 1,413 pass-through budget, not 1,418; its LOC
and ADR 0075 figures therefore do not describe one immutable snapshot.

Historical checks use the last **first-parent** commit before the UTC cutoff:

| Cutoff (exclusive) | SHA | Go LOC without migrations | `compose/` LOC |
|---|---|---:|---:|
| 2026-08-31 00:00 UTC | `ea7804f8dd7b55a750ed676e845d18e53aede318` | 383,004 | 0 |
| 2026-10-03 00:00 UTC | `6edceebe39d78f65bb3ed409eb7d16ae35946c25` | 646,871 | 65,976 |

The October figure reproduces the issue's approximately 66k composition lines.
The August "332k" cannot be reproduced with this explicit cutoff and filter;
the issue supplies neither its August SHA nor a counting command. The selected
first-parent commit is dated 29 August. Do not treat this date proxy as a
reconstruction of the original measurement, or infer feature-versus-migration
cost from the difference. The release/rollback history does not separate them.

### Small projections: cost inventory for #4547

Paths abbreviate `backend/modules/`. `C` is all production LOC in policy role
`compose`, including constructors outside a directory named `compose`.
`K` counts dedicated contract/port files; `A` counts the explicitly selected
owner-local adapter sources below. `R` is selected external root glue, not
whole mixed root files. `W = C + K + A + R` is an **inventoried lower bound**,
not the complete marginal cost or the lines a merger would save. Consumer
ports and shared runtimes outside these selections remain additional costs.
`S` counts distinct `(file,line)` direct projection-selector references in
`api/` and `roots/`, not binding instances: references to an already bound
value are not attributed. It describes fan-out, and is not added to `W`.

| Projection directory | Owner LOC | C | K | A | R | W ≥ | S |
|---|---:|---:|---:|---:|---:|---:|---:|
| `timetableenrollmentprovenance` | 80 | 10 | 0 | 13 | 0 | 23 | 1 |
| `guardianlinkview` | 90 | 8 | 0 | 25 | 0 | 33 | 8 |
| `timetableblockdisplay` | 93 | 9 | 0 | 0 | 0 | 9 | 1 |
| `parentrequestfeedprojection` | 159 | 11 | 0 | 0 | 40 | 51 | 2 |
| `staffmessageinbox` | 205 | 6 | 16 | 0 | 29 | 51 | 2 |
| `timetableprojection` (`timetable-legacy-view`) | 220 | 7 | 0 | 0 | 13 | 20 | 4 |
| `studentdirectoryview` | 327 | 9 | 0 | 0 | 0 | 9 | 20 |
| `staffannouncementaudience` | 344 | 12 | 45 | 60 | 0 | 117 | 1 |
| `staffdirectoryview` | 451 | 0 | 31 | 0 | 196 | 227 | 17 |
| `operatordashboard` | 527 | 16 | 56 | 65 | 28 | 165 | 5 |
| `emergencysnapshot` | 623 | 13 | 108 | 11 | 279 | 411 | 26 |
| `parentmessageinbox` | 848 | 8 | 55 | 0 | 132 | 195 | 10 |

Selection manifest, all line ranges inclusive at the measured SHA:

- K: `contract/*.go` for Staff Message Inbox, Staff Announcement Audience,
  Operator Dashboard and Parent Message Inbox; `emergencysnapshot/ports.go`;
  `staffdirectoryview/staffdirectoryview.go:34-64` (ports plus their Person value).
- A: `timetableenrollmentprovenance/internal/adapters/postgres/query.go:58-70`;
  `guardianlinkview/internal/adapters/postgres/reader.go`; Staff Announcement
  Audience and Operator Dashboard's `internal/adapters/postgres/mapped.go`;
  `emergencysnapshot/internal/adapters/nameorder/legacy.go`.
- R: `backend/api/parent_request_feed.go`, `backend/api/emergency_snapshot.go`;
  `backend/roots/owners/staff_inbox_binding.go`, `parent_inbox_binding.go`,
  `operator_dashboard_projection.go`, `student_deletion_projection.go`;
  `staff_membership_adapters.go:348-543` for Staff Directory View only.

Zero means no selected source, not no dependency or no port. For example,
Student Directory's 20 direct reference lines and Guardian Link's eight are
mostly direct `NewReader` arguments in mixed constructors; their whole files
must not be charged to each projection. Timetable Block Display's consumer
adapter is outside the projection and appears in the seam measurement below.
Emergency Snapshot includes renderer conversion and construction guards, not
only interchangeable type copies. Staff Directory and Emergency Snapshot are
port-composed read models, unlike the direct SQL projections. The inventory
must not silently turn either into an additional SQL grant.

### Three seams: port, adapter and binding per read-implementation line

The denominator `F` is the selected projection's read implementation, including
its SQL, result assembly, validation, comments and source scaffold. Wiring
uses exact file/range selections, disjoint from F. These are local seam
measurements, not a full allocation of every transitive dependency; shared
transaction runtimes, shared import blocks and repeated process-root mounts
are not allocated per seam. Port/adaptation code can itself enforce behavior.

| Consumer ← projection | F | Port/contract P | Adapter A | Binding B | (P+A+B)/F |
|---|---:|---:|---:|---:|---:|
| People Directory ← Guardian Link View | 57 | 9 | 51 | 9 | 69/57 = 1.21 |
| Timetable ← Block Display | 84 | 3 | 47 | 34 | 84/84 = 1.00 |
| Communication ← Staff Message Inbox | 183 | 25 | 29 | 9 | 63/183 = 0.34 |

Exact allocations (paths relative to `backend/`):

- **Guardian Link.** F: `modules/guardianlinkview/internal/adapters/postgres/projection.go`.
  P: `modules/peopledirectory/internal/adapters/peoplepostgres/projection_rows.go:21-29`.
  A: Guardian Link's `reader.go` (25), plus that consumer file's
  `38-43` (6, required reader) and `55-74` (20, tenant/all-school selection).
  B: Guardian Link's `compose/new.go` (8), plus
  `roots/owners/people_directory_composition.go:41` (1). This measures the
  People Directory binding, not all consumers of Guardian Link.
- **Block Display.** F: `modules/timetableblockdisplay/internal/adapters/postgres/projection.go`.
  P: `modules/timetable/ports/timetable_data.go:50-52` (3).
  A: `modules/timetable/internal/adapters/postgres/native_compose_bindings_timetable_data_runtime.go`
  (47, includes the function port and the required ambient-transaction check).
  B: projection `compose/compose.go` (9), consumer
  `modules/timetable/compose/timetable_data_runtime.go` (24), plus
  `roots/owners/root_timetable.go:65` (1).
- **Staff Inbox.** F: the projection's `internal/adapters/postgres/*.go` (183).
  P: projection `contract/rows.go` (16) plus
  `modules/communication/staff_inbox_port.go` (9).
  A: `roots/owners/staff_inbox_binding.go` (29).
  B: projection `new.go` (6) plus
  `roots/owners/communication_projection_roots.go:53-55` (3).
  The shared `staffstore.ProjectionDatabase` is not charged again; it delegates
  to the existing ambient database runtime. The field-by-field row conversion
  here is not a reason to create a shared generic mapper for all three seams.

In particular, removing the Guardian Link tenant selector or Block Display's
transaction check would lower the numerator by deleting correctness, not waste.
Their SQL still joins multiple write owners; moving the query into the
consumer does not by itself remove that ownership boundary.

## Pass-through findings

`TestModulePassthroughRatchet` passes at the measured SHA with exactly 1,410
hits across 30 owners, equal to every budget. ADR 0075's 1,418 and 648 Edge
figures describe its amendment's historical classification, not a current
reclassification. Compared with that table, Identity & Access is 218 → 215,
People Directory 210 → 208, Timetable 208 → 204, and Communication 57 → 58;
all other owner totals are unchanged. The added Communication hit is explicitly
approved by [ADR 0079](0079-neue-nachricht-markiert-kinder-ohne-erreichbare-eltern.md).
The issue reference's budget is already 1,413. Do not subtract eight from the
648 Edge bucket: changed totals alone do not identify the shapes that changed.
A full five-shape reclassification is not claimed here.

The issue's Identity examples still exist, now in `internal/application/`:

- `account_authentication.go:146-173`: `SchoolDirectory` adapts the root-facing
  `identityaccess/ports.SchoolDirectory` to the internal port. Four counted
  methods convert `identityaccess.School` into `domain.School`, including
  the slice returned by `ListActiveSchoolsByID`.
- `account_lifecycle.go:105-152`: eight counted `StaffDirectory` methods
  convert person/staff/caregiver values or their maps, or convert input to
  the same owner's root-facing value. The signatures and struct conversions
  demonstrate duplicate vocabulary; no foreign owner's DTO has to become
  Identity's domain model to remove it.
- Not everything nearby is a copy: `MoveAccountLink` coordinates unlink and
  link operations; `internal/domain/account_login.go:83` defines `School.Live`;
  `account_lifecycle_errors.go` preserves public sentinel semantics.
  `TestLifecycleErrorTranslatesEverySentinel` and
  `TestLifecycleErrorFallsThroughToTheSessionMapping` lock those error mappings.

The twelve sites are measured using the ratchet's unchanged four AST conditions,
not counted by searching `return`. Free functions, promoted methods, longer
methods and unregistered root paths can escape this shape scan. Green proves
only its declared surface. This decision neither broadens it nor moves copies
out of its scope; scope completeness remains the separate #4542 work.

## Developer-cost sample: feature PR #4617

[#4617](https://github.com/moto-nrw/project-phoenix/pull/4617), "name who filed
each parent request in the request review", is a recent, bounded read feature:
merge `082940f7afc1318de738315f81b47fb12edf3df2`, first parent
`e73b8ed4863677ef5d777a6b86af6b9ddcfd2bf7`. Count the actual merge diff,
not the current directory size or the PR branch's intermediate merges. Use the
policy **at that merge** to classify exact package directories.

| Surface | Files | Added / deleted lines |
|---|---:|---:|
| People Directory | 8 | 39 / 18 |
| Request Review View (`request-review-view`, projection) | 7 | 236 / 4 |
| Root Composition | 7 | 94 / 5 |
| Test Support | 1 | 3 / 1 |
| Frontend, outside backend owner accounting | 6 | 164 / 13 |
| Total | 29 | 536 / 41 |

Backend: 23 files = 15 production + six `_test.go` + two fixture/helper files
(`api/student_owner_http_test_helpers.go`, `test/request_review_fixture.go`).
The 15 production files occupy two read owners and the root, not four business
modules. Four existing queue bindings propagate `SubmittedBy`; the new
`roots/requestreviewbinding/submitters.go` contributes 83 lines, and the
consumer port/values in `modules/requestreview/ports.go` add 24. No policy,
projection grant, owner registration or migration changed.

This is also why "root LOC" is not automatically ceremony: the new binding
resolves profiles and relationships and chooses the lowest profile ID when
stale duplicates exist (`profilesByAccount`, lines 73-82). That selection rule
is a candidate for a separate owner-placement review under ADR 0068 point 4;
replacing the file with a generic mapper would preserve the misplaced decision.
No runtime failure is inferred from the file count. The PR's tenant, relationship
and batching behavior must survive any future relocation.

## Locks and follow-up scope

- **Ownership and isolation:** `backend/internal/architecture/cli_test.go`
  (`TestCheckRejectsMissingOrConflictingTableOwners`,
  `TestCheckRejectsInvalidReadProjection`) and `target_rule_test.go` reject
  foreign private reach and composition outside roots. `policy.json` keeps
  exact packages, roles, write owners and tenant-safe projection grants.
- **Direction:** `acyclic_target_test.go` rejects a target cycle;
  `canonical_owners_test.go` pins the domain/platform lists. No generic helper
  or merger can bypass these by importing a foreign implementation.
- **Composition and counts:** `TestModulePassthroughRatchet`, the
  composition LOC ratchet in `test/module_legacy_budget_ratchet_test.go`,
  `TestModuleComposeAliasRatchet`, and `internal/architecture/ratchet_budgets_test.go`
  lock the measured surfaces and shrink-only rules. `composition_owners_test.go`
  covers root structs holding multiple owners. None proves that all wiring is
  semantically necessary or catches every root decision.
- **Read behavior:** Guardian Link's
  `TestGuardianProjectionJoinsPermissionsAndPickupWithoutCrossingTenants`,
  Block Display's `TestBlockDisplayReturnsOnlyTheNamedTenantsRoomAndTemplate`,
  and Staff Inbox's `TestListInboxAnswersOnlyTheNamedSchool` name existing
  tenant-safety locks. They are pointers for follow-up implementations, not
  tests claimed to have been rerun by this documentation change.

Follow-up proposals, not newly created issues: (a) #4547 uses the per-projection
inventory before choosing mergers; (b) unify Identity's internal/public read
values around the twelve sampled translations, keeping behavioral tests and
lowering budgets only after actual deletions; (c) review the Request Review
submitter selection rule's placement and move it behind an owner capability
if it is a domain decision. No shared mapper, blanket alias, policy exception,
new write edge or new threshold is authorized by this ADR.

## Reproducing the measurements

Run from the repository root. Commands read immutable Git objects. Python uses
only its standard library; no measurement script is installed in the repo.

### LOC, owners, composition roles and root reference lines

```bash
git rev-list -1 --first-parent --before=2026-08-31T00:00:00Z 068511c21700c6dac166cfc3c22a29d524bf7e7e
git rev-list -1 --first-parent --before=2026-10-03T00:00:00Z 068511c21700c6dac166cfc3c22a29d524bf7e7e
for ref in 068511c21700c6dac166cfc3c22a29d524bf7e7e ebad9bbaac50e0b9b2151a4ec14c3929bb07a899 ea7804f8dd7b55a750ed676e845d18e53aede318 6edceebe39d78f65bb3ed409eb7d16ae35946c25; do
python3 - "$ref" <<'PY'
import collections, json, pathlib, re, subprocess, sys
ref = sys.argv[1]
paths = subprocess.check_output(
    ['git', 'ls-tree', '-r', '--name-only', ref, 'backend'], text=True).splitlines()
paths = [p for p in paths if p.endswith('.go') and not p.endswith('_test.go')]
proc = subprocess.Popen(['git', 'cat-file', '--batch'],
                        stdin=subprocess.PIPE, stdout=subprocess.PIPE)
files = {}
for path in paths:
    proc.stdin.write((ref + ':' + path + '\n').encode())
    proc.stdin.flush()
    size = int(proc.stdout.readline().split()[2])
    data = proc.stdout.read(size)
    proc.stdout.read(1)
    if not re.search(rb'^//go:build .*\btesthooks\b', data, re.M):
        files[path] = data
proc.stdin.close()
proc.wait()
loc = {f: len(b.splitlines()) for f, b in files.items()}
print('SHA', ref, 'all', sum(loc.values()), 'no migrations',
      sum(n for f, n in loc.items() if not f.startswith('backend/database/migrations/')))
sets = {'roots': {f for f in files if f.startswith('backend/roots/')}}
sets.update({k: {f for f in files if '/' + k + '/' in f} for k in ('ports', 'compose')})
for key, paths in sets.items():
    print(key, sum(loc[f] for f in paths))
union = sum(loc[f] for f in set.union(*sets.values()))
print('union', union, 'share', union / sum(loc.values()))
p = json.loads(subprocess.check_output(['git', 'show', ref + ':backend/architecture/policy.json']))
print('owner kinds', dict(collections.Counter(o['kind'] for o in p['owners'])))
packages = {x['path']: x for x in p['packages']}
mods = '''timetableenrollmentprovenance guardianlinkview timetableblockdisplay
parentrequestfeedprojection staffmessageinbox timetableprojection studentdirectoryview
staffannouncementaudience staffdirectoryview operatordashboard emergencysnapshot
parentmessageinbox timetable careplan workforce identityaccess'''.split()
for mod in mods:
    subset = {f: n for f, n in loc.items() if f.startswith('backend/modules/' + mod + '/')}
    roles = collections.Counter()
    for f, n in subset.items():
        pkg = str(pathlib.PurePosixPath(f).parent).removeprefix('backend/')
        roles[packages.get(pkg, {}).get('role', 'unclassified')] += n
    hits = set()
    for f, data in files.items():
        if not f.startswith(('backend/roots/', 'backend/api/')):
            continue
        source = data.decode()
        pattern = r'(?m)^\s*(\w+)?\s*"github.com/moto-nrw/project-phoenix/modules/' + mod + r'(/[^"\n]*)?"'
        for match in re.finditer(pattern, source):
            suffix = match[2] or ''
            alias = match[1] or (suffix.split('/')[-1] if suffix else mod)
            hits.update((f, i + 1) for i, line in enumerate(source.splitlines())
                        if re.search(r'\b' + alias + r'\.', line))
    print(mod, sum(subset.values()), dict(roles), 'root reference lines', len(hits))
PY
done
```

### File and range allocations

Use the selection manifest above with these functions. Whole-file counts include
imports, comments and blanks; range counts include only the named inclusive
lines. Shared mixed-file import blocks are not silently allocated several times.

```bash
ref=068511c21700c6dac166cfc3c22a29d524bf7e7e
file_loc() { git show "$ref:$1" | wc -l; }
range_loc() { git show "$ref:$1" | sed -n "$2,${3}p" | wc -l; }
file_loc backend/modules/guardianlinkview/internal/adapters/postgres/projection.go
range_loc backend/modules/peopledirectory/internal/adapters/peoplepostgres/projection_rows.go 21 29
range_loc backend/roots/owners/staff_membership_adapters.go 348 543
# Repeat file_loc/range_loc for each named allocation; add the disjoint spans.
git diff ebad9bbaac50e0b9b2151a4ec14c3929bb07a899 "$ref" -- backend/architecture/policy.json backend/test/module_passthrough_ratchet_test.go
```

### Pass-through total and the twelve sampled translations

The first command's success proves measured count = budget in each owner;
read the register at the same SHA for the 1,410 total. The second command uses
the existing AST predicate unchanged and prints the twelve named sites. Run
on a checkout of the measured tree with its pinned Devbox toolchain installed.

```bash
(cd backend && ../scripts/run-go-toolchain.sh go test -tags=testhooks ./test -run '^TestModulePassthroughRatchet$' -count=1 -v)
python3 - <<'PY'
import pathlib, subprocess, tempfile
source = pathlib.Path('backend/test/module_passthrough_ratchet_test.go').read_text()
header = '''package main
import("fmt";"go/ast";"go/parser";"go/token";"os")
const modulePassthroughMaxBodyLines=14
func main() {
 for _, path := range os.Args[1:] {
  if path == "--" { continue }
  fs := token.NewFileSet()
  f, err := parser.ParseFile(fs, path, nil, 0); if err != nil { panic(err) }
  for _, decl := range f.Decls {
   fn, ok := decl.(*ast.FuncDecl)
   if !ok || !modulePassthroughIsDelegation(fn, fs) { continue }
   r, ok := fn.Recv.List[0].Type.(*ast.Ident)
   if ok && (r.Name == "SchoolDirectory" || r.Name == "StaffDirectory") {
    fmt.Printf("%s:%d %s.%s\\n", path, fs.Position(fn.Pos()).Line, r.Name, fn.Name.Name)
   }
  }
 }
}
'''
with tempfile.TemporaryDirectory(prefix='owner-cost-') as tmp:
    path = pathlib.Path(tmp) / 'scan.go'
    path.write_text(header + source[source.index('func modulePassthroughIsDelegation'):])
    subprocess.run(['scripts/run-go-toolchain.sh', 'go', 'run', str(path), '--',
        'backend/modules/identityaccess/internal/application/account_authentication.go',
        'backend/modules/identityaccess/internal/application/account_lifecycle.go'], check=True)
PY
```

### Feature diff accounting

```bash
gh pr view 4617 --repo moto-nrw/project-phoenix --json title,mergeCommit,files
python3 - <<'PY'
import collections, json, pathlib, subprocess
sha = '082940f7afc1318de738315f81b47fb12edf3df2'
p = json.loads(subprocess.check_output(['git', 'show', sha + ':backend/architecture/policy.json']))
packages = {x['path']: x for x in p['packages']}
rows = collections.defaultdict(list)
kinds = collections.Counter()
for line in subprocess.check_output(['git', 'diff', '--numstat', sha + '^', sha], text=True).splitlines():
    added, deleted, path = line.split('\t')
    pkg = str(pathlib.PurePosixPath(path).parent).removeprefix('backend/')
    owner = packages.get(pkg, {}).get('owner', 'unclassified') if path.startswith('backend/') else 'frontend'
    rows[owner].append((int(added), int(deleted)))
    if path.startswith('backend/'):
        kinds['test' if path.endswith('_test.go') else 'helper' if
              'test_helpers' in path or path.startswith('backend/test/') else 'production'] += 1
for owner, changes in rows.items():
    print(owner, len(changes), 'files', sum(a for a, _ in changes), '/', sum(d for _, d in changes))
print('backend', dict(kinds))
PY
```
