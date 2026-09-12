# Data Import bounded-batch completion

Issue: [#2708](https://github.com/moto-nrw/project-phoenix/issues/2708).
Code baseline: `8f1f4d78a0`; measurements include this working-tree cutover.
Measured locally on 2026-09-12, macOS arm64, PostgreSQL 17.11.
This ordinary migration supplements accepted checkpoint
[#3020](https://github.com/moto-nrw/project-phoenix/issues/3020), not a new
checkpoint acceptance or a staging/production observation.

## Cutover and replay contract

All four supported row types now enter `ImportBatches` from the actual API:
students, staff, class-list entries and opening balances. Parsing finishes in
the handlers. The workflow validates the entire upload in a short tenant
transaction before committing write batches of at most 100 rows. Preview
remains validation-only, with its existing required audit record.

Every write uses an owner command. The opening-balance path now consumes
Workforce's public `OpeningBalanceBookings` and `VacationTakeovers`
capabilities, not `services/active` types. The existing Workforce provider
retains its ledger implementation. The importer still has read-only legacy
lookups, but no writing repository or SQL executor.

Each committed batch appends its receipt to `audit.data_imports.metadata`
through Audit Platform, in the same UnitOfWork as the owner commands.
Migration 1.15.384 adds a unique, tenant-scoped checkpoint index; it adds no
table and changes no owner. An owner or audit failure rolls back the whole
current batch, including earlier rows in that batch. Earlier batches remain
committed.

Re-uploading identical parsed input, filename, mode, actor and request options
under the same tenant resumes from the last receipt. Parser-only permission
and column-presence flags participate in the identity without being exposed
in public row JSON. The caller's real-import rows are not mutated. Changed
input or options are a different import and use the existing duplicate guards.
The checkpoint also verifies the processing order. Mutable match caches are
reloaded for every transaction attempt.

A tenant-scoped advisory lock serializes identical concurrent uploads.
History is reread after acquiring it, so a waiting request catches up rather
than executing already committed rows. Database deadlocks and serialization
conflicts retry the entire current UnitOfWork. A completed replay restores
its recorded counts and row errors, without new owner writes or audit rows.
Current authorization and tenant reference access are checked on every run.

An unsuccessful actual import remains HTTP 500. A nonnil partial result is
retained in `details.result`, with code `import_batch_failed`, including
committed counts and row errors. It is not reported as a successful upload.
Permission-denied staff modes remain 403; opening actor-resolution failures
remain distinct from transaction infrastructure errors.

## Verification scenarios

| Scenario | Observable contract |
| --- | --- |
| 205 class-list rows, second batch fails at file row 152 | 100 entries and one receipt survive; 50 preceding writes in the failed batch roll back; a new service resumes to 205 entries and three receipts |
| Completed identical replay | Same committed outcomes, no new rows or receipts; runtime created/batch counters stay zero |
| Two concurrent identical 205-row uploads | Both return 205 created; database and combined runtime counters contain 205 entries and three committed batches |
| One injected 40P01 or 40001 after the first row | Whole two-row transaction retries once; one receipt, no duplicate rows; one failed command attempt is observed |
| Invalid final row in a 205-row file | Preview writes only its audit; actual import accepts 204; replay preserves serialized row errors and timestamps |
| 101 student rows sharing a guardian | One guardian and 101 links across two batches; completed replay writes nothing |
| Create-mode student row whose name+class matches more than one existing child | The clash is a `duplicate_check_failed` row error; later valid rows in the same 100-row unit of work still commit; identical replay restores that receipt |
| Absent versus explicit false guardian flags | Absent cells preserve flags; explicit false revokes them; the otherwise identical JSON inputs do not share a checkpoint |
| Student owner insert failures | Person, student, consent, guardian, phone, link, arrival, pickup and consent-history failures each roll back owner writes; replay succeeds after failure removal |
| Staff owner insert failures | Person, staff, caregiver, master data, qualifications and invitation failures each roll back; replay creates each once |
| Opening-balance owner failures | Hours, quota, vacation opening and checkpoint-audit failures roll back both ledgers; preview writes no ledger rows; replay books and restores correct quota-derived values |
| Audit checkpoint adapter isolation | Real RLS plus explicit tenant filtering, missing-tenant rejection, duplicate receipt rejection and transaction rollback |

Legacy owner-level create/update, duplicate matching, cross-tenant and preview
tests remain in place. The opening workflow fixtures use injected calendar
clocks to make current-year and cutoff checks independent of the test date.

## Local runtime evidence

Reproduce from the repository root:

```sh
CGO_ENABLED=0 scripts/run-go-toolchain.sh go test -C backend \
  ./services/import/integration -run TestDataImportRuntimeEvidence -count=1 -v
```

[Raw samples](data-import-batches-2708.raw.json) contain two warmups and ten
measured samples per operation, at concurrency one. Each iteration previews
and imports the same ten-row student fixture used by the
[earlier partial cutover](data-import-2708.md). Every row has a person,
student, address/departure plan, guardian/phone/link, pickup schedule and
retention consent. Percentiles are nearest-rank.

| Operation | Statements | Write rows | p50 | p95 |
| --- | ---: | ---: | ---: | ---: |
| Preview, 10 rows | 16 | 1 | 5.088 ms | 5.371 ms |
| Import, 10 rows | 244 | 81 | 79.006 ms | 105.675 ms |

Statement and write counts are identical across all measured samples.
Against the earlier local artifact, preview stays at 16 statements and import
grows from 236 to 244: bounded transaction setup, checkpoint reads/locking
and refreshed preload account for the additional work. The same 81 write
rows now include a checkpoint in the existing audit row. These were not
side-by-side runs on the same commit, so latency differences are descriptive,
not evidence of a speedup or regression.

All 24 runs, including warmup, report ten parsed/accepted rows and no rejected
rows. The 12 actual imports each report one committed batch, no retries and
zero checkpoint lag. There are 36 committed UnitOfWork transactions:
12 previews, 12 validations and 12 write batches. The database deadlock delta
and measured driver pool-wait counts/durations are zero.

The raw observer records 1,092 owner-command attempts, with zero failures.
Per-command duration, owner and operation are retained, including no-op consent
history calls. Retry fixtures separately prove failed attempts are counted.
No filename, account, tenant ID, row content or checkpoint key is a metric label.

Production exports existing import row/run/duration metrics plus:

- `phoenix_data_import_batches_total`: committed, retried and deadlock counts.
- `phoenix_data_import_checkpoint_lag_rows`: unprocessed rows after the last durable receipt.
- `phoenix_data_import_owner_commands_total` and
  `phoenix_data_import_owner_command_duration_seconds`: owner attempts, failures and latency.
- `phoenix_data_import_wait_seconds`: cumulative UnitOfWork pool and lock acquisition observations.

The runtime lock evidence has 372 acquisition events. These include the
existing SQL hook's acquisition-statement durations, an upper bound rather
than wait-only lock measurements. UnitOfWork pool acquisition durations can
be nonzero even when the driver reports no queued pool wait. Neither should
be interpreted as sampled PostgreSQL contention time.

Acceptance bounds for this local fixture: exactly ten accepted/no rejected
rows; 16 preview statements and no more than 250 import statements; 1/81
write rows; one committed actual batch with zero lag; no owner errors,
retries or deadlocks in the uncontended workload. All observed samples satisfy
these bounds. The concurrency, failure and retry assertions are behavioral
tests, not a load benchmark. No production throughput claim is made.

## Rollback and cleanup

1. Stop admitting new import requests and let the active batch finish, or
   cancel it so its transaction rolls back.
2. Revert the four actual-import call sites to the retained owner-command
   importer with its former handler-owned transaction and audit wiring.
   Do not run both paths for an upload.
3. Retain `audit.data_imports` rows and checkpoint metadata. Do not truncate,
   rewrite or replay committed batches manually. The index migration's Down
   drops only the index, never receipts or audit data.
4. When restoring this cutover, deploy the checkpoint index before admitting
   batched imports. Re-upload the identical file/options/identity to resume.
   If the legacy path ran meanwhile, reconcile those imports before resume:
   legacy runs do not advance batch receipts.

There is no new direct writer to keep as a rollback path. The retained
`Import` entry point already uses owner commands; actual API routes have one
active path. Ownership is unchanged. The legacy ratchet shrinks from 1,630 to
1,628 by removing both import-to-active edges; composition remains 793.

This is a backend transaction and capability cutover. Screens, controls,
upload steps, labels and help screenshots are unchanged; the help-guide rule's
backend-only exemption applies. Deployment, staging smoke checks and a
production observation window are not part of this local evidence.
