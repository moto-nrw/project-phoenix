package test

import (
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/uudashr/gocognit"
)

// TestModuleComplexityRatchet extends .claude/rules/backend-conventions.md
// Rule 4 — gocognit cognitive complexity ≤ 15 — from api/ to the capability
// modules and application workflows.
//
// Why a second ratchet instead of a wider api/ one: migration #2580 moved
// 228,306 of 638,717 production LOC into modules/ and workflows/, and every
// existing quality gate stops at the api/ and models/ directory boundary.
// Moved code left the ratchet's reach without a single line changing. This
// test closes that gap by freezing today's state under the new roots.
//
// TestHandlerComplexityRatchet stays separate and stays at zero. Its empty
// allowlist is a hard-won result (75 offenders retired since 2026-07-12);
// merging the two registers would bury that zero inside a four-hundred-entry
// map, where nobody would notice it growing back.
//
// Allowlist semantics (per function, not per file), identical to the api/
// ratchet:
//
//   - A function NOT in the allowlist must score ≤ 15.
//   - An allowlisted function may never exceed its recorded score.
//   - When refactoring lowers a score (or drops it to ≤ 15), the test fails
//     until the entry is lowered/removed — the ratchet only turns one way.
//     Never raise a number, never add an entry.
//
// Keys are "relative/file.go:FuncName" with methods rendered exactly like the
// gocognit CLI ("(*service).ListOptions"), so CLI output maps 1:1 onto
// entries here. Scan roots are modules/ and workflows/, non-test .go files
// only; "legacy" subdirectories are counted deliberately — the retained
// legacy code is the bulk of the debt this ratchet is meant to drain.
const moduleComplexityThreshold = 15

// Seeded 2026-09-18 at merge commit ecf0003369, the state of modules/ and
// workflows/ right after the #2580 move, measured with:
//
//	cd backend && ../scripts/run-go-toolchain.sh \
//	    go run github.com/uudashr/gocognit/cmd/gocognit -over 15 ./modules ./workflows
//
// (_test.go hits filtered out). 444 functions. The first seed was taken at
// 19feca2822 and re-measured here after the merge of origin/development
// (a47f77b2c3): PR #3408 (issue #3350) moved eleven functions over the limit
// into modules/, which the ratchet cannot absorb without a fresh measurement.
// The entries record the debt as it was on that day; every one of them may
// only shrink or disappear.
//
// Re-measured on 2026-10-19 at 148e5aeb86 for the gocognit 1.2.1 → 1.3.0
// upgrade (ADR 0072). The new version scores else nesting, parenthesized
// logical operators and recursion differently; no code changed. Thirteen
// entries rose, three functions entered at 16, and two entries fell: the
// register's sum went from 5,460 to 5,527 over 228 entries. This is a tool
// rebaseline, not new debt; ADR 0072 records why the functions were not
// refactored instead. Every entry keeps falling from here.
//
// Sorted by key, with a blank line between owner modules so gofmt aligns each
// block on its own and removing an entry reflows only that module's block.
var moduleComplexityAllowlist = map[string]int{
	"modules/careplan/carerequests/weekly.go:weeklyEntries":                                                      39,
	"modules/careplan/internal/adapters/postgres/offering_bookings.go:(*Store).RecordCareOfferingBookings":       16,
	"modules/careplan/internal/adapters/postgres/offering_bookings.go:(*Store).replaceCareOfferingBookings":      17,
	"modules/careplan/internal/adapters/postgres/status_days.go:(*statusDayStore).ArchiveStudentStatusFlags":     22,
	"modules/careplan/internal/adapters/postgres/status_days.go:(*statusDayStore).CountEffectiveStudentAbsences": 16,
	"modules/careplan/internal/adapters/postgres/withdrawal_completions.go:(*DirectoryStore).ListWithdrawals":    20,
	"modules/careplan/internal/application/excused_requests.go:(*ExcusedRequests).Correct":                       18,
	"modules/careplan/internal/application/excused_requests.go:(*ExcusedRequests).Decide":                        34,
	"modules/careplan/internal/application/excused_requests.go:(*ExcusedRequests).ListPending":                   21,
	"modules/careplan/internal/application/offering_review_pending.go:(*OfferingReviews).pendingFacts":           28,
	"modules/careplan/internal/application/offering_review_pending.go:(*OfferingReviews).pendingReviews":         56,
	"modules/careplan/internal/application/offering_review_pending.go:materializeOfferingReview":                 23,
	"modules/careplan/internal/application/offering_reviews.go:(*OfferingReviews).ListHistory":                   21,
	"modules/careplan/internal/application/offering_reviews.go:(*OfferingReviews).ListPending":                   17,
	"modules/careplan/internal/application/review_weekly_plan.go:(reviewPlanFacts).pickupWeek":                   25,
	"modules/careplan/internal/application/schedule_reviews.go:(*ScheduleReviews).ListPending":                   35,
	"modules/careplan/internal/application/schedule_reviews.go:(*ScheduleReviews).plan":                          23,
	// Moved in by PR #3408 (#3350) from services/users and
	// database/repositories/users; the scores crossed the boundary unchanged.
	"modules/careplan/offeringrequests/diff.go:CorrectionDiff": 21,

	"modules/enrollment/delete_schema.go:(*Module).DeleteUnusedSchema":                                       27,
	"modules/enrollment/form_schema.go:(*FormField).validateQuestion":                                        34,
	"modules/enrollment/form_schema.go:(*FormLegalBlock).Validate":                                           26,
	"modules/enrollment/form_schema.go:(*FormSchema).Validate":                                               23,
	"modules/enrollment/form_schema.go:(*VisibilityCondition).Validate":                                      20,
	"modules/enrollment/internal/adapters/postgres/change_request_reads.go:(*Store).ChangeRequestsForReview": 17,
	"modules/enrollment/phase.go:(*Phase).Validate":                                                          41,
	"modules/enrollment/publish_schema.go:(*Module).PublishSchema":                                           21,
	"modules/careplan/selection/automatic_shares.go:AutomaticShares":                                         18,
	"modules/careplan/selection/materialize.go:MaterializeAdjustments":                                       44,
	"modules/careplan/selection/materialize.go:autoLunchDaysForTarget":                                       16,
	"modules/careplan/selection/materialize.go:materializeOfferingSelections":                                44,
	"modules/careplan/selection/materialize.go:validateCareOfferingSelectionModeWithMissing":                 29,
	"modules/careplan/selection/types.go:(*AvailabilityRule).NormalizeAndValidate":                           21,

	"modules/planexport/betreuungsplan.go:(*betreuungsplanData).rows":      23,
	"modules/planexport/betreuungsplan.go:(*betreuungsplanData).staffLine": 17,
	"modules/planexport/betreuungsplan.go:(*service).blockColors":          21,
	"modules/planexport/dienstplan.go:(*dienstplanData).rowsByArea":        51,
	"modules/planexport/service.go:(*service).nonWorkingDays":              28,

	"modules/schoolcalendar/internal/adapters/postgres/store.go:(*Store).ListDateframes":                          18,
	"modules/schoolcalendar/internal/domain/ical.go:formatRRULE":                                                  22,
	"modules/schoolcalendar/portal/internal/application/feed.go:(*service).ParentCalendarFeedByToken":             58,
	"modules/schoolcalendar/portal/internal/application/ics.go:(*service).ParentAppointmentICS":                   24,
	"modules/schoolcalendar/portal/internal/application/notifications.go:(*service).notifyGuardians":              26,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).GetParentAppointmentOverview":       28,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).ListMyParentEvents":                 22,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).RecipientOptions":                   52,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).RespondToParentInvitation":          41,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).buildAppointmentOverview":           16,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).cancelStaffAppointment":             18,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).createStaffAppointment":             22,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).expandAppointmentEvents":            18,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).expandGuardianAppointmentEvents":    20,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).guardianRecipientStatusForStudents": 16,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).staffShiftEventsWithCancelled":      19,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).updateStaffAppointment":             29,

	"modules/schoolmembership/internal/adapters/postgres/store.go:(*Store).ListGuests":                         18,
	"modules/schoolmembership/internal/adapters/postgres/store.go:(*Store).ListTeachers":                       17,
	"modules/schoolmembership/internal/adapters/postgres/teaching_assignment.go:(*Store).ListGroupAssignments": 16,
	"modules/schoolmembership/internal/application/offboarding.go:(*Service).RetireStaff":                      17,
	"modules/schoolmembership/internal/application/offboarding.go:(*Service).retirementSnapshot":               24,

	"modules/schoolstructure/internal/adapters/postgres/transition_store.go:(*Store).ListTransitions": 16,
	"modules/schoolstructure/internal/application/transition.go:(*Service).UpdateTransition":          32,

	"modules/studentpresence/internal/adapters/postgres/live_groups.go:(*Store).QueryGroupSupervisions": 19,
	"modules/studentpresence/internal/adapters/postgres/live_groups.go:(*Store).QueryLiveGroups":        19,
	"modules/studentpresence/internal/application/live_groups.go:(*Service).QueryGroupSupervisions":     26,
	"modules/studentpresence/internal/application/live_groups.go:(*Service).QueryLiveGroups":            21,
	"modules/studentpresence/internal/application/school_status.go:(*Service).ListSchoolStatuses":       21,

	"modules/timetable/internal/application/activity_supervisor_operations.go:buildReplacementSupervisorIDs": 22,
	"modules/timetable/internal/application/activity_instances.go:(*Service).lockOperationalInstanceStaff":   20,
	"modules/timetable/internal/application/pickup_extensions.go:(*Service).ensurePickupWeekdayEnrollment":   16,
	"modules/timetable/internal/application/pickup_extensions.go:(*Service).pickupExtensionBlocks":           23,
	"modules/timetable/internal/application/recurrence_events.go:(*Service).GenerateRecurrenceEvents":        21,
	"modules/timetable/record_validation.go:validStudentEnrollment":                                          16,

	// Moved in by #3996 from modules/workforce/legacy/absence_repositories.go;
	// the scores crossed the boundary unchanged.
	"modules/workforce/internal/adapters/postgres/shift_store.go:applyStaffShiftFilter":                                                         25,
	"modules/workforce/internal/adapters/postgres/substitution_store.go:(*Store).ListGroupSubstitutions":                                        19,
	"modules/workforce/internal/application/absence.go:(*Service).UpdateAbsenceType":                                                            23,
	"modules/workforce/internal/application/absence_allowance.go:(*Service).PreviewAllowanceRebooking":                                          17,
	"modules/workforce/internal/application/absence_allowance.go:(*Service).SetAllowance":                                                       22,
	"modules/workforce/internal/application/offboarding.go:(*Service).ExecuteStaffOffboarding":                                                  39,
	"modules/workforce/internal/application/substitution.go:(*Service).UpdateGroupSubstitution":                                                 16,
	"modules/workforce/internal/domain/absence_allowance.go:BuildAllowanceLedger":                                                               23,
	"modules/workforce/internal/domain/shift.go:ValidateStaffShiftSeries":                                                                       20,
	"modules/workforce/internal/domain/staffrecord.go:(StaffMasterData).Validate":                                                               16,
	"modules/workforce/internal/planning/staff_schedule_overview.go:(*staffScheduleOverviewService).resolveWeeklyTargets":                       44,
	"modules/workforce/internal/planning/staff_shift_move.go:(*staffShiftService).MoveShift":                                                    42,
	"modules/workforce/internal/planning/staff_shift_series_service.go:(*staffShiftSeriesService).SplitSeries":                                  34,
	"modules/workforce/internal/planning/staff_shift_cancellation.go:(*staffShiftService).ApplyCancellation":                                    49,
	"modules/workforce/internal/planning/staff_shift_service.go:(*staffShiftService).DeleteShift":                                               16,
	"modules/workforce/internal/planning/staff_shift_service.go:(*staffShiftService).updateShiftWithOptions":                                    45,
	"modules/workforce/internal/application/timetracking/labor_time_policy.go:EvaluateWorkSessionsLaborTime":                                    16,
	"modules/workforce/internal/application/timetracking/labor_time_policy.go:netMinutesByDate":                                                 29,
	"modules/workforce/internal/application/timetracking/staff_absence_rebooking.go:(*staffAbsenceService).RebookAbsences":                      21,
	"modules/workforce/internal/application/timetracking/staff_absence_rebooking.go:(*staffAbsenceService).previewVacationRebooking":            20,
	"modules/workforce/internal/application/timetracking/staff_absence_rebooking.go:(*staffAbsenceService).validateRebookedAbsence":             19,
	"modules/workforce/internal/application/timetracking/staff_absence_creation.go:(*staffAbsenceService).CreateAbsenceFor":                     19,
	"modules/workforce/internal/application/timetracking/staff_absence_comp_time.go:(*staffAbsenceService).PreviewCompTimeBalance":              17,
	"modules/workforce/internal/application/timetracking/staff_absence_mutations.go:(*staffAbsenceService).UpdateAbsence":                       33,
	"modules/workforce/internal/application/timetracking/staff_absence_quota.go:(*staffAbsenceService).ensureVacationFits":                      16,
	"modules/workforce/internal/application/timetracking/staff_balance_adjustment_service.go:(*staffBalanceAdjustmentService).DeleteAdjustment": 19,
	"modules/workforce/internal/application/timetracking/staff_balance_adjustment_service.go:(*staffBalanceAdjustmentService).ResetBalance":     23,
	"modules/workforce/internal/application/timetracking/staff_overview_service.go:(*staffOverviewService).GetDashboardSummary":                 18,
	"modules/workforce/internal/application/timetracking/staff_overview_service.go:(*staffOverviewService).GetTimeTrackingOverview":             31,
	"modules/workforce/internal/application/timetracking/staff_overview_service.go:(*staffOverviewService).addTodayCounters":                    18,
	"modules/workforce/internal/application/timetracking/staff_overview_service.go:(*staffOverviewService).buildPrefetch":                       26,
	"modules/workforce/internal/application/timetracking/staff_time_export_datev.go:buildDatevLines":                                            22,
	"modules/workforce/internal/application/timetracking/staff_time_export_rows.go:(*staffOverviewService).GetMonthExportRows":                  20,
	"modules/workforce/internal/application/timetracking/work_session_checkin.go:(*workSessionService).checkIn":                                 23,
	"modules/workforce/internal/application/timetracking/work_session_history.go:(*workSessionService).buildWeeklySummaries":                    17,
	"modules/workforce/internal/application/timetracking/work_session_maintenance.go:(*workSessionService).AutoCheckoutDueSessions":             53,
	"modules/workforce/internal/application/timetracking/work_session_updates.go:(*workSessionService).UpdateSession":                           18,
	"modules/workforce/internal/application/timetracking/work_time_month_service.go:(*workTimeMonthService).GetDailyProjection":                 17,
	"modules/workforce/internal/application/timetracking/work_time_month_service.go:(*workTimeMonthService).getOpeningBalanceOnDate":            21,
	"modules/workforce/internal/application/timetracking/work_time_month_service.go:walkCreditedAbsenceDays":                                    19,
}

func TestModuleComplexityRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	scores, err := moduleComplexityScan(backendRoot)
	if err != nil {
		t.Fatalf("complexity scan failed: %v", err)
	}

	violations := moduleComplexityViolations(scores, moduleComplexityAllowlist)
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("Module-complexity ratchet check failed (%d issue(s)):\n\n%s",
			len(violations), strings.Join(violations, "\n\n"))
	}
}

// moduleComplexityViolations compares per-function scores over the threshold
// against the allowlist. Like its api/ counterpart it cannot reuse
// ratchetViolations: entries here are capped scores, not hit counts, and an
// allowlisted function dropping to or below the threshold disappears from the
// scan entirely (which is the "ratchet down" signal, not a stale-entry error
// in the per-file sense).
func moduleComplexityViolations(scores, allowlist map[string]int) []string {
	var violations []string
	for fn, got := range scores {
		allowed, ok := allowlist[fn]
		switch {
		case !ok:
			violations = append(violations, fmt.Sprintf(
				"[gocognit] %s scores %d (limit %d), and is not in the allowlist.\n  Move branching/orchestration behind a capability method (Rule 4) — do not add new entries.",
				fn, got, moduleComplexityThreshold))
		case got > allowed:
			violations = append(violations, fmt.Sprintf(
				"[gocognit] %s scores %d, allowed %d.\n  Complexity grew. Simplify the function — never raise the allowlist.", fn, got, allowed))
		case got < allowed:
			violations = append(violations, fmt.Sprintf(
				"[gocognit] %s scores %d, allowlist says %d.\n  Nice — ratchet the entry down to %d so the improvement cannot regress.", fn, got, allowed, got))
		}
	}
	for fn, allowed := range allowlist {
		if _, ok := scores[fn]; !ok {
			violations = append(violations, fmt.Sprintf(
				"[gocognit] %s is allowlisted at %d but now scores ≤ %d (or was deleted/renamed).\n  Remove the entry.",
				fn, allowed, moduleComplexityThreshold))
		}
	}
	return violations
}

// Moved root bindings retain their original coverage and thresholds (ADR 0064).
// moduleComplexityScan returns gocognit scores above the threshold for every
// function in non-test .go files under modules/ and workflows/, keyed
// "relpath:FuncName" with methods rendered like the gocognit CLI
// ("(*service).name"). It shares funcDisplayName with the api/ ratchet so
// both registers speak the same key language.
func moduleComplexityScan(backendRoot string) (map[string]int, error) {
	scores := make(map[string]int)
	for _, sub := range []string{"modules", "workflows", "roots/requestreviewbinding", "roots/sessionendbinding", "roots/openroommovebinding", "roots/owners/parent_announcement_audience_binding.go", "roots/owners/parent_inbox_binding.go", "roots/owners/staff_inbox_binding.go"} {
		root := filepath.Join(backendRoot, sub)
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if strings.HasPrefix(d.Name(), ".") && path != root {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, relErr := filepath.Rel(backendRoot, path)
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)

			_, file, parseErr := parseGoSourceCached(path, nil)
			if parseErr != nil {
				return fmt.Errorf("parse %s: %w", rel, parseErr)
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				if score := gocognit.Complexity(fn); score > moduleComplexityThreshold {
					scores[rel+":"+funcDisplayName(fn)] = score
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return scores, nil
}
