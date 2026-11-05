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

- Standard GitHub-hosted runners (`ubuntu-latest`, 4 vCPU, 16 GB) are free for
  this public repository. Blacksmith bills every job, rounded up to whole
  minutes, at the same price per vCPU-minute on every size.
- A bigger Blacksmith runner therefore saves money only on the serial part of
  a job (checkout, toolchain, install). Size it for the critical path, not to
  make a side job faster.
- The GitHub cache (10 GB) is full. A job moved to a hosted runner must not save
  a per-commit cache from pull requests; restore on pull requests and save
  from nightly/`development`, as `lint.yml` does for `backend-quality`.
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
- Prefer a hosted runner for a job that is not on the critical path and saves
  no per-commit cache. Keep Blacksmith for critical-path jobs and for jobs
  whose build caches need Blacksmith's cache.
- A new job gets path filters through `detect-changes` in `main.yml` and runs
  only on non-draft pull requests.

## Pushes

Every push to a ready pull request starts a full CI run. Run
`scripts/run-go-toolchain.sh scripts/check-changed.mjs` before each push, put
all fixes for one review round in one push, and keep a pull request draft
while you still expect follow-up pushes.
