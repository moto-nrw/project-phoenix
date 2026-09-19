# Student deletion migration evidence

Issue: [#2710](https://github.com/moto-nrw/project-phoenix/issues/2710).
Implementation baseline: `4ee996e92503e21e722c12595edcead1c3ec61d5`.
Measured locally on 2026-09-15, Go 1.27.0, macOS arm64, PostgreSQL 17.11.
This flow-specific evidence supplements the accepted checkpoint
[#3020](https://github.com/moto-nrw/project-phoenix/issues/3020#issuecomment-5591674205).
It is not a staging or production observation window.

## Cutover and ownership

`workflows/studentdeletion` is the only student-deletion coordinator. The
three HTTP entry points (`GET /students/{id}/delete-impact` with
`DELETE /students/{id}`, `DELETE /students/{id}/purge`, and
`GET/DELETE /students/care-withdrawals/{completionId}[/deletion-impact]`)
call its preview and commands. The previous `services/users` deletion
service, the cross-schema `database/repositories/users` deletion repository,
the `models/users` deletion contract and the handler-side purge transaction
are removed. People Directory, Care Plan, Timetable, School Structure,
Student Presence, Communication, Identity & Access and Audit perform their own
reads and writes; the workflow contains no SQL or repository imports.

Lock order: the tenant care-booking gate and the withdrawal task (withdrawal
path only), then the subject and every companion far end in ascending id order
(late lower ids NOWAIT), the message threads, the person row. Every owner
count is read twice, once for the fingerprint and once under the locks, and
the confirmed fingerprint is compared before the first mutation.

Existing table ownership is unchanged except for the ADR 0015 adoption of
`users.persons_guardians` into People Directory. Workflow registration uses
[ADR 0016](../adr/0016-student-deletion-is-an-application-workflow.md) and
raises the policy epoch from 7 to 8. The domain/platform owner counts stay
18/10. The composition inventory shrinks from 772 to 765 field/setter targets
and the legacy baseline from 1249 to 1244 entries.

## Local runtime workload

Reproduce from the repository root:

```sh
CGO_ENABLED=0 scripts/run-go-toolchain.sh go test -C backend -p 4 -parallel 8 \
  ./services/users -run TestStudentDeletionRuntimeEvidence -count=1 -v
```

The isolated-clone workload runs five warmups followed by 30 sequential samples
per operation. Each subject has one future timetable assignment and one
guardian link. It uses the real owner composition and the real audit command;
there are no document bytes in this latency fixture, so no cleanup intent is
queued. Fixture creation is outside the timed/query-counted region.
Percentiles use nearest rank.

Lossless evidence: [raw JSON](student-deletion-2710.raw.json).

| Operation | Statements | p50 | p95 | Successful DML rows | Errors |
| --- | ---: | ---: | ---: | ---: | ---: |
| Preview | 22 | 7.373 ms | 8.744 ms | 0 | 0/30 |
| Execute | 54 | 19.129 ms | 24.186 ms | 5 | 0/30 |

Execute includes the locked re-read of every count, the owner deletes, the
person tombstone and both audit appends. DML counts are driver-reported
statement rows, not distinct entities. Both operations had zero
connection-pool waits in their measured samples. The database deadlock counter
changed by zero.

Including warmups, the trace records 70 committed transactions and zero
rollback events. There are 210 lock-acquisition statement events totalling
67.4 ms. The existing hook measures SQL acquisition duration, an upper bound
on lock wait, not wait-only time. Separate contention tests observe actual
blocked database sessions.

No latency SLO or deletion query budget is specified by #2710. These values
are observations, not a claim of improvement over the old provider.

## Failure, conflict and cleanup evidence

1. `TestStudentDeletionWorkflow_RollsBackAfterEachOwnerCommand` injects a
   failure after each of the eight owner commands (timetable assignments,
   legacy guardian links, document cleanup intents, withdrawal redaction,
   student row, ledger anonymization, person tombstone, audit). Every row,
   the withdrawal task, the ledger name and the person survive; no cleanup
   intent and no tombstone is left behind; the same request succeeds once the
   owner recovers.
2. `TestStudentDeletionWorkflow_RejectsStalePreview` covers preview/execute
   drift for an added assignment, a message read cursor, a person rename and a
   concurrent deletion; each is refused before the cascade.
   `TestStudentDeletionWorkflow_RejectsIncompleteConfirmation` covers the
   confirmation contract, including the reserved purge reason.
3. `TestStudentDeletionWorkflow_AuthorizationAndTenantIsolation` proves that an
   unauthorized principal reaches no owner read, that the composed gate refuses
   a missing principal, and that a child of another tenant is invisible.
4. `TestStudentDeletionWorkflow_CompanionGraphLockAndStrandingCheck` refuses
   the deletion that would strand a linked child and proves that a concurrent
   holder of the far end's row blocks the ascending lock pass instead of being
   skipped. `TestStudentDeletionWorkflow_RemovesCompanionEdgesAndNotifies`
   proves the cascade removes the edge from the surviving child and queues the
   companion broadcast.
5. `TestStudentDeletionWorkflow_GraduatesUsePurgeNotDelete` pins the route
   split under lock (a child graduated after the preview is refused by the
   ordinary deletion; an active child is refused by the purge) and the
   retained, anonymized grade-transition ledger.
   `TestStudentDeletionWorkflow_PreviewExcludesPreservedDeletionAudits`
   proves prior deletion audits and detached access logs are retained.
6. `TestStudentDeletionWorkflow_QueuesDocumentCleanupInsideTheTransaction`
   proves the intents are written before the cascade, are eligible
   immediately, exclude objects whose bytes are already gone, and are
   reactivated rather than duplicated on a repeated enqueue. The existing
   document cleanup worker tests cover the leased removal and its retry.
7. Idempotent retry: a second execution of a committed confirmation answers
   "student not found" and changes nothing (first and second test above); a
   second withdrawal deletion answers "already resolved"
   (`TestStudentDeletionWorkflow_WithdrawalDeletesStudentAndRedactsCompletionAtomically`).

The latency workload contains zero injected failures/conflicts. The cases above
are separate hermetic acceptance tests, not mixed into its percentiles.

## Rollback and limits

Before commit, every reversible owner write, audit append and cleanup intent
rolls back together. After the commit, the person row is an anonymized
tombstone on both the ordinary and the purge path; a deployment rollback of
the workflow wiring must not restore names, tags, account links, deleted
files or credentials. Audit tombstones, prior deletion audits and the
anonymized grade-transition history are retained per policy.
