# Runtime checkpoint workload

Issue [#3019](https://github.com/moto-nrw/project-phoenix/issues/3019) measures
the runtime after the Timetable & Activities cutover in
`ebe8da06033eaa233053c88dcfb7793f512c6990` (PR #3023).
The measurement does not change product behavior or architecture policy.
Raw measurements, interpretation, and acceptance belong in the issue, not here.

## Reproduce version `checkpoint-1-v1`

1. Check out the workload commit linked in #3019. Install the pinned Devbox
   toolchain. The existing test lifecycle starts PostgreSQL 17 through Docker
   Compose, migrates its template, and creates a disposable package database.
   Do not point the test at a development, staging, or production database.
2. Run only the production-router test in a fresh process. The output path
   must not exist; a failed setup cannot leave an older report looking current.

   ```bash
   checkpoint_dir=$(mktemp -d)
   GOMAXPROCS=4 scripts/run-go-toolchain.sh go -C backend test ./api \
     -run '^TestFullProductionRouterGolden$' -count=1 -parallel 8 \
     -runtime-checkpoint-output="$checkpoint_dir/raw.json" \
     > "$checkpoint_dir/test.log" 2>&1
   ```

3. Only after that command succeeds, generate the comparison tables. Python
   uses only the standard library. Preserve the JSON and test log as evidence.

   ```bash
   python3 scripts/runtime-checkpoint-report.py \
     "$checkpoint_dir/raw.json" "$checkpoint_dir"
   ```

The opt-in helper runs before the existing route/auth goldens. It uses their
`api.WithRuntime` instance and `Runtime.Handler`, including production module
wiring and the `phoenix_auth` pool. It adds no composition root. Ordinary test
runs do not execute the measurement. The helper refuses a broader `-run` filter.

## Workload contract

One process performs exactly three measured runs, in the same scenario order.
Before each scenario in each run, five operations warm the path. Each measured
scenario then performs 30 operations at concurrency one. Go test parallelism
stays at eight to preserve the existing 12-connection test-pool configuration;
it is not workload concurrency. `GOMAXPROCS=4` pins Go execution concurrency.

The fixtures contain 50 students, 50 guardians, a teacher/account chain,
11 rooms, 10 school groups, 10 activity groups plus one recurring template
and their categories, one active calendar period, one enrollment phase,
and 10 care offerings. The phase
and calendar windows use fixed calendar dates. The report records exact
tenant row counts, PostgreSQL settings, versions, role, CPU count, pool size,
and start time. Synthetic IDs and fixture-name suffixes vary; scenario names
and cardinalities are stable.

HTTP scenarios cover tenant resolution, Facilities reads and an idempotent
update, School Structure, People Directory, Timetable & Activities categories
and activities, School Calendar, Care Plan, Communication, Appointments,
Settings, Meal Plan, Feedback, and School Membership. Negative scenarios
cover invalid identifiers, a valid but nonexistent activity ID, and missing authentication. The source records
the exact methods, paths, bodies, and expected statuses.

Delivery uses the production worker instance through its public `RunOnce`
and `Backlog` methods. It measures idle polling, a render failure, and the
configured test mailer's explicit provider-unavailable failure. Each busy
sample starts with one owned pending intent. After observing its scheduled
retry, the helper deletes that fixture outside the timed region. The next
sample therefore has the same row count and backlog. This is fixture reset,
not a new production cleanup mechanism. No email leaves the test process.

Timetable Worker scenarios call the production materialization service with
the same public method and `scheduler` source tag as the scheduler. A complete
Monday template uses the fixed day `2026-09-07` and 08:00–16:00 wall-clock times.
One scenario creates an instance per measured call; the other verifies the
existing-instance skip. Owned instance resets happen outside timing, and each
run leaves the same fixture state. There is no persistent job queue or attempt
counter for these synchronous materialization calls, so those metrics are
explicitly not applicable.

## Metric semantics and limits

- Latency measures synchronous `Runtime.Handler` calls, including middleware,
  transactions, response serialization, and logging. It excludes TCP, TLS,
  a reverse proxy, frontend work, and SMTP/network delivery. Several list
  scenarios intentionally exercise empty-state behavior. This is a fixed
  low-concurrency comparison workload, not a capacity or saturation test.
- Query counts include all statements on the production pool during each
  measured operation. Fixture SQL, observer SQL, metric scraping, and explicit
  backlog/status inspection are outside the timed query count. The shared
  query counter also records driver-reported rows and how many statements
  supplied a row count, including SELECTs. These counts are not distinct
  entities or necessarily committed writes. Pool wait
  counts and durations come from before/after `database/sql` pool snapshots.
- Raw module counters retain their original Prometheus labels. Request,
  per-setting lookup, histogram, and unrelated process gauges are omitted.
  Driver row totals are the primary SQL-row measure; module row counters are
  supplementary and can be absent on some adapters. Delivery records rows
  claimed and retry state separately; Timetable records created/skipped instances.
- PostgreSQL `pg_stat_activity.wait_event_type = 'Lock'` is sampled every
  two milliseconds for `phoenix_auth` in this database. Counts and the largest
  actual sampling gap are recorded. Short waits can be missed; a zero sample
  count on a DB-free scenario is not evidence of zero database waits. Lock
  observations include the sampler's active window, which is wider than the
  timed worker calls. Statement/acquisition duration is never reported as
  measured lock-wait duration. Deadlocks use database-counter deltas; these
  counters can lag. Transaction-retry counters provide separate evidence.
- Worker timing covers direct public batch execution, not scheduler startup,
  polling delay, lease takeover, or every scheduled cleanup job.
  Retry evidence proves scheduling after a failure, not a later attempt
  after backoff. Successful SMTP and Web Push are not part of this workload.

The report uses nearest-rank p50/p95. Each numeric metric gets the median and
maximum across the three runs; for observer sample count, the minimum is the
worst coverage. Counter deltas and every measured sample remain available in
the raw evidence. Expected injected errors are reported by their exact text.
Unexpected HTTP statuses or worker outcomes fail the run.

Checkpoints #3020 and #3021 must use the same workload and comparable machine,
toolchain, database settings, and transport scope. Any workload change needs
an old/new bridge run on the same runtime commit. Do not extrapolate these
latencies to deployment capacity.

## Acceptance

The measurement command does not accept a baseline. Follow
[the acceptance registry contract](../../backend/architecture/README.md#recording-acceptance):
explicit issue acceptance precedes a reviewed registry entry. Neither this
helper nor its report edits `runtime-checkpoints.json`, policy, or ratchets.
