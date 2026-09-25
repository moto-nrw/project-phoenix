# Checkpoint workload `checkpoint-1-v2`: bridge run (#3411)

[#3411](https://github.com/moto-nrw/project-phoenix/issues/3411) replaces the
default checkpoint workload before
[#3021](https://github.com/moto-nrw/project-phoenix/issues/3021). The workload
contract is in [README.md](README.md#workload-checkpoint-1-v2). This page holds
the bridge run: `checkpoint-1-v1` and `checkpoint-1-v2` on one commit, and the
comparison of that `checkpoint-1-v1` run with #3019 and #3020. It measures; it
does not accept. The go/no-go stays with #3021.

## Source and environment

- Harness commit: `255d78a934b6d96f32e8caac045a2b864d3e1189`, measured twice in
  fresh processes one after the other, `checkpoint-1-v1` first.
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
| [checkpoint-1-v2-3411.bridge-v1.raw.json.xz](checkpoint-1-v2-3411.bridge-v1.raw.json.xz) | `7608573f14fc300a0521aa7ef01ed3cfe86cc5ed19c5090551115f710396a7b4` | `f36834d75eef366c743afcba65afba32412427970f860f945c20857800f8c9a9` |
| [checkpoint-1-v2-3411.bridge-v2.raw.json.xz](checkpoint-1-v2-3411.bridge-v2.raw.json.xz) | `873b983fa189424090de4648764928f64d616eaaed7ae0d8e87d7e2f3b27094a` | `365e78cb5fee10ad39302c4847f861e6c1910c7f11fe5dd5ad30b608928c0ca4` |

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
| unchanged | 888 |
| within tolerance | 102 |
| material | 8 |
| coverage | 100 |

**Per-metric relationship.** Every exact-invariant metric is identical:
statements, driver rows, write rows, statuses, stable errors, pool waits,
lock observations, deadlocks, retries and rollbacks, and the worker job
counters. Both runs read the same fixture volume in the checkpoint tenant
(`fixture_rows` is equal); the v2 additions live in a second tenant. The eight
material entries are latencies only: `communication.messages` p50 and p95
(faster in v2), `school-structure.list` worst p95, and
`delivery.provider-unavailable` p50, p95 and job duration (about 1.2 ms slower
in v2). No invariant moved with them, so they read as run-to-run jitter of
the same operations, the kind #3063 dispositioned for #3019.

Consequence for later checkpoints: the first 20 HTTP scenarios and the 5 worker
scenarios of a `checkpoint-1-v2` report compare with #3019 and #3020 exactly as
a `checkpoint-1-v1` report would. The 35 new scenarios and the contention runs
have no earlier baseline; this run is their first measurement.

## `checkpoint-1-v1` on this commit against #3019 and #3020

| Classification | against #3019 | against #3020 |
|---|---:|---:|
| unchanged | 784 | 824 |
| within tolerance | 39 | 51 |
| material | 135 | 123 |
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
| `identity-access.login` | 200 | 9.33 | 11.40 | 54 |  |
| `identity-access.login-wrong-password` | 401 | 2.23 | 2.56 | 11 | `invalid username or password` |
| `identity-access.refresh` | 200 | 4.72 | 5.07 | 27 |  |
| `identity-access.refresh-invalid-token` | 401 | 0.04 | 0.05 | 0 | `token unauthorized` |
| `identity-access.mfa-verify` | 200 | 8.95 | 9.77 | 52 |  |
| `identity-access.mfa-verify-wrong-code` | 401 | 3.35 | 3.72 | 18 | `invalid or expired code` |
| `identity-access.passkey-login-options` | 200 | 1.36 | 1.62 | 9 |  |
| `identity-access.passkey-login-options-wrong-origin` | 401 | 1.13 | 1.43 | 8 | `passkey origin is invalid` |
| `device-scan.checkin` | 200 | 7.84 | 10.59 | 37–50 |  |
| `device-scan.checkin-invalid-key` | 401 | 0.21 | 0.24 | 1 | `invalid device API key` |
| `device-scan.pickup-query` | 200 | 4.18 | 4.47 | 23 |  |
| `device-scan.pickup-query-missing-pin` | 401 | 0.78 | 0.87 | 5 | `staff PIN is required` |
| `device-scan.status` | 200 | 2.17 | 2.47 | 15 |  |
| `device-scan.status-wrong-pin` | 401 | 1.40 | 1.69 | 10 | `invalid staff PIN` |
| `device-scan.rfid-lookup` | 200 | 2.59 | 2.89 | 16 |  |
| `device-scan.rfid-lookup-malformed-key` | 401 | 0.04 | 0.05 | 0 | `invalid API key format - use Bearer token` |
| `device-scan.staff-clock` | 200 | 6.53 | 7.26 | 33–37 |  |
| `device-scan.staff-clock-wrong-pin` | 401 | 1.49 | 1.72 | 10 | `invalid staff PIN` |
| `people-directory.students` | 200 | 8.35 | 8.79 | 32 |  |
| `people-directory.students-invalid-view` | 400 | 0.59 | 0.81 | 4 | `invalid view "bogus": …` |
| `request-review.queue` | 200 | 2.20 | 2.39 | 12 |  |
| `request-review.queue-invalid-view` | 400 | 0.58 | 0.81 | 4 | `invalid change request list query` |
| `request-review.care-schedule-decide` | 200 | 22.06 | 25.24 | 100 |  |
| `request-review.care-schedule-decide-not-found` | 404 | 0.95 | 1.25 | 6 | `schedule: care schedule change request not found` |
| `care-plan.withdrawals` | 200 | 2.20 | 2.49 | 6 |  |
| `care-plan.withdrawals-invalid-state` | 400 | 0.58 | 0.79 | 4 | `state must be pending or resolved` |
| `student-deletion.withdrawal-impact` | 200 | 6.81 | 8.57 | 26 |  |
| `student-deletion.withdrawal-impact-not-found` | 404 | 0.79 | 1.12 | 5 | `students.care_withdrawal_not_found` |
| `care-plan.care-end-preview` | 200 | 4.44 | 4.66 | 20 |  |
| `care-plan.care-end-preview-past-day` | 400 | 0.58 | 0.76 | 4 | day in the past |
| `care-plan.care-end` | 200 | 16.63 | 17.89 | 64 |  |
| `care-plan.care-end-replan` | 200 | 16.23 | 18.05 | 64 |  |
| `care-plan.care-end-stale-token` | 409 | 9.79 | 10.57 | 39 | preview changed |
| `student-deletion.execute` | 200 | 17.86 | 19.42 | 64 |  |
| `student-deletion.execute-stale-preview` | 409 | 15.35 | 16.96 | 55 | `students.deletion_preview_changed` |

The kiosk check-in alternates between check-in and check-out, and the
device's `last_seen` flush is debounced onto a later request, so its statement
count varies within a run. The staff clock alternates the same way. The final
database state matches the statuses: 105 approved decisions, 210 ended cares,
105 deletions, 105 staff-clock transitions, and the child behind the stale
deletion still exists.

## Contention run (concurrency 16)

16 requests per round against the pool of 12, three runs of 30 measured
rounds. Slot 0 deletes the subject through `deleteConfirmed` →
`lockedSnapshot`, slot 1 deletes a companion whose graph contains the subject,
the other 14 slots write and read the same three children.

| Metric | Median | Worst |
|---|---:|---:|
| round wall p50 ms | 66.28 | 67.26 |
| round wall p95 ms | 80.55 | 89.39 |
| pool waits | 120 | 120 |
| pool wait ms | 1322.3 | 1360.9 |
| rounds with a pool wait | 30 | 30 |
| backends observed waiting on a lock (sampled) | 3187 | 3462 |
| most backends waiting at once | 8 | 8 |
| lock samples | 2726 | 2704 |
| deadlocks | 0 | 0 |
| unit-of-work retries | 0 | 0 |
| unit-of-work rollbacks | 38 | 38 |

| Operation | Requests per run | p50 ms per run | p95 ms per run | Statuses per run |
|---|---:|---|---|---|
| `student-deletion.execute-subject` | 30 | 48.5 / 45.8 / 45.1 | 68.4 / 60.9 / 57.6 | 200: 2, 409: 28 (each run) |
| `student-deletion.execute-companion` | 30 | 38.9 / 40.3 / 58.4 | 74.7 / 72.0 / 74.2 | 200: 28, 409: 2 (each run) |
| `people-directory.update-subject` | 120 | 52.3 / 54.2 / 46.6 | 71.9 / 79.3 / 72.6 | 200: 112, 404: 8 (each run) |
| `people-directory.update-second-companion` | 120 | 24.3 / 24.9 / 22.8 | 36.2 / 40.0 / 35.2 | 200: 120 |
| `student-deletion.preview-companion` | 90 | 10.3 / 10.4 / 9.7 | 20.0 / 18.0 / 17.2 | 200: 90 |
| `people-directory.students` | 90 | 15.6 / 16.2 / 14.6 | 25.0 / 25.5 / 23.1 | 200: 90 |

What this shows:

- **Pool.** Four of 16 requests per round wait for a connection, every round;
  that is the pool size, not a code property. The wait time is what they pay:
  about 44 ms per round across the four.
- **Locks.** Up to eight backends wait on locks at once. Most rounds, the
  subject's updates reach the subject row first, so its deletion takes the
  locks, re-reads the counts and refuses with `deletion_preview_changed` (409)
  after waiting; the companion's deletion walks a graph that still contains
  the subject, waits for the same rows, and succeeds. The locked deletion
  runs 45 to 48 ms at the median under this load, about two and a half times
  its serial 18 ms, and holds its student-row locks for most of it.
- **Deadlocks and retries.** No deadlock occurred in 90 rounds with real lock
  queues. Zero retries is structural: HTTP transactions never retry, so a
  deadlock would have surfaced as an error status outside the allowed set and
  failed the run. The 38 rollbacks per run are the 409 and
  404 answers above.

## `jsonb_to_recordset` set sizes

| Scenario | Call site | Calls per run | Rows per call |
|---|---|---:|---:|
| `care-plan.care-end-preview` | `CountRunningEnrollmentsForCareExit` (`modules/timetable/internal/adapters/postgres/care_exit_baseline.go`) | 30 | 0 |
| `care-plan.care-end` | the same count | 30 | 0 |
| `care-plan.care-end` | `restoreCappedEnrollments`, `restoreDeletedEnrollments` (`modules/timetable/internal/adapters/postgres/store.go`) | 60 | 0 |
| `care-plan.care-end-replan` | the same count | 30 | 3 |
| `care-plan.care-end-replan` | the same two restores | 60 | 3 |
| `care-plan.care-end-stale-token` | the same count | 30 | 0 |

The set is the earlier exit's removal list of the selected children: empty for
a first exit, three rows when the re-planned child's first exit removed its
three bookings. It grows with the selection and its bookings, not with the
tenant. The other call sites listed in #3411 (Care Plan withdrawal
completions, the users care-exit cleanup, audit booking consistency, the
Enrollment phase expiry and care-exit offerings, the Communication parent
audience, the Timetable projection and roster care-exit paths, room
utilization, session attendance, school account counts) are not reached by
any scenario of this workload: the care ends here lie in the future, so their
cleanup does not run inside the request. Their sizes remain unmeasured, not
bounded.

## Deletion workflow query budget

`TestStudentDeletionWorkflow_QueryBudget`
(`backend/services/users/student_deletion_query_budget_test.go`) runs the
production composition with one and with four linked children. The budgets in
`backend/test/query_budgets.go`:

| Scenario | 1 companion | 4 companions |
|---|---:|---:|
| `Preview` (no locks) | 22 | 22 |
| `Execute` (`deleteConfirmed` → `lockedSnapshot`) | 61 | 70 |
| of which student-row reads, 2(K+1) | 4 | 10 |

Each linked child costs three statements: its locked read with the shared
class-writes gate, and its unlocked re-read in the stranding check. The
doubled owner-count block is 2 × 16 statements (the Feedback counter is a test
double in this scenario), fixed in K.

## Limits

- One machine, in-process HTTP, PostgreSQL in a local container. Latencies are
  comparable between runs on this setup, not with production.
- Lock waits are sampled every 2 ms from `pg_stat_activity`; they are
  observations, not durations, and zero deadlocks is an observation, not a
  proof.
- The contention mix is fixed. Other writer mixes (grade transitions, kiosk
  scans of the same children) are not measured.
- Set sizes are read from inline literals only; a call site whose argument is
  not inline would count as `unparsed` (none was).
