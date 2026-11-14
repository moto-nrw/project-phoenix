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
  at the same time; `scripts/backend-package-budgets.json` holds each
  package's budget.
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
  in the cache: locally 225 CPU s cold against 81 one commit later; in the
  full suite the package took 21-28 s warm against 44-74 s (#7038). With
  `-trimpath`, temporary directories do not change the action IDs.
- Postgres stores SCRAM verifiers with 4096 iterations, paid by server and
  client on every login: 11.3 ms CPU per connection against 3.1 ms with
  `initdb --set scram_iterations=1` (pgbench -C). The setting covers the
  superuser and every role created later. Full suite, same branch: 1153-1280
  busy CPU s against 1292-1440 (#7038).
- A typical feature pull request compiles more than it tests: `api` 21 times
  and `roots/owners` 7 times, 60 % of its compile CPU (#6914). `GOGC=400`
  for the toolchain alone saved 13 % of the build CPU locally; `-exec "env -u
  GOGC"` keeps it away from the test binaries.
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
# Cost of a pull request's runs
gh api "repos/$R/commits/$SHA/check-runs?check_name=CI%20cost" -q '.check_runs[0].output.summary'
```

Billed cost of a job: `ceil(seconds / 60) × vCPU / 2 × 0.004` dollars.
