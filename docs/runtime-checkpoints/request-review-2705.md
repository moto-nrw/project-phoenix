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
database. Each request has five warmups and 30 samples, concurrency one.
Fixtures are outside the timer: 0, 8, then 32 children, each with one
pending Stammdaten, Betreuungszeiten, Angebote and Abwesenheiten request,
two decided Stammdaten requests and one decided Abwesenheiten request. The
counter is scoped to the request context, so statements issued outside the
request tree (the identity memoization warm-up, the per-test fixture
inserts) are not counted on either side. There is no projection cache; the
cache hit rate is not applicable.

## Results

Times are milliseconds, nearest-rank p50/p95 (15th/29th of 30 sorted
samples). Queries are the statements of one request, identical on both
sides for every scenario.

| Scenario | Request | Baseline p50/p95 | Candidate p50/p95 | Queries, both |
|---|---|---:|---:|---:|
| empty | open | 31.49 / 55.96 | 7.16 / 15.73 | 14 |
| empty | history | 11.75 / 40.43 | 4.21 / 5.30 | 10 |
| empty | pending-count | 5.31 / 22.53 | 2.94 / 3.65 | 8 |
| 8 children | open | 226.22 / 760.39 | 126.93 / 252.53 | 320 |
| 8 children | history | 7.96 / 9.79 | 8.12 / 9.75 | 19 |
| 8 children | pending-count | 64.02 / 90.91 | 57.56 / 84.24 | 156 |
| 32 children | open | 272.33 / 423.64 | 271.31 / 329.75 | 728 |
| 32 children | history | 18.53 / 27.77 | 17.51 / 18.33 | 19 |
| 32 children | pending-count | 195.48 / 511.85 | 180.57 / 209.46 | 564 |

All 630 requests across both readers (540 measured after the warmups)
answered 200 with the expected item counts. Every sampled `write_rows_affected` was zero. Pool
wait count/duration and deadlock deltas were zero; no sampled lock waiter
appeared in either run. The baseline run's tails (empty/open p95 55.96 ms,
8 children/open p95 760.39 ms) include the first requests after the
worktree's cold compile and are descriptive local samples, not evidence of
a speedup; the candidate does not add statements anywhere.

The per-request statement count grows with the number of children (320 at
8, 728 at 32 for the open list; 156 and 564 for the badge). That is the
retained owner queues' own per-request hydration (student, person, group,
diff and eligibility reads per row), unchanged by this cutover, which only
replaces the fan-out orchestration. Batching those reads is an owner-side
follow-up outside #2705.

[Baseline log](request-review-2705.baseline.txt) and
[candidate log](request-review-2705.candidate.txt) retain every scenario's
p50/p95/max, statement count, lock-sampling result and deadlock delta.
The per-sample arrays were stripped from the stored logs to keep them
readable; the harness prints them with `-v`.

## Query plans

Not captured. The projection issues no statement of its own; every
statement belongs to the retained owner queues and the tenant transaction
runtime, which this cutover does not change. Their plans are therefore
the pre-cutover plans.

## Correctness and limits

The retained handler tests in `backend/api/students` (in-package fakes and
router tests over real keyset SQL) run unchanged against the projection and
pin the JSON output, the cursor contract, the filters, the permission
narrowing and the badge. Pure projection tests cover ordering, tie-break,
paging, urgency phases, filters, empty pages, cursor progress over filtered
rows, past-request consequences, conflict grouping, decorations and error
passthrough. Adapter tests pin the owner-rule facts per row and prove the
two-tenant RLS boundary over all four request tables through the real
retained queues under the least-privilege role, with a query budget for the
fixture-sized open page.

No deployment, staging or production requests occurred. These local
comparisons do not establish production latency, concurrent load behavior
or HTTP latency under a real reverse proxy.
