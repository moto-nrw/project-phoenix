---
status: accepted
---

# Principle: owners meet through contracts and write together in workflows

Principle ADR of 2026-10-19 under
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580), written for
[#4091](https://github.com/moto-nrw/project-phoenix/issues/4091). It states the
rules for contracts, workflows and transactions that still hold. The ADRs it
cites keep their context and history; this ADR changes none of them.
The [#4538 addendum](#addendum-2026-10-23-command-participation-and-coordination-4538)
narrows point 4 and records the remaining coordinator exceptions.
The [addendum of 2026-11-08](#addendum-2026-11-08-datenimport-schreibt-je-chunk-gebündelt-5552-5827-5887)
extends exception E3 to batch forms of the data import.

## Decision

1. **An owner offers its behaviour through its `public` and `contract`
   roles.** Those packages expose no ORM, repository, internal-model,
   `map[string]any`, generic CRUD or BUN-tagged types; the architecture check
   enforces this. Byte I/O such as `io.Reader.Read` is not entity CRUD
   ([ADR 0046](0046-api-common-is-the-shared-http-runtime.md)).
2. **An owner reaches another owner through that owner's contract or through a
   consumer-owned port.** The consumer declares the port in its own
   vocabulary, and the root binds it to the other owner
   ([ADR 0053](0053-owners-compose-themselves-and-the-roots-compose-the-owners.md)
   point 4). Which of the two a pair uses follows the dependency direction of
   [ADR 0071](0071-principle-owner-dependencies-point-one-way.md).
3. **Owners change their data through commands, not row writes.** Each command
   owns its guard, so guards cannot drift apart behind a row-shaped interface
   ([ADR 0023](0023-student-enrollment-owner-and-directory-projection.md)).
4. **A write across owners runs in an application workflow.** The workflow is
   an owner of kind `workflow` under `workflows/`; it owns no data
   ([ADR 0066](0066-principle-every-module-table-and-cross-owner-read-has-one-owner.md)
   point 5) and issues no SQL of its own. It holds the authorization boundary, the preview
   fingerprint, the lock order and one outer unit of work; each owner keeps its
   own reads and mutations. Putting the orchestration inside one domain would
   make that domain responsible for other owners' lifecycle rules
   ([ADR 0013](0013-staff-offboarding-is-an-application-workflow.md),
   [ADR 0016](0016-student-deletion-is-an-application-workflow.md),
   [ADR 0017](0017-grade-transition-is-an-application-workflow.md),
   [ADR 0018](0018-open-room-move-is-an-application-workflow.md),
   [ADR 0022](0022-parent-portal-is-an-application-workflow.md)). This holds
   whether or not the write can be undone (ADR 0023, amendment of
   2026-10-15).
5. **A workflow re-reads under its locks before the first mutation.** It
   acquires its locks in a recorded order, re-reads the counts or the cohort
   and compares the confirmed fingerprint. Where several tenant-wide gates are
   taken, the order is the class-writes gate of School Membership, then the
   timetable recurrence gate, then the School Structure transition gate
   (ADR 0016, ADR 0017, owner of the first gate per ADR 0023).
6. **A durable side effect commits with the write that causes it.** Cleanup
   intents and outbox entries (mail, push, file removal, export transfer) are
   recorded in the triggering transaction. A leased worker carries out what
   was committed, also when the caller died before dispatch; each feature
   keeps its own retry policy. An after-commit callback is a
   losable hint, never the authority for durable work (ADR 0013, ADR 0016,
   [#4087](https://github.com/moto-nrw/project-phoenix/issues/4087)). The
   exact, shrink-only register `backend/test/durable_side_effect_notices.json`
   (`TestDurableSideEffectsRatchet`) names every remaining `go`, `Dispatch` and
   `AfterCommit` in HTTP, application and composition roles with the harmless
   loss it accepts; a durable write cannot be added to such a callback.
7. **A cross-owner database function needs its own decision.** The demo
   visitor erasure is the one cross-owner database workflow. It runs only as
   its isolated role, has a fixed search path and grants its caller no table
   access. It is not an owner and changes no table ownership; any further
   database workflow needs its own architecture decision and an equally narrow
   runtime authorization
   ([ADR 0062](0062-demo-visitor-erasure-is-a-database-workflow.md)).

## Consequences

- Cross-owner coordinators are visible as workflow owners in `policy.json`,
  or as the finite exceptions in the #4538 addendum. Transaction-participating
  leaf commands need no separate owner. A new workflow owner uses the reviewed
  procedure of
  [ADR 0070](0070-principle-only-the-target-is-a-rule-and-debt-leaves-with-evidence.md).
- The compatibility bindings that ADR 0017, ADR 0018 and ADR 0022 describe for
  their first epochs are gone. Reach that is not target is exact debt
  ([ADR 0057](0057-only-the-target-is-a-rule.md)).
- Each module's public contract has a named contract test
  ([ADR 0069](0069-principle-a-test-reaches-its-owner-the-typed-roots-and-the-shared-kernel.md)).

## References

ADR 0013, ADR 0016, ADR 0017, ADR 0018 and ADR 0022 carry the workflow
decisions; ADR 0023, ADR 0046, ADR 0053 and ADR 0062 the points named above.
The full rule map is the
[traceability appendix](../../backend/architecture/README.md#appendix-principle-traceability).

## Addendum 2026-10-23: command participation and coordination (#4538)

Accepted for [#4538](https://github.com/moto-nrw/project-phoenix/issues/4538).
This replaces the unconditional reading of point 4, not ownership, dependency
directions, the lock order in point 5 or durable effects in point 6. The
[edge inventory](../architecture/4538-cross-owner-write-inventory.md) records
every example in the issue and its comment against fetched development
`e033cf0f27d6ea8d16f24df81176da9ddf042d39`, with transaction and lock evidence.

### Command participation is allowed

A domain may call a foreign owner's **guarded command** through its public
contract or a consumer-owned port bound by the root when:

1. The command joins the caller's transaction and scope. It makes no separate
   commit, tenant/admin scope switch or detached-context write. A command may
   open its own unit when invoked at top level, but not when participating.
   A savepoint is not an independent transaction.
2. Each owner retains its own validation, authorization guards and mutation.
   The caller propagates failure to the outer unit, or a savepoint rolls back
   the failed compound operation before the caller continues. An explicitly
   best-effort subcommand remains best effort; atomicity does not mean every
   requested subcommand must succeed.
3. The call requires no separate cross-owner protocol: no coordinated gate/row
   lock order, re-reading foreign state under locks to validate a combined
   decision, or shared after-commit orchestration. Owner-local row locks and
   a target command's transactional outbox do not alone create a workflow.

This permits the split membership/employment and relationship/pickup/access
record commands in the inventory. It does not permit foreign row repositories,
SQL, generic CRUD, domain-to-projection imports, domain-to-workflow imports,
cycles, late binding or decisions in the root. ADRs 0066, 0068 and 0071 stand.
Consumer ports retain the consumer's vocabulary (ADR 0080); no owners merge
and no internal/public DTO is duplicated just to build a workflow (ADR 0081).

### Coordination needs a workflow

A flow that needs any protocol in condition 3 is a workflow under `workflows/`.
Its root binds the owner commands; the workflow owns the outer unit, ordering
and combined check, and delegates mutations to their owners. A multi-owner
lifecycle assembled in HTTP is a coordinator, not a leaf command. A wrapper
that merely calls an unchanged domain coordinator does not satisfy this cut.

**Care resumption moves now into the existing `care-exit` workflow owner.**
`workflows/careexit/resumption.go` owns the complete command previously in
`CareLifecycle.Resume`/`resumeCare`: input checks, the frozen day, student
read under lock, exit check, conditional School Membership command, both Care
Plan ledger deletions and the Audit change trail. The root binds the workflow
as `careplan.CareResumption`; embedding that command preserves the existing
`CareLifecycle` contract without a forwarding method or workflow import in
Care Plan. It uses the existing student read and commands, so it changes no
SQL, dates, status rules, error identities, error text, scope or callbacks.

Resumption's order is shared class-writes gate → student locks → read exit → guarded
membership resume → delete exit → discard removal ledger → audit. Every step
runs in the same tenant unit. The existing People reader takes the class
gate (`student_records.go:60`) before its sorted row locks. No recurrence or
booking-write gate is added: resumption never restores an old plan. The read is still
`FindCareStudents(..., true)` in `roots/owners/care_resumption.go`; the target's
conditional `ResumeCare` remains authoritative even after that check. Logging
after returning from the unit is unchanged and is not a durable effect.

### Finite retained coordinator exceptions

The inventory marks the following existing commands **E**, rather than
pretending their transaction participation makes them leaf commands. Their
present protocols may stay at the named locations. This is an accepted
exception to point 4, not permission to add another coordinator or extend
these commands to more owners. A change to their cross-owner ordering,
combined guard or effects must move the affected protocol into a workflow
or amend this decision with new evidence first.

| Exception | Kept invariant and reason for retaining this cut |
|---|---|
| E1: Enrollment decision, existing-child approval/sync | Request-before-children serialization, class/recurrence gates, account-before-roster writes and transactional decision outbox already protect the approval. The decision helpers share the renewal/change-request path. Moving only `Decide` would leave the coordinator in Enrollment; a proper cut is a separate PR. |
| E2: Care confirm/cancel/due effects and withdrawal completion | Booking gate, locked binding preview, removal ledger and conditional Membership command stay together. #4531 already cut own booking lock/end back into Care Plan. Moving the whole lifecycle with its preview, completion and scheduler paths would be a second large protocol cut alongside resumption. |
| E3: Identity guardian-role upgrade; People student/companion and staff identity edits; Workforce staff master-data edits | Keep the foreign relationship/student/account/staff locks, in-lock checks, savepoints and audit order recorded in the inventory. These are not blanket exceptions for Identity, People or Workforce. Splitting their row halves into small forwarding workflows would retain the actual locks in the old coordinator. |
| E4: Care booking/roster reconciliation, master-data approval and HTTP child creation | Keep class/recurrence/student ordering, in-lock authorization and all-or-nothing child creation, including its 4xx rollback behavior. They share lower-level commands with E1/E2; extracting them together exceeds this PR's one-workflow scope. |
| E5: Organisation provisioning account, device transfer and person retirement | Keep explicit admin/tenant scope boundaries, locked device transfer and the documented best-effort/post-commit audit behavior. The whole provisioning API is not one atomic transaction. Converting it to one would change behavior and authorization, not just placement. |

The [inventory](../architecture/4538-cross-owner-write-inventory.md#retained-protocols-and-lock-order)
gives the path:line order and limits, including the E2 class/recurrence
correction in #4636 and the remaining static findings, and estimates separate
follow-up slices. This PR implements one workflow capability, not all of those
slices. No follow-up issues are created. No new owner, policy rule, epoch,
projection grant, baseline key, pass-through allowance or budget is needed.

### E5 mandatory operator audit (#3842, slice 3842-b)

The security correction in #3842 narrows E5's best-effort audit exception:
device transfer and person retirement now return audit failures from their
existing administrative transaction. Key rotation and device, school and
organisation deletion use the same fail-closed rule. The scope transitions,
owner commands, device locks and their order do not change; no operation is
combined with another provisioning operation or extended to another owner.

Port-failure tests reproduce successful privileged writes despite a rejected
audit append. Production-composition tests with a rejecting PostgreSQL trigger
verify rollback of the source device, absence of a target device, preserved
credentials, undeleted records and rolled-back session revocations. See
`api/operator_audit_failure_test.go` and the
[operating note](../operations/operator-audit-3842-b.md). A wider workflow
extraction would not improve this bounded audit invariant and remains separate
from this correction. Other E5 best-effort/post-commit paths stay unchanged.

### E2 lock-order correction (#4636)

The two-connection regression in `api/care_exit_lock_order_test.go` reproduces
`40P01` before the fix for confirm, cancel and due effects against grade-transition
Apply, including the scheduler’s ambient-transaction case. This evidence amends
E2's retained protocol: those three entry points,
including withdrawal confirmation through the shared confirm path, now take
School Membership's **shared class gate → recurrence → child/plan row locks**.
The initial booking-expiry reconciliation uses the same order because an ambient
unit retains its gates through the later due-effect pages.
Care Plan requests the class gate through its consumer-owned student port;
the root binds it to School Membership's existing command. Neither owner gains
foreign SQL or a new policy rule. Re-reading, fingerprints, ledger operations,
error identities and the outer tenant unit stay unchanged.

This is a bounded correction of E2 to point 5, not a new coordinator exception
or a claim of global deadlock freedom. Grade-transition Apply/Revert retain
class(exclusive) → recurrence → transition; the materializer takes the suffix
recurrence → transition and no class gate. The inventory records every audited
shared-gate path; #4638 below corrects the two offering findings.

### E1/E4 offering lock-order correction (#4638)

Offering-derived writers use the same prefix as E2: **class(shared) →
recurrence → child rows → offering-change request rows / pickup rows**.
`BookingMaterialization.loadOfferingAdjustmentSubject` acquires the existing
`LockOfferingDerivedWrites` gate itself, protecting the admin correction as
well as already-gated callers. Pickup reset takes those gates before its child
lock. Offering approve/reject, MarkDone and conflict pre-locking locate the
immutable student ID without locking, acquire that prefix, then lock and
re-read the request. Terminal/version checks repeat under the request lock;
authorization, catalog validation, tenant scope and mutations retain their
existing contracts. No transition gate, owner, policy rule or root decision is
added. Existing account-before-roster constraints are unchanged.
#4639 also deliberately corrected two missing-request responses from HTTP 500
to HTTP 404 (`general.input`): offering-change preview and offering-conflict
resolution with a missing offering request ID. This was an error-contract
correction, not a claim that every response stayed unchanged.

`modules/careplan/contracttest/offering_lock_order_test.go` records real
`40P01` failures before the fixes: admin correction versus grade Apply, and
approve/reset versus care end, plus a child-held interleaving for approve,
reject, MarkDone and offering conflict resolution. PostgreSQL lock observation
arranges the interleavings; all commands complete after the fixes. The
[inventory](../architecture/4538-cross-owner-write-inventory.md#offering-lock-order-correction-and-row-audit-4638)
records the before/after paths and the remaining, separately scoped
care-schedule, excused and master-data request-before-child findings. This is
not a claim of global deadlock freedom or an exemption for those inversions.

### E2/E4 family-request lock-order correction (#4640)

The two-connection tests in
`modules/careplan/contracttest/request_lock_order_test.go` reproduce `40P01`
for care-schedule, excused-absence and master-data decisions against care
confirmation before this correction. This evidence amends the retained
request protocols: **class(shared) → child → family request** for these
non-offering writers. Recurrence is not needed by these commands. Offering
writers retain **class(shared) → recurrence → child → request** from #4638.
Care end retains that full gate prefix before its sorted child cohort and open
requests. No new coordinator exception, owner or policy rule is introduced.

Every single decision, completion, correction and conflict pre-lock first
reads the request without a lock to locate its immutable student ID. It locks
the child, then re-reads the request `FOR UPDATE`; status, expected version
and authorization still use the locked state before mutation. Guardian
master-data edit uses the same protocol, with ownership checked before status
and again under lock. Direct excused guardian edit also takes child before
request; its portal caller already held the child. Ownership is checked before
that acquisition and again under lock, so a mismatched request cannot add a
second child lock out of order.

Bulk approval locks **all children in ascending ID order before any request**,
then requests in its existing kind/ID order and applies in child/kind/ID order.
One child per request would not suffice: request kind/ID order can visit a
higher child before a lower one. Conflict candidates must still name one child;
each kind's pre-lock acquires its own gate/child prefix before sorted request
IDs. In particular the coordinator must not take a child before the offering
port has acquired recurrence. Companion updates keep their existing sorted
locks and downward-NOWAIT refusal, not a new blocking acquisition of lower IDs.

The [inventory](../architecture/4538-cross-owner-write-inventory.md#family-request-row-order-correction-4640)
records the full before/after audit, including unchanged submissions,
portal edits, course withdrawal, Enrollment and student deletion. Missing or
tenant-hidden requests, requests deleted while waiting, closed requests and
stale versions retain their service sentinels and HTTP status/code contracts.
Deleting the child after the preliminary read maps back to the request's
not-found sentinel. The new 404/409 regression suites pass on the original
implementation as well as the fix; the child-wait cases additionally cover
the new interleaving. No response change is intended by #4640.

### Evidence and enforcement

- `careplan/contracttest/care_resumption_workflow_test.go` verifies that a late
  audit failure rolls back Membership and both Care Plan deletes, with and
  without an ambient unit, and that another school's child is unknown.
  Existing `TestCareLifecycle_Resume`, `...ResumeForAFutureStartWaitsForTheScheduler`
  and `...ResumeDoesNotBringThePlanBack` preserve status/date/planning behavior.
  `workflows/careexit/resumption_mock_test.go` pins pre-unit validation and
  preserving the ledger after a conditional Membership refusal/error.
- Enrollment's `TestDecisionService_ApprovalRollsBackEveryOwnerAfterMaterialization`
  and `TestExistingStudentApprovalGrantsAccountBeforeClassRosterResync` pin E1's
  rollback and account-first order. `TestCareExitOfferingLinksTenantScopeRollbackAndRestore`
  pins the #4531 storage boundary; it does not prove the whole care lifecycle
  is a workflow. Other exact test pointers are in the inventory.
- Architecture checks enforce data ownership, public contracts and the DAG,
  not command effects behind a root-bound port. Current green gates cannot
  prove this addendum's semantic classification. Review enforces the finite
  exceptions. #4548 should assess a command-effect inventory that follows root
  bindings; #4542 covers root scan completeness. No name-based checker or new
  exception register is introduced here.
- ADR 0075's floor is unchanged. No counted method moves out of a scan to
  lower a budget: the moved `Resume` was not a counted short delegation.
  The new root adapter is recorded explicitly even while #4542 is pending.

ADR 0023's old #4076 promise is replaced by this decision: #4076 closed with
PR #4338 after removing foreign-import debt, not after moving the complete
care coordinator. PR #4619 (#4531, ADR 0075) subsequently cut the source-booking
lock/end round trip; it did not move `EndCare` or `ResumeCare` into a workflow.

## Addendum 2026-11-08: Datenimport schreibt je Chunk gebündelt (#5552, #5827, #5887)

Vorgeschlagen am 08.11.2026 für
[#5552](https://github.com/moto-nrw/project-phoenix/issues/5552),
[#5827](https://github.com/moto-nrw/project-phoenix/issues/5827) und
[#5887](https://github.com/moto-nrw/project-phoenix/issues/5887); gilt mit
dem Merge. Grundlage ist der Entwurf in
[#5552](https://github.com/moto-nrw/project-phoenix/issues/5552#issuecomment-6711342905),
geprüft am Code von `development` @ `82fed4f213`. Das Addendum ändert keinen
Code und kein Register. Es legt fest, in welcher Form der Datenimport die
Owner-Kommandos eines Chunks gebündelt aufrufen darf und was dabei für IDs,
Fehler, Rollback, Frische der Reads und Sperren gilt.

### Begriffe

- **Entscheidung E2, E3, E4, E10** meint die Entscheidungen der
  DB-Roundtrip-Runden vom 07. und 08.11.2026: E2 gemeinsamer Lese-Snapshot
  außer nach Locks, E3 zusätzliche Sequenzlücken bei fehlgeschlagenen
  Batch-Inserts, E4 500-Details dürfen sich ändern, 4xx-Codes und ihre
  Reihenfolge nicht, E10 frühere Sperre späterer Zeilen nur mit Auflagen.
- **Ausnahme E3** meint die Zeile E3 der Tabelle
  [Finite retained coordinator exceptions](#finite-retained-coordinator-exceptions)
  oben. Der Entwurf in #5552 nennt beides „E3“; hier sind sie getrennt.
- **Chunk:** bis zu 100 Positionen der Verarbeitungsreihenfolge in einer
  Tenant-Transaktion (`importservice/import_service.go:75`,
  `batches.go:243–313`). Commit-, Checkpoint-, Retry- und Resume-Einheit.
- **Segment:** zusammenhängender Teil eines Chunks, in dem keine Zeile einen
  Match-Schlüssel mit einer früheren Zeile desselben Segments teilt.
- **Zeilenpfad:** der heutige Ablauf `applyBatchRow` je akzeptierter Zeile
  (`batches.go:384–411`). **Batchpfad:** die Owner-Batch-Kommandos dieses
  Addendums.

Pfade ohne Verzeichnis liegen unter
`backend/modules/dataimport/internal/application/importservice/`.

### Ablauf heute

1. Validierung in einer Tenant-Transaktion (`batches.go:184–188`):
   Mode-Check, Checkpoints, `PreloadReferenceData`, `ValidateBatch`,
   `Validate` je Zeile, `ProcessingOrder` (`:195–218`). Kein Lock.
2. Je Chunk ein `WithinRetry` (`:246`): Advisory Lock `data-import:<key>`
   exklusiv (`:317`), Checkpoint-Read (`:320`), erneutes
   `PreloadReferenceData` (`:296`), optional `BatchWriteConfig` (`:331–341`),
   `applyBatchRow` je Zeile (`:342–347`), Checkpoint als Audit-Zeile (`:352`).
3. Ein Schreibfehler in Zeile k trägt den Fehler in `result.Errors` ein
   (`import_service.go:507–528`) und rollt den ganzen Chunk zurück
   (`batches.go:398–408`); frühere Chunks bleiben committet, die bestätigten
   Zähler des Chunks werden 0 (`:248–251`).
4. Wiederholt wird die ganze Transaktion bei `40P01`/`40001`, höchstens
   dreimal (`modules/transactionruntime/internal/adapters/postgres/tenant_runtime.go:68`,
   `tenant/runtime.go:284`, `:324`). `55P03` kommt im Importpfad nicht vor:
   Er sperrt blockierend, ohne `NOWAIT` und ohne `lock_timeout`.

Antwort bei Abbruch (`modules/dataimport/http/api.go:222–233`,
`api/data_import_http_runtime.go:85–94`): 500 `import.import_batch_failed`
mit dem bisherigen Ergebnis. Ist die Ursache eine `BusinessRejection`, etwa
`students.child_quota_reached`, wird es 409 mit demselben Code und
`details.rejection` (Code und Details der Ablehnung). Fachfehler vor dem
Schreiben (`duplicate_check_failed`, `already_exists`, `not_found`,
`import_service.go:428–443`) sind Zeilenfehler ohne Rollback.

### Messung

Gemessen am 08.11.2026 auf `development` @ `82fed4f213` mit
`testpkg.CaptureQueriesForContext` über `ImportBatches`, nach zwei Warmups
wie `importtest/query_budget_test.go`, in einem temporären, nicht
eingecheckten Test gegen einen eigenen Postgres 17.
Zeilen: Student wie `studentBatch` (ein Guardian, ein Telefon, eine
Abholzeit, Datenschutz); Student-Update spielt dieselben Zeilen im Modus
`update` erneut ein; Staff mit Rolle, Personalnummer, Stunden und
Qualifikation, ohne E-Mail; Klassenliste mit Name und Klasse; Anfangssalden
mit Stunden, Anspruch, Übertrag und Rest.

| Szenario | N10 | N110 | je Zeile |
|---|---:|---:|---:|
| Student create | 235 | 2445 | 22,1 |
| Student update (unveränderte Zeilen) | 282 | 2989 | 27,1 |
| Staff Vorschau | 29 | 229 | 2,0 |
| Staff create ohne E-Mail | 190 | 1901 | 17,1 |
| Klassenliste Vorschau | 24 | 224 | 2,0 |
| Klassenliste create | 80 | 786 | 7,1 |
| Anfangssalden Vorschau | 199 | 2099 | 19,0 |
| Anfangssalden create | 550 | 5861 | 53,1 |

„je Zeile“ ist (N110 − N10) / 100.

Kontrolle: Student-Create trifft die Register
`modules.dataimport.student_batch.import` (235) und `.import_110` (2445).

### Befunde am Code

Die Annahmen des Entwurfs gelten mit diesen Abweichungen:

1. **Ausnahme E3 ist enger als im Entwurf.** Durch People Directory als
   Koordinator über School Membership und Care Plan laufen nur die
   Kinderanlage (`CreateEnrollment`,
   `modules/peopledirectory/internal/application/student_enrollment.go:78–104`)
   und das Student-Update (`ApplyEnrollmentProfile` `:42–76`,
   `RenewEnrollment` `:106–126`). Die
   [Inventur](../architecture/4538-cross-owner-write-inventory.md) führt sie
   als E3 (Zeilen „People student create/profile/renew → School Membership“
   und „… → Care Plan“).
2. **Der Guardian-Link ist ein Leaf.** Beziehung, Abholrecht und
   Portalzugang schreibt People in einem Savepoint
   (`peoplepostgres/guardian_relationships.go:322–357`, `:242–247`); die
   Inventur führt das als Leaf („Identity guardian link insert“, „People
   relationship creation/permissions → Identity access“, „People
   relationship/pickup patch → Care Plan“). Seine Batch-Form braucht keine
   Ausnahme; die Verträge dieses Addendums gelten trotzdem.
3. **Staff-Create und Staff-Update laufen nicht über einen E3-Koordinator.**
   Der Import ruft People, School Membership und Workforce direkt
   (`staff_import_owner.go:211–252`, `:383–433`; Ports
   `modules/dataimport/ports/ports.go:57–65`, `:150–159`, `:180–185`). Data
   Import ist Workflow-Owner (`backend/architecture/policy.json`, Kind
   `workflow`) und hält diese Reihenfolge selbst (Entscheidungspunkte 4 und 5 dieses ADR).
4. **Staff hat keine Advisory Locks mit eigenem Schlüssel je Zeile.** Die
   Advisory Locks einer Staff-Zeile gelten alle demselben mandantenweiten
   Schlüssel `staff-person-merge:<Schule>`, geteilt: in `CreatePerson`
   (`modules/peopledirectory/internal/adapters/postgres/store.go:73`,
   `peoplepostgres/person.go:511`) und in `CreateStaff`
   (`modules/schoolmembership/internal/adapters/postgres/store.go:190`).
   Eine Sortierung nach Schlüssel, wie der Entwurf sie vorsieht, entfällt.
   Exklusiv nimmt ihn nur der Workflow `staffpersonmerge`
   (`workflows/staffpersonmerge/compose/new.go:23`). Auch die
   Student-Anlage nimmt ihn über `CreatePerson`.
5. **Rollen kommen nicht über eine Port-Regel.** Die Identity-Lesungen bindet
   die Root als Funktionsfeld (`api/import_composition.go:107–108`) über die
   Regeln `data-import.compose.identity-queries` und
   `data-import.public.identity-role`. Die Namensauflösung senkt die Eingabe in
   Go (`strings.ToLower`) und vergleicht sie mit SQL `LOWER(name)`
   (`modules/identityaccess/internal/application/school_role.go:33`,
   `internal/adapters/postgres/school_role.go:44–53`). Der eindeutige Index
   ist groß/klein-sensitiv (`idx_roles_name_tenant (tenant_id, name)`); zwei
   Rollen gleichen Rangs mit gleichem `LOWER(name)` sind möglich, und
   `ORDER BY tenant_id IS NULL ASC LIMIT 1` (`:52–53`) legt zwischen ihnen
   keine Ordnung fest. Haben sie verschiedene Rechte, hängen `RoleID` und
   `role_grant_not_permitted` an dieser Wahl. Einen festen Tie-Break legt
   [#6234](https://github.com/moto-nrw/project-phoenix/issues/6234) fest,
   nicht dieses Addendum.
6. **Update-Werte sind nicht mit dem Entwurf vergleichbar.** Der Entwurf
   nennt für das Update 332/3539, ohne die geänderten Spalten festzuhalten.
   Die Messung oben spielt unveränderte Zeilen ein; geänderte Werte kosten
   zusätzliche Writes. Schritt 1 legt die Update-Szenarien deshalb mit
   festen Änderungen an.
7. **Kontingent: Der Entwurf übersieht sinkende Zeilen.** `CheckChildQuota`
   beurteilt einen Schreibvorgang als Ganzes
   (`modules/schoolmembership/internal/application/child_quota.go:25–86`).
   Senkt eine Update-Zeile die Kontingentzahl (Betreuung endet), kann der
   Batch passieren, obwohl sequenziell eine frühere Zeile abgelehnt würde.
   Punkt 8 unten regelt das.
8. **Ein Batch-Deadlock verbraucht einen Versuch.** Der Entwurf lässt
   `40P01` aus dem Batchpfad die ganze Transaktion wiederholen. Die Laufzeit
   zählt jeden solchen Fehler als Versuch (`tenant/runtime.go:324`, höchstens
   drei Wiederholungen nach `:284`, also vier Versuche). Dem Zeilenpfad
   blieben dann drei statt vier Versuche; ein 500 könnte einen heutigen
   Erfolg ersetzen. Punkt 2 und Punkt 5 (F8) regeln das.
9. **Zeilenergebnisse überleben den Savepoint.** `determineImportAction`
   trägt Zeilenfehler wie `already_exists` und `not_found` sofort in das
   Chunk-Ergebnis ein (`import_service.go:428–443`). Der Entwurf setzt beim
   Rollback nur die Config zurück. Ein Ergebnis aus Schritt 2 bliebe stehen
   und würde beim Replay ein zweites Mal gezählt. Punkt 2 regelt das.

### Entscheidung

1. **Ausnahme E3 umfasst Batch-Formen.** People Directory darf für den
   Datenimport Kinderanlage und Student-Update für die Zeilen eines Segments
   in einem Kommando
   ausführen und dabei School Membership und Care Plan je einmal gebündelt
   aufrufen. Damit ändert sich die Reihenfolge der Owner-Schritte über die
   Zeilen: erst alle Profile, dann alle Mitgliedschaften, dann alle
   Betreuungsdaten. Gates, Kontingentprüfung, Alles-oder-nichts und die Owner
   bleiben. Das ist keine Erlaubnis für weitere Owner oder einen weiteren
   Koordinator. Alle übrigen Batch-Kommandos sind Leafs nach der #4538-Regel
   „Command participation is allowed“.
2. **Chunk-Ausführung (#5827).** `applyAcceptedRows` (`batches.go:329–350`)
   bildet Segmente (Schlüssel: Student Karte, Vor- und Nachname,
   Guardian-E-Mail; Staff Personalnummer, E-Mail, Name; Klassenliste Name und
   Klasse). Segmente mit weniger als zwei Zeilen nehmen den Zeilenpfad. Je
   Segment:
   1. `ResolveChunk` als geschützter Read (Savepoint, wie
      `student_import_preview.go:70`); scheitert er, gilt `FindExisting` je
      Zeile wie heute.
   2. `determineImportAction` je Zeile in Verarbeitungsreihenfolge,
      unverändert.
   3. `SAVEPOINT chunk`, Batch-Kommandos, `RELEASE`.
   4. Jeder Fehler in Schritt 3, auch `40P01`/`40001`: `ROLLBACK TO
      SAVEPOINT chunk`, Zeilenergebnisse und Zähler zurücksetzen (unten),
      Speicherzustand der Config verwerfen (Namenskonsum, Staff-Index,
      `rolesByID`, Command-Observations), erneut `PreloadReferenceData`,
      dann das Segment in derselben Transaktion im Zeilenpfad wiederholen
      (Replay). Der Replay bestimmt das Ergebnis.

   **Zurücksetzen der Zeilenergebnisse.** Vor Schritt 1 hält der Executor den
   Stand des Chunk-Ergebnisses fest: `Errors` (Länge), `ErrorCount`,
   `WarningCount`, `CreatedCount`, `UpdatedCount`. Beim Rollback setzt er
   genau auf diesen Stand zurück. Damit fällt alles weg, was Schritt 1 bis 3
   dieses Segments eingetragen haben, auch `duplicate_check_failed`,
   `already_exists` und `not_found`; die Validierungsfehler des Chunks
   (`importValidationSlice`, `batches.go:413–431`) und die Ergebnisse
   früherer Segmente bleiben. Der Replay trägt dann jede Zeile genau einmal
   ein. Danach entspricht das Chunk-Ergebnis in Inhalt, Reihenfolge und
   Zählern exakt dem sequenziellen Lauf.

   **Retry-Budget.** Ein `40P01`/`40001` aus Schritt 3 verlässt die
   Transaktion nicht: Der Executor fängt ihn wie jeden anderen Fehler im
   Savepoint ab (`ROLLBACK TO SAVEPOINT` hebt den abgebrochenen Teil auf und
   gibt dessen Sperren frei), merkt sich am Versuchsobjekt, das die Laufzeit
   für alle Wiederholungen wiederverwendet (`batches.go:269–274`), dass dieser
   Chunk den Zeilenpfad nimmt, und spielt den Replay im selben Versuch ab.
   Der Batch-Deadlock verbraucht so keinen Versuch. Der Zeilenpfad hat
   Versuch 0 und die drei Wiederholungen der Laufzeit, also dasselbe Budget
   wie heute; alle Wiederholungen dieses Chunks laufen im Zeilenpfad. Die
   Laufzeit (`tenant/runtime.go`) ändert sich dafür nicht.

   Im Batchpfad entfällt der Zeilen-Savepoint der Configs; das Legacy-`Import()`
   behält ihn.
3. **Batch-Formen.** Jedes Kommando tritt der Transaktion des Aufrufers bei
   (`RequireTransaction`), schreibt nur eigene Tabellen, filtert
   `tenant_id = ?` aus dem Scope, liefert Ergebnisse indexgleich zur Eingabe
   und ist alles oder nichts. Neue Methoden kommen in eigene, schmale
   exportierte Interfaces, damit `TestModuleFacadeWidthRatchet` nicht steigt.

   | Owner | Kommando | Ersetzt je Zeile | Einordnung |
   |---|---|---|---|
   | People Directory | `CreateEnrollmentStudents(ctx, []EnrollmentStudent) ([]CreatedEnrollmentStudent, error)` | `CreateEnrollmentStudent` | Ausnahme E3 |
   | People Directory | `ReadEnrollmentStudents(ctx, ids []int64, lock string) ([]EnrollmentRecord, error)`; fehlende ID ergibt je Index `ErrStudentNotFound` | `ReadEnrollmentStudent(…, "update")` | Teil der Ausnahme E3 (Klassen-Gate vor Kind-Sperre) |
   | People Directory | `ApplyEnrollmentUpdates(ctx, []EnrollmentUpdate) error` mit `EnrollmentUpdate{StudentID int64; Renewal *EnrollmentStudent; Profile *EnrollmentProfilePatch}` | `RenewEnrollmentStudent`, `ApplyEnrollmentProfile` | Ausnahme E3 |
   | People Directory, intern | Port `StudentOwners`: `EnrollBatch(ctx, []StudentRecord) ([]int64, error)`, `RenewBatch(ctx, []StudentRecord) ([]int64, error)`, `SaveCareBatch(ctx, []StudentCare) error` | `Enroll`, `Renew`, `SaveCare` | Teil der Ausnahme E3 |
   | School Membership | `EnrollStudents(ctx, []StudentEnrollment) ([]int64, error)`, `RenewEnrollments(ctx, []StudentEnrollment) ([]int64, error)`: ein Klassen-Gate geteilt, ein Kontingent-Lock, eine Prüfung | `Enroll`, `RenewEnrollment` | Leaf, aufgerufen von E3 |
   | Care Plan | `SaveStudentCareProfiles(ctx, []StudentCareProfileSave) error` | `SaveStudentCareProfile` | Leaf, aufgerufen von E3 |
   | People Directory | `LinkGuardiansToStudents(ctx, []LinkGuardian) ([]GuardianLink, error)` mit Care Plan `CreateGuardianPickupPermissions` und Identity & Access `GrantGuardianStudentAccesses` | `LinkGuardianToStudent` | Leaf |
   | People Directory | `CreatePersons`, `UpdatePersons`, `CreateGuardians`, `AddGuardianPhones` | Einzelkommandos | Leaf |
   | School Membership | `CreateStaffBatch`, `UpdateStaffBatch`, `CreateTeachers`, `UpdateTeachers`, `CreateClassListEntries` | Einzelkommandos | Leaf |
   | Workforce | `CreateStaffMasterDataBatch`, `UpdateStaffMasterDataBatch`, `ReplaceStaffQualificationsBatch` | Einzelkommandos | Leaf |
   | Identity & Access | `FindRolePermissionsByRoleIDs(ctx, []int64) (map[int64][]string, error)`; `ListSchoolRoles` liefert zusätzlich den Schlüssel `LOWER(name)` aus SQL | `FindSchoolRoleByName`, `FindRolePermissions` (#5887) | Leaf |
   | Student Presence, Audit | `RecordPrivacyConsents`, `RecordTransitionsBatch`, `AppendAll` | Einzelkommandos | Leaf |

   Die Signaturen sind die Form, nicht der genaue Name; die Umsetzung darf
   Namen und Werttypen an das Vokabular des Owners anpassen (ADR 0080). Sie
   darf keine Zeile, keinen Fehler und keine Sperre anders zuordnen als hier
   festgelegt. Anfangssalden (Workforce) brauchen einen eigenen Entwurf vor
   der Umsetzung (Umsetzungsschritt 8).
4. **ID-Vertrag (Entscheidung E3).**
   - Jede erzeugte ID gehört genau ihrer Eingabezeile. Zugeordnet wird über
     einen eindeutigen fachlichen Schlüssel (`student_profiles.person_id`,
     Beziehung `(student_id, guardian_profile_id)`, Schedule
     `(student_id, weekday)`, Consent `student_id`, Staff `person_id`,
     Teacher und Stammdaten `staff_id`) oder, ohne Schlüssel, über die
     Eingabeposition: `INSERT … SELECT … FROM unnest(…) WITH ORDINALITY`
     bzw. `MERGE … ON false … RETURNING src.ord, id`. Nie über eine
     angenommene `RETURNING`-Reihenfolge.
   - Die Reihenfolge der IDs innerhalb eines Chunks ist kein Vertrag.
   - Erlaubt: zusätzlicher Sequenzverbrauch durch fehlgeschlagene
     Batch-Inserts und den Replay, Lücken, andere IDs als heute.
   - Nicht erlaubt: Entitäten einer Zeile an einer anderen Zeile, ein
     teilweise sichtbarer Chunk, IDs in Antwort oder Receipt (beide enthalten
     heute keine, `batches.go:433–439`).
   - Jede Batch-INSERT nennt dieselben Spalten wie der Zeilenpfad. Eine
     Spalte, die der Zeilenpfad dem DB-Default überlässt, überlässt auch der
     Batch ihm; eine Spalte, die der Zeilenpfad mit NULL schreibt, schreibt
     auch der Batch mit NULL.
   - Zeilentrigger feuern je Zeile in Eingabereihenfolge. Statement-Trigger
     gibt es auf den berührten Tabellen heute keine (der einzige Fall ist
     `active.visits`, Migration `001015705`); jeder Umsetzungsschritt prüft
     das erneut.
5. **Fehler-, Status- und Rollback-Vertrag je Fehlerpunkt.** Phase und
   Reihenfolge jedes 4xx bleiben (Entscheidung E4). „Committet“ meint den
   Stand nach dem Ende der Anfrage.

   | Fehlerpunkt | Status, Code heute | neu | committet nach Abbruch |
   |---|---|---|---|
   | F1 Validierung (Mode, Checkpoints, Vorlast, Zeilen) | Zeilenfehler 200 bzw. 500 | gleich; der Bulk-Read der Rollen (#5887) läuft in einem Savepoint und fällt bei Fehler auf den Read je Zeile zurück, der dieselben `role_lookup_failed` liefert | frühere Chunks; nichts aus der Validierung |
   | F2 Import-Lock, Checkpoint, Vorlast im Chunk | 500 `import.import_batch_failed` | gleich, unverändert vor dem Segment | frühere Chunks |
   | F3 `ResolveChunk` scheitert | – (gibt es heute nicht) | kein Ausgang: Rollback des Read-Savepoints, dann `FindExisting` je Zeile wie heute | – |
   | F4 Match-Fehler je Zeile, `already_exists`, `not_found` | Zeilenfehler, Chunk läuft weiter | gleich, gleiche Reihenfolge | wie heute |
   | F5 Batch-Kommando scheitert (SQL-Fehler, `BusinessRejection`, Owner-Fehler) | – | kein Ausgang: `ROLLBACK TO SAVEPOINT chunk`, Zeilenergebnisse auf den Stand vor dem Segment, dann Replay | – |
   | F6 Replay scheitert in Zeile k | 500 `import.import_batch_failed`; 409 mit `details.rejection` bei `BusinessRejection` | gleich: dieselbe erste Fehlerzeile, derselbe Zeilenfehler, dieselben Details (etwa `occupied_places`, `requested_places`), dieselben bestätigten Zähler | frühere Chunks; vom Chunk nichts |
   | F7 Replay gelingt | – | Chunk committet wie heute; Observation `batch_fallback` | frühere Chunks und dieser Chunk |
   | F8 `40P01`/`40001` im Batchpfad | – | wie F5, im selben Versuch; kein Versuch verbraucht; der Chunk bleibt für alle Wiederholungen im Zeilenpfad (Punkt 2, Retry-Budget) | wie heute |
   | F9 `40P01`/`40001` im Zeilenpfad, Retries erschöpft | 500 | gleich | frühere Chunks |
   | F10 Checkpoint-Append | 500 | gleich | frühere Chunks |

   Abbruchpriorität: `StopOnError` vor dem Schreiben (`batches.go:103`), dann
   wiederholbarer DB-Konflikt im Zeilenpfad, dann die erste Fehlerzeile des Replays in
   Verarbeitungsreihenfolge, dann der Checkpoint. Im Erfolgsfall sind
   `Errors` (Inhalt und Reihenfolge), `CreatedCount`, `UpdatedCount` und
   `WarningCount` identisch; `Timestamp` ist kein Vertrag.

   Der Replay ist nötig, weil der Batchpfad Fehler in anderer Reihenfolge
   findet: Das Kontingent-409 aus `EnrollStudents` käme vor einem 500 aus dem
   Guardian-Kommando einer früheren Zeile, und `ChildQuotaReachedError`
   nennte `Occupied`/`Requested` des ganzen Batches statt der Zeile k. Das
   verbietet Entscheidung E4.
6. **Frische der Reads (Entscheidung E2).**

   | Read | Lock, den er voraussetzt | Regel |
   |---|---|---|
   | Rollen und Rechte der Staff-Validierung (#5887) | keiner; die Validierung nimmt keinen Lock | ein Snapshot je Validierungstransaktion erlaubt; über Transaktionsgrenzen nicht |
   | Namens- und Match-Auflösung (`ResolveChunk`) | Import-Schlüssel `data-import:<key>` | nach dem Import-Lock, vor den Gates. Kein Lock serialisiert Namensgleiche aus anderen Anfragen; der Student-Create liest heute schon gebündelt vor dem ersten Gate (`student_import_write_batch.go:11–15`) |
   | Kinddatensatz beim Update | Klassen-Gate geteilt und Zeilensperre des Kindes (`peopledirectory/internal/adapters/postgres/enrollment_read.go:17–29`) | erst nach Gate und Sperre, einmal je Kind im Batch (`ReadEnrollmentStudents`); kein Read davor ersetzt ihn |
   | Kontingentzahl und Grenze | Kontingent-Lock exklusiv (`schoolmembership/internal/adapters/postgres/student_counts.go:113`) | erst nach dem Lock, je Batch-Kommando einmal vorher und einmal nachher |
   | Guardian-Profil beim Link | Zeilensperre des Guardians (Update in `guardian_relationship_insert.go:5–30`) | `account_id` aus `RETURNING` der sperrenden Anweisung, nie aus einem früheren Read |
   | Rollen im Schreibpfad (`rolesByID`, Caregiver-Bedarf) | keiner | aus der Vorlast des Chunks, wie heute |

   Weitere Schreiber unter demselben Lock: Unter dem Klassen-Gate geteilt
   schreiben alle Kind-Schreiber, unter dem Kontingent-Lock alle zählenden
   Schreiber. Kein vorgezogener Read darf Daten liefern, die einer von ihnen
   unter diesen Locks ändert. Deshalb steht kein Kind-, Kontingent- oder
   Guardian-Read vor seinem Lock.
7. **Lock-Invarianten je Position.** Für einen Chunk mit den Zeilen 1…n in
   Verarbeitungsreihenfolge und einem Abbruch in Zeile k:

   | Sperre | heute gehalten bei Abbruch in k | neu während des Batch-Versuchs | neu bei Abbruch in k |
   |---|---|---|---|
   | `data-import:<key>` exklusiv | ab Chunk-Beginn | ab Chunk-Beginn, vor dem Savepoint | gleich |
   | `staff-person-merge:<Schule>` geteilt | ab der ersten Person | ab dem ersten Batch-Kommando, einmal | wie heute (Replay) |
   | Klassen-Gate geteilt, Kontingent exklusiv | ab dem ersten Kind | ab dem ersten Batch-Kommando, einmal | wie heute (Replay) |
   | Kind-Zeilen (Update) | Zeilen 1…k | alle Kinder des Segments, aufsteigend nach ID, vor dem ersten Schreiben | Zeilen 1…k |
   | bestehende Guardian-Zeilen (Link, Telefon) | Guardians der Zeilen 1…k | alle Guardians des Segments, aufsteigend nach ID | Guardians der Zeilen 1…k |
   | sonst geänderte bestehende Zeilen (Personen, Staff, Caregiver, Stammdaten, Schedules) | die der Zeilen 1…k | je Batch-Kommando alle des Segments, aufsteigend nach ID | die der Zeilen 1…k |
   | neu angelegte Zeilen | für andere unsichtbar | für andere unsichtbar | – |

   `ROLLBACK TO SAVEPOINT` gibt alle Sperren frei, die nach dem Savepoint
   genommen wurden; das gilt für Zeilen- und Tabellensperren (PostgreSQL,
   „Explicit Locking“) und wird für transaktionsgebundene Advisory Locks
   durch Test belegt. Der Replay fordert dann in heutiger Reihenfolge an.

   Damit gilt bei Abbruch dieselbe Sperrmenge wie heute. Während des
   Batch-Versuchs sperrt der Batchpfad aber spätere Zeilen, bevor eine
   frühere Zeile ihren Konflikt erreicht hat, und nach einer Ablehnung in
   Zeile k bleiben die Zeilen hinter k bis zum `ROLLBACK TO SAVEPOINT`
   gesperrt. Das ist nur über Entscheidung E10 gedeckt.

   **Bedingung.** Eine Batch-Form darf spätere Zeilen nur früher sperren,
   wenn sie alle Auflagen von E10 erfüllt. Wortlaut der Entscheidung vom
   08.11.2026: „Ein Batch-Lock darf spätere Zeilen früher sperren. Auflagen:
   Sperren in fester PK-Reihenfolge wie bei den anderen Schreibern;
   `lock_timeout`/`NOWAIT` unverändert; die erste Fehlerstelle und ihr Status
   (z. B. 409) sind dieselben wie sequenziell; vor der ersten Fehlerstelle
   wird nichts persistiert, was vorher nicht persistiert wurde; Real-DB-Test
   mit parallelem Schreiber gegen Deadlocks.“ Im Import heißt das:

   1. **Sperren in fester PK-Reihenfolge wie bei den anderen Schreibern:**
      Kinder, Guardians und andere bestehende Zeilen aufsteigend nach ID, wie
      Sammelgenehmigung und Begleiter-Sperren (Abschnitt #4640 oben,
      `modules/careplan/internal/application/companions.go:41–58`).
   2. **`lock_timeout`/`NOWAIT` unverändert:** Der Import sperrt weiter
      blockierend, ohne `NOWAIT` und ohne `lock_timeout`.
   3. **Die erste Fehlerstelle und ihr Status (z. B. 409) sind dieselben wie
      sequenziell:** durch den Replay (Punkt 5 dieses Addendums).
   4. **Vor der ersten Fehlerstelle wird nichts persistiert, was vorher nicht
      persistiert wurde:** durch den Chunk-Savepoint und das Zurücksetzen der
      Zeilenergebnisse (Punkt 2).
   5. **Real-DB-Test mit parallelem Schreiber gegen Deadlocks:** siehe
      „Jeder Schritt prüft“ unten, mit blockierendem und mit
      `NOWAIT`-Schreiber.

   Erfüllt eine Batch-Form eine Auflage nicht, nimmt ihr Segment den
   Zeilenpfad.

   **Folge für parallele `NOWAIT`-Schreiber.** `NOWAIT` nehmen heute nur die
   Begleiter-Protokolle, und nur für ein Kind mit kleinerer ID als einer schon
   gehaltenen:

   | Schreiber | `NOWAIT`-Stelle | Status bei belegtem Kind |
   |---|---|---|
   | Begleiter bearbeiten (Care Plan) | `modules/careplan/internal/application/companions.go:113` → `roots/owners/care_companion_students.go:90` → `modules/peopledirectory/internal/application/capability_student_directory.go:186` | 409 `students.companion_lock_busy` (`modules/careplan/http/companion_handlers.go:53–54`) |
   | Änderungsantrag genehmigen (Enrollment) | `modules/enrollment/internal/application/change_request_apply.go:321` über denselben Begleiter-Graphen; Fernenden `departure_sync.go:285–289` | 409 `students.companion_lock_busy` (`modules/enrollment/http/change_request_handlers.go:368–369`) |
   | Kind mit Begleitern bearbeiten (People) | `modules/peopledirectory/internal/application/student_write.go:323–328` | 409 `students.companion_lock_busy` (über `departure.ErrCompanionLockBusy`, `companion_handlers.go:53–54`) |
   | Kind löschen (Workflow `studentdeletion`) | `workflows/studentdeletion/deletion.go:617–621`, `:630–636` | 409 `students.deletion_companion_lock_busy` (`workflows/studentdeletion/http/student_delete_handlers.go:229–231`) |

   Hält der Import ein solches Kind, bekommen diese Schreiber heute wie
   künftig sofort die 409 dieser Tabelle, kein Warten, keinen 500 und keinen
   `40P01`. Neu ist nur das Fenster: Während des Batch-Versuchs kann auch ein
   Kind belegt sein, dessen Zeile sequenziell noch nicht dran wäre oder das
   hinter einer Ablehnung liegt. Eine solche Anfrage, die heute je nach
   Zeitpunkt durchkäme, bekommt dann die 409; nach `COMMIT` oder `ROLLBACK
   TO SAVEPOINT` gelingt dieselbe Anfrage. Blockierende Schreiber warten
   höchstens so lange; ihr Status ändert sich nicht.

   **Wahl: früher sperren, aber erst nach den Konfliktprüfungen, die vorher
   möglich sind.** Vorzuziehen wäre, Kandidaten erst nach der
   Konfliktprüfung zu sperren. Der Batchpfad tut das, wo die Lock-Reihenfolge
   es zulässt:
   - Alle Prüfungen, die keine Zeilensperre des Segments brauchen, laufen vorher: Validierung (F1),
     `ResolveChunk` und `determineImportAction` (F3, F4). Eine dort
     abgelehnte Zeile sperrt nichts.
   - Bei der Kinderanlage läuft `CreateEnrollmentStudents` mit der
     Kontingentprüfung vor `LinkGuardiansToStudents` und `AddGuardianPhones`.
     Ein volles Kontingent sperrt also keinen bestehenden Guardian, wie im
     Zeilenpfad, der das Kontingent jeder Zeile vor ihrem Guardian prüft.
   - Beim Update geht das nicht: Die Reihenfolge Klassen-Gate → Kind →
     Kontingent (`modules/peopledirectory/internal/application/student_enrollment.go:42–76`,
     `child_quota.go:14–24`) verlangt die Kind-Sperre vor dem Read und vor
     der Kontingentprüfung. Ein Kind erst nach dem Konflikt der vorigen Zeile
     zu sperren hieße, jede Zeile vor der nächsten fertig zu schreiben, also
     auf das gebündelte Update zu verzichten. Selbst eine Sperre je Zeile in
     Verarbeitungsreihenfolge sperrte die Zeile k+1 vor dem Schreiben der
     Zeile k. Sie kostete zudem ein Statement je Zeile (110 bei N110 gegen
     ein nahezu konstantes Ziel) und behielte die unsortierte Reihenfolge,
     die heute gegen die aufsteigend sperrenden Schreiber verklemmen kann.
     Die Sortierung senkt dieses Risiko. Deshalb sperrt das Update früher,
     unter den fünf Auflagen.

   Die Gate-Reihenfolge von Entscheidungspunkt 5 dieses ADR (Klassen-Gate
   vor Recurrence vor Transition) und „Gate vor Kontingent“
   (`child_quota.go:14–24`) bleiben.
8. **Kontingent.** `EnrollStudents` und `RenewEnrollments` prüfen das
   Kontingent einmal für den Batch. Das ist nur dann gleichwertig zur Prüfung
   je Zeile, wenn keine Zeile die Kontingentzahl senkt: Dann steigt die Zahl
   monoton, keine Zwischenstufe liegt über dem Endwert, und ein Batch, der
   passiert, hätte auch Zeile für Zeile passiert. Darum nimmt ein Segment den
   Zeilenpfad, sobald eine Update-Zeile nach `CountsTowardChildQuota`
   (`modules/dataimport/ports/imports.go:116–119`) mit dem unter Sperre
   gelesenen Datensatz vorher zählt und nachher nicht. Lehnt der Batch ab,
   bestimmt der Replay die Zeile und die Details. Der Stichtag wird je
   Batch-Kommando einmal gelesen; ein Chunk über Mitternacht zählt einheitlich.

### Umsetzungsreihenfolge

Jeder Schritt ist ein eigener PR, misst vor und nach und senkt die Register
auf den gemessenen Wert (Regel 15, shrink-only). Die erwartete Ersparnis ist
aus der Messung abgeleitet, nicht gemessen.

| Schritt | Issue | Umfang | Erwartet | Risiko |
|---|---|---|---|---|
| 1 | #5552 | Budget-Szenarien N10/N110 für alle Configs und Modi (`test/query-budgets/modules/dataimport/`, `max`, Vorschau `exact`); keine Verhaltensänderung | 0 | gering |
| 2 | #5887 | Rollen aus `ListSchoolRoles` mit SQL-Schlüssel `LOWER(name)`, Tenant- vor Systemrolle, exakter Name vor Alias; der Batch wählt exakt dieselbe Rolle wie der heutige Einzel-Read `FindSchoolRoleByName`. Hat ein Name mehr als einen Kandidaten gleichen Rangs, löst der Batch ihn mit diesem Einzel-Read auf, denn dessen Wahl ist heute nicht festgelegt (Befund 5); einen festen Tie-Break legt [#6234](https://github.com/moto-nrw/project-phoenix/issues/6234) fest. `FindRolePermissionsByRoleIDs` einmal in `ValidateBatch` | Staff-Vorschau auf etwa 10 je N | gering |
| 3 | #5552 | kein Zeilen-Savepoint im Batchpfad | −2 je Zeile | gering |
| 4 | #5827 | Chunk-Executor: Segmente, `ResolveChunk`, Chunk-Savepoint, Replay; erster Nutzer Klassenliste | Klassenliste N110 auf etwa 25 | mittel |
| 5 | #5552 | Student-Create, Leafs: Personen, Guardians, Telefone, Link, Consent, Schedules, Consent-Verlauf | etwa −8 je Zeile | mittel |
| 6 | #5552 | Staff-Create und Staff-Update, Leafs | etwa −10 je Zeile | mittel |
| 7 | #5552 | Ausnahme E3: Kinderanlage und Student-Update gebündelt, Kontingent nach Punkt 8 | Student-Create N110 auf etwa 60 | hoch |
| 8 | #5552 | Anfangssalden: eigener Workforce-Entwurf (Sperren je Staff sortiert, Guards nach dem Lock mengenbasiert, Einstellungen einmal je Transaktion), danach Umsetzung | etwa −40 je Zeile | mittel |

2, 3 und 4 sind unabhängig. 5 und 6 brauchen 4; 7 braucht 5.

Jeder Schritt prüft:

- **Äquivalenz:** jeder Fehlerfall einmal im Batchpfad und einmal mit
  erzwungenem Zeilenpfad. Gleich sein müssen Antwort-JSON ohne Timestamps,
  Owner-Zeilen ohne `id`/`created_at`/`updated_at`, NULL gegen Default und
  Fehlercodes.
- **Fehler in Zeile k:** k = 1, 5, 10 von 10 und k = 1, 55, 101, 110 von 110
  per Trigger, je Owner-Tabelle; Status, Code, Zeilenfehler, bestätigte
  Zähler und Persistenz wie im Zeilenpfad.
- **Zeilenergebnisse:** Zeile 2 ist `already_exists` (bzw. `not_found` im
  Update), das Batch-Kommando scheitert an Zeile 1. Nach dem Replay sind
  `Errors` (Inhalt, Reihenfolge), `ErrorCount`, `WarningCount`,
  `CreatedCount` und `UpdatedCount` identisch mit dem erzwungenen
  Zeilenpfad; kein Zeilenfehler doppelt, keiner verloren. Dasselbe mit einem
  `duplicate_check_failed` aus `ResolveChunk` und mit zwei Segmenten im
  Chunk, von denen das zweite zurückrollt.
- **Sperren, parallele Schreiber (E10, Auflage 5):** Real-DB-Tests mit zwei
  Verbindungen und PostgreSQL-Sperrbeobachtung. (a) Ein blockierender,
  aufsteigend sperrender Schreiber (Student-Edit mit Begleitern,
  Sammelgenehmigung) gegen ein Update-Segment: kein `40P01`, beide
  committen. (b) Ein `NOWAIT`-Schreiber der Tabelle in Punkt 7 auf ein Kind,
  das der Batch-Versuch hält: genau die dort genannte 409, kein 500; nach
  `COMMIT` bzw. `ROLLBACK TO SAVEPOINT` gelingt dieselbe Anfrage. (c) Ein
  Kontingent-409 im Batch: Nach dem Replay hält die Transaktion keine Sperre
  auf Zeilen hinter k; geprüft für Zeilen- und transaktionsgebundene
  Advisory Locks.
- **Retry-Budget:** ein erzwungener `40P01` im Batchpfad verbraucht keinen
  Versuch. Danach scheitert der Zeilenpfad dreimal mit `40P01` und gelingt
  im vierten Versuch; der Chunk committet wie heute. Ein vierter
  Zeilenpfad-Deadlock endet wie heute mit 500.
- **Bestehende Suiten grün:** `importtest/owner_failure_test.go`,
  `child_quota_batches_test.go`, `row_isolation_test.go`,
  `batches_test.go` (Rollback, Resume, parallele Uploads, Retry),
  `student_write_batch_test.go`, `guardian_execution_test.go`,
  `opening_balance_batches_test.go`, `guardian_link_owners_test.go` und die
  Lock-Order-Suiten der Inventur.

### Messplan N10/N110

Gemessen wird wie im Abschnitt „Messung“, je Modus nach den Warmups.
`N110 − N10` ist das Wachstum für 100 Zeilen; Ziel ist ein nahezu konstanter
Import.

| Szenario | heute N10 / N110 | Ziel | nach Schritt |
|---|---|---|---|
| `student_batch.import(_110)` | 235 / 2445 | etwa 33 / 60 | 7 |
| `student_batch.update(_110)` | 282 / 2989 (unveränderte Zeilen) | nahezu konstant; Wert gemessen | 7 |
| `staff_batch.preview(_110)` | 29 / 229 | 10 / 10 (`exact`) | 2 |
| `staff_batch.import(_110)` | 190 / 1901 | 171 / 1682 nach 2, danach gemessen | 2, 3, 6 |
| `class_list_batch.preview(_110)` | 24 / 224 | etwa 6 / 6 (`exact`) | 4 |
| `class_list_batch.import(_110)` | 80 / 786 | etwa 18 / 25 | 3, 4 |
| `opening_balance_batch.preview(_110)` | 199 / 2099 | im Entwurf zu 8 festgelegt | 8 |
| `opening_balance_batch.import(_110)` | 550 / 5861 | im Entwurf zu 8 festgelegt | 8 |

Die Ziele sind Ableitungen aus den Statements je Tabelle, keine Messung. Jeder
Schritt trägt den gemessenen Wert ein. `exact`-Budgets kollidieren mit
parallelen PRs; vor dem Merge wird gegen den aktuellen `development` neu
gemessen.

### Erwogene Optionen

- **Koordination in den Data-Import-Workflow ziehen:** Data Import riefe
  People-Profil, School-Membership-Mitgliedschaft und Care-Plan-Betreuung
  selbst. Das spaltete `CreateEnrollment`, das auch Enrollment (E1) und die
  HTTP-Kindanlage (E4) nutzen, und ließe die Sperren im alten Koordinator
  (Begründung der Ausnahme E3). Verworfen.
- **Nur Leaf-Batches, keine Ausnahme:** Student-Create endet bei etwa 1470
  statt etwa 60 Statements (N110), das Update bleibt linear. Verworfen.
- **Batch ohne Replay, Fehler direkt melden:** kürzer, aber andere erste
  Fehlerzeile und andere Kontingent-Details (Entscheidung E4). Verworfen.
- **Größere Chunks:** halten Gates und Kontingent länger und vergrößern den
  Wiederholungsumfang bei kaum weiterer Ersparnis. Verworfen; 100 bleibt.
- **Garantierte ID-Reihenfolge je Tabelle:** ein `nextval` vorab je Tabelle
  ohne fachlichen Schlüssel, ohne fachlichen Nutzen. Verworfen.

### Folgen und Risiken

- Ein abgelehnter Batch kostet den Batch-Versuch und den Replay; der
  Fehlerfall wird teurer, der Erfolgsfall billiger.
- Der Replay ist ein zweiter Ausführungsweg. Ohne Äquivalenztests kann er
  vom Batchpfad abweichen; darum sind sie Pflicht in jedem Schritt.
- Die Ausnahme E3 wird größer, nicht breiter: dieselben Owner und
  Sperren, eine Batch-Form mehr. Die Architektur-Checks sehen das nicht
  (Abschnitt „Evidence and enforcement“); der Review prüft es.
- Die Befunde korrigieren den Entwurf in #5552. Seine Tabelle
  „Owner-Batch-Kommandos“ gilt mit den Einordnungen dieses Addendums.
