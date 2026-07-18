package test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/uudashr/gocognit"
)

// TestHandlerComplexityRatchet enforces .claude/rules/backend-conventions.md
// Rule 4: no function under api/ may exceed a gocognit cognitive-complexity
// score of 15. The rule was review-only since June 2026 and the violation
// count grew from ~53 to 79 in a month — this ratchet makes it shrink-only,
// like the other ratchets in this package.
//
// Allowlist semantics (per function, not per file):
//
//   - A function NOT in the allowlist must score ≤ 15.
//   - An allowlisted function may never exceed its recorded score.
//   - When refactoring lowers a score (or drops it to ≤ 15), the test fails
//     until the entry is lowered/removed — the ratchet only turns one way.
//     Never raise a number.
//
// Keys are "relative/file.go:FuncName" with methods rendered exactly like the
// gocognit CLI ("(*Resource).applyDeviations"), so `gocognit -over 15 api/`
// output maps 1:1 onto entries here.
const handlerComplexityThreshold = 15

// Seeded 2026-07-12 from `gocognit -over 15 api/` (non-test files only), the
// issue #575 B0 baseline. 75 functions.
var handlerComplexityAllowlist = map[string]int{
	"api/activities/supervisor_handlers.go:buildReplacementSupervisorIDs": 22,
	"api/auth/invitation_handlers.go:(*Resource).createInvitation":        20,
	"api/base.go:(*API).registerRoutesWithRateLimiting":                   20,
	"api/base.go:setupCORS": 18,
	"api/common/student_locations.go:(*StudentLocationSnapshot).ResolveStudentLocationWithTime": 16,
	"api/enrollment/admin_decision_handlers.go:(*Resource).decideAdminChild":                    20,
	"api/enrollment/admin_decision_handlers.go:(*Resource).toAdminRequestDetailSummary":         21,
	"api/enrollment/admin_late_invite_handlers.go:(*Resource).createManualApprovedEnrollment":   20,
	"api/enrollment/admin_late_invite_handlers.go:(*Resource).getManualEnrollmentBootstrap":     30,
	"api/enrollment/care_offering_handlers.go:(*Resource).listPublicCareOfferings":              23,
	"api/enrollment/care_offering_handlers.go:(*Resource).publicFormBootstrap":                  51,
	"api/enrollment/export_format.go:formatContactList":                                         18,
	"api/enrollment/legal_handlers.go:(*Resource).publicLegalTexts":                             23,
	"api/enrollment/phase_handlers.go:(*PhaseRequest).toModel":                                  23,
	"api/enrollment/report_handlers.go:parseCareUsageFiltersFromQuery":                          16,
	"api/enrollment/schema_handlers.go:(*Resource).getSchemaPreviewBootstrap":                   20,
	"api/enrollment/schema_handlers.go:(*Resource).listPublicActiveSchema":                      22,
	"api/enrollment/schema_handlers.go:(*Resource).publishSchema":                               32,
	"api/enrollment/schema_handlers.go:(*Resource).updateSchema":                                19,
	"api/guardians/handlers.go:(*Resource).deleteGuardian":                                      22,
	"api/iot/checkin/attendance_handlers.go:(*AttendanceResource).handleDailyCheckout":          18,
	"api/iot/checkin/handlers.go:(*Resource).deviceCheckin":                                     33,
	"api/iot/checkin/handlers.go:(*Resource).devicePickupQuery":                                 21,
	"api/iot/checkin/helpers.go:(*Resource).isAfterCheckoutTimeGate":                            17,
	"api/operator/api.go:(*Resource).Router":                                                    20,
	"api/operator/settings.go:(*SettingsResource).ResetSchoolSettingValue":                      17,
	"api/operator/settings.go:(*SettingsResource).SetSchoolSettingValue":                        19,
	"api/parent/enrollment_handlers.go:(*Resource).submitParentEnrollment":                      20,
	"api/server.go:NewServer":                                                                  35,
	"api/sse/api.go:(*Resource).eventsHandler":                                                 17,
	"api/sse/sse_helpers.go:(*Resource).resolveSupervisions":                                   16,
	"api/staff/schedule_handlers.go:(*Resource).buildScheduleResponse":                         19,
	"api/staff/staff_handlers.go:(*Resource).getStaff":                                         19,
	"api/staff/substitution_handlers.go:(*Resource).getStaffByRole":                            17,
	"api/staff/schedule_handlers.go:(*Resource).updateSchedule":                                20,
	"api/students/student_handlers.go:(*Resource).createStudent":                               35,
	"api/students/student_handlers.go:(*Resource).fetchStudentsForList":                        53,
	"api/students/student_handlers.go:(*Resource).updateStudent":                               41,
	"api/students/arrival_schedule_handlers.go:(*Resource).createStudentArrivalException":      19,
	"api/students/arrival_schedule_handlers.go:(*Resource).updateStudentArrivalException":      24,
	"api/students/attendance_history_handlers.go:(*Resource).getStudentAttendanceHistory":      17,
	"api/students/attendance_history_handlers.go:buildAttendanceHistoryDays":                   19,
	"api/students/day_planning.go:(*Resource).enrichWithDayPlanning":                           21,
	"api/students/enrollment_extra_fields.go:(*Resource).studentEnrollmentExtraFieldsForChild": 16,
	"api/students/export_handlers.go:(*Resource).loadActiveEnrollmentSummaries":                19,
	"api/students/export_handlers.go:applyExportFilters":                                       20,
	"api/students/pickup_schedule_handlers.go:(*Resource).createStudentPickupException":        19,
	"api/students/pickup_schedule_handlers.go:(*Resource).updateStudentPickupException":        24,
	"api/students/status_day_handlers.go:(*Resource).bulkCreateStudentStatusDays":              34,
	"api/students/status_day_handlers.go:(*Resource).createStudentStatusDays":                  25,
	"api/students/status_day_handlers.go:(*Resource).deleteStudentStatusDay":                   24,
	"api/students/status_days_response.go:applyEffectiveStatusDays":                            21,
	"api/students/types.go:(*StudentRequest).Bind":                                             23,
	"api/students/types.go:(*UpdateStudentRequest).Bind":                                       19,
	"api/timetable/deviations.go:(*Resource).applyDeviations":                                  91,
	"api/timetable/deviations.go:(*Resource).planSubstitutions":                                29,
	"api/timetable/deviations.go:(*Resource).validateDeviationStaff":                           38,
	"api/timetable/exception_conflicts.go:(*Resource).getExceptionConflicts":                   27,
	"api/timetable/exception_conflicts.go:(*Resource).loadArrivalPreload":                      27,
	"api/timetable/gaps.go:(*Resource).getGaps":                                                31,
	"api/timetable/operations.go:(*Resource).operationsCreateAndStartSpontaneous":              17,
	"api/timetable/student_day.go:(*Resource).preloadWeek":                                     28,
	"api/timetable/student_day.go:buildStudentDayFromPreload":                                  19,
	"api/timetable/substitute.go:(*Resource).buildSubstituteTimeConflicts":                     17,
	"api/timetable/templates_create.go:(*Resource).createTemplate":                             33,
	"api/timetable/templates_update.go:(*Resource).updateTemplate":                             27,
	"api/timetable/understaffed.go:(*Resource).acknowledgeUnderstaffed":                        37,
	"api/work-time-models/api.go:buildModelAndEntries":                                         16,
}

func TestHandlerComplexityRatchet(t *testing.T) {
	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	scores, err := scanHandlerComplexity(backendRoot)
	if err != nil {
		t.Fatalf("complexity scan failed: %v", err)
	}

	violations := complexityViolations(scores, handlerComplexityAllowlist)
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("Handler-complexity ratchet check failed (%d issue(s)):\n\n%s",
			len(violations), strings.Join(violations, "\n\n"))
	}
}

// complexityViolations compares per-function scores over the threshold against
// the allowlist. It cannot reuse ratchetViolations: entries here are capped
// scores, not hit counts, and an allowlisted function dropping to or below the
// threshold disappears from the scan entirely (which is the "ratchet down"
// signal, not a stale-entry error in the per-file sense).
func complexityViolations(scores, allowlist map[string]int) []string {
	var violations []string
	for fn, got := range scores {
		allowed, ok := allowlist[fn]
		switch {
		case !ok:
			violations = append(violations, fmt.Sprintf(
				"[gocognit] %s scores %d (limit %d), and is not in the allowlist.\n  Move branching/orchestration into a service method (Rule 4) — do not add new entries.",
				fn, got, handlerComplexityThreshold))
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
				fn, allowed, handlerComplexityThreshold))
		}
	}
	return violations
}

// scanHandlerComplexity returns gocognit scores above the threshold for every
// function in non-test .go files under api/, keyed "relpath:FuncName" with
// methods rendered like the gocognit CLI ("(*Resource).name").
func scanHandlerComplexity(backendRoot string) (map[string]int, error) {
	scores := make(map[string]int)
	apiRoot := filepath.Join(backendRoot, "api")
	err := filepath.WalkDir(apiRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != apiRoot {
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

		fset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return fmt.Errorf("parse %s: %w", rel, parseErr)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if score := gocognit.Complexity(fn); score > handlerComplexityThreshold {
				scores[rel+":"+funcDisplayName(fn)] = score
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return scores, nil
}

// funcDisplayName renders a function name the way the gocognit CLI does:
// plain functions as "name", methods as "(recv).name" / "(*recv).name".
func funcDisplayName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	return "(" + typeExprString(fn.Recv.List[0].Type) + ")." + fn.Name.Name
}

func typeExprString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + typeExprString(e.X)
	case *ast.IndexExpr: // generic receiver
		return typeExprString(e.X)
	case *ast.IndexListExpr:
		return typeExprString(e.X)
	default:
		return fmt.Sprintf("%T", expr)
	}
}
