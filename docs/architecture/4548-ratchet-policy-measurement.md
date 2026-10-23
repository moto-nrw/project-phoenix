# #4548 measurement record

Measured on 23 October 2026 after `git fetch origin`, at development
`2c7907c2fe725dd7e124a8b866d5dce2d8b09718` (includes #4627, #4635, #4644,
#4631 and #4636/#4638). [ADR 0083](../adr/0083-ratchets-protect-invariants-and-expose-command-effects.md)
contains the decisions. All source counts below use this immutable tree, not
an unspecified future `origin/development`.

`gh issue view 4548 --comments` and `gh issue view 4548 --json body,comments`
returned no comments. `gh pr list --state open --limit 100 --json number` plus
`gh pr view <number> --json files` found no new ADR number in open PRs; only
#4725 touches an existing ADR (0067). Highest local ADR was 0082, so 0083 is
used. #4642 and #4725 surfaces are read-only inputs to this decision.

## Policy and artifacts

| Measure | Result |
|---|---:|
| Rules / production-scoped / test-only | 927 / 620 / 307 |
| Target role public / contract | 409 / 127 |
| Explicit source-owner + target-owner selectors | 690 |
| External-class / same-owner / owner-kind-selector rules | 91 / 92 / 116 |
| Packages / data objects / read-projection grants | 474 / 257 / 13 |
| Domains / platforms / projections / workflows | 18 / 10 / 19 / 13 |
| Owners, all kinds | 68 |
| `test/*_ratchet_test.go` files | 19 |
| Top-level architecture JSON files | 2 |
| Architecture README lines | 5,060 |
| Legacy baseline bytes | 0 |

All rules grant imports; there is no Query/Command effect field. Selector
categories overlap. The glob counts files, not test functions or all gates.
In particular module-contract inventory lives outside that glob.
Reproduce from the repository root (requires Python 3):

```bash
python3 - <<'PY'
import collections, json, subprocess
ref = '2c7907c2fe725dd7e124a8b866d5dce2d8b09718'
def blob(path):
    return subprocess.check_output(['git', 'show', ref + ':' + path])
p = json.loads(blob('backend/architecture/policy.json'))
r = p['rules']
print({k: len(p[k]) for k in ('rules', 'packages', 'data_objects', 'read_projections', 'owners')})
print('production', sum('production' in x['scopes'] for x in r))
print('test only', sum('production' not in x['scopes'] for x in r))
for role in ('public', 'contract'):
    print(role, sum(x.get('target_role') == role for x in r))
print('owner pairs', sum(bool(x.get('source_owner')) and bool(x.get('target_owner')) for x in r))
for label, predicate in (
    ('external', lambda x: x.get('target_class')),
    ('same owner', lambda x: x.get('same_owner')),
    ('owner kind', lambda x: x.get('source_owner_kind') or x.get('target_owner_kind'))):
    print(label, sum(bool(predicate(x)) for x in r))
print(collections.Counter(o['kind'] for o in p['owners']))
paths = subprocess.check_output(['git', 'ls-tree', '-r', '--name-only', ref], text=True).splitlines()
print('ratchet files', sum(s.startswith('backend/test/') and s.endswith('_ratchet_test.go') for s in paths))
print('architecture JSONs', [s for s in paths if s.startswith('backend/architecture/') and s.count('/') == 2 and s.endswith('.json')])
print('README lines', len(blob('backend/architecture/README.md').splitlines()))
print('baseline bytes', len(blob('backend/architecture/legacy.jsonl')))
PY
```

The migration JSONs were already removed under ADR 0026's amendment, not
archived by #4548. The README's Migration evidence section gives their last
immutable tree. The remaining JSONs are policy and composition inventory.

## Executable scanner measurements

Using the actual Go scanner, not a reimplemented predicate:

| Surface | Entries/owners | Measured total |
|---|---:|---:|
| Owner pass-throughs | 30 | 1,408 hits |
| Of those, files tagged testhooks | subset | 42 hits |
| Topmost legacy trees | 0 | 0 LOC |
| `legacy_*.go` outside legacy trees | 1 | 246 LOC |
| Owner composition packages | 55 | 12,385 LOC |

Run the following temporary test **in an isolated checkout of the measured
SHA**, then delete it. It invokes the existing scanner; it is not maintained
measurement tooling or a new gate. The hook count retains the same path and
legacy exclusions as the scanner. Size, length and complexity include hooks
separately and are not affected by this breakdown.

```bash
cat > backend/test/review4548_measurement_test.go <<'GO'
package test

import (
 "testing"
 "strings"
 "go/ast"
)

func TestReview4548Measurement(t *testing.T) {
 t.Parallel()
 root, err := findBackendRoot(); if err != nil { t.Fatal(err) }
 counts, err := modulePassthroughScan(root, modulePassthroughOwnerKey); if err != nil {t.Fatal(err)}
 total := 0; for _, n := range counts {total += n}; t.Log("owner hits",total,"owners",len(counts))
 files, err := goSourceIndex(root); if err != nil {t.Fatal(err)}
 hooks := 0
 for _, f := range files {
  if strings.HasSuffix(f.rel,"_test.go") || strings.Contains(f.rel,"/legacy/") || modulePassthroughOwnerKey(f.rel)=="" || !strings.Contains(string(f.content),"//go:build testhooks") {continue}
  fs, syntax, err := parseGoSourceCached(f.rel,f.content); if err != nil {t.Fatal(err)}
  for _, d := range syntax.Decls {if fn,ok:=d.(*ast.FuncDecl);ok && modulePassthroughIsDelegation(fn,fs){hooks++}}
 }
 t.Log("testhooks hits",hooks)
 for name, scan := range map[string]func(string)(map[string]int,error){"legacy":moduleLegacyBudgetScan,"legacy files":moduleLegacyFileScan,"compose":moduleComposeScan} {
  c,e:=scan(root);if e!=nil{t.Fatal(e)};n:=0;for _,v:=range c{n+=v};t.Log(name,len(c),n)
 }
}
GO
(cd backend && ../scripts/run-go-toolchain.sh go test -tags=testhooks ./test/ -run '^TestReview4548Measurement$' -count=1 -v)
rm backend/test/review4548_measurement_test.go
```

The ordinary check prints **11 constants, 18 budget registers, 2 exception
registers**, composition surface **0 → 0**, legacy violations **0**:

```bash
scripts/run-go-toolchain.sh scripts/backend-architecture.sh check
```

`ratchet_budgets.go` compares compiler-evaluated integer declarations and map
budgets/exception counts. Its header explicitly excludes selector sets and
scanned directory lists. `module_source_ratchet_test.go:walkModuleRatchetSources`
selects modules, workflows, roots, api, cmd recursively and direct root files.
It does not select the other top-level production trees. This is a source-shape
coverage gap, not a claim that the import/SQL checker misses those packages:
that checker loads `./...`, with the hook-enabled superset and separate scopes.

The full-root shape reproduction remains:

```bash
python3 docs/architecture/4542-root-passthroughs.py
python3 docs/architecture/4542-root-passthroughs.py --sites > /tmp/root-sites.json
```

At the measured SHA it reproduces ADR 0082's 1,040 recursive hits (1,014 in
root-composition packages), 830 classified translations (823 in roots), 184
pure forwards (172 in roots), 15 guard-only sites and 11 other sites (4 in
roots). Body review of the seven named sites in ADR 0082's correction changes
translation to **at most 823 / 816** and guards to **at least 22**. No claim of
a full review of the remaining default-to-translation sites is made.

Device Fleet's reads are inspectable with
`git show 2c7907c2fe725dd7e124a8b866d5dce2d8b09718:backend/modules/devicefleet/internal/application/deviceservice/iot_service.go`:
the eleven `Get*` read methods call a converter, not unchanged direct returns.
The converters may still duplicate same-owner vocabulary; this measurement
neither proves their necessity nor licenses their deletion.

## Local runtime and CI runtime

Local host: Darwin arm64, pinned Go 1.27.3, `GOMAXPROCS=4`, `GOFLAGS=-p=4`.
After `devbox install` (the worktree initially lacked its profile):

```bash
/usr/bin/time -p scripts/run-go-toolchain.sh scripts/backend-architecture.sh check
```

Passed, **real 55.49 s, user 162.47 s, sys 30.89 s**. Candidate already included
the hermetic repair and comment correction; neither changes production code,
scan selectors, policy nor numerical registers. Other local checks overlapped,
so this is observed developer latency, not an isolated CPU benchmark. An earlier
unwrapped run took 41.04 s; it is not the pinned-toolchain measurement and is
not used for the decision.

Recent runs were selected with
`gh run list --workflow main.yml --limit 25 --json databaseId,headSha,conclusion,event`.
Inspect each with `gh run view <id> --json jobs,headSha`. Skipped architecture
jobs are excluded. Seconds below are differences between GitHub timestamps,
not total workflow duration or queue time:

| Run | Immutable head | Deterministic check step | Full architecture job |
|---|---|---:|---:|
| [39919914475](https://github.com/moto-nrw/project-phoenix/actions/runs/39919914475) | `3293b99d29aef2a99bd1ee15420e543bf2a3e93d` | 168 s | 228 s |
| [39919696431](https://github.com/moto-nrw/project-phoenix/actions/runs/39919696431) | `ead9911632456b861150f6cf1be19603b664a2e6` | 122 s | 166 s |
| [39918661491](https://github.com/moto-nrw/project-phoenix/actions/runs/39918661491) | `e0956d8a6972096f750ed0bd99fa353c7f915d1a` | 197 s | 254 s |

All three architecture jobs succeeded. Full jobs also fetch history, restore
caches, generate projections and audit issue references. These runs have
different heads from the measured source snapshot; they measure recent CI
latency, not repeated execution of that snapshot.

## Policy edits per feature/fix PR

Compare merge first parent to merge, not arbitrary branch tips. Exact-entry
set differences for packages, rules, data objects and projection grants:

| PR | Merge SHA | Added / removed in each of the four categories |
|---|---|---|
| #4617, request read seam | `082940f7afc1318de738315f81b47fb12edf3df2` | 0 / 0 |
| #4631, settings failure propagation | `722c5aa147a1baaf33d19adb8910663d9b9974db` | 0 / 0 |
| #4619, care source-booking cut | `c206d2d91ef826e74b0cc96ea2b2761de46be1db` | 0 / 0 |

Reproduce each SHA from `gh pr view <number> --json mergeCommit`; compare with:

```bash
python3 - <<'PY'
import json, subprocess
for sha in ('082940f7afc1318de738315f81b47fb12edf3df2',
            '722c5aa147a1baaf33d19adb8910663d9b9974db',
            'c206d2d91ef826e74b0cc96ea2b2761de46be1db'):
    before, after = [json.loads(subprocess.check_output(
        ['git', 'show', ref + ':backend/architecture/policy.json']))
        for ref in (sha + '^1', sha)]
    for key in ('packages', 'rules', 'data_objects', 'read_projections'):
        a = {json.dumps(x, sort_keys=True) for x in before[key]}
        b = {json.dumps(x, sort_keys=True) for x in after[key]}
        print(sha, key, 'added', len(b - a), 'removed', len(a - b))
PY
```

This is a bounded sample of changes reusing existing seams, not a representative
mean for new owners/capabilities. No author-time measurement exists. ADR 0080
records #4617's cross-stack file cost; zero policy edits is not zero wiring cost.

## Small repair evidence

`TestHermeticSetupScanIgnoresProseButRequiresRealSetup` fails with the base
`checkMissingSetupTestDB`: `prose_test.go` is spuriously included alongside
three genuine DB-code fixtures. The AST repair ignores prose but still reports
query calls, `bun.DB` selectors and `repositories` selectors without setup.
The same regression test passes after the repair, as does
`go test -tags=testhooks ./test/ -run 'Ratchet|Hermetic' -count=1`.
Two obsolete file-specific exemptions are removed; setup/mock exemption
semantics elsewhere are not broadened. The historical #4082 filename repair
and shrink-only comment repair change no executable contract.
