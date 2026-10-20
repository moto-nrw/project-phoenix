package test

import (
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestModuleFunctionLengthRatchet caps function body length in the modules
// that migration #2580 already carved out of api/ and models/.
//
// Why a second length gate: every existing quality gate in this package stops
// at the api/ or models/ directory boundary. #2580 moved 228,306 of 638,717
// production LOC into backend/modules/ and backend/workflows/, so the code
// that grows fastest is exactly the code nothing measures. A 300-line service
// method is the shape .claude/rules/backend-conventions.md keeps legislating
// against from three directions — Rule 4 (thin handlers), Rule 8 (services
// encapsulate, not delegate) and Rule 12 (models hold data, not decisions) —
// without any of them counting lines. This ratchet counts lines, which is the
// crude proxy that catches the cases cognitive complexity misses: long, flat,
// branch-free orchestration that should be three named steps.
//
// The threshold is 60 body lines, measured from the opening brace line to the
// closing brace line inclusive, on named declarations only. Function literals
// are not counted: a long literal is a property of its enclosing declaration,
// which the ratchet already measures.
//
// Allowlist semantics (per function, not per file — four cases):
//
//   - A function NOT in the allowlist must have a body of ≤ 60 lines.
//   - An allowlisted function may never exceed its recorded length.
//   - When a split shortens a function, the test fails until the entry is
//     lowered — the ratchet only turns one way. Never raise a number.
//   - When a function drops to ≤ 60 lines (or is deleted or renamed) it
//     disappears from the scan and its entry must be removed.
//
// Keys are "relative/file.go:FuncName" with methods rendered the way the
// gocognit CLI renders them ("(*service).ListOptions"), so entries here line
// up with the neighbouring complexity ratchet's keys.
//
// Scope: every non-test .go file under modules/ and workflows/, including the
// legacy/ subtrees. Those are counted deliberately — the retained legacy code
// is the larger half of the problem, and exempting it would let a migration
// move a 200-line method into legacy/ to silence the gate.
//
// Seed: measured 2026-09-18 at merge commit ecf0003369 with a throwaway
// go/ast walk over modules/ and workflows/ (named FuncDecls with a body,
// non-test files), recording fn.Body.Rbrace.Line - fn.Body.Lbrace.Line + 1
// per function:
//
//	cd backend && ../scripts/run-go-toolchain.sh go run ./tmp/moduleFuncLen
//
// The first seed was taken at 19feca2822 and re-measured here after the merge
// of origin/development (a47f77b2c3): PR #3408 (issue #3350) moved seven
// over-threshold functions into modules/, which the ratchet cannot absorb
// without a fresh measurement.
//
// 299 of 14,623 functions were over the threshold at seed time (483 over 50
// lines, 83 over 100, 5 over 200). The seed freezes that state; it is not a
// statement that these 299 functions are acceptable.
const moduleFuncLenThreshold = 60

// Seeded 2026-09-18 from merge commit ecf0003369. Shrink-only: lower or
// delete entries, never add one and never raise a number.
var moduleFuncLenAllowlist = map[string]int{
	"modules/careplan/internal/adapters/postgres/withdrawal_completions.go:(*DirectoryStore).ListWithdrawals": 71,
	"modules/careplan/internal/application/excused_requests.go:(*ExcusedRequests).Correct":                    73,
	"modules/careplan/internal/application/excused_requests.go:(*ExcusedRequests).Decide":                     149,
	"modules/careplan/internal/application/excused_requests.go:(*ExcusedRequests).ListPending":                74,
	"modules/careplan/internal/application/offering_review_pending.go:(*OfferingReviews).pendingFacts":        92,
	"modules/careplan/internal/application/offering_review_pending.go:(*OfferingReviews).pendingReviews":      100,
	"modules/careplan/internal/application/schedule_reviews.go:(*ScheduleReviews).ListPending":                70,
	"modules/careplan/internal/application/schedule_reviews.go:(*ScheduleReviews).plan":                       66,
	// Moved in by PR #3408 (#3350) from services/users and
	// database/repositories/users; the bodies crossed the boundary at their
	// old length.
	"modules/enrollment/form_schema.go:(*FormField).validateQuestion":                                                                           83,
	"modules/enrollment/form_schema.go:(*FormLegalBlock).Validate":                                                                              67,
	"modules/enrollment/form_schema.go:(*FormSchema).Validate":                                                                                  64,
	"modules/enrollment/internal/adapters/postgres/account_requests.go:(*Store).AccountRequests":                                                93,
	"modules/enrollment/internal/adapters/postgres/request_duplicates.go:(*Store).ActiveDuplicateChildren":                                      68,
	"modules/enrollment/phase.go:(*Phase).Validate":                                                                                             141,
	"modules/careplan/selection/materialize.go:MaterializeAdjustments":                                                                          65,
	"modules/careplan/selection/materialize.go:materializeOfferingSelections":                                                                   95,
	"modules/planexport/betreuungsplan.go:(*betreuungsplanData).rows":                                                                           75,
	"modules/planexport/dienstplan.go:(*dienstplanData).rowsByArea":                                                                             91,
	"modules/schoolcalendar/portal/internal/application/feed.go:(*service).ParentCalendarFeedByToken":                                           124,
	"modules/schoolcalendar/portal/internal/application/ics.go:appointmentICSEvent":                                                             76,
	"modules/schoolcalendar/portal/internal/application/notifications.go:(*service).notifyGuardians":                                            83,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).RecipientOptions":                                                 127,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).RespondToParentInvitation":                                        69,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).createStaffAppointment":                                           90,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).updateStaffAppointment":                                           101,
	"modules/schoolmembership/internal/application/offboarding.go:(*Service).retirementSnapshot":                                                64,
	"modules/schoolstructure/internal/adapters/postgres/transition_store.go:(*Store).ListTransitions":                                           64,
	"modules/schoolstructure/internal/application/transition.go:(*Service).UpdateTransition":                                                    63,
	"modules/studentpresence/inbound/presence/checkin.go:(*Resource).parseAndValidateCheckinRequest":                                            62,
	"modules/studentpresence/inbound/presence/groups_handlers.go:(*Resource).buildVisitDisplayResponses":                                        68,
	"modules/studentpresence/internal/adapters/postgres/group_session.go:(*Store).EndGroupSession":                                              63,
	"modules/studentpresence/internal/adapters/postgres/room_utilization.go:(*Store).RoomUtilization":                                           88,
	"modules/timetable/internal/adapters/postgres/calendar_period_references.go:(*Store).CountCalendarPeriodReferences":                         62,
	"modules/timetable/internal/adapters/postgres/groups.go:(*Store).ListCourseGroups":                                                          71,
	"modules/timetable/internal/adapters/postgres/pickup_extensions.go:(*Store).ListPickupExtensionWeekdayBlocks":                               70,
	"modules/workforce/http/timetracking/staff_admin_documents.go:(*StaffAdminResource).uploadStaffDocument":                                    71,
	"modules/workforce/internal/adapters/postgres/offboarding.go:(*Store).PreviewStaffOffboarding":                                              61,
	"modules/workforce/internal/adapters/postgres/shift_store.go:applyStaffShiftFilter":                                                         63,
	"modules/workforce/internal/adapters/postgres/substitution_store.go:(*Store).ListGroupSubstitutions":                                        66,
	"modules/workforce/internal/planning/staff_schedule_overview.go:(*staffScheduleOverviewService).resolveWeeklyTargets":                       69,
	"modules/workforce/internal/planning/staff_shift_move.go:(*staffShiftService).MoveShift":                                                    124,
	"modules/workforce/internal/planning/staff_shift_series_service.go:(*staffShiftSeriesService).SplitSeries":                                  131,
	"modules/workforce/internal/planning/staff_shift_cancellation.go:(*staffShiftService).ApplyCancellation":                                    223,
	"modules/workforce/internal/planning/staff_shift_service.go:(*staffShiftService).updateShiftWithOptions":                                    148,
	"modules/workforce/internal/application/timetracking/staff_absence_rebooking.go:(*staffAbsenceService).RebookAbsences":                      76,
	"modules/workforce/internal/application/timetracking/staff_absence_creation.go:(*staffAbsenceService).CreateAbsenceFor":                     65,
	"modules/workforce/internal/application/timetracking/staff_absence_comp_time.go:(*staffAbsenceService).PreviewCompTimeBalance":              74,
	"modules/workforce/internal/application/timetracking/staff_absence_mutations.go:(*staffAbsenceService).UpdateAbsence":                       93,
	"modules/workforce/internal/application/timetracking/staff_balance_adjustment_service.go:(*staffBalanceAdjustmentService).CreateAdjustment": 81,
	"modules/workforce/internal/application/timetracking/staff_balance_adjustment_service.go:(*staffBalanceAdjustmentService).DeleteAdjustment": 75,
	"modules/workforce/internal/application/timetracking/staff_balance_adjustment_service.go:(*staffBalanceAdjustmentService).ResetBalance":     97,
	"modules/workforce/internal/application/timetracking/staff_overview_service.go:(*staffOverviewService).GetDashboardSummary":                 77,
	"modules/workforce/internal/application/timetracking/staff_overview_service.go:(*staffOverviewService).GetTimeTrackingOverview":             114,
	"modules/workforce/internal/application/timetracking/staff_overview_service.go:(*staffOverviewService).buildPrefetch":                       95,
	"modules/workforce/internal/application/timetracking/staff_vacation_opening.go:(*staffAbsenceService).SetVacationOpening":                   62,
	"modules/workforce/internal/application/timetracking/work_session_breaks.go:(*workSessionService).AutoEndExpiredBreaks":                     64,
	"modules/workforce/internal/application/timetracking/work_session_checkin.go:(*workSessionService).checkIn":                                 111,
	"modules/workforce/internal/application/timetracking/work_session_checkout.go:(*workSessionService).CheckOutOn":                             63,
	"modules/workforce/internal/application/timetracking/work_session_export.go:(*workSessionService).ExportSessions":                           61,
	"modules/workforce/internal/application/timetracking/work_session_maintenance.go:(*workSessionService).AutoCheckoutDueSessions":             161,
	"modules/workforce/internal/application/timetracking/work_session_updates.go:(*workSessionService).CreateSessionAsAdmin":                    86,
	"modules/workforce/internal/application/timetracking/work_time_month_service.go:(*workTimeMonthService).getMonthSummaryThrough":             106,
}

func TestModuleFunctionLengthRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	lengths, err := moduleFuncLenScan(backendRoot)
	if err != nil {
		t.Fatalf("function-length scan failed: %v", err)
	}

	violations := moduleFuncLenViolations(lengths, moduleFuncLenAllowlist)
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("Module function-length ratchet check failed (%d issue(s)):\n\n%s",
			len(violations), strings.Join(violations, "\n\n"))
	}
}

// moduleFuncLenViolations compares per-function body lengths over the
// threshold against the allowlist. Like the complexity ratchet it cannot reuse
// ratchetViolations: entries are capped lengths rather than hit counts, and an
// allowlisted function falling to or below the threshold leaves the scan
// entirely, which is the ratchet-down signal rather than a deleted file.
func moduleFuncLenViolations(lengths, allowlist map[string]int) []string {
	var violations []string
	for fn, got := range lengths {
		allowed, ok := allowlist[fn]
		switch {
		case !ok:
			violations = append(violations, fmt.Sprintf(
				"[func-length] %s has a %d-line body (limit %d), and is not in the allowlist.\n  Split the function into named steps — do not add new entries.",
				fn, got, moduleFuncLenThreshold))
		case got > allowed:
			violations = append(violations, fmt.Sprintf(
				"[func-length] %s has a %d-line body, allowed %d.\n  The function grew. Extract a step — never raise the allowlist.", fn, got, allowed))
		case got < allowed:
			violations = append(violations, fmt.Sprintf(
				"[func-length] %s has a %d-line body, allowlist says %d.\n  Nice — ratchet the entry down to %d so the improvement cannot regress.", fn, got, allowed, got))
		}
	}
	for fn, allowed := range allowlist {
		if _, ok := lengths[fn]; !ok {
			violations = append(violations, fmt.Sprintf(
				"[func-length] %s is allowlisted at %d lines but is now ≤ %d (or was deleted/renamed).\n  Remove the entry.",
				fn, allowed, moduleFuncLenThreshold))
		}
	}
	return violations
}

// Moved root bindings retain their original coverage and thresholds (ADR 0064).
// moduleFuncLenScan returns the body length of every named function over the
// threshold in non-test .go files under modules/ and workflows/, keyed
// "relpath:FuncName" with methods rendered like the gocognit CLI. Declarations
// without a body (assembly stubs, cgo) are skipped; function literals are not
// counted.
func moduleFuncLenScan(backendRoot string) (map[string]int, error) {
	lengths := make(map[string]int)
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

			fset, file, parseErr := parseGoSourceCached(path, nil)
			if parseErr != nil {
				return fmt.Errorf("parse %s: %w", rel, parseErr)
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				length := fset.Position(fn.Body.Rbrace).Line - fset.Position(fn.Body.Lbrace).Line + 1
				if length > moduleFuncLenThreshold {
					lengths[rel+":"+moduleFuncLenDisplayName(fn)] = length
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return lengths, nil
}

// moduleFuncLenDisplayName renders a function name the way the gocognit CLI
// does: plain functions as "name", methods as "(recv).name" / "(*recv).name".
func moduleFuncLenDisplayName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	return "(" + moduleFuncLenTypeExpr(fn.Recv.List[0].Type) + ")." + fn.Name.Name
}

func moduleFuncLenTypeExpr(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + moduleFuncLenTypeExpr(e.X)
	case *ast.IndexExpr: // generic receiver
		return moduleFuncLenTypeExpr(e.X)
	case *ast.IndexListExpr:
		return moduleFuncLenTypeExpr(e.X)
	default:
		return fmt.Sprintf("%T", expr)
	}
}
