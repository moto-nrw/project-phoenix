---
status: accepted
---

# Principle: owner dependencies point one way

Principle ADR of 2026-10-19 under
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580), written for
[#4091](https://github.com/moto-nrw/project-phoenix/issues/4091). It states the
rules for dependency direction and cycles that still hold. They come from
[ADR 0058](0058-owner-dependencies-form-a-dag.md) and its addenda, which keep
their measurements and history; this ADR changes none of them.

## Decision

1. **The target owner graph is acyclic.** The policy loader rejects a policy
   whose target projection has a cyclic component, and `scripts/backend-architecture.sh
   cycles` names it. There is no member list and no allowance; ADR 0058
   supersedes
   [ADR 0032](0032-the-target-may-keep-two-cyclic-components-and-they-only-shrink.md)
   (ADR 0058 point 1).
2. **Each owner kind depends in one direction.**
   - A platform depends on platforms and the shared kernel only. Where it needs
     domain facts, it declares a consumer-owned port that the root binds.
   - A domain depends on platforms and on other domains' `public` or
     `contract` roles, in one direction per pair.
   - A projection reads domains and may consume domain and platform `public`
     or `contract` roles; a domain that needs a projection's result declares
     its own reader port.
   - A workflow depends on domains, projections and platforms. Only roots
     depend on workflows.
   - No owner imports a root in production, so the roots add no cycle.
   (ADR 0058 point 2 and addendum #4314.)
3. **Two domains that need each other get one direction.** The other direction
   becomes a consumer-owned port bound in the root, or the shared behaviour
   moves into a workflow when it writes to both (ADR 0058 point 3).
4. **The recorded layering stands.** People Directory is the lowest of the
   coupled domains; Care Plan sits above People Directory, Enrollment above
   Care Plan, and Workforce above Student Presence (ADR 0058, addendum #3417).
5. **A domain's implementation follows the owner's existing edges.** A
   domain's `application`, `postgres` and `port` roles may reach the owner's
   own `public` role, the `public` role of Tenant Runtime and Transaction
   Runtime, and another domain's `public` role where the owner already has
   that edge; `postgres` may reach the owner's own `port` role. Such a rule
   moves an edge into the implementation and adds none to the owner graph; the
   reverse direction stays forbidden (ADR 0058, addendum #4084).
6. **The measurement counts every production package**, including test
   support compiled into production. Test scopes stay out (ADR 0058 point 4).
7. **Tools are outside the dependency graph.** `architecture-check` and
   `dev-tools` own no writable data and take no part in the domain DAG
   ([ADR 0060](0060-the-dev-tools-are-a-binary-of-their-own.md), addendum of
   2026-10-18).

## Consequences

- An acyclic owner graph has a build order, which the roots use to construct
  owners without late binding
  ([ADR 0068](0068-principle-owners-compose-themselves-and-the-roots-only-wire.md)).
  A dependency that seems to need late binding is a cycle and is resolved
  here.
- A rule that would close a cycle is not target and cannot be written
  ([ADR 0070](0070-principle-only-the-target-is-a-rule-and-debt-leaves-with-evidence.md)).
  Both the target and the migration graph currently have no cyclic component.

## References

ADR 0058 with its addenda carries the decision; ADR 0032 is superseded by it,
and ADR 0060 adds point 7. The full rule map is the
[traceability appendix](../../backend/architecture/README.md#appendix-principle-traceability).

## Enforcement addendum (#4540)

From policy epoch 91 the policy loader rejects every rule whose selectors
admit a domain source and a projection target, including `public` and
`contract` targets and test scopes. There are no exceptions. Earlier epochs
remain loadable as immutable PR bases. This enforces decision point 2 rather
than adding a permitted dependency direction.

Timetable's `ports.PresenceReader` owns its activity-instance and participant
attendance values. The root translates the Presence projection's rows and
filters at that port; tenant transactions and the projection's SQL stay
unchanged. Communication already owns its audience port and values; its
announcement-audience mapping moves from the owner composition to the root.
The three domain-to-projection rules are deleted with no replacements.

## Addendum 2026-10-23: explicit care-resumption entry (#4740)

### Runtime dependency rule

Point 2 applies to runtime calls from domain applications, not just imports.
This includes interface ports, function/method values, closures, input ports
passed back by a workflow, and pure functions belonging to an owner whose
policy `kind` is `workflow`. Root binding does not make these calls valid.
Read-only effects do not exempt a domain-to-workflow dependency.

### Only granted runtime exception: care-exit E2 callbacks

Grant only these two existing workflow-backed bindings as the runtime part of
[ADR 0067's E2 exception](0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md#finite-retained-coordinator-exceptions).
Paths below are relative to `backend/`:

- **Roster:** `roots/owners/care_lifecycle_owners.go` binds
  `careexit.NewRoster` to Care Plan's lifecycle roster port. It coordinates
  Timetable participants, attendance and Presence pickup links within the
  retained confirm/cancel/due protocol.
- **Bookings, snapshot/restore only:**
  `roots/owners/care_exit_enrollment_bookings.go` constructs
  `careexit.NewBookings` as `EnrollmentBookingProjection.Bookings`.
  `careExitEnrollment` in `roots/owners/care_exit_enrollment.go` forwards
  Care Plan's `CareExitOfferingSnapshots` and `RestoreCareExitOfferingLinks`
  calls from `modules/careplan/internal/application/care_exit_cleanup.go`.
  Snapshot reads Enrollment applications and submitted choice notes; restore
  replays the ledger's Enrollment-shaped snapshot, as decided in
  [ADR 0075 (#4531)](0075-pass-throughs-keep-a-per-owner-floor.md#amendment-of-2026-10-23)
  and retained by
  [ADR 0083](0083-ratchets-protect-invariants-and-expose-command-effects.md#findings-the-successor-must-represent).
  Locking and ending Care Plan's source bookings stay on its own
  `OfferingBookings`; this exception does not restore that removed round trip.

Moving constructors or adding forwarders would leave this coordination in
the domain. The grant covers only the existing roster-port methods and the
two Enrollment-port snapshot/restore methods within E2. The caller's tenant
transaction, ledger snapshot/restore and class(shared) → recurrence →
child/plan locking remain unchanged. No new method, owner, effect or
independent transaction is covered. Resumption is not covered.

Remove both callbacks from the domain application, and this exception, when
E2's complete end/cancel/due and withdrawal protocol moves into an explicit
workflow, keeping roster and booking snapshot/restore within that protocol.
This full E2 workflow cut is outside #4740. The command-effect inventory must
record both bindings as workflow-backed, not as leaf calls.

### Other existing bindings are ungranted debt

Every other existing runtime binding from a domain application to code owned
by a workflow is direction debt, not a granted exception. No enumeration in
this ADR is needed to establish that debt. New such bindings are forbidden.
The maintained debt inventory is
[#4825](https://github.com/moto-nrw/project-phoenix/issues/4825), including
function fields and function parameters; it is authoritative for tracking and
removal, not permission to retain or add callbacks. Removing this debt is
outside #4740.

The command-effect gate from
[ADR 0083](0083-ratchets-protect-invariants-and-expose-command-effects.md#domain-source-expansion-2026-10-23-4739)
inventories interface-port bindings. Its coverage of function fields and
parameters is incomplete; a green check is not proof that no runtime callback
exists or that a binding is permitted. This coverage limit is tracked in
[#4836](https://github.com/moto-nrw/project-phoenix/issues/4836) /
[PR #4837](https://github.com/moto-nrw/project-phoenix/pull/4837).

### Allowed workflow entries

HTTP, root, worker/CLI and workflow-to-workflow entries may invoke workflows;
these are not domain-application callbacks. Binding a workflow at such an
entry does not authorize passing it back into a domain application or settle
the separate platform-trigger question below. These entry categories are not
a complete binding inventory.

### Open question: platform triggers

Whether platform triggers count as workflow entries remains undecided.
Examples include Scheduler Runtime retention, AppointmentReminders and the
ReminderNotifications tick, and Delivery dispatcher/recovery callbacks.
This addendum neither grants nor rejects those platform callbacks. An HTTP,
root or CLI entry that installs a platform callback does not settle that
callback's direction, even when both paths share a workflow instance.

### Care resumption: domain command replaced by HTTP consumer port

Care Plan no longer publishes `careplan.CareResumption` or embeds `Resume`
in `CareExitCommands` / `CareLifecycle`. Its application and composition
accept no resumption collaborator. This replaces ADR 0067's #4538
compatibility binding with ADR 0083's preferred explicit workflow entry.

`modules/careplan/http.CareResumption` is a consumer-owned transport port,
not a domain command. `roots/owners/care_resumption.go` constructs
`careexit.Resumption`; `api/root_serve.go` passes its `Resume` method value
as a separate mount argument to HTTP. The handler in
`modules/careplan/http/care_exit_handlers.go` calls this port directly;
no Care Plan domain command calls it. Care Plan retains the input value and
error identities, not the workflow command.

## Addendum 2026-10-24: ungranted debt removed (#4825)

[ADR 0099](0099-domain-applications-no-longer-call-workflows.md) removes the
remaining domain-to-workflow runtime bindings of the #4825 inventory. PR #4833
removed the first two. No domain application now calls workflow code at
runtime, apart from the Care Exit Roster/Bookings E2 callbacks granted above.
The command-effect register lists no #4825 debt. The rule of
[Other existing bindings are ungranted debt](#other-existing-bindings-are-ungranted-debt)
still applies: a new binding of this kind is forbidden and is not debt to be
recorded.

The bindings took the two paths of #4825:

- **The capability belongs to a domain owner.** People Directory owns the
  relationship-type vocabulary. Care Plan owns its same-day cutoff value.
  Workforce owns the deletion record of its absences. School Membership offers
  a live-staff lock in place of a closure parameter. School Structure's
  handover store calls Workforce's group-substitution commands directly.
- **A workflow entry reached from HTTP or the root.** These are Parent
  Portal's guardian attachment downloads, Shift Plan Sync's Terminvertretung,
  and Shift Plan Sync's sick reports. For sick reports, Workforce returns a
  `SickCascadePlan`, and the entry applies it in the same tenant transaction.

Each HTTP route reaches its workflow through a port declared in its own
`http` package; the root binds the workflow there. Timetable's
pickup-extension routes and Data Import's observed opening commands now use
the same shape, instead of the domain interface they were typed with. The
open question about platform triggers is unchanged.
