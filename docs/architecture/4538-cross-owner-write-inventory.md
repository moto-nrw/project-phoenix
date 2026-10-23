# Cross-owner command inventory (#4538)

Decision: [ADR 0067 addendum](../adr/0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md#addendum-2026-10-23-command-participation-and-coordination-4538).
Inspected fetched `origin/development` at
`e033cf0f27d6ea8d16f24df81176da9ddf042d39` (2026-10-23), not the issue's
`ccf2209e96` or comment's `95507f6043`. Path:line references below describe
that immutable **pre-change** tree unless marked “new”. Paths start at
`backend/`; `roots/owners/` bindings preserve the passed context.

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
| Timetable check-in/out/move → Student Presence | `timetable_operations_composition.go:73,77,81` | J | N | N | N | **Leaf**. `TT/operations_attendance.go:118,152,241`; target owns visit uniqueness/move/checkout locks, source updates its participant attendance. A conflicting insert uses a savepoint and reloads the active visit, not a combined preview fingerprint. |
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
| E4 | Offering adjustment: `CP/offering_adjustment.go:100` reads request/child, takes recurrence protocol then locks the student before deriving new intervals/roster; template resync requires its caller's recurrence lock (`sourced_roster_resync.go:60`). Master-data: request lock → child lock/current authorization → apply → decision/history (`master_data_decisions.go:128,151,188`). HTTP child create opens its unit at `student_create_handlers.go:292`; person → student/owner halves → guardians → attachments. Business-error rollback and cleanup order stay unchanged. | Care Plan contract suites for template roster maintenance, offering changes, master-data review and HTTP student creation. `TestFullProductionRouterGolden` pins route/middleware/error shape, not transactional semantics. |
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
| Offering adjustment without the preceding gate (`CP/offering_adjustment.go:113,116`, `EN/decision_bookings.go:105`) | The public booking command itself takes recurrence → class(shared) through its locked student read. The admin `UpdateChildOfferings` path does not prepend `LockOfferingDerivedWrites`. **Separate static finding**, not fixed by #4636: it can close the same class/recurrence cycle against grade transition. Form-change apply and the direct pickup editor have the earlier gate above. |
| Offering-change review and pickup reset (`CP/offering_change_decisions.go:22,71`, `offering_pickup_times.go:59`) | Review locks the request, then class(shared)/child before entering the recurrence-taking adjustment. Reset takes class(shared)/child → recurrence. The two gates agree with grade transition, but the child-before-recurrence order can conflict with a care confirmation holding recurrence while waiting for that child. **Separate static row-lock finding**, unchanged here. |
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
findings above need separate regression tests and protocol changes; #4636 does
not weaken the E2 preview, rollback, tenant scope, ledger or error contracts.

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
