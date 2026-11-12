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
   separately. Runner noise is large; one run is not a measurement. Done when
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
  detection reads.
- **Cache what downloads, not what links.** Cache toolchains and browsers by
  version. t3code measured a pnpm store on a sticky disk slower than a cache
  tarball (pnpm copies across filesystems instead of hard-linking).
- **Cached test results need verification.** t3code caches per test file and
  reruns one hit per shard uncached. Here `go test -count=1` stays: database
  tests depend on state outside Go's cache key.

## Measured here (11/2026)

- Blacksmith x64: $0.004 per minute per 2 vCPU. GitHub-hosted runners cost
  $0.006 per minute for 2 cores once the repository is private; keep jobs on
  Blacksmith.
- Sticky disks (`build.yml` Docker layer cache) bill bytes ever written;
  `sticky-disk-cleanup.yml` deletes them nightly.
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
