# Enrollment #2696: change requests and parent/OGS dialogue

This records local development verification of the change-request capability
after the last legacy provider (`models/enrollment.ChangeRequest`) was removed.
It is not a deployment observation. The change set is based on
`e763702f10cd12a2d53a660021959e204da7e7a4`.

## Reproduce

From the repository root, with the pinned Devbox toolchain and the Docker
Compose test PostgreSQL:

```sh
checkpoint_dir=$(mktemp -d)
GOMAXPROCS=4 CGO_ENABLED=0 scripts/run-go-toolchain.sh go -C backend test ./api \
  -run '^TestFullProductionRouterGolden$' -count=1 -parallel 8 \
  -runtime-checkpoint-output="$checkpoint_dir/raw.json" \
  -runtime-checkpoint-enrollment-change-requests \
  > "$checkpoint_dir/test.log" 2>&1
python3 scripts/runtime-checkpoint-report.py "$checkpoint_dir/raw.json" "$checkpoint_dir"
```

The workload contract is in [README.md](README.md) under
`enrollment-2696-change-requests-v1`. The recorded run used Go 1.27.0,
PostgreSQL 17.11 (aarch64, Docker), `GOMAXPROCS=4` on a 16-CPU machine, the
production `Runtime.Handler` with the `phoenix_auth` pool, concurrency one,
three runs, five warmups and 30 measured requests per scenario. Raw JSON,
summary and test log are retained locally outside the repository.

## Measured results

Median / worst across the three runs. Queries are per request; the report's
per-run totals divided by 30. Every scenario returned its expected status in
all 90 measured samples, so `http_unexpected_status_rate` is 0 everywhere.
The two stable-error scenarios (`parent-reply-wrong-status`, 400; the two
not-found scenarios, 404) have an `http_error_rate` of 1 by definition.

| Operation | Expected | p50 ms | p95 ms | Queries |
|---|---:|---:|---:|---:|
| enrollment.change-request.public-list | 200 | 4.276 / 4.590 | 4.912 / 24.253 | 18 |
| enrollment.change-request.admin-list | 200 | 2.836 / 3.241 | 3.464 / 3.999 | 10 |
| enrollment.change-request.admin-detail | 200 | 2.651 / 2.762 | 3.267 / 3.654 | 9 |
| enrollment.change-request.review-open | 200 | 1.890 / 1.900 | 2.273 / 2.300 | 7 |
| enrollment.change-request.review-history | 200 | 2.664 / 3.140 | 3.090 / 13.026 | 8 |
| enrollment.change-request.pending-count | 200 | 1.026 / 1.038 | 1.181 / 1.255 | 5 |
| enrollment.change-request.question | 200 | 5.058 / 5.389 | 7.526 / 10.612 | 16 |
| enrollment.change-request.parent-reply | 200 | 6.483 / 10.136 | 8.011 / 39.670 | 25 |
| enrollment.change-request.parent-reply-wrong-status | 400 | 1.900 / 1.980 | 2.519 / 2.543 | 9 |
| enrollment.change-request.create | 201 | 15.143 / 15.625 | 17.305 / 33.731 | 45 |
| enrollment.change-request.reject | 200 | 4.788 / 5.287 | 15.986 / 29.972 | 16 |
| enrollment.change-request.approve | 200 | 12.285 / 12.785 | 16.874 / 22.815 | 43 |
| enrollment.change-request.admin-not-found | 404 | 1.008 / 1.071 | 1.200 / 2.996 | 5 |
| enrollment.change-request.public-list-unknown-token | 404 | 0.795 / 0.963 | 0.994 / 1.101 | 4 |

Query counts were identical in every sample of every run: the minimum and
maximum per operation are equal, so no operation scales its statement count
with the number of change requests or messages in the tenant.

Driver-reported rows do grow between runs for every operation that loads the
dialogue change request (`public-list`, `admin-list`, `admin-detail`,
`question`, `parent-reply`): the family's view returns the complete message
list, and the dialogue accumulates 138 messages per run on one change request
(69 questions and 69 replies, counting preparation requests). That is the
workload's own dialogue length, not an unbounded read. Operations on the
decision request and the review lists (limit 20) keep constant row counts.

Pool waits: zero count and zero duration in every sample. Lock sampling of
`pg_stat_activity.wait_event_type = 'Lock'` for `phoenix_auth` observed no
waiting backend in any scenario; the largest sampling gap was 11.869 ms, so
shorter waits cannot be excluded. Deadlock counters did not move. The
`transaction_rollbacks` deltas contain only the expected 400/404 refusals.

The worst-run p95 outliers (`parent-reply` 39.670 ms, `create` 33.731 ms,
`reject` 29.972 ms) are single samples on a shared development machine; the
medians across runs are within 3 ms of the run-one values recorded during
development. This is a fixed low-concurrency comparison workload, not a
capacity or saturation test.

## Final database state

The workload tracks every successful response and compares it with the
database after the last run. All checks passed:

| Counter | Final value |
|---|---:|
| change_requests | 313 |
| approved | 105 |
| rejected | 207 |
| open (pending_review or needs_parent_response) | 1 |
| messages | 726 |
| questions | 207 |
| replies | 207 |

313 = one dialogue change request plus 3 × 104 decision-request proposals
(35 filed during `create`, 34 prepared for `reject`, 35 prepared for
`approve`; the last `create` proposal is rejected by the first `reject`).
726 messages = 207 questions + 207 replies + 207 rejection notes +
105 approval notes. The decision request's guardian last name equals the last
approved proposal, and the dialogue request holds exactly one open change
request.

## Failure and isolation evidence

- `TestChangeRequestReviewCountIsTenantAndStatusScoped`
  (`backend/modules/enrollment/integration/change_request_counts_test.go`)
  covers both tables with two tenants: foreign reads return nothing or
  "not found", foreign inserts fail, and an injected failure after each
  insert, status update, review mark and message insert rolls the write back.
- `TestChangeRequestTablesAreRowLevelSecured`
  (`change_request_failures_test.go`) issues raw selects and updates on
  both tables under the verified non-bypass request role, without an
  application tenant predicate, and sees only the current tenant's rows.
- `TestChangeRequestReadsPreserveStoreFailures` (same file) proves every
  change-request and message read surfaces a driver failure with its stable
  operation prefix and `context.Canceled` in the chain, never an empty
  result, and refuses to read without a tenant transaction.
- `TestFullProductionRouterGolden/contracts/enrollment submission/public`
  pins the parent/OGS dialogue at the production router: the family's list
  without reviewer identity in
  `backend/api/testdata/enrollment_change_request_dialogue.golden`, the
  staff history entry in `enrollment_change_request_review_history.golden`,
  the pending-count badge, the stable 400 for a reply nobody asked for, and
  the 404 for an unknown status token.

This is not evidence for authenticated parent-portal sessions, e-mail
delivery of the dialogue notifications, concurrent decisions on the same
change request, or a real database outage.
