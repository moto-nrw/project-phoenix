---
status: accepted
---

# Kiosk count revision comes from append-only change events

Decision of 2026-10-28 for
[#5128](https://github.com/moto-nrw/project-phoenix/issues/5128), option A of
the [decision comment](https://github.com/moto-nrw/project-phoenix/issues/5128#issuecomment-6464477975).
It records how the revision of the kiosk student count is stored, approves
two new command-effect diagnostics and one pass-through of the wrapper form
ADR 0075 keeps. It adds no owner, no policy rule and no runtime exception.

## Context

The kiosk ordered its student counter by the start of its requests. A poll
that started after a scan but read before the scan committed replaced the
scan's newer count. The kiosk needs a server revision that belongs to the
count it reads.

A first design kept a counter on the session row (`active.groups`) and
raised it in every visit writer's transaction. To keep revision order equal
to commit order, every writer had to lock the session row until its commit.
That lock entered about ten write paths (scan, checkout, admin transfer,
reattach, session end, care exit, restore, heartbeat, nightly close,
erasure) that lock child, visit and session in different orders today. Two
review rounds reproduced new deadlocks (`40P01`) after each fix. The design
was withdrawn.

## Decision

Student Presence records each change of a session's open visit count as a
row in `active.visit_revision_events` (migration 1.15.705). A statement
trigger on `active.visits` inserts one event per session whose count a
statement changed: an opened, closed, moved or deleted open visit. The
revision of a session is the sum of its events (`changes`) a read sees. The
count and the revision come from one statement and so from one snapshot.

- **No new lock edge.** The trigger only inserts. The event table has no
  foreign key to the session, whose key-share lock would reach the session
  row; it references the school like every tenant table, a lock visit
  inserts already take. Visit writers lock exactly what they locked before.
- **Monotone, also for deletes and moves.** Events are never updated; the
  request role has no `UPDATE`. A later snapshot sees every event an earlier
  one saw, so a revision never falls. A deleted or moved open visit adds an
  event instead of removing one.
- **Older never wins.** A read that started before a write committed does
  not see that write's events and carries a lower revision than the
  writer's own answer, which sees them.
- **Cleanup.** The nightly close (`EndGroupSessions`) replaces the events of
  each ended session by one row with their sum and drops the events of
  deleted sessions, whose ids never return. A reopened session continues
  above its compacted sum. A concurrent writer's events stay outside the
  compaction's snapshot and keep their own row.
- **Tenant boundary.** The table carries `tenant_id` with RLS. The school
  erasure deletes it after `active.visits` (a static copy of the 1.15.681
  erasure functions with the table added); the tenant export leaves it out
  as internal counters.
- **Remaining blur.** Two writers of one session that overlap in time each
  read their own change without the other's. Their answers can carry the
  same revision for different counts, and the kiosk keeps the one it has.
  No answer is older than a committed state, and the next poll after both
  commits carries both changes with a higher revision. The kiosk therefore
  shows such a count at most one poll interval (15 seconds) late.

### Command effects and pass-throughs

The Student Presence application service runs every operation through
`Service.run`, which reports measurements to the injected observer
`observe`. The binding-flow scan of
[ADR 0098](0098-command-effect-binding-flow-and-platform-protocols.md) cannot
follow that function field, so every port of the service records the
diagnostic `unresolved:function:…/studentpresence/internal/application.Service.observe`
(binding digest `e227a8013824f329765590468b7e22d7`). An ADR already in the
base cannot approve new ones. The read adds one operation:

| Port | Binding |
|---|---|
| `studentpresence.Capability.CountOpenVisitsInGroupWithRevision` | `studentpresence.Owner.CountOpenVisitsInGroupWithRevision` |
| `studentpresence.Owner.CountOpenVisitsInGroupWithRevision` | `application.Service.CountOpenVisitsInGroupWithRevision` |

Its diagnostics cite this ADR as `approval_adr`. The observer only reports
measurements; it reaches no other owner's state, takes no lock and widens no
scope. The read is a transaction and observation wrapper of
[ADR 0075](0075-pass-throughs-keep-a-per-owner-floor.md); its evidence in
`backend/architecture/passthrough.json` cites this ADR, and it raises the
Student Presence budget of `TestModulePassthroughRatchet` from 120 to 121
(total 1412 to 1413). No query budget changes: the read replaces the
previous count statement one for one.

After this ADR enters the base it cannot approve another diagnostic.
