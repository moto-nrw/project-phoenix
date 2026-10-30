---
status: accepted
---

# Kiosk room count revision extends the change events

Decision of 2026-10-28 for
[#5205](https://github.com/moto-nrw/project-phoenix/issues/5205), option A of
the [decision comment](https://github.com/moto-nrw/project-phoenix/issues/5205):
one revision per room. It extends the append-only change events of
[ADR 0131](0131-kiosk-count-revision-comes-from-append-only-change-events.md)
by a room scope that the read resolves, approves two new command-effect diagnostics and one
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

`active.visit_revision_events` (Student Presence) gets two row kinds next to
the session events of ADR 0131 (migration 1.15.710):

- an **entry row** (`room_id`, `entered_group_id`): a session was in a room.
  The `active.groups` trigger writes it when a session starts in a room,
  moves into it or reopens in it; the migration writes one for every
  existing session;
- a **room row** (`room_id` only): a membership change. The trigger writes
  it for the room a session leaves and for the room of a session that ends.

The revision of a room is the sum of its entry and room rows plus the
session revision of every session it ever held, read in one statement
together with the count. The session revision stays the sum of the rows
with its `active_group_id`; the visit trigger of 1.15.705 is unchanged.

### Invariant

**Every committed change of a room's count raises that room's revision in
every later snapshot, and nothing lowers it.** A room's count is the open
visits of the open sessions in it. A transaction changes it only by

1. changing the open visits of a session in the room (check-in, checkout,
   transfer, deletion of the session with its visits): the visit trigger
   adds a session event, and the session is in the room's set, because the
   transaction that put it into the room wrote its entry row;
2. moving a session into or out of the room: an entry row for the new room,
   a room row for the old one;
3. ending or reopening a session in the room: a room row, or an entry row on
   reopening.

The read, not the writer, decides which sessions belong to the room, and the
set only grows. A visit write racing a room change therefore raises both
rooms: the session counts in the room it left and in the room it entered
(the finding of PR #5232: a checkout racing a room change left the new room
at its revision while its count fell from 2 to 1). Events are never updated,
entry rows of existing sessions are never removed, and the compaction keeps
every sum, so no revision falls. (Amended by #5273: the set only grows
until the nightly close folds a settled session's entries into room rows
with its whole revision, see the addendum.)

Path by path (tests in `modules/studentpresence/compose/room_visit_revision_test.go`):

| Path | Rows it adds | Concurrency test |
|---|---|---|
| Checkout, check-in | session event | racing a room change; two sessions of one room; poll before commit |
| Room change | entry row (new room), room row (old room) | racing checkout, check-in and transfer |
| Group transfer (`TransferOpenVisits`, `ReviseVisit`) | session events of both sessions | racing a room change of the target |
| Session end, reopening | room row; entry row on reopening | checkout racing the end; check-in racing the reopening |
| Session deletion | session events of its cascaded visits; no trigger | sequential (FK serializes visit inserts) |
| Nightly compaction | none; folds and collapses | late checkout of an ended session during the compaction; since #5273 late checkout, check-in and visit revision racing the fold (`room_revision_fold_test.go`) |

- **No new lock edge on the write paths.** The triggers only insert. `room_id` and
  `entered_group_id` have no foreign key, so no key-share lock reaches a room
  or a session, and no trigger fires on deletes. A room change, session end
  or reopening does not wait for a visit writer of that session, and a visit
  writer does not wait for them (`room lifecycle takes no visit lock`); the
  deadlock replays of ADR 0131, including reattach against scan (#5206),
  stay green.
- **One compaction lock.** Two overlapping compactions could each see
  another state of a session deleted in between: one replaces the session's
  entry rows, the other folds its session events without those
  replacements, and the room would lose the session's sum. The compaction
  therefore takes a transaction-scoped advisory lock per school
  (`pg_advisory_xact_lock`) in a statement of its own, so a waiting
  compaction reads a fresh snapshot after its predecessor commits
  (`TestRoomRevisionSurvivesOverlappingCompactionsAndSessionDeletion`). Only
  the compaction takes this lock, and it is the last step of the nightly
  close's transaction: sessions locked `FOR UPDATE` in id order, visits and
  sessions ended, then the lock, one compaction statement and the commit.
  After the lock the holder waits for no row a scan, checkout, transfer or
  reattach holds: event rows are never updated, and rows other writers have
  not committed are invisible to its delete. Two nightly closes of one school
  meet first on the session rows in id order. The lock therefore closes no
  cycle with the visit and session writers, including reattach against scan
  (#5206).
- **Compaction.** The nightly close (`EndGroupSessions`) folds the events of
  a deleted session into one room row per room it entered, with the
  session's sum plus that room's entry sum. A deleted session gets no event
  after its deletion commits, so the fold is exact. It collapses the
  session rows of each ended session, the entry rows of each session and
  room, and the room rows of each room into one row each. It never removes
  an entry row of an existing session: an ended session can still receive a
  late event, which then counts for every room it held. (Amended by #5273:
  the entry rows of a settled session fold as well, see the addendum.)
- **Read cost.** A room keeps every session it ever held until that session
  is deleted; `active.groups` has no retention. The read sums the room's
  rows through `idx_visit_revision_events_room` and one compacted session
  row per held session through the session index, so its cost grows with the sessions a room held, not with
  visits. It runs only for a room-wide kiosk count. Since #5273 the nightly
  close bounds it (addendum below).
- **Tenant boundary.** The trigger function runs as its owner with a fixed
  `search_path` and takes the school from the changed session row; the read
  filters every subquery by the ambient school and RLS applies. The school
  erasure deletes the table by school; no trigger writes while it deletes
  sessions, so the existing plan needs no new edge.
- **Remaining blur.** Overlapping writers of one room can answer the same
  revision with different counts; the next read after both commits is higher
  (ADR 0131). There is no further blur.

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


## Addendum (#5273): the nightly close folds settled sessions

Decision of 2026-10-29. A room kept every session it ever held, so the read
summed one session revision per held session and its cost grew with the
room's history (synthetic history on PostgreSQL 17: 4.7–5.7 ms for 3,650
sessions in one room, 12–13.5 ms for 20 such rooms, 17–19 ms for 36,500
sessions in one room). The nightly
close now folds every **settled** session into its rooms: its entry rows
become one room row per room it entered, carrying that room's entry sum
plus the session revision, exactly like the fold of a deleted session. Its
session rows stay, because the session revision keeps them. A read then sums
the sessions since the last nightly close; on the same synthetic history it
takes 0.15–0.19 ms whatever the history. The read, the triggers and the wire
stay as they are; no migration is needed.

A session is settled when the compaction transaction holds it and every
visit it has:

1. After the advisory lock it locks the ended sessions that still have an
   entry row `FOR UPDATE OF g SKIP LOCKED`. A visit inserted into or moved
   into a session takes a key-share lock on its row, a reopening, room change
   or deletion a stronger one, so each of them either holds the row already
   (the session is skipped) or waits for the nightly close to commit.
2. It locks the visits of those sessions `FOR SHARE SKIP LOCKED` and counts
   them in a later statement. A session settles only if every visit it has
   was locked. Every other visit change (checkout, `ReviseVisit`/`UpdateVisit`
   reopening a closed visit, transfer out) needs a lock on the visit, so
   again it either holds one (the session is skipped) or waits.

So every event of a settled session commits before the compaction
statement's snapshot, which folds it, or after the nightly close commits.
A later event cannot change any room count while the session stays ended,
because a room's count holds only running sessions; a reopening writes a
new entry row, and the session's whole revision counts for the room again.
No revision falls. A skipped session stays in its rooms and settles on a
later night. Overlapping compactions still serialize on the advisory lock,
and a settled session that is deleted later leaves only session rows, which
the fold of deleted sessions removes without a room row: the rooms already
carry its sum.

**Locks.** Both new statements use `SKIP LOCKED` and never wait, and the
compaction statement after them waits for no row a writer holds, as before.
The locks last until the surrounding tenant transaction commits: the
scheduler's session-end job runs `EndDailySessions` inside it and then
completes the timetable instances of the sessions it ended that night. A
writer of an ended session can wait for that commit. The only writer that
also holds timetable rows, the recovery of a completed activity, locks the
attendance of an instance that is already completed, not one the job
completes, so this edge closes no cycle; neither does it with the scan,
checkout, transfer or reattach paths (#5206). Writers of running sessions
meet none of these locks.

**Tests** (`modules/studentpresence/compose/room_revision_fold_test.go`): the
fold keeps every room revision through end, second compaction, late write,
deletion and reopening, and leaves no held session once all ended. A late
checkout, a check-in into the ended session and a closed visit revised open
each race the fold both ways: holding the session or visit during the fold
(skipped, folded the next night, no revision falls) and arriving after the
fold's locks (waits, then changes no room count). Without the visit locks
the first and third race lower a revision a read already saw; without the
session lock the second does.

**Cost of the first night.** The first nightly close after the deploy folds
the whole history at once. With 73,000 ended sessions and 30,000 visits in
one school it took 0.55 s; the next nightly compaction 0.12 s.
