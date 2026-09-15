# Plan export local runtime evidence (#2706)

## Reproduce

Run from the repository root:

```bash
CGO_ENABLED=0 scripts/run-go-toolchain.sh go -C backend test ./modules/planexport/legacy -run '^TestPlanExportRuntimeEvidence$' -parallel 8 -count=1 -v
```

The pre-cutover service is fixed at `939126a901186f7bbaeb984fee5ba7b10771a329`.
Copy [the baseline harness](plan-export-2706.baseline_test.go.txt) into
`backend/services/planexport/runtime_checkpoint_wave_test.go` in a checkout of
that commit, add a `TestMain` that opts into per-test tenants, and run the same
command against `./services/planexport` with `-run '^TestPlanExportRuntimeEvidenceBaseline$'`.
The temporary files were removed after measurement; the harness source is
retained here.

Both runs use local PostgreSQL 17.11, Go 1.27.0 and disposable isolated test
databases. Each scenario has five warmups and 30 samples, concurrency one.
Fixtures are outside the timer: one room, four staff members, then 0, 20 and
80 spontaneous blocks (four per weekday, twenty distinct titles per week, one
staff member per block) over one or four printed weeks. Both readers bind the
same sources: the retained activity-instance and instance-staff repositories,
the public Facilities facade for room names, and a fixture-backed staff-name
reader. Child counts, Planungsspur colours, closing days and holidays stay
unbound on both sides, exactly as an unwired optional source prints. These
measurements cover the tenant transaction, the owner reads, the sheet layout
and, in the last scenario, the PDF renderer; not HTTP latency. There is no
projection cache.

## Results

Times are milliseconds, nearest-rank p50/p95 (15th/29th of 30 sorted samples).

| Scenario | Baseline p50/p95 | Candidate p50/p95 | Queries, both | Returned driver rows, both |
|---|---:|---:|---:|---:|
| Empty week | 1.278 / 1.614 | 1.328 / 1.571 | 5 | 1 |
| One week, 20 blocks | 2.378 / 2.790 | 2.351 / 2.730 | 7 | 42 |
| Four weeks, 80 blocks | 2.920 / 3.185 | 2.915 / 4.208 | 7 | 162 |
| Four weeks, 80 blocks, PDF | 38.347 / 39.656 | 39.662 / 43.051 | 7 | 162 |

Counts include the three transaction-scope statements and the commit; the
non-empty export issues three owner reads (blocks, their staff, the room
names). Driver rows are the rows the statements returned, not distinct
physical rows. The capability issues the same statements as the retained
service: the adapters translate rows, they do not query.

All 240 measured exports across both readers succeeded; the row counts and
the PDF signature were asserted on every sample. All measured DML row counts
were zero. Pool wait count/duration and deadlock deltas were zero, with no
sampled lock waiters. Finite sampler gaps are in the raw records; shorter
waits cannot be excluded. The timing differences are descriptive local
samples inside the run-to-run noise of a laptop, not proof of a change.

[Candidate raw samples](plan-export-2706.raw.json) and
[baseline raw samples](plan-export-2706.baseline.raw.json) retain every
duration, query count, driver row count, pool delta and lock-sampling result.

## Query plans and scanned rows

Full `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON)` plans were captured for all
SELECTs of the four-week document scenario, outside the timer and counter:
[candidate](plan-export-2706.plans.json). The block range read sorts 80
instance rows through an incremental sort on the tenant/date index, the
instance-staff batch read sorts 80 rows, the room read returns one row; local
execution stays under 0.13 ms per statement. The candidate binds the retained
repositories unchanged, so the plans are the baseline's plans.

## Dienstplan

Run from the repository root:

```bash
CGO_ENABLED=0 scripts/run-go-toolchain.sh go -C backend test ./modules/planexport/legacy -run '^TestDienstplanRuntimeEvidence$' -parallel 8 -count=1 -v
```

The Dienstplan reads the retained staff schedule overview, whose shift and
staff sources are Workforce adapters composed only by the legacy repository
factory. The harness therefore reconstructs those two reads over the tenant
transaction the export runs in (one shift range read, one staff read, one
person read per staff member) and binds the instance, assignment and room
reads to the same retained sources as above. Fixtures are one room and four
staff members; each printed week adds one shift per staff member per
weekday and one supervised block per shift, so 20 shifts and 20 blocks per
week. Same sampling as above: five warmups, 30 samples, concurrency one. No
pre-cutover baseline exists for this path because the retained service had
no fixture-only composition either.

| Scenario | p50 / p95 (ms) | Queries | Returned driver rows |
|---|---:|---:|---:|
| Empty week | 3.277 / 3.692 | 11 | 9 |
| One week, 20 shifts, 20 blocks | 4.010 / 4.689 | 13 | 70 |
| Four weeks, 80 shifts, 80 blocks | 4.561 / 5.115 | 13 | 250 |
| Four weeks, 80 shifts, 80 blocks, PDF | 38.737 / 40.274 | 13 | 250 |

Counts include the transaction-scope statements and the commit; the four
per-staff person reads are the harness's substitute for the retained staff
join. All 140 measured exports succeeded with zero DML rows, zero pool waits
and zero deadlocks, with no sampled lock waiters.
[Raw samples](plan-export-2706.dienstplan.raw.json) retain every duration,
query count, driver row count, pool delta and lock-sampling result.

## Scope and limits

The Dienstplan numbers above measure the capability and the overview
service over reconstructed shift and staff reads, not the Workforce-backed
repositories the production root composes; their statements are unchanged
by this ticket. The birthday HTTP adapter and the File Storage object-store
adapter moved packages without a code change to their queries; their route
tests pin the wire contract.

## Correctness

Rendering tests pin every printed cell of both plans against in-memory port
records (63 cases ported unchanged from the retained service). Adapter tests
pin the row-to-record translation field by field, the nil-row and
missing-person handling, error propagation and the malformed-day refusal.
The two-tenant RLS test seeds users.staff, facilities.rooms,
activities.groups, schedule.activity_instances, schedule.instance_staff,
schedule.instance_students, schedule.staff_shifts, schedule.shift_types,
schedule.planning_tracks and schedule.closing_days in two schools, proves
each table invisible across the boundary under the least-privilege role, and
renders each school's Betreuungsplan over the real instance, staff and room
sources with only its own block, room and staff member on the sheet. The same
test renders each school's Dienstplan through the retained staff schedule
overview, with the shift and staff reads bound to the tenant transaction under
test, and finds only the school's own staff member, shift and block on the
sheet. A second two-tenant test, since #2707 in
`modules/filestorage/http/files`, stores one file and queues one cleanup
intent per school through the public routes and proves `documents.files` and
`documents.file_cleanup` (with #2707: every File Storage table) invisible
across the boundary under the least-privilege role.
