# Request review local runtime evidence (#2705)

## Reproduce

Run from the repository root:

```bash
CGO_ENABLED=0 scripts/run-go-toolchain.sh go -C backend test ./api/students -run '^TestRequestReviewRuntimeEvidence$' -count=1 -v
```

The harness (`backend/api/students/change_request_review_runtime_test.go`)
drives the production students router with a signed admin JWT holding
`users:read` and `users:update`: the open list, the decision history and the
pending-count badge, each over the four parent-request queues. The
pre-cutover reader is fixed at `939126a901186f7bbaeb984fee5ba7b10771a329`
(the merge-base of this branch). The same harness file was copied into a
detached worktree of that commit and run there; both runs happened alone on
the machine, one after the other.

Both runs use local PostgreSQL 17.11, Go 1.27.0 and the package test
database. Each request has five warmups and 30 measured samples, concurrency
one. Fixtures are outside the timer: 0, 8, then 32 children, each with one
pending Stammdaten, Betreuungszeiten, Angebote and Abwesenheiten request,
two decided Stammdaten requests and one decided Abwesenheiten request. The
counter is scoped to the request context, so statements issued outside the
request tree (the identity memoization warm-up, the per-test fixture
inserts) are not counted on either side. There is no projection cache; the
cache hit rate is not applicable.

## Results

Times are milliseconds, nearest-rank p50/p95 (15th/29th of 30 sorted
samples). Statements and driver-reported rows are identical on both sides
for every scenario.

| Scenario | Request | Baseline p50/p95 | Candidate p50/p95 | Queries | Rows |
|---|---|---:|---:|---:|---:|
| empty | open | 4.80 / 5.34 | 6.36 / 7.33 | 14 | 3 |
| empty | history | 2.97 / 3.83 | 3.97 / 5.27 | 10 | 2 |
| empty | pending-count | 1.90 / 2.11 | 2.30 / 5.93 | 8 | 1 |
| 8 children | open | 93.29 / 121.90 | 102.78 / 127.55 | 320 | 590 |
| 8 children | history | 6.23 / 25.50 | 6.87 / 8.60 | 19 | 92 |
| 8 children | pending-count | 42.25 / 49.35 | 51.74 / 112.23 | 156 | 209 |
| 32 children | open | 214.52 / 258.52 | 254.59 / 279.53 | 728 | 1550 |
| 32 children | history | 15.49 / 17.04 | 16.21 / 17.49 | 19 | 322 |
| 32 children | pending-count | 160.09 / 175.92 | 171.07 / 183.60 | 564 | 833 |

All 540 measured requests per reader answered 200 with the expected item
counts. Every sample reported `write_rows_affected` zero. Pool wait counts
and deadlock deltas were zero on both sides, and no sampled lock waiter
appeared in either run.

The candidate sits consistently a few percent above the baseline at p50
(worst case 32 children/open, 254.59 ms against 214.52 ms, about 19%). The
statement count and the driver-reported row count are byte-identical per
scenario, so the difference is not extra database work: it is the Go-side
merge and decoration path plus run-to-run variation between two separately
compiled binaries on one laptop. These are descriptive local samples, not
an SLO measurement, and no regression issue is opened for them. An earlier
candidate run recorded a 1,271 ms p95 for 32 children/open that did not
reproduce on the retained run; it was machine contention, not a code path.

The per-request statement count grows with the number of children (320 at
8, 728 at 32 for the open list; 156 and 564 for the badge). That is the
retained owner queues' own per-request hydration (student, person, group,
diff and eligibility reads per row), unchanged by this cutover, which only
replaces the fan-out orchestration. `OpenCount` for three of the four
queues still lists and counts the hydrated rows, exactly as the retained
badge did; only the offering queue has a real count. Batching those reads
is an owner-side follow-up outside #2705.

[Baseline log](request-review-2705.baseline.txt) and
[candidate log](request-review-2705.candidate.txt) retain every scenario's
full per-sample array: duration, statement count, rows affected, statements
with rows, write rows, pool wait count and duration, plus the lock-sampling
result and deadlock delta.

## Query plans

Not captured. The projection issues no statement of its own; every
statement belongs to the retained owner queues and the tenant transaction
runtime, which this cutover does not change. Their plans are therefore the
pre-cutover plans.

## Correctness and limits

The retained handler tests in `backend/api/students` (in-package fakes and
router tests over real keyset SQL) run unchanged against the projection and
pin the JSON output, the cursor contract, the filters, the permission
narrowing and the badge; their expectations were not edited, which is the
old-versus-new parity evidence for this cutover. Pure projection tests cover
ordering, tie-break, paging, urgency phases, the injected review day,
filters, empty pages, cursor progress over filtered rows, past-request
consequences, conflict grouping, decorations and error passthrough. Adapter
tests pin the owner-rule facts per row and prove the two-tenant RLS boundary
over all four request tables through the real retained queues under the
least-privilege role, with a query budget for the fixture-sized open page.

No deployment, staging or production requests occurred. These local
comparisons do not establish production latency, concurrent load behavior
or HTTP latency under a real reverse proxy.
