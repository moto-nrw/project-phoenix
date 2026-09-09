# Emergency snapshot local runtime evidence (#2704)

## Reproduce

Run from the repository root:

```bash
CGO_ENABLED=0 scripts/run-go-toolchain.sh go -C backend test ./modules/emergencysnapshot/legacy -run '^TestEmergencySnapshotRuntimeEvidence$' -parallel 8 -count=1 -v
```

The pre-cutover reader is fixed at `0d93d9163a22b6a117aa010514bb382ce5c9843a`.
Copy [the baseline harness](emergency-snapshot-2704.baseline_test.go.txt) into
`backend/services/emergency/runtime_checkpoint_wave_test.go` in a checkout of
that commit, and run the same command against `./services/emergency`.
The measured baseline checkout had an identical tree to this fixed commit.
The temporary test was removed after measurement; the source is retained here.

Both runs use local PostgreSQL 17.11, Go 1.27.0 and disposable test databases.
Each scenario has five warmups and 30 samples, concurrency one. Fixtures are
outside the timer: 0, 32, then 128 present children, each with an open room
visit and linked guardian, one room/session, one device and one supervisor.
The runtime fixture has no guardian phone rows; separate RLS/output fixtures
include them. Mode is fixed to detailed and the health-column setting to true
on both sides. These measurements cover the real owner reads, tenant
transaction, document shaping and PDF renderer, not settings-service or HTTP
latency. There is no projection cache; cache hit rate is not applicable.

## Results

Times are milliseconds, nearest-rank p50/p95 (15th/29th of 30 sorted samples).

| Scenario | Baseline p50/p95 | Candidate p50/p95 | Queries, both | Returned driver rows, both |
|---|---:|---:|---:|---:|
| Empty document | 1.286 / 1.567 | 1.308 / 1.518 | 5 | 1 |
| 32-child document | 4.602 / 5.016 | 4.520 / 5.084 | 11 | 194 |
| 128-child document | 9.489 / 9.834 | 9.080 / 9.554 | 11 | 770 |
| 128-child PDF | 46.590 / 47.948 | 45.822 / 47.387 | 11 | 770 |

Counts include transaction commit. The existing query-budget assertion runs
inside the transaction and therefore observes 10, not 11, nonempty statements:
three transaction-scope statements and seven owner reads. Driver rows include
repeated facts and the tenant-setting statement, not distinct children or
physical rows scanned. The nonempty snapshots return exactly 32 or 128 children.

All 240 measured calls across both readers succeeded; expected output counts
and PDF signatures passed. All measured DML row counts were zero. Pool wait
count/duration and deadlock deltas were zero, with no sampled lock waiters.
Finite sampler gaps are in the raw records; shorter waits cannot be excluded.
No warning/error lines appeared in either captured runtime log.
The timing differences are descriptive local samples, not proof of a speedup.

[Raw samples and counters](emergency-snapshot-2704.raw.json) retain every
duration, query count, driver row count, pool delta and lock-sampling result.

## Query plans and scanned rows

Full `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON)` plans were captured for all
SELECTs in the 128-child document scenario, outside the timer and counter:
[candidate](emergency-snapshot-2704.plans.json),
[baseline](emergency-snapshot-2704.baseline-plans.json).

The slowest owner statement is the retained guardian-contact query: local
execution 3.088 ms candidate versus 3.179 ms baseline. Its nested loop scans
128 student-guardian rows 128 times in this fixture. Other table scan nodes
read 128 attendance, 128 student rows twice (retained hydration), 128 person,
128 visit and 128 guardian rows, plus one room and repeated single-group reads.
These are scan-node events, not distinct rows. Full plans record loops,
filtered rows and buffers. This inherited join shape is an optional performance
follow-up if larger-school workloads justify it; this cutover does not change it.

## Correctness and limits

The retained document fixture pins column order, labels, contacts, locations,
health values and export metadata. Pure projection tests cover empty/order/error
and binary cases. Real adapter fixtures prove bidirectional tenant RLS for
every named input table, including guardian phone rows, and own-school output.
The direct-download path does not persist `documents.files` metadata.

Routed tests use signed JWTs and the production permission/tenant middleware:
authorized staff succeed; permissionless, parent, school and anonymous callers
are rejected before projection. A body/query tenant selector cannot replace
the token's school. Binary owner failures retain the old stable HTTP 500 text,
without raw database details. Both old and new person readers exclude soft-
deleted identities, verified against the real ORM and facade.

No deployment, staging or production requests occurred. These controlled local
comparisons replace external parity checks for this task and do not establish
production latency, concurrent load behavior, binary-mode timing, or full
header-to-real-PDF latency. Those limits are not presented as passed checks.
