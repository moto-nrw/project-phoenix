# Backend architecture policy

`policy.json` is the source for package and runtime write ownership, package
roles, tenant-safe read projections, legacy-composition symbols, dependency
classes, import scopes, and target import rules. Generated lint configs and
diagrams must derive from this file and must not be committed.

## Architecture principles

Six principle ADRs state the architecture rules that hold. Read the matching
one before a backend design, implementation or review; the decision ADRs they
cite keep the context, measurements and history.

| Principle | ADR |
|---|---|
| Module, data and projection ownership | [ADR 0066](../../docs/adr/0066-principle-every-module-table-and-cross-owner-read-has-one-owner.md) |
| Contracts, workflows and transactions | [ADR 0067](../../docs/adr/0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md) |
| Construction and runtime boundaries | [ADR 0068](../../docs/adr/0068-principle-owners-compose-themselves-and-the-roots-only-wire.md) |
| Test boundaries and the shared kernel | [ADR 0069](../../docs/adr/0069-principle-a-test-reaches-its-owner-the-typed-roots-and-the-shared-kernel.md) |
| Ratchet, debt and evidence | [ADR 0070](../../docs/adr/0070-principle-only-the-target-is-a-rule-and-debt-leaves-with-evidence.md) |
| Dependency direction and cycles | [ADR 0071](../../docs/adr/0071-principle-owner-dependencies-point-one-way.md) |

The [traceability appendix](#appendix-principle-traceability) maps every rule
that still holds to exactly one principle and every architecture ADR to its
principles. The sections below describe how the checker enforces the rules and
record the migration history; where they differ from a principle, the
principle states the target.

## Canonical module map

[ADR 0066](../../docs/adr/0066-principle-every-module-table-and-cross-owner-read-has-one-owner.md)
is the principle for this map.
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
#2706 adopts `documents.file_cleanup` under `file-storage` through the
[ADR 0045](../../docs/adr/0045-owners-adopt-tables-a-retired-generic-store-reached.md)
path (policy epoch 31 to 32): File Storage's Postgres adapter serves the
intents with a static table name, and the binder, the generic repository and
its model are deleted with their five baseline entries.
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

#2726 creates `platform.worker_leases` (migration 1.15.437) and assigns it to
`scheduler-runtime`: the Worker's runtime leadership, one fenced holder per
lease name, no tenant data. The only package that reads or writes it is
`modules/schedulerruntime/workerlease` (`scheduler-runtime`/`postgres`); it
opens its administrative transaction and joins the job transaction through
the tenant runtime. `services/scheduler` reaches it through its own
`LeaseStore` port, which the Serve root binds. Job transactions assert the
term through `platform.assert_worker_lease` before they commit, so the tenant
role needs no grant on the table. Runbook:
[worker lease](../../docs/operations/worker-lease-2726.md).

#2746 closed the carrier of `services/scheduler`: its 27 keys fell without a
new rule. The scheduler opens its transactions through its own
`TenantRuntime` port; `modules/schedulerruntime/jobruntime`
(`scheduler-runtime`/`postgres`, the role the lease store already holds)
binds it to the tenant runtime. The settings keys are the scheduler's own
constants: the Serve root refuses a Worker whose key the registry does not
define or defines as a secret (`settingsCompose.VerifyPreloadKeys`), and binds
the minute snapshot to the Settings Platform's cross-tenant read through
`settingsCompose.NewTenantSnapshots` behind the `SettingsResolver` port; an
unbindable snapshot fails its school instead of reading the database, and
`services.TestSchedulerMinuteSnapshotReadsEverySchoolOnce` holds the one-read
budget over the real binding. The
overdue tick publishes through `realtimeevents` and reads the day's blocks
and rooms through ports; the booking audit, the student lifecycle and the
change-history sweep are ports too. `api/scheduler_ports.go` binds them to
the retained repositories and services without importing their packages.
The internal suites compose through test support
(`testpkg.PassthroughTenantRuntime`, `testpkg.ScriptedTenantRuntime`) instead
of the repository factory.

#2742 closed the carrier of the retained School Structure packages: its 63
keys fell together with `api -> api/admin` and `api -> api/groups` (#2750),
`services/users -> models/education` (#2728), the internal-test
`models -> models/education` (#2751) and `test -> database/repositories/education`
(#2748), 350 -> 282. The group routes moved file for file from `api/groups`
to `modules/schoolstructure/http` (`school-structure`/`http`), the grade
transition routes from `api/admin` to `workflows/gradetransition/http`
(`grade-transition`/`http`); the owners `inbound-groups` and `inbound-admin`
are deleted and their two rules moved to the new points, whose own rules
name the shared HTTP runtime, the token adapter, the permission registry, the
tenant runtime and, for the group routes, their own group service and rows
and the People Directory's public types. The group routes read the children
and persons of a group through their `GroupPeople` port, which
`compose.NewGroupRoutePeople` (`modules/peopledirectory/compose`, #3753) binds
over the retained person directory.
`services/education` names no foreign model, no ORM and no tenant runtime:
its writes run on the `Runtime` port `schoolStructureCompose.LegacyRepositoryRuntime`
binds, the rooms, teachers, staff, caregivers, handovers and both audit
trails are its own ports over the `models/education` vocabulary (`Teacher`,
`Caregiver`, `HandoverQuery`, `SubstitutionChange`, `SchoolClassChange`),
and `database/repositories` and `services` translate the retained rows into
it. `models/education` carries its row columns itself instead of the shared
base shapes, keeps the room as its own `GroupRoom` projection and the
substitution's staff as `SubstitutionStaff`; the repository contracts moved
to the legacy composition (`services/education_repositories.go`).
`database/repositories/education` runs on the same runtime, reads and writes
`education.groups` with explicit statements and no longer serves the
Timetable owner's `education.class_arrival_times`. The suites compose through
`api/testutil` and name the rows through `test`; the two policy rules that
only those suites used are deleted.

#3556 deleted `models/education`; 239 -> 236. Its values moved to
`modules/schoolstructure/internal/domain` without an ORM mapping: the
retained group store maps `education.groups` through a private row, and the
not-found and store-failure shapes are `domain.RecordNotFound` and
`domain.StoreError`. The legacy composition names the values through
`modules/schoolstructure/compose` aliases (`Group`, `GroupSubstitution`,
`HandoverQuery` and the other port values), which drops
`database/repositories -> models/education` (#2743). Values another owner's
table carries go to that owner. The teacher assignments are School
Membership's `GroupAssignment` and `ClassAssignment` alone: the group
service's ports name no assignment type but take and return identifiers and
class strings (`AssignTeacher`, `TeacherAssignmentIDs`,
`SchoolClassAssignmentsOfStaff`, ...), which the retained teacher assignment
repositories implement over that owner's contract. The Workforce adapter in
`modules/workforce/legacy` serves its own `GroupSubstitution` row, and the
legacy root's `groupSubstitutions` decorator attaches School Structure's
group and School Membership's staff to the reads with relations and serves
the deletion guard's `FindGroupHandovers`, which shrinks the Workforce
legacy budget (15,926 -> 15,903 LOC). The notification recipients read
supervising staff as their own `StaffGroupPair` through
`ListGroupSupervisors`, which the retained group repository serves. The
root passes the same repositories as before: `services/factory.go` is
unchanged. The grade transition lock
key is `schoolstructure.TransitionsLockKey`. `test` maps the rows its
fixtures write itself (`test/school_structure_rows.go`), which drops
`test -> models/education` (#2748); the People Directory suites write the
transition history through `CreateTestGradeTransitionHistoryForTenant`, which drops
the external-test `services/users -> models/education` (#2728). The rules
`delivery-cutover.009`, `delivery-cutover.023` and
`workforce.adapter.school-structure-domain` allowed only those imports and
are deleted with the package entry. #2742 already deleted the legacy
`education.class_arrival_times` repository; the arrival fixtures in `test`
now write and read the table through the Timetable owner's
`ClassArrivals` contract, which the suites compose, so only
`timetable-activities` touches it.

#3349 settles the one table two owners reached for: `users.privacy_consents`
stays with `student-presence`. The recorded window bounds how long presence
data is kept, and the GDPR cleanup reads it through that owner's
`ListVisitRetentionCounts` (children without an accepted consent use the
tenant's `gdpr.privacy_consent_retention_days`, #3835); People Directory gives up the consent
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
consumer-owned ports bound in `api/care_lifecycle_owners.go` (in `services` until #4011).
The 46 compatibility permissions and the `care-exit-view` owner are gone; the
two consumers that could only reach the adapter and the moved behavior suites
use the one-time replacement path of
[ADR 0035](../../docs/adr/0035-care-lifecycle-cutover-replaces-legacy-permissions.md).

#3354 moves the staff side of the parent requests out of `services/users`.
`users.student_data_change_requests` was always Care Plan's; the decision on a
Stammdaten request now is too. `internal/application` decides, corrects and
notifies (the stale-value and invalid-target checks, the correction that
restores the old value only while the approval's value is still live, the
co-guardian notice, the ledger entry), `internal/domain` holds the value rules
(departure plans, JSON baselines), and the contract is
`masterdatarequests.Decisions` with the sentinels the students routes render.
The child's record itself stays People Directory's: the decision reaches the
person and student rows, the "läuft mit" reconciliation and the change
history through the consumer-owned `MasterDataRecords` port, and the bulk
facts through People Directory's field review, the same facts the open queue
shows. The cross-queue commands, bulk approval and conflict resolution, are a
Care Plan coordinator over the four queues it owns (the contract
`modules/careplan/parentrequests`, a package the candidate creates, with the
`parent requests: …` sentinels byte for byte); the care-schedule and offering
queues contribute through their root adapters, the excused queue through
Care Plan's own composition. The permission model stays with its owner: the
root hands the coordinator the caller's rights, evaluated by
`securityruntime.ParentRequestReviewRights`. The shared ledger
`users.parent_request_events` stays People Directory's; every queue records
through its ledger port, the root binds it to the retained repository, and
the parents portal reads and appends through that repository directly. The
reads were already native (#3182) and still go through `modules/requestreview`.
No rule was added: five resolved `services/users` keys and the two rules only
the deleted coordinator and its tests used are gone.

#3753 (slice 4 of #2728) moved the person and student services out of
`services/users`, which keeps only the caregiver and guardian clusters. The
person service is the concrete `PersonDirectory` in
`modules/peopledirectory/compose`, the one People Directory package that may
name `models/users` and `userscontract` in production. It has no interface:
every consumer keeps the port it owns (#3771), and the root passes the
concrete type. Its staff half stays the embedded `StaffDirectory` port bound
by `services.NewStaffDirectory`; the staff write inputs (`CreateStaffInput`,
`TeacherAction`) moved with that port, so the open owner question for the two
staff writes stays open. The student-route and group-route adapters moved
beside it. The change-history retention sweep is a People Directory compose
service over three ports (`StudentChangeLogStore`,
`StudentChangeLogDeletionLog`, `StudentChangeLogRetention`) that
`services.NewStudentChangeLogCleanup` binds. The photo, audit-actor and
student-directory contracts and the student directory service moved there as
well; the audit contract no longer carries `GetChangeHistory`, which needs
`models/audit` and which only suites called (they read the trail through the
repository seam), and `ErrPhotoNoTenant` moved to its one producer in
`services`. The error aliases are gone: callers name
`userscontract` directly, `ErrPersonNotFound` and `ErrStudentNotFound` moved
into `userscontract` with their instances, and the three staff write refusals
moved into `services` beside the staff directory, because the root may not
import `userscontract`. The Care Plan participation resolver now arrives at
construction, late-bound, instead of through a setter, so the composition
surface stays at 591. The test scopes decided where the suites live: no
People Directory test package may import `models/users`, so the behavior
suites stay in `services/users` and reach the directory through the services
factory, and the module suites outside build it with
`services.NewTestPersonDirectory` and name the compose contracts through the
`PeopleDirectorySuite*` entries in `services/people_directory_test_helpers.go`.
Moving those suites out of `services/users` would add keys in every scope the
policy offers (`models/users`, `models/audit`, `internal/timezone` and more),
so they stay there, as the staff suites did in #3752. Eight rules that only allowed imports of
the moved services and the `services/users/userstest` package are removed;
ten baseline keys fall (206 → 196), none is added.

#3754 (slice 5 of #2728) moved the caregiver capability out of
`services/users` into `modules/peopledirectory/compose`; it stays with People
Directory and does not move to Identity & Access (decided 28.09.2026). The
capability lock already ran through the four owner ports of #2727, so the
`database.direct-access` and `tables.unresolved` keys were gone before the
move. What the compose package may not name became consumer-owned ports in
`models/users` vocabulary: `CaregiverCapabilityAudit` (bound in `services` to
the authentication audit trail, same event types and metadata),
`SupervisionBlockers` and `PlannedSupervisionBlockers` (bound by
`database/repositories` adapters over the same Student Presence and Timetable
repositories, same errors). The two typed refusals moved into `models/users`
beside `CaregiverCapabilityState`, so the caregiver suites, which stay in
`services/users` for the same reason as the person suites, keep asserting
on them; the views suite drives the moved views through their exported
constructor. `services/users` now holds only the guardian cluster. The rule
`people-directory.application.student-presence-public` allowed no import any
more and is removed; two baseline keys fall (191 → 189), none is added.

#3755 (slice 6 of #2728) moved the guardian services into
`modules/peopledirectory/compose` and deleted `services/users`. The guardian,
payment and profile-loader services keep their API; what the compose package
may not name became consumer-owned ports: `GuardianFinancialAudit` and
`GuardianDataAccessLog` (bound in `services` to the guardian change trail and
the data-access log, same field names and resource types) and
`GuardianRolePresets` (bound to the Security Runtime's role presets, which
store the same role and permissions as `authorize.ApplyStudentGuardianRole`).
The unused mail dependencies of the guardian service are dropped. The
authorize wrappers the root used (`GrantedGuardianPermissions`, the role
classes, the legal-guardian promotion) moved into `services` over
`securityruntime`, and the parents-portal enrollment profile route reads the
autofill profile through its own `GuardianProfileLoader` port. The behaviour
suites could not stay: no existing test point may import what they import.
They moved into the People Directory contract suite
`modules/peopledirectory/contracttest` (`people-directory`/`e2e-test`,
test-only like the Care Plan and Workforce contract suites); the five tests
of unexported helpers went to the compose package's internal tests. The
seven `people-directory.module-behavior-test.*` rules that served only those
suites moved to the new point, and twelve rules there keep the fixture reach
the suites had in `services/users`, one per target. The suites left the roots
the composition inventory scans, so 18 test calls of `repositories.NewFactory`
leave `composition.json`; they still run in the contract suite. The last 22
#2728 keys fall with the package, together with `services -> services/users`
(#2747) and the two #3421 keys of the parents portal (164 → 139); none is
added, and the composition surface stays at 490.

#3356 cuts the six owner edges `api/students` held besides its carrier's
shared plumbing. The HTTP role may not import these owners' public packages,
and PR mode refuses a new rule between points that already exist, so five of
them became consumer-owned ports of the students inbound, bound by the root:
`TenantSettings` (the Settings Platform service as is: override, else
registry default, with the keys pinned to `modules/settings` by a test),
`GuardianWake` (the Communication parent event emitter; the
`parent_child_updated` fan-out is unchanged), `SchoolGroups` (the School
Structure group service, reduced to id, name and room), `ActiveEnrollments`
(the Timetable enrollment read behind the "angemeldet" column) and
`OfferingSourceResyncer` (Care Plan's booking materialization; the recurrence
gate is still taken before the student row locks). The unused
`services/iot` field is gone; the device authenticator already came from the
Device Fleet composition. The audit rows keep going through Student
Presence's `RecordDataAccess` and the consent and change-history ports; the
stored resource types, consent source and document field names they carry
are spelled out in the inbound, and the sweep's batch size is Care Plan's
`StudentDocumentSweepBatchSize`. The list export is the one edge that needed
the owner itself: `modules/documentrendering/lists` is a new Document
Rendering contract that names the list renderer's document types, so the
four export paths render byte for byte what they did; the root binds its
`NewRenderer`. Its three rules (`inbound-students.http.document-rendering-lists`,
`root-composition.compose.document-rendering-lists` and
`document-rendering.contract.list-renderer`) are anchored to that new
contract point. 19 `api/students` keys are gone (463 → 444).

#2731 closed the carrier of `api/students`: its 37 remaining keys, the
root's `api -> api/students` key (#2750) and `api/enrollment -> api/students`
(#2734) are gone, and with the list snapshot that moved from `api/common`
into the routes three #2738 model keys of `api/common` fell too
(444 -> 402). No route path, status code, error body,
middleware chain or authorization check changed; the route and middleware
goldens are byte-identical.

- The routes moved file for file to `modules/peopledirectory/inbound/students`
  (package `students`), classified `people-directory`/`http` with
  `adapter-test` for both test roles. PR mode rejects an owner change of an
  existing path and a new permission on the existing `inbound-students`/`http`
  point, so the `inbound-students` owner is deleted with its package entry and
  its rules moved to the new point, like #2732's timetable routes. The new
  point's other target dependencies are the ones the other owners' HTTP
  adapters hold: `api/common`, the session token adapter (ADR 0031, also for
  the adapter tests), the permission registry, the public contracts of
  Security Runtime and Settings Platform, the tenant runtime, the shared value
  helpers of `legacy-shared`, the Delivery Platform broadcaster and People
  Directory's own `departure` contract.
- A child is read and written through People Directory's public capability.
  The routes carry it as their own `Student` view over
  `peopledirectory.StudentRecord` (`student_record.go`), with the same
  translation the retained `database/repositories` seams applied, so the
  departure-plan baseline, the companion refusals and the not-found shapes the
  handlers branch on are unchanged. The change history, the photo lifecycle
  and the consent projection are consumer-owned ports in the owner's types,
  bound to the People Directory capability; the Audit Platform consent trail
  takes snapshots (`repositories.StudentConsents.RecordStudentConsentTransitions`).
- The person half the retained person service still decides (person writes
  with their account and RFID-card checks, the student-aware bracelet
  assignment, the dated day-log roster, the staff member behind a person) is
  the `PersonRecords` port, bound over that service by
  `compose.NewStudentRoutePersons` (`modules/peopledirectory/compose`, #3753). The group teachers the detail lists are
  plain `GroupTeacher` values. The companion sentinels and the companion link
  helpers moved to the `departure` contract; `models/users` aliases them, so
  every `errors.Is` keeps matching.
- `bun`, `*bun.DB` and `database/sql` left the package: the handler
  transactions join the request's through `tenant.WithinTenant`, and the device
  routes read the kiosk through the Device Fleet authenticator's `DeviceID`.
  The school-class grammar and the weekday names are local copies, the RFID
  request type is declared next to its route, and `auth/authorize` became the
  Security Runtime capability. `api/enrollment` renders the two shared
  companion codes from `api/common`.
- The route suites compose the owner through
  `services/student_http_test_helpers.go` behind `api/testutil` and import
  neither repositories nor retained models or services.
- The move put the handlers under the module ratchets (Rule 16): `Router` and
  eleven handlers were split into named steps and five files fell below 800
  lines. No allowlist entry was added.
- The named File Storage adapter exception stays (see below), re-anchored and
  now tracked by #2706.

#2745 deleted the API aggregate: `api.API` and `api.New` have no declaration
or reference left (`architecture/callers/api-aggregate-2745.json`, generated by
`scripts/backend-api-aggregate-callers`), and the `api` entry left
`legacy_composition`. `newServeGraph` composes the Serve root in `New`'s
place; `mountRoutes` and its route-group functions build each resource and
mount its routers at once, the tenant routes on the `/api` sub-router and the
public and portal routers at the root. The package-private `serveGraph` keeps
the router, the pool, the tenant runtime, the observability handles and the
retained factories the embedded Worker still reads (#2749), plus the two
document sweeps the Worker took from the aggregate's resource fields. The
root named `api/absence-types`, `api/shift-types`, `api/staff-shifts`,
`api/substitutions` and `api/work-time-models` only as field types, so their
five #2750 keys fell (206 -> 201); the composition surface dropped from 591 to
490 field/setter targets. The route builders read the composed module set, so
the typed references to `services.Factory` stay at the base's eight
declarations and those to `repositories.Factory` drop from five to three. The
route table and every scope, auth and error golden are unchanged; the
middleware table changes only in its first entry, where the router-wide
request-ID middleware now carries its own name (`api.requestIDs`) instead of
the deleted constructor's.

#2743 deleted the repository Factory: `repositories.Factory` and
`repositories.NewFactory` have no declaration or reference left
(`architecture/callers/repository-factory-2743.json`, generated by
`scripts/backend-repository-factory-callers`), and the root package
`database/repositories` has no Go file left; its subpackages keep their
owners. The Audit and Settings persistence is built in `modules/auditlog/compose`
and `modules/settings/compose` (ADR 0049), every other adapter of the root
moved to the owner composition that may construct it or into `services`, and
each retained repository is built where it is used. The package left the
package list and `legacy_composition` in `policy.json`. Its 15 keys fell, and
with them the imports of the root package and the references to the Factory
in `api`, `cmd` and `services` and in the external tests of `services` and
`modules/careplan/inbound/parent` (eight #2750, three #2747 and one #3421
key, 139 -> 112); the composition surface
dropped from 490 to 260 field/setter targets. The Timetable composition still
serves the retained Care Plan schedule and request rows and the Dienstplan
rows; their seams `CarePlanRecords` and `WorkforceShiftRecords` carry only the
methods the remaining callers reach (50 -> 40 and 15 -> 9). The evidence
record is `contract-repository-factory-2743.json`.

#2749 deletes the last post-construction wiring of the Worker. Its blockers
had already removed the `Scheduler.Set*` methods and `SetupAPITest`;
`newScheduler` still filled the Scheduler through three pointer-parameter
functions, and the root filled `WorkerDependencies` through two. Both are now
one literal (`scheduler.newScheduler`, `api.workerDependencies`). The
session-cleanup interval and threshold fields, which only tests read, are
deleted; the break auto-end interval is computed where the task is
registered instead of being stored on the Scheduler. The generated inventory
`architecture/callers/scheduler-setters-2749.json`
(`scripts/backend-scheduler-setter-callers`) shows no declaration or
reference of the deleted functions and fields, of a `Scheduler.Set*` method or
of `SetupAPITest`; at the evidence commit the same generator finds 31 `Set*`
methods and `SetupAPITest` with 73 references. The composition surface drops
from 219 to 140 field/setter targets (45 in `services/scheduler`, 34 in
`api`); no ratchet key changes. The Worker keeps its job set: `NewWorker`
still refuses a graph without every required job. What remains are the two
day-rollover caches of the overdue and reminder re-fire guards (task state)
and the internal test builder `unitScheduler`. The Worker still reads the
retained service Factory through `serveGraph`; that read falls with the
Factory (#2747). The evidence record is `contract-scheduler-setters-2749.json`.

#2747 contracts the service Factory in part (ADR 0050, policy epoch raised to 38).
`services` names the retained activity, facility, IoT, staff-clock, import,
audit, settings, parent-event, list-export and database-statistics services
through the owners' compositions or contracts, and the records and keys of
`models/{config,platform,audit,users}` through the owners'
`compose/vocabulary` packages. Mail and live events come through
`modules/delivery/compose/{email,realtime}`, calendar dates through
`sharedkernel/calendar`, the missing-row sentinel through
`modules/transactionruntime`. The root builds the analytics tracker, and the
legacy composition's own suites run as internal tests. Five pairs between
existing points are granted by `serviceFactoryCutoverPermission`. 23 of the
26 #2747 keys fell (109 -> 86), and the composition surface dropped from 260
to 259. The production adapters no longer hold a database handle, tenant
transaction or JWT read: those live in the owner compositions or arrive as
built values from the root. The `bun`, `tenant` and `sessiontoken`
(`legacy/jwt` before #3487) keys remain,
because the builders that compose several owners over one database
(`services/*_roots.go`, `factory.go`) and the test hooks still need them. The Factory still exists.
[ADR 0053](../../docs/adr/0053-owners-compose-themselves-and-the-roots-compose-the-owners.md)
decides where that code goes: an owner's composition builds its own owner,
and the typed roots compose the owners. #4001 grants the standing reaches in
epoch 41 (see [Standing composition reaches](#standing-composition-reaches-adr-0053)).
The `sessiontoken` key fell with #4006, when the root took over the Identity &
Access composition. The `bun` and `tenant` keys stay until #4007 to #4013
have moved the rest of the code out of `services` and #2747 deletes the
package.

#4008 moved the Workforce wiring:
`services` keeps 225 of its 260 production files (32,949 of 35,482 LOC; the
merge-base measures 35,473, the base fix of #4049 adds nine), and
the Factory takes the Workforce bindings from the root as one value,
`WorkforceWiring` (see [Workforce wiring out of services](#workforce-wiring-out-of-services-4008)).

#4007 moved the People Directory, School Membership and School Structure
wiring out of `services` without a key or a policy rule. Single-owner code went
into those owners' compositions, the substitution visibility decision into
Workforce (`CanSeeAllSubstitutions`) and the guardian role preset into People
Directory. The root builds the retained repositories and the cross-owner
adapters (`api.NewPeopleWiring`) and hands them to `NewFactoryWithModules` as
`services.PeopleWiring`, in place of what the Factory built itself; an adapter
over a value the Factory composes arrives as a constructor. The root's adapters
take narrow ports of their own instead of the retained repository types where
they read a few methods, which keeps `TestHandlerLayerRatchet` R1 green; the
root's builders still return and embed retained repository types, which R1's
pattern does not cover. Code that other slices still call stays with those callers in
`services` until they move. Two stale offboarding rules of the legacy
composition are gone, and the composition surface fell from 250 to 248.

#4009 moved the Timetable, School Calendar and reminder wiring: single-owner
code into the Timetable and School Calendar compositions, the cross-owner
glue into the root, which hands it to `NewFactoryWithModules` as
`services.TimetableWiring` and builds the planning tracks it mounts itself.
`services` keeps 127 of its 156 production files (18,141 of 21,330 LOC), and
the composition surface goes from 237 to 236 (see
[Timetable, School Calendar and reminder wiring out of services](#timetable-school-calendar-and-reminder-wiring-out-of-services-4009)).

#4010 moved the Student Presence, Facilities, Device Fleet and class-day
wiring out of `services` without a key or a policy rule (see
[Student Presence wiring out of services](#student-presence-wiring-out-of-services-4010)).
The root builds Student Presence, its session records, the retained room
service and the class-day bindings (`api.NewPresenceWiring`) and hands them to
`NewFactoryWithModules` as `services.PresenceWiring`. Facilities decides the
room deletion behind its `RoomUsage` port. The Factory fields `Facilities`
and `StudentStatusDays` are gone, and the composition surface fell from 236 to
234 (measured after #4049, which retired the unregistered tag scan adapter and
its field).

#4011 moved the Care Plan wiring out of `services` without a key or a policy
rule: `services` keeps 82 of its 126 production files (12,603 of 18,049 LOC,
measured after #4049 and #4009).
The root binds Care Plan to the other owners (`api.newCarePlanWiring`) and
hands the result to `NewFactoryWithModules` as `services.CarePlanWiring`. The
two decisions the Factory still held now live in Care Plan: the pickup
auto-excusal resync runs in Care Plan's own unit of work, and the student lock
order of the weekly pickup approval sits behind a consumer-owned port. The
Factory fields `Students` and `RequestReviewPolicy` are gone, so the
composition surface fell from 235 to 233 (see
[Care Plan wiring out of services](#care-plan-wiring-out-of-services-4011)).

#4012 moved the Enrollment wiring out of `services` without a key or a policy
rule. The booking projection's reads, capacity peaks and approved bookings are
Enrollment's own rules (`modules/enrollment`, `BookingProjection` behind the
consumer-owned port `OfferingBookings`, ADR 0053 point 4); Enrollment's
composition builds the admin deletion and the retention cleanup over one
preview (`NewDeletionModule`). The root binds
Care Plan's bookings to that port, builds every cross-owner adapter, and hands
the Factory `services.EnrollmentWiring`; the flows over values only the Factory
builds arrive as constructors. The root builds the form schemas, the captcha
and the phase expiry warnings where it mounts them, so three Factory fields
fall and the composition surface goes from 237 to 234 (see
[Enrollment wiring out of services](#enrollment-wiring-out-of-services-4012)).

#4013 moved the platform, Communication, Organisation & Tenancy and command
wiring out of `services`, again without a key or a new rule. Delivery's
composition builds what binds Delivery to Delivery: the mail and push
provider, the push authorization, the expired-subscription cleanup and the
durable e-mail store (`MailPushProvider`, `NewDurableEmails`). The Audit
composition reads the ambient transaction of its producers itself
(`NewTenantPersistence` for the service graph, `NewHandoffPersistence` for
the cleanup command, `NewObservedCommand`); the switch over `bun` transaction
types lives in its Postgres adapter (`NewHandoffRuntime`), so no composition
picks a transaction out of the context. The root builds the rest
(`api.NewPlatformWiring`) and hands it to `NewFactoryWithModules` as
`services.PlatformWiring`: the school settings store, the operator ledger, the
parent-portal stores, the unobserved Device Fleet and the list renderer as
values, and the Audit, Delivery and provisioning adapters over values the
Factory composes as constructors. It builds the Data Import and the database
statistics where it mounts them and reads the schools and the announcements
from the owners it composed, so the Factory fields `Schools`, `Announcement`,
`ListExport`, `Database`, `DatabaseStatsCapabilities`, `Import`,
`StaffImport`, `ClassListImport` and `OpeningBalanceImport`, the
`observeDataImport` parameter and three Factory methods are gone (composition
surface 237 → 228). The cleanup command composes its sweeps in
`cmd/cleanup_roots.go`; what its suites share (`NewCleanupAuditCommand`,
`NewRetentionCleanupRepositories`, `NewSessionCleanupRepositories`) is a root
function in `api`. `OutboxEnqueuer` and the retained-error helpers stay with
their callers in `services`. Three rules that allowed no import any more are
deleted (1,431 → 1,428). `services` keeps 136 of 156 production files (19,108
of 21,330 LOC). The settings, the realtime hub, the Audit command, Delivery,
the File Storage module, the operator provisioning and the messaging
services stay Factory fields: the Factory builds them over values only it
composes, or the root builders of the other slices read them through
`*services.Factory`.

#2734 closed the carrier of `api/enrollment`: its 26 remaining keys, the
root's `api -> api/enrollment` key (#2750) and three parent-portal keys of
#3421 (the production and internal-test import of `api/enrollment`, and the
internal-test `models/enrollment` import that only its stub needed) are gone
(239 -> 209). No route path, status code, error body, middleware chain or
authorization check changed; the route and middleware goldens are
byte-identical, and the composition surface stays at 591.

- The routes moved file for file to `modules/enrollment/http` (package
  `enrollmenthttp`), classified `enrollment`/`http`. PR mode rejects an owner
  change of an existing path and a new permission on the existing
  `inbound-enrollment`/`http` point, so the `inbound-enrollment` owner is
  deleted with its package entry, like #2731's and #2732's carriers. The new
  point's target dependencies are the ones the other owners' HTTP adapters
  hold: `api/common`, the session token adapter (ADR 0031), the permission
  registry, the tenant runtime, the collation helper of `legacy-shared`, the
  Document Rendering list contract, People Directory's `departure` contract
  for the companion refusals, and the owner's own public capability.
- Enrollment's adapter-test point already exists (the Turnstile adapter
  holds it), so no rule may be added for it. The internal suites therefore
  name no retained model, no `api/common` and no token adapter: claims come
  from `api/testutil/routetest`, the permission gates are pinned through the
  real `Router()` with signed sessions instead of hand-mounted
  `RequiresPermission` copies, error codes are asserted as their wire
  literals, and two database suites that composed the owner through its test
  support moved to the external package. The external suites are integration
  suites (they open the test database and compose the owner, the retained
  repositories and Identity & Access), so the package's external test role is
  `workflow-integration-test`, the classification `staffmessages` and the
  timetable route suites already carry; their rules are anchored on that new
  point. Those rules are standing permissions without an issue, as the other
  owners' integration suites hold them, and they include the suites' reach to
  the retained repository factory and to the retained `models/enrollment`,
  `models/users`, `models/config` and `models/platform` rows they seed and
  read. That reach is test debt the ratchet no longer counts: the four
  external-test keys fell because the suites changed classification, not
  because their imports went away.
- `bun` and `database/sql` left the package: the handlers open their tenant
  and administrative transactions through `tenant.WithinTenant` and
  `tenant.WithinAdmin`, a bare `Resource` from a unit test still runs
  `runInTenantTx` on the request context, and the form-schema lookups rely on
  the owner's `ErrFormSchemaNotFound`, which already wraps the no-rows case.
- `models/enrollment` left the package. A request and a care offering are the
  routes' own decoded values over Enrollment's public `Request` and
  `CareOffering`; the catalog administration reaches Care Plan through the new
  `enrollmentCompose.CareOfferingValues` (bound by the free function
  `api.NewEnrollmentCareOfferingValues`), which serves the same row
  translation as `CareOfferingRows` in the public values. The status, consent
  and phase constants come from the public package.
- The autofill profile is Enrollment's consumer-owned `GuardianAutofillReader`
  in its public package; `services.NewEnrollmentGuardianAutofill` binds it to
  the retained People Directory loader, so neither the routes nor the root
  name the loader's rows. Both bindings are free functions, so
  `services.Factory` gains no method and no reference.
- The parents portal no longer imports the enrollment routes. Its own
  `EnrollmentForms` port carries the form load and the submission in the
  owner's wire shape, validation and error mapping; the root binds it to
  `enrollmenthttp.ParentForms`. The portal keeps its school, guardian and
  hidden-school gates and its transactions; the stamping of the guardian
  account and the submit eligibility moved with the decoding and is pinned by
  the `ParentForms` suite.
- The move put the handlers under the module ratchets (Rule 16): `Router`
  is split into mount steps, the submit error mapping became a rule table,
  seven more functions were split into named steps and five files fell below
  800 lines. No allowlist entry was added; the parent portal's
  `getEnrollmentBootstrap` entry fell from 87 to 83.
- `architecture/callers/enrollment-routes-2734.json`, generated by
  `scripts/backend-enrollment-route-callers`, records that `api/enrollment`
  has no importer or source file left and that the four route helpers once
  exported for the parents portal have no declaration or reference.

#2727 closed the carrier of `database/repositories/users`: its 14 remaining
keys are gone (402 -> 388), the package keeps its owner, role and tables, and
no rule was added. The one rule that allowed the Communication suites to
construct the retained person repository went stale with its last import and
is deleted.

- The retained repositories read the tenant and the ambient transaction
  through the consumer-owned `Runtime` port. People Directory's composition
  binds it over the tenant runtime (`compose.LegacyRepositoryRuntime`), and
  `database/repositories` passes it to every constructor. The generic
  repository of `database/repositories/base` is replaced by explicit CRUD
  methods with literal table expressions and a local copy of the query-option
  applicator; the error and query shapes the retained contracts name
  (`DatabaseError`, the not-found sentinel, `QueryOptions`, the
  unique-violation checks) come through the `models/users` vocabulary, so
  callers that classify with `errors.As` keep matching. Calendar dates use
  `sharedkernel/calendar`.
- The authorization policy stays out of the package: the two stored
  parent-portal permission keys the guardian reads filter on are local copies,
  and a link without a role takes the default preset through the
  `WithGuardianDefaultRole` option, which the composition binds over
  `securityruntime.DefaultStudentGuardianRole` and `StudentGuardianRolePreset`;
  without it the store refuses such a link. The legacy composition reads the
  caller's school through `peopledirectory/compose` and the permission
  matcher through `modules/securityruntime` instead of helpers of this package.
- The 39 suites stay external tests of the package. They compose the retained
  repositories and the legacy factory through `services/people_repository_test_helpers.go`
  behind `api/testutil`, and name rows, values and errors through
  `test/people_repository_vocabulary.go` and the tenant helpers of
  `test/tenant_runtime.go`. The 208 tests and their subtests are unchanged in
  number and assertion.
  #3966 moved 34 of those suite files into the People Directory contract
  suite (`modules/peopledirectory/contracttest`), whose existing rule
  `people-directory.contracttest.people-directory.domain` lets them name the
  `models/users` rows themselves; the two stranding-batch suites, which open
  `departure.ContextWithStrandingBatch`, stay in `modules/peopledirectory`.

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
`services/presence_bindings.go`. The retained list endpoints
still read one row per block or participant, so the tenant-safe projection
`modules/presenceprojection` (owner `presence-legacy-view`) joins the plan
with the owner rows in one statement each (`ListLegacyInstances`,
`ListLegacyParticipants`, the partial-absence, parallel-presence, course and
manual-planning reads), and `timetable/compose.PresenceReads` wraps it for
the legacy composition. The old execution and attendance columns stayed as a
trigger-kept rollback mirror with the `active.presence_compatibility_writes`
counter until the Contract #2763 (migration 1.15.432) removed them and
narrowed the planning status to planned and cancelled;
`TestPresenceStorageCallerInventory` keeps application code, fixtures and
behavior tests off the retired names. The later-pickup decision that writes to both owners is
bound at the composition root (`api/pickup_extensions.go`) rather than as a
new workflow owner. `database/repositories/student_presence.go`, the old
provider, is gone. The evidence lives in
[presence-cutover-2762.json](presence-cutover-2762.json) and the runbook in
[docs/operations/presence-storage-cutover-2762.md](../../docs/operations/presence-storage-cutover-2762.md),
the Contract in
[docs/operations/presence-storage-contract-2763.md](../../docs/operations/presence-storage-contract-2763.md).

#2756 cut the student-guardian relationship over to its three owners
(migration 1.15.417, one release with the caller switch). People Directory
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

The command reports. From policy epoch 48 the target graph has no cyclic
component (ADR 0058 point 1, #3417): the policy loader rejects a policy whose
target projection holds one (`internal/architecture/acyclic_target.go`) and
names the component and the rules carrying its edges, so `check`, CI and
`cycles` itself fail on it. The migration graph, which includes the exact
debt, may stay cyclic until its keys are gone.

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

### Composition surface

PR mode also derives the composition surface of the base commit and of the
candidate from source and rejects every key the candidate adds. A key is
`scope|field|package|Struct.Field` or `scope|setter|package|Function->Target`.
It counts:

- the fields of the central aggregates: `database/repositories.Factory`,
  `api.API` and `api.serviceGraph`, which also names the fields of the deleted
  `services.Factory`;
- setters, assignments to the dependency fields of a receiver or pointer
  parameter after construction;
- since #4071, every field of every struct a `root-composition` package
  (`.`, `api`, `cmd`, read from `policy.json`) writes down that holds values
  of several owners (ADR 0059 point 1).

The root-struct part is semantic (`internal/architecture/composition_owners.go`).
PR mode loads the root packages of the candidate and of the base commit,
whose tree it extracts with `git archive`, with full type information,
production and test variants alike, as a standalone module (`GOWORK=off`; a
`go.work` in the repository is reported as unsupported). A root package that
does not load or type-check fails the comparison; it never counts as empty. Every struct the root writes down is a carrier: a struct
type literal anywhere (package-level or local type, anonymous variable,
literal, parameter or field, nested structs included), a type expression that
instantiates a generic root struct (`type Wiring = Graph[alpha.Port,
beta.Port]`, `var wiring Graph[...]`, `Graph[...]{}`), and a variable a call
types as such an instance, also as one of several results.

A carrier counts when its struct holds values of at least two owners other
than the root. A type holds a value of owner O when it is, after resolving
aliases and unwrapping pointers, slices, arrays, maps and channels, a named
type declared in a package `policy.json` assigns to O; the type arguments of a
foreign generic type count as well. The root's own named types, local ones
included, are followed into their underlying type, so wrapper structs, local
aliases and generic instances resolve to what they hold. A function type and
the own methods of an interface are ports of the root and hold no owner value;
an interface that embeds a named interface holds what that one holds. Owners
of the `kernel` kind are shared vocabulary and do not count; an internal
package without classification counts as an owner of its own. Base and
candidate are classified with both policies, and a struct counts when either
sees two owners, so a reclassification cannot drop a struct the base counts.

The carrier name is lexical: the type or variable name, prefixed with the
enclosing function or method when it is local (`root.Wiring.A`), the parent
carrier and field for a nested struct (`wiring.Inner.More`), or `struct<n>`
for the n-th counted unnamed struct of a declaration (`root.struct1.A`). A name
that repeats within a package scope gets `#2`, `#3` in source order; only
counted carriers take part in the numbering. Known limits, which only
deliberate code reaches: a generic instance a call infers and passes on
without a variable or written type, and structs built by reflection.
`internal/architecture/composition_owners_test.go` pins the definition.

On development `c49b81c3a9` the surface rose from 139 to 1,732 keys when #4071
added the root structs: 192 structs with 1,684 fields, among them
`serviceGraph` (91), `graphBuild` (46), `PeopleWiring` (40),
`serviceGraphInputs` (38), `IdentityMaterial` (33) and the other `*Wiring`,
`*Sources` and `*Inputs` structs. That rise is the purpose of the measurement,
not new debt: the fields existed and no ratchet counted them. It is the
measure of #4080 and reaches 0 when no root struct holds values of several
owners. From here it only falls.

### Source ratchet budgets

The ratchet tests in `test/*ratchet*_test.go` keep their budgets and
allowlists in Go declarations. PR mode reads them at the base commit and
rejects a raise (ADR 0057 point 5). The measurements, compared by name across
the files of the package:

- every top-level integer constant (a threshold or a register total);
- every value of a top-level `map[string]int` register and its sum, so a new
  key cannot add what an existing one may not;
- the entry count of a top-level `map[string]string` exception register.

The ratchet files are type-checked together with `go/types`, so every value
is the compiler's: typed conversions and complements (`^uint8(0)` is 255),
`iota` and references to other constants included, and a register may have a
named map type. Imports resolve to empty packages; a value that depends on one
or on a declaration outside the ratchet files has no value. A base
measurement whose candidate declaration has no integer value is an error. A
base measurement may not disappear either, also not by moving it out of the
ratchet files: it stays, its register was empty, or exactly one candidate
declaration of the same kind says `renamed from <old name>` in its comment
and is compared with the old values. Two successors for one name, or one
declaration naming two predecessors, fail. A new declaration has no base and
starts a measurement; other literals (selector sets, scanned directories) are
not compared.

A raise passes only under an ADR the change adds: the comment on the raised
entry or its declaration cites `ADR 0123`, and the base has no ADR with that
number. Renaming or copying an existing ADR file authorizes nothing. A
measurement that disappears needs the same: a comment naming it that cites an
ADR the change adds. The only exception is a closed, one-time list bound to ADR
0057 (`retiredByADR0057`): the three temporary-rule registers of
`test/policy_temporary_rules_ratchet_test.go` that #4070 replaced by a
structural check.
`internal/architecture/ratchet_budgets_test.go` pins the rules.

### Converting temporary permissions to exact debt

A rule states the target
([ADR 0057](../../docs/adr/0057-only-the-target-is-a-rule.md), #4070). From
policy epoch 47 the loader rejects these rules
(`internal/architecture/target_rule.go`):

- a rule that lets one owner import another owner's `application`, `domain`,
  `adapter`, `postgres`, `cli` or `legacy-model` role, for any pair of owners
  its source and target selectors can match; `same_owner` rules and rules
  whose source and target name the same owner are the owner's own reach;
- a rule that lets a `platform` owner import a `domain`, `projection` or
  `workflow` owner, again for any pair its selectors can match, so an
  owner-agnostic rule into a domain role must name its source kinds;
- a rule that lets the composition of a `domain`, `workflow`, `projection` or
  `platform` owner (source role `compose`, or no source role) import another
  owner's `compose` or `http` role: cross-owner glue belongs in the root
  ([ADR 0059](../../docs/adr/0059-the-roots-hold-no-service-graph.md) point
  6, #4004). The typed roots, the legacy composition and `inbound` owners are
  not owner compositions. A target in a composition owner stays allowed in
  the test scopes only (the roots' standing test reach); no owner imports a
  root in production (ADR 0058);
- a rule with an `issue` field;
- a rule whose description contains a transition marker. The markers are a
  fixed list, matched case-insensitively: "convert … to exact debt",
  "compatibility permission", "compatibility binding", "compatibility
  adapter", "temporary"/"temporarily", "until #N" and "tracked by #N".

From epoch 48 a policy whose target graph closes a cycle is rejected too
([ADR 0058](../../docs/adr/0058-owner-dependencies-form-a-dag.md), #3417); a
second direction between two domains is rejected as part of a cycle. The
loader does not judge a platform rule into an `inbound` owner: `inbound`
owners are outside the target graph, so `settings-platform.http.inbound-common`
closes no cycle and stays a rule. A temporary allowance is an exact `legacy.jsonl` key
with its open carrier issue, never a rule. Older epochs stay loadable as
immutable PR bases: there an `issue` field still marks a temporary
permission, and an explicit empty, null, malformed, query, fragment or
pull-request URL stays invalid. Keep that read path while a supported base
predates epoch 47. `audit-issues` reads only the baseline's issues, with one
request per distinct URL; it still loads the policy, so an invalid policy
fails it. Closed issues, pull requests and failed requests fail the audit.

The retained `models` and `models/*` packages carry their own role,
`legacy-model`, which only their owner may import (ADR 0057 point 4). From
epoch 47 every non-test-support package under `models/` has that role and no
other package has it. The session token runtime
`modules/identityaccess/sessiontoken` is classified as Identity & Access's
`http` role (point 3): every HTTP adapter, composition and suite that
verifies or mints sessions reaches it through a rule into that role, and the
typed roots reach it through `root-composition.reach.domain.http`. `http`
rather than `contract`: the package holds the signing and verification
runtime, not a pure contract, and the roots' standing reach into every
domain's `http` role already covers them.

The comparison carries exactly these two decided reclassifications over, bound
to their packages and to epoch 47, the epoch that makes them
(`internal/architecture/role_reclassification.go`):
`modules/identityaccess/sessiontoken` from `adapter` to `http` and `models`
and `models/*` from `domain` to `legacy-model`, each with unchanged owner and
test roles and only while the base still has the old role. A rule or import
into the new role of the owner counts as covered when the base let the same
source into the old role or into the new role; so the owner's other packages
in the new role become reachable for those sources too. Out of a package the
old role's reach carries over only for the reclassified package itself; a
rule out of an owner's role counts as covered only when every package of the
owner in that role was reclassified into it. An unchanged package keeps its
own base permissions. A role the base gives no package (`legacy-model`)
takes the old role's permissions for every owner, so `same_owner` rules
selected by kind keep their reach. Any other package, role change, later
epoch, owner change or test-role change stays under the ordinary guards. The
epoch may not decrease, so the epoch-gated validation cannot be switched off.

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

The three conditions hold per tuple, so one change may delete any number of
rules and record exactly the imports they allowed. A key without a deleted
rule behind it describes an allowed import and is a stale entry.

#4070 (ADR 0057, policy epoch 46 → 47) applied the target-only validation to
the whole policy. It deleted 405 rules and recorded the imports they allowed as
514 exact keys, one carrier per family (6 → 520 keys, 1,351 → 976 rules):

| Carrier | Family | Keys added | Keys now |
|---|---|---:|---:|
| #4076 | domains, workflows, platforms and test support in another owner's private role, `models/*` | 102 | 102 |
| #4004 | owner compositions in another owner's private, `compose` or `http` role | 97 | 97 |
| #4081 | `internal/timezone` wrapper | 87 | 87 |
| #4073 | Parent Portal and `inbound-parent` | 61 | 61 |
| #4077 | projections, compatibility adapters | 49 | 49 |
| #4074 | Communication parent compatibility | 44 | 44 |
| #4072 | platform into a domain, projection or workflow owner | 34 | 34 |
| #4075 | `delivery-cutover.*` | 16 | 16 |
| #4078 | `inbound` owners | 12 | 12 |
| #2729 | rules with `issue` (`models/users`) | 8 | 9 |
| #2750 | rules with `issue` (root) | 2 | 4 |
| #2751 | legacy composition | 0 | 3 |
| #4096 | dev tools of the commands root (ADR 0056) | 2 | 2 |
| | total | 514 | 520 |

The audit's rule list per carrier decides the family of a deleted rule; a rule
outside it follows the same order: imports of `internal/timezone` go to #4081,
the nine rules with an `issue` field keep #2729 or #2750, `delivery-cutover.*`
goes to #4075, `communication.parent-compatibility.*` to #4074, Parent Portal
and `inbound-parent` sources to #4073, platform sources to #4072 (into a
domain, projection or workflow owner) or #4076, an owner composition into
another owner's private role, or into its `compose` or `http` role, to #4004, and the remaining sources by kind:
projections #4077, `inbound` owners #4078, domains, workflows and test support
#4076, the roots #2750, the legacy composition #2751.

The same epoch also:

- classifies `models` and `models/*` as `legacy-model` and gives the owners'
  own rules into or out of them a `legacy-model` twin, or retargets a rule
  that only reached them;
- classifies `modules/identityaccess/sessiontoken` as `identity-access`/`http`
  and retargets the 55 rules into `identity-access`/`adapter` to that role.
  The settings platform's two session rules are platform rules into a domain
  owner and became keys (#4072); the compositions of Facilities, Timetable
  and Device Fleet reach the runtime from their `compose` role, which is
  glue, and became keys too (#4004). The root's two session rules of ADR 0056
  are deleted: `root-composition.reach.domain.http` covers the root's
  imports. Identity's own composition and suites reach the runtime through
  `identity-access.{compose,e2e-test,module-behavior-test}.session-runtime`;
- splits the eight owner-agnostic test rules into a domain, projection or
  workflow role (`adapter-test.{domain,projection,workflow}.*`,
  `external-test.{domain,projection,workflow}.*`) by the source kinds that use
  them, `<kind>.<rule>`, without the platform kind; the platform imports they
  allowed are keys (#4072);
- renames the target-shaped `care-schedule-cutover.*`,
  `timetable-planning-cutover.*` and `delivery-cutover.*` rules without
  changing a selector: the cutover prefix goes, a numbered delivery rule is
  named `<source owner>.<source role>.<target owner>-<target role>`;
- deletes the epoch-gated acceptors of the finished cutovers (ADR 0030, 0035,
  0036, 0037, 0038, 0049, 0050, 0052 and the Enrollment application move):
  each needs a base that classifies a package development no longer has
  (`modules/careplan/legacy/careschedule`, `…/carelifecycle`,
  `modules/studentpresence/legacy/*`, `modules/workforce/legacy/shiftplanning`,
  `…/timetracking`, `modules/timetable/legacy/timetableplanning`,
  `services/enrollment`, `database/repositories`, `services`), so none can
  fire again.

Kept after review: rules into another owner's `public`, `contract`, `port` or
`test-support` role, and into its `compose` or `http` role from a source that
is not an owner composition (the roots, `inbound` owners, HTTP adapters and
tests). That is not a claim that all of them are target: the cycle criterion
of ADR 0058 came with #3417 below.

#3417 (policy epoch 47 → 48) made the target graph acyclic. After the
domain → projection and domain → workflow rules ADR 0058 point 2 already
excludes, six domains still formed one cyclic component. The layering
recorded in the addendum to ADR 0058 decides their directions: People
Directory lowest, Care Plan above it, Enrollment above Care Plan, Workforce
above Student Presence. The 14 rules that closed the cycle are deleted and
the 18 imports they allowed are exact keys, a one-time rise of 480 → 498
approved only for this conversion:

| Rules | Carrier | Keys |
|---|---|---:|
| `people-directory.http.*` into Care Plan, Enrollment, Student Presence, `group-live-view`, `request-review-view`, `student-deletion`; `student-presence.http.calendar-view`, `student-presence.http.to.open-room-move`, `timetable-activities.http.class-day-view` | #4079 | 15 |
| `care-plan.application.native-enrollment-selection`, `care-plan.postgres.enrollment-query`, `student-presence.application.workforce-public` | #4076 | 3 |

`cycles --graph target` reports 0 components (58 nodes, 219 edges).

#4076 cuts the three #3417 keys of the layering in code: Student Presence
names the stamp channel of its automatic check-in in its own vocabulary
(`studentpresence.StampSource`), the root maps it onto Workforce; the care
offering selection rules moved from `modules/enrollment/selection` to
`modules/careplan/selection` (`care-plan`/`public`), because Care Plan owns
the care offerings and Enrollment sits above it; the guardian portal's
enrollment views reach the Care Plan parent repositories as
`careplan.ParentEnrollmentPhase` and `ParentEnrollmentRequest`, which the root
binds to Enrollment.

#4072 removed 25 of the 34 platform keys (519 → 494) without a rule. Each
platform declares what it needs in its own vocabulary and the root binds the
owner:

- Settings Platform: the operator routes take the operator and the school
  slug through ports (`OperatorIdentity`, `SchoolLookup`); Care Plan's
  review before booking-led care moved to Care Plan's own HTTP adapter
  `modules/careplan/http`, which the operator router mounts; the root wraps
  Care Plan's refusal of the switch in `settings.BookingAuthorityBlockedError`.
  The wizard tables stay Settings Platform's: their repositories are generic
  over the state shape, School Setup instantiates them with its own types.
- Document Rendering: the plan export's translations of Workforce, Timetable
  and Facilities records moved from `modules/planexport/compose` into the root
  (`api/plan_export.go`); the package is deleted. `lists` names the plan columns the root's suites read.
- Audit Platform: the booking consistency audit reads approved bookings in
  `auditlog.ApprovedBooking` and `auditlog.ApprovedBookingOfferingLink`; the
  root binds Enrollment. Its suite runs in the root over the real owners.
- Delivery Platform: the staff recipient resolver reads groups, accounts,
  thread claims and the staff on duty through its own ports;
  `workforce.OnDutyStaffIDs` keeps the on-duty rule in Workforce. The strings
  Delivery shares with Care Plan's status days and the reminder workflow are
  pinned where they are handed over.

The nine `services/scheduler` keys remain with the Worker track (#4067). The
cut of `organization-tenancy -> delivery-platform` that ADR 0058 point 6
assigns to #4072 is still open; its keys are carried by #4075 and #4076.

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

#3487 moves the session token adapter `modules/identityaccess/legacy/jwt` to
`modules/identityaccess/sessiontoken` the same way (policy epoch 41 to 42,
[ADR 0054](../../docs/adr/0054-the-session-token-adapter-leaves-legacy-now.md)),
with the package clause `sessiontoken`. It keeps the `identity-access` owner,
the `adapter` role and both test roles, and the 27 Go files keep their names.
The three remaining baseline keys are renamed and keep their issues: `api` in
production and internal test (#2750) and `services` (#2747). No key falls, no
rule is added. The 254 importing files (150 production files in 35 packages)
change only the import path and the qualifier; the 27 that aliased the
package (`authjwt`, `jwtPkg`, `jwtpkg`, `projectJWT`) now use its name. The
inactive declaration
`auth/jwt -> modules/identityaccess/legacy/jwt` of #3226 is removed, because
its target is no longer classified and every `to` must be. The move empties
`modules/identityaccess/legacy`, so its legacy LOC budget entry is gone
(1,358 LOC, total 4,408 to 3,050). The middleware golden only renames the
authenticator and scope middleware package.

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
the planning package, because the Timetable coverage probe and staff pool
share that vocabulary; they go with #3424. Since slice S4 (#3550) the
overview takes the shift-coverage interval math and the calendar-week
helpers from the public Timetable contract (`timetable.UncoveredShiftIntervals`,
`timetable.ContainingCalendarWeek`) and keeps its own indexing and ordering
helpers.

The retained timetable and instance services that `services/schedule` used to
hold (templates, splits and updates, materialization, the instance lifecycle
and deviation pipeline, conflict detection, the timetable data and operations
reads, calendar periods, holidays and closing days, planning tracks, the
roster reconciler, cleanup and the attendance mirror) are retained as the
`inbound-timetable`/`adapter` compatibility package
`modules/timetable/legacy/timetableplanning` (#3218; dissolved slice by slice
under #3424 and deleted by slice S1, #3554), moved file for file with
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
test providers the cross-package tests built went to the test-only root
`modules/timetable/legacy/timetablesqltest`, which #3554 deleted with its
package entry and its `inbound-timetable.test-support.*`,
`inbound-timetable.e2e-test.*` and `<consumer>.<role>.inbound-timetable-test-support`
compatibility rules: the consumer suites read through the repositories the
composition root binds to the Timetable owner, the repository behaviour tests
of those bound repositories moved to `modules/timetable/compose/httpintegration`
(`legacy_*_test.go`) and the staff-notice and listing-adapter tests to
`modules/timetable/compose`, and the tests of the providers themselves were
dropped. With both packages gone,
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
their fourteen cross-owner implementation edges are gone. The tenant runtime
edge `tenant` went next: the suites open their tenant, admin and current-tenant
transactions and name the savepoint failure through the tenant runtime helpers
of `test`, which the `shared-fixtures.tenant-runtime-public` rule already lets
reach `tenant`. ADR 0053 point 7 settled the suites' role: they are
Identity's own suites, and #4001 lets them import the module's public contract
and its token adapter `modules/identityaccess/sessiontoken`. The refresh
rotation application `auth/rotation` stays forbidden to them. The recovery
proof is therefore an input of the public refresh capability
(`AccountAuthentication.RefreshTokenWithAudit`): the compose engine binds it
for the rotation policy, the account refresh route passes its header on, and
the suites pass the proof as an argument and pin the five-minute grace and the
sha256 family fingerprint by value. No `legacy.jsonl` entry names #3446.
The school-portal router suites bind their runtimes through
`modules/schoolportal/portaltest` (`inbound-school`/`test-support`), the
owner's own test support, instead of
naming the legacy composition from the suite; its three rules are anchored to
that new point for the same reason.

The Workforce time-tracking HTTP composition
(`modules/workforce/inbound/timetracking`) is classified `workforce`/`http`
under the same compatibility permissions (#2690). It replaced
`api/time-tracking` and serves the public work-session, absence, month,
ledger, month-close, overview, audit-log, export and personnel-record
contracts of `modules/workforce`. Since #3413 the Workforce composition
builds the time-tracking contracts (`compose.NewTimeTracking`) and the root
hands the routes the values it returns; the staff directory still comes
from the retained People Directory services. Its own-shift and assignment
routes read the public `OwnShiftQuery` and `StaffAssignmentQuery` contracts
the Workforce planning composition serves (#3418); the staff-notice route
moved to Timetable with its service. The kiosk staff
clock consumes the public device-scan contract in `modules/devicescan`
(`process-device-scan`/`public`); `staff-clock.to.process-device-scan` is the
one rule anchored to that new point, and the staff-clock workflow reaches the
Workforce time clock only through its own port.

#3752 later moved the personnel-record administration out of `services/users`:
the payroll number, the Stammdaten sections, the audited bank and tax reads and
the Dokumente tab (the retained staff Stammdaten, payroll, document and default
permission services) are Workforce's own `StaffAdmin`
(`modules/workforce/internal/application`, rules in `internal/domain`, bound by
`modules/workforce/compose.NewStaffAdmin`). The staff and person rows behind a
record and the audit trail stay with School Membership, People Directory and the
audit platform: Workforce reaches them through the consumer-owned ports
`StaffAdminSubjects` and `StaffAdminAudit`, which the root binds over the
retained repositories in `services/staff_admin_composition.go`, so every read
and write still joins the caller's tenant transaction and reports its failure
in the repository shape the HTTP layer classifies. The staff and teacher
lookups and the two staff writes of the person service leave as the
`users.StaffDirectory` port (`services/users/staff_directory_port.go`), bound
by `services.NewStaffDirectory` and embedded in the person service, so the
callers keep the verbatim repository results the IoT flows depend on. #3753
moved that port with the person service to `modules/peopledirectory/compose`.

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

#3413 dissolved the package. The services moved file for file to
`modules/workforce/internal/application/timetracking` (`workforce`/`application`, test roles
`module-internal-test` and `module-behavior-test`), a subpackage of the
native application rather than part of it, because the native CRUD speaks the
domain types while the moved services speak time-record rows of their own
(until #3996 the `modules/workforce/adapters/timerecords` rows). The adapters that served the
public `WorkSessions`, `TimeClock`, `StaffAbsences`, `WorkTimeMonths`,
`BalanceAdjustments`, `MonthClosing`, `StaffOverview`, `TimeTrackingAuditLog`,
`StaffTimeExport`, `VacationTakeovers`, `OpeningBalanceBookings` and
`AbsenceTypeAdministration` contracts from `services` moved into
`modules/workforce/compose`, beside the `*_engine.go` mappings, so no package
outside `modules/workforce` implements them.
`modules/workforce/compose.NewTimeTracking` builds every service over the
capability and the consumer-owned ports the root binds in
`services/time_tracking_ports.go`. The work-session repository shim, its five
constructors and their root bindings are deleted. #4084 moved the row
mapping into the time-tracking package (`capability_*_rows.go`) and the
Dienstplan rows into `internal/planning/capability_shift_rows.go`; both build
the retained error shape through `transactionruntime.RecordErrors`.

#3996 retired the row vocabulary of `modules/workforce/adapters/timerecords`.
The rows and their repository contracts are declared in the time-tracking
package itself (`*_record.go`, `time_record_repositories.go`), not in
`internal/domain`, because the Workforce domain may not import `models/base`
or `internal/timezone`, which the rows embed. The staff absence and absence
audit adapters moved from `modules/workforce/legacy` into
`modules/workforce/compose` (`absence_rows.go`), since a Workforce adapter may
not import the application. The two staff absence decorators of the root stay
in `services`, because they need School Membership and People Directory; they
name the contract through `modules/workforce/compose` aliases, and the Student
Presence suites that test them name the rows through `services` aliases until
#4008 and #4007 move the decorators into the root. #4008 did: the decorators
live in `api/workforce_staff_absences.go`, and the suites name the rows
through aliases in `api/workforce_test_helpers.go`. The Student Presence
auto-check-in port returns the public `workforce.WorkSession`, which
`compose.TimeTracking.AutoCheckIn` maps to and passes the time clock's error
through unchanged. The package entry, the nine rules naming #3996 and
`workforce.adapter-test.time-records-calendar-dates` are deleted.

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
owner facades (`class-day-view.from.*`). Since #3563 the application also
builds the school portal's day report, the supervision sheet and the class-wide
arrival exceptions itself; Enrollment's class roster of a day reaches it
through the `DayRosters` port the root binds. Its `class-day-view.compose.*`
permissions for the retained schedule services (care-day derivation, effective
times, pickup baselines), the settings service, the user context and the JWT
permission check exist only
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
`student-presence.adapter-test.inbound-common` rules are standing target
rules (#3447): `api/common` is the shared HTTP runtime the other owners' HTTP
adapters and adapter tests bind as well, and it has no replacement. The
`root-composition.to.student-presence-http` mount stays a compatibility
permission; convert it to exact debt with the rule above once the package
exists at a base SHA. #4001 deleted it: the standing root reach of ADR 0053
(`root-composition.reach.domain.http`) is the target permission for it.
The calendar-date edges are gone: the adapter and its tests name
`sharedkernel/calendar` directly under the shared-kernel rule
([ADR 0040](../../docs/adr/0040-every-role-may-import-the-shared-kernel.md)).
The settings test edge is gone too: the tests name the setting keys through
the public `modules/settings` contract, which now also exports the four
tracking-indicator keys.

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

Slice S4 (#3550, policy epoch 25 to 26) moved conflict detection and
staffing to the Timetable owner: the start and planning conflict checks,
the window conflicts with their persisted acknowledgement fingerprints, the
exception conflicts, the staff pool, the shift-coverage probe and the
staffing rules. The public contract is `timetable.ConflictDetectionCapability`
with the pure capacity, understaffing, slot-precedence and interval
functions beside it; the implementation lives in `modules/timetable/compose`
and still reads the retained repository rows, whose status carries the
Student Presence session state the owner's own rows do not. Two collaborators
the owner may not name are bound by the legacy composition
(`services.NewTimetableConflictDetection`): Security Runtime's content
fingerprint, of which the persisted fingerprint keeps the first 32 hex
characters, and Care Plan's arrival baseline behind a consumer-owned port.
`api/timetable` holds the capability directly; the retained
`TimetableDataService` only carries it to the root (`ConflictDetection()`),
because `services.Factory` may not grow a field. The instance lifecycle and
the auto-start tick call the owner's start check, the Dienstplan overview
and the supervision dashboard its public types, and the plan export reads
staff names through a root-bound port instead of the nest's staff reader.
Four emptied compatibility rules are deleted, five replacement permissions
use the extended exception of ADR 0038, and `legacy.jsonl` is unchanged.

Slice S5 (#3551, policy epoch 26 to 27) moved the planner's reads, the
operational day, the retention cleanup, the ended-session completion and the
planning-track administration to the Timetable owner. The public contracts
are `timetable.TimetableDataCapability` (blocks with their rows, staffing
gaps, a child's week, the Vorlagen list, the Änderungsprotokoll, the
spontaneous-start preparation and the conflict acknowledgements),
`timetable.OperationCapability`, `timetable.TimetableCleanup`,
`timetable.EndedSessionCompletion` and `timetable.PlanningTrackAdministration`,
with the lifecycle clock policy, the attendance-patch rules and the reopen
gate as pure functions beside them; the retained lifecycle and template
services delegate to those functions. The implementation lives in
`modules/timetable/compose` and still reads the retained repository rows.
The collaborators the owner may not name are consumer-owned ports bound at
the root: `repositories.TimetableOwnerRows` hands over the retained rows,
the Facilities room names, the School Structure group names and the Audit
Platform's Änderungsprotokoll; `services` binds Care Plan's care days,
baselines and effective times, Student Presence's attendance writes, the
settings, the retained instance lifecycle (until slice S1) and the tenant
runtime's advisory locks. The retained `TemplateService` (template writes
and the attendance correction, slices S2 and S3) carries the planner reads
to the root beside the conflict detection (`TimetableData()`), because
`services.Factory` may not grow a field. The scheduler, the command line,
the supervision dashboard, the kiosk session mirror, the sick cascade and
the timetable end-to-end flows no longer name the nest. `careplan.ScheduleError`
stays the one operation-wrapping error type (decision on #3551). Seventeen
emptied compatibility rules are deleted, three replacement permissions use
the exception of ADR 0038, and three resolved `legacy.jsonl` entries are
removed; no key is added.

Slice S2 (#3552, policy epoch 27 to 28) moved the template writes (create,
update, split, end, the weekday rosters, the offering source and the
grade-limit checks), the materialization with its edited-occurrence
detection, the roster maintenance and the tenant recurrence gate to the
Timetable owner. The public contracts are `timetable.TemplateAdministration`,
`timetable.MaterializationCapability`, `timetable.RosterMaintenance` and
`timetable.RecurrenceWriteLock` (one capability for both gates, recurrence
first and grade transitions second), with `timetable.OfferingRosterResyncInput`,
`timetable.ErrOfferingSourceInvalid` and the template errors beside them. The
implementation lives in `modules/timetable/compose`, takes the advisory
locks through the owner's Postgres adapter and asks the School Calendar for
the A/B-week decision directly. The collaborators the owner may not name are
consumer-owned ports bound at the root: `repositories.TimetableTemplateRows`
hands over the retained rows, `services` binds School Structure's grade
range and class-name identity, Enrollment's care-offering checks and roster
resync, the instance lifecycle's deviation snapshot for the split and the
realtime staffing announcement. `services.Factory.TimetableData` carries the
template administration, the recurrence gate, the planner reads, the
conflict detection and the retained attendance correction (slice S3) to the
root, and `TemplateSplit` is gone, because the factory may not grow a field.
Enrollment, the scheduler, `api/timetable` and the composition root no
longer name the nest for these paths; the retained instance lifecycle and
the conversion of one occurrence into a series (slice S1) consume the public
contract. The moved code wraps its steps in a compose-local `ScheduleError`
with the unchanged message format, because the owner may not name Care Plan.
Thirteen emptied compatibility rules are deleted, one replacement permission
uses the exception of ADR 0038, and `legacy.jsonl` is unchanged.

Slice S3 (#3553, policy epoch 28 to 29) moved the deviation, substitution and
sick-report writes (the single-day Vertretungsplan save, the
Sammel-Vertretung and the #1843 sick stamps), the substitute time-overlap
advisory, the attendance correction of completed blocks and the Student
Presence attendance mirror to the Timetable owner. The public contracts are
`timetable.StaffDeviations` (with `timetable.SubstituteConflictQuery`),
`timetable.AttendanceCorrections` and `timetable.AttendanceMirror`, over the
owner's own types (`timetable.DeviationError`, `timetable.TouchedActivities`,
`timetable.AttendanceVisit`, the correction sentinels); the implementation
sits in `modules/timetable/compose` and takes the day-wide staffing lock
through the owner's Postgres adapter. The attendance mirror has exactly one
runtime write owner: Student Presence's `studentpresence.AttendanceSyncer`
port is bound at the root (`services.NewTimetableAttendanceMirror`) to the
Timetable command, which runs inside Presence's tenant transaction with the
lock order unchanged, so a failing mirror still rolls the visit write back.
Collaborators the owner may not name are consumer-owned ports bound at the
root: the Audit Platform's Änderungsprotokoll and correction trail
(`repositories.TimetableDeviationProtocol`,
`repositories.TimetableAttendanceCorrectionTrail`), the retained instance
lifecycle for the cancel branch and the understaffed acknowledgement, and the
realtime activity update. `services.Factory.TimetableData` carries the
deviation writes and the correction to `api/timetable`; the shift-plan-sync
workflow's sick cascade and Terminvertretung write through its own ports onto
`timetable.StaffDeviations`. The standalone understaffed acknowledgement, the
staff move and the Änderungsprotokoll entries of lifecycle writes stay with
the retained lifecycle (slice S1), which asks the owner for the substitute
advisory. Five emptied compatibility rules are deleted, one replacement
permission uses the exception of ADR 0038, and `legacy.jsonl` is unchanged.

Slice S1 (#3554, policy epoch 29 to 30) moved the instance lifecycle and
deleted the nest. The transitions (start with the absorption of
unsupervised sessions, completion with its snapshot, reopen, cancellation
with the guardian notice, delete), the planning writes (create with its
idempotency key, planned edit, re-plan with the deviation snapshot and
reapply), the staff move, the standalone understaffed acknowledgement, the
series conversion and the scheduler's automatic start and end are the
public contracts `timetable.InstanceLifecycle`, `timetable.InstancePlanning`,
`timetable.InstanceStaffing` (together `timetable.InstanceLifecycleCapability`),
`timetable.InstanceSeriesConversion`, `timetable.InstanceAutoStart` and
`timetable.InstanceAutoEnd`, over the owner's own types
(`timetable.LifecycleInstance` and the lifecycle inputs, results and
sentinels). The implementation lives in `modules/timetable/compose`
(`instance_*.go`) and still writes the retained repository rows; it serves
the lifecycle ports of the operational day and the deviation writes and the
template split's deviation machinery itself, so those bindings left
`services`. The collaborators the owner may not name are consumer-owned
ports bound at the root (`services/timetable_lifecycle_composition.go`): the
Facilities rooms through `repositories.TimetableOwnerRows.LifecycleRooms`,
Care Plan's care days and day locks, Communication's cancellation notice
(`compose.GuardianNotices`, whose refusals arrive wrapped in the owner's
notice sentinels), the Settings Platform's clock policy, the Audit
Platform's Änderungsprotokoll, Security Runtime's content fingerprint for the
idempotency key and the realtime hub (`compose.LifecycleBroadcaster`, whose
event types mirror the Delivery Platform's vocabulary). School Structure's
class rules reach the root through `modules/schoolstructure/compose`
instead of the nest. The group live view and the students inbound read the
planned-student lookup through consumer-owned ports. `services.Factory`
keeps its four lifecycle fields with owner types. The nest's behaviour tests
moved to `modules/timetable/compose` and `compose/httpintegration`; the SQL
test providers of `modules/timetable/legacy/timetablesqltest` were deleted
with their tests, and the tests of the bound repositories moved beside
them. The Dienstplan services and the shift-plan-sync workflow no longer
speak the retained `models/schedule` rows, so the seven Workforce and
shift-plan-sync rules to them fell too. `api/timetable` no longer imports
`models/schedule`, `models/activities`, `models/base`, `internal/timezone`,
`internal/schoolclass` or `realtime`: the thirteen #2732 keys left
`legacy.jsonl` (the fourteenth, the internal-test `models/education` import,
had gone with slice S5), and the vocabulary the handlers borrow is mirrored
in the public contract and pinned against the models. Every rule that named
#3424 is deleted, the package entries of both packages are gone, and ADR
0038's exception ends with them; fifteen of its replacement rules remain as
ordinary permissions to the owners' public contracts.

#2732 closed the carrier of `api/timetable`: its 27 remaining keys, the
root's `api -> api/timetable` key (#2750) and the end-to-end fixture's
`test/e2e/timetable -> api/timetable` key (#2748) are gone (571 -> 542). No
route path, status code, error body, middleware chain or authorization check
changed; the middleware golden only renames the package and the spelling of
two inlined closures.

- The routes moved file for file to `modules/timetable/http` (package
  `timetablehttp`), classified `timetable-activities`/`http` with
  `adapter-test` for both test roles. PR mode rejects an owner change of an
  existing path and a new permission on the existing `inbound-timetable`/`http`
  point, so the `inbound-timetable` owner is deleted with its package entry.
  Its six production rules and the adapter tests' compose rule moved to the
  new point, the school portal's two rules now target it, and the two
  test-construction rules to Student Presence and Care Plan compose had no
  import left and are deleted. The new point's other target
  dependencies are the ones the other owners' HTTP adapters hold: `api/common`,
  the session token adapter (ADR 0031, also for the internal tests), the
  permission registry, the public contracts of Security Runtime and Settings
  Platform, and the tenant runtime. The root, the school portal and its tests,
  and the timetable end-to-end fixture mount the new point.
- The legacy imports became public contracts or consumer-owned ports.
  `modules/settings` gains the timetable setting keys, the request-scoped
  `Resolver` and the `Resolve*OrDefault` helpers; `modules/securityruntime`
  gains `CanReadStudent`; the class-day projection and the plan export name
  their file formats; `common.ProtectedTenantRoutes` and the new
  `common.ProtectedSchoolRoutes` replace the route groups that took a
  `*bun.DB` they never used. The People Directory reads are the routes' own
  `People` port in builtin types, the Enrollment support of the Regeltermin
  editor is `timetable.OfferingSourceSupport` in the owner's public contract,
  and the school portal's supervision sheet is the `SupervisionSheets` port,
  whose refusal the route recognises by a marker method.
  `services/timetable_http_ports.go` binds the three over the retained
  services at the root.
- The route suites import neither repositories nor retained models. Their
  composition of the owner moved to `services/timetable_http_test_helpers.go`
  behind `api/testutil`, rollback probes use `test.TransactionProbe`,
  correction trails are read through the owner's `GetAttendanceCorrections`,
  deviation events through a row type of the suite, and the database-free
  suites fake the ports. The pin of the Audit Platform vocabulary the owner
  mirrors became a compile-time guard in
  `database/repositories/timetable_audit_ports.go`, the one package that
  binds both vocabularies.
- The move put the handlers under the module ratchets (Rule 16): `Router`
  and fourteen handlers were split into named steps, and `api.go` and
  `instances_list.go` fell below 800 lines. No allowlist entry was added.

#3559 (G1 of the `services/enrollment` dissolution under #2733) moved the
care-offering catalog to its owner. The admin catalog, the timetable-link
validation, the materializability, calendar-period, room, timeframe and phase
guards, the offering-source guard and the rollover clone now live in
`modules/careplan/internal/application/care_offering_*.go` behind the ports in
`internal/ports/care_offering_catalog.go`, composed by
`compose.NewCareOfferingCatalog`. The public contract is
`careplan.CareOfferingCatalog`, `CareOfferingGuards`, `CareOfferingRollover`
and `CareOfferingLinks` (`care_offering_catalog.go`, `care_offering_links.go`),
with capability-specific method names because `contracts.generic-crud` refuses
`List`/`Create`/`Update`/`Delete`/`GetByID` on a public contract. Error texts,
statements and transaction boundaries are unchanged; the key count stays at
542 and no rule was added.

- The catalog reads the Timetable, the School Calendar and Enrollment only
  through its own ports; `api/care_plan_offering_catalog.go` binds
  them over the owners' public contracts, the settings service, Enrollment's
  translation rules and the Timetable owner's offering-source refusal
  (`timetable.ErrOfferingSourceInvalid`), which the catalog therefore never
  imports. The ports call the same owner queries the retained activity,
  schedule, calendar-period, timeframe and exception repositories delegated
  to, with the same filters. Tenant rollback on a rejected update comes from the tenant runtime
  in Care Plan's compose.
- The decision service that resyncs sourced rosters and the pickup projection
  is still composed after the catalog. Instead of the two setters the catalog
  had, it resolves the service through a factory-local variable at call time,
  so the composition surface fell from 620 to 618.
- The enrollment routes and the retained enrollment services still speak
  enrollment rows. Only Enrollment's own packages may name `models/enrollment`,
  and `inbound-enrollment`/`http` has no permission to `care-plan`/`public`
  (PR mode rejects adding one on an existing point), so `api/enrollment`
  declares its own `CareOfferingCatalog` port in rows, bound at the root to
  `services/enrollment.CareOfferingRows` over the owner. For the same reason
  the row translation of care offerings and offering-change requests
  (`services/enrollment/care_plan_offering_records.go`) stays with its
  consumers until G2 to E4 move them. `services/enrollment` keeps its error
  names pointed at the owner values, as #3558 did.
- Booking materialization in `services/enrollment` calls the moved link rules
  through `careplan.CareOfferingLinks` and the pure
  `careplan.SchedulesOverlapPhase` / `ValidatePhaseWithinPeriod`, instead of
  a second copy. The unused legacy sentinels
  `models/enrollment.ErrCareOfferingInvalid`,
  `ErrCareOfferingPickupTimesRequired` and
  `models/schedule.ErrCalendarPeriodCareOfferingConflict` are gone.
- The catalog's behaviour suites stay in the `services/enrollment` test
  package, which already reaches the retained fixtures, and compose the owner
  through `api/testutil.NewCareOfferingCatalog`; the pure booking-stats and
  availability tests run against fakes in the application package.

#3560 (G2 of the `services/enrollment` dissolution under #2733) moved the
booking materialization to Care Plan. The roster rows an approval derives
from a child's bookings, the offering-sourced Regeltermin resync, detach and
editor support, the Regeltermin roster indicator, the dated offering
adjustments and the offering pickup times now live in
`modules/careplan/internal/application` (`booking_*.go`, `sourced_*.go`,
`offering_adjustment*.go`, `offering_pickup_times.go`,
`offering_source_editor.go`, `template_roster_feeds.go`) behind the ports in
`internal/ports/booking_materialization.go`, composed by
`compose.NewBookingMaterialization` over the catalog of #3559. The public
contract is `careplan.SourcedRosters`, `OfferingSourceEditor`,
`BookingMaterializer`, `OfferingAdjustments` and `OfferingPickupTimes`
(`booking_materialization.go`), plus the pure rules
`DeriveTemplateRosterMaintenance`, `ExplainEmptyOfferingRoster` and
`ExcludedAutoTargetOverrides`. `careplan.CareOfferingLinks`, which only
served the materialization while it lived in `services/enrollment`, is gone.
Error texts, statements and transaction boundaries are unchanged; the key
count falls from 542 to 540 and no rule was added.

- The materialization writes `activities.student_enrollments` and reconciles
  `schedule.instance_students` through ports that
  `api/care_plan_booking_materialization.go` binds to the Timetable
  owner's `StudentEnrollmentCommand` and roster maintenance, the way
  `NewCareLifecycle` already works. It reads Enrollment's requests, children,
  phases, booked selections and the approved-booking projection, People
  Directory students and locks, the School Calendar periods, the settings and
  Audit Platform's adjustment trail through the same binding; Care Plan's own
  bookings, pickup rows and withdrawal follow-up are passed in directly.
  Every write joins the ambient tenant transaction. The recurrence gate is
  the Timetable owner's; the class-writes gate and the student lock are People
  Directory's, in the order every care writer takes them. The pickup
  announcement is deferred to commit in Care Plan's compose; the services
  binding only broadcasts.
- The Jahrgang of a child comes from Enrollment's rule
  (`enrollment.SchoolClassGradeLevel`) through the ports, because Care Plan's
  application may not import School Structure's class grammar.
- The decision, change-request and offering-change services still speak
  enrollment rows. They drive the owner through
  `services/enrollment.DecisionBookings` and translate its results in
  `care_plan_bookings.go`; `services.Factory.EnrollmentCareOffering` now
  carries the catalog together with the materialization
  (`careplan.CareOfferingCapability`), so the Timetable hook, the grade
  transition, the phase service, the Regeltermin editor, the student class
  resync and the pickup reset route reach it without a new field. The
  intake-side selection helpers the retained request, change-request and
  offering-change services share with enrollment rows
  (`intake_offering_rules.go`, `changeCareBookings`) and the decision's own
  targeted-field pickup hooks stay with those services until E3, E4 and G3
  move them. The offering-change decision validates excluded co-booking
  targets through `careplan.ExcludedAutoTargetOverrides` instead of its own
  copy.
- The legacy repository methods only the materialization called
  (`FindTemplatesBySourceOffering(s)`, `FindTemplatesWithOfferingSource`,
  `BackfillEnrollmentRequestChildSource`, `DeleteByEnrollmentRequestChild`)
  are deleted. The behaviour suites stay in the `services/enrollment` test
  package and compose the owner through `api/testutil.NewBookingMaterialization`.

#3561 (G3 of the `services/enrollment` dissolution under #2733) moved the
offering-change, course-request and pickup-adjustment reviews to Care Plan.
The parent's switch request, its staff decision and preview, the edit and
withdrawal, the conflict resolver side, the course request with its
waitlist, the staff-side direct offering correction and the permanent
pickup-time adjustment now live in `modules/careplan/internal/application`
(`offering_change*.go`, `course_request*.go`,
`direct_offering_adjustments.go`, `pickup_adjustment*.go`) behind the ports in
`internal/ports/offering_changes.go` and `internal/ports/pickup_adjustments.go`,
composed by `compose.NewOfferingChanges` and `compose.NewPickupAdjustments`.
The public contract is `careplan.OfferingChangeRequests`, `CourseRequests`,
`OfferingChangeConflicts`, `DirectOfferingAdjustments` (together
`OfferingChangeCapability`) and `PickupAdjustments`. Routes, status codes,
error texts, authorization and tenant scoping are unchanged; the key count
falls from 540 to 535 and no rule was added (one stale test rule is gone).

- `services/offering_change_composition.go` and
  `services/pickup_adjustment_composition.go` bind the ports over Care Plan's
  own request rows, Enrollment's care periods, children, phases, selections,
  catalog state and capacity peak, People Directory students and locks, the
  withdrawal follow-up, the settings, the review scope of the caller's JWT
  permissions, the parent-messaging ledger and the co-guardian notices. The
  course groups of the manual planning come from the Timetable owner
  (`services/offering_change_planning.go`). Every write joins the ambient
  tenant transaction; the pickup adjustment joins it through Care Plan's
  compose (`tenant.WithTenantTx`) and marks it for rollback the way the
  retained service did.
- Approvals and direct corrections reach the booking materialization of #3560
  through `careplan.OfferingAdjustments` with the request or direct source;
  `services/enrollment` keeps no offering-change, course-request, pickup or
  direct-correction rule. The pickup adjustment token is the Security Runtime
  fingerprint; it is a content fingerprint, not a secret, so the comparison no
  longer needs `crypto/subtle`.
- `services.Factory` loses the `OfferingChanges` and `PickupAdjustments`
  fields: `EnrollmentCareOffering` (`careplan.CareOfferingCapability`) now
  carries both reviews, so `api/students`, the parent portal and the conflict
  coordinator reach them without a new field (composition surface
  618 -> 615). The lifecycle sentinels are translated to the
  `services/users` ones in a services decorator so rendered texts stay
  byte-identical; the conflict coordinator's staff value is parsed there too,
  so the public contract stays typed.
- The legacy queue methods (`ListPending`, `ListHistory`, `PendingCount`,
  `ListDirectCorrections`) had no production caller since the native request
  review (#3179) and are deleted. The behaviour suites stay in the
  `services/enrollment` test package, compose the owner through
  `api/testutil.NewOfferingChanges` and assert the staff queue through
  `api/testutil.NewOfferingReviewQuery`.

#3562 (E1 of the `services/enrollment` dissolution under #2733) moved
Enrollment's phases, form schemas, the phase-response overview, the
phase-expiry warnings, the approved-offering projection, captcha
verification and the parent mail renderers and decision notifications into
`modules/enrollment/internal/application` (`enrollment`/`application`, the
same point as the retained package), composed by `modules/enrollment/compose`
and published through `modules/enrollment` (`PhaseAdministration`,
`FormSchemaAdministration`, `PhaseExpiryWarnings`, `CaptchaVerifier`,
`Notifications`, `MailRenderers`). The Turnstile call is the adapter
`modules/enrollment/internal/adapters/turnstile` (`enrollment`/`adapter`).
Routes, status codes, error texts, authorization and tenant scoping are
unchanged; 21 files are deleted and the key count falls from 535 to 529
(`email`, `pgdriver` and `net/http` in production, three test keys).

- The composition could not construct its application under an ordinary
  `enrollment.compose.application` rule, because `services/enrollment`
  already holds that point at the base. ADR 0041 grants exactly that
  production permission in epoch 31 while the retained package exists.
- Settings reach the application through typed ports (`CollectionSettings`,
  `CaptchaSettings`, `NotificationSettings`) that `services` binds over the
  settings resolver; the captcha keeps its deployment fallbacks there. Mails
  leave as `application.Mail` intents that `services` puts on the platform
  outbox; the renderers take the stored JSON payload.
- The Postgres adapter marks a duplicate phase name and a missing form schema
  or calendar period with `ErrPhaseNameTaken` and `ErrPhaseReferenceMissing`,
  so neither the phase administration nor the rollover reads driver errors.
- The decoded request and child glue (`services/enrollment/owner_records.go`)
  stays with the retained decision, rollover and intake flows until #3564 and
  #3565 move them: the public contract may not carry the decoded answer maps.
- The guardian portal reads care periods and offering history through its own
  ports (`care.CarePeriodReads`, `care.OfferingHistoryReads`), which removes
  the two parent-portal rules to the retained Enrollment application.
- The behaviour suites stay in the `services/enrollment` test package and
  compose the owner through `modules/enrollment/enrollmenttest`; the external
  `api/enrollment` router suites reach it through exported helpers of the
  package's internal tests.

#3563 (E2 under #2733) moved the school portal's class day report, the
supervision sheet and the class-wide arrival exceptions into the Class Day View
application (`modules/classday/internal/application`, behind the one
`classday.ClassDay` capability, which gained `SupervisionStudentSheet`), and
the care usage and class roster reports with their export audit into
Enrollment (`modules/enrollment/internal/application`, published as
`enrollment.Reports`, composed by `modules/enrollment/compose.NewReports`).
The five retained files are deleted; routes, status codes, error texts, the
report and export output, the GDPR access-log rows, authorization and tenant
scoping are unchanged. The key count falls from 529 to 523
(`internal/collation`, `internal/sliceutil` and `models/education` in
production, three test keys), and eight rules that no import used any more are
deleted, among them `class-day-view.compose.enrollment-application` and
`parent-portal.application.enrollment-application`. No rule is added.

- Enrollment publishes the class roster of a day
  (`Reports.ClassRosterDay`: covering phases, merged roster rows and each
  student's departure modes for the weekday). Class Day View reads it through
  its own `DayRosters` port, which `services` binds; a direct
  `class-day-view.compose → enrollment/public` import would have been a new
  permission.
- Neither application may import People Directory's domain, the audit or
  settings models, the realtime hub or the tenant runtime. Students, persons,
  guardian contacts, status days, the care-offering setting, the GDPR access
  log and the after-commit arrival announcement reach them through ports
  `services` binds; Care Plan's effective times, care days and class arrival
  exceptions reach Class Day View through its compose bindings, which also
  keep Care Plan's refusal texts on the wire. Group names come from the
  retained group repository through `repositories.NewGroupNames`.
- The companion link helpers (`FilterCompanionLinksToDays`,
  `CompanionDisplayName`, `FormatCompanionLinks`) moved from `models/users` to
  People Directory's departure contract (`modules/peopledirectory/departure`),
  aliased from `models/users` and the People Directory facade.
- The reports sort with the settings of `internal/collation` through
  `golang.org/x/text/collate`, which the policy keeps out of an owner's
  application; a table test pins the order against the shared cases.
- `BookingViewDate` moved to Care Plan (`careplan.BookingViewDate`), so the
  guardian portal no longer imports the retained Enrollment application;
  `ReportOfferingDate` is public in `modules/enrollment` for the retained
  decision flow, which keeps its own export audit writer until #3564.
- `services.Factory` keeps its field names: `EnrollmentReport` now holds
  `enrollment.Reports` and `ClassDayArrivalExceptions` the whole
  `classday.ClassDay` capability, so the composition surface does not grow
  (615 → 610 targets).

#3564 (E3 under #2733) moved the decision flow, the restore of a withdrawn
request, the rollover with its review queue and deadline worker, the admin
deletion with its impact preview and the retention cleanup of rejected
requests into Enrollment (`modules/enrollment/internal/application`). The
public package publishes them as `enrollment.Decisions`,
`ApprovedChildChanges`, `Rollovers`, `EnrollmentDeletions` and
`RejectedEnrollmentCleaner`; `modules/enrollment/compose` composes them
(`NewDecisions`, `NewRollovers`, `NewDeletionPreview`, `NewDeletions`,
`NewRejectedCleanup`) in the ambient tenant transaction. The thirteen retained
files and three helper files are deleted; routes, status codes, error texts,
authorization and tenant scoping are unchanged. The key count falls from 523 to
513 (`models/audit`, `models/base`, `models/schedule`, `realtime`,
`services/import` and `bun.DB.RunInTx` of `services/enrollment` in production,
`models/audit` of `api/enrollment` in both scopes, two test keys). No rule is
added.

- The capacity gate the submissions share with the restore is the public
  `enrollment.OfferingCapacity`, composed by `compose.NewOfferingCapacity`
  and bound into the retained intake by `services`. The request sentinels,
  the status token (`NewStatusToken`), the submission lock key
  (`SubmissionDedupLockKey`) and `NormalizedSubmissionSource` moved to the
  public package with it.
- The public contract carries the owner's raw request and child values; until
  #3565, `services/enrollment` keeps a decoding facade (`NewDecisionService`,
  `NewRolloverService`, `NewChangeRequestDecisionApplier`) for the retained
  intake, change-request and scheduler consumers, and a copy of the answer
  decoders its visibility checks read.
- People Directory rows, the audit trails, the settings, the realtime hub and
  the outbox reach the application through ports `services` binds
  (`NewEnrollmentDecisions`, `NewEnrollmentRollovers`,
  `NewEnrollmentDeletionModule`); the weekly pickup and arrival schedules come
  from `repositories.NewEnrollmentPickupSchedules` /
  `NewEnrollmentArrivalSchedules`. Role presets reach the binding as a typed
  permission list through `securityruntime.StudentGuardianRolePreset`.
- The rollover opens its transaction through the tenant runtime instead of
  `bun.DB.RunInTx`; the postgres adapter marks a child insert refused by the
  rollover source index with `ErrRolloverSourceChildTaken`.
- The care offering record adapter left in `services/enrollment` keeps the
  retained error shape and not-found marker without `models/base`.
- Suites compose the flow through `api/testutil` aliases of the root
  bindings; the savepoint suite of the retention cleanup runs in
  `modules/enrollment/integration`.

#3565 (E4 under #2733) moved the parent intake (submission, edit, withdrawal,
renewal confirmation, status link, public form loads, late invites and the
manual approved enrollment) and the change requests with their review list
into Enrollment (`modules/enrollment/internal/application`). The public
package publishes them as `enrollment.IntakeSubmissions`, `IntakeStatus`,
`IntakeForms` and `ChangeRequests`; `modules/enrollment/compose` composes
them (`NewIntake`, `NewChangeRequests`) in the ambient tenant transaction.
`backend/services/enrollment` is deleted with its `enrollmenttest` helper and
its goldens; routes, status codes, error texts, authorization and tenant
scoping are unchanged. The key count falls from 513 to 468 (every remaining
`services/enrollment` key, among them `crypto/rand`, `crypto/sha256`,
`database/sql`, `internal/schoolclass`, `internal/strutil`, `services/config`,
`services/users` and `bun`, and the consumer keys of `api`, `api/enrollment`,
`api/students`, `modules/careplan/inbound/parent`, `services` and
`services/scheduler`); #2733 holds no key any more. Fourteen rules no import
used are deleted, among them `parent-portal.compose.enrollment.domain`, the
`enrollment.behavior-test.*` rules of the retained test package and
`people-directory.test-support.compose`, whose `peopletest` package lost its
last caller and is deleted. No rule is added.

- The public contract carries answers, consent flags and source metadata as
  raw JSON. `api/enrollment` decodes them for its handlers and the parents
  portal (`owner_intake.go`, `owner_change_requests.go`, `owner_decisions.go`,
  `owner_rollovers.go`) under the type names the retained package exported.
- Refusals that originate in the selection contract or in Care Plan come back
  marked with Enrollment's public mirrors (`intake_errors.go`), so the
  handlers classify them with `errors.Is` while the text stays the
  originating one; the decision flow reaches the routes through
  `compose.PublicDecisions`.
- Settings reach the intake as the typed `IntakeSettings` port; random bytes
  and fingerprints as functions `services` binds from `securityruntime`;
  guardian profiles, phones and reviewer names through ports; Care Plan's
  offerings through `compose.NewCareOfferingRecords` and its bookings through
  `enrollment.CareBookingCommands`.
- The capacity seam of #3564 is a direct call; the scheduler reads the
  retention cleanup through its own port.
- The public change-request operation is `Propose`, not `Create`: public
  contracts use capability-specific operations.
- Suites: the unit suites run in `modules/enrollment/internal/application`
  and `modules/enrollment/selection`, the intake, change-request, decision,
  rollover and deletion suites in `api/enrollment`, the care-offering and
  offering-change suites in `modules/careplan/contracttest`. ADR 0041's
  exception grants nothing once this merges, because its anchor package is
  gone from the base.

The import HTTP composition (`modules/dataimport/inbound`, with its runtime
binding in `modules/dataimport/inbound/compose`) keeps the `inbound-import`
owner and its `http` / `compose` roles after replacing `api/import` (#3217).
It serves the student, staff, class-list and opening-balance preview, import
and template routes from the public Data Import contract with unchanged paths,
status codes, error strings, multipart handling, file-size limit and
permission checks. The `root-composition.to.import-http` mount replaces the
`api -> api/import` debt entry that fell with the old package; it and the
`inbound-import.compose.*` rules that bind `api/common`, `modules/identityaccess/sessiontoken`, `tenant`
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
(`sessiontoken`), the one Identity & Access package `api/common` and the
legacy composition already reach, so `api/common` keeps attaching it
router-wide and group-wide as before and the root binds it as the caller
context's memo; the memo's contents stay in the application.

The settings test support (`services/config/settingstest`, `settings-platform`/
`test-support`) scripts the payroll and work-schedule settings that presence
behaviour tests drive through their real services, so those tests name a school's
configuration instead of registry keys and ORM rows. Its `models/config` import is
a target dependency of the settings owner. Its calendar dates are the
shared-kernel `sharedkernel/calendar` type under the shared-kernel rule
([ADR 0040](../../docs/adr/0040-every-role-may-import-the-shared-kernel.md),
#3448), so it no longer reaches the retained `internal/timezone` wrapper.

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
`inbound-operator.http.communication-http`). The operator error
constructors and the operator-audited id action live in `api/common`
(`Operator*`, `OperatorAuditedIDAction`), so every half of the operator
surface renders one wire format, the shared error envelope since #2507;
#3231 removed the thin `Err*` delegations `api/operator` kept until then.
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
  `modules/peopledirectory/compose.CaregiverCapabilityViews` since #3754), `RoleGrants` (Security
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
satisfies `InstanceCompletion` with the Timetable owner's
`timetable.EndedSessionCompletion` since slice S5 of #3424 (#3551; the
attendance finalization that #1747 requires before an instance may close),
which replaced the retained `TimetableBridgeService`. The kiosk
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
#3421. Its second stage retired the last of them (20 after later carriers had
taken nine, 81 -> 61): calendar dates come from `sharedkernel/calendar`; the
retained People Directory, Care Plan and Enrollment values the workflow hands
back are named through the workflow port's vocabulary, declared once in the
flow package that returns them; the enrollment profile port returns the HTTP
values the response renders, translated at the root; the locale check uses
Delivery's notification copy; the enrollment routes open their transactions
through `tenant.WithinTenant` and `tenant.WithinAdmin` and the resource holds
no database; the handler suites take setting keys from `modules/settings`,
compose through `api/testutil` and leave viper to the fixture package. The
shared HTTP runtime, the session adapter and the tenant runtime are standing
reaches of every HTTP point, so [ADR 0051](../../docs/adr/0051-guardian-portal-http-reaches-the-standing-http-runtimes.md)
grants them in epoch 39 as four rules (`inbound-parent.http.inbound-common`,
`inbound-parent.http.identity-adapter`, `inbound-parent.http.tenant-runtime`,
`inbound-parent.adapter-test.identity-adapter`), and the module HTTP ORM
ratchet loses both of the package's entries. `root-composition.to.inbound-parent-http` (#2750) is
still a compatibility permission; convert it to exact debt with the rule
above under its issue. `test-support.e2e-test.inbound-parent-http` is gone
(#2748): the calendar end-to-end suite mounts the guardian portal through
`api.CalendarTestModule.ParentCalendarRouter` and no longer names the adapter. The move replaced
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
and never writes. Its calendar days use the shared kernel. Its composition
(`modules/planexport/compose`, `document-rendering`/`compose`) binds those
ports to reads in the owners' public vocabulary and maps their records field
by field: Workforce for the staff week and the Schichtarten, Timetable &
Activities for the blocks, their staff, head counts, Angebote and
Planungsspuren, Facilities for the room names. The Schichtarten, Planungsspuren
and rooms are public capability calls; the root serves the block, staff,
Angebot and head-count reads from its retained repositories in the Timetable
vocabulary (#3424), as it does for the Workforce Dienstplan, so they leave
with the repository Factory (#2743). The root also binds the staff names,
closing days and holidays it already serves to other consumers. The former compatibility adapter
`modules/planexport/legacy` and its eleven baseline entries are deleted, and
the composition's four `document-rendering.compose.*` rules are target shape.
The composition tests (`adapter-test` seam) pin the translation and prove the
Betreuungsplan's tenant isolation and runtime; the Dienstplan's isolation and
runtime evidence live in Workforce's behaviour suite
(`modules/workforce/compose`), because the overview's staff and room ports
still name retained People Directory and Facilities rows. The retained
`services/listexport` renderer keeps its `module-internal-test` seam, so the
capability's rendering tests declare the `workflow-decision-test` seam.
The `inbound-timetable.to.document-rendering` and
`workforce.http.document-rendering-public` edges are the target shape (an
inbound adapter calling the public capability) and stay.
The same change moves the birthday routes to `modules/birthdays/http`
(`inbound-birthdays`/`http`) and the document upload coordinator to
`modules/filestorage/documents` (`file-storage`/`adapter`). Their
compatibility bindings, the birthday handlers' retained user-context, birthday
service and birthday row imports and the coordinator's retained storage
backend, were exact debt under #2706 as well; the coordinator now declares
its own private object-store port, bound to the shared uploads backend; the remaining `inbound-birthdays.*`
permissions are the inbound target shape. #3751 later moved the birthday
service into People Directory and the routes to
`modules/peopledirectory/inbound/birthdays`; the `inbound-birthdays` owner and its
rules are retired. The
`inbound-students.to.file-storage-adapter` binding is different: its source
package existed before the move and imported the old path as debt under
#2731, which PR mode cannot carry over to the new target. It is a
[named exception](https://github.com/moto-nrw/project-phoenix/issues/2580#issuecomment-5638973300)
to rule 5 of `backend/CLAUDE.md`. #2731 re-anchored it on the People
Directory student routes as `people-directory.http.file-storage-adapter`
without widening it: the public File Storage capability offers no child
document storage, and adding it is capability work a Contract step may not
do. It is tracked by #2706 with the rest of the document coordinator's debt;
it goes when the student document handlers move to the public File Storage
capability, and no new caller may rely on it. The file store handlers' equivalent exception went
with #2707. The generic file-metadata repository
(`database/repositories/documents`) and model (`models/documents`) are
deleted with their five entries (ADR 0045).

The File Storage capability (`modules/filestorage`, `file-storage`/`public`,
#2707) owns the school file storage and the attachments of
Elternmitteilungen: folders with their visibility rule and share lists, the
files inside them, the storage quota, the audit trail, the intent protocol
that makes an interrupted upload recoverable, and the sweep the worker runs.
`internal/application` holds the authority and upload rules,
`internal/adapters/postgres` serves `documents.folders`,
`documents.folder_roles`, `documents.folder_accounts`, `documents.files`,
`documents.file_cleanup`, `documents.announcement_attachments` and
`documents.announcement_attachment_cleanup` with static table names, and
`compose` binds the ports: membership, account roles and shareable roles to
the public Identity & Access capability, the audience picker's names to the
public People Directory query (both target shape), and, as compatibility
permissions, the two file settings, the retained Audit file-event contract,
the shared permission matcher and the retained storage backend; since #2706
the `documents.file_cleanup` intents are the owner's own adapter. The
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
is classified as `device-fleet`/`http`. School lookup and Settings use existing
consumer ports bound by Root. SQL not-found and transient error classification
live in Device Fleet's Postgres adapter and reach the authenticator through its
`SchoolErrors` port, bound by Root's owner composition. No HTTP package receives
an ORM-class grant. Display credential generation uses the existing token port;
its byte-identical mint/hash implementation lives in a credential adapter, so
the cryptographic class grant is limited to production adapters, not HTTP.

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
#2745 deleted `api.API`; the guard keeps its name so the aggregate cannot
return, and `api.TestServeGraphFieldsAreShrinkOnly` keeps the field set of
its replacement, the package-private `serveGraph`, shrink-only.
#2743 deleted `database/repositories.Factory` and `NewFactory`: their
symbols left `legacy_composition` in `policy.json`, the guard keeps the name,
and each retained repository is built where it is used.
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
- every call to the evidence-only `api.New` (deleted by #2745) and
  `api.NewServer`, the current
  `api.WithRuntime`, the evidence-only `repositories.NewFactory` (deleted by #2743),
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

#2738 closes the `api/common` carrier without deleting the shared HTTP runtime.
[ADR 0046](../../docs/adr/0046-api-common-is-the-shared-http-runtime.md)
keeps its owner and role and defines eight standing rules for the existing
session, tenant, security and settings contracts and the native byte-storage
and SQL-failure capabilities. The epoch-gated comparison accepts only their
finite selector shapes. Implementation imports, owner-kind rules, expanded
scopes and ORM access remain forbidden. The DB-bearing route wrappers and the
old `internal/storage` provider are deleted; their callers use database-free
routes and Delivery's native objects. All 20 keys fall (262 → 242 on current development; 265 → 245 at the original task base), with
composition unchanged (596 → 596). The migration record and generated caller
inventory are `contract-common-http-2738.json` and
`callers/common-http-2738.json` (regenerate with
`scripts/run-go-toolchain.sh go run scripts/backend-common-http-callers/main.go`).

### Standing composition reaches (ADR 0053)

[ADR 0053](../../docs/adr/0053-owners-compose-themselves-and-the-roots-compose-the-owners.md)
(#4001, policy epoch 40 → 41) replaces per-pair rules with four standing
reaches. Each reach is a finite selector set with its own acceptance function
in `internal/architecture/composition_reach_rule.go`. The comparison accepts a
rule only in an increased epoch and only when everything the rule allows lies
inside one reach: a subset test over source owner or kind, source role, target
owner or kind, target role, `same_owner` and scopes. The first-party import
check accepts the imports such a rule implies. None of the rules carries an
`issue`; they are the target.

| Reach | Acceptance function | Selectors | Rules in epoch 41 |
|---|---|---|---|
| The typed roots (point 2) | `reviewedRootCompositionRule` | `root-composition`, any role and scope, to the `compose`, `public`, `contract`, `port` and `http` roles of every `domain`, `platform`, `workflow` and `projection` owner, to `migrations`/`migration`, and to the root's own packages | 23 `root-composition.reach.*` |
| An owner's composition (point 3) | `reviewedOwnerCompositionRule` | role `compose` of a non-composition owner, production, every role of its own owner | 27 `owner-compose.*` |
| A test of one owner (point 7.1) | `reviewedOwnTestRule` | the test scopes of an owner to its own `public`, `contract`, `compose`, `port`, `adapter` and `test-support`; its `test-support` role in production to the same roles except `test-support` | 30 `own-test.*`, 5 `own-test-support.*` |
| A test of several owners (point 7.2) | `reviewedRootTestRule` | the test scopes of every owner to `root-composition`/`compose` | 1 `root-test.domain.compose` |

Three constraints shape the rules:

1. The loader rejects statically overlapping rules. The generic rules
   `adapter-test.{domain,platform,projection,workflow}.public`,
   `adapter-test.{domain,platform}.contract` and the `external-test.*` rules
   of the `workflow-integration-test` role already grant those roles their
   own owner's `public` and `contract`. The rules into `public` and
   `contract` therefore name a source role, both for the root's test scopes
   and for an owner's own tests; the rules into the other roles do not.
   #4070 split the rules into a domain, projection or workflow role by
   source kind (`<kind>.adapter-test.domain.public` and so on), without
   the platform kind (ADR 0057).
2. The rules of points 3, 7.1 and 7.2 name a non-composition source owner
   kind, so they cannot overlap the root's rules: the roots reach their own
   packages through point 2. `reviewedOwnerCompositionRule` also rejects a
   composition-kind source, because the legacy composition gains no
   production reach (ADR 0053 point 1). The test reaches follow ADR 0053
   point 7 to the letter and accept every test package, including the
   legacy composition's external tests; Go keeps its internal tests from
   importing the root that imports it.
3. `rules.stale` keeps only live rules, so the rules cover the selectors that
   current imports use. The root had no `port` rule in epoch 41, for
   example. The first import that needs one adds it in a reviewed epoch
   under the same acceptance function, as #4005 did for
   `root-composition.reach.workflow.port`.

Epoch 41 deletes the 301 rules the reaches cover: 62 root rules into the
covered owner kinds (the 19 rules into `inbound` owners stay), 149 same-owner
composition rules, and 90 same-owner test and test-support rules, among them
the generic `internal-test.same-owner.{public,contract,compose,port,adapter}`,
`module-behavior-test.{public,contract}` and `workflow-decision-test.port`. Four
of them carried an issue: `root-composition.to.student-presence-http` (#2750),
`audit-platform.compose.retained-application`,
`audit-platform.compose.vocabulary-domain` and
`data-import.compose.retained-application` (#2747). Their edges are target
edges now.
`e2e-test.compose` narrows to `legacy-composition` and carries #2747 until
`services` is deleted.

The three root rules into implementation roles of other owners
(`root-composition.to.workforce-adapter`,
`root-composition.compose.production.data-import.adapter`,
`root-composition.compose.scheduler-runtime-postgres`) became exact debt under
[#4003](https://github.com/moto-nrw/project-phoenix/issues/4003). They yield
four tuples, because the scheduler rule covered `jobruntime` and
`workerlease`. Point 7.3 adds `identity-access`/`adapter` to the shared
fixture targets (`shared-fixtures.identity-access-adapter`).

#4005 (policy epoch 43 → 44) moves the legacy composition's test hooks out of
`services`. The hooks live in `api/*_test_helpers.go` and take the
Transaction Runtime value; Identity's composition builds the session codec
from the secret and the token lifetimes (`NewConfiguredSessionTokens`), so the
hooks import no session token adapter. Tests of the owner a hook or an alias
only passed through to now call or name that owner's composition themselves;
tests of other owners keep reaching it through the root, because they may not
import another owner's composition. Hooks and aliases no test used are
deleted. The two hook files production code used became ordinary production
files.
The imports the move needs come from the same acceptance functions:
`root-test.{inbound,platform,projection,workflow,test-support}.compose` and
`root-test.legacy-composition.compose` (the external tests of `services`)
under `reviewedRootTestRule`,
`own-test.legacy-composition.module-behavior-test.compose` under
`reviewedOwnTestRule`, `root-composition.reach.internal-test.self.compose`
and `root-composition.reach.workflow.port` under
`reviewedRootCompositionRule`, and `owner-compose.projection.adapter` under
`reviewedOwnerCompositionRule`. No reach is new: each rule is an instance of a
reach ADR 0053 grants (7.2 lets every test package import the root, 7.1 every
owner's tests its own composition, points 2 and 3 the root's and an owner
composition's reaches), and #4001 wrote only the instances the imports of its
time used. A hook that builds one owner sits in that owner's test support
when its consumers may import it: `careplantest.SchoolWideReviewScope`, the
parent message store in `communicationtest` and the Enrollment compositions in
`enrollmenttest`, which Care Plan's and the Parent Portal's suites reach under
their existing rules. Tests of the same owner call its composition directly.
A test of another owner without such a rule builds the root (7.2), so those
hooks stay in `api` and say why. `rules.stale` removed 13 rules no import used
any more, among them `own-test.projection.test-support` with the deleted
`modules/classday/classdaytest`. An internal test of a package the root
imports cannot import `api`: it declares the composition it needs as a
package variable that an external `root_composition_test.go` binds, or
reaches it through `modules/workforce/workforcetest` when its external test
may not import its own package. `api/testutil` keeps only owner-agnostic HTTP
helpers, and `api/testutil -> services` falls (#2748, 41 → 40).

#4006 moves the Identity & Access wiring out of `services` (ADR 0053 points 1,
4 and 8). The legacy composition still consumes the module, so the root
composes it over values the Factory hands over: `NewFactoryWithModules` takes
a `services.IdentityRoot` instead of the token adapter. The Factory calls its
`Compose` with what it builds itself: the settings, the audit command and
ledger, the mail transport, the identity chain rows, and the outbox and
delivery state read at call time. It gets back the module, the caller
context, the bindings of the owners that may not name the module, and the
three Identity mail renderers. Every seam that binds Identity to another
owner lives in `api/identity_*.go`. Those seams declare narrow row ports of
their own (`api/identity_rows.go`), so no repository type reaches the root
(Rule 1). The root binds the portal runtimes straight to the portal types,
so `modules/schoolportal/portaltest`, its two rules and the standing rule
`own-test.inbound.test-support` are gone. Identity owns the order of the
guardian welcome mail (`GuardianWelcomeOrder` behind the consumer-owned port
`compose.PrecedingMails`); the root only reports Delivery's state. The
cleanup CLI and the dev seeder build their Identity parts through
`modules/identityaccess/compose` (`NewSessionMaintenance`,
`NewInvitationMaintenance`, `NewGuardianSeedAccess`, `NewDemoAccessExpiry`).
The session sweep never signs or verifies a session, so its composition has
no signer. The Factory loses its ten Identity fields and seven Identity
methods (composition surface 250 → 240). `services` shrinks from 260 to 232
production files and from 35,473 to 30,454 LOC, and `services ->
sessiontoken` falls (#2747, 40 → 39). Declarations whose callers stay in
`services` moved into those callers' files. `api -> sessiontoken` (#2750)
stays: the root's middleware and claim reads still use the adapter.

### Reach of a domain owner's implementation (#4084)

The addendum #4084 to
[ADR 0058](../../docs/adr/0058-owner-dependencies-form-a-dag.md) (policy epoch
49 → 50) lets the code #4084 moves out of the owner compositions keep its
imports. `reviewedInternalReachRule`
(`internal/architecture/internal_reach_rule.go`) accepts, only in an
increased epoch and only in production:

| Shape | Rules |
|---|---|
| a domain's `application`/`postgres` into its own `public` | `domain.application-public`, `domain.postgres-public` |
| one domain's `application`/`postgres` into `tenant-runtime`/`public` or `transaction-runtime`/`public` | per owner, in the change that first uses it |
| one domain's `application`/`postgres` into another domain's `public`, when the base already grants the owner a production rule into it | per owner, in the change that first uses it |

Epoch 50 adds the two generic rules and deletes the nine per-owner rules
they cover, 945 → 938 rules: `care-plan.application.public`,
`care-plan.postgres.public`, `device-fleet.compatibility-application.public`,
`enrollment.application.to.enrollment.public`,
`enrollment.postgres.to.enrollment.public`,
`facilities.compatibility-application.public`,
`organization-tenancy.application-public`,
`student-presence.application.public` and `workforce.application.public`.
The existing per-owner rules into the tenant runtime and into other domains
stay; they already have the accepted shape.

Epoch 53 adds `timetable-activities.postgres.transaction-runtime`: the
Timetable composition stops importing `models/base` and `internal/sliceutil`
(both #4004 keys go). List options, the store error, the not-found sentinel
and the ID helpers are Timetable's own values in `internal/domain`; the
unique-violation classification is reached through the owner's postgres
adapter, which delegates to the transaction runtime.

Epoch 54 moves the staff notices (#2180), the Timetable templates, the
materialization, the series conversion, the roster maintenance and the
conflict detection (11,264 LOC) from the composition into
`internal/application`; the staff notice repository and its row mapping go
to `internal/adapters/postgres`. The template services take a
`TenantTransactions` port instead of the database handle; the composition
binds it to the tenant runtime. It adds
`timetable-activities.application.school-calendar.public`,
`timetable-activities.application.tenant-runtime`,
`timetable-activities.application.transaction-runtime`,
`timetable-activities.application.student-presence.public` and, for the moved
internal tests, `timetable-activities.module-internal-test.tenant-runtime`
(the internal-test mirror of the reach, ADR 0058 addendum point 4).

Without a new rule or epoch, #4084 then moves the instance lifecycle, its
automatic start and end, the status slots, the staff deviation writes, the
bulk substitution and the substitute conflicts (6,869 LOC) into
`internal/application`. The lifecycle takes a `TenantDatabase` port (tenant
transaction and advisory locks) and the staff deviations a
`TransactionLocks` port instead of the database handle. The composition's
unique-violation wrappers have no caller left, so they and
`timetable-activities.postgres.transaction-runtime` are deleted.

The operational day, the timetable data reads, the attendance corrections
and history (3,913 LOC) follow, again without a rule. The session token and
the shared authorization rules stay in the composition behind the
application's `OperationCaller` port (`compose/operation_caller.go`), so the
moved code imports neither `auth/authorize` nor the session token adapter.

Epoch 57 replaces the 99 same-owner aliases of the Timetable composition
(`compose/templates.go`, `instances.go`, `operations.go`) with defined types
in the non-internal package `modules/timetable/ports` (role `port`). The
roots, the HTTP tests and the Care Plan contract suite name those types
directly; the composition only constructs. ADR 0058 addendum point 5 makes
`port` a source role of the reach (`reviewedInternalReachRule`). The epoch
adds `domain.port-public` (and deletes `care-plan.port.care-plan.public`,
which it covers), `timetable-activities.port.student-presence.public`,
`timetable-activities.port.legacy-model` (temporary until #4086 dissolves
`models/schedule` and `models/activities`),
`care-plan.contracttest.timetable-activities.port` (narrowing of the
contract suite's existing compose reach) and, accepted by the ADR 0053
composition reach, `root-composition.reach.domain.port`; 902 → 906 rules.

The retained repository adapters that run on the Timetable capability alone,
the Care Plan and Workforce row adapters, the end-of-session completion, the
cleanup and the legacy repository vocabulary follow, again without a rule.
`application.NewCapabilityRepositories` builds the capability-only adapters;
the composition adds the instance and participant adapters, which also run on
the database and Student Presence. The data and dependency types the roots
name go to `modules/timetable/ports`; the composition builds the adapters
through constructors that return ports interfaces, without aliases. The
`legacy_*.go` files that move keep their names and budgets under their new
paths; `legacy_repository_options.go` splits, the pickup exception and student
status day values the roots name going to the ports package with their own
entry. `legacy_errors.go` stays in the composition: only its retained activity
service uses it.
The same change moves the capability engine (the mapping of the public
vocabulary onto the domain, `application.Engine`) and the attendance mirror;
the mirror reports a recovered panic through a port the composition binds to
Sentry (`compose/attendance_mirror_report.go`). The Timetable composition goes
from 14,106 to 6,195 LOC.

The retained instance and participant adapters, their transitions and the
recovery's Timetable and Student Presence halves follow into
`internal/application`; they read the joined presence projection through the
application's `PresenceReads` port, which the composition binds over the
database (`compose/presence_reads.go`), and the filters of those reads are
ports values. Epoch 60 adds `test-support.to.timetable-port`, the narrowing of
`test-support.to.timetable-compose` the Timetable test support needs to name
the filters (ADR 0058 addendum point 5). The legacy School Calendar
repositories (calendar periods, closing days, date frames) follow; their
not-found shape keeps `sql.ErrNoRows` through `transactionruntime.ErrNoRows`.
The composition goes to 4,218 LOC.

People Directory follows the same way. The engine that serves the public
capability over the application services (the mapping of the public
vocabulary onto the domain and of the domain errors onto the contract,
`application.Engine`, `application.MapError`) and the adapter of the Audit
Platform's change trail move from `modules/peopledirectory/compose` into
`internal/application`; the composition goes from 8,943 to 7,947 LOC.
The retention cleanup of the per-child change history and the adapters of
the root's student seams (Care Plan's companions, the enrollment and care
owners, Audit's note deletion ledger) follow; the seams and the cleanup's
dependency types are values of the non-internal `modules/peopledirectory/ports`
package, so the roots name them without aliases. Epoch 62 adds
`student-owner-integration.people-directory.port`, the narrowing of
`student-owner-integration.people-directory.compose` the student
unit-of-work integration test needs to name the owner seam (ADR 0058
addendum point 5). The composition goes to 7,627 LOC.

Workforce follows the same way: the engine that serves the public capability
over the application service and the staff administration's engine and
adapters (`application.Engine`, `application.StaffAdminEngine`,
`application.MapError`) move from `modules/workforce/compose` into
`internal/application`; the composition goes from 7,021 to 5,190 LOC. Epoch
63 adds `workforce.application.transaction-runtime` (ADR 0058 addendum point
2): the staff administration keeps its not-found shape through
`transactionruntime.ErrNoRows` instead of `database/sql`.

Care Plan follows: the engine that serves the capability over the Care Plan
stores, the exception queries and the offering change reads
(`application.Engine`, `application.ExceptionQueryService`,
`application.MapError`) move from `modules/careplan/compose` into
`internal/application`. The engine reaches the tenant runtime and the
student and exception-day locks through the `EngineRuntime` port the
composition binds; the student name projection and the exception query seam
the roots name are values of `modules/careplan/ports`. Epoch 64 adds
`own-test-support.domain.port` (ADR 0053 point 7.1, next to
`own-test-support.domain.compose`): the Care Plan test support names the
student name projection. The composition goes from 4,333 to 3,147 LOC.

The repository contracts and the query and error vocabulary of the retained
People Directory repositories move from `database/repositories/users` into
`modules/peopledirectory/ports` as defined types, together with the port
declarations of the composition (person writer, staff directory, student
directory, photo, audit and guardian payment ports). The repositories,
the composition and the roots name them there; the matching aliases of
`compose/vocabulary` go (#4085). Epoch 65 adds
`people-directory.port.legacy-model` (temporary until #4004 dissolves
`models/users`) and `people-directory.postgres.port`, the first rule of the
postgres-to-own-port shape of ADR 0058 addendum point 6. The composition goes
from 7,627 to 7,316 LOC.

The retained guardian service (profiles, relationships, phone numbers,
invitations, payment data, validation and masking) follows into
`internal/application` as `application.RetainedGuardianService`. Its request,
result and dependency types, its errors and the `GuardianService` port the
roots and the HTTP layer name are values of `modules/peopledirectory/ports`;
its transactions run through the `GuardianTransactions` port the composition
binds to the tenant runtime (`compose/guardian_service.go`). The composition
goes to 5,074 LOC.

The retained person and staff directories follow as
`application.RetainedPersonDirectory` and the staff directory behind
`ports.StaffDirectory`; their dependencies, errors and the `PersonDirectory`
port are values of `modules/peopledirectory/ports`, the staff writes run
through `ports.TenantTxRunner`, and `MissingStudentError` leaves
`legacy_composition_support.go` for the ports' error vocabulary. Epoch 66 adds
`people-directory.application.tenant-runtime`,
`people-directory.application.transaction-runtime` and
`people-directory.port.transaction-runtime`. The composition goes to 4,033
LOC.

The caregiver capability service and the guardian directory provider follow
(`application.NewCaregiverCapabilityService`, `application.GuardianDirectory`).
The caregiver service's Bun-transaction check stays in the composition and is
handed in; the provider's constraint classification moves to the ports'
error vocabulary (`IsConstraintViolation`). The composition goes to 2,779
LOC.

Identity & Access follows: the engine that serves the capability over the
application services, with its error mapping, result mapping and the second
factor's flow bundle (`application.NewEngine`, `application.EngineParts`,
`application.MapError`, `application.MFAFlows`, `application.CalendarFeeds`),
moves from `modules/identityaccess/compose` into `internal/application`; the
refresh rotation's recovery-proof binding is handed in. Epoch 67 adds
`identity-access.application.tenant-runtime`,
`identity-access.application.transaction-runtime` and
`identity-access.module-internal-test.tenant-runtime`. The composition goes
from 7,356 to 4,075 LOC.

Care Plan follows with its record adapters: the generic effective-time record
adapters (`application.EffectiveRecords`, `ExceptionRecords`, `NoteRecords`
over `RecordFuncs`), the companion edges (`application.CompanionEdges`) and the
student profile commands (`application.NewStudentProfiles` over
`ports.StudentProfileStore`) move from `modules/careplan/compose` into
`internal/application`; the composition binds the store and the record
functions. Epoch 68 adds `care-plan.application.tenant-runtime`. The
composition goes from 3,147 to 2,876 LOC.

Student Presence follows: the engine that serves the capability over the
application service, with its public mapping of attendance, visits, sessions,
supervisions, live groups and privacy consent (`application.NewEngine`), moves
from `modules/studentpresence/compose` into `internal/application`; a missing
attendance or visit row is recognised through the transaction runtime's no-rows
sentinel. Epoch 69 adds `student-presence.application.transaction-runtime`.
The composition goes from 1,805 to 621 LOC.

School Structure follows: the engine that serves the capability over the
application service, with the group, transition and ledger mapping and the
error mapping (`application.NewEngine`, `application.Engine`,
`application.MapError`), moves from `modules/schoolstructure/compose` into
`internal/application`. Epoch 70 adds
`school-structure.application.tenant-runtime`. The composition goes from 1,573
to 1,258 LOC.

Device Fleet, Organization & Tenancy and School Membership follow: each
engine that serves the owner's capability over its application service, with
the public and error mapping (`application.NewEngine`, `application.Engine`,
`application.MapError`), moves from the owner's composition into
`internal/application`. Epoch 71 adds
`school-membership.application.tenant-runtime` for the staff identity read.
The compositions go from 701 to 372, 708 to 479 and 869 to 354 LOC.

The Identity & Access session token codec (`application.SessionCodec`,
`application.NewSessionCodec`, `application.JWTSigner`) and the Enrollment
decision adapter (`application.PublicDecisions`) move into their owners'
`internal/application`. With the codec gone, the Identity & Access composition
no longer imports the token library; epoch 72 removes the stale
`external.crypto.identity-access.production.compose` grant. The compositions
go from 1,963 to 1,801 and 823 to 716 LOC.

#4085 starts with a register: `test/module_compose_alias_ratchet_test.go`
counts, per composition package, the type aliases and value re-exports that
name a package outside the composition's owner — the vocabulary the
compositions hand out without the import the architecture ratchet would see.
It starts at 522 (366 of them in `compose/vocabulary` and `compose/retained`)
and only falls; #4085 is done when it is empty.

Care Plan's record capabilities the roots bind (`ArrivalBaselineRecords`,
`PickupBaselineRecords`, `PickupApprovalRecords`, `PickupExcusalRecords`) are
defined types in `modules/careplan/ports`; the baseline sources, the approval
exceptions and the excusal rows over them move into `internal/application`
(`application.NewArrivalBaselines`, `NewPickupBaselines`,
`NewPickupApprovalExceptions`, `NewPickupExcusalRows`). The composition goes
from 2,876 to 2,572 LOC.

The School Structure group facade follows: the public group management over
the group application and the private group reads, with the group value and
error mapping (`groups.NewManagement`, `groups.PublicGroup`,
`groups.LegacyGroup`, `groups.PublicGroupError`), moves from
`modules/schoolstructure/compose` into `internal/application/groups`; the
retained testhooks adapters keep calling them there. The composition goes from
1,258 to 904 LOC.

### Enrollment wiring out of services (#4012)

None of the 19 Enrollment files of #4012 remains in `services`. Measured
against `development` after #4006 to #4008: `services` goes from 156 to 137
production files and from 21,330 to 18,532 LOC; the baseline stays at 39 keys
and the policy at 1,431 rules.

After merging #4009 to #4011, the combined composition surface is 228
(231 on the merged `development`), with 39 legacy keys. The Care Plan,
Presence and Timetable wiring remains root-composed alongside Enrollment.

- **Enrollment** decides what is its own: `BookingProjection` in the public
  package (selections at a date, the first upcoming interval before a phase
  starts, capacity peaks, grade counts, approved bookings and the offering
  links of the booking audit) reads Care Plan's bookings through the
  consumer-owned port `OfferingBookings`, next to the owner's other ports such
  as `PhaseExpiryStudents`. It is not in the application layer, whose
  pass-through ratchet would count its port reads. The three functions above
  the module complexity limit are split into helpers;
  `booking_capacity_internal_test.go` pins their boundaries. Enrollment's
  composition only constructs: `NewDeletionModule` shares one impact preview
  between the admin deletion and the retention cleanup.
- **The root** (`api`) holds the glue: Care Plan's bookings behind the port
  (`api/enrollment_bookings.go`), the Settings Platform, Delivery outbox,
  Audit trail, People Directory, Care Plan and Timetable adapters of the
  intake, decision, change-request, rollover, report, phase and
  offering-change flows, the booking consistency audit next to the Worker job
  that runs it (`api/booking_consistency_audit.go`), and the settings guards.
  The adapters read People Directory and Audit rows through narrow ports
  (`api/enrollment_rows.go`, Rule 1).
- **The Factory** takes `services.EnrollmentWiring` as one new constructor
  parameter: the projection, the adapters over owners the root composes, and
  constructors for the flows over the settings, audit trails, outbox,
  realtime hub and booking materialization only the Factory builds. The
  source types of those constructors stay in `factory.go`. The root builds
  `EnrollmentFormSchema`, `EnrollmentCaptcha` and `EnrollmentPhaseExpiry`
  where it mounts the routes, so those fields are gone (237 → 234);
  `EnrollmentRejectedCleanup` keeps its name with Enrollment's own type, and
  the root adds the scheduler's counts.
- **The guard closures** of `newFactory` (ADR 0053 point 4): each of the three
  settings guards (class restriction, grade restriction, grade cap) binds a
  Settings port to an Enrollment phase read, and Settings decides what it
  refuses. All three are glue and moved to the root
  (`bindEnrollmentSettingsGuards`); none decides anything.
- **Care Plan integration after #4011:** the care exit's booking writes now
  live in `api/care_exit_enrollment_bookings.go` around Enrollment's root-built
  projection. `NewCareLifecycleOwners` receives that adapter. The shared
  offering-change settings adapter lives next to the root's pickup adjustment;
  `BookedOfferings` and `BookingOfferingPhase` live next to booking
  materialization. The Factory consumes Enrollment's projection directly.

Delivery's composition names the e-mail recipient resolver of the "Neue
Anmeldung" mail, and the People Directory vocabulary the emergency contact
row, because the root may not import an owner's application role.

### Workforce wiring out of services (#4008)

The 34 Workforce files of #4008 left `services`, and so did
`rfid_card_lookup.go`, which only the staff clock used. Each part went where
ADR 0053 point 8 puts it:

- **Workforce's composition** builds what binds Workforce to Workforce: the
  planned shifts over the public shift query, the schedule assignment over the
  employment profile, the schedule and work-time-template adapters over the
  rows of `repositoryadapter`, and today's work presence
  (`time_tracking_planned_shifts.go`, `time_tracking_staff_schedule.go`,
  `time_tracking_work_schedules.go`, `work_presence.go`).
- **The root** (`api`) holds the cross-owner glue: the staff absence
  decorators over School Membership and the People Directory, the time-tracking
  ports over the staff rows, the Audit trails, the settings, Delivery and
  Document Rendering, the personnel-record administration, the staff clock
  over Identity & Access and the People Directory, the admin capability
  bundle, the substitution capability, the Timetable shift rows, the plan
  export's staff names, and Workforce's School Membership callbacks. The
  time-tracking ports take the Transaction Runtime value instead of
  `*bun.DB`. The cleanup command composes Workforce through
  `api.NewUnobservedWorkforce` and hands it to
  `services.NewTimeTrackingCleanupService`.
- **The Factory** takes `services.WorkforceWiring` as one new constructor
  parameter, alongside `IdentityRoot` (#4006). It carries the values the root
  builds (the staff absences, the shift rows, the plan export's staff names)
  and two bindings the Factory calls with what only it builds (the retained
  staff and person rows, the Audit persistence, the settings, the broadcaster
  and the tenant mail). The root builds the staff clock where it mounts the
  IoT routes, so the Factory field `StaffClock` is gone and the composition
  surface goes from 250 to 249. The rule
  `legacy-composition.to.staff-clock-compose` became stale and is deleted.
- **Callers in `services`** keep the helpers no other code uses, because a
  function leaves `services` only when no `services` code calls it. The
  supervision resolver moved into `education_group_repository.go`, the group
  substitutions into `membership_repositories.go`, `MembershipStaffEmployment`
  into `school_membership.go` (#4007), the session staff mapping into
  `presence_session_records.go` (#4010), and the caregiver chains into
  `person_projection_auth.go` (combined with #4006). They leave with those
  builders.
  Until #4007 moves them, three small lookups exist twice: `api` has
  `staffMembersByID`, `absencePersonsByID` and `positiveUniqueIDs` next to
  `services`' `staffByID`, `personsByID` and `uniqueIDs`. #4007 deletes the
  `services` copies.

Delivery's composition exposes `QueueStaffTimeTrackingChanged`, and the Audit
vocabulary exposes `TimeTrackingAuditLogEntry`, because the root may not import
an owner's application or domain role. The root's adapters declare the narrow
ports they call instead of naming whole repositories, which the
`TestHandlerLayerRatchet` R1 gate requires of non-hook files in `api`, and the
two moved functions above the complexity limit are split into helpers. No key
is added or removed (40 → 40), and the policy goes from 1,437 to 1,436 rules.

### Student Presence wiring out of services (#4010)

None of the 39 files of #4010 remains in `services`; `rfid_card_lookup.go`
and `sse_presence.go` had already left with #4008 and #4006. Each part went
where ADR 0053 point 8 puts it:

- **Owner compositions** hold what binds one owner: Facilities builds the
  retained room repository and the yard colour read (`NewRoomRepository`,
  `YardRoomColor`) and names `OpenGroupCatalog`; Settings answers the named
  presence questions (`PresenceSettings`); Student Presence logs its
  operations (`FailureLogger`, `DebugLogger`); Delivery queues the
  instance-started and arrival-schedule events (`QueueInstanceStarted`,
  `QueueArrivalScheduleChanged`), because the root may not import its
  `application` role. Six wrappers that only forwarded to an owner are
  deleted; their callers call Timetable, Process Device Scan, Facilities,
  Student Presence, the calendar view and Security Runtime directly.
- **The decision of point 4.** A room cannot be deleted while a session runs
  in it or a care offering needs it. Facilities' application asks its
  consumer-owned port `RoomUsage` (`HasActiveSessions`,
  `RequiredByCareOffering`) inside the delete transaction and keeps the
  mapping to `ErrRoomInUse` and `ErrRoomRequiredByOffering`. The root binds
  the port (`api.RoomUsage`) to Student Presence's open live groups and Care
  Plan's `ValidateRoomDeletion` once the service graph exists. The retained
  room service lost `ValidateDeletion`, and the root's error-code mapping is
  gone.
- **The root** (`api`) holds the cross-owner glue: the Student Presence roots
  (`NewStudentPresence`, `NewLoggedStudentPresence`, `NewPresenceFacts`,
  `NewSessionRecords`, `NewPartialAbsencePreview`, `NewPickupReviewBlocks`)
  over the Transaction Runtime value, the session record directories, the
  attendance reads, the session device directory, the yard's open groups, the
  dashboard's open rooms and sessions, the statistics inputs, the class day,
  the class list binding, the kiosk mirror and the session rosters.
- **The Factory** takes `services.PresenceWiring` as one new constructor
  parameter. It carries the values the root builds and five constructors the
  Factory calls with what only it builds: the Audit command
  (`BindClassListEntries`), the settings (`SessionDevices`), the person
  service (`AttendanceStaffNames`), the class-day sources (`ClassDay`) and
  the access log with the retention settings (`Statistics`). The fields
  `Facilities` and `StudentStatusDays` are gone; the root mounts the values
  it built, and `api.TestFactory` keeps both under their names for the
  suites. `unregistered_tag_scans.go` had already left with #4049.
- **Tests** move with their code. The Student Presence, Care Plan and
  Workforce suites reach the other owners' compositions through the root's
  test hooks (`api/presence_suite_test_helpers.go`), because a test of one
  owner may not import another owner's composition (point 7).
- **Callers in `services`** keep private copies of the Student Presence
  roots, the presence rules and the partial-absence preview in
  `timetable.go` (#4009), of the session directories and the session device
  directory in `command_roots.go` and `cleanup_composition.go` (#4013), and of
  `stringValue` in `enrollment_report_composition.go` (#4012). The root's
  Care Plan exception directory (`api/care_plan_exception_directory.go`)
  is also used by Care Plan after #4011; only the command roots keep a
  private `pickupExceptionDirectory` until #4013. They leave with those builders.

The Device Fleet session device directory binds a Student Presence port, so it
is root glue and its legacy tree keeps its budget. No key is added or removed
(39 → 39), and the policy keeps its rules. Measured after #4049, `services`
shrinks from 155 to 119 production files and from 21,238 to 19,506 LOC.

### Timetable, School Calendar and reminder wiring out of services (#4009)

The 29 files of #4009 left `services` (measured on `030716737b`, after #4006,
#4007 and #4008: 156 → 127 production files, 21,330 → 18,141 LOC). Each part
went where ADR 0053 point 8 puts it:

- **The owners' compositions** build what binds an owner to itself. School
  Calendar: the non-working-day, statutory-holiday and closing-day date sets
  and the period ids (`modules/schoolcalendar/compose/non_working_days.go`),
  and the portal's iCalendar renderer over the calendar's renderer
  (`modules/schoolcalendar/portal/compose/calendar_renderer.go`). Timetable:
  the planning bundle the routes mount (`Planning`), the auto start and auto
  end over the lifecycle, and the substitute conflicts over the owner rows
  (`modules/timetable/compose/planning.go`).
- **The root** (`api`) holds the cross-owner glue: the retained Timetable
  repositories and their owner directories, the Timetable bindings to Care
  Plan, Communication, Audit, Settings, Student Presence, School Structure,
  Security Runtime and the realtime hub, the calendar portal and its fact
  ports, the reminder readers and the reminder delivery, the timetable route
  ports, the birthday display, the period administration, the planning
  calendar and the plan export's non-working days. The retained Timetable
  repositories take the Transaction Runtime value instead of `*bun.DB`.
  The root's adapters declare narrow ports instead of naming repositories,
  as `TestHandlerLayerRatchet` R1 requires of non-hook files in `api`.
- **The Factory** takes `services.TimetableWiring` as one new constructor
  parameter, beside `PeopleWiring` (#4007). It carries the values the root
  builds (the Timetable repositories, the group names, the planning tracks,
  the class-day and history reads, the plan export's calendar readers, the
  calendar's guardian directory) and the constructors the Factory calls with
  what only it builds. Their input types (`TimetableLifecycleInputs`,
  `CalendarDependencies` and the rest) sit with the Factory in `factory.go`,
  because both the Factory and the root name them. The root builds the
  planning-track administration and mounts it, so the Factory field
  `PlanningTracks` is gone and the composition surface goes from 237 to 236.
  The root also builds the tenant recurrence gate the Factory's recurrence
  writers serialize on and hands it in. The Factory method
  `SchoolCalendarAdministration` is the root function
  `api.GuardedCalendarAdministration`, which takes the settings, the gate and
  the care-offering guard instead of the Factory.
- **Callers in `services`** keep what no other code uses, because a function
  leaves `services` only when no `services` code calls it. The unobserved
  Timetable the presence rules read moved into `student_presence_roots.go`
  (#4010), and the cleanup command hands `api.NewTimetableCleanup` to
  `services.NewTimetableCleanupService` (`cleanup_composition.go`, #4013).
  Until #4010 moves the presence roots, the unobserved Timetable is composed
  twice: `services`' `newPresenceTimetable` and the root's
  `NewUnobservedTimetable`, with their own student and room mappings. #4010
  deletes the `services` copy.

After merging #4009 and #4010, the root passes both `PresenceWiring` and
`TimetableWiring` to the Factory. The root's Timetable builders use its
Student Presence roots directly. Their private copies in `services/timetable.go`
and `services/student_presence_roots.go` are gone; only the session and
retention cleanup callers retain private roots in `command_roots.go` until
#4013. Both migrations' owner boundaries remain in place. The combined
composition surface is 231 after merging #4010 (233 before that merge), with 39 legacy keys. The
stale `legacy-composition.module-internal-test.student-presence-public` rule
is removed because the reminder tests now live in the root.

Delivery's composition exposes the outbox request, the notification event,
its audience, the push sentinels, the parent appointment copy and the mail
branding the calendar portal binds, because the root may not import an
owner's application role. The rules `legacy-composition.reminder-compose`
and `legacy-composition.reminder-ports` became stale and are deleted. No key
is added or removed (39 → 39), and the policy goes from 1,431 to 1,429 rules.

### Care Plan wiring out of services (#4011)

None of the 45 Care Plan files of #4011 remains in `services`. Each part went
where ADR 0053 point 8 puts it:

- **The root** (`api`) holds the cross-owner glue: Care Plan's adapters over
  the People Directory, Timetable, School Calendar, Enrollment, Student
  Presence, Communication, Delivery, Identity & Access and the Audit Platform,
  the parent request review policy over Identity & Access
  (`parent_request_review_policy.go`), and `api/care_plan_wiring.go`, which
  binds them. The adapters declare narrow ports instead of retained repository
  types (`TestHandlerLayerRatchet` R1), and the moved care-exit restore is
  split into helpers to stay under the complexity limit.
- **The Factory** takes `services.CarePlanWiring` as one new constructor
  parameter. It carries the adapters over the owners the root composes and
  bindings the Factory calls with what only it builds: the settings, the Audit
  trails, the realtime hub, the parent event emitter, the caller context and
  Care Plan's own services. The Care Plan observer parameter is gone; the root
  binds the observer into the excused-absence binding. The root builds the
  companion graph and the review policy where it mounts them, so the Factory
  fields `Students` and `RequestReviewPolicy` are gone (235 → 233). Without
  `Students`, People Directory's retained `NewStudentService` was reachable
  only from tests and is deleted. Care Plan's composition gains
  `NewCareOfferings`, the one capability over the catalog, the booking
  materialization, the offering changes and the pickup adjustments.
- **Decisions** (ADR 0053 point 4). The pickup auto-excusal resync was the
  only tenant transaction in `services`. `ResyncFutureExceptions` now takes
  the students and opens or joins Care Plan's own unit of work. It writes Care
  Plan's pickup exceptions, and Student Presence's attendance and Timetable's
  pickup extensions only through Care Plan's existing ports
  (`PartialAbsenceBlocks`, `PickupExtensions`), the same ports every Care Plan
  excusal command writes through, so it stays in Care Plan rather than
  becoming a workflow. The student lock order of the weekly pickup approval is
  Care Plan's `LockWeeklyPickupStudents` behind the consumer-owned port
  `PickupStudentLocks`; the root binds it to the People Directory and reports
  `ErrStudentNotFound` as not found, which Care Plan skips.
- **Callers in `services`** keep the helpers they still call:
  `EnrollmentBookingProjection` in `enrollment_bookings.go`, the booking and
  settings adapters, the request ledger and the request messenger in
  `offering_change_composition.go` (#4012), the parent-portal repositories in
  `owner_roots.go` (#4013), `pickupExceptionDirectory` in
  `command_roots.go` (#4013), and `CareRequestStaff`
  next to the caller context in `factory.go`. The root calls the exported
  ones.

Delivery's composition exposes `AbsenceNotifier` and `AbsenceReport`, because
the root may not import Delivery's application role. The enrollment route
suites no longer import `services`, so the rule
`enrollment.http-integration-test.legacy-composition` became stale and is
deleted (1,429 → 1,428 rules). No key is added or removed (39 → 39), and the
Care Plan pass-through budget stays at 111.

After merging #4010, both `PresenceWiring` and `CarePlanWiring` remain root
inputs. The obsolete manual partial-absence repository is gone, and the
command roots keep the exception adapter they still call until #4013. The
combined composition surface is 231. The parent-portal decision tests no
longer import `services`, so `parent-portal.decision-test.legacy-composition.compose`
is also stale and removed (1,427 → 1,426 rules).

Twelve baseline entries fall: nine of #2750 (`. -> cmd`, `cmd -> api`,
`cmd -> tenant`, `cmd -> database/migrations` in production and internal test,
`api -> observability`, and the internal-test `api -> auth/authorize/permissions`,
`api -> realtime` and `api -> tenant`), two of #3446 (`behavior ->
modules/identityaccess` and `behavior -> legacy/jwt`) and one of #2748
(`test -> legacy/jwt`). With the four #4003 entries the baseline goes from
54 to 46. The policy goes from 1,663 to 1,438 rules, and the composition
surface stays at 252. `cycles --graph target` is unchanged at 58 owners, 335
edges and one component of 32 members: the target projection omits
composition owners and self-edges, so the reaches add no edge to it.

#4003 removes the four tuples. Each owner's composition now builds what the
root took from its implementation roles:

- `modules/workforce/compose/httpadapter` serves `/absence-types` and
  `/substitutions` (`AbsenceTypesRoutes`, `SubstitutionsRoutes`). It returns
  `chi.Router`, because a Workforce composition may not name the
  `inbound-absence-types` and `inbound-substitutions` resources. The routes
  live in `httpadapter` and not in `compose`: the internal tests of
  `modules/workforce/inbound` import `services`, which imports `compose`, so
  `compose` importing `inbound` would be an import cycle in those tests.
- `modules/dataimport/compose` returns the file decoder (`NewFileDecoder`).
  That import is the first use of a workflow composition's own `adapter`
  role. Policy epoch 42 → 43 adds `owner-compose.workflow.adapter` under
  `reviewedOwnerCompositionRule`, as constraint 3 above describes.
- The new package `modules/schedulerruntime/compose`
  (`scheduler-runtime`/`compose`) binds the job runtime and the Worker lease
  store to the scheduler's ports (`NewTenantRuntime`, `NewWorkerLeaseStore`).
  Its signatures take the `tenant.UnitOfWork` the root holds (ADR 0053
  point 5), so the package has `scheduler-runtime.compose.tenant-runtime`,
  like the 34 other owner compositions. PR mode accepts that rule because it
  is anchored to the new package. It is the one new cross-owner reach of an
  owner composition: the #4004 measurement goes from 206 to 207 rules, 83 of
  them into `public`.

The baseline goes from 46 to 42 and the policy from 1,438 to 1,440 rules. The
composition surface stays at 252, and `cycles --graph target` is unchanged at
58 owners, 335 edges and one component of 32 members.

### Service Factory deleted (#2747)

The top-level package `services` and `services.Factory` are gone; the
classified packages below `services/` stay with their owners. The root
(`api`) composes the retained services itself, as ADR 0053 point 8 planned:

- **Stages, not a Factory.** `newServiceGraph` (`api/service_graph_compose*.go`)
  binds the services over the owners the root composed and the wiring it built
  for them (`PeopleWiring`, `PresenceWiring`, `TimetableWiring`,
  `PlatformWiring`, `CarePlanWiring`, `EnrollmentWiring`, `WorkforceWiring`).
  Each stage takes the build so far and returns it extended instead of setting
  graph fields through a receiver; the binding calls the Factory made on
  composed services (`WithDecisionNotifications`, `SetAttachmentPurger`,
  `WireCareParticipation`, `BindGuardianDirectory`, `BindClassListEntries`,
  `BindSettingsGuards`) stay as they were. A
  value a later stage binds (Delivery's outbox and state, Care Plan's
  participation, the sick cascade, the booking materialization, the
  cancellation notice, the parents portal's sharing rules) sits in one slot the
  earlier services read at call time, as the Factory's local variables did.
  `newServeServiceGraph` requires every owner, observer and wiring and reads the
  process configuration; `NewServiceGraphForTests` (formerly
  `NewFactoryForTests`) builds the test graphs. The graph's
  `bindTenantRuntime` replaces `Factory.SetTenantRuntime`. The photo lifecycle
  is composed from the root's photo slot and file unlinker instead of the
  post-construction setter `EnableStudentPhotos`.
- **Imports through compositions.** The root may not import Delivery's
  application role, the projections' adapters, Observability's store or
  Workforce's adapter. Delivery's composition names the notification, outbox
  and PWA usage services; `modules/grouplive/compose` and
  `modules/supervisiondashboard/compose` name the callers and the open room
  sessions; the new `modules/emergencysnapshot/compose` and
  `observability/compose` build the Notfallliste and the PWA usage store;
  `modules/workforce/compose` names the group substitution row. The two new
  packages are created packages of their owners, so PR mode accepts their
  classification.
- **Rule 1 and the complexity limit hold under `api/`** without an allowlist
  entry. Source fields name the narrow ports the root's adapters already
  declare. The Timetable composition takes consumer-owned ports for the
  lifecycle staff, the deviation staff and the correction actor
  (`LifecycleStaff`, `DeviationStaff`, `CorrectionPeople`). The People Directory
  composition names the retained row stores (`Repositories`) the root hands the
  graph. Stateless adapters such as the room, guardian phone and request event
  stores are built where a stage uses them.
- **Tests.** The legacy composition's suites run as internal tests of `api`.
  `TestAPITestsUseNarrowRouteBuilders` treats `newServiceGraph` and
  `newServeServiceGraph` as broad composition, as it treated
  `services.NewFactory*`; only the graph's own tests (`api/service_graph*_test.go`)
  may build it.

The policy loses the package entry of `services`, the `legacy_composition`
entry `services.Factory` and the 69 rules only `services` used (1,415 → 1,346
rules); the epoch stays 44. The baseline goes from 39 to 34: `services -> bun`
and `services -> tenant` (#2747) and `api -> services`, `cmd -> services` and
the composition reference `api -> services.Factory` (#2750). The composition
ratchet counts `api.serviceGraph` as a central aggregate and compares a base's
`services.Factory` fields with it (`relocatedAggregate`), so the moved fields
stay shrink-only and a new one fails the check. Measured after merging #2749,
the surface goes from 140 to 139: the setter `EnableStudentPhotos` is gone. `cycles --graph target` is unchanged at 58
owners, 335 edges and one component of 32 members. The caller inventory
`callers/service-factory-2747.json` (`scripts/backend-service-factory-callers`)
lists no Go file and no importer of `services` and no declaration or reference
of the retired symbols; at the merge base it lists 29 Go files and 109 imports.

### Root without foreign implementation roles, first cut (#2750)

Nine #2750 keys fall without a policy change (34 -> 25). Each import of an
implementation role of another owner moves behind a seam this slice adds to
a contract the root may already reach, or into the owner that holds the code:

- **Shared kernel and the settings contract.** `cmd` reads calendar dates
  from `sharedkernel/calendar` instead of `internal/timezone` (production and
  internal test). `cmd settings overrides` and the two root suites that seed
  settings name their keys through `modules/settings`, whose contract gains
  the seven keys they use. The CLI reads stored overrides through
  `settingsCompose.NewStoredOverrideReader`, which returns the stored JSON and
  whether a row exists. It replaces `NewSettingsCommandRepository`, whose only
  caller was the CLI, so `cmd` names no `models/config` type.
- **The Database value.** `transactionruntime.Database` gains `LogQueries` (the
  `db_debug` hook) and `Capacity` (the pool statistics of the metrics endpoint
  and the capacity log). `api` no longer imports `database`; `Capacity` still
  returns `database.CapacityStats`, whose fields the root reads.
- **Owner compositions.** The startup check of the students table runs through
  `peopleCompose.VerifyRetainedStudentSchema`, which leaves the composition's
  test hooks because production calls it now. `observability/compose.InstallProcessLogger`
  builds and installs the process logger for `serve` and the migration
  commands (`cmd -> applog`).
- **Session token adapter.** The rate-limit key derived from verified claims
  (#2064) is `sessiontoken.TokenAuth.RateLimitKey`. Its JWT decoding sits
  with the codec, so `api` no longer imports `jwx/jwt` or `jwx/transform`. The
  twelve key tests moved with it, and one more pins the IP fallback without a
  codec; the two limiter tests stay in `api`.

The other 21 keys keep #2750 as carrier. Sixteen need a decision
([#4063](https://github.com/moto-nrw/project-phoenix/issues/4063)): the
`http-router` class in the server root, `cli-config` in the internal tests of
`cmd`, the shared HTTP runtime `api/common`, the `dev-tools` seeder and
simulator, and four implementation roles whose use would otherwise need
aliases: Tenant Runtime's `middleware`, the session token adapter, the
Worker's application `services/scheduler` (37 symbols) and the analytics
adapter. Five need a move into their owners
([#4064](https://github.com/moto-nrw/project-phoenix/issues/4064)):
`api/config`, `api/operator`, `api/platform`, `api/pwa` and `api/reminders`.
The composition surface stays at 139. The evidence record is
`root-implementation-roles-2750.json`.

### Four inbound packages move into their owners (#4064)

Four of the five inbound packages the root still mounted move file for file
into the `http` role of the owner whose capability they call, as #2742, #2734
and #2731 did. Their four `inbound-*` owners are deleted; routes, status
codes, error bodies and the route and middleware goldens are unchanged.

| Old package | New package | Owner |
|---|---|---|
| `api/config` | `modules/settings/http` | `settings-platform` |
| `api/platform` | `modules/communication/http/platformannouncements` | `communication` |
| `api/pwa` | `modules/delivery/http/pwa` | `delivery-platform` |
| `api/reminders` | `workflows/reminderdelivery/http` | `reminder-delivery` |

- The PWA usage package holds the route shape; the root binds its handler to
  Delivery's usage service (`modules/delivery/application/pwa`), which owns
  the capability. Observability only owns the table the service writes
  through. The reminder route calls the workflow's own `CallerQuery`,
  so it stays with `reminder-delivery`, not with School Calendar or Timetable.
- `inbound-reminders.to.reminder-delivery` becomes
  `reminder-delivery.http.public`; `reminder-delivery.compose.http` now targets
  the workflow's own `http` role. `reminder-delivery.adapter-test.http` and
  `inbound-platform.to.communication` are deleted: `adapter-test.http-subject`
  and `communication.http.public` already allow those imports.
- The root reaches Communication and the reminder workflow through
  `root-composition.reach.domain.http` and `reach.workflow.http`. ADR 0053
  point 2 also lists the `http` role of platform owners, but the policy had no
  such rule; policy epoch 44 -> 45 adds `root-composition.reach.platform.http`
  in all three scopes, which `reviewedRootCompositionRule` accepts. It adds no
  `issue` field and no compatibility rule. Without it, the root's imports of
  `modules/settings/http` and `modules/delivery/http/pwa` are new violations.

`api -> api/config`, `api/platform`, `api/pwa` and `api/reminders` (#2750)
leave `legacy.jsonl`, 25 -> 21. `api -> api/operator` stays: the operator
router mounts the routes of five owners and needs the `http-router` decision
of [#4063](https://github.com/moto-nrw/project-phoenix/issues/4063). The
composition surface stays at 139. The evidence record is
`relocate-inbound-packages-4064.json`.

### The operator router moves into the HTTP host (#4064)

`api/operator` mounted the operator routes of Identity & Access,
Organisation & Tenancy, Settings Platform, Communication and Device Fleet.
That is cross-owner glue, which ADR 0056 lets the HTTP host mount, so the
router moves into the server root as `api/operator_routes.go`. Routes,
status codes, error bodies and the route and middleware goldens are
unchanged.

- `operator.Resource` and `operator.ResourceConfig` held values of several
  owners, which a root may not (ADR 0059 point 1). `operatorRouter` takes
  each owner's part as its own value instead: the Identity & Access inputs
  (`operatorIdentity`), Organisation & Tenancy's `ProvisioningConfig` and
  billing routes, Settings Platform's school settings routes, the
  Communication capability and the Device Fleet handler. The rate limiters
  come as the root's `authRateLimiters` instead of three setters.
- `IdentityResponses` becomes `operatorIdentityResponses` in `api`.
- The router's suites move into `api` (`api/operator_*_test.go`) with their
  assertions; only the router construction changes, and the panic for
  missing session chains no longer names the deleted `ResourceConfig`.
- The owner `inbound-operator`, its package entry and its eight rules are
  deleted. The root reaches every import through the standing reaches of
  ADR 0053 point 2 (`root-composition.reach.domain.{http,public}`,
  `reach.platform.{http,public}` and the internal-test reaches); no rule is
  added.

`api -> api/operator` (#2750) leaves `legacy.jsonl`, 520 -> 519. The
composition surface falls from 1,746 to 1,743. The evidence record is
`relocate-operator-router-4064.json`.

### The Worker contract names no other owner (#4067)

ADR 0056 point 4 moves the contract the root fills into a
`scheduler-runtime`/`contract` package. The contract may not name another
owner's types: a platform importing a domain, a workflow or another owner's
`application` role is never a rule (ADR 0057). Each Worker dependency typed
by another owner therefore becomes a port the Worker declares and the root
binds (ADR 0058 point 2) before the contract moves. #4067 carries every key
with source `services/scheduler`, whichever carrier names it (#4072, #4075).

Step 1, the personal reminder notifications. The producer that turns the
reminder list into notifications joins the reminder batch with the
notification audience, so it moves into Reminder Delivery as
`workflows/reminderdelivery.PersonalReminderNotifications`. Its notices and
ports use the workflow's own vocabulary; the root binds the notification
service, the consent of the notification preferences, the staff and admin
accounts and the on-duty staff, which Workforce answers
(`workforce.OnDutyStaffIDs`). The Worker keeps the minute tick, the
notification feature flag gate and the on-duty setting, and calls the
producer through `scheduler.ReminderNotificationTick`. Construction refuses
an incomplete producer instead of silently not registering the tick.
`services/scheduler -> modules/delivery/application/notifications` and
`-> modules/workforce` in production and internal test, and
`-> workflows/reminderdelivery` in internal test leave `legacy.jsonl`,
519 -> 514. The composition surface falls from 1,743 to 1,742. The evidence
record is `scheduler-reminder-notifications-4067.json`.

Step 2, Student Presence, Care Plan and the Delivery Platform. The Worker
declares its own ports for the session end and the abandoned-session sweep
(`SessionMaintenance`, `DailySessionsEnded`), the nightly presence cleanup
(`PresenceCleanup` and its three results), the end-of-day status flag archive
(`StudentStatusFlagArchiver.ArchiveEndOfDayStatusFlag`), the standalone usage
sweep (`PWAUsageCleanup`) and the overdue announcements
(`InstanceOverdueBroadcaster`). The root binds them in
`api/scheduler_presence_ports.go`. Care Plan's vocabulary moves with the
binding: the root archives the sick flag as a sick day and the excused flag
as an excused day from the end-of-day source. The overdue event envelopes
become the Delivery Platform's `realtime.NewInstanceOverdueEvent` and
`NewInstanceOverdueRefreshEvent`, which the root broadcasts tenant-wide;
`realtimeevents.PublishInstanceOverdue` and `PublishInstanceOverdueRefresh`
had no other caller and are deleted. Seven keys leave `legacy.jsonl`
(512 -> 505): `services/scheduler -> modules/studentpresence`,
`-> modules/delivery/application/pwa` and
`-> modules/delivery/application/realtimeevents` in production and internal
test, and `-> modules/careplan/absencerecords` in production. The
composition surface stays at 1,740. The evidence record is
`scheduler-presence-delivery-ports-4067.json`.

Step 3, Timetable and Reminder Delivery. The Worker declares its own ports
for the weekly materialization (`TimetableMaterializer`,
`MaterializationRun`), the timetable retention sweep (`TimetableCleanup`,
`TimetableCleanupRun`), the automatic start and end (`InstanceAutoStart`,
`InstanceAutoEnd` and their runs) and the scheduled reminders
(`AppointmentReminderCommand`, `ParentAnnouncementReminderCommand`,
`ReminderCommands`, which name the methods the Worker calls). The root binds
them in `api/scheduler_timetable_ports.go` and tags the Worker's
materializations as the scheduler's; the overdue tick reads
`DayInstance.Planned`, which the root derives from Timetable's status. Three
keys leave `legacy.jsonl` (498 -> 495): `services/scheduler ->
modules/timetable` in production and internal test and `->
workflows/reminderdelivery` in production. `services/scheduler` carries no
key any more, and #4072 has none left. The composition surface stays at
1,740. The evidence record is `scheduler-timetable-reminder-ports-4067.json`.

Step 4, the contract moves. The Worker's dependency struct, its ports and
their results are defined types in `modules/schedulerruntime/ports`
(`scheduler-runtime`/`port`, ADR 0056 amendment of 2026-10-16), together with
`Worker`, the interface the roots run. `services/scheduler` names them and
keeps the implementation. The roots import the ports and build the Worker
through `modules/schedulerruntime/compose.NewWorker`; the setting keys come
through `compose.SettingKeys`. The retention run is built inside `NewWorker`
from `RetentionSteps` and `RetentionLog`, and the root implements the
rollover runner, so neither constructor reaches the root. The composition
merged into one file and fell from 85 to 83 LOC. Policy epoch 50 -> 51 adds
the standing ADR 0053 reaches `root-composition.reach.platform.port`,
`owner-compose.platform.port` and `own-test.platform.port`, accepted by
`reviewedRootCompositionRule`, `reviewedOwnerCompositionRule` and
`reviewedOwnTestRule`. `api -> services/scheduler` (#2750) leaves
`legacy.jsonl`, 316 -> 315; no other key, no alias and no exception is
added, and the composition surface stays at 1,720. The evidence record is
`scheduler-runtime-ports-4067.json`.

### Care-exit writes out of the root (#4082)

The Care Plan writes that reach several owners are the application workflow
`workflows/careexit` (owner `care-exit`, kind `workflow`, one `public`
package). It serves the care lifecycle's roster port over Timetable and
Student Presence, the care exit's booking end, snapshot and restore over
Care Plan's offering bookings and Enrollment's applications, and the later
pickup decision that adds a child to Timetable blocks and lets Student
Presence reapply the day statuses. It declares its own ports. The root
binds the owners' public facades and maps types. Every call joins the
caller's tenant transaction, and the care-exit transaction stays the unit
of work. Policy epoch 76 -> 77 registers the owner, its package and four
production rules from `care-exit/public` into the `public` roles of
`care-plan`, `enrollment`, `student-presence` and `timetable-activities`.
No key, alias or exception is added.
The group handovers of School Structure's substitution module are written
through Workforce's public commands by the existing `shift-plan-sync`
workflow, whose public role already reaches Workforce.

The staff directory joins of the retained staff repository (account links,
reachable and permitted staff, staff contacts, staff by role and the staff
birthdays) are the read projection `modules/staffdirectoryview` (owner
`staff-directory-view`, kind `projection`, one `public` package). It reads
School Membership's staff rows through the production rule
`staff-directory-view.to.school-membership` and declares its own person and
account ports, which the root binds to the People Directory and Identity &
Access. Its join tests build staff rows through
`staff-directory-view.internal-test.school-membership-public`. The
employment writes of live staff (notes and the birthday opt-out) run through
`staffoffboarding.LiveStaffEmployment`, whose public role already reaches
School Membership and Workforce; the membership lock stays first and the
Workforce write runs inside it.

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
`schema_version` only when the JSON wire grammar changes. Change `policy_epoch` only
for a reviewed architecture decision; it does not approve or rebuild legacy
findings.

Policy candidates must use `schema_version: 5`. Schema 4 added optional
`source_package` and `target_package` rule selectors (ADR 0057 addendum,
#4078). Each is an exact, classified module-relative package path. An omitted
selector keeps the owner/role reach; an explicitly empty, null, unknown,
absolute, unclean or wildcard selector is rejected. Selectors intersect the
existing owner, owner-kind, role and scope constraints; they cannot replace
those constraints or bypass target-only validation. `source_package` also
narrows an external-class rule; `target_package` is first-party only.

Adding a selector to an otherwise unchanged rule is narrowing. Removing or
changing it is compared as a possible permission increase, including paths
whose owner/role pairs are identical. Exact selectors also participate in
rule-overlap detection, permission memoization and the target owner graph.
The completed #4078 cutover removed its owner-transition acceptors and
historical Common fixtures. Ordinary ownership, classification and permission
strictness applies to every candidate; selectors remain intersections only.

Schema 5 replaces the removed `inbound` kind with the distinct engineering
kind `tool`. The canonical tooling owners are `architecture-check` and
`dev-tools`; all six existing package roles and test roles remain unchanged.
Tools own no writable data and are not nodes of the domain dependency graph.
No live policy may name `inbound`, including an owner-kind rule selector.
`own-test.tool.contract` succeeds `own-test.inbound.contract` with identical
scopes, target and same-owner restriction; no other rule changes.

Only `LoadBasePolicyAndManifest`, reading an immutable Git base, accepts
schemas 2, 3 or 4. Schema 2/3 still rejects package selectors, and every old
schema rejects the new `tool` spelling. Full historical validation runs before
any wire upgrade. The final schema-4 inventory, whose only inbound-kind
owners are the two canonical tools, uses schema-5 tooling vocabulary in the
ordinary strictness comparison. Owner IDs, package classifications, scopes
and permissions do not change. Older histories with actual inbound HTTP
owners remain historical schema-4 policies, never aliases for tools or an
ownership-transition exemption. Candidates never use this decoder path.
Schema 1, future schemas, unknown fields and invalid metadata still fail.
Evidence and projection schemas remain separate contracts.

#3416 leaves `policy_epoch` at the checkout's existing value, 17. Adding issue
metadata and deleting unused permissions grants no new architectural permission
and must not unlock the epoch-gated strictness exceptions. Add positive and
negative fixtures when changing these contracts, and remove every `rules.stale`
finding before committing a policy change.

The finished cutovers of ADR 0030 (Care Schedule), 0035 (Care Lifecycle),
0036 (Student Presence nest), 0037 (shift planning), 0038 (timetable
planning), 0052 (time tracking) and the Enrollment application move had
epoch-gated paths that let the successors keep the retired packages' reach.
Each path required a base that classifies the retired package. Development
classifies none of them since those cutovers, and ADR 0057 makes such reach
exact debt instead of a rule, so #4070 deleted the paths and their tests. A
future cutover converts the retired reach into keys or removes it.

The guardian portal HTTP composition has a four-rule path
([ADR 0051](../../docs/adr/0051-guardian-portal-http-reaches-the-standing-http-runtimes.md),
#3421). In a reviewed epoch, `inbound-parent`/`http` may reach
`inbound-common`/`http`, `identity-access`/`adapter` and
`tenant-runtime`/`public` in production, and `inbound-parent`/`adapter-test`
may reach `identity-access`/`adapter` in both test scopes: the standing
reaches every other HTTP point holds. Epoch 39 registers them as
`inbound-parent.http.inbound-common`, `inbound-parent.http.identity-adapter`,
`inbound-parent.http.tenant-runtime` and
`inbound-parent.adapter-test.identity-adapter`. Other owners, roles, targets,
external classes and scopes stay forbidden. Since epoch 47 the session runtime
is `identity-access`/`http` (ADR 0057 point 3); both acceptors, this one and
the shared HTTP runtime's of ADR 0046, name that role instead of the adapter
role.

The standing composition and test reaches of
[ADR 0053](../../docs/adr/0053-owners-compose-themselves-and-the-roots-compose-the-owners.md)
(#4001) are the widest such path: a reviewed epoch may add any rule that lies
inside one of the four reaches, and nothing else. See
[Standing composition reaches](#standing-composition-reaches-adr-0053).

The server root is the HTTP host
([ADR 0056](../../docs/adr/0056-the-server-root-is-the-http-host.md), #4063).
In a reviewed epoch, `reviewedHTTPHostRule`
(`internal/architecture/http_host_rule.go`) accepts exactly six
`root-composition` rules, each pinned to one source role and one scope:
`compose` to the class `http-router`, to `inbound-common`/`http` and to
`identity-access`/`adapter` in production; `module-internal-test` to
`identity-access`/`adapter` and to the class `cli-config` in the internal
test scope; `cli` to `dev-tools`/`cli` in production. Like the test
infrastructure rule, it also admits the external-class imports its rules
imply. Epoch 46 registers them as `root-composition.compose.http-router`,
`root-composition.http-host.{inbound-common,session-adapter}`,
`root-composition.internal-test.{session-adapter,cli-config}` and
`root-composition.cli.dev-tools`, and reclassifies `middleware` and
`analytics` from `adapter` to their owners' `http` role, which
`root-composition.reach.platform.http` (#4064) covers. Another source owner,
role, target, class or scope stays forbidden. Epoch 47 (ADR 0057, #4070)
keeps three of the six: the session token runtime is
`identity-access`/`http`, so `root-composition.reach.domain.http` covers the
root's session imports and both session rules are deleted; `dev-tools`/`cli`
is another owner's private role, so `root-composition.cli.dev-tools` is
deleted and `cmd -> seed/api` and `cmd -> simulate` are keys (#4096).
Epoch 49 (ADR 0060, #4096) removes both keys: the dev tools are the binary
`cmd/devtools`, the only package in `dev-tools`/`compose`. Its eight
`dev-tools.compose.*` rules reach `dev-tools`/`cli`, the class `http-router`,
the `compose` role of `transaction-runtime`, `organization-tenancy`,
`identity-access` and `student-presence`, and the `public` role of
`tenant-runtime` and `security-runtime`, all in production only. PR mode
accepts them because they are anchored to a role only the candidate's new
package uses.

The repository Factory contraction has a two-permission path
([ADR 0049](../../docs/adr/0049-owner-compositions-construct-audit-and-settings-persistence.md),
#2743). In a reviewed epoch whose base classifies `database/repositories`
exactly as `legacy-composition`/`compose`, production
`legacy-composition`/`compose` may import `audit-platform`/`compose` and
`settings-platform`/`compose` may import `settings-platform`/`postgres`.
Epoch 36 registers them as `legacy-composition.to.audit-platform-compose`
(temporary, #2747) and `settings-platform.compose.postgres`.
`modules/auditlog/compose` builds the Audit persistence (`NewPersistence`)
and `modules/settings/compose` the setting stores; `services` obtains them
there instead of from Factory fields. The root package reached the Audit
composition only for the People Directory's audit seams until #2743
dissolved it and removed its policy entry, so the anchor no longer holds in
any later base and the exception cannot authorize another addition; the two
registered rules stay. Other roles, owners and scopes stay forbidden.
#4070 deleted the acceptor with the other finished cutover paths: its base
condition, `database/repositories` as `legacy-composition`/`compose`, no
longer holds in any base.

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
the fixtures still import stays exact debt on #2748; a rule for it would hide
it. Most of it does not fall with a carrier: it names implementation roles
their owners keep, so the fixtures leave them by moving to owner test support
or by reaching owner contracts. Only the legacy composition `services`
(#2747, #2751) and possibly the session token adapter (#3487) are dissolved
by a carrier.
The third cut of #2748 needed no rule: the fixtures map the device, room,
activity and schedule rows they write themselves, the API helpers bind the
kiosk staff principal from IDs and re-export the People Directory suite
constructors from the legacy composition, and twelve keys fell (191 -> 179).
Slice A of #3944 moved the mail, broadcaster, Argon2 and device doubles to new
owner test-support points without a rule (177 -> 173).

[ADR 0047](../../docs/adr/0047-shared-fixtures-reach-the-security-and-settings-contracts.md)
(#3944, slice B) adds `security-runtime/public` and `settings-platform/public`
to those standing targets under the same shape, and pins three owner-suite
reaches for the tests that left the fixture package: the settings suite
(`settings-platform`/`module-internal-test`, internal_test) to
`security-runtime/public`, and the Audit integration suite
(`audit-platform`/`adapter-test`, external_test) to `enrollment/test-support`
and `care-plan/test-support`. A wider scope, another role or another target
stays a loosening. Epoch 35 registers five rules under it and seven keys fall
(173 -> 166). `configtest` kept `services/config` then: the settings schema it
returned carries `models/config` types no contract may expose.
[#3965](https://github.com/moto-nrw/project-phoenix/issues/3965) needed no
rule either: no consumer read the schema. Every consumer that took the full
settings service declares a narrow port; the supervision dashboard's is
`supervisiondashboard.Resolver` in its public ports, and the group live
projection gets the same `grouplive.Resolver`. Both carry `Prefetch`, a new
settings-service read that resolves keys at once and returns the context with
the snapshot, so neither legacy adapter names a snapshot type or imports the
settings application any more, the settings constructor returns the batch
service, and both legacy trees shrink (710 -> 701, 548 -> 539). Each test
package keeps a local func-field `settingsDouble` with only the methods its
ports need, and `configtest` is deleted with its package entry. Both `configtest` keys fall,
and the now unused rules `delivery.application.settings`,
`communication.parent-compatibility.application-settings-application`,
`calendar-view.adapter-test.settings-platform-application`,
`calendar-view.adapter.settings-platform-application` and
`group-live-view.adapter.settings-platform-application` are deleted
(166 -> 164).
[#3970](https://github.com/moto-nrw/project-phoenix/issues/3970) gives the
PostgreSQL tenant runtime bootstrap an owner seam,
`modules/transactionruntime/compose` (`transaction-runtime`/`compose`), and
adds it to the shared fixture targets in a narrower shape: only the
`test-support` role in production may reach it (the addendum to ADR 0047).
Epoch 37 registers `shared-fixtures.transaction-runtime-compose` with the
seam's own rules, which are anchored to the new package. The seam reaches the
Audit context adapters through `modules/auditlog`; `test -> database` falls
(112 -> 111). The roots `api` and `cmd` build their runtimes and pools there
too, under two rules anchored to the new package, and
`services.BindTenantRuntime` is deleted.
[#3966](https://github.com/moto-nrw/project-phoenix/issues/3966) needed no
rule either: the People Directory fixtures return test-owned rows (`Person`,
`Staff`, `Teacher`, `Student`, `GuardianProfile`, `StudentGuardian`, `Guest`,
`PrivacyConsent`, `ClassListEntry`, `ParentAnnouncement`,
`CareWithdrawalCompletion` in `test/people_rows.go`), and a caller that hands
the owner's model to People Directory code reads it back through the owner or,
where it may import `models/users`, builds it itself. The withdrawal fixtures
write the pending task row instead of handing a model to Care Plan. The
review-policy student, the substitution query options and the timetable
operations' person double name the retained models from the legacy
composition's suite helpers (`services/*_test_helpers.go`), the path the
People Directory suites already compose through. Both `test -> models/users`
keys fall without a new key (111 -> 109), and the rule
`people-directory.behavior-test.enrollment-support` is deleted with its last
importer.
[#4014](https://github.com/moto-nrw/project-phoenix/issues/4014) needed no
rule either: the fixtures map their `enrollment.care_offerings` row
(`CareOffering` in `test/care_offering_rows.go`) and take the phase, request,
child and care-offering values from `modules/enrollment/enrollmenttest` under
the existing `test-support.to.enrollment-support` rule. The care-offering
values `fixed` and `parent_choice` have no public owner constant,
so Enrollment test support names the stored column values. The Timetable
template suites insert their source offerings through the fixtures, and the
then unused `Factory.EnrollmentCareOfferingRows` is deleted. `test ->
models/enrollment` falls without a new key (60 -> 59).
[ADR 0053](../../docs/adr/0053-owners-compose-themselves-and-the-roots-compose-the-owners.md)
point 7.3 (#4001) adds `identity-access/adapter` to the standing targets in
the same shape: the session token adapter is the HTTP session runtime (ADR
0031, ADR 0046), and the fixtures reach it for the same reason they reach the
tenant runtime. Epoch 41 registers `shared-fixtures.identity-access-adapter`,
and `test -> modules/identityaccess/legacy/jwt` falls (#2748). Epoch 47 (ADR
0057) names the runtime's new role, `identity-access`/`http`, among the
targets and drops `legacy-shared`/`domain`: the wrapper is another owner's
private role, so the fixtures' `internal/timezone` imports are keys (#4081).
ADR 0047's two pinned Audit suite reaches into Enrollment and Care Plan test
support leave the owner-suite shapes for the same reason (platform into a
domain owner, #4072).

A reviewed epoch may also add the one owner-agnostic shared-kernel rule
([ADR 0040](../../docs/adr/0040-every-role-may-import-the-shared-kernel.md),
[#3447](https://github.com/moto-nrw/project-phoenix/issues/3447),
[#3448](https://github.com/moto-nrw/project-phoenix/issues/3448)): no
`source_owner`, no `source_owner_kind`, no `source_role`, all three scopes, and
the target `shared-kernel`/`contract`. Epoch 25 registers it as
`shared-kernel.contract` and removes the 15 owner-specific rules it subsumes, so
a per-owner rule to the kernel would now overlap and fail to load. A
`same_owner` rule never selects a kernel owner: the kernel's own tests reach
its contract through this rule alone. A narrower source, another target, an
owner-kind target and an external class are still loosenings.

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
[ADR 0045](../../docs/adr/0045-owners-adopt-tables-a-retired-generic-store-reached.md)
(#2706) adds a second evidence for a table the base reached only through a
generic store's unresolved expression: the base records a
`tables.unresolved` finding for a package the candidate no longer
classifies, and an analysis of the candidate with the table assigned to a
non-existent owner shows it named statically by packages of the adopting
owner and no other.

## Appendix: principle traceability

Every rule that still holds appears once, with its sources and the one
principle that states it. Principles: 0066 ownership, 0067 contracts and
workflows, 0068 construction and runtime, 0069 tests and shared kernel, 0070
ratchet and evidence, 0071 dependency direction (links in
[Architecture principles](#architecture-principles)). A point refers to the
numbered decision of the principle ADR.

### Rule → principle

| Rule that holds | Sources | Principle |
|---|---|---|
| 18 domain and 10 platform modules, pinned by validation; a change needs its own decision | ADR 0010, ADR 0012 | 0066 point 1 |
| A domain owns a business concept, a platform a runtime; File Storage, Document Rendering and Export Transfer boundaries | ADR 0010, ADR 0012 | 0066 point 2 |
| Projection, workflow, tool, composition, kernel, migration and test-support owners are not modules; no `inbound` kind | ADR 0057 and ADR 0060 addenda | 0066 point 3 |
| One runtime write owner per table; no invented owner for an obsolete table | this README | 0066 point 4 |
| Projections and workflows own no writable data | ADR 0012, ADR 0013, ADR 0023 | 0066 point 5 |
| A foreign read uses the owner's capability or an exact tenant-safe projection; a projection only for a joined statement, named after its read | ADR 0023, ADR 0037, ADR 0052 | 0066 point 6 |
| Installation-wide `tenant_safe` means operator scope; a repeated owner literal changes with its owner | ADR 0033 | 0066 point 7 |
| Student record split, class-writes gate, public student ID, child documents, work-session edit audit | ADR 0023, ADR 0025, ADR 0035, ADR 0052 | 0066 point 8 |
| `public` and `contract` expose no ORM, repository, internal model or generic CRUD | this README, ADR 0046 | 0067 point 1 |
| A foreign owner is reached through its contract or a consumer-owned port the root binds | ADR 0053 point 4 | 0067 point 2 |
| Owners change data through commands that own their guards | ADR 0023 | 0067 point 3 |
| A write across owners is a workflow with one outer unit of work, reversible or not | ADR 0013, ADR 0016, ADR 0017, ADR 0018, ADR 0022, ADR 0023 amendment | 0067 point 4 |
| A workflow re-reads under its locks; the tenant-wide gate order | ADR 0016, ADR 0017, ADR 0023 | 0067 point 5 |
| A durable side effect commits with its write; callbacks are losable hints in an exact register | ADR 0013, ADR 0016, #4087 | 0067 point 6 |
| A cross-owner database function needs its own decision and narrow runtime authorization | ADR 0062 | 0067 point 7 |
| An owner composition builds its own owner, including its application and persistence; glue lives in the root | ADR 0053 points 1 and 3, ADR 0041, ADR 0049 | 0068 point 1 |
| The root reaches only `compose`, `public`, `contract`, `port` and `http` of the owners | ADR 0053 point 2 | 0068 point 2 |
| An owner composition reaches no foreign implementation, `compose` or `http` role | ADR 0059 point 6 | 0068 point 3 |
| Compositions construct and bind; the root holds wiring only | ADR 0053 point 4, #4082 | 0068 point 4 |
| No service graph, no late binding, no nil-tolerant production branch; each root builds what it runs | ADR 0059 points 1 to 5 | 0068 point 5 |
| The database reaches the roots as one Transaction Runtime value | ADR 0053 point 5, ADR 0047 addendum #3970 | 0068 point 6 |
| One session token runtime, built by Identity's composition; session facts cross ports as values | ADR 0031, ADR 0054, ADR 0053 point 6, ADR 0046 amendment | 0068 point 7 |
| A shared runtime is classified, not granted; `api/common` and `middleware` are Tenant Runtime `http`; HTTP roles reach the standing runtimes | ADR 0057 point 3, ADR 0046 amendment, ADR 0051, ADR 0056 point 3 | 0068 point 8 |
| The server root is the HTTP host; each handler belongs to the `http` role of its data's owner | ADR 0056 point 1, ADR 0058 addendum #4079/#4314 | 0068 point 9 |
| The Worker's contract is Scheduler Runtime's `port` role | ADR 0056 point 4 and amendment, ADR 0059 point 5 | 0068 point 10 |
| The dev tools are the binary `cmd/devtools`, absent from the production image | ADR 0060 | 0068 point 11 |
| No alias or re-export stands in for moved code | ADR 0050, ADR 0053 option 4 | 0068 point 12 |
| Test roles name the seam; production permissions stay out of test scopes | this README | 0069 point 1 |
| Test roles import ORM, router and test framework directly | ADR 0014 | 0069 point 2 |
| A test of one owner uses that owner's roles and fakes of its ports | ADR 0053 point 7.1 | 0069 point 3 |
| A test of several owners builds the typed root | ADR 0053 point 7.2, ADR 0059 point 5 | 0069 point 4 |
| Shared fixtures create rows and hold a finite set of standing targets | ADR 0039, ADR 0047, ADR 0053 points 7.3 and 7.4 | 0069 point 5 |
| A test driving a foreign implementation moves to that owner's suite | ADR 0047 | 0069 point 6 |
| Every module's public contract has an inventoried contract test, 28 of 28 | #4090 | 0069 point 7 |
| Every role may import the closed shared kernel | ADR 0040 | 0069 point 8 |
| A rule states the target only | ADR 0057 points 1 and 2 | 0070 point 1 |
| Everything else is an exact key with one open carrier; a cutover never keeps old reach as a rule | ADR 0057 point 2 and consequences, ADR 0030, ADR 0035 to ADR 0038, ADR 0052 | 0070 point 2 |
| `models/*` carry `legacy-model`, which no foreign owner imports | ADR 0057 point 4 | 0070 point 3 |
| Ratchet budgets only fall; a raise needs an ADR the change adds | ADR 0057 point 5, ADR 0063, ADR 0064, ADR 0072 | 0070 point 4 |
| Pass-throughs count per owner; moved code stays in the scan | ADR 0061, ADR 0064 | 0070 point 5 |
| A relocation renames keys and grants nothing | ADR 0020 | 0070 point 6 |
| The epoch rises only for a reviewed decision with a finite, tested acceptor; dead acceptors are deleted | ADR 0053 point 9, ADR 0013, ADR 0015, ADR 0045, ADR 0057 point 7 | 0070 point 7 |
| Exact package selectors only narrow | ADR 0057 addendum of 2026-10-16 | 0070 point 8 |
| Migration evidence keeps its provenance and fails closed | ADR 0026 | 0070 point 9 |
| The target owner graph is acyclic | ADR 0058 point 1 | 0071 point 1 |
| Each owner kind depends in one direction | ADR 0058 point 2 and addendum #4314 | 0071 point 2 |
| Two domains that need each other get one direction | ADR 0058 point 3 | 0071 point 3 |
| People Directory lowest, Care Plan above it, Enrollment above Care Plan, Workforce above Student Presence | ADR 0058 addendum #3417 | 0071 point 4 |
| A domain's implementation follows the owner's existing edges | ADR 0058 addendum #4084 | 0071 point 5 |
| The cycle measurement counts every production package | ADR 0058 point 4 | 0071 point 6 |
| Tools are outside the dependency graph | ADR 0060 addendum | 0071 point 7 |

### Completed parts without a rule

These parts of accepted ADRs describe one-time work that is done; no rule
follows from them: ADR 0010's count of nine platform modules (ADR 0012 added
the tenth); ADR 0031's timing (replaced by ADR 0054, whose relocation is done);
ADR 0032 (superseded by ADR 0058; its platform-triangle cut holds no edge any
more); ADR 0040's wrapper rules (the wrapper is deleted, #4081); ADR 0046's
`inbound-common` owner (replaced by its amendment); ADR 0047's `configtest`
keys (#3965); the compatibility bindings of ADR 0017, ADR 0018 and ADR 0022;
ADR 0053 point 8, the way out of `services` (deleted); ADR 0057 point 6, the
one-time conversion (#4070); ADR 0058 points 5 and 6, the transition (#3417,
#4072).

### ADR → principle

| ADR | Status | Principles |
|---|---|---|
| 0010, 0012, 0033 | accepted | 0066 |
| 0013 | accepted | 0066, 0067, 0070 |
| 0014, 0039, 0040 | accepted | 0069 |
| 0015, 0020, 0026, 0045, 0061, 0063, 0064 | accepted | 0070 |
| 0016, 0017, 0018, 0022, 0062 | accepted | 0067 |
| 0023 | accepted | 0066, 0067 |
| 0025 | historical | 0066 |
| 0030, 0036, 0038 | historical | 0070 |
| 0031, 0051, 0054, 0056 | accepted | 0068 |
| 0032 | superseded by ADR 0058 | none; its successor is in 0071 |
| 0035, 0037, 0052 | historical | 0066, 0070 |
| 0041, 0049, 0050 | historical | 0068 |
| 0046 | accepted | 0067, 0068 |
| 0047, 0059 | accepted | 0068, 0069 |
| 0053 | accepted | 0067, 0068, 0069, 0070 |
| 0057 | accepted | 0066, 0068, 0070 |
| 0058 | accepted | 0068, 0071 |
| 0060 | accepted | 0066, 0068, 0071 |

ADRs 0001 to 0009, 0011, 0019, 0021, 0024, 0027 to 0029, 0034, 0042 to 0044,
0048 and 0055 record product, data-contract and operations decisions outside
#2580 and are not part of this map.
