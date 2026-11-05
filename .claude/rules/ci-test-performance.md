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

- A new job, matrix shard or larger runner states its measured minutes and
  cost per run in the PR, and the critical-path job it runs beside.
- Split work across jobs only when one runner leaves cores idle (serial
  tests) or the job sets the pull request's wall-clock time. Every shard pays
  the setup again; the PR shows vCPU-minutes before and after.
- Every job sets `timeout-minutes` at about three times its measured maximum.
- A new job gets path filters through `detect-changes` in `main.yml` and runs
  only on non-draft pull requests.

## Pushes

Every push to a ready pull request starts a full CI run. Run
`scripts/run-go-toolchain.sh scripts/check-changed.mjs` before each push, put
all fixes for one review round in one push, and keep a pull request draft
while you still expect follow-up pushes.
