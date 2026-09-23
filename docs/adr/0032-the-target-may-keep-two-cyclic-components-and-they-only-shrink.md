---
status: accepted
---

# The target may keep two cyclic components, and they only shrink

[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580) names the
cycle among the backend package families as a problem and then only requires an
acyclic ticket plan.
[#3417](https://github.com/moto-nrw/project-phoenix/issues/3417) added the
`cycles` report and left two decisions to a human: what happens to the platform
triangle, and which cycle degree the end state permits. We decide both here. A
cyclic component is a strongly connected component of the owner graph with at
least two owners, as `backend/architecture/README.md` defines it.

## Measurement

`scripts/backend-architecture.sh cycles --graph target` on `cf45b8dcf6`, policy
epoch 18: 54 nodes, 231 edges, 2 cyclic components. Every edge is permitted by
target rules, so none of this is ratchet debt and none of it disappears when
the baseline reaches zero.

- **Component 1, 17 members, 47 edges:** `calendar-view`, `care-plan`,
  `delivery-platform`, `document-rendering`, `enrollment`, `facilities`,
  `file-storage`, `open-room-move`, `operator-dashboard-view`,
  `organization-tenancy`, `people-directory`, `school-structure`,
  `settings-platform`, `student-directory-view`, `student-presence`,
  `timetable-activities`, `timetable-legacy-view`.
- **Component 2, 5 members, 8 edges:** `communication`,
  `parent-announcement-audience`, `parent-message-inbox`,
  `staff-announcement-audience`, `staff-message-inbox`.

The numbers in #3417 (52 nodes, 315 edges, one component of 21) were taken at
epoch 15 and are superseded. Since then `identity-access`, `school-calendar` and
`workforce` have left, the Communication members have split off as component 2,
and `care-plan`, `document-rendering`, `file-storage` and
`student-directory-view` have joined component 1.

## The platform triangle

`delivery-platform → settings-platform → organization-tenancy →
delivery-platform` is dissolved at its narrowest edge,
`organization-tenancy → delivery-platform`.
`modules/organizationtenancy/internal/application/provisioning_dashboard.go`
takes a usage window and two portal identifiers from Delivery to build the
operator PWA summary. That becomes a consumer-owned port in Organisation &
Tenancy. Both permitting rules, `organization-tenancy.to.delivery-domain` and
`delivery-cutover.038`, are deleted. Neither is converted to exact debt and no
baseline entry is added. It is the only edge of the triangle on which a domain
depends back on a platform for presentation constants, and it is the cheapest
to cut.

Between the two platforms, `delivery-platform → settings-platform` is the
permanent direction: Delivery reads tenant notification settings
(`delivery.application.settings`, `delivery.application.settings-domain`). The
reverse edge exists only through `settings-platform.http.delivery-public`, by
which the operator school settings routes broadcast a settings change through
the Delivery producer. Its own description marks it as a compatibility
permission of
[#2736](https://github.com/moto-nrw/project-phoenix/issues/2736), to be
converted to exact debt. It stays temporary and must not be rewritten into a
permanent rule. When #2736 removes it, the two platforms no longer form a
cycle on their own. `settings-platform.http.organization-public` stays;
without the deleted edge it no longer closes a triangle.

Dissolving the edge does not shrink component 1. `organization-tenancy` stays
reachable through `people-directory` and `operator-dashboard-view`
([ADR 0033](0033-operator-dashboard-view-is-a-projection-owner.md)). The cut
is made because the edge is wrong, not to move the number.

## The permitted cycle degree

Component 2 is structural. `communication` composes its four audience and
inbox projections, and each projection reads the Communication domain. That is
the projection pattern of this policy, and it is not a defect. Component 1
contains real domain cycles, for example `care-plan ↔ enrollment`. Removing
them is modelling work beyond the migration, so "zero components" is not an
honest exit criterion for #2580, and "no cycle among domain modules" would be
red today for the same reason.

The end state permits **at most the two components listed above, and they only
shrink**:

- the target graph has no cyclic component other than these two;
- no component gains a member that is not in its list above;
- a component may lose members, split into smaller components whose members
  all come from one list, or disappear.

`scripts/backend-architecture.sh cycles --graph target --json` decides this
without judgement: every reported component's member set must be a subset of
one of the two lists. A change that needs a new member or a new component needs
a new ADR that supersedes this one.

## Follow-up under #3417

- Cut the `organization-tenancy → delivery-platform` edge and delete its two
  rules.
- Rewrite the descriptions of `delivery.application.settings`,
  `delivery.application.settings-domain` and
  `settings-platform.http.organization-public` so they state this decision and
  point here instead of describing an import. Leave
  `settings-platform.http.delivery-public` marked as the #2736 compatibility
  permission it is.
- Add the criterion above to the Definition of Done of #2580, in its German
  wording.
- Make the subset check a failing mode of `cycles`, or a test over its JSON
  report, with these two lists as the fixture. Until then `cycles` only
  reports.
