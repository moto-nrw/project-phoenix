# Grade transition migration evidence

Issue: [#2711](https://github.com/moto-nrw/project-phoenix/issues/2711).
Implementation baseline: `082fb9ac61487c56c2fe30ebc1b9a62a3d87854c`.
Measured locally on 2026-09-16, Go 1.27.0, macOS arm64, PostgreSQL 17.11.
This flow-specific evidence supplements the accepted checkpoint
[#3020](https://github.com/moto-nrw/project-phoenix/issues/3020#issuecomment-5591674205).
It is not a staging or production observation window.

## Cutover and ownership

`workflows/gradetransition` is the only grade-transition coordinator. The
admin HTTP surface (`/api/admin/grade-transitions`: list, create, get,
update, delete, `/classes`, `/suggest`, `/{id}/preview`, `/{id}/history`,
`/{id}/apply`, `/{id}/revert`) calls its public queries and commands; every
path, status code, error code and message is unchanged. The previous
`services/education` grade transition service with its class-ledger
helpers, the cross-schema `database/repositories/education` transition
repository, the `models/education` repository contract and the
handler-side transactions are removed. School Structure, People Directory,
School Membership, Timetable & Activities, Student Presence, Enrollment and
Audit perform their own reads and writes; the workflow contains no SQL or
repository imports.

Lock order: the People Directory class-writes gate exclusively, then the
timetable recurrence gate, then the School Structure transition gate; then
every child of the mapped classes FOR UPDATE in ascending id order with a
re-validation under the lock, a cohort re-read that refuses late arrivals,
and the confirmed fingerprint comparison before the first mutation. Draft
edits take only the transition gate. The revert additionally locks the
latest applied transition and refuses any other target.

The two ratchet keys the ticket lists
(`tables.foreign-read` of `database/repositories/users` on
`education.grade_transition_history` and
`schedule.grade_transition_roster_removals`) were already absent at the
baseline; #2710 had removed the last reader. This cutover removes fifteen
further baseline entries (the admin adapter's `database/sql`, `auth/jwt`,
`models/base`, `models/education` and `services/education` imports, the
education repository's `models/schedule`, `models/users`, `repositories/
base`-adjacent and `tables.unresolved` findings, the service's `crypto/
sha256` import, and four external-test imports of `services/education`) and
introduces no key. Existing table ownership is unchanged; the School
Structure capability grows inside its existing owner. Workflow registration
uses [ADR 0017](../adr/0017-grade-transition-is-an-application-workflow.md)
and raises the policy epoch from 9 to 10. The domain/platform owner counts
stay 18/10. The composition inventory shrinks from 765 to 761 field/setter
targets and the legacy baseline from 1240 to 1225 entries.

## Local runtime workload

Reproduce from the repository root:

```sh
CGO_ENABLED=0 scripts/run-go-toolchain.sh go test -C backend \
  ./services/education -run '^TestGradeTransitionRuntimeEvidence$' -count=1 -v
```

The isolated-clone workload runs five warmups followed by 30 sequential
samples per operation. Each transition promotes one child and graduates one
child who holds one future timetable assignment. It uses the real owner
composition and the ambient-transaction audit command. Fixture creation is
outside the timed/query-counted region. Percentiles use nearest rank.

Lossless evidence: [raw JSON](grade-transition-2711.raw.json).

| Operation | Statements | p50 | p95 | Successful DML rows | Errors |
| --- | ---: | ---: | ---: | ---: | ---: |
| Preview | 9 | 2.469 ms | 2.879 ms | 0 | 0/30 |
| Apply | 28 | 8.364 ms | 9.551 ms | 7 | 0/30 |
| Revert | 25 | 7.167 ms | 8.192 ms | 5 | 0/30 |

Apply includes the three gates, the locked cohort re-read, the bracelet
release, the history and ledger appends, the class and status writes, the
roster archive, the baseline marker and the status transition. Revert
includes the class and status restores, the bracelet restore, the archive
replay and the ledger replays. DML counts are driver-reported statement
rows, not distinct entities. Every operation had zero connection-pool waits
in its measured samples. The database deadlock counter changed by zero.

Including warmups, the trace records 140 committed transactions and zero
rollback events. There are 490 lock-acquisition statement events totalling
116.2 ms. The existing hook measures SQL acquisition duration, an upper
bound on lock wait, not wait-only time. Separate contention tests observe
actual blocked database sessions.

No latency SLO or query budget is specified by #2711. The retained
`services.education.suggest_mappings.reads` budget still holds. These
values are observations, not a claim of improvement over the old provider.

## Failure, conflict and cleanup evidence

1. `TestGradeTransitionWorkflow_RollsBackAfterEachOwnerCommand` injects a
   failure after each of the twelve owner commands (bracelet release,
   history append, graduation, promotion, class-teacher removal and ledger,
   class-list removal and ledger, roster archive, offering-roster resync,
   roster baseline, status transition). The draft, both children, the
   bracelet, the Klassenlehrer assignment, the class-list entry, the roster
   row and the audit trail survive untouched; the same confirmed preview
   applies in full once the owner recovers.
2. `TestGradeTransitionWorkflow_RejectsStalePreviewBeforeAnyMutation`
   covers preview/execute drift for a child entering a mapped class, a
   mapping edit and a garbled fingerprint; each is refused before the first
   mutation, and the restored draft applies with its original digest.
   `TestGradeTransitionWorkflow_Apply_RejectsStalePreview` and
   `TestApplyRefusesChildAddedAfterCohortSnapshot` (workflow decision test)
   cover the fingerprint contract and the late-arrival refusal under the
   locks.
3. `TestGradeTransitionWorkflow_AuthorizationAndTenantIsolation` proves that
   an unauthorized principal reaches no owner read, that the composed gate
   refuses a request without a tenant principal for every operation, and
   that a transition of another tenant is invisible to every query and
   command.
4. `TestGradeTransitionWorkflow_LockOrderSerializesConcurrentWriters`
   observes blocked database sessions: an apply waits behind a shared
   class-writes holder, a draft edit waits behind the transition gate, and a
   revert waits behind the recurrence gate, each completing once the holder
   commits. The ported check-in guard, roster reconciliation, RFID, class
   ledger and offering-resync tests cover the remaining owner interactions.
5. `TestGradeTransitionWorkflow_IdempotentRetryAndHistoryRetention` proves
   that a repeated apply, a draft edit or deletion of an applied transition,
   a repeated revert and an apply of a reverted transition are answered as
   stale-state conflicts without changes, and that the history and both
   ledgers are retained after the revert with applied_by and reverted_by
   recorded.
6. Cleanup intents: not applicable. A grade transition performs no
   irreversible work; graduation is a soft delete, every rewrite is
   ledgered, and nothing is enqueued after commit. The student deletion
   workflow (#2710) owns the durable cleanup of a purged graduate.

The latency workload contains zero injected failures/conflicts. The cases
above are separate hermetic acceptance tests, not mixed into its percentiles.

## Rollback and limits

Before commit, every owner write, ledger append and audit row rolls back
together with the transition status. A deployment rollback of the workflow
wiring restores the previous provider without data migration: the tables
and their rows are unchanged, and an applied transition remains revertable
through either provider's history and ledgers. No credentials, files or
deleted rows are involved; a purged graduate is never resurrected by a
revert.
