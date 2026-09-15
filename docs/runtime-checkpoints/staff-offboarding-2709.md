# Staff offboarding migration evidence

Issue: [#2709](https://github.com/moto-nrw/project-phoenix/issues/2709).
Implementation baseline: `a45d4fe3dc29141762709dd48eef82683b4a3210`.
Measured locally on 2026-09-09, Go 1.27.0, macOS arm64, PostgreSQL 17.11.
This flow-specific evidence supplements accepted checkpoint
[#3020](https://github.com/moto-nrw/project-phoenix/issues/3020#issuecomment-5591674205).
It is not a staging or production observation window.

## Cutover and ownership

`workflows/staffoffboarding` is the only staff-offboarding coordinator.
Production Membership HTTP deletion invokes its composed command. The previous
`services/users/staff_offboarding.go` provider and Factory field are removed.
Membership, Workforce, Timetable, People Directory and Identity perform their
own writes. The workflow contains no SQL or repository imports.

Account access is locked before Workforce's balance/absence/shift locks, then
Membership staff/teacher/assignments, People, current supervision, and the
remaining operational rows. Exact owner revisions are rechecked before writes.
Assignment writers revalidate live staff under the Membership lock. Historical
planning cannot be moved back into an operational assignment after retirement.
Session starts share activity → staff/balance → room/group locking; force-start
discovers transferred staff before locking, then rechecks that set under group
locks before mutation.

Existing table ownership is unchanged. Migration 1.15.374 adds the Workforce-owned
`users.staff_offboarding_cleanup` outbox. Workflow registration uses the reviewed
[#3130](https://github.com/moto-nrw/project-phoenix/issues/3130) prerequisite and
[ADR 0013](../adr/0013-staff-offboarding-is-an-application-workflow.md).
The existing domain/platform owner counts stay 18/10.

## Local runtime workload

Reproduce from the repository root:

```sh
CGO_ENABLED=0 scripts/run-go-toolchain.sh go test -C backend -p 4 -parallel 8 \
  ./services/users -run TestOffboardingRuntimeEvidence -count=1 -v
```

The isolated-clone workload runs five warmups followed by 30 sequential samples
per operation. Each subject has a linked account, refresh token, future shift
and future timetable assignment. It uses real owner composition, real audit
and token cleanup, and durable file-cleanup enqueue. There are no document bytes
in this latency fixture; leased file removal is measured through the separate
failure/retry test below. Fixture creation is outside the timed/query-counted
region. Percentiles use nearest rank.

Lossless evidence: [raw JSON](staff-offboarding-2709.raw.json).

| Operation | Statements | p50 | p95 | Successful DML rows | Errors |
| --- | ---: | ---: | ---: | ---: | ---: |
| Preview | 29 | 7.099 ms | 8.673 ms | 0 | 0/30 |
| Execute | 103 | 25.187 ms | 27.268 ms | 13 | 0/30 |

Execute includes owner deletes/tombstones, audit/intent writes and Identity's
after-commit work. These measurements call Preview and Execute separately;
they are not end-to-end DELETE timings. DELETE runs both inside one outer
transaction and reports an `offboard` observation that includes commit failure
or success. Its nested preview/execute observations are provisional until that
outer transaction commits. DML counts are driver-reported statement rows, not distinct
entities. Both operations had zero connection-pool waits in their measured
samples. The database deadlock counter changed by zero.

Including warmups, the trace records 280 committed transactions and zero
rollback events. These include joined owner scopes and after-commit Identity
transactions; they do not imply 280 separate retirement commits. There are
1,785 lock-acquisition statement events totalling 468.751 ms. The existing hook
measures SQL acquisition duration, an upper bound on lock wait, not wait-only
time. Separate contention tests observe actual blocked database sessions.

No latency SLO or offboarding query budget is specified by #2709. These values
are observations, not a claim of improvement over the old provider. Acceptance
requires the behavior and existing ratchets below, not a new threshold fitted
to this sample.

## Failure, conflict and cleanup evidence

1. `TestWorkflowRollsBackAfterEachOwnerCommand` injects five failures, after
   Workforce, Timetable, Membership, final audit and cleanup. All operational
   rows survive each failed command. The real-composition cleanup-intent failure
   test additionally checks Identity roles, tenant membership, tokens, person
   links and audit rollback, then successful/idempotent retries.
2. Workflow and owner drift tests reject changed revisions before mutation.
   Foreign-tenant, unauthorized-principal, guardian and multi-school cases
   retain their isolation/access contracts. Active supervision and a required
   handover reject retirement.
3. Real concurrent writer tests cover supervision creation/replacement,
   timetable/shift insertion and moving retained timetable history into the
   future. Writers observed waiting on locks cannot attach after retirement.
   The Workforce shift-key test uses PostgreSQL `lock_timeout` to demonstrate
   contention without depending on client/network deadline ordering.
4. Audit failure tests inject failures in absence audit and final deletion
   audit. Neither case commits partial offboarding; final-audit failure emits
   no access-change broadcast. These are injected failures, not production
   error-rate observations.
5. `TestOffboardingCleanupQueueRollsBackRetriesAndFencesExpiredClaims` checks
   enqueue rollback, duplicate enqueue, committed claim exclusion, foreign
   tenant rejection, expired-lease recovery, stale-token rejection, retry delay
   and idempotent completion.
6. `TestStaffDocumentsAPI_OffboardingCleanupLeaseRetriesFileFailure` injects
   one actual filesystem removal failure. Retirement remains committed;
   backlog is one. After advancing the injected clock by 60 seconds, attempt
   two removes the file and backlog becomes zero. The directory-retry test
   holds a committed lease and proves the UI retry cannot unlink its file.

The latency workload contains zero injected failures/conflicts. The cases above
are separate hermetic acceptance tests, not mixed into its percentiles.

## Rollback and limits

Before commit, every reversible owner write, audit and cleanup intent rolls
back together. After files or credentials are removed, deployment rollback
must not recreate them. Use owner-specific restoration/anonymization policy.
Retained attendance, work sessions, past planning and audit history remain.

The new job contains tenant/staff identity, lease and retry metadata only.
File names remain in existing document/upload records. Cleanup uses immutable
stored names and treats missing bytes as success; final job completion is
token-fenced. External filesystem operations cannot be transactionally rolled
back. No production files, credentials or external delivery were used here.

## Integrated review on 2026-09-09

Rebased onto `de4a1258089e8ffb3be399078a56c73f90772767`, including Enrollment
acceptance and the Calendar portal. The original measurements above remain
historical evidence. Migration 1.15.373 was occupied by the integrated Staff
Notices change, so this migration now uses 1.15.374.

Three concurrency regressions were demonstrated and fixed:

1. Guardian grant previously held the tenant mapping while waiting for the
   offboarding account lock. The controlled PostgreSQL regression observed
   `55P03` when offboarding attempted its mapping update. Grants now lock the
   account before the mapping, without changing global account fields.
2. Existing-student approval resynchronized class-driven rosters before its
   guardian grant. The regression observed the resync before the pending
   guardian role existed. Attachment now precedes roster work in the same
   transaction, preserving account-before-instance order.
3. Timetable Update and Patch could discover assignments, then miss a newly
   assigned and retired staff member before moving history into the future.
   Both regression variants previously succeeded incorrectly. Editors now
   lock known staff, lock the instance and recheck assignments; newly
   discovered staff produce a conflict without inverted lock acquisition.
   Assignment Create/Update share the instance lock after their staff lock.

All three regression tests pass after the fixes. These controlled cases do
not establish general deadlock freedom. The existing seeder's inactive-account
step exercises the real staff DELETE path; no new seed-coverage allowance was
added.

The isolated runtime workload was rerun after these fixes with the same
five warmups and 30 sequential samples per operation. Raw evidence:
[integration samples](staff-offboarding-2709.integration.raw.json).

| Operation | Statements | p50 | p95 | Successful DML rows | Errors |
| --- | ---: | ---: | ---: | ---: | ---: |
| Preview | 29 | 6.174 ms | 7.363 ms | 0 | 0/30 |
| Execute | 103 | 21.402 ms | 24.320 ms | 13 | 0/30 |

Pool waits and deadlock-counter delta were zero. These remain local capability
measurements, not end-to-end DELETE latency or production observations.

The first integrated CI seed smoke exposed a fourth regression: authenticated
sessions with an empty display name failed the deletion audit's required
`deleted_by` validation. The legacy provider's `system` fallback is restored
for that exact case. The real-composition regression failed before the fix and
passes afterward; audit-append failures still roll back the workflow.
