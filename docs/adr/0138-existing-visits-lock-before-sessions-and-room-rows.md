---
status: accepted
---

# Existing visits lock before sessions and room rows

Decision of 2026-10-29 for [#5280](https://github.com/moto-nrw/project-phoenix/issues/5280).
Variant A is the decision: existing visit → session → room row, including
bulk moves. Leaving the bulk move in the opposite order would introduce an
admin-transfer/bulk-move cycle. Putting sessions first in scans would add
coordination and queries to the hottest path. Neither alternative is adopted.

## Owner and enforcement

Student Presence owns `active.visits` and `active.groups`. Its
`PrepareVisitWrites` capability and private PostgreSQL adapter prepare the
existing visit cohort, lock it in ascending visit ID order, take room-session
advisories in ascending room ID order, then lock existing sessions in ascending
session ID order. Only afterwards may a caller lock a Facilities room row or
insert a session: that INSERT's room foreign key already takes a row lock.

The scope distinguishes source visits from target sessions. A kiosk start
locks the device's open visits in running/recently ended sessions, but not
unrelated children already in its target room. Timetable absorption also
includes the existing room sessions' visits. Admin updates include the exact
interval, even when closed, so reopening follows the same order. Bulk moves
prepare the selected children's visits before their source/target sessions.

A waiting visit query can outlive a scan's checkout and replacement check-in.
Before any session/advisory lock, a fresh snapshot includes newly committed
intervals only when their IDs follow the already locked IDs. An older unseen
interval is a conflict. After taking the session locks, another fresh snapshot
checks source membership, target-room sessions, room assignments and the visit
cohort. A newly discovered visit is a conflict, never a late visit lock.
Disappeared source candidates are left to the caller's locked-state checks.
Force start freezes the activity/device source IDs before preparation and
rejects any later re-discovery outside that cohort before locking it. An admin
can create a same-activity session in another room without the activity-start
advisory; that new session is never silently added to the transfer.
There is no deadlock retry and no new session-revision lock. Cohort changes
map to the existing timetable stale-operation conflict and kiosk
`general.business_rejection` (409); the workflow exposes its own conflict sentinel.

All preparation runs in the existing tenant transaction, requires that
transaction, filters every query by school ID and retains RLS. Failed commands
roll back the compound write; successful notifications remain after commit.
The session-end workflow uses the public capability through its consumer-owned
port. Timetable uses its existing Student Presence port. No foreign internal
package, workflow SQL, Facilities SQL or new coordinator exception is introduced.
The new capability enforces an owner-local protocol over Student Presence's
existing data, rather than moving a foreign owner's mutation into a caller
(ADRs 0066–0071 and ADR 0067's command-participation distinction).

## Complete lock graph

The graph is a partial order: paths that do not need a class do not take it.

| Path | Lock edges retained or enforced |
|---|---|
| Detailed scan | Shared class gate → child → existing visit → target session → room row; timetable attendance mirror and session heartbeat remain in the same request transaction. No query or lock is added to a scan. A new check-in has no existing visit to lock. |
| Bulk move | Children in ascending ID order → open attendance in student order → selected visits in visit-ID order → room-session advisories → sorted source/target sessions → target room row → new visit and timetable attendance writes. |
| Admin amend/reopen | Sorted child rows → exact visit → room-session advisories → sorted source/target sessions → capacity room row → visit revision and timetable mirror. |
| Kiosk start/reattach | Activity-start advisory → sorted supervising staff → source visits → room-session advisories → sorted existing sessions → session INSERT/update, including room/device FKs → device location and capacity row. The reattach savepoint and locked joinability checks remain. |
| Force start | Activity-start advisory → sorted requested/transferred supervising staff → activity/device source visits → sorted room-session advisories → sorted existing sessions → end old sessions → new session and its FKs → capacity, transfers and supervision changes. |
| Presence end/manual group end/timeout, including scheduler | Existing visits → room-session advisories → sessions → timetable completion/attendance and supervision close. Timetable completion's day lock is not acquired by the scan mirror. |
| Nightly end | Existing visits globally by visit ID → sorted room-session advisories → sessions by ID → close visits/sessions/supervisions → tenant revision-compaction advisory → compaction. |
| Workflow end | Owner preparation → locked session recheck → owner end → timetable finalization → after-commit notification. |
| Timetable start/absorb | Timetable day gate → source visits → room-session advisories → existing sessions → Facilities room row → session INSERT/FK → absorption and timetable rows. |
| Timetable completion/active cancellation | Timetable day gate → source visits → room-session advisories → session → supervision/attendance snapshot and completion. |

Staff-assignment locks precede group/supervision writes; session end does not
acquire staff rows after visits. Room advisories are not Facilities row locks.
The implicit session INSERT FK is not exempt from the room-row suffix. The
scan's class gate and child lock stay above visits as decided in ADR 0136
(#5278, merged first in PR #5346). This decision moves the bulk move's room
advisory below its attendance/visit locks, superseding ADR 0136's placement of
that advisory above attendance. It does not change the child-before-visit rule.

The timetable mirror updates participant attendance without taking the day gate
or a lifecycle session lock. Compaction remains last and waits for no scan-owned
row after its advisory. Event inserts have no new room/session foreign key.
Primitive append-only visit writes still do not take a session revision lock;
room moves and visit revisions keep their existing concurrent revision contract.

## ADR 0131/0133 lifecycle clarification

ADR 0133's claim that lifecycle operations take no visit lock describes the
append-only revision triggers and room-move primitives, not a session end or
transfer that must mutate existing visits. Those lifecycle commands now prepare
visits before sessions. Its nightly description, “sessions locked … visits …
ended”, is superseded by visits first, sessions second, compaction last.

ADR 0131's event model, ADR 0133's room/session revisions, trigger behavior and
compaction arithmetic stay unchanged. The merged #5278 budgets remain 38 for
check-in, 26 for checkout and **43** for room change (already lower than #5280's
historical 44). No budget register is increased or otherwise changed here.

## Proof contract

Real PostgreSQL tests pair production lifecycle commands with the real scan
route, in both actor orders, using held rows and `pg_blocking_pids`. Each
parallel scenario owns a tenant and a private pool. A scan's success response
alone is insufficient: its best-effort heartbeat can swallow a deadlock in a
savepoint. The tests also require that no savepoint rollback occurred. Existing
revision, reattach, rollback and #5278 tests remain part of the affected suite.

## Command-effect evidence

This ADR approves exactly three new unresolved observation boundaries, all
ending at `studentpresence/internal/application.Service.observe`, for the
`PrepareVisitWrites` ports on `studentpresence.Capability`,
`studentpresence.Owner` and `presence.StudentPresence`. These are the existing
injected observation callback, not permission for another unresolved mutation.
The tenant runtime's `CurrentTenantTransaction` binding is a context read. The
six exposed Timetable → Student Presence preparations are owner-local locking
leaf capabilities, classified `lock`, not queries. No ratchet budget, policy
edge, composition allowance or data ownership changes.

The hermetic setup scanner recognizes `SetupClosableTestDB`, the documented
private-pool constructor required by lock tests. A scanner regression fixture
pins this recognition; no per-file exclusion or assertion is weakened.

Admin interval revisions also synchronize school attendance. They now take
the affected child rows before their visit, serializing that attendance write
with bulk move and care exit before the two paths' attendance/visit suffixes
can diverge. Historical edits retain their eligibility semantics: the lock does
not apply the check-in guard. The stored child is rechecked after waiting.
Bulk move re-reads visits after preparation before selecting source-session
guards, so a lifecycle transfer during its wait cannot add a stale session lock.
