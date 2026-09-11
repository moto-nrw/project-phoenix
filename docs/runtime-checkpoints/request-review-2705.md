# Request review local runtime evidence (#2705)

## Reproduce

Run from the repository root:

```bash
CGO_ENABLED=0 scripts/run-go-toolchain.sh go -C backend test ./api/students -run '^TestRequestReviewRuntimeEvidence$' -count=1 -v
CGO_ENABLED=0 scripts/run-go-toolchain.sh go -C backend test ./api/students -run '^TestRequestReviewGolden$' -count=1
```

The harness (`backend/api/students/change_request_review_runtime_test.go`)
drives the production students router with a signed admin JWT holding
`users:read` and `users:update`: the open list, the decision history and the
pending-count badge, each over the four parent-request queues. The
pre-cutover reader is fixed at `939126a901186f7bbaeb984fee5ba7b10771a329`
(the merge-base of #3156). The same harness file was copied into a detached
worktree of that commit and run there; both runs happened alone on the
machine, one after the other. The retained logs and plans come from the run
against `78f35b2996` (`development` after #3157). Since #3156 the harness also
explains the read statements after the timed samples; the timed part is
unchanged.

The stored files are cut from the `-v` log of each run with `jq`: the
`request-review-runtime:` lines become the `.txt` logs, and the
`request-review-plans:` line becomes the plan files, with each shape cut to
160 characters and, for the candidate, the full plan kept only for the most
expensive shape per request:

```bash
grep -o 'request-review-plans: .*' run.log | sed 's/^request-review-plans: //' |
  jq 'with_entries(.value |= (to_entries | map(.key as $rank
    | (.value | .shape |= (if length > 160 then .[:160] + "…" else . end))
    | if $rank < 1 then . else del(.plan) end)))' > request-review-2705.plans.json
```

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
| empty | open | 5.63 / 7.04 | 5.42 / 9.12 | 14 | 3 |
| empty | history | 3.65 / 4.51 | 3.40 / 3.93 | 10 | 2 |
| empty | pending-count | 2.32 / 2.61 | 2.21 / 2.41 | 8 | 1 |
| 8 children | open | 95.86 / 111.68 | 95.32 / 114.57 | 320 | 590 |
| 8 children | history | 6.99 / 8.13 | 6.25 / 7.00 | 19 | 92 |
| 8 children | pending-count | 44.79 / 49.46 | 44.41 / 47.85 | 156 | 209 |
| 32 children | open | 222.53 / 238.66 | 218.65 / 239.78 | 728 | 1550 |
| 32 children | history | 16.20 / 20.26 | 15.25 / 16.31 | 19 | 322 |
| 32 children | pending-count | 161.88 / 176.90 | 160.26 / 176.84 | 564 | 833 |

All 540 measured requests per reader answered 200 with the expected item
counts. Every sample reported `write_rows_affected` zero. Pool wait counts
and deadlock deltas were zero on both sides, and no lock sample saw a
waiting backend in either run.

The two readers are within run-to-run variation of each other. In this run
the candidate's p50 lies within 11% of the baseline in every scenario, and
within 2% for the 32-child open list (218.65 ms against 222.53 ms); in
#3156's run it sat a few percent above (worst case 19% on 32
children/open). The statement count and the driver-reported row count are
byte-identical per scenario in both runs, so neither direction is database
work: it is the Go-side merge and decoration path plus variation between two
separately compiled binaries on one laptop. These are descriptive local
samples, not an SLO measurement, and no regression issue is opened for them.

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

## Query plans and scanned rows

After the last measured request of each 32-child scenario the harness
explains one exemplar of every distinct read statement shape with
`EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON)`, inside a tenant transaction under
the least-privilege role and outside the timer and the counter. Bun inlines
the arguments, so the exemplar replays the request's own values; literals are
folded only to group the executions of one shape. The
[candidate plans](request-review-2705.plans.json) keep the summary of every
shape and the full plan of the most expensive shape per request; the
[baseline plans](request-review-2705.baseline-plans.json) keep the
summaries. Rows scanned sum actual rows times loops over every scan node of
the exemplar, weighted by how often the request issued the shape, so they are
an estimate; returned rows are the exact driver-reported rows of the
request. Execution time sums the exemplars the same way.

| Request, 32 children | Read shapes | Read statements | Execution ms, baseline / candidate | Rows scanned (estimate) | Rows returned | Shared hit blocks |
|---|---:|---:|---:|---:|---:|---:|
| open | 38 | 724 | 28.03 / 25.66 | 78,626 | 1,550 | 4,893 |
| history | 11 | 15 | 8.50 / 8.45 | 73,162 | 322 | 3,393 |
| pending-count | 21 | 560 | 12.90 / 13.01 | 3,904 | 833 | 1,187 |

Shapes, statement counts, scanned rows and buffer hits are identical on both
sides; no block was read from disk. The only sequential scan is
`enrollment.request_child_offerings` in the offering queue's hydration.
Summed execution time differs only by noise.

The slowest statements are the owner queues' keyset reads under the search
filter: the open reads of the Stammdaten, Betreuungszeiten and Abwesenheiten
queues take about 2.9 ms each, the Stammdaten history about 5.5 ms. Their
plans start from the tenant/status indexes (for example
`idx_care_schedule_change_requests_tenant_status`) and then evaluate the
child-name search in a nested loop: an index scan over the tenant's students
per request row and a bitmap heap scan on `users.persons` with the `ILIKE`
filter per student (528 loops for 32 requests). That is where the 24,304
scanned rows per queue page come from, and the cost grows with pending
requests times children in the school. This is the retained queues' SQL,
unchanged by the cutover. Rewriting the search as a join on the request's
own child is an owner-side follow-up, like batching the per-row hydration
that makes up the other 700 statements of the open page.

## Output parity

`TestRequestReviewGolden`
(`backend/api/students/change_request_review_golden_test.go`) records 41
responses of the production students router, 34 single requests and three
cursor chains followed to their last page, over one fixed fixture: two
children in two groups, six open requests over all four queues (one sharing
its instant with a request of another type, one lying wholly in the past) and
five decisions on three Berlin days. The requests cover
the merged order and the type tie-break, keyset paging to the last page with
and without an explicit limit, every type, search, student, status and
date-range filter, empty pages, every malformed query (400), the missing
right (403), the absence-only 403 contract, the narrowing of a reviewer
without `users:update` to the excused queue, the group-leader scope with the
school setting off and on, and the badge for each caller.

`backend/api/students/testdata/request_review.golden.json` was written by the
pre-cutover four-service fan-out reader: the same test file, copied into the
`939126a901` worktree and run with `-update-request-review-golden`. The
projection reproduces that file byte for byte, and so does a second run in a
fresh process with new synthetic IDs. Synthetic IDs, suffixed group and
offering names and the offering ID inside a conflict key are replaced by
named tokens; the next cursor is decoded to its per-type positions.

The golden has four deliberate limits. The pre-cutover reader judged
urgency against the wall clock, so every calendar day lies long before or
after any plausible run date and the fixture has no weekly-plan request; the
urgency phases stay pinned by the projection's unit tests. The selectable
effective-from window of an offering request is the owner's rule over the
run date and is tokenized. The office's booking corrections are not in the
fixture; `TestAggregatedChangeRequests_RouterDirectCorrections` pins them.
`review_access` is absent throughout because neither side's students test
harness wires the review policy.

## Scope decisions

**Staff RSS feed: proposed rescoping, needs a maintainer decision.** The
personal RSS feed (`/students/change-requests/rss-feed`, Care Plan's
`modules/careplan/requestfeed`) keeps its registered
`parent-request-feed-read-model` projection; this change does not move it.
Three functional differences argue against serving it from this projection.
The feed also announces parent-origin `enrollment.change_requests` to
`config:manage` holders, which the staff list leaves to its own list. It
selects requests by submission instant (the last 30 days), which the
projection has no filter for. And it costs one `UNION ALL` statement (budget
`modules.careplan.request_feed.list`, exactly one), while the projection's
open page costs 14 to 728 statements over the JWT-scoped owner queues, for
which a token-authenticated feed has no caller. The feed was added on
2026-09-06, after #2705's evidence commit `8dc3a9ca` (2026-08-28), so it is
not one of the ticket's named consumers.

The ratchet offers no shortcut either. `scripts/backend-architecture.sh
explain` reports that `care-plan/compose` and `care-plan/postgres` may not
import `request-review-view/public`, and that `request-review-view/adapter`
and `root-composition/compose` may not import
`parent-request-feed-view/postgres`. A new rule between these existing role
points is a policy loosening, and PR mode accepts a new `read_projections`
grant only for a newly created projection owner. The existing
`request-review-view.adapter.care-plan-contract` rule would let the adapter
implement a new Care Plan contract port, but the adapter would still have no
grant for the tables and would have to fall back to the owner queues.
Whether #2705 is rescoped without the feed, or the feed moves under a
reviewed #2580 policy decision, is the maintainer's call.

**`enrollment.change_requests`.** The ticket's table list names the public
enrollment change requests; the staff list serves the parent offering
switches in `enrollment.offering_change_requests`, which already existed at
the evidence commit. The enrollment change requests keep their own list
behind `config:manage` instead of `users:update` (#2435,
`backend/api/enrollment/change_request_review_list_handlers.go`), and the
client merges both lists into the one Eltern list on `occurred_at`
(`frontend/src/components/students/requests/use-request-feed.ts`). Folding
them into `GET /students/change-requests` would cross that permission
boundary and change the wire contract, so the projection does not.

## Correctness and limits

The golden above is the old-versus-new comparison of the wire output. The
retained handler tests in `backend/api/students` (in-package fakes and
router tests over real keyset SQL) run unchanged against the projection and
pin the JSON output, the cursor contract, the filters, the permission
narrowing and the badge. Pure projection tests cover ordering, tie-break,
paging, urgency phases, the injected review day, filters, empty pages,
cursor progress over filtered rows, past-request consequences, conflict
grouping, decorations and error passthrough. Adapter tests pin the
owner-rule facts per row and prove the two-tenant RLS boundary over all four
request tables through the real retained queues under the least-privilege
role, with a query budget for the fixture-sized open page.

No deployment, staging or production requests occurred for this evidence.
The staging check after #3156's deploy (recorded on #2705) was a read-only
health check, not an old-versus-new comparison; the golden above is the
old-versus-new evidence, and it came after the fan-out was deleted.
These local comparisons do not establish production latency, concurrent load
behavior or HTTP latency under a real reverse proxy, and the plans come from
a fixture-sized table, not production statistics.
