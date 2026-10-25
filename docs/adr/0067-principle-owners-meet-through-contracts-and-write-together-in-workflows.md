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
