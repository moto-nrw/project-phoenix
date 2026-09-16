---
status: accepted
---

# Parent portal is an application workflow

`services/parent` serves the guardian portal
([#3227](https://github.com/moto-nrw/project-phoenix/issues/3227)). Add
`parent-portal` as a workflow owner under
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580), with no
runtime data ownership, and move the package's flows into
`workflows/parentportal`. It does not introduce a new domain.

## Why a workflow

The package is classified as the Care Plan application, but most of its flows
coordinate other owners on behalf of one guardian and one child:

1. Care Plan: the guardian-child link and its relationship permissions, the
   care interval, and the absence and pickup requests.
2. Enrollment: care periods, booked care offerings, offering-change requests
   and course requests.
3. Timetable & Activities: the weekly arrival and pickup plan and its change
   requests.
4. Communication: parent messages, announcements and the request-sharing
   ledger.
5. People Directory, Settings and Student Presence: student rows and the
   request-reason policy, the school settings, and the attendance and
   status-day reads.

A package under Care Plan cannot hold these flows. The existing
`care-plan/adapter` and `care-plan/adapter-test` role points are already
used by `modules/careplan/legacy` and `modules/careplan/requestfeed/http`. In
PR mode, every permission for the relocated code would widen those packages
and would also admit their recorded debt. A separate workflow owner keeps the
18 domain and 10 platform owners unchanged and transfers no table, following
[ADR 0013](0013-staff-offboarding-is-an-application-workflow.md) and
[ADR 0018](0018-open-room-move-is-an-application-workflow.md).

## Package layout

- `workflows/parentportal/care` holds the child's today status, the weekly
  care plan and its change requests, the booked care offerings and their
  change requests, and course requests. It also owns the guardian-child
  resolution (`ResolvePermittedChild`) that every other parent flow uses. Its
  writes to and reads from the request-sharing ledger go through its
  consumer-owned `RequestSharer` port.
- `workflows/parentportal/messaging` holds announcements and their
  attachments, request sharing, parent messaging and the self-service chat
  pills. It resolves the child through its own `ChildResolver` port. It
  returns the guardian-child and note sentinels declared in `care`
  (`parent-portal.application.care`), so `errors.Is` still matches a single
  value.

Both packages are `parent-portal`/`application`, with
`workflow-integration-test` in both test scopes. `services/parent` keeps the
public `parent.Service` contract. It binds both ports and delegates the
relocated methods, so no HTTP path, status code, error string, authorization
check or tenant scoping changes.

## Policy registration

This registration raises the policy epoch from 11 to 12 and uses only
candidate-created packages. Existing-owner import expansions, workflow-owned
writable data and legacy-baseline growth remain forbidden.

The relocated code still imports the retained models, services and shared
helpers it used inside `services/parent`. Every
`parent-portal.application.*` and `parent-portal.integration-test.*`
permission is a compatibility binding. It exists only because PR mode cannot
record debt for a package the candidate creates. Convert each one to exact
debt once the packages exist at a base SHA, and rebind each port or import to
its owner's public capability as that capability appears.
`care-plan.application.parent-portal` is the transitional consumer edge from
`services/parent`.

## Follow-up

[#3228](https://github.com/moto-nrw/project-phoenix/issues/3228) moves the
remaining write, master-data and guardian flows into the workflow and deletes
`services/parent` together with that consumer edge.
[#3229](https://github.com/moto-nrw/project-phoenix/issues/3229) moves
`api/parent` onto the workflow.

This decision was authorized in the implementation session on 2026-09-16.
