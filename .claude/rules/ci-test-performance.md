---
paths:
  - ".github/workflows/**"
  - ".github/actions/**"
  - "scripts/backend-test-matrix*"
  - "scripts/test-*"
  - "backend/**/*_test.go"
  - "backend/test/**"
  - "backend/internal/testdb/**"
  - "frontend/**/*.test.*"
  - "frontend/src/test/**"
  - "frontend/vitest.config.ts"
---

# Keep tests and CI cheap

Every pull request push runs CI on paid Blacksmith runners, and this repo
produces about a thousand pull requests a month. A few CPU seconds per test
or a minute per job is real money at that volume. Treat CPU time, RAM and
billed runner minutes as a budget a change has to justify, like a bundle
delta.

## Cost model

Verify prices before relying on them; `.github/actions/ci-cost/ci-cost.mjs`
holds the checked list prices.

- Put CI jobs on Blacksmith. In a private repository GitHub-hosted runners
  bill too, at a higher price per vCPU ($0.006/min for 2 cores against
  Blacksmith's $0.004/min for 2 vCPU).
- Blacksmith bills every job, rounded up to whole minutes, at the same price
  per vCPU-minute on every size. A bigger runner saves money only on the
  serial part of a job (checkout, toolchain, install), and a job of a few
  seconds still bills a full minute. Size a job for the critical path, not to
  make a side job faster, and fold tiny jobs into a neighbouring one.
- `scripts/test-perf.mjs` measures local CPU-seconds, peak RAM and wall time;
  the "CI cost" check run shows billed minutes and dollars per pull request.

## Runtime targets

A slow run costs money on every push and blocks every other pull request
waiting for CI. Wall-clock time of a pull request run with warm caches, from
push to `ci-gate`:

| Scope | Target | Act at |
|---|---|---|
| Whole pull request run | 3–5 min | over 5 min |
| `Test / Backend tests` job | ≤ 4 min | over 5 min |
| One Go package in that job | ≤ 60 s | over 2 min |
| Any other test job or shard | ≤ 3 min | over 4 min |

A per-package budget checked by CI (#6784) is the source of truth for that
row once it exists. Runs above target are debt to pay down, never headroom
for more tests. The isolated-database deadline (10 min Go default minus
`isolatedTestDatabaseDeadlineReserve`, about 7 min 25 s) is the failure
point, not a budget.

**Before and after.** A change that adds or changes tests, or the code they
exercise, records the duration of each affected Go package or job on the base and on the branch:
the gotestsum line `✓ <package> (<duration>)` in the CI log, or a local run
with CI flags (`../scripts/run-go-toolchain.sh go test -tags=testhooks
-count=1 -parallel 8 ./<pkg>`) alternated on base and branch on one machine.
The PR states both when a package grows by more than 15 s or a job by more
than 30 s.

**Too slow, in this order:**

1. Make it faster: shared setup, the package pool instead of an isolated
   database slot, fewer writes or uploads per case, `t.Parallel()`. Tests
   on an isolated clone queue for the package's two slots: their hold
   times add up to the package's wall time.
2. Question the form: thin the matrix to representative cases, merge cases,
   move the check to a unit test, or delete what another test covers. This is
   a test change the [test rule](no-test-modifications.md) allows.
3. Keep the go test `-timeout`, `isolatedTestDatabaseDeadlineReserve` and
   job `timeout-minutes` where they are; raising them makes a slow package
   pass without making it fast. The slot limit changes only with a
   suite measurement on base and branch.

**Lesson of #6784 (November 2026).** No single pull request went red; the
creep did. In four days the median of `api` grew from 39 s to 313 s,
`dataimport/importtest` from 4 s to 310 s, and the backend job from 244 s
to 504 s. Matrices merged without a runtime check drove it (#6510, #6514,
#6307: about 5000 uploads), pull requests compiled cold, and #6517
turned slow into red. The repair stalled when an
agent read the test rule as forbidding to thin those matrices.

## New or changed tests

- Test at the cheapest level that catches the defect: a pure function or unit
  test before a database or DOM integration test, one rendered state before a
  full user flow.
- Share expensive setup per package or file (`TestMain`, `beforeAll`, existing
  fixtures in `backend/test/` and `frontend/src/test/`) instead of repeating it
  per test.
- Wait for an observable condition (`findBy…`, `waitFor`, a channel), never a
  fixed sleep.
- Test infrastructure, Vitest/Go test config or shared test helpers: run
  `scripts/test-perf.mjs` (`full`, `backend-full`) before and after and put both
  results in the PR. A regression in CPU-seconds or peak RAM needs a reason.

## CI workflows and runners

Before changing a workflow, runner, job split, shard, cache or checkout, use
the `ci-cost` skill: it measures the baseline and holds the proven patterns.

- A new job, matrix shard or larger runner states its measured minutes and
  cost per run in the PR, and the critical-path job it runs beside.
- Setup (services, checkout, history, toolchain, install) is the main cost of
  a short job. Put new work into a job that already paid that setup before
  adding a job.
- Split work across jobs only when one runner leaves cores idle (serial
  tests) or the job sets the pull request's wall-clock time. Every shard pays
  the setup again; the PR shows vCPU-minutes before and after.
- Size a runner by the cores its work keeps busy.
- Change detection fails open: unknown paths run everything, and
  protected-branch pushes and the nightly run stay full.
- Every job sets `timeout-minutes` at about three times its measured maximum.
- The repository's Actions cache holds 10 GB; beyond that GitHub evicts the
  oldest entries, including the Go build cache. A new cache key states its
  size, checked with `gh cache list`.
- A new job gets path filters through `detect-changes` in `main.yml` and runs
  only on non-draft pull requests.
- Report CI timings from at least two warm runs; the first run after a cache
  key change is cold.
