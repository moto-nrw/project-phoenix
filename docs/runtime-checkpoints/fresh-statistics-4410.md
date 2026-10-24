# Fresh-statistics measurement condition (Refs #4410)

The runtime checkpoint harness now runs a full `ANALYZE` on its disposable
fixture database after setup and before warm-ups and counters. Raw evidence
labels this condition **fresh statistics**. Ordinary tests, template sources,
production, workload versions, fixture builders, budgets and comparator
tolerances stay unchanged.

This is a measurement-series break, not a new checkpoint acceptance. The
[register](https://github.com/moto-nrw/project-phoenix/blob/46506b00c695a6f102865cae2dc9f24010371b7c/backend/architecture/runtime-checkpoints.json) retains the
accepted #3019, #3020 and #3021 records and documents the boundary. Their
original evidence remains valid for its original condition, without an explicit
fixture `ANALYZE`. Cross-boundary latency claims need a paired bridge or an
explicit qualification. This change does not measure populated-inbox capacity.
Later stateful request preparation and worker writes are unchanged; statistics
are refreshed once after initial setup, not continuously.

## Paired bridge

Source: `ee90b8cb1da8034b36e728029bfd670ec3ffbbf8` plus this change. A temporary
Go overlay removes only the new `ANALYZE` and labels A's statistics condition;
B uses the changed harness without an overlay. No workload or assertion is
changed. Order: **A1 B1 A2 B2 A3 B3**, fresh process and output directory each,
three internal runs, five warm-ups and 30 measured requests per scenario,
`checkpoint-1-v1`, serial concurrency one, `GOMAXPROCS=4`, `-count=1 -parallel 8`,
`testhooks`. All six complete production-router tests passed, exit 0.

Both sides use pinned Go 1.27.2, macOS/arm64 and the same PostgreSQL 17.11
loopback server, configured from this worktree's `.env` only. Recorded settings
include 128 MiB `shared_buffers`, 4 MiB `work_mem`, 300 connections, disabled
fsync/synchronous commit and Europe/Berlin. The production auth pool has 12
connections. SMTP variables are unset. These are same-environment controls,
not replacements for older captures or their environment contracts.

`communication.messages`, milliseconds:

| Pair | A internal p50 | B internal p50 | Median p50 A → B | Worst p95 A → B |
| --- | --- | --- | --- | --- |
| 1 | 4.277 / 4.098 / 4.077 | 2.820 / 2.404 / 3.352 | 4.098 → 2.820 | 4.833 → 5.019 |
| 2 | 4.505 / 4.186 / 4.324 | 2.471 / 2.308 / 2.414 | 4.324 → 2.414 | 4.937 → 3.027 |
| 3 | 4.197 / 4.901 / 4.321 | 2.424 / 2.644 / 2.668 | 4.321 → 2.644 | 10.880 → 3.352 |

Every capture retains 5 statements per inbox call, 150 statements and 30
driver-reported rows per inbox run. Every serial HTTP run has 3,870 statements,
5,430 driver rows and 2,550 statements with row counts. Fixture inventories
match across all six captures. The unchanged comparator (20% and 0.5 ms)
reports **no material non-latency changes** across any pair, including outcomes,
row counts, pool/lock waits, deadlocks, retries, rollbacks and worker counters.
The bundle includes every metric's median/worst classification, not just inbox
results.

Host snapshots show an active Time Machine backup and roughly one CPU core
of `fseventsd`; no system process was stopped. A1's retained host snapshot is
post-capture, taken when reporting resumed; the other snapshots precede their
processes. Pair 1 has 47 adverse latency classifications, pair 2 has one, and
pair 3 has seven. These captures do not establish quiet-host latency stability
or explain every tail. The causal evidence below is the query plan and its
work, not an attribution of all timing changes to statistics.

## Query-plan control

Two separate diagnostic production-router processes, A then B, also passed,
exit 0. After each internal run's measured inbox window, an overlay takes the
SQL recorded by the existing query counter and executes
`EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON)` outside timing, under
`phoenix_tenant` and the same fixture tenant. The SQL shape is identical across
A/B; only the process-specific fixture account ID differs. These are companion
plan probes, not extra timed samples or plans captured inside a real request.
The original six measurements contain no plan probes.

| Diagnostic | Internal run | Profile lookups / loops | Planning ms | Execution ms |
| --- | --- | --- | --- | --- |
| A, no explicit statistics | 1 | 3,100 primary-key probes | 2.386 | 1.776 |
| A, no explicit statistics | 2 | 3,100 primary-key probes | 4.786 | 1.864 |
| A, no explicit statistics | 3 | one bitmap scan | 1.423 | 0.556 |
| B, fresh statistics | 1 | 0, branch never executed | 2.249 | 0.138 |
| B, fresh statistics | 2 | 0, branch never executed | 1.500 | 0.162 |
| B, fresh statistics | 3 | 0, branch never executed | 5.012 | 0.266 |

A reproduces the two-plan pattern from
[#4401](https://github.com/moto-nrw/project-phoenix/issues/4401#issuecomment-6277821151):
50 memberships × 62 persons, 3,100 profile probes, then a different plan in the
third run. B keeps the same early-empty plan in all three probes. Its empty
thread scan ends the join before scanning profiles or persons. This supports
the chosen measurement condition without changing SQL, adding an index or
claiming an optimization of populated data.

## Evidence and local setup

[Lossless evidence bundle](fresh-statistics-4410.evidence.tar.xz) contains all
eight original captures, logs, reporting copies, summaries, three comparator
outputs, complete diagnostic plans/SQL, host snapshots, commands/exit codes,
overlay sources, source patch, provenance and SHA-256 manifest. Raw captures
are untouched. Reporting copies replace only UUID-valued error `instance`
fields with a fixed UUID, as in #4401. Verification checked 3,600 error samples
and deep equality of every other field, including every numeric measurement.
The reporter initially rejected the unnormalized request IDs; that failure is
retained. A prior wrong-working-directory attempt failed later goldens and is
retained separately, excluded from the eight successful captures.

The worktree's new test server required template setup. To obey the explicit
ban on `ALTER ROLE`, a local-only setup overlay omitted role password rewrites,
used a database-level search path instead of the migration's role-level search
path, and left the unused demo role without a password. The normal clone path
sets its database search path before fixtures. Both A and B use this same
template, whose migration hash is `38fe315bd58803eff761e2c29b2c5865`.
No role was altered, no tracked template/migration source changed, and no
setup overlay is part of the patch. The dedicated captures use the existing
`PHX_TEST_TEMPLATE` path without an infrastructure overlay. Local changed-test
and review gates use only the bootstrap role-pin overlay to avoid a redundant
password write; test assertions and the three affected packages stay unchanged.

Affected owners: `root-composition` (opt-in API test) and `architecture-check`
(registry decoding). Neither has an entry in the module pass-through register.
All registered owner budgets are unchanged; total **2,924 → 2,924**. No policy
rule, alias, exception, tolerance or query budget changes.
