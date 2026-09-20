# Student storage Contract: production observation, 2026-09-20

This is a read-only observation for #2760, not permission to remove storage.
No production migration, counter reset, backup, repair or application write ran.
The implementation and its local migration evidence are not complete.

## Provenance

- Worktree base: `4aa36c315fc19e8ae8dda50193ffa34843e25575`, equal to fetched
  `origin/development` at inspection. Includes #3441's evidence gate.
- Production server and frontend: image tag `ed76e88`, both healthy as reported
  by `docker ps` over SSH on `moto-app-server`.
- Database: `production-postgres-1`, database `postgres`, PostgreSQL 17.
- Connection: SSH, container-local psql, `-X`, `ON_ERROR_STOP=1`,
  `default_transaction_read_only=on`, 30-second statement timeout and
  5-second lock timeout. Each SQL batch used a read-only transaction.
- First batch: the checked-in `student-owner-storage-cutover.sql`, unchanged.
- No query selected the compatibility view. Additional queries inspected
  catalog/statistics metadata and aggregate owner/archive counts only. They
  therefore add maintenance archive reads, but do not advance the view counters.

## Observed results

At **01:44:40 UTC (03:44:40 CEST)**:

| Check | Observed result |
| --- | --- |
| Compatibility reads / writes | 0 / 0, cumulative sequence values |
| Cutover checkpoint | All 12 schools stable, verified at `2026-09-20T00:07:19.503045Z` |
| Frozen source / target counts | 1,448 / 1,448 in total |
| Frozen checksums / row and guardian mismatches | Equal per school / 0 / 0 |
| Current owner profiles / complete live joined rows | 1,448 / 1,448 |
| Hidden, broken, stale archive, missing archive rows | All 0 per school |
| Unvalidated student-profile foreign keys | None |
| Foreign keys still referencing archive | None |
| Database deadlocks / current lock-wait sessions | 0 / 0 |

At **01:44:58 UTC**:

- Care Plan has 1,448 rows, including 16 with `sick = true` and none with
  `excused = true`. The tuple `(sick, sick_since, excused, excused_since)`
  matches the corresponding archive tuple for every joined child: 0 mismatches.
- `active.student_status_days` has 2,830 rows. This is a count, not a new
  checksum of the history. The release's earlier checksum evidence is in
  [#2760](https://github.com/moto-nrw/project-phoenix/issues/2760).
- Historical `care_state_mismatch_count` remains 3, 8 and 3 for schools
  4, 8 and 9. It compares dated status history, not copied absence fields;
  it is diagnostic and is not a Contract storage-loss gate.

At **01:45:42 UTC**, `pg_stat_statements_info` reported a statistics reset at
`2026-06-04T21:07:58.599924Z` and `dealloc = 449`. Old statement entries predate
the cutover. The archive's table statistics also include its life before the
rename, so their cumulative scan totals do not establish post-cutover access.

At **01:46:10 UTC**:

| Application role | Retained statement entries mentioning archive | Old-name entries | Old-name entries whose statistics began after cutover |
| --- | --- | --- | --- |
| phoenix_admin | 0 | 21 | 0 |
| phoenix_auth | 0 | 0 | 0 |
| phoenix_tenant | 0 | 853 | 0 |

Compatibility reads and writes were still **0 / 0**, approximately 99 minutes
after the cutover checkpoint. Statement text and personal data were not exported.

## What this establishes and what it does not

The durable view counters show no recorded compatibility-view usage since their
creation, assuming they have not been reset or replaced. The retained query
statistics contain no direct archive statements for the application roles.
They are not a complete historical audit: entries have been evicted, no
cutover-time statement snapshot is available here, and `stats_since` is not a
last-execution timestamp. Maintenance queries against the archive are expected
and must not be counted as application dependencies.

The code search found the remaining `users.Student` ORM table binding and
test-fixture queries. Current owner adapters use the three target tables.
This preliminary search is not the final AST caller-inventory gate and does
not prove the absence of external consumers.

The observation occurred early on Sunday. It does not exercise a school day,
all scheduled jobs or dormant external consumers. The user asked to evaluate
actual code and production use before choosing a minimum rollback duration;
at the time of this observation no duration or waiver had been agreed. The user
subsequently chose at least 24 hours plus a complete regular school day and
relevant jobs. The 48-hour test fixture remains synthetic input, not the
production policy. See [the agreed rule](student-owner-storage-contract.md#operating-evidence).

Before execution, Contract still needs the agreed operational acceptance,
current caller/query evidence, a fresh restorable pre-Contract backup, and
the local migration, failure, recovery, integrity and public-contract checks.
No production execution is authorized by the implementation request.
