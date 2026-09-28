# Checkpoint workload `checkpoint-1-v2`: bridge run (#3411)

[#3411](https://github.com/moto-nrw/project-phoenix/issues/3411) replaces the
default checkpoint workload before
[#3021](https://github.com/moto-nrw/project-phoenix/issues/3021). The workload
contract is in [README.md](README.md#workload-checkpoint-1-v2). This page holds
the bridge run: `checkpoint-1-v1` and `checkpoint-1-v2` on one commit, and the
comparison of that `checkpoint-1-v1` run with #3019 and #3020. It measures; it
does not accept. The go/no-go stays with #3021.

## Source and environment

- Harness commit: `423c29f473008add9db415e42fb26a12b6ba8c3f`, measured twice
  in fresh processes one after the other, `checkpoint-1-v1` first.
- Command: the one in the README, `GOMAXPROCS=4`, `-parallel 8`, with
  `-runtime-checkpoint-workload=checkpoint-1-v1` for the first run.
- go1.27.0 darwin/arm64, 16 logical CPUs, PostgreSQL 17.11 in the local
  postgres-test container, role `phoenix_auth`, pool of 12 connections.
- In-process `Runtime.Handler`, no TCP or TLS; production mock mailer.

## Raw evidence

Lossless raw JSON, xz-compressed. `scripts/runtime-checkpoint-report.py` reads
the `.xz` files directly.

| File | Raw JSON SHA-256 | `.xz` SHA-256 |
|---|---|---|
| [checkpoint-1-v2-3411.bridge-v1.raw.json.xz](checkpoint-1-v2-3411.bridge-v1.raw.json.xz) | `f116ba32ed6b4fe138751f302432444b6b4badf54c30fcdb0299e07a3cbc76e7` | `f973a9e956f617eecc092daabee5080f3074cba7bf37c98052ce8076629b6fca` |
| [checkpoint-1-v2-3411.bridge-v2.raw.json.xz](checkpoint-1-v2-3411.bridge-v2.raw.json.xz) | `46f1d98f1b83841ae010f7b1291754018b3eb8da5859180cb8e8991c13f90bd2` | `236c9645584c1c3a1bd1f5473d16188e9a5e9816f988f270b6824e1be1a0502a` |

Generated tables: [v2 results](checkpoint-1-v2-3411.results.md),
[bridge comparison](checkpoint-1-v2-3411.bridge-comparison.md),
[v1 against #3019](checkpoint-1-v2-3411.vs-3019.md) and
[v1 against #3020](checkpoint-1-v2-3411.vs-3020.md). The #3019 and #3020 raw
files were decoded from their issue comments; their SHA-256 values match the
ones those comments state (`f37f0bcf…c9df2f`, `4cf06c6e…868370`).

## Bridge: `checkpoint-1-v1` → `checkpoint-1-v2` on one commit

Compared over the 25 shared scenarios (20 HTTP, 5 worker) with
`runtime-checkpoint-compare.py --bridge`:

| Classification | Count |
|---|---:|
| unchanged | 889 |
| within tolerance | 106 |
| material | 3 |
| coverage | 100 |

**Per-metric relationship.** Every exact-invariant metric is identical:
statements, driver rows, write rows, statuses, stable errors, pool waits,
lock observations, deadlocks, retries and rollbacks, and the worker job
counters. Both runs read the same fixture volume in the checkpoint tenant
(`fixture_rows` is equal); the v2 additions live in a second tenant. The three
material entries are latencies only: `communication.messages` p50 and p95
(faster in v2) and `settings.schema` worst p95 (faster in v2). There is no
direction to the latency differences: of the 20 HTTP p50 medians, 12 are
higher in v2 and 8 lower, median difference +0.015 ms.

An earlier bridge on the previous harness commit showed a one-sided shift
(17 of 20 p50s higher, about +0.1 ms): the v2 harness rebuilt its scenario
list inside the timed window of every request. `423c29f473` moves that out of
the window; the run above is on that commit.

Consequence for later checkpoints: the first 20 HTTP scenarios and the 5
worker scenarios of a `checkpoint-1-v2` report compare with #3019 and #3020
as a `checkpoint-1-v1` report would. The 35 new scenarios and the contention
runs have no earlier baseline; this run is their first measurement.
`runtime-checkpoint-compare.py` classifies scenario metrics only; the
`concurrent` and `jsonb_recordsets` sections of two v2 summaries have to be
compared by reading them.

## `checkpoint-1-v1` on this commit against #3019 and #3020

| Classification | against #3019 | against #3020 |
|---|---:|---:|
| unchanged | 784 | 824 |
| within tolerance | 40 | 55 |
| material | 134 | 119 |
| coverage | 100 | 100 |

The workload is the same, so these differences are the code that merged after
the measured commits. No stable error contract changed. The invariant
changes against #3020, per request:

| Scenario | Statements | Other |
|---|---|---|
| `school-calendar.periods` | 8 → 7 | module rows 30 → 60 per run |
| `school-membership.staff` | 14 → 15 | driver rows 810 → 1170 per run |
| `delivery.provider-unavailable` | 26 → 34 | |
| `delivery.render-failure` | 24 → 32 | |
| `delivery.idle` | 12 → 20 | |
| `timetable.materialize-create` | 16 → 19 | driver rows 240 → 270 per run |
| `timetable.materialize-existing` | 15 → 17 | module rows 150 → 120 per run |

Against #3019 the same list applies, except `school-calendar.periods`, which
went 6 → 8 at #3020 and is 7 now. The remaining material entries are latency.
Explaining these changes is #3021's job; they are listed here so the bridge
does not hide them.

## New scenarios (serial, concurrency 1)

Median of three runs. Statements per request come from the query counter on
the `phoenix_auth` pool.

| Scenario | Status | p50 ms | p95 ms | Statements per request | Stable error |
|---|---:|---:|---:|---:|---|
| `identity-access.login` | 200 | 8.42 | 10.28 | 54 |  |
| `identity-access.login-wrong-password` | 401 | 2.16 | 2.44 | 11 | `invalid username or password` |
| `identity-access.refresh` | 200 | 4.30 | 4.67 | 27 |  |
| `identity-access.refresh-invalid-token` | 401 | 0.02 | 0.03 | 0 | `token unauthorized` |
| `identity-access.mfa-verify` | 200 | 8.08 | 8.76 | 52 |  |
| `identity-access.mfa-verify-wrong-code` | 401 | 3.01 | 3.49 | 18 | `invalid or expired code` |
| `identity-access.passkey-login-options` | 200 | 1.21 | 1.32 | 9 |  |
| `identity-access.passkey-login-options-wrong-origin` | 401 | 1.00 | 1.11 | 8 | `passkey origin is invalid` |
| `device-scan.checkin` | 200 | 8.75 | 9.96 | 37–50 |  |
| `device-scan.checkin-invalid-key` | 401 | 0.18 | 0.23 | 1 | `invalid device API key` |
| `device-scan.pickup-query` | 200 | 3.84 | 4.37 | 23 |  |
| `device-scan.pickup-query-missing-pin` | 401 | 0.65 | 0.89 | 5 | `staff PIN is required` |
| `device-scan.status` | 200 | 2.00 | 2.30 | 15 |  |
| `device-scan.status-wrong-pin` | 401 | 1.36 | 1.58 | 10 | `invalid staff PIN` |
| `device-scan.rfid-lookup` | 200 | 2.34 | 2.52 | 16 |  |
| `device-scan.rfid-lookup-malformed-key` | 401 | 0.02 | 0.03 | 0 | `invalid API key format - use Bearer token` |
| `device-scan.staff-clock` | 200 | 6.16 | 6.96 | 33–37 |  |
| `device-scan.staff-clock-wrong-pin` | 401 | 1.42 | 1.58 | 10 | `invalid staff PIN` |
| `people-directory.students` | 200 | 7.69 | 8.79 | 32 (29 in run 2) |  |
| `people-directory.students-invalid-view` | 400 | 0.57 | 0.72 | 4 | `invalid view "bogus": …` |
| `request-review.queue` | 200 | 2.12 | 2.23 | 12 |  |
| `request-review.queue-invalid-view` | 400 | 0.49 | 0.71 | 4 | `invalid change request list query` |
| `request-review.care-schedule-decide` | 200 | 21.02 | 23.30 | 100 |  |
| `request-review.care-schedule-decide-not-found` | 404 | 0.81 | 1.13 | 6 | `schedule: care schedule change request not found` |
| `care-plan.withdrawals` | 200 | 1.84 | 2.04 | 6 |  |
| `care-plan.withdrawals-invalid-state` | 400 | 0.59 | 0.70 | 4 | `state must be pending or resolved` |
| `student-deletion.withdrawal-impact` | 200 | 5.92 | 6.25 | 26 |  |
| `student-deletion.withdrawal-impact-not-found` | 404 | 0.76 | 0.91 | 5 | `students.care_withdrawal_not_found` |
| `care-plan.care-end-preview` | 200 | 4.43 | 5.49 | 20 |  |
| `care-plan.care-end-preview-past-day` | 400 | 0.58 | 0.84 | 4 | day in the past |
| `care-plan.care-end` | 200 | 15.55 | 17.54 | 64 |  |
| `care-plan.care-end-replan` | 200 | 16.05 | 17.49 | 64 |  |
| `care-plan.care-end-stale-token` | 409 | 9.83 | 12.34 | 39 | preview changed |
| `student-deletion.execute` | 200 | 17.15 | 18.67 | 64 |  |
| `student-deletion.execute-stale-preview` | 409 | 14.37 | 14.81 | 55 | `students.deletion_preview_changed` |

The kiosk check-in alternates between check-in and check-out, and the
device's `last_seen` flush is debounced onto a later request, so its statement
count varies within a run and its latency mixes two operations. The staff
clock alternates the same way. `people-directory.students` issues 32
statements per request in runs 1 and 3 and 29 in run 2, the same pattern in
both bridge runs and in the earlier one; it is stable within a run, and its
cause is not identified here. A v2-to-v2 comparison will see it as material
unless both sides show the same pattern.

The final database state matches the statuses: 105 approved decisions, 210
ended cares, 105 deletions, 105 staff-clock transitions, and the child behind
the stale deletion still exists.

## Contention run (concurrency 16)

16 requests per round against the pool of 12, three runs of 30 measured
rounds. Slot 0 deletes the subject through `deleteConfirmed` →
`lockedSnapshot`, slot 1 deletes a companion whose graph contains the subject,
the other 14 slots write and read the same three children.

| Metric | Median | Worst |
|---|---:|---:|
| round wall p50 ms | 61.22 | 61.68 |
| round wall p95 ms | 76.23 | 77.28 |
| pool waits | 120 | 120 |
| pool wait ms | 1238.3 | 1246.2 |
| rounds with a pool wait | 30 | 30 |
| backends observed waiting on a lock (sampled) | 2904 | 3014 |
| most backends waiting at once | 8 | 9 |
| lock samples | 2455 | 2427 |
| deadlocks | 0 | 0 |
| unit-of-work retries | 0 | 0 |
| unit-of-work rollbacks | 30 | 38 |

| Operation | Requests per run | p50 ms per run | p95 ms per run | Statuses per run |
|---|---:|---|---|---|
| `student-deletion.execute-subject` | 30 | 48.2 / 47.1 / 48.9 | 61.1 / 58.5 / 65.3 | 200: 2, 409: 28 / 409: 30 / 409: 30 |
| `student-deletion.execute-companion` | 30 | 37.9 / 33.8 / 37.7 | 74.0 / 68.1 / 77.0 | 200: 28, 409: 2 / 200: 30 / 200: 30 |
| `people-directory.update-subject` | 120 | 41.0 / 48.1 / 40.2 | 64.3 / 67.3 / 64.2 | 200: 112, 404: 8 / 200: 120 / 200: 120 |
| `people-directory.update-second-companion` | 120 | 23.0 / 22.6 / 22.0 | 33.7 / 32.9 / 33.0 | 200: 120 |
| `student-deletion.preview-companion` | 90 | 9.7 / 9.7 / 10.2 | 18.1 / 18.6 / 19.2 | 200: 90 |
| `people-directory.students` | 90 | 14.4 / 14.7 / 14.7 | 23.4 / 22.1 / 23.6 | 200: 90 |

What this shows, and what it does not:

- **Pool.** Four of 16 requests per round wait for a connection, every round.
  That count follows from 16 requests against 12 connections, not from the
  code; the wait time is what they pay, about 41 ms per round across the four.
- **Locks.** Up to nine backends wait on locks at once. The sampler counts
  every `phoenix_auth` backend waiting on a lock; it does not attribute a wait
  to the deletion path. The subject's deletion ends in 409
  (`deletion_preview_changed`) in 88 of 90 rounds, after taking its locks and
  re-reading the counts; a subject update or the companion's deletion
  committing first both produce that answer, and the raw data does not tell
  the two apart. The companion's deletion succeeds in 88 of 90 rounds. The
  locked deletion runs 47 to 49 ms at the median under this load, about three
  times its serial 17 ms.
- **Deadlocks and retries.** No deadlock occurred in 90 rounds with lock
  queues up to nine deep. The retry count cannot be anything but zero: the
  deletion runs through `RunInTx` inside the request transaction, and HTTP
  transactions never retry. A deadlock would therefore have ended as an error
  status outside the allowed set and failed the run, not as a retry. The
  rollbacks are the 409 and 404 answers above.

The exit criterion of #3411 asks for non-trivial deadlock and
serialization-retry numbers for the locked deletion path. On this
architecture those numbers are zero by measurement (deadlocks) and zero by
construction (retries); the evidence is that real lock queues formed around
the deletion and no deadlock followed.

## `jsonb_to_recordset` set sizes

| Scenario | Call site | Calls per run | Rows per call |
|---|---|---:|---:|
| `care-plan.care-end-preview` | `CountRunningEnrollmentsForCareExit` (`modules/timetable/internal/adapters/postgres/care_exit_baseline.go`) | 30 | 0 |
| `care-plan.care-end` | the same count | 30 | 0 |
| `care-plan.care-end` | `restoreCappedEnrollments` and `restoreDeletedEnrollments` (`modules/timetable/internal/adapters/postgres/store.go`), one key for both | 60 | 0 |
| `care-plan.care-end-replan` | the same count | 30 | 3 |
| `care-plan.care-end-replan` | the same two restores | 60 | 3 |
| `care-plan.care-end-stale-token` | the same count | 30 | 0 |

The set is the earlier exit's removal list of the selected children: empty for
a first exit, three rows when the re-planned child's first exit removed its
three bookings. The code builds it from the selected children's removals, so
it should grow with the selection and its bookings rather than with the
tenant; the tenant size was not varied, so that is read from the code, not
measured. The two restore statements share one call-site key because their
text after the literal is identical up to the line end.

The other call sites listed in #3411 (Care Plan withdrawal completions, the
users care-exit cleanup, audit booking consistency, the Enrollment phase
expiry and care-exit offerings, the Communication parent audience, the
Timetable projection and roster care-exit paths, room utilization, session
attendance, school account counts) are not reached by any scenario of this
workload: the care ends here lie in the future, so their cleanup does not run
inside the request. Their sizes remain unmeasured, not bounded.

## Deletion workflow query budget

`TestStudentDeletionWorkflow_QueryBudget`
(`backend/services/users/student_deletion_query_budget_test.go`) runs the
production owner composition with one and with four linked children. The
budgets in `backend/test/query_budgets.go`:

| Scenario | 1 companion | 4 companions |
|---|---:|---:|
| `Preview` (no locks) | 22 | 22 |
| `Execute` (`deleteConfirmed` → `lockedSnapshot`) | 61 | 70 |
| of which student-row reads, 2(K+1) | 4 | 10 |

Each linked child costs three statements: its locked read with the shared
class-writes gate, and its unlocked re-read in the stranding check. The
doubled owner-count block is 2 × 16 statements, fixed in K. The scenario
replaces the Feedback counter, the authorization and the staff check with test
doubles, so their statements are not part of these numbers.

## Limits

- One machine, in-process HTTP, PostgreSQL in a local container. Latencies are
  comparable between runs on this setup, not with production.
- Lock waits are sampled every 2 ms from `pg_stat_activity`; they are
  observations, not durations, and zero deadlocks is an observation, not a
  proof.
- The contention mix is fixed. Other writer mixes (grade transitions, kiosk
  scans of the same children) are not measured.
- The care-end scenarios use the fixed last care day `2027-07-30`, and the
  enrollment phase fixture ends on 2027-07-31. After that day the preview
  refuses the day as past and the v2 run fails in warmup; the date has to move
  with the fixture before then.
- Set sizes are read from inline literals only; a call site whose argument is
  not inline would count as `unparsed` (none was). A v2 result that reached no
  call site omits `jsonb_recordsets`, like a result recorded before #3411.
