# Round 9 set A: constructor-only evidence

Base: `12608383f0f793e811f4379ec293f61efd6150c9`. Own packages only; set B is not changed.

## Result

- Runtime-retained functions in set A: **324 → 2**. The two exact #4088 register blockers remain named in the [complete audit](final-legacy-cluster-round-eight-compose.md).
- Legacy baseline stays empty. No legacy key is retired or added, so `exact_ratchet_keys` is empty.
- Composition surface stays **0** across production and test scopes.
- No new aliases, DTO definitions, table grants, hook allowances, register entries or checker permissions. Existing native record/port definitions move once; the narrow generic source/view ports bind existing values without another service forwarding layer.

## Native boundaries

- Communication parent announcements: service decisions and public facade are application code; repository conversion remains an adapter. Its boundary probes use the existing external workflow-integration role, not an internal adapter-test application grant. Pure application suites bind real Delivery/Security policy through a cycle-free typed root test-support package (`roots/communicationtest`).
- Parent/staff stores: native owner ports and private persistence/decorator adapters. Root projection bindings map onto those ports; historical declaration access reuses existing submission snapshots, not a cloned history DTO.
- Messaging: native application facades accept bounded service/view ports; the original adapters retain their runtime and errors. Immutable row/error bindings preserve nil/empty collections, payload number spelling, partial-error precedence and public error identities.
- Student Presence: application facades, row mapping, supervision parsing and transaction requirements. The operations value remains a struct with the original fields; session-record consumers use the existing bounded methods, not an implementation pointer.
- Request Review: cross-owner contract mappings live in typed root bindings. Owner integration fixtures remain in their original test package. Enrollment rows and booking selections are separate narrow capabilities; no root owns both broad values.
- Session End/Open Room Move: roots bind the existing consumer ports. Session events and guardian wake-up are separate notifier components, retaining their order. No handler, route, middleware identity or golden is changed.
- Parent Portal: care application owns row iteration, live-flag interpretation, refusal translation and tenant deduplication. Composition callbacks are one-statement casts/field bindings, all with zero branches/loops. Existing source records are bound generically rather than copied or imported into a newly granted application edge.

## LOC and delegation budgets

| Package | Base LOC | Candidate LOC |
|---|---:|---:|
| `modules/communication/composition` | 1267 | 367 |
| `modules/studentpresence/compose` | 613 | 243 |
| `modules/studentpresence/compose/presenceservice` | 873 | 110 |
| `modules/requestreview/compose` | 988 | 12 |
| `workflows/openroommove/compose` | 81 | 46 |
| `workflows/parentportal/compose` | 450 | 279 |
| `workflows/sessionend/compose` | 221 | 59 |

Compose cap: **20,119 → 16,742**. Delegations: Communication **72 → 71**, Parent Portal **19 → 17**; total **2,974 → 2,971**. Consent wrappers and the three identical phone/locale forwarders are actually deleted. All formerly counted moves remain scanned, including the new root paths and Parent Portal internal care/messaging files (ADR 0064).

`parentstore` and `staffstore` were not counted at the base: the old selector only includes `/compose`, `/composition`, `/internal/application` and explicit moved trees. Their native row/decorator code belongs to persistence adapters, not application services; its location change is not presented as a delegation reduction.

## Rule successors

| Added rule | Precedent and bound |
|---|---|
| `communication.application.organization-tenancy.public` | Retired `communication.compose.organization-tenancy`; native error/target mapping, same public owner direction, production only. |
| `communication.postgres.port` | `student-presence.postgres.presence-cutover.student-presence.port`; same-owner native persistence port, production only (ADR 0058). |
| `communication.postgres.tenant-runtime.public` | `student-presence.application.tenant-runtime-public` and the original `communication.compose.tenant-runtime`; same existing public runtime direction, production only. |
| `root-composition.reach.internal-test.communication.port` | The original root test reach to Communication compose plus `root-composition.reach.internal-test.audit-platform.port`; exact `api` → native Communication port, internal test only (ADR 0053). |

Rules **921 → 911**: 4 narrow successors, 14 unused rules retired. No policy-guard or permission-acceptor changes. The delegation scan gains roots traversal and exact moved paths; complexity and function-length scans gain only the moved root files/packages. Their predicates and thresholds are unchanged, and the moved correction-history limits remain 19 / 64.

Retired: `communication.compose.organization-tenancy`, `communication.compose.parentannouncementaudience.native-contract`, `communication.compose.parentmessageinbox.native-contract`, `communication.compose.staffmessageinbox.native-contract`, `open-room-move.compose.student-presence-public`, `request-review-view.compose.audit-corrections.public`, `request-review-view.compose.care-plan.contract`, `request-review-view.compose.care-plan.public`, `request-review-view.compose.enrollment.public`, `request-review-view.compose.people-directory.public`, `request-review-view.compose.production.identity-access.review-contract`, `request-review-view.compose.school-structure.public`, `request-review-view.compose.timetable.public`, `session-end.compose.delivery-platform`.

## One-for-one path literals

The `module_complexity_ratchet_test.go` and `module_function_length_ratchet_test.go` keys follow the same functions from `modules/communication/internal/staffannouncements/<file>` to `modules/communication/internal/application/staff_<file>`. Function names, receiver names and each numeric threshold are unchanged. No entry is added. Request Review’s package-scoped query budget remains attached to its retained owner integration tests, so no budget path is changed there.

## Remaining blockers

| Function | Exact constraint |
|---|---|
| `modules/communication/composition/new.go:transaction.RunAdmin` | The #4088 transaction register allows shrinkage or a verified same-owner/all-role package rename with identical filename set, operation, reason and body. A partial `compose` → `application` extraction is not such a rename. Retained unchanged; no replacement key or checker extension. |
| `modules/studentpresence/compose/new.go:transaction.Run` | The #4088 transaction register allows shrinkage or a verified same-owner/all-role package rename with identical filename set, operation, reason and body. A partial `compose` → `application` extraction is not such a rename. Retained unchanged; no replacement key or checker extension. |

## Verification

Required local gates are recorded in the PR after execution: architecture, all-ticket validation against the resolved base, full no-fast affected tests through the shared heavy wrapper, lint and full hooks. No production requests or credential changes.
