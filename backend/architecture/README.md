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
it from. The root now binds its intent operations through File Storage's
consumer-owned `CleanupStore` port and `modules/documentrendering/compose`
(#3445), without changing table ownership. File Storage imports neither the
generic repository nor its persistence models. Its settings, audit, permission
matching and private object storage are also supplied through consumer ports;
the seven temporary File Storage composition permissions are removed.
#2710 adopts `users.persons_guardians` under `people-directory` the same way
(policy epoch 7 to 8). #3221 adopts `users.guardian_financial_data` under
`people-directory` and moves the staff messaging persistence out of
`database/repositories/users`, adopting `users.staff_message_threads`,
`users.staff_message_participants`, `users.staff_messages` and
`users.staff_message_reads` under `communication` (policy epoch 12 to 13).
With that the baseline records no `tables.unclassified` debt.
The same ticket moves the two foreign accesses the package kept on owned
tables: `users.profiles` belongs to the account, not the person, so its model,
contract and repository move to `modules/identityaccess/legacy/authmodels` and
`modules/identityaccess/legacy/authpostgres` under `identity-access`.

#3462 assigns the demo access of the public demo (ADR 0029),
`auth.demo_accesses`, to `identity-access`: it is a passwordless credential
whose only effect is a session, and the session is minted by that owner's
existing account authentication. It is a capability of its own
(`identityaccess.DemoAccess`, composed only under `APP_ENV=demo`), not a
wider module engine. The demo school stays with `organization-tenancy`: the
capability resolves it through that owner's `FindSchoolBySlug`, bound in
`services.NewDemoAccess`, and reads no `platform` table itself.

#3349 settles the one table two owners reached for: `users.privacy_consents`
stays with `student-presence`. The recorded window bounds how long presence
data is kept, and the GDPR cleanup reads it through that owner's
`ListAcceptedRetentionSettings`; People Directory gives up the consent
lifecycle it kept in `services/users` instead. The policy entry is unchanged —
the decision is that the existing owner keeps the table, not that it changes
hands. `audit.student_consent_changes` and `audit.student_field_edits` stay
with `audit-platform` for the same reason: People Directory decides which of
its fields are tracked and how a change reads, and appends through a
consumer-owned port.

#3349 moved the child's whole lifecycle to its owner, including the write.
`StudentRepository.Update` looked like it crossed a boundary: between the row
lock and the plan write it reconciles the `users.student_companions` edges a
narrowed departure plan no longer allows, takes the far children's row locks for
that, and refuses the write when dropping an edge would strand one of them. It
does not cross one. `database/repositories/users` is already
`people-directory/postgres` in `policy.json`, so that code was always on this
owner's side of the line, reaching Care Plan through a port. Relocating it into
`modules/peopledirectory/internal` changed no ownership; the port is now
`ports.StudentCompanions`, bound to Care Plan's `compose.CompanionRecords` — a
slice of that owner narrow enough to build from the database alone, which is
what lets the directory have it without waiting for a Care Plan that needs the
directory.

Two things stayed shared rather than moving. `departure.StrandingBatch` is the
scope a coordinated multi-child write defers its verdicts into: the owner
decides them, but the caller opens the scope and carries it on its context, so
the type lives in the leaf both sides already import. And
`RecordCompanionChange` stays with the composition seam, because the
`student_companions_changed` announcement travels on the caller's context too.

`database/repositories/users/student.go` issues no SQL at all any more. What
remains is the interface the retained `StudentRepository` still declares, each
method reporting a configuration error when a graph reaches it without the
owner bound — the composition root binds it for every graph, so the shell is
reachable only by a mistake.

The reads moved with the same care as the writes, because three of them are
not what their names suggest: the class lookup matches trimmed and
case-insensitively, the id list keeps graduates because one who is actually
present must stay reachable, and the participation candidates and the class
roster count graduates while every other roster read does not. That last
distinction is now stated as `peopledirectory.StudentScope` rather than left to
each query.

#3350 moves the application the same tables' persistence already left behind.
The care-exit lifecycle ("Betreuung beenden", #2487), the "läuft mit" graph and
the child Dokumente tab were written in `services/users`, which is
`people-directory/application`, although `users.student_care_exits`,
`users.student_care_exit_removals`, `users.student_care_exit_source_removals`,
`users.student_companions`, `users.student_documents` and
`users.care_withdrawal_completions` are all Care Plan's. #2682 moved the
persistence behind `CareRecordsQuery`/`CareRecordsCommand`; the services that
drive it now live in `modules/careplan/legacy/carelifecycle`, together with the
care-exit cleanup repository that used to be
`database/repositories/users/care_exit*.go`.

They could not land on Care Plan's `application` point. The relocated code
still speaks the retained `models/users` rows, the retained audit contracts,
the shared authorization helpers and the Bun database, and PR mode rejects a
new permission on a point that exists at the base SHA — the same reason
#3219's shift-planning services went to `inbound-staff-shifts`/`adapter`. The
package is therefore `inbound-students`/`adapter`, the consumer-side twin of
the former `modules/careplan/legacy/careschedule` (`inbound-schedules`/`adapter`, #3220; dissolved into native Care Plan capabilities by #3351),
and every `inbound-students.adapter.*` rule and every
`<consumer>.<role>.inbound-students-adapter` rule is a compatibility
permission: convert them to exact debt once the package exists at a base SHA,
and dissolve the services into the Care Plan application and domain layers
under #2731.

Two things the move settled rather than carried. The care-exit flow reads two
People Directory tables — the archive lists the children whose enrolment
interval has run out, the binding preview freezes their person rows, and the
booking evaluation reads their enrolment bounds — so those queries are now the
named tenant-safe projection `modules/careplan/legacy/careexitview`
(`care-exit-view`, the one owner this epoch adds). It takes the tenant id
explicitly and joins the Enrollment and Care Plan recordsets its caller
already loaded; the adapter itself issues no SQL. And the three owner queries
the cleanup repository used to receive through `Bind*` setters arrive at
construction instead, as resolvers over the repository factory's own fields:
the composition-surface guard records mutable wiring per package, so a
relocated setter counts as growth (#3219 hit the same wall).
`Factory.BindTimetable` went with them — it only ever forwarded the Timetable
owner to that one repository, which now reads the capability the same graph
hands `NewFactory`.

`services/users` keeps the People Directory half of the child record.
`StudentService` no longer carries the companion graph; `api/students` holds
the companion capability beside it, and the group roster read asks Care Plan
for the dated participation decision through a narrow
`CareParticipationResolver` instead of holding the whole lifecycle service.

#3427 dissolved both packages (epoch 20). The care lifecycle, withdrawal
tasks, booking authority, companions and child documents are native Care Plan:
the public contract in `modules/careplan` (`CareLifecycle`,
`StudentCompanions`, `StudentDocuments`), the rules in `internal/domain`
(the booking-state evaluator, the preview token content, the Go joins that
replaced the jsonb recordset SQL), the orchestration in
`internal/application`, and the directory reads in the Care Plan Postgres
adapter over `student-directory-view`, which grants the same four tables
`care-exit-view` did. The binding preview's roster and booking counts are a
Timetable query (`timetable.CareExitBaselineQuery`), the scheduler's
effect-day pass is the owner command `CareExitCommands.ApplyDueEffects`, and
the document file sweep is `StudentDocumentCommands.SweepStudentDocumentFiles`.
Every read that takes a set of foreign ids pages it by
`careplan.MaxCareExitBatchSize`. The People Directory, Enrollment, Timetable,
Presence, School Calendar and change-history owners reach the lifecycle as
consumer-owned ports bound in `database/repositories/care_lifecycle_owners.go`.
The 46 compatibility permissions and the `care-exit-view` owner are gone; the
two consumers that could only reach the adapter and the moved behavior suites
use the one-time replacement path of
[ADR 0035](../../docs/adr/0035-care-lifecycle-cutover-replaces-legacy-permissions.md).

#2762 cut the execution and the attendance of a block over to Student
Presence (migration 1.15.415, one release with the caller switch). Timetable
& Activities keeps the plan in `schedule.activity_instances` and
`schedule.instance_students`; Student Presence owns `active.activity_sessions`
(status active/completed, live group, actor, timestamps, completion snapshot)
and `active.activity_session_attendance` (status, substatus, note, check-in and
checkout, walk-in and non-booking markers, manual decision, care-plan
provenance). Timetable asks the owner which planned rows already run, ended,
were observed or are not booked through its consumer-owned `SessionFacts`
port (`timetable/compose.NewPresenceSessionFacts`); Student Presence resolves
the participants of its attendance rules (reported day statuses, partial
excusals, block end) through its `PlannedRoster` port, bound in
`database/repositories/presence_bindings.go`. The retained list endpoints
still read one row per block or participant, so the tenant-safe projection
`modules/presenceprojection` (owner `presence-legacy-view`) joins the plan
with the owner rows in one statement each (`ListLegacyInstances`,
`ListLegacyParticipants`, the partial-absence, parallel-presence, course and
manual-planning reads), and `timetable/compose.PresenceReads` wraps it for
the legacy composition. The old execution and attendance columns stay as a
trigger-kept rollback mirror with the `active.presence_compatibility_writes`
counter until #2763; `TestPresenceStorageCallerInventory` keeps every
provider off them. The later-pickup decision that writes to both owners is
bound at the composition root (`api/pickup_extensions.go`) rather than as a
new workflow owner. `database/repositories/student_presence.go`, the old
provider, is gone. The evidence lives in
[presence-cutover-2762.json](presence-cutover-2762.json) and the runbook in
[docs/operations/presence-storage-cutover-2762.md](../../docs/operations/presence-storage-cutover-2762.md).

#2756 cut the student-guardian relationship over to its three owners
(migration 1.15.416, one release with the caller switch). People Directory
owns `users.student_guardian_relationships` (type, role, primary, emergency
contact and priority, payer), Care Plan `users.student_guardian_pickup_permissions`
(`careplan.GuardianPickupPermissions`) and Identity & Access
`auth.guardian_student_access` (account binding and parents-portal
permissions, `identityaccess.GuardianStudentAccess`). The retained
`StudentGuardianRepository` seam is People Directory's relationship store in
`database/repositories/users/guardian_relationships.go`: it writes the
relationship and then the other two halves through consumer-owned ports,
inside one savepoint that holds the relationship row lock, and the legacy
composition binds the ports in `database/repositories/guardian_relationship_owners.go`.
The People Directory parents-portal link writes take the same ports through
`compose.Dependencies.GuardianLinkOwners`. Reads that need the whole row join
the three owners through the tenant-safe projection `modules/guardianlinkview`
(owner `guardian-link-view`), embedded as a subquery so every retained read,
including the Communication audience and inbox projections, keeps its one
statement; those two projections gave up their `users.students_guardians`
grant instead of gaining the three owner tables. The account binding follows
the guardian profile through a migration trigger. The old table stays a
trigger-kept rollback mirror with the `users.students_guardians_compatibility_writes`
counter until #2757, because the previous image links guardians with
`INSERT ... ON CONFLICT`; `TestGuardianStorageCallerInventory` keeps every
provider off it. `database/repositories/users/student_guardian.go`, the old
provider, is gone. The evidence lives in
[guardian-owner-cutover-2756.json](guardian-owner-cutover-2756.json) and the
runbook in
[docs/operations/guardian-owner-storage-cutover.md](../../docs/operations/guardian-owner-storage-cutover.md).

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
scripts/backend-architecture.sh cycles
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

## Cyclic components

A **cyclic component** is a strongly connected component of an owner graph
with at least two owners: every member reaches every other member. It is the
unit this policy measures cycles in. The number of individual cycles inside a
component is not reported, because it grows combinatorially and names nothing
a ticket can remove.

`cycles` reports the cyclic components of the same two owner graphs `diagram`
draws, the target projection and the migration projection. Go forbids package
import cycles, so a cyclic component only appears once packages are collapsed
to their owners. Self-edges and `external:`/`unclassified:` nodes never take
part. `--graph target|migration|both` selects the graph (`both` is the
default; `target` needs no package load), and `--json` prints report schema
version 1 instead of text.

Each component lists its members and every owner edge inside it with the kinds
of both owners. An edge is `target` when at least one target rule permits it,
`baseline` when it exists only through entries of the exact legacy baseline,
and `new` when it exists only through violations the baseline does not hold.
`target` edges name their rule IDs, the others their violation keys. For the
migration graph, `after ratchet` lists what remains of a component once only
`target` edges are kept: those cycles are in the target policy and do not go
away when the baseline reaches zero.

The command only reports. It exits non-zero when the policy, packages, or
baseline fail to load, never because a component exists (#3417).

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

Schema 3 gives each temporary policy rule an `issue` field containing the same
canonical GitHub issue URL as a legacy entry. Presence of this field defines a
temporary permission; descriptions are not parsed by the evaluator. Omit it
for ordinary target permissions. An explicit empty, null, malformed, query,
fragment or pull-request URL is invalid. The issue identifies the open cleanup
ticket, not the relocation that originally introduced the permission.
`audit-issues` checks the union of rule and legacy issues, with one request per
distinct URL. Closed issues, pull requests and failed requests fail the audit.
Issue metadata does not change a rule's permission or policy strictness.

`rules.stale` reports a rule that allows no edge across the union of production,
internal-test and external-test scopes in the fixed build context. A rule used
only in one test scope is live. Each unused rule produces one finding whose
source and target are its rule ID and whose scope is empty (policy-wide).
Delete it in the same change: `rules.stale` can never enter `legacy.jsonl`.
An overlapping match is not an allowed edge and cannot keep a rule alive.

The reviewed [#3416 backfill](rule-backfill-3416/README.md) preserves the
mechanical proposal separately from the human-approved cleanup assignments.
It removed 167 stale rules from 2,040, assigned 485 live temporary permissions,
and left all 682 legacy entries byte-identical. These are rollout measurements,
not a replacement baseline or a permission to add debt.

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

### Relocating a package without losing its debt

A violation is identified by `scope|rule|source|target`, so moving a package
renames every key it appears in, in both directions, although no import, owner
or role changed. [ADR 0020](../../docs/adr/0020-a-relocated-package-keeps-its-debt.md)
lets a reviewed epoch declare that move in the optional `relocations` array,
sorted by `from`:

```json
{"from":"models/auth","to":"modules/identityaccess/legacy/authmodels","issue":"https://github.com/moto-nrw/project-phoenix/issues/3226"}
```

PR mode then reads the base baseline at the candidate's paths. A declaration is
active while `from` is still classified in the base policy; afterwards it is a
record of which path became which, and it cannot replay. An active declaration
must prove the epoch increased, that `from` is classified in the base policy
and gone from the candidate while `to` is classified in the candidate and
absent from the base, that owner and all three roles are identical on both
sides, and that `to` holds exactly the Go files `from` held at the base commit.
The file set comes from Git, not from the declaration, so a move that adds,
drops or renames a file is a rewrite and keeps the ordinary guards. It compares
names, not contents, which it cannot do because a relocation rewrites import
paths and may rename the package clause. It is a sanity gate; the safety
property is the unchanged violation-key set below.

The declaration renames keys and grants nothing else. An added import has no
renamed twin and stays a new violation, a changed migration issue is still a
reassignment, and a base entry whose import disappeared is still stale. Rule
anchoring does not read the declaration: whether a package counts as
candidate-created still comes from Git, as for any package the candidate
writes. Prefer a relocation over compatibility permissions whenever a package
moves unchanged — it keeps each consumer's debt under the consumer's own
migration issue instead of dropping it and re-deriving it later.

#3226 moves `auth/jwt`, `models/auth` and `database/repositories/auth` into
`modules/identityaccess/legacy/jwt`, `.../legacy/authmodels` and
`.../legacy/authpostgres` this way (policy epoch 13 to 15), with no rule, owner
or role change. They stay outside the module's `internal` tree because
`database/repositories`, `services/users`, `models/users`, `api/auth`, `cmd`
and the retained parent-portal and delivery packages still import them; that
debt is recorded against #2727 through #2751 and falls with those tickets, at
which point the packages can join `internal/domain` and
`internal/adapters/postgres`. The typed-root composition inventory only scans
`api`, `cmd`, `services`, `database/repositories` and `test`, so the moved
repository tests' factory calls left `composition.json` with the move, as with
#3218 and #3219; they are not fewer callers, and the composition surface count
is unchanged at 673.

#3230 moves `api/auth` to `modules/identityaccess/inbound/auth` the same way
(policy epoch 18 to 19), keeping the `inbound-auth` owner and the `http` /
`adapter-test` roles. Of its 29 baseline keys, 26 keep their #2736 issue under
the new path. Three fall with the move: the tenant shell reads
`operations.parent_notes_enabled` through its own fail-open resolver instead
of `services/parentmessaging`, it holds the `grade_level_max` bounds 1..13
itself instead of importing `internal/schoolclass`, and the `*bun.DB` handle,
which only switched the invitation routes' transaction on, became the
consumer-owned `UnitOfWork` port (`TenantUnitOfWork` in production, nil in
handler unit tests). The RBAC routes open `tenant.WithinAdmin` directly, as
before. Moving into `modules/` also puts the package under the Rule 16
ratchets, so the rate-limiter, MFA and passkey setters gave way to
`RouterWithAuthRateLimiter` and field assignment at the composition root
(composition surface 641 to 638), and the router and tenant-resolve handlers
were split into named steps. Routes, methods, status codes, error bodies and
middleware chains are unchanged; the middleware golden only renames
`requirePlatformScope`'s package.

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

The staff-shift, shift-type, assignment, Dienstplan overview, staff-notice,
shift-plan sync and substitution services that `services/schedule` used to
hold spent #3219 to #3418 as the `inbound-staff-shifts`/`adapter`
compatibility package `modules/workforce/legacy/shiftplanning`: 5,336
production LOC no ratchet could point at, hidden behind 37
`inbound-staff-shifts.*` rules and 7 rules naming that point as a target.
#3418 dissolved the package into its owners
([ADR 0037](../../docs/adr/0037-shift-planning-cutover-replaces-legacy-permissions.md),
policy epoch 21 to 22). The shift, series, move, lock, planning facade,
shift-type, assignment and overview services are Workforce commands and
queries in `modules/workforce/internal/planning` (`workforce`/`application`),
bound by `modules/workforce/compose` behind the public `StaffShiftPlanning`,
`ShiftTypeAdministration`, `StaffAssignmentQuery` and
`StaffScheduleOverviewQuery` contracts; the retained `models/schedule`
Dienstplan repository contracts and their adapter in `database/repositories`
are deleted, the planning package declares its own row ports, and the
compose package serves the retained rows to the Timetable coverage probe,
the staff calendar feed and the plan export through `ShiftRows` and
`ShiftTypeRows`. The Tagesinformationen service and route are Timetable's,
whose write ownership of `users.staff_notices` and `users.staff_notice_acks`
was already recorded: the service sits beside the native repository in
`modules/timetable/compose` behind the public `timetable.StaffNotices`
contract, the route in `modules/timetable/compose/httpadapter`. The #1843
sick cascade and the schedule substitution moves cross the Workforce and
Timetable line inside one tenant transaction and are the application
workflow `workflows/shiftplansync` (owner `shift-plan-sync`, kind
`workflow`): the absence service reaches the cascade through the port
Workforce declares publicly (`workforce.ShiftPlanSync`), the substitution
module through the port `services/education` still owns, and the two
composition bridges in `services` are gone. No HTTP path, status code, error
string, authorization check, tenant scoping, coverage-conflict or
substitution semantics changed. The 44 compatibility rules are deleted, not
converted; `inbound-staff-shifts.to.workforce` is the owner's only rule and
`api/staff-shifts` its only package. `legacy.jsonl` is unchanged: the
package never had a key, which is what the ticket set out to repair. The
Dienstplan rows still speak the retained `models/schedule` structs inside
the planning package, because the Timetable coverage probe and staff pool in
`modules/timetable/legacy/timetableplanning` share that vocabulary; they go
with #3424, together with the shift-coverage interval vocabulary
(`shift_coverage_intervals.go`) the overview binds through its own aliases.

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
them (`modules/careplan/legacy/careschedule` from #3220 until #3351 dissolved it) for these and for
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

`services/auth`, `services/auth/authtest` and `services/platform` no longer
exist (#3364). What they held last were the retained consumer ports over
Identity & Access: the capability interfaces, value types and error sentinels
the HTTP surfaces consumed because the policy does not let them name the
module. Every consumer now either calls `modules/identityaccess` directly
(`api/auth`), reaches the owner's own seam (`api/operator` through
`modules/identityaccess/inbound/operator`), or consumes a plain-typed runtime
the composition root binds (the school and parents portals, the staff
offboarding workflow), so no status code, error string or wire shape moved.
The credential primitives the compositions hand the module — the Argon2id
hash, the password policy and the entropy source — are Security Runtime's
(`auth/authorize/credentials.go`, served through `modules/securityruntime`);
the school-role lookup is `authmodels.ResolveSystemRoleByName`; the
"no row" classification is `models/users.RowMissing`. The module binds the
unit of work it opens its own transactions under through
`identityaccess.Module.SetTenantRuntime`, which the legacy factory's existing
runtime wiring calls. The facade does not name the runtime package for it:
`identityaccess.TenantRuntimeBinding` takes the unit of work as an opaque
value, `modules/identityaccess/compose` supplies the `tenant.RuntimeRef`
behind it and the root supplies the runtime, each naming `tenant` where it
already may. The type check therefore sits in the root, which owns the
runtime, and a value it cannot bind fails the server's startup. The binding
replaced the retained service's own runtime field and setter, so the
composition surface shrank by two and the cutover needs no production rule.

The behaviour suites of both packages moved with their subject into
`modules/identityaccess/behavior` (`identity-access`/`test-support`,
`e2e-test` in both test scopes), retargeted to the module's contract. The
point is deliberately one no other Identity & Access package occupies, so the
`identity-access.behaviour.*` rules widened nothing else; they existed only
because the suites drive the composed service root. #3446 removed all of
them. The suites now pin the rows other owners persist for the identity flows
by their stored values, read straight from the tables, and reach the
Security Runtime hash, the Delivery Platform mail contract, the outbox
capture and the guardian link presets through the neutral `test` support;
their fourteen cross-owner implementation edges are gone. The four edges the
legacy factory still forces on an `e2e-test` root — the module's own public
contract, its token adapter `modules/identityaccess/legacy/jwt`, the refresh
rotation application `auth/rotation` and the tenant runtime `tenant` — are
exact `legacy.jsonl` debt under #3446 and go when the suites drive the module
without the legacy factory. The school-portal router
suites bind their runtimes through `modules/schoolportal/portaltest`
(`inbound-school`/`test-support`), the owner's own test support, instead of
naming the legacy composition from the suite; its three rules are anchored to
that new point for the same reason.

The Workforce time-tracking HTTP composition
(`modules/workforce/inbound/timetracking`) is classified `workforce`/`http`
under the same compatibility permissions (#2690). It replaced
`api/time-tracking` and serves the public work-session, absence, month,
ledger, month-close, overview, audit-log, export and personnel-record
contracts of `modules/workforce`, which the root composition adapts from the
retained time-tracking services in `modules/workforce/legacy/timetracking`
and the retained `services/users` services. Its own-shift and assignment
routes read the public `OwnShiftQuery` and `StaffAssignmentQuery` contracts
the Workforce planning composition serves (#3418); the staff-notice route
moved to Timetable with its service. The kiosk staff
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

`users.care_withdrawal_completions` is persisted by its `care-plan` owner
alone since #3221: the statements moved from `database/repositories/users`
into `modules/careplan/internal/adapters/postgres` (`withdrawal_completions.go`)
and reach consumers through the public `WithdrawalQuery`/`WithdrawalCommand`
capability. `database/repositories.careWithdrawalCompletionRepository` serves
the retained `models/users` contract over it without persistence of its own,
and supplies the children's names and classes, which the queue queries used to
join from `users.students` and `users.persons`, as a People Directory
recordset. The care-exit cleanup's booking-expiry query no longer reads the
table either: it filters its grouped rows against the owner's completion keys,
which the former `NOT EXISTS` clause tested on the same grouping key. Both
#2727 baseline entries for the table are gone.

The Care Plan compatibility adapter (`modules/careplan/legacy`, root package)
used this representation from #3032 until #3410 dissolved it and removed its
14 exact imports. The care-offering and offering-change repositories are now
the enrollment services' own translation onto the Care Plan Commands and
Queries (`services/enrollment/care_plan_offering_records.go`), with the name
search bound to the People Directory by the services composition. The
companion and child-document repositories sit with their sibling Care Plan
adapters in `database/repositories`, as constructors instead of
`repositories.Factory` fields. The calendar day comes from
`modules/careplan/compose.Today`, the ambient transaction from
`compose.TenantAmbientDatabase`.
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
common package moves and the calendar-date edge with the retained
`internal/timezone` type. Convert them to exact debt with the rule above
once the package exists at a base SHA. The settings test edge is gone (#3447):
the tests name the setting keys through the public `modules/settings`
contract, which now also exports the four tracking-indicator keys.

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
already existed and can admit no new permission. #3422 dissolved all four
packages; `modules/workforce/legacy/timetracking` now names the public Student
Presence session records and the Workforce rows of
`modules/workforce/adapters/timerecords`.

#3422 dissolves the nest in slices. `legacy/statistics` became
`internal/application/statistics`; the Workforce and Care Plan rows left
`legacy/models/active`; and `legacy/services/active` moved into
`modules/studentpresence/internal/application/presence`
(`student-presence`/`application`, `adapter-test` in the package, `e2e-test`
for the behaviour tests). Consumers call it through the small public facades
in `modules/studentpresence/presence.go` (`SessionReads`, `KioskSessions`,
`SessionCommands`, `VisitReads`, `AttendanceCommands`, the status-day,
history and cleanup facades, and the wiring composite `Presence`), built by
`modules/studentpresence/compose/presenceservice`. That compose package is
separate from `modules/studentpresence/compose` because the shared `test`
fixtures import the latter, and the application's in-package tests import
`test`.

The nest is gone. `legacy/models/active` and `legacy/repositories/active`
followed: the session and supervision working types are plain values in
`modules/studentpresence/internal/ports` and the repository bodies sit in
`internal/adapters/postgres` over two record ports the native
`internal/application.Service` satisfies, so reads and writes keep the
validation, transaction and observation path they took through the module
before. Callers outside the engine use the public `SessionRecords` and
`SupervisionRecords` facades; the composition root binds them through
`compose.NewSessionRecords` and hands the internal repositories to the
presence application with `compose.SessionRepositories`. The only BUN mapping
of the two session tables that remains is the fixture package
`modules/studentpresence/sessionrecordstest` (`student-presence`/`test-support`),
which keeps `TestDateColumnTypes` mapping `active.group_supervisors`. No Go
file imports the nest, `policy.json` classifies no package under it, and no
rule names `student-presence`/`adapter` or `student-presence`/`domain`: 96
such rules when the ticket was written, 79 at the merge base, 0 now, none of
them converted to debt. `legacy.jsonl` is unchanged at 624 entries; the nest
never had one, which is what the ticket set out to repair. The consumers'
replacement permissions are the 57 rules of the one-time, epoch-gated
exception in [ADR 0036](../../docs/adr/0036-student-presence-cutover-replaces-legacy-permissions.md)
(policy epoch 20 to 21), and the `legacy` budget entry for
`modules/studentpresence/legacy` is deleted rather than lowered.

#3352 was the students-inbound half of that dependency: `api/students`
imported the nest's services in 12 production files and its models in 8.
#3422 switched those 16 files to the public contract while deleting the nest,
so the status days (`StatusDays`, `StatusDayOverviews` with
`ErrStudentStatusDayPartialAbsenceConflict`), the presence history
(`StudentHistory`) and the attendance and visit reads already ran through
`modules/studentpresence` when #3352 landed. What #3352 changed is the shape
of the remaining binding: `api/students` no longer takes the wiring composite
`Presence` but its own port `students.StudentPresence`: the thirteen reads
and commands the student list, day planning, visit and school check-in
handlers call, eleven declared on the port and two inherited from the
embedded `common.StudentLocationReader`, the four-method port the shared
location snapshot in `api/common` reads. The composition root still
hands over the public capability; compile-time assertions keep both ports
satisfied by it. No key, rule or baseline entry changed; the two remaining
`inbound-students.*.student-presence-*` rules are target permissions on the
`public` and `compose` roles, not compatibility.

The timetable planning nest `modules/timetable/legacy/timetableplanning`
(`inbound-timetable`/`adapter`, #3424) dissolves in slices. Slice S6 moved the
calendar-period administration (name uniqueness, the same-type overlap
invariant, the recurrence gate and the care-offering guard), the tenant's
non-working days, the A/B-week engine (`schoolcalendar.WeekPatternApplies`)
and the dateframe routes of the schedules HTTP composition to the School
Calendar owner; `api/timetable`, the schedules composition, the Workforce
planning commands, Enrollment and the timetable e2e suite consume the public
`schoolcalendar.Calendar` contract. The three compatibility rules the slice
emptied are deleted, not converted; the seven replacement permissions are the
epoch-gated exception in
[ADR 0038](../../docs/adr/0038-timetable-planning-dissolution-replaces-legacy-permissions.md)
(policy epoch 22 to 23), which every later slice of #3424 reuses and which
ends when the last slice removes the package from the base.

The import HTTP composition (`modules/dataimport/inbound`, with its runtime
binding in `modules/dataimport/inbound/compose`) keeps the `inbound-import`
owner and its `http` / `compose` roles after replacing `api/import` (#3217).
It serves the student, staff, class-list and opening-balance preview, import
and template routes from the public Data Import contract with unchanged paths,
status codes, error strings, multipart handling, file-size limit and
permission checks. The `root-composition.to.import-http` mount replaces the
`api -> api/import` debt entry that fell with the old package; it and the
`inbound-import.compose.*` rules that bind `api/common`, `modules/identityaccess/legacy/jwt`, `tenant`
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
a new permission on a point that exists at the base SHA. The 46 compatibility
permissions this move needed (the `inbound-usercontext.adapter.*`,
`inbound-usercontext.http.*` and `inbound-usercontext.adapter-test.*` rules,
every `<consumer>.<role>.inbound-usercontext-adapter` rule and the
`root-composition.compose.inbound-usercontext-http` mount) are converted: the
rules are deleted and their 55 import tuples are exact debt under #2725. The
baseline grew by those 55 entries without a new import; the edges were hidden
behind allow-rules before. The `student-presence` domain and adapter rules of
the same packages stay temporary under #3422. The tuples fall when the adapter
dissolves into the Identity & Access application and public contract, which
first needs owner contracts for the staff and teacher membership read
(`school-membership`), the caller's groups and substitutions
(`school-structure`) and the supervised and active groups
(`student-presence`). The adapter role also
covers the `net/http` status constants the SSE setup error carries through
the generic `external.http-router.adapter` rule, and the former
`inbound-usercontext.to.identity-access` target rule is removed until that
contract exists.

#3501 dissolved both packages; all 55 #2725 entries fell without a new one.
The read side is the Identity & Access caller context: public ID-based
contract types and sentinels in `modules/identityaccess` (`CallerContext`,
`RequestIdentityCache`), use cases in `internal/application` (`caller_*.go`,
`parent_request_review.go`) over their own `internal/ports`, and
`compose.NewCallerContext`, whose public-typed seams the legacy composition
binds in `services/caller_context.go`: People Directory rows for the person,
the School Membership facade for staff and teacher (#3498), the School
Structure staff group reads (#3499), the Student Presence supervised and
live-group reads (#3500), and the retained activity repository for the
planned supervisions, which no Timetable contract serves yet. The `/api/me`
routes moved to `modules/identityaccess/inbound/me`, a new package on the
existing `identity-access`/`http` point, so the root mounts it under
`root-composition.to.identity-access-http` and no rule was added; the tenant
transaction the writes ran in is the use case's own. Consumers that are
forbidden the public package declare their own port; the legacy composition
serves them `database/repositories.CallerRows`, which resolves the reach
through the caller context and loads only the retained rows a consumer still
renders. The `inbound-usercontext` owner and its four `student-presence`
rules are gone. The request identity memo slot lives in the session adapter
(`legacy/jwt`), the one Identity & Access package `api/common` and the
legacy composition already reach, so `api/common` keeps attaching it
router-wide and group-wide as before and the root binds it as the caller
context's memo; the memo's contents stay in the application.

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
their compatibility adapters are deleted; the operator passkey records
followed under #2724. The operator invitation and e-mail change links (#2722):
`platform.operator_invitation_tokens` and
`platform.operator_email_change_tokens` are read and written only through the
public `OperatorTokens` capability, with the same platform-wide transaction
rule. The capability decides expiry with one clock: it never stores a link
that is already expired or used, never extends one that expired meanwhile,
and redeems a link exactly once. The retained `services/platform` invitation
and e-mail change flows keep their orchestration and reach the rows through
the `OperatorInvitationTokens` and `OperatorEmailChangeTokens` ports the root
binds to the public module; invite, accept and confirm stay one
administrative transaction the module operations join. The operator password
change revokes pending e-mail change links inside the module, so the former
root-supplied credential cleanup seam is gone. The `models/platform` token
repository contracts, their `database/repositories/platform` adapters and the
`repositories.Factory` fields are deleted; the token models remain as the
retained ports' value types. The school invitations, the password reset
and the guardian invitations (#2722) follow the same shape:
`auth.invitation_tokens`, `auth.password_reset_tokens` with its rate-limit
window, and `auth.guardian_invitations` are read and written only through the
public `SchoolInvitations`, `PasswordResets` and `GuardianInvitations`
capabilities, and the account an acceptance provisions is written there too.
Expiry is decided with one clock: a link is never stored already expired or
spent, a resend never revives one, and redemption spends it exactly once. The
retained `services/auth` invitation, password reset and guardian invitation
services keep their contracts and reach the rows through the consumer-owned
ports the root binds to the public module; the acceptance holds one
administrative transaction, so the account, its school mapping, the role, the
identity chain and the spent link commit together. The mail stays in the
composition root, which knows the portal hosts, the tenant reply-to identity
and the e-mail outbox, and the enrollment requests a guardian acceptance
claims arrive through their own port. Every reader of `auth.guardian_invitations` goes
through the owner: the People Directory guardian list and the parents portal
related-accounts view consume their own record types over the public
capability, so the `modules/identityaccess/legacy/authmodels` guardian invitation contract, its
`modules/identityaccess/legacy/authpostgres` adapter, the `repositories.Factory` field and
the People Directory's deprecated invitation twin are deleted with the
retained token lookup, approval-queue and acceptance methods. The operator MFA
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
the MFA models remain as the retained port's value types. The operator
passkey records (#2724), `platform.operator_passkey_credentials` and
`platform.operator_passkey_sessions`, follow the same shape through the
public `OperatorPasskeyRecords` capability. The retained `services/platform`
operator passkey service keeps the WebAuthn ceremonies, the origin check and
the token exchange and reaches the rows through the `OperatorPasskeyRecords`
port the root binds to the public module. A ceremony completes at most once,
in one administrative transaction with the credential insert or use record
the module operations join: a refused ceremony commits and stays spent, a
failed read or write rolls back and leaves the ceremony open. Only the
owner's not-found outcomes become an invalid session or a missing passkey,
so store failures reach the caller as errors. The `models/platform`
passkey repository contracts and their adapter are deleted; the passkey
models remain as the retained port's value types. The school-portal passkeys
(`auth.passkey_credentials`, `auth.passkey_sessions`) follow the same shape
through `AccountPasskeyRecords`: the retained `services/auth` passkey service
keeps the ceremonies, the tenant-origin check and the school-membership gate
and reaches the rows through its own `PasskeyRecords` port. A credential
belongs to the account and carries no school; the ceremony records the school
whose portal started it, and login refuses a school the account has no access
to. Their `modules/identityaccess/legacy/authmodels` models and `modules/identityaccess/legacy/authpostgres` adapter are
deleted; the port's value types live in `services/auth`. The foreign
`auth.accounts` reads of the People Directory, Care Plan parent and CLI
packages use owner queries bound the same way (`identity_ports.go`): the
account lookup and active-account subquery of `modules/identityaccess/legacy/authpostgres`
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
`authmodels.TokenRepository` contract and its compatibility adapter are
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
membership locks the mutations serialize on use module-owned persistence in
`internal/adapters/postgres`. Account visibility reuses the module's account
administration, preserving tenant and organization checks and the lock order.
#3226 removed the root role adapter and `compose.RoleDirectory` seam; other
legacy consumers still need migration before the old repositories can be
deleted. The school-role
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

The non-identity operator handlers left `api/operator` for their owners
(#3232), moved file for file with their adapter tests: provisioning and its
summaries to `modules/organizationtenancy/inbound/operator`
(`organization-tenancy`/`http`), the school settings routes to
`modules/settings/inbound/operator` (`settings-platform`/`http`), and the
announcement routes to `modules/communication/http/operatorannouncements`
(`communication`/`http`), all with `adapter-test` in both test scopes. No
route path, status code, error string or authorization check changed.
`api/operator` keeps the router with its middleware chain, builds these
resources from its configuration and mounts them
(`inbound-operator.http.organization-tenancy-http`,
`inbound-operator.http.settings-platform-http`,
`inbound-operator.http.communication-http`). The operator error body and
the operator-audited id action live in `api/common` (`OperatorErrResponse`,
`OperatorAuditedIDAction`), so every half of the operator surface renders
one wire format; #3231 removed the thin `Err*` delegations `api/operator`
kept until then.
The three packages are the only packages of their points, which exist only
in the candidate. The owner rules `organization-tenancy.http.public`,
`settings-platform.http.organization-public` and `communication.http.public`
are the target shape (an inbound adapter calling a public capability).
Every other `organization-tenancy.http.*`,
`organization-tenancy.adapter-test.*`, `settings-platform.http.*`,
`settings-platform.adapter-test.*`, `communication.http.*` and
`communication.adapter-test.*` rule is a compatibility permission for the
retained services, rows, token claims, calendar date, tenant runtime and
ORM the handlers still speak, including the `internal/timezone` edge the
ticket names. #2736 removed those imports instead of converting them (see
below). The operator settings hook is construction-time configuration
instead of a setter, because the composition surface guard records mutable
wiring per package; since #2736 it is `OperatorDependencies.OnValueSet` of
`modules/settings/compose`.

The review of unregistered RFID scans could not join a candidate-only
point: `device-fleet`/`http` already exists (`api/iot/devices`,
`modules/devicefleet/deviceauth`), so PR mode admits no new permission for
it, and `inbound-operator` may not import that point. Its handlers in
`modules/devicefleet/inbound/operator` therefore read and resolve scans
through the public Device Fleet capability, label them through a
consumer-owned school directory, and take the operator surface (error
bodies, response envelope, authenticated operator, resolution fallback) as
plain functions. The root composition binds the directory to Organisation &
Tenancy (`api/school_directories.go`) and hands the built router to
`api/operator`, which mounts it as a plain handler. The wire shape of a scan,
the school and Träger narrowing and labelling, and the administrative
transaction are unchanged. The retained `services/audit` review path
(`ListForOperator`, `Resolve`) and its repository methods are deleted; the
service keeps recording and expiring scans.

The identity half of `api/operator` moved to
`modules/identityaccess/inbound/operator` (`identity-access`/`http`, #3231):
the operator second factor, the passkey ceremonies (`PasskeyResource`), the
invitations, the profile read and e-mail change, the school-account MFA
administration, the operator scope and active-operator middleware and the
authentication and profile error mappings, next to the login, refresh,
profile and school access routes #3252 moved. No route path, status code,
error string or authorization check changed; the middleware golden only
names the new package. `api/operator` keeps `api.go` and the package: it is
the only point the policy lets mount the Organisation & Tenancy, Settings
Platform and Communication operator routes
(`inbound-operator.http.*-http`), and PR mode admits no new permission for
the root or for an existing `identity-access` point. For the same reason
the handler tests that mint operator claims (`legacy/jwt`) or map the
retained `models/platform` rows stay in `api/operator` as its adapter tests
and drive the moved handlers through their exported constructors
(`inbound-operator.adapter-test.identity-access-http`). The module keeps
the tests of its unexported mappings and guards and the rendered-body tests
of `AuthErrorRenderer`, none of which name `legacy/jwt`, `api/common` or
the retained rows. The school-scoped
MFA admin writes now take their tenant context in
`modules/identityaccess/compose` (`identity-access.compose.tenant-runtime`)
instead of the HTTP adapter, which removes the
`production|api/operator|tenant` debt.

#2736 closed the carrier of `api/auth` and `api/operator`: all 37 keys it
still held, the root's `api -> modules/identityaccess/inbound/auth` key
and every rule that named #2736 are gone (624 -> 586). No route path, status
code, error body or authorization check changed.

- The account routes left the `inbound-auth` point. PR mode rejects an
  owner change of an existing package, so the files moved file for file to
  the new package `modules/identityaccess/inbound/account`, classified
  `identity-access`/`http` like `inbound/me` and `inbound/operator`; the
  `inbound-auth` owner, its one rule and the #3230 relocation record are
  deleted. The session adapter, `api/common` and the refresh rotation are
  target dependencies of that point (ADR 0031). The rest became
  consumer-owned ports the root binds: `Caregivers` (People Directory's
  caregiver capability as rendered JSON with behaviour-classified errors,
  `services/users.CaregiverCapabilityViews`), `RoleGrants` (Security
  Runtime's grant decision, `services.RoleGrantPolicy`), the extended
  `UnitOfWork` (`modules/identityaccess/compose.TenantUnitOfWork`, handed
  out by `services.AccountRouteTenantRuntime`), the demo switch as a bool,
  and the tenant shell's reads through the Settings Platform contract. The
  driver's no-rows error became `identityaccess.ErrRecordMissing`, marked
  in the module's composition with the store's text kept.
- Settings Platform gained its public contract `modules/settings` (the
  setting keys other owners read, `TenantReader`, and
  `OperatorSchoolSettings`) and the composition `modules/settings/compose`,
  both new points with rules anchored to them. The operator school settings
  routes call only that capability; the tenant transaction, the side-effect
  hook, the presence-mode guard and the booking-authority preview live in the
  composition. The root binds the broadcast and the open-attendance check
  from `services`, so `settings-platform.http.delivery-public`, the
  settings → delivery edge ADR 0032 names, is deleted without a
  replacement.
- The Organisation & Tenancy provisioning routes take the caregiver
  capability as the same kind of port, run the seed-token gate through
  `api/common`, and classify a failed identity store by the new
  `organizationtenancy.ErrIdentityStoreFailed`, which `services` sets on
  errors carrying `StoreFailure()` (`models/base.DatabaseError`).
- `api/operator` is now the operator router only. The session chains are
  Identity & Access's (`identityoperator.Sessions`), and the review of
  unregistered RFID scans maps its own refusals in Device Fleet.
- The `inbound-common` and `token-claims` rules of the Organisation &
  Tenancy, Settings Platform and Communication HTTP points and their adapter
  tests are target rules now, as they are for the eleven other owner HTTP
  points (ADR 0031). Every other #3232 compatibility permission of those
  points is deleted.
- Tests reach claims, the test module wiring and raw setting rows through
  `api/testutil` and `test`, which already held those imports. One
  assertion could not follow: no test role that may import `auth/rotation`
  may drive the operator refresh route, so the forwarding of the
  `X-Refresh-Recovery-Proof` header is no longer asserted
  (`api/operator/auth_test.go`, marked as a coverage gap).

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
[ADR 0022](../../docs/adr/0022-parent-portal-is-an-application-workflow.md))
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

The retained parent services that #3228 had moved into
`workflows/parentportal/legacy` (`parent-portal`/`adapter`) are dissolved
(#3420). Their coordination moved into `workflows/parentportal/care`, the
business writes onto owner commands, and the package is gone together with
the `parent-portal`/`adapter` and `adapter-test` points and their 43
compatibility rules, plus the three consumer rules
`inbound-parent.http.parent-portal-adapter`,
`inbound-parent.adapter-test.parent-portal-adapter` and
`legacy-composition.compose.parent-portal-adapter`. None of them became exact
debt; `legacy.jsonl` is unchanged. The stale
`parent-portal.application.orm-sql` rule fell as well: no parent-portal
package imports Bun any more.

Every parent-portal write is now an owner command called inside one tenant
unit of work that the flow opens from the request context, after the
relationship's `parent_portal.*` check with the same permission constant as
before. Care Plan reports guardian absences (`GuardianAbsenceReports`, with
the manual partial-absence guard) and writes the guardian leg of a day's
pickup exception together with its derived excusal
(`GuardianPickupExceptions`), both constructed in `modules/careplan/compose`;
the Stammdaten requests use its existing request commands, and its care
profile commands (`StudentProfileCommands.SetStudentLiveStatus`,
`SetStudentHealthInfo`) write the child's live absence flags and health
information in `users.student_care_profiles`. People Directory writes the
guardian profile, phone and relationship rows and the child's photo consent
(`GuardianPortalCommand`, `StudentPortalCommand`), each tenant-scoped; the
portal language is written once per school the account has a profile in, so
no parent-portal write of the child flows runs in an administrative
transaction any more. Audit Platform appends the guardian change trail
(`GuardianChangeLog`, `modules/auditlog/compose.NewGuardianChangeLog`) over
the ambient transaction. The child flows hold consumer-owned read ports and
owner-command ports only, and no `*bun.DB`: `tenant.WithinTenant` and
`tenant.WithinAdmin` take the unit of work from the context, which the
database argument of `tenant.WithTenantTx` never reached.

`workflows/parentportal` (`parent-portal`/`port`) is the workflow's driving
port: the vocabulary the guardian portal HTTP composition matches and the
composed `Portal`, which promotes the methods of both flow packages. It is not
a `public` contract, because that vocabulary still carries retained model
types that the semantic contract checks reject; a clean public contract needs
the wire DTOs rewritten. `workflows/parentportal/compose`
(`parent-portal`/`compose`) binds the owners and the platform (meal plan, live
events, language catalog) to the flows' ports, and the HTTP composition
declares its own `PortalService` port. The `parent-portal.compose.*` rules to
the retained read models (`models/users`, `models/parent`, `models/schedule`,
`models/enrollment`, `models/active`, `services/config`, `services/enrollment`,
`services/users`, `services/parentmessaging`) are compatibility bindings that
fall together with the `parent-portal.application.*` ones when the flows read
through owner queries. The portal behaviour tests moved with the flows to the
workflow root (`workflow-decision-test`), because the existing
`workflow-integration-test` point of `care` and `messaging` admits no new
permission in PR mode. That test point has no owner-kind rules to lean on,
so its `parent-portal.decision-test.*` rules name each target; together with
the compose, port and consumer rules the policy holds more rules than before
although the 46 compatibility rules of the adapter point are gone.

Still open: `workflows/parentportal/messaging` keeps writing announcement
read, acknowledgement and poll-response rows through the retained announcement
repository inside an administrative transaction, as before #3420, and still
holds Communication's retained repositories. That is Communication's side of
the portal and needs its own owner commands.

The guardian portal HTTP composition (`modules/careplan/inbound/parent`,
#3229) replaced `api/parent`, moved file for file with its adapter tests. It
keeps the `inbound-parent` owner and the `http` / `adapter-test` roles, so the
36 `legacy.jsonl` entries #2735 recorded for `api/parent` fell with the path,
together with the root mount (#2750) and the calendar end-to-end import
(#2748). No route path, method, status code, error string, authorization check
or tenant scoping changed. Besides the `care-plan` capability that
`inbound-parent.to.care-plan` names, the handlers still call the retained
auth, enrollment and users services, the parent-portal workflow port and the shared
HTTP helpers directly, and match their result types and error values:
routing those calls through the `modules/careplan` contract would add imports
to the existing `care-plan`/`public` point, which PR mode rejects. The
care-schedule diff row is named through the parent-portal workflow port
(`CareRequestDiffEntry`), so the production `services/schedule` import fell
with the move; the adapter tests still build that vocabulary. After the move the package
is the only `inbound-parent` package, so the point exists only in the
candidate. The `inbound-parent.http.*` and `inbound-parent.adapter-test.*`
compatibility rules this move added are converted (#3421): the 27 rules are
gone from `policy.json`, and the 29 imports they allowed (12 production,
11 internal-test, 6 external-test) are exact `legacy.jsonl` entries under
#3421, which closes when the last of them falls. `root-composition.to.inbound-parent-http` (#2750) and
`test-support.e2e-test.inbound-parent-http` (#2748) are still compatibility
permissions; convert them to exact debt with the rule above under their
issues. The `modules/identityaccess/legacy/jwt` and `services/auth` edges wait for #2725. The move replaced
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
`services/users`, the Care Plan `carerequests` contract (#3351), `services/enrollment` review queues,
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
The wrapper supplies the committed baseline; the policy defaults to
`architecture/policy.json` and can be selected with `--policy`. Callers must provide `--api-url`,
and `GITHUB_TOKEN` is optional for authenticated requests. A GitHub or network
error fails this audit and cannot change the deterministic `check` result or
appear as a green audit.

## Migration evidence

Migration records use schema 3; checkpoint records retain schema 2. Convert
older migration records explicitly. Both kinds retain prerequisites, owner/capability,
packages, tables, exact ratchet keys, atomic cutover, tests, rollback/cleanup,
and measurable exit criteria. Unknown fields and blank required evidence fail.

### Ordinary waves

Copy `migration-ticket-template.json` and replace all example/guidance values.
Set `ticket_kind` to `migration`. `checkpoint_reference` must be the canonical
issue URL of the latest accepted checkpoint in `runtime-checkpoints.json`.
Missing, malformed, future, unaccepted, and superseded references fail.

Record flow-specific evidence, not an independent rerun of the checkpoint
benchmark suite. `runtime_evidence.source`, `workload` and `thresholds` describe
provenance, environment/observation window and acceptance criteria.
`runtime_evidence.sources` lists explicit repository-relative file paths or
HTTP(S) references. Local files must exist and resolve inside the repository;
the validator does not fetch remote references.

Every remaining evidence field is required: `affected_rows`, `query_count`,
`latency_p50`, `latency_p95`, `errors`, `pool_wait`, `lock_wait`, `deadlocks`,
`job_duration`, `job_retries`, `job_backlog`, `failure`, `rollback` and `smoke`.
Each has one of these shapes:

```json
{"state": "measured", "result": "0 unexpected errors in 30 calls; raw samples in the declared source."}
```

```json
{"state": "not_applicable", "reason": "The migrated read capability has no worker path."}
```

Observed qualitative results are valid. Planned instrumentation and structural
query estimates are not measurements. Bare placeholders and unchanged template
example values fail. CI validates representation, not the truth of observations
or adequacy of reasons; those remain review responsibilities. Keep deployment
rollback and cleanup in `rollback_and_cleanup`.

[ADR 0026](../../docs/adr/0026-migration-evidence-gate-preserves-provenance.md)
allows a third state, `historical_gap`, only for exact existing files and metrics
frozen at commit `e4bb1a38c94180ae337a68726bd565bcfe23ea00`. The field requires
a concrete `reason` and preserves any old text in `original`. The validator
checks the frozen record's metadata and source/workload against that Git object;
renaming a new flow to an old filename does not grant an exemption. Full Git
history is required when validating these records.

A historical-gap record cannot cover a new retirement. Resolve its gaps or add
a complete follow-up record for the same issue and scoped keys. Updating only
the current checkpoint reference does not imply that old measurements were
rerun. No exemption exists for newly written evidence.

`student-owner-backfill-2758.json` and `student-owner-cutover-2759.json`
retain the pre-gate observations and acceptance criteria, not current rollout
approval. The subsequent absence-state fix (#3442) makes care-state mismatches
diagnostic and preserves absence fields through migration 1.15.398. Current
behavior and rollout checks are documented in the
[backfill guide](../../docs/operations/student-owner-storage-backfill.md) and
[cutover guide](../../docs/operations/student-owner-storage-cutover.md).
Do not rewrite the frozen records to imply that their old measurements tested
the later fix.

### Inventory and removal coverage

```bash
scripts/backend-architecture.sh validate-ticket --ticket backend/architecture/migration-ticket-template.json
scripts/backend-architecture.sh validate-ticket --all
scripts/backend-architecture.sh validate-ticket --all --base-ref <full-base-sha>
```

`--ticket` validates one file. `--all` discovers every JSON file directly under
`backend/architecture/`, including both required templates. A missing
`ticket_kind` fails rather than excluding a file. Four named documents use
other contracts and are excluded: `policy.json` (architecture policy),
`composition.json` (composition inventory), `runtime-checkpoints.json`
(acceptance registry), and `contract-active-2737-progress.json` (historical
progress notes, not cutover evidence). The two templates contain synthetic
examples and never provide retirement coverage.

`--base-ref` requires a full immutable SHA and always validates the entire
inventory, even when `--ticket` selects an additional file. It compares the
base and candidate `legacy.jsonl` sets, reporting raw removals and additions
separately. It reuses the checked relocation mapping from ADR 0020: a renamed
debt key is not retirement. Existing architecture checks still decide which
additions and relocations are permitted.

`exact_ratchet_keys` is scoped debt, not a promise to finish in one PR. Each
record reports pending keys, keys removed in this diff and keys already absent.
Every actual retirement needs at least one complete non-template record;
coverage is the union across records, including records from earlier PRs.
Both old and new paths of a verified relocation can identify the same debt.
New claims absent from both base and candidate debt fail. Previously committed
claims remain historical; the gate does not demand retrospective backfill.

CI runs this command in `Backend architecture ratchet`, using the PR base SHA,
merge-group base SHA, or pre-push SHA. Missing or zero bases fail explicitly.
Same-repository development-to-main PRs reuse exact-tree, successful development
push evidence after the release promotion checks. For the first release push
to `main` whose base predates this gate, a direct development parent or the
fast-forward commit must prove the same successful CI and tree equality;
otherwise the release fails. All current ticket files are still validated.
Changes under `docs/runtime-checkpoints/` and `docs/operations/` also trigger
the job. Keep declared local evidence in these routed directories or add its
directory to the workflow when introducing another source location.

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
policy. Ordinary waves require at least #3019 to be accepted; checkpoint
measurements can be validated before acceptance. After explicit acceptance in the checkpoint issue,
append an entry in a reviewed change, in order without gaps:

```json
{"issue": "https://github.com/moto-nrw/project-phoenix/issues/3019", "acceptance": "https://github.com/moto-nrw/project-phoenix/issues/3019#issuecomment-123"}
```

Replace the example comment ID with the actual acceptance comment. Validation
checks issue order and comment-link shape, not comment truth or measurement
accuracy. Review must verify acceptance, closed blockers, comparable runs,
and explanations. A ticket-local `accepted` flag cannot grant acceptance.
In the same change, repoint every migration record and the migration template
to the new latest accepted checkpoint. Preserve measurement provenance and run
`validate-ticket --all`; the inventory and checkpoint-transition tests enforce
this maintenance step. The closed sequence ends at #3021; adding a fourth
checkpoint requires a separate contract change.
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

Policy candidates must use `schema_version: 3` because rules now carry optional
issue metadata. Only `LoadBasePolicyAndManifest`, reading an immutable Git base,
accepts schema 2 during the schema-3 rollout and normalizes it to 3 before full
validation. Candidates never use that compatibility path. Schema 1, future
schemas, unknown fields and invalid metadata still fail. Keep this bounded
2-to-3 allowance while supported merge bases predate the rollout; remove it
once development and all supported comparison bases use schema 3. Existing
projection and migration-evidence schema versions are separate contracts.

#3416 leaves `policy_epoch` at the checkout's existing value, 17. Adding issue
metadata and deleting unused permissions grants no new architectural permission
and must not unlock the epoch-gated strictness exceptions. Add positive and
negative fixtures when changing these contracts, and remove every `rules.stale`
finding before committing a policy change.

The Care Schedule retirement has one scoped permission-replacement path
([ADR 0030](../../docs/adr/0030-care-schedule-cutover-replaces-legacy-permissions.md),
#3351). In a reviewed epoch, consumers may replace their existing, same-scope
access to `inbound-schedules/adapter` with Care Plan's public or contract role.
Tests with that same prior permission may instead construct the replacement
through Care Plan's compose role, in their existing test scope only.
The candidate must remove the legacy package and every `inbound-schedules`
package. The exception also covers the exact canonical-calendar replacement
and the four native owner-internal contract bindings described in the ADR.
Four pinned consumer points may also replace the retired adapter's Timetable
contracts and Presence capacity-error alias with their actual public owners;
each requires its own same-scope historical adapter permission. The ADR lists
these points. Other owners and non-public target roles remain forbidden.
Two retained arrival-integration fixture points may construct Timetable's
class projections, only in their historical test scopes, and bind them through
Care Plan composition themselves. The ADR pins both source points and the
single `timetable-activities/compose` target. Root composition is not a target:
its role also covers `database/repositories`, so the repository imports of
these tests stay tracked debt. No production access is authorized.
Those bindings apply only to Care Plan's composition, application, and port
roles in production scope, not to test helpers or new cross-owner access.
It does not allow new debt, ownership changes, private
implementation access, or reuse after retirement reaches the comparison base.
Intermediate states with the legacy service still present remain invalid.

The Care Lifecycle retirement has the same kind of one-time path
([ADR 0035](../../docs/adr/0035-care-lifecycle-cutover-replaces-legacy-permissions.md),
#3427). In a reviewed epoch that no longer classifies
`modules/careplan/legacy/carelifecycle`, a consumer may replace its existing,
same-scope access to that `inbound-students/adapter` with Care Plan's public or
contract role, and its tests may construct the replacement through Care Plan's
compose role. Care Plan's contract suite may keep, in `external_test` scope,
the reach the retired adapter's own behavior suites had, except the retired
owner. The ADR lists the seven rules epoch 20 adds under it.

The Student Presence nest has the third and widest such path
([ADR 0036](../../docs/adr/0036-student-presence-cutover-replaces-legacy-permissions.md),
#3422). In a reviewed epoch that classifies no package under
`modules/studentpresence/legacy` and whose base classifies all four retired
packages exactly, a consumer may replace its existing, same-scope access to
one of them with Student Presence's `public` role, Care Plan's `contract`
role or Workforce's `public` and row-holding `adapter` role, and its tests may
instead construct the replacement through Student Presence's `compose` role.
Care Plan's `test-support` role is reachable from a `test-support` source
only; `facilities`/`compose` is the single pinned production point allowed to
name `student-presence`/`compose`, for the attendance-room port whose filter
map keeps it off the public contract. The roles that received the retired code
keep exactly that code's reach in the same scope, the nest's own suites keep
theirs on the role the candidate gives their new package, the two row packages
inherit only the canonical calendar-date value, and two pinned production
bindings carry the handed-back rows into the contracts that name them. The ADR
lists the 57 rules epoch 21 adds under it. Everything else — other owners,
other roles, other scopes, production composition access — stays forbidden.

The shift-planning compatibility package has the fourth such path
([ADR 0037](../../docs/adr/0037-shift-planning-cutover-replaces-legacy-permissions.md),
#3418). In a reviewed epoch that no longer classifies
`modules/workforce/legacy/shiftplanning` and whose base classifies it as
`inbound-staff-shifts`/`adapter`, a consumer may replace its existing,
same-scope access to that package with Workforce's or Timetable's `public`
role, and its tests may instead construct the replacement through that
owner's `compose` role. The roles that received the retired code, Workforce's
`application` and `compose` and Timetable's `compose` with the test roles of
their suites, keep exactly that code's reach in the same scope. The
shift-plan-sync workflow's points are created by the candidate and need no
exception. Everything else stays forbidden.

The three existing student read projections have one fixed replacement path
([ADR 0025](../../docs/adr/0025-replace-student-read-projection-grants.md),
[#3432](https://github.com/moto-nrw/project-phoenix/issues/3432)). In a reviewed
epoch, `parent-message-inbox`, `parent-announcement-audience` and `care-exit-view`
may replace their base `users.students` grant with exactly
`users.student_profiles`, `users.student_school_memberships` and
`users.student_care_profiles`. The last table preserves the view's care-row
existence filter; the queries select no extra care fields. Package, projection
ID, owner, production/test roles, tenant safety, other grants and target write
owners must stay unchanged. The checker exempts only these exact replacement
pairs, not other additions in the same candidate. Without the old grant in the
immutable base, the exception cannot activate again. Epoch 16 performs this
cutover without changing the baseline or composition surface. Epoch 20 (#3427)
has since removed `care-exit-view`; its reads live in `student-directory-view`.

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

A reviewed epoch may also grant the shared fixture owner its four standing
reaches ([ADR 0039](../../docs/adr/0039-shared-fixtures-reach-their-standing-targets.md),
[#2748](https://github.com/moto-nrw/project-phoenix/issues/2748)): a rule with
`source_owner` `test-support`, the `test-support` role in production or the
`e2e-test` role in the test scopes, and a target of `test-support/test-support`,
`tenant-runtime/public`, `legacy-shared/domain` or `security-runtime/contract`.
Epoch 24 registers the six `shared-fixtures.*` rules under it. Everything else
the fixtures still import — the models, services and repositories other
carriers are dissolving, the device authenticator, the legacy composition —
stays exact debt on #2748 and falls with those carriers; a rule would hide it.

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
