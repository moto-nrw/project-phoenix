---
status: accepted
---

# Every backend source tree stays in the ratchets

Decision of 2026-10-23 for [#4737](https://github.com/moto-nrw/project-phoenix/issues/4737).
Complete the source gates requested by [ADR 0083](0083-ratchets-protect-invariants-and-expose-command-effects.md)
section 1. A move must retain its gate (Rule 16, ADRs 0064 and 0070).
The thresholds and all existing allowances stay unchanged.

## Decision

1. `walkModuleRatchetSources` walks **all of backend recursively**, rather
   than a growing list of package roots. It includes every policy-classified
   production package, both engineering-tool owners and test-support packages,
   plus source fixtures, regardless of build tags, GOOS or GOARCH. New package
   trees are covered before they enter `policy.json`; the architecture gate
   still rejects unclassified production packages. This conservative superset
   avoids a second policy loader or build-dependent gaps in these syntax gates.
2. Keep only the existing exclusions: `_test.go` and hidden directories.
   Unit/integration test bodies have their own behavior and hermetic gates;
   hidden directories are not Go package trees and contain local tool caches.
   Non-`_test.go` fixture helpers, `testhooks`, `legacy/`, `testdata`, generated
   sources and CLI/devtools are **included**. No owner, role, policy rule or
   import permission changes. All visible trees, including future ones, use
   the same thresholds: 800 newlines/file, 60 body lines/named function (braces
   inclusive), gocognit 1.3.0 score 15.
3. **Include migrations deliberately.** The 616 files in `database/` are all
   in `database/migrations`, the policy's `migrations` owner / `migration` role.
   They include SQL schema history, backfills and the active migration runner.
   SQL strings inflate physical length but are not Go branches. Exempting this
   tree would also hide handwritten backfill/control logic and create another
   destination for unmeasured glue. Freeze existing large strings at their
   exact lengths, rather than exempting the tree or rewriting historical SQL
   merely to meet the threshold. Future long migrations need named steps or
   a separate reviewed ADR, not an automatic allowance. These syntax gates do
   not establish SQL complexity or migration correctness.
4. **Include generated declarations deliberately.** There are exactly two
   generated Go files: `api/common/error_codes.generated.go` (1,348 lines)
   and its 4-line generator fixture under `api/common/testdata/errorcodes/`.
   ADR 0082 already measures both. Keep its 1,348-line allowance unchanged;
   a generated banner or filename is not permission to hide handwritten code.
   Future generator expansion needs its own decision under ADR 0070.
5. Add only the exact newly exposed entries below to the existing registers,
   citing **ADR 0086 on each new entry**, not on the whole register. This is the
   new-numbered decision required by ADR 0057 point 5 and
   `internal/architecture/ratchet_budgets.go`. It does not authorize raises of
   existing entries. Every entry remains shrink-only: growth, missing entries,
   stale entries and unrecorded improvements all fail as before. No existing
   assertion or threshold is weakened. The handler-complexity deduplication,
   compose construction gate and pass-through successor remain separate cuts.

## Audited inventory

Measured at `origin/development` **`8aca1925f79c1e723e35f14e65398cc82877fd7f`**,
using the command below. Counts are non-`_test.go` Go files and physical newlines
(`wc -l` semantics), including tagged sources and fixtures. Category labels
come from the exact package entries in `architecture/policy.json`, with
fixture/generated overlays. No non-fixture source package is unclassified.
Policy entries without Go source add no files to the source set.

| Backend tree | Files | LOC | Category and decision |
|---|---:|---:|---|
| `.` | 1 | 9 | Production process root: include |
| `analytics` | 7 | 1,424 | Production, Observability HTTP: include |
| `api` | 364 | 34,335 | 347 production, 10 test-support, 5 handwritten fixtures, 2 generated: include all |
| `auth` | 2 | 512 | Production, Security Runtime contracts: include |
| `cmd` | 23 | 5,712 | 11 process CLI, 12 Dev Tools: include all |
| `database` | 616 | 74,064 | Migrations/runner/backfills: include deliberately, as above |
| `demoprofile` | 1 | 687 | Dev Tools contract: include |
| `internal` | 160 | 31,186 | 5 production, 103 tool, 15 test-support, 37 source fixtures: include all |
| `middleware` | 4 | 537 | Production, Tenant Runtime HTTP: include |
| `modules` | 4,039 | 538,943 | 3,984 production, 55 test-support: retain all |
| `observability` | 9 | 2,314 | Production platform/adapters/composition: include |
| `realtime` | 6 | 1,430 | 5 production, 1 test-support: include all |
| `roots` | 355 | 37,557 | Production composition: retain all |
| `seed` | 2 | 271 | Dev Tools contracts: include |
| `sharedkernel` | 7 | 592 | Production kernel contracts: include |
| `tenant` | 12 | 1,064 | Production, Tenant Runtime: include |
| `test` | 89 | 12,284 | Test-support/fixtures/scanners: include |
| `workflows` | 237 | 34,162 | Production workflows: retain all |
| **Total** | **5,934** | **777,083** | **No category exclusion** |

Disjoint category totals: 4,986 production files / 648,760 LOC;
616 migration files / 74,064 LOC; 118 tool files / 31,020 LOC;
170 test-support files / 21,253 LOC; 42 handwritten source fixtures / 634 LOC;
2 generated files / 1,352 LOC. CLI entry points in `cmd` remain production
composition; Dev Tools and Architecture Check are policy owner kind `tool`.
`internal/architecture` has 74 files / 10,347 LOC including its 37 fixtures;
`internal/devtools` has 66 / 17,690; `internal/testdb` has 14 / 2,866.

Reproduce the inventory from the repository root at the measured revision:

```sh
git rev-parse HEAD
python3 - <<'PY'
import collections, json, os
from pathlib import Path
root = Path('backend')
policy = json.loads((root / 'architecture/policy.json').read_text())
packages = {p['path']: p for p in policy['packages']}
owners = {o['id']: o['kind'] for o in policy['owners']}
trees = collections.defaultdict(lambda: [0, 0])
categories = collections.defaultdict(lambda: [0, 0])
for directory, dirs, files in os.walk(root):
    dirs[:] = sorted(d for d in dirs if not d.startswith('.'))
    for name in sorted(files):
        if not name.endswith('.go') or name.endswith('_test.go'):
            continue
        path = Path(directory) / name
        rel = path.relative_to(root)
        data = path.read_bytes()
        package = packages.get(rel.parent.as_posix())
        if 'testdata' not in rel.parts:
            assert package is not None, rel
        if b'Code generated' in data[:2000]:
            category = 'generated'
        elif 'testdata' in rel.parts:
            category = 'testdata'
        elif package['role'] in ('migration', 'test-support'):
            category = package['role']
        elif owners[package['owner']] == 'tool':
            category = 'tool'
        else:
            category = 'production'
        tree = rel.parts[0] if len(rel.parts) > 1 else '.'
        for stats, key in ((trees, tree), (categories, category)):
            stats[key][0] += 1
            stats[key][1] += data.count(b'\n')
for label, stats in (('trees', trees), ('categories', categories)):
    print(label)
    for key, counts in sorted(stats.items()):
        print(key, *counts)
PY
```

## Exact budgets and measurement

Extend the shared scanner first, then run the three unchanged ratchets against
it before adding entries. Every newly exposed diagnostic states its exact
measurement; keep existing register values unchanged. Reproduce/check from
`backend/` with:

```sh
../scripts/run-go-toolchain.sh go test -tags=testhooks ./test/ \
  -run '^TestModule(FileSize|FunctionLength|Complexity)Ratchet$' -count=1
```

The initial red run at the revision above reported exactly 12/265/142 missing
entries. The new entries live in the three test registers, each with `ADR 0086`.
Their counts/sums are:

| Register | Existing entries / sum | Added entries / sum | Complete entries / sum |
|---|---:|---:|---:|
| `moduleFileSizeAllowlist` | 2 / 2,487 | 12 / 15,645 | 14 / 18,132 |
| `moduleFuncLenAllowlist` | 27 / 3,834 | 265 / 25,624 | 292 / 29,458 |
| `moduleComplexityAllowlist` | 13 / 285 | 142 / 3,454 | 155 / 3,739 |

The new entries by tree (size / length / complexity): `database` 3/213/40,
`internal` 7/43/87, `test` 1/6/10, `observability` 1/1/1, `realtime` 0/1/1,
`seed` 0/1/1, `analytics` 0/0/1, `middleware` 0/0/1. Other new trees have no
above-threshold source. These sums describe different instruments, not a
combined debt score. No headroom is added.

## Failure evidence and consequences

`TestModuleRootSourceRatchetsRejectNewViolations` retains the original root
cases and adds every newly covered top-level category, a migration, generated
source, a legacy source, architecture `testdata`, and a future tree absent from
the policy. Each synthetic source must exceed all three thresholds, and all
three violation evaluators must reject every unregistered key. The original
`_test.go` and hidden-directory exclusions remain checked.

A local negative run added a temporary source to the newly covered `tenant/`
with an 866-line file and a 64-line named function scoring 16. All three
ratchets failed with the probe's path; the probe was removed before the final
checks. The PR records the actual output and checks. No production source
changes are needed for this issue.

Larger cleanup candidates are reported, not refactored here: `test/fixtures.go`
(3,201 lines), `observability/prometheus.go` (1,722),
`internal/architecture/semantic.go` (1,498),
`internal/devtools/seedapi/stammdaten.go` (1,477), and
`internal/devtools/seedapi/parent_enrollment.go` (1,440).
`migrationSQLBodyDataObjects` scores 85 and `compareRatchetBudgets` 70;
`migrateUniqueConstraints` has a 340-line body. A length gate does not decide
whether rewriting historical SQL is warranted.

### Proposals for the parallel documentation cut

Do not edit the files owned by #4541 or #4736 in this change.

- Rule 16 in `backend-conventions.md`: describe the three source gates as
  backend-wide, cite ADR 0086's category audit, retain the thresholds and
  shrink-only rule, and distinguish the narrower facade/pass-through scopes.
- `backend/architecture/WORKPLAN.md`: mark #4737's complete-source-gates cut
  done after merge; leave handler-complexity deduplication, typed composition
  checks and the pass-through successor as separate unfinished cuts.
