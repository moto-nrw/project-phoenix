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

Care Plan no longer publishes `CareResumption` or embeds `Resume` in
`CareExitCommands` / `CareLifecycle`. Its application and composition accept
no resumption collaborator. This supersedes ADR 0067's #4538 compatibility
binding and implements ADR 0083's preferred explicit workflow entry.

`modules/careplan/http.CareResumption` is a consumer-owned transport port,
not a domain command. The root constructs `careexit.Resumption` and passes it
as a separate mount argument to that HTTP adapter, not as a field of the
Care Plan schedules or another root service graph (ADR 0059). The handler
calls the workflow directly;
no Care Plan domain command calls it. Point 2 applies to runtime calls as
well as imports: declaring this port in HTTP grants no new workflow callback
from a domain application. The two granted E2 callbacks are bounded below;
other existing domain callbacks are debt, not additional exceptions.
Only the root names the workflow type.
Care Plan retains its input value and error identities; neither declares a
workflow dependency. No policy rule, owner, epoch or cycle is added.

The workflow keeps its complete unit and protocol: shared class gate → child
lock and re-read → exit check → guarded Membership resume → exit deletion →
removal-ledger deletion → audit. It restores no plan and acquires no recurrence
gate. Tenant scope, conditional refusal, HTTP responses and audit stay intact.
The contract/workflow tests keep their assertions through the separate root
entry; the HTTP regression removes the domain lifecycle and still resumes
through the real workflow binding.

### Remaining roster and booking callbacks: bounded E2 compatibility

`NewCareLifecycleOwners` still binds `careexit.NewRoster` into the lifecycle's
private roster port. This is the same runtime direction, not evidence that
root binding makes domain-to-workflow calls generally valid. Unlike the
standalone resumption entry, the roster reads attendance, removes/restores
Timetable participants and restores/reconnects Presence state inside the
existing confirm/cancel/due protocol. Moving its constructor or adding a
forwarder would leave that coordination in the domain.

The second callback is `careexit.NewBookings`, constructed in
`roots/owners/care_exit_enrollment_bookings.go`. Care Plan's
`internal/application/care_exit_cleanup.go` calls
`s.owners.Enrollment.CareExitOfferingSnapshots` and
`RestoreCareExitOfferingLinks` through `careExitEnrollment` →
`EnrollmentBookingProjection.Bookings`; `NewCareLifecycleOwners` binds that
Enrollment port in `roots/owners/care_lifecycle_owners.go`.
[ADR 0075's #4531 amendment](0075-pass-throughs-keep-a-per-owner-floor.md#amendment-of-2026-10-23)
retains snapshot and restore in the workflow because they read Enrollment
applications and submitted choice notes and preserve the ledger's
Enrollment-shaped snapshot. [ADR 0083](0083-ratchets-protect-invariants-and-expose-command-effects.md#findings-the-successor-must-represent)
keeps that boundary. Locking and ending source bookings stay on Care Plan's
own `OfferingBookings`; this exception does not restore that removed round trip.

Grant only these two workflow-backed bindings as the runtime part of
ADR 0067's E2 exception: the existing roster-port methods and the two
Enrollment-port snapshot/restore methods, caller's tenant transaction, ledger
snapshot/restore and class(shared) → recurrence → child/plan locking remain
unchanged. No new method, owner, effect or independent transaction is covered.
Remove both callbacks from the domain application, and this exception, when
E2's complete end/cancel/due and withdrawal protocol moves into an explicit
workflow, keeping both roster and booking snapshot/restore within that protocol.
This larger cut is outside #4740; resumption is not covered by this exception.
ADR 0083's typed effect successor must report both bindings as workflow-backed,
not as leaf calls. The lifecycle's separate `Bookings` port uses the Timetable
owner through `careExitBookings`, not `careexit.Bookings`, and is not another
workflow callback.

### Workflow entries, known debt and open platform-trigger question

HTTP → root → workflow, direct root, Worker and CLI entries may invoke a
workflow. A domain application's call through a root-bound port is not such
an entry: point 2 also forbids that runtime dependency, unless an ADR grants
a bounded exception. The E2 grant above covers Roster and Bookings only;
it does not claim that these are the only existing domain callbacks.

The following existing bindings are known debt, **not granted exceptions**.
Paths below are relative to `backend/`:

- **Care Plan → Parent Portal share visibility:**
  `roots/owners/root_careplan_requests.go:43-48` constructs
  `parentportalcompose.NewRequestShareVisibility` and binds it to the request
  queues' share-visibility ports. Domain calls include
  `modules/careplan/internal/application/care_request_effects.go:305` and
  `offering_change_notices.go:130` (`SharedRecipientAccountIDs`).
- **People Directory → Staff Offboarding live employment:**
  `modules/peopledirectory/internal/application/birthday.go:123` calls the
  birthday-store port; the root adapter in
  `roots/owners/staff_membership_adapters.go:545-551` constructs
  `staffoffboarding.NewLiveStaffEmployment` and calls
  `SetBirthdayDisplayOptOut`. The same adapter's `AddNotes` binding at
  `:554-559` constructs that workflow for `AppendNotes`; the inventory found
  no production caller of `AddNotes`, so it is not a second proven live path.

[#4825](https://github.com/moto-nrw/project-phoenix/issues/4825) tracks those
two bindings. Their presence does not widen E2; their removal is outside
#4740. The broader root/API inventory also found these ungranted domain
callbacks, outside #4825's two named paths:

- **File Storage → Parent Portal:** `roots/owners/root_portal.go` passes the
  portal as `GuardianAudience` to File Storage. Its attachment application
  calls `GuardianAnnouncementTenant` and
  `GuardianDeclarationProofAttachmentTenant` in
  `modules/filestorage/internal/application/attachments.go:435,452`.
- **Workforce → Shift Plan Sync:** `roots/owners/root_workforce.go:101`
  constructs `NewSickCascade` and passes it through `WithSickCascade` to the
  absence application's `ShiftPlanSync` port. Creation and mutation call
  `MarkSickForRange`, `ReassignSickStamps`, `ReconcileSickRange` and
  `ClearSickForRange` inside `modules/workforce/internal/application/timetracking/`.
- **School Structure → Shift Plan Sync:** `api/root_views.go:65,76` binds
  `NewSubstitution` as `Schedule` and `NewGroupHandovers` through
  `EducationHandovers` as `Substitutions`. The domain's
  `modules/schoolstructure/internal/application/groups/substitution_module.go:112,298,397`
  calls the
  schedule workflow; `:338,453` reaches the handover workflow through the
  adapter's `Create` / `Delete`. These are two bindings, neither covered by E2.

For runtime calls, this addendum applies point 2 to **domain applications**.
It does not decide whether platform triggers count as workflow entries:
`roots/owners/reminder_outbox.go:90,99` binds Reminder Delivery's
`NewPushDelivery` behind the Delivery dispatcher/recovery provider, and
`roots/owners/personal_reminder_notifications.go:23` binds
`NewPersonalReminderNotifications().NotifyDue` to Scheduler Runtime's
`ReminderNotificationTick`. Whether those platform callbacks are permitted
entries remains open; this addendum neither grants nor rejects them.
