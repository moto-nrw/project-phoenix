---
status: accepted
---

# A presence write locks the child before its visits

Decision of 2026-10-29 for
[#5278](https://github.com/moto-nrw/project-phoenix/issues/5278). It fixes
the order of the child row and the child's visit rows. The order of visit,
session and room row is
[#5280](https://github.com/moto-nrw/project-phoenix/issues/5280); this
decision sits above it and does not change it. It approves one command-effect
entry and adds no owner, no data object, no policy rule and no migration.

## Context

The bulk move (`student_move.go`) locks the children
(`StudentRepo.FindByIDsForUpdate`), then, for a restricted caller, the target
room's session advisory, the open attendance rows and the source and target
sessions, and finally re-reads the children's visits `FOR UPDATE`.

A kiosk room change (`devicescan`, `scanVisit`) closed the child's current
visit first (`EndVisit`, an `UPDATE` of the visit row) and locked the child
only in the check-in guard of the following `CreateVisit`
(`ensureStudentCheckinAllowed`). The move held the child and waited for the
visit; the scan held the visit and waited for the child. PostgreSQL aborted
one of them with `40P01`, reproduced 3 of 3 times on `development`.

## Decision

### Child before visit

A presence write that locks a child takes that lock before it locks or
writes any visit of the child. For one child the order is:

1. School Membership's shared class-writes gate, where the lock takes it
   (a single child lock does, the batch lock of a move or school check-in
   does not),
2. the child row,
3. the room session advisory of a push move's target room,
4. the child's open attendance row,
5. the child's visit, then session and room row as decided in #5280.

The bulk move, the school check-in batch and the check-in guard already keep
this order. The kiosk scan in detailed presence mode now locks the child
through `studentpresence.VisitCommands.LockStudent` before it reads the
child's current visit. It therefore works on the state a concurrent move
committed: it closes the visit the move opened, never a visit the move ended.
`LockStudent` validates nothing else, so a departed child can still be
checked out; the check-in guard keeps rejecting its check-in.

Paths that lock no child keep their order: the web checkout, the visit
transfer and reopen, session ends, the kiosk start and reattach, and the
timetable paths. None of them waits for a child while it holds a visit.

### The class gate rides in the row-lock statement

People Directory's locking student read (`StudentService.FindRecord` with a
lock, used by `FindByIDForUpdate`) joins School Membership's
`StudentClassWriteGate` as a materialized CTE of the row-lock statement. One
statement takes the gate and then the row, in the order the separate gate
statement kept before. While a grade transition holds the gate exclusively,
the waiting read holds no student row
(`TestStudentRowLockWaitsForTheClassGateFirst`). Without a wired gate query
the read keeps the two statements.

This is the existing E3 People student edit protocol of ADR 0067 (hand rows
14–19), reached through a new port: the command-effect entry
`peopledirectory/internal/ports.StudentStore.LockRecord` →
`schoolmembership/compose.StudentClassWriteGate` (effect `lock`,
classification `E3`) cites this ADR as `approval_adr`. After this ADR enters
the base it cannot approve another entry. `Presence.LockStudent` is a leaf
lock of Student Presence and needs no approval.

### Ending a visit

`EndVisit` closes the open visit first and reads it only when the close ended
nothing, to tell a missing visit from an ended one. Its results and errors
are unchanged.

## Consequences

- The scan's statement count stays within its budgets: the child lock costs
  one statement, the check-in guard's lock one statement less and `EndVisit`
  one read less. `api.iot.checkin.check_in` stays at 38, `check_out` at 26,
  `room_change` falls from 44 to 43. Every other locking student read saves
  one statement.
- A scan waits for a running bulk move of the same child and the move waits
  for a running scan; neither aborts. `TestBulkMoveAndScanDoNotDeadlock` and
  `TestScanAndBulkMoveDoNotDeadlock` drive both orders through the
  production scan route and bulk move.
- A scan now holds the child before it holds the visit, so the reattach test
  of #5206 stops the scan at the stay's row, the first lock after the closed
  visit, instead of at the child row.
