# Student Presence #2692: live group and supervisor recovery

This records development verification, not a deployment observation. The change
set is based on `4e25eea702` (merge of PR #3088). It closes the last three
ratchet keys of #2692: the schedule repository's foreign reads of
`active.group_supervisors` and its foreign writes of `active.groups` and
`active.group_supervisors`. The two `database/repositories/facilities` keys
listed in the issue were already absent from `backend/architecture/legacy.jsonl`
at the base commit.

## Cutover

Activity completion and reopen (`services/schedule/instance_service.go`) lock
and restore the live group's supervisors and the group row through
`models/schedule.ActivityRecoveryRepository`. Its implementation in
`database/repositories/schedule/activity_recovery_repo.go` issued those
statements directly. It now depends on a consumer-owned `PresenceRecovery` port
that the Student Presence module satisfies:

| Operation | Facade method | Table |
|---|---|---|
| `lock_open_supervisors` | `LockOpenSupervisors` | `active.group_supervisors` (FOR UPDATE) |
| `lock_supervisors` | `LockSupervisors` | `active.group_supervisors` (FOR UPDATE) |
| `restore_group` | `RestoreGroup` | `active.groups` |
| `restore_supervisors` | `RestoreSupervisors` | `active.group_supervisors` |

Every operation requires the caller's tenant transaction and adds an explicit
`tenant_id` predicate. The restore operations keep the snapshot contract:
the row count must equal the snapshot, otherwise the error names the mismatch
and the surrounding unit of work rolls back. The restore order within
`Restore` is unchanged: group, visits, supervisors, attendance, instance.
The schedule repository keeps only the `schedule.activity_instances` write.

Policy ownership is unchanged: `active.groups` and `active.group_supervisors`
stay with `student-presence`. `scripts/backend-architecture.sh check` passes
with 2338 remaining legacy violations (2341 before), and the composition
surface stays at 818 field/setter targets.

## Reproduce

From `backend/`, using the repository toolchain:

```sh
CGO_ENABLED=0 ../scripts/run-go-toolchain.sh go test ./modules/studentpresence/compose -run TestGroupRecoveryMigrationRuntime -parallel 8 -count=1 -v
```

The test prints environment metadata and JSON samples under
`group-recovery-runtime:`. It uses an isolated hermetic database with one
staff member, room, activity, active group, and supervisor. Before each
iteration the fixture is ended outside measurement. Each flow has five warm-up
iterations followed by 30 measured iterations at concurrency one.

The recorded run used Go 1.27.0 and PostgreSQL 17.11. These numbers are local
smoke measurements, not an equivalent-load before/after comparison or
production latency limits. They do not replace checkpoint #3019.

## Flow measurements

| Flow | Queries/sample | p50 ms | p95 ms | DML rows/sample | Commits | Rollbacks |
|---|---:|---:|---:|---:|---:|---:|
| reopen-recovery | 13 | 25.373 | 64.974 | 2 | 30 | 30 |
| rollback-after-supervisor-restore | 21 | 39.964 | 81.106 | 4 | 30 | 60 |

Each measured iteration runs the full recovery (two locks, group restore,
supervisor restore) and then attempts the same group restore again in a new
transaction. That repeat is rejected with `snapshot mismatch for active group`
and is counted as the duplicate-prevention conflict: 30 per flow, one rollback
each. The rollback flow additionally injects a failure after the supervisor
restore in every iteration, verifies that both rows stayed ended, and completes
the same recovery on retry.

All flows recorded zero unexpected errors, transaction retries, deadlocks,
pool-wait count and pool-wait duration. DML rows are driver-reported successful
statement effects, including statements later rolled back; they are not net
committed rows.

Lock sampling inspected `pg_stat_activity.wait_event_type = 'Lock'` in the
isolated database. The two flows collected 312 and 514 samples and observed no
waiting backends. Maximum sample gaps were 23.861 and 24.546 ms. Sampling
cannot exclude shorter waits.

## Stable operation measurements

Durations come from the facade observer. Each observation reported one store
query. The only errors are the 30 expected duplicate rejections per flow in
`restore_group`.

| Operation | Samples (recovery / rollback flow) | p50 ms | p95 ms | Errors |
|---|---:|---:|---:|---:|
| lock_open_supervisors | 30 / 60 | 1.360 / 1.587 | 5.646 / 3.816 | 0 / 0 |
| lock_supervisors | 30 / 60 | 1.940 / 1.830 | 4.323 / 6.004 | 0 / 0 |
| restore_group | 60 / 90 | 1.942 / 1.568 | 6.113 / 4.086 | 30 / 30 |
| restore_supervisors | 30 / 60 | 1.900 / 1.220 | 4.815 / 4.281 | 0 / 0 |

## Isolation and rollback coverage

- `TestGroupRecoveryPreservesTenantAndRollsBackSnapshotMismatch`
  (`modules/studentpresence/compose/group_recovery_test.go`) seeds an ended
  group and supervisor in two tenants. A foreign group ID and a mixed
  supervisor ID list fail with the mismatch error and leave the own-tenant
  row unchanged. A failure after both restores rolls both back. The retry
  succeeds and leaves the foreign tenant's rows ended. Locks and restores
  outside a transaction fail.
- `TestInstance_Reopen_RollsBackRestorationAndRetries`
  (`services/schedule/instance_service_integration_test.go`) injects a trigger
  failure after each authoritative write of the reopen path (group, visits,
  supervisors, both assignments, instance) plus a failure after the whole
  restore, proves complete rollback, and retries successfully. It runs
  unchanged against the facade-backed repository.
- `TestExpectRestoredRows` keeps the stable mismatch error for the remaining
  instance write in the schedule repository.

## Rollback and cleanup

Reverting the repository wiring restores the previous direct statements; the
table shape is unchanged. The facade methods stay in place after a revert and
have no other callers. No adapter or model was deleted in this wave; the
legacy `GroupSupervisor` and `Group` repositories in
`database/repositories/active` remain the owner's package and are outside
the listed ratchet keys.
