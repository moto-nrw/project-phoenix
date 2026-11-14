---
name: ci-cost
description: Use when changing CI workflows, runners, job splits or shards, CI caches or checkouts, or when the CI bill or a slow pull request run needs investigating. Measures billed vCPU-minutes before and after and applies the cost patterns proven in moto's and t3code's CI.
metadata:
  author: moto-ogs
  version: "1.0.0"
---

# CI cost

The unit is **vCPU-minutes**: billed minutes per job (rounded up per job)
times the runner's vCPUs. Wall-clock time matters only on the critical path,
the job that finishes last. The binding rules are in
[ci-test-performance](../../../.claude/rules/ci-test-performance.md); this
skill is how to measure and decide.

## Steps

1. **Baseline.** For each change type the edit touches (backend PR, frontend
   PR, full suite), take at least two recent successful runs of `main.yml`
   with warm caches. Record per job: runner, duration, billed minutes, cost,
   and the split into setup steps and work steps (commands below). Done when
   a table shows billed vCPU-minutes and dollars per run for each type.
2. **Locate the cost.** Mark the critical-path job of each type. For every
   Blacksmith job, note setup seconds, work seconds and whether its work keeps
   the runner's cores busy. Done when every paid job is classified as setup-
   bound, work-bound or idle-core.
3. **Choose** from the patterns below. Prefer removing or merging a job over
   resizing it; prefer fewer setups over faster ones.
4. **Prove on the PR.** A change to `main.yml` or `test.yml` runs the full test
   suites; `main.yml` or `lint.yml` runs all lint jobs (`detect-changes` in
   `main.yml`). A changed cache key makes the first run cold: rerun the job
   once (`gh run rerun <run> --job <job>`) and report warm and cold
   separately. Runner noise is large; one run is not a measurement. A cache
   only development writes is cold in every run of the PR that changes its
   key or paths: a temporary commit that saves it from the PR makes reruns
   warm. A rerun keeps the workflow of its run's commit, so rerunning two
   variant runs in turn compares them on the same cache (#7032). Done when
   a before/after table covers every change type the edit affects.
5. **Write the PR** with that table (vCPU-minutes, dollars, critical-path
   wall-clock) and the options you measured and rejected, with numbers.

## Patterns

Proven here (#5700, #5762, #5770) or in pingdotgg/t3code (#14025, #19587,
#20044, #24966):

- **Setup is the cost.** After skipping unaffected jobs, setup was about half
  of every t3code job. Backend shards here paid 40–60 s of services, checkout,
  history and Go setup for 7 s of tests. Merge jobs that share an install;
  start independent installs (apt, browsers) in the background of the job
  that needs them and wait for them in a later step.
- **Shard only idle cores.** t3code shards serial server tests (about
  1.5 cores busy) over 4-vCPU runners at the same vCPU-minutes. A suite that
  saturates its runner gains wall-clock and pays setup per shard. Balance
  shards by measured duration, not by file count.
- **Size by cores used.** Same price per vCPU-minute on every size: a larger
  runner pays only where the work scales with cores. `frontend-build` costs a
  third less on 4 vCPU (4 billed minutes at half the rate instead of 3).
- **Skip what a change cannot affect, fail open.** Unknown paths, workflow and
  lockfile changes run everything; protected-branch pushes and nightly run full
  as the backstop. The gate job (`ci-gate`) is the only required check, so
  renamed or merged jobs need no ruleset change.
- **Small jobs on 2 vCPU**, every job with `timeout-minutes`, superseded pull
  request runs cancelled.
- **Cheap checkouts.** `fetch-depth: 1` with `filter: blob:none` unless the
  job reads history or blobs; t3code checks out only the manifests its change
  detection reads. A comparison with one fixed commit needs only that commit:
  `git fetch --depth=1 origin <sha>`. Only merge-base and ancestry checks need
  history; the architecture ratchet's fetch took 30–45 s with `--unshallow`
  and 0–1 s flat (#7032).
- **Cache what downloads, not what links.** Cache toolchains and browsers by
  version. t3code measured a pnpm store on a sticky disk slower than a cache
  tarball (pnpm copies across filesystems instead of hard-linking).
- **Bound a cache that restores by prefix and saves again.** Go keeps an
  entry five unused days, so such a cache keeps every older build: the
  backend build cache grew from 2.2 to 9.65 GB in twelve hours and its
  restore from 15 to 75 s (#6784). `scripts/go-cache-sweep.sh` keeps the
  entries one run uses.
- **One writer on development.** A cache saved by a pull request serves
  that pull request alone; save shared entries on development pushes (the
  default branch every pull request reads) and let pull requests only
  restore: `backend-build-cache.yml` (#6784, architecture ratchet #6962),
  `frontend-bundle-base.yml` (#7023). Writer and readers need the same
  runner, env and key, so a test ties the two jobs together
  (`scripts/architecture-build-cache.test.mjs`).
- **`-trimpath` for a tree in a temporary directory.** Without it the
  directory enters every Go action ID: the architecture ratchet extracted its
  base into a new directory per run and compiled it cold every time (#6962).
- **Cached test results need verification.** t3code caches per test file and
  reruns one hit per shard uncached. Here `go test -count=1` stays: database
  tests depend on state outside Go's cache key.

## Measured here (11/2026)

- Blacksmith x64: $0.004 per minute per 2 vCPU. GitHub-hosted runners cost
  $0.006 per minute for 2 cores once the repository is private; keep jobs on
  Blacksmith.
- Sticky disks (`build.yml` Docker layer cache) bill bytes ever written;
  `sticky-disk-cleanup.yml` deletes them nightly.
- Service containers start before the first step: 9-11 s in the seed
  smoke. Started in the background of its first step, they cost nothing
  visible; the image pulls slow checkout and cache restore by 1-5 s, because
  hosted setup shares one network link (#6985). setup-go's cache beside an own
  Go cache loads the build cache twice: `cache: false` with the modules in the
  own cache saved 4-8 s, in the architecture ratchet 10 s (#7032).
- The seed smoke stalled on Blacksmith once (run 38232330367) and runs on a
  fresh hosted VM; test it repeatedly before moving it.
- Hosted runners come with three CPUs (14.11.): Xeon 6975P-C, EPYC 9V74 and
  EPYC 7763. One school seed takes 52-58 s, 67-69 s or 77-86 s on them, so
  a hosted job spreads by a factor of 1.5 without any change. Compare
  hosted jobs per CPU (`lscpu`), or take at least six.
- A seed that looks bimodal may be a query plan, not the runner. Autovacuum
  analyzes a database once a minute; until then the planner took freshly
  filled tables for one row and joined in nested loops (74-107 statements of
  0.7-1.8 s per school). The first seed of "isolation" took 10-45 s longer
  in 30 of 56 runs, and in 5 of 11 local seeds on one machine. With
  `autovacuum_naptime = 5s` in the smoke's Postgres: 0-2 such statements,
  local seeds 51-55 s against 54-94 s, the longest of six jobs 257 s
  against up to 296 s (#6962). Find such a case with
  `log_min_duration_statement` and the server log of a slow and a fast
  run, then `auto_explain` locally. The demo server and production keep the
  default minute.
- The nightly race run keeps four backend shards (`SPLIT_SUITES`): race
  builds allow two packages per runner.
- The backend test job saturates its 8 vCPU (about 5 % idle, load 23–37), so
  its wall time follows total CPU work. A cold Go build cache cost 260 s of
  the 574 s test step on the same commit (#6784); before #6784 every pull
  request compiled cold, because pull requests saved 2 GB caches no other
  pull request could read and the repository sat above its 10 GB limit.
  Check a slow job's "Restore backend build cache" line first.
- Package wall times under that load are 2–3 times a local run: `importtest`
  136 s locally, 245–389 s in the full suite. Starting the long packages first
  gained nothing (305 s against 311 s) and slowed them, because they then ran
  at the same time.
- Package budgets (`scripts/backend-package-budgets.json`) come from 26
  backend jobs after #7038 (14.11.): `api` ran 91–106 s in full suites
  on 8 vCPU but up to 140 s in selections there and 155 s in two full
  suites on 4 vCPU. Measure every combination; the maximum rarely sits in
  the full 8-vCPU suite.
- `ci-runtime-watch.yml` reports every Monday the median of the week's
  green pull request runs (all, with backend tests, the backend job, the
  jobs that end last) and opens one issue above 300 s.
- "Backend tests" writes a "CPU seconds" table into its job summary (build,
  vet, link, Postgres, test binaries). Compare shares of the busy seconds, not
  job durations: the same full suite took 1431 to 2183 busy CPU seconds and
  193 to 295 s on six runners of one size (13.11.).
- In a full suite with a warm build cache Postgres takes about half of the
  busy CPU seconds, the test binaries a third, linking a seventh. Cloning
  and dropping databases is a few seconds of that; the rest is fixture work,
  about 920,000 SELECT and 313,000 INSERT statements. The local server's
  `fsync=off`, `full_page_writes=off` and `wal_level=minimal` changed nothing
  on the job's tmpfs.
- A test that runs `go list -export` or `packages.Load` in a child process
  with its own environment (`internal/architecture`: `CGO_ENABLED=0`, policy
  tags) compiles actions the warm job's `go test -exec true ./...` never
  touches. Only running that test in the warm job, before the sweep, puts them
  in the cache: locally 225 CPU s cold against 81 one commit later. In CI the
  restored cache already held two thirds of them (199 MB new): the package
  took 21-28 s warm against 40-41 s, the job's busy CPU did not move (#7038).
  With `-trimpath`, temporary directories do not change the action IDs.
- Postgres stores SCRAM verifiers with 4096 iterations, paid by server and
  client on every login: 11.3 ms CPU per connection against 3.1 ms with
  `initdb --set scram_iterations=1` (pgbench -C). The setting covers the
  superuser and every role created later. Full suite, same branch: 1153-1280
  busy CPU s against 1292-1440 (#7038).
- A fault matrix with a private pool per cell pays new sessions per cell.
  One shared pool per matrix with a handle per cell (`testpkg.CellPool`),
  local, CI flags, three alternating runs (14.11.): `database/migrations`
  3660 against 1115 sessions, Postgres 94 against 71 CPU s, binary 13.7
  against 9.3 CPU s, wall 26 against 23 s; `dataimport/importtest` 1230
  against 611 sessions, Postgres and wall within noise. The meal-write matrix
  in `api` has nothing to gain: 1517 of its 1534 connections replace ones
  its `bad_conn` and 57014 cells break on purpose. Count connections before
  sharing a pool.
- A typical feature pull request compiles more than it tests: `api` 21 times
  and `roots/owners` 7 times, 60 % of its compile CPU (#6914). `GOGC=400`
  for the toolchain alone saved 13 % of the build CPU locally; `-exec "env -u
  GOGC"` keeps it away from the test binaries.
- `GOGC=200` for the test binaries, rejected (#7045): locally `api` used 19 %
  less CPU, but in three full suites per variant busy CPU did not fall (1151 to
  1241 s without, 1171 to 1292 s with; 1146 to 1571 s without and 1146 to
  1215 s with, both with `-vet=off`), and peak memory rose 0.6-1.0 GiB. The
  job summary prints peak memory in use (MemTotal minus MemAvailable, tmpfs
  included): 5.8-7.3 of 30.9 GiB on 8 vCPU. The full suite on 4 vCPU, an
  upper bound for the small runner's selections, peaked at 5.7-6.2 of
  15.4 GiB, with `GOGC=200` at 7.2.
- `go test` vets with `-vet=off` here (#7045): golangci-lint's default govet
  set holds every analyzer of go test's subset, and lint selects the same
  packages. Its default `exclusions.generated: strict` dropped findings in
  generated files, which go test vets; `generated: disable` closes that and
  found nothing. vet cost 17-55 CPU s (about 2 % of busy) in pull requests
  with Go changes and 0-12 s in full suites of an unchanged tree, whose vet
  results come from the warmed build cache.
- A job in front of the job that sets the wall-clock time costs its duration
  plus a runner start: "Select backend tests" took 40–67 s plus 6–15 s. The
  long job selects for itself, as t3code's does.
- Blacksmith jobs keep `actions/cache` entries in Blacksmith's own store,
  with GitHub's branch scoping: they are missing from `gh cache list`, do not
  count against the 10 GB, and a GitHub-hosted job cannot read them. That
  store holds 25 GB and evicts entries unused for seven days; `gh cache
  delete` cannot reach them, so bound them with `scripts/go-cache-sweep.sh`.
- Two cold `next experimental-analyze` builds of one tree differed by 1-4 B
  on 256 of 257 routes in 16 of 33 pairs (13.11.), identical otherwise;
  the ratchet tolerance is 2 % or 10 KiB.
- Two cold Next.js builds started side by side on four cores took as long as
  one after the other (207 s against 210 s).
- The architecture ratchet's projections (25–33 s) beside its check on four
  vCPU, alternating reruns with a warm cache: 43–76 s against 59–86 s one
  after the other. The check itself slowed down, the pair still gained 7–16 s
  (#7032).
- The script suite "restore" took 303–313 s on a hosted runner, 232 s of it
  the encrypted daily backup restore test; a change to
  `scripts/posthog-dashboards.mjs` alone waited for it. Pull requests now
  start it only for its own inputs (`restore-suite` filter, #6962).
- Script suites on hosted runners cost nothing, so one that sets the
  wall-clock time is split, not sped up: "ratchets" ran 233-271 s as fetch,
  Go ratchets and script self-tests in series and is now "ratchets" and
  "selection"; the release restore (41-43 s) left "restore" (230-281 s) as
  "release-restore". Split suites start on the same condition
  (`scripts/backend-affected-packages_test.sh`).
- History for a merge base needs no file contents: `git fetch --unshallow
  --filter=blob:none` took 13 s against 58 s locally and gave the same
  merge base; git loads a base file on demand (two reads 2.8 s). A job that
  reads many base files one by one pays a fetch per file.
- A pull request run with the full suites cost about $0.18–0.27 after #5770
  (check "CI cost"); the backend tests alone, warm, about $0.05.

## Commands

```bash
R=moto-ogs/project-phoenix
# Runs of one workflow in a window
gh api "repos/$R/actions/workflows/main.yml/runs?created=2026-11-01..2026-11-05&per_page=100" \
  --paginate -q '.workflow_runs[]|select(.conclusion=="success")|.id'
# Jobs of a run: seconds, runner, name
gh api "repos/$R/actions/runs/$RUN/jobs?per_page=100&filter=latest" -q '.jobs[]
  | select(.started_at and .completed_at)
  | "\((.completed_at|fromdate)-(.started_at|fromdate))s \(.labels[0]) \(.name)"'
# Steps of one job (setup versus work)
gh api "repos/$R/actions/jobs/$JOB" -q '.steps[]|select(.started_at and .completed_at)
  | "\((.completed_at|fromdate)-(.started_at|fromdate))s \(.name)"'
# Package lines of one job; logs are about 4 MB, stream them
gh api --allow-escape-sequences "repos/$R/actions/jobs/$JOB/logs" \
  | sed 's/\x1b\[[0-9;]*m//g' | grep -aE '(✓|✖) +\S+ \('
# Weekly runtime report without writing an issue
GITHUB_REPOSITORY=$R GITHUB_TOKEN=$(gh auth token) node scripts/ci-runtime-watch.mjs --dry-run
# Cost of a pull request's runs
gh api "repos/$R/commits/$SHA/check-runs?check_name=CI%20cost" -q '.check_runs[0].output.summary'
```

Billed cost of a job: `ceil(seconds / 60) × vCPU / 2 × 0.004` dollars.
