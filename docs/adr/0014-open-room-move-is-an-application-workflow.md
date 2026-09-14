---
status: accepted
---

# Open-room move is an application workflow

The open-room specification [#3062](https://github.com/moto-nrw/project-phoenix/issues/3062)
lets staff on a phone ([#3066](https://github.com/moto-nrw/project-phoenix/issues/3066))
and, later, a child at an NFC device ([#3067](https://github.com/moto-nrw/project-phoenix/issues/3067))
record the same **Ortswechsel** into a released room. Add `open-room-move` as
a workflow owner under [#2580](https://github.com/moto-nrw/project-phoenix/issues/2580),
with no runtime data ownership. It does not introduce a new domain.

## Why a workflow

One move writes into three owners in one UnitOfWork:

1. Facilities: the room row is locked and its release is required, so a
   release removed concurrently rejects the move instead of racing it.
2. Timetable & Activities: the room's system activity is resolved and, on first
   use in a school, provisioned. New schools and newly released rooms have no
   bootstrap step that could create it ahead of time; the Schulhof and WC
   infrastructure is likewise provisioned lazily at runtime today.
3. Student Presence: the room's device-less session for that activity is
   found or created, and the children are moved into it with the existing
   locks, source-side rights, capacity check and unique active-visit guard.

Putting the orchestration inside Student Presence would make it write
Timetable & Activities data; putting it inside Facilities would make the
release own presence rules. The separate workflow keeps the 18 domain and 10
platform owners unchanged and transfers no table, following
[ADR 0013](0013-staff-offboarding-is-an-application-workflow.md) and the
session-end and device-scan workflows.

## Independent room stays

An independent room stay ("angebotsunabhängiger Raumaufenthalt") is a visit in
the released room's own session: a device-less `active.groups` row whose
activity is a system activity. The canonical Schulhof keeps its existing
`Schulhof Freispiel` activity, so phone moves and the Schulhof kiosk journey
share one room session; every other released room uses the shared system
activity `Offener Raum`. A session with no activity already means a
spontaneous activity, so it cannot mark an independent stay.

This reuses the existing presence primitives instead of a second presence
engine. The stay is never timetable participation and never supervision. Ending
an activity closes only that activity's session; the daily close ends room
sessions like every other session and leaves attendance open, so the children
become Unterwegs. Removing the release leaves the running room session and its
visits in place; only the independent booking path is rejected afterwards.

## Policy registration

This registration raises the policy epoch from 5 to 6 and uses only
candidate-created packages (`workflows/openroommove`, its `ports`,
`internal/application` and `compose`). Existing-owner import expansions,
workflow-owned writable data and legacy-baseline growth remain forbidden.

The composition binds the room session lookup-or-create and the move to the
retained Student Presence service (`services/active`, `models/active`) through
a narrow consumer-owned interface. Those two `open-room-move.compose.*`
permissions are compatibility bindings that exist only because PR mode cannot
record debt for a package the candidate creates. Convert them to exact debt
once the packages exist at a base SHA, and rebind the ports when Student
Presence exposes the room session and the move publicly. The inbound
`api/active` route and the API root call the public contract and the
composition, which is the target shape.

This decision was authorized in the implementation session on 2026-09-13.
