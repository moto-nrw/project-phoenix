# Command-effect reconciliation (#4739, all domain sources)

Historical second-census snapshot: later corrections and current counts are
recorded in [ADR 0094](../adr/0094-command-effect-gap-reconciliation.md).
Its round-2 addendum covers query direction debt and the remaining domain-owned
callback-field gap; the counts below are not the current register.

Second census starts at fetched `origin/development` `673652d030` (PR #4822),
then integrates `62bc33f3c5` (#4826 and #4760) and `e2ac57ec88`
(#4828, #4829 and #4757). Regeneration after those merges changes no effect
identities or fingerprints.
Paths below start at `backend/`. The original hand inventory references its own
immutable pre-change tree; no historical protocol evidence is silently replaced.

The [machine register](../../backend/architecture/command-effects.json) contains
**5341 exact source-owner/port/binding/target identities**. These are type-level
may-bind methods, not runtime calls or owner-pair counts. The first 128 entries
retain their labels and fingerprints; the wider provider resolution also exposes
new Enrollment identities. The generator preserves exact labels and leaves new
entries `unknown` until reviewed. It does not infer effects from names.

All **18 policy domain owners** are scanned, including their production-compiled
support, aliases, native contract fields and additive testhooks. Feedback,
Meal Plan and Appointments currently have no discovered foreign binding site;
they are not excluded by a hardcoded source list. Fixtures also cover an unknown
future policy domain. One shared foreign interface retains separate source-owner
identities for multiple consumers.

Effects: **1 durable, 298 lock, 1611 mutation, 5 notice, 2779 query, 7 scope, 640 unresolved**. Review labels: **46 E1, 18 E2, 48 E3, 58 E4, 32 E5, 73 debt, 1621 leaf, 2779 query, 640 unresolved, 26 workflow**. The 640 unresolved entries represent
**253 distinct exact unresolved targets**, often repeated through shared/full
interfaces. They are visible limitations, not approved leaves.

`ListVisits`, `ListAttendance` and `QueryGroupSupervisions` are conditional
locking reads: `ForUpdate=true` requires an ambient transaction and executes
`SELECT FOR UPDATE`. All 26 identities for these complete method surfaces are
classified as `lock`/`leaf`, including bindings whose current callers only read.

## All 40 hand-inventory rows

The previous 32 “outside the source-owner slice” rows are no longer out of scope.
Every row has a machine comparison below; rows 5, 24, 30, 32 and 36 explain
callable, direction, no-write or shared-method granularity rather than claim an
unqualified one-row/one-entry match. The former row-2 first-hop gap is covered at
both actual source owners without flattening it into an invented edge.

| Row | Manual edge / operation | Machine comparison and reason |
|---:|---|---|
| 1 | Enrollment decision → Care Plan materialization/adjustment/pickup | Covered: `DecisionBookings` and Care resync/materialization/pickup ports. E1, including the explicit locks; not leaf. |
| 2 | Enrollment decision → Timetable sourced templates, via Care Plan | Covered as two real first hops: Enrollment → Care Plan resync, then Care Plan `BookingTemplates`/`BookingRosters` → Timetable (E4). No invented direct Enrollment → Timetable edge. |
| 3 | Enrollment decision → Identity guardian tenant access | Covered: `DecisionGuardianAccess.GrantGuardianTenantAccess` → Identity Owner. E1; account reads remain queries. |
| 4 | Enrollment decision → People student enrollment/profile | Covered: alias `modules/peopledirectory/enrollment.Commands`, including additional Owner providers. E1; `ReadEnrollmentStudent` is lock-capable, not query. |
| 5 | Enrollment approved-child sync → People/Care Plan | Covered with a bounded deviation: People/Care Plan and direct weekly Timetable effects are recorded E1. `DepartureCompanions.DeleteCompanionEdges` remains exact unresolved function-field evidence, not an inferred safe call; shared after-commit protocol still needs runtime review. |
| 6 | Care end/cancel → School Membership | Covered: `CareStudentDirectory`/`CareExitEnrollment` → School Membership, E2; Care Exit Roster/Bookings providers are explicitly workflow-backed finite callbacks, not normal leaves. |
| 7 | Care resume → School Membership | Covered: `CareResumption.Resume` → careexit.Resumption. Workflow-backed outer HTTP entry supplied by root. #4753 changes this seam separately; ADR 0071 is untouched here. |
| 8 | Identity school identity → People person/link/tag | Covered: Identity `StaffDirectory`/`CallerPeople` → People person/account/tag commands. Leaf; exact target guards/ambient-unit evidence from hand row 8 stands. |
| 9 | Identity school identity → School Membership staff/teacher | Covered: Identity staff/caregiver commands → School Membership. Leaf; no new cohort/preview protocol is asserted. |
| 10 | Identity guardian profile create/link → People | Covered: Identity `GuardianDirectory` create/profile/account-link commands → People. Leaf insertion paths; locking upgrade is separate E3. |
| 11 | Identity guardian role upgrade → People relationship | Covered: `FindStudentGuardianLinkForUpdate`, student/profile locks and promotion targets. E3 role-upgrade surface, not all Identity writes. |
| 12 | Identity guardian link insert → People (and its pickup/access halves) | Covered: guardian-link insert and People-owned access/pickup commands. Leaf insertion; no implicit promotion approval. |
| 13 | Identity demo access → Organisation demo orders/retirement/last-use | Covered: `DemoSchools` prepare/replace/mark-use → Organisation. Guarded commands, leaf; root scope transitions remain separate non-query sites. |
| 14 | People student create/profile/renew → School Membership | Covered: People `StudentOwners` → School Membership guarded enrollment/renewal/class gate. E3 student/cohort/savepoint protocol. |
| 15 | People student create/profile/renew → Care Plan | Covered: People `StudentOwners.SaveCare` and companion surface → Care Plan. E3; foreign reads are still queries. |
| 16 | People relationship/pickup patch → Care Plan | Covered: `GuardianLinkOwners`, `GuardianPickupPermissionCommand` → Care Plan permission patches. Leaf; People relationship lock is local, split-record savepoint remains required. |
| 17 | People relationship creation/permissions → Identity access | Covered: `GuardianStudentAccessCommand`/`GuardianLinkOwners` → Identity access commands. Leaf guarded access halves; no membership-only permission claim. |
| 18 | People staff edits → School Membership staff | Covered: People `StaffRepository` create/update/delete → School Membership. E3 staff-identity edits. Birthday opt-out is separately workflow-backed, Schuld #4825. |
| 19 | People teacher edits → School Membership caregiver | Covered: People `TeacherRepository`/caregiver-role commands → School Membership/Identity. E3; tolerated caregiver failure is not converted to all-or-nothing behavior. |
| 20 | School Membership staff → Workforce employment | Covered: School Membership composition `StaffEmployment` → Workforce SaveStaffEmployment/StaffEmployments. Leaf target savepoint/record guards. |
| 21 | Workforce staff edits → School Membership employment fields | Covered: Workforce `StaffAdminSubjects` → School Membership lock/patch commands. E3 foreign-subject/audit protocol. |
| 22 | Workforce staff name/contact edits → People | Covered: the same `StaffAdminSubjects` port → People name/contact patch. E3, not a merged owner. |
| 23 | School Structure class/group assignments → School Membership | Covered: School Structure teacher/class/group assignment ports → School Membership. Leaf guarded assignment commands; query/lock sites remain distinct. |
| 24 | School Structure group handover → Workforce | Covered and qualified: `GroupHandoverStore` → shiftplansync.GroupHandovers Create/Delete is workflow-backed as the hand row says W. It is also runtime-direction debt under ADR 0071, not permission for this domain callback. |
| 25 | Timetable check-in/out/move → Student Presence | Covered: Timetable `OperationVisits`/attendance/session ports → Student Presence commands, including fuller supplied interfaces. Leaf authority A from #4736/PR #4742 remains authoritative, not a claim that Timetable and Presence permissions are identical. |
| 26 | Timetable cancellation → Communication announcement | Covered: `GuardianNotices.PublishCareCancellation` → Communication. Leaf; target persistence/outbox is not a harmless SSE callback. |
| 27 | Student Presence check-in status clear → Care Plan live flags | Covered: Presence `StatusDayStudents`/status-day ports → Care Plan live-flag commands. Leaf, with the hand row affected-count/failure evidence retained. |
| 28 | Student Presence NFC start → Workforce check-in | Covered: `WorkSessionService.EnsureCheckedIn` → Workforce. Leaf, explicitly best effort with savepoint, not guaranteed work-session creation. |
| 29 | Care Plan offering roster/source change → Timetable | Covered: Care `BookingRosters`/`BookingTemplates`, catalog/reconciliation ports → Timetable. E4, class/recurrence/child ordering retained. |
| 30 | Alleged Care Plan booking materialization → People class/status writes | Correction confirmed: `BookingStudents` supplies People reads/locks and Membership class gate, not a People class/status mutation at this binding. Locks are not labelled query. |
| 31 | Care Plan master-data approval → People student/person | Covered: `MasterDataDirectory`/`MasterDataRecords` → People locked child/person writes. E4; pending/version/authorization and #4640 ordering remain required. |
| 32 | Care Plan HTTP child create → People person/student/guardians | Covered with granularity note: `http.PersonRecords.CreatePerson`/cleanup Delete plus full native People Capability create/student/guardian methods. E4 HTTP child creation, not all exposed People methods. Full supplied native fields also occur in compiled support; this is may-bind coverage, not call-site provenance. |
| 33 | Enrollment intake → Care Plan booking intervals | Covered: `CareBookingCommands.RecordCareBookings` → Care Plan RecordCareOfferingBookings. Leaf. Multiple resolved providers are method identities, not separate runtime operations. |
| 34 | Enrollment rollover → Care Plan booking intervals | Covered by the same exact booking command as row 33. Rollover approval remains E1, not reclassified by this leaf recording edge. |
| 35 | Enrollment form-change apply → Care Plan replace bookings | Covered: `CareBookingChanges.ReplaceCareBookings` → Care Plan ReplaceCareOfferingBookings, leaf. Approved-child resync/gates remain separately E1. |
| 36 | Organisation school creation → Device Fleet | Covered and qualified: `ProvisioningDevices.CreateDevice` serves both school creation (this leaf operation) and E5 device transfer. The shared exact port surface is conservatively E5; this is no new transfer exception. |
| 37 | Organisation device transfer → Device Fleet | Covered: provisioning-device locked read/archive/create/history command surface, E5. No independent Device Fleet commit is invented. |
| 38 | Organisation person retirement → People | Covered: `ProvisioningPeople` anonymize/unlink/account/tag retirement commands, E5; tolerated supervision/account failures remain as documented. |
| 39 | Organisation provisioning/retirement → Identity | Covered: `ProvisioningIdentity` account/invitation/role/session effects plus explicit WithinAdmin/WithTenantID scope sites, E5. Split tenant/admin units and best-effort audit remain visible, not one atomic operation. |
| 40 | Organisation school creation → Timetable categories | Covered: `ProvisioningCategories.SeedCategories` → Timetable CreateCategory, leaf in the school-creation admin unit. |

## Counts by policy source owner

Counts include each exact supplied method/provider and unresolved target. Zero
is deliberate. `Q/M/L/S/D/N/U` mean query/mutation/lock/scope/durable/notice/
unresolved, not a runtime count. E labels describe named retained source
protocols; workflow describes implementation backing, not direction approval.

| Source owner | Entries | Q | M | L | S | D | N | U | Leaf | Workflow | E1 | E2 | E3 | E4 | E5 | Debt |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| appointments | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| care-plan | 1091 | 576 | 364 | 83 | 0 | 1 | 3 | 64 | 351 | 8 | 0 | 18 | 0 | 58 | 0 | 16 |
| communication | 86 | 60 | 2 | 3 | 0 | 0 | 0 | 21 | 5 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| device-fleet | 118 | 92 | 8 | 16 | 0 | 0 | 0 | 2 | 24 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| enrollment | 185 | 113 | 56 | 9 | 0 | 0 | 0 | 7 | 19 | 0 | 46 | 0 | 0 | 0 | 0 | 0 |
| facilities | 117 | 69 | 48 | 0 | 0 | 0 | 0 | 0 | 48 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| feedback | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| file-storage | 28 | 20 | 7 | 1 | 0 | 0 | 0 | 0 | 8 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| identity-access | 235 | 137 | 37 | 5 | 0 | 0 | 0 | 56 | 37 | 0 | 0 | 0 | 5 | 0 | 0 | 0 |
| meal-plan | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| organization-tenancy | 86 | 30 | 27 | 1 | 7 | 0 | 0 | 21 | 3 | 0 | 0 | 0 | 0 | 0 | 32 | 0 |
| people-directory | 347 | 155 | 41 | 13 | 0 | 0 | 0 | 138 | 20 | 2 | 0 | 0 | 32 | 0 | 0 | 0 |
| school-calendar | 189 | 96 | 56 | 9 | 0 | 0 | 0 | 28 | 8 | 0 | 0 | 0 | 0 | 0 | 0 | 57 |
| school-membership | 15 | 14 | 1 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| school-structure | 351 | 184 | 88 | 11 | 0 | 0 | 0 | 68 | 95 | 4 | 0 | 0 | 0 | 0 | 0 | 0 |
| student-presence | 836 | 416 | 315 | 44 | 0 | 0 | 2 | 59 | 361 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| timetable-activities | 1322 | 625 | 519 | 69 | 0 | 0 | 0 | 109 | 583 | 5 | 0 | 0 | 0 | 0 | 0 | 0 |
| workforce | 335 | 218 | 42 | 8 | 0 | 0 | 0 | 67 | 32 | 7 | 0 | 0 | 11 | 0 | 0 | 0 |

## Review evidence and interpretation

- The 40-row source/transaction/lock audit in the hand inventory remains the
  protocol basis for E1–E5 and the simple record commands. New exact entries
  cite their target contract/implementation file and that protocol where used.
  No E1–E5 exception is inferred from an owner's name. Full native interface
  fields can expose many unused methods; they are not proof of production use.
- Read/value/construction helpers are queries, including DTO `SetTenantID`,
  error translation, calendar rendering and `ContextWithCompanionChangeRecorder`.
  Administrative `WithinAdmin`/`WithTenantID` are scope effects. Lock-capable
  `ReadEnrollmentStudent`, `Find*ForMutation`/`ForUpdate`/`ForShare`, class/row
  gates and projection `Hold*` permission leases are locks, not queries.
- People `GuardianPaymentMasked`, `RevealGuardianPayment`, `ListPaymentOverview`
  and `ListPaymentExportRows` write their required access log. They are
  mutations, despite serving reads (`guardian_payment_service.go:45,62,243,250`).
  The log's failure must still refuse the read. Communication pill emission
  mutates persistent messages; `EnqueueRequestDecision` is durable delivery
  intent. The current Care messenger preserves the ambient context and emits
  pills before commit (`care_request_effects.go:213-230`, root
  `offering_change_composition.go:496-514`, Communication `events.go:116-144`);
  older “pills after commit” comments are not evidence of current behavior. Only `BroadcastChildUpdateToGuardians` is a non-durable notice
  (`modules/communication/internal/application/parentmessaging/emitter.go:145`).
- The two original Enrollment callable limits remain: `enrollment_decision_people.go:566`
  calls `c.delete` (supplied by `root_enrollment_services.go:99`); the generic
  School Directory helper's `find` parameter is called at `api/school_directories.go:185`.
  Additional generic instantiated adapters, anonymous interfaces, dynamic
  callable fields/parameters and type-level root forwarding cycles remain exact
  unresolved entries. A known first-hop contract and an unresolved provider can
  coexist; the latter is not downgraded to the former's query label.
- Foreign contract terminals are not expanded into their implementation's
  transitive commands. Runtime provider forwarding is different: a foreign
  interface supplied by another owner, including a workflow, is exposed.
  Tuple-forwarded returns are supported; arbitrary tuple assignments,
  variadics, generic instance/container/function/reflection flow are not solved.
- The same interface's assignments are conservatively unioned across roots and
  compiled support. This can overapproximate actual wiring or infer type-level
  cycles that no live instance has. Root/source call sites and runtime protocol
  tests still decide liveness, authorization, scope and concurrency correctness.

## Workflow backing and newly visible debt

`workflow` is not a normal leaf or a new permission for domain callbacks.
The concrete ShareVisibility provider behind the Communication interface is
now visible; all corresponding first-hop reasons carry **workflow-backed,
Schuld #4825**. LiveStaffEmployment opt-out entries carry the same label.
`StaffMembershipRepository.AddNotes` has no discovered consumer-port binding or
production caller in this cut, so no invented live edge is added for it.

| Finding / surface | Source evidence | Placement and proposed follow-up (not created) |
|---|---|---|
| Care → Parent Portal Share-Visibility | `modules/careplan/internal/application/care_request_effects.go:305`, `offering_change_notices.go:130`; `roots/owners/root_careplan_requests.go:43` | Workflow-backed, Schuld #4825. Move recipient facts to an owner/projection or make this a root workflow entry. Not leaf. |
| People → Staff Offboarding LiveStaffEmployment | `modules/peopledirectory/internal/application/birthday.go:123`; `roots/owners/staff_membership_adapters.go:545` | Workflow-backed, Schuld #4825. Keep the membership-first lock/check, but relocate the command or outer entry. Not leaf. |
| School Structure → Shift Plan Sync assignment/end and group handover | `modules/schoolstructure/internal/application/groups/substitution_module.go:298,338,397,453` | Confirmed application callbacks. The hand row's W correctly describes backing, not an ADR 0071 exception. Proposed complete substitution/handover entry cut; do not add to E2. |
| Workforce → Shift Plan Sync sickness cascade | `modules/workforce/internal/application/timetracking/staff_absence_creation.go:126,218`, `staff_absence_mutations.go:205,320` | Confirmed application callbacks outside the E3 staff-master-data exception. Proposed absence/cascade workflow entry preserving sick-stamp reconciliation and rollback. |
| Care → Parent Portal day-cutoff policy | `modules/careplan/carerequests/submission.go:63`, `internal/application/request_edits.go:63` | Read-only workflow callback, not a new write coordinator. Proposed owner/shared policy relocation, not a new E exception. |
| File Storage → Parent Portal attachment audience | `modules/filestorage/internal/application/attachments.go:435,452` | Read-only workflow callbacks. Proposed tenant-safe owner/projection capability; do not treat backing as direction approval. |
| School Calendar → Appointments lifecycle/reminders | `modules/schoolcalendar/portal/internal/application/appointment_lifecycle.go:194,253,274,338` | `debt`, not E1–E5: locked foreign appointment/organizer re-read, recurrence/recipient changes and notification coordination. Proposed full portal lifecycle/reminder workflow cut. |
| Care family-request decision/correction | `modules/careplan/internal/application/request_decisions.go:72,88`, `excused_request_decisions.go:38`; weekly departure writes via `WeeklyApprovalPeople` | `debt`, not a widened E4: locked foreign child, combined decision, departure/live-absence mutation and own request ledger/audit. Proposed full family-request decision/correction workflow, preserving #4640 ordering. |

The finite ADR 0071 / PR #4753 **Care Exit Roster and Bookings snapshot/restore**
callbacks are explicitly workflow-backed E2 evidence. Care resumption is an
outer HTTP entry, not an application callback. Timetable's pickup-extension
workflow is called from `timetable/http/pickup_extensions.go:162`; its full
interface/promoted methods are a supplied workflow entry, not evidence that the
Timetable application calls back into it. Data-import observed providers are
compiled support/contract-union evidence, not proof that Workforce invokes its
import workflow at runtime. `dataimport.MapRelationshipType` is a pure helper.

These are placement/protocol findings, not demonstrated production incidents or
new authorization/atomicity guarantees. This change migrates none of them and
creates no issue. Accepted E1–E5 sites stay separate from the debt classification.

## Gates, counterprobe and remaining recognition cuts

The ADR 0083 domain-source census decision is a one-time review against the
immutable base. Remove that new addendum and the expanded exceptions/debt/
unresolved entries must fail. Existing labels/fingerprints get no implicit
approval; newly inferred providers/diagnostics must belong to already reviewed
source/port/bindings, or to an as-yet unscanned source owner. A new source port or
reuse of a merged addendum is rejected by fixtures.

Fingerprints retain called root helpers. Binding-only digests would miss changed
tenant mappings, ordering and failure handling with the same target set. Fixtures
pin changes to the outer binding and to a called helper. Query/leaf labels remain
review assertions; canonical JSON and target-method effect consistency do not
prove the labels true.

Remaining recognition cuts, proposed but not created:

1. **Binding flow / instance provenance:** function-valued, generic,
   tuple-assignment, variadic/container, anonymous-only source ports, foreign
   interfaces used solely as parameters and embedded foreign dependency-object
   flows; direct foreign calls outside supplied interfaces; precise instances,
   liveness and relocation evidence. Current exact unresolveds stay visible.
2. **Platform and protocol effects:** complete platform durable/scope/notice
   inventory, transaction/savepoint and authorization boundaries, coordinated
   locks and after-commit guarantees. The few exposed domain terminal effects
   above are not a claim of a complete platform/protocol census.

[ADR 0098](../adr/0098-command-effect-binding-flow-and-platform-protocols.md)
implements the function-valued, generic and platform/protocol parts of points 1
and 2 for #4836. Direct foreign calls outside supplied interfaces, anonymous-only
source ports and protocol proofs (transaction, lock order, authorization,
after-commit timing) remain outside the static inventory.

The original 17-source-domain follow-up is complete. Issue closure concerns the
inventory gate's acceptance criteria, not migrating the new placement findings.

## Local verification and warm check measurements

Measured serially with the pinned Go 1.27.3 toolchain (default `-p=4`,
`GOMAXPROCS=4`), from this worktree. An unmeasured check warmed each initial
sample; the final integration check followed regeneration with warm build caches.
No own tests/linter ran concurrently with the timed checks.

| State / immutable comparison base | Entries | Warm wall time | User CPU | System CPU |
|---|---:|---:|---:|---:|
| Before: Enrollment, `673652d030` | 128 | 29.95 s | 84.87 s | 19.13 s |
| All domains, `62bc33f3c5` | 5,341 | 41.58 s | 91.11 s | 22.63 s |
| Final development integration, `e2ac57ec88` | 5,341 | 56.33 s | 103.45 s | 23.26 s |

Inputs and possible machine contention differ, so these samples establish
neither an isolated causal slowdown nor a CI SLA/job duration. No average or
unmeasured speedup is claimed. The final check reports 5,213 added entries,
0 changed fingerprints, 0 legacy violations and composition surface 0 → 0.

The real counterprobe removes only this PR's new domain-source ADR addendum,
runs the normal `check`, and restores the exact original bytes in `finally`.
It rejects **877** new exception/debt/unresolved identities (exit 1); restoring
the addendum makes the normal check green. Fixture/base tests separately reject
reuse after merge, new Enrollment ports, changed existing exceptions and fake
approval identifiers. All 40 original hand rows remain accounted for above.
