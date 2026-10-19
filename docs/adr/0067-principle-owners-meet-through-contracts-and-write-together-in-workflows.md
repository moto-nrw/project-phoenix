---
status: accepted
---

# Principle: owners meet through contracts and write together in workflows

Principle ADR of 2026-10-19 under
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580), written for
[#4091](https://github.com/moto-nrw/project-phoenix/issues/4091). It states the
rules for contracts, workflows and transactions that still hold. The ADRs it
cites keep their context and history; this ADR changes none of them.

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

- Every write that spans owners is visible as a workflow owner in
  `policy.json`. A new workflow owner is registered through the reviewed
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
