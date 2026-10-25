package test

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// queryBudget is one register entry: the number of SQL statements a scenario
// may issue. `max` entries are ceilings (less is fine, more fails); `exact`
// entries pin a dedup contract (one bulk load, one settings snapshot) where a
// count of zero would mean the code path silently stopped running.
type queryBudget struct {
	max   int
	exact bool
}

// queryBudgets is the query-budget register (#2940): scenario → statements.
//
// Shrink-only, like every other ratchet in this package:
//
//   - Never raise a number. A scenario that needs more statements is an N+1
//     regression until proven otherwise; the fix is a batch load keyed by an
//     ID set (modules/timetable/compose/exception_conflicts.go, the
//     FindByStudentIDsAndDate calls, is the reference shape).
//   - Lower a number when a fix removes statements, so the win cannot regress.
//   - Every new list endpoint gets an entry plus a test calling
//     AssertQueryBudget with it (`.claude/rules/backend-conventions.md`,
//     Rule 15). TestQueryBudgetRatchet fails on entries no test references
//     and on scenario names no entry defines.
//
// The counts are what the fixture-sized scenario in the referenced test
// issues; small fixtures are enough because N+1 shows up at N=3 already.
var queryBudgets = map[string]queryBudget{
	// modules/mealplan — GET /api/meal-plan/participants/week (#3164): the
	// tenant settings, then per school day one candidate read and the
	// participation reads of the Tagesliste. Flat in the number of children.
	"modules.mealplan.weekly_participants": {max: 24},
	// Operator-reviewed staff person repairs read all school people and
	// employments in two bulk queries, independent of the number of cases.
	"api.operator.staff_person_merge.preview": {max: 2, exact: true},
	// services — the operator device listing, organizationtenancy
	// Provisioning.ListAllDevices (#2676, #3253), inside the request's
	// administrative transaction. Three statements: school summaries,
	// organization summaries, and one device read through the Device Fleet
	// owner. Flat in the number of devices.
	"repositories.operator.device_rows": {max: 3},
	// modules/traegerview — GET /traeger/schools (ADR 0084, #4649): the Träger
	// row, the school IDs of the Träger and the school rows of that set, in
	// one administrative transaction. Flat in the number of schools.
	"api.traeger.schools":  {max: 3, exact: true},
	"api.traeger.accounts": {max: 8, exact: true},
	// modules/enrollment — GET /traeger/holiday-periods (#4659): the Träger
	// binding of the Träger transaction, then the periods of the Träger,
	// their weeks and their time models. Flat in the number of periods and
	// weeks.
	"api.traeger.holiday_periods": {max: 4, exact: true},
	// modules/enrollment — GET /traeger/holiday-periods/{id}/registrations
	// (#4663, ADR 0114): the Träger binding of the Träger transaction, the
	// period with its weeks and time models, then the registrations, their
	// children with the duplicate hint and their bookings; the audit entry
	// writes but selects nothing. Flat in the number of registrations.
	"api.traeger.holiday_registrations": {max: 7, exact: true},
	// modules/organizationtenancy — GET /traeger/master-data (#4653): the
	// Träger binding of the Träger transaction, then the profile row and the
	// bank accounts of the Träger; the read log writes but selects nothing.
	// Flat in the number of bank accounts.
	"api.traeger.master_data": {max: 3, exact: true},
	// modules/traegerview — GET /traeger/access-log (#4655, ADR 0109): the
	// Träger row, the school IDs of the Träger, one access trail read of the
	// Audit Platform, the school rows, the staff members behind staff IDs and
	// their persons, the persons of the accounts and one Träger-Büro address
	// read, in one administrative transaction; then the tenant binding of
	// each shown school's transaction, which writes the read to that
	// school's access log (two schools in the scenario). Flat in the number
	// of entries.
	"api.traeger.access_log": {max: 10, exact: true},
	// organizationtenancy Provisioning.ListTraegerMembers (#4649): the Träger
	// row, its memberships and one read of address and Träger roles for all
	// members. Flat in the number of members.
	"api.operator.traeger_members": {max: 3, exact: true},
	// services — the operator school listing (#3568), organizationtenancy
	// Provisioning.ListSchoolSummaries: the school rows plus one aggregate
	// each for accounts, devices, persons and the Kontingentzahl, eight
	// statements in all. Flat in the number of schools and children.
	"services.operator.school_summaries": {max: 8},
	// modules — the operator billing report (#2791), organizationtenancy
	// BillingReport. The listing is one read of the captured rows. A capture
	// is the key day, the schools still missing, and one insert for all of
	// them; the owners' counts are stubbed in the scenario and add one read
	// each in production. Both stay flat in the number of schools.
	"modules.organizationtenancy.billing.key_date_counts": {max: 1, exact: true},
	"modules.organizationtenancy.billing.capture":         {max: 3, exact: true},
	// workflows/parentportal/http — GET /me/children/{studentId}/courses resolves the catalog,
	// capacity and pending-request queue through this bounded service scenario.
	"api.parent.child_courses": {max: 14},
	// workflows/parentportal — the pickup permission request list resolves the
	// caller's child, then reads all requests once. The test uses three requests
	// to prevent a per-row request lookup.
	"workflows.parentportal.pickup_permission_requests.list": {max: 12},
	// modules/enrollment/http — GET /phases/{id}/responses: phase plus four batched
	// response sources inside the tenant transaction. The matching test proves
	// the total stays flat from three to eight children.
	"api.enrollment.phase_responses.list": {max: 9},
	// modules/enrollment/http — GET /admin/requests: the caller's read state of every
	// listed enrollment in one batched statement (#3778).
	"api.enrollment.admin_requests.read_state": {max: 1, exact: true},
	// api/students — #2059: schema capabilities are fixed at startup.
	"api.students.requests.schema_introspection": {max: 0, exact: true},
	// api/students — #2098: each planning-time bulk load runs once per list request.
	"api.students.list.planning_times.per_table": {max: 1, exact: true},
	// api/class-list-entries — GET the class-list-only entries (#2382), 3
	// entries each with a namesake student. The "Zuordnen" hint is resolved
	// per entry (the entry set is a hand-maintained handful per school), so
	// this budget is the fixture-sized total: a third statement per entry, or
	// a constant read moved inside the loop, fails the test.
	"api.class_list_entries.list": {max: 8},
	// api/students — GET /students list, 10 students, page_size=50.
	// Lowered from 31 in #3349: the owner's directory read selects the
	// departure plans in the same statement the rows come from, where the
	// retained repository needed a second one to reach the scan-only columns.
	"api.students.list": {max: 30},
	// api/students — GET /students?location_state=present&care_offering_id=
	// (#4571), 10 booked present children: the offering filter reads its
	// bookings, exceptions and status days once per request.
	"api.students.list.care_offering": {max: 26},
	// api/students — GET /students/care-offerings (#4571), 10 offerings:
	// the request frame plus one catalog read for the picker selection.
	"api.students.care_offerings.list": {max: 6},
	// api/students — GET /care-withdrawals, page_size=1 over four pending
	// withdrawals (#3412). The count and the page each run as one Care Plan
	// statement that joins the named student directory projection; the
	// matching test also pins the rows read at one and at four children.
	"api.students.care_withdrawals.list": {max: 6},
	// workflows/studentdeletion — the permanent deletion behind DELETE
	// /students/{id} and DELETE /students/care-withdrawals/{id} (#3411), over
	// the production composition with one and with four linked children.
	// Preview takes no lock and costs the same at both sizes: the tenant
	// transaction (4), the student and person reads and one owner-count block
	// of 16 statements (the Feedback counter is a test double here).
	// Execute runs deleteConfirmed -> lockedSnapshot: the companion snapshot
	// twice plus the stranding re-read, one locked student-row read (with its
	// shared class-writes gate) per child in the graph, K unlocked re-reads,
	// the subject's second locked read, and the owner-count block twice
	// (fingerprint, then the recheck before the cascade: 32 statements), then
	// the nine-statement cascade and audit. Each linked child adds three
	// statements; .student_rows pins the 2(K+1) row round trips on
	// their own so a third pass over the graph cannot hide behind a saving
	// elsewhere.
	"workflows.studentdeletion.preview.companions_1":              {max: 22},
	"workflows.studentdeletion.preview.companions_4":              {max: 22},
	"workflows.studentdeletion.execute.companions_1":              {max: 61},
	"workflows.studentdeletion.execute.companions_4":              {max: 70},
	"workflows.studentdeletion.execute.companions_1.student_rows": {max: 4},
	"workflows.studentdeletion.execute.companions_4.student_rows": {max: 10},
	// api/students — #2056: aggregated OGS group view, 10 students.
	"api.students.ogs_group_live": {max: 41},
	// api/students — GET /students/ogs-group-navigation over every tenant
	// group: the group list, rooms, substitutions and one bulk Gruppenleitung
	// lookup (#4351), independent of the number of groups and teachers.
	"api.students.ogs_group_navigation": {max: 19},
	// api/students — #2099: identity chain resolved once per request.
	"api.students.ogs_group_live.identity.person":        {max: 1, exact: true},
	"api.students.ogs_group_live.identity.staff":         {max: 1, exact: true},
	"api.students.ogs_group_live.identity.teacher":       {max: 1, exact: true},
	"api.students.ogs_group_live.identity.substitutions": {max: 2, exact: true},
	// api/auth — #2065: shell settings prefetch → one config.setting_values read.
	"api.auth.tenant_resolve.setting_values": {max: 1, exact: true},
	// api/active — #2065: toggle + three labels share one settings read.
	"api.active.tracking_indicators.setting_values": {max: 1, exact: true},
	// api/active — aggregated supervision dashboard, 10 checked-in students.
	// 41 since #3065: the projection reads which rooms the administration has
	// released, once per request. The measured count sat at 40 — the cap
	// itself — before that read, so this is a raise, which normally means an
	// N+1. It is not one, and the tests say so rather than the comment:
	// TestSupervisionDashboard_QueryBudget asserts the count is identical at 3
	// and at 10 students, and TestOpenRoomLoadCostDoesNotGrowWithRoomsOrSessions
	// asserts one call per port for one room with one session and for four
	// rooms with four. The shared view costs a fixed three reads when a school
	// has released rooms and this single read when it has none.
	//
	// Reviewer approval for the raise is recorded in the #3065 pull request,
	// per the deviation clause in .claude/rules/backend-conventions.md.
	//
	// 42 since #3946: the Kinderkarte marks an Abholart set for today only,
	// read for all listed children in one statement. The same 3-vs-10 check
	// above holds; approval is recorded in the #3946 pull request.
	//
	// 45 since #4354: the page head names who supervises the open session or
	// room. One read of the sessions' current supervisors and one bulk name
	// read for them (staff, profile, person) serve every tab and every
	// released room together, so the cost stays fixed: the 3-vs-10 check
	// above and the rooms/blocks check below hold unchanged. Approval for the
	// raise belongs in the #4354 pull request.
	"api.active.supervision_dashboard": {max: 45},
	// api/active — the same aggregate with released rooms running timetable
	// blocks (#3281): three rooms, five blocks, every block supervised. The
	// block rows cost four fixed statements on top of the shared view (the
	// day's instances, their plan entries, the caller's person and staff
	// rows); TestOpenRooms_BlockCostDoesNotGrowWithRoomsOrBlocks asserts the
	// count is identical for one room with one block. 51 since #3946: the
	// shared view's one read of today's Abholart (see above). 54 since #4354:
	// the supervisors' names (see above).
	"api.active.supervision_dashboard.open_room_blocks": {max: 54},
	// api/active — GET /active/groups list, 8 active groups with visits.
	"api.active.groups.list": {max: 9},
	// #2941: formerly one identity query per supervisor / teacher.
	"api.activities.available_supervisors.specialization.reads": {max: 5},
	"api.iot.teacher_students.reads":                            {max: 5},
	"api.rooms.snapshot_export.reads":                           {max: 4},
	// modules/timetable/http — 14-day /week: FindByID + 7 preloads + class-exception
	// lookup (#2962) with headroom for bun metadata reads; was ~98 pre-fix.
	"api.timetable.student_week.14d": {max: 13},
	// modules/timetable/http — GET /instances over a week, 8 instances on 3 days
	// from one template with an education group: instances + one display-metadata
	// projection + staff batch + student batch + one cutoff read per day.
	"api.timetable.instances.list": {max: 7},
	// Group filters use one Timetable-owned candidate read and one existing
	// People Directory read for the planned children's current OGS groups.
	// The scenario stays flat from three to eight planned blocks.
	"api.timetable.instances.list.group_filter": {max: 9},
	// modules/timetable/http — GET /templates: template rows, retained list enrichments,
	// plus the setting, offering and series-root reads for roster maintenance
	// (#3140). The test proves all 11 statements stay flat from 3 to 8 rows.
	"api.timetable.templates.list": {max: 11},
	// modules/timetable/http — GET /pickup-extensions (#3261), day tasks only: tenant
	// transaction and tenant setup, task read, one batched block read,
	// student and person names. Flat in the number of open tasks.
	"api.timetable.pickup_extensions.list": {max: 9},
	// modules/timetable/http — GET /periods (#3124): tenant transaction (BEGIN, SET
	// LOCAL ROLE, set_config, COMMIT) + period list + one usage read per
	// owner (Enrollment phases, Timetable planning tables). The two owner
	// round trips are the accepted #2580 boundary cost; the count is flat in
	// the number of periods. Pinned exact so an owner-boundary move fails
	// here instead of at a runtime checkpoint (#3020).
	"api.timetable.periods.list": {max: 7, exact: true},
	// modules/timetable/compose operational day (#3551) — GET /planned-now backing list, 8 eligible instances:
	// instance list + staff batch + student batch (#2941) + block display
	// batch, which names the room and the planning track (#4413).
	"services.schedule.planned_now": {max: 4},
	// modules/timetable/compose — POST /instances/bulk-cancel
	// dry run (#3594): tenant transaction (BEGIN, SET LOCAL ROLE, set_config,
	// COMMIT) + one instance range read + one series read for the closing-day
	// flag. Flat from 2 to 5 occurrences. The execution reuses Cancel and
	// DeleteCancelled per occurrence by design and is not budgeted here.
	"services.schedule.bulk_cancel_dry_run": {max: 6, exact: true},
	// modules/schoolcalendar/portal — ListMyStaffEvents over a week, 8 appointments.
	"services.calendar.list_my_staff_events": {max: 11},
	// modules/schoolcalendar/portal — one authenticated CalDAV snapshot. The operation
	// resolves the capability owner once, then batch-loads the shared personal
	// calendar projection regardless of the number of returned VEVENTs.
	"services.calendar.caldav_snapshot": {max: 24},
	// #2941: list/read enrichment stays flat as result rows grow.
	"services.active.work_session_history.reads":           {max: 7},
	"services.active.future_comp_time_commitment.reads":    {max: 3, exact: true},
	"services.auth.pending_guardian_approvals.reads":       {max: 6},
	"services.auth.pending_invitations.reads":              {max: 1, exact: true},
	"services.platform.pending_operator_invitations.reads": {max: 1, exact: true},
	// ADR 0077: the school's caterer invitations and the caterer's schools,
	// including one bulk school-directory read to exclude deleted schools.
	"modules.identityaccess.caterer_invitations.reads": {max: 1, exact: true},
	"modules.identityaccess.caterer_schools.reads":     {max: 3, exact: true},
	// ADR 0077: the caterer's delivery list of a week over three schools
	// through the production router. Each school runs in its own
	// transaction with the supply check and the order read of
	// modules.mealplan.orders.read_week, so it grows by a fixed number per
	// school and never with the children; the window is at most a week.
	"workflows.catererportal.delivery_export":            {max: 112, exact: true},
	"services.education.suggest_mappings.reads":          {max: 4},
	"services.reminders.present_students_in_rooms.reads": {max: 6},
	"services.users.list_guardians.reads":                {max: 2},
	"services.users.student_guardians.reads":             {max: 4},
	"api.active.combination_groups.reads":                {max: 3, exact: true},
	// Enrollment decision and Care Plan offering source reads (formerly
	// services/enrollment) stay flat as rows grow (#2941).
	"services.enrollment.list_child_offerings.reads":    {max: 5},
	"services.enrollment.offering_source_options.reads": {max: 5},
	// Source validation adds one batched read each for the offering catalog,
	// its auto-add rules, periods, phases and series roots; flat from 3 to 8.
	"services.enrollment.template_roster_maintenance_feeds.reads": {max: 5},
	"services.enrollment.rollover_review_queue.reads":             {max: 3},
	// modules/schoolcalendar/portal — appointment target resolution, 8 explicit guardians.
	"services.calendar.resolve_targets.reads": {max: 5, exact: true},
	// modules/schoolcalendar/portal — reminder scan, 8 due appointments; writes scale with
	// recipients, while every read relation remains a fixed-size batch (#2941).
	"services.calendar.reminder_scan.reads": {max: 11, exact: true},
	// modules/schoolmembership — assignment lists, 8 rows each.
	"modules.schoolmembership.list_class_assignments": {max: 5},
	"modules.schoolmembership.list_group_assignments": {max: 5},
	// modules/schoolmembership/http — GET /staff?employment=former, the
	// Ehemalige filter (#3403): the school transaction's setup, the
	// memberships with the retired ones, then one batch of their persons.
	// Flat in the number of former employments.
	"modules.schoolmembership.staff.list_former": {max: 6},
	// The role-filtered Ehemalige variant adds one auth.account_roles batch;
	// it stays flat in the number of former employments.
	"modules.schoolmembership.staff.list_former.role": {max: 7},
	// modules/schoolstructure — staff-keyed group reads (#3499): teacher
	// groups, substituted groups and school classes in one pass, 1 and then
	// 4 rows each, on the caller's tenant transaction. Every read resolves
	// ids at its owner and loads groups in one batch.
	"modules.schoolstructure.staff_group_reads": {max: 6},
	// modules/classday — one slot reconciliation over the owner facades
	// inside its tenant transaction, 3 and then 8 planned children (#2701).
	// Every read is a bulk load by ID set; the count must not move with the
	// roster.
	"modules.classday.slot_list.build": {max: 16},
	// modules/mealplan — a week of orders reads each weekday in bulk (#3873),
	// without a lock or a write; a due order not stored yet shares its day's
	// reads with the current registration (ADR 0077).
	"modules.mealplan.orders.read_week": {max: 27},
	// modules/timetable — group and target lookups stay one bulk query as IDs grow.
	"modules.timetable.groups.list":              {max: 1, exact: true},
	"modules.timetable.group_targets.list":       {max: 1, exact: true},
	"modules.timetable.enrollments.list":         {max: 1, exact: true},
	"modules.timetable.timeframes.list":          {max: 1, exact: true},
	"modules.timetable.planning_tracks.list":     {max: 1, exact: true},
	"modules.timetable.activity_exceptions.list": {max: 1, exact: true},
	"modules.timetable.activity_instances.list":  {max: 1, exact: true},
	"modules.timetable.instance_staff.list":      {max: 1, exact: true},
	"modules.timetable.instance_students.list":   {max: 1, exact: true},
	"modules.timetable.schedules.list":           {max: 1, exact: true},
	"modules.timetable.supervisors.list":         {max: 1, exact: true},
	"modules.timetable.target_students.list":     {max: 1, exact: true},
	// The child's activities on one day (#4268): the enrollments, then their
	// groups in one batch. The note form's reference picker reads it.
	"modules.timetable.active_enrollments.list": {max: 2, exact: true},
	// modules/peopledirectory — one child's note card (#3632). Two statements,
	// whatever the timeline holds: the notes, and one bulk read that resolves
	// every author's name. Exact, because a count of one would mean the name
	// resolution silently stopped running and every entry lost its author.
	"modules.peopledirectory.student_notes.list": {max: 2, exact: true},
	// modules/communication — inbox reads remain fixed as thread count grows.
	"modules.communication.parent_messages.list_inbox": {max: 1, exact: true},
	"modules.communication.staff_messages.list_inbox":  {max: 2, exact: true},
	// The children with a reachable guardian (#4362): the portal links of the
	// whole school, then one batched membership check for their accounts.
	"modules.communication.parent_messages.list_reachable_students": {max: 2, exact: true},
	// The group threads (#4784): the OGS list and a guardian's list read the
	// threads with their newest message, counts and read state in one
	// statement each, however many threads exist.
	"modules.communication.parent_group_threads.list":              {max: 1, exact: true},
	"modules.communication.parent_group_threads.list_for_guardian": {max: 1, exact: true},
	// The surveys (#4781): the school's list with the participant counts and
	// a staff member's open surveys in one statement each; the evaluation
	// reads the survey, its questions, its options and three grouped counts.
	"modules.communication.surveys.list":      {max: 1, exact: true},
	"modules.communication.surveys.to_answer": {max: 1, exact: true},
	"modules.communication.surveys.results":   {max: 6},
	// database/repositories/users — due announcement reminders read their rows
	// and targets in two batches, regardless of the number due in one tick.
	"repositories.parent_announcements.due_reminders": {max: 2, exact: true},
	// modules/workforce/http/timetracking — GET
	// /api/staff-notices/{id}/acknowledgements (#2208): four tenant-transaction
	// statements (BEGIN, SET ROLE, set_config, COMMIT) plus one notice read, one
	// acknowledgement list and ONE batched People-Directory name lookup for the
	// whole list. Flat at three acknowledgements — a per-row lookup would show
	// as N+1 here.
	"api.staff_notices.acknowledgements": {max: 7, exact: true},
	// modules/workforce/http/timetracking — GET
	// /api/staff/{id}/target-overrides (#3259): the tenant-transaction
	// statements, the two staff lookups (membership, employment profile) and
	// ONE read of the staff member's
	// Sonderarbeitszeiten. Flat in the number of ranges.
	"workforce.staff_target_overrides.list": {max: 7, exact: true},
	"modules.careplan.request_feed.list":    {max: 1, exact: true},
	// modules/identityaccess caller context (behavior/caller_request_cache_test.go) —
	// the #2099 request cache dedups the identity chain.
	"services.usercontext.identity_chain.persons":       {max: 1, exact: true},
	"services.usercontext.identity_chain.staff":         {max: 1, exact: true},
	"services.usercontext.identity_chain.teachers":      {max: 1, exact: true},
	"services.usercontext.identity_chain.substitutions": {max: 2, exact: true},
	// Worker minute snapshot (settingsCompose.NewTenantSnapshots, #2746) — one
	// settings read for all tenants.
	"services.scheduler.minute_snapshot.setting_values": {max: 1, exact: true},
	// services/schedule — Dienstplan overview reads are fixed batches.
	// The second entry is cumulative on the same counter: 6 without shifts
	// plus 7 for a Dienstplan-active week (#1837).
	"services.schedule.staff_overview.week":                      {max: 6, exact: true},
	"services.schedule.staff_overview.week_plus_dienstplan_week": {max: 13, exact: true},
	"services.schedule.staff_overview.weekly_summaries":          {max: 9, exact: true},
	// A shift-only week (no timetable assignments) early-returns the
	// assignment-dependent reads; series_id/detached add ZERO reads.
	"services.schedule.staff_overview.shift_only_week": {max: 5, exact: true},
	"services.schedule.shift_coverage.series":          {max: 8, exact: true},
	// modules/emergencysnapshot — the Notfallliste projection (#2704) reads
	// each owner once, flat in the number of children: open attendance,
	// student rows, person identities, current visits, room names and
	// guardian contacts, after the three tenant-scope statements of the
	// transaction runtime. The presence mode is a settings read that the
	// scenario fakes.
	//
	// Lowered from 10 in #3349: the owner reads a child's departure plan in
	// the same statement as the row, where the retained repository needed a
	// second one to reach the scan-only columns.
	"modules.emergencysnapshot.snapshot": {max: 9, exact: true},
	// modules/dataimport/importtest — one ten-row student file after two
	// warmups, the bounds TestDataImportRuntimeEvidence asserted until it
	// became opt-in (#4426). The preview is exact; the import is a ceiling
	// (six owner statements per guardian link, three lock statements per
	// child since #3567).
	"modules.dataimport.student_batch.preview": {max: 16, exact: true},
	"modules.dataimport.student_batch.import":  {max: 324},
	// The same file at 110 rows. The preview reads one person per row
	// (6 + 1 per row), the import grows by about 31 statements per row; a
	// further per-row statement moves these counts by 110.
	"modules.dataimport.student_batch.preview_110": {max: 116, exact: true},
	"modules.dataimport.student_batch.import_110":  {max: 3432},
	// modules/requestreview/compose — #3179: the open review page of one
	// child with one request in each of the four native queues (the RLS
	// proof fixture), inside one tenant transaction: the urgent and the
	// normal phase of every queue, the conflict scan, the group and
	// Familienschutz decorations. This remains a ceiling for the fixture,
	// not a claim that every owner read has flat cost.
	"modules.requestreview.open_page": {max: 101},
	// test/e2e/timetable — end-to-end counts include TenantTxMiddleware overhead.
	"e2e.timetable.exception_conflicts.cancelled": {max: 22},
	"e2e.timetable.exception_conflicts.modified":  {max: 22},
	"e2e.timetable.student_week.14d":              {max: 25},
}

// AssertQueryBudget checks the statements a scenario issued against the
// register. Pass the matched statements (counter.Queries(), counter.Selects(...)
// or a Matching(...) bucket), never a bare number, so a failure can print them.
func AssertQueryBudget(tb testing.TB, scenario string, queries []string) {
	tb.Helper()
	budget, ok := queryBudgets[scenario]
	if !ok {
		tb.Fatalf("query budget %q is not registered in backend/test/query_budgets.go", scenario)
	}
	got := len(queries)
	switch {
	case got > budget.max:
		tb.Errorf("query budget exceeded: scenario %q issued %d statements, budget %d.\n"+
			"  Likely an N+1: a query inside a loop over rows. Batch-load by ID set instead\n"+
			"  (reference: modules/timetable/compose/exception_conflicts.go, FindByStudentIDsAndDate).\n"+
			"  Never raise the register entry.\n%s",
			scenario, got, budget.max, indentQueries(queries))
	case budget.exact && got < budget.max:
		tb.Errorf("query budget %q pins exactly %d statement(s) but %d ran.\n"+
			"  The dedup contract it guards no longer executes; check the scenario before touching the register.\n%s",
			scenario, budget.max, got, indentQueries(queries))
	case got < budget.max:
		tb.Logf("query budget %q: %d of %d statements used. Lower the register entry to lock the headroom in.",
			scenario, got, budget.max)
	}
}

func indentQueries(queries []string) string {
	if len(queries) == 0 {
		return ""
	}
	lines := make([]string, 0, len(queries)+1)
	lines = append(lines, "  Statements:")
	for i, q := range queries {
		q = strings.Join(strings.Fields(q), " ")
		if len(q) > 200 {
			q = q[:200] + "..."
		}
		lines = append(lines, fmt.Sprintf("  %3d. %s", i+1, q))
	}
	return strings.Join(lines, "\n")
}

// QueryBudgetScenarios lists the registered scenario names, sorted.
func QueryBudgetScenarios() []string {
	names := make([]string, 0, len(queryBudgets))
	for name := range queryBudgets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
