# Device scan local runtime evidence (#2698)

## Workload and reproduction

Run from the repository root:

```bash
CGO_ENABLED=0 scripts/run-go-toolchain.sh go -C backend test ./api/iot/checkin -run '^TestDeviceScanRuntimeEvidence$' -count=1 -v
```

Measured locally on 2026-09-09 with Go 1.27.0 and PostgreSQL 17.11,
against base `8d340bd86f1c44b8004127358af1025eac9261ea`.
Each operation has five warmups followed by 30 measured calls, concurrency one.
The test uses a disposable per-test database and the real composed workflow,
tenant transaction and owner commands. Fixtures and assertions are outside
the timer and query-count context. Each fixture contains one device, student,
staff supervisor and destination session; transfer/checkout also have an
existing source visit. A duplicate supervisor is already assigned to the session.
No student names are included in measurements.

## Results

Times are milliseconds. DML counts report affected rows inside the transaction,
not committed rows: the rollback case commits none.

| Operation | p50 | p95 | Max | Queries/call | DML rows/call |
|---|---:|---:|---:|---:|---:|
| check-in | 13.012 | 20.343 | 26.958 | 40 | 3 |
| room-transfer | 14.756 | 29.787 | 31.100 | 47 | 4 |
| check-out | 6.694 | 8.257 | 10.552 | 22 | 1 |
| duplicate-supervisor | 4.087 | 5.013 | 5.082 | 14 | 0 |
| rollback-after-scan | 13.176 | 15.496 | 19.248 | 40 | 3 |

Every operation returned its expected action in all 30 samples. Duplicate
supervisor scans returned `supervisor_authenticated` without duplicate writes.
All 30 injected late failures rolled back the created visit. There were zero
unexpected errors or conflicts. The captured run contained zero warning/error
log lines, including zero best-effort device-location warnings. The separate
failure-injection decision tests exercise that warning path; it is not a
measured forced-failure workload here.

Pool wait-count and wait-duration deltas were zero, as were database deadlock
deltas. The lock sampler observed no waiting backend. Maximum polling gaps
were 2.799–8.837 ms; shorter lock waits cannot be excluded. Sampling spans
fixture gaps between calls, while durations and query counters do not.
Nearest-rank p50/p95 use the 15th/29th sorted samples. Raw samples and
result-code counts: [device-scan-2698.raw.json](device-scan-2698.raw.json).

## Correctness evidence and limits

`TestScanPreservesEveryNamedTableAcrossTenants` populates all seven issue
tables in two schools, checks least-privilege RLS in both directions, performs
a real transfer, injects failure after the writes, compares full-row snapshots,
and retries successfully. Every foreign row is unchanged; group mappings,
supervisors, scheduled checkouts and daily attendance are preserved by the
successful room transfer. This does not claim every scan mutates every table.

`TestComposedWritesRequireTenantTransaction` verifies that the public write
entrypoints reject calls without an ambient transaction. The HTTP middleware
supplies that transaction. Existing kiosk route tests cover capacity rollback
and error wire data; device-authentication tests cover headers separately.

These measurements are not HTTP latency, a concurrent load test, a historical
before/after comparison, or a staging/production SLO check. No external moto
requests were made. Binary-mode behavior and forced location-write failures
have functional tests but no separate latency distribution in this workload.
