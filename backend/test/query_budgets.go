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
//     ID set (services/schedule/timetable_read_exception_conflicts.go, the
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
	// api/students — #2059: schema capabilities are fixed at startup.
	"api.students.requests.schema_introspection": {max: 0, exact: true},
	// api/students — #2098: each planning-time bulk load runs once per list request.
	"api.students.list.planning_times.per_table": {max: 1, exact: true},
	// api/students — GET /students list, 10 students, page_size=50.
	"api.students.list": {max: 32},
	// api/students — #2056: aggregated OGS group view, 10 students. Measured
	// well below; the cap leaves room for benign changes only.
	"api.students.ogs_group_live": {max: 41},
	// api/students — #2099: identity chain resolved once per request.
	"api.students.ogs_group_live.identity.person":        {max: 1, exact: true},
	"api.students.ogs_group_live.identity.staff":         {max: 1, exact: true},
	"api.students.ogs_group_live.identity.teacher":       {max: 1, exact: true},
	"api.students.ogs_group_live.identity.substitutions": {max: 2, exact: true},
	// api/auth — #2065: shell settings prefetch → one config.setting_values read.
	"api.auth.tenant_resolve.setting_values": {max: 1, exact: true},
	// api/active — #2065: toggle + three labels share one settings read.
	"api.active.tracking_indicators.setting_values": {max: 1, exact: true},
	// api/active — aggregated supervision dashboard, 10 checked-in students
	// (measured 29 flat; headroom for benign changes only).
	"api.active.supervision_dashboard": {max: 40},
	// api/active — GET /active/groups list, 8 active groups with visits.
	"api.active.groups.list": {max: 9},
	// #2941: formerly one identity query per supervisor / teacher.
	"api.activities.available_supervisors.specialization.reads": {max: 5},
	"api.iot.teacher_students.reads":                            {max: 5},
	"api.rooms.snapshot_export.reads":                           {max: 4},
	// api/timetable — 14-day /week: FindByID + 7 preloads + class-exception
	// lookup (#2962) with headroom for bun metadata reads; was ~98 pre-fix.
	"api.timetable.student_week.14d": {max: 13},
	// api/timetable — GET /instances over a week, 8 instances on 3 days:
	// instances + room + staff batch + student batch + one cutoff read per day.
	"api.timetable.instances.list": {max: 7},
	// services/schedule — GET /planned-now backing list, 8 eligible instances:
	// instance list + rooms + staff batch + student batch (#2941).
	"services.schedule.planned_now": {max: 4},
	// services/calendar — ListMyStaffEvents over a week, 8 appointments.
	"services.calendar.list_my_staff_events": {max: 11},
	// #2941: list/read enrichment stays flat as result rows grow.
	"services.active.work_session_history.reads":           {max: 7},
	"services.active.future_comp_time_commitment.reads":    {max: 4, exact: true},
	"services.auth.pending_guardian_approvals.reads":       {max: 6},
	"services.education.suggest_mappings.reads":            {max: 4},
	"services.reminders.present_students_in_rooms.reads":   {max: 6},
	"services.users.list_guardians.reads":                  {max: 2},
	"services.users.student_guardians.reads":               {max: 4},
	"repositories.active.combined_group_with_groups.reads": {max: 3, exact: true},
	"repositories.activities.supervisors_by_staff.reads":   {max: 2, exact: true},
	// services/enrollment — list/read paths stay flat as rows grow (#2941).
	"services.enrollment.list_child_offerings.reads":    {max: 5},
	"services.enrollment.offering_source_options.reads": {max: 5},
	"services.enrollment.rollover_review_queue.reads":   {max: 3},
	// services/calendar — appointment target resolution, 8 explicit guardians.
	"services.calendar.resolve_targets.reads": {max: 5, exact: true},
	// services/calendar — reminder scan, 8 due appointments; writes scale with
	// recipients, while every read relation remains a fixed-size batch (#2941).
	"services.calendar.reminder_scan.reads": {max: 11, exact: true},
	// modules/schoolmembership — assignment lists, 8 rows each.
	"modules.schoolmembership.list_class_assignments": {max: 5},
	"modules.schoolmembership.list_group_assignments": {max: 5},
	// modules/timetable — group and target lookups stay one bulk query as IDs grow.
	"modules.timetable.groups.list":          {max: 1, exact: true},
	"modules.timetable.group_targets.list":   {max: 1, exact: true},
	"modules.timetable.enrollments.list":     {max: 1, exact: true},
	"modules.timetable.timeframes.list":      {max: 1, exact: true},
	"modules.timetable.schedules.list":       {max: 1, exact: true},
	"modules.timetable.supervisors.list":     {max: 1, exact: true},
	"modules.timetable.target_students.list": {max: 1, exact: true},
	// modules/communication — inbox reads remain fixed as thread count grows.
	"modules.communication.parent_messages.list_inbox": {max: 1, exact: true},
	"modules.communication.staff_messages.list_inbox":  {max: 2, exact: true},
	// services/usercontext — #2099 request cache dedups the identity chain.
	"services.usercontext.identity_chain.persons":       {max: 1, exact: true},
	"services.usercontext.identity_chain.staff":         {max: 1, exact: true},
	"services.usercontext.identity_chain.teachers":      {max: 1, exact: true},
	"services.usercontext.identity_chain.substitutions": {max: 2, exact: true},
	// services/scheduler — one minute snapshot = one settings read for all tenants.
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
			"  (reference: services/schedule/timetable_read_exception_conflicts.go, FindByStudentIDsAndDate).\n"+
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
