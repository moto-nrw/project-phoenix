---
status: accepted
---

# Kiosk count revision uses Student Presence's observer

Decision of 2026-10-28 for
[#5128](https://github.com/moto-nrw/project-phoenix/issues/5128). It approves
eight new command-effect entries, one pass-through of the wrapper form
ADR 0075 keeps and one more statement in each kiosk scan's query budget. It
adds no owner, no policy rule and no runtime exception.

## Context

The kiosk orders its student counter by a server revision instead of the
start order of its requests (#5128). Student Presence owns `active.groups`
and `active.visits`, so it stores the revision (`active.groups.visit_revision`)
and reads count and revision of a session in one statement. The device scan
consumes that read through its own ports.

The Student Presence application service runs every operation through
`Service.run`, which reports duration, counts and error to the injected
observer `observe` (`compose.Dependencies.Observe`). The binding-flow scan of
[ADR 0098](0098-command-effect-binding-flow-and-platform-protocols.md) cannot
follow that function field to a terminal, so every port of the service
records the diagnostic
`unresolved:function:…/studentpresence/internal/application.Service.observe`
(binding digest `e227a8013824f329765590468b7e22d7`). An ADR already in the
base cannot approve new ones.

The revision serializes the writers of a session through its row lock. To
keep that lock from closing a deadlock cycle, every change of existing
visits locks its sessions in ascending id order before any visit row, and a
scan locks all sessions its checkout and check-in can change up front:
the session of the child's open visit and every open session of the scanned
room. Without that, two scans moving children in opposite directions lock
their sources first and then wait for each other's.

The read and the scan's lock add two operations of the same service:

| Port | Binding |
|---|---|
| `studentpresence.Capability.CountOpenVisitsInGroupWithRevision` | `studentpresence.Owner.CountOpenVisitsInGroupWithRevision` |
| `studentpresence.Owner.CountOpenVisitsInGroupWithRevision` | `application.Service.CountOpenVisitsInGroupWithRevision` |
| `studentpresence.Capability.LockVisitSessions` | `studentpresence.Owner.LockVisitSessions` |
| `studentpresence.Owner.LockVisitSessions` | `application.Service.LockVisitSessions` |

The count read is a transaction and observation wrapper of [ADR 0075](0075-pass-throughs-keep-a-per-owner-floor.md); its exact evidence in `backend/architecture/passthrough.json` cites this ADR, and it raises the Student Presence budget of `TestModulePassthroughRatchet` from 120 to 121 (total 1412 to 1413).

## Decision

Both operations run through `Service.run` like the module's other
operations, and their diagnostics cite this ADR as `approval_adr`. The observer only reports
measurements; it reaches no other owner's state, takes no lock and widens no
scope. The diagnostics carry the same binding digest as the entries ADR 0098
approved, so the observer's assignment is unchanged.

The scan's lock is one statement. It raises the query budgets
`api.iot.checkin.check_in`, `check_out` and `room_change` in
`backend/test/query_budgets.go` by one each (39, 27, 45); no existing
statement of the scan can carry the lock, because it must precede the
checkout.

Leaving the observer out would remove the operations from the module's runtime
metrics and break the service's single operation wrapper. Resolving the
observer for the whole module is a scanner change outside #5128.

After this ADR enters the base it cannot approve another diagnostic. A
changed observer assignment changes the digest and needs fresh approval.

## Review correction: student before sessions

A kiosk scan must take the student lock before its session locks, matching
`CreateVisit`, school checkout batches and staff moves. An additional locked
student lookup would add two statements and exceed the unchanged scan budgets.
Instead, People Directory's `StudentRecords.FindByPersonIDForUpdate` replaces
the scan's initial unlocked lookup. Its adapter evaluates School Membership's
existing shared class-writes gate in a materialized CTE before locking the
owned student profile, all in one statement. The ordinary read stays unlocked.
The order is class(shared), student, ascending sessions, visits. The request
transaction, tenant scope and error propagation do not change.

This is the same gate-before-student prefix that `StudentService.FindRecord`
already uses for `UPDATE`, not a new cross-owner coordinator protocol. The
existing `StudentClassWriteGateQuery` binding builds the gate SQL; it writes
no class or membership. The exact `StudentStore.LockRecordByPersonID` effect
is recorded as `lock`/`debt`, like the same constructor through enrollment
creation. This records the inherited boundary and does not approve a wider
runtime exception. Three observer diagnostics of the new repository method
(the concrete `StudentRecords` binding and its two promoted root bindings)
also cite this ADR; they retain the existing People Directory observer digest.

`TestDeviceCheckinLocksStudentBeforeSessions` exercises the production HTTP
scan against `CreateVisit`, school batch checkout and staff moves with an
observed PostgreSQL lock wait. The original scan deadlocks in all three
cases (`40P01`); the locked initial lookup waits before taking any session.
The scan query-budget tests retain their original limits (39, 27, 45).
`TestUpdateVisitLocksBothSessionsBeforeTarget` exercises opposite public
`UpdateVisit` calls and verifies the committed destinations. Source and
destination session locks now follow ascending ID order in the target guard.

The exact pass-through register records the new root translation
`api/DeviceScanPeople.GetStudentByPersonIDForUpdate`, which maps the locked
record to `ScanStudent`, like its existing ID-based counterpart. The retained
`StudentRecords` boundary validates the person ID and requires the caller's
transaction before invoking its private observed read and translating the
result. The existing device-scan binding selects the unlocked or locked read;
read-only callers keep the unlocked path. No forwarding, composition LOC or
query budget grows for this correction.
