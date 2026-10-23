# Cross-owner command inventory (#4538)

Decision: [ADR 0067 addendum](../adr/0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md#addendum-2026-10-23-command-participation-and-coordination-4538).
Inspected fetched `origin/development` at
`e033cf0f27d6ea8d16f24df81176da9ddf042d39` (2026-10-23), not the issue's
`ccf2209e96` or comment's `95507f6043`. Path:line references below describe
that immutable **pre-change** tree unless marked “new”. Paths start at
`backend/`; `roots/owners/` bindings preserve the passed context.

## Machine-checked first slice (#4739)

The generated/reviewed [command-effect inventory](../../backend/architecture/command-effects.json)
checks Enrollment's supplied interface bindings to foreign domains/workflows/
projections. The [40-row reconciliation](4739-command-effect-reconciliation.md)
records coverage, additional method surfaces and explicit gaps. It is a
conservative may-bind inventory, not a replacement for this protocol/lock
evidence; the other source owners and two exact callable bindings remain follow-up.

## Reading the table

This inventories **every example in the issue and comment**, splitting target
owners and distinct protocols. It is not a whole-repository command census.
Calls were followed from the named binding through the source application to
the target command/runtime. Flags are a static assessment, not a claim that
every interleaving was tested.

- **Tx J**: yes, the target joins the ambient tenant transaction. **Tx A**:
  yes, it uses the ambient administrative transaction. **J/A**: depends on
  the entry point; no independent target commit. **Split**: individual writes
  join, but the full operation has several units. **–**: no write edge.
- **L**: the coordinator needs a dedicated cross-owner lock order, not merely
  a target's local row/unique-key guard. **R**: it checks foreign state under
  those locks for a combined decision. **Aft**: it coordinates common
  after-commit effects. `Y/N` mean yes/no, with evidence in the last column.
  A transactional outbox publish and an owner-local invalidation alone are
  **N**. “N” means no *additional* protocol was found in this named path; it
  does not assert the absence of database locks or all possible races. A grouped
  row is Y if any listed branch needs that protocol; simple insert branches
  may not need every lock.
- **Leaf**: permitted by the addendum. **W**: workflow, implemented here or
  already present. **E1–E5**: workflow-shaped coordinator retained by the
  finite ADR exception, with its present guarantees and follow-up scope below.
  An HTTP lifecycle composition is E4 even without L/R/Aft (not a leaf).

Application path abbreviations:

| Prefix | Directory |
|---|---|
| IA | `modules/identityaccess/internal/application/` |
| PD | `modules/peopledirectory/internal/application/` |
| SM | `modules/schoolmembership/internal/application/` |
| WF | `modules/workforce/internal/application/` |
| SS | `modules/schoolstructure/internal/application/groups/` |
| TT | `modules/timetable/internal/application/` |
| SP | `modules/studentpresence/internal/application/presence/` |
| CP | `modules/careplan/internal/application/` |
| EN | `modules/enrollment/internal/application/` |
| OT | `modules/organizationtenancy/internal/application/` |

### Transaction evidence used by the rows

1. `tenant/transaction_runner.go:14` calls the body directly for an ambient
   transaction. `modules/transactionruntime/internal/adapters/postgres/tenant_runtime.go:70`
   checks the ambient tenant; `:116` checks admin scope. It does not silently
   nest a tenant transaction in an admin one. The runtime installs the RLS
   role and tenant configuration only for a new outer tenant unit.
2. People writes bind that runner in `modules/peopledirectory/compose/new.go:174`;
   School Membership's context database is bound at
   `modules/schoolmembership/compose/new.go:71`.
   `SM/service.go:205` wraps membership/employment writes in a savepoint.
   Care Plan's `internal/adapters/postgres/composition_engine_runtime_within_tenant_binding.go:10`
   uses the ambient context directly. Its offering commands wrap `RunInTx`
   (`CP/booking_commands.go:40`), bound to the tenant runner by composition.
3. Identity's `internal/adapters/postgres/composition_tenant_runtime_run_in_tx_binding.go:9`
   explicitly joins before selecting tenant/admin top-level scope. Workforce's
   `internal/adapters/postgres/composition_transaction_run_write_binding.go:9`
   does the same. Timetable owner stores use the ambient database; operations rely on
   the caller's request unit (`TT/operations_attendance.go:112`). Communication's `internal/adapters/parentpostgres/parent_record_database.go:28`
   reads that transaction for the announcement command.
4. Provisioning's outer units are `OT/provisioning.go:81` and `:85`, bound in
   `modules/organizationtenancy/compose/new.go:33`. Device and person root
   adapters preserve the context, including admin context when they add a
   target school ID. This is intentional operator scope, not tenant RLS.
5. At caller-dependent edges, “joins” is conditional on a supplied outer
   unit. In particular `EN/decide.go:46` is not itself a transaction wrapper:
   `modules/enrollment/http/admin_decision_handlers.go:583` supplies the unit
   and creates the pending invitation at `:596` **before commit**, not after
   commit as the old `Decide` comment said. Review and scheduler callers must
   retain their outer units. A green import check proves none of this.

## Edge classification

| Edge / operation | Root binding | Tx | L | R | Aft | Decision and source evidence |
|---|---|:--:|:--:|:--:|:--:|---|
| Enrollment decision → Care Plan materialization/adjustment/pickup | `root_enrollment_services.go:102` (`offerings.Bookings`) | J | Y | Y | N | **E1**. `EN/decision_bookings.go:46,80`; gates in `CP/booking_materialization.go:126`; approval status flip precedes union resync (`EN/decide.go:202`). |
| Enrollment decision → Timetable sourced templates, via Care Plan | `root_enrollment_services.go:102`, `CP/sourced_template_resync.go:14` | J | Y | Y | N | **E1**. `EN/approval_existing_student.go:56,102`; class gate → recurrence → student; account grant before instance FK/roster writes. |
| Enrollment decision → Identity guardian tenant access | `root_enrollment_services.go:102` (`identity`) | J | Y | Y | N | **E1**. `EN/guardian_access.go:141`; account-first rule at `EN/approval_existing_student.go:56`. The target owns account/role/mapping guards. |
| Enrollment decision → People student enrollment/profile | `root_enrollment_services.go:98` (`persons`) | J | Y | Y | N | **E1**. `EN/approval.go:165`, `EN/approval_existing_student.go:102`; People in turn invokes the membership/care commands below. |
| Enrollment approved-child sync → People/Care Plan | `root_enrollment_services.go:98,102,106` | J | Y | Y | Y | **E1**. `EN/approved_child_sync.go:104` takes offering/class gates before student mutations; `:320` registers the two plan invalidations together. |
| Care end/cancel → School Membership | `care_students.go:112` | J | Y | Y | N | **E2**. `CP/care_lifecycle.go` (`prepareLockedCareExitPreview`, `cancelCareExits`, `endCare`), `CP/care_exit_preview.go:72`; combined preview under locks, conditional affected-count check. |
| Care resume → School Membership | `care_students.go:116` | J | Y | Y | N | **W, moved here**. Former `CP/care_lifecycle.go:385,409`; new `workflows/careexit/resumption.go`, root `care_resumption.go`. Shared class gate → sorted student locks → guarded resume → Care Plan deletes → audit, one unit. |
| Identity school identity → People person/link/tag | `identity_account_lifecycle.go:172,178` | J/A | N | N | N | **Leaf**. `IA/school_identity.go:124,128`; `:285` explicitly distinguishes the pre-write tag check from a lock; target uniqueness guards collisions. No cohort/preview protocol. |
| Identity school identity → School Membership staff/teacher | `identity_account_lifecycle.go:238,312` | J/A | N | N | N | **Leaf**. `IA/school_identity.go:315,350`; commands own staff continuation and membership/employment savepoint. Person → staff → caregiver construction is not a new gate protocol. |
| Identity guardian profile create/link → People | `identity_account_lifecycle.go:453,463` | J/A | N | N | N | **Leaf**. `IA/guardian_relative_access.go:175`; guarded contact/profile association, same caller unit. |
| Identity guardian role upgrade → People relationship | `identity_account_lifecycle.go:483,490` | J | Y | Y | N | **E3**. `IA/guardian_relative_access.go:145` locks the foreign relationship and rechecks its role before promotion; the rest of the invitation still belongs to Identity. |
| Identity guardian link insert → People (and its pickup/access halves) | `identity_account_lifecycle.go:483` | J | N | N | N | **Leaf**. `IA/guardian_relative_access.go:296` uses `LinkStudentGuardianIfAbsent`; People owns insertion and its savepoint. Role upgrades are the separate row above. |
| Identity demo access → Organisation demo orders/retirement/last-use | `identity_account_authentication.go:109,113,117` | A | N | N | N | **Leaf**. `IA/demo_access.go:98,178,236`; order capacity/retirement are guarded within `modules/organizationtenancy/demo_school_access.go:65,80,94`, not a caller-defined cross-owner lock order. Demo mail enqueue stays in the caller's admin unit. |
| People student create/profile/renew → School Membership | `people_directory_composition.go:155,159` | J | Y | Y | N | **E3**. `PD/student_write.go:44,92,234`, `PD/student_enrollment.go:42,78,106`; class gate, locked student/companion cohort; composite savepoint at `student_write.go:33`. |
| People student create/profile/renew → Care Plan | `people_directory_composition.go:170` | J | Y | Y | N | **E3**, same student/companion protocol. `PD/student_write.go:67,180`, `PD/student_enrollment.go:71,95`; Care Plan owns its profile/plan write. |
| People relationship/pickup patch → Care Plan | `people_directory_composition.go:254,260` | J/A | N | N | N | **Leaf**. `PD/guardian_portal_commands.go:135,191,215`; People locks its own relationship, target applies only pickup fields, savepoint protects the split record. No foreign-cohort decision. |
| People relationship creation/permissions → Identity access | `people_directory_composition.go:283,289` | J/A | N | N | N | **Leaf**. `PD/guardian_portal_commands.go:185`; Identity owns the live-account check and relationship permission grant; membership alone never supplies child authority. |
| People staff edits → School Membership staff | `staff_membership_adapters.go:154,178` | J | Y | Y | N | **E3**. `PD/staff_directory.go:74,114,265`; account-state check before person/staff changes protects against concurrent offboarding; teacher creation is deliberately best effort. |
| People teacher edits → School Membership caregiver | `staff_membership_teacher.go:57,81` | J | Y | Y | N | **E3**, same staff command. `PD/staff_directory.go:234,316`; failed caregiver creation does not roll back successful staff creation. Do not turn it into all-or-nothing behavior. |
| School Membership staff → Workforce employment | `school_membership.go:52` | J/A | N | N | N | **Leaf**. `SM/service.go:145,163,205,214`; one logical staff record, owner-local guards plus shared savepoint, no preview/gate protocol. |
| Workforce staff edits → School Membership employment fields | `staff_admin_composition.go:136,141` | J | Y | Y | N | **E3**. `WF/staffadmin.go:66,272`; lock foreign staff, derive before/after, audit before applying. Source uses a consumer port, not a reverse import. |
| Workforce staff name/contact edits → People | `staff_admin_composition.go:127` | J | Y | Y | N | **E3**. `WF/staffadmin.go:124,272`; same locked subject/audit protocol. This extra target is visible in the cited binding, although the comment grouped the row as Membership ↔ Workforce. |
| School Structure class/group assignments → School Membership | `teaching_assignment_adapters.go:321,326,331,350,355` | J | N | N | N | **Leaf**. `SS/education_service.go`, assignment commands: owner validates each assignment; source has no independent foreign-cohort protocol for these calls. |
| School Structure group handover → Workforce | `api/staff_projection_education.go:43,61` | J | Y | Y | N | **W, already present**, not a direct Workforce write. `SS/substitution_module.go:319,425` locks group/target, rechecks authority; binding invokes `shiftplansync.GroupHandovers.CreateHandover/DeleteHandover`. School Structure keeps its own group authorization. |
| Timetable check-in/out/move → Student Presence | `timetable_operations_composition.go:73,77,81` | J | N | N | N | **Transaction-participating; Timetable authority guarded under A (#4736)**. `TT/operations_attendance.go:118,152,241`; target owns visit uniqueness/move/checkout locks, source updates its participant attendance. [The bypass audit below](#move-resource-authorization-audit-4736) proves that Timetable assignment access is broader than Presence push/pull access. Under decision A, Timetable assignment/attendance access governs this edge and the outsider denial test proves its boundary. The long-term A/B product question remains open; B would be a separate behavior change. This is not a demonstrated unauthorized write or a new ADR exception. A conflicting insert uses a savepoint and reloads the active visit, not a combined preview fingerprint. |
| Timetable cancellation → Communication announcement | `timetable_lifecycle_composition.go:75` | J | N | N | N | **Leaf**. `TT/instance_guardian_notice.go:50,118`; validate before cancel, `MarkRollback` on publication failure. Publication/outbox are target commands, not a shared after-commit callback. |
| Student Presence check-in status clear → Care Plan live flags | `student_records.go:63` | J | N | N | N | **Leaf**. `SP/visit_helpers.go:207,254`; target receives the live-flag patch; zero affected rows aborts the visit in `student_records.go:77`. Presence status-day writes remain with Presence. |
| Student Presence NFC start → Workforce check-in | `presence_work_sessions.go:19` | J | N | N | N | **Leaf, best effort**. `SP/session_start.go:264,277,413`; savepoint isolates automatic staff check-in failure. Workforce owns its time-clock lock; no guarantee that every supervisor gets a work session. |
| Care Plan offering roster/source change → Timetable | `care_plan_booking_materialization.go:119,135,139,165,219` | J | Y | Y | N | **E4**. `CP/sourced_roster_resync.go:62`, `CP/offering_adjustment_rows.go:156`; class/recurrence/student lock protocol in `CP/booking_materialization.go:126`, `CP/offering_adjustment.go:100`. |
| Alleged Care Plan booking materialization → People class/status writes | `care_plan_booking_materialization.go:401,431,437` | – | N | N | N | **No write at this binding**. `bookingStudents.Student`, `LockClassWrites`, `LockStudents` only read/lock. Lines 119–219 write Timetable, not People. Actual profile/master-data writes are the next row. |
| Care Plan master-data approval → People student/person | `master_data_decisions.go:192,221` | J | Y | Y | N | **E4**. `CP/master_data_decisions.go:128,151,188`, `CP/master_data_apply.go:53,76,97`; pending request → locked foreign child → current authorization → apply → request ledger. |
| Care Plan HTTP child create → People person/student/guardians | `api/serve_mounts.go:567,568` | J | N | N | N | **E4, HTTP coordinator**. `modules/careplan/http/student_create_handlers.go:198,212,223,292,320`; explicit outer unit includes attachments/schedules; validates guardians first and unwinds on business errors. Not a public leaf command in a domain. |
| Enrollment intake → Care Plan booking intervals | `enrollment_care_bookings.go:16` | J | N | N | N | **Leaf**. `EN/intake_offerings.go:207`; source selection and target booking interval inserts join the submission unit. Capacity guard is owner-provided, no new caller lock order. |
| Enrollment rollover → Care Plan booking intervals | same binding | J | N | N | N | **Leaf edge**. `EN/rollover_requests.go:216`; recording the new child's intervals adds no gate protocol. Approval of that rollover child is E1, not exempted by this row. |
| Enrollment form-change apply → Care Plan replace bookings | `enrollment_care_bookings.go:20` | J | N | N | N | **Leaf edge**. `EN/change_request_apply.go:286`; target replaces bounded intervals in the applying transaction. Approved-child sync/adjustment protocols remain E1/E4. |
| Organisation school creation → Device Fleet | `operator_provisioning_adapters.go:128` | A | N | N | N | **Leaf edge**. `OT/provisioning_schools.go:36,89`; create the manual device inside school creation's admin unit; target guards device identity/key uniqueness. |
| Organisation device transfer → Device Fleet | `operator_provisioning_adapters.go:117,139,152` | A | Y | Y | N | **E5**. `OT/provisioning_devices.go:319,442,478`; locked source → destination/supervision checks → archive → create target → link history. No independent Device Fleet commit. |
| Organisation person retirement → People | `operator_provisioning_adapters.go:324,328,332` | A | N | N | N | **E5**, composite retirement rather than a single leaf edge. `OT/provisioning_people.go:87,149,166`; supervision/account deactivation failures are intentionally tolerated, other writes propagate. |
| Organisation provisioning/retirement → Identity | `operator_provisioning_composition.go:148,171,193` | Split / A | Y | Y | Y | **E5**. `OT/provisioning_accounts.go:62,85,98`; resolve school/role in admin unit, create account/identity in tenant unit, then separate best-effort admin audit. Retirement stays in its ambient admin unit. These are not one shared transaction. |
| Organisation school creation → Timetable categories | `operator_provisioning_adapters.go:381` | A | N | N | N | **Leaf**. `OT/provisioning_schools.go:52`; seeding target categories joins school creation; target owns uniqueness, no new cross-owner locks. |

## Move resource authorization audit (#4736)

Audited against fetched `origin/development`
`8aca1925f79c1e723e35f14e65398cc82877fd7f` on 2026-10-23.
`rg -n 'BypassResourceChecks' backend --glob '*.go'` finds three production
constructions of `true`. The HTTP open-room adapter and
`roots/openroommovebinding/presence.go` only forward the checked actor;
they grant no additional bypass. The test-only `activeSvcBypassAuth` in
`SP/transit_service_test.go` arranges trusted moves; it is not a runtime entry.

### Presence rule without the bypass

The HTTP visit move requires `visits:update`, an authenticated tenant principal,
a tenant transaction and enabled web attendance. Presence then re-checks the
locked child/session state in `SP/student_move.go` and
`SP/student_move_access.go`:

- **Pull:** active supervision of the target session permits taking a child
  from any source session in the school. Source supervision is not required
  (#2329).
- **Push:** active supervision of every present child's current source session
  permits the move only when the target is the sole running session in its
  room and has an active supervisor (#2969). Membership of an education group
  or visibility of a room alone grants neither right.
- **School-wide extension:** the inbound adapter must mark a verified OGS
  staff actor with the move permission as eligible; Presence requires
  `operations.attendance_edit_scope=all_staff` together with
  `operations.operational_overview_scope=all_staff` and staff membership in
  the current tenant. This is not the admin bypass: locked-state validation
  and stale-visit handling remain active.
- **Released-room destination:** ordinary staff still need source rights;
  destination supervision and the single-session rule do not apply to an
  independent room stay (ADR 0018). The ordinary pull rule still applies if
  the actor already supervises that target session.

Existing real-DB coverage: `TestActiveService_MoveStudentsToActiveGroupAuthorized_`
`AllowsUnsupervisedSource`, `AllowsSupervisedSourceIntoColleagueTarget`,
`RejectsWhenNeitherRoomSupervised`, `RejectsPushIntoUnsupervisedTarget`,
`RejectsPushIntoAmbiguousTargetRoom` in `SP/transit_service_test.go`;
`TestActiveService_SchoolWideAttendanceMove` in that file;
`TestOpenRoomMoveKeepsTheSourceSideRights` and
`TestOpenRoomMoveNeedsNoSupervisionAtTheDestination` in
`SP/open_room_session_test.go`.

### Bypass constructions and guard evidence

| Construction | Upstream guard and remaining owner checks | Executable evidence |
|---|---|---|
| `TT/operations_attendance.go`, `moveStudentFromOtherSession` | The `/api/timetable/operations/...` check-in routes require `schedules:read`; the school-portal check-in in `modules/timetable/http/school_supervisions.go` requires `supervision:own`. Both use a tenant transaction and enabled web attendance, not `visits:update`. `requireScopedAction(ScopedAttendance)` admits tenant admins, planned block staff, current supervisors, or eligible school-wide attendance editors. `requireRosterStudent` additionally limits assignment-bound school-portal actors to today's assigned block and its non-excluded roster; it does not limit OGS walk-ins. The target instance must be active with a session. Neither check requires source supervision; planned assignment need not imply target supervision. Presence still checks tenant-bound child/session state, graduated/ended-care children, open attendance, capacity and visit uniqueness, but skips its resource authorization and restricted-path stale filtering/target-room gate. **Not equivalent to the Presence rule.** | New real-owner/real-DB `TestTimetableCheckInPlannedOperatorWithoutMoveSupervision` in `modules/timetable/compose/httpintegration/operations_move_authorization_integration_test.go` covers both `CheckInStudent` and `CheckInStudents`: identical actor/child denied by ordinary Presence, allowed by Timetable assignment, unassigned actor refused without ending the source visit; an assigned operator in another school cannot move the child into their own block (single and bulk). Existing `TestTimetableSchoolWideAttendancePreservesAccessBoundaries` covers scope clamps; real-DB `TestSchoolSupervisionStudentBoundary` in `api/school_portal_school_supervisions_test.go` covers the school-portal roster/excluded-child clamp. |
| `modules/studentpresence/inbound/presence/move_authorization.go`, `BulkStudentMoveAuthorization` | `common.HasEffectiveAdminScope` reads a validated principal, not raw JWT-context claims. The route still authenticates/scopes the caller and requires the visit permission. An admin wildcard/signed admin scope is school-wide; Presence keeps non-resource guards. The same authorization value serves move-to-group, transit assignment and the HTTP released-room workflow. | `TestBulkMoveBypassRequiresValidatedAdminPrincipal` and `TestMoveHandlerOnlyPassesSchoolWideEligibilityForTenantStaff` in `inbound/presence/move_handlers_test.go`; new production-router `TestFullProductionRouterGolden/contracts/move_bypass_authorization` in `api/move_bypass_contract_test.go` proves ordinary `visits:update` without supervision is 403 with unchanged visit, anonymous is 401, validated admin is 200 with the move persisted; a foreign-school admin targeting their own session is refused without moving the child. |
| `workflows/openroommove/openroommove.go`, `KioskActor` | Device API key plus the school's shared PIN, device-derived tenant, and resolution of the scanned card to a child in that tenant precede `api/device_open_room.go` constructing the actor. No person is authenticated; caller-supplied `X-Staff-ID` is ignored. School-wide resource access is intentional, not delegated staff supervision. The workflow requires and re-checks the destination release; Presence still requires open attendance and enforces capacity/visit integrity. The child needs no device or supervisor at the destination. | Production-router `TestFullProductionRouterGolden/contracts/open_room_kiosk_booking` in `api/open_room_kiosk_contract_test.go`: successful unsupervised move, idempotence, unreleased/foreign/unknown destination, absent child, wrong PIN, missing key, and a valid foreign-school device scanning the original child's card (404 `iot.rfid_tag_not_found`, original stay unchanged). `TestMoveToOpenRoomStopsAtTheTenantBoundary` and `TestMoveToOpenRoomRejectsAnUnreleasedRoomWithoutWriting` in workflow compose tests preserve owner guards. |
| Nil authorization: `SP/transit_service.go`, `AssignTransitStudentsToActiveGroup` | This internal “trusted, pre-authorized” entry passes `auth=nil`; `authorizeTransitTarget` skips resource authorization for nil as well as explicit bypass, analogous to `restricted := auth != nil && !auth.BypassResourceChecks` in `SP/student_move.go`. No production caller exists today; this is not a fourth `BypassResourceChecks: true` construction or a tested upstream runtime guard. Any new runtime caller must supply a named, tested authorization guard or use the authorized entry instead. | Existing `TestActiveService_AssignTransitStudentsToActiveGroup` and transit capacity/rollback tests exercise the trusted entry, not delegated runtime authorization. A search for `AssignTransitStudentsToActiveGroup(` finds only the internal declaration/interface and test calls. |

The kiosk trust model is documented in `backend/CLAUDE.md` (RFID/IoT),
`docs/agents/contracts.md` (Ecosystem and IoT), and the `Actor`/`KioskActor`
contract itself (#3067). ADR 0018 records the independent released-room stay;
it does not turn the school PIN into a personal staff identity. These tests
preserve IoT statuses/codes; they do not add a new device permission.

### Open business decision: planned Timetable operators

Scenario: child is still in running session A. The caller is planned for
running block B but supervises neither A nor B. The attendance scope is `own`.
Timetable currently permits check-in to B; ordinary Presence refuses it.
[#2386](https://github.com/moto-nrw/project-phoenix/issues/2386) requires the
automatic move when checking into another block, but does not decide whether
planned assignment suffices for this case. ADR 0083 explicitly leaves that
permission contract undecided. CONTEXT.md supplies no stricter move rule.

- **A, current decision for #4736:** Timetable assignment/attendance access is
  the governing permission for this edge. A planned operator may check the
  child into B without active supervision of A or B. The named Timetable
  guards and the outsider refusal test establish this contract; they do not
  establish equivalence to ordinary Presence authorization or create a new
  ADR 0067 exception.
- **B, not implemented:** require Presence's push/pull (or eligible school-wide)
  rights too. This scenario becomes 403; a target supervisor may still pull
  from a foreign source under the existing rule. Preserve explicit admin and
  school-wide eligibility when making that change rather than simply deleting
  the bypass flag for every actor.

The Product Owner could not settle the long-term A/B product question. The
binding decision for this round is A (status quo), under which #4736 can close:
all three explicit bypass constructions have named, tested upstream guards.
The second acceptance criterion is proved by the outsider case in
`operations_move_authorization_integration_test.go`: an actor with neither
assignment nor supervision is denied in single and bulk check-in, and the
source visit remains open. A planned-only actor is authorized under A, so its
successful check-in is not an unmet denial condition. The long-term A/B
question remains open as a product question, not an incomplete criterion;
adopting B would be a separate behavior change. No production rule, status,
code, transaction or lock order changes in this round.

## Retained protocols and lock order

These are current implementation facts, **not a claim that all global lock
orders are proved deadlock-free**. Row/unique-index locks inside leaf commands
still exist. A blanket exception based only on “same transaction” would miss
these protocols and Provisioning's explicit split units.

| Exception | Current order and invariants | Existing executable coverage |
|---|---|---|
| E1 | `EN/decide.go:116`: parent request → sibling children in stable sort order, then inspect terminal/idempotent status. Existing-student tail `EN/approval_existing_student.go:40,102`: shared class gate → recurrence → student state; `:56` ensures account attachment precedes roster/instance FK locks. `CP/booking_materialization.go:126` applies class gate → recurrence before booking-derived changes. Decision mail and handler-created invitation enqueue are in the outer unit, not losable after-commit work. | `modules/enrollment/http/decision_approval_rollback_test.go`: `TestDecisionService_ApprovalRollsBackEveryOwnerAfterMaterialization`, `TestExistingStudentApprovalGrantsAccountBeforeClassRosterResync`; sibling digest serialization in `decision_service_test.go`. |
| E2 | `CP/care_lifecycle.go`, `lockCareExitWrites`: shared class gate → booking-write/recurrence gate → locked child read → open requests → planned roster → activity bookings → own source bookings → weekly plans → quoted offering/person/presence rows. Confirm (including withdrawal completion), cancel, booking-expiry reconciliation and due-effect pages use the same gate helper (#4636). Rebuild the confirmation token before mutations; mismatch/blocked preview rejects all. Cancel restores the ledger before rewriting last care day. Due effects remain in this exception, not resumption. | `api/care_exit_lock_order_test.go`: real two-connection interleavings against grade-transition Apply for confirm, cancel and due effects with and without an ambient unit. Existing lifecycle, storage-cutover and booking-authority contract suites retain the preview/rollback and ledger coverage. |
| E3 | Identity upgrade: foreign relationship `FOR UPDATE` → role recheck → promote (`IA/guardian_relative_access.go:145`). People student edits: class gate → subject/companion locks, sorted far ends → compare plan → profile/membership/care writes inside savepoint (`PD/student_write.go:33,234,251,307`; enrollment patch `student_enrollment.go:42`). Staff identity: account-state guard before writes (`PD/staff_directory.go:114`); caregiver creation failure stays non-fatal. Workforce: lock foreign staff → derive diff → Audit → apply person/employment command (`WF/staffadmin.go:66,272`). | `modules/peopledirectory/compose/guardian_lifecycle_commands_test.go`; `guardian_link_owners_test.go` (`TestGuardianPortalLinkRollsBackAfterEveryOwnerCommand`); People student/staff behavioral suites; `modules/schoolmembership/compose/student_enrollment_test.go` and staff-employment savepoint tests. These protect named scenarios, not every interleaving. |
| E4 | Offering adjustment: `CP/offering_adjustment.go`, `loadOfferingAdjustmentSubject`, reads request/child, takes class(shared) → recurrence then locks the student before deriving new intervals/roster (#4638); template resync requires its caller's recurrence lock (`sourced_roster_resync.go:60`). Master-data: request lock → child lock/current authorization → apply → decision/history (`master_data_decisions.go:128,151,188`). HTTP child create opens its unit at `student_create_handlers.go:292`; person → student/owner halves → guardians → attachments. Business-error rollback and cleanup order stay unchanged. | Care Plan contract suites for template roster maintenance, offering changes, master-data review and HTTP student creation. `TestFullProductionRouterGolden` pins route/middleware/error shape, not transactional semantics. |
| E5 | School creation: organization mutation guard → school → categories/manual device in admin unit (`OT/provisioning_schools.go:36`). Device transfer order above. Account creation **does not** re-read school/role under a lock in its later tenant unit (`provisioning_accounts.go:67,85`); no atomic admin-to-tenant guarantee is claimed. Person retirement checks supervision, deactivates/anonymizes/unlinks account, then anonymizes person; some failures are logged by design (`provisioning_people.go:87,149,166`). | `modules/organizationtenancy/internal/application/provisioning_*_internal_test.go`, notably `TestProvisioningCreateSchoolAccount`, `TestProvisioningTransferDevice`, `TestProvisioningSoftDeletePerson`. Double-based tests do not establish cross-owner database atomicity. |

### Scope and follow-up proposals

Only **one capability** moves in this PR: resumption, into the existing
`care-exit` owner. Its entire coordinator moves, not a forwarding facade.
Other E rows are accepted retained cuts under ADR 0067, not work claimed done.
No issues are created by these proposals. Estimates are planning ranges of
changed production lines (additions plus deletions), **not measured diffs**:

1. E1 approval + existing-child/sync: roughly 1,800–3,000 production lines plus
   tests/root bindings. Even five directly shared source files already total
   1,623 physical lines (`decide.go`, `approval.go`, `approval_existing_student.go`,
   `guardian_access.go`, `approved_child_sync.go`), before contracts/roots. A
   file move alone is not the solution: Enrollment must expose guarded steps,
   not export its internal `Decisions` graph or ORM-shaped records.
2. E2 remaining confirm/cancel/due/withdrawal protocol: roughly 2,000–3,500
   changed production lines plus tests. The five inspected lifecycle/cleanup/
   preview/due/withdrawal files contain 1,689 physical lines after the resume
   extraction. Keep the #4531 booking boundary and ledger compatibility.
3. E3 independent slices: guardian upgrade 200–500, student/companion write
   800–1,500, staff identity/admin 600–1,200 lines each plus tests. Preserve
   savepoint recovery and teacher best effort. Do not merge owners to avoid
   the workflow (ADR 0081).
4. E4 independent slices: offering reconciliation 1,200–2,000, master-data
   approval 500–900, HTTP child creation 600–1,000 plus tests. They depend on
   the E1/E2 cut decisions; avoid extracting a second recurrence engine.
5. E5 split by capability, not the whole provisioning interface: device
   transfer 500–900; account/identity 600–1,200; person retirement 400–800
   plus tests. First pin scope boundaries and tolerated failures. Whether a
   concurrent school deactivation must invalidate account creation is a
   business question, not authorization here to change that behavior.

The combined changes exceed the task's roughly 1,500-line/two-workflow bound.
This decision chooses one complete small workflow over partial relocation of
several large coordinators. None of these estimates authorizes a policy or
ratchet-budget increase.

### Lock-order audit and correction (#4636)

At `a8df171694`, a real two-connection test reproduces PostgreSQL `40P01`
for care confirmation, cancellation and due-effect pages against grade-transition
Apply. The serving root supplies the recurrence lock as `LockCareBookingWrites`;
the later locked People read takes School Membership's shared class gate.
The correction acquires that class gate explicitly before recurrence, through a
consumer-owned port bound to School Membership. No policy rule or owner changes.

| Entry point / shared helper | Gate order and following locks |
|---|---|
| Care confirm / withdrawal confirm, cancel, due-effect page (`CP/care_lifecycle.go`, `CP/care_due_effects.go`) | **Before:** recurrence → class(shared) → sorted joined child rows. **After:** class(shared) → recurrence → those same rows. Confirmation then locks open requests, roster, activity bookings, source bookings, weekly plans, offering/person/presence rows; withdrawal completion is re-read `FOR UPDATE` after the binding preview. Cancel reads children/exits before restoring the ledger. Due effects take the same gates during the initial expiry reconciliation, before effect pages revalidate children and clean up presence/requests/tags/plans. This also covers the scheduler’s ambient transaction, which retains the expiry locks. No transition gate is taken. |
| Grade transition Apply and Revert (`workflows/gradetransition/apply.go:63`, `revert.go`) | Unchanged: class(exclusive) → recurrence → transition → transition row / sorted cohort rows. The early gates precede the cohort and fingerprint re-read. |
| Timetable materializer (`TT/materialization.go:146`) | Unchanged: recurrence → transition → materialization rows. **No class gate** is acquired. Its order is the matching suffix of ADR 0067 point 5, not a second class-gate implementation. Replan takes recurrence before ascending day locks and planned-row deletion, then enters this suffix. Template writes/splits, instance lifecycle/conversion and pickup-extension resolution also start with recurrence before recurrence-derived rows; none adds a class gate in its own gate helper. |
| Student-deletion withdrawal completion (`workflows/studentdeletion/deletion.go:335`) | Unchanged: class(shared) → recurrence → withdrawal/student rows. Ordinary deletion has no recurrence gate. |
| Existing-child approval and due class switch (`EN/approval_existing_student.go:102`, `class_switch.go:60`) | Unchanged: class(shared) → recurrence → student state. Enrollment approved-child sync and form-change apply use `CP/booking_materialization.go:126` before their child writes. Fresh child creation already holds the shared class gate before later booking materialization. |
| Offering-derived gate / direct pickup editor (`CP/booking_materialization.go:126`, `direct_offering_adjustments.go:30`) | Unchanged: class(shared) → recurrence → child/booking rows. Later locked reads reacquire class safely. |
| Offering adjustment, including admin correction (`CP/offering_adjustment.go`, `loadOfferingAdjustmentSubject`; `EN/decision_bookings.go`, `UpdateChildOfferings`) | **Before #4638:** recurrence → class(shared) → child. **After:** the booking command itself takes class(shared) → recurrence → child via `LockOfferingDerivedWrites`. Form-change apply and the direct pickup editor retain their earlier acquisition of the same gates. |
| Offering-change review and pickup reset (`CP/offering_change_decisions.go`, `lockPendingRow`; `offering_pickup_times.go`, `ResetStudentPickupDayToOffering`) | **Before #4638:** review request → class(shared)/child → recurrence; reset class(shared)/child → recurrence. **After:** class(shared) → recurrence → child → offering-change request (review), or pickup rows (reset). Review includes approve, reject, MarkDone and offering conflict resolution. No transition gate. |
| Booking-authority setting (`CP/care_participation.go`), offering catalog, enrollment phase updates/deletes | Recurrence only at these boundaries; the booking-fact projection does not acquire class or child row locks. Sourced-roster resync inherits the caller's recurrence gate. |
| School Membership class writes, People directory locked reads and class-gate CTE | Class only, before child rows; no recurrence lock in these implementations. Care resumption remains class(shared) → child, with no recurrence or old-plan restore. |

The test holds class exclusively on connection 1, starts the care operation on
connection 2 and waits for `pg_locks` plus `pg_blocking_pids` to confirm the class
wait. It then runs the real grade-transition Apply in connection 1. Before the
fix this closes the cycle and reports `40P01`; afterwards both operations commit.
Due effects also run inside an ambient transaction, matching the scheduler and
proving that the preceding expiry pass does not acquire recurrence before class.
The transition promotes a different child in the **same school**, so confirmation
has no unrelated stale-token failure. The test uses an isolated database, real
fixture IDs and bounded waits, not sleeps to arrange the race.

This proves the named interleavings, not global deadlock freedom. The two static
findings recorded by #4636 are corrected below in #4638. Neither correction weakens the E2 preview, rollback, tenant scope, ledger or error contracts.

### Offering lock-order correction and row audit (#4638)

`CP/contracttest/offering_lock_order_test.go` exercises the public Enrollment
correction and Care Plan review/reset commands on isolated PostgreSQL databases.
The correction/grade test holds class(exclusive), observes the correction waiting
through `pg_locks`/`pg_blocking_pids`, then runs real grade-transition Apply.
The care tests hold class(shared) → recurrence before starting approve/reset;
a second interleaving also holds the child before approve, reject, MarkDone or
offering conflict resolution. Only after PostgreSQL reports the writer blocked
does the first connection run real care confirmation. No sleeps arrange a race.
All seven cases reproduce `40P01` before their fixes and pass afterwards.
A further two-connection test edits the request while review waits on the gate:
the locked re-read must reject the old version and leave the request pending.

The care exit is scheduled after the offering switch, so both commands must
succeed, not merely avoid `40P01`. Request terminal state and expected version
are checked again under `FOR UPDATE`; missing, terminal and stale requests may
be rejected before the gates. The preliminary read supplies the immutable student
ID, not authority to mutate. Authorization and the mutation still use locked state.
Tenant transactions and preview fingerprints are unchanged. #4639 deliberately
changed two responses from HTTP 500 to HTTP 404 with the existing `general.input`
code: previewing a missing offering-change request and resolving an offering
conflict with a missing offering request ID. Other error/status contracts stay
unchanged.
The existing consumer-owned gates suffice; no new root logic or policy rule.

Round-2 error-path regression: the unlocked `Rows.Find` must return
`ErrOfferingChangeNotFound`, just like `FindForUpdate`, for missing and
tenant-hidden IDs. Review can also lose the child (and its cascading request)
while waiting on the gates; its locked child read maps a missing row to the
same sentinel. Other database failures retain their wrappers. Service and real
HTTP regressions cover approve/reject/MarkDone (404, existing `general.input`);
service tests also cover offering conflict pre-lock/decision and preview. Four
two-connection deletion cases pin the gate-wait race. Missing request/child
adjustments and missing/foreign-child pickup reset retain their existing errors.

All unlocked `Rows.Find` callers were audited: `pendingRow` serves conflict
candidates and the new review pre-read; `previewableRow` serves decision preview.
Both now expose the domain not-found outcome. `Edit` reloads after its guarded
update and `recordOfferingDecision` reloads after deciding; both already hold
the request lock in the same tenant transaction, so concurrent deletion cannot
remove that row before either reload. Those reloads still wrap other failures.

Additional request/child audit at #4638 (historical findings; corrected below by #4640):

| Flow that takes both request and child rows | Current order / disposition |
|---|---|
| Care confirm / effective-day cleanup (`CP/care_exit_preview.go`, `care_exit_cleanup.go`) | Gates → sorted child rows → open family requests (care-schedule, excused, master-data and offering changes). Confirmation locks requests even for a future exit; closing pending requests is an effective-day effect. |
| Offering approve/reject/MarkDone and conflict resolution | Gates → child → offering request, including the pre-lock used by `ParentRequestCoordinator.prepareConflictGroup`. Repeated downstream gate/child acquisition is transaction-reentrant. |
| Offering guardian edit / course withdrawal (`CP/offering_change_submissions.go`, `Edit`; `course_requests.go`, `WithdrawCourseRequest`) | Request only; these paths take no child row lock or class/recurrence gate. Their catalog reads do not call `assertCareReaches`. No row inversion was introduced by moving review locks. |
| Enrollment approval / approved-child sync / form-change apply (`EN/decide.go`, `approved_child_sync.go`, `change_request_apply.go`) | Enrollment request/children first, then class → recurrence → directory child. These are **Enrollment** request rows, not the family-request rows locked by care end. Existing account-before-roster protocol remains unchanged. |
| Care-schedule review (`CP/request_decisions.go:65,76`) and schedule conflicts (`request_conflicts.go:31`) | **Remaining static finding:** request → child. Against care end's child → open requests, concurrent review can form a row cycle. Not changed by this bounded offering correction. |
| Excused review/MarkDone and excused conflicts (`CP/excused_request_decisions.go`, `lockDecidableRequest`, `MarkDone`) | **Remaining static finding:** pending request → child, followed by child/day plan locks. Same care-end row-cycle risk. |
| Excused correction (`CP/excused_request_decisions.go`, `lockCorrectableRequest`) | Decided request → child → child/day plan locks. Care end locks only pending requests (`CP/internal/adapters/postgres/requests.go`, `LockOpenCareRequests`), so this order alone does **not** establish a direct same-request cycle with care end. |
| Master-data review (`CP/master_data_decisions.go:113,152`), bulk (`parent_request_coordinator.go:105,109`) and non-offering conflicts (`parent_request_resolution.go:91,99`) | **Remaining static finding:** request(s) → child(ren). Bulk takes master-data/excused requests in kind/id order, then sorted children. Changing this shared protocol needs a separate regression and coordinated fix across those kinds. |
| Withdrawal deletion (`workflows/studentdeletion/deletion.go`, `ExecuteWithdrawal`) | class(shared) → recurrence → withdrawal-completion row → child, then family-request cleanup. Care withdrawal confirmation uses the same recurrence prefix, so its later completion re-read cannot invert that pair. Ordinary deletion and resumption do not take recurrence. |

The earlier gate table also audits transition creation/update (transition only),
Apply/Revert (class → recurrence → transition → rows), timetable materialization
(recurrence → transition → rows), class writes/locked directory reads and
resumption (class → child), and offering catalog/phase/participation (recurrence
only). The remaining request-kind findings above predate #4638; #4640 corrects them
below without a new exception to ADR 0067 point 5.

### Family-request row-order correction (#4640)

Audited `origin/development` `c40cab16f54590b58a573ca7e1fad4bc06e9314e`
with recursive `grep -RnE` and `rg -n` for `Records.Find`, `Find*Request`, `FindByIDForUpdate`,
`LockStudent`, `Lock*Request`, `LockOfferingDerivedWrites`, `Withdraw` and
`Edit` in Care Plan, Parent Portal, Enrollment, care-exit/student-deletion and
their root bindings. Paths below are relative to `backend/`; CP abbreviates
`modules/careplan/internal/application/`, PP abbreviates
`workflows/parentportal/internal/care/`. This table includes the direct owner
commands and their portal/coordinator callers, not just HTTP decisions.

| Flow / code entry point | Before #4640 | After #4640 |
|---|---|---|
| Care-schedule and pickup approve/reject/MarkDone (`CP/request_decisions.go`, `loadAuthorized`; `request_completion.go`) | Request → class(shared)/child | Unlocked request locator → class(shared) → child → locked request; pending/version/scope checks before mutation |
| Care/pickup correction, including undo (`CP/request_corrections.go`, `loadCorrection`) | Decided request → class/child → pickup/departure exception rows | Class → child → decided request → same exception rows; correction/version/scope guards unchanged. Care end locks pending rows only, so a decided correction alone was not a direct same-request cycle with care end |
| Care/pickup conflict pre-lock and winner/reject/staff-value paths (`CP/request_conflicts.go`; `parent_request_resolution.go`) | Sorted requests → child, then optional child/day plan locks | Each pre-lock locates the child unlocked, then class → child → sorted request IDs. Decisions and optional child/day writes reacquire that same child |
| Excused approve/reject/MarkDone (`CP/excused_request_decisions.go`) | Pending request → class/child → optional sorted day locks/status rows | Class → child → request → same day/status suffix; status/version/reviewer checks use locked rows |
| Excused correction/undo (`CP/excused_request_decisions.go`, `lockCorrectableRequest`) | Decided request → class/child → day/status rows | Class → child → decided request → same suffix; care end did not directly lock this decided request either |
| Direct excused guardian edit (`CP/excused_requests.go`, `EditRequest`) | Request → class/child/day via `ensureNoPartialAbsence` | Unlocked locator → class → child → request → sorted day locks; ownership still precedes pending/version checks |
| Excused bulk/conflict pre-lock (`CP/excused_request_coordination.go`) | Request first; decision later locks child | Class → child → request, even when invoked directly. Coordinator keeps the multi-child frontier below |
| Master-data approve/reject, bulk/conflict pre-lock (`CP/master_data_decisions.go`, `lockPendingRequest`; `master_data_conflicts.go`) | Request → class/child → person or guarded child update | Class → child → request → same person/update suffix; locked pending/version and authorization checks retained |
| Master-data correction/undo (`CP/master_data_corrections.go`) | Decided request → class/child → live field check/write | Class → child → decided request → same guarded undo/apply |
| Parent master-data edit (`PP/parent_master_data_request_service.go`, `editMasterDataRequestInTx`) | Request → class/child → current person read | Unlocked request/ownership → class → child → locked request/ownership/status/version → same field validation. This was another confirmed inversion, fixed in #4640 |
| Mixed master-data/excused bulk approval (`CP/parent_request_coordinator.go`, `BulkApprove`) | Requests in kind/ID order → children ascending | Children ascending → requests in unchanged kind/ID order → apply in child/kind/ID order. Pre-lock helpers may safely reacquire children already held |
| Cross-kind conflict coordinator (`CP/parent_request_resolution.go`, `prepareConflictGroup`) | Non-offering requests ascending → child; offering port already gates/locks child before its request | Candidates/version check remain unlocked; prove one child, then each port takes its prefix before ascending request IDs. Later `LockBulkStudents` is reentrant; no child is moved ahead of offering recurrence |
| Offering approve/reject/MarkDone/conflict and offering adjustment/reset (`CP/offering_change_decisions.go`, `offering_adjustment.go`, `offering_pickup_times.go`) | Class(shared) → recurrence → child → request/pickup rows after #4638 | Unchanged. These commands retain recurrence before child |
| Portal care/pickup/excused/offering edits and submissions (`PP/parent_care_schedule_service.go`, `pickup_change_requests.go`, `absence_requests.go`, `parent_care_offerings_service.go`); master-data submission (`parent_master_data_request_service.go`, `lockMasterDataSubject`) | Class/child before calling the request command; excused submission also locks sorted days before its duplicate-submission advisory key | Unchanged outer protocol. Direct care/pickup/offering edit only locks a request and reads plan/catalog facts; it acquires no child or recurrence lock. New inserts lock no previously existing request first |
| Course withdrawal (`CP/course_requests.go`, `WithdrawCourseRequest`; `PP/parent_courses_service.go`) | Request only; permission/catalog reads acquire no child or recurrence lock | Unchanged. The old generic care/master-data/absence withdrawal flows were replaced by edits in #2267; there is no additional active withdrawal command to reorder. Guardian pickup-exception removal locks child/day then its exception, not a family-request row |
| Care confirmation/withdrawal confirmation and effective-day cleanup (`CP/care_lifecycle.go`, `care_exit_preview.go`, `care_exit_cleanup.go`, `care_due_effects.go`) | Class(shared) → recurrence → ascending child cohort → open family requests | Unchanged. Family rows lock master-data, excused, schedule, then offering, ascending ID within each table. Future confirmation also locks pending requests; effective-day effects close them |
| Enrollment decision/sync/form-change (`modules/enrollment/internal/application/decide.go`, `approved_child_sync.go`, `change_request_apply.go`) | Enrollment request/sibling children → class → recurrence → directory child | Unchanged. These are different request tables, not the family-request rows care end locks; Enrollment's account-before-roster protocol is untouched |
| Student deletion / withdrawal completion (`workflows/studentdeletion/deletion.go`); care resumption (`workflows/careexit/resumption.go`) | Deletion locks child before request cleanup; withdrawal completion takes class → recurrence → completion row → child. Resumption takes class → child | Unchanged. Withdrawal/care confirmation share recurrence before completion/child, so those phases serialize; resumption takes no family-request lock or recurrence |

The request's student ID is immutable in every audited edit/decision command.
This makes the unlocked locator safe for choosing a lock, not for deciding a
status, version, payload or permission. Both request read modes already expose
the same not-found sentinel; no preview/error normalization like #4639 is needed.
Root bindings translate a missing child after the locator to the existing
request-not-found sentinel. They add no ordering, SQL or policy rule. The
parent edit workflow rechecks request existence on a failed child lookup to
preserve that same outcome without changing its generic student-reader port.
Excused edit checks ownership before locking the child, then repeats it under
the request lock: its portal caller can already hold the supplied child, so a
mismatched request must not cause a second, possibly descending child lock.

Bulk's request-kind order differs from care end's table order. That cannot form
a same-child cycle once **all** children precede **all** requests: overlapping
children serialize first, and disjoint children have disjoint immutable request
rows. Sorted child IDs, request IDs, day keys and companion locking remain in
place. Existing downward companion locks use NOWAIT and refuse rather than wait
on a lower child ID. No claim of global deadlock freedom follows from this audit.

Evidence:

- `request_lock_order_test.go`: all six approve/reject decision/care-end cases
  reproduce `40P01` before the fix. Approve and reject now both commit after a
  scheduled care exit. Child-held tests cover MarkDone, corrections, all three
  conflict pre-locks, direct excused edit, mixed bulk and grouped conflict paths.
  Request rows must remain lockable while a writer waits for its child.
- `TestBulkRequestLocksAllChildrenBeforeRequests` fails if only individual
  pre-locks are reordered but the old bulk coordinator remains: a higher child
  is already held while the lower child is awaited. The fixed frontier passes.
  `TestRequestDecisionRechecksStateAfterWaitingForChild` pins fresh status/version
  checks after the wait. A cascading deletion at the new bulk child frontier
  retains the old request-race outcome (409 `students.change_request_stale`),
  not an unclassified student error.
- `request_not_found_test.go` and `http/request_not_found_db_test.go`: missing
  and tenant-hidden IDs, request deletion during the row wait, and unchanged
  404 `general.input` / 409 status-and-code responses pass before and after the
  fix. Additional child-deletion waits cover the new preliminary-read race.
- `workflows/parentportal/parent_request_lock_order_test.go` fails before the
  parent master-data edit correction and passes afterwards, including request
  and cascading-child deletion. Existing ownership/permission tests stay intact.

### Checker proposal for #4548

Assess a typed, binding-aware **command-effect inventory**, not a rule based
on method names such as `Set`, `Create` or `Save`. Consumer ports can conceal
the owner; embedded methods and function fields also carry writes. Follow
root bindings to public commands, distinguish read/lock/write/outbox and
ambient/split/independent-unit effects, and report exact callsites with the
workflow or ADR decision. Pin representative positive and negative fixtures
from this table before making it a hard gate. It must scan roots and every
production package (#4542), retain DAG/private-role guards and need no broad
allowlist. Static analysis will still not prove lock safety, authorization or
after-commit durability. No such checker is implemented by #4538.

## Historical corrections and measurement limits

- [#4076](https://github.com/moto-nrw/project-phoenix/issues/4076) and comments
  describe an import-debt carrier. [PR #4338](https://github.com/moto-nrw/project-phoenix/pull/4338)
  (`b033bd74ff7f0c0c9d3377800818ecf49818f1ee`) removed its last four keys and
  closed it. That is not evidence of a complete care-exit workflow cut.
- [#4531](https://github.com/moto-nrw/project-phoenix/issues/4531) has no comments.
  [PR #4619](https://github.com/moto-nrw/project-phoenix/pull/4619)
  (`c206d2d91ef826e74b0cc96ea2b2761de46be1db`) implements ADR 0075 option A:
  own source-booking lock/end no longer make the Enrollment/root/workflow
  round trip. Snapshot/restore still live in `workflows/careexit/bookings.go`.
  Membership end/resume stayed in Care Plan until this PR moves resumption.
- The table classifies placement; the E2 gate inversion is reproduced in a local test,
  not a production incident; the additional audit findings remain static. Provisioning's split scope and tolerated failures predate this PR. No new
  all-or-nothing or school-liveness guarantee is inferred from them.
- Policy inspected: epoch 91, 927 rules, 68 owners, zero `legacy.jsonl` keys.
  `enrollment` and `care-plan` application packages remain their domain roles;
  `workflows/careexit` is `care-exit/public`; `roots/owners` is
  `root-composition/compose`. Resumption reuses the existing Care Plan public
  and shared-kernel reaches, with root-bound School Membership and Audit
  ports. No new import direction, private-role grant or writable object is
  necessary. The table is a semantic inventory, not a policy allowlist.
