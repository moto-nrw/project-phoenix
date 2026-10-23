---
status: accepted
---

# Owner granularity follows invariants, not source size

Decision of 2026-10-23 for [#4547](https://github.com/moto-nrw/project-phoenix/issues/4547).
Keep the examined projection and domain owners. Source size triggers inspection,
not a merger or split. This adds selection criteria to
[ADR 0066](0066-principle-every-module-table-and-cross-owner-read-has-one-owner.md),
not a policy rule, budget, write edge or implementation change. There are no
`proposed` mergers or splits awaiting approval in this decision. Any later
recommendation to change ownership must remain `proposed` until the product
owner approves it, before an implementation changes policy or code.

## Evidence and limits

Issue #4547 had no comments when read. The inspected tree is fetched
`origin/development` at `1d1c6985ade16653d8f1fa208a29e358663ab303`.
[ADR 0080's inventory](0080-owner-boundary-costs-are-measured-before-consolidation.md#small-projections-cost-inventory-for-4547)
is the cost basis for all twelve small projections. Its C/K/A/R columns and
three seam allocations describe selected constructors, contracts/ports,
adapters and root glue at its recorded SHA. Refer to those figures rather
than counting them again. W is a lower bound, not removable LOC; S counts
reference lines, not binding instances. Zero selected port LOC does not mean
there is no consumer port.

Since that measurement, #4540 has enforced the existing prohibition of
**domain-to-projection imports**, including test scopes (ADR 0071's addendum).
Timetable owns its Presence values; Communication's announcement-audience
binding now lives in `api/root_serve_owners.go`, not its composition. That
reinforces the boundary being assessed, without invalidating ADR 0080's
historical allocation. No old allocation is claimed to be today's exact glue.

The issue's workflow count is stale: the inspected policy has 19 projections,
18 domains, 13 workflows and 10 platforms. ADR 0080 explains the added
`staff-person-merge` workflow. Folder counts are not owner counts. The following
supplement fills the policy-entry counts absent from ADR 0080, without
remeasuring its wiring inventory.

### Policy cost per small projection

Paths abbreviate `backend/modules/`. P counts exact `packages` entries; G counts
`read_projections` entries at those packages. Rprod/Rtest count rules with an
**explicit source_owner or target_owner** equal to this owner, by scope. A rule
that has both scopes would count in both columns. Generic owner-kind,
same-owner, external-class and root rules are shared costs and are excluded;
a zero does not mean the package is unreachable. These are existing entries,
not a forecast of entries deleted by a merger.

| Directory | P | G | Rprod | Rtest |
|---|---:|---:|---:|---:|
| `timetableenrollmentprovenance` | 2 | 1 | 0 | 0 |
| `guardianlinkview` | 2 | 1 | 0 | 0 |
| `timetableblockdisplay` | 2 | 1 | 1 | 0 |
| `parentrequestfeedprojection` | 2 | 1 | 0 | 1 |
| `staffmessageinbox` | 3 | 1 | 1 | 0 |
| `timetableprojection` | 3 | 1 | 0 | 1 |
| `studentdirectoryview` | 2 | 1 | 0 | 0 |
| `staffannouncementaudience` | 3 | 1 | 1 | 0 |
| `staffdirectoryview` | 1 | 0 | 1 | 1 |
| `operatordashboard` | 3 | 1 | 1 | 1 |
| `emergencysnapshot` | 4 | 0 | 3 | 0 |
| `parentmessageinbox` | 3 | 1 | 1 | 0 |

## Small projections: keep each owner

Every row is an **accepted belassen** decision. Keeping them saves no current
policy entry, port or binding. The alternative's possible savings below are
conditional: grouping directories can remove owner registrations or shared
construction, but cannot erase independent read semantics, tenant checks or
consumer vocabulary. Only retiring owner declarations is certain when grouping
owners while retaining each query package; package/rule deletions need a concrete
replacement, and distinct grants and consumer ports still serve their reads. We do not claim a removable-LOC total without a concrete
approved replacement. Exact grants stay separate even if a future merger
places the queries under one projection owner; no union grant follows from it.

| Candidate | Read boundary and reason to keep | Rejected alternative and cost trade-off |
|---|---|---|
| `timetableenrollmentprovenance` | `internal/adapters/postgres/query.go`, `EligibleEnrollmentIDs`, correlates enrollment approval with Timetable's sourced enrollments, in the caller's transaction and with a row lock. It is not block display. | Grouping with the other Timetable reads might share a constructor, but retains the Enrollment-table grant and Timetable's provenance port. ADR 0080's C/A and root reference are not interchangeable display glue. |
| `timetableblockdisplay` | `internal/adapters/postgres/projection.go`, `List`, joins Timetable instances/templates to Facilities rooms and School Structure groups. Display metadata is independent of provenance and lifecycle counts. | Combining directories saves no foreign-table boundary. Moving SQL into Timetable would instead transfer a cross-owner grant. ADR 0080's measured Block Display seam includes a required ambient-transaction guard, which must survive. |
| `timetableprojection` (`timetable-legacy-view`) | `internal/adapters/postgres/projection.go` serves course capacity locks/peak occupancy, source-enrollment counts and deletion counts. `roots/owners/offering_change_planning.go`, `enrollment_deletion_composition.go` and `student_deletion_projection.go` bind different consumers. The legacy name does not make these reads obsolete. | A common Timetable projection owner would still need these workflow ports and the Enrollment-source grant. Removing owner records alone loses a useful distinction between display reads and lock-bearing lifecycle/capacity reads. Revisit capability placement separately, not by merging on the folder prefix. |
| `studentdirectoryview` | `internal/adapters/postgres/projection.go` joins profile, live school membership and care profile; public child identity remains the profile ID (ADR 0066 point 8). Its row source serves several domains and roots. | Joining it with Staff Directory cannot remove its SQL grant, live-membership predicate or consumers' student ports. ADR 0080's small C and large S describe reuse, not duplicated staff construction. |
| `staffdirectoryview` | `staffdirectoryview.go` joins School Membership staff with person/account ports for contactability, permission, role and birthday reads. It owns no SQL grant. `roots/owners/staff_membership_adapters.go` translates those values and errors. | A combined directory would mix a SQL row-source contract with a port-composed contact model. The person/account ports and membership reads remain; ADR 0080's K/R allocation is not eliminated by renaming the owner. No shared person DTO should replace either consumer vocabulary. |
| `staffmessageinbox` | `internal/adapters/postgres/projection.go`, `ListInbox`, joins Communication's staff threads, participants and receipts with People Directory names. Its reader is an account in one school. | Combining it with Parent Inbox or Announcement Audience could group constructors but cannot remove the person join, staff receipt rules, contract or `roots/owners/staff_inbox_binding.go`. ADR 0080's Staff Inbox seam already uses the shared ambient database runtime. |
| `parentmessageinbox` | `internal/adapters/postgres/projection.go` and `predicates.go` distinguish guardian/staff counterpart activity, per-person cursors, team handling and child-specific guardian links across schools. | A common inbox abstraction risks treating school membership as child access or equating staff and guardian unread rules. Its contract and `roots/owners/parent_inbox_binding.go` would still translate this vocabulary; ADR 0080's K/R are not fungible with Staff Inbox. |
| `staffannouncementaudience` | `internal/adapters/postgres/projection.go`, `unreadFrom`/`unreadWhere`, combines announcements, effective roles, school/organisation targeting and view state. It is broadcast audience selection, not a conversation inbox. | Sharing an owner with inboxes removes neither this grant nor Communication's audience port. Its local `Map` and contract (ADR 0080 K/A) serve this result shape. The root binding introduced by #4540 must not move back into Communication. |
| `guardianlinkview` → People Directory | `internal/adapters/postgres/projection.go` combines People Directory relationships, Identity permissions and Care Plan pickup facts. `roots/owners/people_directory_composition.go` and `communication_projection_roots.go` reuse it. Pickup and portal permission remain different facts. | Absorbing it would remove a projection owner/constructor but retain a cross-owner read and tenant/admin selectors. ADR 0080's Guardian Link seam includes those selectors. Moving that SQL grant to the lowest domain hides an Identity/Care Plan boundary rather than removing it. |
| `parentrequestfeedprojection` → Care Plan request feed | `internal/adapters/postgres/query.go`, `ResolveAccess` and `ListNewParentRequests`, combines active school/account rights and pending queues from Care Plan and Enrollment. The personal RSS feed serves authorised staff, despite the `parent` prefix. | Absorbing it saves a projection constructor and may colocate `api/parent_request_feed.go`'s adapter, but still needs the AccessResolver/List ports and foreign grant. Permission-filtered queue union and subscription/token writes are different responsibilities. Care Plan must not gain Identity/Enrollment SQL merely to shorten this seam. |
| `operatordashboard` | `internal/adapters/postgres/projection.go` combines installation-level school/organisation, account-role and PWA usage facts behind operator access (ADR 0033). | Not an inbox, directory or tenant read. Keeping its separate audience and administrative scope outweighs potential constructor grouping; ADR 0080's contract, mapper and root allocation remain scope-bearing work. |
| `emergencysnapshot` | `emergencysnapshot.go` and `ports.go` assemble present children, people, locations and health facts through ports, then render the Notfallliste. No SQL grant. | Absorbing it into a source domain would give that domain responsibility for another owner's facts and the emergency document. ADR 0080's K/R include renderer conversion and missing-dependency guards, not only removable row copying. |

### Port-composed read models and ADR 0066 point 6

The statement-join criterion applies to a new direct foreign-SQL projection
grant. Staff Directory and Emergency Snapshot are already named projection
owners but compose capabilities through ports rather than joining SQL.
**Retain these two named port-composed read owners**, as recorded in the
architecture README's #4082 and Emergency Snapshot sections and ADR 0080.
This is a narrow clarification of ADR 0066 point 6, not permission for every
consumer-port read to become an owner, nor for either owner to acquire SQL.
Request Review View is another existing port-composed model, outside this
issue's small-projection list; this decision does not change it.

## Large domains: keep the owner, use internal capabilities

The issue's directory totals are historical. These current physical LOC use
ADR 0080's filter, but **policy roles**, not directory names, for the columns.
They include subpackages such as `careplan/requestfeed` and
`workforce/internal/planning`. Untagged fixture helpers remain included.
These are not counts of business rules or a measure of team throughput.

| Directory | Total | application | http | postgres + adapter | domain |
|---|---:|---:|---:|---:|---:|
| `timetable` | 68,326 | 34,887 | 13,977 | 6,437 | 2,271 |
| `careplan` | 63,231 | 30,959 | 9,054 | 7,223 | 2,040 |
| `workforce` | 53,552 | 30,606 | 7,873 | 5,953 | 2,705 |
| `identityaccess` | 51,446 | 22,938 | 9,141 | 6,800 | 3,839 |
| `enrollment` | 48,427 | 25,267 | 11,195 | 4,593 | 0 |

The columns omit public/contract/port/compose/test-support roles, so they do not
sum to Total. In particular, Workforce's application role includes planning;
the issue's directory-labelled application figure excludes that package.

1. **Timetable & Activities: belassen (accepted).** Templates/recurrence,
   materialised instances, participants, staff assignments, planning tracks
   and class-arrival times are recognisable capabilities with dedicated tables
   (`activities.groups`, `schedule.recurrence_rules`,
   `schedule.activity_instances`, `schedule.instance_students`,
   `schedule.instance_staff`, `schedule.planning_tracks`,
   `education.class_arrival_times`). They are not independent lifecycles:
   `internal/application/template_series_roster.go`,
   `reconcileSeriesPredecessorRoster`, updates bounded predecessor rosters and
   already planned instances under the same recurrence lock and tenant
   transaction. Splitting templates from instances/rosters would turn this
   invariant into a cross-owner workflow and add contracts without evidence
   of independent change cadence. Class-arrival planning alone has identifiable
   tables, but no demonstrated team or lifecycle benefit from a new owner.
   Keep internal seams; do not create a second recurrence engine.
2. **Care Plan: belassen (accepted), including requestfeed.** Offerings/bookings,
   weekly arrival/pickup plans, requests, care exits and documents have table
   clusters, but a care exit freezes and ends bookings and weekly plans as
   one care invariant. `internal/application/care_exit_cleanup.go`,
   `lockPlanning`, `endSourceBookings` and `endSourceBookingRows`, records this
   coordination; ADR 0075's latest amendment already cuts the old booking
   lock/end round trip back to Care Plan's own commands. Splitting bookings
   from care lifecycle would introduce another workflow around that invariant.
   `requestfeed` is an existing encapsulated capability, not a separate owner:
   its own durable object is `users.parent_request_rss_feeds` (Care Plan),
   while request contents/access arrive through ports bound to the separate
   projection. A feed-only owner is technically plausible and less coupled,
   but would add an owner, composition, contracts and a table transfer for a
   subscription/delivery surface with no demonstrated independent domain
   policy or operational need. Keep its existing internal boundary, without
   moving the read projection into it.
3. **Workforce: belassen (accepted), including planning.** Shift types, concrete
   shifts, series and exceptions form an identifiable cluster
   (`schedule.shift_types`, `schedule.staff_shifts`,
   `schedule.staff_shift_series`, `schedule.staff_shift_series_exceptions`).
   `internal/planning` is nevertheless an application-role package of
   Workforce, not a new owner or table namespace. Its `ports.go` consumes
   native Workforce records and work-target facts. The overview combines
   planned minutes with work models and Sonderarbeitszeit
   (`staff_schedule_overview.go`, `buildWeeklySummaries`); series and
   single-shift writes share the staff lock (`staff_shift_series_service.go`,
   `lockShiftWrites`), and moves preserve series deviation identity
   (`staff_shift_move.go`, `MoveShift`). A planning owner can be designed, but
   the current separation already hides its implementation without an extra
   inter-owner record contract. Table separability is necessary, not enough.
   A split would need an explicit contract for shifts, absence/target facts,
   employment lifecycle and shared locks before promising a simpler boundary.
4. **Identity & Access: belassen (accepted).** Account sessions/credentials,
   invitations, caller context and operator authentication remain security
   capabilities within the same owner. `internal/domain/account_session.go`
   and `operator_authentication.go` and their application callers already
   separate their contracts. The table inventory distinguishes account and
   operator credentials but does not prove that splitting them improves
   session revocation or portal isolation. ADR 0080's same-owner value-copy
   cleanup is the narrower follow-up; no auth/table transfer is justified by
   the total LOC. This is not an exhaustive security audit.
5. **Enrollment: belassen (accepted).** Phase/form configuration, applications,
   request children, selected offerings, review/read state and change requests
   share the application-to-approved-child lifecycle. `internal/application`
   holds the intake/review services; the offering booking tables belong to
   Care Plan, not Enrollment, despite their `enrollment.*` schema names.
   ADR 0023's assignment already separates these meanings. A new owner based
   on schema or the missing `internal/domain` directory would undo that
   distinction without a demonstrated independent invariant.

### Thin domain layer: inspect rules, not folder ratios

The issue's claim that nearly all *business logic* lives in application does
not follow from LOC: application includes transaction wrappers, coordination,
port adaptation and value translation. Domain contains rules already, for
example Workforce's `ValidateStaffTargetOverrideFields`,
`RejectTargetOverrideOverlap`, `RejectClosedTargetOverrideMonths` and
`ExpandTargetOverrides` in `internal/domain/target_override.go`.
Timetable's application-local `segmentIsReconcilablePredecessor` is already
pure and tested without ports in `template_series_roster_pure_test.go`.
Enrollment's absent domain-role package is not absent business policy.

Keep the current layers in this issue. On a behaviour change, consider moving
a deterministic rule to an owner-local domain function when its inputs are
explicit facts and the result needs no context, transaction, clock read or
port. Pass dates/time explicitly. Keep loading, permission resolution, locks,
unit-of-work and durable effects in application. Do not create a duplicate
public/internal value just to populate domain, or move a forwarding facade
there. ADR 0075's floor protects real translation and transaction behaviour;
it does not justify a new hop or a raised budget.

## Selection criteria (accepted)

- **New SQL projection:** one named read needs a statement across write owners,
  cannot use existing capabilities without changing required consistency,
  filtering or query budget, and has an exact tenant-safe table grant plus
  owner-boundary tests. Name the read, not a generic data-access bucket.
  A single consumer or a small source file is neither a veto nor a reason.
- **Port-only read:** prefer a consumer-owned reader port within the existing
  consumer. Retain the named existing port-composed views above; a new
  independent read owner needs its own ADR showing a stable read purpose,
  reuse/coordination benefit and why the consumer is not its natural home.
- **Merge projections:** require a common read purpose, compatible audience,
  consistency and access semantics, and a concrete deletion plan for policy
  entries, ports and bindings using ADR 0080's allocation. Shared table names
  or fewer folders do not suffice. Keep disjoint grants exact; never substitute
  a domain-to-projection import or a generic foreign repository.
- **Split a domain:** require independent vocabulary, invariant/lifecycle and
  an unambiguous table partition, plus a credible benefit for independent
  changes or team ownership. Trace every operation crossing the proposed cut,
  its commands, lock order, transaction and durable effects. If those become
  workflows, count their added contracts and root bindings against the benefit.
  Use internal capabilities first when they achieve the same independence.
- **Approval and locks:** an approved owner change updates ADR 0066's affected
  assignments, the module map, glossary if terminology changes, policy and
  canonical validation together. Keep the DAG, no foreign implementation
  reach, no writable projection/workflow data and no budget growth by moving
  code out of scans. Size thresholds are inspection prompts, not gates.

## Invariants and enforcement limits

Existing locks, not checks claimed to have been run by this docs-only change:

| Invariant | Lock locations under `backend/` |
|---|---|
| Exact write ownership and projection grants | `internal/architecture/cli_test.go`: `TestCheckRejectsMissingOrConflictingTableOwners`, `TestCheckRejectsInvalidReadProjection`, `TestPolicyRejectsProjectionPackageAliasesWithDuplicateGrants` |
| Canonical domains/platforms, acyclic graph and no domain-to-projection rules | `internal/architecture/canonical_owners_test.go`, `acyclic_target_test.go`, `target_rule_test.go`: `TestPolicyRejectsDomainProjectionRules` |
| Tenant and relationship-sensitive reads | ADR 0080's Guardian Link, Block Display and Staff Inbox test pointers; `modules/parentmessageinbox/internal/adapters/postgres/projection_test.go`; `modules/parentrequestfeedprojection/internal/adapters/postgres/query_test.go` and `isolation_test.go`; `modules/staffdirectoryview/isolation_test.go` |
| Lifecycle and planning behaviour | `modules/careplan/contracttest/care_exit_storage_cutover_test.go`; `modules/timetable/internal/application/template_series_roster_pure_test.go`; `modules/workforce/internal/planning/staff_shift_move_mock_test.go` and `staff_shift_series_integration_test.go` |
| No forwarding/composition-budget expansion | `test/module_passthrough_ratchet_test.go`, `test/module_legacy_budget_ratchet_test.go`, `test/module_compose_alias_ratchet_test.go`, `internal/architecture/ratchet_budgets_test.go` |

`scripts/backend-architecture.sh check` enforces the declared architecture, not
whether an audience or lifecycle *should* be one owner. Canonical validation
pins domain/platform owners, not this exact projection list. Behaviour tests
protect their named scenarios, not every data leak. ADR review enforces the
selection criteria; no new size, owner-count or domain-layer-LOC gate is added.
Work to cover root scan gaps remains #4542; write-workflow/exception review
remains #4538. This ADR changes neither their policy nor the parallel #4543
settings work.

## Follow-up proposals, not newly created issues

- Use ADR 0080's existing Identity same-owner value-copy proposal before
  adding an owner boundary. Preserve validation, sentinels and collection
  semantics; this issue adds no new implementation scope to it.
- When changing Timetable series reconciliation or Workforce planning, assess
  extracting the identified pure rule functions into an owner-local domain
  package only if that improves reuse/testability. Keep current behavioural
  tests and transaction/lock placement. This is optional local deepening,
  not approval for a module split or a layer-wide migration.

No runtime defect is claimed. No implementation issue
for a merger or split is required by these keep decisions. Reopen with concrete
independent lifecycle/team evidence if those conditions change.

## Reproducing the new measurements

Run from the repo root. The script reads immutable Git objects and uses the
standard library. LOC excludes `_test.go` and files tagged `testhooks`, matching
ADR 0080; role accounting includes all production subpackages. Its output gives
P/G/Rprod/Rtest, owner kinds and total/role LOC used above. The wiring numbers
are referenced from ADR 0080, not remeasured here.

```bash
python3 - <<'PY'
import collections, json, pathlib, re, subprocess
ref = '1d1c6985ade16653d8f1fa208a29e358663ab303'
p = json.loads(subprocess.check_output(
    ['git', 'show', ref + ':backend/architecture/policy.json']))
packages = {x['path']: x for x in p['packages']}
mods = '''timetableenrollmentprovenance guardianlinkview timetableblockdisplay
parentrequestfeedprojection staffmessageinbox timetableprojection studentdirectoryview
staffannouncementaudience staffdirectoryview operatordashboard emergencysnapshot
parentmessageinbox'''.split()
print('SHA', ref)
for mod in mods:
    paths = {path for path in packages
             if path == 'modules/' + mod or path.startswith('modules/' + mod + '/')}
    owners = {packages[path]['owner'] for path in paths}
    grants = [g for g in p['read_projections'] if g['package'] in paths]
    rules = [r for r in p['rules'] if r.get('source_owner') in owners
             or r.get('target_owner') in owners]
    print('policy', mod, len(paths), len(grants),
          sum('production' in r['scopes'] for r in rules),
          sum(any(s != 'production' for s in r['scopes']) for r in rules))
print('owner kinds', dict(collections.Counter(o['kind'] for o in p['owners'])))
paths = subprocess.check_output(
    ['git', 'ls-tree', '-r', '--name-only', ref, 'backend/modules'], text=True).splitlines()
proc = subprocess.Popen(['git', 'cat-file', '--batch'],
                        stdin=subprocess.PIPE, stdout=subprocess.PIPE)
counts = collections.defaultdict(collections.Counter)
for path in paths:
    if not path.endswith('.go') or path.endswith('_test.go'):
        continue
    mod = path.split('/')[2]
    if mod not in ('timetable', 'careplan', 'workforce', 'identityaccess', 'enrollment'):
        continue
    proc.stdin.write((ref + ':' + path + '\n').encode())
    proc.stdin.flush()
    size = int(proc.stdout.readline().split()[2])
    data = proc.stdout.read(size)
    proc.stdout.read(1)
    if re.search(rb'^//go:build .*\btesthooks\b', data, re.M):
        continue
    pkg = str(pathlib.PurePosixPath(path).parent).removeprefix('backend/')
    counts[mod][packages[pkg]['role']] += len(data.splitlines())
proc.stdin.close()
proc.wait()
for mod, roles in counts.items():
    print('LOC', mod, sum(roles.values()), dict(sorted(roles.items())))
PY
```
