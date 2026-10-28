---
status: accepted
---

# Kiosk room count revision extends the change events

Decision of 2026-10-28 for
[#5205](https://github.com/moto-nrw/project-phoenix/issues/5205), option A of
the [decision comment](https://github.com/moto-nrw/project-phoenix/issues/5205):
one revision per room. It extends the append-only change events of
[ADR 0131](0131-kiosk-count-revision-comes-from-append-only-change-events.md)
by a room scope, approves two new command-effect diagnostics and one
pass-through of the wrapper form ADR 0075 keeps. It adds no owner, no data
object, no policy rule and no runtime exception.

## Context

A kiosk without its own session in the scanned room shows the room-wide
count: the open visits of every running session in that room. ADR 0131 gave
only a session count a revision, so the room-wide count still followed the
start order of requests, and an older answer could replace a newer one
(#5128). Session revisions of different sessions do not compare, and their
sum falls when a session leaves the room. A lock on a room or session row
would bring back the deadlocks ADR 0131 withdrew.

## Decision

`active.visit_revision_events` (Student Presence) gets a nullable `room_id`
(migration 1.15.710). The revision of a room is the sum of the events with its
`room_id` that a read sees; count and revision come from one statement. The
session revision stays the sum of the events with its `active_group_id`.

- **Every change of a room count records a room event in its own
  transaction.** The visit trigger of ADR 0131 stamps each session event with
  the room the session has when the statement runs, also when the session has
  ended, so a check-in racing a session end or reopening still raises the
  room. A row trigger on `active.groups` records an event without a session
  when a session changes its room (old and new room), ends or reopens, and
  before an open session with open visits is deleted (the cascaded visit
  deletion can no longer name the room). Inserting a session changes no
  count and records nothing.
- **Monotone.** Events are never updated. The nightly close
  (`EndGroupSessions`) replaces the events of each ended session and room by
  one row with their sum and the room events of each room by one row. A
  deleted session's events keep their room as room events, so neither a
  session revision nor a room revision falls.
- **Older never wins.** A read that started before a write committed carries
  a lower revision than the writer's own answer, as in ADR 0131.
- **No new lock edge.** Both triggers only insert events and read
  `active.groups` or `active.visits` without a lock; `room_id` has no foreign
  key. A room change, session end or reopening does not wait for a visit
  writer of that session, and a visit writer does not wait for them.
- **Tenant boundary.** The trigger functions run as their owner with a fixed
  `search_path` and take the school from the changed rows; the read filters
  by the ambient school and RLS applies. The school erasure deletes
  `active.visits` before `active.groups` (cascading foreign-key child first),
  so its session deletion finds no open visit and records no event after the
  event table is gone; the existing plan needs no new edge.
- **Remaining blur.** Overlapping writers of one room can answer the same
  revision with different counts; the next read after both commits is higher
  (ADR 0131). Beyond that, a visit write whose statement runs before its
  session's room change commits, and which commits after it, raises the room
  it saw. The new room then names its count before and after that write with
  one revision until its next change. Room changes of a running session are
  administrative edits; the blur needs both to overlap within one
  transaction's lifetime, and no revision falls.

### Wire

`POST /api/iot/checkin` (detailed mode) returns `active_students_room_id` and
`active_students_room_revision` together with a room-wide `active_students`.
They are new fields, not `active_students_revision`: a kiosk built on the
session contract (PyrePortal#489) cannot read a room revision as a session
revision, and an older kiosk ignores them. A session count keeps
`active_students_session_id` and `active_students_revision` and carries no
room fields.

### Command effects and pass-throughs

The read adds one operation to the Student Presence application service.
As in ADR 0131, the binding-flow scan cannot follow `Service.observe`, so its
ports record the diagnostic
`unresolved:function:…/studentpresence/internal/application.Service.observe`
(binding digest `e227a8013824f329765590468b7e22d7`):

| Port | Binding |
|---|---|
| `studentpresence.Capability.CountOpenVisitsInRoomWithRevision` | `studentpresence.Owner.CountOpenVisitsInRoomWithRevision` |
| `studentpresence.Owner.CountOpenVisitsInRoomWithRevision` | `application.Service.CountOpenVisitsInRoomWithRevision` |

Their diagnostics cite this ADR as `approval_adr`. The observer only reports
measurements; it reaches no other owner's state, takes no lock and widens no
scope. The read is a transaction and observation wrapper of
[ADR 0075](0075-pass-throughs-keep-a-per-owner-floor.md); its evidence in
`backend/architecture/passthrough.json` cites this ADR, and it raises the
Student Presence budget of `TestModulePassthroughRatchet` from 121 to 122
(total 1413 to 1414). No query budget changes: the room-wide kiosk count
replaces its count statement one for one.

After this ADR enters the base it cannot approve another diagnostic.
