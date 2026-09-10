# Data Import cutover evidence

Issue: [#2708](https://github.com/moto-nrw/project-phoenix/issues/2708).
Implementation baseline: `939126a901186f7bbaeb984fee5ba7b10771a329`.
Measured locally on 2026-09-10, macOS arm64, PostgreSQL 17.11.
This flow-specific evidence supplements accepted checkpoint
[#3020](https://github.com/moto-nrw/project-phoenix/issues/3020).
It is not a staging or production observation window.

## Cutover and ownership

`services/import` holds no repository and issues no SQL. Every accepted row
is committed through the public commands of its owners:

| Row type | Owners invoked |
| --- | --- |
| Student | People Directory (person, student, guardians, links, phone numbers), Care Plan (arrival and pickup schedules), Student Presence (retention consent), Audit Platform (consent history, GDPR import record) |
| Staff | People Directory (person), School Membership (staff, caregiver profile), Workforce (master data, qualifications), Identity & Access (invitation, via the existing invitation service), Audit Platform |
| Class-list entry | School Membership (entry), People Directory (student duplicate check), Audit Platform (entry change, GDPR import record) |
| Opening balance | unchanged Workforce services (#2132) |

The owner ports live in `services/import/ports`; the composition root binds
the production capabilities. `api/import` is unchanged: the same four routes
call the same import services, so there is one switch and no dual-write.

A batch is now validated completely before the first owner command runs.
Each accepted row is applied inside its own PostgreSQL savepoint, so a row
the owners refuse leaves nothing behind while the rest of the batch commits.
The surrounding tenant transaction still commits the GDPR audit record with
the batch, so a failure after the last owner command rolls the whole batch
back across every owner.

Table ownership is unchanged; no migration. The new writable surface is the
Student Presence retention-consent capability over the existing
`users.privacy_consents` table, whose write owner was already
`student-presence`.

## Local runtime workload

Reproduce from the repository root:

```sh
CGO_ENABLED=0 scripts/run-go-toolchain.sh go test -C backend \
  ./services/import/integration -run TestDataImportRuntimeEvidence -count=1 -v
```

The isolated-clone workload runs two warmups followed by ten measured
iterations. Each iteration previews and then imports the same ten-row student
file: every row carries a person, a student with a per-day departure plan and
address, one new guardian with a phone number, a weekly pickup schedule and a
retention consent. Percentiles use nearest rank over the ten measured
samples.

Lossless evidence: [raw JSON](data-import-2708.raw.json).

| Operation | Statements | Write rows | p50 | p95 |
| --- | ---: | ---: | ---: | ---: |
| Preview (10 rows) | 16 | 1 | 4.887 ms | 6.455 ms |
| Import (10 rows) | 236 | 81 | 85.917 ms | 91.213 ms |

Statement and write-row counts were identical in all ten samples of each
operation. The preview's single write row is the GDPR import record the
preview commits, as before the cutover. The import's 81 write rows are the
ten children with their persons, students, guardians, links, phone numbers,
pickup schedules, consents, consent-history entries and the one audit record.

Rows parsed, accepted, rejected, created and updated are reported per run by
the production observer (`phoenix_data_import_rows_total`,
`phoenix_data_import_runs_total`, `phoenix_data_import_duration_seconds`),
labelled by entity and mode only. Every measured run reported ten rows
parsed, ten accepted, zero rejected.

Twenty-four transactions committed across warmups and measured samples, with
zero rollbacks and zero retries. Both operations recorded zero
connection-pool waits in their measured samples. The database deadlock
counter changed by zero. The 360 lock-acquisition statement events are the
existing hook's SQL acquisition durations, an upper bound on lock wait, not
wait-only measurements.

## Coverage limits

Local development evidence on an isolated database clone, single writer, no
concurrency. It measures the ten-row student file only; the staff and
class-list imports are covered by behaviour tests, not by this workload. The
opening-balance import was not re-measured because its owner path is
unchanged. There is no checkpoint or resume state to measure: an import is
one batch in one transaction, and a failed batch is retried by re-uploading
the file.
