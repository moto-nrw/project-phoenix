---
status: accepted
---

# Pass-throughs keep a per-owner floor of four shapes

Decision of 2026-10-21 for
[#4459](https://github.com/moto-nrw/project-phoenix/issues/4459), part of
[#4403](https://github.com/moto-nrw/project-phoenix/issues/4403) under
[#3423](https://github.com/moto-nrw/project-phoenix/issues/3423). It records
the owner decision of 2026-10-20 on #4403 and changes no budget, no rule and
no scanner condition. The [amendment of 2026-10-21](#amendment-of-2026-10-21)
admits the owner-edge translations and four named exceptions into the floor,
lowers every budget to its floor and replaces the measurement. The
[amendment of 2026-10-23](#amendment-of-2026-10-23) cuts the Care Plan lock
round trip of decision point 3.

## Context

`TestModulePassthroughRatchet` counts a shape: an exported method of at most
14 body lines with exactly one call through a field of its receiver. Rule 8 of
`.claude/rules/backend-conventions.md` forbids a service method that only
forwards; it does not forbid every method of that shape. After the
facade-collapse rollout of #4403 (#4492, #4506, #4507, #4509, #4511; #4414,
#4480 and #4495 closed) the register reads 1,555 hits in 31 owners at
development `1ff680016b`, down from 2,925 on 2026-10-20. Every entry equals
its measured count.

The owner decided on #4403 not to drive the register to zero: about 600 of the
then 2,925 hits wrapped a tenant transaction and observation around one store
call, and the consumer-port adapters are the boundary ADR 0067 asks for.
Removing either would delete behaviour or break the boundary. The owner
decision on #4480 (section 5) and the #4495 measurement in #4507 named the
further items of this floor.

## Decision

1. **Four shapes form the floor.** A counted method belongs to the floor when
   it has one of these shapes:
   1. *Transaction and observation wrapper.* The field call sits in a function
      literal handed to the owner's tenant, administrative or platform
      transaction helper or its observation helper (`s.run`, `s.runWrite`,
      `runValue`, `requestCommand`, `withinTenant` and their siblings), or the
      field call is the transaction runner itself (`RunInTx`,
      `WithinTenant`). The wrapper is the unit of work; inlining it moves the
      transaction to every caller.
   2. *Consumer-port adapter* ([ADR 0067](0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md)
      point 2). The field is another owner's contract, the owner's
      root-facing `ports` package
      ([ADR 0064](0064-moved-code-stays-in-the-pass-through-scan.md)), or the
      method sits in a binding the root owns (`roots/*binding`, `api/`, the
      parent portal's care adapters), and the method translates values,
      errors or arguments on the way.
   3. *The Timetable adapters of #4480 section 5.* `CalendarPeriodRecords`
      over School Calendar, the enrollment schedules over Care Plan and the
      Records-to-public reads for Workforce stay as consumer-port adapters
      (owner decision on #4480).
   4. *Scanner limitation 5 of the ratchet header.* `Error()` on an error type
      that forwards to its wrapped cause, a fail-closed log line, a clock
      read. None of them reaches a repository or a port.
2. **Two documented items stay without a counted hit.**
   - `careplan.OfferingBookings` stays in the public package (#4495, #4507
     section 3). Moving it into the application layer turns
     `LockCareOfferingBookings` (observation plus one store call) into a
     counted hop and would raise `modules/careplan` from 163 to 164.
   - Identity & Access keeps the `engine` value that bundles its services for
     the `Owner` interface and its `DemoAccess()` accessor, which returns a
     field and is not counted (#4507 section 2).
3. **The Care Plan lock round trip is a contract-cut candidate, not a floor.**
   `CareLifecycle.lockPlanning` reaches its own booking lock through
   `ports.CareExitEnrollment.LockCareExitOfferingLinks` →
   `roots/owners` (`careExitEnrollment`, `EnrollmentBookingProjection`) →
   `workflows/careexit.Bookings` →
   `OfferingBookings.LockCareOfferingBookings`: Care Plan to Enrollment's port,
   the roots, the care exit workflow and back to Care Plan. Cutting it removes
   a tested cross-owner contract
   (`modules/careplan/contracttest/care_exit_storage_cutover_test.go`) and the
   `lock` booking observation, so it needs its own decision with the care exit
   contract. Until then the chain stays as it is.
4. **Everything else is outside the floor.** 789 hits have none of the four
   shapes. At `1ff680016b` they fell into three groups (the amendment admits
   the third and removes or names the other two):
   - *pure forward* (80): body is `return s.x.M(params...)` with the
     parameters and the result unchanged, inside the owner;
   - *binding without translation* (60): the same pure forward, but over
     another owner's contract or a root-facing port;
   - *owner-edge translation* (649): one call inside the owner with
     validation, or with internal values or sentinels mapped onto the owner's
     public contract. The group also holds seams whose bound owner the scan
     cannot establish: a locally declared interface or an `internal/ports`
     interface may be a store or another owner's port.

   Rule 8 names validation and error mapping as encapsulation, but #4459
   admits only transaction wrappers and consumer-port adapters (shapes 1 to
   3; shape 4 is no forward at all), so this ADR does not admit the
   owner-edge translations. Each hit outside the floor leaves by removal, or
   joins the floor by a later decision that names its shape and the
   evidence.
5. **The register stays as it is.** The scanner keeps its four conditions,
   every entry keeps its measured count and may only fall, and no entry,
   alias, allowlist or exception is added. The floor is a record, not a
   budget: an entry may fall below its floor when a floor hit goes away.

## Amendment of 2026-10-21

Decision of 2026-10-21 for #4403, recorded in its last two owner comments.
It follows #4522 and #4523, which removed the pure forwards and the bindings
without translation (#4520, #4519), and the last PR of #4403, which removed
the four People Directory hits the boundary between those two tracks had
left. It adds no rule, alias, allowlist entry, exception or scanner condition.

1. **Owner-edge translations join the floor.** A counted method whose one
   field call sits inside the owner and that validates, or maps internal
   values or sentinels onto the owner's public contract, belongs to the floor
   as a fifth shape. Rule 8 of `.claude/rules/backend-conventions.md` names
   validation and error mapping as encapsulation; the owner decided to apply
   that reading to the register. Decision point 4 no longer leaves these hits
   outside the floor. The group keeps the seams whose bound owner the scan
   cannot establish (a locally declared interface or an `internal/ports`
   interface).
2. **Pure forwards and bindings without translation leave.** All 140 of
   `1ff680016b` are gone except the four exceptions of point 3. A new one is
   no floor shape; it is removed, or it needs its own amendment that names it.
3. **Four named exceptions stay as counted hits.** Each is a pure forward that
   cannot go without a rename or a policy edge this decision does not grant:
   - `modules/careplan` `DocumentFileRecords.MarkFileDeleted`,
     `MarkQueuedCleanupCompleteByFilename` and `ActivateQueuedCleanup`
     (`internal/application/compose_bindings_care_lifecycle.go`). Neither side
     can take the other's name. File Storage's `documents.Store` port names
     the three methods and People Directory implements it as well. The Care
     Plan targets (`MarkCareDocumentFileDeleted`,
     `CompleteCareDocumentCleanupByFilename`, `ActivateCareDocumentCleanup`)
     are keys in `transaction_boundaries.json`, and that register admits no
     rename (`TestTransactionBoundaryRatchet` rejected one in #4522).
   - `modules/settings` `TenantOperations.Schema`
     (`internal/application/tenant_operations.go`). The HTTP port returns
     `(any, error)` and `GetSchema` returns `(*records.SettingsSchema, error)`.
     Typing the port makes `modules/settings/http` import `settings/records`,
     and no policy rule admits that edge; a new edge needs its own decision.
4. **Budget equals floor.** Every entry of `modulePassthroughOwnerBudgets`
   equals its measured count, and every counted hit is in the floor. Decision
   point 5 stays: the scanner keeps its four conditions, entries only fall,
   and a raise needs an ADR (ADR 0057 point 5). When a floor hit goes away,
   the entry falls and the floor with it.
5. **Decision points 2 and 3 stand.** `careplan.OfferingBookings` and the
   Identity & Access `engine` stay as recorded. The Care Plan lock round trip
   stays a later contract-cut candidate, not part of the floor.

## Measurement

Measured on this amendment's change (development `7146054191` plus the removal
of the four People Directory hits) over the ratchet's own scan
(`modulePassthroughScan`, `modulePassthroughOwnerKey`,
`modulePassthroughIsDelegation`): 1418 hits in 30 owners, each owner equal to
its budget.

The #4518 classification at `1ff680016b` was carried over by file, receiver
and method. 1417 of the 1418 hits matched. The one new hit,
`modules/classday` `CallerBinding.CurrentStaffID` (`StaffID` before #4522),
maps a not-found onto `classday.ErrStaffRecordRequired` through a locally
declared interface, so it is an owner-edge translation by the #4518 rule. 129
matched hits changed their body since `1ff680016b`; none of them became a pure
forward. What is left of the two removed groups is exactly the four exceptions
of amendment point 3.

The #4518 criteria per shape stay as they were: a transaction or observation
helper around the field call (56 helper and package pairs read) or a
`RunInTx`/`WithinTenant` call; a field typed from another owner's package or
from `modules/<owner>/ports`, or a file in `roots/`, `api/` or
`workflows/parentportal/internal/care/`, with a body that is no pure forward;
`Error` over `.Error`, or a call through `logger`, `clock` or a `Now` field.
Two counted files are fixture helpers without a `_test.go` suffix
(`modules/delivery/internal/application/intent_worker_test_helpers.go`,
`modules/timetable/compose/class_arrival_exceptions_test_helpers.go`); they
are counted like production code.

### Floor per owner

Columns: budget = measured count; the five floor shapes (Tx/obs, port
adapter, #4480 section 5, scanner limitation, owner-edge translation), the
named exceptions of amendment point 3, and their sum. No hit lies outside the
floor.

| Owner | Budget | Tx/obs | Port adapter | #4480 §5 | Scanner | Edge | Named | Floor |
|---|--:|--:|--:|--:|--:|--:|--:|--:|
| `modules/appointments` | 21 | 21 | 0 | 0 | 0 | 0 | 0 | 21 |
| `modules/auditlog` | 1 | 0 | 0 | 0 | 0 | 1 | 0 | 1 |
| `modules/careplan` | 159 | 115 | 7 | 0 | 0 | 34 | 3 | 159 |
| `modules/classday` | 14 | 0 | 3 | 0 | 1 | 10 | 0 | 14 |
| `modules/communication` | 57 | 22 | 3 | 0 | 1 | 31 | 0 | 57 |
| `modules/dataimport` | 1 | 0 | 1 | 0 | 0 | 0 | 0 | 1 |
| `modules/delivery` | 5 | 0 | 0 | 0 | 0 | 5 | 0 | 5 |
| `modules/devicefleet` | 33 | 7 | 2 | 0 | 3 | 21 | 0 | 33 |
| `modules/devicescan` | 35 | 0 | 25 | 0 | 3 | 7 | 0 | 35 |
| `modules/enrollment` | 24 | 7 | 0 | 0 | 0 | 17 | 0 | 24 |
| `modules/exporttransfer` | 1 | 0 | 0 | 0 | 0 | 1 | 0 | 1 |
| `modules/facilities` | 13 | 0 | 0 | 0 | 0 | 13 | 0 | 13 |
| `modules/feedback` | 2 | 2 | 0 | 0 | 0 | 0 | 0 | 2 |
| `modules/filestorage` | 6 | 1 | 0 | 0 | 0 | 5 | 0 | 6 |
| `modules/identityaccess` | 218 | 64 | 58 | 0 | 4 | 92 | 0 | 218 |
| `modules/mealplan` | 3 | 3 | 0 | 0 | 0 | 0 | 0 | 3 |
| `modules/organizationtenancy` | 11 | 11 | 0 | 0 | 0 | 0 | 0 | 11 |
| `modules/peopledirectory` | 210 | 58 | 47 | 0 | 1 | 104 | 0 | 210 |
| `modules/requestreview` | 15 | 0 | 15 | 0 | 0 | 0 | 0 | 15 |
| `modules/schedulerruntime` | 4 | 0 | 0 | 0 | 0 | 4 | 0 | 4 |
| `modules/schoolcalendar` | 16 | 9 | 0 | 0 | 0 | 7 | 0 | 16 |
| `modules/schoolmembership` | 16 | 14 | 0 | 0 | 0 | 2 | 0 | 16 |
| `modules/schoolsetup` | 1 | 0 | 0 | 0 | 0 | 1 | 0 | 1 |
| `modules/schoolstructure` | 43 | 0 | 0 | 0 | 1 | 42 | 0 | 43 |
| `modules/settings` | 19 | 0 | 0 | 0 | 0 | 18 | 1 | 19 |
| `modules/studentpresence` | 120 | 63 | 0 | 0 | 0 | 57 | 0 | 120 |
| `modules/timetable` | 208 | 85 | 45 | 21 | 1 | 56 | 0 | 208 |
| `modules/workforce` | 144 | 20 | 0 | 0 | 4 | 120 | 0 | 144 |
| `workflows/openroommove` | 1 | 0 | 1 | 0 | 0 | 0 | 0 | 1 |
| `workflows/parentportal` | 17 | 0 | 17 | 0 | 0 | 0 | 0 | 17 |
| **Total** | **1418** | **502** | **224** | **21** | **19** | **648** | **4** | **1418** |

## Consequences

- #4403 has its result: budget equals floor in every owner, 1418 hits =
  502 transaction and observation wrappers + 224 consumer-port adapters + 21
  #4480 section 5 adapters + 19 scanner limitations + 648 owner-edge
  translations + 4 named exceptions. With the other seven #3423 ratchets at
  zero, #4403 and #3423 are done.
- A new hit has to fit one of the five shapes; the budget still rejects
  growth, so the floor never licenses a raise.
- The owner-edge translations shrink owner by owner where the rollout's rule
  applies: a domain copy that only duplicates a public value field by field
  is deleted ([ADR 0074](0074-delivery-and-export-transfer-speak-their-own-contract.md)
  context).
- A named exception leaves with its blocker: the three Care Plan methods when
  the transaction register admits the rename, `TenantOperations.Schema` when a
  decision grants the `settings/http` to `settings/records` edge.
- The Care Plan lock round trip (decision point 3) is cut by the amendment
  of 2026-10-23.

## Amendment of 2026-10-23

Owner decision of 2026-10-23 on
[#4531](https://github.com/moto-nrw/project-phoenix/issues/4531): option A,
the contract cut. It replaces decision point 3 and the last sentence of
amendment point 5. It adds no rule, alias, allowlist entry, exception or
scanner condition, and no budget changes.

1. **Care Plan locks and ends its own source bookings.** The care lifecycle
   takes the lock in `lockPlanning` and the end in `endSourceBookingRows`
   through Care Plan's own `OfferingBookings`
   (`LockCareOfferingBookings`, `EndCareOfferingBookings`), which the Care
   Plan composition binds. The request child ids still come from Enrollment
   (`CreatedStudentRequestChildIDs`). The lock order inside `lockPlanning`
   (planned roster, activity bookings, source bookings, weekly plans) and the
   ambient tenant transaction stay as they were; both calls reach the same
   `OfferingBookings` methods as before, so the `lock` and `end` observations
   keep their names and now sit at the new boundary.
2. **The end is cut with the lock.** `EndCareExitOfferingLinks` took the same
   round trip; its only production caller passed no source child, so the
   workflow's source-child narrowing did nothing but forward. Leaving it would
   keep half of the round trip for a later decision.
3. **The contract shrinks to what needs Enrollment.**
   `LockCareExitOfferingLinks` and `EndCareExitOfferingLinks` leave
   `ports.CareExitEnrollment`, the `roots/owners` bindings
   (`careExitEnrollment`, `EnrollmentBookingProjection`) and
   `workflows/careexit.Bookings`; the workflow's `BookingStore` loses
   `LockCareOfferingBookings` and `EndCareOfferingBookings`. The snapshot and
   the restore stay in the care exit workflow: they read the applications and
   the submitted choice notes and keep the ledger's Enrollment-shaped snapshot.
4. **The contract test moves with the boundary.**
   `modules/careplan/contracttest/care_exit_storage_cutover_test.go` checks
   the lock and the end on Care Plan's `OfferingBookings`: tenant scope,
   rollback after a failed transaction and the error inside an aborted one,
   with the same assertions as before. Snapshot and restore stay checked on
   the composed projection.
5. **Decision point 2 stands.** `careplan.OfferingBookings` stays in the
   public package. Enrollment's booking projection
   (`roots/owners/enrollment_bookings.go`) and the care exit workflow still
   consume it there, and `LockCareOfferingBookings` remains an observation
   around one store call, so moving the type into the application layer would
   still add a counted hop. The round trip no longer blocks that move; the
   remaining consumers do.

Measurement: `TestModulePassthroughRatchet` requires every entry to equal its
measured count and passes unchanged, so `modules/careplan` stays at 159 and the
total at 1418. The removed methods lay in `roots/owners` and
`workflows/careexit`, which the scan does not cover.
