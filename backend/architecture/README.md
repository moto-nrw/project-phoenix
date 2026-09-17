# Backend architecture policy

`policy.json` is the source for package and runtime write ownership, package
roles, tenant-safe read projections, legacy-composition symbols, dependency
classes, import scopes, and target import rules. Generated lint configs and
diagrams must derive from this file and must not be committed.

## Canonical module map

[ADR 0010](../../docs/adr/0010-file-storage-is-the-eighteenth-domain.md)
records File Storage as the eighteenth domain, and
[ADR 0012](../../docs/adr/0012-export-transfer-is-the-tenth-platform.md)
records Export Transfer as the tenth platform module, for
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580), resolved in
[#3034](https://github.com/moto-nrw/project-phoenix/issues/3034).
The canonical owner lists are:

| Domain (18) | Policy owner |
|---|---|
| Identity & Access | `identity-access` |
| People Directory | `people-directory` |
| School Membership | `school-membership` |
| Organisation & Tenancy | `organization-tenancy` |
| School Structure | `school-structure` |
| School Calendar | `school-calendar` |
| Facilities | `facilities` |
| Enrollment | `enrollment` |
| Care Plan | `care-plan` |
| Timetable & Activities | `timetable-activities` |
| Student Presence | `student-presence` |
| Workforce | `workforce` |
| Appointments | `appointments` |
| Communication | `communication` |
| Device Fleet | `device-fleet` |
| Meal Plan | `meal-plan` |
| Feedback | `feedback` |
| File Storage | `file-storage` |

| Platform (10) | Policy owner |
|---|---|
| Tenant Runtime | `tenant-runtime` |
| Transaction Runtime | `transaction-runtime` |
| Security Runtime | `security-runtime` |
| Settings Platform | `settings-platform` |
| Delivery Platform | `delivery-platform` |
| Observability | `observability` |
| Audit Platform | `audit-platform` |
| Scheduler Runtime | `scheduler-runtime` |
| Document Rendering | `document-rendering` |
| Export Transfer | `export-transfer` |

Policy validation pins these exact IDs and kinds for the moto module path;
missing, extra, renamed, or reclassified owners fail before graph analysis.
Declaration order is irrelevant. Independent evaluator fixtures retain their
own module maps. A future domain/platform change requires an explicit
architecture decision, reflected in #2580, this map, and validation.

File Storage owns the managed file lifecycle: file metadata, folders,
folder-role/account grants, quota, and cleanup intents. Identity supplies
account and role facts; File Storage applies school-scoped folder grants.
Communication retains announcement-access rules, including for attachments.
Document Rendering produces output without owning its audience or storage;
rendering alone neither saves a file nor grants access.

#2707 registers `documents.files`, `documents.folders`,
`documents.folder_roles` and `documents.folder_accounts` under `file-storage`
through the ADR 0015 adoption path (policy epoch 6 to 7); the announcement
attachment and cleanup tables were owned before. `documents.file_cleanup`
still has no owner: the retained generic document repository reaches it only
dynamically, so the baseline records no `tables.unclassified` finding to adopt
it from, and the File Storage composition binds its intent operations to that
repository as a compatibility permission until the table can be adopted.
#2710 adopts `users.persons_guardians` under `people-directory` the same way
(policy epoch 7 to 8). #3221 adopts `users.guardian_financial_data` under
`people-directory` and moves the staff messaging persistence out of
`database/repositories/users`, adopting `users.staff_message_threads`,
`users.staff_message_participants`, `users.staff_messages` and
`users.staff_message_reads` under `communication` (policy epoch 12 to 13).
With that the baseline records no `tables.unclassified` debt.
The same ticket moves the two foreign accesses the package kept on owned
tables: `users.profiles` belongs to the account, not the person, so its model,
contract and repository move to `models/auth` and
`database/repositories/auth` under `identity-access`.

The staff messaging writes live in the Communication Postgres adapter
`modules/communication/internal/adapters/staffpostgres`. The inbox and unread
badge join People Directory's person rows, so they read through the tenant-safe
projection `modules/communication/internal/adapters/staffinbox` (owner
`staff-message-inbox`). `modules/communication/staffstore` is the
Communication composition seam that keeps the `models/users` staff message
repository contracts for the legacy factory. The colleague picker, its
authorization predicate and the role kinds stay in People Directory as
`MessageableStaffRepository` and reach the seam as a colleague directory.
`target.svg` shows File Storage as domain and Document Rendering as platform;
do not commit the generated diagram.

## Commands

Run commands from the repository root:

```bash
scripts/backend-architecture.sh check
scripts/backend-architecture.sh audit-issues \
  --api-url https://api.github.com
scripts/backend-architecture.sh explain \
  --scope production \
  --source github.com/moto-nrw/project-phoenix/services/mealplan \
  --target github.com/moto-nrw/project-phoenix/models/mealplan
scripts/backend-architecture.sh diagram
scripts/backend-architecture.sh dependencies \
  --focus module:meal-plan
scripts/backend-architecture.sh dependencies \
  --focus package:services/mealplan
scripts/backend-architecture.sh validate-ticket \
  --ticket backend/architecture/checkpoint-ticket-template.json
```

`check` loads packages with `GOOS=linux`, `GOARCH=amd64`, and `CGO_ENABLED=0`.
It checks production imports, same-package test imports, and external-package
test imports as separate scopes. Every package explicitly declares all three
roles. Test roles describe the seam (`module-internal-test`,
`module-behavior-test`, `workflow-decision-test`,
`workflow-integration-test`, `adapter-test`, or `e2e-test`), so production
permissions cannot leak into test scopes. A finding uses this stable key:

```text
scope|rule|source|target
```

The key identifies the violation. Source locations are evidence and do not
form part of that identity. Each location contains a project-relative Go file,
line, and the affected import, function, method, or declaration. `check` prints
all locations below the key. It sorts and deduplicates them, so the same source
tree produces the same output on every host.

Locations may change when code moves without changing the violation key. The
exact JSONL baseline therefore keeps only `scope`, `rule`, `source`, `target`,
and `issue`; it never stores locations. Generated JSON projections attach a
`locations` array to each violation instead. This lets migration tooling group
current evidence by the stable key without turning line changes into new debt.

Production analysis also uses Go syntax and type information to enforce these
semantic boundaries:

- a table has one runtime write owner;
- foreign reads require an owner query or an exact `read_projections` grant
  whose `tenant_safe` flag is true;
- foreign reads and writes are detected in BUN table/model calls, joins, SQL
  fragments, and raw SQL; writes also include `MERGE` and `TRUNCATE`;
- unresolved or unclassified table expressions remain findings;
- direct BUN/SQL access is restricted to Postgres, migration, test-support,
  and named projection adapters;
- public/contract packages cannot expose ORM, repository, internal-model,
  `map[string]any`, generic CRUD, or BUN-tagged types;
- references to the exact symbols in `legacy_composition` are findings outside
  the defining package.

`data_objects` contains target-owned runtime data, not every historical table
ever created by a migration. Access to an obsolete table with no target owner
therefore remains a `tables.unclassified` finding until that access and table
are removed; do not invent an owner merely to make the check quiet.

The current backend still violates the target policy, so the normal `check`
loads the committed exact baseline from `architecture/legacy.jsonl`. It passes
only when the current violation set is exactly equal to that baseline. Pull
request CI additionally reads the policy and baseline from the event's full
base-commit SHA. Debt is shrink-only, except when a policy tightening exposes
an exact import already present and allowed at that SHA (see below). The required
status is `Backend architecture ratchet`.

Release promotion (`development` to `main`, same repository) uses the exact
development head as its accepted comparison point. It requires a successful
complete push CI run for that SHA, main ancestry, and an identical release
tree. The current graph still has to match its exact baseline. This avoids
retroactively applying newer ratchet mechanics to all historical cutovers in
one release. Diverged main must first be merged into development and pass CI.
Other PRs retain the strict base-SHA comparison. The promotion guard is
`scripts/check-release-architecture.mjs`; missing CI evidence fails closed.

## Generated projections

`diagram` evaluates the real graph once, loads the committed baseline by
default, and writes these files to a newly created system temporary directory:

- `target.svg` contains only owners and production edges declared by the
  target policy, including declared target modules that do not have packages
  yet.
- `migration.svg` condenses the current production graph by owner. Allowed
  edges are gray, violations present in `--baseline` are orange-red, and new
  violations are dashed red.
- `architecture.json` uses projection schema version 2 and contains both
  graphs plus exact violation keys, their
  source/target owners, and sorted source locations, so follow-up tooling can
  group ratchet work by owner and capability.
- `go-arch-lint.yml` projects the target policy into go-arch-lint's coarser
  owner-level model. It is an additional guard; the evaluator remains
  authoritative for roles, scopes, semantic checks, and exact edges.

`dependencies --focus ...` also loads the committed baseline and writes
`dependencies.svg`, `dependencies.json`, and `dependencies.goda`. Prefix a
focus with `module:` or `package:` when the same text names both. The generated
Goda query pins `GOOS=linux`, `GOARCH=amd64`, and `CGO_ENABLED=0` from the
policy. Unknown, ambiguous, and target-only modules without current packages
fail with a concrete error.

All files are generated artifacts. The default location and every accepted
`--output` location are inside the system temp tree; do not commit them.

## Exact legacy ratchet

The legacy baseline is canonical JSONL sorted by `scope|rule|source|target`.
Each record has exactly these fields in this order:

```json
{"scope":"production","rule":"imports.forbidden","source":"example/source","target":"example/target","issue":"https://github.com/moto-nrw/project-phoenix/issues/1234"}
```

The first four fields identify one exact violation. `issue` identifies its one
open migration ticket. Locations are non-identifying evidence and stay out of
this file. Wildcards, package-family patterns, blank lines,
duplicates, unsorted records, non-canonical JSON, and issue reassignment are
errors. The normal command has no init, approve, update, or rebaseline mode.

Local mode requires exact equality between the current violations and the
committed baseline. PR mode adds `--base-ref` with the event's full 40-character
base commit SHA and reads the baseline and policy directly from that Git object.
Candidate entries must be a subset of the base entries or meet the import-debt
conversion rule below. Unchanged entries must keep their issue. Candidate
policy, package classification, and ownership changes may not weaken the checks
enforced by the base policy.

### Converting temporary permissions to exact debt

Temporary compatibility imports belong in `legacy.jsonl`, not target-allowed
rules. Remove their policy permissions and record each resulting
`imports.forbidden` tuple with its open cleanup issue. PR mode accepts a newly
recorded tuple only when all three conditions hold:

1. The base policy allowed that exact source, target, and scope.
2. The source package imported that target in that scope at the immutable base
   SHA, under the policy's fixed build context.
3. The candidate policy no longer allows the import.

The checker reads import headers from Git blobs and lets `go list` select the
production, internal-test, and external-test files. Candidate imports cannot
supply base evidence. New targets, new sources, scope changes, and imports
excluded by build tags, OS, or cgo cannot become historical debt. Existing
violations and semantic findings do not qualify for this conversion.

Once recorded, the tuple follows the ordinary shrink-only and issue-audit
rules. Restoring its target permission is still policy loosening. Removing the
import requires removing its debt entry in the same change. No schema change
or second allowlist is needed for the final contract step.

The Workforce compatibility adapter (`modules/workforce/legacy`) and the
Workforce HTTP composition (`modules/workforce/inbound`) are classified as
`workforce`/`adapter`. Their imports of the retained `models/active`,
`models/education`, `models/users`, `models/base` and calendar-date contracts
are target-allowed `workforce.adapter.*` rules only because PR mode cannot
record debt for a package the candidate creates. They are compatibility
permissions for #2688's retained consumers, not target dependencies: convert
them to exact debt with the rule below once the packages exist at a base SHA,
and delete the adapter with the last consumer of those contracts.

The Workforce shift-planning HTTP composition
(`modules/workforce/inbound/shiftplanning`) is classified `workforce`/`http`
for the same reason (#2689). It mounts the `/api/staff-shifts` and
`/api/shift-types` route adapters over the public `StaffShiftPlanning` and
`ShiftTypeAdministration` contracts. Every `workforce.http.*` rule is a
compatibility permission, not a target dependency: the `inbound-common`,
`security-runtime`, `tenant-runtime`, `orm-sql` and route-adapter edges
follow the same conversion the `workforce.adapter.*` rules above are bound
to, and the `timetable-domain` and `legacy-shared-domain` edges remain for
the time-tracking composition below. The `timetable-application` and
`document-rendering` edges fell with #3219. Convert the rest to exact debt
once the package exists at a base SHA.
The retained `models/schedule` repository contracts are served from
`database/repositories` over the Workforce facade; that package's
`models/schedule` import is already recorded debt.

The retained staff-shift, shift-type, assignment, Dienstplan overview,
staff-notice, shift-plan sync and substitution services that
`services/schedule` used to hold are retained as the
`inbound-staff-shifts`/`adapter` compatibility package
`modules/workforce/legacy/shiftplanning` (#3219), moved file for file with
their behaviour tests. The package also serves the public `StaffShiftPlanning`
and `ShiftTypeAdministration` contracts that the shift-planning HTTP
composition used to map, so the implementations now sit behind the contracts
and the HTTP composition only mounts the routes. No HTTP path, status code,
error string, authorization check, tenant scoping, coverage-conflict or
substitution semantics changed. The services could not land on the existing
`workforce`/`adapter` point: they still speak the retained `models/schedule`,
`models/audit`, `models/config` and `models/facilities` rows, the Delivery
producer, the plan export capability and the retained timetable services (in
`services/schedule` then, `modules/timetable/legacy/timetableplanning` since
#3218), and PR mode rejects a new permission on a point that exists at the
base SHA. Every `inbound-staff-shifts.adapter.*`,
`inbound-staff-shifts.module-internal-test.*` and
`inbound-staff-shifts.module-behavior-test.*` rule and every
`<consumer>.<role>.inbound-staff-shifts-adapter` rule is a compatibility
permission that exists only because PR mode cannot record debt for a package
the candidate creates: convert them to exact debt with the rule above once the
package exists at a base SHA, and dissolve the services into the Workforce
application and domain layers under #2730. The `adapter` role also covers the
`cmp.Or` logger fallback the helper-consolidation ratchet requires
(`inbound-staff-shifts.adapter.test`). The move replaced the post-construction
setters of the shift and series services with construction-time options,
because the composition surface guard records mutable wiring per package and
a relocated setter would count as growth. Four subjects the ticket listed stay
in `services/schedule` because they are methods of, or are served through,
the retained timetable services and the two packages may only depend in one
direction: the shift-coverage probe (`DetectShiftCoverage`, reached through
`TimetableDataService`), the staff pool (a `TimetableDataService` method),
the whole-day bulk substitution (an `InstanceService` method over the
deviation pipeline) and the student partial-absence service (a student-care
write over the care-exception lock). The shift-coverage interval vocabulary
the overview and the probe share stays in `services/schedule`
(`shift_coverage_intervals.go`) and the moved overview binds it through the
package's own aliases (`vocabulary.go`). Seven package-private helpers the
moved services shared with the retained timetable services (`isoWeekday`,
`marshalDeviationValue`, `normalizeActor`, `int64FilterArgs`, `legacyList`,
`broadcastStaffingChanged`, `isPlannableInstance`; `legacy_helpers.go`) and
the overview test fakes now exist in both packages, because exporting them
would widen the retained timetable package that #3218 dissolves. The copies
are temporary debt that goes with the retained services under #2730 and
#3218; a fix to the #1844 staffing broadcast must land in both until then.
#3218 moved the coverage probe, the staff pool, the bulk substitution and the
shared interval vocabulary on with the timetable services (below); the
partial-absence service moved with the care services in #3220.

The retained timetable and instance services that `services/schedule` used to
hold (templates, splits and updates, materialization, the instance lifecycle
and deviation pipeline, conflict detection, the timetable data and operations
reads, calendar periods, holidays and closing days, planning tracks, the
roster reconciler, cleanup and the attendance mirror) are retained as the
`inbound-timetable`/`adapter` compatibility package
`modules/timetable/legacy/timetableplanning` (#3218), moved file for file with
their behaviour tests. No HTTP path, status code, error string, authorization
check, tenant scoping, recurrence, exception or materialization semantics
changed. The care, arrival and pickup services stayed behind for #3220 (below),
together with what only they need or what both sides share: the generic
effective-time engine (`effective_time_service.go`,
`effective_time_domains.go`, which the arrival and pickup services
instantiate), the day-planning resolver the care-day resolver calls, and the
`ScheduleError` type with its sentinels (the moved package aliases the type,
so `errors.As` matches either name). The moved package imports
them (`modules/careplan/legacy/careschedule` since #3220) for these and for
the care-day, care-exception-lock, baseline and effective-time contracts;
that package never imports it back, so the packages depend in one direction
only. `germanDateLayout` now
exists in both packages, and the care-day resolver uses the effective-time
engine's identical weekday helper instead of the materializer's copy. The
package could not land on an existing point: it still speaks the retained
models, repositories, Delivery producer, Presence services, settings and
identity helpers, and PR mode rejects a new permission on a point that exists
at the base SHA. Every `inbound-timetable.adapter.*`,
`inbound-timetable.module-internal-test.*` and
`inbound-timetable.module-behavior-test.*` rule and every
`<consumer>.<role>.inbound-timetable-adapter` rule is a compatibility
permission that exists only because PR mode cannot record debt for a package
the candidate creates: convert them to exact debt with the rule above once the
package exists at a base SHA, and dissolve the services into the Timetable
application and domain layers under #2730. The five
`timetable-activities`/`application` rules of the Workforce and Presence
consumers fell with the move because those packages no longer import
`services/schedule`. The materialization and split services and the instance
lifecycle take their optional collaborators at construction
(`WithCareBoundReader`, `WithOfferingRosterResync`,
`InstanceServiceDependencies.GuardianNotices`) instead of post-construction
setters, because the composition surface guard records mutable wiring per
package; the legacy factory binds the later-constructed enrollment decision
and announcement services behind construction-time closures (the announcement
side through the delegating `lateCareCancellationPublisher`, which replaces
the former setter; `GuardianNotices` is therefore never nil in production).
The moved test helpers the guard flagged for appending cleanups to shared
fixture structs drop their no-op appends and register their sweeps with
`t.Cleanup` in the same order; no-op appends inside test bodies are unchanged.
The typed-root composition inventory only scans `api`, `cmd`, `services`,
`database/repositories` and `test`, so the moved tests' factory calls left
`composition.json` with the move (as with #3219); they are not fewer callers.
The only SQL call site the cluster still reached outside the owner's
persistence adapter, the legacy weekend-instance cleanup in
`database/repositories/schedule`, is deleted; production already used the
Timetable owner's `DeleteRemovedWeekendActivityInstances`, whose owner test now
also pins the retained-weekday and empty-set cases.

The retained care, arrival and pickup services that `services/schedule` held
last (care-schedule change requests, arrival and pickup schedules and their
baselines, class arrival exceptions, the care-day and day-planning resolvers,
the effective-time engine, pickup auto-excusal, partial absences, the
same-day cutoff and the care-exception lock) are retained as the
`inbound-schedules`/`adapter` compatibility package
`modules/careplan/legacy/careschedule` (#3220), moved file for file with their
behaviour tests; `services/schedule` no longer exists. No HTTP path, status
code, error string, authorization check, tenant scoping, auto-excusal,
attendance-correction or parent-permission semantics changed. The package
could not land on the existing `care-plan`/`adapter` point for the same reason
as #3218; `inbound-schedules` had no package left since #3073. Every
`inbound-schedules.*` rule and every `<consumer>.<role>.inbound-schedules-*`
rule is a compatibility permission that exists only because PR mode cannot
record debt for a package the candidate creates: convert them to exact debt
once the package exists at a base SHA, and dissolve the services into the Care
Plan application and domain layers. The test doubles of
`services/schedule/scheduletest` split with their subjects: the pickup
baseline double is `modules/careplan/legacy/careschedule/carescheduletest`
(`inbound-schedules`/`test-support`), and the closing-day mock became a test
file of its only consumer, `api/timetable`. The care request service takes its
sharing resolver and its test clock at construction
(`WithRequestShareVisibility`, `WithCareRequestToday`) instead of the former
setters; the legacy factory passes the same lazy resolver the excused requests
use (`requestShares`), so a request the parents service has not been bound to
yet still yields the neutral co-guardian line. Tests that pinned the clock
after construction now pass it at construction (the students route through its
module clock).

`database/repositories/schedule` no longer exists either (#3220). Its three
remaining SQL repositories (class arrival exceptions, staff notices and the
activity-reopen instance update) write Timetable-owned tables, so their SQL
moved into the owner's persistence adapter
(`modules/timetable/internal/adapters/postgres`, `retained_rows.go`) with an
explicit tenant filter on every statement; `modules/timetable/compose` serves
the retained `models/schedule` and `models/users` repository contracts over it
with the legacy error contract. `ClassArrivalExceptionRepository` dropped its
unused generic CRUD methods. The pure query-option translations and listing
adapters the legacy composition uses moved to `modules/timetable/compose`
(`legacy_repository_options.go`, `legacy_composition_support.go`,
`calendar_period_usage.go`), which may already import their models. The SQL
test providers the cross-package tests build and the repository behaviour
tests are the test-only root `modules/timetable/legacy/timetablesqltest`
(`inbound-timetable`/`test-support`, `e2e-test` in both test scopes, as
`modules/workforce/contracttest`); its `inbound-timetable.test-support.*`,
`inbound-timetable.e2e-test.*` and `<consumer>.<role>.inbound-timetable-test-support`
rules are compatibility permissions of the same kind. With both packages gone,
all #2730 entries left `legacy.jsonl`, together with the other resolved
imports of the two packages, and the 25 rules that only those imports used
(for example `parent-portal.adapter.timetable-application` and
`inbound-timetable.adapter.timetable-activities-application`) were deleted.

The Workforce time-tracking HTTP composition
(`modules/workforce/inbound/timetracking`) is classified `workforce`/`http`
under the same compatibility permissions (#2690). It replaced
`api/time-tracking` and serves the public work-session, absence, month,
ledger, month-close, overview, audit-log, export and personnel-record
contracts of `modules/workforce`, which the root composition adapts from the
retained time-tracking services in `modules/workforce/legacy/timetracking`
and the retained `services/users` services. Its own-shift, assignment and
staff-notice routes reach the retained services in
`modules/workforce/legacy/shiftplanning` (`workforce.http.inbound-staff-shifts-adapter`,
#3219) and still map their `models/schedule` rows. The kiosk staff
clock consumes the public device-scan contract in `modules/devicescan`
(`process-device-scan`/`public`); `staff-clock.to.process-device-scan` is the
one rule anchored to that new point, and the staff-clock workflow reaches the
Workforce time clock only through its own port.

The retained Workforce time-tracking services
(`modules/workforce/legacy/timetracking`, #3213) are classified
`workforce`/`adapter` with `adapter-test` in both test scopes. They are the
work-session, absence, month-card, ledger, month-close, overview, audit-log,
export, vacation-opening, absence-type, labor-policy, payroll-configuration
and GDPR-cleanup services that `services/active` used to hold, moved file for
file with their tests: the root composition adapts them behind the public
`modules/workforce` contracts exactly as before, and no HTTP path, status
code, error string, authorization check, tenant scoping, payroll or DATEV
output changed. Their production imports (`internal/timezone`,
`models/active`, `models/base`, `tenant`) are covered by the existing
`workforce.adapter.*` compatibility permissions; the Delivery producer, the
scheduler, the CLI and the schedule-owned sick cascade are reached through
ports the composition root binds (`TimeTrackingEvents`,
`schedulerTimeTrackingCleanupPort`, the `services` root aliases and
`ShiftPlanSyncBridge`). The move also replaced every post-construction setter
of these services with construction-time options, because the composition
surface guard records mutable wiring per package and a relocated setter would
count as growth. The in-package tests build the retained calendar-date, model
and base vocabulary through the package's own aliases (`vocabulary.go`)
because the `workforce`/`adapter-test` internal seam already existed and can
admit no new permission. The external behaviour tests keep importing the
retained contracts directly under the `external_test`-scoped
`workforce.adapter-test.*` rules added with the package: they exist only
because PR mode cannot record debt for a package the candidate creates.
Convert them to exact debt with the rule above once the package exists at a
base SHA; the package itself goes when the Presence half (#3214) deletes
`services/active` and its keys, and finally when the retained services
dissolve into the Workforce application and domain layers.

The Care Plan compatibility adapter (`modules/careplan/legacy`) uses this
representation. Its remaining imports and repository-composition caller are
bound to #2743, the root API caller to #2750, and the test-support caller to
#2748. The conversion in #3032 removes 13 target permissions and records 14
existing imports; it adds no runtime dependency or composition caller.
`target.svg` has no compatibility-rule edges. `migration.svg` renders these
exact imports as orange-red `legacy` debt, separate from gray target-valid
imports and dashed-red new violations, even when they share owner endpoints.

The class-day read projection (`modules/classday`, #2701) is the `class-day-view`
projection owner: `modules/classday` is its public contract, `internal/application`
builds the slot lists and the school-portal day view, `internal/ports` declares
the consumer-owned read seams, and `compose` binds them. The projection reads
`schedule.activity_instances`, `schedule.instance_students`, `active.visits`,
`active.attendance`, `users.students`, `users.persons`, `education.groups`, the
rooms and the Care Plan status days and pickup exceptions only through the public
owner facades (`class-day-view.from.*`). Its `class-day-view.compose.*`
permissions for the retained schedule services (care-day derivation, effective
times, pickup baselines), the enrollment report and class-day write seam, the
settings service, the user context and the JWT permission check exist only
because PR mode cannot record debt for a package the candidate creates. They are
compatibility bindings, not target dependencies: convert them to exact debt with
the rule above once the packages exist at a base SHA, and remove each binding
when its owner exposes the read publicly. The same applies to the
`inbound-classday.*` permissions of the class-day HTTP adapter
(`modules/classday/http`: common HTTP rendering, the permission contract, the
JWT claims, the calendar-date type and the Bun database the shared school-scope
middleware takes), to the `inbound-school.identity-application` and
`inbound-school.orm-sql` permissions of the school portal (`modules/schoolportal`),
and to the retained-repository and retained-service permissions of the
projection's integration and adapter tests.

The operator dashboard read projection
(`modules/organizationtenancy/internal/adapters/operatordashboard`,
`operator-dashboard-view`/`postgres`, #3253) serves the counts, the
organisation and school summaries and the PWA standalone-usage buckets that
Organisation & Tenancy's operator provisioning shows. It replaces the raw
platform summaries repository with constant, named queries and holds the
exact `operator-dashboard` grant for `platform.organizations`,
`platform.schools`, `auth.accounts`, `auth.account_tenants`,
`auth.account_roles`, `auth.roles` and `iot.pwa_standalone_usage`; it never
writes, and table write ownership is unchanged. Only
`modules/organizationtenancy/compose` constructs it, over the caller's
administrative transaction. Devices and persons are not part of it: the
provisioning flow reads them through Device Fleet and People Directory.
Replace each grant with an owner query once Identity & Access and Delivery
expose the account and usage counts.

The live-group read projection (`modules/grouplive`, `group-live-view`/`public`)
reads every foreign fact through consumer-owned ports and the Care Plan
excused-request contract; it persists nothing and never writes. Its
compatibility adapter (`modules/grouplive/legacy`, `group-live-view`/`adapter`)
binds those ports to the retained identity, settings, presence, schedule,
people, and education services and to the shared authorize rules. Every
`group-live-view.adapter.*` rule is a compatibility permission that exists
only because PR mode cannot record debt for a package the candidate creates
(#2702): convert them to exact debt with the rule above once the package
exists at a base SHA, rebind each port to its owner's public capability as it
appears, and delete the adapter with the last legacy source.

The supervision read projection (`modules/supervisiondashboard`,
`calendar-view`/`public`, #2703) builds the "Aktuelle Aufsicht" aggregate
from plain owner facts through consumer-owned ports; it persists nothing and
never writes. Its compatibility adapter (`modules/supervisiondashboard/legacy`,
`calendar-view`/`adapter`) binds those ports to the retained identity,
settings, presence, Schulhof, timetable-operations and day-planning services
and to the shared authorize rules, and maps the Timetable and Facilities wire
rows field by field (the adapter tests pin byte-identical JSON). Every
`calendar-view.adapter.*` and `calendar-view.adapter-test.*` rule is a
compatibility permission that exists only because PR mode cannot record debt
for a package the candidate creates: convert them to exact debt with the rule
above once the package exists at a base SHA, rebind each port to its owner's
public capability as it appears, and delete the adapter with the last legacy
source. The same applies to the staff calendar HTTP adapter
(`modules/staffcalendar/http`, `inbound-calendar`/`http`: common HTTP
rendering, the permission contract, the JWT middleware, and the calendar-date
type; its calendar binding now uses the native
`modules/schoolcalendar/portal` contract) and to the
statistics HTTP adapter (`modules/statistics/http`, `inbound-statistics`/`http`:
the same shared HTTP dependencies, the Bun database the tenant middleware
takes, the Document Rendering renderer and, as compatibility binding, the
retained `services/statistics`), including their `*.adapter-test.*`
permissions.

The presence HTTP composition (`modules/studentpresence/inbound/presence`) is
classified `student-presence`/`http` for the same reason (#3207). It replaced
`api/active` and serves the visit, check-in, check-out, group, supervision and
Schulhof routes from the public Student Presence contract and the public
supervision projection, with unchanged paths, status codes and error strings.
Its `student-presence.http.inbound-common` and
`student-presence.http.legacy-shared-domain` rules, the
`root-composition.to.student-presence-http` mount and every
`student-presence.adapter-test.*` rule are compatibility permissions, not
target dependencies: the shared HTTP rendering edge goes when the inbound
common package moves, the calendar-date edge with the retained
`internal/timezone` type, and the settings and user-context test edges with the
fixtures that still name them. Convert them to exact debt with the rule above
once the package exists at a base SHA.

The retained Presence services, rows and repositories (#3214) moved file for
file, with their tests, out of the legacy packages the HTTP composition left
behind: `services/active` into
`modules/studentpresence/legacy/services/active` (`student-presence`/`adapter`,
`adapter-test` in the package, `e2e-test` for the behaviour tests that compose
the legacy repository and service graph over a real database),
`services/statistics` into `modules/studentpresence/legacy/statistics`
(`student-presence`/`adapter`, `adapter-test`), `models/active` into
`modules/studentpresence/legacy/models/active` (`student-presence`/`domain`,
`adapter-test`) and `database/repositories/active` into
`modules/studentpresence/legacy/repositories/active`
(`student-presence`/`postgres`, `e2e-test`). The root composition serves the
public `modules/studentpresence` contracts from them exactly as before; no
HTTP path, status code, error string, authorization check or tenant scoping
changed, and the IoT error strings PyrePortal maps stay byte-identical. Every
`*.student-presence-adapter`, `*.student-presence-domain`,
`student-presence.adapter.*`, `student-presence.domain.*`,
`student-presence.e2e-test.*`, `student-presence.compose.student-presence-domain`
and `student-presence.adapter-test.student-presence-domain` rule that names
#3214 is a compatibility permission, not a target dependency: PR mode cannot
record debt for a package the candidate creates, so the 87 baseline entries
that named the four old paths (the 28 recorded against #2737 and the 59 that
other Contract tickets held for their own imports of those packages) became
these rules instead of new keys. Convert them to exact
debt with the rule above once the packages exist at a base SHA, and remove
each rule with the last consumer of the retained contract. The move also
replaced the retained services' post-construction setters (settings resolver,
tenant runtime, guardian waker) and the session repository's room-directory
binder with construction-time options, because the composition surface guard
records mutable wiring per package; the legacy repository root rebinds the
room owner behind the directory it installed at construction. The legacy
composition roots and the retained timetable and isolation tests reach the
retained repositories only through the `Legacy*` names in
`modules/studentpresence/compose`, so no package outside Student Presence
imports the retained Postgres package. The retained repositories parse
calendar dates through the row vocabulary (`models/active/vocabulary.go`) and
the retained services' in-package tests build the shared persistence base
through the package's own aliases (`vocabulary.go`), because the
`student-presence`/`postgres` and `student-presence`/`adapter-test` seams
already existed and can admit no new permission. The package goes when the
retained services dissolve into the Student Presence application and domain
layers; `modules/workforce/legacy/timetracking` keeps importing the retained
rows until then.

The import HTTP composition (`modules/dataimport/inbound`, with its runtime
binding in `modules/dataimport/inbound/compose`) keeps the `inbound-import`
owner and its `http` / `compose` roles after replacing `api/import` (#3217).
It serves the student, staff, class-list and opening-balance preview, import
and template routes from the public Data Import contract with unchanged paths,
status codes, error strings, multipart handling, file-size limit and
permission checks. The `root-composition.to.import-http` mount replaces the
`api -> api/import` debt entry that fell with the old package; it and the
`inbound-import.compose.*` rules that bind `api/common`, `auth/jwt`, `tenant`
and the ORM are compatibility permissions, not target dependencies. Convert
them to exact debt with the rule above once the package exists at a base SHA.

The user-context read side (who is the caller, which tenant, which
permissions: the request-memoized identity chain, the SSE subscription
resolution, the parent request-review policy and the student-access decision)
is retained as the `inbound-usercontext`/`adapter` compatibility package
`modules/identityaccess/legacy/usercontext`, and its `/api/me` routes are
served by `modules/identityaccess/inbound/usercontext` under the same owner's
`http` role (#3224). Both replaced `services/usercontext` and `api/usercontext`
with unchanged behaviour, paths, status codes, error strings and authorization
checks; their 49 baseline entries fell with the old packages. The read side
could not land on the existing `identity-access`/`application` point: it still
returns the retained `models/*` rows to its consumers, and PR mode rejects
a new permission on a point that exists at the base SHA. Every
`inbound-usercontext.adapter.*`, `inbound-usercontext.http.*` and
`inbound-usercontext.adapter-test.*` rule, every
`<consumer>.<role>.inbound-usercontext-adapter` rule and the
`root-composition.compose.inbound-usercontext-http` mount is a compatibility
permission that exists only because PR mode cannot record debt for a package
the candidate creates: convert them to exact debt with the rule above once the
packages exist at a base SHA, and dissolve the adapter into the Identity &
Access application and public contract under #2725. The adapter role also
covers the `net/http` status constants the SSE setup error carries through
the generic `external.http-router.adapter` rule, and the former
`inbound-usercontext.to.identity-access` target rule is removed until that
contract exists.

The settings test support (`services/config/settingstest`, `settings-platform`/
`test-support`) scripts the payroll and work-schedule settings that presence
behaviour tests drive through their real services, so those tests name a school's
configuration instead of registry keys and ORM rows. Its `models/config` import is
a target dependency of the settings owner; its calendar-date import exists only
because the settings package still carries the contractual work-schedule rows
(#3207) and converts to exact debt with them.

The Identity & Access guardian-access capability (`modules/identityaccess`,
`identity-access`/`public`) is the first just-in-time slice of the late
Identity & Access migration (#2580 sequencing, #2699): it resolves platform
accounts and grants an account guardian access to the tenant in context over
`auth.accounts`, `auth.account_tenants`, `auth.account_roles` and
`auth.roles`. Enrollment acceptance consumes it through a consumer-owned port;
the retained `services/auth` invitation flow still reaches the same tables
through its own repositories. The other tables the acceptance cutover (#2699)
names are not served by Identity & Access. Enrollment acceptance now reads,
locks, creates and renews `users.students` through the bounded People Directory
enrollment contract. Profile writes distinguish unchanged fields from explicit
NULL. The application coordinates departure-plan locks and companion-stranding
checks; People Directory writes the normalized plan and its legacy mirrors,
and Care Plan removes companion edges. All commands join the same tenant
transaction. `users.class_list_entries`,
`enrollment.request_child_offerings` and `schedule.instance_students` reach
their owners through the legacy-composition adapters over the School
Membership, Enrollment and Timetable facades. The retained in-memory student
mapping and owner-backed compatibility adapters are cleanup debt under #2733,
not alternative acceptance writers. Its `identity-access.compose.*` and
integration-test rules mirror the People Directory owner; the
`legacy-composition.to.identity-access-*` permissions exist because the
legacy service factory still composes the enrollment decision service and
go with #2751.

The same owner serves platform operator identity and refresh sessions
(#2720): `platform.operators` and `platform.operator_refresh_tokens` are read
and written only through the public `OperatorAccess` capability. Operator
rows are platform-wide, so the operator operations join an ambient
administrative transaction and otherwise run on the root connection; the
operator flows open the transaction where rotation or revocation and its
audit evidence must commit together. `platform.operator_audit_log` is
appended through the Audit owner's appender as a platform-scoped ledger
(`models/audit.OperatorAuditEntry`, no tenant handshake). Operator login with
its mandatory MFA gate, the MFA-proven token issue, refresh with rotation
recovery, profile and password changes with their session revocation, and
the operator-led school access of accounts (which schools an existing
account reaches, with which role; `auth.account_tenants`,
`auth.account_roles` and `auth.account_permissions` writes with the
account's session revocation) live in the module's application layer as the
public `OperatorAuthentication` and `OperatorAccountAccess` capabilities
(#3252). The facts those flows need from other owners (the operator MFA
service, the operator audit ledger, the pending e-mail change links, the
password policy, schools and organisations, the People Directory and School
Membership identity chain a school access provisions) are bound at the
serving root through public-typed seams (`compose.OperatorDependencies`);
the role assignment rules are the module's own since #3314. The login,
refresh, profile, password and
school-access handlers live in `modules/identityaccess/inbound/operator`,
the owner's HTTP adapter, and call the public contract there. `api/operator`
keeps the operator router with its middleware chain and rate limiters,
mounts those handlers (`inbound-operator.http.identity-access-http`) and
hands them its error bodies, so the operator surface keeps one wire format.
The rules of the new `identity-access/http` role are anchored to that
package; `api/operator` itself still has no rule for the public package.
The retained `services/platform` operator service keeps the MFA and passkey
token exchange (`OperatorSessions`), the profile read and the e-mail change;
operator rows reach the retained MFA, passkey, invitation and e-mail change
flows through the `OperatorDirectory` port the root binds to the public
module. The public audit evidence is typed (`TenantAccessEvidence`,
`OperatorAccessChange`); the root renders it into the ledger keys. The former
`models/platform` operator and refresh-session repository contracts and
their compatibility adapters are deleted; the operator invitation, e-mail
change and passkey flows stay under #2722 and #2724. The operator MFA
records (#2723): `platform.operator_mfa_credentials`,
`platform.operator_mfa_email_challenges` and
`platform.operator_mfa_trusted_devices` are read and written only through the
public `OperatorMFARecords` capability, with the same platform-wide
transaction rule as the operator rows. The retained `services/platform`
operator MFA service keeps the challenge, verification, enrollment and
trusted-device flow and reaches the rows through the `OperatorMFARecords`
port the root binds to the public module; its disable cascade (enrollment
delete, device revocation, lockout reset) stays one administrative
transaction the module operations join. The `models/platform` MFA repository
contracts and their `database/repositories/platform` adapters are deleted;
the MFA models remain as the retained port's value types. The foreign
`auth.accounts` reads of the People Directory, Care Plan parent and CLI
packages use owner queries bound the same way (`identity_ports.go`): the
account lookup and active-account subquery of `database/repositories/auth`
and the public account fact. The
same owner serves the account refresh sessions behind tenant, parent and
school login, refresh, tenant switching, logout, session validation and
revocation (#2720): `auth.tokens` is read and written only through the
public `AccountSessionAccess` capability. Session statements apply the tenant
filter the runtime scoped the caller to, so tenant-scoped callers see only
their school's sessions while the tenantless pre-authentication flows
resolve every school's rows; the writes join the caller's transaction, which
for login, refresh, switch and logout is the administrative transaction the
module opens around rotation and its audit evidence. The login, refresh,
tenant and school switch, logout, session validation, session cleanup and
revocation orchestration lives in the module's application layer (#3251),
with the `auth.accounts`, `auth.account_tenants`, role and permission reads
it needs served by the module's own store; the retained
`models/auth.TokenRepository` contract and its compatibility adapter are
deleted. The facts the flows need from other owners (schools, persons, the
password verifier, the JWT codec, the MFA gate, the settings lock, the audit
ledger, push subscriptions) are bound at the serving root through
public-typed seams (`compose.SessionDependencies`). The retained
`services/auth.Service` keeps the `AuthService` contract by delegating its
session methods to a consumer-owned port the root binds to the public
module; `api/auth` and the operator identity routes
(`modules/identityaccess/inbound/operator`) call the public contract
directly, while `api/parent`, the school portal and the SSE routes keep the
delegation
because no target rule lets those inbound packages import the identity-access
public package. The legacy `auth.accounts_parents` model, repository and its
six `/auth/parent-accounts` routes stay unchanged; the table has no target
owner and that conflict stays open under #2720.

The role and permission administration (#3314) lives in the module's
application layer as the public `RoleAdministration` capability
(`RoleQuery`, `RoleCommand`, `PermissionQuery`, `PermissionCommand`): role
and permission CRUD, account role assignment with the school identity it
owes, direct account grants, role-permission selections and the default
staff permission. `auth.roles`, `auth.permissions`, `auth.role_permissions`,
`auth.account_roles`, `auth.account_permissions` and the account and
membership locks the mutations serialize on are still read and written
through the retained `database/repositories/auth` repositories:
`database/repositories/identity_roles.go` serves the module's
`compose.RoleDirectory` seam over them, without changing a statement or the
lock order, until #3226 moves the stores into the module. The school-role
assignment policy (`ValidateAssignableSchoolRole` and the Lehrkraft and
guardian-tier classification) is a public function of the module; the
operator school access binds it inside the module, and the retained
registration, linking and invitation flows of `services/auth` reach it and
the caregiver-profile fact through their `SchoolIdentityProvisioning` port.
`api/auth`, the staff membership runtime, operator provisioning and the data
import call the public contract; the caregiver capability and the person
service of `services/users` reach it through consumer-owned ports the root
binds. Classifying and promoting a stored `users.students_guardians` role
applies the security-runtime guardian presets, which neither the module nor
the root may import, so it moved to its write owner, the People Directory
(`services/users`).

The session end workflow (`workflows/sessionend`, owner `session-end`, kind
`workflow`, #2697) is a cross-module write workflow of #2580. Its
public command closes one live kiosk session in one UnitOfWork: it joins the
caller's tenant transaction (or opens its own), locks the group, closes the
open visits, supervisions, and the group through the Student Presence
`GroupSessionCommand`, stamps the slot check-outs through the Timetable
owner and finalizes the mirrored instance through its `InstanceCompletion`
port, resolves the
announcement data while the tenant role is still set, and queues every SSE
event and guardian wake for after the commit. Its `ports` name the four owner
capabilities it consumes (`session-end.port.*`, `session-end.application.*`);
`compose` binds the tenant runtime and the realtime broadcaster. The root
still satisfies `InstanceCompletion` with the retained
`TimetableBridgeService` in `modules/timetable/legacy/timetableplanning`
(#3218; the attendance finalization that #1747 requires before an instance
may close); that binding is a legacy edge
of the root composition tracked by #2762, and the port is rebound to the
Timetable owner's public capability when that cutover lands. The kiosk
endpoint (`api/iot/sessions`, `inbound-iot-sessions.to.session-end`) calls
exactly this facade and no longer orchestrates the Timetable bridge and the
active service itself. The other session-ending paths of the retained active
service (the manual group end, the instance Complete and Cancel transitions
through `EndActivitySession`, the timeout, and the nightly bulk end) write
their presence rows through the same owner commands (`EndGroupSession`,
`EndGroupSessions`). The two takeover paths that end a group and move its
visits and supervisors to a replacement (force start, absorption of an
unsupervised group into a started instance) release the group through the
owner's `EndGroup`. The legacy group end and bulk end repository methods
are deleted. Those paths still complete the mirrored instance through the
retained bridge inside the active service; moving them onto the workflow is
the remaining #2762 work. The workflow owns no data object: policy validation
refuses a write owner of kind `workflow`. Under ADR 0013, this registration
raises the policy epoch from 3 to 4 and uses only candidate-created packages.
Existing-owner import and data-ownership guards remain unchanged.
Its adapter-test permissions for the presence,
timetable, people, and facilities compositions bind the real owners in the
workflow integration tests; they are test-only permissions, not target
dependencies.

The device-scan workflow (`process-device-scan`, #2698) runs every kiosk
scan, pickup query, heartbeat and attendance toggle through one orchestrator:
`modules/devicescan` is its public contract, `internal/application` the
workflow over the public Device Fleet, Student Presence and Facilities
capabilities, `internal/ports` its consumer-owned seams, and `compose` binds
them. `api/iot/checkin` calls the contract and renders through the runtime the
root injects; the retained IoT session, device and feedback routes keep their
own error tables. Every `process-device-scan.compose.*` permission for the
retained presence, people, activity, education, pickup and settings services,
the device principals, the tenant runtime, the calendar-date type and the SQL
driver is a compatibility binding that exists only because PR mode cannot
record debt for a package the candidate creates: convert them to exact debt
with the rule above once the packages exist at a base SHA, and rebind each
port to its owner's public capability as it appears.

Under ADR 0013 this data-less workflow registration raises the integrated
policy epoch from 4 to 5 and uses only candidate-created packages. Existing
owner/import guards remain unchanged. Scan and attendance writes require an
existing tenant transaction, supplied by the IoT middleware; direct callers
without one fail before mutations. Daily checkout reads tenant overrides or
the registry default, never a process environment fallback. Before deployment,
any intended prior environment value must be stored as an explicit per-school
setting. This integration does not inspect or modify deployed settings.

The student deletion workflow (`workflows/studentdeletion`, owner
`student-deletion`, kind `workflow`, #2710,
[ADR 0016](../../docs/adr/0016-student-deletion-is-an-application-workflow.md))
is the one coordinator of a permanent child deletion: the confirmed deletion
of an active child, the graduate purge from the Abgänge view and the deletion
behind a pending care withdrawal. Its public package exposes the preview and
the commands over the People Directory and Care Plan contracts
(`student-deletion.to.*`); `compose` binds the remaining owners (Timetable,
School Structure, Student Presence, Communication, Enrollment, Appointments,
Feedback, Identity & Access, Audit), the tenant UnitOfWork, the permission
principal and the realtime broadcaster. Every owner performs its own read and
mutation: People Directory counts guardian links, hard-deletes the student
row and anonymizes the person tombstone; Care Plan resolves and redacts
withdrawal tasks and records the document cleanup intents before the cascade;
Timetable deletes the child's assignments and counts the archived roster
removals; School Structure anonymizes the grade-transition ledger; Identity
counts guardian invitations; Student Presence counts attendance and the
cross-tenant holiday visits; the Audit command appends the two tombstones.
The workflow contains no SQL and no repository import. The retained
`services/users` deletion service, the cross-schema
`database/repositories/users` deletion repository and the handler-side purge
transaction of `api/students` are deleted; the legacy composition builds the
workflow once (`legacy-composition.compose.deletion.*`) and the students HTTP
adapter calls exactly its public commands. The
`student-deletion.compose.deletion.timetable-legacy-view.postgres` rule is a
compatibility binding for the activity-enrollment count, not a target
dependency: rebind it when the Timetable owner exposes the count publicly.
The `people-directory.module-behavior-test.deletion.*` permissions bind the
real composition in the workflow's hermetic tests; they are test-only.

Under ADR 0013 this data-less workflow registration raises the policy epoch
from 7 to 8 and uses only candidate-created packages. The same epoch adopts
`users.persons_guardians` into People Directory through the ADR 0015 path:
its one recorded `tables.unclassified` finding named a package already
classified under that owner, and the deletion unlinks the legacy guardian rows
through the owner command. Existing owner/import guards remain unchanged.

The grade transition workflow (`workflows/gradetransition`, owner
`grade-transition`, kind `workflow`, #2711,
[ADR 0017](../../docs/adr/0017-grade-transition-is-an-application-workflow.md))
is the one coordinator of the school-year rollover: the draft and its
mappings, the preview with its cohort fingerprint, the apply and the revert.
Its public package exposes the queries and commands over the School
Structure, People Directory, School Membership and Student Presence
contracts (`grade-transition.to.*`); `compose` binds the tenant UnitOfWork,
the permission principal per operation, the Audit command for the class-list
rewrites, and composes School Structure and Student Presence over the shared
database. The Timetable roster reconciliation, the recurrence gate and the
enrollment offering-roster resync are consumer-owned ports the legacy
composition binds from the retained application services of those owners.
Every owner performs its own read and mutation: School Structure stores the
draft, the transition gate, the status transitions and the three ledgers
(history, class-teacher, class-list) through its new transition capability;
People Directory takes the exclusive class-writes gate, locks and
re-validates the cohort, promotes, graduates and reactivates the children
and releases or restores their bracelets; School Membership rewrites the
class-teacher assignments and the class-list entries; Timetable archives
and replays the materialized rosters; Student Presence answers the check-in
guard. The workflow contains no SQL and no repository import. The retained
`services/education` grade transition service, its class-ledger helpers, the
cross-schema `database/repositories/education` transition repository and the
`models/education` repository contract are deleted; the legacy composition
builds the workflow once (`legacy-composition.compose.transition.*`) and the
admin HTTP adapter calls exactly its public commands. The
`school-structure.module-behavior-test.transition.*` permissions bind the
real composition in the workflow's hermetic tests; they are test-only.

Under ADR 0013 this data-less workflow registration raises the policy epoch
from 9 to 10 and uses only candidate-created packages. The School Structure
capability grows inside its existing owner and roles; no table changes
owner and no existing-owner import guard is expanded.

The parent-portal workflow (`workflows/parentportal`, owner `parent-portal`,
kind `workflow`, #3227,
[ADR 0019](../../docs/adr/0019-parent-portal-is-an-application-workflow.md))
takes the guardian-portal flows out of `services/parent`, which coordinates
Care Plan, Enrollment, Timetable & Activities, Communication, People
Directory, Settings and Student Presence for one guardian and one child.
`workflows/parentportal/care` holds the today status, the weekly care plan
and its requests, the booked care offerings and their change requests, the
course requests and the guardian-child resolution. It reaches the
request-sharing ledger only through its `RequestSharer` port.
`workflows/parentportal/messaging` holds announcements, request sharing,
messaging and the self-service chat pills behind its `ChildResolver` port.
It returns the guardian-child and note sentinels declared in `care`
(`parent-portal.application.care`). Both packages are
`parent-portal`/`application` with `workflow-integration-test` in both test
scopes. The code moved file for file with its tests. The retained parent
services (see below) keep the public `Service` contract, bind both ports and
delegate the moved methods. No HTTP path, status code, error string,
authorization check or tenant scoping changed. Every
`parent-portal.application.*` and `parent-portal.integration-test.*` rule is
a compatibility binding to the retained models, services and shared helpers
the moved code already imported. These rules exist only because PR mode
cannot record debt for a package the candidate creates. Convert them to exact
debt with the rule above once the packages exist at a base SHA. #3229 moves
`api/parent` onto the workflow.

Under ADR 0013 this data-less workflow registration raises the policy epoch
from 11 to 12 and uses only candidate-created packages. No table changes
owner and no existing-owner import guard is expanded. The move resolved six
recorded `services/parent` internal-test imports, which left `legacy.jsonl`.

The retained parent services (`workflows/parentportal/legacy`,
`parent-portal`/`adapter`, #3228) are the write, master-data, guardian,
related-account, consent, profile, enrollment-list, sick-note and meal-plan
flows that `services/parent` still held after #3227, moved file for file with
their behaviour tests. `services/parent` and its `care-plan`/`application`
classification are gone, and with them the transitional
`care-plan.application.parent-portal` edge and the last 47 `legacy.jsonl`
entries that named the package (#3227 had already resolved six of its
internal-test imports). The package keeps the public `Service` contract that
the guardian portal HTTP composition and the service factory consume. No HTTP path, status code,
error string, validation rule, authorization check or tenant scoping changed,
and no table changed owner. The package could not join the existing
`parent-portal`/`application` point: it still speaks the retained audit,
identity, base, localization and realtime vocabulary, and PR mode rejects a
new permission on a point that exists at the base SHA. It uses the
candidate-created `parent-portal`/`adapter` point with `adapter-test` in both
test scopes instead. Every `parent-portal.adapter.*` and
`parent-portal.adapter-test.*` rule and the consumer rules
`inbound-parent.http.parent-portal-adapter`,
`inbound-parent.adapter-test.parent-portal-adapter` and
`legacy-composition.compose.parent-portal-adapter` are compatibility
permissions that exist only because PR mode cannot record debt for a package
the candidate creates. They cover every future `parent-portal`/`adapter`
package, so no other package may join that point before the conversion.
Convert them to exact debt with the rule above under #2735 once the package
exists at a base SHA, and dissolve the package into the workflow
as each flow reaches its owner's public capability. The move replaced the
post-construction absence-notifier and student-photo setters with
construction-time configuration, because the composition surface guard
records mutable wiring per package and a relocated setter would count as
growth. The factory resolves the student-photo lifecycle on use, since the
API bootstrap builds it after the parent services.

The guardian portal HTTP composition (`modules/careplan/inbound/parent`,
#3229) replaced `api/parent`, moved file for file with its adapter tests. It
keeps the `inbound-parent` owner and the `http` / `adapter-test` roles, so the
36 `legacy.jsonl` entries #2735 recorded for `api/parent` fell with the path,
together with the root mount (#2750) and the calendar end-to-end import
(#2748). No route path, method, status code, error string, authorization check
or tenant scoping changed. Besides the `care-plan` capability that
`inbound-parent.to.care-plan` names, the handlers still call the retained
auth, enrollment and users services, the parent-portal adapter and the shared
HTTP helpers directly, and match their result types and error values:
routing those calls through the `modules/careplan` contract would add imports
to the existing `care-plan`/`public` point, which PR mode rejects. The
care-schedule diff row is named through the parent-portal adapter
(`CareRequestDiffEntry`), so the production `services/schedule` import fell
with the move; the adapter tests still build that vocabulary. After the move the package
is the only `inbound-parent` package, so the point exists only in the
candidate. Every `inbound-parent.http.*` and `inbound-parent.adapter-test.*`
rule this move added, `root-composition.to.inbound-parent-http` and
`test-support.e2e-test.inbound-parent-http` are compatibility permissions, not
target dependencies. They cover every future `inbound-parent` package, so no
other package may join that point before the conversion. Convert them to exact
debt with the rule above under #2580 once the package exists at a base SHA;
the `auth/jwt` and `services/auth` edges then wait for #2725. The move replaced
the calendar, push, notification-preference and PWA-usage setters with a
construction-time `ResourceConfig`, and the auth rate-limiter setter with
`RouterWithAuthRateLimiter` (the school portal shape), because the
composition surface guard records mutable wiring per package and a relocated
setter would count as growth.

The emergency snapshot read projection (`modules/emergencysnapshot`,
`emergency-snapshot`/`public`, #2704) builds the Notfallliste, the present
children with location, reachable adults and the optional health note, from
plain owner facts through consumer-owned ports and owns the row order and
the document shape; it persists nothing and never writes. Its compatibility
adapter (`modules/emergencysnapshot/legacy`, `emergency-snapshot`/`adapter`)
binds the presence ports, the room names and the person identities to the
public Student Presence, Facilities and People Directory facades
(`emergency-snapshot.from.*`) and, as compatibility bindings, the student
row with its health note and legacy contact columns and the guardian contact
rows to the retained People Directory repositories, the health-info switch to
the retained settings service, the presence-mode read to the retained active
service, the calendar day and German collation to the shared helpers, and
the PDF to the Document Rendering renderer; the adapter tests pin the
rendered document field by field and prove the two-tenant RLS boundary over
the real facades and repositories. Every `emergency-snapshot.adapter.*` and
`emergency-snapshot.adapter-test.*` rule is a compatibility permission that
exists only because PR mode cannot record debt for a package the candidate
creates: convert them to exact debt with the rule above once the package
exists at a base SHA, rebind each port to its owner's public capability as it
appears, and delete the adapter with the last legacy source. The same applies
to the emergency HTTP adapter (`modules/emergencysnapshot/http`,
`inbound-emergency`/`http`: common HTTP rendering, the permission contract
and the Bun database the shared tenant middleware takes) and its
`inbound-emergency.adapter-test.*` permission.

The plan export capability (`modules/planexport`, `document-rendering`/`public`,
#2706) renders the printable Dienstplan and Betreuungsplan from plain records
through consumer-owned ports declared in the same package; it owns no table
and never writes. Its compatibility adapter (`modules/planexport/legacy`,
`document-rendering`/`adapter`) binds those ports to the retained schedule
services and repositories and maps their rows field by field. The
temporary permissions for the adapter's legacy imports, its tests, the
capability's calendar-date test import and the root-composition call into the
adapter are recorded as exact `imports.forbidden` debt in `legacy.jsonl` under
#2706. The adapter's own public-capability binding, the public capability's
calendar-date value type and the tests' imports of the owner's public and
application packages remain target-allowed. Rebind each legacy port to its
owner's public capability as it appears, remove each tuple with its import,
and delete the adapter with the last legacy source. The two-tenant RLS test
binds the Dienstplan's shift and staff reads to the tenant transaction under
test, because their retained sources are Workforce adapters composed only by
the legacy repository factory. The retained `services/listexport` renderer keeps its
`module-internal-test` seam, so the capability's rendering tests declare the
`workflow-decision-test` seam and the adapter tests the `adapter-test` seam.
The `inbound-timetable.to.document-rendering` and
`workforce.http.document-rendering-public` edges are the target shape (an
inbound adapter calling the public capability) and stay.
The same change moves the birthday routes to `modules/birthdays/http`
(`inbound-birthdays`/`http`) and the document upload coordinator to
`modules/filestorage/documents` (`file-storage`/`adapter`). Their
compatibility bindings, the birthday handlers' retained user-context, birthday
service and birthday row imports and the coordinator's retained storage
backend, are exact debt under #2706 as well; the remaining `inbound-birthdays.*`
permissions are the inbound target shape. The
`inbound-students.to.file-storage-adapter` binding is different: its source
package existed before the move and imported the old path as debt under
#2731, which PR mode cannot carry over to the new target. It is a
[named exception](https://github.com/moto-nrw/project-phoenix/issues/2580#issuecomment-5638973300)
to rule 5 of `backend/CLAUDE.md`, still tracked by #2731; it goes when the
student document handlers move to the public File Storage capability, and no
new caller may rely on it. The file store handlers' equivalent exception went
with #2707. The generic file-metadata repository
(`database/repositories/documents`) and model (`models/documents`) keep their
five `document-rendering` debt entries under #2706: their tables belong to
File Storage (ADR 0010); the student and staff document handlers are their
remaining consumers.

The File Storage capability (`modules/filestorage`, `file-storage`/`public`,
#2707) owns the school file storage and the attachments of
Elternmitteilungen: folders with their visibility rule and share lists, the
files inside them, the storage quota, the audit trail, the intent protocol
that makes an interrupted upload recoverable, and the sweep the worker runs.
`internal/application` holds the authority and upload rules,
`internal/adapters/postgres` serves `documents.folders`,
`documents.folder_roles`, `documents.folder_accounts`, `documents.files`,
`documents.announcement_attachments` and
`documents.announcement_attachment_cleanup` with static table names, and
`compose` binds the ports: membership, account roles and shareable roles to
the public Identity & Access capability, the audience picker's names to the
public People Directory query (both target shape), and, as compatibility
permissions, the two file settings, the retained Audit file-event contract,
the shared permission matcher, the retained storage backend and the retained
generic document repository for the `documents.file_cleanup` intents. The
folder visibility rule is now owner SQL over the owner's own tables with the
viewer's membership and role ids supplied by Identity & Access; the former
foreign reads of `auth.account_tenants`, `auth.account_roles` and
`auth.roles` are gone. The HTTP adapter (`modules/filestorage/http/files`,
`inbound-filestore`/`http`) serves the unchanged `/api/files`,
`/api/announcement-attachments` and `/parent-news-attachments` contracts
through the public capability only; multipart parsing and magic-byte
validation stay in the adapter, bytes and intents are the owner's. Every
`file-storage.compose.*` compatibility permission, the
`inbound-filestore.*` shared HTTP permissions, the `root-composition.to.*`,
`legacy-composition.to.*` and `test-support.to.file-storage-public` bindings
exist only because PR mode cannot record debt for a package the candidate
creates: convert them to exact debt with the rule above once the packages
exist at a base SHA, and rebind each port to its owner's public capability as
it appears. The legacy service factory composes the module because the
announcement guard and the guardian audience it binds live there; the root
supplies the uploads backend and the metrics sink. The retired
`api/filestore`, `services/filestore`, `database/repositories/filestore` and
`models/filestore` packages are deleted with their 36 baseline entries.

The shared request-review projection (`modules/requestreview`,
`request-review-view`/`public`, #2705) is the one staff-facing list of every
parent request awaiting or carrying a decision (Stammdaten, Betreuungszeiten,
Angebote, Abwesenheiten and the office's own booking corrections): origin,
type, child, requested change and decision. It owns the merged newest-first
order with its deterministic tie-break, the one keyset cursor across the
queues, the urgent-before-normal phases, the permission narrowing of a caller
without `users:update` to the excused queue, the past-request bulk
consequences, the whole-queue conflict grouping and the badge count; it
persists nothing and never writes. The per-type wire shapes live in its
public package because the decide, preview and detail routes answer with the
same shapes. Every foreign fact enters through consumer-owned ports: the four
queues, the correction log, the caller's rights, the group names and the
Familienschutz flag. Its compatibility adapter (`modules/requestreview/legacy`,
`request-review-view`/`adapter`) binds those ports to the retained
`services/users`, `modules/careplan/legacy/careschedule`, `services/enrollment` review queues,
the Care Plan excused-request contract, the retained review policy, people,
education and Familienschutz services, and derives the per-row facts
(urgency, past scope, version, conflict keys) with the owners' own rules.
`api/students` serves `GET /students/change-requests` and
`/change-requests/pending-count` through the public capability; the old
four-service fan-out in that package is deleted. The `inbound-students` import
of the adapter exists only because the per-type decide routes still render
the same wire shapes from the retained service items; it goes when those
routes move to their owners. #3174 converts the temporary permissions for the
adapter's legacy imports, its tests, and its students and root-composition
callers to exact `imports.forbidden` debt in `legacy.jsonl`, tracked by #3179.
Keep #3179 open until all 27 imports and the legacy adapter are removed.
The adapter's own public-capability binding, its
Care Plan public-contract import, and the students HTTP call to the public
projection remain target-allowed. Rebind each legacy port to its owner's
public capability as it appears, remove each tuple with its import, and delete
the adapter with the last legacy source.
The projection reads no table directly, so it needs no `read_projections`
grant; the owner queues keep their per-child scope and tenant isolation. Its
`Today` port resolves the review day once per call, so the queues' urgency
phase and the rows' urgency and past flags cannot straddle midnight or
disagree under a test clock. The public enrollment change requests keep
their own `config:manage`-gated list (#2435), which the client merges into
the same Eltern list, so the projection does not cross that permission
boundary. The staff RSS feed (`/students/change-requests/rss-feed`) keeps its
own registered `parent-request-feed-read-model` grant. It postdates #2705's
evidence, also announces the `config:manage` enrollment change requests and
selects by submission instant in one statement, and this owner cannot take it
over in PR mode: a `read_projections`
grant is accepted only for a newly created projection owner, `care-plan` may
not import `request-review-view/public`, and neither
`request-review-view/adapter` nor the root may import
`parent-request-feed-view/postgres`. Moving the feed needs a reviewed policy
decision under #2580.

The Device Fleet authentication composition (`modules/devicefleet/deviceauth`)
is classified as `device-fleet`/`http`. Its `device-fleet.device-auth.*`
permissions for the retained `models/platform` school row, the
`models/config` setting key, and the `database/sql`/`pgdriver` error
classification exist only because PR mode cannot record debt for a package
the candidate creates. They are compatibility permissions for the retained
school service and settings service, not target dependencies: convert them
to exact debt with the rule above once the package exists at a base SHA, and
replace them with an Organisation & Tenancy school query and a Settings
Platform read when those owners expose them.

PR mode allows a classification when the candidate adds the first Go file in
that exact package. Rules added with it must be anchored to an owner and role
used only by candidate-created packages; owner-kind rules remain forbidden.
Existing unclassified packages and modified packages do not qualify. A legacy
composition guard may be removed only after the guarded package declaration is
deleted.

PR mode requires every new runtime data object created in candidate
migrations to have exactly one valid `data_objects` write owner in the same
candidate. This is a hard policy check, not baseline debt. A candidate may add
the ownership entry when a new non-test Go file under `database/migrations/`
creates that exact schema-qualified object through static SQL passed to
`NewRaw`, `Exec`, `Query`, `QueryRow`, or their context variants. Detection
includes ordinary, unlogged, and foreign tables, views, materialized views,
explicit sequences, and `SELECT INTO`, including `CREATE OR REPLACE VIEW`
and quoted identifiers. Implicit serial/identity sequences follow their table.
Views are conservatively treated as writable because PostgreSQL can expose
writes through them. SQL comments and string literals, test-only files,
temporary tables, and non-data DDL such as schemas, indexes, and types do not
require runtime write ownership.

SQL discovery resolves string constants, concatenation, and singly assigned
local query variables, and inspects executable `DO` and function bodies.
It preserves quoted-name case: identifiers that the policy cannot represent
fail rather than silently matching a different object. Unresolved queries,
schema builders, dynamic procedural SQL, and encoded executable bodies fail
closed with a request to use static SQL or dollar quoting. Adjacent continued
SQL string literals are joined before body analysis. The lexer follows
[PostgreSQL's lexical rules](https://www.postgresql.org/docs/current/sql-syntax-lexical.html)
for identifier quoting, comments, and literal continuation; it does not
execute migrations or attempt to interpret arbitrary Go or procedural code.

The object must not be mentioned by any migration at the base SHA, the write
owner must already exist, and changing or newly assigning ownership for an
existing object remains a policy loosening. Modified historical migrations
never qualify for this exception, and historical unowned objects do not need
rebaselining. The check uses the immutable local Git base, not GitHub.

`audit-issues` performs the network-dependent GitHub liveness check separately.
The wrapper supplies the committed baseline; callers must provide `--api-url`,
and `GITHUB_TOKEN` is optional for authenticated requests. A GitHub or network
error fails this audit and cannot change the deterministic `check` result or
appear as a green audit.

## Migration evidence

Schema version 2 separates ordinary waves from runtime checkpoints. Convert
version 1 tickets explicitly. Both kinds retain prerequisites, owner/capability,
packages, tables, exact ratchet keys, atomic cutover, tests, rollback/cleanup,
and measurable exit criteria. Unknown fields and blank required evidence fail.

### Ordinary waves

Copy `migration-ticket-template.json` and replace all example/guidance values.
Set `ticket_kind` to `migration`. `checkpoint_reference` must be the canonical
issue URL of the latest accepted checkpoint in `runtime-checkpoints.json`.
Missing, malformed, future, unaccepted, and superseded references fail.

Record only flow-specific evidence needed for the cutover: raw source,
workload/environment/observation window, thresholds, query counts, stable
errors, failure-path and transaction rollback results, and smoke results.
Keep deployment rollback and cleanup in `rollback_and_cleanup`. Non-applicable
evidence needs a concrete reason. Additional metrics are allowed when the flow
needs them; a reference does not waive failure, rollback, query-budget, or smoke
checks. Ordinary waves do not repeat the full benchmark suite independently.

### Checkpoint measurements

Copy `checkpoint-ticket-template.json` and set `ticket_kind` to `checkpoint`.
Only these canonical checkpoint issue URLs are valid:

1. #3019 establishes the one-third baseline across completed migrations.
2. #3020 compares with #3019 after its blocking core migrations and before
   session-end, device-scan, and enrollment-acceptance workflow cutovers.
3. #3021 compares with both predecessors after contract/storage cutovers and
   before #2751 removes the remaining legacy composition.

The `checkpoint` object requires an exact commit, reproducible PostgreSQL 17
and production composition environment, toolchain, versioned workload, data
volume, concurrency, and warm-up. Cover successful and failing HTTP/module
flows and applicable Worker paths. `runs` contains exactly three measured runs
after warm-up; `median` and `worst` report every applicable metric. Each report
records source, workload, thresholds, query count, p50/p95, stable errors,
pool waits, measured lock waits, deadlocks/serialization retries, Worker
duration/retries/backlog, and affected rows. Non-applicable metrics need reasons.
Full statement duration is not lock-wait evidence.

`comparison` explains every metric against the required predecessors (or
establishes #3019's baseline). Keep workload/environment equivalent;
`workload_bridge` confirms equivalence or links old/new workloads measured on
the same commit. `regression_issues` links focused optimization issues for
material regressions; unexplained regressions block acceptance. `decision`
records completed blockers and acceptance/rejection; #3021 permits or blocks
#2751. Raw output and interpretation stay in the checkpoint issue or review
evidence, not the repository. Template values are examples, not measurements.

### Recording acceptance

`runtime-checkpoints.json` is a reviewed acceptance registry, not architecture
policy. It starts empty because no checkpoint has been accepted. Ordinary
waves cannot pass until #3019 is accepted; checkpoint measurements can be
validated before acceptance. After explicit acceptance in the checkpoint issue,
append an entry in a reviewed change, in order without gaps:

```json
{"issue": "https://github.com/moto-nrw/project-phoenix/issues/3019", "acceptance": "https://github.com/moto-nrw/project-phoenix/issues/3019#issuecomment-123"}
```

Replace the example comment ID with the actual acceptance comment. Validation
checks issue order and comment-link shape, not comment truth or measurement
accuracy. Review must verify acceptance, closed blockers, comparable runs,
and explanations. A ticket-local `accepted` flag cannot grant acceptance.
`--checkpoints path/to/registry.json` supports a reviewed registry snapshot
and isolated test fixtures; paths resolve from the repository root. Do not
use a self-authored registry to bypass acceptance.

No ownership or ratchet entries change. This contract sets no
machine-independent latency limits.

## Composition inventory

### Shrink-only fields and mutable wiring (#3030)

The normal `scripts/backend-architecture.sh check` compares composition source
with the merge-base of `HEAD` and `origin/development`. PR CI supplies the event's
full base SHA through `--base-ref`. Fetch `origin/development` before a local
check. Explicit project/baseline arguments remain available for isolated
analyzer fixtures; CI always supplies the real base commit.

The guard reads Go declarations directly from that Git object and the working
tree. It rejects new named or embedded fields on `services.Factory`,
`database/repositories.Factory`, and `api.API`, including local type aliases.
There is no accepted field/setter manifest and no approve, regenerate,
rebaseline, or wildcard option. Deletion spends the removed declaration's
budget permanently once it reaches the base branch.

Mutable wiring is identified by its destination, not by a `Set` prefix:
assignments to interface/function dependencies, dependency-containing bundles,
external pointer dependencies, and Worker/Scheduler scalar configuration are
guarded. Receiver methods and pointer-parameter wiring functions are covered,
including direct receiver aliases and whole-receiver replacement. The key
includes the destination field, so an existing setter cannot acquire another
dependency silently. Newly constructed local values are not mutable wiring.
Domain/model receivers, ordinary result records, and scheduler task state are
outside this guard. This is source analysis, not general interprocedural alias
or reflection analysis; review still checks indirect wiring.

Production, same-package tests, and external-package tests have separate keys.
Test declarations cannot authorize production growth. Parsing includes
build-tagged Go files, but excludes vendor, hidden directories, and fixture
`testdata` trees. Fixtures cover both additions and permitted non-composition
changes; the other architecture checks continue to enforce package boundaries.

The existing Factory-removal and composition-contract tickets retain ownership
of shrinking the current debt. This guard does not remove their work or change
runtime composition. The historical inventory below is separate evidence:
regenerating it cannot approve new fields or mutable wiring.

### Typed-root migration evidence

`composition.json` is the checked-in prerequisite inventory for the typed-root
migration. It is pinned to evidence commit
`8dc3a9ca8ac7cb8edbfa3c17760d92c02751bc3e` and records:

- the Serve, embedded Worker, and CLI roots, plus the affected packages'
  discoverable `TestMain` roots and one smoke test per root;
- every Cobra command path and scheduler job ID;
- every typed legacy-composition reference reported by this evaluator;
- every call to `api.New`, the evidence-only `api.NewServer`, the current
  `api.WithRuntime`, `repositories.NewFactory`,
  `services.NewFactory`, the evidence-only `scheduler.NewScheduler`, the current
  `scheduler.NewWorker`, and `SetupAPITest` under the
  affected production and test trees (`api`, `cmd`, `services`,
  `database/repositories`, and `test`); this is deliberately not a scan of
  unrelated unit-test packages or migration tests;
- each constructor caller's production/test-support scope, declaration, exact
  call lines, and concrete policy-owned tables; and
- the route count, cold Serve-root construction time, registered jobs, and
  full backend-suite duration measured at the evidence commit.

`evidence_legacy_callers` and `evidence_constructor_calls` preserve the exact
files, declarations, and lines from that commit. The unprefixed caller lists
mirror the current tree so the tests still reject newly unlisted callers after
the evidence commit; they must not be presented as historical measurements.

The runtime measurements are comparison evidence, not timing thresholds. The
composition tests fail when a root, caller, command, or job drifts. Typed
legacy references come from the architecture evaluator's existing `go/types`
analysis; the constructor inventory does not reimplement that analyzer.

After an intentional caller migration, regenerate the typed legacy locations,
then the discovered test-root, constructor-caller, and job sections with:

```bash
cd backend
go test ./internal/architecture \
  -run '^TestCompositionLegacyCallerInventory$' \
  -update-composition-legacy
go test ./test -run '^TestCompositionInventory$' \
  -update-composition-inventory
```

Review the manifest diff. Fixed evidence, production roots, command paths,
smoke-test names, and runtime measurements are deliberate evidence and are
never regenerated by these normal update flags.

## Changing the policy

1. Add or move the exact package classification and data-object owner.
2. Classify each new direct dependency. Do not use path wildcards.
3. Name every projection package, readable data object, and tenant-safety
   guarantee explicitly; projection grants never permit writes.
4. Add the narrowest owner, owner-kind, role, scope, and target selector that
   describes the intended seam.
5. Run `scripts/backend-architecture.sh check` and inspect every changed
   finding.
6. Run `cd backend && go test ./internal/architecture`.

The shared-kernel declaration is a closed list: `Date`, `WallClock`,
`TenantID`, and `CorrelationID`; kernel-owned packages must be contracts. The
loader rejects unknown JSON fields,
unsupported schema versions, invalid enum values, missing or conflicting table
owners, unsafe or overlapping projection grants, stale legacy symbols,
duplicate classifications, stale packages, unknown dependencies, and
logically overlapping rules. Change
`schema_version` only when the JSON shape changes. Change `policy_epoch` only
for a reviewed architecture decision; it does not approve or rebuild legacy
findings.

Reviewed data-less workflow additions may register new workflow owners when
all their packages are candidate-created and the policy epoch increases
([ADR 0013](../../docs/adr/0013-staff-offboarding-is-an-application-workflow.md),
[#3130](https://github.com/moto-nrw/project-phoenix/issues/3130)). This does not
permit adopting existing packages, expanding existing-owner permissions, or
owning writable data. Those guards are checked independently.

A reviewed epoch may also add owner-agnostic test-infrastructure rules
([ADR 0014](../../docs/adr/0014-test-roles-may-import-test-infrastructure.md),
[#3215](https://github.com/moto-nrw/project-phoenix/issues/3215)): a rule with
no `source_owner`, a test role (`*-test` or `test-support`), and a
`target_class` of `orm-sql`, `http-router` or `test`. A `*-test` role may not
carry the production scope under this path. Everything else — owner-specific
grants, first-party targets, production roles, other external classes — is still
a loosening. Epoch 6 registered `external.<class>.<role>` for every test role,
replacing the 65 owner-specific rules they subsume, so a per-owner test-role
rule for these classes would now overlap and fail to load.

A reviewed epoch may also let a target owner adopt an existing table
([ADR 0015](../../docs/adr/0015-owners-adopt-existing-unowned-tables.md),
[#3235](https://github.com/moto-nrw/project-phoenix/issues/3235)): a new
`data_objects` entry is accepted when the table has no owner in the base
policy, the base `legacy.jsonl` records at least one production
`tables.unclassified` finding for it, and every package those findings name is
classified under the adopting owner in the candidate, no longer exists, or no
longer reads or writes the table in the candidate's current findings. The last
case covers a shared legacy package whose access to the table moved to the
adopting owner while the package itself stays for other tables
([#3221](https://github.com/moto-nrw/project-phoenix/issues/3221)). The
candidate must remove those findings from the baseline as usual. Transferring
an owned table, adopting a table with no recorded debt, and adopting while a
package of another owner still accesses it remain loosenings; after the
adoption the ordinary `tables.foreign-write` and `tables.foreign-read` rules
apply to every other accessor, and a new finding there cannot become debt.
