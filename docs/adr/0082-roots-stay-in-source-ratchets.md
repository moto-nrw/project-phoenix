---
status: accepted
---

# Roots stay in the source ratchets

Decision of 2026-10-23 for [#4542](https://github.com/moto-nrw/project-phoenix/issues/4542).
Rule 16 and [ADR 0064](0064-moved-code-stays-in-the-pass-through-scan.md)
require a move to retain its gate. This decision extends the measured surface,
not the thresholds or the permitted architecture.

## Decision

1. File size (800 physical newlines), named function body length (60 lines,
   braces inclusive), and cognitive complexity (gocognit 1.3.0, 15) share
   `walkModuleRatchetSources`: every non-`_test.go` Go source recursively in
   `modules/`, `workflows/`, `roots/`, `api/`, `cmd/`, plus direct Go files in
   the backend root package `.`. Hidden subdirectories stay excluded, as
   before. Build tags do not remove files: hooks, fixture helpers, generated
   code and nested `testdata` remain measured. The directory scope is a
   conservative superset of root ownership, not a reclassification of
   `api/common` (Tenant Runtime), `api/iot`, `api/display`, test support or
   `cmd/devtools` (Dev Tools).
2. The three existing registers take exactly the newly measured values below,
   without headroom. This is the new-numbered ADR required by ADR 0057 point 5
   and `internal/architecture/ratchet_budgets.go`; an amendment to an existing
   ADR number would not authorize the additions. No budget-check mechanism,
   assertion, threshold, ownership, policy rule or existing module allowance
   changes. Each new entry must fall when its measured value falls and leave
   when it reaches the threshold, like every existing entry.
3. Split `api/serve_mounts.go` mechanically by mount area, retaining function
   declarations, bodies and call sites. The 1,372-line file becomes seven
   files of 159, 299, 320, 174, 273, 201 and 69 lines (mounts, middleware,
   students, presence, school, portals, public). Its proposed size entry is
   therefore unnecessary. The three overlength functions keep their lengths
   under their new file keys. Roots still build and mount locally: no new
   service graph, shared construction state or late binding (ADRs 0059, 0068).
4. Keep the legacy register's explicit `legacy_*.go` convention, and rename
   `roots/owners/workforce_legacy_schedule_repositories.go` to
   `roots/owners/legacy_workforce_schedule_adapter.go`, unchanged at 246
   LOC. Its record conversion is retained compatibility glue, not native
   Workforce vocabulary. Add exactly that entry and a total of 246. A blanket
   `*legacy*.go` expansion would also count historical migrations and the
   architecture checker's own `legacy.go`; a name alone does not make those
   removable adapter debt. This change closes the reported gap by using the
   existing register, rather than inventing exceptions for a broader glob.
   Other non-prefix legacy names remain a stated limit for #4548.
5. **Do not add a root pass-through allowance.** The measurement below contains
   pure forwards, which are outside ADR 0075's floor. Freezing them together
   with real translations would turn measured wiring into permitted debt.
   The existing consumer-attributed `modulePassthroughMovedTrees` and the
   1,408 owner total remain unchanged. The complete-root measurement is
   evidence for #4548, not a second floor or permission to delete translation,
   transactions, validation, tenant checks or error mappings.

## Measurement basis and reproduction

Fetched `origin/development` before branching. Baseline:
`722c5aa147a1baaf33d19adb8910663d9b9974db`, including #4631, #4635 and #4637.
Issue #4542 had no comments. Before final verification, development
`c40cab16f5` (#4639) was merged without conflicts; all root pass-through counts
and all three source-ratchet budgets remained unchanged. All before figures
use the original immutable tree;
"after" adds this change's scan expansion, rename and mechanical split.
No app servers or remote moto API are used for measurement.

The baseline's actual root-composition packages are `.`, `api`, `cmd`,
`roots/owners`, `roots/communicationtest`, `roots/devicescanbinding`,
`roots/openroommovebinding`, `roots/requestreviewbinding`, and
`roots/sessionendbinding` (`architecture/policy.json`). The new recursive
scope also catches future packages anywhere in those directory trees.

Before this change:

- File size scanned `modules/` and `workflows/` only.
- Length and complexity scanned those trees plus three recursive bindings:
  `roots/requestreviewbinding`, `roots/sessionendbinding`,
  `roots/openroommovebinding`; and exactly three files in `roots/owners`:
  `parent_announcement_audience_binding.go`, `parent_inbox_binding.go`,
  `staff_inbox_binding.go`.
- The separate handler complexity ratchet scanned the default production
  `api/` build only, excluding test hooks. It remains at zero and unchanged.
- Pass-through traversal visited `modules`, `workflows`, `api`, `roots`, but
  the key selector counted only application/composition packages and the
  exact `modulePassthroughMovedTrees`. Root coverage was limited to four
  bindings (`devicescanbinding`, `openroommovebinding`, `sessionendbinding`,
  `requestreviewbinding`) and three API files (`device_scan_catalog.go`,
  `import_staff_invitation.go`, `kiosk_timetable.go`), attributed to consumers.
  The legacy exclusion and four AST conditions stay unchanged.

### Source-size reference, excluding `_test.go`

Direct package files, not recursive totals; LOC includes comments and blanks.
The "default" columns exclude files tagged `testhooks`, as ADR 0080's LOC
measurement does. Ratchets include both columns.

| Package | Default files | Default LOC | Hook files | Hook LOC |
|---|---:|---:|---:|---:|
| `api` | 172 | 16,050 | 128 | 11,249 |
| `roots/owners` | 313 | 34,232 | 19 | 1,389 |
| `roots/communicationtest` | 1 | 18 | 0 | 0 |
| `roots/devicescanbinding` | 6 | 594 | 0 | 0 |
| `roots/openroommovebinding` | 1 | 58 | 0 | 0 |
| `roots/requestreviewbinding` | 14 | 1,067 | 0 | 0 |
| `roots/sessionendbinding` | 1 | 173 | 0 | 0 |
| `cmd` | 11 | 3,215 | 0 | 0 |
| `.` | 1 | 9 | 0 | 0 |

The issue's approximate 311/171-file snapshot is not today's baseline.
Counting hooks as ordinary production or mixing recursive and direct packages
would also give different figures.

### Source ratchet registers

| Ratchet | Existing registered entries / sum | Newly visible at baseline | Final entries / sum |
|---|---:|---:|---:|
| File size >800 | 0 / 0 | 3 / 3,859 LOC | 2 / 2,487 LOC |
| Function bodies >60 | 0 / 0 | 27 / 3,834 lines | 27 / 3,834 lines |
| Cognitive complexity >15 | 0 / 0 | 13 / 285 points | 13 / 285 points |
| `legacy_*.go` LOC | 0 / 0 | Workforce file not matched | 1 / 246 LOC |
| Owner pass-throughs | 30 / 1,408 | measured separately below | 30 / 1,408 |

Final size entries: `api/common/error_codes.generated.go` 1,348 and
`cmd/cleanup.go` 1,139. Length entries: `api/**` 21, `roots/owners` four,
`cmd/**` two. Complexity entries: seven API hooks, six `cmd/**` functions.
No complexity exception is needed in `roots/**` or `.`. Register entries keep
exact path/function keys, not package-wide headroom. Whole-register sums are
not interchangeable across metrics.

Reproduce the gate with the pinned toolchain, from the repository root:

```bash
(cd backend && ../scripts/run-go-toolchain.sh go test -tags=testhooks ./test/ -run 'Ratchet' -count=1)
scripts/run-go-toolchain.sh scripts/backend-architecture.sh check
```

Each scanner's failure lists exact measured values. To inspect newly covered
baseline values, use a checkout of the SHA above, apply only the shared scan
expansion and run the three module ratchets with their empty baseline maps.
For file counts an independent `wc` cross-check is:

```bash
find backend/modules backend/workflows backend/roots backend/api backend/cmd -name '*.go' ! -name '*_test.go' -exec wc -l {} + | awk '$1 > 800 && $2 != "total"'
find backend -maxdepth 1 -name '*.go' ! -name '*_test.go' -exec wc -l {} +
```

### Root pass-throughs

Run from the repository root on the measured checkout (the mechanical moves
and filename rename do not change these counts):

```bash
python3 docs/architecture/4542-root-passthroughs.py
python3 docs/architecture/4542-root-passthroughs.py --sites > /tmp/root-passthrough-sites.json
```

The [measurement script](../architecture/4542-root-passthroughs.py) extracts
`modulePassthroughIsDelegation` and its helpers unchanged from the ratchet:
exported receiver method, body at most 14 lines, exactly one receiver-field
call. It prints every matched body for review. Shape classification is a
reviewed snapshot, not type analysis or a new CI gate. The initial syntactic
forward filter recognizes one direct call (return or void), or an assignment
followed by an unchanged return, passing all named parameters in order.
Conversions, fixed/default arguments, dropped results, extracted fields,
collection projection and error mapping are translations. Method renames
alone are not. The 15 named guard-only sites and the infrastructure/locking
sites are kept separate; they are not quietly approved as floor hits.

| Package | Hits | ADR 0075 form 2 translation | Pure forward | Guard only | Other |
|---|---:|---:|---:|---:|---:|
| `roots/owners` | 787 | 636 | 136 | 15 | 0 |
| `roots/devicescanbinding` | 19 | 18 | 1 | 0 | 0 |
| `roots/openroommovebinding` | 1 | 1 | 0 | 0 | 0 |
| `roots/requestreviewbinding` | 15 | 15 | 0 | 0 | 0 |
| `roots/sessionendbinding` | 0 | 0 | 0 | 0 | 0 |
| `roots/communicationtest` | 0 | 0 | 0 | 0 | 0 |
| `api` | 191 | 153 | 35 | 0 | 3 |
| `cmd` | 1 | 0 | 0 | 0 | 1 |
| `.` | 0 | 0 | 0 | 0 | 0 |
| **Root-composition packages** | **1,014** | **823** | **172** | **15** | **4** |
| `api/common` (Tenant Runtime, not a root) | 6 | 0 | 0 | 0 | 6 |
| `cmd/devtools` (tool, not a root) | 20 | 7 | 12 | 0 | 1 |
| **Recursive scan superset** | **1,040** | **830** | **184** | **15** | **11** |

All other nested packages have zero hits. Of the eleven "other" hits, one
is form 1: `cmd` `settingOverrideRoot.StoredOverride` wraps the read in
`tenant.WithinTenant`. One is `cmd/devtools` `sharedOperatorSession.LoginOperator`,
which synchronizes a shared login rather than translating it. Nine are HTTP
writer/request runtime behavior (`api/common` six, `api/iot_http_metrics.go`
two, `api/serve_host.go` one), not consumer-port bindings. They do not become
form 2 merely because their files sit under `api/`.

Examples checked against the bodies: `roots/devicescanbinding` `sessions.Current`
converts not-found into nil and maps the session; `roots/owners`
`calendarStaffPort.ListAllWithPerson` maps collection values;
`enrollmentStudentRecords.FindStudent` projects away a result;
`api` `supervisionSettings.StudentPhotosEnabled` supplies a settings key.
Conversely `roots/owners` `mfaPolicyLock.LockMFAPolicySharedForTenant` only
adds a nil guard, and `PushSubscriptionCleanup`'s six methods add no translation.
This records their shape, not a new authorization to tolerate nil dependencies.

The issue's rough 468 `return x.f(...)` count is a different predicate: it does
not enforce the existing ratchet's export/body-size/field-call rules or separate
argument conversion from forwarding. Neither figure proves all wiring is
necessary. ADR 0080's warning still holds: conversion can duplicate vocabulary,
and a short body can contain a misplaced rule. No domain decisions, write
edges, tenant permissions or concurrent #4639/#4641 files are changed here.

## Consequences and follow-up proposals

- A new oversized file, long function or complex function anywhere in the root
  trees fails CI, including a new package. The fixture test
  `TestModuleRootSourceRatchetsRejectNewViolations` proves this for nested
  `roots`, `api`, `cmd`, and `.` and verifies the original exclusions. A local
  negative run also added temporary 866-line sources with a 64-line function
  scoring 16 to both `roots/owners` and `api`: all three ratchets failed with
  both paths (six diagnostics). The probes were removed before the green run.
- #4548 should distinguish necessary consumer translation from removable
  forwarding before designing a root pass-through register. Preserve existing
  consumer attribution instead of double-counting it in a second allowance.
  Decide whether scope selectors should be checked against the base too:
  the budget checker compares numeric declarations, not directory lists.
- Consider separate cuts for `cmd/cleanup.go`, long root builders and large
  test compositions. Generated error declarations are also currently source
  debt by the literal size predicate; any generated-source policy belongs to
  #4548, not an unreviewed exclusion in this change.
- Audit guard-only bindings and non-prefix legacy names separately, keeping
  their existing behavior and testing their construction requirements. No
  follow-up issue is created by this decision.
