# Data Import contract cleanup

Issue: [#2744](https://github.com/moto-nrw/project-phoenix/issues/2744).
Implementation baseline: `93e9f5f2cca74716b39a28fe37248e3bd3ff9999`.
Local evidence: 2026-09-13, macOS arm64, with CGO disabled.

## Scope and ownership

The issue's blocker interfaces lacked several reads required by the retained
importers. The user authorized adding those native contracts and migrating
their callers. Identity & Access now supplies tenant-scoped RFID, active school
account, unused invitation-person, school-role and role-permission reads.
School Structure supplies bounded group listing. The import workflow consumes
native owner facts, audit events and narrow transaction, permission and
invitation ports. Composition binds the existing runtime and policy behavior.

File parsing and templates live in Data Import. Date and WallClock have one
implementation in the approved shared kernel; the old timezone package keeps
type aliases and forwarding functions for unrelated callers. Departure rules
likewise have one native implementation with legacy model adapters.

No table, owner, existing package classification or runtime write ownership
changed. No fallback provider was added. The retained database-backed import
tests moved to the existing integration boundary; pure decision tests remain
beside the workflow. Screens, labels, upload steps and help content are
unchanged, so the backend-only help-guide exemption applies.

## Static contract evidence

The [generated caller inventory](data-import-contract-2744.inventory.json)
records 48 removed exported symbols/providers with zero remaining callers.
It scans Go import headers and qualified references; compilation and the
architecture evaluator provide the independent type-checked gates.

| Surface | Before | After |
| --- | ---: | ---: |
| Repository-wide exact legacy keys | 1,594 | 1,544 |
| Remaining #2744 keys at implementation baseline | 49 | 0 |
| Composition field/setter targets | 793 | 792 |
| New legacy keys | 0 | 0 |

The original ticket listed 58 keys; nine were already absent at this baseline.
The additional resolved key belongs to #2747: the root services package no
longer imports the deleted `models/import` package. The generated composition
inventory retains the same two test factory constructions at their new
integration-test paths. `SetOpeningBalanceImportFactory` and `LegacyReads`
have no callers or definitions. All 292 original test function names from the
affected import and timezone trees remain in the backend.

## Verification

Commands run from `backend/` through `scripts/run-go-toolchain.sh` with
`CGO_ENABLED=0`:

```sh
go test ./... -parallel 8
golangci-lint run --timeout 10m
go test ./internal/architecture -run '^TestCompositionLegacyCallerInventory$' -update-composition-legacy
go test ./test -run '^TestCompositionInventory$' -update-composition-inventory
go test ./services/import/integration -run '^TestDataImportRuntimeEvidence$' -count=1 -v
```

Architecture verification runs from the repository root:

```sh
scripts/backend-architecture.sh check
```

The first full run caught stale date-definition paths, reflection names and
composition locations, plus a pure DTO test's previous model-directory
exemption. Those references were updated without changing expected behavior
or weakening assertions. Focused rechecks pass.

The final full run passes all 242 packages with tests. Lint reports zero
issues. The final architecture check passes with the counts above; both
generated composition inventories and whitespace checks also pass.

The HTTP contract suite constructs the production import router and exercises
CSV/XLSX downloads, preview/import, permissions, missing actor profiles,
invitation creation, audit persistence and partial-batch failure envelopes.
Workflow tests cover owner failures, rollback, retries, replay, concurrent
uploads and tenant isolation. Native identity tests cover missing tenant,
foreign/inactive mappings, expired unused invitations, tenant role precedence
and hidden foreign-role permissions. No scheduler/job or process-startup path
changed. No deployed server or staging/production smoke is claimed.

The independent standards and spec reviews found no blocking defect. Standards
noted one non-blocking maintenance concern: consent-transition decisions also
remain in the legacy users service. No current divergence was found. Actor
lookup retains its HTTP status and German error prefix; nested diagnostic
wording now names the native lookup rather than the old service wrapper.

## Local runtime samples

[Raw evidence](data-import-contract-2744.raw.json) contains two warmups and ten
measured samples per operation, ten student rows per batch, concurrency one.
Each actual row includes the same person, student, guardian, phone, relationship,
pickup and consent writes as the earlier #2708 fixture. Percentiles use nearest
rank. These are workflow durations, not HTTP latency or a production SLO.

| Operation | Statements | Write rows | p95 | Errors |
| --- | ---: | ---: | ---: | ---: |
| Preview | 16 | 1 | 22.541 ms | 0/12 |
| Import | 244 | 81 | 209.105 ms | 0/12 |

All runs accepted ten rows without row errors. Statement and write counts are
unchanged from the previous bounded-batch fixture. Pool-wait counters and the
database deadlock delta are zero. Other local verification ran concurrently;
these samples do not establish a latency improvement or regression against
earlier measurements. The primary Contract gate is zero old callers and zero
listed keys, not these timing samples.

## Rollback

Revert this code-only commit if a hidden caller appears. No DDL, data rewrite,
deployment or receipt deletion is needed. Existing checkpoint rows and their
tenant-scoped transaction behavior remain unchanged.
