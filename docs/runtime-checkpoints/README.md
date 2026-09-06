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

HTTP samples recorded with `write_rows_affected` separate driver-reported
INSERT, UPDATE, DELETE, and MERGE rows from SELECT results. Reports retain
these as `executed_write_rows_affected`. This counts successful statements,
including writes later rolled back; it is not a count of committed changes
or distinct entities. Older samples without this field remain unmeasured,
not zero. Data-modifying CTEs are not included in this DML-only counter.

### Enrollment read workload (`enrollment-2694-reads-v1`)

For #2694, add `-runtime-checkpoint-enrollment` to the command above. This
selects six Enrollment HTTP scenarios instead of the checkpoint baseline:
phase list, phase detail, schema-version list, public phase list, public form
bootstrap, and invalid phase ID. It preserves the three runs, five warmups,
30 measured requests per scenario, and concurrency one. The original
`checkpoint-1-v1` workload is unchanged when the flag is absent.

Setup enables Enrollment through the production settings HTTP endpoint for
the owned test tenant and resolves its school slug separately from its
subdomain. Setup requests are outside measurement. This workload skips the
unrelated delivery and timetable workers. It records the same HTTP query,
latency, status, pool-wait, sampled lock-wait, and driver-row metrics as the
baseline. Schema versions exercise the empty-list path; public bootstrap
uses a phase without a pinned schema and ten care offerings.

This read workload is not evidence for successful submission, captcha or
parent authentication, write rollback, duplicate prevention, or serialization
retry behavior. Those require separate scenarios. Do not compare these
scenario latencies with unrelated `checkpoint-1-v1` operations or treat the
read-only observation window as proof of the complete #2694 cutover.

### Enrollment write workload (`enrollment-2694-writes-v7`)

The separate `-runtime-checkpoint-enrollment-writes` option selects
`enrollment-2694-writes-v7`: a repeated phase update, an invalid phase update,
a schema rename, a conflicting schema rename, schema version publication,
public intake, IP- and email-rate-limited intake, and duplicate rejection through the production
HTTP router. Use it instead of the read
option. Each scenario has the same three runs and 30 measured requests per
run. The successful PUT keeps the phase window unchanged and writes the same
name, so cardinality stays fixed. A final database read verifies that invalid
updates did not overwrite the successful name. Two named schemas are created
through HTTP outside timing. Renaming the first to the second's name must
return 409; a final database read verifies that the first schema retains its
successful name. Setup remains outside timing. Publication uses the original
schema ID and appends a version on every warmup and measured request. The final
database check requires 106 consecutive versions, including the initial schema.
Lineage size therefore grows between runs and affects subsequent rename work.
Version 7 adds duplicate blocking to email-throttling version 6, IP-throttling version 5, intake version 4, publication version 3,
rename-only version 2, and phase-only version 1;
compare only scenarios with matching names, setup, and lineage sizes.

Public intake submits one child per request with a unique synthetic email and
IPv6 client address from the documentation range. Rate-limit enforcement stays
active. The `{{attempt}}` body marker
is replaced by the process-local request sequence. All warmup statuses are
checked as well as measured statuses. Final database counts require exactly
210 requests and 210 children for the phase, including the duplicate originals described below. Captcha configuration is checked
before timing and must be disabled for this fresh tenant; this does not test
provider-backed captcha verification or authenticated parent submissions.

Outside timing, ten invalid anonymous submissions exhaust a separate IP bucket.
Each must return 400. Subsequent warmup and measured requests from that client
must return 429 with `Retry-After: 3600`. They share the final enrollment-row
count check, so no rejected attempt may add a request or child to the phase.
Five further invalid submissions from distinct synthetic IPs exhaust one
email bucket outside timing. Every measured email-throttling request also uses
a distinct IP, isolating the email limit from the IP limit. These requests
must likewise return 429 and `Retry-After: 3600` without enrollment writes.
Final database reads require 115 attempts in the exhausted IP bucket and 110
in the exhausted email bucket, proving rejected attempts remain committed.
The raw report's `final_state` records these counts alongside request, child,
and schema-version totals; these reads run outside the timed workload.
This measures exhausted IP and email paths, not window expiry or an injected
database failure.

Duplicate rejection uses `enrollment.duplicate_handling=block`, configured
through the settings API before timing. The fresh-tenant default is `warn`,
which accepts duplicates with a warning and is not the policy measured here.
Before each duplicate scenario, 35 distinct original submissions are created
outside warmup, timing, and metric-counter snapshots. Each original is retried
once with the same client and payload, expecting 409. Two attempts per client
remain below both throttle thresholds. The final row counts include 105
originals but no duplicates. This verifies sequential duplicate prevention,
not concurrent conflict behavior.

The report's per-operation `http_error_rate` counts all HTTP responses at or
above 400. Expected validation failures and throttling therefore have an error
rate of 1, while `http_unexpected_status_rate` remains 0. The report verifies
the declared unexpected-status count against the individual samples and rejects
any unexpected status. Run the report checks with
`python3 -m unittest discover -s scripts -p runtime_checkpoint_report_test.py`.

`transaction_rollbacks` sums measured deltas of
`phoenix_unit_of_work_rollbacks_total`; it is not inferred from HTTP status.
A rejected request may roll back its request transaction while its throttle
attempt remains committed in the separate rate-limit transaction. These
observed rollbacks do not replace injected post-write rollback/retry tests.

The invalid phase request fails validation; it is not an
injected failure after a write and does not prove transaction rollback.
Driver-reported rows include SELECT results as well as writes, as described
below, and must not be reported as the number of mutated rows.

### Enrollment parent workload (`enrollment-2694-writes-v8`)

Add `-runtime-checkpoint-enrollment-parents` alongside
`-runtime-checkpoint-enrollment-writes` to select version 8. Without the parent
option, version 7 remains available for a same-runtime comparison run.
Version 8 retains the nine version-7 scenarios and adds successful parent
intake and rejection of a staff token on the parent submission route.

Setup creates a real guardian account, active school mapping, guardian role,
and permitted parent-child relationship outside measurement. A signed test JWT
uses parent scope with no admin privileges or tenant binding. Each successful
request submits one new child through the production router, without a captcha
token. The wrong-scope scenario uses the staff JWT and must return the existing
401 contract. It must not create enrollment rows.

Final database checks require 315 requests and children, including the 105
duplicate originals, and exactly 105 requests linked to the authenticated
guardian account. These counts include warmups. The extra parent fixture also
adds one existing student and guardian profile to the environment inventory.
Compare shared operations with that cardinality difference in view.

This workload covers authenticated new-child submission, not parent login,
refresh, existing-child re-enrollment permissions, or captcha-provider behavior.
The fresh tenant still has captcha disabled. The route resolves a school from
its subdomain; the anonymous routes retain their separate slug contract.

### Checkpoint baseline

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
