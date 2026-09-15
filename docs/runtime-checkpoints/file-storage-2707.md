# File Storage local runtime evidence (#2707)

## Reproduce

Run from the repository root:

```bash
CGO_ENABLED=0 scripts/run-go-toolchain.sh go -C backend test ./modules/filestorage/http/files -run '^TestFileStorageRuntimeEvidence$' -parallel 8 -count=1 -v
```

The pre-cutover routes are fixed at `219273c63099a57010b1482507e9810ad160383e`
(the merge base of this change). Copy
[the baseline harness](file-storage-2707.baseline_test.go.txt) into
`backend/api/filestore/runtime_evidence_test.go` in a checkout of that commit
and run the same command against `./api/filestore`. The harness is the
candidate's test file with only the package clause changed; the route helpers
it drives (`setupFileStoreRoute`, `createFolder`, `upload`, `assignRole`) exist
under the same names on both sides. The temporary worktree was removed after
measurement; the harness source is retained here.

Both runs use local PostgreSQL 17.11, Go 1.27.0 and the disposable per-package
test database with per-test tenants. Each scenario has five warmups and 30
samples, concurrency one, measured through the real Chi router with the tenant
runtime middleware, the JWT chain and the local storage backend. Fixtures are
outside the timer: one manager (`admin:*`), one member (`users:read`) holding
one school role, 20 folders (seven `all_staff`, seven `admins`, six `selected`
shared with that role and the member's account) and 20 PDF files in the
listed folder. The candidate's earlier run happened while the full changed
package suite was executing on the same machine and was discarded; the
numbers below come from an idle machine.

## Results

Times are milliseconds, nearest-rank p50/p95 (15th/29th of 30 sorted
samples). Query counts include the transaction-scope statements of every
transaction the route opens and are identical across all 30 samples of a
scenario (the harness fails otherwise).

| Scenario | Baseline queries | Candidate queries | Baseline p50/p95 | Candidate p50/p95 | Returned rows, base/cand | Write rows | Errors, base/cand |
|---|---:|---:|---:|---:|---:|---:|---:|
| `GET /files/folders` as manager | 10 | 11 | 2.506 / 2.929 | 2.791 / 3.137 | 34 / 34 | 0 | 0/35 / 0/35 |
| `GET /files/folders` as member | 6 | 8 | 1.774 / 2.567 | 2.188 / 4.588 | 14 / 15 | 0 | 0/35 / 0/35 |
| `GET /files/folders/{id}/files` (20 files) | 10 | 9 | 2.653 / 2.860 | 2.211 / 2.538 | 23 / 22 | 0 | 0/35 / 0/35 |
| `GET .../files/{id}/download` as member | 7 | 9 | 1.901 / 2.082 | 2.166 / 2.502 | 3 / 4 | 0 | 0/35 / 0/35 |
| `POST /files/folders/{id}/files` (PDF) | 23 | 25 | 4.517 / 5.165 | 5.096 / 5.789 | 11 / 11 | 4 / 4 | 0/35 / 0/35 |

The error column counts non-expected statuses over all 35 requests of a
scenario (warmups included); the harness fails on the first one, so a
non-zero rate would have aborted the run.

All 350 measured requests on both sides answered with the expected status
(200, 201 for the upload); no read scenario wrote a row on either side, and
the upload writes the same four rows (intent, file row, intent settlement,
audit event). Pool wait count and duration deltas were zero for every sample,
the lock sampler saw no waiting backend in any sample of any scenario, and
the deadlock counter did not move. Finite sampler gaps are in the raw
records; shorter waits cannot be excluded. The timing differences are
descriptive local samples inside the run-to-run noise of a laptop, not proof
of a change.

## Where the statement counts moved

Each raw record lists the statement shapes of one sample (verb and first
table), so the differences are observed, not inferred:

- The folder visibility rule used to be one SQL fragment that read
  `auth.account_tenants` and `auth.account_roles` inline (three
  `tables.foreign-read` findings under #2721). The owner now asks Identity &
  Access for the two facts it needs, the viewer's active membership
  (`SELECT auth.account_tenants`) and, for a non-manager, the viewer's role
  ids at this school (`SELECT auth.account_roles`), and applies the share
  lists with owner SQL over its own tables. That is the one extra statement
  on the manager list and the two extra statements on the member list and
  the member download.
- The file list runs one statement fewer: the retained handler resolved the
  folder's visibility a second time for the cleanup retry lookup (two more
  `documents.folders` reads), while the owner reuses the folder it has
  already checked and only adds the membership read.
- The upload gains the membership read in both its authorization
  transaction and its metadata transaction; the intent, the quota lock, the
  settings read, the quota sum, the insert, the intent settlement and the
  audit append are unchanged.

Every count is fixed per scenario; no statement scales with the number of
folders or files.

## Correctness

The route tests moved with the handlers and pin the unchanged wire contract:
string ids, folder visibility per viewer, files:manage gating, the staff
upload setting, upload, inline and attachment download headers, delete
rights, the audit trail count, the folder-delete cleanup intents and the
sweep, and the attachment lifecycle (limit, published draft, unknown
announcement, admin gate). The two-tenant RLS test seeds a shared folder with
a role and an account on its share list, one file, one folder-delete cleanup
intent and one announcement attachment per school, and proves
`documents.folders`, `documents.folder_roles`, `documents.folder_accounts`,
`documents.files`, `documents.file_cleanup`,
`documents.announcement_attachments` and
`documents.announcement_attachment_cleanup` invisible across the tenant
boundary under the least-privilege role.

[Candidate raw samples](file-storage-2707.raw.json) and
[baseline raw samples](file-storage-2707.baseline.raw.json) retain every
duration, query count, row count, pool delta and lock-sampling result.
