# Session-end integration evidence (#2697)

## Scope and environment

Measured locally on 2026-09-09 after rebasing onto
`cb482db372f2ec24859ae516364c9b7424c8804c`, fixing Cancel's lock order and
restoring the active-supervision date predicate identified by Quorum.
Go 1.27.0, PostgreSQL 17.11, isolated test clone, sequential concurrency 1.
Five warmups and 30 measured calls per operation.

The real Active test composition binds the session-end workflow, Presence,
Timetable and retained completion provider. Each fixture has a mirrored active
group, one supervisor, one present student with an open visit/check-in and one
expected student. Setup and verification are outside the timer and query context.
After-commit local broadcast work is inside the capability call.

This is a capability measurement, not an HTTP, TCP, staging or production
benchmark. Kiosk HTTP behavior is covered separately by
`TestEndSession_ClosesPresenceAndMirroredInstance`.
There is no equivalent historical workload for a before/after latency claim.
The accepted checkpoint remains #3020; this does not accept another checkpoint.

## Observations

| Operation | Queries | DML rows | p50 | p95 | Unexpected errors |
|---|---:|---:|---:|---:|---:|
| End mirrored session | 34 | 5 | 12.474 ms | 15.766 ms | 0/30 |
| Reject already-ended session | 5 | 0 | 1.290 ms | 1.468 ms | 0/30 |

All 30 repeated calls returned the stable `ErrSessionAlreadyEnded` error.
No pool waits or deadlock-counter increments occurred. Two-millisecond lock
polling observed no waiting backends, with maximum sampling gaps of 8.626 ms
and 2.931 ms respectively. The polling window also includes unmeasured fixture
setup between samples; it cannot prove absence of shorter waits.
Raw samples: [session-end-2697.raw.json](session-end-2697.raw.json).
These are observations, not fitted performance thresholds.

## Failure, concurrency and retained boundaries

`TestSessionEndSerializesWithTimetableCancel` reproduced SQLSTATE 55P03:
Cancel held assignment rows while the kiosk workflow held the group, and each
needed the other's rows. Cancel now takes the group before attendance, matching
Complete and the workflow. The test runs real Cancel and EndSession, proves the
workflow completes while Cancel waits, and proves Cancel rejects the completed
group afterwards. This does not claim general deadlock freedom.

Workflow integration tests cover owner failure/caller rollback, after-commit
notifications, no mirror, two-tenant isolation and repeat-close rejection.
The kiosk contract test exercises actual mirrored-instance completion and
foreign-tenant assignment preservation.

`TestSessionEndPreservesAttendanceAndStaffAcrossTenants` populates every
named table in both schools, verifies RLS in both directions, and compares
full-row snapshots after an injected outer rollback following real owner writes.
After a successful retry, all columns of daily attendance and staff assignments
remain unchanged, as do every foreign school's fixture rows.

`TestSessionEndPreservesActiveSupervisionDateSelection` reproduces the
Quorum finding for both single and bulk close. Supervisors whose start date
has arrived and whose end date is absent or later than the closing day are
ended. Future-start, already-ended and historical rows remain byte-identical.
Both entrypoints failed before restoring this legacy predicate and pass after.

The retained Timetable bridge delegates mutations to owner capabilities.
The later operational-state schema split remains #2762, not a second writer.
No schema changes or irreversible cleanup occur here. Before commit, failure
rolls back the shared unit; rollback of deployment wiring retains the same
table shape.

## Reproduce

From the repository root:

```sh
CGO_ENABLED=0 scripts/run-go-toolchain.sh go -C backend test ./services/schedule -run '^TestSessionEndRuntimeEvidence$' -count=1 -v
CGO_ENABLED=0 scripts/run-go-toolchain.sh go -C backend test ./services/schedule -run '^TestSessionEndSerializesWithTimetableCancel$' -count=1
```
