# Enrollment #2699: acceptance through owner capabilities

This records local development verification of the enrollment acceptance
workflow after its account, tenant-mapping and role repositories were
replaced by the public Identity & Access guardian-access capability
(`modules/identityaccess`). The completed cutover also routes acceptance
student reads, locks, creation, renewal and profile writes through People
Directory. Departure reconciliation coordinates People Directory and Care Plan
under the existing graph-lock protocol. The other tables #2699 names,
`users.class_list_entries`, `enrollment.request_child_offerings` and
`schedule.instance_students`, remain behind the legacy-composition adapters over
their owner facades (see `backend/architecture/README.md`). This is not a
deployment observation. The original measurement below predates the student
cutover and used base `116cd0b0026baaa927b65b2efaed90baf1ceee32`.
The integration review rebased the PR onto
`a45d4fe3dc29141762709dd48eef82683b4a3210`.
Before push, PR #3123 advanced development; both acceptance commits were
then rebased without conflict or patch changes onto
`36429d4a58ec68432d5d8bdb9d5a319af47709b1`.

## Reproduce

From the repository root, with the pinned Devbox toolchain and the Docker
Compose test PostgreSQL:

```sh
checkpoint_dir=$(mktemp -d)
GOMAXPROCS=4 CGO_ENABLED=0 scripts/run-go-toolchain.sh go -C backend test ./api \
  -run '^TestFullProductionRouterGolden$' -count=1 -parallel 8 \
  -runtime-checkpoint-output="$checkpoint_dir/raw.json" \
  -runtime-checkpoint-enrollment-acceptance \
  > "$checkpoint_dir/test.log" 2>&1
python3 scripts/runtime-checkpoint-report.py "$checkpoint_dir/raw.json" "$checkpoint_dir"
```

The workload contract is in [README.md](README.md) under
`enrollment-2699-acceptance-v1`. The recorded run used Go 1.27.0,
PostgreSQL 17.11 (aarch64, Docker), `GOMAXPROCS=4` on a 16-CPU machine, the
production `Runtime.Handler` with the `phoenix_auth` pool (12 connections),
concurrency one, three runs, five warmups and 30 measured requests per
scenario. Raw JSON, summary and test log are retained locally outside the
repository.

## Original Identity-only measurement

Median / worst across the three runs. Queries are per request; the report's
per-run totals divided by 30. Every scenario returned its expected status in
all 90 measured samples, so `http_unexpected_status_rate` is 0 everywhere.
The three stable-error scenarios (`approve-terminal` and `invalid-status`,
400; `child-not-found`, 404) have an `http_error_rate` of 1 by definition.

| Operation | Expected | p50 ms | p95 ms | Queries | Write rows |
|---|---:|---:|---:|---:|---:|
| enrollment.acceptance.approve | 200 | 12.718 / 12.904 | 15.697 / 34.134 | 40 to 41 | 7 |
| enrollment.acceptance.approve-parent-account | 200 | 12.331 / 12.813 | 14.283 / 19.340 | 41 | 7 |
| enrollment.acceptance.reject | 200 | 4.093 / 4.640 | 4.778 / 14.562 | 13 | 1 |
| enrollment.acceptance.waitlist | 200 | 4.175 / 4.176 | 4.943 / 5.119 | 14 | 1 |
| enrollment.acceptance.approve-terminal | 400 | 1.487 / 1.553 | 1.876 / 1.922 | 6 | 0 |
| enrollment.acceptance.invalid-status | 400 | 0.880 / 0.926 | 1.910 / 4.900 | 4 | 0 |
| enrollment.acceptance.child-not-found | 404 | 1.382 / 1.438 | 1.638 / 1.939 | 6 | 0 |

Query counts were identical in every sample of every run for six of the seven
operations. The anonymous approval measured 40 statements in most samples and
41 in a few: an anonymous approval owes the new guardian an invitation, which
the handler dispatches after the commit on a separate goroutine, and its
statements can land in the measurement window of the neighbouring sample.
The parent-account approval attaches the existing platform account inside
the request transaction and dispatches nothing afterwards; its 41 statements
never varied. Both approvals report seven driver-counted write rows per
request in every sample; the reject and waitlist decisions report one and
the stable-error decisions none. The tenant-mapping upsert counts as one
write row even when it changes nothing, exactly like the legacy
`EnsureActive` statement it replaces.

Zero pool waits in every sample; no waiting backend in any lock sample
(largest sample gap 8.946 ms); no deadlocks. The worst-run p95 values above
10 ms (anonymous approval, parent-account approval, reject) come from single
outlier samples in the third run; the median runs stay below 16 ms and the
p50 values differ by less than 1.2 ms between runs. Two earlier executions
of the same workload on this change set produced the same query counts,
write rows and final state.

## Observed final database state

Measured after the workload, outside request timing.

| Counter | Final value |
|---|---:|
| submitted_children | 631 |
| approved | 210 |
| created_students | 210 |
| parent_submissions | 105 |
| parent_approved | 105 |
| rejected | 210 |
| waitlisted | 105 |
| undecided | 106 |

Every prepared submission is persisted: 631 children equal the anchor request
plus 630 preparation submissions. Every approval linked exactly one created
student, and every one of those students exists. All 105 parent-account
approvals link the student to the parent's guardian profile as primary
guardian. The parent account keeps its active school mapping and exactly one
guardian role assignment after 105 grants, and no invitation exists for it.
The 210 rejected children are the 105 measured rejections plus the 105
children the terminal scenario rejected during preparation; the 106
undecided children are the anchor request and the 105 children the
invalid-status scenario left untouched. The 105 not-found decisions
addressed an unused child ID and changed nothing.

This is local development evidence, not production traffic, and does not
cover parent login, e-mail delivery of the decision notifications or
invitations, care-offering materialization (the phase fixture keeps the
optional selection mode and the submissions select no offerings), concurrent
decisions on one request, or an injected database failure. The Identity &
Access integration tests cover tenant scoping, reactivation, role idempotency,
rollback with the caller's transaction and failure propagation separately.

## Final owner-cutover measurement, 2026-09-09

The workload was rerun after the integration fixes and after the broad test,
lint and architecture processes had finished. The same workload version,
three runs, five warmups and 30 measured requests per scenario were used.
The production-router golden passed. A preceding run that overlapped the
quality checks is excluded from latency comparison.

Median / worst across the three isolated runs:

| Operation suffix | Expected | p50 ms | p95 ms | Queries | Write rows |
|---|---:|---:|---:|---:|---:|
| approve | 200 | 12.524 / 13.064 | 13.548 / 15.772 | 41 to 42 | 7 |
| approve-parent-account | 200 | 12.800 / 12.895 | 14.367 / 14.431 | 42 | 7 |
| reject | 200 | 3.848 / 3.867 | 4.275 / 4.587 | 13 | 1 |
| waitlist | 200 | 4.138 / 4.152 | 4.635 / 4.736 | 14 | 1 |
| approve-terminal | 400 | 1.514 / 1.562 | 1.634 / 1.830 | 6 | 0 |
| invalid-status | 400 | 0.885 / 0.909 | 1.004 / 1.025 | 4 | 0 |
| child-not-found | 404 | 1.493 / 1.555 | 1.692 / 1.966 | 6 | 0 |

Every operation has the prefix `enrollment.acceptance.`. Unexpected status
rate was zero in all 630 measured requests. Expected-error rates remain one
for the three refusal scenarios and zero for the other four. All pool-wait
counts and durations were zero. No sampled backend waited on a lock; the
largest sampling gap was 3.965 ms. No deadlocks were reported.

Approvals now execute one additional query: People Directory verifies and
key-share-locks the same-tenant person before creating its student. This
closes the cross-tenant foreign-key gap at the new public write boundary.
Write-row counts did not grow. Parent-account approval median p95 is
14.367 ms versus 14.283 ms in the original measurement; the other timing
differences do not demonstrate a material regression in this local workload.
The historical and current runs were not a controlled same-session A/B test.

Artifacts are retained outside the repository under
`/tmp/backend-refactor-wave-20260909/`: `3129-runtime-isolated.json`,
`3129-runtime-isolated.log`, and `3129-runtime-isolated-report/`.
These are local evidence, not staging or production verification.

### Additional acceptance evidence

`services/enrollment/decision_approval_rollback_test.go` selects a fixed-day
offering with a future instance and injects failure at the final notification.
The test observes actual pending writes to student/person, guardian
relationship, activity enrollment, instance roster, tenant mapping and role
before returning the error. It then compares complete tenant-table snapshots
against the pre-decision state, checks unchanged global account data and a
foreign tenant, and verifies successful retry and repeated-approval
idempotency. The fixture observes `users.class_list_entries` unchanged:
that table is not written by this approval, and its existing Membership
isolation tests remain the named-table access evidence.

People Directory contract tests additionally cover cross-tenant rejection,
NULL patches, unrelated-field preservation, mixed departure-mode mirrors,
per-day companion coverage and rollback. Existing enrollment companion tests
exercise stranding refusals and change-notification behavior through the new
owner write path. These behavior tests complement the seven timed scenarios;
care-offering materialization, concurrent graph edits and injected failures
are not represented in the latency table.
